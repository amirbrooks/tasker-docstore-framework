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
# after install
tasker init --project "Work"
tasker add "First task" --project Work --today
tasker tasks --project Work
```

## Setup (CLI + OpenClaw)

0) Bootstrap OpenClaw (recommended):

```bash
openclaw onboard
# or
openclaw configure
```

OpenClaw stores config at `~/.openclaw/openclaw.json` by default (override with `OPENCLAW_CONFIG_PATH`).
The default workspace is `~/.openclaw/workspace`.

1) Install the CLI (pick one):

**npm (recommended for non‑Go users)**  
```bash
npm install -g @amirbrooks/tasker-docstore
```

**Go (recommended for Go users)**  
```bash
go install github.com/amirbrooks/tasker-docstore-framework/cmd/tasker@latest
```

2) Install the OpenClaw plugin (registers `tasker_cmd`):

Copy `extensions/tasker/` to one of:
- `<workspace>/.openclaw/extensions/tasker/`
- `~/.openclaw/extensions/tasker/`

Or install via the CLI (copy or link):

```bash
openclaw plugins install ./extensions/tasker
openclaw plugins install -l ./extensions/tasker
```

3) Configure the OpenClaw plugin:

```json
{
  "plugins": {
    "entries": {
      "tasker": {
        "enabled": true,
        "config": {
          "binary": "tasker",
          "rootPath": "~/.tasker",
          "timeoutMs": 15000,
          "allowWrite": true
        }
      }
    }
  }
}
```

4) Allowlist the tool (required):

```json
{
  "agents": {
    "list": [
      {
        "id": "main",
        "tools": { "allow": ["tasker_cmd"] }
      }
    ]
  }
}
```

5) Install the unified skill to:
- `<workspace>/skills/task/` (preferred)
- `~/.openclaw/skills/task/`

Skill:
- Unified skill: `skills/task/`
- Optional helper: `./scripts/install-skill.sh --dest ~/.openclaw/skills`

Full docs:
- `docs/OPENCLAW_INTEGRATION.md`
- `docs/AGENT_WORKFLOW.md`
- `docs/CLI_SPEC.md`
- `docs/STORAGE_SPEC.md`
- `docs/SECURITY.md`

It is designed to integrate with **OpenClaw** via:
- a plugin tool (`tasker_cmd`) that safely spawns the `tasker` CLI with `shell:false`
- a unified skill that maps natural language or `/task` to `tasker_cmd`

OpenClaw helpers:
- `TASKER_BIN=/path/to/tasker` (env fallback if binary is not on PATH)
- `./scripts/install-skill.sh --dest ~/.openclaw/skills`

Explicit-only mode (optional):
```json
{
  "agent": {
    "require_explicit": true
  }
}
```

Workflow setup (optional):
```bash
tasker workflow init
tasker workflow prompts init
tasker workflow schedule init --window 24h --heartbeat-every 2h
```
Workflow config lives at `management/tasker/workflow.md` in the OpenClaw workspace by default.

## Install

See Setup above for npm/Go install. Additional options:

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
tasker add --text "Draft proposal | outline scope | due 2026-01-23" --project Work
tasker capture "Quick note | due 2026-01-23"
```

4) Capture ideas (plain text):
```bash
tasker idea add "Draft onboarding flow"
tasker idea capture "Pricing experiment | explore enterprise tier | #pricing"
tasker idea capture "Prototype +Work @design"
cat notes.txt | tasker idea add --stdin
tasker idea note add "Draft onboarding flow" -- "share with design"
tasker idea promote "Pricing experiment" --project Work --column todo
```

5) List tasks:
```bash
tasker ls --project Work
tasker ls --project Work --json   # writes JSON to <root>/exports
```

6) Due today + overdue:
```bash
tasker tasks --project Work
```

7) Board view:
```bash
tasker board --project Work --ascii
```

8) Week view:
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
- `TASKER_ROOT=/path/to/store` (env) or `--root <path>` (flag)
- `TASKER_PROJECT=work` (env) or `agent.default_project` (config)
- `TASKER_VIEW=week` (env) or `agent.default_view=week` (config)
- `TASKER_WEEK_DAYS=7` (env) or `agent.week_days=7` (config)
- `TASKER_OPEN_ONLY=true` (env) or `agent.open_only=true` (config)
- `TASKER_GROUP=project` + `TASKER_TOTALS=true` (env) for grouped summaries

Output options:
- Human‑readable summaries by default
- `--plain` for stable tab‑separated output
- `--json` / `--ndjson` write to `<root>/exports` (stdout JSON disabled by default)
- `--ascii` for board rendering
- `--format telegram` for lean chat output (plain text)

Agent helpers:
- `tasker resolve "<selector>"` returns JSON to stdout with matching IDs

## FAQ

**Why use tasker instead of a plain Markdown list?**  
Tasker keeps tasks as Markdown but adds structured metadata (due/status/tags), deterministic views (today/week/board), and agent‑safe IDs without cluttering human output.

## Codex / agent usage

- Natural language: “tasks today for Work” → `tasker tasks --project Work`
- Natural language: “what’s our week looking like?” → `tasker week --project Work`
- Natural language: “capture Draft proposal | due 2026-01-23” → `tasker capture "Draft proposal | due 2026-01-23"`
- Onboarding: `tasker onboarding`

JSON/NDJSON exports write to `<root>/exports` and are not printed to stdout unless `--stdout-json` or `--stdout-ndjson` is used.

## Storage model (no DB)

Everything lives under a root folder (default: `~/.tasker`):

- `.tasker/config.json` (store config)
- `.tasker/projects/<project-slug>/project.json`
- `.tasker/projects/<project-slug>/columns/<column-dir>/*.md` (tasks)

See `docs/STORAGE_SPEC.md`.

## OpenClaw integration

See `docs/OPENCLAW_INTEGRATION.md`.

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
