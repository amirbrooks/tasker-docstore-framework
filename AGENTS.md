# Repository Guidelines

## Project Structure & Module Organization
- `cmd/tasker/main.go` is the CLI entrypoint.
- `internal/cli/` handles command parsing and output formatting.
- `internal/store/` implements docstore storage logic and file operations.
- `docs/` contains specs (CLI, storage, security) that define expected behavior.
- `extensions/tasker/` is the Clawdbot plugin (TypeScript); `skills/task/` is the companion skill; `skills/tasker-codex/` is the Codex natural-language skill.
- `examples/` shows a sample store layout and task files.

## Build, Test, and Development Commands
- `go build -o tasker ./cmd/tasker` builds the CLI binary.
- `./tasker init` creates a root store (default `~/.tasker` or `TASKER_ROOT`).
- `./tasker ls --project Work` or `./tasker board --project Work --ascii` are quick smoke checks.
- `./tasker tasks --project Work` shows due today + overdue tasks.
- `go test ./...` runs Go package checks (no test files currently, but compiles packages).

## Coding Style & Naming Conventions
- Go code should be formatted with `gofmt` and organized under `internal/`.
- TypeScript in `extensions/` uses 2-space indentation and `spawn` with argv (no shell).
- Task files are named `tsk_<ULID>__<slug-title>.md`.
- Project slugs are kebab-case under `projects/<project-slug>/`; columns use numeric prefixes like `01-todo`.

## Testing Guidelines
- No dedicated test suite yet; add Go tests as `*_test.go` alongside the package under `internal/` or `cmd/`.
- Name tests by behavior (e.g., `TestStoreAddTask`, `TestCLIProjectList`).

## Commit & Pull Request Guidelines
- No Git history is present in this repo; use concise, imperative commit subjects (e.g., "Add task slug validator").
- PRs should include a summary, rationale, and example CLI output when behavior changes, plus tests run.

## Security & Configuration Notes
- Avoid shell execution and slugify user input (see `docs/SECURITY.md`).
- Use `--root` or `TASKER_ROOT` to control storage location; do not assume `~/.tasker` in tests.
- `--json` and `--ndjson` write to `<root>/exports` (no stdout JSON by default).
