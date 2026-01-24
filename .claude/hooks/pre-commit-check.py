#!/usr/bin/env python3
"""
Pre-commit check hook for Claude Code.
Tracks test/lint/fmt execution and requires them before git commit.
"""

import json
import os
import re
import sys
from datetime import datetime

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
    return {"tests_run": False, "last_test_time": None}

def save_state(session_id, state):
    """Save state to file."""
    state_file = get_state_file(session_id)
    try:
        os.makedirs(os.path.dirname(state_file), exist_ok=True)
        with open(state_file, "w") as f:
            json.dump(state, f)
    except IOError:
        pass

def is_test_command(command):
    """Check if command is a test/lint/fmt command."""
    test_patterns = [
        r"go\s+test",
        r"make\s+lint",
        r"make\s+fmt",
        r"make\s+test",
        r"golangci-lint",
        r"go\s+fmt",
        r"gofmt",
    ]
    for pattern in test_patterns:
        if re.search(pattern, command):
            return True
    return False

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

    # If this is a test command, mark tests as run
    if is_test_command(command):
        state["tests_run"] = True
        state["last_test_time"] = datetime.now().isoformat()
        save_state(session_id, state)
        sys.exit(0)

    # If this is a commit command, check if tests were run
    if is_commit_command(command):
        if not state.get("tests_run", False):
            print("Warning: Tests have not been run in this session.", file=sys.stderr)
            print("Please run the following before committing:", file=sys.stderr)
            print("  - go test ./...", file=sys.stderr)
            print("  - make lint (or golangci-lint run)", file=sys.stderr)
            print("  - make fmt (or go fmt ./...)", file=sys.stderr)
            sys.exit(2)
        # Tests were run, allow commit
        sys.exit(0)

    sys.exit(0)

if __name__ == "__main__":
    main()
