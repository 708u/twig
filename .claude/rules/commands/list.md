# list subcommand

List all worktrees.

## Usage

```txt
gwt list [flags]
```

## Flags

| Flag     | Short | Description                              |
|----------|-------|------------------------------------------|
| `--path` | `-p`  | Show full paths instead of branch names  |

## Behavior

- Lists all worktrees including the main worktree
- Default output shows branch names only
- With `--path`: shows full filesystem paths

## Examples

```txt
# List branch names
gwt list
main
feat/add-list-command
feat/add-move-command

# List full paths (for cd integration)
gwt list --path
/Users/user/repo
/Users/user/repo-worktree/feat/add-list-command
/Users/user/repo-worktree/feat/add-move-command
```

## Shell Integration

Combine with fzf for quick worktree navigation:

```bash
gcd() {
  local selected
  selected=$(gwt list --path | fzf +m) &&
  cd "$selected"
}
```
