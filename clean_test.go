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
			wantStdout: "twig clean: feat/a\ntwig clean: feat/b\n",
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
				Git:    &GitRunner{Executor: mockGit},
				Config: tt.config,
			}

			result, err := cmd.Run(tt.cwd, tt.opts)

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
				Git:    &GitRunner{Executor: mockGit},
				Config: tt.config,
			}

			got, err := cmd.resolveTarget(tt.target)

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

func TestCleanCommand_CheckPrunableSkipReason(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		branch     string
		target     string
		force      WorktreeForceLevel
		setupGit   func() *testutil.MockGitExecutor
		wantReason SkipReason
	}{
		{
			name:   "no_skip_for_merged_branch",
			branch: "feat/a",
			target: "main",
			force:  WorktreeForceLevelNone,
			setupGit: func() *testutil.MockGitExecutor {
				return &testutil.MockGitExecutor{
					MergedBranches: map[string][]string{
						"main": {"feat/a"},
					},
				}
			},
			wantReason: "",
		},
		{
			name:   "skip_not_merged",
			branch: "feat/a",
			target: "main",
			force:  WorktreeForceLevelNone,
			setupGit: func() *testutil.MockGitExecutor {
				return &testutil.MockGitExecutor{
					MergedBranches: map[string][]string{
						"main": {},
					},
				}
			},
			wantReason: SkipNotMerged,
		},
		{
			name:   "force_bypasses_not_merged",
			branch: "feat/a",
			target: "main",
			force:  WorktreeForceLevelUnclean,
			setupGit: func() *testutil.MockGitExecutor {
				return &testutil.MockGitExecutor{
					MergedBranches: map[string][]string{
						"main": {},
					},
				}
			},
			wantReason: "",
		},
		{
			name:   "upstream_gone_is_merged",
			branch: "feat/a",
			target: "main",
			force:  WorktreeForceLevelNone,
			setupGit: func() *testutil.MockGitExecutor {
				return &testutil.MockGitExecutor{
					MergedBranches:       map[string][]string{"main": {}},
					UpstreamGoneBranches: []string{"feat/a"},
				}
			},
			wantReason: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mockGit := tt.setupGit()

			cmd := &CleanCommand{
				Git: &GitRunner{Executor: mockGit},
			}

			got := cmd.checkPrunableSkipReason(tt.branch, tt.target, tt.force)

			if got != tt.wantReason {
				t.Errorf("got %q, want %q", got, tt.wantReason)
			}
		})
	}
}

func TestCleanCommand_CheckSkipReason(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		wt         Worktree
		cwd        string
		target     string
		force      WorktreeForceLevel
		setupGit   func() *testutil.MockGitExecutor
		wantReason SkipReason
	}{
		// Basic cases (no force)
		{
			name:   "no_skip_for_valid_candidate",
			wt:     Worktree{Path: "/repo/feat/a", Branch: "feat/a"},
			cwd:    "/other/dir",
			target: "main",
			force:  WorktreeForceLevelNone,
			setupGit: func() *testutil.MockGitExecutor {
				return &testutil.MockGitExecutor{
					MergedBranches: map[string][]string{
						"main": {"feat/a"},
					},
				}
			},
			wantReason: "",
		},
		{
			name:       "skip_detached",
			wt:         Worktree{Path: "/repo/feat/a", Detached: true},
			cwd:        "/other/dir",
			target:     "main",
			force:      WorktreeForceLevelNone,
			setupGit:   func() *testutil.MockGitExecutor { return &testutil.MockGitExecutor{} },
			wantReason: SkipDetached,
		},
		{
			name:       "skip_locked",
			wt:         Worktree{Path: "/repo/feat/a", Branch: "feat/a", Locked: true},
			cwd:        "/other/dir",
			target:     "main",
			force:      WorktreeForceLevelNone,
			setupGit:   func() *testutil.MockGitExecutor { return &testutil.MockGitExecutor{} },
			wantReason: SkipLocked,
		},
		{
			name:       "skip_current_dir",
			wt:         Worktree{Path: "/repo/feat/a", Branch: "feat/a"},
			cwd:        "/repo/feat/a/subdir",
			target:     "main",
			force:      WorktreeForceLevelNone,
			setupGit:   func() *testutil.MockGitExecutor { return &testutil.MockGitExecutor{} },
			wantReason: SkipCurrentDir,
		},
		{
			name:   "skip_has_changes",
			wt:     Worktree{Path: "/repo/feat/a", Branch: "feat/a"},
			cwd:    "/other/dir",
			target: "main",
			force:  WorktreeForceLevelNone,
			setupGit: func() *testutil.MockGitExecutor {
				return &testutil.MockGitExecutor{
					HasChanges: true,
				}
			},
			wantReason: SkipHasChanges,
		},
		{
			name:   "skip_not_merged",
			wt:     Worktree{Path: "/repo/feat/a", Branch: "feat/a"},
			cwd:    "/other/dir",
			target: "main",
			force:  WorktreeForceLevelNone,
			setupGit: func() *testutil.MockGitExecutor {
				return &testutil.MockGitExecutor{
					MergedBranches: map[string][]string{
						"main": {},
					},
				}
			},
			wantReason: SkipNotMerged,
		},
		// Force level: Unclean (-f)
		{
			name:   "force_unclean_bypasses_has_changes",
			wt:     Worktree{Path: "/repo/feat/a", Branch: "feat/a"},
			cwd:    "/other/dir",
			target: "main",
			force:  WorktreeForceLevelUnclean,
			setupGit: func() *testutil.MockGitExecutor {
				return &testutil.MockGitExecutor{
					HasChanges: true,
				}
			},
			wantReason: "",
		},
		{
			name:   "force_unclean_bypasses_not_merged",
			wt:     Worktree{Path: "/repo/feat/a", Branch: "feat/a"},
			cwd:    "/other/dir",
			target: "main",
			force:  WorktreeForceLevelUnclean,
			setupGit: func() *testutil.MockGitExecutor {
				return &testutil.MockGitExecutor{
					MergedBranches: map[string][]string{
						"main": {},
					},
				}
			},
			wantReason: "",
		},
		{
			name:       "force_unclean_does_not_bypass_locked",
			wt:         Worktree{Path: "/repo/feat/a", Branch: "feat/a", Locked: true},
			cwd:        "/other/dir",
			target:     "main",
			force:      WorktreeForceLevelUnclean,
			setupGit:   func() *testutil.MockGitExecutor { return &testutil.MockGitExecutor{} },
			wantReason: SkipLocked,
		},
		// Force level: Locked (-ff)
		{
			name:       "force_locked_bypasses_locked",
			wt:         Worktree{Path: "/repo/feat/a", Branch: "feat/a", Locked: true},
			cwd:        "/other/dir",
			target:     "main",
			force:      WorktreeForceLevelLocked,
			setupGit:   func() *testutil.MockGitExecutor { return &testutil.MockGitExecutor{} },
			wantReason: "",
		},
		{
			name:   "force_locked_bypasses_has_changes",
			wt:     Worktree{Path: "/repo/feat/a", Branch: "feat/a"},
			cwd:    "/other/dir",
			target: "main",
			force:  WorktreeForceLevelLocked,
			setupGit: func() *testutil.MockGitExecutor {
				return &testutil.MockGitExecutor{
					HasChanges: true,
				}
			},
			wantReason: "",
		},
		// Never bypassed (even with -ff)
		{
			name:       "force_locked_does_not_bypass_detached",
			wt:         Worktree{Path: "/repo/feat/a", Detached: true},
			cwd:        "/other/dir",
			target:     "main",
			force:      WorktreeForceLevelLocked,
			setupGit:   func() *testutil.MockGitExecutor { return &testutil.MockGitExecutor{} },
			wantReason: SkipDetached,
		},
		{
			name:       "force_locked_does_not_bypass_current_dir",
			wt:         Worktree{Path: "/repo/feat/a", Branch: "feat/a"},
			cwd:        "/repo/feat/a/subdir",
			target:     "main",
			force:      WorktreeForceLevelLocked,
			setupGit:   func() *testutil.MockGitExecutor { return &testutil.MockGitExecutor{} },
			wantReason: SkipCurrentDir,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mockGit := tt.setupGit()

			cmd := &CleanCommand{
				Git: &GitRunner{Executor: mockGit},
			}

			got := cmd.checkSkipReason(tt.wt, tt.cwd, tt.target, tt.force)

			if got != tt.wantReason {
				t.Errorf("got %q, want %q", got, tt.wantReason)
			}
		})
	}
}

func TestCleanCommand_IntegrityInfo(t *testing.T) {
	t.Parallel()

	t.Run("detects_orphan_branches", func(t *testing.T) {
		t.Parallel()

		mockGit := &testutil.MockGitExecutor{
			Worktrees: []testutil.MockWorktree{
				{Path: "/repo/main", Branch: "main"},
				{Path: "/repo/feat/a", Branch: "feat/a"},
			},
			AllLocalBranches: []string{"main", "feat/a", "feat/orphan", "fix/abandoned"},
			MergedBranches: map[string][]string{
				"main": {"main", "feat/a"},
			},
		}

		cmd := &CleanCommand{
			FS:     &testutil.MockFS{},
			Git:    &GitRunner{Executor: mockGit},
			Config: &Config{WorktreeSourceDir: "/repo/main"},
		}

		result, err := cmd.Run("/other/dir", CleanOptions{Check: true})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(result.OrphanBranches) != 2 {
			t.Errorf("expected 2 orphan branches, got %d", len(result.OrphanBranches))
		}

		orphanNames := make(map[string]bool)
		for _, o := range result.OrphanBranches {
			orphanNames[o.Name] = true
		}

		if !orphanNames["feat/orphan"] {
			t.Error("expected feat/orphan to be detected as orphan")
		}
		if !orphanNames["fix/abandoned"] {
			t.Error("expected fix/abandoned to be detected as orphan")
		}
	})

	t.Run("detects_locked_worktrees", func(t *testing.T) {
		t.Parallel()

		mockGit := &testutil.MockGitExecutor{
			Worktrees: []testutil.MockWorktree{
				{Path: "/repo/main", Branch: "main"},
				{Path: "/repo/feat/a", Branch: "feat/a", Locked: true, LockReason: "USB drive work"},
				{Path: "/repo/feat/b", Branch: "feat/b", Locked: true},
			},
			MergedBranches: map[string][]string{
				"main": {"main"},
			},
		}

		cmd := &CleanCommand{
			FS:     &testutil.MockFS{},
			Git:    &GitRunner{Executor: mockGit},
			Config: &Config{WorktreeSourceDir: "/repo/main"},
		}

		result, err := cmd.Run("/other/dir", CleanOptions{Check: true})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(result.LockedWorktrees) != 2 {
			t.Errorf("expected 2 locked worktrees, got %d", len(result.LockedWorktrees))
		}

		// Check first locked worktree has reason
		found := false
		for _, l := range result.LockedWorktrees {
			if l.Branch == "feat/a" {
				found = true
				if l.LockReason != "USB drive work" {
					t.Errorf("expected lock reason 'USB drive work', got %q", l.LockReason)
				}
			}
		}
		if !found {
			t.Error("expected feat/a to be in locked worktrees")
		}
	})

	t.Run("detects_detached_worktrees", func(t *testing.T) {
		t.Parallel()

		mockGit := &testutil.MockGitExecutor{
			Worktrees: []testutil.MockWorktree{
				{Path: "/repo/main", Branch: "main"},
				{Path: "/repo/detached1", Detached: true, HEAD: "abc1234567890"},
				{Path: "/repo/detached2", Detached: true, HEAD: "def5678901234"},
			},
			MergedBranches: map[string][]string{
				"main": {"main"},
			},
		}

		cmd := &CleanCommand{
			FS:     &testutil.MockFS{},
			Git:    &GitRunner{Executor: mockGit},
			Config: &Config{WorktreeSourceDir: "/repo/main"},
		}

		result, err := cmd.Run("/other/dir", CleanOptions{Check: true})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(result.DetachedWorktrees) != 2 {
			t.Errorf("expected 2 detached worktrees, got %d", len(result.DetachedWorktrees))
		}

		// Check short HEAD is used
		for _, d := range result.DetachedWorktrees {
			if d.Path == "/repo/detached1" && d.HEAD != "abc1234" {
				t.Errorf("expected short HEAD 'abc1234', got %q", d.HEAD)
			}
		}
	})

	t.Run("integrity_info_collected_regardless_of_check_mode", func(t *testing.T) {
		t.Parallel()

		mockGit := &testutil.MockGitExecutor{
			Worktrees: []testutil.MockWorktree{
				{Path: "/repo/main", Branch: "main"},
				{Path: "/repo/feat/a", Branch: "feat/a", Locked: true},
			},
			AllLocalBranches: []string{"main", "feat/a", "orphan"},
			MergedBranches: map[string][]string{
				"main": {"main", "feat/a"},
			},
		}

		cmd := &CleanCommand{
			FS:     &testutil.MockFS{},
			Git:    &GitRunner{Executor: mockGit},
			Config: &Config{WorktreeSourceDir: "/repo/main"},
		}

		// Without Check mode - integrity info should still be collected
		result, err := cmd.Run("/other/dir", CleanOptions{Check: false})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(result.OrphanBranches) != 1 {
			t.Errorf("expected 1 orphan branch, got %d", len(result.OrphanBranches))
		}
		if len(result.LockedWorktrees) != 1 {
			t.Errorf("expected 1 locked worktree, got %d", len(result.LockedWorktrees))
		}
	})
}

func TestCleanResult_Format_IntegrityInfo(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		result     CleanResult
		opts       FormatOptions
		wantStdout string
	}{
		{
			name: "hides_detached_worktrees_by_default",
			result: CleanResult{
				Check: true,
				DetachedWorktrees: []DetachedWorktreeInfo{
					{Path: "/repo/detached", HEAD: "abc1234"},
				},
			},
			opts:       FormatOptions{},
			wantStdout: "No worktrees to clean\n",
		},
		{
			name: "shows_detached_worktrees_in_verbose",
			result: CleanResult{
				Check: true,
				DetachedWorktrees: []DetachedWorktreeInfo{
					{Path: "/repo/detached", HEAD: "abc1234"},
				},
			},
			opts:       FormatOptions{Verbose: true},
			wantStdout: "\ndetached:\n  /repo/detached (HEAD at abc1234)\n\nNo worktrees to clean\n",
		},
		{
			name: "shows_locked_worktrees_in_verbose",
			result: CleanResult{
				Check: true,
				LockedWorktrees: []LockedWorktreeInfo{
					{Branch: "feat/a", Path: "/repo/feat/a", LockReason: "USB drive"},
					{Branch: "feat/b", Path: "/repo/feat/b"},
				},
			},
			opts:       FormatOptions{Verbose: true},
			wantStdout: "\nlocked:\n  feat/a (reason: USB drive)\n  feat/b (no reason)\n\nNo worktrees to clean\n",
		},
		{
			name: "hides_locked_worktrees_by_default",
			result: CleanResult{
				Check: true,
				LockedWorktrees: []LockedWorktreeInfo{
					{Branch: "feat/a", Path: "/repo/feat/a", LockReason: "USB drive"},
				},
			},
			opts:       FormatOptions{},
			wantStdout: "No worktrees to clean\n",
		},
		{
			name: "shows_orphan_branches_in_verbose",
			result: CleanResult{
				Check: true,
				OrphanBranches: []OrphanBranch{
					{Name: "feat/orphan"},
					{Name: "fix/abandoned"},
				},
			},
			opts:       FormatOptions{Verbose: true},
			wantStdout: "\norphan branches:\n  feat/orphan (no worktree)\n  fix/abandoned (no worktree)\n\nNo worktrees to clean\n",
		},
		{
			name: "hides_orphan_branches_by_default",
			result: CleanResult{
				Check: true,
				OrphanBranches: []OrphanBranch{
					{Name: "feat/orphan"},
				},
			},
			opts:       FormatOptions{},
			wantStdout: "No worktrees to clean\n",
		},
		{
			name: "combined_with_cleanable_candidates_no_verbose",
			result: CleanResult{
				Candidates: []CleanCandidate{
					{Branch: "feat/a", CleanReason: CleanMerged},
				},
				DetachedWorktrees: []DetachedWorktreeInfo{
					{Path: "/repo/detached", HEAD: "abc1234"},
				},
				Check: true,
			},
			opts:       FormatOptions{},
			wantStdout: "clean:\n  feat/a (merged)\n",
		},
		{
			name: "combined_with_cleanable_candidates_verbose",
			result: CleanResult{
				Candidates: []CleanCandidate{
					{Branch: "feat/a", CleanReason: CleanMerged},
				},
				DetachedWorktrees: []DetachedWorktreeInfo{
					{Path: "/repo/detached", HEAD: "abc1234"},
				},
				Check: true,
			},
			opts:       FormatOptions{Verbose: true},
			wantStdout: "clean:\n  feat/a (merged)\n\ndetached:\n  /repo/detached (HEAD at abc1234)\n",
		},
		{
			name: "all_integrity_info_verbose",
			result: CleanResult{
				Check: true,
				DetachedWorktrees: []DetachedWorktreeInfo{
					{Path: "/repo/detached", HEAD: "abc1234"},
				},
				LockedWorktrees: []LockedWorktreeInfo{
					{Branch: "feat/locked", LockReason: "USB"},
				},
				OrphanBranches: []OrphanBranch{
					{Name: "feat/orphan"},
				},
			},
			opts:       FormatOptions{Verbose: true},
			wantStdout: "\ndetached:\n  /repo/detached (HEAD at abc1234)\n\nlocked:\n  feat/locked (reason: USB)\n\norphan branches:\n  feat/orphan (no worktree)\n\nNo worktrees to clean\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tt.result.Format(tt.opts)
			if got.Stdout != tt.wantStdout {
				t.Errorf("Stdout = %q, want %q", got.Stdout, tt.wantStdout)
			}
		})
	}
}
