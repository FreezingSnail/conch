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

link_files() {
  local src_dir="$1"
  local dst_dir="$2"
  local pattern="$3"
  [[ -d "$src_dir" ]] || return 0
  mkdir -p "$dst_dir"
  for item in "$src_dir"/$pattern; do
    [[ -e "$item" ]] || continue
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

link_files "$SCRIPT_DIR/agents" "$KIRO_DIR/agents" "*.json"

copy_git_hooks() {
  local src_dir="$SCRIPT_DIR/git"
  [[ -d "$src_dir" ]] || return 0
  local git_dir
  git_dir="$(git -C "$SCRIPT_DIR" rev-parse --git-dir)"
  local hooks_dir="$git_dir/hooks"
  mkdir -p "$hooks_dir"
  for hook in "$src_dir"/*; do
    [[ -f "$hook" ]] || continue
    local name
    name="$(basename "$hook")"
    cp "$hook" "$hooks_dir/$name"
    chmod +x "$hooks_dir/$name"
    echo "installed git hook: $hooks_dir/$name"
  done
}

copy_git_hooks
