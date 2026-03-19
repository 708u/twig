#!/usr/bin/env bash
set -euo pipefail

input="$(cat)"

name="$(echo "$input" | sed -n 's/.*"name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')"
cwd="$(echo "$input" | sed -n 's/.*"cwd"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')"

if [ -z "$name" ]; then
  echo "error: WorktreeCreate: 'name' field is required" >&2
  exit 1
fi

if ! command -v twig &>/dev/null; then
  echo "error: WorktreeCreate: twig not found in PATH" >&2
  exit 1
fi

# twig add --quiet outputs only the absolute worktree path to stdout,
# which satisfies the WorktreeCreate hook contract.
# -C ensures .twig/settings.toml is loaded from the correct repo root.
args=()
if [ -n "$cwd" ]; then
  args+=(-C "$cwd")
fi
args+=(add "$name" --quiet)

exec twig "${args[@]}"
