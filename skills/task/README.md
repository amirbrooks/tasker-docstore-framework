# Tasker Skill — Task + Idea CLI for OpenClaw

Trigger: `/task` (optional)

Lean task + idea management via the `tasker` CLI. Natural language maps to CLI args; `/task` passes through unchanged.

## Flow
1) Determine intent  
If the user uses `/task`, pass args through unchanged. Otherwise translate to CLI args.  
If `agent.require_explicit` is true, only act on `/task` or explicit "tasker" requests.

2) Resolve selectors safely  
Use smart matching (exact → prefix → contains → search). If not unique, ask to clarify.

3) Execute and respond  
Run `tasker_cmd`, summarize in plain text, and keep IDs hidden in chat.

4) Notes are explicit  
Prefer `note add <selector...> -- <text...>` to avoid ambiguity.

## What it does
- Converts natural language to deterministic `tasker` commands
- Keeps output human‑friendly (no IDs in chat)
- Uses smart selector matching (exact → prefix → contains → search)
- Supports chat‑friendly output with `--format telegram`

## Explicit-only mode
If you want **only** `/task` or "tasker" requests to execute, set:
```json
{
  "agent": {
    "require_explicit": true
  }
}
```
This lives in `<root>/config.json` (see `docs/CLI_SPEC.md`).

## Workspace artifacts (spec/tasks/handoff)
This skill can help agents create project run artifacts using templates:
- Templates live in `docs/templates/`
  - `docs/templates/spec.md`
  - `docs/templates/tasks.md`
  - `docs/templates/HANDOFF.md`
- If your workspace has a "Tasker Workflow" section in `management/tasker/workflow.md`, the agent should follow those paths.
- Default run path: `<workspace>/management/RUNS/<YYYY-MM-DD>-<short-name>/`
See `docs/AGENT_WORKFLOW.md` for the full workflow and example config.

## Heartbeat behavior
On heartbeat requests, the agent should **suggest** commands (not execute) such as:
- `tasker tasks [--project <default>] --format telegram`
- `tasker week [--project <default>] --days 7 --format telegram`

## Requirements
- OpenClaw plugin installed (`extensions/tasker`) to provide `tasker_cmd`
- `tasker_cmd` tool allowlisted
- `tasker` CLI available via plugin `binary`, `TASKER_BIN`, or PATH

## Install
- Use this folder as `task/`
- Install to one of:
  - `<workspace>/skills/task/` (preferred)
  - `~/.openclaw/skills/task/`
- OpenClaw loads skills from `<workspace>/skills` and `~/.openclaw/skills` (workspace takes precedence).
- Default workspace is `~/.openclaw/workspace`, so `<workspace>/skills` is `~/.openclaw/workspace/skills`.
- Installer: `./scripts/install-skill.sh --dest <skills-dir>`

Full setup: `docs/OPENCLAW_INTEGRATION.md`

## Quick usage
Natural language:
- “tasks today” → `tasks --open --format telegram`
- “what’s our week” → `week --days 7 --format telegram`
- “add draft spec | due 2026-01-23” → `add --text "draft spec | due 2026-01-23" --format telegram`
- “capture UI review | due 2026-01-23” → `capture "UI review | due 2026-01-23" --format telegram`
- “capture idea pricing notes | #pricing” → `idea capture "pricing notes | #pricing" --format telegram`
- “add idea note pricing notes | follow up” → `idea note add "pricing notes" -- "follow up"`
- “promote idea pricing notes” → `idea promote "pricing notes" --format telegram`

Slash commands:
- `/task tasks --project Work --format telegram`
- `/task add --text "Fix auth | due 2026-01-23" --project Work`
- `/task resolve "auth" --match search`
- `/task idea ls --scope all --format telegram`
- `/task idea note add "pricing notes" -- "follow up"`
- `/task idea promote "pricing notes" --project Work --column todo`

## Selector behavior (important)
- Default `--match` is **auto**: exact → prefix → contains → search
- `--match search` includes title + notes/body
- Actions only proceed on a **unique** match; otherwise you’ll be prompted to clarify

## Why use this over a plain Markdown list?
Tasker keeps Markdown but adds structured metadata and deterministic views while hiding machine IDs from human output.
