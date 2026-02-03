package cli

import (
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/amirbrooks/tasker-docstore-framework/internal/store"
)

type stringList []string

func (s *stringList) String() string {
	return strings.Join(*s, ", ")
}

func (s *stringList) Set(value string) error {
	*s = append(*s, value)
	return nil
}

func cmdWorkflow(ws *store.Workspace, gf GlobalFlags, args []string) int {
	_ = ws
	_ = gf
	if len(args) == 0 {
		printWorkflowHelp()
		return ExitUsage
	}

	switch args[0] {
	case "init":
		return cmdWorkflowInit(args[1:])
	case "prompts":
		return cmdWorkflowPrompts(args[1:])
	case "schedule":
		return cmdWorkflowSchedule(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "Unknown workflow command: %s\n\n", args[0])
		printWorkflowHelp()
		return ExitUsage
	}
}

func printWorkflowHelp() {
	fmt.Print(`tasker workflow

Usage:
  tasker workflow init [flags]
  tasker workflow prompts init [flags]
  tasker workflow schedule init [flags]

Flags:
  --workspace <path>     OpenClaw workspace path (default: ~/.openclaw/workspace or OPENCLAW_WORKSPACE)
  --file <name>          Target file in workspace (default: management/tasker/workflow.md)
  --runs-dir <path>      Runs directory (relative to workspace)
  --templates-dir <path> Templates directory (relative to workspace)
  --run-name <pattern>   Run name pattern (default: YYYY-MM-DD-<short-name>)
  --heartbeat <command>  Heartbeat command to suggest (repeatable)
  --no-heartbeat         Omit heartbeat commands from the config
  --force                Overwrite existing workflow section and templates

Notes:
  - Paths must resolve inside the workspace. Use --workspace to change the root.
`)
}

func cmdWorkflowInit(args []string) int {
	fs := flag.NewFlagSet("workflow init", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var workspace string
	var fileName string
	var runsDir string
	var templatesDir string
	var runName string
	var noHeartbeat bool
	var force bool
	var heartbeat stringList

	fs.StringVar(&workspace, "workspace", "", "OpenClaw workspace path")
	fs.StringVar(&fileName, "file", "", "Target file in workspace (AGENTS.md or USER.md)")
	fs.StringVar(&runsDir, "runs-dir", "management/RUNS", "Runs directory (relative to workspace)")
	fs.StringVar(&templatesDir, "templates-dir", "management/templates", "Templates directory (relative to workspace)")
	fs.StringVar(&runName, "run-name", "YYYY-MM-DD-<short-name>", "Run name pattern")
	fs.BoolVar(&noHeartbeat, "no-heartbeat", false, "Omit heartbeat commands")
	fs.BoolVar(&force, "force", false, "Overwrite existing workflow section and templates")
	fs.Var(&heartbeat, "heartbeat", "Heartbeat command to suggest (repeatable)")

	if err := fs.Parse(args); err != nil {
		return ExitUsage
	}

	workspace = resolveWorkspacePath(workspace)
	if workspace == "" {
		fmt.Fprintln(os.Stderr, "workflow init: unable to resolve workspace path")
		return ExitUsage
	}

	if err := os.MkdirAll(workspace, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "workflow init:", err)
		return ExitInternal
	}

	filePath, err := resolveWorkflowConfigPath(workspace, fileName)
	if err != nil {
		fmt.Fprintln(os.Stderr, "workflow init:", err)
		return ExitUsage
	}

	runsDir = strings.TrimSpace(runsDir)
	if runsDir == "" {
		runsDir = "management/RUNS"
	}

	templatesDir = strings.TrimSpace(templatesDir)
	if templatesDir == "" {
		templatesDir = "management/templates"
	}

	runName = strings.TrimSpace(runName)
	if runName == "" {
		runName = "YYYY-MM-DD-<short-name>"
	}

	if len(heartbeat) == 0 && !noHeartbeat {
		heartbeat = []string{
			"tasker tasks --format telegram",
			"tasker week --days 7 --format telegram",
		}
	}

	if err := os.MkdirAll(filepath.Join(workspace, runsDir), 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "workflow init:", err)
		return ExitInternal
	}

	templatesPath := filepath.Join(workspace, templatesDir)
	if err := os.MkdirAll(templatesPath, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "workflow init:", err)
		return ExitInternal
	}

	if err := writeTemplateFile(filepath.Join(templatesPath, "spec.md"), workflowSpecTemplate, force); err != nil {
		fmt.Fprintln(os.Stderr, "workflow init:", err)
		return ExitInternal
	}
	if err := writeTemplateFile(filepath.Join(templatesPath, "tasks.md"), workflowTasksTemplate, force); err != nil {
		fmt.Fprintln(os.Stderr, "workflow init:", err)
		return ExitInternal
	}
	if err := writeTemplateFile(filepath.Join(templatesPath, "HANDOFF.md"), workflowHandoffTemplate, force); err != nil {
		fmt.Fprintln(os.Stderr, "workflow init:", err)
		return ExitInternal
	}

	section := renderWorkflowSection(runsDir, runName, templatesDir, heartbeat, noHeartbeat)

	current, err := os.ReadFile(filePath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		fmt.Fprintln(os.Stderr, "workflow init:", err)
		return ExitInternal
	}

	content := string(current)
	if strings.TrimSpace(content) == "" {
		header := workflowHeaderForFile(filePath)
		if header != "" {
			content = header + "\n\n"
		}
	}

	updated := upsertWorkflowSection(content, section)
	if err := atomicWriteFile(filePath, []byte(updated), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "workflow init:", err)
		return ExitInternal
	}

	fmt.Fprintf(os.Stdout, "OpenClaw workspace: %s\n", workspace)
	fmt.Fprintf(os.Stdout, "Workflow config written to %s\n", filePath)
	fmt.Fprintf(os.Stdout, "Templates written to %s\n", templatesPath)
	fmt.Fprintf(os.Stdout, "Runs directory: %s\n", filepath.Join(workspace, runsDir))
	fmt.Fprintln(os.Stdout, "Tip: set OPENCLAW_WORKSPACE or --workspace to change the workspace root.")
	return ExitOK
}

func cmdWorkflowPrompts(args []string) int {
	if len(args) == 0 {
		printWorkflowHelp()
		return ExitUsage
	}
	switch args[0] {
	case "init":
		return cmdWorkflowPromptsInit(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "Unknown workflow prompts command: %s\n\n", args[0])
		printWorkflowHelp()
		return ExitUsage
	}
}

func cmdWorkflowPromptsInit(args []string) int {
	fs := flag.NewFlagSet("workflow prompts init", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var workspace string
	var fileName string
	var promptsDir string
	var nightShift string
	var proactive string
	var force bool

	fs.StringVar(&workspace, "workspace", "", "OpenClaw workspace path")
	fs.StringVar(&fileName, "file", "", "Target file in workspace")
	fs.StringVar(&promptsDir, "prompts-dir", "management", "Prompts directory (relative to workspace)")
	fs.StringVar(&nightShift, "night-shift", "", "Night Shift prompt path (relative to workspace)")
	fs.StringVar(&proactive, "proactive", "", "Proactive Operator prompt path (relative to workspace)")
	fs.BoolVar(&force, "force", false, "Overwrite existing prompt files and config section")

	if err := fs.Parse(args); err != nil {
		return ExitUsage
	}

	workspace = resolveWorkspacePath(workspace)
	if workspace == "" {
		fmt.Fprintln(os.Stderr, "workflow prompts init: unable to resolve workspace path")
		return ExitUsage
	}

	filePath, err := resolveWorkflowConfigPath(workspace, fileName)
	if err != nil {
		fmt.Fprintln(os.Stderr, "workflow prompts init:", err)
		return ExitUsage
	}

	promptsDir = strings.TrimSpace(promptsDir)
	if promptsDir == "" {
		promptsDir = "management"
	}

	if nightShift == "" {
		nightShift = filepath.Join(promptsDir, "NIGHT_SHIFT.md")
	}
	if proactive == "" {
		proactive = filepath.Join(promptsDir, "PROACTIVE_OPERATOR.md")
	}

	nightShiftPath, err := resolveWorkspaceRelative(workspace, nightShift)
	if err != nil {
		fmt.Fprintln(os.Stderr, "workflow prompts init:", err)
		return ExitUsage
	}
	proactivePath, err := resolveWorkspaceRelative(workspace, proactive)
	if err != nil {
		fmt.Fprintln(os.Stderr, "workflow prompts init:", err)
		return ExitUsage
	}

	if err := writeTemplateFile(nightShiftPath, workflowNightShiftPrompt, force); err != nil {
		fmt.Fprintln(os.Stderr, "workflow prompts init:", err)
		return ExitInternal
	}
	if err := writeTemplateFile(proactivePath, workflowProactivePrompt, force); err != nil {
		fmt.Fprintln(os.Stderr, "workflow prompts init:", err)
		return ExitInternal
	}

	section := renderPromptsSection(workspace, nightShiftPath, proactivePath)

	updated, err := writeWorkflowSection(filePath, section, upsertPromptsSection)
	if err != nil {
		fmt.Fprintln(os.Stderr, "workflow prompts init:", err)
		return ExitInternal
	}

	fmt.Fprintf(os.Stdout, "OpenClaw workspace: %s\n", workspace)
	fmt.Fprintf(os.Stdout, "Prompts config written to %s\n", filePath)
	fmt.Fprintf(os.Stdout, "Night Shift prompt: %s\n", nightShiftPath)
	fmt.Fprintf(os.Stdout, "Proactive Operator prompt: %s\n", proactivePath)
	fmt.Fprintln(os.Stdout, "Tip: set OPENCLAW_WORKSPACE or --workspace to change the workspace root.")
	if updated {
		// no-op: updated indicates write success; keep output lean
	}
	return ExitOK
}

func cmdWorkflowSchedule(args []string) int {
	if len(args) == 0 {
		printWorkflowHelp()
		return ExitUsage
	}
	switch args[0] {
	case "init":
		return cmdWorkflowScheduleInit(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "Unknown workflow schedule command: %s\n\n", args[0])
		printWorkflowHelp()
		return ExitUsage
	}
}

func cmdWorkflowScheduleInit(args []string) int {
	fs := flag.NewFlagSet("workflow schedule init", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var workspace string
	var fileName string
	var windowRaw string
	var heartbeatRaw string
	var heartbeatPrompt string
	var nightShift string
	var tz string
	var nightlyCron string
	var noHeartbeatPrompt bool
	var noNightly bool
	var force bool
	var heartbeatCmds stringList
	var heartbeatReads stringList

	fs.StringVar(&workspace, "workspace", "", "OpenClaw workspace path")
	fs.StringVar(&fileName, "file", "", "Target file in workspace")
	fs.StringVar(&windowRaw, "window", "24h", "Monitoring window duration (e.g., 24h)")
	fs.StringVar(&heartbeatRaw, "heartbeat-every", "2h", "Heartbeat cadence (e.g., 2h)")
	fs.StringVar(&heartbeatPrompt, "heartbeat-prompt", "", "Heartbeat prompt path (relative to workspace)")
	fs.StringVar(&nightShift, "night-shift", "", "Night Shift prompt path (relative to workspace)")
	fs.StringVar(&tz, "tz", "", "Timezone for cron examples (e.g., America/New_York)")
	fs.StringVar(&nightlyCron, "nightly-cron", "0 23 * * *", "Cron schedule for Night Shift example")
	fs.BoolVar(&noHeartbeatPrompt, "no-heartbeat-prompt", false, "Do not write a heartbeat prompt file")
	fs.BoolVar(&noNightly, "no-nightly", false, "Do not include Night Shift cron example")
	fs.BoolVar(&force, "force", false, "Overwrite existing schedule section and heartbeat prompt")
	fs.Var(&heartbeatCmds, "heartbeat", "Heartbeat command to suggest (repeatable)")
	fs.Var(&heartbeatReads, "read", "Heartbeat read target (repeatable)")

	if err := fs.Parse(args); err != nil {
		return ExitUsage
	}

	workspace = resolveWorkspacePath(workspace)
	if workspace == "" {
		fmt.Fprintln(os.Stderr, "workflow schedule init: unable to resolve workspace path")
		return ExitUsage
	}

	filePath, err := resolveWorkflowConfigPath(workspace, fileName)
	if err != nil {
		fmt.Fprintln(os.Stderr, "workflow schedule init:", err)
		return ExitUsage
	}

	windowDur, windowLabel, err := parseDurationFlexible(windowRaw, "24h")
	if err != nil {
		fmt.Fprintln(os.Stderr, "workflow schedule init:", err)
		return ExitUsage
	}
	heartbeatDur, heartbeatLabel, err := parseDurationFlexible(heartbeatRaw, "2h")
	if err != nil {
		fmt.Fprintln(os.Stderr, "workflow schedule init:", err)
		return ExitUsage
	}
	if heartbeatDur <= 0 {
		fmt.Fprintln(os.Stderr, "workflow schedule init: heartbeat cadence must be > 0")
		return ExitUsage
	}

	heartbeatCount := windowHeartbeatCount(windowDur, heartbeatDur)

	if len(heartbeatCmds) == 0 {
		heartbeatCmds = []string{
			"tasker tasks --format telegram",
			"tasker week --days 7 --format telegram",
		}
	}
	if len(heartbeatReads) == 0 {
		heartbeatReads = []string{
			"AGENTS.md (if present)",
			"USER.md (if present)",
			"MEMORY.md (if present)",
			"management/tasker/workflow.md",
			"management/BACKLOG.md",
			"latest run in management/RUNS",
		}
	}

	if heartbeatPrompt == "" {
		heartbeatPrompt = filepath.Join("management", "HEARTBEAT.md")
	}
	if nightShift == "" {
		nightShift = filepath.Join("management", "NIGHT_SHIFT.md")
	}

	heartbeatPath, err := resolveWorkspaceRelative(workspace, heartbeatPrompt)
	if err != nil {
		fmt.Fprintln(os.Stderr, "workflow schedule init:", err)
		return ExitUsage
	}
	nightShiftPath, err := resolveWorkspaceRelative(workspace, nightShift)
	if err != nil {
		fmt.Fprintln(os.Stderr, "workflow schedule init:", err)
		return ExitUsage
	}

	if !noHeartbeatPrompt {
		if err := writeTemplateFile(heartbeatPath, workflowHeartbeatPrompt, force); err != nil {
			fmt.Fprintln(os.Stderr, "workflow schedule init:", err)
			return ExitInternal
		}
	}

	section := renderScheduleSection(scheduleSectionInput{
		Workspace:        workspace,
		WindowLabel:      windowLabel,
		HeartbeatLabel:   heartbeatLabel,
		HeartbeatCount:   heartbeatCount,
		HeartbeatPrompt:  heartbeatPath,
		HeartbeatReads:   heartbeatReads,
		HeartbeatCmds:    heartbeatCmds,
		NightShiftPrompt: nightShiftPath,
		Timezone:         tz,
		NightlyCron:      nightlyCron,
		NoHeartbeat:      noHeartbeatPrompt,
		NoNightly:        noNightly,
	})

	updated, err := writeWorkflowSection(filePath, section, upsertScheduleSection)
	if err != nil {
		fmt.Fprintln(os.Stderr, "workflow schedule init:", err)
		return ExitInternal
	}

	fmt.Fprintf(os.Stdout, "OpenClaw workspace: %s\n", workspace)
	fmt.Fprintf(os.Stdout, "Schedule config written to %s\n", filePath)
	if !noHeartbeatPrompt {
		fmt.Fprintf(os.Stdout, "Heartbeat prompt: %s\n", heartbeatPath)
	}
	fmt.Fprintln(os.Stdout, "Tip: set OPENCLAW_WORKSPACE or --workspace to change the workspace root.")
	printScheduleExamples(scheduleExampleInput{
		HeartbeatEvery: heartbeatDur,
		HeartbeatLabel: heartbeatLabel,
		HeartbeatPath:  pathForConfig(workspace, heartbeatPath),
		NightShiftPath: pathForConfig(workspace, nightShiftPath),
		Timezone:       tz,
		NightlyCron:    nightlyCron,
		NoNightly:      noNightly,
	})
	if updated {
		// no-op
	}
	return ExitOK
}

func defaultOpenClawWorkspace() string {
	if v := strings.TrimSpace(os.Getenv("OPENCLAW_WORKSPACE")); v != "" {
		return v
	}
	home, _ := os.UserHomeDir()
	if home == "" {
		return ""
	}
	return filepath.Join(home, ".openclaw", "workspace")
}

func resolveWorkspacePath(input string) string {
	value := strings.TrimSpace(input)
	if value == "" {
		value = defaultOpenClawWorkspace()
	}
	value = expandHomePath(value)
	if value == "" {
		return ""
	}
	if abs, err := filepath.Abs(value); err == nil {
		return abs
	}
	return value
}

func expandHomePath(p string) string {
	home, _ := os.UserHomeDir()
	if home == "" {
		return p
	}

	if p == "~" {
		return home
	}

	if strings.HasPrefix(p, "~"+string(os.PathSeparator)) {
		rel := strings.TrimPrefix(p, "~"+string(os.PathSeparator))
		if rel == "" {
			return home
		}
		return filepath.Join(home, rel)
	}

	if strings.HasPrefix(p, "~/") {
		rel := strings.TrimPrefix(p, "~/")
		if rel == "" {
			return home
		}
		return filepath.Join(home, rel)
	}

	return p
}

func resolveWorkspaceRelative(workspace string, target string) (string, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return "", nil
	}
	target = expandHomePath(target)
	workspace = expandHomePath(strings.TrimSpace(workspace))
	if workspace == "" {
		return "", errors.New("workspace path is required")
	}
	workspaceAbs, err := filepath.Abs(workspace)
	if err != nil {
		return "", err
	}

	if filepath.IsAbs(target) {
		targetAbs, err := filepath.Abs(target)
		if err != nil {
			return "", err
		}
		if !pathWithin(workspaceAbs, targetAbs) {
			return "", fmt.Errorf("path must be within workspace: %s (workspace: %s)", target, workspaceAbs)
		}
		return targetAbs, nil
	}

	joined := filepath.Join(workspaceAbs, target)
	joinedAbs, err := filepath.Abs(joined)
	if err != nil {
		return "", err
	}
	if !pathWithin(workspaceAbs, joinedAbs) {
		return "", fmt.Errorf("path must be within workspace: %s (workspace: %s)", target, workspaceAbs)
	}
	return joinedAbs, nil
}

func pathForConfig(workspace string, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	value = expandHomePath(value)
	if !filepath.IsAbs(value) {
		return filepath.ToSlash(value)
	}
	rel, err := filepath.Rel(workspace, value)
	if err == nil && rel != "." && !strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(rel)
	}
	return filepath.ToSlash(value)
}

func pathWithin(root string, target string) bool {
	root = filepath.Clean(root)
	target = filepath.Clean(target)
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	rel = filepath.Clean(rel)
	if rel == "." {
		return true
	}
	if rel == ".." {
		return false
	}
	return !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}

func resolveWorkflowConfigPath(workspace string, fileName string) (string, error) {
	fileName = strings.TrimSpace(fileName)
	if fileName == "" {
		fileName = filepath.Join("management", "tasker", "workflow.md")
	}
	fileName = expandHomePath(fileName)
	path, err := resolveWorkspaceRelative(workspace, fileName)
	if err != nil {
		return "", err
	}
	return path, nil
}

func workflowHeaderForFile(filePath string) string {
	base := strings.ToUpper(filepath.Base(filePath))
	switch base {
	case "AGENTS.MD":
		return "# AGENTS"
	case "USER.MD":
		return "# USER"
	case "WORKFLOW.MD":
		return "# Tasker Workspace"
	default:
		return ""
	}
}

func renderWorkflowSection(runsDir string, runName string, templatesDir string, heartbeat []string, noHeartbeat bool) string {
	var b strings.Builder
	b.WriteString("## Tasker Workflow\n")
	b.WriteString(fmt.Sprintf("Runs dir: %s\n", runsDir))
	b.WriteString(fmt.Sprintf("Run name: %s\n", runName))
	b.WriteString("Templates:\n")
	b.WriteString(fmt.Sprintf("  spec: %s\n", joinWorkflowPath(templatesDir, "spec.md")))
	b.WriteString(fmt.Sprintf("  tasks: %s\n", joinWorkflowPath(templatesDir, "tasks.md")))
	b.WriteString(fmt.Sprintf("  handoff: %s\n", joinWorkflowPath(templatesDir, "HANDOFF.md")))
	if !noHeartbeat {
		b.WriteString("Heartbeat mode: suggest\n")
		if len(heartbeat) > 0 {
			b.WriteString("Heartbeat commands:\n")
			for _, cmd := range heartbeat {
				cmd = strings.TrimSpace(cmd)
				if cmd == "" {
					continue
				}
				b.WriteString("  - " + cmd + "\n")
			}
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func joinWorkflowPath(dir string, name string) string {
	return filepath.ToSlash(filepath.Join(dir, name))
}

func upsertWorkflowSection(content string, section string) string {
	return upsertSection(content, section, "## Tasker Workflow", workflowSectionStart, workflowSectionEnd)
}

const (
	workflowSectionStart = "<!-- TASKER_WORKFLOW_START -->"
	workflowSectionEnd   = "<!-- TASKER_WORKFLOW_END -->"
	promptsSectionStart  = "<!-- TASKER_PROMPTS_START -->"
	promptsSectionEnd    = "<!-- TASKER_PROMPTS_END -->"
	scheduleSectionStart = "<!-- TASKER_SCHEDULE_START -->"
	scheduleSectionEnd   = "<!-- TASKER_SCHEDULE_END -->"
)

func wrapWorkflowSection(section string) string {
	return wrapMarkedSection(section, workflowSectionStart, workflowSectionEnd)
}

func replaceWorkflowBlock(content string, block string) (string, bool) {
	return replaceMarkedBlock(content, block, workflowSectionStart, workflowSectionEnd)
}

func writeTemplateFile(path string, content string, force bool) error {
	if !force {
		if _, err := os.Stat(path); err == nil {
			return nil
		}
	}
	data := strings.TrimRight(content, "\n") + "\n"
	return atomicWriteFile(path, []byte(data), 0o644)
}

func wrapMarkedSection(section string, start string, end string) string {
	section = strings.TrimRight(section, "\n")
	return start + "\n" + section + "\n" + end
}

func replaceMarkedBlock(content string, block string, startMarker string, endMarker string) (string, bool) {
	start := strings.Index(content, startMarker)
	if start == -1 {
		return content, false
	}
	end := strings.Index(content[start:], endMarker)
	if end == -1 {
		return content, false
	}
	end = start + end + len(endMarker)
	updated := content[:start] + block + content[end:]
	return updated, true
}

func upsertSection(content string, section string, heading string, startMarker string, endMarker string) string {
	content = strings.TrimRight(content, "\n")
	if strings.TrimSpace(content) == "" {
		return wrapMarkedSection(section, startMarker, endMarker) + "\n"
	}

	block := wrapMarkedSection(section, startMarker, endMarker)
	if updated, ok := replaceMarkedBlock(content, block, startMarker, endMarker); ok {
		return strings.TrimRight(updated, "\n") + "\n"
	}

	lines := strings.Split(content, "\n")
	sectionLines := strings.Split(strings.TrimRight(block, "\n"), "\n")
	out := make([]string, 0, len(lines)+len(sectionLines)+4)
	inSection := false
	replaced := false

	for _, line := range lines {
		if strings.TrimSpace(line) == strings.TrimSpace(heading) {
			if !replaced {
				out = append(out, sectionLines...)
				replaced = true
			}
			inSection = true
			continue
		}

		if inSection {
			if strings.HasPrefix(strings.TrimSpace(line), "#") {
				inSection = false
				out = append(out, line)
			}
			continue
		}

		out = append(out, line)
	}

	if !replaced {
		out = append(out, "", "")
		out = append(out, sectionLines...)
	}

	return strings.Join(out, "\n") + "\n"
}

func writeWorkflowSection(filePath string, section string, updater func(string, string) string) (bool, error) {
	current, err := os.ReadFile(filePath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, err
	}

	content := string(current)
	if strings.TrimSpace(content) == "" {
		header := workflowHeaderForFile(filePath)
		if header != "" {
			content = header + "\n\n"
		}
	}

	updated := updater(content, section)
	if err := atomicWriteFile(filePath, []byte(updated), 0o644); err != nil {
		return false, err
	}
	return true, nil
}

func renderPromptsSection(workspace string, nightShiftPath string, proactivePath string) string {
	var b strings.Builder
	b.WriteString("## Tasker Prompts\n")
	if nightShiftPath != "" {
		b.WriteString("Night Shift: " + pathForConfig(workspace, nightShiftPath) + "\n")
	}
	if proactivePath != "" {
		b.WriteString("Proactive Operator: " + pathForConfig(workspace, proactivePath) + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func upsertPromptsSection(content string, section string) string {
	return upsertSection(content, section, "## Tasker Prompts", promptsSectionStart, promptsSectionEnd)
}

type scheduleSectionInput struct {
	Workspace        string
	WindowLabel      string
	HeartbeatLabel   string
	HeartbeatCount   int
	HeartbeatPrompt  string
	HeartbeatReads   []string
	HeartbeatCmds    []string
	NightShiftPrompt string
	Timezone         string
	NightlyCron      string
	NoHeartbeat      bool
	NoNightly        bool
}

func renderScheduleSection(in scheduleSectionInput) string {
	var b strings.Builder
	b.WriteString("## Tasker Schedule\n")
	if in.WindowLabel != "" {
		b.WriteString("Window: " + in.WindowLabel + "\n")
	}
	if in.HeartbeatLabel != "" {
		b.WriteString("Heartbeat every: " + in.HeartbeatLabel + "\n")
	}
	if in.HeartbeatCount > 0 {
		b.WriteString(fmt.Sprintf("Heartbeat count: %d\n", in.HeartbeatCount))
	}
	if !in.NoHeartbeat && in.HeartbeatPrompt != "" {
		b.WriteString("Heartbeat prompt: " + pathForConfig(in.Workspace, in.HeartbeatPrompt) + "\n")
	}
	if in.NightShiftPrompt != "" {
		b.WriteString("Night Shift prompt: " + pathForConfig(in.Workspace, in.NightShiftPrompt) + "\n")
	}
	if strings.TrimSpace(in.Timezone) != "" {
		b.WriteString("Timezone: " + strings.TrimSpace(in.Timezone) + "\n")
	}
	if !in.NoNightly && strings.TrimSpace(in.NightlyCron) != "" {
		b.WriteString("Nightly cron: " + strings.TrimSpace(in.NightlyCron) + "\n")
	}
	if len(in.HeartbeatReads) > 0 {
		b.WriteString("Heartbeat reads:\n")
		for _, entry := range in.HeartbeatReads {
			entry = strings.TrimSpace(entry)
			if entry == "" {
				continue
			}
			b.WriteString("  - " + entry + "\n")
		}
	}
	if len(in.HeartbeatCmds) > 0 {
		b.WriteString("Heartbeat commands:\n")
		for _, cmd := range in.HeartbeatCmds {
			cmd = strings.TrimSpace(cmd)
			if cmd == "" {
				continue
			}
			b.WriteString("  - " + cmd + "\n")
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func upsertScheduleSection(content string, section string) string {
	return upsertSection(content, section, "## Tasker Schedule", scheduleSectionStart, scheduleSectionEnd)
}

func parseDurationFlexible(value string, fallback string) (time.Duration, string, error) {
	raw := strings.TrimSpace(value)
	if raw == "" {
		raw = fallback
	}
	if d, err := time.ParseDuration(raw); err == nil {
		return d, raw, nil
	}
	if n, err := strconv.Atoi(raw); err == nil && n > 0 {
		return time.Duration(n) * time.Hour, fmt.Sprintf("%dh", n), nil
	}
	return 0, "", fmt.Errorf("invalid duration %q (use 24h, 2h, 30m)", raw)
}

func windowHeartbeatCount(window time.Duration, heartbeat time.Duration) int {
	if window <= 0 || heartbeat <= 0 {
		return 0
	}
	count := int(window / heartbeat)
	if window%heartbeat != 0 {
		count++
	}
	if count < 1 {
		count = 1
	}
	return count
}

type scheduleExampleInput struct {
	HeartbeatEvery time.Duration
	HeartbeatLabel string
	HeartbeatPath  string
	NightShiftPath string
	Timezone       string
	NightlyCron    string
	NoNightly      bool
}

func printScheduleExamples(in scheduleExampleInput) {
	fmt.Println()
	fmt.Println("Cron examples (OpenClaw):")
	tz := strings.TrimSpace(in.Timezone)
	tzArg := " --tz <Your/Timezone>"
	if tz != "" {
		tzArg = " --tz " + tz
	}
	if !in.NoNightly {
		cron := strings.TrimSpace(in.NightlyCron)
		if cron == "" {
			cron = "0 23 * * *"
		}
		fmt.Printf("  openclaw cron add --name \"Night Shift Builder\" --cron \"%s\"%s --session isolated --message \"Read and execute %s. Create spec.md, tasks.md, HANDOFF.md in management/RUNS/YYYY-MM-DD-night-shift/ and send a handoff summary.\"\n", cron, tzArg, in.NightShiftPath)
	}

	heartbeatCron := heartbeatCronExpr(in.HeartbeatEvery)
	if heartbeatCron == "" {
		fmt.Printf("  Heartbeat: schedule every %s and use message: \"Read and execute %s\"\n", in.HeartbeatLabel, in.HeartbeatPath)
		return
	}
	fmt.Printf("  openclaw cron add --name \"Tasker Heartbeat\" --cron \"%s\"%s --session isolated --message \"Read and execute %s\"\n", heartbeatCron, tzArg, in.HeartbeatPath)
}

func heartbeatCronExpr(d time.Duration) string {
	if d <= 0 {
		return ""
	}
	if d%time.Hour == 0 {
		hours := int(d / time.Hour)
		if hours <= 0 {
			return ""
		}
		return fmt.Sprintf("0 */%d * * *", hours)
	}
	if d%time.Minute == 0 {
		minutes := int(d / time.Minute)
		if minutes <= 0 || minutes >= 60 {
			return ""
		}
		return fmt.Sprintf("*/%d * * * *", minutes)
	}
	return ""
}
func atomicWriteFile(path string, data []byte, perm fs.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp := filepath.Join(dir, fmt.Sprintf(".tmp-%d", time.Now().UnixNano()))
	if err := os.WriteFile(tmp, data, perm); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}
