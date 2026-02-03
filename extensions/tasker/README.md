Setup (quick):

OpenClaw discovers plugins from `<workspace>/.openclaw/extensions` and `~/.openclaw/extensions`
by scanning `*.ts` and `*/index.ts`.
This plugin ships `openclaw.plugin.json`, which OpenClaw requires.

1) Copy this folder to one of:
   - `<workspace>/.openclaw/extensions/tasker/`
   - `~/.openclaw/extensions/tasker/`

   Or install via the CLI (copy or link):

   ```bash
   openclaw plugins install ./extensions/tasker
   openclaw plugins install -l ./extensions/tasker
   ```

2) Enable in `~/.openclaw/openclaw.json`:

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

Full setup: `docs/OPENCLAW_INTEGRATION.md`
