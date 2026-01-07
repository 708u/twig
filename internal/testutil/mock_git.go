package testutil

import (
	"errors"
	"slices"
	"strings"
)

// MockWorktree represents a worktree entry for testing.
type MockWorktree struct {
	Path           string
	Branch         string
	HEAD           string
	Detached       bool
	Locked         bool
	LockReason     string
	Prunable       bool
	PrunableReason string
	Bare           bool
}

// MockGitExecutor is a mock implementation of twig.GitExecutor for testing.
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

	// StashHash is returned when stash push succeeds and used for subsequent stash operations.
	StashHash string

	// StashApplyErr is returned when stash apply is called.
	StashApplyErr error

	// StashPopErr is returned when stash pop is called.
	StashPopErr error

	// StashDropErr is returned when stash drop is called.
	StashDropErr error

	// MergedBranches maps target branch to list of branches merged into it.
	MergedBranches map[string][]string

	// UpstreamGoneBranches is a list of branches whose upstream is gone.
	// Used by git for-each-ref to detect squash/rebase merged branches.
	UpstreamGoneBranches []string

	// WorktreePruneErr is returned when worktree prune is called.
	WorktreePruneErr error

	// Remotes is a list of configured remote names.
	Remotes []string

	// RemoteBranches maps remote name to list of branches on that remote.
	// Used by for-each-ref to check local remote-tracking branches.
	RemoteBranches map[string][]string

	// FetchErr is returned when fetch is called.
	FetchErr error
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
			case "prune":
				return m.handleWorktreePrune()
			}
		}
	case "branch":
		return m.handleBranch(args)
	case "status":
		return m.handleStatus(args)
	case "stash":
		return m.handleStash(args)
	case "for-each-ref":
		return m.handleForEachRef(args)
	case "fetch":
		return m.handleFetch(args)
	}
	return nil, nil
}

func (m *MockGitExecutor) handleRevParse(args []string) ([]byte, error) {
	// Handle stash@{0} for StashPush hash retrieval
	if len(args) >= 2 && args[1] == "stash@{0}" {
		hash := m.StashHash
		if hash == "" {
			hash = "abc123def456"
		}
		return []byte(hash + "\n"), nil
	}

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
			head = "abc1234567890"
		}
		lines = append(lines, "worktree "+wt.Path)
		lines = append(lines, "HEAD "+head)
		if wt.Bare {
			lines = append(lines, "bare")
		} else if wt.Detached {
			lines = append(lines, "detached")
		} else {
			lines = append(lines, "branch refs/heads/"+wt.Branch)
		}
		if wt.Locked {
			if wt.LockReason != "" {
				lines = append(lines, "locked "+wt.LockReason)
			} else {
				lines = append(lines, "locked")
			}
		}
		if wt.Prunable {
			if wt.PrunableReason != "" {
				lines = append(lines, "prunable "+wt.PrunableReason)
			} else {
				lines = append(lines, "prunable")
			}
		}
		lines = append(lines, "")
	}
	return []byte(strings.Join(lines, "\n")), nil
}

func (m *MockGitExecutor) handleWorktreeAdd(args []string) ([]byte, error) {
	if m.CapturedArgs != nil {
		*m.CapturedArgs = append(*m.CapturedArgs, args...)
	}
	return nil, m.WorktreeAddErr
}

func (m *MockGitExecutor) handleWorktreeRemove(args []string) ([]byte, error) {
	if m.CapturedArgs != nil {
		*m.CapturedArgs = append(*m.CapturedArgs, args...)
	}
	return nil, m.WorktreeRemoveErr
}

func (m *MockGitExecutor) handleWorktreePrune() ([]byte, error) {
	return nil, m.WorktreePruneErr
}

func (m *MockGitExecutor) handleBranch(args []string) ([]byte, error) {
	if m.CapturedArgs != nil {
		*m.CapturedArgs = append(*m.CapturedArgs, args...)
	}
	// args: ["branch", "-d"/"-D", "branch-name"]
	if len(args) >= 3 && (args[1] == "-d" || args[1] == "-D") {
		return nil, m.BranchDeleteErr
	}
	// args: ["branch", "--merged", "target", "--format=%(refname:short)"]
	if len(args) >= 3 && args[1] == "--merged" {
		target := args[2]
		branches := m.MergedBranches[target]
		return []byte(strings.Join(branches, "\n")), nil
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
	case "create":
		// stash create returns hash on stdout
		if m.StashPushErr != nil {
			return nil, m.StashPushErr
		}
		hash := m.StashHash
		if hash == "" {
			hash = "abc123def456"
		}
		return []byte(hash + "\n"), nil
	case "store":
		// stash store adds to reflog, no output
		return nil, nil
	case "push":
		return nil, m.StashPushErr
	case "apply":
		return nil, m.StashApplyErr
	case "pop":
		return nil, m.StashPopErr
	case "drop":
		return nil, m.StashDropErr
	case "list":
		// Return stash list with format "%gd %H"
		hash := m.StashHash
		if hash == "" {
			hash = "abc123def456"
		}
		return []byte("stash@{0} " + hash + "\n"), nil
	}
	return nil, nil
}

func (m *MockGitExecutor) handleForEachRef(args []string) ([]byte, error) {
	if len(args) < 3 {
		return nil, nil
	}

	ref := args[2]

	// Handle refs/heads/<branch> for upstream tracking check
	if branch, ok := strings.CutPrefix(ref, "refs/heads/"); ok {
		if slices.Contains(m.UpstreamGoneBranches, branch) {
			return []byte("[gone]\n"), nil
		}
		return []byte("\n"), nil
	}

	// Handle refs/remotes/*/<branch> for remote branch detection
	if strings.HasPrefix(ref, "refs/remotes/*/") {
		branch := strings.TrimPrefix(ref, "refs/remotes/*/")
		var results []string
		for remote, branches := range m.RemoteBranches {
			if slices.Contains(branches, branch) {
				results = append(results, remote+"/"+branch)
			}
		}
		if len(results) > 0 {
			return []byte(strings.Join(results, "\n") + "\n"), nil
		}
		return []byte{}, nil
	}

	return nil, nil
}

func (m *MockGitExecutor) handleFetch(args []string) ([]byte, error) {
	// args: ["fetch", "remote", "branch"]
	if m.CapturedArgs != nil {
		*m.CapturedArgs = append(*m.CapturedArgs, args...)
	}
	return nil, m.FetchErr
}
