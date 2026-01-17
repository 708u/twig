# Work Log: Add Logger and -vv Flag

Date: 2026-01-17
Branch: feat/add-logger-vv-flag
PR: <https://github.com/708u/twig/pull/114>

## Summary

Introduced a Logger infrastructure using Go's standard log/slog library
to support -vv flag for debug logging. Phase 1 implementation focuses on
the list command only.

## Session 2: Review Response and Log Format Update

### Review Response

Simplified NewGitRunner API based on review feedback:

- Changed `NewGitRunner(dir, nil)` to `NewGitRunner(dir)` (no nil needed)
- Added `NewGitRunnerWithLogger(dir, log)` for explicit logger injection
- Removed nil check in `Run()` since Log is always non-nil now
- Added `Log: NewNopLogger()` to test structs that create GitRunner directly

### Log Format Update

Changed log output format from:

```txt
[git] git -C /path worktree list --porcelain
```

To:

```txt
12:34:56 [DEBUG] git: git -C /path worktree list --porcelain
```

Design decisions:

- Timestamp: `15:04:05` format (time only, no date for CLI brevity)
- Level: `[DEBUG]` in brackets, uppercase for grep-ability
- Category: `git:` with colon separator (avoids `[][]` redundancy)
- No category case: `12:34:56 [DEBUG] message` (omit colon)

## Files Read

### Core Implementation Files

- @git.go - GitRunner and git command abstractions
- @list.go - ListCommand implementation
- @cmd/twig/main.go - CLI entrypoint with cobra

### Documentation Files

- @docs/reference/commands/list.md - List command documentation
- @docs/reference/configuration.md - Configuration reference
- @docs/reference/commands/add.md - Add command documentation
- @docs/reference/commands/clean.md - Clean command documentation
- @docs/reference/commands/remove.md - Remove command documentation
- @docs/reference/commands/init.md - Init command documentation
- @docs/reference/commands/version.md - Version command documentation
- @docs/development/unit-testing.md - Testing guidelines
- @docs/development/integration-testing.md - Integration testing guidelines

### Test Files

- @git_integration_test.go - Git integration tests
- @add_integration_test.go - Add command integration tests
- @clean_integration_test.go - Clean command integration tests
- @remove_integration_test.go - Remove command integration tests
- @list_integration_test.go - List command integration tests
- @cmd/twig/main_integration_test.go - CLI integration tests

### Configuration Files

- @.github/pull_request_template.md - PR template
- @CLAUDE.md - Project instructions
- @.claude/rules/development/development-workflow.md - Development workflow

## Files Created

- @logger.go - CLIHandler (slog.Handler) and helper functions
- @logger_test.go - Unit tests for logger

## Files Modified

### Production Code

| File              | Changes                                                            |
|-------------------|--------------------------------------------------------------------|
| @git.go           | NewGitRunner(dir), NewGitRunnerWithLogger(dir, log), removed nil check |
| @list.go          | Uses NewGitRunnerWithLogger                                        |
| @cmd/twig/main.go | Uses simplified NewGitRunner(dir)                                  |
| @add.go           | Uses simplified NewGitRunner(dir)                                  |
| @clean.go         | Uses simplified NewGitRunner(dir)                                  |
| @remove.go        | Uses simplified NewGitRunner(dir)                                  |
| @logger.go        | New log format with timestamp and level                            |

### Test Code

| File                               | Changes                                |
|------------------------------------|----------------------------------------|
| @git_test.go                       | Added Log: NewNopLogger() to structs   |
| @add_test.go                       | Added Log: NewNopLogger() to structs   |
| @clean_test.go                     | Added Log: NewNopLogger() to structs   |
| @remove_test.go                    | Added Log: NewNopLogger() to structs   |
| @list_test.go                      | Added Log: NewNopLogger() to structs   |
| @logger_test.go                    | Updated for new log format             |
| @git_integration_test.go           | Uses simplified NewGitRunner(dir)      |
| @add_integration_test.go           | Uses simplified NewGitRunner(dir)      |
| @clean_integration_test.go         | Uses simplified NewGitRunner(dir)      |
| @remove_integration_test.go        | Uses simplified NewGitRunner(dir)      |
| @cmd/twig/main_integration_test.go | Uses simplified NewGitRunner(dir)      |
| @cmd/twig/main_test.go             | Added Log: twig.NewNopLogger()         |

### Documentation

| File                                                                              | Changes                    |
|-----------------------------------------------------------------------------------|----------------------------|
| @docs/reference/commands/list.md                                                  | Fixed table alignment, updated output example |
| @external/claude-code/plugins/twig/skills/twig-guide/references/commands/list.md  | Synced from docs           |

## Implementation Details

### Logger Design

- Used Go's standard log/slog library (no external dependencies)
- Created CLIHandler implementing slog.Handler interface
- Output format: `15:04:05 [LEVEL] category: message`
- Categories: git, debug, config, glob

### Verbosity Levels

| Level | Flag   | slog.Level | Output               |
|-------|--------|------------|----------------------|
| 0     | (none) | LevelWarn  | Errors/warnings only |
| 1     | -v     | LevelInfo  | Verbose output       |
| 2+    | -vv    | LevelDebug | Git command traces   |

### Key Functions

```go
// logger.go
func NewCLIHandler(w io.Writer, level slog.Level) *CLIHandler
func NewNopLogger() *slog.Logger
func VerbosityToLevel(verbosity int) slog.Level

// git.go
func NewGitRunner(dir string) *GitRunner           // uses NopLogger
func NewGitRunnerWithLogger(dir string, log *slog.Logger) *GitRunner
```

## Commits

1. feat(list): add -vv flag for debug logging with slog-based logger
2. chore: sync plugin docs for list command
3. refactor: simplify NewGitRunner API with separate logger factory
4. docs: fix table alignment in list command documentation

## Skills Used

- /continue
- /commit-push-update-pr
- /export-session

## Pending Changes

- @logger.go - Log format with timestamp and level
- @logger_test.go - Updated tests for new format
- @docs/reference/commands/list.md - Updated output example

## Future Work

- Extend -vv support to other commands (add, clean, remove)
- Add more debug categories as needed
