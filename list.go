package twig

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"text/tabwriter"
)

// ListCommand lists all worktrees.
type ListCommand struct {
	Git *GitRunner
	Log *slog.Logger
}

// NewListCommand creates a ListCommand with explicit dependencies (for testing).
func NewListCommand(git *GitRunner, log *slog.Logger) *ListCommand {
	if log == nil {
		log = NewNopLogger()
	}
	return &ListCommand{
		Git: git,
		Log: log,
	}
}

// NewDefaultListCommand creates a ListCommand with production defaults.
func NewDefaultListCommand(dir string, log *slog.Logger) *ListCommand {
	return NewListCommand(NewGitRunner(dir, WithLogger(log)), log)
}

// ListWorktreeInfo holds a worktree and its additional status information.
type ListWorktreeInfo struct {
	Worktree
	ChangedFiles []FileStatus
}

// ListResult holds the result of a list operation.
type ListResult struct {
	Worktrees []ListWorktreeInfo
}

// ListOptions configures the list operation.
type ListOptions struct {
	Verbose bool
}

// Format formats the ListResult for display.
func (r ListResult) Format(opts FormatOptions) FormatResult {
	if opts.Quiet {
		return r.formatQuiet()
	}
	if opts.Verbose {
		return r.formatVerbose()
	}
	return r.formatDefault()
}

// formatQuiet outputs only the worktree paths.
func (r ListResult) formatQuiet() FormatResult {
	var stdout strings.Builder
	for i := range r.Worktrees {
		stdout.WriteString(r.Worktrees[i].Path)
		stdout.WriteString("\n")
	}
	return FormatResult{Stdout: stdout.String()}
}

// formatDefault outputs git worktree list compatible format.
func (r ListResult) formatDefault() FormatResult {
	var buf bytes.Buffer
	w := tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)

	for i := range r.Worktrees {
		wt := &r.Worktrees[i]
		fmt.Fprintf(w, "%s\t%s %s\n", wt.Path, wt.ShortHEAD(), wt.formatStatus())
	}
	w.Flush()

	return FormatResult{Stdout: buf.String()}
}

// formatVerbose outputs worktrees with changed files and lock reasons.
func (r ListResult) formatVerbose() FormatResult {
	// Pass 1: generate aligned main lines with tabwriter
	var buf bytes.Buffer
	w := tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)
	for i := range r.Worktrees {
		wt := &r.Worktrees[i]
		fmt.Fprintf(w, "%s\t%s %s\n", wt.Path, wt.ShortHEAD(), wt.formatStatus())
	}
	w.Flush()

	// Pass 2: interleave detail lines after each main line
	mainLines := strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
	var stdout strings.Builder
	for i := range r.Worktrees {
		wt := &r.Worktrees[i]
		if i < len(mainLines) {
			stdout.WriteString(mainLines[i])
			stdout.WriteString("\n")
		}
		// Lock reason
		if wt.Locked && wt.LockReason != "" {
			fmt.Fprintf(&stdout, "  lock reason: %s\n", wt.LockReason)
		}
		// Changed files
		for _, f := range wt.ChangedFiles {
			fmt.Fprintf(&stdout, "  %s %s\n", f.Status, f.Path)
		}
	}

	return FormatResult{Stdout: stdout.String()}
}

// formatStatus returns the status portion of the worktree line (branch, locked, prunable).
func (w Worktree) formatStatus() string {
	var sb strings.Builder

	switch {
	case w.Bare:
		sb.WriteString("(bare)")
	case w.Detached:
		sb.WriteString("(detached HEAD)")
	default:
		sb.WriteString("[")
		sb.WriteString(w.Branch)
		sb.WriteString("]")
	}

	if w.Locked {
		sb.WriteString(" locked")
	}
	if w.Prunable {
		sb.WriteString(" prunable")
	}

	return sb.String()
}

// Run lists all worktrees.
func (c *ListCommand) Run(ctx context.Context, opts ListOptions) (ListResult, error) {
	c.Log.DebugContext(ctx, "run started",
		LogAttrKeyCategory.String(), LogCategoryList)

	worktrees, err := c.Git.WorktreeList(ctx)
	if err != nil {
		return ListResult{}, err
	}

	c.Log.DebugContext(ctx, "worktrees listed",
		LogAttrKeyCategory.String(), LogCategoryList,
		"count", len(worktrees))

	infos := make([]ListWorktreeInfo, len(worktrees))
	for i, wt := range worktrees {
		infos[i] = ListWorktreeInfo{Worktree: wt}
	}

	// Fetch changed files in verbose mode
	if opts.Verbose {
		type indexedFiles struct {
			index int
			files []FileStatus
		}

		var (
			wg      sync.WaitGroup
			mu      sync.Mutex
			results []indexedFiles
		)

		for i, wt := range worktrees {
			// Skip bare and prunable worktrees (no working tree)
			if wt.Bare || wt.Prunable {
				continue
			}

			wg.Add(1)
			go func(idx int, wt Worktree) {
				defer wg.Done()

				c.Log.DebugContext(ctx, "fetching changed files",
					LogAttrKeyCategory.String(), LogCategoryList,
					"path", wt.Path)

				files, err := c.Git.InDir(wt.Path).ChangedFiles(ctx)
				if err != nil {
					c.Log.DebugContext(ctx, "failed to fetch changed files",
						LogAttrKeyCategory.String(), LogCategoryList,
						"path", wt.Path,
						"error", err.Error())
					return
				}

				if len(files) > 0 {
					mu.Lock()
					results = append(results, indexedFiles{index: idx, files: files})
					mu.Unlock()
				}
			}(i, wt)
		}

		wg.Wait()

		for _, r := range results {
			infos[r.index].ChangedFiles = r.files
		}
	}

	c.Log.DebugContext(ctx, "run completed",
		LogAttrKeyCategory.String(), LogCategoryList,
		"count", len(infos))

	return ListResult{Worktrees: infos}, nil
}
