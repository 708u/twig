#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

DOCS_DIR="$REPO_ROOT/docs/reference"
REFERENCES_DIR="$REPO_ROOT/external/claude-code/plugins/twig-guide/skills/twig-guide/references"

# Clean target directory
rm -rf "$REFERENCES_DIR"
mkdir -p "$REFERENCES_DIR/commands"

# Copy all reference docs
cp "$DOCS_DIR/configuration.md" "$REFERENCES_DIR/"
cp "$DOCS_DIR/commands"/*.md "$REFERENCES_DIR/commands/"

echo "Plugin docs synced successfully"
