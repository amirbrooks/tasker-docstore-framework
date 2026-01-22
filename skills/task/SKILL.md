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
- This is the natural-language profile. For slash-only, use `skills/task-slash/`.

Common mappings:
- "tasks today" / "overdue" -> `tasks --open` (today + overdue)
- "what's our week" -> `week --days 7`
- "show tasks for Work" -> `tasks --project Work`
- "add <task> today" -> `add "<task>" --today [--project <name>]`
- "mark <id> done" -> `done <id>`
- "show config" -> `config show`
