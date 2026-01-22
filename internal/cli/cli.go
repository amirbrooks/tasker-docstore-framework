package cli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/amirbrooks/tasker-docstore-framework/internal/store"
)

// Exit codes
const (
	ExitOK       = 0
	ExitUsage    = 2
	ExitNotFound = 3
	ExitConflict = 4
	ExitInternal = 10
)

type GlobalFlags struct {
	Root          string
	JSON          bool
	NDJSON        bool
	Plain         bool
	ASCII         bool
	Quiet         bool
	Verbose       bool
	StdoutJSON    bool
	StdoutNDJSON  bool
	ExportDir     string
	ExportBaseTag string
}

func reorderFlags(args []string, takesValue map[string]bool) []string {
	if len(args) == 0 {
		return args
	}
	var flags []string
	var rest []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			if i+1 < len(args) {
				rest = append(rest, args[i+1:]...)
			}
			break
		}
		if strings.HasPrefix(a, "-") {
			flags = append(flags, a)
			if takesValue[a] && !strings.Contains(a, "=") {
				if i+1 < len(args) {
					flags = append(flags, args[i+1])
					i++
				}
			}
			continue
		}
		rest = append(rest, a)
	}
	return append(flags, rest...)
}

func agentConfig(ws *store.Workspace) *store.AgentConfig {
	cfg := ws.Config()
	if cfg.Agent == nil {
		return nil
	}
	return cfg.Agent
}

func resolveProject(ws *store.Workspace, project string) string {
	if strings.TrimSpace(project) != "" {
		return strings.TrimSpace(project)
	}
	if ac := agentConfig(ws); ac != nil && strings.TrimSpace(ac.DefaultProject) != "" {
		return strings.TrimSpace(ac.DefaultProject)
	}
	return ""
}

func resolveOpenOnly(ws *store.Workspace, openFlag bool, allFlag bool) bool {
	if allFlag {
		return false
	}
	if openFlag {
		return true
	}
	if ac := agentConfig(ws); ac != nil && ac.OpenOnly {
		return true
	}
	return false
}

func resolveWeekDays(ws *store.Workspace, daysFlag int) int {
	if daysFlag > 0 {
		return daysFlag
	}
	if ac := agentConfig(ws); ac != nil && ac.WeekDays > 0 {
		return ac.WeekDays
	}
	return 7
}

func resolveGroupBy(ws *store.Workspace, groupFlag string) string {
	group := strings.ToLower(strings.TrimSpace(groupFlag))
	if group != "" {
		return group
	}
	if ac := agentConfig(ws); ac != nil && strings.TrimSpace(ac.SummaryGroup) != "" {
		return strings.ToLower(strings.TrimSpace(ac.SummaryGroup))
	}
	return ""
}

func resolveShowTotals(ws *store.Workspace, totalsFlag bool) bool {
	if totalsFlag {
		return true
	}
	if ac := agentConfig(ws); ac != nil && ac.SummaryTotals {
		return true
	}
	return false
}
func Run(args []string) int {
	gf, rest, err := extractGlobalFlags(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return ExitUsage
	}

	if len(rest) == 0 {
		printHelp()
		return ExitUsage
	}

	cmd := rest[0]
	cmdArgs := rest[1:]

	ws, err := store.Open(gf.Root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "tasker:", err)
		return ExitInternal
	}

	switch cmd {
	case "help", "--help", "-h":
		printHelp()
		return ExitOK
	case "init":
		return cmdInit(ws, gf, cmdArgs)
	case "onboarding":
		return cmdOnboarding(ws, gf, cmdArgs)
	case "project":
		return cmdProject(ws, gf, cmdArgs)
	case "add":
		return cmdAdd(ws, gf, cmdArgs)
	case "ls", "list":
		return cmdList(ws, gf, cmdArgs)
	case "show":
		return cmdShow(ws, gf, cmdArgs)
	case "mv", "move":
		return cmdMove(ws, gf, cmdArgs)
	case "done":
		return cmdDone(ws, gf, cmdArgs)
	case "note":
		return cmdNote(ws, gf, cmdArgs)
	case "board":
		return cmdBoard(ws, gf, cmdArgs)
	case "today":
		return cmdToday(ws, gf, cmdArgs)
	case "tasks", "summary":
		return cmdTasks(ws, gf, cmdArgs)
	case "week", "agenda", "upcoming":
		return cmdAgenda(ws, gf, cmdArgs)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", cmd)
		printHelp()
		return ExitUsage
	}
}

func printHelp() {
	fmt.Print(`tasker (docstore) — lightweight task manager (no DB)

Usage:
  tasker [global flags] <command> [args]

Global flags:
  --root <path>    Store root (default: ~/.tasker or TASKER_ROOT)
  --json           Write JSON output to <root>/exports (no stdout JSON)
  --ndjson         Write NDJSON output to <root>/exports (no stdout NDJSON)
  --stdout-json    Allow JSON to stdout (debug only)
  --stdout-ndjson  Allow NDJSON to stdout (debug only)
  --export-dir     Override export directory (default: <root>/exports)
  --plain          TSV output
  --ascii          ASCII rendering for board output
  --quiet
  --verbose

Commands:
  init
  onboarding
  project add "<name>"
  project ls
  add "<title>" --project <name> [--column <col>] [--due <date>] [--priority <p>] [--tag <t>...]
  ls [--project <name>] [--column <col>] [--status <s>] [--tag <t>] [--search <q>] [--all]
  show <id-or-prefix>
  mv <id-or-prefix> <column>
  done <id-or-prefix>
  note add <id-or-prefix> "<text>"
  board --project <name>
  today [--project <name>] [--open|--all] [--group project|column|none] [--totals]
  tasks [today|week] [--project <name>] [--days N] [--open|--all] [--group project|column|none] [--totals]
  summary [today|week] [--project <name>] [--days N] [--open|--all] [--group project|column|none] [--totals]
  week [--project <name>] [--days N] [--open|--all] [--group project|column|none] [--totals]
  agenda [--project <name>] [--days N] [--open|--all] [--group project|column|none] [--totals]
  upcoming [--project <name>] [--days N] [--open|--all] [--group project|column|none] [--totals]

Columns:
  inbox|todo|doing|blocked|done|archive
`)
}

func extractGlobalFlags(args []string) (GlobalFlags, []string, error) {
	// Allow flags anywhere by scanning and stripping known globals.
	gf := GlobalFlags{}

	// Default root from env or home.
	if env := os.Getenv("TASKER_ROOT"); env != "" {
		gf.Root = env
	} else {
		home, _ := os.UserHomeDir()
		if home != "" {
			gf.Root = filepath.Join(home, ".tasker")
		} else {
			gf.Root = ".tasker"
		}
	}

	out := make([]string, 0, len(args))
	skip := 0

	for i := 0; i < len(args); i++ {
		if skip > 0 {
			skip--
			continue
		}
		a := args[i]
		switch a {
		case "--root":
			if i+1 >= len(args) {
				return gf, nil, errors.New("--root requires a value")
			}
			gf.Root = args[i+1]
			skip = 1
		case "--json":
			gf.JSON = true
		case "--ndjson":
			gf.NDJSON = true
		case "--stdout-json":
			gf.StdoutJSON = true
		case "--stdout-ndjson":
			gf.StdoutNDJSON = true
		case "--export-dir":
			if i+1 >= len(args) {
				return gf, nil, errors.New("--export-dir requires a value")
			}
			gf.ExportDir = args[i+1]
			skip = 1
		case "--plain":
			gf.Plain = true
		case "--ascii":
			gf.ASCII = true
		case "--quiet":
			gf.Quiet = true
		case "--verbose":
			gf.Verbose = true
		default:
			out = append(out, a)
		}
	}

	if gf.JSON && gf.NDJSON {
		return gf, nil, errors.New("--json and --ndjson are mutually exclusive")
	}
	if gf.StdoutJSON && !gf.JSON {
		return gf, nil, errors.New("--stdout-json requires --json")
	}
	if gf.StdoutNDJSON && !gf.NDJSON {
		return gf, nil, errors.New("--stdout-ndjson requires --ndjson")
	}
	if gf.ExportDir == "" {
		gf.ExportDir = filepath.Join(gf.Root, "exports")
	}
	return gf, out, nil
}

func cmdOnboarding(ws *store.Workspace, gf GlobalFlags, args []string) int {
	fmt.Println("Welcome to tasker (docstore) — local-first tasks in Markdown.")
	fmt.Println()
	fmt.Println("Store root:")
	fmt.Println(" ", ws.Root)
	fmt.Println()
	fmt.Println("Quickstart:")
	fmt.Println("  tasker init")
	fmt.Println("  tasker project add \"Work\"")
	fmt.Println("  tasker add \"Draft proposal\" --project Work --column todo --due 2026-01-23 --priority high --tag client")
	fmt.Println("  tasker tasks --project Work   # due today + overdue")
	fmt.Println("  tasker board --project Work --ascii")
	fmt.Println()
	fmt.Println("Tip: Use --root or TASKER_ROOT to point to a specific store.")
	fmt.Println("Optional: Add agent defaults in config.json (default project/view, open-only, grouping).")
	return ExitOK
}

func cmdInit(ws *store.Workspace, gf GlobalFlags, args []string) int {
	if err := ws.Init(); err != nil {
		fmt.Fprintln(os.Stderr, "init:", err)
		return ExitInternal
	}
	if !gf.Quiet {
		fmt.Println("Initialized tasker store at:", ws.Root)
	}
	return ExitOK
}

func cmdProject(ws *store.Workspace, gf GlobalFlags, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: tasker project <add|ls> ...")
		return ExitUsage
	}
	sub := args[0]
	switch sub {
	case "add":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: tasker project add \"<name>\"")
			return ExitUsage
		}
		name := strings.Join(args[1:], " ")
		p, err := ws.CreateProject(strings.TrimSpace(name))
		if err != nil {
			fmt.Fprintln(os.Stderr, "project add:", err)
			return ExitInternal
		}
		if gf.JSON {
			if gf.StdoutJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				_ = enc.Encode(map[string]any{"project": p})
			} else {
				path, err := writeJSONExport(gf, "project", map[string]any{"project": p})
				if err != nil {
					fmt.Fprintln(os.Stderr, "project add:", err)
					return ExitInternal
				}
				if !gf.Quiet {
					fmt.Println("Wrote JSON to:", path)
				}
			}
		} else {
			fmt.Printf("Created project %s (%s)\n", p.Name, p.Slug)
		}
		return ExitOK
	case "ls", "list":
		projects, err := ws.ListProjects()
		if err != nil {
			fmt.Fprintln(os.Stderr, "project ls:", err)
			return ExitInternal
		}
		if gf.Plain {
			fmt.Fprintln(os.Stdout, "SLUG\tNAME\tUPDATED")
			for _, p := range projects {
				fmt.Fprintf(os.Stdout, "%s\t%s\t%s\n", p.Slug, p.Name, p.UpdatedAt.Format(time.RFC3339))
			}
			return ExitOK
		}
		if gf.JSON {
			if gf.StdoutJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				_ = enc.Encode(map[string]any{"projects": projects})
			} else {
				path, err := writeJSONExport(gf, "projects", map[string]any{"projects": projects})
				if err != nil {
					fmt.Fprintln(os.Stderr, "project ls:", err)
					return ExitInternal
				}
				if !gf.Quiet {
					fmt.Println("Wrote JSON to:", path)
				}
			}
			return ExitOK
		}
		w := tabwriter.NewWriter(os.Stdout, 2, 4, 2, ' ', 0)
		fmt.Fprintln(w, "SLUG	NAME	UPDATED")
		for _, p := range projects {
			fmt.Fprintf(w, "%s	%s	%s\n", p.Slug, p.Name, p.UpdatedAt.Format(time.RFC3339))
		}
		_ = w.Flush()
		return ExitOK
	default:
		fmt.Fprintln(os.Stderr, "Usage: tasker project <add|ls> ...")
		return ExitUsage
	}
}

func cmdAdd(ws *store.Workspace, gf GlobalFlags, args []string) int {
	args = reorderFlags(args, map[string]bool{
		"--project":  true,
		"--column":   true,
		"--due":      true,
		"--priority": true,
		"--tag":      true,
		"--desc":     true,
	})
	fs := flag.NewFlagSet("add", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	project := fs.String("project", "", "Project name/slug")
	column := fs.String("column", "inbox", "Column id (inbox|todo|doing|blocked|done|archive)")
	due := fs.String("due", "", "Due date (YYYY-MM-DD) or RFC3339")
	priority := fs.String("priority", "normal", "Priority (low|normal|high|urgent)")
	searchTag := multiFlag{}
	fs.Var(&searchTag, "tag", "Tag (repeatable)")
	desc := fs.String("desc", "", "Description (short)")
	if err := fs.Parse(args); err != nil {
		return ExitUsage
	}
	rest := fs.Args()
	if len(rest) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: tasker add \"<title>\" --project <name> [--column todo] ...")
		return ExitUsage
	}
	title := strings.Join(rest, " ")
	input := store.AddTaskInput{
		Title:       strings.TrimSpace(title),
		Project:     strings.TrimSpace(*project),
		Column:      strings.TrimSpace(*column),
		Due:         strings.TrimSpace(*due),
		Priority:    strings.TrimSpace(*priority),
		Tags:        searchTag.Values,
		Description: strings.TrimSpace(*desc),
	}
	task, err := ws.AddTask(input)
	if err != nil {
		fmt.Fprintln(os.Stderr, "add:", err)
		return ExitInternal
	}
	if gf.NDJSON {
		if gf.StdoutNDJSON {
			b, _ := json.Marshal(task)
			fmt.Println(string(b))
		} else {
			path, err := writeNDJSONExport(gf, "task", []any{task})
			if err != nil {
				fmt.Fprintln(os.Stderr, "add:", err)
				return ExitInternal
			}
			if !gf.Quiet {
				fmt.Println("Wrote NDJSON to:", path)
			}
		}
		return ExitOK
	}
	if gf.JSON {
		if gf.StdoutJSON {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			_ = enc.Encode(map[string]any{"task": task})
		} else {
			path, err := writeJSONExport(gf, "task", map[string]any{"task": task})
			if err != nil {
				fmt.Fprintln(os.Stderr, "add:", err)
				return ExitInternal
			}
			if !gf.Quiet {
				fmt.Println("Wrote JSON to:", path)
			}
		}
		return ExitOK
	}
	fmt.Printf("%s [%s/%s] %s\n", task.ID, task.Project, task.Column, task.Title)
	return ExitOK
}

func cmdList(ws *store.Workspace, gf GlobalFlags, args []string) int {
	args = reorderFlags(args, map[string]bool{
		"--project": true,
		"--column":  true,
		"--status":  true,
		"--tag":     true,
		"--search":  true,
		"--all":     false,
	})
	fs := flag.NewFlagSet("ls", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	project := fs.String("project", "", "Project name/slug")
	column := fs.String("column", "", "Column id")
	status := fs.String("status", "", "Status (open|doing|blocked|done|archived)")
	tag := fs.String("tag", "", "Filter by tag (single)")
	search := fs.String("search", "", "Search query (title/description)")
	all := fs.Bool("all", false, "Include archive column")
	if err := fs.Parse(args); err != nil {
		return ExitUsage
	}

	filter := store.ListFilter{
		Project: *project,
		Column:  *column,
		Status:  *status,
		Tag:     *tag,
		Search:  *search,
		All:     *all,
	}

	tasks, err := ws.ListTasks(filter)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ls:", err)
		return ExitInternal
	}

	if gf.NDJSON {
		if gf.StdoutNDJSON {
			for _, t := range tasks {
				b, _ := json.Marshal(t)
				fmt.Println(string(b))
			}
		} else {
			items := make([]any, 0, len(tasks))
			for i := range tasks {
				items = append(items, tasks[i])
			}
			path, err := writeNDJSONExport(gf, "tasks", items)
			if err != nil {
				fmt.Fprintln(os.Stderr, "ls:", err)
				return ExitInternal
			}
			if !gf.Quiet {
				fmt.Println("Wrote NDJSON to:", path)
			}
		}
		return ExitOK
	}

	if gf.Plain {
		fmt.Fprintln(os.Stdout, "ID\tST\tPRI\tDUE\tPROJECT/COL\tTITLE")
		for _, t := range tasks {
			dueStr := "-"
			if t.Due != "" {
				dueStr = t.Due
			}
			fmt.Fprintf(os.Stdout, "%s\t%s\t%s\t%s\t%s/%s\t%s\n",
				t.ID, t.StatusAbbrev(), t.PriorityAbbrev(), dueStr, t.Project, t.Column, t.Title)
		}
		return ExitOK
	}

	if gf.JSON {
		if gf.StdoutJSON {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			_ = enc.Encode(map[string]any{"tasks": tasks})
		} else {
			path, err := writeJSONExport(gf, "tasks", map[string]any{"tasks": tasks})
			if err != nil {
				fmt.Fprintln(os.Stderr, "ls:", err)
				return ExitInternal
			}
			if !gf.Quiet {
				fmt.Println("Wrote JSON to:", path)
			}
		}
		return ExitOK
	}

	// Table output
	w := tabwriter.NewWriter(os.Stdout, 2, 4, 2, ' ', 0)
	fmt.Fprintln(w, "ID	ST	PRI	DUE	PROJECT/COL	TITLE")
	for _, t := range tasks {
		dueStr := "-"
		if t.Due != "" {
			dueStr = t.Due
		}
		fmt.Fprintf(w, "%s	%s	%s	%s	%s/%s	%s\n",
			t.ID, t.StatusAbbrev(), t.PriorityAbbrev(), dueStr, t.Project, t.Column, t.Title)
	}
	_ = w.Flush()
	return ExitOK
}

func cmdShow(ws *store.Workspace, gf GlobalFlags, args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: tasker show <id-or-prefix>")
		return ExitUsage
	}
	task, err := ws.GetTaskByPrefix(args[0])
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			fmt.Fprintln(os.Stderr, "show: not found")
			return ExitNotFound
		}
		if errors.Is(err, store.ErrConflict) {
			fmt.Fprintln(os.Stderr, "show: ambiguous id prefix")
			return ExitConflict
		}
		fmt.Fprintln(os.Stderr, "show:", err)
		return ExitInternal
	}
	if gf.JSON {
		if gf.StdoutJSON {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			_ = enc.Encode(map[string]any{"task": task, "body": task.Body})
		} else {
			path, err := writeJSONExport(gf, "task", map[string]any{"task": task, "body": task.Body})
			if err != nil {
				fmt.Fprintln(os.Stderr, "show:", err)
				return ExitInternal
			}
			if !gf.Quiet {
				fmt.Println("Wrote JSON to:", path)
			}
		}
		return ExitOK
	}
	fmt.Println(task.RenderHuman())
	return ExitOK
}

func cmdMove(ws *store.Workspace, gf GlobalFlags, args []string) int {
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: tasker mv <id-or-prefix> <column>")
		return ExitUsage
	}
	task, err := ws.MoveTask(args[0], args[1])
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			fmt.Fprintln(os.Stderr, "mv: not found")
			return ExitNotFound
		}
		if errors.Is(err, store.ErrConflict) {
			fmt.Fprintln(os.Stderr, "mv: ambiguous id prefix")
			return ExitConflict
		}
		fmt.Fprintln(os.Stderr, "mv:", err)
		return ExitInternal
	}
	if gf.JSON {
		if gf.StdoutJSON {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			_ = enc.Encode(map[string]any{"task": task})
		} else {
			path, err := writeJSONExport(gf, "task", map[string]any{"task": task})
			if err != nil {
				fmt.Fprintln(os.Stderr, "mv:", err)
				return ExitInternal
			}
			if !gf.Quiet {
				fmt.Println("Wrote JSON to:", path)
			}
		}
		return ExitOK
	}
	fmt.Printf("Moved %s -> %s\n", task.ID, task.Column)
	return ExitOK
}

func cmdDone(ws *store.Workspace, gf GlobalFlags, args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: tasker done <id-or-prefix>")
		return ExitUsage
	}
	task, err := ws.MoveTask(args[0], "done")
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			fmt.Fprintln(os.Stderr, "done: not found")
			return ExitNotFound
		}
		if errors.Is(err, store.ErrConflict) {
			fmt.Fprintln(os.Stderr, "done: ambiguous id prefix")
			return ExitConflict
		}
		fmt.Fprintln(os.Stderr, "done:", err)
		return ExitInternal
	}
	if gf.JSON {
		if gf.StdoutJSON {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			_ = enc.Encode(map[string]any{"task": task})
		} else {
			path, err := writeJSONExport(gf, "task", map[string]any{"task": task})
			if err != nil {
				fmt.Fprintln(os.Stderr, "done:", err)
				return ExitInternal
			}
			if !gf.Quiet {
				fmt.Println("Wrote JSON to:", path)
			}
		}
		return ExitOK
	}
	fmt.Printf("Done %s\n", task.ID)
	return ExitOK
}

func cmdNote(ws *store.Workspace, gf GlobalFlags, args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: tasker note add <id> \"<text>\"")
		return ExitUsage
	}
	sub := args[0]
	if sub != "add" {
		fmt.Fprintln(os.Stderr, "Usage: tasker note add <id> \"<text>\"")
		return ExitUsage
	}
	if len(args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: tasker note add <id> \"<text>\"")
		return ExitUsage
	}
	id := args[1]
	text := strings.Join(args[2:], " ")
	task, err := ws.AddNote(id, strings.TrimSpace(text))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			fmt.Fprintln(os.Stderr, "note: not found")
			return ExitNotFound
		}
		if errors.Is(err, store.ErrConflict) {
			fmt.Fprintln(os.Stderr, "note: ambiguous id prefix")
			return ExitConflict
		}
		fmt.Fprintln(os.Stderr, "note:", err)
		return ExitInternal
	}
	if gf.JSON {
		if gf.StdoutJSON {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			_ = enc.Encode(map[string]any{"task": task})
		} else {
			path, err := writeJSONExport(gf, "task", map[string]any{"task": task})
			if err != nil {
				fmt.Fprintln(os.Stderr, "note:", err)
				return ExitInternal
			}
			if !gf.Quiet {
				fmt.Println("Wrote JSON to:", path)
			}
		}
		return ExitOK
	}
	fmt.Printf("Noted %s\n", task.ID)
	return ExitOK
}

func cmdBoard(ws *store.Workspace, gf GlobalFlags, args []string) int {
	args = reorderFlags(args, map[string]bool{
		"--project": true,
	})
	fs := flag.NewFlagSet("board", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	project := fs.String("project", "", "Project name/slug")
	if err := fs.Parse(args); err != nil {
		return ExitUsage
	}
	if strings.TrimSpace(*project) == "" {
		fmt.Fprintln(os.Stderr, "Usage: tasker board --project <name>")
		return ExitUsage
	}
	out, err := ws.RenderBoard(strings.TrimSpace(*project), gf.ASCII)
	if err != nil {
		fmt.Fprintln(os.Stderr, "board:", err)
		return ExitInternal
	}
	fmt.Println(out)
	return ExitOK
}

func cmdToday(ws *store.Workspace, gf GlobalFlags, args []string) int {
	args = reorderFlags(args, map[string]bool{
		"--project": true,
		"--open":    false,
		"--all":     false,
		"--group":   true,
		"--totals":  false,
	})
	fs := flag.NewFlagSet("today", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	project := fs.String("project", "", "Project name/slug")
	openOnly := fs.Bool("open", false, "Only open/doing/blocked")
	all := fs.Bool("all", false, "Include done/archived")
	group := fs.String("group", "", "Group by project|column|none")
	totals := fs.Bool("totals", false, "Show per-group totals")
	if err := fs.Parse(args); err != nil {
		return ExitUsage
	}
	rest := fs.Args()
	if len(rest) > 0 {
		if len(rest) == 1 && (rest[0] == "today" || rest[0] == "now") {
			// allow "tasks today" or "summary today"
		} else {
			fmt.Fprintln(os.Stderr, "Usage: tasker today [--project <name>]")
			return ExitUsage
		}
	}
	projectName := resolveProject(ws, *project)
	open := resolveOpenOnly(ws, *openOnly, *all)
	groupBy := resolveGroupBy(ws, *group)
	if groupBy == "none" {
		groupBy = ""
	}
	if groupBy != "" && groupBy != "project" && groupBy != "column" {
		fmt.Fprintln(os.Stderr, "today: invalid --group (use project|column|none)")
		return ExitUsage
	}
	showTotals := resolveShowTotals(ws, *totals)
	out, err := ws.RenderToday(projectName, open, groupBy, showTotals)
	if err != nil {
		fmt.Fprintln(os.Stderr, "today:", err)
		return ExitInternal
	}
	fmt.Println(out)
	return ExitOK
}

func cmdAgenda(ws *store.Workspace, gf GlobalFlags, args []string) int {
	args = reorderFlags(args, map[string]bool{
		"--project": true,
		"--days":    true,
		"--open":    false,
		"--all":     false,
		"--group":   true,
		"--totals":  false,
	})
	fs := flag.NewFlagSet("week", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	project := fs.String("project", "", "Project name/slug")
	days := fs.Int("days", 0, "Days ahead (default 7)")
	openOnly := fs.Bool("open", false, "Only open/doing/blocked")
	all := fs.Bool("all", false, "Include done/archived")
	group := fs.String("group", "", "Group by project|column|none")
	totals := fs.Bool("totals", false, "Show per-group totals")
	if err := fs.Parse(args); err != nil {
		return ExitUsage
	}
	rest := fs.Args()
	if len(rest) > 0 {
		if len(rest) == 1 && (rest[0] == "week" || rest[0] == "this-week" || rest[0] == "next") {
			// allow "week" tokens
		} else {
			fmt.Fprintln(os.Stderr, "Usage: tasker week [--project <name>] [--days N]")
			return ExitUsage
		}
	}
	projectName := resolveProject(ws, *project)
	open := resolveOpenOnly(ws, *openOnly, *all)
	window := resolveWeekDays(ws, *days)
	groupBy := resolveGroupBy(ws, *group)
	if groupBy == "none" {
		groupBy = ""
	}
	if groupBy != "" && groupBy != "project" && groupBy != "column" {
		fmt.Fprintln(os.Stderr, "week: invalid --group (use project|column|none)")
		return ExitUsage
	}
	showTotals := resolveShowTotals(ws, *totals)
	out, err := ws.RenderAgenda(projectName, window, open, groupBy, showTotals)
	if err != nil {
		fmt.Fprintln(os.Stderr, "week:", err)
		return ExitInternal
	}
	fmt.Println(out)
	return ExitOK
}

func cmdTasks(ws *store.Workspace, gf GlobalFlags, args []string) int {
	args = reorderFlags(args, map[string]bool{
		"--project": true,
		"--days":    true,
		"--open":    false,
		"--all":     false,
		"--group":   true,
		"--totals":  false,
	})
	fs := flag.NewFlagSet("tasks", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	project := fs.String("project", "", "Project name/slug")
	days := fs.Int("days", 0, "Days ahead (for week/agenda)")
	openOnly := fs.Bool("open", false, "Only open/doing/blocked")
	all := fs.Bool("all", false, "Include done/archived")
	group := fs.String("group", "", "Group by project|column|none")
	totals := fs.Bool("totals", false, "Show per-group totals")
	if err := fs.Parse(args); err != nil {
		return ExitUsage
	}
	rest := fs.Args()
	mode := ""
	if len(rest) > 0 {
		token := rest[0]
		switch token {
		case "today", "now":
			mode = "today"
		case "week", "this-week", "next", "upcoming", "agenda":
			mode = "week"
		default:
			fmt.Fprintln(os.Stderr, "Usage: tasker tasks [today|week] [--project <name>] [--days N]")
			return ExitUsage
		}
	}
	if mode == "" {
		if ac := agentConfig(ws); ac != nil && strings.ToLower(ac.DefaultView) == "week" {
			mode = "week"
		} else {
			mode = "today"
		}
	}
	projectName := resolveProject(ws, *project)
	open := resolveOpenOnly(ws, *openOnly, *all)
	groupBy := resolveGroupBy(ws, *group)
	if groupBy == "none" {
		groupBy = ""
	}
	if groupBy != "" && groupBy != "project" && groupBy != "column" {
		fmt.Fprintln(os.Stderr, "tasks: invalid --group (use project|column|none)")
		return ExitUsage
	}
	showTotals := resolveShowTotals(ws, *totals)
	if mode == "week" {
		window := resolveWeekDays(ws, *days)
		out, err := ws.RenderAgenda(projectName, window, open, groupBy, showTotals)
		if err != nil {
			fmt.Fprintln(os.Stderr, "tasks:", err)
			return ExitInternal
		}
		fmt.Println(out)
		return ExitOK
	}
	out, err := ws.RenderToday(projectName, open, groupBy, showTotals)
	if err != nil {
		fmt.Fprintln(os.Stderr, "tasks:", err)
		return ExitInternal
	}
	fmt.Println(out)
	return ExitOK
}

// multiFlag supports repeated --tag flags.
type multiFlag struct{ Values []string }

func (m *multiFlag) String() string { return strings.Join(m.Values, ",") }
func (m *multiFlag) Set(v string) error {
	m.Values = append(m.Values, v)
	return nil
}

func writeJSONExport(gf GlobalFlags, base string, payload any) (string, error) {
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "", err
	}
	return writeExportFile(gf.ExportDir, base, "json", data)
}

func writeNDJSONExport(gf GlobalFlags, base string, items []any) (string, error) {
	var b strings.Builder
	for _, item := range items {
		line, err := json.Marshal(item)
		if err != nil {
			return "", err
		}
		b.Write(line)
		b.WriteByte('\n')
	}
	return writeExportFile(gf.ExportDir, base, "ndjson", []byte(b.String()))
}

func writeExportFile(dir, base, ext string, data []byte) (string, error) {
	if strings.TrimSpace(dir) == "" {
		return "", errors.New("export directory is empty")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	t := time.Now().UTC()
	ts := t.Format("20060102-150405")
	name := fmt.Sprintf("%s-%s.%s", base, ts, ext)
	path := filepath.Join(dir, name)
	for i := 1; ; i++ {
		if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
			break
		}
		name = fmt.Sprintf("%s-%s-%d.%s", base, ts, i, ext)
		path = filepath.Join(dir, name)
	}
	tmp := filepath.Join(dir, fmt.Sprintf(".tmp-%d", time.Now().UTC().UnixNano()))
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	return path, nil
}
