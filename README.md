# tasker (Docstore) — lightweight task manager (no DB)

This is a **framework + scaffolding** for a local-first task manager that stores tasks as **Markdown files** with YAML frontmatter,
organized in a **filesystem Kanban** layout (projects + columns).

It is designed to integrate with **Clawdbot** via:
- a plugin tool (`tasker_cmd`) that safely spawns the `tasker` CLI with `shell:false`
- a skill (`/task`) that uses `command-dispatch: tool` to bypass the model (low-bloat)

## Quickstart (CLI)

1) Build (Go 1.22+):
```bash
go build -o tasker ./cmd/tasker
```

Install (local user bin):
```bash
./scripts/install.sh
```

Install (from source checkout, local):
```bash
go install ./cmd/tasker
```

2) Initialize store (defaults to `~/.tasker`):
```bash
./tasker init
```

Optional onboarding:
```bash
./tasker onboarding
```

3) Create a project:
```bash
./tasker project add "Work"
```

4) Add tasks:
```bash
./tasker add "Draft proposal" --project Work --column todo --due 2026-01-23 --priority high --tag client
./tasker add "Fix auth bug" --project Work --column doing
```

5) List tasks:
```bash
./tasker ls --project Work
./tasker ls --project Work --json   # writes JSON to <root>/exports
```

6) Due today + overdue:
```bash
./tasker tasks --project Work
```

7) Board view:
```bash
./tasker board --project Work --ascii
```

## Codex / agent usage

- Natural language: “tasks today for Work” → `tasker tasks --project Work`
- Onboarding: `tasker onboarding`

JSON/NDJSON exports write to `<root>/exports` and are not printed to stdout unless `--stdout-json` or `--stdout-ndjson` is used.

## Storage model (no DB)

Everything lives under a root folder (default: `~/.tasker`):

- `.tasker/config.json` (store config)
- `.tasker/projects/<project-slug>/project.json`
- `.tasker/projects/<project-slug>/columns/<column-dir>/*.md` (tasks)

See `docs/STORAGE_SPEC.md`.

## Clawdbot integration

See `docs/CLAWDBOT_INTEGRATION.md`.
