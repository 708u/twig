#!/usr/bin/env python3
"""
Pre-commit check hook for Claude Code.
Tracks test/lint/fmt execution and warns if not run before git commit.
"""

import json
import os
import random
import re
import sys
from datetime import datetime

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
                return {
                    "checks": set(data.get("checks", [])),
                    "warnings": set(data.get("warnings", [])),
                }
        except (json.JSONDecodeError, IOError):
            pass
    return {"checks": set(), "warnings": set()}


def save_state(session_id, state):
    """Save state to file."""
    state_file = get_state_file(session_id)
    try:
        os.makedirs(STATE_DIR, exist_ok=True)
        with open(state_file, "w") as f:
            json.dump(
                {
                    "checks": list(state["checks"]),
                    "warnings": list(state["warnings"]),
                },
                f,
            )
    except IOError:
        pass


def detect_check_types(command):
    """Detect which check types the command contains."""
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
    return "git commit" in command


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

    state = load_state(session_id)

    # If this is a check command, mark those checks as done
    check_types = detect_check_types(command)
    if check_types:
        for check_type in check_types:
            state["checks"].add(check_type)
        save_state(session_id, state)
        sys.exit(0)

    # If this is a commit command, check if all checks were run
    if is_commit_command(command):
        required_checks = {"test", "lint", "fmt"}
        missing = required_checks - state["checks"]

        if missing:
            # Create warning key for this specific set of missing checks
            warning_key = "commit_missing_" + "_".join(sorted(missing))

            # Already warned for this, allow commit
            if warning_key in state["warnings"]:
                sys.exit(0)

            # First time: warn and block
            state["warnings"].add(warning_key)
            save_state(session_id, state)

            missing_commands = {
                "test": "go test ./...",
                "lint": "make lint",
                "fmt": "make fmt",
            }
            print("Warning: The following checks have not been run:", file=sys.stderr)
            for check in sorted(missing):
                print(f"  - {missing_commands[check]}", file=sys.stderr)
            print(
                "Run them if needed, or commit again to proceed anyway.",
                file=sys.stderr,
            )
            sys.exit(2)

    sys.exit(0)


if __name__ == "__main__":
    main()
