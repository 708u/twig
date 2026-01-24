#!/usr/bin/env python3
"""
Post-commit reset hook for Claude Code.
Resets check execution state after successful git commit.
"""

import json
import os
import sys

# State file prefix (must match pre-commit-check.py)
STATE_FILE_PREFIX = "pre_commit_state_"


def get_state_file(session_id):
    """Get session-specific state file path."""
    return os.path.expanduser(f"~/.claude/{STATE_FILE_PREFIX}{session_id}.json")


def reset_state(session_id):
    """Reset state after successful commit."""
    state_file = get_state_file(session_id)
    state = {"checks": [], "warnings": []}
    try:
        os.makedirs(os.path.dirname(state_file), exist_ok=True)
        with open(state_file, "w") as f:
            json.dump(state, f)
    except IOError:
        pass


def main():
    """Main hook function."""
    # Read input from stdin
    try:
        raw_input = sys.stdin.read()
        input_data = json.loads(raw_input)
    except json.JSONDecodeError:
        sys.exit(0)

    session_id = input_data.get("session_id", "default")
    tool_name = input_data.get("tool_name", "")
    tool_input = input_data.get("tool_input", {})
    tool_response = input_data.get("tool_response", {})

    # Only handle Bash commands
    if tool_name != "Bash":
        sys.exit(0)

    command = tool_input.get("command", "")
    if not command:
        sys.exit(0)

    # Check if this was a successful git commit
    if "git commit" in command:
        stdout = tool_response.get("stdout", "")
        # git commit output contains [branch hash] on success
        if "[" in stdout:
            reset_state(session_id)

    sys.exit(0)


if __name__ == "__main__":
    main()
