---
name: task
description: Tasker docstore tasks via tool-dispatch, slash-only. Use for explicit /task commands and deterministic task operations.
user-invocable: true
disable-model-invocation: true
command-dispatch: tool
command-tool: tasker_cmd
command-arg-mode: raw
metadata: {"clawdbot":{}}
---

Use `/task` to manage tasks via the deterministic `tasker` CLI.
This profile is slash-only (natural language disabled). For NL, use `skills/task/`.

Examples:
- `/task init`
- `/task project add "Work"`
- `/task add "Draft proposal" --project Work --column todo --due 2026-01-23 --priority high --tag client --details "Send a revised scope summary."`
- `/task ls --project Work`
- `/task board --project Work --format telegram`
- `/task tasks --project Work --format telegram`   # due today + overdue
- `/task week --project Work --days 7 --format telegram`
- `/task done "Draft proposal"`

Output modes:
- `--json`, `--ndjson`, `--plain`, `--ascii`, `--format telegram`
- JSON/NDJSON write to `<root>/exports` by default (no stdout JSON unless `--stdout-json`/`--stdout-ndjson` is used).
