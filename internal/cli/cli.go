package cli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
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
	case "config", "cfg":
		return cmdConfig(ws, gf, cmdArgs)
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
  init [--project <name>]
  onboarding
  config show
  config set <key> <value>
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
	fmt.Println("  tasker init --project \"Work\"")
	fmt.Println("  tasker add \"Draft proposal\" --project Work --column todo --today --priority high --tag client")
	fmt.Println("  tasker tasks --project Work   # due today + overdue")
	fmt.Println("  tasker board --project Work --ascii")
	fmt.Println()
	fmt.Println("Tip: Use --root or TASKER_ROOT to point to a specific store.")
	fmt.Println("Optional: Add agent defaults in config.json (default project/view, open-only, grouping).")
	fmt.Println("See current config: tasker config show")
	return ExitOK
}

func cmdConfig(ws *store.Workspace, gf GlobalFlags, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: tasker config <show|set> ...")
		return ExitUsage
	}
	sub := args[0]
	switch sub {
	case "show":
		// handled below
	case "set":
		return cmdConfigSet(ws, gf, args[1:])
	default:
		fmt.Fprintln(os.Stderr, "Usage: tasker config <show|set> ...")
		return ExitUsage
	}

	cfg := ws.Config()
	cfgPath := filepath.Join(ws.Root, "config.json")
	_, err := os.Stat(cfgPath)
	exists := err == nil

	payload := map[string]any{
		"root":        ws.Root,
		"config_path": cfgPath,
		"exists":      exists,
		"config":      cfg,
	}

	if gf.NDJSON {
		if gf.StdoutNDJSON {
			b, _ := json.Marshal(payload)
			fmt.Println(string(b))
		} else {
			path, err := writeNDJSONExport(gf, "config", []any{payload})
			if err != nil {
				fmt.Fprintln(os.Stderr, "config show:", err)
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
			_ = enc.Encode(payload)
		} else {
			path, err := writeJSONExport(gf, "config", payload)
			if err != nil {
				fmt.Fprintln(os.Stderr, "config show:", err)
				return ExitInternal
			}
			if !gf.Quiet {
				fmt.Println("Wrote JSON to:", path)
			}
		}
		return ExitOK
	}

	if gf.Plain {
		w := tabwriter.NewWriter(os.Stdout, 2, 4, 2, ' ', 0)
		fmt.Fprintln(w, "KEY\tVALUE")
		fmt.Fprintf(w, "root\t%s\n", ws.Root)
		fmt.Fprintf(w, "config_path\t%s\n", cfgPath)
		fmt.Fprintf(w, "exists\t%t\n", exists)
		if cfg.Agent != nil {
			fmt.Fprintf(w, "agent.require_explicit\t%t\n", cfg.Agent.RequireExplicit)
			fmt.Fprintf(w, "agent.default_project\t%s\n", cfg.Agent.DefaultProject)
			fmt.Fprintf(w, "agent.default_view\t%s\n", cfg.Agent.DefaultView)
			fmt.Fprintf(w, "agent.week_days\t%d\n", cfg.Agent.WeekDays)
			fmt.Fprintf(w, "agent.open_only\t%t\n", cfg.Agent.OpenOnly)
			fmt.Fprintf(w, "agent.summary_group\t%s\n", cfg.Agent.SummaryGroup)
			fmt.Fprintf(w, "agent.summary_totals\t%t\n", cfg.Agent.SummaryTotals)
		} else {
			fmt.Fprintf(w, "agent\t(none)\n")
		}
		for _, c := range cfg.Columns {
			fmt.Fprintf(w, "column.%s\tname=%s dir=%s status=%s\n", c.ID, c.Name, c.Dir, c.Status)
		}
		_ = w.Flush()
		return ExitOK
	}

	fmt.Println("Config")
	fmt.Println("  Root:", ws.Root)
	if exists {
		fmt.Println("  Config file:", cfgPath)
	} else {
		fmt.Println("  Config file:", cfgPath, "(not found; defaults shown)")
	}
	fmt.Println()
	if cfg.Agent == nil {
		fmt.Println("Agent defaults: (not set)")
		fmt.Println("  Add an agent block to config.json to set default view/project and grouping.")
	} else {
		fmt.Println("Agent defaults:")
		fmt.Printf("  require_explicit: %t\n", cfg.Agent.RequireExplicit)
		fmt.Printf("  default_project: %s\n", cfg.Agent.DefaultProject)
		fmt.Printf("  default_view: %s\n", cfg.Agent.DefaultView)
		fmt.Printf("  week_days: %d\n", cfg.Agent.WeekDays)
		fmt.Printf("  open_only: %t\n", cfg.Agent.OpenOnly)
		fmt.Printf("  summary_group: %s\n", cfg.Agent.SummaryGroup)
		fmt.Printf("  summary_totals: %t\n", cfg.Agent.SummaryTotals)
	}
	fmt.Println()
	fmt.Println("Columns:")
	for _, c := range cfg.Columns {
		fmt.Printf("  %s: %s (dir=%s, status=%s)\n", c.ID, c.Name, c.Dir, c.Status)
	}
	return ExitOK
}

func cmdConfigSet(ws *store.Workspace, gf GlobalFlags, args []string) int {
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: tasker config set <key> <value>")
		return ExitUsage
	}
	key := strings.ToLower(strings.TrimSpace(args[0]))
	value := strings.TrimSpace(strings.Join(args[1:], " "))
	cfg := ws.Config()
	if cfg.Agent == nil {
		cfg.Agent = &store.AgentConfig{}
	}

	switch key {
	case "agent.require_explicit":
		v, ok := parseBool(value)
		if !ok {
			return configSetInvalid("agent.require_explicit", value)
		}
		cfg.Agent.RequireExplicit = v
	case "agent.default_project":
		if value == "" || value == "none" || value == "null" {
			cfg.Agent.DefaultProject = ""
		} else {
			cfg.Agent.DefaultProject = value
		}
	case "agent.default_view":
		switch strings.ToLower(value) {
		case "today", "week":
			cfg.Agent.DefaultView = strings.ToLower(value)
		case "", "none", "null":
			cfg.Agent.DefaultView = ""
		default:
			return configSetInvalid("agent.default_view", value)
		}
	case "agent.week_days":
		n, err := strconv.Atoi(value)
		if err != nil || n < 1 {
			return configSetInvalid("agent.week_days", value)
		}
		cfg.Agent.WeekDays = n
	case "agent.open_only":
		v, ok := parseBool(value)
		if !ok {
			return configSetInvalid("agent.open_only", value)
		}
		cfg.Agent.OpenOnly = v
	case "agent.summary_group":
		switch strings.ToLower(value) {
		case "project", "column":
			cfg.Agent.SummaryGroup = strings.ToLower(value)
		case "", "none", "null":
			cfg.Agent.SummaryGroup = ""
		default:
			return configSetInvalid("agent.summary_group", value)
		}
	case "agent.summary_totals":
		v, ok := parseBool(value)
		if !ok {
			return configSetInvalid("agent.summary_totals", value)
		}
		cfg.Agent.SummaryTotals = v
	default:
		fmt.Fprintln(os.Stderr, "Unknown config key:", key)
		fmt.Fprintln(os.Stderr, "Allowed keys: agent.require_explicit, agent.default_project, agent.default_view, agent.week_days, agent.open_only, agent.summary_group, agent.summary_totals")
		return ExitUsage
	}

	if err := ws.SaveConfig(cfg); err != nil {
		fmt.Fprintln(os.Stderr, "config set:", err)
		return ExitInternal
	}
	if !gf.Quiet {
		fmt.Printf("Updated %s\n", key)
	}
	return ExitOK
}

func parseBool(s string) (bool, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "yes", "y", "on":
		return true, true
	case "0", "false", "no", "n", "off":
		return false, true
	default:
		return false, false
	}
}

func configSetInvalid(key, value string) int {
	fmt.Fprintf(os.Stderr, "Invalid value for %s: %q\n", key, value)
	return ExitUsage
}

func cmdInit(ws *store.Workspace, gf GlobalFlags, args []string) int {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	project := fs.String("project", "", "Default project name (optional)")
	if err := fs.Parse(args); err != nil {
		return ExitUsage
	}
	if err := ws.Init(strings.TrimSpace(*project)); err != nil {
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
		"--project":   true,
		"--column":    true,
		"--due":       true,
		"--priority":  true,
		"--tag":       true,
		"--desc":      true,
		"--today":     false,
		"--tomorrow":  false,
		"--next-week": false,
	})
	fs := flag.NewFlagSet("add", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	project := fs.String("project", "", "Project name/slug")
	column := fs.String("column", "inbox", "Column id (inbox|todo|doing|blocked|done|archive)")
	due := fs.String("due", "", "Due date (YYYY-MM-DD) or RFC3339")
	dueToday := fs.Bool("today", false, "Shortcut: due today")
	dueTomorrow := fs.Bool("tomorrow", false, "Shortcut: due tomorrow")
	dueNextWeek := fs.Bool("next-week", false, "Shortcut: due in 7 days")
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
	if strings.TrimSpace(*due) != "" && (*dueToday || *dueTomorrow || *dueNextWeek) {
		fmt.Fprintln(os.Stderr, "Usage: --due cannot be combined with --today/--tomorrow/--next-week")
		return ExitUsage
	}
	if *dueToday && (*dueTomorrow || *dueNextWeek) {
		fmt.Fprintln(os.Stderr, "Usage: choose only one of --today/--tomorrow/--next-week")
		return ExitUsage
	}
	if *dueTomorrow && *dueNextWeek {
		fmt.Fprintln(os.Stderr, "Usage: choose only one of --today/--tomorrow/--next-week")
		return ExitUsage
	}
	now := time.Now().UTC()
	if *dueToday {
		*due = now.Format("2006-01-02")
	}
	if *dueTomorrow {
		*due = now.AddDate(0, 0, 1).Format("2006-01-02")
	}
	if *dueNextWeek {
		*due = now.AddDate(0, 0, 7).Format("2006-01-02")
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
