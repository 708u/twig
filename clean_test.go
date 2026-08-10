package twig

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/708u/twig/internal/testutil"
)

// countingExecutor wraps a GitExecutor and counts calls per git subcommand.
type countingExecutor struct {
	inner  GitExecutor
	mu     sync.Mutex
	counts map[string]int
	total  int
}

func newCountingExecutor(inner GitExecutor) *countingExecutor {
	return &countingExecutor{inner: inner, counts: map[string]int{}}
}

func (e *countingExecutor) Run(ctx context.Context, args ...string) ([]byte, error) {
	e.mu.Lock()
	e.total++
	e.counts[gitSubcommandKey(args)]++
	e.mu.Unlock()
	return e.inner.Run(ctx, args...)
}

// gitSubcommandKey derives a stable key from git args, ignoring the -C <dir>
// prefix. Subcommands whose cost depends on the second token (e.g. branch
// --merged vs branch -d) are keyed on both tokens.
func gitSubcommandKey(args []string) string {
	for len(args) >= 2 && args[0] == "-C" {
		args = args[2:]
	}
	if len(args) == 0 {
		return ""
	}
	switch args[0] {
	case "worktree", "submodule", "stash":
		if len(args) >= 2 {
			return args[0] + " " + args[1]
		}
	case "branch":
		if len(args) >= 2 && (args[1] == "--merged" || args[1] == "-d" || args[1] == "-D") {
			return "branch " + args[1]
		}
	case "rev-parse":
		// The cwd worktree-root lookup is the rev-parse variant whose count the
		// clean path optimizes, so key it separately from other rev-parse uses.
		if len(args) >= 2 && args[1] == "--show-toplevel" {
			return "rev-parse --show-toplevel"
		}
	}
	return args[0]
}

// TestCleanCommand_Run_ReusesChecks verifies the removal phase reuses the
// check phase results instead of re-running per-branch git queries.
func TestCleanCommand_Run_ReusesChecks(t *testing.T) {
	t.Parallel()

	mockGit := &testutil.MockGitExecutor{
		Worktrees: []testutil.MockWorktree{
			{Path: "/repo/main", Branch: "main"},
			{Path: "/repo/feat/a", Branch: "feat/a"},
			{Path: "/repo/feat/b", Branch: "feat/b"},
		},
		MergedBranches: map[string][]string{
			"main": {"main", "feat/a"},
		},
		UpstreamGoneBranches: []string{"feat/b"},
	}
	counter := newCountingExecutor(mockGit)

	cmd := &CleanCommand{
		// .gitmodules present so the submodule status query is exercised; this
		// test asserts it runs once per candidate and is reused during removal.
		FS: &testutil.MockFS{ExistingPaths: []string{
			"/repo/feat/a/.gitmodules",
			"/repo/feat/b/.gitmodules",
		}},
		Git:    &GitRunner{Executor: counter, Log: NewNopLogger()},
		Config: &Config{WorktreeSourceDir: "/repo/main", DefaultSource: "main"},
		Log:    NewNopLogger(),
	}

	result, err := cmd.Run(t.Context(), "/other/dir", CleanOptions{Yes: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.CleanableCount() != 2 {
		t.Fatalf("expected 2 cleanable, got %d", result.CleanableCount())
	}

	// branch --merged and the repo-wide for-each-ref classification run once
	// for the whole command, not once per candidate.
	if got := counter.counts["branch --merged"]; got != 1 {
		t.Errorf("branch --merged called %d times, want 1", got)
	}
	if got := counter.counts["for-each-ref"]; got != 1 {
		t.Errorf("for-each-ref called %d times, want 1", got)
	}
	// Submodule status is checked once per candidate in the check phase and
	// reused during removal, not re-fetched.
	if got := counter.counts["submodule status"]; got != 2 {
		t.Errorf("submodule status called %d times, want 2", got)
	}
	// Removal reuses the check result: only worktree remove + branch delete run.
	if got := counter.counts["worktree remove"]; got != 2 {
		t.Errorf("worktree remove called %d times, want 2", got)
	}
	if got := counter.counts["branch -d"]; got != 1 {
		t.Errorf("branch -d (merged) called %d times, want 1", got)
	}
	if got := counter.counts["branch -D"]; got != 1 {
		t.Errorf("branch -D (upstream gone) called %d times, want 1", got)
	}
}

// TestCleanCommand_Run_MinimizesGitSpawns verifies the per-command git spawns
// that the check phase reduces: no submodule status query when .gitmodules is
// absent, a single cwd worktree-root lookup, and a single worktree list.
func TestCleanCommand_Run_MinimizesGitSpawns(t *testing.T) {
	t.Parallel()

	newMock := func() *testutil.MockGitExecutor {
		return &testutil.MockGitExecutor{
			Worktrees: []testutil.MockWorktree{
				{Path: "/repo/main", Branch: "main"},
				{Path: "/repo/feat/a", Branch: "feat/a"},
				{Path: "/repo/feat/b", Branch: "feat/b"},
				{Path: "/repo/feat/c", Branch: "feat/c"},
			},
			MergedBranches: map[string][]string{
				"main": {"main", "feat/a"},
			},
			UpstreamGoneBranches: []string{"feat/b"},
		}
	}

	newCmd := func(counter *countingExecutor) *CleanCommand {
		return &CleanCommand{
			// No .gitmodules paths: repos without submodules must not spawn
			// `git submodule status`.
			FS:     &testutil.MockFS{},
			Git:    &GitRunner{Executor: counter, Log: NewNopLogger()},
			Config: &Config{WorktreeSourceDir: "/repo/main", DefaultSource: "main"},
			Log:    NewNopLogger(),
		}
	}

	assertCounts := func(t *testing.T, counter *countingExecutor) {
		t.Helper()
		if got := counter.counts["submodule status"]; got != 0 {
			t.Errorf("submodule status called %d times, want 0 (no .gitmodules)", got)
		}
		if got := counter.counts["rev-parse --show-toplevel"]; got != 1 {
			t.Errorf("rev-parse --show-toplevel called %d times, want 1", got)
		}
		if got := counter.counts["worktree list"]; got != 1 {
			t.Errorf("worktree list called %d times, want 1", got)
		}
	}

	t.Run("check mode", func(t *testing.T) {
		t.Parallel()

		counter := newCountingExecutor(newMock())
		cmd := newCmd(counter)

		if _, err := cmd.Run(t.Context(), "/other/dir", CleanOptions{Check: true}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertCounts(t, counter)
	})

	t.Run("execute mode", func(t *testing.T) {
		t.Parallel()

		counter := newCountingExecutor(newMock())
		cmd := newCmd(counter)

		if _, err := cmd.Run(t.Context(), "/other/dir", CleanOptions{Yes: true}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertCounts(t, counter)
	})
}

// TestCleanCommand_Run_SquashMerged verifies that a branch whose content was
// squashed into the target is cleaned, that a branch without an equivalent in
// the target is kept, and that the probe runs only for branches the cheap
// classification leaves undecided.
func TestCleanCommand_Run_SquashMerged(t *testing.T) {
	t.Parallel()

	newMock := func() *testutil.MockGitExecutor {
		return &testutil.MockGitExecutor{
			Worktrees: []testutil.MockWorktree{
				{Path: "/repo/main", Branch: "main", HEAD: "commit-main"},
				{Path: "/repo/feat/merged", Branch: "feat/merged", HEAD: "commit-merged"},
				{Path: "/repo/feat/squashed", Branch: "feat/squashed", HEAD: "commit-squashed"},
				{Path: "/repo/feat/wip", Branch: "feat/wip", HEAD: "commit-wip"},
			},
			MergedBranches: map[string][]string{
				"main": {"main", "feat/merged"},
			},
			SquashMergedBranches: []string{"feat/squashed"},
		}
	}

	newCmd := func(exec GitExecutor) *CleanCommand {
		return &CleanCommand{
			FS:     &testutil.MockFS{},
			Git:    &GitRunner{Executor: exec, Log: NewNopLogger()},
			Config: &Config{WorktreeSourceDir: "/repo/main", DefaultSource: "main"},
			Log:    NewNopLogger(),
		}
	}

	t.Run("detects squash merged branch", func(t *testing.T) {
		t.Parallel()

		counter := newCountingExecutor(newMock())
		cmd := newCmd(counter)

		result, err := cmd.Run(t.Context(), "/other/dir", CleanOptions{Check: true})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		byBranch := make(map[string]CleanCandidate, len(result.Candidates))
		for _, c := range result.Candidates {
			byBranch[c.Branch] = c
		}

		squashed := byBranch["feat/squashed"]
		if squashed.Skipped {
			t.Errorf("feat/squashed should be cleanable, skipped with %q", squashed.SkipReason)
		}
		if squashed.CleanReason != CleanSquashMerged {
			t.Errorf("feat/squashed clean reason = %q, want %q", squashed.CleanReason, CleanSquashMerged)
		}

		wip := byBranch["feat/wip"]
		if !wip.Skipped {
			t.Error("feat/wip should be skipped")
		}
		if wip.SkipReason != SkipNotMerged {
			t.Errorf("feat/wip skip reason = %q, want %q", wip.SkipReason, SkipNotMerged)
		}

		merged := byBranch["feat/merged"]
		if merged.CleanReason != CleanMerged {
			t.Errorf("feat/merged clean reason = %q, want %q", merged.CleanReason, CleanMerged)
		}

		// Only the two branches the pre-fetched classification leaves undecided
		// are probed; the merged branch is answered without git spawns.
		if got := counter.counts["merge-base"]; got != 2 {
			t.Errorf("merge-base called %d times, want 2", got)
		}
		if got := counter.counts["commit-tree"]; got != 2 {
			t.Errorf("commit-tree called %d times, want 2", got)
		}
	})

	t.Run("deletes squash merged branch with force", func(t *testing.T) {
		t.Parallel()

		mock := newMock()
		counter := newCountingExecutor(mock)
		cmd := newCmd(counter)

		result, err := cmd.Run(t.Context(), "/other/dir", CleanOptions{Yes: true})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.CleanableCount() != 2 {
			t.Fatalf("cleanable count = %d, want 2", result.CleanableCount())
		}
		// A squashed branch has no commit in common with the target, so -d
		// would refuse to delete it.
		if got := counter.counts["branch -D"]; got != 1 {
			t.Errorf("branch -D called %d times, want 1", got)
		}
		if got := counter.counts["branch -d"]; got != 1 {
			t.Errorf("branch -d called %d times, want 1", got)
		}
	})

	t.Run("skips probe when force bypasses merge check", func(t *testing.T) {
		t.Parallel()

		counter := newCountingExecutor(newMock())
		cmd := newCmd(counter)

		if _, err := cmd.Run(t.Context(), "/other/dir", CleanOptions{
			Check: true,
			Force: WorktreeForceLevelUnclean,
		}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if got := counter.counts["merge-base"]; got != 0 {
			t.Errorf("merge-base called %d times, want 0", got)
		}
	})
}

// TestCleanCommand_Run_SquashMergedWithChanges verifies that a squash merged
// branch holding uncommitted changes is reported with its clean reason so
// --stale can act on it.
func TestCleanCommand_Run_SquashMergedWithChanges(t *testing.T) {
	t.Parallel()

	mockGit := &testutil.MockGitExecutor{
		Worktrees: []testutil.MockWorktree{
			{Path: "/repo/main", Branch: "main", HEAD: "commit-main"},
			{Path: "/repo/feat/squashed", Branch: "feat/squashed", HEAD: "commit-squashed"},
		},
		MergedBranches: map[string][]string{
			"main": {"main"},
		},
		SquashMergedBranches: []string{"feat/squashed"},
		StatusOutputMap: map[string]string{
			"/repo/feat/squashed": " M main.go\n",
		},
		// The branch carries its own commits, so it is not on the first-parent
		// lineage of the target and the WIP protection does not apply.
		FirstParentAncestors: map[string][]string{
			"main": {"commit-main"},
		},
	}

	cmd := &CleanCommand{
		FS:     &testutil.MockFS{},
		Git:    &GitRunner{Executor: mockGit, Log: NewNopLogger()},
		Config: &Config{WorktreeSourceDir: "/repo/main", DefaultSource: "main"},
		Log:    NewNopLogger(),
	}

	result, err := cmd.Run(t.Context(), "/other/dir", CleanOptions{Check: true, Stale: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Candidates) != 1 {
		t.Fatalf("got %d candidates, want 1", len(result.Candidates))
	}

	cand := result.Candidates[0]
	if cand.CleanReason != CleanSquashMerged {
		t.Errorf("clean reason = %q, want %q", cand.CleanReason, CleanSquashMerged)
	}
	if cand.Skipped {
		t.Errorf("--stale should clean the branch, skipped with %q", cand.SkipReason)
	}
	if !cand.StaleOverride {
		t.Error("stale override should be applied")
	}
}

func TestCleanResult_CleanableCount(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		result CleanResult
		want   int
	}{
		{
			name:   "empty",
			result: CleanResult{},
			want:   0,
		},
		{
			name: "all_cleanable",
			result: CleanResult{
				Candidates: []CleanCandidate{
					{Branch: "feat/a", Skipped: false},
					{Branch: "feat/b", Skipped: false},
				},
			},
			want: 2,
		},
		{
			name: "all_skipped",
			result: CleanResult{
				Candidates: []CleanCandidate{
					{Branch: "feat/a", Skipped: true},
					{Branch: "feat/b", Skipped: true},
				},
			},
			want: 0,
		},
		{
			name: "mixed",
			result: CleanResult{
				Candidates: []CleanCandidate{
					{Branch: "feat/a", Skipped: false},
					{Branch: "feat/b", Skipped: true},
					{Branch: "feat/c", Skipped: false},
				},
			},
			want: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.result.CleanableCount(); got != tt.want {
				t.Errorf("CleanableCount() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCleanResult_Format(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		result     CleanResult
		opts       FormatOptions
		wantStdout string
		wantStderr string
	}{
		{
			name: "check_with_candidates",
			result: CleanResult{
				Candidates: []CleanCandidate{
					{Branch: "feat/a", Skipped: false, CleanReason: CleanMerged},
					{Branch: "feat/b", Skipped: true, SkipReason: SkipNotMerged},
				},
				Check: true,
			},
			opts:       FormatOptions{},
			wantStdout: "clean:\n  feat/a (merged)\n",
			wantStderr: "",
		},
		{
			name: "check_verbose_shows_skipped",
			result: CleanResult{
				Candidates: []CleanCandidate{
					{Branch: "feat/a", Skipped: false, CleanReason: CleanMerged},
					{Branch: "feat/b", Skipped: true, SkipReason: SkipNotMerged},
				},
				Check: true,
			},
			opts:       FormatOptions{Verbose: true},
			wantStdout: "clean:\n  feat/a (merged)\n\nskip:\n  feat/b\n    ✗ not merged\n",
			wantStderr: "",
		},
		{
			name: "no_candidates",
			result: CleanResult{
				Candidates: []CleanCandidate{},
				Check:      true,
			},
			opts:       FormatOptions{},
			wantStdout: "No worktrees to clean\n",
			wantStderr: "",
		},
		{
			name: "all_skipped",
			result: CleanResult{
				Candidates: []CleanCandidate{
					{Branch: "feat/a", Skipped: true, SkipReason: SkipLocked},
				},
				Check: true,
			},
			opts:       FormatOptions{},
			wantStdout: "No worktrees to clean\n",
			wantStderr: "",
		},
		{
			name: "execution_results",
			result: CleanResult{
				Removed: []RemovedWorktree{
					{Branch: "feat/a"},
					{Branch: "feat/b"},
				},
				Check: false,
			},
			opts:       FormatOptions{},
			wantStdout: "",
			wantStderr: "",
		},
		{
			name: "execution_results_verbose",
			result: CleanResult{
				Removed: []RemovedWorktree{
					{Branch: "feat/a"},
					{Branch: "feat/b"},
				},
				Check: false,
			},
			opts:       FormatOptions{Verbose: true},
			wantStdout: "Removed worktree and branch: feat/a\nRemoved worktree and branch: feat/b\n",
			wantStderr: "",
		},
		// Prunable branch tests
		{
			name: "prunable_only",
			result: CleanResult{
				Candidates: []CleanCandidate{
					{Branch: "feat/prunable", Prunable: true, Skipped: false, CleanReason: CleanMerged},
				},
				Check: true,
			},
			opts:       FormatOptions{},
			wantStdout: "clean:\n  feat/prunable (prunable, merged)\n",
			wantStderr: "",
		},
		{
			name: "clean_and_prunable",
			result: CleanResult{
				Candidates: []CleanCandidate{
					{Branch: "feat/a", Skipped: false, CleanReason: CleanMerged},
					{Branch: "feat/prunable", Prunable: true, Skipped: false, CleanReason: CleanUpstreamGone},
				},
				Check: true,
			},
			opts:       FormatOptions{},
			wantStdout: "clean:\n  feat/a (merged)\n  feat/prunable (prunable, upstream gone)\n",
			wantStderr: "",
		},
		{
			name: "clean_prunable_and_skipped_verbose",
			result: CleanResult{
				Candidates: []CleanCandidate{
					{Branch: "feat/a", Skipped: false, CleanReason: CleanMerged},
					{Branch: "feat/prunable", Prunable: true, Skipped: false, CleanReason: CleanMerged},
					{Branch: "feat/wip", Skipped: true, SkipReason: SkipNotMerged},
				},
				Check: true,
			},
			opts:       FormatOptions{Verbose: true},
			wantStdout: "clean:\n  feat/a (merged)\n  feat/prunable (prunable, merged)\n\nskip:\n  feat/wip\n    ✗ not merged\n",
			wantStderr: "",
		},
		{
			name: "prunable_skipped_shows_no_worktrees",
			result: CleanResult{
				Candidates: []CleanCandidate{
					{Branch: "feat/prunable", Prunable: true, Skipped: true, SkipReason: SkipNotMerged},
				},
				Check: true,
			},
			opts:       FormatOptions{},
			wantStdout: "No worktrees to clean\n",
			wantStderr: "",
		},
		// Verbose with changed files tests
		{
			name: "verbose_with_changed_files",
			result: CleanResult{
				Candidates: []CleanCandidate{
					{Branch: "feat/a", Skipped: false, CleanReason: CleanMerged},
					{
						Branch:      "feat/wip",
						Skipped:     true,
						SkipReason:  SkipHasChanges,
						CleanReason: CleanMerged, // merged but has uncommitted changes
						ChangedFiles: []FileStatus{
							{Status: " M", Path: "src/main.go"},
							{Status: "??", Path: "tmp/debug.log"},
						},
					},
				},
				Check: true,
			},
			opts:       FormatOptions{Verbose: true},
			wantStdout: "clean:\n  feat/a (merged)\n\nskip:\n  feat/wip\n    ✓ merged\n    ✗ has uncommitted changes\n       M src/main.go\n      ?? tmp/debug.log\n",
			wantStderr: "",
		},
		{
			name: "verbose_with_dirty_submodule_changed_files",
			result: CleanResult{
				Candidates: []CleanCandidate{
					{
						Branch:      "feat/submod",
						Skipped:     true,
						SkipReason:  SkipDirtySubmodule,
						CleanReason: CleanMerged, // merged but dirty submodule
						ChangedFiles: []FileStatus{
							{Status: " M", Path: "submodule/file.go"},
						},
					},
				},
				Check: true,
			},
			opts:       FormatOptions{Verbose: true},
			wantStdout: "skip:\n  feat/submod\n    ✓ merged\n    ✗ submodule has uncommitted changes\n       M submodule/file.go\n\nNo worktrees to clean\n",
			wantStderr: "",
		},
		{
			name: "verbose_skip_reason_without_changed_files",
			result: CleanResult{
				Candidates: []CleanCandidate{
					{Branch: "feat/a", Skipped: false, CleanReason: CleanMerged},
					{Branch: "feat/locked", Skipped: true, SkipReason: SkipLocked, CleanReason: CleanMerged},
				},
				Check: true,
			},
			opts:       FormatOptions{Verbose: true},
			wantStdout: "clean:\n  feat/a (merged)\n\nskip:\n  feat/locked\n    ✓ merged\n    ✗ locked\n",
			wantStderr: "",
		},
		// Skip without CleanReason (merge-related skip reasons)
		{
			name: "verbose_skip_not_merged_no_clean_reason",
			result: CleanResult{
				Candidates: []CleanCandidate{
					{Branch: "feat/a", Skipped: false, CleanReason: CleanMerged},
					{Branch: "feat/wip", Skipped: true, SkipReason: SkipNotMerged},
				},
				Check: true,
			},
			opts:       FormatOptions{Verbose: true},
			wantStdout: "clean:\n  feat/a (merged)\n\nskip:\n  feat/wip\n    ✗ not merged\n",
			wantStderr: "",
		},
		{
			name: "verbose_skip_upstream_gone_with_uncommitted_changes",
			result: CleanResult{
				Candidates: []CleanCandidate{
					{
						Branch:      "feat/a",
						Skipped:     true,
						SkipReason:  SkipHasChanges,
						CleanReason: CleanUpstreamGone,
						ChangedFiles: []FileStatus{
							{Status: " M", Path: "src/main.go"},
						},
					},
				},
				Check: true,
			},
			opts:       FormatOptions{Verbose: true},
			wantStdout: "skip:\n  feat/a\n    ✓ upstream gone\n    ✗ has uncommitted changes\n       M src/main.go\n\nNo worktrees to clean\n",
			wantStderr: "",
		},
		// StaleOverride tests
		{
			name: "stale_override_merged",
			result: CleanResult{
				Candidates: []CleanCandidate{
					{Branch: "feat/dirty", Skipped: false, CleanReason: CleanMerged, StaleOverride: true},
				},
				Check: true,
			},
			opts:       FormatOptions{},
			wantStdout: "clean:\n  feat/dirty (merged, stale)\n",
			wantStderr: "",
		},
		{
			name: "stale_override_upstream_gone",
			result: CleanResult{
				Candidates: []CleanCandidate{
					{Branch: "feat/gone", Skipped: false, CleanReason: CleanUpstreamGone, StaleOverride: true},
				},
				Check: true,
			},
			opts:       FormatOptions{},
			wantStdout: "clean:\n  feat/gone (upstream gone, stale)\n",
			wantStderr: "",
		},
		{
			name: "stale_override_prunable",
			result: CleanResult{
				Candidates: []CleanCandidate{
					{Branch: "feat/prunable", Prunable: true, Skipped: false, CleanReason: CleanMerged, StaleOverride: true},
				},
				Check: true,
			},
			opts:       FormatOptions{},
			wantStdout: "clean:\n  feat/prunable (prunable, merged, stale)\n",
			wantStderr: "",
		},
		{
			name: "stale_override_mixed_with_normal",
			result: CleanResult{
				Candidates: []CleanCandidate{
					{Branch: "feat/a", Skipped: false, CleanReason: CleanMerged},
					{Branch: "feat/dirty", Skipped: false, CleanReason: CleanMerged, StaleOverride: true},
					{Branch: "feat/wip", Skipped: true, SkipReason: SkipNotMerged},
				},
				Check: true,
			},
			opts:       FormatOptions{Verbose: true},
			wantStdout: "clean:\n  feat/a (merged)\n  feat/dirty (merged, stale)\n\nskip:\n  feat/wip\n    ✗ not merged\n",
			wantStderr: "",
		},
		{
			name: "detached_candidate_shows_path_and_reason",
			result: CleanResult{
				Candidates: []CleanCandidate{
					{WorktreePath: "/repo/detached", Detached: true, Skipped: false, CleanReason: CleanMerged},
				},
				Check: true,
			},
			opts:       FormatOptions{},
			wantStdout: "clean:\n  /repo/detached (detached, merged)\n",
			wantStderr: "",
		},
		{
			name: "detached_prunable_candidate",
			result: CleanResult{
				Candidates: []CleanCandidate{
					{WorktreePath: "/repo/detached", Detached: true, Prunable: true, Skipped: false, CleanReason: CleanMerged},
				},
				Check: true,
			},
			opts:       FormatOptions{},
			wantStdout: "clean:\n  /repo/detached (prunable, detached, merged)\n",
			wantStderr: "",
		},
		{
			name: "detached_skipped_shows_path",
			result: CleanResult{
				Candidates: []CleanCandidate{
					{Branch: "feat/a", Skipped: false, CleanReason: CleanMerged},
					{WorktreePath: "/repo/detached", Detached: true, Skipped: true, SkipReason: SkipNotMerged},
				},
				Check: true,
			},
			opts:       FormatOptions{Verbose: true},
			wantStdout: "clean:\n  feat/a (merged)\n\nskip:\n  /repo/detached\n    ✗ not merged\n",
			wantStderr: "",
		},
		{
			name: "detached_execution_results_verbose",
			result: CleanResult{
				Removed: []RemovedWorktree{
					{WorktreePath: "/repo/detached", Detached: true},
				},
				Check: false,
			},
			opts:       FormatOptions{Verbose: true},
			wantStdout: "Removed worktree: /repo/detached\n",
			wantStderr: "",
		},
		{
			name: "detached_execution_error_shows_path",
			result: CleanResult{
				Removed: []RemovedWorktree{
					{WorktreePath: "/repo/detached", Detached: true, Err: errors.New("boom")},
				},
				Check: false,
			},
			opts:       FormatOptions{},
			wantStdout: "",
			wantStderr: "error: /repo/detached: boom\n",
		},
		// ColorEnabled tests - output should be identical when color disabled
		{
			name: "color_disabled_same_as_no_color",
			result: CleanResult{
				Candidates: []CleanCandidate{
					{Branch: "feat/a", Skipped: false, CleanReason: CleanMerged},
					{Branch: "feat/b", Skipped: true, SkipReason: SkipNotMerged},
				},
				Check: true,
			},
			opts:       FormatOptions{ColorEnabled: false},
			wantStdout: "clean:\n  feat/a (merged)\n",
			wantStderr: "",
		},
		{
			name: "color_disabled_verbose_same_as_no_color",
			result: CleanResult{
				Candidates: []CleanCandidate{
					{Branch: "feat/a", Skipped: false, CleanReason: CleanMerged},
					{Branch: "feat/b", Skipped: true, SkipReason: SkipNotMerged},
				},
				Check: true,
			},
			opts:       FormatOptions{Verbose: true, ColorEnabled: false},
			wantStdout: "clean:\n  feat/a (merged)\n\nskip:\n  feat/b\n    ✗ not merged\n",
			wantStderr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tt.result.Format(tt.opts)
			if got.Stdout != tt.wantStdout {
				t.Errorf("Stdout = %q, want %q", got.Stdout, tt.wantStdout)
			}
			if got.Stderr != tt.wantStderr {
				t.Errorf("Stderr = %q, want %q", got.Stderr, tt.wantStderr)
			}
		})
	}
}

func TestCleanCommand_Run(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		cwd            string
		opts           CleanOptions
		config         *Config
		setupGit       func() *testutil.MockGitExecutor
		wantErr        bool
		errContains    string
		wantCandidates int
		wantSkipped    int
	}{
		{
			name: "finds_merged_candidates",
			cwd:  "/other/dir",
			opts: CleanOptions{},
			config: &Config{
				WorktreeSourceDir: "/repo/main",
				DefaultSource:     "main",
			},
			setupGit: func() *testutil.MockGitExecutor {
				return &testutil.MockGitExecutor{
					Worktrees: []testutil.MockWorktree{
						{Path: "/repo/main", Branch: "main"},
						{Path: "/repo/feat/a", Branch: "feat/a"},
						{Path: "/repo/feat/b", Branch: "feat/b"},
					},
					MergedBranches: map[string][]string{
						"main": {"main", "feat/a"},
					},
				}
			},
			wantCandidates: 2,
			wantSkipped:    1, // feat/b not merged
		},
		{
			name: "skips_locked_worktrees",
			cwd:  "/other/dir",
			opts: CleanOptions{},
			config: &Config{
				WorktreeSourceDir: "/repo/main",
				DefaultSource:     "main",
			},
			setupGit: func() *testutil.MockGitExecutor {
				return &testutil.MockGitExecutor{
					Worktrees: []testutil.MockWorktree{
						{Path: "/repo/main", Branch: "main"},
						{Path: "/repo/feat/a", Branch: "feat/a", Locked: true},
					},
					MergedBranches: map[string][]string{
						"main": {"main", "feat/a"},
					},
				}
			},
			wantCandidates: 1,
			wantSkipped:    1,
		},
		{
			name: "skips_current_directory",
			cwd:  "/repo/feat/a/subdir",
			opts: CleanOptions{},
			config: &Config{
				WorktreeSourceDir: "/repo/main",
				DefaultSource:     "main",
			},
			setupGit: func() *testutil.MockGitExecutor {
				return &testutil.MockGitExecutor{
					Worktrees: []testutil.MockWorktree{
						{Path: "/repo/main", Branch: "main"},
						{Path: "/repo/feat/a", Branch: "feat/a"},
					},
					MergedBranches: map[string][]string{
						"main": {"main", "feat/a"},
					},
				}
			},
			wantCandidates: 1,
			wantSkipped:    1,
		},
		{
			name: "skips_worktrees_with_changes",
			cwd:  "/other/dir",
			opts: CleanOptions{},
			config: &Config{
				WorktreeSourceDir: "/repo/main",
				DefaultSource:     "main",
			},
			setupGit: func() *testutil.MockGitExecutor {
				return &testutil.MockGitExecutor{
					Worktrees: []testutil.MockWorktree{
						{Path: "/repo/main", Branch: "main"},
						{Path: "/repo/feat/a", Branch: "feat/a"},
					},
					MergedBranches: map[string][]string{
						"main": {"main", "feat/a"},
					},
					HasChanges: true,
				}
			},
			wantCandidates: 1,
			wantSkipped:    1,
		},
		{
			name: "skips_detached_head_not_contained_in_target",
			cwd:  "/other/dir",
			opts: CleanOptions{},
			config: &Config{
				WorktreeSourceDir: "/repo/main",
				DefaultSource:     "main",
			},
			setupGit: func() *testutil.MockGitExecutor {
				return &testutil.MockGitExecutor{
					Worktrees: []testutil.MockWorktree{
						{Path: "/repo/main", Branch: "main"},
						{Path: "/repo/detached", Detached: true, HEAD: "own-commit"},
					},
					MergedBranches: map[string][]string{
						"main": {"main"},
					},
				}
			},
			wantCandidates: 1,
			wantSkipped:    1,
		},
		{
			name: "cleans_detached_head_contained_in_target",
			cwd:  "/other/dir",
			opts: CleanOptions{},
			config: &Config{
				WorktreeSourceDir: "/repo/main",
				DefaultSource:     "main",
			},
			setupGit: func() *testutil.MockGitExecutor {
				return &testutil.MockGitExecutor{
					Worktrees: []testutil.MockWorktree{
						{Path: "/repo/main", Branch: "main"},
						{Path: "/repo/detached", Detached: true, HEAD: "merged-commit"},
					},
					MergedBranches: map[string][]string{
						"main": {"main"},
					},
					Ancestors: map[string][]string{
						"main": {"merged-commit"},
					},
				}
			},
			wantCandidates: 1,
			wantSkipped:    0,
		},
		{
			name: "skips_detached_head_in_current_directory",
			cwd:  "/repo/detached/subdir",
			opts: CleanOptions{},
			config: &Config{
				WorktreeSourceDir: "/repo/main",
				DefaultSource:     "main",
			},
			setupGit: func() *testutil.MockGitExecutor {
				return &testutil.MockGitExecutor{
					Worktrees: []testutil.MockWorktree{
						{Path: "/repo/main", Branch: "main"},
						{Path: "/repo/detached", Detached: true, HEAD: "merged-commit"},
					},
					MergedBranches: map[string][]string{
						"main": {"main"},
					},
					Ancestors: map[string][]string{
						"main": {"merged-commit"},
					},
				}
			},
			wantCandidates: 1,
			wantSkipped:    1,
		},
		{
			name: "skips_locked_detached_head",
			cwd:  "/other/dir",
			opts: CleanOptions{},
			config: &Config{
				WorktreeSourceDir: "/repo/main",
				DefaultSource:     "main",
			},
			setupGit: func() *testutil.MockGitExecutor {
				return &testutil.MockGitExecutor{
					Worktrees: []testutil.MockWorktree{
						{Path: "/repo/main", Branch: "main"},
						{Path: "/repo/detached", Detached: true, HEAD: "merged-commit", Locked: true},
					},
					MergedBranches: map[string][]string{
						"main": {"main"},
					},
					Ancestors: map[string][]string{
						"main": {"merged-commit"},
					},
				}
			},
			wantCandidates: 1,
			wantSkipped:    1,
		},
		{
			name: "force_cleans_detached_head_not_contained_in_target",
			cwd:  "/other/dir",
			opts: CleanOptions{Force: WorktreeForceLevelUnclean, Check: true},
			config: &Config{
				WorktreeSourceDir: "/repo/main",
				DefaultSource:     "main",
			},
			setupGit: func() *testutil.MockGitExecutor {
				return &testutil.MockGitExecutor{
					Worktrees: []testutil.MockWorktree{
						{Path: "/repo/main", Branch: "main"},
						{Path: "/repo/detached", Detached: true, HEAD: "own-commit"},
					},
					MergedBranches: map[string][]string{
						"main": {"main"},
					},
				}
			},
			wantCandidates: 1,
			wantSkipped:    0,
		},
		{
			name: "uses_target_flag",
			cwd:  "/other/dir",
			opts: CleanOptions{Target: "develop"},
			config: &Config{
				WorktreeSourceDir: "/repo/main",
				DefaultSource:     "main",
			},
			setupGit: func() *testutil.MockGitExecutor {
				return &testutil.MockGitExecutor{
					Worktrees: []testutil.MockWorktree{
						{Path: "/repo/main", Branch: "main"},
						{Path: "/repo/feat/a", Branch: "feat/a"},
					},
					MergedBranches: map[string][]string{
						"develop": {"develop", "feat/a"},
					},
				}
			},
			wantCandidates: 1,
			wantSkipped:    0,
		},
		{
			name: "auto_detects_target",
			cwd:  "/other/dir",
			opts: CleanOptions{},
			config: &Config{
				WorktreeSourceDir: "/repo/main",
			},
			setupGit: func() *testutil.MockGitExecutor {
				return &testutil.MockGitExecutor{
					Worktrees: []testutil.MockWorktree{
						{Path: "/repo/main", Branch: "main"},
						{Path: "/repo/feat/a", Branch: "feat/a"},
					},
					MergedBranches: map[string][]string{
						"main": {"main", "feat/a"},
					},
				}
			},
			wantCandidates: 1,
			wantSkipped:    0,
		},
		{
			name: "skips_bare_worktrees",
			cwd:  "/other/dir",
			opts: CleanOptions{},
			config: &Config{
				WorktreeSourceDir: "/repo/main",
				DefaultSource:     "main",
			},
			setupGit: func() *testutil.MockGitExecutor {
				return &testutil.MockGitExecutor{
					Worktrees: []testutil.MockWorktree{
						{Path: "/repo/main", Branch: "main"},
						{Path: "/repo/.git/worktrees/bare", Bare: true},
						{Path: "/repo/feat/a", Branch: "feat/a"},
					},
					MergedBranches: map[string][]string{
						"main": {"main", "feat/a"},
					},
				}
			},
			wantCandidates: 1,
			wantSkipped:    0,
		},
		// Orphaned branch tests
		{
			name: "detects_prunable_as_orphaned",
			cwd:  "/other/dir",
			opts: CleanOptions{Check: true},
			config: &Config{
				WorktreeSourceDir: "/repo/main",
				DefaultSource:     "main",
			},
			setupGit: func() *testutil.MockGitExecutor {
				return &testutil.MockGitExecutor{
					Worktrees: []testutil.MockWorktree{
						{Path: "/repo/main", Branch: "main"},
						{Path: "/repo/feat/orphaned", Branch: "feat/orphaned", Prunable: true},
					},
					MergedBranches: map[string][]string{
						"main": {"main", "feat/orphaned"},
					},
				}
			},
			wantCandidates: 1,
			wantSkipped:    0,
		},
		{
			name: "orphaned_not_merged_is_skipped",
			cwd:  "/other/dir",
			opts: CleanOptions{Check: true},
			config: &Config{
				WorktreeSourceDir: "/repo/main",
				DefaultSource:     "main",
			},
			setupGit: func() *testutil.MockGitExecutor {
				return &testutil.MockGitExecutor{
					Worktrees: []testutil.MockWorktree{
						{Path: "/repo/main", Branch: "main"},
						{Path: "/repo/feat/orphaned", Branch: "feat/orphaned", Prunable: true},
					},
					MergedBranches: map[string][]string{
						"main": {"main"},
					},
				}
			},
			wantCandidates: 1,
			wantSkipped:    1,
		},
		{
			name: "mixed_worktree_and_orphaned",
			cwd:  "/other/dir",
			opts: CleanOptions{Check: true},
			config: &Config{
				WorktreeSourceDir: "/repo/main",
				DefaultSource:     "main",
			},
			setupGit: func() *testutil.MockGitExecutor {
				return &testutil.MockGitExecutor{
					Worktrees: []testutil.MockWorktree{
						{Path: "/repo/main", Branch: "main"},
						{Path: "/repo/feat/a", Branch: "feat/a"},
						{Path: "/repo/feat/orphaned", Branch: "feat/orphaned", Prunable: true},
					},
					MergedBranches: map[string][]string{
						"main": {"main", "feat/a", "feat/orphaned"},
					},
				}
			},
			wantCandidates: 2,
			wantSkipped:    0,
		},
		{
			name: "stale_overrides_has_changes_when_merged",
			cwd:  "/other/dir",
			opts: CleanOptions{Check: true, Stale: true},
			config: &Config{
				WorktreeSourceDir: "/repo/main",
				DefaultSource:     "main",
			},
			setupGit: func() *testutil.MockGitExecutor {
				return &testutil.MockGitExecutor{
					Worktrees: []testutil.MockWorktree{
						{Path: "/repo/main", Branch: "main"},
						{Path: "/repo/feat/a", Branch: "feat/a"},
					},
					MergedBranches: map[string][]string{
						"main": {"main", "feat/a"},
					},
					HasChanges: true,
				}
			},
			wantCandidates: 1,
			wantSkipped:    0, // stale overrides SkipHasChanges
		},
		{
			name: "stale_does_not_override_not_merged",
			cwd:  "/other/dir",
			opts: CleanOptions{Check: true, Stale: true},
			config: &Config{
				WorktreeSourceDir: "/repo/main",
				DefaultSource:     "main",
			},
			setupGit: func() *testutil.MockGitExecutor {
				return &testutil.MockGitExecutor{
					Worktrees: []testutil.MockWorktree{
						{Path: "/repo/main", Branch: "main"},
						{Path: "/repo/feat/a", Branch: "feat/a"},
					},
					MergedBranches: map[string][]string{
						"main": {"main"},
					},
					HasChanges: true,
				}
			},
			wantCandidates: 1,
			wantSkipped:    1, // not merged, stale does not override
		},
		{
			name: "stale_does_not_override_locked",
			cwd:  "/other/dir",
			opts: CleanOptions{Check: true, Stale: true},
			config: &Config{
				WorktreeSourceDir: "/repo/main",
				DefaultSource:     "main",
			},
			setupGit: func() *testutil.MockGitExecutor {
				return &testutil.MockGitExecutor{
					Worktrees: []testutil.MockWorktree{
						{Path: "/repo/main", Branch: "main"},
						{Path: "/repo/feat/a", Branch: "feat/a", Locked: true},
					},
					MergedBranches: map[string][]string{
						"main": {"main", "feat/a"},
					},
				}
			},
			wantCandidates: 1,
			wantSkipped:    1, // locked, stale does not override
		},
		{
			name: "stale_does_not_override_wip_on_first_parent",
			cwd:  "/other/dir",
			opts: CleanOptions{Check: true, Stale: true},
			config: &Config{
				WorktreeSourceDir: "/repo/main",
				DefaultSource:     "main",
			},
			setupGit: func() *testutil.MockGitExecutor {
				return &testutil.MockGitExecutor{
					Worktrees: []testutil.MockWorktree{
						{Path: "/repo/main", Branch: "main"},
						{Path: "/repo/feat/a", Branch: "feat/a", HEAD: "wip-commit"},
					},
					MergedBranches: map[string][]string{
						"main": {"main", "feat/a"},
					},
					HasChanges: true,
					FirstParentAncestors: map[string][]string{
						"main": {"wip-commit"},
					},
				}
			},
			wantCandidates: 1,
			wantSkipped:    1, // WIP on first-parent: CleanReason cleared, stale does not override
		},
		{
			name: "stale_overrides_genuinely_merged_not_first_parent",
			cwd:  "/other/dir",
			opts: CleanOptions{Check: true, Stale: true},
			config: &Config{
				WorktreeSourceDir: "/repo/main",
				DefaultSource:     "main",
			},
			setupGit: func() *testutil.MockGitExecutor {
				return &testutil.MockGitExecutor{
					Worktrees: []testutil.MockWorktree{
						{Path: "/repo/main", Branch: "main"},
						{Path: "/repo/feat/a", Branch: "feat/a", HEAD: "merged-commit"},
					},
					MergedBranches: map[string][]string{
						"main": {"main", "feat/a"},
					},
					HasChanges: true,
					// FirstParentAncestors not set -> not on first-parent -> genuinely merged
				}
			},
			wantCandidates: 1,
			wantSkipped:    0, // genuinely merged: stale override applies
		},
		{
			name: "skips_new_branch_pointing_to_same_commit_as_target",
			cwd:  "/other/dir",
			opts: CleanOptions{},
			config: &Config{
				WorktreeSourceDir: "/repo/main",
				DefaultSource:     "main",
			},
			setupGit: func() *testutil.MockGitExecutor {
				return &testutil.MockGitExecutor{
					Worktrees: []testutil.MockWorktree{
						{Path: "/repo/main", Branch: "main", HEAD: "same-commit-abc123"},
						{Path: "/repo/feat/new", Branch: "feat/new", HEAD: "same-commit-abc123"},
					},
					MergedBranches: map[string][]string{
						"main": {"main", "feat/new"}, // git branch --merged returns this
					},
				}
			},
			wantCandidates: 1,
			wantSkipped:    1, // feat/new should be skipped because same commit as main
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mockGit := tt.setupGit()

			cmd := &CleanCommand{
				FS:     &testutil.MockFS{},
				Git:    &GitRunner{Executor: mockGit, Log: NewNopLogger()},
				Config: tt.config,
				Log:    NewNopLogger(),
			}

			result, err := cmd.Run(t.Context(), tt.cwd, tt.opts)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errContains)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(result.Candidates) != tt.wantCandidates {
				t.Errorf("got %d candidates, want %d", len(result.Candidates), tt.wantCandidates)
			}

			skippedCount := 0
			for _, c := range result.Candidates {
				if c.Skipped {
					skippedCount++
				}
			}
			if skippedCount != tt.wantSkipped {
				t.Errorf("got %d skipped, want %d", skippedCount, tt.wantSkipped)
			}
		})
	}
}

func TestCleanCommand_Run_DetachedWorktrees(t *testing.T) {
	t.Parallel()

	newCmd := func(mockGit *testutil.MockGitExecutor) *CleanCommand {
		return &CleanCommand{
			FS:     &testutil.MockFS{},
			Git:    &GitRunner{Executor: mockGit, Log: NewNopLogger()},
			Config: &Config{WorktreeSourceDir: "/repo/main", DefaultSource: "main"},
			Log:    NewNopLogger(),
		}
	}

	t.Run("contained_head_is_cleanable_without_branch", func(t *testing.T) {
		t.Parallel()

		mockGit := &testutil.MockGitExecutor{
			Worktrees: []testutil.MockWorktree{
				{Path: "/repo/main", Branch: "main"},
				{Path: "/repo/detached", Detached: true, HEAD: "merged-commit"},
			},
			MergedBranches: map[string][]string{"main": {"main"}},
			Ancestors:      map[string][]string{"main": {"merged-commit"}},
		}

		result, err := newCmd(mockGit).Run(t.Context(), "/other/dir", CleanOptions{Check: true})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result.Candidates) != 1 {
			t.Fatalf("got %d candidates, want 1", len(result.Candidates))
		}

		got := result.Candidates[0]
		if got.Skipped {
			t.Errorf("candidate skipped with reason %q, want cleanable", got.SkipReason)
		}
		if !got.Detached {
			t.Error("Detached = false, want true")
		}
		if got.Branch != "" {
			t.Errorf("Branch = %q, want empty", got.Branch)
		}
		if got.WorktreePath != "/repo/detached" {
			t.Errorf("WorktreePath = %q, want /repo/detached", got.WorktreePath)
		}
		if got.CleanReason != CleanMerged {
			t.Errorf("CleanReason = %q, want %q", got.CleanReason, CleanMerged)
		}
		if got.DisplayName() != "/repo/detached" {
			t.Errorf("DisplayName() = %q, want /repo/detached", got.DisplayName())
		}
	})

	t.Run("stale_does_not_bypass_changes", func(t *testing.T) {
		t.Parallel()

		mockGit := &testutil.MockGitExecutor{
			Worktrees: []testutil.MockWorktree{
				{Path: "/repo/main", Branch: "main"},
				{Path: "/repo/detached", Detached: true, HEAD: "merged-commit"},
			},
			MergedBranches:  map[string][]string{"main": {"main"}},
			Ancestors:       map[string][]string{"main": {"merged-commit"}},
			StatusOutputMap: map[string]string{"/repo/detached": " M work.go\n"},
		}

		result, err := newCmd(mockGit).Run(t.Context(), "/other/dir", CleanOptions{Check: true, Stale: true})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result.Candidates) != 1 {
			t.Fatalf("got %d candidates, want 1", len(result.Candidates))
		}

		got := result.Candidates[0]
		if !got.Skipped {
			t.Fatal("candidate is cleanable, want skipped: uncommitted work is all a detached worktree holds")
		}
		if got.SkipReason != SkipHasChanges {
			t.Errorf("SkipReason = %q, want %q", got.SkipReason, SkipHasChanges)
		}
		if got.CleanReason != "" {
			t.Errorf("CleanReason = %q, want empty so --stale cannot override", got.CleanReason)
		}
	})

	t.Run("removal_skips_branch_delete", func(t *testing.T) {
		t.Parallel()

		var captured []string
		mockGit := &testutil.MockGitExecutor{
			Worktrees: []testutil.MockWorktree{
				{Path: "/repo/main", Branch: "main"},
				{Path: "/repo/detached", Detached: true, HEAD: "merged-commit"},
			},
			MergedBranches: map[string][]string{"main": {"main"}},
			Ancestors:      map[string][]string{"main": {"merged-commit"}},
			CapturedArgs:   &captured,
		}

		result, err := newCmd(mockGit).Run(t.Context(), "/other/dir", CleanOptions{Yes: true})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result.Removed) != 1 {
			t.Fatalf("got %d removed, want 1", len(result.Removed))
		}
		if result.Removed[0].Err != nil {
			t.Fatalf("removal failed: %v", result.Removed[0].Err)
		}
		if !result.Removed[0].Detached {
			t.Error("Removed[0].Detached = false, want true")
		}

		var removedPath string
		for i, arg := range captured {
			if arg == "remove" && i+1 < len(captured) {
				removedPath = captured[i+1]
			}
			if arg == "branch" && i+1 < len(captured) &&
				(captured[i+1] == "-d" || captured[i+1] == "-D") {
				t.Errorf("branch delete was called for a detached worktree: %v", captured)
			}
		}
		if removedPath != "/repo/detached" {
			t.Errorf("removed path = %q, want /repo/detached", removedPath)
		}
	})
}

func TestCleanCommand_ResolveTarget(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		target     string
		config     *Config
		worktrees  []testutil.MockWorktree
		wantTarget string
		wantErr    bool
	}{
		{
			name:       "uses_provided_target",
			target:     "develop",
			config:     &Config{},
			wantTarget: "develop",
		},
		{
			name:   "auto_detects_from_worktrees",
			target: "",
			config: &Config{},
			worktrees: []testutil.MockWorktree{
				{Path: "/repo/main", Branch: "main"},
			},
			wantTarget: "main",
		},
		{
			name:      "error_when_no_target_found",
			target:    "",
			config:    &Config{},
			worktrees: []testutil.MockWorktree{},
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mockGit := &testutil.MockGitExecutor{
				Worktrees: tt.worktrees,
			}

			cmd := &CleanCommand{
				Git:    &GitRunner{Executor: mockGit, Log: NewNopLogger()},
				Config: tt.config,
				Log:    NewNopLogger(),
			}

			worktrees, err := cmd.Git.WorktreeList(t.Context())
			if err != nil {
				t.Fatalf("unexpected error listing worktrees: %v", err)
			}

			got, err := cmd.resolveTarget(tt.target, worktrees)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != tt.wantTarget {
				t.Errorf("got %q, want %q", got, tt.wantTarget)
			}
		})
	}
}
