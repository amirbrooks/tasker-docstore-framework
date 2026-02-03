# Security Notes (Docstore CLI + OpenClaw)

## CLI
- Treat all user inputs as untrusted.
- Never call a shell with interpolated user input.
- Prefer direct filesystem operations (create/move files) using safe path handling.
- Slugify user-provided names for filesystem paths.

## OpenClaw
- Prefer a plugin tool that spawns the CLI with `shell:false` (argv array).
- Register the tool as optional so users must allowlist it.
- Consider a read-only mode (`allowWrite=false`) for conservative deployments.
