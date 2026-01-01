package gwt

import (
	"fmt"
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

// RemoveResult holds the result of a remove operation.
type RemoveResult struct {
	Branch       string
	WorktreePath string
	DryRun       bool
	GitOutput    []byte
}

// Format formats the RemoveResult for display.
func (r RemoveResult) Format(opts FormatOptions) FormatResult {
	var stdout strings.Builder

	if r.DryRun {
		stdout.WriteString(fmt.Sprintf("Would remove worktree: %s\n", r.WorktreePath))
		stdout.WriteString(fmt.Sprintf("Would delete branch: %s\n", r.Branch))
		return FormatResult{Stdout: stdout.String()}
	}

	if opts.Verbose {
		if len(r.GitOutput) > 0 {
			stdout.Write(r.GitOutput)
		}
		stdout.WriteString(fmt.Sprintf("Removed worktree and branch: %s\n", r.Branch))
	}

	stdout.WriteString(fmt.Sprintf("gwt remove: %s\n", r.Branch))

	return FormatResult{Stdout: stdout.String()}
}

// Run removes the worktree and branch for the given branch name.
// cwd is the current working directory (absolute path) passed from CLI layer.
func (c *RemoveCommand) Run(branch string, cwd string, opts RemoveOptions) (RemoveResult, error) {
	var result RemoveResult
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
