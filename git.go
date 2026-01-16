package twig

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
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
	GitCmdSubmodule  = "submodule"
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

// LocalBranchExists checks if a branch exists in the local repository.
func (g *GitRunner) LocalBranchExists(branch string) bool {
	_, err := g.Run(GitCmdRevParse, "--verify", RefsHeadsPrefix+branch)
	return err == nil
}

// BranchList returns all local branch names.
func (g *GitRunner) BranchList() ([]string, error) {
	output, err := g.Run(GitCmdBranch, "--format=%(refname:short)")
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
func (g *GitRunner) FindRemotesForBranch(branch string) []string {
	out, err := g.Run(GitCmdForEachRef, "--format=%(refname:short)",
		fmt.Sprintf("refs/remotes/*/%s", branch))
	if err != nil {
		return nil
	}

	var remotes []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		// Extract "origin" from "origin/branch"
		if idx := strings.Index(line, "/"); idx > 0 {
			remotes = append(remotes, line[:idx])
		}
	}
	return remotes
}

// FindRemoteForBranch finds the remote that has the specified branch.
// Returns the remote name if exactly one remote has the branch.
// Returns empty string if no remote has the branch.
// Returns error if multiple remotes have the branch (ambiguous).
func (g *GitRunner) FindRemoteForBranch(branch string) (string, error) {
	remotes := g.FindRemotesForBranch(branch)

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
func (g *GitRunner) Fetch(remote string, refspec ...string) error {
	args := []string{GitCmdFetch, remote}
	args = append(args, refspec...)
	_, err := g.Run(args...)
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
func (g *GitRunner) WorktreeList() ([]Worktree, error) {
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
func (g *GitRunner) WorktreeListBranches() ([]string, error) {
	output, err := g.worktreeListPorcelain()
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
func (g *GitRunner) WorktreeFindByBranch(branch string) (*Worktree, error) {
	worktrees, err := g.WorktreeList()
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

// ChangedFiles returns a list of files with uncommitted changes
// including staged, unstaged, and untracked files.
func (g *GitRunner) ChangedFiles() ([]string, error) {
	output, err := g.Run(GitCmdStatus, "--porcelain", "-uall")
	if err != nil {
		return nil, fmt.Errorf("failed to check git status: %w", err)
	}

	var files []string
	for _, line := range strings.Split(string(output), "\n") {
		if len(line) < 3 {
			continue
		}
		// Format: "XY filename" where XY is 2-char status
		file := strings.TrimSpace(line[2:])
		// Handle renamed files "old -> new"
		if idx := strings.Index(file, " -> "); idx != -1 {
			file = file[idx+4:]
		}
		files = append(files, file)
	}
	return files, nil
}

// HasChanges checks if there are any uncommitted changes (staged, unstaged, or untracked).
func (g *GitRunner) HasChanges() (bool, error) {
	files, err := g.ChangedFiles()
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
func (g *GitRunner) StashPush(message string, pathspecs ...string) (string, error) {
	args := []string{GitCmdStash, GitStashPush, "-u", "-m", message}
	if len(pathspecs) > 0 {
		args = append(args, "--")
		args = append(args, pathspecs...)
	}
	if _, err := g.Run(args...); err != nil {
		return "", err
	}
	out, err := g.Run(GitCmdRevParse, "stash@{0}")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// StashApplyByHash applies the stash with the given hash without dropping it.
func (g *GitRunner) StashApplyByHash(hash string) ([]byte, error) {
	return g.Run(GitCmdStash, GitStashApply, hash)
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
	out, err := g.Run(GitCmdStash, GitStashList, "--format=%gd %H")
	if err != nil {
		return nil, err
	}
	for line := range strings.SplitSeq(string(out), "\n") {
		if strings.HasSuffix(line, hash) {
			ref := strings.Fields(line)[0]
			return g.Run(GitCmdStash, GitStashDrop, ref)
		}
	}
	return nil, fmt.Errorf("stash not found: %s", hash)
}

// private methods for git command execution

func (g *GitRunner) worktreeAdd(path, branch string, o worktreeAddOptions) ([]byte, error) {
	args := []string{GitCmdWorktree, GitWorktreeAdd}
	args = append(args, o.lockArgs()...)
	args = append(args, path, branch)
	return g.Run(args...)
}

func (g *GitRunner) worktreeAddWithNewBranch(branch, path string, o worktreeAddOptions) ([]byte, error) {
	args := []string{GitCmdWorktree, GitWorktreeAdd}
	args = append(args, o.lockArgs()...)
	args = append(args, "-b", branch, path)
	return g.Run(args...)
}

func (g *GitRunner) worktreeListPorcelain() ([]byte, error) {
	return g.Run(GitCmdWorktree, GitWorktreeList, "--porcelain")
}

func (g *GitRunner) worktreeRemove(path string, forceLevel WorktreeForceLevel) ([]byte, error) {
	args := []string{GitCmdWorktree, GitWorktreeRemove}
	// git worktree remove:
	// -f (once): remove unclean worktree
	// -f -f (twice): also remove locked worktree
	for range forceLevel {
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
	return g.Run(GitCmdBranch, flag, branch)
}

// IsBranchMerged checks if branch is merged into target.
// First checks using git branch --merged (detects traditional merges).
// If not found, falls back to checking if upstream is gone (squash/rebase merges).
func (g *GitRunner) IsBranchMerged(branch, target string) (bool, error) {
	out, err := g.Run(GitCmdBranch, "--merged", target, "--format=%(refname:short)")
	if err != nil {
		return false, fmt.Errorf("failed to check merged branches: %w", err)
	}
	for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		if line == branch {
			return true, nil
		}
	}

	// Fallback: check if upstream branch is gone (deleted after merge)
	return g.IsBranchUpstreamGone(branch)
}

// IsBranchUpstreamGone checks if the branch's upstream tracking branch is gone.
// This indicates the remote branch was deleted, typically after a PR merge.
func (g *GitRunner) IsBranchUpstreamGone(branch string) (bool, error) {
	// git for-each-ref --format='%(upstream:track)' refs/heads/<branch>
	// Returns "[gone]" if upstream was deleted
	out, err := g.Run("for-each-ref", "--format=%(upstream:track)", "refs/heads/"+branch)
	if err != nil {
		return false, fmt.Errorf("failed to check upstream status: %w", err)
	}
	return strings.TrimSpace(string(out)) == "[gone]", nil
}

// WorktreePrune removes references to worktrees that no longer exist.
func (g *GitRunner) WorktreePrune() ([]byte, error) {
	out, err := g.Run(GitCmdWorktree, GitWorktreePrune)
	if err != nil {
		return nil, fmt.Errorf("failed to prune worktrees: %w", err)
	}
	return out, nil
}

// SubmoduleState represents the status prefix from git submodule status.
type SubmoduleState int

const (
	// SubmoduleStateUninitialized: prefix '-' (submodule not initialized).
	SubmoduleStateUninitialized SubmoduleState = iota
	// SubmoduleStateClean: prefix ' ' (submodule checked out at recorded commit).
	SubmoduleStateClean
	// SubmoduleStateModified: prefix '+' (submodule at different commit than recorded).
	SubmoduleStateModified
	// SubmoduleStateConflict: prefix 'U' (submodule has merge conflicts).
	SubmoduleStateConflict
)

// SubmoduleInfo holds information about a submodule from git submodule status.
type SubmoduleInfo struct {
	Path  string
	SHA   string
	State SubmoduleState
}

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

// SubmoduleStatus runs `git submodule status --recursive` and parses the output.
// Returns a list of SubmoduleInfo for all submodules.
func (g *GitRunner) SubmoduleStatus() ([]SubmoduleInfo, error) {
	out, err := g.Run(GitCmdSubmodule, "status", "--recursive")
	if err != nil {
		return nil, fmt.Errorf("failed to get submodule status: %w", err)
	}

	var submodules []SubmoduleInfo
	for _, line := range strings.Split(string(out), "\n") {
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
func (g *GitRunner) CheckSubmoduleCleanStatus() (SubmoduleCleanStatus, error) {
	submodules, err := g.SubmoduleStatus()
	if err != nil {
		return SubmoduleCleanStatusNone, err
	}

	hasInitialized := false
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
		hasChanges, err := smRunner.HasChanges()
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
