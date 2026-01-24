---
name: task
description: Tasker docstore tasks + ideas via tool-dispatch, slash-only. Use for explicit /task commands and deterministic operations.
user-invocable: true
disable-model-invocation: true
command-dispatch: tool
command-tool: tasker_cmd
command-arg-mode: raw
metadata: {"clawdbot":{}}
---

Use `/task` to manage tasks and ideas via the deterministic `tasker` CLI.
This profile is slash-only (natural language disabled). For NL, use `skills/task/`.

Examples:
- `/task init`
- `/task project add "Work"`
- `/task idea add "Product vision notes" --tag strategy`
- `/task idea capture "Pricing experiment | #pricing"`
- `/task idea note add "Pricing experiment" -- "ask sales for feedback"`
- `/task add "Draft proposal" --project Work --column todo --due 2026-01-23 --priority high --tag client --details "Send a revised scope summary."`
- `/task add --text "Draft proposal | outline scope | due 2026-01-23" --project Work`
- `/task capture "Quick note | due 2026-01-23"`
- `/task ls --project Work`
- `/task idea ls --scope all`
- `/task board --project Work --format telegram`
- `/task tasks --project Work --format telegram`   # due today + overdue
- `/task week --project Work --days 7 --format telegram`
- `/task done "Draft proposal"`
- `/task idea promote "Pricing experiment" --project Work --column todo --delete`
- `/task resolve "Draft proposal"`

Output modes:
- `--json`, `--ndjson`, `--plain`, `--ascii`, `--format telegram`
- JSON/NDJSON write to `<root>/exports` by default (no stdout JSON unless `--stdout-json`/`--stdout-ndjson` is used).
