package testutil

import (
	"errors"
	"slices"
	"strings"
)

// MockWorktree represents a worktree entry for testing.
type MockWorktree struct {
	Path   string
	Branch string
	HEAD   string
}

// MockGitExecutor is a mock implementation of gwt.GitExecutor for testing.
type MockGitExecutor struct {
	// RunFunc overrides the default behavior if set.
	RunFunc func(args ...string) ([]byte, error)

	// ExistingBranches is a list of branches that exist locally.
	ExistingBranches []string

	// Worktrees is a list of worktrees with their paths and branches.
	Worktrees []MockWorktree

	// WorktreeAddErr is returned when worktree add is called.
	WorktreeAddErr error

	// CapturedArgs captures the args passed to worktree add.
	CapturedArgs *[]string
}

func (m *MockGitExecutor) Run(args ...string) ([]byte, error) {
	if m.RunFunc != nil {
		return m.RunFunc(args...)
	}
	return m.defaultRun(args...)
}

func (m *MockGitExecutor) defaultRun(args ...string) ([]byte, error) {
	if len(args) == 0 {
		return nil, nil
	}

	switch args[0] {
	case "rev-parse":
		return m.handleRevParse(args)
	case "worktree":
		if len(args) > 1 {
			switch args[1] {
			case "list":
				return m.handleWorktreeList()
			case "add":
				return m.handleWorktreeAdd(args)
			}
		}
	}
	return nil, nil
}

func (m *MockGitExecutor) handleRevParse(args []string) ([]byte, error) {
	// args: ["rev-parse", "--verify", "refs/heads/{branch}"]
	if len(args) < 3 {
		return nil, nil
	}
	ref := args[2]
	branch, ok := strings.CutPrefix(ref, "refs/heads/")
	if !ok {
		return nil, nil
	}
	// Check ExistingBranches first
	if slices.Contains(m.ExistingBranches, branch) {
		return nil, nil
	}
	// Check branches in Worktrees (worktree branches always exist)
	for _, wt := range m.Worktrees {
		if wt.Branch == branch {
			return nil, nil
		}
	}
	return nil, errors.New("not found")
}

func (m *MockGitExecutor) handleWorktreeList() ([]byte, error) {
	var lines []string
	for _, wt := range m.Worktrees {
		head := wt.HEAD
		if head == "" {
			head = "abc123"
		}
		lines = append(lines, "worktree "+wt.Path)
		lines = append(lines, "HEAD "+head)
		lines = append(lines, "branch refs/heads/"+wt.Branch)
		lines = append(lines, "")
	}
	return []byte(strings.Join(lines, "\n")), nil
}

func (m *MockGitExecutor) handleWorktreeAdd(args []string) ([]byte, error) {
	if m.CapturedArgs != nil {
		*m.CapturedArgs = args
	}
	return nil, m.WorktreeAddErr
}
