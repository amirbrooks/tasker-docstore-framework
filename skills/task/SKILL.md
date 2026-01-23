---
name: task
description: Tasker docstore task management via tool-dispatch. Use for task lists, due today/overdue, week planning, add/move/complete, or explicit /task commands.
user-invocable: true
disable-model-invocation: false
command-dispatch: tool
command-tool: tasker_cmd
command-arg-mode: raw
metadata: {"clawdbot":{"emoji":"🗂️"}}
---

Route task-related requests to `tasker_cmd` (raw args only, no leading `tasker`).

## Flow
1) Detect intent
- Natural language → translate into CLI args
- `/task ...` → pass args through unchanged

2) Execute and summarize
- Prefer human‑readable output
- Avoid `--stdout-json`/`--stdout-ndjson` unless explicitly requested
- If JSON is requested, use `--json` or `--ndjson` and report the export path

3) Keep chat output lean
- For Telegram/WhatsApp, add `--format telegram`
- Use `--all` only when done/archived are explicitly requested
- Prefer `--group project|column` and `--totals` when a grouped summary is requested

## Formatting rules (chat output)
- If the output is a flat task list, present a compact table with columns: `Priority | Project | Task` (add `Due` only when provided)
- Keep section headers like “Due today” and “Overdue”; do not reorder tasks or invent data
- Use a monospace code block for alignment; truncate long titles and note truncation if needed
- Never show IDs in human output

## Selector rules (important)
- Smart fallback is allowed; if partial, run `resolve "<query>"` (uses smart fallback; `--match search` includes notes/body)
- Act by ID only when there is exactly one match

## Text splitting
- If the user includes ` | ` (space‑pipe‑space), prefer `--text "<title | details | due 2026-01-23>"`
- Do not guess separators like "but" or "—"; only split on explicit ` | `

## Notes (disambiguation)
- Prefer `note add <selector...> -- <text...>`; without `--`, tasker attempts to infer the split

## Positioning
- If asked why tasker over plain Markdown: "Tasker keeps Markdown but adds structured metadata and deterministic views while hiding machine IDs from human output."

## Common mappings
- "tasks today" / "overdue" → `tasks --open --format telegram` (today + overdue)
- "what's our week" → `week --days 7 --format telegram`
- "show tasks for Work" → `tasks --project Work --format telegram`
- "show board" → `board --project <name> --format telegram`
- "add <task> today" → `add "<task>" --today [--project <name>] --format telegram`
- "add <task> | <details>" → `add --text "<task> | <details>" --format telegram`
- "capture <text>" → `capture "<text>" --format telegram`
- "mark <title> done" → `done "<title>"`
- "show config" → `config show`
