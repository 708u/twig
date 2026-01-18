# list subcommand

List all worktrees.

## Usage

```txt
twig list [flags]
```

## Flags

| Flag        | Short | Description                               |
|-------------|-------|-------------------------------------------|
| `--quiet`   | `-q`  | Output only worktree paths                |
| `--verbose` | `-v`  | Enable verbose output (use -vv for debug) |

## Behavior

- Lists all worktrees including the main worktree
- Default output shows path, commit hash, and branch name
  (compatible with `git worktree list`)
- With `--quiet`: shows only worktree paths
- With `-vv`: shows git command execution traces (for debugging)

## Examples

```txt
# Default output (git worktree list compatible)
twig list
/Users/user/repo                                   abc1234 [main]
/Users/user/repo-worktree/feat/add-list-command    def5678 [feat/add-list-command]
/Users/user/repo-worktree/feat/add-move-command    012abcd [feat/add-move-command]

# Quiet output (paths only, for scripting)
twig list -q
/Users/user/repo
/Users/user/repo-worktree/feat/add-list-command
/Users/user/repo-worktree/feat/add-move-command

# Debug output (shows git command traces)
twig list -vv
2026-01-17 12:34:56.000 [DEBUG] git: git -C /Users/user/repo worktree list --porcelain
/Users/user/repo                                   abc1234 [main]
/Users/user/repo-worktree/feat/add-list-command    def5678 [feat/add-list-command]
/Users/user/repo-worktree/feat/add-move-command    012abcd [feat/add-move-command]
```

## Shell Integration

Combine with fzf for quick worktree navigation:

```bash
gcd() {
  local selected
  selected=$(twig list -q | fzf +m) &&
  cd "$selected"
}
```
