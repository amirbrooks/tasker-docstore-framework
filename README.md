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

Install (Go, from GitHub):
```bash
go install github.com/amirbrooks/tasker-docstore-framework/cmd/tasker@latest
```

Install (npm wrapper):
```bash
npm install -g @amirbrooks/tasker-docstore
```

2) Initialize store (defaults to `~/.tasker`):
```bash
./tasker init --project "Work"
```

Optional onboarding:
```bash
./tasker onboarding
```

View config (human summary):
```bash
./tasker config show
```

Update config (agent defaults):
```bash
./tasker config set agent.default_project Work
./tasker config set agent.default_view week
./tasker config set agent.open_only true
```

3) Create another project (optional):
```bash
./tasker project add "Side"
```

4) Add tasks:
```bash
./tasker add "Draft proposal" --project Work --column todo --today --priority high --tag client
./tasker add "Send recap" --project Work --tomorrow
./tasker add "Plan next sprint" --project Work --next-week
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

8) Week view:
```bash
./tasker week --project Work --days 7
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
