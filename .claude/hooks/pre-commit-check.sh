#!/bin/bash
set -euo pipefail

input=$(cat)
tool_name=$(echo "$input" | jq -r '.tool_name')
command=$(echo "$input" | jq -r '.tool_input.command // ""')

# Warn only for git commit commands via Bash tool
if [[ "$tool_name" == "Bash" ]] && [[ "$command" == *"git commit"* ]]; then
  echo "Warning: Have you run test/lint/fmt?" >&2
  echo "  - go test ./..." >&2
  echo "  - make lint (or golangci-lint run)" >&2
  echo "  - make fmt (or go fmt ./...)" >&2
  exit 2  # Block and display warning
fi

exit 0
