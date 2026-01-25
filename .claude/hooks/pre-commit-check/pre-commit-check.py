#!/usr/bin/env python3
"""
Pre-commit check hook for Claude Code.
Warns before git commit and allows on second attempt.
"""

import json
import os
import random
import sys
from datetime import datetime

# Checks by category
# Each category has a description of when it's needed and the commands to run
CHECKS = {
    "code": {
        "when": "When Go code (*.go, go.mod, go.sum) is modified",
        "commands": [
            "make test",
            "make lint",
            "make fmt",
            "go mod tidy",
        ],
    },
    "cli": {
        "when": "When CLI behavior is modified (cmd/twig/**, *.go)",
        "commands": [
            "Review docs/reference/ for accuracy",
            "Update docs if command options/behavior changed",
        ],
    },
    "docs": {
        "when": "When docs (docs/reference/**) is modified",
        "commands": [
            "make sync-plugin-docs",
            "Bump version in external/claude-code/plugins/twig/.claude-plugin/plugin.json",
        ],
    },
}

# Checks required by file extension
CHECKS_BY_EXT = {
    ".go": [
        "go test ./...",
        "make lint",
        "make fmt",
    ],
}

# State directory (relative to this script)
STATE_DIR = os.path.join(os.path.dirname(os.path.abspath(__file__)), "state")


def get_state_file(session_id):
    """Get session-specific state file path."""
    return os.path.join(STATE_DIR, f"{session_id}.json")


def cleanup_old_state_files():
    """Remove state files older than 30 days."""
    try:
        if not os.path.exists(STATE_DIR):
            return

        current_time = datetime.now().timestamp()
        thirty_days_ago = current_time - (30 * 24 * 60 * 60)

        for filename in os.listdir(STATE_DIR):
            if filename.endswith(".json"):
                file_path = os.path.join(STATE_DIR, filename)
                try:
                    file_mtime = os.path.getmtime(file_path)
                    if file_mtime < thirty_days_ago:
                        os.remove(file_path)
                except (OSError, IOError):
                    pass
    except Exception:
        pass


def load_state(session_id):
    """Load state from file."""
    state_file = get_state_file(session_id)
    if os.path.exists(state_file):
        try:
            with open(state_file, "r") as f:
                data = json.load(f)
                return {"warned": data.get("warned", False)}
        except (json.JSONDecodeError, IOError):
            pass
    return {"warned": False}


def save_state(session_id, state):
    """Save state to file."""
    state_file = get_state_file(session_id)
    try:
        os.makedirs(STATE_DIR, exist_ok=True)
        with open(state_file, "w") as f:
            json.dump(state, f)
    except IOError:
        pass


def main():
    """Main hook function."""
    # Periodically clean up old state files (10% chance per run)
    if random.random() < 0.1:
        cleanup_old_state_files()

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

    # Check for git commit command
    if "git commit" not in command:
        sys.exit(0)

    state = load_state(session_id)

    # Already warned this session, allow commit
    if state["warned"]:
        sys.exit(0)

    # First commit attempt: warn and block
    state["warned"] = True
    save_state(session_id, state)

    print("BLOCKED: Required checks not confirmed.", file=sys.stderr)
    print("", file=sys.stderr)
    print("Run before committing:", file=sys.stderr)
    for category, info in CHECKS.items():
        print(f"  [{category}] {info['when']}", file=sys.stderr)
        for cmd in info["commands"]:
            print(f"    - {cmd}", file=sys.stderr)
    print("", file=sys.stderr)
    print("After running checks, commit again to confirm.", file=sys.stderr)
    sys.exit(2)


if __name__ == "__main__":
    main()
