package store

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
	"gopkg.in/yaml.v3"
)

type randReader struct{}

func (randReader) Read(p []byte) (int, error) { return rand.Read(p) }

var (
	ErrNotFound = errors.New("not found")
	ErrConflict = errors.New("conflict")
	ErrInvalid  = errors.New("invalid")
	timeNow     = func() time.Time { return time.Now().UTC() }
)

// MatchConflictError provides details when a selector matches multiple tasks.
// It still satisfies errors.Is(err, ErrConflict).
type MatchConflictError struct {
	Reason  string
	Matches []Task
}

func (e *MatchConflictError) Error() string {
	if e == nil || strings.TrimSpace(e.Reason) == "" {
		return "conflict"
	}
	return "conflict: " + e.Reason
}

func (e *MatchConflictError) Is(target error) bool {
	return target == ErrConflict
}

type Workspace struct {
	Root string
	cfg  Config
}

type SelectorFilter struct {
	Project         string
	Column          string
	Status          string
	IncludeArchived bool
	Match           string
}

const (
	MatchExact    = "exact"
	MatchPrefix   = "prefix"
	MatchContains = "contains"
	MatchSearch   = "search"
)

type Config struct {
	Schema  int          `json:"schema"`
	Columns []ColumnDef  `json:"columns"`
	Agent   *AgentConfig `json:"agent,omitempty"`
}

type ColumnDef struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Dir    string `json:"dir"`
	Status string `json:"status"` // open|doing|blocked|done|archived
}

type AgentConfig struct {
	RequireExplicit bool   `json:"require_explicit"`
	DefaultProject  string `json:"default_project"`
	DefaultView     string `json:"default_view"` // today|week
	WeekDays        int    `json:"week_days"`
	OpenOnly        bool   `json:"open_only"`
	SummaryGroup    string `json:"summary_group"`  // none|project|column
	SummaryTotals   bool   `json:"summary_totals"` // show per-group counts
}

type Project struct {
	Schema    int       `json:"schema"`
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type TaskMeta struct {
	Schema      int        `yaml:"schema" json:"schema"`
	ID          string     `yaml:"id" json:"id"`
	Title       string     `yaml:"title" json:"title"`
	Status      string     `yaml:"status" json:"status"`
	Project     string     `yaml:"project" json:"project"`
	Column      string     `yaml:"column" json:"column"`
	Priority    string     `yaml:"priority" json:"priority"`
	Tags        []string   `yaml:"tags" json:"tags"`
	Due         string     `yaml:"due" json:"due"`
	CreatedAt   *time.Time `yaml:"created_at" json:"created_at"`
	UpdatedAt   *time.Time `yaml:"updated_at" json:"updated_at"`
	CompletedAt *time.Time `yaml:"completed_at" json:"completed_at"`
	ArchivedAt  *time.Time `yaml:"archived_at" json:"archived_at"`
}

type Task struct {
	TaskMeta `json:",inline"`
	Path     string `json:"path"`
	Body     string `json:"-"`
}

type AddTaskInput struct {
	Title       string
	Project     string
	Column      string
	Due         string
	Priority    string
	Tags        []string
	Description string
}

type ListFilter struct {
	Project string
	Column  string
	Status  string
	Tag     string
	Search  string
	All     bool
}

// Open opens a workspace rooted at root. It does not create files until Init is called.
func Open(root string) (*Workspace, error) {
	ws := &Workspace{Root: expandHome(root)}
	if err := ws.loadOrDefaultConfig(); err != nil {
		// If config doesn't exist, that's ok until Init.
	}
	return ws, nil
}

func (w *Workspace) Init(defaultProject string) error {
	if err := os.MkdirAll(w.Root, 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(w.Root, "projects"), 0o755); err != nil {
		return err
	}

	if err := w.ensureConfig(); err != nil {
		return err
	}

	projectName := strings.TrimSpace(defaultProject)
	if projectName == "" {
		projectName = "Personal"
	}
	// Create default project if none exists.
	_, _ = w.CreateProject(projectName)
	return nil
}

func (w *Workspace) ensureConfig() error {
	cfgPath := filepath.Join(w.Root, "config.json")
	if _, err := os.Stat(cfgPath); err == nil {
		return w.loadOrDefaultConfig()
	}
	w.cfg = defaultConfig()
	b, _ := json.MarshalIndent(w.cfg, "", "  ")
	return atomicWriteFile(cfgPath, b, 0o644)
}

func defaultConfig() Config {
	return Config{
		Schema: 1,
		Columns: []ColumnDef{
			{ID: "inbox", Name: "Inbox", Dir: "00-inbox", Status: "open"},
			{ID: "todo", Name: "To Do", Dir: "01-todo", Status: "open"},
			{ID: "doing", Name: "Doing", Dir: "02-doing", Status: "doing"},
			{ID: "blocked", Name: "Blocked", Dir: "03-blocked", Status: "blocked"},
			{ID: "done", Name: "Done", Dir: "04-done", Status: "done"},
			{ID: "archive", Name: "Archive", Dir: "99-archive", Status: "archived"},
		},
	}
}

func (w *Workspace) loadOrDefaultConfig() error {
	cfgPath := filepath.Join(w.Root, "config.json")
	b, err := os.ReadFile(cfgPath)
	if err != nil {
		w.cfg = defaultConfig()
		return err
	}
	var cfg Config
	if err := json.Unmarshal(b, &cfg); err != nil {
		return err
	}
	if cfg.Schema == 0 {
		cfg.Schema = 1
	}
	if len(cfg.Columns) == 0 {
		cfg.Columns = defaultConfig().Columns
	}
	w.cfg = cfg
	return nil
}

func (w *Workspace) Config() Config {
	return w.cfg
}

func (w *Workspace) SaveConfig(cfg Config) error {
	if cfg.Schema == 0 {
		cfg.Schema = 1
	}
	if len(cfg.Columns) == 0 {
		cfg.Columns = defaultConfig().Columns
	}
	w.cfg = cfg
	b, _ := json.MarshalIndent(cfg, "", "  ")
	cfgPath := filepath.Join(w.Root, "config.json")
	return atomicWriteFile(cfgPath, b, 0o644)
}

func (w *Workspace) CreateProject(name string) (*Project, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("%w: project name is required", ErrInvalid)
	}
	slug := slugify(name)

	projDir := filepath.Join(w.Root, "projects", slug)
	columnsDir := filepath.Join(projDir, "columns")

	if err := os.MkdirAll(columnsDir, 0o755); err != nil {
		return nil, err
	}
	// Ensure column dirs
	for _, c := range w.cfg.Columns {
		if err := os.MkdirAll(filepath.Join(columnsDir, c.Dir), 0o755); err != nil {
			return nil, err
		}
	}

	now := timeNow()
	p := &Project{
		Schema:    1,
		ID:        "prj_" + newULID(),
		Name:      name,
		Slug:      slug,
		CreatedAt: now,
		UpdatedAt: now,
	}
	metaPath := filepath.Join(projDir, "project.json")
	// If exists, do not overwrite; just return existing.
	if _, err := os.Stat(metaPath); err == nil {
		existing, e2 := readProject(metaPath)
		if e2 == nil {
			return existing, nil
		}
	}
	b, _ := json.MarshalIndent(p, "", "  ")
	if err := atomicWriteFile(metaPath, b, 0o644); err != nil {
		return nil, err
	}
	return p, nil
}

func readProject(path string) (*Project, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var p Project
	if err := json.Unmarshal(b, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

func (w *Workspace) ListProjects() ([]Project, error) {
	root := filepath.Join(w.Root, "projects")
	entries, err := os.ReadDir(root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []Project{}, nil
		}
		return nil, err
	}
	var out []Project
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		meta := filepath.Join(root, e.Name(), "project.json")
		p, err := readProject(meta)
		if err != nil {
			// ignore broken project metadata
			continue
		}
		out = append(out, *p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Slug < out[j].Slug })
	return out, nil
}

func (w *Workspace) AddTask(in AddTaskInput) (*Task, error) {
	if strings.TrimSpace(in.Title) == "" {
		return nil, fmt.Errorf("%w: title is required", ErrInvalid)
	}
	projectName := strings.TrimSpace(in.Project)
	if projectName == "" {
		projectName = "Personal"
	}
	projectSlug := slugify(projectName)
	_, err := w.CreateProject(projectName) // idempotent create (name preserved)
	if err != nil {
		return nil, err
	}

	colID := strings.TrimSpace(in.Column)
	if colID == "" {
		colID = "inbox"
	}
	col, ok := w.columnByID(colID)
	if !ok {
		return nil, fmt.Errorf("%w: unknown column %q", ErrInvalid, colID)
	}

	now := timeNow()
	id := "tsk_" + newULID()
	meta := TaskMeta{
		Schema:    1,
		ID:        id,
		Title:     strings.TrimSpace(in.Title),
		Status:    col.Status,
		Project:   projectSlug,
		Column:    colID,
		Priority:  normalizePriority(in.Priority),
		Tags:      dedupeStrings(in.Tags),
		Due:       strings.TrimSpace(in.Due),
		CreatedAt: &now,
		UpdatedAt: &now,
	}
	body := ""
	if strings.TrimSpace(in.Description) != "" {
		body = "## Notes\n\n" + strings.TrimSpace(in.Description) + "\n"
	}

	filename := fmt.Sprintf("%s__%s.md", id, slugify(meta.Title))
	path := filepath.Join(w.projectColumnsDir(projectSlug), col.Dir, filename)

	task := &Task{TaskMeta: meta, Path: path, Body: body}
	if err := writeTaskFile(task); err != nil {
		return nil, err
	}
	return task, nil
}

func (w *Workspace) GetTaskByPrefix(prefix string) (*Task, error) {
	candidates, err := w.findTasksByPrefix(prefix)
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		return nil, ErrNotFound
	}
	if len(candidates) > 1 {
		matches := w.tasksFromPaths(candidates)
		return nil, &MatchConflictError{Reason: "prefix", Matches: matches}
	}
	t, err := readTaskFile(candidates[0])
	if err != nil {
		return nil, err
	}
	w.reconcileTaskFromPath(t)
	return t, nil
}

func (w *Workspace) GetTaskBySelector(selector string, project string) (*Task, error) {
	return w.GetTaskBySelectorFiltered(selector, SelectorFilter{Project: project})
}

func (w *Workspace) GetTaskBySelectorFiltered(selector string, filter SelectorFilter) (*Task, error) {
	selector = strings.TrimSpace(selector)
	if selector == "" {
		return nil, ErrInvalid
	}
	matches, err := w.resolveSelectorCandidates(selector, filter)
	if err != nil {
		return nil, err
	}
	if len(matches) == 0 {
		return nil, ErrNotFound
	}
	if len(matches) == 1 {
		match := matches[0]
		return &match, nil
	}
	return nil, &MatchConflictError{Reason: "selector", Matches: matches}
}

func (w *Workspace) ResolveTasks(selector string, filter SelectorFilter) ([]Task, error) {
	selector = strings.TrimSpace(selector)
	if selector == "" {
		return nil, ErrInvalid
	}
	matches, err := w.resolveSelectorCandidates(selector, filter)
	if err != nil {
		return nil, err
	}
	return matches, nil
}

func (w *Workspace) resolveSelectorCandidates(selector string, filter SelectorFilter) ([]Task, error) {
	filter = normalizeSelectorFilter(filter)
	if isLikelyIDSelector(selector) {
		matches, err := w.findTasksByPrefixFiltered(selector, filter)
		if err != nil {
			return nil, err
		}
		if len(matches) > 0 {
			return matches, nil
		}
		return w.findTasksByMatchMode(selector, filter)
	}
	matches, err := w.findTasksByMatchMode(selector, filter)
	if err != nil {
		return nil, err
	}
	if len(matches) > 0 {
		return matches, nil
	}
	return w.findTasksByPrefixFiltered(selector, filter)
}

func normalizeSelectorFilter(filter SelectorFilter) SelectorFilter {
	project := strings.TrimSpace(filter.Project)
	if project != "" {
		project = slugifyOrDefault(project, project)
	}
	column := strings.TrimSpace(strings.ToLower(filter.Column))
	status := strings.TrimSpace(strings.ToLower(filter.Status))
	includeArchived := filter.IncludeArchived
	if status == "archived" || column == "archive" {
		includeArchived = true
	}
	match := normalizeMatchMode(filter.Match)
	return SelectorFilter{
		Project:         project,
		Column:          column,
		Status:          status,
		IncludeArchived: includeArchived,
		Match:           match,
	}
}

func normalizeMatchMode(match string) string {
	match = strings.TrimSpace(strings.ToLower(match))
	switch match {
	case MatchExact, MatchPrefix, MatchContains, MatchSearch:
		return match
	case "":
		return MatchExact
	default:
		return MatchExact
	}
}

func (w *Workspace) findTasksByMatchMode(selector string, filter SelectorFilter) ([]Task, error) {
	switch filter.Match {
	case MatchSearch:
		return w.findTasksBySearchFiltered(selector, filter)
	case MatchPrefix:
		return w.findTasksByTitlePrefixFiltered(selector, filter)
	case MatchContains:
		return w.findTasksByTitleContainsFiltered(selector, filter)
	case MatchExact:
		return w.findTasksByTitleExactFiltered(selector, filter)
	default:
		return w.findTasksByTitleExactFiltered(selector, filter)
	}
}

func (w *Workspace) findTasksByTitleExactFiltered(selector string, filter SelectorFilter) ([]Task, error) {
	listFilter := ListFilter{
		Project: filter.Project,
		Column:  filter.Column,
		Status:  filter.Status,
		All:     filter.IncludeArchived,
	}
	tasks, err := w.ListTasks(listFilter)
	if err != nil {
		return nil, err
	}
	sel := strings.TrimSpace(selector)
	selLower := strings.ToLower(sel)
	selSlug := slugify(sel)
	var matches []Task
	for _, t := range tasks {
		title := strings.TrimSpace(t.Title)
		if title == "" {
			continue
		}
		if strings.ToLower(title) == selLower || slugify(title) == selSlug {
			matches = append(matches, t)
		}
	}
	return sortSelectorMatches(matches), nil
}

func (w *Workspace) findTasksByTitlePrefixFiltered(selector string, filter SelectorFilter) ([]Task, error) {
	listFilter := ListFilter{
		Project: filter.Project,
		Column:  filter.Column,
		Status:  filter.Status,
		All:     filter.IncludeArchived,
	}
	tasks, err := w.ListTasks(listFilter)
	if err != nil {
		return nil, err
	}
	sel := strings.TrimSpace(selector)
	selLower := strings.ToLower(sel)
	selSlug := slugify(sel)
	var matches []Task
	for _, t := range tasks {
		title := strings.TrimSpace(t.Title)
		if title == "" {
			continue
		}
		titleLower := strings.ToLower(title)
		titleSlug := slugify(title)
		if strings.HasPrefix(titleLower, selLower) || strings.HasPrefix(titleSlug, selSlug) {
			matches = append(matches, t)
		}
	}
	return sortSelectorMatches(matches), nil
}

func (w *Workspace) findTasksByTitleContainsFiltered(selector string, filter SelectorFilter) ([]Task, error) {
	listFilter := ListFilter{
		Project: filter.Project,
		Column:  filter.Column,
		Status:  filter.Status,
		All:     filter.IncludeArchived,
	}
	tasks, err := w.ListTasks(listFilter)
	if err != nil {
		return nil, err
	}
	sel := strings.TrimSpace(selector)
	selLower := strings.ToLower(sel)
	selSlug := slugify(sel)
	var matches []Task
	for _, t := range tasks {
		title := strings.TrimSpace(t.Title)
		if title == "" {
			continue
		}
		titleLower := strings.ToLower(title)
		titleSlug := slugify(title)
		if strings.Contains(titleLower, selLower) || strings.Contains(titleSlug, selSlug) {
			matches = append(matches, t)
		}
	}
	return sortSelectorMatches(matches), nil
}

func (w *Workspace) findTasksBySearchFiltered(selector string, filter SelectorFilter) ([]Task, error) {
	listFilter := ListFilter{
		Project: filter.Project,
		Column:  filter.Column,
		Status:  filter.Status,
		All:     filter.IncludeArchived,
		Search:  strings.TrimSpace(selector),
	}
	tasks, err := w.ListTasks(listFilter)
	if err != nil {
		return nil, err
	}
	return sortSelectorMatches(tasks), nil
}

func (w *Workspace) findTasksByPrefixFiltered(prefix string, filter SelectorFilter) ([]Task, error) {
	paths, err := w.findTasksByPrefix(prefix)
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		return nil, nil
	}
	var matches []Task
	for _, path := range paths {
		t, err := readTaskFile(path)
		if err != nil {
			continue
		}
		w.reconcileTaskFromPath(t)
		if !matchesSelectorFilter(*t, filter) {
			continue
		}
		matches = append(matches, *t)
	}
	return sortSelectorMatches(matches), nil
}

func matchesSelectorFilter(t Task, filter SelectorFilter) bool {
	if filter.Project != "" && t.Project != filter.Project {
		return false
	}
	if filter.Column != "" && t.Column != filter.Column {
		return false
	}
	if filter.Status != "" && t.Status != filter.Status {
		return false
	}
	if !filter.IncludeArchived && t.Status == "archived" {
		return false
	}
	return true
}

func sortSelectorMatches(matches []Task) []Task {
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].Project != matches[j].Project {
			return matches[i].Project < matches[j].Project
		}
		if matches[i].Column != matches[j].Column {
			return matches[i].Column < matches[j].Column
		}
		return strings.ToLower(matches[i].Title) < strings.ToLower(matches[j].Title)
	})
	return matches
}

func (w *Workspace) tasksFromPaths(paths []string) []Task {
	out := make([]Task, 0, len(paths))
	for _, path := range paths {
		t, err := readTaskFile(path)
		if err != nil {
			continue
		}
		w.reconcileTaskFromPath(t)
		out = append(out, *t)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Project != out[j].Project {
			return out[i].Project < out[j].Project
		}
		if out[i].Column != out[j].Column {
			return out[i].Column < out[j].Column
		}
		return strings.ToLower(out[i].Title) < strings.ToLower(out[j].Title)
	})
	return out
}

func isLikelyIDSelector(selector string) bool {
	selector = strings.TrimSpace(selector)
	if selector == "" {
		return false
	}
	lower := strings.ToLower(selector)
	if strings.HasPrefix(lower, "tsk_") {
		return true
	}
	if len(selector) < 8 {
		return false
	}
	allowed := "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
	hasDigit := false
	for _, r := range strings.ToUpper(selector) {
		if r >= '0' && r <= '9' {
			hasDigit = true
		}
		if !strings.ContainsRune(allowed, r) {
			return false
		}
	}
	return hasDigit
}

func (w *Workspace) MoveTask(prefix string, toColumnID string) (*Task, error) {
	task, err := w.GetTaskByPrefix(prefix)
	if err != nil {
		return nil, err
	}
	w.reconcileTaskFromPath(task)
	col, ok := w.columnByID(toColumnID)
	if !ok {
		return nil, fmt.Errorf("%w: unknown column %q", ErrInvalid, toColumnID)
	}
	projectSlug := strings.TrimSpace(task.Project)
	if projectSlug == "" {
		return nil, fmt.Errorf("%w: task project unknown", ErrInvalid)
	}

	// Move file to new directory
	oldPath := task.Path
	newDir := filepath.Join(w.projectColumnsDir(projectSlug), col.Dir)
	if err := os.MkdirAll(newDir, 0o755); err != nil {
		return nil, err
	}
	newPath := filepath.Join(newDir, filepath.Base(oldPath))
	if err := os.Rename(oldPath, newPath); err != nil {
		return nil, err
	}

	now := timeNow()
	task.Path = newPath
	task.Column = toColumnID
	task.Status = col.Status
	task.UpdatedAt = &now
	if col.Status == "done" {
		task.CompletedAt = &now
	} else {
		task.CompletedAt = nil
	}
	if col.Status == "archived" {
		task.ArchivedAt = &now
	} else {
		task.ArchivedAt = nil
	}
	if err := writeTaskFile(task); err != nil {
		return nil, err
	}
	return task, nil
}

func (w *Workspace) AddNote(prefix string, note string) (*Task, error) {
	task, err := w.GetTaskByPrefix(prefix)
	if err != nil {
		return nil, err
	}
	now := timeNow()
	task.UpdatedAt = &now
	entry := fmt.Sprintf("- %s — %s\n", now.Format(time.RFC3339), strings.TrimSpace(note))
	if task.Body == "" {
		task.Body = "## Notes\n\n" + entry
	} else {
		task.Body = strings.TrimRight(task.Body, "\n") + "\n" + entry
	}
	if err := writeTaskFile(task); err != nil {
		return nil, err
	}
	return task, nil
}

func (w *Workspace) ListTasks(f ListFilter) ([]Task, error) {
	var projects []string
	if strings.TrimSpace(f.Project) != "" {
		projects = []string{slugifyOrDefault(f.Project, f.Project)}
	} else {
		ps, err := w.ListProjects()
		if err != nil {
			return nil, err
		}
		for _, p := range ps {
			projects = append(projects, p.Slug)
		}
	}
	var out []Task
	for _, prj := range projects {
		cols := w.cfg.Columns
		for _, c := range cols {
			if !f.All && c.ID == "archive" {
				continue
			}
			if f.Column != "" && c.ID != f.Column {
				continue
			}
			dir := filepath.Join(w.projectColumnsDir(prj), c.Dir)
			_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
				if err != nil || d == nil {
					return nil
				}
				if d.IsDir() {
					return nil
				}
				if !strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
					return nil
				}
				t, err := readTaskFile(path)
				if err != nil {
					return nil
				}
				// reconcile path -> column/status
				t.Project = prj
				t.Column = c.ID
				t.Status = c.Status

				if f.Status != "" && t.Status != f.Status {
					return nil
				}
				if f.Tag != "" && !containsString(t.Tags, f.Tag) {
					return nil
				}
				if f.Search != "" {
					q := strings.ToLower(f.Search)
					if !strings.Contains(strings.ToLower(t.Title), q) && !strings.Contains(strings.ToLower(t.descriptionText()), q) {
						return nil
					}
				}
				out = append(out, *t)
				return nil
			})
		}
	}
	// simple sort: due then updated
	sort.Slice(out, func(i, j int) bool {
		di := out[i].Due
		dj := out[j].Due
		if di != "" && dj != "" && di != dj {
			return di < dj
		}
		if out[i].UpdatedAt != nil && out[j].UpdatedAt != nil {
			return out[i].UpdatedAt.After(*out[j].UpdatedAt)
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

func (w *Workspace) RenderBoard(project string, ascii bool, format string, openOnly bool) (string, error) {
	if isTelegramFormat(format) {
		return w.renderTelegramBoard(project, openOnly)
	}
	projectSlug := slugifyOrDefault(project, project)
	// Collect tasks per column.
	type card struct{ Title, Pri string }
	colCards := map[string][]card{}
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
			title := taskTitle(t.Title)
			title = truncate(title, 80, ascii)
			colCards[c.ID] = append(colCards[c.ID], card{Title: title, Pri: t.PriorityAbbrev()})
		}
	}

	// Simple board rendering (no box drawing; keep it lean).
	var b strings.Builder
	b.WriteString(projectSlug + "\n\n")
	wroteAny := false
	for _, c := range w.cfg.Columns {
		if openOnly && !isOpenStatus(c.Status) {
			continue
		}
		cards := colCards[c.ID]
		if len(cards) == 0 {
			continue
		}
		if wroteAny {
			b.WriteString("\n")
		}
		b.WriteString(c.Name + "\n")
		for _, cd := range cards {
			pri := priorityLabel(cd.Pri)
			b.WriteString(fmt.Sprintf("  - %s%s\n", pri, cd.Title))
		}
		wroteAny = true
	}
	if !wroteAny {
		b.WriteString("(no tasks)\n")
	}
	return b.String(), nil
}

func (w *Workspace) RenderToday(project string, openOnly bool, groupBy string, showTotals bool, format string) (string, error) {
	filter := ListFilter{Project: project, All: false}
	tasks, err := w.ListTasks(filter)
	if err != nil {
		return "", err
	}
	today := timeNow().Format("2006-01-02")
	var dueToday []Task
	var overdue []Task
	for _, t := range tasks {
		if openOnly && !isOpenStatus(t.Status) {
			continue
		}
		dueDate, ok := parseDueDate(t.Due)
		if !ok {
			continue
		}
		d := dueDate.In(time.UTC).Format("2006-01-02")
		if d == today {
			dueToday = append(dueToday, t)
		} else if d < today {
			overdue = append(overdue, t)
		}
	}
	if isTelegramFormat(format) {
		return w.renderTelegramToday(project, today, dueToday, overdue, groupBy, showTotals), nil
	}
	if len(dueToday) == 0 && len(overdue) == 0 {
		return fmt.Sprintf("Today (%s) - nothing due, nothing overdue", today), nil
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Today (%s) - due %d, overdue %d\n\n", today, len(dueToday), len(overdue)))
	writeTaskSection(&b, "Due today", dueToday, groupBy, showTotals, false)
	writeTaskSection(&b, "Overdue", overdue, groupBy, showTotals, true)
	return b.String(), nil
}

func (w *Workspace) RenderAgenda(project string, days int, openOnly bool, groupBy string, showTotals bool, format string) (string, error) {
	filter := ListFilter{Project: project, All: false}
	tasks, err := w.ListTasks(filter)
	if err != nil {
		return "", err
	}
	if days <= 0 {
		days = 7
	}
	start := timeNow().UTC()
	start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 0, days-1)

	var overdue []Task
	byDate := map[string][]Task{}
	for _, t := range tasks {
		if openOnly && !isOpenStatus(t.Status) {
			continue
		}
		dueDate, ok := parseDueDate(t.Due)
		if !ok {
			continue
		}
		d := time.Date(dueDate.Year(), dueDate.Month(), dueDate.Day(), 0, 0, 0, 0, time.UTC)
		if d.Before(start) {
			overdue = append(overdue, t)
			continue
		}
		if d.After(end) {
			continue
		}
		key := d.Format("2006-01-02")
		byDate[key] = append(byDate[key], t)
	}
	if isTelegramFormat(format) {
		return w.renderTelegramAgenda(days, start, end, overdue, byDate, groupBy, showTotals), nil
	}

	var b strings.Builder
	rangeLabel := fmt.Sprintf("%s -> %s", start.Format("2006-01-02"), end.Format("2006-01-02"))
	if lenByDate(byDate) == 0 && len(overdue) == 0 {
		return fmt.Sprintf("Week (%d days) - %s - nothing due, nothing overdue", days, rangeLabel), nil
	}
	b.WriteString(fmt.Sprintf("Week (%d days) - %s - due %d, overdue %d\n\n", days, rangeLabel, lenByDate(byDate), len(overdue)))

	writeTaskSection(&b, "Overdue", overdue, groupBy, showTotals, true)

	for i := 0; i < days; i++ {
		d := start.AddDate(0, 0, i)
		key := d.Format("2006-01-02")
		label := fmt.Sprintf("%s (%s)", key, d.Weekday().String()[:3])
		items := byDate[key]
		writeTaskSection(&b, label, items, groupBy, showTotals, false)
	}
	return b.String(), nil
}

func lenByDate(byDate map[string][]Task) int {
	n := 0
	for _, list := range byDate {
		n += len(list)
	}
	return n
}

func isOpenStatus(status string) bool {
	switch status {
	case "open", "doing", "blocked":
		return true
	default:
		return false
	}
}

func writeTaskSection(b *strings.Builder, title string, tasks []Task, groupBy string, showTotals bool, includeDue bool) {
	if len(tasks) == 0 {
		return
	}
	b.WriteString(title + "\n")
	groupBy = normalizeGroupBy(groupBy)
	if groupBy == "" {
		for _, t := range tasks {
			b.WriteString(formatTaskLine(t, "", includeDue))
		}
		b.WriteString("\n")
		return
	}
	keys, grouped := groupTasks(tasks, groupBy)
	for _, key := range keys {
		header := fmt.Sprintf("%s: %s", strings.Title(groupBy), key)
		if showTotals {
			header = fmt.Sprintf("%s (%d)", header, len(grouped[key]))
		}
		b.WriteString("  " + header + "\n")
		for _, t := range grouped[key] {
			b.WriteString(formatTaskLine(t, groupBy, includeDue))
		}
		b.WriteString("\n")
	}
}

func normalizeGroupBy(groupBy string) string {
	groupBy = strings.TrimSpace(strings.ToLower(groupBy))
	switch groupBy {
	case "", "none":
		return ""
	case "project", "column":
		return groupBy
	default:
		return ""
	}
}

func groupTasks(tasks []Task, groupBy string) ([]string, map[string][]Task) {
	grouped := map[string][]Task{}
	for _, t := range tasks {
		key := ""
		switch groupBy {
		case "project":
			key = t.Project
		case "column":
			key = t.Column
		}
		grouped[key] = append(grouped[key], t)
	}
	keys := make([]string, 0, len(grouped))
	for k := range grouped {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys, grouped
}

func formatTaskLine(t Task, groupBy string, includeDue bool) string {
	due := formatDueSuffix(t.Due, includeDue)
	title := taskTitle(t.Title)
	pri := priorityLabel(t.PriorityAbbrev())
	indent := "  "
	if groupBy != "" {
		indent = "    "
	}
	switch groupBy {
	case "project":
		return fmt.Sprintf("%s- %s%s: %s%s\n", indent, pri, t.Column, title, due)
	case "column":
		return fmt.Sprintf("%s- %s%s: %s%s\n", indent, pri, t.Project, title, due)
	default:
		return fmt.Sprintf("%s- %s%s/%s: %s%s\n", indent, pri, t.Project, t.Column, title, due)
	}
}

func taskTitle(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return "(untitled)"
	}
	return title
}

func formatDueSuffix(due string, includeDue bool) string {
	if !includeDue {
		return ""
	}
	due = strings.TrimSpace(due)
	if due == "" {
		return ""
	}
	return fmt.Sprintf(" (due %s)", due)
}

func priorityLabel(abbrev string) string {
	abbrev = strings.TrimSpace(abbrev)
	if abbrev == "" || abbrev == "N" {
		return ""
	}
	return "[" + abbrev + "] "
}

func parseDueDate(due string) (time.Time, bool) {
	due = strings.TrimSpace(due)
	if due == "" {
		return time.Time{}, false
	}
	if len(due) >= 10 {
		datePart := due[:10]
		if t, err := time.Parse("2006-01-02", datePart); err == nil {
			return t, true
		}
	}
	if t, err := time.Parse(time.RFC3339, due); err == nil {
		return t, true
	}
	return time.Time{}, false
}

func (t *Task) descriptionText() string {
	// In this MVP, body is treated as description text for searching.
	return t.Body
}

func (t *Task) IDShort(n int) string {
	s := t.ID
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func (t *Task) StatusAbbrev() string {
	switch t.Status {
	case "open":
		return "o"
	case "doing":
		return "d"
	case "blocked":
		return "b"
	case "done":
		return "✓"
	case "archived":
		return "a"
	default:
		return "?"
	}
}

func (t *Task) PriorityAbbrev() string {
	switch normalizePriority(t.Priority) {
	case "low":
		return "L"
	case "normal":
		return "N"
	case "high":
		return "H"
	case "urgent":
		return "U"
	default:
		return "?"
	}
}

func (t *Task) RenderHuman() string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%s\n", t.Title))
	b.WriteString(fmt.Sprintf("Project: %s\n", t.Project))
	b.WriteString(fmt.Sprintf("Column: %s\n", t.Column))
	b.WriteString(fmt.Sprintf("Status: %s\n", t.Status))
	b.WriteString(fmt.Sprintf("Priority: %s\n", t.Priority))
	if t.Due != "" {
		b.WriteString(fmt.Sprintf("Due: %s\n", t.Due))
	}
	if len(t.Tags) > 0 {
		b.WriteString(fmt.Sprintf("Tags: %s\n", strings.Join(t.Tags, ", ")))
	}
	b.WriteString("\n")
	if strings.TrimSpace(t.Body) != "" {
		b.WriteString(strings.TrimRight(t.Body, "\n"))
		b.WriteString("\n")
	}
	return b.String()
}

func writeTaskFile(t *Task) error {
	yamlBytes, err := yaml.Marshal(&t.TaskMeta)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	buf.WriteString("---\n")
	buf.Write(yamlBytes)
	buf.WriteString("---\n\n")
	if strings.TrimSpace(t.Body) != "" {
		buf.WriteString(t.Body)
		if !strings.HasSuffix(t.Body, "\n") {
			buf.WriteString("\n")
		}
	}
	return atomicWriteFile(t.Path, buf.Bytes(), 0o644)
}

func readTaskFile(path string) (*Task, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	meta, body, err := parseFrontmatter(b)
	if err != nil {
		return nil, err
	}
	t := &Task{TaskMeta: *meta, Path: path, Body: body}
	return t, nil
}

func parseFrontmatter(b []byte) (*TaskMeta, string, error) {
	s := strings.ReplaceAll(string(b), "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	if !strings.HasPrefix(s, "---\n") {
		// No frontmatter; treat as invalid for v0.1.
		return nil, "", fmt.Errorf("%w: missing frontmatter", ErrInvalid)
	}
	parts := strings.SplitN(s, "\n---\n", 2)
	if len(parts) != 2 {
		return nil, "", fmt.Errorf("%w: invalid frontmatter delimiters", ErrInvalid)
	}
	// parts[0] includes leading ---\n
	yamlPart := strings.TrimPrefix(parts[0], "---\n")
	body := parts[1]
	var meta TaskMeta
	if err := yaml.Unmarshal([]byte(yamlPart), &meta); err != nil {
		return nil, "", err
	}
	if meta.Schema == 0 {
		meta.Schema = 1
	}
	return &meta, body, nil
}

func (w *Workspace) findTasksByPrefix(prefix string) ([]string, error) {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return nil, nil
	}
	prefixNorm := strings.ToUpper(prefix)
	var hits []string
	root := filepath.Join(w.Root, "projects")
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d == nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
			return nil
		}
		// Prefer parsing meta for correctness
		t, err := readTaskFile(path)
		if err != nil {
			return nil
		}
		if strings.HasPrefix(strings.ToUpper(t.ID), prefixNorm) {
			hits = append(hits, path)
		}
		return nil
	})
	sort.Strings(hits)
	return hits, nil
}

func (w *Workspace) projectColumnsDir(projectSlug string) string {
	return filepath.Join(w.Root, "projects", projectSlug, "columns")
}

func (w *Workspace) columnByID(id string) (ColumnDef, bool) {
	id = strings.TrimSpace(strings.ToLower(id))
	for _, c := range w.cfg.Columns {
		if c.ID == id {
			return c, true
		}
	}
	return ColumnDef{}, false
}

func (w *Workspace) columnIDByDir(dir string) (string, bool) {
	dir = strings.TrimSpace(dir)
	for _, c := range w.cfg.Columns {
		if c.Dir == dir {
			return c.ID, true
		}
	}
	return "", false
}

func normalizePriority(p string) string {
	p = strings.TrimSpace(strings.ToLower(p))
	switch p {
	case "low", "l":
		return "low"
	case "normal", "n", "med", "medium":
		return "normal"
	case "high", "h":
		return "high"
	case "urgent", "u", "p0":
		return "urgent"
	default:
		if p == "" {
			return "normal"
		}
		return p
	}
}

func newULID() string {
	t := ulid.Timestamp(timeNow())
	entropy := ulid.Monotonic(randReader{}, 0)
	id, err := ulid.New(t, entropy)
	if err != nil {
		// fallback
		return fmt.Sprintf("%d", timeNow().UnixNano())
	}
	return strings.ToUpper(id.String())
}

func slugifyOrDefault(s, def string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		s = def
	}
	return slugify(s)
}

func slugify(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return "x"
	}
	// Replace non-alnum with hyphen
	var b strings.Builder
	lastHyphen := false
	for _, r := range s {
		isAlnum := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if isAlnum {
			b.WriteRune(r)
			lastHyphen = false
		} else {
			if !lastHyphen {
				b.WriteByte('-')
				lastHyphen = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "x"
	}
	return out
}

func dedupeStrings(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		key := strings.ToLower(s)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

func containsString(list []string, v string) bool {
	v = strings.TrimSpace(strings.ToLower(v))
	for _, s := range list {
		if strings.ToLower(s) == v {
			return true
		}
	}
	return false
}

func truncate(s string, n int, ascii bool) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n <= 3 {
		return string(r[:n])
	}
	if ascii {
		return string(r[:n-2]) + ".."
	}
	// unicode ellipsis
	return string(r[:n-1]) + "…"
}

func (w *Workspace) projectAndColumnFromPath(path string) (string, string) {
	projectsRoot := filepath.Join(w.Root, "projects")
	rel, err := filepath.Rel(projectsRoot, path)
	if err != nil {
		return "", ""
	}
	if strings.HasPrefix(rel, "..") {
		return "", ""
	}
	parts := strings.Split(rel, string(os.PathSeparator))
	if len(parts) < 3 {
		return "", ""
	}
	project := parts[0]
	if parts[1] != "columns" {
		return project, ""
	}
	colDir := parts[2]
	colID, _ := w.columnIDByDir(colDir)
	return project, colID
}

func (w *Workspace) reconcileTaskFromPath(t *Task) {
	if t == nil {
		return
	}
	project, colID := w.projectAndColumnFromPath(t.Path)
	if project != "" {
		t.Project = project
	}
	if colID != "" {
		t.Column = colID
		if col, ok := w.columnByID(colID); ok {
			t.Status = col.Status
		}
	}
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~"+string(os.PathSeparator)) || path == "~" {
		home, _ := os.UserHomeDir()
		if home != "" {
			return filepath.Join(home, strings.TrimPrefix(path, "~"))
		}
	}
	return path
}

func atomicWriteFile(path string, data []byte, perm fs.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp := filepath.Join(dir, fmt.Sprintf(".tmp-%d", timeNow().UnixNano()))
	if err := os.WriteFile(tmp, data, perm); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	// Rename is atomic on same filesystem.
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}
