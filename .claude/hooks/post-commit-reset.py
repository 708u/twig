#!/usr/bin/env python3
"""
Post-commit reset hook for Claude Code.
Resets test execution state after successful git commit.
"""

import json
import os
import sys

def get_state_file(session_id):
    return os.path.expanduser(f"~/.claude/pre_commit_state_{session_id}.json")

def reset_state(session_id):
    """Reset state after successful commit."""
    state_file = get_state_file(session_id)
    state = {"tests_run": False, "last_test_time": None}
    try:
        os.makedirs(os.path.dirname(state_file), exist_ok=True)
        with open(state_file, "w") as f:
            json.dump(state, f)
    except IOError:
        pass

def main():
    # Read input from stdin
    try:
        raw_input = sys.stdin.read()
        input_data = json.loads(raw_input)
    except json.JSONDecodeError:
        sys.exit(0)

    session_id = input_data.get("session_id", "default")
    tool_name = input_data.get("tool_name", "")
    tool_input = input_data.get("tool_input", {})
    tool_result = input_data.get("tool_result", {})

    # Only handle Bash commands
    if tool_name != "Bash":
        sys.exit(0)

    command = tool_input.get("command", "")
    if not command:
        sys.exit(0)

    # Check if this was a successful git commit
    if "git commit" in command:
        # Check if the command succeeded (exit code 0 or stdout contains commit hash)
        stdout = tool_result.get("stdout", "")
        exit_code = tool_result.get("exit_code", None)

        # If commit was successful, reset state
        if exit_code == 0 or "[" in stdout:  # git commit output contains [branch hash]
            reset_state(session_id)

    sys.exit(0)

if __name__ == "__main__":
    main()
