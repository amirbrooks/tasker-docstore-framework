# Clawdbot Integration (Lean)

Goal: expose tasker via a plugin tool + skill that supports **natural language** and `/task ...` slash commands. For low-bloat slash-only mode, set `disable-model-invocation: true` in the skill.

Approach:
1) Install the plugin tool (`extensions/tasker`) — registers optional tool `tasker_cmd`
2) Allowlist `tasker_cmd` for your agent (optional tools are opt-in)
3) Install the skill (`skills/task`) — configures `/task` and NL routing to dispatch to `tasker_cmd`

## Why plugin tool?
Clawdbot `exec` runs shell commands. Forwarding user args into a shell is hard to secure. The plugin tool spawns `tasker` with `shell:false` and an argv array.

## Plugin install
Copy `extensions/tasker/` to one of:
- `<workspace>/.clawdbot/extensions/tasker/`
- `~/.clawdbot/extensions/tasker/`

Enable in `~/.clawdbot/clawdbot.json`:

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

## Skill profiles and install
Choose one profile, then copy it to your skills folder as `task/`:
- Natural language (recommended): `skills/task/` (disable-model-invocation: false)
- Slash-only (low-bloat): `skills/task-slash/` (disable-model-invocation: true)

Install to one of:
- `<workspace>/skills/task/` (preferred)
- `~/.clawdbot/skills/task/`

Optional installer script:
```bash
./scripts/install-skill.sh --profile nl
./scripts/install-skill.sh --profile slash --dest ~/.clawdbot/skills
```

Restart the relevant processes/sessions after enabling plugins/skills.

Install only one profile at a time. If you switch, remove the other and restart the session.

## Usage
Natural language (preferred):
- "tasks today for Work" -> `tasker_cmd` with `tasks --project Work --open`
- "what's our week looking like?" -> `week --days 7`
- "add Draft proposal today" -> `add "Draft proposal" --today`

Slash command (explicit):
- `/task ls --project Work`
- `/task add "Draft proposal" --project Work --column todo`
- `/task done "Draft proposal"`
- `/task tasks --project Work` (due today + overdue)
- `/task week --project Work --days 7` (upcoming + overdue)
- `/task tasks today --open --group project --totals`
- `/task config show`
