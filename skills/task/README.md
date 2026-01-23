# Tasker Skill — Task CLI for Clawdbot

Trigger: `/task` (optional)

Lean task management via the `tasker` CLI. Natural language maps to CLI args; `/task` passes through unchanged.

## Flow
1) Determine intent  
If the user uses `/task`, pass args through unchanged. Otherwise translate to CLI args.

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

## Requirements
- Clawdbot plugin installed (`extensions/tasker`) to provide `tasker_cmd`
- `tasker_cmd` tool allowlisted
- `tasker` CLI available via plugin `binary`, `TASKER_BIN`, or PATH

## Install
- Natural language profile (recommended): use this folder as `task/`
- Slash‑only profile: copy `skills/task-slash/` into your skills folder as `task/`
- Install to one of:
  - `<workspace>/skills/task/` (preferred)
  - `~/.clawdbot/skills/task/`
- Install only one profile at a time (remove the other if switching)

Full setup: `docs/CLAWDBOT_INTEGRATION.md`

## Quick usage
Natural language:
- “tasks today” → `tasks --open --format telegram`
- “what’s our week” → `week --days 7 --format telegram`
- “add draft spec | due 2026-01-23” → `add --text "draft spec | due 2026-01-23" --format telegram`
- “capture UI review | due 2026-01-23” → `capture "UI review | due 2026-01-23" --format telegram`

Slash commands:
- `/task tasks --project Work --format telegram`
- `/task add --text "Fix auth | due 2026-01-23" --project Work`
- `/task resolve "auth" --match search`

## Selector behavior (important)
- Default `--match` is **auto**: exact → prefix → contains → search
- `--match search` includes title + notes/body
- Actions only proceed on a **unique** match; otherwise you’ll be prompted to clarify

## Why use this over a plain Markdown list?
Tasker keeps Markdown but adds structured metadata and deterministic views while hiding machine IDs from human output.
