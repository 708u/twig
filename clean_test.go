package twig

import (
	"strings"
	"testing"

	"github.com/708u/twig/internal/testutil"
)

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
		// Integrity info tests
		{
			name: "detached_always_shown",
			result: CleanResult{
				Candidates: []CleanCandidate{
					{Branch: "feat/a", Skipped: false, CleanReason: CleanMerged},
				},
				Check: true,
				Integrity: IntegrityInfo{
					DetachedWorktrees: []DetachedWorktreeInfo{
						{Path: "/repo/detached", HEAD: "abc1234"},
					},
				},
			},
			opts:       FormatOptions{},
			wantStdout: "clean:\n  feat/a (merged)\n\ndetached:\n  /repo/detached (HEAD at abc1234)\n",
			wantStderr: "",
		},
		{
			name: "orphan_branches_verbose_only",
			result: CleanResult{
				Candidates: []CleanCandidate{
					{Branch: "feat/a", Skipped: false, CleanReason: CleanMerged},
				},
				Check: true,
				Integrity: IntegrityInfo{
					OrphanBranches: []string{"feat/old", "fix/abandoned"},
				},
			},
			opts:       FormatOptions{},
			wantStdout: "clean:\n  feat/a (merged)\n",
			wantStderr: "",
		},
		{
			name: "orphan_branches_shown_in_verbose",
			result: CleanResult{
				Candidates: []CleanCandidate{
					{Branch: "feat/a", Skipped: false, CleanReason: CleanMerged},
				},
				Check: true,
				Integrity: IntegrityInfo{
					OrphanBranches: []string{"feat/old", "fix/abandoned"},
				},
			},
			opts:       FormatOptions{Verbose: true},
			wantStdout: "clean:\n  feat/a (merged)\n\norphan branches:\n  feat/old (no worktree)\n  fix/abandoned (no worktree)\n",
			wantStderr: "",
		},
		{
			name: "locked_verbose_only",
			result: CleanResult{
				Candidates: []CleanCandidate{
					{Branch: "feat/a", Skipped: false, CleanReason: CleanMerged},
				},
				Check: true,
				Integrity: IntegrityInfo{
					LockedWorktrees: []LockedWorktreeInfo{
						{Branch: "feat/usb", Path: "/repo/feat/usb", LockReason: "USB drive work"},
					},
				},
			},
			opts:       FormatOptions{},
			wantStdout: "clean:\n  feat/a (merged)\n",
			wantStderr: "",
		},
		{
			name: "locked_shown_in_verbose",
			result: CleanResult{
				Candidates: []CleanCandidate{
					{Branch: "feat/a", Skipped: false, CleanReason: CleanMerged},
				},
				Check: true,
				Integrity: IntegrityInfo{
					LockedWorktrees: []LockedWorktreeInfo{
						{Branch: "feat/usb", Path: "/repo/feat/usb", LockReason: "USB drive work"},
					},
				},
			},
			opts:       FormatOptions{Verbose: true},
			wantStdout: "clean:\n  feat/a (merged)\n\nlocked:\n  feat/usb (reason: USB drive work)\n",
			wantStderr: "",
		},
		{
			name: "locked_without_reason",
			result: CleanResult{
				Candidates: []CleanCandidate{
					{Branch: "feat/a", Skipped: false, CleanReason: CleanMerged},
				},
				Check: true,
				Integrity: IntegrityInfo{
					LockedWorktrees: []LockedWorktreeInfo{
						{Branch: "feat/locked", Path: "/repo/feat/locked"},
					},
				},
			},
			opts:       FormatOptions{Verbose: true},
			wantStdout: "clean:\n  feat/a (merged)\n\nlocked:\n  feat/locked (locked)\n",
			wantStderr: "",
		},
		{
			name: "all_integrity_sections_verbose",
			result: CleanResult{
				Candidates: []CleanCandidate{
					{Branch: "feat/a", Skipped: false, CleanReason: CleanMerged},
				},
				Check: true,
				Integrity: IntegrityInfo{
					DetachedWorktrees: []DetachedWorktreeInfo{
						{Path: "/repo/detached", HEAD: "abc1234"},
					},
					OrphanBranches: []string{"feat/old"},
					LockedWorktrees: []LockedWorktreeInfo{
						{Branch: "feat/usb", Path: "/repo/feat/usb", LockReason: "USB drive"},
					},
				},
			},
			opts:       FormatOptions{Verbose: true},
			wantStdout: "clean:\n  feat/a (merged)\n\ndetached:\n  /repo/detached (HEAD at abc1234)\n\norphan branches:\n  feat/old (no worktree)\n\nlocked:\n  feat/usb (reason: USB drive)\n",
			wantStderr: "",
		},
		{
			name: "integrity_with_no_candidates",
			result: CleanResult{
				Candidates: []CleanCandidate{},
				Check:      true,
				Integrity: IntegrityInfo{
					DetachedWorktrees: []DetachedWorktreeInfo{
						{Path: "/repo/detached", HEAD: "abc1234"},
					},
				},
			},
			opts:       FormatOptions{},
			wantStdout: "No worktrees to clean\n\ndetached:\n  /repo/detached (HEAD at abc1234)\n",
			wantStderr: "",
		},
		{
			name: "integrity_not_shown_in_removal_results",
			result: CleanResult{
				Removed: []RemovedWorktree{
					{Branch: "feat/a"},
				},
				Check: false,
				Integrity: IntegrityInfo{
					DetachedWorktrees: []DetachedWorktreeInfo{
						{Path: "/repo/detached", HEAD: "abc1234"},
					},
				},
			},
			opts:       FormatOptions{Verbose: true},
			wantStdout: "Removed worktree and branch: feat/a\n",
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
		wantOrphans    int
		wantLocked     int
		wantDetached   int
		checkIntegrity func(t *testing.T, integrity IntegrityInfo)
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
						{Path: "/repo/feat/a", Branch: "feat/a", Locked: true, LockReason: "USB drive"},
					},
					MergedBranches: map[string][]string{
						"main": {"main", "feat/a"},
					},
				}
			},
			wantCandidates: 1,
			wantSkipped:    1,
			wantLocked:     1,
			checkIntegrity: func(t *testing.T, integrity IntegrityInfo) {
				t.Helper()
				if integrity.LockedWorktrees[0].Branch != "feat/a" {
					t.Errorf("locked branch = %q, want %q", integrity.LockedWorktrees[0].Branch, "feat/a")
				}
				if integrity.LockedWorktrees[0].LockReason != "USB drive" {
					t.Errorf("lock reason = %q, want %q", integrity.LockedWorktrees[0].LockReason, "USB drive")
				}
			},
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
			name: "skips_detached_head",
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
						{Path: "/repo/feat/a", Detached: true, HEAD: "deadbeef1234567"},
					},
					MergedBranches: map[string][]string{
						"main": {"main"},
					},
				}
			},
			wantCandidates: 1,
			wantSkipped:    1,
			wantDetached:   1,
			checkIntegrity: func(t *testing.T, integrity IntegrityInfo) {
				t.Helper()
				if integrity.DetachedWorktrees[0].Path != "/repo/feat/a" {
					t.Errorf("detached path = %q, want %q", integrity.DetachedWorktrees[0].Path, "/repo/feat/a")
				}
				if integrity.DetachedWorktrees[0].HEAD != "deadbee" {
					t.Errorf("detached HEAD = %q, want %q", integrity.DetachedWorktrees[0].HEAD, "deadbee")
				}
			},
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
			wantLocked:     1,
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
		// Integrity tests
		{
			name: "detects_orphan_branches",
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
					},
					MergedBranches: map[string][]string{
						"main": {"main", "feat/a"},
					},
					LocalBranches: []string{"main", "feat/a", "feat/orphan1", "fix/orphan2"},
				}
			},
			wantCandidates: 1,
			wantSkipped:    0,
			wantOrphans:    2,
			checkIntegrity: func(t *testing.T, integrity IntegrityInfo) {
				t.Helper()
				if integrity.OrphanBranches[0] != "feat/orphan1" {
					t.Errorf("orphan[0] = %q, want %q", integrity.OrphanBranches[0], "feat/orphan1")
				}
				if integrity.OrphanBranches[1] != "fix/orphan2" {
					t.Errorf("orphan[1] = %q, want %q", integrity.OrphanBranches[1], "fix/orphan2")
				}
			},
		},
		{
			name: "no_orphan_branches_when_all_have_worktrees",
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
					},
					MergedBranches: map[string][]string{
						"main": {"main", "feat/a"},
					},
					LocalBranches: []string{"main", "feat/a"},
				}
			},
			wantCandidates: 1,
			wantSkipped:    0,
			wantOrphans:    0,
		},
		{
			name: "collects_multiple_locked_worktrees",
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
						{Path: "/repo/feat/a", Branch: "feat/a", Locked: true, LockReason: "reason A"},
						{Path: "/repo/feat/b", Branch: "feat/b", Locked: true},
					},
					MergedBranches: map[string][]string{
						"main": {"main", "feat/a", "feat/b"},
					},
				}
			},
			wantCandidates: 2,
			wantSkipped:    2,
			wantLocked:     2,
			checkIntegrity: func(t *testing.T, integrity IntegrityInfo) {
				t.Helper()
				if integrity.LockedWorktrees[0].LockReason != "reason A" {
					t.Errorf("locked[0] reason = %q, want %q", integrity.LockedWorktrees[0].LockReason, "reason A")
				}
				if integrity.LockedWorktrees[1].LockReason != "" {
					t.Errorf("locked[1] reason = %q, want empty", integrity.LockedWorktrees[1].LockReason)
				}
			},
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

			if len(result.Integrity.OrphanBranches) != tt.wantOrphans {
				t.Errorf("got %d orphan branches, want %d", len(result.Integrity.OrphanBranches), tt.wantOrphans)
			}
			if len(result.Integrity.LockedWorktrees) != tt.wantLocked {
				t.Errorf("got %d locked worktrees, want %d", len(result.Integrity.LockedWorktrees), tt.wantLocked)
			}
			if len(result.Integrity.DetachedWorktrees) != tt.wantDetached {
				t.Errorf("got %d detached worktrees, want %d", len(result.Integrity.DetachedWorktrees), tt.wantDetached)
			}
			if tt.checkIntegrity != nil {
				tt.checkIntegrity(t, result.Integrity)
			}
		})
	}
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

			got, err := cmd.resolveTarget(t.Context(), tt.target)

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

func TestCleanCommand_FindOrphanBranches(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		worktrees []Worktree
		branches  []string
		want      []string
	}{
		{
			name: "finds_orphans",
			worktrees: []Worktree{
				{Path: "/repo/main", Branch: "main"},
				{Path: "/repo/feat/a", Branch: "feat/a"},
			},
			branches: []string{"main", "feat/a", "feat/orphan", "fix/old"},
			want:     []string{"feat/orphan", "fix/old"},
		},
		{
			name: "no_orphans",
			worktrees: []Worktree{
				{Path: "/repo/main", Branch: "main"},
				{Path: "/repo/feat/a", Branch: "feat/a"},
			},
			branches: []string{"main", "feat/a"},
			want:     nil,
		},
		{
			name: "ignores_detached_worktrees",
			worktrees: []Worktree{
				{Path: "/repo/main", Branch: "main"},
				{Path: "/repo/detached", Branch: ""},
			},
			branches: []string{"main", "feat/orphan"},
			want:     []string{"feat/orphan"},
		},
		{
			name:      "empty_branches",
			worktrees: []Worktree{{Path: "/repo/main", Branch: "main"}},
			branches:  []string{},
			want:      nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mockGit := &testutil.MockGitExecutor{
				LocalBranches: tt.branches,
			}

			cmd := &CleanCommand{
				Git: &GitRunner{Executor: mockGit, Log: NewNopLogger()},
				Log: NewNopLogger(),
			}

			got, err := cmd.findOrphanBranches(t.Context(), tt.worktrees)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(got) != len(tt.want) {
				t.Fatalf("got %d orphans, want %d: %v", len(got), len(tt.want), got)
			}
			for i, g := range got {
				if g != tt.want[i] {
					t.Errorf("orphan[%d] = %q, want %q", i, g, tt.want[i])
				}
			}
		})
	}
}
