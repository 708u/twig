---
name: twig-guide
description: |
  This skill should be used when the user:
  - Wants to work on multiple branches simultaneously or in parallel
  - Needs to start a new feature/task while preserving current work
  - Asks about git worktree operations (create, remove, list, clean)
  - Mentions "twig" commands (add, remove, clean, list, init)
  - Wants to carry or move uncommitted changes to a new branch
  - Wants to copy/sync changes between branches
  - Needs to isolate work in a separate directory
  - Asks about switching context without stashing
  - Wants to clean up old/merged branches and their worktrees
  - Says phrases like "new worktree", "create worktree", "branch off",
    "work on something else", "start new work", "parallel work",
    "separate workspace", "another branch"

  Use this skill for ANY worktree-related operation, not just when
  explicitly asking about twig.

allowed-tools: Read
---

# twig CLI Guide

twig is a CLI tool that simplifies git worktree workflows by automating branch
creation, symlinks, and change management in a single command.

## Commands Overview

| Command | Purpose |
| ------- | ------- |
| `twig init` | Initialize twig configuration |
| `twig add <name>` | Create a new worktree with symlinks |
| `twig remove <branch>...` | Remove worktrees and their branches |
| `twig list` | List all worktrees |
| `twig clean` | Remove unneeded worktrees |

## Typical Workflows

### Start new feature work

Create a new worktree for a feature branch:

```bash
twig add feat/new-feature
```

This creates a worktree at the configured destination directory, creates
a new branch if it doesn't exist, and sets up symlinks.

### Move current changes to a new branch

When you realize current work should be on a different branch:

```bash
twig add feat/correct-branch --carry
```

The `--carry` flag moves uncommitted changes to the new worktree.
The source worktree becomes clean.

### Copy changes to a new branch

When you want changes in both the current and new worktree:

```bash
twig add feat/experiment --sync
```

The `--sync` flag copies uncommitted changes to both worktrees.

### Carry only specific files

When you want to carry only certain files:

```bash
twig add feat/new --carry --file "*.go" --file "cmd/**"
```

### Clean up after merging

Remove worktrees for branches that have been merged:

```bash
twig clean
```

This shows candidates and prompts for confirmation. Use `--yes` to skip
the prompt.

### Force remove a worktree

Remove a worktree even with uncommitted changes:

```bash
twig remove feat/abandoned -f
```

Use `-ff` to also remove locked worktrees.

## Configuration

see ./references/configuration.md

## Command Details

For detailed information on each command, refer to:

- ./references/commands/add.md - Create worktrees with sync/carry options
- ./references/commands/remove.md - Remove worktrees and branches
- ./references/commands/list.md - List worktrees
- ./references/commands/clean.md - Clean merged worktrees
- ./references/commands/init.md - Initialize configuration
- ./references/configuration.md - Configuration file details
