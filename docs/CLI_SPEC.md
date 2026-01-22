# CLI Spec — tasker docstore v0.1 (MVP)

Binary: `tasker`

## Global flags

- `--root <path>`: store root (default: `~/.tasker` or `TASKER_ROOT`)
- `--format <human|telegram>`: output format for summary/board commands
- `--json`: write JSON to `<root>/exports` (no stdout JSON)
- `--ndjson`: write NDJSON to `<root>/exports` (no stdout NDJSON)
- `--stdout-json`: allow JSON to stdout (debug only)
- `--stdout-ndjson`: allow NDJSON to stdout (debug only)
- `--export-dir <path>`: override export directory
- `--plain`: TSV output
- `--ascii`: ASCII rendering for board output
- `--quiet`, `--verbose`

### Environment defaults (optional)
- `TASKER_PROJECT`: default project if `--project` is omitted
- `TASKER_VIEW`: `today` or `week` (default view for `tasker tasks`)
- `TASKER_WEEK_DAYS`: integer days for week view
- `TASKER_OPEN_ONLY`: `true`/`false` (open‑only by default)
- `TASKER_GROUP`: `project` or `column`
- `TASKER_TOTALS`: `true`/`false` for per‑group counts

Flags may appear **before or after** the subcommand in v0.1.

## Commands (v0.1)

### `tasker init [--project <name>]`
Create root config and a default project (defaults to `Personal`).

### `tasker onboarding`
Print quickstart instructions and common commands.

### `tasker config show`
Print current config (defaults shown if config file is missing). Supports `--plain` and `--json` export.

### `tasker config set <key> <value>`
Update config keys (agent defaults).

Allowed keys:
- `agent.require_explicit` (true/false)
- `agent.default_project` (string, or `none`)
- `agent.default_view` (`today`|`week`|`none`)
- `agent.week_days` (integer)
- `agent.open_only` (true/false)
- `agent.summary_group` (`project`|`column`|`none`)
- `agent.summary_totals` (true/false)

### `tasker project add "<name>"`
Create a project (slugified).

### `tasker project ls`
List projects.

### `tasker add "<title>" --project <name> [--column <col>] [--due <date>] [--today|--tomorrow|--next-week] [--priority <p>] [--tag <t>...] [--desc <text>|--details <text>]`
Create a task. If `--project` is omitted, it uses `TASKER_PROJECT` / `agent.default_project` when set (otherwise `Personal`).
`--details` is an alias for `--desc`. When `--format telegram` is set, `add` prints a lean confirmation line suitable for chat.

### `tasker add --text "<title | details | due 2026-01-23 | #tag>" --project <name> [--column <col>] [--priority <p>] [--tag <t>...]`
Create a task from a single text string. Split parts with ` | ` (space‑pipe‑space). Explicit flags override parsed parts.

### `tasker capture "<title | details | due 2026-01-23 | #tag>" [--project <name>] [--column <col>] [--priority <p>] [--tag <t>...]`
Quick add using the same `--text` parsing. Defaults to inbox and your default project.

Columns: `inbox|todo|doing|blocked|done|archive`

### `tasker ls [--project <name>] [--column <col>] [--status <s>] [--tag <t>] [--search <q>] [--all]`
List tasks (defaults to non-archived).

### `tasker show <selector>`
Show a task file (frontmatter + notes). Selector can be an ID/prefix or an exact title. Title matching ignores archived tasks. Use `--project/--column/--status` to scope matches.

### `tasker resolve <selector>`
Return JSON to stdout with all matching tasks (IDs included for agents). Supports `--project/--column/--status` and `--all` to include archived.

### `tasker mv <selector> <column>`
Move task to another column (atomic rename).

### `tasker done <selector>`
Shortcut for `mv <selector> done`.

### `tasker note add <selector> "<text>"`
Append a note entry.
If multiple tasks share a title, the CLI returns a conflict and lists matching tasks (by project/column) so you can refine the title or set a default project.

Selector flags (show/mv/done/note/resolve):
- `--project <name>` to scope matching (use `none`/`all` to disable the default project)
- `--column <col>` to scope matching by column
- `--status <s>` to scope matching by status
- `--all` to include archived

### `tasker board --project <name> [--open|--all]`
Print project kanban board. `--open` hides done/archived; `--all` includes them. With `--format telegram`, done/archived are omitted unless `--all` is set.

### `tasker today [--project <name>]`
List due today + overdue tasks.

### `tasker tasks [--project <name>]`
Alias for `today` (due today + overdue).

### `tasker summary [--project <name>]`
Alias for `today` (due today + overdue).

`today`/`tasks` accept an optional trailing `today`/`now` token (e.g., `tasker tasks today --project Work`).

### `tasker week [--project <name>] [--days N]`
Show upcoming tasks for the next N days (default 7), plus overdue.

### `tasker agenda [--project <name>] [--days N]`
Alias for `week`.

### `tasker upcoming [--project <name>] [--days N]`
Alias for `week`.

`tasks` also accepts `week`/`this-week`/`agenda` tokens (e.g., `tasker tasks week --project Work`).

### Flags for today/week/tasks
- `--open`: only open/doing/blocked tasks
- `--all`: include done/archived (overrides `--open`)
- `--group project|column|none`: group output for human summaries
- `--totals`: show per-group counts when grouping
- Use `--format telegram` for lean chat-friendly output (plain text; defaults to open-only unless `--all` is set).

## Exit codes

- 0 success
- 2 usage/validation error
- 3 not found
- 4 conflict (ambiguous prefix)
- 10 internal error
