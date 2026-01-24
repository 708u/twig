#!/usr/bin/env python3
"""
Pre-commit check hook for Claude Code.
Tracks test/lint/fmt execution and requires them before git commit.
"""

import json
import os
import re
import sys

# State file to track test execution (session-scoped)
def get_state_file(session_id):
    return os.path.expanduser(f"~/.claude/pre_commit_state_{session_id}.json")

def load_state(session_id):
    """Load state from file."""
    state_file = get_state_file(session_id)
    if os.path.exists(state_file):
        try:
            with open(state_file, "r") as f:
                return json.load(f)
        except (json.JSONDecodeError, IOError):
            pass
    return {"test": False, "lint": False, "fmt": False}

def save_state(session_id, state):
    """Save state to file."""
    state_file = get_state_file(session_id)
    try:
        os.makedirs(os.path.dirname(state_file), exist_ok=True)
        with open(state_file, "w") as f:
            json.dump(state, f)
    except IOError:
        pass

def detect_check_types(command):
    """Detect which check types the command contains. Returns list of check types."""
    checks = []
    if re.search(r"go\s+test", command):
        checks.append("test")
    if re.search(r"make\s+lint", command) or re.search(r"golangci-lint\s+run", command):
        checks.append("lint")
    if re.search(r"make\s+fmt", command) or re.search(r"golangci-lint\s+fmt", command):
        checks.append("fmt")
    return checks

def is_commit_command(command):
    """Check if command is a git commit command."""
    return "git commit" in command or "git commit" in command.replace("  ", " ")

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

    # Only handle Bash commands
    if tool_name != "Bash":
        sys.exit(0)

    command = tool_input.get("command", "")
    if not command:
        sys.exit(0)

    state = load_state(session_id)

    # If this is a check command, mark those checks as done
    check_types = detect_check_types(command)
    if check_types:
        for check_type in check_types:
            state[check_type] = True
        save_state(session_id, state)
        sys.exit(0)

    # If this is a commit command, check if all checks were run
    if is_commit_command(command):
        missing = []
        if not state.get("test", False):
            missing.append("go test ./...")
        if not state.get("lint", False):
            missing.append("make lint")
        if not state.get("fmt", False):
            missing.append("make fmt")

        if missing:
            print("Warning: Required checks have not been run in this session.", file=sys.stderr)
            print("Please run the following before committing:", file=sys.stderr)
            for cmd in missing:
                print(f"  - {cmd}", file=sys.stderr)
            sys.exit(2)
        # All checks passed, allow commit
        sys.exit(0)

    sys.exit(0)

if __name__ == "__main__":
    main()
