package gwt

import (
	"errors"
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

// GitOp represents the type of git operation.
type GitOp int

const (
	OpWorktreeRemove GitOp = iota + 1
	OpBranchDelete
)

func (op GitOp) String() string {
	switch op {
	case OpWorktreeRemove:
		return "remove worktree"
	case OpBranchDelete:
		return "delete branch"
	default:
		return "unknown operation"
	}
}

// GitError represents an error from a git operation with structured information.
type GitError struct {
	Op     GitOp
	Stderr string
	Err    error
}

func (e *GitError) Error() string {
	if e.Stderr != "" {
		return fmt.Sprintf("failed to %s: %v: %s", e.Op, e.Err, e.Stderr)
	}
	return fmt.Sprintf("failed to %s: %v", e.Op, e.Err)
}

func (e *GitError) Unwrap() error {
	return e.Err
}

// Hint returns a helpful hint message based on the error content.
func (e *GitError) Hint() string {
	switch {
	case strings.Contains(e.Stderr, "modified or untracked files"):
		return "use 'gwt remove --force' to force removal"
	case strings.Contains(e.Stderr, "locked working tree"):
		return "run 'git worktree unlock <path>' first, or use 'gwt remove --force'"
	default:
		return ""
	}
}

// newGitError creates a GitError from a git operation error.
// It extracts stderr from exec.ExitError if available.
func newGitError(op GitOp, err error) *GitError {
	gitErr := &GitError{
		Op:  op,
		Err: err,
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && len(exitErr.Stderr) > 0 {
		gitErr.Stderr = strings.TrimSpace(string(exitErr.Stderr))
	}
	return gitErr
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
	lock         bool
	lockReason   string
}

func (o worktreeAddOptions) lockArgs() []string {
	if !o.lock {
		return nil
	}
	if o.lockReason != "" {
		return []string{"--lock", "--reason", o.lockReason}
	}
	return []string{"--lock"}
}

// WorktreeAddOption is a functional option for WorktreeAdd.
type WorktreeAddOption func(*worktreeAddOptions)

// WithCreateBranch creates a new branch when adding the worktree.
func WithCreateBranch() WorktreeAddOption {
	return func(o *worktreeAddOptions) {
		o.createBranch = true
	}
}

// WithLock locks the worktree after creation.
func WithLock() WorktreeAddOption {
	return func(o *worktreeAddOptions) {
		o.lock = true
	}
}

// WithLockReason sets the reason for locking the worktree.
func WithLockReason(reason string) WorktreeAddOption {
	return func(o *worktreeAddOptions) {
		o.lockReason = reason
	}
}

// WorktreeAdd creates a new worktree at the specified path.
func (g *GitRunner) WorktreeAdd(path, branch string, opts ...WorktreeAddOption) ([]byte, error) {
	var o worktreeAddOptions
	for _, opt := range opts {
		opt(&o)
	}

	if o.createBranch {
		return g.worktreeAddWithNewBranch(branch, path, o)
	}
	return g.worktreeAdd(path, branch, o)
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
	// forceLevel specifies how many -f flags to pass.
	// 0 = no force, 1 = remove unclean, 2 = remove locked.
	forceLevel int
}

// WorktreeRemoveOption is a functional option for WorktreeRemove.
type WorktreeRemoveOption func(*worktreeRemoveOptions)

// WithForceRemove forces worktree removal.
// level 1: remove unclean worktrees (-f)
// level 2+: also remove locked worktrees (-f -f)
func WithForceRemove(level int) WorktreeRemoveOption {
	return func(o *worktreeRemoveOptions) {
		o.forceLevel = level
	}
}

// WorktreeRemove removes the worktree at the given path.
// By default fails if there are uncommitted changes. Use WithForceRemove() to force.
func (g *GitRunner) WorktreeRemove(path string, opts ...WorktreeRemoveOption) ([]byte, error) {
	var o worktreeRemoveOptions
	for _, opt := range opts {
		opt(&o)
	}

	out, err := g.worktreeRemove(path, o.forceLevel)
	if err != nil {
		return nil, newGitError(OpWorktreeRemove, err)
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
		return nil, newGitError(OpBranchDelete, err)
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
//
// Race condition note:
// There is a small race window between "stash push" and "rev-parse stash@{0}".
// If another process creates a stash in this window, we may get the wrong hash.
// However, this window is very small (milliseconds) and acceptable in practice.
//
// Why not use "stash create" + "stash store" pattern?
// "stash create" does not support -u/--include-untracked option (git limitation).
// It can only stash tracked file changes, not untracked files.
// See: https://git-scm.com/docs/git-stash
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

func (g *GitRunner) worktreeAdd(path, branch string, o worktreeAddOptions) ([]byte, error) {
	args := []string{"worktree", "add"}
	args = append(args, o.lockArgs()...)
	args = append(args, path, branch)
	return g.Run(args...)
}

func (g *GitRunner) worktreeAddWithNewBranch(branch, path string, o worktreeAddOptions) ([]byte, error) {
	args := []string{"worktree", "add"}
	args = append(args, o.lockArgs()...)
	args = append(args, "-b", branch, path)
	return g.Run(args...)
}

func (g *GitRunner) worktreeListPorcelain() ([]byte, error) {
	return g.Run("worktree", "list", "--porcelain")
}

func (g *GitRunner) worktreeRemove(path string, forceLevel int) ([]byte, error) {
	args := []string{"worktree", "remove"}
	// git worktree remove:
	// -f (once): remove unclean worktree
	// -f -f (twice): also remove locked worktree
	for i := 0; i < forceLevel; i++ {
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

// IsBranchMerged checks if branch is merged into target.
func (g *GitRunner) IsBranchMerged(branch, target string) (bool, error) {
	out, err := g.Run("branch", "--merged", target, "--format=%(refname:short)")
	if err != nil {
		return false, fmt.Errorf("failed to check merged branches: %w", err)
	}
	for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		if line == branch {
			return true, nil
		}
	}
	return false, nil
}

// WorktreePrune removes references to worktrees that no longer exist.
func (g *GitRunner) WorktreePrune() ([]byte, error) {
	out, err := g.Run("worktree", "prune")
	if err != nil {
		return nil, fmt.Errorf("failed to prune worktrees: %w", err)
	}
	return out, nil
}
