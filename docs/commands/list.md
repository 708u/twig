# list subcommand

List all worktrees.

## Usage

```txt
gwt list [flags]
```

## Flags

| Flag      | Short | Description                 |
|-----------|-------|-----------------------------|
| `--quiet` | `-q`  | Output only worktree paths  |

## Behavior

- Lists all worktrees including the main worktree
- Default output shows path, commit hash, and branch name
  (compatible with `git worktree list`)
- With `--quiet`: shows only worktree paths

## Examples

```txt
# Default output (git worktree list compatible)
gwt list
/Users/user/repo                                   abc1234 [main]
/Users/user/repo-worktree/feat/add-list-command    def5678 [feat/add-list-command]
/Users/user/repo-worktree/feat/add-move-command    012abcd [feat/add-move-command]

# Quiet output (paths only, for scripting)
gwt list -q
/Users/user/repo
/Users/user/repo-worktree/feat/add-list-command
/Users/user/repo-worktree/feat/add-move-command
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
