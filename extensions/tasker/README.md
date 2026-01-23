Setup (quick):

1) Copy this folder to one of:
   - `<workspace>/.clawdbot/extensions/tasker/`
   - `~/.clawdbot/extensions/tasker/`

2) Enable in `~/.clawdbot/clawdbot.json`:

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

3) Allowlist the tool:

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

Full setup: `docs/CLAWDBOT_INTEGRATION.md`
