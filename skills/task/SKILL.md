---
name: task
description: Tasker docstore tasks via tool-dispatch (no model bloat)
user-invocable: true
disable-model-invocation: true
command-dispatch: tool
command-tool: tasker_cmd
command-arg-mode: raw
metadata: {"clawdbot":{"emoji":"🗂️","requires":{"bins":["tasker"]}}}
---

Use `/task` to manage tasks via the deterministic `tasker` CLI.

Examples:
- `/task init`
- `/task project add "Work"`
- `/task add "Draft proposal" --project Work --column todo --due 2026-01-23 --priority high --tag client`
- `/task ls --project Work`
- `/task board --project Work --ascii`
- `/task tasks --project Work`   # due today + overdue
- `/task done tsk_01J4F3N8`

Output modes:
- `--json`, `--ndjson`, `--plain`, `--ascii`
- JSON/NDJSON write to `<root>/exports` by default (no stdout JSON unless `--stdout-json`/`--stdout-ndjson` is used).
