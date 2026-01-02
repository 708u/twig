package gwt

import (
	"strings"
)

// ListCommand lists all worktrees.
type ListCommand struct {
	Git *GitRunner
}

// NewListCommand creates a new ListCommand.
func NewListCommand(dir string) *ListCommand {
	return &ListCommand{
		Git: NewGitRunner(dir),
	}
}

// ListResult holds the result of a list operation.
type ListResult struct {
	Worktrees []WorktreeInfo
}

// ListFormatOptions configures list output formatting.
type ListFormatOptions struct {
	Quiet bool
}

// Format formats the ListResult for display.
func (r ListResult) Format(opts ListFormatOptions) FormatResult {
	if opts.Quiet {
		return r.formatQuiet()
	}
	return r.formatDefault()
}

// formatQuiet outputs only the worktree paths.
func (r ListResult) formatQuiet() FormatResult {
	var stdout strings.Builder
	for _, wt := range r.Worktrees {
		stdout.WriteString(wt.Path)
		stdout.WriteString("\n")
	}
	return FormatResult{Stdout: stdout.String()}
}

// formatDefault outputs git worktree list compatible format.
func (r ListResult) formatDefault() FormatResult {
	var stdout strings.Builder

	for _, wt := range r.Worktrees {
		stdout.WriteString(wt.formatLine())
		stdout.WriteString("\n")
	}

	return FormatResult{Stdout: stdout.String()}
}

// formatLine returns git worktree list compatible format for a single worktree.
func (w WorktreeInfo) formatLine() string {
	var sb strings.Builder
	sb.WriteString(w.Path)
	sb.WriteString("  ")
	sb.WriteString(w.ShortHEAD())
	sb.WriteString(" ")

	if w.Bare {
		sb.WriteString("(bare)")
	} else if w.Detached {
		sb.WriteString("(detached HEAD)")
	} else {
		sb.WriteString("[")
		sb.WriteString(w.Branch)
		sb.WriteString("]")
	}

	if w.Locked {
		sb.WriteString(" locked")
	}
	if w.Prunable {
		sb.WriteString(" prunable")
	}

	return sb.String()
}

// Run lists all worktrees.
func (c *ListCommand) Run() (ListResult, error) {
	worktrees, err := c.Git.WorktreeList()
	if err != nil {
		return ListResult{}, err
	}

	return ListResult{Worktrees: worktrees}, nil
}
