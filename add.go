package twig

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

const lfsFetchExcludeKey = "lfs.fetchexclude"

// Older git enables extensions.worktreeConfig in git sparse-checkout set
// without moving core.bare and core.worktree to the main worktree's config,
// which breaks bare-repository layouts.
const (
	minSparseGitMajor = 2
	minSparseGitMinor = 36
)

// Version from which git worktree add copies the source worktree's
// config.worktree.
const (
	minConfigCopyGitMajor = 2
	minConfigCopyGitMinor = 36
)

// AddCommand creates git worktrees with symlinks.
type AddCommand struct {
	FS                 FileSystem
	Git                *GitRunner
	Config             *Config
	Log                *slog.Logger
	Sync               bool
	CarryFrom          string
	FilePatterns       []string
	Lock               bool
	LockReason         string
	InitSubmodules     bool
	SubmoduleReference bool
	LFSFetchExclude    []string
	NoLFSFetchExclude  bool
	SparseCheckout     []string
	NoSparseCheckout   bool
}

// AddOptions holds options for the add command.
type AddOptions struct {
	Sync               bool
	CarryFrom          string   // empty: no carry, non-empty: resolved path to carry from
	FilePatterns       []string // file patterns to carry (empty means all files)
	Lock               bool
	LockReason         string
	InitSubmodules     bool
	SubmoduleReference bool
	LFSFetchExclude    []string // non-empty: replaces lfs_fetch_exclude for this run
	NoLFSFetchExclude  bool     // disable lfs_fetch_exclude and reset an inherited one
	SparseCheckout     []string // non-empty: replaces sparse_checkout for this run
	NoSparseCheckout   bool     // disable sparse_checkout and reset an inherited one
}

// NewAddCommand creates an AddCommand with explicit dependencies (for testing).
func NewAddCommand(fs FileSystem, git *GitRunner, cfg *Config, log *slog.Logger, opts AddOptions) *AddCommand {
	if log == nil {
		log = NewNopLogger()
	}
	return &AddCommand{
		FS:                 fs,
		Git:                git,
		Config:             cfg,
		Log:                log,
		Sync:               opts.Sync,
		CarryFrom:          opts.CarryFrom,
		FilePatterns:       opts.FilePatterns,
		Lock:               opts.Lock,
		LockReason:         opts.LockReason,
		InitSubmodules:     opts.InitSubmodules,
		SubmoduleReference: opts.SubmoduleReference,
		LFSFetchExclude:    opts.LFSFetchExclude,
		NoLFSFetchExclude:  opts.NoLFSFetchExclude,
		SparseCheckout:     opts.SparseCheckout,
		NoSparseCheckout:   opts.NoSparseCheckout,
	}
}

// NewDefaultAddCommand creates an AddCommand with production defaults.
func NewDefaultAddCommand(cfg *Config, log *slog.Logger, opts AddOptions) *AddCommand {
	return NewAddCommand(osFS{}, NewGitRunner(cfg.WorktreeSourceDir, WithLogger(log)), cfg, log, opts)
}

// SymlinkResult holds information about a symlink operation.
type SymlinkResult struct {
	Src     string
	Dst     string
	Skipped bool
	Reason  string
}

// SubmoduleInitResult holds information about submodule initialization.
type SubmoduleInitResult struct {
	Attempted             bool     // true if initialization was attempted
	Count                 int      // number of initialized submodules
	Skipped               bool     // true if initialization failed
	Reason                string   // reason for failure (warning message)
	NoReferenceSubmodules []string // submodules that couldn't use reference
}

// HookResult holds the result of a single hook execution.
type HookResult struct {
	Command string
	Output  []byte
	Err     error
}

// AddResult holds the result of an add operation.
type AddResult struct {
	Branch          string
	WorktreePath    string
	Symlinks        []SymlinkResult
	GitOutput       []byte
	ChangesSynced   bool
	ChangesCarried  bool
	SubmoduleInit   SubmoduleInitResult
	HookResults     []HookResult
	LFSFetchExclude []string
	// LFSFetchExcludeInitialOnly is true when the exclusion covers only the
	// first checkout because extensions.worktreeConfig is not enabled.
	LFSFetchExcludeInitialOnly bool
	SparseCheckout             []string
}

// AddFormatOptions configures add output formatting.
type AddFormatOptions struct {
	Verbose bool
	Quiet   bool
}

// Format formats the AddResult for display.
func (r AddResult) Format(opts AddFormatOptions) FormatResult {
	if opts.Quiet {
		return r.formatQuiet()
	}
	return r.formatDefault(opts)
}

// formatQuiet outputs only the worktree path.
func (r AddResult) formatQuiet() FormatResult {
	return FormatResult{Stdout: r.WorktreePath + "\n"}
}

// formatDefault outputs the default or verbose format.
func (r AddResult) formatDefault(opts AddFormatOptions) FormatResult {
	var stdout, stderr strings.Builder

	var createdCount int
	for _, s := range r.Symlinks {
		if s.Skipped {
			fmt.Fprintf(&stderr, "warning: %s\n", s.Reason)
		} else {
			createdCount++
		}
	}

	// Output submodule init warning
	if r.SubmoduleInit.Skipped {
		fmt.Fprintf(&stderr, "warning: %s\n", r.SubmoduleInit.Reason)
	}

	// Output warning for submodules that couldn't use reference
	for _, sm := range r.SubmoduleInit.NoReferenceSubmodules {
		fmt.Fprintf(&stderr, "warning: submodule %s: reference not available, initialize in main worktree first\n", sm)
	}

	// Output hook results (single pass: warnings to stderr, count successes)
	var hookRanCount int
	for _, h := range r.HookResults {
		if h.Err != nil {
			fmt.Fprintf(&stderr, "warning: hook %q failed: %v\n", h.Command, h.Err)
			if len(h.Output) > 0 {
				stderr.Write(h.Output)
			}
		} else {
			hookRanCount++
		}
	}

	if opts.Verbose {
		if len(r.GitOutput) > 0 {
			stdout.Write(r.GitOutput)
		}
		fmt.Fprintf(&stdout, "Created worktree at %s\n", r.WorktreePath)
		for _, s := range r.Symlinks {
			if !s.Skipped {
				fmt.Fprintf(&stdout, "Created symlink: %s -> %s\n", s.Dst, s.Src)
			}
		}
		if r.ChangesSynced {
			stdout.WriteString("Synced uncommitted changes\n")
		}
		if r.ChangesCarried {
			stdout.WriteString("Carried uncommitted changes (source is now clean)\n")
		}
		if len(r.LFSFetchExclude) > 0 {
			fmt.Fprintf(&stdout, "Excluded LFS files: %s\n", strings.Join(r.LFSFetchExclude, ", "))
			if r.LFSFetchExcludeInitialOnly {
				stdout.WriteString("LFS exclusion covers the initial checkout only: extensions.worktreeConfig is not enabled\n")
			}
		}
		if len(r.SparseCheckout) > 0 {
			fmt.Fprintf(&stdout, "Applied sparse checkout: %s\n", strings.Join(r.SparseCheckout, ", "))
		}
		if r.SubmoduleInit.Attempted && r.SubmoduleInit.Count > 0 {
			fmt.Fprintf(&stdout, "Initialized %d submodule(s)\n", r.SubmoduleInit.Count)
		}
		for _, h := range r.HookResults {
			if h.Err == nil {
				fmt.Fprintf(&stdout, "Ran hook: %s\n", h.Command)
				if len(h.Output) > 0 {
					stdout.Write(h.Output)
				}
			}
		}
	}

	var syncInfo string
	if r.ChangesSynced {
		syncInfo = ", synced"
	} else if r.ChangesCarried {
		syncInfo = ", carried"
	}

	var submoduleInfo string
	if r.SubmoduleInit.Attempted && r.SubmoduleInit.Count > 0 {
		submoduleInfo = fmt.Sprintf(", %d submodules", r.SubmoduleInit.Count)
	}

	var checkoutInfo string
	if len(r.LFSFetchExclude) > 0 {
		checkoutInfo += fmt.Sprintf(", %d lfs excludes", len(r.LFSFetchExclude))
	}
	if len(r.SparseCheckout) > 0 {
		checkoutInfo += fmt.Sprintf(", %d sparse patterns", len(r.SparseCheckout))
	}

	var hookInfo string
	if hookRanCount > 0 {
		hookInfo = fmt.Sprintf(", %d hooks ran", hookRanCount)
	}
	fmt.Fprintf(&stdout, "twig add: %s (%d symlinks%s%s%s%s)\n", r.Branch, createdCount, syncInfo, submoduleInfo, checkoutInfo, hookInfo)

	return FormatResult{Stdout: stdout.String(), Stderr: stderr.String()}
}

// Run creates a new worktree for the given branch name.
func (c *AddCommand) Run(ctx context.Context, name string) (AddResult, error) {
	var result AddResult
	result.Branch = name

	if name == "" {
		return result, fmt.Errorf("branch name is required")
	}

	if c.Config.WorktreeSourceDir == "" {
		return result, fmt.Errorf("worktree source directory is not configured")
	}
	if c.Config.WorktreeDestBaseDir == "" {
		return result, fmt.Errorf("worktree destination base directory is not configured")
	}

	wtPath := filepath.Join(c.Config.WorktreeDestBaseDir, name)
	result.WorktreePath = wtPath

	lfsFetchExclude := overridePatterns(c.Config.LFSFetchExclude, c.LFSFetchExclude, c.NoLFSFetchExclude)
	sparseCheckout := overridePatterns(c.Config.SparseCheckout, c.SparseCheckout, c.NoSparseCheckout)
	for _, p := range lfsFetchExclude {
		// git-lfs splits lfs.fetchexclude on commas
		if strings.Contains(p, ",") {
			return result, fmt.Errorf("lfs_fetch_exclude pattern must not contain a comma: %q", p)
		}
	}
	if len(sparseCheckout) > 0 {
		ok, found, err := c.gitAtLeast(ctx, minSparseGitMajor, minSparseGitMinor)
		if err != nil {
			return result, err
		}
		if !ok {
			return result, fmt.Errorf("sparse_checkout requires git %d.%d or later (found %s)",
				minSparseGitMajor, minSparseGitMinor, found)
		}
	}

	plan := checkoutPlan{sparseCheckout: sparseCheckout}
	if len(lfsFetchExclude) > 0 {
		plan.lfsFetchExclude = strings.Join(lfsFetchExclude, ",")
	}
	if c.NoLFSFetchExclude || c.NoSparseCheckout || plan.lfsFetchExclude != "" {
		var err error
		plan.worktreeConfig, err = c.Git.WorktreeConfigEnabled(ctx)
		if err != nil {
			return result, fmt.Errorf("failed to read extensions.worktreeConfig: %w", err)
		}
	}
	// git worktree add copies the source worktree's config.worktree into the
	// new worktree. That inheritance is kept unless a --no-* flag turns the
	// setting off.
	if (c.NoLFSFetchExclude || c.NoSparseCheckout) && plan.worktreeConfig {
		inherited, err := c.inheritedCheckoutConfig(ctx)
		if err != nil {
			return result, err
		}
		plan.resetSparse = c.NoSparseCheckout && inherited.SparseCheckout
		plan.resetLFS = c.NoLFSFetchExclude && inherited.LFSFetchExclude != ""
	}

	// Runs worktree add and the first checkout. -c reaches the git-lfs smudge
	// filter, so excluded LFS files are written as pointers.
	checkoutGit := c.Git
	if plan.lfsFetchExclude != "" {
		checkoutGit = c.Git.WithConfig(lfsFetchExcludeKey, plan.lfsFetchExclude)
	}

	// Determine stash mode and source
	var stashMsg string
	var isCarry bool
	var stashSourceGit *GitRunner
	if c.Sync {
		stashMsg = "twig sync"
		stashSourceGit = c.Git
	}
	if c.CarryFrom != "" {
		stashMsg = "twig carry"
		isCarry = true
		stashSourceGit = c.Git.InDir(c.CarryFrom)
	}

	// Stash changes if sync or carry is enabled
	var stashHash string
	if stashMsg != "" {
		hasChanges, err := stashSourceGit.HasChanges(ctx)
		if err != nil {
			return result, fmt.Errorf("failed to check for changes: %w", err)
		}
		if hasChanges {
			var pathspecs []string
			if len(c.FilePatterns) > 0 {
				// Expand glob patterns to actual file paths using doublestar
				globDir := c.Config.WorktreeSourceDir
				if isCarry {
					globDir = c.CarryFrom
				}
				seen := make(map[string]bool)
				for _, pattern := range c.FilePatterns {
					matches, err := c.FS.Glob(globDir, pattern)
					if err != nil {
						return result, fmt.Errorf("invalid glob pattern %q: %w", pattern, err)
					}
					for _, match := range matches {
						if !seen[match] {
							seen[match] = true
							pathspecs = append(pathspecs, match)
						}
					}
				}
			}
			hash, err := stashSourceGit.StashPush(ctx, stashMsg, pathspecs...)
			if err != nil {
				return result, fmt.Errorf("failed to stash changes: %w", err)
			}
			stashHash = hash
		}
	}

	gitOutput, branchCreated, err := c.createWorktree(ctx, checkoutGit, name, wtPath, plan.noCheckout())
	if err != nil {
		if stashHash != "" {
			_, _ = stashSourceGit.StashPopByHash(ctx, stashHash)
		}
		return result, err
	}
	result.GitOutput = gitOutput

	if plan.active() {
		err = c.applyCheckoutSettings(ctx, checkoutGit.InDir(wtPath), plan)
		if err != nil {
			rbErr := c.removeCreatedWorktree(ctx, wtPath, name, branchCreated)
			if stashHash != "" {
				_, _ = stashSourceGit.StashPopByHash(ctx, stashHash)
			}
			return result, rollbackError(err, rbErr)
		}
		result.LFSFetchExclude = lfsFetchExclude
		result.LFSFetchExcludeInitialOnly = plan.lfsFetchExclude != "" && !plan.persistLFS()
		result.SparseCheckout = sparseCheckout
	}

	// Initialize submodules in new worktree (CLI flag forces enable)
	if c.InitSubmodules || c.Config.ShouldInitSubmodules() {
		wtGit := c.Git.InDir(wtPath)
		var opts []SubmoduleUpdateOption

		if c.SubmoduleReference || c.Config.ShouldUseSubmoduleReference() {
			if mainPath, err := c.Git.MainWorktreePath(ctx); err == nil {
				opts = append(opts, WithSubmoduleReference(mainPath))
			}
		}

		subResult, subErr := wtGit.SubmoduleUpdate(ctx, opts...)
		if subErr != nil {
			result.SubmoduleInit.Attempted = true
			result.SubmoduleInit.Skipped = true
			result.SubmoduleInit.Reason = subErr.Error()
		} else if subResult.Count > 0 {
			result.SubmoduleInit.Attempted = true
			result.SubmoduleInit.Count = subResult.Count
			result.SubmoduleInit.NoReferenceSubmodules = subResult.NoReference
		}
	}

	// Apply stashed changes to new worktree
	if stashHash != "" {
		// An lfs.fetchexclude in the new worktree's config, set by twig or
		// copied from the source worktree, would smudge carried LFS edits
		// as pointers. The empty -c value lifts it for this command only.
		_, err = c.Git.WithConfig(lfsFetchExcludeKey, "").InDir(wtPath).StashApplyByHash(ctx, stashHash)
		if err != nil {
			rbErr := c.removeCreatedWorktree(ctx, wtPath, name, branchCreated)
			_, _ = stashSourceGit.StashPopByHash(ctx, stashHash)
			return result, rollbackError(fmt.Errorf("failed to apply changes to new worktree: %w", err), rbErr)
		}
		if isCarry {
			// Carry: drop stash (source becomes clean)
			_, _ = stashSourceGit.StashDropByHash(ctx, stashHash)
			result.ChangesCarried = true
		} else {
			// Sync: restore stash in source (both have changes)
			_, err = stashSourceGit.StashPopByHash(ctx, stashHash)
			if err != nil {
				return result, fmt.Errorf("failed to restore changes in source: %w", err)
			}
			result.ChangesSynced = true
		}
	}

	symlinks, err := createSymlinks(c.FS, c.Config.WorktreeSourceDir, wtPath, c.Config.Symlinks)
	if err != nil {
		return result, err
	}
	result.Symlinks = symlinks

	// Run post-create hooks
	if len(c.Config.Hooks) > 0 {
		result.HookResults = c.runHooks(ctx, wtPath)
	}

	return result, nil
}

func (c *AddCommand) runHooks(ctx context.Context, dir string) []HookResult {
	var results []HookResult
	for _, hook := range c.Config.Hooks {
		c.Log.DebugContext(ctx, "running hook", "command", hook, "dir", dir)
		cmd := exec.CommandContext(ctx, "sh", "-c", hook)
		cmd.Dir = dir
		output, err := cmd.CombinedOutput()
		results = append(results, HookResult{
			Command: hook,
			Output:  output,
			Err:     err,
		})
		if err != nil {
			c.Log.WarnContext(ctx, "hook failed", "command", hook, "error", err)
			break
		}
		c.Log.DebugContext(ctx, "hook completed", "command", hook)
	}
	return results
}

// checkoutPlan describes how the new worktree's checkout differs from a
// plain git worktree add.
type checkoutPlan struct {
	sparseCheckout  []string
	lfsFetchExclude string // comma-joined lfs_fetch_exclude
	worktreeConfig  bool   // extensions.worktreeConfig before the worktree is added
	resetSparse     bool   // sparse checkout inherited from the source worktree
	resetLFS        bool   // lfs.fetchexclude inherited from the source worktree
}

func (p checkoutPlan) active() bool {
	return len(p.sparseCheckout) > 0 || p.lfsFetchExclude != "" || p.resetSparse || p.resetLFS
}

// persistLFS reports whether git config --worktree can keep lfs.fetchexclude.
// git sparse-checkout set enables extensions.worktreeConfig before it is set.
func (p checkoutPlan) persistLFS() bool {
	return p.worktreeConfig || len(p.sparseCheckout) > 0
}

// noCheckout reports whether the worktree must be populated only after its
// config.worktree is adjusted.
func (p checkoutPlan) noCheckout() bool {
	return len(p.sparseCheckout) > 0 || p.resetSparse || p.resetLFS
}

// applyCheckoutSettings adjusts the new worktree's sparse checkout and
// lfs.fetchexclude, then populates it if it was created with --no-checkout.
func (c *AddCommand) applyCheckoutSettings(ctx context.Context, wtGit *GitRunner, p checkoutPlan) error {
	if len(p.sparseCheckout) > 0 {
		if _, err := wtGit.SparseCheckoutSet(ctx, p.sparseCheckout); err != nil {
			return fmt.Errorf("failed to set sparse checkout: %w", withGitStderr(err))
		}
	} else if p.resetSparse {
		if _, err := wtGit.SparseCheckoutDisable(ctx); err != nil {
			return fmt.Errorf("failed to disable sparse checkout: %w", withGitStderr(err))
		}
	}

	// Without extensions.worktreeConfig, git config --worktree fails once a
	// linked worktree exists, and twig does not enable it because that
	// rewrites repository-wide config.
	switch {
	case p.lfsFetchExclude != "" && p.persistLFS():
		if _, err := wtGit.SetWorktreeConfig(ctx, lfsFetchExcludeKey, p.lfsFetchExclude); err != nil {
			return fmt.Errorf("failed to set %s: %w", lfsFetchExcludeKey, withGitStderr(err))
		}
	case p.lfsFetchExclude != "":
		c.Log.DebugContext(ctx, "skip worktree config: extensions.worktreeConfig is not enabled",
			"key", lfsFetchExcludeKey)
	case p.resetLFS:
		if _, err := wtGit.UnsetWorktreeConfig(ctx, lfsFetchExcludeKey); err != nil {
			return fmt.Errorf("failed to unset %s: %w", lfsFetchExcludeKey, withGitStderr(err))
		}
	}

	if p.noCheckout() {
		if _, err := wtGit.CheckoutHEAD(ctx); err != nil {
			return fmt.Errorf("failed to check out worktree: %w", withGitStderr(err))
		}
	}
	return nil
}

// createWorktree adds the worktree and reports whether its local branch did
// not exist before, either created with -b or tracking a remote branch.
func (c *AddCommand) createWorktree(ctx context.Context, addGit *GitRunner, branch, path string, noCheckout bool) (output []byte, branchCreated bool, err error) {
	if _, err := c.FS.Stat(path); err == nil {
		return nil, false, fmt.Errorf("directory already exists: %s", path)
	}

	var opts []WorktreeAddOption
	exists, err := c.Git.LocalBranchExists(ctx, branch)
	if err != nil {
		return nil, false, fmt.Errorf("failed to check branch existence: %w", err)
	}
	if exists {
		var branches []string
		branches, err = c.Git.WorktreeListBranches(ctx)
		if err != nil {
			return nil, false, fmt.Errorf("failed to list worktree branches: %w", err)
		}
		if slices.Contains(branches, branch) {
			return nil, false, fmt.Errorf("branch %s is already checked out in another worktree", branch)
		}
	} else {
		var remote string
		remote, err = c.Git.FindRemoteForBranch(ctx, branch)
		if err != nil {
			return nil, false, err
		}

		if remote != "" {
			// Remote branch found, fetch it
			err = c.Git.Fetch(ctx, remote, branch)
			if err != nil {
				return nil, false, fmt.Errorf("failed to fetch %s from %s: %w", branch, remote, err)
			}
			// After fetch, git worktree add will auto-track the remote branch
		} else {
			// No remote branch found, create new local branch
			opts = append(opts, WithCreateBranch())
		}
	}

	if c.Lock {
		opts = append(opts, WithLock())
		if c.LockReason != "" {
			opts = append(opts, WithLockReason(c.LockReason))
		}
	}

	if noCheckout {
		opts = append(opts, WithNoCheckout())
	}

	output, err = addGit.WorktreeAdd(ctx, path, branch, opts...)
	if err != nil {
		return nil, false, fmt.Errorf("failed to create worktree: %w", err)
	}

	return output, !exists, nil
}

// overridePatterns resolves a pattern setting for one run: disable wins, then
// patterns given on the command line replace the configured ones.
func overridePatterns(configured, override []string, disable bool) []string {
	switch {
	case disable:
		return nil
	case len(override) > 0:
		return override
	default:
		return configured
	}
}

// removeCreatedWorktree undoes createWorktree after a later step fails.
// --lock may have locked the worktree, so removal uses -f -f.
func (c *AddCommand) removeCreatedWorktree(ctx context.Context, path, branch string, branchCreated bool) error {
	if _, err := c.Git.WorktreeRemove(ctx, path, WithForceRemove(WorktreeForceLevelLocked)); err != nil {
		return fmt.Errorf("failed to remove worktree %s: %w", path, err)
	}
	removeEmptyParentDirs(c.FS, c.Config.WorktreeDestBaseDir, path)
	if branchCreated {
		if _, err := c.Git.BranchDelete(ctx, branch, WithForceDelete()); err != nil {
			return fmt.Errorf("failed to delete branch %s: %w", branch, err)
		}
	}
	return nil
}

// rollbackError reports err together with a failed rollback, which leaves
// the worktree or branch behind.
func rollbackError(err, rollbackErr error) error {
	if rollbackErr == nil {
		return err
	}
	return fmt.Errorf("%w (rollback failed: %v)", err, rollbackErr)
}

// gitAtLeast reports whether git is major.minor or later, along with the
// version found.
func (c *AddCommand) gitAtLeast(ctx context.Context, major, minor int) (bool, string, error) {
	gotMajor, gotMinor, err := c.Git.Version(ctx)
	if err != nil {
		return false, "", fmt.Errorf("failed to check git version: %w", err)
	}
	ok := gotMajor > major || (gotMajor == major && gotMinor >= minor)
	return ok, fmt.Sprintf("%d.%d", gotMajor, gotMinor), nil
}

// inheritedCheckoutConfig returns the checkout settings git worktree add will
// copy from the source worktree. Older git copies nothing.
func (c *AddCommand) inheritedCheckoutConfig(ctx context.Context) (WorktreeCheckoutConfig, error) {
	copies, _, err := c.gitAtLeast(ctx, minConfigCopyGitMajor, minConfigCopyGitMinor)
	if err != nil || !copies {
		return WorktreeCheckoutConfig{}, err
	}
	inherited, err := c.Git.WorktreeCheckoutConfig(ctx)
	if err != nil {
		return WorktreeCheckoutConfig{}, fmt.Errorf("failed to read worktree config: %w", err)
	}
	return inherited, nil
}
