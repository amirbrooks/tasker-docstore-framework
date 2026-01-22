# Clawdbot Integration (Lean)

Goal: expose `/task ...` as a low-bloat slash command that **bypasses the model** and directly runs the deterministic CLI.

Approach:
1) Install the plugin tool (`extensions/tasker`) — registers optional tool `tasker_cmd`
2) Allowlist `tasker_cmd` for your agent (optional tools are opt-in)
3) Install the skill (`skills/task`) — configures `/task` to dispatch to `tasker_cmd`

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
Copy `skills/task/` to:
- `<workspace>/skills/task/` (preferred)
- or `~/.clawdbot/skills/task/`

Restart the relevant processes/sessions after enabling plugins/skills.

## Usage
- `/task ls --project Work`
- `/task add "Draft proposal" --project Work --column todo`
- `/task done tsk_01J4F3N8`
- `/task tasks --project Work` (due today + overdue)
