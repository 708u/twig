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
- With `--verbose`: shows uncommitted changes and lock reasons
- With `-vv`: shows git command execution traces (for debugging)
- When `--quiet` is specified, `--verbose` is ignored

### Verbose Output

With `--verbose`, each worktree line is followed by additional
detail lines:

- **Lock reason**: displayed when a worktree is locked with a
  reason
- **Changed files**: uncommitted changes in the worktree,
  using `git status --porcelain` format

Changed files are fetched in parallel for all worktrees.
Bare and prunable worktrees are skipped (no working tree).

## Examples

```txt
# Default output (git worktree list compatible)
twig list
/Users/user/repo                abc1234 [main]
/Users/user/repo-worktree/a     def5678 [feat/a]
/Users/user/repo-worktree/b     012abcd [feat/b]

# Verbose output (shows changes and lock reasons)
twig list -v
/Users/user/repo                abc1234 [main]
/Users/user/repo-worktree/a     def5678 [feat/a]
   M src/main.go
  ?? tmp/debug.log
/Users/user/repo-worktree/usb   789abcd [feat/usb] locked
  lock reason: USB drive work
   M config.toml
/Users/user/repo-worktree/b     012abcd [feat/b]

# Quiet output (paths only, for scripting)
twig list -q
/Users/user/repo
/Users/user/repo-worktree/a
/Users/user/repo-worktree/b

# Debug output (shows git command traces)
twig list -vv
2026-01-17 12:34:56.000 [DEBUG] git: git -C ... worktree list --porcelain
/Users/user/repo                abc1234 [main]
/Users/user/repo-worktree/a     def5678 [feat/a]
/Users/user/repo-worktree/b     012abcd [feat/b]
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
