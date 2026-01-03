package gwt

import (
	"fmt"
	"os/exec"
	"strings"
)

// GitExecutor abstracts git command execution for testability.
// Commands are fixed to "git" - only subcommands and args are passed.
type GitExecutor interface {
	// Run executes git with args and returns stdout.
	Run(args ...string) ([]byte, error)
}

type osGitExecutor struct{}

func (e osGitExecutor) Run(args ...string) ([]byte, error) {
	return exec.Command("git", args...).Output()
}

// GitRunner provides git operations using GitExecutor.
type GitRunner struct {
	Executor GitExecutor
	Dir      string
}

// NewGitRunner creates a new GitRunner with the default executor.
func NewGitRunner(dir string) *GitRunner {
	return &GitRunner{
		Executor: osGitExecutor{},
		Dir:      dir,
	}
}

// InDir returns a GitRunner that executes commands in the specified directory.
func (g *GitRunner) InDir(dir string) *GitRunner {
	return &GitRunner{Executor: g.Executor, Dir: dir}
}

// Run executes git command with -C flag.
func (g *GitRunner) Run(args ...string) ([]byte, error) {
	return g.Executor.Run(append([]string{"-C", g.Dir}, args...)...)
}

type worktreeAddOptions struct {
	createBranch bool
}

// WorktreeAddOption is a functional option for WorktreeAdd.
type WorktreeAddOption func(*worktreeAddOptions)

// WithCreateBranch creates a new branch when adding the worktree.
func WithCreateBranch() WorktreeAddOption {
	return func(o *worktreeAddOptions) {
		o.createBranch = true
	}
}

// WorktreeAdd creates a new worktree at the specified path.
func (g *GitRunner) WorktreeAdd(path, branch string, opts ...WorktreeAddOption) ([]byte, error) {
	var o worktreeAddOptions
	for _, opt := range opts {
		opt(&o)
	}

	if o.createBranch {
		return g.worktreeAddWithNewBranch(branch, path)
	}
	return g.worktreeAdd(path, branch)
}

// BranchExists checks if a branch exists in the local repository.
func (g *GitRunner) BranchExists(branch string) bool {
	_, err := g.Run("rev-parse", "--verify", "refs/heads/"+branch)
	return err == nil
}

// BranchList returns all local branch names.
func (g *GitRunner) BranchList() ([]string, error) {
	output, err := g.Run("branch", "--format=%(refname:short)")
	if err != nil {
		return nil, err
	}
	var branches []string
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if line != "" {
			branches = append(branches, line)
		}
	}
	return branches, nil
}

// WorktreeInfo holds worktree path and branch information.
type WorktreeInfo struct {
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

// ShortHEAD returns the first 7 characters of the HEAD commit hash.
func (w WorktreeInfo) ShortHEAD() string {
	if len(w.HEAD) >= 7 {
		return w.HEAD[:7]
	}
	return w.HEAD
}

// WorktreeList returns all worktrees with their paths and branches.
func (g *GitRunner) WorktreeList() ([]WorktreeInfo, error) {
	out, err := g.worktreeListPorcelain()
	if err != nil {
		return nil, fmt.Errorf("failed to list worktrees: %w", err)
	}

	// porcelain format:
	// worktree /path/to/worktree
	// HEAD abc123
	// branch refs/heads/branch-name
	// detached (optional)
	// bare (optional)
	// locked [reason] (optional)
	// prunable [reason] (optional)
	// (blank line)

	var worktrees []WorktreeInfo
	var current WorktreeInfo
	for _, line := range strings.Split(string(out), "\n") {
		switch {
		case strings.HasPrefix(line, "worktree "):
			current = WorktreeInfo{Path: strings.TrimPrefix(line, "worktree ")}
		case strings.HasPrefix(line, "HEAD "):
			current.HEAD = strings.TrimPrefix(line, "HEAD ")
		case strings.HasPrefix(line, "branch refs/heads/"):
			current.Branch = strings.TrimPrefix(line, "branch refs/heads/")
		case line == "detached":
			current.Detached = true
		case line == "bare":
			current.Bare = true
		case strings.HasPrefix(line, "locked"):
			current.Locked = true
			if reason, ok := strings.CutPrefix(line, "locked "); ok {
				current.LockReason = reason
			}
		case strings.HasPrefix(line, "prunable"):
			current.Prunable = true
			if reason, ok := strings.CutPrefix(line, "prunable "); ok {
				current.PrunableReason = reason
			}
		case line == "" && current.Path != "":
			worktrees = append(worktrees, current)
			current = WorktreeInfo{}
		}
	}
	return worktrees, nil
}

// WorktreeListBranches returns a list of branch names currently checked out in worktrees.
func (g *GitRunner) WorktreeListBranches() ([]string, error) {
	output, err := g.worktreeListPorcelain()
	if err != nil {
		return nil, err
	}

	var branches []string
	for line := range strings.SplitSeq(string(output), "\n") {
		if branch, ok := strings.CutPrefix(line, "branch refs/heads/"); ok {
			branches = append(branches, branch)
		}
	}
	return branches, nil
}

// WorktreeFindByBranch returns the worktree path for the given branch.
// Returns an error if the branch is not checked out in any worktree.
func (g *GitRunner) WorktreeFindByBranch(branch string) (string, error) {
	out, err := g.worktreeListPorcelain()
	if err != nil {
		return "", fmt.Errorf("failed to list worktrees: %w", err)
	}

	// porcelain format:
	// worktree /path/to/worktree
	// HEAD abc123
	// branch refs/heads/branch-name
	// (blank line)

	lines := strings.Split(string(out), "\n")
	var currentPath string
	for _, line := range lines {
		if path, ok := strings.CutPrefix(line, "worktree "); ok {
			currentPath = path
		}
		if branchName, ok := strings.CutPrefix(line, "branch refs/heads/"); ok {
			if branchName == branch {
				return currentPath, nil
			}
		}
	}

	return "", fmt.Errorf("branch %q is not checked out in any worktree", branch)
}

type worktreeRemoveOptions struct {
	force bool
}

// WorktreeRemoveOption is a functional option for WorktreeRemove.
type WorktreeRemoveOption func(*worktreeRemoveOptions)

// WithForceRemove forces worktree removal even if there are uncommitted changes.
func WithForceRemove() WorktreeRemoveOption {
	return func(o *worktreeRemoveOptions) {
		o.force = true
	}
}

// WorktreeRemove removes the worktree at the given path.
// By default fails if there are uncommitted changes. Use WithForceRemove() to force.
func (g *GitRunner) WorktreeRemove(path string, opts ...WorktreeRemoveOption) ([]byte, error) {
	var o worktreeRemoveOptions
	for _, opt := range opts {
		opt(&o)
	}

	out, err := g.worktreeRemove(path, o.force)
	if err != nil {
		return nil, fmt.Errorf("failed to remove worktree: %w", err)
	}
	return out, nil
}

type branchDeleteOptions struct {
	force bool
}

// BranchDeleteOption is a functional option for BranchDelete.
type BranchDeleteOption func(*branchDeleteOptions)

// WithForceDelete forces branch deletion even if not fully merged.
func WithForceDelete() BranchDeleteOption {
	return func(o *branchDeleteOptions) {
		o.force = true
	}
}

// BranchDelete deletes a local branch.
// By default uses -d (safe delete). Use WithForceDelete() to use -D (force delete).
func (g *GitRunner) BranchDelete(branch string, opts ...BranchDeleteOption) ([]byte, error) {
	var o branchDeleteOptions
	for _, opt := range opts {
		opt(&o)
	}

	out, err := g.branchDelete(branch, o.force)
	if err != nil {
		return nil, fmt.Errorf("failed to delete branch: %w", err)
	}
	return out, nil
}

// HasChanges checks if there are any uncommitted changes (staged, unstaged, or untracked).
func (g *GitRunner) HasChanges() (bool, error) {
	output, err := g.Run("status", "--porcelain")
	if err != nil {
		return false, fmt.Errorf("failed to check git status: %w", err)
	}
	return len(output) > 0, nil
}

// StashPush stashes all changes including untracked files.
// Returns the stash commit hash for later reference.
func (g *GitRunner) StashPush(message string) (string, error) {
	if _, err := g.Run("stash", "push", "-u", "-m", message); err != nil {
		return "", err
	}
	out, err := g.Run("rev-parse", "stash@{0}")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// StashApplyByHash applies the stash with the given hash without dropping it.
func (g *GitRunner) StashApplyByHash(hash string) ([]byte, error) {
	return g.Run("stash", "apply", hash)
}

// StashPopByHash applies and drops the stash with the given hash.
func (g *GitRunner) StashPopByHash(hash string) ([]byte, error) {
	if _, err := g.StashApplyByHash(hash); err != nil {
		return nil, err
	}
	return g.StashDropByHash(hash)
}

// StashDropByHash drops the stash with the given hash.
func (g *GitRunner) StashDropByHash(hash string) ([]byte, error) {
	out, err := g.Run("stash", "list", "--format=%gd %H")
	if err != nil {
		return nil, err
	}
	for line := range strings.SplitSeq(string(out), "\n") {
		if strings.HasSuffix(line, hash) {
			ref := strings.Fields(line)[0]
			return g.Run("stash", "drop", ref)
		}
	}
	return nil, fmt.Errorf("stash not found: %s", hash)
}

// private methods for git command execution

func (g *GitRunner) worktreeAdd(path, branch string) ([]byte, error) {
	return g.Run("worktree", "add", path, branch)
}

func (g *GitRunner) worktreeAddWithNewBranch(branch, path string) ([]byte, error) {
	return g.Run("worktree", "add", "-b", branch, path)
}

func (g *GitRunner) worktreeListPorcelain() ([]byte, error) {
	return g.Run("worktree", "list", "--porcelain")
}

func (g *GitRunner) worktreeRemove(path string, force bool) ([]byte, error) {
	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "-f")
	}
	args = append(args, path)
	return g.Run(args...)
}

func (g *GitRunner) branchDelete(branch string, force bool) ([]byte, error) {
	flag := "-d"
	if force {
		flag = "-D"
	}
	return g.Run("branch", flag, branch)
}
