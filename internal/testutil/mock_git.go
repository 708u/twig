package testutil

import (
	"context"
	"slices"
	"strings"
)

// MockExitError simulates exec.ExitError for testing.
type MockExitError struct {
	Code int
}

func (e *MockExitError) Error() string {
	return "exit status " + string(rune('0'+e.Code))
}

func (e *MockExitError) ExitCode() int {
	return e.Code
}

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
	RunFunc func(ctx context.Context, args ...string) ([]byte, error)

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

	// StatusOutput is the custom output for git status --porcelain.
	// If set, this overrides HasChanges.
	StatusOutput string

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

	// BranchHEADs maps branch name to its HEAD commit hash.
	// Used by rev-parse and for-each-ref to return commit hashes for branches.
	BranchHEADs map[string]string

	// Remotes is a list of configured remote names.
	Remotes []string

	// RemoteBranches maps remote name to list of branches on that remote.
	// Used by for-each-ref to check local remote-tracking branches.
	RemoteBranches map[string][]string

	// FetchErr is returned when fetch is called.
	FetchErr error

	// SubmoduleStatusOutput is the output of `git submodule status --recursive`.
	// Empty string means no submodules.
	SubmoduleStatusOutput string

	// SubmoduleUpdateErr is returned when submodule update is called.
	SubmoduleUpdateErr error

	// SubmoduleUpdateCalled is set to true when submodule update is called.
	SubmoduleUpdateCalled bool

	// SubmoduleUpdateArgs captures the args passed to submodule update.
	SubmoduleUpdateArgs []string

	// WorktreeRootMap maps directory to its worktree root.
	// Used by rev-parse --show-toplevel to return the worktree root for a directory.
	WorktreeRootMap map[string]string

	// StatusByDir maps directory to status output for that directory.
	// Used when git status is called with -C <dir> to return per-worktree status.
	StatusByDir map[string]string
}

func (m *MockGitExecutor) Run(ctx context.Context, args ...string) ([]byte, error) {
	if m.RunFunc != nil {
		return m.RunFunc(ctx, args...)
	}
	return m.defaultRun(args...)
}

func (m *MockGitExecutor) defaultRun(args ...string) ([]byte, error) {
	// Extract -C <dir> option and track it for commands that need it
	var dir string
	for len(args) >= 2 && args[0] == "-C" {
		dir = args[1]
		args = args[2:]
	}

	if len(args) == 0 {
		return nil, nil
	}

	switch args[0] {
	case "rev-parse":
		return m.handleRevParse(args, dir)
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
		return m.handleStatus(args, dir)
	case "stash":
		return m.handleStash(args)
	case "for-each-ref":
		return m.handleForEachRef(args)
	case "fetch":
		return m.handleFetch(args)
	case "submodule":
		return m.handleSubmodule(args)
	}
	return nil, nil
}

func (m *MockGitExecutor) handleRevParse(args []string, dir string) ([]byte, error) {
	// Handle --show-toplevel for WorktreeRoot
	if len(args) >= 2 && args[1] == "--show-toplevel" {
		// Look up the worktree root for the given directory
		if m.WorktreeRootMap != nil && dir != "" {
			if root, ok := m.WorktreeRootMap[dir]; ok {
				return []byte(root + "\n"), nil
			}
		}
		// Find worktree that contains dir
		// dir must be equal to wt.Path or start with wt.Path + "/"
		for _, wt := range m.Worktrees {
			if dir == wt.Path || strings.HasPrefix(dir, wt.Path+"/") {
				return []byte(wt.Path + "\n"), nil
			}
		}
		// No worktree contains dir
		return nil, &MockExitError{Code: 128}
	}

	// Handle stash@{0} for StashPush hash retrieval
	if len(args) >= 2 && args[1] == "stash@{0}" {
		hash := m.StashHash
		if hash == "" {
			hash = "abc123def456"
		}
		return []byte(hash + "\n"), nil
	}

	// Handle rev-parse <branch> (without --verify) for commit hash lookup
	if len(args) == 2 && args[1] != "--verify" {
		branch := args[1]
		if m.BranchHEADs != nil {
			if hash, ok := m.BranchHEADs[branch]; ok {
				return []byte(hash + "\n"), nil
			}
		}
		// Fallback: check worktrees for HEAD
		for _, wt := range m.Worktrees {
			if wt.Branch == branch {
				head := wt.HEAD
				if head == "" {
					head = "commit-" + branch
				}
				return []byte(head + "\n"), nil
			}
		}
		// Default hash based on branch name if not found
		return []byte("default-" + branch + "\n"), nil
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
	return nil, &MockExitError{Code: 1}
}

func (m *MockGitExecutor) handleWorktreeList() ([]byte, error) {
	var lines []string
	for _, wt := range m.Worktrees {
		head := wt.HEAD
		if head == "" {
			head = "abc1234567890"
		}
		lines = append(lines, "worktree "+wt.Path, "HEAD "+head)
		switch {
		case wt.Bare:
			lines = append(lines, "bare")
		case wt.Detached:
			lines = append(lines, "detached")
		default:
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

func (m *MockGitExecutor) handleStatus(args []string, dir string) ([]byte, error) {
	// args: ["status", "--porcelain", ...]
	if len(args) >= 2 && args[1] == "--porcelain" {
		// Check per-directory status first
		if dir != "" && m.StatusByDir != nil {
			if output, ok := m.StatusByDir[dir]; ok {
				return []byte(output), nil
			}
		}
		// Use StatusOutput if set (allows custom status output)
		if m.StatusOutput != "" {
			return []byte(m.StatusOutput), nil
		}
		if m.HasChanges {
			return []byte(" M modified.go\n"), nil
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
		if m.CapturedArgs != nil {
			*m.CapturedArgs = append(*m.CapturedArgs, args...)
		}
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

	format := ""
	ref := ""
	for i, arg := range args {
		if strings.HasPrefix(arg, "--format=") {
			format = strings.TrimPrefix(arg, "--format=")
		}
		if i >= 2 && !strings.HasPrefix(arg, "--") {
			ref = arg
		}
	}

	// Handle refs/heads/ for MergedBranches (all branches with commit hash and upstream status)
	// Format: "%(refname:short) %(objectname) %(upstream:track)"
	if ref == "refs/heads/" && strings.Contains(format, "%(objectname)") {
		var lines []string
		seen := make(map[string]bool)

		// First, add branches from Worktrees
		for _, wt := range m.Worktrees {
			if wt.Bare || wt.Detached {
				continue
			}
			head := wt.HEAD
			if head == "" {
				head = "default-" + wt.Branch
			}
			line := wt.Branch + " " + head
			if slices.Contains(m.UpstreamGoneBranches, wt.Branch) {
				line += " [gone]"
			}
			lines = append(lines, line)
			seen[wt.Branch] = true
		}

		// Then, add branches from BranchHEADs (if not already added)
		for branch, head := range m.BranchHEADs {
			if seen[branch] {
				continue
			}
			line := branch + " " + head
			if slices.Contains(m.UpstreamGoneBranches, branch) {
				line += " [gone]"
			}
			lines = append(lines, line)
			seen[branch] = true
		}

		// Finally, add branches from UpstreamGoneBranches (if not already added)
		for _, branch := range m.UpstreamGoneBranches {
			if seen[branch] {
				continue
			}
			line := branch + " default-" + branch + " [gone]"
			lines = append(lines, line)
		}

		return []byte(strings.Join(lines, "\n") + "\n"), nil
	}

	// Handle refs/heads/ for upstream status only (legacy format)
	// Format: "%(refname:short) %(upstream:track)"
	if ref == "refs/heads/" && strings.Contains(format, "%(upstream:track)") {
		var lines []string
		for _, branch := range m.UpstreamGoneBranches {
			lines = append(lines, branch+" [gone]")
		}
		return []byte(strings.Join(lines, "\n") + "\n"), nil
	}

	// Handle refs/heads/<branch> for single branch upstream tracking check
	if branch, ok := strings.CutPrefix(ref, "refs/heads/"); ok && branch != "" {
		if slices.Contains(m.UpstreamGoneBranches, branch) {
			return []byte("[gone]\n"), nil
		}
		return []byte("\n"), nil
	}

	// Handle refs/remotes/*/<branch> for remote branch detection
	if branch, ok := strings.CutPrefix(ref, "refs/remotes/*/"); ok {
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

func (m *MockGitExecutor) handleSubmodule(args []string) ([]byte, error) {
	if len(args) < 2 {
		return nil, nil
	}

	switch args[1] {
	case "status":
		// args: ["submodule", "status", "--recursive"]
		return []byte(m.SubmoduleStatusOutput), nil
	case "update":
		// args: ["submodule", "update", "--init", "--recursive", ...]
		m.SubmoduleUpdateCalled = true
		m.SubmoduleUpdateArgs = args
		return nil, m.SubmoduleUpdateErr
	}
	return nil, nil
}
