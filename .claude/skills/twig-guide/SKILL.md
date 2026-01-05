---
name: twig-guide
description: |
  Guide to twig, a CLI tool that manages git worktrees and branches together.
  Creates worktrees with automatic branch creation and symlinks (twig add),
  lists worktrees (twig list), removes worktrees with branch deletion (twig remove).
allowed-tools: Read
---

# twig Command Guide

twig simplifies git worktree workflows by automating branch creation,
symlink setup, and cleanup in a single command.

## Commands

| Command      | Description                                     |
|--------------|-------------------------------------------------|
| `twig add`    | Create a new worktree with optional symlinks    |
| `twig list`   | List all worktrees                              |
| `twig remove` | Remove worktrees and delete associated branches |

## Command Documentation

For detailed usage, flags, and examples:

- [twig add](add.md) - Create worktrees with symlinks
- [twig list](list.md) - List worktrees
- [twig remove](remove.md) - Remove worktrees
