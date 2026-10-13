#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
skills_source="$repo_root/skills"

usage() {
  echo "Usage: $0 <claude|agents|/path/to/skills-dir> [skill-name ...]"
  exit 1
}

resolve_target() {
  case "$1" in
    claude) echo "$HOME/.claude/skills" ;;
    agents) echo "$HOME/.agents/skills" ;;
    /*)     echo "$1" ;;
    *)      usage ;;
  esac
}

link_skill() {
  local name="$1" target_dir="$2"
  local source="$skills_source/$name"
  local destination="$target_dir/$name"

  if [[ ! -f "$source/SKILL.md" ]]; then
    echo "skip   $name (no SKILL.md)"
    return
  fi

  if [[ -L "$destination" ]]; then
    ln -sfn "$source" "$destination"
    echo "linked $name"
    return
  fi

  if [[ -e "$destination" ]]; then
    echo "skip   $name (exists and is not a symlink)"
    return
  fi

  ln -s "$source" "$destination"
  echo "linked $name"
}

[[ $# -ge 1 ]] || usage

target_dir="$(resolve_target "$1")"
shift

mkdir -p "$target_dir"

if [[ $# -gt 0 ]]; then
  names=("$@")
else
  names=()
  for dir in "$skills_source"/*/; do
    names+=("$(basename "$dir")")
  done
fi

for name in "${names[@]}"; do
  link_skill "$name" "$target_dir"
done
