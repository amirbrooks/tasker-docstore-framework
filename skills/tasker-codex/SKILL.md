---
name: tasker-codex
description: Manage tasks in the tasker docstore CLI using natural language or explicit commands. Use when a user asks for “tasks today/overdue,” adding tasks, listing tasks, moving tasks between columns, marking done, viewing a board, or onboarding to tasker.
---

# Tasker Codex

## Overview
Use the `tasker` CLI in this repo to manage docstore tasks. Interpret plain‑text requests and execute the matching CLI command, then summarize the human output. Avoid printing raw JSON in the Codex interface.

## Quick start
- If `./tasker` is missing, build it: `go build -o tasker ./cmd/tasker`.
- Respect `--root <path>` when provided; otherwise let the CLI default to `~/.tasker`.

## Intent → command mapping
- “tasks today”, “what’s due”, “tasks available/running”, “overdue tasks”
  - Run: `./tasker tasks [--project <name>]`
  - This shows **due today + overdue** in human format.
- “what tasks left for today”, “what’s left today”
  - Run: `./tasker tasks today --open [--project <name>]`
- “list tasks”, “show tasks for <project>”
  - Run: `./tasker ls [--project <name>] [--column <col>] [--status <s>] [--tag <t>]`
- “what’s our week looking like”, “upcoming tasks”, “agenda”
  - Run: `./tasker week [--project <name>] [--days N]`
- “add task …”
  - Run: `./tasker add "<title>" --project <name> [--column <col>] [--due <YYYY-MM-DD>] [--priority <p>] [--tag <t>]`
- “mark done”, “complete task <id>”
  - Run: `./tasker done <id-or-prefix>`
- “move task <id> to <column>”
  - Run: `./tasker mv <id-or-prefix> <column>`
- “show task <id>”
  - Run: `./tasker show <id-or-prefix>`
- “add note to task <id>”
  - Run: `./tasker note add <id-or-prefix> "<text>"`
- “show board”
  - Run: `./tasker board --project <name> [--ascii]`
- “how do I start?”, “onboarding”
  - Run: `./tasker onboarding`

## Output rules (Codex interface)
- Prefer human output only. Do not print raw JSON to the Codex interface.
- If a user explicitly asks for JSON, run with `--json` (or `--ndjson`) so the CLI writes to `<root>/exports`, then report the export path.
- Summarize key results in plain text even when exporting JSON.

## Agent activation (optional config)
If `<root>/config.json` has `agent.require_explicit: true`, only act when the user explicitly uses `/task` or “tasker”. Otherwise, ask them to confirm running tasker commands.
