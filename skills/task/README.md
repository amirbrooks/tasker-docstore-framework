# /task skill (tasker docstore)

This skill is designed to be low-bloat:
- `disable-model-invocation: true` keeps it out of model prompt context
- `command-dispatch: tool` calls the tool directly for deterministic behavior

It requires:
- `tasker` binary on PATH
- plugin tool `tasker_cmd` allowlisted (recommended)

See `docs/CLAWDBOT_INTEGRATION.md` at repo root for end-to-end setup.
