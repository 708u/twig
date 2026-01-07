# twig

A CLI tool that creates, deletes, and manages git worktrees and branches in a single command.
Focused on simplifying git operations, keeping features minimal.

## Design Philosophy

twig treats branches and worktrees as a single unified concept.
Users don't need to think about whether they're managing a "branch" or a "worktree" -
they simply work with named development contexts.

- `twig add feat/x` creates both the branch and worktree together
- `twig remove feat/x` deletes both together
- Even if a worktree directory is deleted externally, `twig remove` still works

This 1:1 mapping simplifies the mental model: one name, one workspace, one command.

## Motivation

twig is designed to be friendly for both humans and agentic coding tools,
and to integrate easily with other CLI tools:

- `--quiet` minimizes output to paths only, making it easy to pass to other tools
- For human use, `--verbose` and interactive confirmations ensure safety

Examples:

```bash
cd $(twig add feat/x -q)            # cd into the created worktree
twig list -q | fzf                  # select a worktree with fzf
twig list -q | xargs -I {} code {}  # open all worktrees in VSCode
twig clean -v                       # confirm before deletion, show all skipped items
```

## Features

### Create worktree and branch in one command

`twig add feat/xxx` executes worktree creation, branch creation, and symlink setup all at once.
Use `--source` to create from any branch regardless of current worktree.
Set `default_source` in config to always branch from a fixed base (e.g., main).

### Automatic symlink management via config

Create new worktrees with personal settings like .envrc and Claude configs carried over.
Start working immediately in new worktrees.

### Move uncommitted changes to a new branch

Use `--carry` to move changes to a new worktree, or `--sync` to copy them to both.
Use `--file` with `--carry` to move only specific files matching a glob pattern.

Examples:

- Move refactoring ideas to a separate worktree and continue main work
- Extract WIP changes to a new branch before switching tasks

### Bulk delete merged worktrees

`twig clean` deletes merged worktrees and their branches together.

## Installation

Requires Git 2.15+.

### Homebrew

```bash
brew install 708u/tap/twig
```

### Go

```bash
go install github.com/708u/twig/cmd/twig@latest
```

## Shell Completion

Shell completion is available for all commands and flags.
For example, `twig remove <TAB>` completes existing branch names.

Add the following to your shell configuration:

### Bash

```bash
# Add to ~/.bashrc
eval "$(twig completion bash)"
```

### Zsh

```bash
# Add to ~/.zshrc
eval "$(twig completion zsh)"
```

### Fish

```sh
# Add to ~/.config/fish/config.fish
twig completion fish | source
```

## Quick Start

```bash
# Initialize settings
twig init

# Create a new worktree and branch
twig add feat/new-feature

# Copy uncommitted changes to a new worktree
twig add feat/wip --sync

# Move uncommitted changes to a new worktree
twig add feat/wip --carry

# List worktrees
twig list

# Bulk delete merged worktrees
twig clean

# Delete a specific worktree
twig remove feat/done
```

## Configuration

Configure in `.twig/settings.toml`:

- `worktree_destination_base_dir`: Destination directory for worktrees
- `default_source`: Source branch for symlinks (creates symlinks from main even when adding from a derived worktree)
- `symlinks`: Glob patterns for symlink targets

Personal settings can be overridden in `.twig/settings.local.toml` (.gitignore recommended).

- `extra_symlinks`: Add personal patterns while preserving team settings

Details: [docs/configuration.md](docs/configuration.md)

## Command Specs

| Command                              | Description                                      |
| ------------------------------------ | ------------------------------------------------ |
| [init](docs/commands/init.md)        | Initialize settings                              |
| [add](docs/commands/add.md)          | Create worktree and branch                       |
| [list](docs/commands/list.md)        | List worktrees                                   |
| [remove](docs/commands/remove.md)    | Delete worktree and branch (multiple supported)  |
| [clean](docs/commands/clean.md)      | Bulk delete merged worktrees                     |

See the documentation above for detailed flags and specifications.

## License

[MIT](LICENSE)
