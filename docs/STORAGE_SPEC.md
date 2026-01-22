# Storage Spec (No-DB) — tasker docstore v0.1

This storage format is designed for:
- human editability (Markdown)
- machine parsability (YAML frontmatter)
- low risk of corruption (atomic writes)
- Git friendliness (one task per file; clean diffs)

## Root layout

Default root: `~/.tasker` (configurable via `--root` or `TASKER_ROOT`)

```
<root>/
  config.json
  projects/
    <project-slug>/
      project.json
      columns/
        00-inbox/
        01-todo/
        02-doing/
        03-blocked/
        04-done/
        99-archive/
```

## Projects

A project is a folder under `projects/<project-slug>/` with metadata in `project.json`:

```json
{
  "schema": 1,
  "id": "prj_01J...",
  "name": "Work",
  "slug": "work",
  "created_at": "2026-01-21T10:00:00Z",
  "updated_at": "2026-01-21T10:00:00Z"
}
```

## Columns

Columns are directories under `columns/`. The directory name has an ordering prefix to produce stable listings:
- `00-inbox`, `01-todo`, `02-doing`, `03-blocked`, `04-done`, `99-archive`

The canonical column IDs are:
- `inbox`, `todo`, `doing`, `blocked`, `done`, `archive`

Columns are configured in `<root>/config.json` (generated on `tasker init`).

Optional agent config can be added to `<root>/config.json`:

```json
{
  "agent": {
    "require_explicit": false,
    "default_project": "work",
    "default_view": "today",
    "week_days": 7,
    "open_only": true,
    "summary_group": "project",
    "summary_totals": true
  }
}
```

## Tasks

Each task is a Markdown file, named:

`tsk_<ULID>__<slug-title>.md`

Example path:
`<root>/projects/work/columns/01-todo/tsk_01J4...__draft-proposal.md`

### Task file format

YAML frontmatter (schema 1), then markdown body.

```md
---
schema: 1
id: "tsk_01J4F3N8Q3FZ5G2KJZP6N6Y9QH"
title: "Draft proposal"
status: "open"            # open|doing|blocked|done|archived
project: "work"           # project slug
column: "todo"            # inbox|todo|doing|blocked|done|archive
priority: "high"          # low|normal|high|urgent
tags: ["client", "writing"]
due: "2026-01-23"         # YYYY-MM-DD or RFC3339
created_at: "2026-01-21T10:20:30Z"
updated_at: "2026-01-21T10:20:30Z"
completed_at: null
archived_at: null
---

## Notes

- Optional markdown content.
```

### Source of truth rules

- **File location determines column**. On load, if frontmatter `column` differs from path, the CLI may reconcile and prefer the path.
- `status` is derived from `column`:
  - inbox/todo => open
  - doing => doing
  - blocked => blocked
  - done => done
  - archive => archived

### Atomic writes

All task updates must be written using:
- write to temp file in the same directory
- `fsync` (optional; recommended on Linux/macOS)
- rename/replace original (atomic on same filesystem)

### Concurrency

For v0.1:
- per-task operations are file-scoped (low contention)
- avoid updating multiple tasks without a store-level lock
- index caches (if added later) must be protected with a lockfile

## Portability

- Plain text + JSON + Markdown
- Works with Git (recommended) for backup/history
