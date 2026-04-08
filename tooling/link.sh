#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
KIRO_DIR="${HOME}/.kiro"

link_dir() {
  local src_dir="$1"
  local dst_dir="$2"
  [[ -d "$src_dir" ]] || return 0
  mkdir -p "$dst_dir"
  for item in "$src_dir"/*/; do
    [[ -d "$item" ]] || continue
    local name
    name="$(basename "$item")"
    local dst="$dst_dir/$name"
    if [[ -L "$dst" ]]; then
      echo "skip (already linked): $dst"
    elif [[ -e "$dst" ]]; then
      echo "skip (exists, not a symlink): $dst"
    else
      ln -s "$item" "$dst"
      echo "linked: $dst -> $item"
    fi
  done
}

link_dir "$SCRIPT_DIR/skills" "$KIRO_DIR/skills"
link_dir "$SCRIPT_DIR/agents" "$KIRO_DIR/agents"
