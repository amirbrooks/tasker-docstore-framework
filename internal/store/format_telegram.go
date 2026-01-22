package store

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const telegramMaxChars = 3800

func isTelegramFormat(format string) bool {
	return strings.ToLower(strings.TrimSpace(format)) == "telegram"
}

func trimTelegramOutput(s string) string {
	s = strings.TrimRight(s, "\n")
	runes := []rune(s)
	if len(runes) <= telegramMaxChars {
		return s
	}
	suffix := "\n… (truncated)"
	suffixRunes := []rune(suffix)
	limit := telegramMaxChars - len(suffixRunes)
	if limit < 1 {
		return string(runes[:telegramMaxChars])
	}
	return string(runes[:limit]) + suffix
}

func telegramPriorityEmoji(priority string) string {
	switch normalizePriority(priority) {
	case "urgent", "high":
		return "🔴"
	case "low":
		return "🟡"
	default:
		return ""
	}
}

func telegramColumnEmoji(id string) string {
	switch strings.ToLower(strings.TrimSpace(id)) {
	case "inbox":
		return "📥"
	case "todo":
		return "📝"
	case "doing":
		return "🔨"
	case "blocked":
		return "⛔"
	case "done":
		return "✅"
	case "archive", "archived":
		return "🗄️"
	default:
		return ""
	}
}

func (w *Workspace) columnDisplayName(id string) string {
	if col, ok := w.columnByID(id); ok {
		name := strings.TrimSpace(col.Name)
		if name != "" {
			return name
		}
	}
	if id == "" {
		return ""
	}
	return strings.Title(id)
}

func (w *Workspace) telegramColumnLabel(id string) string {
	name := w.columnDisplayName(id)
	emoji := telegramColumnEmoji(id)
	if name == "" {
		return emoji
	}
	if emoji == "" {
		return name
	}
	return emoji + " " + name
}

func cleanTaskTitle(title string) string {
	title = strings.ReplaceAll(title, "\n", " ")
	title = strings.ReplaceAll(title, "\r", " ")
	title = strings.TrimSpace(title)
	if title == "" {
		return "(untitled)"
	}
	return title
}

func formatDueShort(due string) string {
	due = strings.TrimSpace(due)
	if due == "" {
		return ""
	}
	if t, ok := parseDueDate(due); ok {
		now := timeNow().UTC()
		if t.Year() == now.Year() {
			return t.Format("Jan 02")
		}
		return t.Format("Jan 02 2006")
	}
	return due
}

func (w *Workspace) telegramContext(groupBy string, t Task) string {
	switch groupBy {
	case "project":
		return w.columnDisplayName(t.Column)
	case "column":
		return strings.TrimSpace(t.Project)
	default:
		parts := make([]string, 0, 2)
		if strings.TrimSpace(t.Project) != "" {
			parts = append(parts, strings.TrimSpace(t.Project))
		}
		if strings.TrimSpace(t.Column) != "" {
			parts = append(parts, w.columnDisplayName(t.Column))
		}
		return strings.Join(parts, "/")
	}
}

func (w *Workspace) telegramTaskLine(t Task, context string, includeDue bool) string {
	var b strings.Builder
	b.WriteString("• ")
	if pri := telegramPriorityEmoji(t.Priority); pri != "" {
		b.WriteString(pri)
		b.WriteString(" ")
	}
	b.WriteString(cleanTaskTitle(t.Title))
	context = strings.TrimSpace(context)
	if context != "" {
		b.WriteString(" — ")
		b.WriteString(context)
	}
	if includeDue {
		if due := formatDueShort(t.Due); due != "" {
			b.WriteString(" (due ")
			b.WriteString(due)
			b.WriteString(")")
		}
	}
	b.WriteString("\n")
	return b.String()
}

func (w *Workspace) telegramTaskLineForGroup(t Task, groupBy string, includeDue bool) string {
	return w.telegramTaskLine(t, w.telegramContext(groupBy, t), includeDue)
}

func (w *Workspace) telegramGroupHeader(groupBy, key string, count int, showTotals bool) string {
	label := strings.TrimSpace(key)
	switch groupBy {
	case "project":
		if label == "" {
			label = "(no project)"
		}
		label = "📁 " + label
	case "column":
		label = w.telegramColumnLabel(key)
	}
	if showTotals {
		return fmt.Sprintf("%s (%d)", label, count)
	}
	return label
}

func (w *Workspace) writeTelegramSection(b *strings.Builder, title string, tasks []Task, groupBy string, showTotals bool, includeDue bool) bool {
	if len(tasks) == 0 {
		return false
	}
	if title != "" {
		b.WriteString(title)
		b.WriteString("\n")
	}
	if groupBy == "" {
		for _, t := range tasks {
			b.WriteString(w.telegramTaskLineForGroup(t, "", includeDue))
		}
		b.WriteString("\n")
		return true
	}
	keys, grouped := groupTasks(tasks, groupBy)
	for _, key := range keys {
		b.WriteString(w.telegramGroupHeader(groupBy, key, len(grouped[key]), showTotals))
		b.WriteString("\n")
		for _, t := range grouped[key] {
			b.WriteString(w.telegramTaskLineForGroup(t, groupBy, includeDue))
		}
	}
	b.WriteString("\n")
	return true
}

func (w *Workspace) renderTelegramBoard(project string, openOnly bool) (string, error) {
	projectSlug := slugifyOrDefault(project, project)
	displayName := strings.TrimSpace(project)
	if displayName == "" {
		displayName = projectSlug
	}

	colTasks := map[string][]Task{}
	for _, c := range w.cfg.Columns {
		if openOnly && !isOpenStatus(c.Status) {
			continue
		}
		dir := filepath.Join(w.projectColumnsDir(projectSlug), c.Dir)
		entries, _ := os.ReadDir(dir)
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".md") {
				continue
			}
			t, err := readTaskFile(filepath.Join(dir, e.Name()))
			if err != nil {
				continue
			}
			colTasks[c.ID] = append(colTasks[c.ID], *t)
		}
		if len(colTasks[c.ID]) > 1 {
			sort.SliceStable(colTasks[c.ID], func(i, j int) bool {
				return strings.ToLower(colTasks[c.ID][i].Title) < strings.ToLower(colTasks[c.ID][j].Title)
			})
		}
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("📋 Tasks — %s\n\n", displayName))

	wrote := false
	for _, c := range w.cfg.Columns {
		if openOnly && !isOpenStatus(c.Status) {
			continue
		}
		tasks := colTasks[c.ID]
		if len(tasks) == 0 {
			continue
		}
		wrote = true
		b.WriteString(w.telegramColumnLabel(c.ID))
		b.WriteString("\n")
		for _, t := range tasks {
			b.WriteString(w.telegramTaskLine(t, "", true))
		}
		b.WriteString("\n")
	}

	if !wrote {
		b.WriteString("No open tasks.\n")
	}
	return trimTelegramOutput(b.String()), nil
}

func (w *Workspace) renderTelegramToday(project string, today string, dueToday []Task, overdue []Task, groupBy string, showTotals bool) string {
	var b strings.Builder
	header := fmt.Sprintf("📅 Today — %s", today)
	if len(dueToday)+len(overdue) > 0 {
		header = fmt.Sprintf("📅 Today — %s (due %d, overdue %d)", today, len(dueToday), len(overdue))
	}
	b.WriteString(header)
	b.WriteString("\n\n")

	wrote := false
	if w.writeTelegramSection(&b, "⏰ Due today", dueToday, groupBy, showTotals, false) {
		wrote = true
	}
	if w.writeTelegramSection(&b, "⚠️ Overdue", overdue, groupBy, showTotals, true) {
		wrote = true
	}

	if !wrote {
		b.WriteString("No tasks due.\n")
	}
	return trimTelegramOutput(b.String())
}

func (w *Workspace) renderTelegramAgenda(days int, start time.Time, end time.Time, overdue []Task, byDate map[string][]Task, groupBy string, showTotals bool) string {
	var b strings.Builder
	header := fmt.Sprintf("📅 Week — %s → %s", start.Format("2006-01-02"), end.Format("2006-01-02"))
	if lenByDate(byDate)+len(overdue) > 0 {
		header = fmt.Sprintf("📅 Week — %s → %s (due %d, overdue %d)", start.Format("2006-01-02"), end.Format("2006-01-02"), lenByDate(byDate), len(overdue))
	}
	b.WriteString(header)
	b.WriteString("\n\n")

	wrote := false
	if w.writeTelegramSection(&b, "⚠️ Overdue", overdue, groupBy, showTotals, true) {
		wrote = true
	}

	for i := 0; i < days; i++ {
		d := start.AddDate(0, 0, i)
		key := d.Format("2006-01-02")
		items := byDate[key]
		if len(items) == 0 {
			continue
		}
		label := fmt.Sprintf("📆 %s (%s)", key, d.Weekday().String()[:3])
		if w.writeTelegramSection(&b, label, items, groupBy, showTotals, false) {
			wrote = true
		}
	}

	if !wrote {
		b.WriteString("No upcoming tasks.\n")
	}
	return trimTelegramOutput(b.String())
}
