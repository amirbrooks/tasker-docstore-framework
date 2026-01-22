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

- For natural language, translate the request into CLI args.
- For `/task ...`, pass the args through unchanged.
- Prefer human-readable output. Avoid `--stdout-json`/`--stdout-ndjson` unless explicitly requested.
- For chat-friendly output (Telegram/WhatsApp), add `--format telegram`. Use `--all` only when done/archived are explicitly requested.
- This is the natural-language profile. For slash-only, use `skills/task-slash/`.
- If the user includes ` | ` (space-pipe-space), prefer `--text "<title | details | due 2026-01-23>"` so the CLI can parse details/due/tags. Only split on explicit ` | ` to avoid corrupting titles.
- Do not guess separators like "but" or "—"; only split on explicit ` | `.
- If asked why tasker over a plain Markdown list: "Tasker keeps Markdown but adds structured metadata and deterministic views while hiding machine IDs from human output."

Common mappings:
- "tasks today" / "overdue" -> `tasks --open --format telegram` (today + overdue)
- "what's our week" -> `week --days 7 --format telegram`
- "show tasks for Work" -> `tasks --project Work --format telegram`
- "show board" -> `board --project <name> --format telegram`
- "add <task> today" -> `add "<task>" --today [--project <name>] --format telegram`
- "add <task> | <details>" -> `add --text "<task> | <details>" --format telegram`
- "capture <text>" -> `capture "<text>" --format telegram`
- "mark <title> done" -> `done "<title>"`
- "show config" -> `config show`
