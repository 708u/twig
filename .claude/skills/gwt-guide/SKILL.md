---
name: gwt-guide
description: |
  Guide to gwt, a CLI tool that manages git worktrees and branches together.
  Creates worktrees with automatic branch creation and symlinks (gwt add),
  lists worktrees (gwt list), removes worktrees with branch deletion (gwt remove).
allowed-tools: Read
---

# gwt Command Guide

gwt simplifies git worktree workflows by automating branch creation,
symlink setup, and cleanup in a single command.

## Commands

| Command      | Description                                     |
|--------------|-------------------------------------------------|
| `gwt add`    | Create a new worktree with optional symlinks    |
| `gwt list`   | List all worktrees                              |
| `gwt remove` | Remove worktrees and delete associated branches |

## Command Documentation

For detailed usage, flags, and examples:

- [gwt add](add.md) - Create worktrees with symlinks
- [gwt list](list.md) - List worktrees
- [gwt remove](remove.md) - Remove worktrees
