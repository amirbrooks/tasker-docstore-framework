---
name: tasker-codex
description: Manage tasks and ideas in the tasker docstore CLI using natural language or explicit commands. Use for tasks today/overdue, idea capture/promotion, listing, moving, marking done, or onboarding to tasker.
---

# Tasker Codex

## Overview
Use the `tasker` CLI in this repo to manage docstore tasks. Interpret plain‑text requests and execute the matching CLI command, then summarize the human output. Avoid printing raw JSON in the Codex interface.
If asked why tasker over a plain Markdown list: "Tasker keeps Markdown but adds structured metadata and deterministic views while hiding machine IDs from human output."
If a selector is partial, run `./tasker resolve "<query>"` (uses smart fallback; `--match search` includes notes/body), then act by ID if there is exactly one match.
For notes, prefer `./tasker note add <selector...> -- <text...>` to avoid ambiguity; without `--`, tasker will attempt to infer the split.

## Quick start
- If `./tasker` is missing, build it: `go build -o tasker ./cmd/tasker`.
- Respect `--root <path>` when provided; otherwise let the CLI default to `~/.tasker`.
- If users have recurring defaults, suggest `TASKER_PROJECT`, `TASKER_VIEW`, and `TASKER_OPEN_ONLY`.
- For long idea content, prefer `./tasker idea add --stdin` with piped input.

## Workspace artifacts (spec/tasks/handoff)
When asked to produce a run (spec/tasks/handoff), use the repo templates:
- `docs/templates/spec.md`
- `docs/templates/tasks.md`
- `docs/templates/HANDOFF.md`
If the user has a workspace config (e.g., a "Tasker Workflow" section in `management/tasker/workflow.md`), follow its run directory and template paths.  
Default run path: `<workspace>/management/RUNS/<YYYY-MM-DD>-<short-name>/`.
See `docs/AGENT_WORKFLOW.md` for the full workflow and example config.

## Heartbeat behavior (suggestions only)
On heartbeat requests, do **not** run `tasker` commands automatically.  
Suggest the configured commands to run, or default to:
- `tasker tasks [--project <default>] --format telegram`
- `tasker week [--project <default>] --days 7 --format telegram`

## Intent → command mapping
- “tasks today”, “what’s due”, “tasks available/running”, “overdue tasks”
  - Run: `./tasker tasks [--project <name>]`
  - This shows **due today + overdue** in human format.
- “what tasks left for today”, “what’s left today”
  - Run: `./tasker tasks today --open [--project <name>] [--group <project|column>] [--totals]`
- “list tasks”, “show tasks for <project>”
  - Run: `./tasker ls [--project <name>] [--column <col>] [--status <s>] [--tag <t>]`
- “what’s our week looking like”, “upcoming tasks”, “agenda”
  - Run: `./tasker week [--project <name>] [--days N] [--group <project|column>] [--totals]`
- “add task …”
  - Run: `./tasker add "<title>" --project <name> [--column <col>] [--due <YYYY-MM-DD> | --today | --tomorrow | --next-week] [--priority <p>] [--tag <t>] [--details "<text>"]`
  - If the user includes ` | `, prefer `./tasker add --text "<title | details | due 2026-01-23>" ...`
- “capture task …”
  - Run: `./tasker capture "<title | details | due 2026-01-23>"`
- “add idea …”
  - Run: `./tasker idea add "<title>" [--project <name>] [--tag <t>...]`
- “capture idea …”
  - Run: `./tasker idea capture "<title | details | #tag>"`
- “list ideas …”
  - Run: `./tasker idea ls [--scope root|project|all] [--project <name>] [--tag <t>] [--search <q>]`
- “show idea …”
  - Run: `./tasker idea show "<title>"`
- “add note to idea …”
  - Run: `./tasker idea note add "<title>" -- "<text>"`
- “promote idea …”
  - Run: `./tasker idea promote "<title>" [--project <name>] [--column <col>] [--link] [--delete]`
- “mark done”, “complete task <title>”
  - Run: `./tasker done "<title>"`
- “move task <title> to <column>”
  - Run: `./tasker mv "<title>" <column>`
- “show task <title>”
  - Run: `./tasker show "<title>"`
- “resolve task <title>”
  - Run: `./tasker resolve "<title>"` (returns JSON with IDs)
- “add note to task <title>”
  - Run: `./tasker note add "<title>" "<text>"`
- “show board”
  - Run: `./tasker board --project <name> [--ascii]`
- “how do I start?”, “onboarding”
  - Run: `./tasker onboarding`
- “show config”, “what are my settings?”
  - Run: `./tasker config show`
- “set default project to Work”
  - Run: `./tasker config set agent.default_project "Work"`
- “default view should be week”
  - Run: `./tasker config set agent.default_view week`

## Output rules (Codex interface)
- Prefer human output only. Do not print raw JSON to the Codex interface.
- If a user explicitly asks for JSON, run with `--json` (or `--ndjson`) so the CLI writes to `<root>/exports`, then report the export path.
- Summarize key results in plain text even when exporting JSON.

## Agent activation (optional config)
If `<root>/config.json` has `agent.require_explicit: true`, only act when the user explicitly uses `/task` or "tasker". Otherwise, ask them to confirm running tasker commands.

## User preference prompts (first-time setup)
If no agent defaults are set, ask the user for preferences and suggest adding them to config:
- Default project?
- Default view: today or week?
- Open-only by default?
- Group summaries by project or column? Show per-group totals?
