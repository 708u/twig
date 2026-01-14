# check subcommand

Validate twig configuration and symlink patterns.

## Usage

```txt
twig check [flags]
```

## Flags

| Flag        | Short | Description                     |
|-------------|-------|---------------------------------|
| `--verbose` | `-v`  | Show all checks including passed|
| `--quiet`   | `-q`  | Show only errors                |

## Behavior

Performs validation checks on:

1. Configuration files
2. Symlink patterns

### Config Checks

| Check                                    | Severity | Description                     |
|------------------------------------------|----------|---------------------------------|
| TOML syntax                              | Error    | Validates TOML file parsing     |
| `worktree_destination_base_dir` exists   | Warning  | Directory must exist            |
| `worktree_destination_base_dir` writable | Warning  | Must be able to write files     |

### Symlink Checks

| Check                    | Severity | Description                            |
|--------------------------|----------|----------------------------------------|
| Invalid glob pattern     | Error    | Pattern syntax is invalid              |
| Pattern matches no files | Warning  | No files found matching the pattern    |
| Matched file is gitignored| Info    | File may not behave as expected        |

## Output Format

Default output groups items by category:

```txt
config:
  [ok] TOML syntax valid
  [warn] worktree_destination_base_dir does not exist: /path/to/worktrees
         suggestion: run 'mkdir -p /path/to/worktrees'

symlinks:
  [warn] pattern ".envrc" matches no files
         suggestion: remove from symlinks or create the file
  [info] ".claude" is gitignored (symlink may not work as expected)

Summary: 0 errors, 2 warnings, 1 info
```

With `--quiet`, only errors are shown:

```txt
[error] invalid TOML syntax: line 5: unexpected character
```

With `--verbose`, passed checks are also shown:

```txt
config:
  [ok] TOML syntax valid
  [ok] worktree_destination_base_dir exists and is writable

symlinks:
  [ok] pattern ".envrc" matches 1 file(s)
  [ok] pattern "config/**" matches 3 file(s)

Summary: 0 errors, 0 warnings, 0 info
```

## Exit Code

- 0: No errors (warnings and info are allowed)
- 1: One or more errors found

## Examples

```txt
# Basic check
twig check
config:
  [ok] TOML syntax valid
  [ok] worktree_destination_base_dir exists and is writable

symlinks:
  [ok] pattern ".envrc" matches 1 file(s)

Summary: 0 errors, 0 warnings, 0 info

# Verbose output (shows all checks)
twig check --verbose

# Quiet output (errors only, for CI)
twig check --quiet

# Check in a different directory
twig check -C /path/to/repo
```
