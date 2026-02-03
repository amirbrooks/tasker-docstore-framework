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

### `tasker workflow init [--workspace <path>] [--file <name>] [--runs-dir <path>] [--templates-dir <path>] [--run-name <pattern>] [--heartbeat <cmd>] [--no-heartbeat] [--force]`
Initialize workflow artifacts in an OpenClaw workspace and write a "Tasker Workflow" section
to `management/tasker/workflow.md` (or the file passed via `--file`). The command writes templates
into the workspace and records their paths in the workflow section.
The `--file` path must resolve inside the workspace.

Defaults:
- Workspace: `~/.openclaw/workspace` (or `OPENCLAW_WORKSPACE`)
- Runs dir: `management/RUNS`
- Templates dir: `management/templates`
- Run name: `YYYY-MM-DD-<short-name>`
- Heartbeat commands:
  - `tasker tasks --format telegram`
  - `tasker week --days 7 --format telegram`

### `tasker workflow prompts init [--workspace <path>] [--file <name>] [--prompts-dir <path>] [--night-shift <path>] [--proactive <path>] [--force]`
Write prompt files into the workspace and add a "Tasker Prompts" section to
`management/tasker/workflow.md` (or the file passed via `--file`).
Prompt paths must resolve inside the workspace.

Defaults:
- Prompts dir: `management`
- Night Shift: `management/NIGHT_SHIFT.md`
- Proactive Operator: `management/PROACTIVE_OPERATOR.md`

### `tasker workflow schedule init [--workspace <path>] [--file <name>] [--window <dur>] [--heartbeat-every <dur>] [--heartbeat-prompt <path>] [--night-shift <path>] [--nightly-cron <expr>] [--tz <tz>] [--no-heartbeat-prompt] [--no-nightly] [--heartbeat <cmd>] [--read <item>] [--force]`
Write a "Tasker Schedule" section and an optional heartbeat prompt into the workspace.
Prints example `openclaw cron add` commands, but does not execute them.
Schedule paths must resolve inside the workspace.

Defaults:
- Window: `24h`
- Heartbeat every: `2h`
- Heartbeat prompt: `management/HEARTBEAT.md`
- Night Shift prompt: `management/NIGHT_SHIFT.md`
- Nightly cron: `0 23 * * *`

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

### `tasker idea add "<title>" [--project <name>] [--body <text>] [--tag <t>...] [--stdin]`
Create a plain-text idea. If `--project` is omitted, the idea is stored at the root.

### `tasker idea add --text "<title | details | #tag>" [--project <name>] [--stdin]`
Create an idea from a single text string. Split parts with ` | ` (space‑pipe‑space).

### `tasker idea capture "<title | details | #tag>" [--project <name>] [--stdin]`
Quick add using the same pipe parsing.
If `--stdin` is set (or the lone `-` token is used), the idea is read from stdin.

### `tasker idea ls [--scope root|project|all] [--project <name>] [--tag <t>] [--search <q>]`
List ideas. Defaults to root ideas unless `--project` is provided. Use `--scope all` for root + all projects.

### `tasker idea show [--scope root|project|all] [--project <name>] [--match <m>] <selector>`
Show an idea (title + body). Uses the same selector matching rules as tasks.

### `tasker idea resolve [--scope root|project|all] [--project <name>] [--match <m>] <selector>`
Return JSON to stdout with matching ideas (IDs included for agents).

### `tasker idea note add [--scope root|project|all] [--project <name>] [--match <m>] <selector> -- <text...>`
Append a note line to an idea (timestamped).

### `tasker idea append [--scope root|project|all] [--project <name>] [--match <m>] <selector> -- <text...>`
Alias for `idea note add`.

### `tasker idea promote [--scope root|project|all] [--project <name>] [--to-project <name>] [--column <col>] [--due <date>] [--priority <p>] [--tag <t>...] [--link] [--delete] <selector>`
Create a task from an idea. Defaults to the idea's project if set, otherwise the default project. Use `--delete` to remove the idea after promotion.
Use `--link` to append a backlink to the idea in the task notes.

### Idea text shorthand
When using `idea add`/`idea capture` text input, inline tokens are parsed:
- `+Project` in the title line sets the project (if `--project` is omitted)
- `@context` and `#tag` are converted to tags
Inline `#tag` and `@context` tokens in the body are reflected in the `tags:` line.
Markdown headings like `# Title` are treated as headings (not tags).
Fenced code blocks (``` or ~~~) are ignored for tag extraction.

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
Show a task file (frontmatter + notes). Selector can be an ID/prefix or an exact title. Title matching ignores archived tasks. Use `--project/--column/--status` to scope matches, and `--match` for partial queries (default is smart fallback).

### `tasker resolve <selector>`
Return JSON to stdout with all matching tasks (IDs included for agents). Supports `--project/--column/--status`, `--all` to include archived, and `--match` for partial queries (search includes notes/body; default is smart fallback).

### `tasker mv <selector> <column>`
Move task to another column (atomic rename).

### `tasker done <selector>`
Shortcut for `mv <selector> done`.

### `tasker note add <selector...> -- <text...>`
Append a note entry.
If multiple tasks share a title, the CLI returns a conflict and lists matching tasks (by project/column) so you can refine the title or set a default project.
Tip: use `--` to separate selector text from the note; without it, tasker will try to infer the split.

Selector flags (show/mv/done/note/resolve):
- `--project <name>` to scope matching (use `none`/`all` to disable the default project)
- `--column <col>` to scope matching by column
- `--status <s>` to scope matching by status
- `--all` to include archived
- `--match auto|exact|prefix|contains|search` (`auto` tries exact → prefix → contains → search; `search` matches title + notes/body)

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
