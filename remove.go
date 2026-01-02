package gwt

import (
	"fmt"
	"path/filepath"
	"strings"
)

// RemoveCommand removes git worktrees with their associated branches.
type RemoveCommand struct {
	FS     FileSystem
	Git    *GitRunner
	Config *Config
}

// RemoveOptions configures the remove operation.
type RemoveOptions struct {
	Force  bool
	DryRun bool
}

// NewRemoveCommand creates a new RemoveCommand with the given config.
func NewRemoveCommand(cfg *Config) *RemoveCommand {
	return &RemoveCommand{
		FS:     osFS{},
		Git:    NewGitRunner(cfg.WorktreeSourceDir),
		Config: cfg,
	}
}

// RemovedWorktree holds the result of a single worktree removal.
type RemovedWorktree struct {
	Branch       string
	WorktreePath string
	CleanedDirs  []string // Empty parent directories that were removed
	DryRun       bool
	GitOutput    []byte
	Err          error // nil if success
}

// RemoveResult aggregates results from remove operations.
type RemoveResult struct {
	Removed []RemovedWorktree
}

// HasErrors returns true if any errors occurred.
func (r RemoveResult) HasErrors() bool {
	for _, wt := range r.Removed {
		if wt.Err != nil {
			return true
		}
	}
	return false
}

// ErrorCount returns the number of failed removals.
func (r RemoveResult) ErrorCount() int {
	count := 0
	for _, wt := range r.Removed {
		if wt.Err != nil {
			count++
		}
	}
	return count
}

// Format formats the RemoveResult for display.
func (r RemoveResult) Format(opts FormatOptions) FormatResult {
	var stdout, stderr strings.Builder

	for _, wt := range r.Removed {
		if wt.Err != nil {
			fmt.Fprintf(&stderr, "error: %s: %v\n", wt.Branch, wt.Err)
			continue
		}
		formatted := wt.Format(opts)
		stdout.WriteString(formatted.Stdout)
		stderr.WriteString(formatted.Stderr)
	}

	return FormatResult{Stdout: stdout.String(), Stderr: stderr.String()}
}

// Format formats the RemovedWorktree for display.
func (r RemovedWorktree) Format(opts FormatOptions) FormatResult {
	var stdout strings.Builder

	if r.DryRun {
		fmt.Fprintf(&stdout, "Would remove worktree: %s\n", r.WorktreePath)
		fmt.Fprintf(&stdout, "Would delete branch: %s\n", r.Branch)
		for _, dir := range r.CleanedDirs {
			fmt.Fprintf(&stdout, "Would remove empty directory: %s\n", dir)
		}
		return FormatResult{Stdout: stdout.String()}
	}

	if opts.Verbose {
		if len(r.GitOutput) > 0 {
			stdout.Write(r.GitOutput)
		}
		fmt.Fprintf(&stdout, "Removed worktree and branch: %s\n", r.Branch)
		for _, dir := range r.CleanedDirs {
			fmt.Fprintf(&stdout, "Removed empty directory: %s\n", dir)
		}
	}

	fmt.Fprintf(&stdout, "gwt remove: %s\n", r.Branch)

	return FormatResult{Stdout: stdout.String()}
}

// Run removes the worktree and branch for the given branch name.
// cwd is the current working directory (absolute path) passed from CLI layer.
func (c *RemoveCommand) Run(branch string, cwd string, opts RemoveOptions) (RemovedWorktree, error) {
	var result RemovedWorktree
	result.Branch = branch
	result.DryRun = opts.DryRun

	if branch == "" {
		return result, fmt.Errorf("branch name is required")
	}
	if c.Config.WorktreeSourceDir == "" {
		return result, fmt.Errorf("worktree source directory is not configured")
	}

	wtPath, err := c.Git.WorktreeFindByBranch(branch)
	if err != nil {
		return result, err
	}
	result.WorktreePath = wtPath

	if strings.HasPrefix(cwd, wtPath) {
		return result, fmt.Errorf("cannot remove: current directory is inside worktree %s", wtPath)
	}

	if opts.DryRun {
		result.CleanedDirs = c.predictEmptyParentDirs(wtPath)
		return result, nil
	}

	var gitOutput []byte
	var wtOpts []WorktreeRemoveOption
	if opts.Force {
		wtOpts = append(wtOpts, WithForceRemove())
	}
	wtOut, err := c.Git.WorktreeRemove(wtPath, wtOpts...)
	if err != nil {
		return result, err
	}
	gitOutput = append(gitOutput, wtOut...)

	result.CleanedDirs = c.cleanupEmptyParentDirs(wtPath)

	var branchOpts []BranchDeleteOption
	if opts.Force {
		branchOpts = append(branchOpts, WithForceDelete())
	}
	brOut, err := c.Git.BranchDelete(branch, branchOpts...)
	if err != nil {
		return result, err
	}
	gitOutput = append(gitOutput, brOut...)

	result.GitOutput = gitOutput
	return result, nil
}

// cleanupEmptyParentDirs removes empty parent directories up to WorktreeDestBaseDir.
// Returns the list of directories that were removed. Errors are ignored since
// cleanup failures should not fail the overall remove operation.
func (c *RemoveCommand) cleanupEmptyParentDirs(wtPath string) []string {
	var cleaned []string
	baseDir := c.Config.WorktreeDestBaseDir
	if baseDir == "" {
		return cleaned
	}

	current := filepath.Dir(wtPath)
	for current != baseDir && strings.HasPrefix(current, baseDir) {
		entries, err := c.FS.ReadDir(current)
		if err != nil {
			break
		}
		if len(entries) > 0 {
			break
		}
		if err := c.FS.Remove(current); err != nil {
			break
		}
		cleaned = append(cleaned, current)
		current = filepath.Dir(current)
	}

	return cleaned
}

// predictEmptyParentDirs predicts which parent directories would become empty
// if wtPath were removed. Used for dry-run mode.
func (c *RemoveCommand) predictEmptyParentDirs(wtPath string) []string {
	var wouldClean []string
	baseDir := c.Config.WorktreeDestBaseDir
	if baseDir == "" {
		return wouldClean
	}

	// Track the path being "removed" in simulation
	removedPath := wtPath
	current := filepath.Dir(wtPath)

	for current != baseDir && strings.HasPrefix(current, baseDir) {
		entries, err := c.FS.ReadDir(current)
		if err != nil {
			break
		}
		// Check if directory would be empty after removing the simulated path
		remaining := 0
		for _, entry := range entries {
			entryPath := filepath.Join(current, entry.Name())
			if entryPath != removedPath {
				remaining++
			}
		}
		if remaining > 0 {
			break
		}
		wouldClean = append(wouldClean, current)
		removedPath = current
		current = filepath.Dir(current)
	}

	return wouldClean
}
