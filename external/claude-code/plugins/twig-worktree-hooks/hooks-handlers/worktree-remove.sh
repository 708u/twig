#!/usr/bin/env bash
set -euo pipefail

input="$(cat)"

worktree_path="$(echo "$input" | sed -n 's/.*"worktree_path"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')"
cwd="$(echo "$input" | sed -n 's/.*"cwd"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')"

if [ -z "$worktree_path" ]; then
  echo "error: WorktreeRemove: 'worktree_path' field is required" >&2
  exit 1
fi

if ! command -v twig &>/dev/null; then
  echo "error: WorktreeRemove: twig not found in PATH" >&2
  exit 1
fi

# Resolve branch name from the worktree path
branch="$(git -C "$worktree_path" rev-parse --abbrev-ref HEAD 2>/dev/null)" || true

if [ -z "$branch" ]; then
  # Worktree already deleted — prune stale records and exit
  if [ -n "$cwd" ]; then
    git -C "$cwd" worktree prune 2>/dev/null || true
  fi
  exit 0
fi

args=()
if [ -n "$cwd" ]; then
  args+=(-C "$cwd")
fi
# -f: bypass uncommitted changes/unmerged checks
# Agent worktrees are disposable, so this is safe
args+=(remove "$branch" --force)

exec twig "${args[@]}"
