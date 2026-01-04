package gwt

import (
	"bytes"
	"fmt"
	"strings"
	"text/tabwriter"
)

// ListCommand lists all worktrees.
type ListCommand struct {
	Git *GitRunner
}

// NewListCommand creates a ListCommand with explicit dependencies (for testing).
func NewListCommand(git *GitRunner) *ListCommand {
	return &ListCommand{
		Git: git,
	}
}

// NewDefaultListCommand creates a ListCommand with production defaults.
func NewDefaultListCommand(dir string) *ListCommand {
	return NewListCommand(NewGitRunner(dir))
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
	var buf bytes.Buffer
	w := tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)

	for _, wt := range r.Worktrees {
		fmt.Fprintf(w, "%s\t%s %s\n", wt.Path, wt.ShortHEAD(), wt.formatStatus())
	}
	w.Flush()

	return FormatResult{Stdout: buf.String()}
}

// formatStatus returns the status portion of the worktree line (branch, locked, prunable).
func (w WorktreeInfo) formatStatus() string {
	var sb strings.Builder

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
