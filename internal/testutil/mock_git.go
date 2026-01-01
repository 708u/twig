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

	// WorktreeRemoveErr is returned when worktree remove is called.
	WorktreeRemoveErr error

	// BranchDeleteErr is returned when branch -d/-D is called.
	BranchDeleteErr error

	// CapturedArgs captures the args passed to git commands.
	CapturedArgs *[]string

	// HasChanges indicates if git status --porcelain returns output.
	HasChanges bool

	// StashPushErr is returned when stash push is called.
	StashPushErr error

	// StashApplyErr is returned when stash apply is called.
	StashApplyErr error

	// StashPopErr is returned when stash pop is called.
	StashPopErr error
}

func (m *MockGitExecutor) Run(args ...string) ([]byte, error) {
	if m.RunFunc != nil {
		return m.RunFunc(args...)
	}
	return m.defaultRun(args...)
}

func (m *MockGitExecutor) defaultRun(args ...string) ([]byte, error) {
	// Skip -C <dir> option (directory specification, not a command)
	for len(args) >= 2 && args[0] == "-C" {
		args = args[2:]
	}

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
			case "remove":
				return m.handleWorktreeRemove(args)
			}
		}
	case "branch":
		return m.handleBranch(args)
	case "status":
		return m.handleStatus(args)
	case "stash":
		return m.handleStash(args)
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

func (m *MockGitExecutor) handleWorktreeRemove(args []string) ([]byte, error) {
	if m.CapturedArgs != nil {
		*m.CapturedArgs = args
	}
	return nil, m.WorktreeRemoveErr
}

func (m *MockGitExecutor) handleBranch(args []string) ([]byte, error) {
	if m.CapturedArgs != nil {
		*m.CapturedArgs = args
	}
	// args: ["branch", "-d"/"-D", "branch-name"]
	if len(args) >= 3 && (args[1] == "-d" || args[1] == "-D") {
		return nil, m.BranchDeleteErr
	}
	return nil, nil
}

func (m *MockGitExecutor) handleStatus(args []string) ([]byte, error) {
	// args: ["status", "--porcelain"]
	if len(args) >= 2 && args[1] == "--porcelain" {
		if m.HasChanges {
			return []byte("M  modified.go\n"), nil
		}
		return []byte{}, nil
	}
	return nil, nil
}

func (m *MockGitExecutor) handleStash(args []string) ([]byte, error) {
	if len(args) < 2 {
		return nil, nil
	}
	switch args[1] {
	case "push":
		return nil, m.StashPushErr
	case "apply":
		return nil, m.StashApplyErr
	case "pop":
		return nil, m.StashPopErr
	}
	return nil, nil
}
