# list subcommand

List all worktrees in `git worktree list` compatible format.

## Usage

```txt
gwt list [flags]
```

## Flags

| Flag      | Short | Description               |
|-----------|-------|---------------------------|
| `--quiet` | `-q`  | Output only worktree paths |

## Behavior

- Lists all worktrees including the main worktree
- Output format is compatible with `git worktree list`
- Shows path, short commit hash, and branch name for each worktree
- Displays additional status: locked, prunable, detached HEAD

## Examples

```txt
gwt list
/Users/user/repo                                   d9ef543 [main]
/Users/user/repo-worktree/feat/add-list-command    abc1234 [feat/add-list-command]
/Users/user/repo-worktree/feat/add-move-command    def5678 [feat/add-move-command]

# Detached HEAD example
/Users/user/repo-worktree/detached                 1234abc (detached HEAD)

# Locked worktree example
/Users/user/repo-worktree/locked                   5678def [locked-branch] locked
```

### Quiet Option

With `--quiet`, only worktree paths are output, one per line.
This is useful for piping to other commands.

```bash
cd $(gwt list -q | fzf)
```

## Shell Integration

Combine with fzf for quick worktree navigation:

```bash
gcd() {
  local selected
  selected=$(gwt list -q | fzf +m) &&
  cd "$selected"
}
```
