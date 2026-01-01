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
	ShowPath bool
}

// Format formats the ListResult for display.
func (r ListResult) Format(opts ListFormatOptions) FormatResult {
	var stdout strings.Builder

	for _, wt := range r.Worktrees {
		if opts.ShowPath {
			stdout.WriteString(wt.Path)
		} else {
			stdout.WriteString(wt.Branch)
		}
		stdout.WriteString("\n")
	}

	return FormatResult{Stdout: stdout.String()}
}

// Run lists all worktrees.
func (c *ListCommand) Run() (ListResult, error) {
	worktrees, err := c.Git.WorktreeList()
	if err != nil {
		return ListResult{}, err
	}

	return ListResult{Worktrees: worktrees}, nil
}
