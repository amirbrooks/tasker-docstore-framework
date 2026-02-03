# OpenClaw Integration (Lean)

Prereqs:
- OpenClaw installed and configured. The onboarding wizard `openclaw onboard` sets up your workspace + skills.
- You can rerun config via `openclaw configure` (same as `openclaw config`).
- Config lives at `~/.openclaw/openclaw.json` by default (override with `OPENCLAW_CONFIG_PATH`).
- Default workspace is `~/.openclaw/workspace`.

Goal: expose tasker via a plugin tool + unified skill that supports **natural language** and `/task ...` slash commands. For explicit-only behavior, set `agent.require_explicit: true` in the tasker config.

Approach:
1) Install the tasker CLI (npm/go/build)
2) Install the plugin tool (`extensions/tasker`) — registers optional tool `tasker_cmd`
3) Allowlist `tasker_cmd` for your agent (optional tools are opt-in)
4) Install the skill (`skills/task`) — configures `/task` and NL routing to dispatch to `tasker_cmd`

## Why plugin tool?
OpenClaw `exec` runs shell commands. Forwarding user args into a shell is hard to secure. The plugin tool spawns `tasker` with `shell:false` and an argv array.

## Install the tasker CLI

Pick one:

```bash
npm install -g @amirbrooks/tasker-docstore
```

```bash
go install github.com/amirbrooks/tasker-docstore-framework/cmd/tasker@latest
```

Or build locally:

```bash
go build -o tasker ./cmd/tasker
```

If the binary is not on PATH, set `TASKER_BIN` or configure `binary` in the plugin config below.

## Plugin install
OpenClaw discovers plugins by scanning:
- `<workspace>/.openclaw/extensions/*.ts`
- `<workspace>/.openclaw/extensions/*/index.ts`
- `~/.openclaw/extensions/*.ts`
- `~/.openclaw/extensions/*/index.ts`

Each plugin must include `openclaw.plugin.json`.

Copy `extensions/tasker/` to one of:
- `<workspace>/.openclaw/extensions/tasker/`
- `~/.openclaw/extensions/tasker/`

Or install via the CLI (copy or link):

```bash
openclaw plugins install ./extensions/tasker
openclaw plugins install -l ./extensions/tasker
```

Enable in `~/.openclaw/openclaw.json`:

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

If `tasker` is not on PATH, set `binary` to an absolute path (or `tasker.exe` on Windows):

```json
{
  "plugins": {
    "entries": {
      "tasker": {
        "enabled": true,
        "config": {
          "binary": "/usr/local/bin/tasker",
          "rootPath": "~/.tasker",
          "timeoutMs": 15000,
          "allowWrite": true
        }
      }
    }
  }
}
```

You can also set `TASKER_BIN` as a fallback. The tool returns a clear error if the binary is missing.

Note: `rootPath` maps to the CLI `--root` flag. The default store root is `~/.tasker`. See `docs/STORAGE_SPEC.md` for the directory layout.

## Tool allowlist
Because `tasker_cmd` is optional, allowlist it:

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

## Skill install
Copy `skills/task/` to your skills folder as `task/`.

Install to one of:
- `<workspace>/skills/task/` (preferred)
- `~/.openclaw/skills/task/`

OpenClaw loads skills from `<workspace>/skills` and `~/.openclaw/skills` (workspace takes precedence).
Default workspace is `~/.openclaw/workspace`, so `<workspace>/skills` is `~/.openclaw/workspace/skills`.

Optional installer script:
```bash
./scripts/install-skill.sh --dest ~/.openclaw/skills
```

Restart the relevant processes/sessions after enabling plugins/skills.

Explicit-only mode (optional): set in `<root>/config.json`:
```json
{
  "agent": {
    "require_explicit": true
  }
}
```

Workflow artifacts (spec/tasks/handoff) and heartbeat config: see `docs/AGENT_WORKFLOW.md`.
You can scaffold workspace config + templates with:
```bash
tasker workflow init
```
By default this writes `management/tasker/workflow.md` inside the OpenClaw workspace.
Prompt and schedule helpers:
```bash
tasker workflow prompts init
tasker workflow schedule init --window 24h --heartbeat-every 2h
```

## Usage
Natural language (preferred):
- "tasks today for Work" -> `tasker_cmd` with `tasks --project Work --open --format telegram`
- "what's our week looking like?" -> `week --days 7 --format telegram`
- "add Draft proposal today" -> `add "Draft proposal" --today --format telegram`
- "add Draft proposal | outline scope | due 2026-01-23" -> `add --text "Draft proposal | outline scope | due 2026-01-23" --format telegram`
- "capture Draft proposal | due 2026-01-23" -> `capture "Draft proposal | due 2026-01-23" --format telegram`
- "mark done follow up" -> `resolve "follow up"` then `done "<id>"` if exactly one match (IDs stay internal)
- "capture idea Pricing experiment | #pricing" -> `idea capture "Pricing experiment | #pricing" --format telegram`
- "list ideas for Work" -> `idea ls --project Work --scope project --format telegram`
- "add note to idea Pricing experiment" -> `idea note add "Pricing experiment" -- "follow up" --format telegram`
- "promote idea Pricing experiment" -> `idea promote "Pricing experiment" --project Work --column todo --link --format telegram`

Inline shorthand for ideas:
- `+Project` in the title line sets the idea project if `--project` is omitted
- `@context` and `#tag` are converted to tags
- For long inputs, pipe content to stdin: `cat notes.txt | tasker idea add --stdin`

Slash command (explicit):
- `/task ls --project Work`
- `/task add "Draft proposal" --project Work --column todo`
- `/task add --text "Draft proposal | due 2026-01-23" --project Work`
- `/task done "Draft proposal"`
- `/task tasks --project Work` (due today + overdue)
- `/task week --project Work --days 7` (upcoming + overdue)
- `/task tasks today --open --group project --totals`
- `/task config show`
- `/task resolve "Draft proposal"`
- `/task idea capture "Pricing experiment | #pricing"`
- `/task idea ls --scope all`
- `/task idea note add "Pricing experiment" -- "follow up"`
- `/task idea promote "Pricing experiment" --project Work --column todo --link`
