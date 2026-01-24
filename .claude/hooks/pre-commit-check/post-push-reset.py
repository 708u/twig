#!/usr/bin/env python3
"""
Post-push reset hook for Claude Code.
Resets check execution state after successful git push.
"""

import json
import os
import sys

# State directory (relative to this script, must match pre-commit-check.py)
STATE_DIR = os.path.join(os.path.dirname(os.path.abspath(__file__)), "state")


def get_state_file(session_id):
    """Get session-specific state file path."""
    return os.path.join(STATE_DIR, f"{session_id}.json")


def reset_state(session_id):
    """Reset state after successful push."""
    state_file = get_state_file(session_id)
    state = {"checks": [], "warnings": []}
    try:
        os.makedirs(STATE_DIR, exist_ok=True)
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

    # Reset state after successful git push
    if "git push" in command:
        stderr = tool_response.get("stderr", "")
        if not stderr:
            reset_state(session_id)

    sys.exit(0)


if __name__ == "__main__":
    main()
