# tasker (Docstore) — lightweight task manager (no DB)

One binary. Local‑first tasks in Markdown. Works great with agents.

This is a **framework + scaffolding** for a local-first task manager that stores tasks as **Markdown files** with YAML frontmatter,
organized in a **filesystem Kanban** layout (projects + columns).

## Why tasker

- Local‑first: tasks live as plain Markdown files.
- Agent‑friendly: human summaries by default, machine exports on demand.
- Simple: one binary, no database, easy backup with Git.

## 30‑second quickstart

```bash
npm install -g @amirbrooks/tasker-docstore
tasker init --project "Work"
tasker add "First task" --project Work --today
tasker tasks --project Work
```

It is designed to integrate with **Clawdbot** via:
- a plugin tool (`tasker_cmd`) that safely spawns the `tasker` CLI with `shell:false`
- a skill (`/task`) that uses `command-dispatch: tool` to bypass the model (low-bloat)

## Install

Pick one:

**npm (recommended for non‑Go users)**  
Downloads the prebuilt binary from GitHub releases.
```bash
npm install -g @amirbrooks/tasker-docstore
```

**Go (recommended for Go users)**
```bash
go install github.com/amirbrooks/tasker-docstore-framework/cmd/tasker@latest
```

**Build from source**
```bash
go build -o tasker ./cmd/tasker
```

**Local install script**
```bash
./scripts/install.sh
```

## Quickstart

1) Initialize store (defaults to `~/.tasker`):
```bash
tasker init --project "Work"
```

Optional onboarding:
```bash
tasker onboarding
```

View config (human summary):
```bash
tasker config show
```

Update config (agent defaults):
```bash
tasker config set agent.default_project Work
tasker config set agent.default_view week
tasker config set agent.open_only true
```

2) Create another project (optional):
```bash
tasker project add "Side"
```

3) Add tasks:
```bash
tasker add "Draft proposal" --project Work --column todo --today --priority high --tag client
tasker add "Send recap" --project Work --tomorrow
tasker add "Plan next sprint" --project Work --next-week
tasker add "Fix auth bug" --project Work --column doing
```

4) List tasks:
```bash
tasker ls --project Work
tasker ls --project Work --json   # writes JSON to <root>/exports
```

5) Due today + overdue:
```bash
tasker tasks --project Work
```

6) Board view:
```bash
tasker board --project Work --ascii
```

7) Week view:
```bash
tasker week --project Work --days 7
```

Optional agent defaults (`<root>/config.json`):
```json
{
  "agent": {
    "default_project": "work",
    "default_view": "today",
    "week_days": 7,
    "open_only": true,
    "require_explicit": false,
    "summary_group": "project",
    "summary_totals": true
  }
}
```

### Defaults & UX

Set defaults to reduce flags:
- `TASKER_PROJECT=work` (env) or `agent.default_project` (config)
- `TASKER_VIEW=week` (env) or `agent.default_view=week` (config)
- `TASKER_OPEN_ONLY=true` (env) or `agent.open_only=true` (config)
- `TASKER_GROUP=project` + `TASKER_TOTALS=true` (env) for grouped summaries

Output options:
- Human‑readable summaries by default
- `--plain` for stable tab‑separated output
- `--json` / `--ndjson` write to `<root>/exports` (stdout JSON disabled by default)

## Codex / agent usage

- Natural language: “tasks today for Work” → `tasker tasks --project Work`
- Natural language: “what’s our week looking like?” → `tasker week --project Work`
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

## Contributing

See `CONTRIBUTING.md`.

## Security

See `SECURITY.md` and `docs/SECURITY.md`.

## Support

See `SUPPORT.md`.

## Release (maintainers)

1) Tag a release (triggers GoReleaser):
```bash
git tag v0.1.0
git push origin v0.1.0
```

2) Publish npm wrapper (requires 2FA-enabled npm token):
```bash
cd npm
npm publish --access public
```

Notes:
- The npm wrapper downloads binaries from GitHub Releases, so the tag must exist before `npm publish`.

## License

MIT (see `LICENSE`).
