#!/usr/bin/env sh
set -eu

profile="nl"
dest="${HOME}/.clawdbot/skills"

usage() {
  echo "Usage: ./scripts/install-skill.sh [--profile nl|slash] [--dest <skills-dir>]"
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
  nl|natural|default)
    src="skills/task"
    ;;
  slash|slash-only)
    src="skills/task-slash"
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

echo "Installed '$profile' profile to $dest/task"
