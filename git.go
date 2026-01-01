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

type osGitExecutor struct {
	dir string
}

func (e osGitExecutor) Run(args ...string) ([]byte, error) {
	fullArgs := append([]string{"-C", e.dir}, args...)
	return exec.Command("git", fullArgs...).Output()
}

// GitRunner provides git operations using GitExecutor.
type GitRunner struct {
	Executor GitExecutor
}

// NewGitRunner creates a new GitRunner with the default executor.
func NewGitRunner(dir string) *GitRunner {
	return &GitRunner{
		Executor: osGitExecutor{dir: dir},
	}
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
	_, err := g.Executor.Run("rev-parse", "--verify", "refs/heads/"+branch)
	return err == nil
}

// BranchList returns all local branch names.
func (g *GitRunner) BranchList() ([]string, error) {
	output, err := g.Executor.Run("branch", "--format=%(refname:short)")
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

// private methods for git command execution

func (g *GitRunner) worktreeAdd(path, branch string) ([]byte, error) {
	return g.Executor.Run("worktree", "add", path, branch)
}

func (g *GitRunner) worktreeAddWithNewBranch(branch, path string) ([]byte, error) {
	return g.Executor.Run("worktree", "add", "-b", branch, path)
}

func (g *GitRunner) worktreeListPorcelain() ([]byte, error) {
	return g.Executor.Run("worktree", "list", "--porcelain")
}

func (g *GitRunner) worktreeRemove(path string, force bool) ([]byte, error) {
	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "-f")
	}
	args = append(args, path)
	return g.Executor.Run(args...)
}

func (g *GitRunner) branchDelete(branch string, force bool) ([]byte, error) {
	flag := "-d"
	if force {
		flag = "-D"
	}
	return g.Executor.Run("branch", flag, branch)
}
