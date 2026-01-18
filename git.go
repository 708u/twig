package twig

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"path/filepath"
	"strings"
)

// GitExecutor abstracts git command execution for testability.
// Commands are fixed to "git" - only subcommands and args are passed.
type GitExecutor interface {
	// Run executes git with args and returns stdout.
	Run(ctx context.Context, args ...string) ([]byte, error)
}

type osGitExecutor struct{}

func (e osGitExecutor) Run(ctx context.Context, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, "git", args...).Output()
}

// GitOp represents the type of git operation.
type GitOp int

const (
	OpWorktreeRemove GitOp = iota + 1
	OpBranchDelete
)

// Git command names.
const (
	GitCmdWorktree   = "worktree"
	GitCmdBranch     = "branch"
	GitCmdStash      = "stash"
	GitCmdStatus     = "status"
	GitCmdRevParse   = "rev-parse"
	GitCmdDiff       = "diff"
	GitCmdFetch      = "fetch"
	GitCmdForEachRef = "for-each-ref"
)

// Git worktree subcommands.
const (
	GitWorktreeAdd    = "add"
	GitWorktreeRemove = "remove"
	GitWorktreeList   = "list"
	GitWorktreePrune  = "prune"
)

// Git stash subcommands.
const (
	GitStashPush  = "push"
	GitStashApply = "apply"
	GitStashDrop  = "drop"
	GitStashList  = "list"
)

// Porcelain output format prefixes and values.
const (
	PorcelainWorktreePrefix = "worktree "
	PorcelainHEADPrefix     = "HEAD "
	PorcelainBranchPrefix   = "branch refs/heads/"
	PorcelainDetached       = "detached"
	PorcelainBare           = "bare"
	PorcelainLocked         = "locked"
	PorcelainPrunable       = "prunable"
)

// RefsHeadsPrefix is the git refs prefix for local branches.
const RefsHeadsPrefix = "refs/heads/"

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
	Log      *slog.Logger
}

type gitRunnerOptions struct {
	log *slog.Logger
}

// GitRunnerOption configures GitRunner.
type GitRunnerOption func(*gitRunnerOptions)

// WithLogger sets the logger for GitRunner.
func WithLogger(log *slog.Logger) GitRunnerOption {
	return func(o *gitRunnerOptions) {
		o.log = log
	}
}

// NewGitRunner creates a new GitRunner with the default executor.
func NewGitRunner(dir string, opts ...GitRunnerOption) *GitRunner {
	o := &gitRunnerOptions{
		log: NewNopLogger(),
	}
	for _, opt := range opts {
		opt(o)
	}
	return &GitRunner{
		Executor: osGitExecutor{},
		Dir:      dir,
		Log:      o.log,
	}
}

// InDir returns a GitRunner that executes commands in the specified directory.
func (g *GitRunner) InDir(dir string) *GitRunner {
	return &GitRunner{Executor: g.Executor, Dir: dir, Log: g.Log}
}

// Run executes git command with -C flag.
func (g *GitRunner) Run(ctx context.Context, args ...string) ([]byte, error) {
	fullArgs := append([]string{"-C", g.Dir}, args...)
	g.Log.Debug(strings.Join(append([]string{"git"}, fullArgs...), " "), "category", LogCategoryGit)
	return g.Executor.Run(ctx, fullArgs...)
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
func (g *GitRunner) WorktreeAdd(ctx context.Context, path, branch string, opts ...WorktreeAddOption) ([]byte, error) {
	var o worktreeAddOptions
	for _, opt := range opts {
		opt(&o)
	}

	if o.createBranch {
		return g.worktreeAddWithNewBranch(ctx, branch, path, o)
	}
	return g.worktreeAdd(ctx, path, branch, o)
}

// LocalBranchExists checks if a branch exists in the local repository.
func (g *GitRunner) LocalBranchExists(ctx context.Context, branch string) (bool, error) {
	_, err := g.Run(ctx, GitCmdRevParse, "--verify", RefsHeadsPrefix+branch)
	if err != nil {
		// git rev-parse returns exit code 128 for non-existent refs
		var exitErr interface{ ExitCode() int }
		if errors.As(err, &exitErr) {
			// ExitError means git ran but ref doesn't exist
			return false, nil
		}
		// Other errors (context canceled, etc.)
		return false, err
	}
	return true, nil
}

// BranchList returns all local branch names.
func (g *GitRunner) BranchList(ctx context.Context) ([]string, error) {
	output, err := g.Run(ctx, GitCmdBranch, "--format=%(refname:short)")
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

// FindRemotesForBranch returns all remotes that have the specified branch
// in local remote-tracking branches.
// This checks refs/remotes/*/<branch> locally without network access.
func (g *GitRunner) FindRemotesForBranch(ctx context.Context, branch string) ([]string, error) {
	out, err := g.Run(ctx, GitCmdForEachRef, "--format=%(refname:short)",
		fmt.Sprintf("refs/remotes/*/%s", branch))
	if err != nil {
		return nil, err
	}

	remotes := []string{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		// Extract "origin" from "origin/branch"
		if idx := strings.Index(line, "/"); idx > 0 {
			remotes = append(remotes, line[:idx])
		}
	}
	return remotes, nil
}

// FindRemoteForBranch finds the remote that has the specified branch.
// Returns the remote name if exactly one remote has the branch.
// Returns empty string if no remote has the branch.
// Returns error if multiple remotes have the branch (ambiguous).
func (g *GitRunner) FindRemoteForBranch(ctx context.Context, branch string) (string, error) {
	remotes, err := g.FindRemotesForBranch(ctx, branch)
	if err != nil {
		return "", err
	}

	switch len(remotes) {
	case 0:
		return "", nil
	case 1:
		return remotes[0], nil
	default:
		return "", fmt.Errorf("branch %q exists on multiple remotes: %v", branch, remotes)
	}
}

// Fetch fetches the specified refspec from the remote.
func (g *GitRunner) Fetch(ctx context.Context, remote string, refspec ...string) error {
	args := []string{GitCmdFetch, remote}
	args = append(args, refspec...)
	_, err := g.Run(ctx, args...)
	return err
}

// Worktree holds worktree path and branch information.
type Worktree struct {
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
func (w Worktree) ShortHEAD() string {
	if len(w.HEAD) >= 7 {
		return w.HEAD[:7]
	}
	return w.HEAD
}

// WorktreeList returns all worktrees with their paths and branches.
func (g *GitRunner) WorktreeList(ctx context.Context) ([]Worktree, error) {
	out, err := g.worktreeListPorcelain(ctx)
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

	var worktrees []Worktree
	var current Worktree
	for _, line := range strings.Split(string(out), "\n") {
		switch {
		case strings.HasPrefix(line, PorcelainWorktreePrefix):
			current = Worktree{Path: strings.TrimPrefix(line, PorcelainWorktreePrefix)}
		case strings.HasPrefix(line, PorcelainHEADPrefix):
			current.HEAD = strings.TrimPrefix(line, PorcelainHEADPrefix)
		case strings.HasPrefix(line, PorcelainBranchPrefix):
			current.Branch = strings.TrimPrefix(line, PorcelainBranchPrefix)
		case line == PorcelainDetached:
			current.Detached = true
		case line == PorcelainBare:
			current.Bare = true
		case strings.HasPrefix(line, PorcelainLocked):
			current.Locked = true
			if reason, ok := strings.CutPrefix(line, PorcelainLocked+" "); ok {
				current.LockReason = reason
			}
		case strings.HasPrefix(line, PorcelainPrunable):
			current.Prunable = true
			if reason, ok := strings.CutPrefix(line, PorcelainPrunable+" "); ok {
				current.PrunableReason = reason
			}
		case line == "" && current.Path != "":
			worktrees = append(worktrees, current)
			current = Worktree{}
		}
	}
	return worktrees, nil
}

// WorktreeListBranches returns a list of branch names currently checked out in worktrees.
func (g *GitRunner) WorktreeListBranches(ctx context.Context) ([]string, error) {
	output, err := g.worktreeListPorcelain(ctx)
	if err != nil {
		return nil, err
	}

	var branches []string
	for line := range strings.SplitSeq(string(output), "\n") {
		if branch, ok := strings.CutPrefix(line, PorcelainBranchPrefix); ok {
			branches = append(branches, branch)
		}
	}
	return branches, nil
}

// WorktreeFindByBranch returns the Worktree for the given branch.
// Returns an error if the branch is not checked out in any worktree.
func (g *GitRunner) WorktreeFindByBranch(ctx context.Context, branch string) (*Worktree, error) {
	worktrees, err := g.WorktreeList(ctx)
	if err != nil {
		return nil, err
	}

	for i := range worktrees {
		if worktrees[i].Branch == branch {
			return &worktrees[i], nil
		}
	}

	return nil, fmt.Errorf("branch %q is not checked out in any worktree", branch)
}

// WorktreeForceLevel represents the force level for worktree removal.
// Matches git worktree remove behavior.
type WorktreeForceLevel uint8

const (
	// WorktreeForceLevelNone means no force - fail on uncommitted changes or locked.
	WorktreeForceLevelNone WorktreeForceLevel = iota
	// WorktreeForceLevelUnclean removes unclean worktrees (-f).
	WorktreeForceLevelUnclean
	// WorktreeForceLevelLocked also removes locked worktrees (-f -f).
	WorktreeForceLevelLocked
)

type worktreeRemoveOptions struct {
	forceLevel WorktreeForceLevel
}

// WorktreeRemoveOption is a functional option for WorktreeRemove.
type WorktreeRemoveOption func(*worktreeRemoveOptions)

// WithForceRemove forces worktree removal.
func WithForceRemove(level WorktreeForceLevel) WorktreeRemoveOption {
	return func(o *worktreeRemoveOptions) {
		o.forceLevel = level
	}
}

// WorktreeRemove removes the worktree at the given path.
// By default fails if there are uncommitted changes. Use WithForceRemove() to force.
func (g *GitRunner) WorktreeRemove(ctx context.Context, path string, opts ...WorktreeRemoveOption) ([]byte, error) {
	var o worktreeRemoveOptions
	for _, opt := range opts {
		opt(&o)
	}

	out, err := g.worktreeRemove(ctx, path, o.forceLevel)
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
func (g *GitRunner) BranchDelete(ctx context.Context, branch string, opts ...BranchDeleteOption) ([]byte, error) {
	var o branchDeleteOptions
	for _, opt := range opts {
		opt(&o)
	}

	out, err := g.branchDelete(ctx, branch, o.force)
	if err != nil {
		return nil, newGitError(OpBranchDelete, err)
	}
	return out, nil
}

// FileStatus represents a file with its git status.
type FileStatus struct {
	Status string // e.g., " M", "A ", "??"
	Path   string
}

// ChangedFiles returns files with uncommitted changes including staged,
// unstaged, and untracked files. Status codes are the first 2 characters
// from git status --porcelain output.
func (g *GitRunner) ChangedFiles(ctx context.Context) ([]FileStatus, error) {
	output, err := g.Run(ctx, GitCmdStatus, "--porcelain", "-uall")
	if err != nil {
		return nil, fmt.Errorf("failed to check git status: %w", err)
	}

	var files []FileStatus
	for _, line := range strings.Split(string(output), "\n") {
		if len(line) < 3 {
			continue
		}
		// Format: "XY filename" where XY is 2-char status
		status := line[:2]
		path := strings.TrimSpace(line[2:])
		// Handle renamed files "old -> new"
		if idx := strings.Index(path, " -> "); idx != -1 {
			path = path[idx+4:]
		}
		files = append(files, FileStatus{Status: status, Path: path})
	}
	return files, nil
}

// HasChanges checks if there are any uncommitted changes (staged, unstaged, or untracked).
func (g *GitRunner) HasChanges(ctx context.Context) (bool, error) {
	files, err := g.ChangedFiles(ctx)
	if err != nil {
		return false, err
	}
	return len(files) > 0, nil
}

// StashPush stashes changes including untracked files.
// If pathspecs are provided, only matching files are stashed.
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
func (g *GitRunner) StashPush(ctx context.Context, message string, pathspecs ...string) (string, error) {
	args := []string{GitCmdStash, GitStashPush, "-u", "-m", message}
	if len(pathspecs) > 0 {
		args = append(args, "--")
		args = append(args, pathspecs...)
	}
	if _, err := g.Run(ctx, args...); err != nil {
		return "", err
	}
	out, err := g.Run(ctx, GitCmdRevParse, "stash@{0}")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// StashApplyByHash applies the stash with the given hash without dropping it.
func (g *GitRunner) StashApplyByHash(ctx context.Context, hash string) ([]byte, error) {
	return g.Run(ctx, GitCmdStash, GitStashApply, hash)
}

// StashPopByHash applies and drops the stash with the given hash.
func (g *GitRunner) StashPopByHash(ctx context.Context, hash string) ([]byte, error) {
	if _, err := g.StashApplyByHash(ctx, hash); err != nil {
		return nil, err
	}
	return g.StashDropByHash(ctx, hash)
}

// StashDropByHash drops the stash with the given hash.
func (g *GitRunner) StashDropByHash(ctx context.Context, hash string) ([]byte, error) {
	out, err := g.Run(ctx, GitCmdStash, GitStashList, "--format=%gd %H")
	if err != nil {
		return nil, err
	}
	for line := range strings.SplitSeq(string(out), "\n") {
		if strings.HasSuffix(line, hash) {
			ref := strings.Fields(line)[0]
			return g.Run(ctx, GitCmdStash, GitStashDrop, ref)
		}
	}
	return nil, fmt.Errorf("stash not found: %s", hash)
}

// private methods for git command execution

func (g *GitRunner) worktreeAdd(ctx context.Context, path, branch string, o worktreeAddOptions) ([]byte, error) {
	args := []string{GitCmdWorktree, GitWorktreeAdd}
	args = append(args, o.lockArgs()...)
	args = append(args, path, branch)
	return g.Run(ctx, args...)
}

func (g *GitRunner) worktreeAddWithNewBranch(ctx context.Context, branch, path string, o worktreeAddOptions) ([]byte, error) {
	args := []string{GitCmdWorktree, GitWorktreeAdd}
	args = append(args, o.lockArgs()...)
	args = append(args, "-b", branch, path)
	return g.Run(ctx, args...)
}

func (g *GitRunner) worktreeListPorcelain(ctx context.Context) ([]byte, error) {
	return g.Run(ctx, GitCmdWorktree, GitWorktreeList, "--porcelain")
}

func (g *GitRunner) worktreeRemove(ctx context.Context, path string, forceLevel WorktreeForceLevel) ([]byte, error) {
	args := []string{GitCmdWorktree, GitWorktreeRemove}
	// git worktree remove:
	// -f (once): remove unclean worktree
	// -f -f (twice): also remove locked worktree
	for range forceLevel {
		args = append(args, "-f")
	}
	args = append(args, path)
	return g.Run(ctx, args...)
}

func (g *GitRunner) branchDelete(ctx context.Context, branch string, force bool) ([]byte, error) {
	flag := "-d"
	if force {
		flag = "-D"
	}
	return g.Run(ctx, GitCmdBranch, flag, branch)
}

// IsBranchMerged checks if branch is merged into target.
func (g *GitRunner) IsBranchMerged(ctx context.Context, branch, target string) (bool, error) {
	merged, err := g.MergedBranches(ctx, target)
	if err != nil {
		return false, err
	}
	return merged[branch], nil
}

// MergedBranches returns a map of branch names that are considered merged.
// A branch is merged if it's in `git branch --merged <target>` or if its upstream is gone.
// This is more efficient than calling IsBranchMerged for each branch individually.
func (g *GitRunner) MergedBranches(ctx context.Context, target string) (map[string]bool, error) {
	result := make(map[string]bool)

	// Get traditionally merged branches
	out, err := g.Run(ctx, GitCmdBranch, "--merged", target, "--format=%(refname:short)")
	if err != nil {
		return nil, fmt.Errorf("failed to check merged branches: %w", err)
	}
	for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		if line != "" {
			result[line] = true
		}
	}

	// Get branches with gone upstream (squash/rebase merges)
	// Format: "branch-name [gone]" or "branch-name" (no upstream or tracking)
	upstreamOut, err := g.Run(ctx, GitCmdForEachRef, "--format=%(refname:short) %(upstream:track)", "refs/heads/")
	if err != nil {
		// Non-fatal: return what we have from --merged
		return result, nil
	}
	for line := range strings.SplitSeq(strings.TrimSpace(string(upstreamOut)), "\n") {
		if branch, found := strings.CutSuffix(line, " [gone]"); found {
			result[branch] = true
		}
	}

	return result, nil
}

// IsBranchUpstreamGone checks if the branch's upstream tracking branch is gone.
// This indicates the remote branch was deleted, typically after a PR merge.
func (g *GitRunner) IsBranchUpstreamGone(ctx context.Context, branch string) (bool, error) {
	// git for-each-ref --format='%(upstream:track)' refs/heads/<branch>
	// Returns "[gone]" if upstream was deleted
	out, err := g.Run(ctx, "for-each-ref", "--format=%(upstream:track)", "refs/heads/"+branch)
	if err != nil {
		return false, fmt.Errorf("failed to check upstream status: %w", err)
	}
	return strings.TrimSpace(string(out)) == "[gone]", nil
}

// WorktreePrune removes references to worktrees that no longer exist.
func (g *GitRunner) WorktreePrune(ctx context.Context) ([]byte, error) {
	out, err := g.Run(ctx, GitCmdWorktree, GitWorktreePrune)
	if err != nil {
		return nil, fmt.Errorf("failed to prune worktrees: %w", err)
	}
	return out, nil
}

// GitCmdSubmodule is the git submodule command.
const GitCmdSubmodule = "submodule"

// Git submodule subcommands.
const (
	GitSubmoduleUpdate = "update"
)

// SubmoduleCleanStatus indicates whether it's safe to remove a worktree with submodules.
type SubmoduleCleanStatus int

const (
	// SubmoduleCleanStatusNone: no initialized submodules exist.
	SubmoduleCleanStatusNone SubmoduleCleanStatus = iota
	// SubmoduleCleanStatusClean: submodules exist but all are clean.
	SubmoduleCleanStatusClean
	// SubmoduleCleanStatusDirty: submodules have uncommitted changes or are at different commits.
	SubmoduleCleanStatusDirty
)

// SubmoduleState represents the state of a submodule.
type SubmoduleState int

const (
	SubmoduleStateUninitialized SubmoduleState = iota
	SubmoduleStateClean
	SubmoduleStateModified
	SubmoduleStateConflict
)

// SubmoduleInfo holds information about a submodule.
type SubmoduleInfo struct {
	SHA   string
	Path  string
	State SubmoduleState
}

// SubmoduleStatus runs `git submodule status --recursive` and parses the output.
// Returns a list of SubmoduleInfo for all submodules.
func (g *GitRunner) SubmoduleStatus(ctx context.Context) ([]SubmoduleInfo, error) {
	out, err := g.Run(ctx, GitCmdSubmodule, "status", "--recursive")
	if err != nil {
		return nil, fmt.Errorf("failed to get submodule status: %w", err)
	}

	var submodules []SubmoduleInfo
	for line := range strings.SplitSeq(string(out), "\n") {
		if len(line) == 0 {
			continue
		}

		// Format: " SHA path (desc)" or "+SHA path (desc)" or "-SHA path (desc)" or "USHA path (desc)"
		// The first character is the state prefix
		var state SubmoduleState
		switch line[0] {
		case '-':
			state = SubmoduleStateUninitialized
		case ' ':
			state = SubmoduleStateClean
		case '+':
			state = SubmoduleStateModified
		case 'U':
			state = SubmoduleStateConflict
		default:
			// Unknown prefix, skip
			continue
		}

		// Parse SHA and path
		rest := strings.TrimSpace(line[1:])
		fields := strings.Fields(rest)
		if len(fields) < 2 {
			continue
		}

		submodules = append(submodules, SubmoduleInfo{
			SHA:   fields[0],
			Path:  fields[1],
			State: state,
		})
	}

	return submodules, nil
}

// CheckSubmoduleCleanStatus determines if it's safe to remove a worktree with submodules.
// Returns:
//   - SubmoduleCleanStatusNone: no initialized submodules
//   - SubmoduleCleanStatusClean: submodules exist but are clean (safe to auto-force)
//   - SubmoduleCleanStatusDirty: submodules have changes (requires user --force)
func (g *GitRunner) CheckSubmoduleCleanStatus(ctx context.Context) (SubmoduleCleanStatus, error) {
	submodules, err := g.SubmoduleStatus(ctx)
	if err != nil {
		return SubmoduleCleanStatusNone, err
	}

	var hasInitialized bool
	for _, sm := range submodules {
		if sm.State == SubmoduleStateUninitialized {
			continue
		}
		hasInitialized = true

		// Check for modified commit (+ prefix) or conflict (U prefix)
		if sm.State == SubmoduleStateModified || sm.State == SubmoduleStateConflict {
			return SubmoduleCleanStatusDirty, nil
		}

		// Check for uncommitted changes within the submodule
		// sm.Path is relative to the worktree, so we need to join it with g.Dir
		smAbsPath := filepath.Join(g.Dir, sm.Path)
		smRunner := g.InDir(smAbsPath)
		hasChanges, err := smRunner.HasChanges(ctx)
		if err != nil {
			// If we can't check, assume dirty for safety
			return SubmoduleCleanStatusDirty, nil
		}
		if hasChanges {
			return SubmoduleCleanStatusDirty, nil
		}
	}

	if !hasInitialized {
		return SubmoduleCleanStatusNone, nil
	}
	return SubmoduleCleanStatusClean, nil
}

// SubmoduleUpdate runs git submodule update --init --recursive.
// Returns the number of initialized submodules.
func (g *GitRunner) SubmoduleUpdate(ctx context.Context) (int, error) {
	args := []string{GitCmdSubmodule, GitSubmoduleUpdate, "--init", "--recursive"}

	_, err := g.Run(ctx, args...)
	if err != nil {
		return 0, fmt.Errorf("failed to initialize submodules: %w", err)
	}

	// Count initialized submodules
	submodules, err := g.SubmoduleStatus(ctx)
	if err != nil {
		return 0, nil // Initialization succeeded, but count failed
	}

	var count int
	for _, sm := range submodules {
		if sm.State != SubmoduleStateUninitialized {
			count++
		}
	}
	return count, nil
}
