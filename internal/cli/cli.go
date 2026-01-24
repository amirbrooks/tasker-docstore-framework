package cli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
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
	Format        string
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

func envString(key string) string {
	return strings.TrimSpace(os.Getenv(key))
}

func envBool(key string) (bool, bool) {
	s := envString(key)
	if s == "" {
		return false, false
	}
	return parseBool(s)
}

func envInt(key string) (int, bool) {
	s := envString(key)
	if s == "" {
		return 0, false
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		return 0, false
	}
	return n, true
}

func resolveProject(ws *store.Workspace, project string) string {
	if strings.TrimSpace(project) != "" {
		return strings.TrimSpace(project)
	}
	if v := envString("TASKER_PROJECT"); v != "" {
		return v
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
	if v, ok := envBool("TASKER_OPEN_ONLY"); ok {
		return v
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
	if v, ok := envInt("TASKER_WEEK_DAYS"); ok {
		return v
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
	if v := envString("TASKER_GROUP"); v != "" {
		return strings.ToLower(v)
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
	if v, ok := envBool("TASKER_TOTALS"); ok {
		return v
	}
	if ac := agentConfig(ws); ac != nil && ac.SummaryTotals {
		return true
	}
	return false
}

func resolveSelectorProject(ws *store.Workspace, project string) string {
	project = strings.TrimSpace(project)
	if project == "" {
		return resolveProject(ws, "")
	}
	switch strings.ToLower(project) {
	case "all", "none":
		return ""
	default:
		return project
	}
}

func normalizeMatchMode(mode string) (string, error) {
	mode = strings.TrimSpace(strings.ToLower(mode))
	switch mode {
	case "", "auto":
		return store.MatchAuto, nil
	case "exact":
		return store.MatchExact, nil
	case "prefix", "starts", "starts-with", "startswith":
		return store.MatchPrefix, nil
	case "contains", "substring", "substr":
		return store.MatchContains, nil
	case "search", "text", "body":
		return store.MatchSearch, nil
	default:
		return "", fmt.Errorf("unknown --match %q (use auto|exact|prefix|contains|search)", mode)
	}
}

func selectorFilter(ws *store.Workspace, project string, column string, status string, includeArchived bool, match string) (store.SelectorFilter, error) {
	matchMode, err := normalizeMatchMode(match)
	if err != nil {
		return store.SelectorFilter{}, err
	}
	return store.SelectorFilter{
		Project:         resolveSelectorProject(ws, project),
		Column:          strings.TrimSpace(column),
		Status:          strings.TrimSpace(status),
		IncludeArchived: includeArchived,
		Match:           matchMode,
	}, nil
}

func resolveIdeaScope(scopeFlag string, project string) (string, error) {
	scope := strings.TrimSpace(strings.ToLower(scopeFlag))
	if scope != "" {
		switch scope {
		case store.IdeaScopeRoot, store.IdeaScopeProject, store.IdeaScopeAll:
			// ok
		default:
			return "", fmt.Errorf("invalid --scope (use root|project|all)")
		}
	}
	if scope == "" {
		if strings.TrimSpace(project) != "" {
			return store.IdeaScopeProject, nil
		}
		return store.IdeaScopeRoot, nil
	}
	if scope == store.IdeaScopeProject && strings.TrimSpace(project) == "" {
		return "", errors.New("--scope project requires --project")
	}
	return scope, nil
}

func ideaSelectorFilter(project string, scope string, match string) (store.IdeaSelectorFilter, error) {
	matchMode, err := normalizeMatchMode(match)
	if err != nil {
		return store.IdeaSelectorFilter{}, err
	}
	scopeValue, err := resolveIdeaScope(scope, project)
	if err != nil {
		return store.IdeaSelectorFilter{}, err
	}
	return store.IdeaSelectorFilter{
		Project: strings.TrimSpace(project),
		Scope:   scopeValue,
		Match:   matchMode,
	}, nil
}

func handleMatchConflict(cmd string, err error) bool {
	var mc *store.MatchConflictError
	if !errors.As(err, &mc) {
		return false
	}
	if len(mc.Matches) == 0 {
		fmt.Fprintf(os.Stderr, "%s: ambiguous selector\n", cmd)
		return true
	}
	fmt.Fprintf(os.Stderr, "%s: multiple tasks match\n", cmd)
	for _, t := range mc.Matches {
		title := strings.TrimSpace(t.Title)
		if title == "" {
			title = "(untitled)"
		}
		due := ""
		if strings.TrimSpace(t.Due) != "" {
			due = fmt.Sprintf(" (due %s)", strings.TrimSpace(t.Due))
		}
		loc := t.Project
		if t.Column != "" {
			loc = loc + "/" + t.Column
		}
		fmt.Fprintf(os.Stderr, "  - %s: %s%s\n", loc, title, due)
	}
	fmt.Fprintln(os.Stderr, "Tip: use a more specific title, pass --project/--column/--status/--match, or quote multi-word selectors.")
	return true
}

func handleIdeaMatchConflict(cmd string, err error) bool {
	var mc *store.IdeaMatchConflictError
	if !errors.As(err, &mc) {
		return false
	}
	if len(mc.Matches) == 0 {
		fmt.Fprintf(os.Stderr, "%s: ambiguous selector\n", cmd)
		return true
	}
	fmt.Fprintf(os.Stderr, "%s: multiple ideas match\n", cmd)
	for _, idea := range mc.Matches {
		title := strings.TrimSpace(idea.Title)
		if title == "" {
			title = "(untitled)"
		}
		loc := strings.TrimSpace(idea.Project)
		if loc == "" {
			loc = "root"
		}
		fmt.Fprintf(os.Stderr, "  - %s: %s\n", loc, title)
	}
	fmt.Fprintln(os.Stderr, "Tip: use a more specific selector, --scope/--project, or --match.")
	return true
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
	case "idea", "ideas":
		return cmdIdea(ws, gf, cmdArgs)
	case "add":
		return cmdAdd(ws, gf, cmdArgs)
	case "capture":
		return cmdCapture(ws, gf, cmdArgs)
	case "ls", "list":
		return cmdList(ws, gf, cmdArgs)
	case "show":
		return cmdShow(ws, gf, cmdArgs)
	case "resolve":
		return cmdResolve(ws, gf, cmdArgs)
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
  --format <f>     Output format: human|telegram (default: human)
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
  idea add "<title>" [--project <name>] [--body <text>] [--tag <t>...] [--stdin]
  idea add --text "<title | details | #tag>" [--project <name>] [--stdin]
  idea capture "<title | details | #tag>" [--project <name>] [--stdin]
  idea ls [--scope root|project|all] [--project <name>] [--tag <t>] [--search <q>]
  idea show [--scope root|project|all] [--project <name>] [--match <m>] <selector...>
  idea resolve [--scope root|project|all] [--project <name>] [--match <m>] <selector...>
  idea note add [--scope root|project|all] [--project <name>] [--match <m>] <selector...> -- <text...>
  idea append [--scope root|project|all] [--project <name>] [--match <m>] <selector...> -- <text...>
  idea promote [--scope root|project|all] [--project <name>] [--to-project <name>] [--column <col>] [--due <date>] [--priority <p>] [--tag <t>...] [--link] [--delete] <selector...>
  add "<title>" --project <name> [--column <col>] [--due <date>] [--priority <p>] [--tag <t>...] [--desc <text>|--details <text>]
  add --text "<title | details | due 2026-01-23>" --project <name> [--column <col>] [--priority <p>] [--tag <t>...]
  capture "<title | details | due 2026-01-23>" [--project <name>] [--column <col>] [--priority <p>] [--tag <t>...]
  ls [--project <name>] [--column <col>] [--status <s>] [--tag <t>] [--search <q>] [--all]
  show [--project <name>|none|all] [--column <col>] [--status <s>] [--all] [--match <m>] <selector...>
  resolve [--project <name>|none|all] [--column <col>] [--status <s>] [--all] [--match <m>] <selector...>
  mv [--project <name>|none|all] [--column <col>] [--status <s>] [--all] [--match <m>] <selector...> <column>
  done [--project <name>|none|all] [--column <col>] [--status <s>] [--all] [--match <m>] <selector...>
  note add [--project <name>|none|all] [--column <col>] [--status <s>] [--all] [--match <m>] <selector...> -- <text...>
  board --project <name> [--open|--all]
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
	gf.Format = "human"

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
		case "--format":
			if i+1 >= len(args) {
				return gf, nil, errors.New("--format requires a value")
			}
			gf.Format = args[i+1]
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
	format, err := normalizeFormat(gf.Format)
	if err != nil {
		return gf, nil, err
	}
	gf.Format = format
	if gf.ExportDir == "" {
		gf.ExportDir = filepath.Join(gf.Root, "exports")
	}
	return gf, out, nil
}

func normalizeFormat(format string) (string, error) {
	format = strings.ToLower(strings.TrimSpace(format))
	switch format {
	case "", "human":
		return "human", nil
	case "telegram", "tg":
		return "telegram", nil
	default:
		return "", fmt.Errorf("unknown --format %q (use human or telegram)", format)
	}
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

func cmdIdea(ws *store.Workspace, gf GlobalFlags, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: tasker idea <add|capture|ls|show|resolve|note|append|promote> ...")
		return ExitUsage
	}
	sub := args[0]
	switch sub {
	case "add":
		return cmdIdeaAdd(ws, gf, args[1:])
	case "capture":
		return cmdIdeaCapture(ws, gf, args[1:])
	case "ls", "list":
		return cmdIdeaList(ws, gf, args[1:])
	case "show":
		return cmdIdeaShow(ws, gf, args[1:])
	case "resolve":
		return cmdIdeaResolve(ws, gf, args[1:])
	case "note":
		return cmdIdeaNote(ws, gf, args[1:])
	case "append":
		return cmdIdeaAppend(ws, gf, args[1:])
	case "promote":
		return cmdIdeaPromote(ws, gf, args[1:])
	default:
		fmt.Fprintln(os.Stderr, "Usage: tasker idea <add|capture|ls|show|resolve|note|append|promote> ...")
		return ExitUsage
	}
}

func cmdIdeaAdd(ws *store.Workspace, gf GlobalFlags, args []string) int {
	args = reorderFlags(args, map[string]bool{
		"--project": true,
		"--body":    true,
		"--text":    true,
		"--tag":     true,
		"--stdin":   false,
	})
	fs := flag.NewFlagSet("idea add", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	project := fs.String("project", "", "Project name/slug (optional)")
	body := fs.String("body", "", "Body text")
	text := fs.String("text", "", "Raw input using \" | \" separators")
	stdin := fs.Bool("stdin", false, "Read idea content from stdin")
	searchTag := multiFlag{}
	fs.Var(&searchTag, "tag", "Tag (repeatable)")
	if err := fs.Parse(args); err != nil {
		return ExitUsage
	}
	rest := fs.Args()
	if len(rest) == 1 && rest[0] == "-" {
		*stdin = true
		rest = rest[:0]
	}
	textValue := strings.TrimSpace(*text)
	if textValue != "" && strings.TrimSpace(*body) != "" {
		fmt.Fprintln(os.Stderr, "Usage: choose only one of --text or --body")
		return ExitUsage
	}
	if *stdin && (textValue != "" || strings.TrimSpace(*body) != "") {
		fmt.Fprintln(os.Stderr, "Usage: --stdin cannot be combined with --text or --body")
		return ExitUsage
	}
	if textValue != "" && len(rest) > 0 {
		fmt.Fprintln(os.Stderr, "Usage: provide either --text or a title, not both")
		return ExitUsage
	}
	if !*stdin && textValue == "" && len(rest) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: tasker idea add \"<title>\" [--project <name>] [--body <text>]")
		return ExitUsage
	}
	var title string
	var bodyText string
	var tags []string
	projectName := strings.TrimSpace(*project)
	if *stdin {
		stdinText, err := readStdinText()
		if err != nil {
			fmt.Fprintln(os.Stderr, "idea add:", err)
			return ExitUsage
		}
		if len(rest) > 0 {
			rawTitle := strings.Join(rest, " ")
			cleanTitle, projectFromText, inlineTags := extractIdeaInlineTokens(rawTitle, true, true)
			if strings.TrimSpace(cleanTitle) == "" {
				fmt.Fprintln(os.Stderr, "Usage: tasker idea add \"<title>\" [--project <name>] [--body <text>]")
				return ExitUsage
			}
			parsedTitle, parsedTags, parsedBody := store.ParseIdeaContent(cleanTitle + "\n" + stdinText)
			title = strings.TrimSpace(parsedTitle)
			bodyText = parsedBody
			tags = append(tags, parsedTags...)
			tags = append(tags, inlineTags...)
			if projectName == "" {
				projectName = projectFromText
			}
		} else {
			parsedTitle, parsedTags, parsedBody := store.ParseIdeaContent(stdinText)
			cleanTitle, projectFromText, inlineTags := extractIdeaInlineTokens(parsedTitle, true, true)
			title = strings.TrimSpace(cleanTitle)
			bodyText = parsedBody
			tags = append(tags, parsedTags...)
			tags = append(tags, inlineTags...)
			if projectName == "" {
				projectName = projectFromText
			}
		}
		if strings.TrimSpace(title) == "" {
			fmt.Fprintln(os.Stderr, "Usage: tasker idea add \"<title>\" [--project <name>] [--body <text>]")
			return ExitUsage
		}
		tags = append(tags, searchTag.Values...)
	} else if textValue != "" {
		var projectFromText string
		title, bodyText, tags, projectFromText = parseIdeaTextParts(textValue)
		if strings.TrimSpace(title) == "" {
			fmt.Fprintln(os.Stderr, "Usage: idea add --text \"<title | details | #tag>\" [--project <name>]")
			return ExitUsage
		}
		if projectName == "" {
			projectName = projectFromText
		}
		tags = append(tags, searchTag.Values...)
	} else {
		rawTitle := strings.Join(rest, " ")
		cleanTitle, projectFromText, inlineTags := extractIdeaInlineTokens(rawTitle, true, true)
		title = strings.TrimSpace(cleanTitle)
		bodyText = strings.TrimSpace(*body)
		tags = append(tags, inlineTags...)
		tags = append(tags, searchTag.Values...)
		if projectName == "" {
			projectName = projectFromText
		}
		if title == "" {
			fmt.Fprintln(os.Stderr, "Usage: tasker idea add \"<title>\" [--project <name>] [--body <text>]")
			return ExitUsage
		}
	}
	input := store.AddIdeaInput{
		Title:   strings.TrimSpace(title),
		Project: strings.TrimSpace(projectName),
		Body:    bodyText,
		Tags:    tags,
	}
	idea, err := ws.AddIdea(input)
	if err != nil {
		fmt.Fprintln(os.Stderr, "idea add:", err)
		return ExitInternal
	}
	return emitIdeaAddResult(gf, idea)
}

func cmdIdeaCapture(ws *store.Workspace, gf GlobalFlags, args []string) int {
	args = reorderFlags(args, map[string]bool{
		"--project": true,
		"--text":    true,
		"--tag":     true,
		"--stdin":   false,
	})
	fs := flag.NewFlagSet("idea capture", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	project := fs.String("project", "", "Project name/slug (optional)")
	text := fs.String("text", "", "Raw input using \" | \" separators")
	stdin := fs.Bool("stdin", false, "Read idea content from stdin")
	searchTag := multiFlag{}
	fs.Var(&searchTag, "tag", "Tag (repeatable)")
	if err := fs.Parse(args); err != nil {
		return ExitUsage
	}
	rest := fs.Args()
	if len(rest) == 1 && rest[0] == "-" {
		*stdin = true
		rest = rest[:0]
	}
	textValue := strings.TrimSpace(*text)
	if *stdin && textValue != "" {
		fmt.Fprintln(os.Stderr, "Usage: --stdin cannot be combined with --text")
		return ExitUsage
	}
	if textValue != "" && len(rest) > 0 {
		fmt.Fprintln(os.Stderr, "Usage: provide either --text or capture text, not both")
		return ExitUsage
	}
	projectName := strings.TrimSpace(*project)
	var title string
	var bodyText string
	var tags []string
	if *stdin {
		stdinText, err := readStdinText()
		if err != nil {
			fmt.Fprintln(os.Stderr, "idea capture:", err)
			return ExitUsage
		}
		if len(rest) > 0 {
			rawTitle := strings.Join(rest, " ")
			cleanTitle, projectFromText, inlineTags := extractIdeaInlineTokens(rawTitle, true, true)
			if strings.TrimSpace(cleanTitle) == "" {
				fmt.Fprintln(os.Stderr, "Usage: tasker idea capture \"<title | details | #tag>\" [--project <name>]")
				return ExitUsage
			}
			parsedTitle, parsedTags, parsedBody := store.ParseIdeaContent(cleanTitle + "\n" + stdinText)
			title = strings.TrimSpace(parsedTitle)
			bodyText = parsedBody
			tags = append(tags, parsedTags...)
			tags = append(tags, inlineTags...)
			if projectName == "" {
				projectName = projectFromText
			}
		} else {
			parsedTitle, parsedTags, parsedBody := store.ParseIdeaContent(stdinText)
			cleanTitle, projectFromText, inlineTags := extractIdeaInlineTokens(parsedTitle, true, true)
			title = strings.TrimSpace(cleanTitle)
			bodyText = parsedBody
			tags = append(tags, parsedTags...)
			tags = append(tags, inlineTags...)
			if projectName == "" {
				projectName = projectFromText
			}
		}
	} else {
		if textValue == "" {
			textValue = strings.Join(rest, " ")
		}
		if strings.TrimSpace(textValue) == "" {
			fmt.Fprintln(os.Stderr, "Usage: tasker idea capture \"<title | details | #tag>\" [--project <name>]")
			return ExitUsage
		}
		var projectFromText string
		title, bodyText, tags, projectFromText = parseIdeaTextParts(textValue)
		if projectName == "" {
			projectName = projectFromText
		}
	}
	if strings.TrimSpace(title) == "" {
		fmt.Fprintln(os.Stderr, "Usage: tasker idea capture \"<title | details | #tag>\" [--project <name>]")
		return ExitUsage
	}
	tags = append(tags, searchTag.Values...)
	input := store.AddIdeaInput{
		Title:   strings.TrimSpace(title),
		Project: strings.TrimSpace(projectName),
		Body:    bodyText,
		Tags:    tags,
	}
	idea, err := ws.AddIdea(input)
	if err != nil {
		fmt.Fprintln(os.Stderr, "idea capture:", err)
		return ExitInternal
	}
	return emitIdeaAddResult(gf, idea)
}

func cmdIdeaList(ws *store.Workspace, gf GlobalFlags, args []string) int {
	args = reorderFlags(args, map[string]bool{
		"--scope":   true,
		"--project": true,
		"--tag":     true,
		"--search":  true,
	})
	fs := flag.NewFlagSet("idea ls", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	scope := fs.String("scope", "", "Scope (root|project|all)")
	project := fs.String("project", "", "Project name/slug")
	tag := fs.String("tag", "", "Filter by tag (single)")
	search := fs.String("search", "", "Search query (title/body)")
	if err := fs.Parse(args); err != nil {
		return ExitUsage
	}
	scopeValue, err := resolveIdeaScope(*scope, *project)
	if err != nil {
		fmt.Fprintln(os.Stderr, "idea ls:", err)
		return ExitUsage
	}
	filter := store.IdeaListFilter{
		Project: *project,
		Scope:   scopeValue,
		Tag:     *tag,
		Search:  *search,
	}
	ideas, err := ws.ListIdeas(filter)
	if err != nil {
		fmt.Fprintln(os.Stderr, "idea ls:", err)
		return ExitInternal
	}
	if gf.NDJSON {
		if gf.StdoutNDJSON {
			for _, idea := range ideas {
				b, _ := json.Marshal(idea)
				fmt.Println(string(b))
			}
		} else {
			items := make([]any, 0, len(ideas))
			for i := range ideas {
				items = append(items, ideas[i])
			}
			path, err := writeNDJSONExport(gf, "ideas", items)
			if err != nil {
				fmt.Fprintln(os.Stderr, "idea ls:", err)
				return ExitInternal
			}
			if !gf.Quiet {
				fmt.Println("Wrote NDJSON to:", path)
			}
		}
		return ExitOK
	}
	if gf.Plain {
		fmt.Fprintln(os.Stdout, "ID\tSCOPE\tPROJECT\tTITLE\tTAGS")
		for _, idea := range ideas {
			scopeLabel := "root"
			projectLabel := "-"
			if idea.Project != "" {
				scopeLabel = "project"
				projectLabel = idea.Project
			}
			fmt.Fprintf(os.Stdout, "%s\t%s\t%s\t%s\t%s\n",
				idea.ID, scopeLabel, projectLabel, idea.Title, strings.Join(idea.Tags, ","))
		}
		return ExitOK
	}
	if gf.JSON {
		if gf.StdoutJSON {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			_ = enc.Encode(map[string]any{"ideas": ideas})
		} else {
			path, err := writeJSONExport(gf, "ideas", map[string]any{"ideas": ideas})
			if err != nil {
				fmt.Fprintln(os.Stderr, "idea ls:", err)
				return ExitInternal
			}
			if !gf.Quiet {
				fmt.Println("Wrote JSON to:", path)
			}
		}
		return ExitOK
	}
	for _, idea := range ideas {
		fmt.Fprintln(os.Stdout, formatIdeaListBullet(idea))
	}
	return ExitOK
}

func cmdIdeaShow(ws *store.Workspace, gf GlobalFlags, args []string) int {
	args = reorderFlags(args, map[string]bool{
		"--scope":   true,
		"--project": true,
		"--match":   true,
	})
	fs := flag.NewFlagSet("idea show", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	scope := fs.String("scope", "", "Scope (root|project|all)")
	project := fs.String("project", "", "Project name/slug")
	match := fs.String("match", "auto", "Match mode (auto|exact|prefix|contains|search)")
	if err := fs.Parse(args); err != nil {
		return ExitUsage
	}
	rest := fs.Args()
	if len(rest) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: tasker idea show [--scope root|project|all] [--project <name>] [--match <m>] <selector>")
		return ExitUsage
	}
	selector := strings.Join(rest, " ")
	filter, err := ideaSelectorFilter(*project, *scope, *match)
	if err != nil {
		fmt.Fprintln(os.Stderr, "idea show:", err)
		return ExitUsage
	}
	idea, err := ws.GetIdeaBySelectorFiltered(selector, filter)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			fmt.Fprintln(os.Stderr, "idea show: not found")
			return ExitNotFound
		}
		if errors.Is(err, store.ErrConflict) {
			if handleIdeaMatchConflict("idea show", err) {
				return ExitConflict
			}
			fmt.Fprintln(os.Stderr, "idea show: ambiguous selector")
			return ExitConflict
		}
		fmt.Fprintln(os.Stderr, "idea show:", err)
		return ExitInternal
	}
	if gf.JSON {
		if gf.StdoutJSON {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			_ = enc.Encode(map[string]any{"idea": idea, "body": idea.Body})
		} else {
			path, err := writeJSONExport(gf, "idea", map[string]any{"idea": idea, "body": idea.Body})
			if err != nil {
				fmt.Fprintln(os.Stderr, "idea show:", err)
				return ExitInternal
			}
			if !gf.Quiet {
				fmt.Println("Wrote JSON to:", path)
			}
		}
		return ExitOK
	}
	fmt.Println(idea.RenderHuman())
	return ExitOK
}

func cmdIdeaResolve(ws *store.Workspace, gf GlobalFlags, args []string) int {
	args = reorderFlags(args, map[string]bool{
		"--scope":   true,
		"--project": true,
		"--match":   true,
	})
	fs := flag.NewFlagSet("idea resolve", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	scope := fs.String("scope", "", "Scope (root|project|all)")
	project := fs.String("project", "", "Project name/slug")
	match := fs.String("match", "auto", "Match mode (auto|exact|prefix|contains|search)")
	if err := fs.Parse(args); err != nil {
		return ExitUsage
	}
	rest := fs.Args()
	if len(rest) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: tasker idea resolve [--scope root|project|all] [--project <name>] [--match <m>] <selector>")
		return ExitUsage
	}
	selector := strings.Join(rest, " ")
	filter, err := ideaSelectorFilter(*project, *scope, *match)
	if err != nil {
		fmt.Fprintln(os.Stderr, "idea resolve:", err)
		return ExitUsage
	}
	matches, err := ws.ResolveIdeas(selector, filter)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			fmt.Fprintln(os.Stderr, "idea resolve: not found")
			return ExitNotFound
		}
		if errors.Is(err, store.ErrInvalid) {
			fmt.Fprintln(os.Stderr, "idea resolve: invalid selector")
			return ExitUsage
		}
		fmt.Fprintln(os.Stderr, "idea resolve:", err)
		return ExitInternal
	}
	type resolveIdeaMatch struct {
		ID      string   `json:"id"`
		Title   string   `json:"title"`
		Project string   `json:"project"`
		Tags    []string `json:"tags"`
	}
	out := make([]resolveIdeaMatch, 0, len(matches))
	for _, idea := range matches {
		out = append(out, resolveIdeaMatch{
			ID:      idea.ID,
			Title:   idea.Title,
			Project: idea.Project,
			Tags:    idea.Tags,
		})
	}
	payload := map[string]any{
		"selector": selector,
		"count":    len(out),
		"matches":  out,
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(payload)
	if len(out) == 0 {
		return ExitNotFound
	}
	return ExitOK
}

func cmdIdeaPromote(ws *store.Workspace, gf GlobalFlags, args []string) int {
	args = reorderFlags(args, map[string]bool{
		"--scope":      true,
		"--project":    true,
		"--match":      true,
		"--to-project": true,
		"--column":     true,
		"--due":        true,
		"--priority":   true,
		"--tag":        true,
		"--today":      false,
		"--tomorrow":   false,
		"--next-week":  false,
		"--link":       false,
		"--delete":     false,
	})
	fs := flag.NewFlagSet("idea promote", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	scope := fs.String("scope", "", "Scope (root|project|all)")
	project := fs.String("project", "", "Idea project name/slug")
	match := fs.String("match", "auto", "Match mode (auto|exact|prefix|contains|search)")
	toProject := fs.String("to-project", "", "Target task project name/slug")
	column := fs.String("column", "inbox", "Target column id (inbox|todo|doing|blocked|done|archive)")
	due := fs.String("due", "", "Due date (YYYY-MM-DD) or RFC3339")
	dueToday := fs.Bool("today", false, "Shortcut: due today")
	dueTomorrow := fs.Bool("tomorrow", false, "Shortcut: due tomorrow")
	dueNextWeek := fs.Bool("next-week", false, "Shortcut: due in 7 days")
	priority := fs.String("priority", "", "Priority (low|normal|high|urgent)")
	searchTag := multiFlag{}
	fs.Var(&searchTag, "tag", "Tag (repeatable)")
	link := fs.Bool("link", false, "Add a backlink to the idea in task notes")
	deleteIdea := fs.Bool("delete", false, "Delete idea after promoting")
	if err := fs.Parse(args); err != nil {
		return ExitUsage
	}
	rest := fs.Args()
	if len(rest) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: tasker idea promote [--scope root|project|all] [--project <name>] [--to-project <name>] [--column <col>] [--due <date>] [--priority <p>] [--tag <t>...] [--link] [--delete] <selector>")
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
	selector := strings.Join(rest, " ")
	filter, err := ideaSelectorFilter(*project, *scope, *match)
	if err != nil {
		fmt.Fprintln(os.Stderr, "idea promote:", err)
		return ExitUsage
	}
	idea, err := ws.GetIdeaBySelectorFiltered(selector, filter)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			fmt.Fprintln(os.Stderr, "idea promote: not found")
			return ExitNotFound
		}
		if errors.Is(err, store.ErrConflict) {
			if handleIdeaMatchConflict("idea promote", err) {
				return ExitConflict
			}
			fmt.Fprintln(os.Stderr, "idea promote: ambiguous selector")
			return ExitConflict
		}
		fmt.Fprintln(os.Stderr, "idea promote:", err)
		return ExitInternal
	}
	targetProject := strings.TrimSpace(*toProject)
	if targetProject == "" {
		if idea.Project != "" {
			targetProject = idea.Project
		} else {
			targetProject = resolveProject(ws, "")
		}
	}
	dueValue := strings.TrimSpace(*due)
	now := time.Now().UTC()
	if *dueToday {
		dueValue = now.Format("2006-01-02")
	}
	if *dueTomorrow {
		dueValue = now.AddDate(0, 0, 1).Format("2006-01-02")
	}
	if *dueNextWeek {
		dueValue = now.AddDate(0, 0, 7).Format("2006-01-02")
	}
	title := strings.TrimSpace(idea.Title)
	if title == "" {
		title = "(untitled idea)"
	}
	tags := append([]string{}, idea.Tags...)
	tags = append(tags, searchTag.Values...)
	desc := strings.TrimSpace(idea.Body)
	if *link {
		linkLine := ideaBacklinkLine(ws, idea)
		if desc == "" {
			desc = linkLine
		} else {
			desc = desc + "\n\n" + linkLine
		}
	}
	input := store.AddTaskInput{
		Title:       title,
		Project:     strings.TrimSpace(targetProject),
		Column:      strings.TrimSpace(*column),
		Due:         strings.TrimSpace(dueValue),
		Priority:    strings.TrimSpace(*priority),
		Tags:        tags,
		Description: desc,
	}
	task, err := ws.AddTask(input)
	if err != nil {
		fmt.Fprintln(os.Stderr, "idea promote:", err)
		return ExitInternal
	}
	code := emitAddResult(ws, gf, task, desc)
	if *deleteIdea {
		if err := ws.DeleteIdea(idea); err != nil {
			fmt.Fprintln(os.Stderr, "idea promote:", err)
			return ExitInternal
		}
		if !gf.Quiet && gf.Format != "telegram" && !gf.JSON && !gf.NDJSON {
			fmt.Println("Removed idea:", title)
		}
	}
	return code
}

func ideaNoteUsage(label string) string {
	return fmt.Sprintf("Usage: tasker %s [--scope root|project|all] [--project <name>] [--match <m>] <selector...> -- <text...>", label)
}

func cmdIdeaNote(ws *store.Workspace, gf GlobalFlags, args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, ideaNoteUsage("idea note add"))
		return ExitUsage
	}
	sub := args[0]
	if sub != "add" {
		fmt.Fprintln(os.Stderr, ideaNoteUsage("idea note add"))
		return ExitUsage
	}
	return cmdIdeaNoteAdd(ws, gf, args[1:], "idea note add", "idea note")
}

func cmdIdeaAppend(ws *store.Workspace, gf GlobalFlags, args []string) int {
	return cmdIdeaNoteAdd(ws, gf, args, "idea append", "idea append")
}

func cmdIdeaNoteAdd(ws *store.Workspace, gf GlobalFlags, args []string, usageLabel string, errLabel string) int {
	rawArgs := args
	noteSplit := -1
	for i, arg := range rawArgs {
		if arg == "--" {
			noteSplit = i
			break
		}
	}
	var noteTokens []string
	if noteSplit >= 0 {
		noteTokens = rawArgs[noteSplit+1:]
		rawArgs = rawArgs[:noteSplit]
	}
	args = reorderFlags(rawArgs, map[string]bool{
		"--scope":   true,
		"--project": true,
		"--match":   true,
	})
	fs := flag.NewFlagSet(usageLabel, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	scope := fs.String("scope", "", "Scope (root|project|all)")
	project := fs.String("project", "", "Project name/slug")
	match := fs.String("match", "auto", "Match mode (auto|exact|prefix|contains|search)")
	if err := fs.Parse(args); err != nil {
		return ExitUsage
	}
	rest := fs.Args()
	filter, err := ideaSelectorFilter(*project, *scope, *match)
	if err != nil {
		fmt.Fprintln(os.Stderr, errLabel+":", err)
		return ExitUsage
	}
	var selector string
	var text string
	if len(noteTokens) > 0 {
		if len(rest) == 0 {
			fmt.Fprintln(os.Stderr, ideaNoteUsage(usageLabel))
			return ExitUsage
		}
		selector = strings.Join(rest, " ")
		text = strings.Join(noteTokens, " ")
	} else {
		if len(rest) < 2 {
			fmt.Fprintln(os.Stderr, ideaNoteUsage(usageLabel))
			return ExitUsage
		}
		selector, text, err = splitIdeaNoteInput(ws, filter, rest)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				fmt.Fprintln(os.Stderr, errLabel+": not found")
				return ExitNotFound
			}
			if err.Error() == "ambiguous selector split" {
				fmt.Fprintln(os.Stderr, errLabel+": ambiguous selector; use -- to separate selector and note text")
				return ExitConflict
			}
			fmt.Fprintln(os.Stderr, errLabel+":", err)
			return ExitInternal
		}
	}
	if strings.TrimSpace(text) == "" {
		fmt.Fprintln(os.Stderr, errLabel+": text is required (use -- to separate selector and note text)")
		return ExitUsage
	}
	idea, err := ws.GetIdeaBySelectorFiltered(selector, filter)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			fmt.Fprintln(os.Stderr, errLabel+": not found")
			return ExitNotFound
		}
		if errors.Is(err, store.ErrConflict) {
			if handleIdeaMatchConflict(errLabel, err) {
				return ExitConflict
			}
			fmt.Fprintln(os.Stderr, errLabel+": ambiguous selector")
			return ExitConflict
		}
		fmt.Fprintln(os.Stderr, errLabel+":", err)
		return ExitInternal
	}
	idea, err = ws.AddIdeaNote(idea, strings.TrimSpace(text))
	if err != nil {
		fmt.Fprintln(os.Stderr, errLabel+":", err)
		return ExitInternal
	}
	if gf.JSON {
		if gf.StdoutJSON {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			_ = enc.Encode(map[string]any{"idea": idea})
		} else {
			path, err := writeJSONExport(gf, "idea", map[string]any{"idea": idea})
			if err != nil {
				fmt.Fprintln(os.Stderr, "idea note:", err)
				return ExitInternal
			}
			if !gf.Quiet {
				fmt.Println("Wrote JSON to:", path)
			}
		}
		return ExitOK
	}
	title := strings.TrimSpace(idea.Title)
	if title == "" {
		title = "(untitled)"
	}
	fmt.Printf("Noted idea %s\n", title)
	return ExitOK
}

func emitIdeaAddResult(gf GlobalFlags, idea *store.Idea) int {
	if gf.NDJSON {
		if gf.StdoutNDJSON {
			b, _ := json.Marshal(idea)
			fmt.Println(string(b))
		} else {
			path, err := writeNDJSONExport(gf, "idea", []any{idea})
			if err != nil {
				fmt.Fprintln(os.Stderr, "idea add:", err)
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
			_ = enc.Encode(map[string]any{"idea": idea})
		} else {
			path, err := writeJSONExport(gf, "idea", map[string]any{"idea": idea})
			if err != nil {
				fmt.Fprintln(os.Stderr, "idea add:", err)
				return ExitInternal
			}
			if !gf.Quiet {
				fmt.Println("Wrote JSON to:", path)
			}
		}
		return ExitOK
	}
	titleText := strings.TrimSpace(idea.Title)
	if titleText == "" {
		titleText = "(untitled)"
	}
	loc := ideaLocationLabel(idea.Project)
	fmt.Printf("Added idea (%s): %s\n", loc, titleText)
	return ExitOK
}

func emitAddResult(ws *store.Workspace, gf GlobalFlags, task *store.Task, descText string) int {
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
	titleText := strings.TrimSpace(task.Title)
	if titleText == "" {
		titleText = "(untitled)"
	}
	if gf.Format == "telegram" {
		colLabel := columnLabel(ws, task.Column)
		line := formatChatAddLine(titleText, descText, task.Due)
		fmt.Printf("Added to %s:\n%s\n", colLabel, line)
		return ExitOK
	}
	fmt.Printf("Added %s (%s/%s)\n", titleText, task.Project, task.Column)
	return ExitOK
}

func cmdAdd(ws *store.Workspace, gf GlobalFlags, args []string) int {
	args = reorderFlags(args, map[string]bool{
		"--project":   true,
		"--column":    true,
		"--due":       true,
		"--priority":  true,
		"--tag":       true,
		"--desc":      true,
		"--details":   true,
		"--text":      true,
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
	priority := fs.String("priority", "", "Priority (low|normal|high|urgent)")
	searchTag := multiFlag{}
	fs.Var(&searchTag, "tag", "Tag (repeatable)")
	desc := fs.String("desc", "", "Description (short)")
	details := fs.String("details", "", "Details (alias for --desc)")
	text := fs.String("text", "", "Raw input using \" | \" separators")
	if err := fs.Parse(args); err != nil {
		return ExitUsage
	}
	rest := fs.Args()
	textValue := strings.TrimSpace(*text)
	if textValue != "" && len(rest) > 0 {
		fmt.Fprintln(os.Stderr, "Usage: provide either --text or a title, not both")
		return ExitUsage
	}
	if textValue == "" && len(rest) == 0 {
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
	descText := strings.TrimSpace(*desc)
	detailsText := strings.TrimSpace(*details)
	if descText != "" && detailsText != "" && descText != detailsText {
		fmt.Fprintln(os.Stderr, "Usage: choose only one of --desc or --details")
		return ExitUsage
	}
	if detailsText != "" {
		descText = detailsText
	}
	title := strings.Join(rest, " ")
	textTitle, textDetails, textDue, textPriority, textTags := parseTextParts(textValue)
	if textValue != "" {
		title = textTitle
	}
	if strings.TrimSpace(title) == "" {
		fmt.Fprintln(os.Stderr, "Usage: tasker add \"<title>\" --project <name> [--column todo] ...")
		return ExitUsage
	}
	if descText == "" {
		descText = textDetails
	}
	dueValue := strings.TrimSpace(*due)
	if dueValue == "" {
		dueValue = textDue
	}
	now := time.Now().UTC()
	if *dueToday {
		dueValue = now.Format("2006-01-02")
	}
	if *dueTomorrow {
		dueValue = now.AddDate(0, 0, 1).Format("2006-01-02")
	}
	if *dueNextWeek {
		dueValue = now.AddDate(0, 0, 7).Format("2006-01-02")
	}
	priorityValue := strings.TrimSpace(*priority)
	if priorityValue == "" {
		priorityValue = textPriority
	}
	if priorityValue == "" {
		priorityValue = "normal"
	}
	tags := append([]string{}, searchTag.Values...)
	if len(textTags) > 0 {
		tags = append(tags, textTags...)
	}
	projectName := resolveProject(ws, *project)
	input := store.AddTaskInput{
		Title:       strings.TrimSpace(title),
		Project:     strings.TrimSpace(projectName),
		Column:      strings.TrimSpace(*column),
		Due:         strings.TrimSpace(dueValue),
		Priority:    strings.TrimSpace(priorityValue),
		Tags:        tags,
		Description: descText,
	}
	task, err := ws.AddTask(input)
	if err != nil {
		fmt.Fprintln(os.Stderr, "add:", err)
		return ExitInternal
	}
	return emitAddResult(ws, gf, task, descText)
}

func cmdCapture(ws *store.Workspace, gf GlobalFlags, args []string) int {
	args = reorderFlags(args, map[string]bool{
		"--project":   true,
		"--column":    true,
		"--due":       true,
		"--priority":  true,
		"--tag":       true,
		"--desc":      true,
		"--details":   true,
		"--text":      true,
		"--today":     false,
		"--tomorrow":  false,
		"--next-week": false,
	})
	fs := flag.NewFlagSet("capture", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	project := fs.String("project", "", "Project name/slug")
	column := fs.String("column", "inbox", "Column id (inbox|todo|doing|blocked|done|archive)")
	due := fs.String("due", "", "Due date (YYYY-MM-DD) or RFC3339")
	dueToday := fs.Bool("today", false, "Shortcut: due today")
	dueTomorrow := fs.Bool("tomorrow", false, "Shortcut: due tomorrow")
	dueNextWeek := fs.Bool("next-week", false, "Shortcut: due in 7 days")
	priority := fs.String("priority", "", "Priority (low|normal|high|urgent)")
	searchTag := multiFlag{}
	fs.Var(&searchTag, "tag", "Tag (repeatable)")
	desc := fs.String("desc", "", "Description (short)")
	details := fs.String("details", "", "Details (alias for --desc)")
	text := fs.String("text", "", "Raw input using \" | \" separators")
	if err := fs.Parse(args); err != nil {
		return ExitUsage
	}
	rest := fs.Args()
	textValue := strings.TrimSpace(*text)
	if textValue != "" && len(rest) > 0 {
		fmt.Fprintln(os.Stderr, "Usage: provide either --text or capture text, not both")
		return ExitUsage
	}
	if textValue == "" {
		textValue = strings.TrimSpace(strings.Join(rest, " "))
	}
	if textValue == "" {
		fmt.Fprintln(os.Stderr, "Usage: tasker capture \"<title | details | due 2026-01-23>\" [--project <name>] ...")
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
	descText := strings.TrimSpace(*desc)
	detailsText := strings.TrimSpace(*details)
	if descText != "" && detailsText != "" && descText != detailsText {
		fmt.Fprintln(os.Stderr, "Usage: choose only one of --desc or --details")
		return ExitUsage
	}
	if detailsText != "" {
		descText = detailsText
	}
	title, textDetails, textDue, textPriority, textTags := parseTextParts(textValue)
	if strings.TrimSpace(title) == "" {
		fmt.Fprintln(os.Stderr, "Usage: tasker capture \"<title | details | due 2026-01-23>\" [--project <name>] ...")
		return ExitUsage
	}
	if descText == "" {
		descText = textDetails
	}
	dueValue := strings.TrimSpace(*due)
	if dueValue == "" {
		dueValue = textDue
	}
	now := time.Now().UTC()
	if *dueToday {
		dueValue = now.Format("2006-01-02")
	}
	if *dueTomorrow {
		dueValue = now.AddDate(0, 0, 1).Format("2006-01-02")
	}
	if *dueNextWeek {
		dueValue = now.AddDate(0, 0, 7).Format("2006-01-02")
	}
	priorityValue := strings.TrimSpace(*priority)
	if priorityValue == "" {
		priorityValue = textPriority
	}
	if priorityValue == "" {
		priorityValue = "normal"
	}
	tags := append([]string{}, searchTag.Values...)
	if len(textTags) > 0 {
		tags = append(tags, textTags...)
	}
	projectName := resolveProject(ws, *project)
	input := store.AddTaskInput{
		Title:       strings.TrimSpace(title),
		Project:     strings.TrimSpace(projectName),
		Column:      strings.TrimSpace(*column),
		Due:         strings.TrimSpace(dueValue),
		Priority:    strings.TrimSpace(priorityValue),
		Tags:        tags,
		Description: descText,
	}
	task, err := ws.AddTask(input)
	if err != nil {
		fmt.Fprintln(os.Stderr, "capture:", err)
		return ExitInternal
	}
	return emitAddResult(ws, gf, task, descText)
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

	for _, t := range tasks {
		fmt.Fprintln(os.Stdout, formatListBullet(t))
	}
	return ExitOK
}

func formatListBullet(t store.Task) string {
	title := strings.TrimSpace(t.Title)
	if title == "" {
		title = "(untitled)"
	}
	loc := t.Project
	if t.Column != "" {
		loc = loc + "/" + t.Column
	}
	due := ""
	if strings.TrimSpace(t.Due) != "" {
		due = fmt.Sprintf(" (due %s)", strings.TrimSpace(t.Due))
	}
	status := strings.TrimSpace(t.StatusAbbrev())
	label := status
	pri := strings.TrimSpace(t.PriorityAbbrev())
	if pri != "" && pri != "N" {
		if label != "" {
			label = label + " " + pri
		} else {
			label = pri
		}
	}
	if label != "" {
		label = "[" + label + "] "
	}
	return fmt.Sprintf("- %s%s: %s%s", label, loc, title, due)
}

func ideaLocationLabel(project string) string {
	project = strings.TrimSpace(project)
	if project == "" {
		return "root"
	}
	return project
}

func formatIdeaListBullet(idea store.Idea) string {
	title := strings.TrimSpace(idea.Title)
	if title == "" {
		title = "(untitled)"
	}
	loc := ideaLocationLabel(idea.Project)
	snippet := cleanSummary(idea.Body, 140)
	if snippet != "" {
		return fmt.Sprintf("- %s: %s — %s", loc, title, snippet)
	}
	return fmt.Sprintf("- %s: %s", loc, title)
}

func ideaBacklinkLine(ws *store.Workspace, idea *store.Idea) string {
	if idea == nil {
		return ""
	}
	scope := ideaLocationLabel(idea.Project)
	path := idea.Path
	if ws != nil && strings.TrimSpace(ws.Root) != "" && strings.TrimSpace(idea.Path) != "" {
		if rel, err := filepath.Rel(ws.Root, idea.Path); err == nil {
			path = rel
		}
	}
	if path != "" {
		return fmt.Sprintf("Source idea: %s (%s, %s)", idea.ID, scope, path)
	}
	return fmt.Sprintf("Source idea: %s (%s)", idea.ID, scope)
}

func columnLabel(ws *store.Workspace, colID string) string {
	colID = strings.TrimSpace(colID)
	if colID == "" {
		return "inbox"
	}
	cfg := ws.Config()
	for _, col := range cfg.Columns {
		if col.ID == colID {
			name := strings.TrimSpace(col.Name)
			if name != "" {
				return name
			}
		}
	}
	return colID
}

func formatChatAddLine(title string, details string, due string) string {
	line := title
	detailText := cleanSummary(details, 160)
	if detailText != "" {
		line = line + " — " + detailText
	}
	if strings.TrimSpace(due) != "" {
		if dueShort := formatDueShort(due); dueShort != "" {
			line = line + " (due " + dueShort + ")"
		}
	}
	return line
}

func cleanSummary(s string, max int) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	parts := strings.Fields(s)
	s = strings.Join(parts, " ")
	if max <= 0 {
		return s
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max <= 3 {
		return string(r[:max])
	}
	return string(r[:max-3]) + "..."
}

func formatDueShort(due string) string {
	due = strings.TrimSpace(due)
	if due == "" {
		return ""
	}
	if len(due) >= 10 {
		datePart := due[:10]
		if t, err := time.Parse("2006-01-02", datePart); err == nil {
			now := time.Now().UTC()
			if t.Year() == now.Year() {
				return t.Format("Jan 02")
			}
			return t.Format("Jan 02 2006")
		}
	}
	if t, err := time.Parse(time.RFC3339, due); err == nil {
		now := time.Now().UTC()
		if t.Year() == now.Year() {
			return t.Format("Jan 02")
		}
		return t.Format("Jan 02 2006")
	}
	return due
}

func readStdinText() (string, error) {
	b, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", err
	}
	text := string(b)
	if strings.TrimSpace(text) == "" {
		return "", errors.New("stdin is empty")
	}
	return text, nil
}

func splitPipeParts(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	raw := strings.Split(text, " | ")
	parts := make([]string, 0, len(raw))
	for _, part := range raw {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		parts = append(parts, part)
	}
	return parts
}

func cutPrefixFold(text string, prefix string) (string, bool) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", false
	}
	lower := strings.ToLower(text)
	if strings.HasPrefix(lower, prefix) {
		return strings.TrimSpace(text[len(prefix):]), true
	}
	return "", false
}

func parseTagsPart(text string) []string {
	text = strings.ReplaceAll(text, ",", " ")
	fields := strings.Fields(text)
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		f = strings.TrimSpace(strings.TrimLeft(f, "#@+"))
		if f == "" {
			continue
		}
		out = append(out, f)
	}
	return out
}

func extractIdeaInlineTokens(text string, strip bool, allowProject bool) (string, string, []string) {
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return text, "", nil
	}
	var kept []string
	project := ""
	var tags []string
	for _, field := range fields {
		token := trimIdeaTokenPunct(field)
		if token == "" {
			continue
		}
		if strings.HasPrefix(token, "+") && isIdeaToken(token[1:]) {
			if allowProject && project == "" {
				project = token[1:]
			} else {
				tags = append(tags, token[1:])
			}
			if strip {
				continue
			}
		}
		if strings.HasPrefix(token, "@") && isIdeaToken(token[1:]) {
			tags = append(tags, token[1:])
			if strip {
				continue
			}
		}
		if strings.HasPrefix(token, "#") && isIdeaToken(token[1:]) {
			tags = append(tags, token[1:])
			if strip {
				continue
			}
		}
		if strip {
			kept = append(kept, field)
		}
	}
	if strip {
		return strings.Join(kept, " "), project, tags
	}
	return text, project, tags
}

func trimIdeaTokenPunct(token string) string {
	return strings.TrimRightFunc(token, func(r rune) bool {
		switch r {
		case ',', '.', ';', ':':
			return true
		default:
			return false
		}
	})
}

func isIdeaToken(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !isIdeaTokenChar(r) {
			return false
		}
	}
	return true
}

func isIdeaTokenChar(r rune) bool {
	if r >= 'a' && r <= 'z' {
		return true
	}
	if r >= 'A' && r <= 'Z' {
		return true
	}
	if r >= '0' && r <= '9' {
		return true
	}
	if r == '-' || r == '_' {
		return true
	}
	return false
}

func parsePriorityToken(text string) string {
	switch strings.TrimSpace(strings.ToLower(text)) {
	case "low", "l":
		return "low"
	case "normal", "n", "med", "medium":
		return "normal"
	case "high", "h":
		return "high"
	case "urgent", "u", "p0":
		return "urgent"
	default:
		return ""
	}
}

func parseDueToken(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	lower := strings.ToLower(text)
	now := time.Now().UTC()
	switch lower {
	case "today":
		return now.Format("2006-01-02")
	case "tomorrow":
		return now.AddDate(0, 0, 1).Format("2006-01-02")
	case "next week", "next-week", "nextweek":
		return now.AddDate(0, 0, 7).Format("2006-01-02")
	default:
		return text
	}
}

func parseTextParts(text string) (string, string, string, string, []string) {
	parts := splitPipeParts(text)
	if len(parts) == 0 {
		return "", "", "", "", nil
	}
	title := strings.TrimSpace(parts[0])
	var details []string
	var due string
	var priority string
	var tags []string
	for _, part := range parts[1:] {
		if part == "" {
			continue
		}
		if value, ok := cutPrefixFold(part, "due "); ok {
			if due == "" {
				due = parseDueToken(value)
			} else {
				details = append(details, part)
			}
			continue
		}
		if value, ok := cutPrefixFold(part, "due:"); ok {
			if due == "" {
				due = parseDueToken(value)
			} else {
				details = append(details, part)
			}
			continue
		}
		if value, ok := cutPrefixFold(part, "by "); ok {
			if due == "" {
				due = parseDueToken(value)
			} else {
				details = append(details, part)
			}
			continue
		}
		if value, ok := cutPrefixFold(part, "by:"); ok {
			if due == "" {
				due = parseDueToken(value)
			} else {
				details = append(details, part)
			}
			continue
		}
		if value, ok := cutPrefixFold(part, "priority "); ok {
			if priority == "" {
				if parsed := parsePriorityToken(value); parsed != "" {
					priority = parsed
				} else {
					details = append(details, part)
				}
			} else {
				details = append(details, part)
			}
			continue
		}
		if value, ok := cutPrefixFold(part, "priority:"); ok {
			if priority == "" {
				if parsed := parsePriorityToken(value); parsed != "" {
					priority = parsed
				} else {
					details = append(details, part)
				}
			} else {
				details = append(details, part)
			}
			continue
		}
		if value, ok := cutPrefixFold(part, "pri "); ok {
			if priority == "" {
				if parsed := parsePriorityToken(value); parsed != "" {
					priority = parsed
				} else {
					details = append(details, part)
				}
			} else {
				details = append(details, part)
			}
			continue
		}
		if value, ok := cutPrefixFold(part, "pri:"); ok {
			if priority == "" {
				if parsed := parsePriorityToken(value); parsed != "" {
					priority = parsed
				} else {
					details = append(details, part)
				}
			} else {
				details = append(details, part)
			}
			continue
		}
		if value, ok := cutPrefixFold(part, "tag "); ok {
			tags = append(tags, parseTagsPart(value)...)
			continue
		}
		if value, ok := cutPrefixFold(part, "tag:"); ok {
			tags = append(tags, parseTagsPart(value)...)
			continue
		}
		if value, ok := cutPrefixFold(part, "tags "); ok {
			tags = append(tags, parseTagsPart(value)...)
			continue
		}
		if value, ok := cutPrefixFold(part, "tags:"); ok {
			tags = append(tags, parseTagsPart(value)...)
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(part), "#") {
			tags = append(tags, parseTagsPart(part)...)
			continue
		}
		details = append(details, part)
	}
	detailText := strings.TrimSpace(strings.Join(details, " — "))
	return title, detailText, due, priority, tags
}

func parseIdeaTextParts(text string) (string, string, []string, string) {
	parts := splitPipeParts(text)
	if len(parts) == 0 {
		return "", "", nil, ""
	}
	cleanTitle, project, tags := extractIdeaInlineTokens(parts[0], true, true)
	title := strings.TrimSpace(cleanTitle)
	var details []string
	for _, part := range parts[1:] {
		if part == "" {
			continue
		}
		if value, ok := cutPrefixFold(part, "tag "); ok {
			tags = append(tags, parseTagsPart(value)...)
			continue
		}
		if value, ok := cutPrefixFold(part, "tag:"); ok {
			tags = append(tags, parseTagsPart(value)...)
			continue
		}
		if value, ok := cutPrefixFold(part, "tags "); ok {
			tags = append(tags, parseTagsPart(value)...)
			continue
		}
		if value, ok := cutPrefixFold(part, "tags:"); ok {
			tags = append(tags, parseTagsPart(value)...)
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(part), "#") {
			tags = append(tags, parseTagsPart(part)...)
			continue
		}
		cleanPart, _, inlineTags := extractIdeaInlineTokens(part, false, false)
		if len(inlineTags) > 0 {
			tags = append(tags, inlineTags...)
		}
		details = append(details, cleanPart)
	}
	detailText := strings.TrimSpace(strings.Join(details, " — "))
	return title, detailText, tags, project
}

func splitNoteInput(ws *store.Workspace, filter store.SelectorFilter, tokens []string) (string, string, string, error) {
	if len(tokens) < 2 {
		return "", "", "", errors.New("note input requires selector and text")
	}
	var selectedID string
	var selectedSelector string
	var selectedText string
	for i := len(tokens) - 1; i >= 1; i-- {
		selector := strings.Join(tokens[:i], " ")
		text := strings.Join(tokens[i:], " ")
		matches, err := ws.ResolveTasks(selector, filter)
		if err != nil {
			continue
		}
		if len(matches) != 1 {
			continue
		}
		if selectedID != "" && matches[0].ID != selectedID {
			return "", "", "", errors.New("ambiguous selector split")
		}
		if selectedID == "" {
			selectedID = matches[0].ID
			selectedSelector = selector
			selectedText = text
		}
	}
	if selectedID == "" {
		return "", "", "", store.ErrNotFound
	}
	return selectedID, selectedSelector, selectedText, nil
}

func splitIdeaNoteInput(ws *store.Workspace, filter store.IdeaSelectorFilter, tokens []string) (string, string, error) {
	if len(tokens) < 2 {
		return "", "", errors.New("note input requires selector and text")
	}
	var selectedID string
	var selectedSelector string
	var selectedText string
	for i := len(tokens) - 1; i >= 1; i-- {
		selector := strings.Join(tokens[:i], " ")
		text := strings.Join(tokens[i:], " ")
		matches, err := ws.ResolveIdeas(selector, filter)
		if err != nil {
			continue
		}
		if len(matches) != 1 {
			continue
		}
		if selectedID != "" && matches[0].ID != selectedID {
			return "", "", errors.New("ambiguous selector split")
		}
		if selectedID == "" {
			selectedID = matches[0].ID
			selectedSelector = selector
			selectedText = text
		}
	}
	if selectedID == "" {
		return "", "", store.ErrNotFound
	}
	return selectedSelector, selectedText, nil
}

func cmdShow(ws *store.Workspace, gf GlobalFlags, args []string) int {
	args = reorderFlags(args, map[string]bool{
		"--project": true,
		"--column":  true,
		"--status":  true,
		"--all":     false,
		"--match":   true,
	})
	fs := flag.NewFlagSet("show", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	project := fs.String("project", "", "Project name/slug (use none|all for all projects)")
	column := fs.String("column", "", "Column id")
	status := fs.String("status", "", "Status (open|doing|blocked|done|archived)")
	all := fs.Bool("all", false, "Include archived")
	match := fs.String("match", "auto", "Match mode (auto|exact|prefix|contains|search)")
	if err := fs.Parse(args); err != nil {
		return ExitUsage
	}
	rest := fs.Args()
	if len(rest) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: tasker show [--project <name>|none|all] [--column <col>] [--status <s>] [--all] [--match <m>] <selector>")
		return ExitUsage
	}
	selector := strings.Join(rest, " ")
	filter, err := selectorFilter(ws, *project, *column, *status, *all, *match)
	if err != nil {
		fmt.Fprintln(os.Stderr, "show:", err)
		return ExitUsage
	}
	task, err := ws.GetTaskBySelectorFiltered(selector, filter)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			fmt.Fprintln(os.Stderr, "show: not found")
			return ExitNotFound
		}
		if errors.Is(err, store.ErrConflict) {
			if handleMatchConflict("show", err) {
				return ExitConflict
			}
			fmt.Fprintln(os.Stderr, "show: ambiguous selector")
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

func cmdResolve(ws *store.Workspace, gf GlobalFlags, args []string) int {
	args = reorderFlags(args, map[string]bool{
		"--project": true,
		"--column":  true,
		"--status":  true,
		"--all":     false,
		"--match":   true,
	})
	fs := flag.NewFlagSet("resolve", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	project := fs.String("project", "", "Project name/slug (use none|all for all projects)")
	column := fs.String("column", "", "Column id")
	status := fs.String("status", "", "Status (open|doing|blocked|done|archived)")
	all := fs.Bool("all", false, "Include archived")
	match := fs.String("match", "auto", "Match mode (auto|exact|prefix|contains|search)")
	if err := fs.Parse(args); err != nil {
		return ExitUsage
	}
	rest := fs.Args()
	if len(rest) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: tasker resolve [--project <name>|none|all] [--column <col>] [--status <s>] [--all] [--match <m>] <selector>")
		return ExitUsage
	}
	selector := strings.Join(rest, " ")
	filter, err := selectorFilter(ws, *project, *column, *status, *all, *match)
	if err != nil {
		fmt.Fprintln(os.Stderr, "resolve:", err)
		return ExitUsage
	}
	matches, err := ws.ResolveTasks(selector, filter)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			fmt.Fprintln(os.Stderr, "resolve: not found")
			return ExitNotFound
		}
		if errors.Is(err, store.ErrInvalid) {
			fmt.Fprintln(os.Stderr, "resolve: invalid selector")
			return ExitUsage
		}
		fmt.Fprintln(os.Stderr, "resolve:", err)
		return ExitInternal
	}
	type resolveMatch struct {
		ID       string   `json:"id"`
		Title    string   `json:"title"`
		Project  string   `json:"project"`
		Column   string   `json:"column"`
		Status   string   `json:"status"`
		Due      string   `json:"due"`
		Priority string   `json:"priority"`
		Tags     []string `json:"tags"`
	}
	out := make([]resolveMatch, 0, len(matches))
	for _, t := range matches {
		out = append(out, resolveMatch{
			ID:       t.ID,
			Title:    t.Title,
			Project:  t.Project,
			Column:   t.Column,
			Status:   t.Status,
			Due:      t.Due,
			Priority: t.Priority,
			Tags:     t.Tags,
		})
	}
	payload := map[string]any{
		"selector": selector,
		"count":    len(out),
		"matches":  out,
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(payload)
	if len(out) == 0 {
		return ExitNotFound
	}
	return ExitOK
}

func cmdMove(ws *store.Workspace, gf GlobalFlags, args []string) int {
	args = reorderFlags(args, map[string]bool{
		"--project": true,
		"--column":  true,
		"--status":  true,
		"--all":     false,
		"--match":   true,
	})
	fs := flag.NewFlagSet("mv", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	project := fs.String("project", "", "Project name/slug (use none|all for all projects)")
	column := fs.String("column", "", "Column id (filter)")
	status := fs.String("status", "", "Status (open|doing|blocked|done|archived)")
	all := fs.Bool("all", false, "Include archived")
	match := fs.String("match", "auto", "Match mode (auto|exact|prefix|contains|search)")
	if err := fs.Parse(args); err != nil {
		return ExitUsage
	}
	rest := fs.Args()
	if len(rest) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: tasker mv [--project <name>|none|all] [--column <col>] [--status <s>] [--all] [--match <m>] <selector> <column>")
		return ExitUsage
	}
	destColumn := rest[len(rest)-1]
	selector := strings.Join(rest[:len(rest)-1], " ")
	filter, err := selectorFilter(ws, *project, *column, *status, *all, *match)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mv:", err)
		return ExitUsage
	}
	taskRef, err := ws.GetTaskBySelectorFiltered(selector, filter)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			fmt.Fprintln(os.Stderr, "mv: not found")
			return ExitNotFound
		}
		if errors.Is(err, store.ErrConflict) {
			if handleMatchConflict("mv", err) {
				return ExitConflict
			}
			fmt.Fprintln(os.Stderr, "mv: ambiguous selector")
			return ExitConflict
		}
		fmt.Fprintln(os.Stderr, "mv:", err)
		return ExitInternal
	}
	task, err := ws.MoveTask(taskRef.ID, destColumn)
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
	title := strings.TrimSpace(task.Title)
	if title == "" {
		title = "(untitled)"
	}
	fmt.Printf("Moved %s -> %s\n", title, task.Column)
	return ExitOK
}

func cmdDone(ws *store.Workspace, gf GlobalFlags, args []string) int {
	args = reorderFlags(args, map[string]bool{
		"--project": true,
		"--column":  true,
		"--status":  true,
		"--all":     false,
		"--match":   true,
	})
	fs := flag.NewFlagSet("done", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	project := fs.String("project", "", "Project name/slug (use none|all for all projects)")
	column := fs.String("column", "", "Column id (filter)")
	status := fs.String("status", "", "Status (open|doing|blocked|done|archived)")
	all := fs.Bool("all", false, "Include archived")
	match := fs.String("match", "auto", "Match mode (auto|exact|prefix|contains|search)")
	if err := fs.Parse(args); err != nil {
		return ExitUsage
	}
	rest := fs.Args()
	if len(rest) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: tasker done [--project <name>|none|all] [--column <col>] [--status <s>] [--all] [--match <m>] <selector>")
		return ExitUsage
	}
	selector := strings.Join(rest, " ")
	filter, err := selectorFilter(ws, *project, *column, *status, *all, *match)
	if err != nil {
		fmt.Fprintln(os.Stderr, "done:", err)
		return ExitUsage
	}
	taskRef, err := ws.GetTaskBySelectorFiltered(selector, filter)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			fmt.Fprintln(os.Stderr, "done: not found")
			return ExitNotFound
		}
		if errors.Is(err, store.ErrConflict) {
			if handleMatchConflict("done", err) {
				return ExitConflict
			}
			fmt.Fprintln(os.Stderr, "done: ambiguous selector")
			return ExitConflict
		}
		fmt.Fprintln(os.Stderr, "done:", err)
		return ExitInternal
	}
	task, err := ws.MoveTask(taskRef.ID, "done")
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
	title := strings.TrimSpace(task.Title)
	if title == "" {
		title = "(untitled)"
	}
	fmt.Printf("Done %s\n", title)
	return ExitOK
}

func cmdNote(ws *store.Workspace, gf GlobalFlags, args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: tasker note add <selector...> -- <text...>")
		return ExitUsage
	}
	sub := args[0]
	if sub != "add" {
		fmt.Fprintln(os.Stderr, "Usage: tasker note add <selector...> -- <text...>")
		return ExitUsage
	}
	rawArgs := args[1:]
	noteSplit := -1
	for i, arg := range rawArgs {
		if arg == "--" {
			noteSplit = i
			break
		}
	}
	var noteTokens []string
	if noteSplit >= 0 {
		noteTokens = rawArgs[noteSplit+1:]
		rawArgs = rawArgs[:noteSplit]
	}
	args = reorderFlags(rawArgs, map[string]bool{
		"--project": true,
		"--column":  true,
		"--status":  true,
		"--all":     false,
		"--match":   true,
	})
	fs := flag.NewFlagSet("note add", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	project := fs.String("project", "", "Project name/slug (use none|all for all projects)")
	column := fs.String("column", "", "Column id (filter)")
	status := fs.String("status", "", "Status (open|doing|blocked|done|archived)")
	all := fs.Bool("all", false, "Include archived")
	match := fs.String("match", "auto", "Match mode (auto|exact|prefix|contains|search)")
	if err := fs.Parse(args); err != nil {
		return ExitUsage
	}
	rest := fs.Args()
	filter, err := selectorFilter(ws, *project, *column, *status, *all, *match)
	if err != nil {
		fmt.Fprintln(os.Stderr, "note:", err)
		return ExitUsage
	}
	var taskID string
	var selector string
	var text string
	if len(noteTokens) > 0 {
		if len(rest) == 0 {
			fmt.Fprintln(os.Stderr, "Usage: tasker note add [--project <name>|none|all] [--column <col>] [--status <s>] [--all] [--match <m>] <selector...> -- <text...>")
			return ExitUsage
		}
		selector = strings.Join(rest, " ")
		text = strings.Join(noteTokens, " ")
		taskRef, err := ws.GetTaskBySelectorFiltered(selector, filter)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				fmt.Fprintln(os.Stderr, "note: not found")
				return ExitNotFound
			}
			if errors.Is(err, store.ErrConflict) {
				if handleMatchConflict("note", err) {
					return ExitConflict
				}
				fmt.Fprintln(os.Stderr, "note: ambiguous selector")
				return ExitConflict
			}
			fmt.Fprintln(os.Stderr, "note:", err)
			return ExitInternal
		}
		taskID = taskRef.ID
	} else {
		if len(rest) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: tasker note add [--project <name>|none|all] [--column <col>] [--status <s>] [--all] [--match <m>] <selector...> -- <text...>")
			return ExitUsage
		}
		taskID, selector, text, err = splitNoteInput(ws, filter, rest)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				fmt.Fprintln(os.Stderr, "note: not found")
				return ExitNotFound
			}
			if err.Error() == "ambiguous selector split" {
				fmt.Fprintln(os.Stderr, "note: ambiguous selector; use -- to separate selector and note text")
				return ExitConflict
			}
			fmt.Fprintln(os.Stderr, "note:", err)
			return ExitInternal
		}
	}
	if strings.TrimSpace(text) == "" {
		fmt.Fprintln(os.Stderr, "note: text is required (use -- to separate selector and note text)")
		return ExitUsage
	}
	task, err := ws.AddNote(taskID, strings.TrimSpace(text))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			fmt.Fprintln(os.Stderr, "note: not found")
			return ExitNotFound
		}
		if errors.Is(err, store.ErrConflict) {
			if handleMatchConflict("note", err) {
				return ExitConflict
			}
			fmt.Fprintln(os.Stderr, "note: ambiguous selector")
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
	title := strings.TrimSpace(task.Title)
	if title == "" {
		title = "(untitled)"
	}
	fmt.Printf("Noted %s\n", title)
	return ExitOK
}

func cmdBoard(ws *store.Workspace, gf GlobalFlags, args []string) int {
	args = reorderFlags(args, map[string]bool{
		"--project": true,
		"--open":    false,
		"--all":     false,
	})
	fs := flag.NewFlagSet("board", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	project := fs.String("project", "", "Project name/slug")
	openOnly := fs.Bool("open", false, "Only open/doing/blocked")
	all := fs.Bool("all", false, "Include done/archived")
	if err := fs.Parse(args); err != nil {
		return ExitUsage
	}
	if strings.TrimSpace(*project) == "" {
		fmt.Fprintln(os.Stderr, "Usage: tasker board --project <name> [--open|--all]")
		return ExitUsage
	}
	open := *openOnly
	if *all {
		open = false
	}
	if gf.Format == "telegram" && !*all && !*openOnly {
		open = true
	}
	out, err := ws.RenderBoard(strings.TrimSpace(*project), gf.ASCII, gf.Format, open)
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
	if gf.Format == "telegram" && !*all {
		open = true
	}
	groupBy := resolveGroupBy(ws, *group)
	if groupBy == "none" {
		groupBy = ""
	}
	if groupBy != "" && groupBy != "project" && groupBy != "column" {
		fmt.Fprintln(os.Stderr, "today: invalid --group (use project|column|none)")
		return ExitUsage
	}
	showTotals := resolveShowTotals(ws, *totals)
	out, err := ws.RenderToday(projectName, open, groupBy, showTotals, gf.Format)
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
	if gf.Format == "telegram" && !*all {
		open = true
	}
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
	out, err := ws.RenderAgenda(projectName, window, open, groupBy, showTotals, gf.Format)
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
		if v := strings.ToLower(envString("TASKER_VIEW")); v == "week" || v == "today" {
			mode = v
		} else if ac := agentConfig(ws); ac != nil && strings.ToLower(ac.DefaultView) == "week" {
			mode = "week"
		} else {
			mode = "today"
		}
	}
	projectName := resolveProject(ws, *project)
	open := resolveOpenOnly(ws, *openOnly, *all)
	if gf.Format == "telegram" && !*all {
		open = true
	}
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
		out, err := ws.RenderAgenda(projectName, window, open, groupBy, showTotals, gf.Format)
		if err != nil {
			fmt.Fprintln(os.Stderr, "tasks:", err)
			return ExitInternal
		}
		fmt.Println(out)
		return ExitOK
	}
	out, err := ws.RenderToday(projectName, open, groupBy, showTotals, gf.Format)
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
