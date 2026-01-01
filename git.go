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

func (osGitExecutor) Run(args ...string) ([]byte, error) {
	return exec.Command("git", args...).Output()
}

// GitRunner provides git operations using GitExecutor.
type GitRunner struct {
	Executor GitExecutor
}

// NewGitRunner creates a new GitRunner with the default executor.
func NewGitRunner() *GitRunner {
	return &GitRunner{Executor: osGitExecutor{}}
}

// WorktreeAdd creates a new worktree at the specified path.
// If createBranch is true, creates a new branch with the given name.
func (g *GitRunner) WorktreeAdd(path, branch string, createBranch bool) error {
	var output []byte
	var err error
	if createBranch {
		output, err = g.Executor.Run("worktree", "add", "-b", branch, path)
	} else {
		output, err = g.Executor.Run("worktree", "add", path, branch)
	}
	if len(output) > 0 {
		fmt.Print(string(output))
	}
	return err
}

// BranchExists checks if a branch exists in the local repository.
func (g *GitRunner) BranchExists(branch string) bool {
	_, err := g.Executor.Run("rev-parse", "--verify", "refs/heads/"+branch)
	return err == nil
}

// WorktreeListBranches returns a list of branch names currently checked out in worktrees.
func (g *GitRunner) WorktreeListBranches() ([]string, error) {
	output, err := g.Executor.Run("worktree", "list", "--porcelain")
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
