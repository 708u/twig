# twig

A CLI tool that creates, deletes, and manages git worktrees and branches in a single command.
Focused on simplifying git operations, keeping features minimal.

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

## Design Philosophy

twig treats branches and worktrees as a single unified concept.
Users don't need to think about whether they're managing a "branch" or a "worktree" -
they simply work with named development contexts.

- `twig add feat/x` creates both the branch and worktree together
- `twig remove feat/x` deletes both together
- Even if a worktree directory is deleted externally, `twig remove` still works

This 1:1 mapping simplifies the mental model: one name, one workspace, one command.

## Features

### Create worktree and branch in one command

`twig add feat/xxx` executes worktree creation, branch creation, and symlink setup all at once.
Use `--source` to create from any branch regardless of current worktree.
Set `default_source` in config to always branch from a fixed base (e.g., main).

### Automatic symlink management via config

Create new worktrees with personal settings like .envrc and Claude configs carried over.
Git worktree operations don't copy gitignored files, so twig uses symlinks to share these files across worktrees.
Start working immediately in new worktrees without manual setup.

### Move uncommitted changes to a new branch

Use `--carry` to move changes to a new worktree, or `--sync` to copy them to both.
Use `--file` with `--carry` to move only specific files matching a glob pattern.

Examples:

- Move refactoring ideas to a separate worktree and continue main work
- Extract WIP changes to a new branch before switching tasks

### Clean up worktrees no longer needed

`twig clean` removes worktrees that are merged, have upstream gone, or are prunable.

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

# Clean up worktrees no longer needed
twig clean

# Delete a specific worktree
twig remove feat/done
```

## Configuration

Configure in `.twig/settings.toml`:

- `worktree_destination_base_dir`: Destination directory for worktrees
- `default_source`: Source branch for symlinks (creates symlinks from main even when adding from a derived worktree)
- `symlinks`: Glob patterns for symlink targets
- `init_submodules`: Initialize submodules when creating worktrees

Personal settings can be overridden in `.twig/settings.local.toml` (.gitignore recommended).

- `extra_symlinks`: Add personal patterns while preserving team settings

Details: [docs/reference/configuration.md](docs/reference/configuration.md)

## Shell Completion

Shell completion is available for all commands and flags.
For example, `twig remove <TAB>` completes existing branch names.

Add the following to your shell configuration:

### Bash

Add to `~/.bashrc`:

```bash
eval "$(twig completion bash)"
```

### Zsh

Add to `~/.zshrc`:

```bash
eval "$(twig completion zsh)"
```

### Fish

Add to `~/.config/fish/config.fish`:

```sh
twig completion fish | source
```

## Command Specs

| Command                                            | Description                                      |
| -------------------------------------------------- | ------------------------------------------------ |
| [init](docs/reference/commands/init.md)            | Initialize settings                              |
| [add](docs/reference/commands/add.md)              | Create worktree and branch                       |
| [list](docs/reference/commands/list.md)            | List worktrees                                   |
| [remove](docs/reference/commands/remove.md)        | Delete worktree and branch (multiple supported)  |
| [clean](docs/reference/commands/clean.md)          | Bulk delete merged worktrees                     |
| [sync](docs/reference/commands/sync.md)            | Sync symlinks and submodules to worktrees        |

See the documentation above for detailed flags and specifications.

## Claude Code Plugin

A [Claude Code](https://docs.anthropic.com/en/docs/claude-code) plugin is
available for AI-assisted worktree management. The plugin provides an agent
and skill that help Claude understand twig commands and execute worktree
operations.

### Plugin Installation

Run the following slash commands in a Claude Code session:

```txt
# Add the marketplace
/plugin marketplace add 708u/twig

# Install the plugin
/plugin install twig@708u-twig
```

### What the Plugin Provides

| Component           | Description                                 |
| ------------------- | ------------------------------------------- |
| twig-operator agent | Executes twig commands based on user intent |
| twig-guide skill    | Provides command syntax and usage details   |

### Usage Examples

Once installed, Claude can help with worktree operations:

- "Create a new worktree for feat/user-auth"
- "Move my current changes to a new branch"
- "Clean up merged worktrees"
- "I want to work on the API and frontend in parallel"

## License

[MIT](LICENSE)
