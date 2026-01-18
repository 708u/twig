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
			wantStdout: "clean:\n  feat/a (merged)\n\nskip:\n  feat/b (not merged)\n",
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
			wantStdout: "clean:\n  feat/a (merged)\n  feat/prunable (prunable, merged)\n\nskip:\n  feat/wip (not merged)\n",
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
						Branch:     "feat/wip",
						Skipped:    true,
						SkipReason: SkipHasChanges,
						ChangedFiles: []FileStatus{
							{Status: " M", Path: "src/main.go"},
							{Status: "??", Path: "tmp/debug.log"},
						},
					},
				},
				Check: true,
			},
			opts:       FormatOptions{Verbose: true},
			wantStdout: "clean:\n  feat/a (merged)\n\nskip:\n  feat/wip (has uncommitted changes)\n     M src/main.go\n    ?? tmp/debug.log\n",
			wantStderr: "",
		},
		{
			name: "verbose_with_dirty_submodule_changed_files",
			result: CleanResult{
				Candidates: []CleanCandidate{
					{
						Branch:     "feat/submod",
						Skipped:    true,
						SkipReason: SkipDirtySubmodule,
						ChangedFiles: []FileStatus{
							{Status: " M", Path: "submodule/file.go"},
						},
					},
				},
				Check: true,
			},
			opts:       FormatOptions{Verbose: true},
			wantStdout: "skip:\n  feat/submod (submodule has uncommitted changes)\n     M submodule/file.go\n\nNo worktrees to clean\n",
			wantStderr: "",
		},
		{
			name: "verbose_skip_reason_without_changed_files",
			result: CleanResult{
				Candidates: []CleanCandidate{
					{Branch: "feat/a", Skipped: false, CleanReason: CleanMerged},
					{Branch: "feat/locked", Skipped: true, SkipReason: SkipLocked},
				},
				Check: true,
			},
			opts:       FormatOptions{Verbose: true},
			wantStdout: "clean:\n  feat/a (merged)\n\nskip:\n  feat/locked (locked)\n",
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
						{Path: "/repo/feat/a", Detached: true},
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mockGit := tt.setupGit()

			cmd := &CleanCommand{
				FS:     &testutil.MockFS{},
				Git:    &GitRunner{Executor: mockGit, Log: NewNopLogger()},
				Config: tt.config,
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
