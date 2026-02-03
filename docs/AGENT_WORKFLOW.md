# Agent Workflow (spec/tasks/handoff + heartbeat)

This document describes how agents should create run artifacts without changing the CLI.
It relies on the unified `skills/task` skill and the templates in `docs/templates/`.

## Workflow artifacts
Each run should create three files in the user-selected workspace path:
- `spec.md`
- `tasks.md`
- `HANDOFF.md`

Use the templates from `docs/templates/` to create these files. If direct copying is not
possible, mirror the headings and structure exactly.
`tasker workflow init` writes embedded templates that mirror `docs/templates/`.
The CLI wraps the config section in markers so re-running init is safe:
```
<!-- TASKER_WORKFLOW_START -->
...
<!-- TASKER_WORKFLOW_END -->
```

### Default run path
`<workspace>/management/RUNS/<YYYY-MM-DD>-<short-name>/`

### Optional commands to scaffold a run
```bash
mkdir -p <run-path>
cp docs/templates/spec.md <run-path>/spec.md
cp docs/templates/tasks.md <run-path>/tasks.md
cp docs/templates/HANDOFF.md <run-path>/HANDOFF.md
```

### CLI helper (writes workspace config + templates)
```bash
tasker workflow init
```

### Prompt setup (writes Night Shift + Proactive Operator prompts)
```bash
tasker workflow prompts init
```

### Schedule setup (writes heartbeat prompt + schedule)
```bash
tasker workflow schedule init --window 24h --heartbeat-every 2h
```

Prompt templates live in `docs/prompts/` and are mirrored by the embedded CLI templates.

### Optional tasker tracking (CLI remains unchanged)
Use tasker to track the run without adding new commands:
```bash
tasker add "Run: <short-name>" --project <project> --details "Spec: <run-path>/spec.md"
tasker note add "Run: <short-name>" -- "Handoff: <run-path>/HANDOFF.md"
```

## Workspace configuration (optional)
Users can override defaults by adding sections to
`management/tasker/workflow.md` in their OpenClaw workspace.

Example:
```md
## Tasker Workflow
Runs dir: management/RUNS
Run name: YYYY-MM-DD-<short-name>
Templates:
  spec: docs/templates/spec.md
  tasks: docs/templates/tasks.md
  handoff: docs/templates/HANDOFF.md
Heartbeat mode: suggest
Heartbeat commands:
  - tasker tasks --project Work --format telegram
  - tasker week --project Work --days 7 --format telegram

## Tasker Prompts
Night Shift: management/NIGHT_SHIFT.md
Proactive Operator: management/PROACTIVE_OPERATOR.md

## Tasker Schedule
Window: 24h
Heartbeat every: 2h
Heartbeat count: 12
Heartbeat prompt: management/HEARTBEAT.md
Heartbeat reads:
  - AGENTS.md (if present)
  - USER.md (if present)
  - MEMORY.md (if present)
  - management/tasker/workflow.md
  - management/BACKLOG.md
  - latest run in management/RUNS
Heartbeat commands:
  - tasker tasks --format telegram
  - tasker week --days 7 --format telegram
```

Notes:
- Paths can be relative to the workspace or repo.
- If a template path is missing, fall back to the default templates in this repo.
- Keep output lean; do not dump full files in chat.

## Heartbeat behavior (suggestions only)
On heartbeat requests, do **not** run commands automatically. Suggest the commands
listed in the workspace config. If none are configured, default to:
- `tasker tasks [--project <default>] --format telegram`
- `tasker week [--project <default>] --days 7 --format telegram`

## Explicit-only mode (optional)
Set `agent.require_explicit: true` in `<root>/config.json` to require `/task` or an
explicit "tasker" request before running commands.
