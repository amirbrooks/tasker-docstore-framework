#!/usr/bin/env sh
set -eu

profile="unified"
dest="${HOME}/.openclaw/skills"

usage() {
  echo "Usage: ./scripts/install-skill.sh [--dest <skills-dir>] [--profile <name>]"
}

while [ $# -gt 0 ]; do
  case "$1" in
    --profile)
      profile="${2:-}"
      shift 2
      ;;
    --dest)
      dest="${2:-}"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown argument: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

case "$profile" in
  ""|unified|nl|natural|default)
    src="skills/task"
    ;;
  slash|slash-only)
    src="skills/task"
    echo "Note: slash-only profile is deprecated; installing unified skill instead." >&2
    ;;
  *)
    echo "Unknown profile: $profile" >&2
    usage >&2
    exit 2
    ;;
esac

if [ ! -d "$src" ]; then
  echo "Missing $src (run from repo root)." >&2
  exit 1
fi

mkdir -p "$dest"
if [ -e "$dest/task" ]; then
  rm -rf "$dest/task"
fi
cp -R "$src" "$dest/task"

echo "Installed unified skill to $dest/task"
