package twig

import (
	"errors"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/708u/twig/internal/testutil"
)

func TestRemoveResult_HasErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		result RemoveResult
		want   bool
	}{
		{
			name:   "no_errors",
			result: RemoveResult{},
			want:   false,
		},
		{
			name: "success_only",
			result: RemoveResult{
				Removed: []RemovedWorktree{{Branch: "feature/a"}},
			},
			want: false,
		},
		{
			name: "error_only",
			result: RemoveResult{
				Removed: []RemovedWorktree{{Branch: "feature/b", Err: errors.New("failed")}},
			},
			want: true,
		},
		{
			name: "mixed",
			result: RemoveResult{
				Removed: []RemovedWorktree{
					{Branch: "feature/a"},
					{Branch: "feature/b", Err: errors.New("failed")},
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.result.HasErrors(); got != tt.want {
				t.Errorf("HasErrors() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRemoveResult_Format(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		result     RemoveResult
		opts       FormatOptions
		wantStdout string
		wantStderr string
	}{
		{
			name:       "empty",
			result:     RemoveResult{},
			opts:       FormatOptions{},
			wantStdout: "",
			wantStderr: "",
		},
		{
			name: "single_success",
			result: RemoveResult{
				Removed: []RemovedWorktree{{Branch: "feature/a", WorktreePath: "/repo/feature/a"}},
			},
			opts:       FormatOptions{},
			wantStdout: "twig remove: feature/a\n",
			wantStderr: "",
		},
		{
			name: "multiple_success",
			result: RemoveResult{
				Removed: []RemovedWorktree{
					{Branch: "feature/a", WorktreePath: "/repo/feature/a"},
					{Branch: "feature/b", WorktreePath: "/repo/feature/b"},
				},
			},
			opts:       FormatOptions{},
			wantStdout: "twig remove: feature/a\ntwig remove: feature/b\n",
			wantStderr: "",
		},
		{
			name: "single_error",
			result: RemoveResult{
				Removed: []RemovedWorktree{{Branch: "feature/c", Err: errors.New("not found")}},
			},
			opts:       FormatOptions{},
			wantStdout: "",
			wantStderr: "error: feature/c: not found\n",
		},
		{
			name: "mixed_success_and_error",
			result: RemoveResult{
				Removed: []RemovedWorktree{
					{Branch: "feature/a", WorktreePath: "/repo/feature/a"},
					{Branch: "feature/b", Err: errors.New("failed")},
				},
			},
			opts:       FormatOptions{},
			wantStdout: "twig remove: feature/a\n",
			wantStderr: "error: feature/b: failed\n",
		},
		{
			name: "dry_run",
			result: RemoveResult{
				Removed: []RemovedWorktree{{Branch: "feature/a", WorktreePath: "/repo/feature/a", Check: true}},
			},
			opts:       FormatOptions{},
			wantStdout: "Would remove worktree: /repo/feature/a\nWould delete branch: feature/a\n",
			wantStderr: "",
		},
		{
			name: "prunable_success",
			result: RemoveResult{
				Removed: []RemovedWorktree{{Branch: "feature/deleted", WorktreePath: "/repo/feature/deleted", Pruned: true}},
			},
			opts:       FormatOptions{},
			wantStdout: "twig remove: feature/deleted\n",
			wantStderr: "",
		},
		{
			name: "prunable_dry_run",
			result: RemoveResult{
				Removed: []RemovedWorktree{{Branch: "feature/deleted", WorktreePath: "/repo/feature/deleted", Pruned: true, Check: true}},
			},
			opts:       FormatOptions{},
			wantStdout: "Would prune stale worktree record\nWould delete branch: feature/deleted\n",
			wantStderr: "",
		},
		{
			name: "prunable_verbose",
			result: RemoveResult{
				Removed: []RemovedWorktree{{Branch: "feature/deleted", WorktreePath: "/repo/feature/deleted", Pruned: true}},
			},
			opts:       FormatOptions{Verbose: true},
			wantStdout: "Pruned stale worktree and deleted branch: feature/deleted\ntwig remove: feature/deleted\n",
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

func TestRemoveCommand_Run(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		branch         string
		cwd            string
		opts           RemoveOptions
		config         *Config
		setupGit       func(t *testing.T, captured *[]string) *testutil.MockGitExecutor
		wantErr        bool
		errContains    string
		wantForceLevel WorktreeForceLevel
		wantCheck      bool
	}{
		{
			name:   "success",
			branch: "feature/test",
			cwd:    "/other/dir",
			opts:   RemoveOptions{},
			config: &Config{WorktreeSourceDir: "/repo/main"},
			setupGit: func(t *testing.T, captured *[]string) *testutil.MockGitExecutor {
				t.Helper()
				return &testutil.MockGitExecutor{
					Worktrees:    []testutil.MockWorktree{{Path: "/repo/feature/test", Branch: "feature/test"}},
					CapturedArgs: captured,
				}
			},
			wantErr: false,
		},
		{
			name:   "dry_run",
			branch: "feature/test",
			cwd:    "/other/dir",
			opts:   RemoveOptions{Check: true},
			config: &Config{WorktreeSourceDir: "/repo/main"},
			setupGit: func(t *testing.T, captured *[]string) *testutil.MockGitExecutor {
				t.Helper()
				return &testutil.MockGitExecutor{
					Worktrees:    []testutil.MockWorktree{{Path: "/repo/feature/test", Branch: "feature/test"}},
					CapturedArgs: captured,
				}
			},
			wantErr:   false,
			wantCheck: true,
		},
		{
			name:   "force_level_unclean",
			branch: "feature/test",
			cwd:    "/other/dir",
			opts:   RemoveOptions{Force: WorktreeForceLevelUnclean},
			config: &Config{WorktreeSourceDir: "/repo/main"},
			setupGit: func(t *testing.T, captured *[]string) *testutil.MockGitExecutor {
				t.Helper()
				return &testutil.MockGitExecutor{
					Worktrees:    []testutil.MockWorktree{{Path: "/repo/feature/test", Branch: "feature/test"}},
					CapturedArgs: captured,
				}
			},
			wantErr:        false,
			wantForceLevel: WorktreeForceLevelUnclean,
		},
		{
			name:   "force_level_locked",
			branch: "feature/test",
			cwd:    "/other/dir",
			opts:   RemoveOptions{Force: WorktreeForceLevelLocked},
			config: &Config{WorktreeSourceDir: "/repo/main"},
			setupGit: func(t *testing.T, captured *[]string) *testutil.MockGitExecutor {
				t.Helper()
				return &testutil.MockGitExecutor{
					Worktrees:    []testutil.MockWorktree{{Path: "/repo/feature/test", Branch: "feature/test"}},
					CapturedArgs: captured,
				}
			},
			wantErr:        false,
			wantForceLevel: WorktreeForceLevelLocked,
		},
		{
			name:   "empty_branch",
			branch: "",
			cwd:    "/other/dir",
			opts:   RemoveOptions{},
			config: &Config{WorktreeSourceDir: "/repo/main"},
			setupGit: func(t *testing.T, captured *[]string) *testutil.MockGitExecutor {
				t.Helper()
				return &testutil.MockGitExecutor{}
			},
			wantErr:     true,
			errContains: "branch name is required",
		},
		{
			name:   "no_source_dir",
			branch: "feature/test",
			cwd:    "/other/dir",
			opts:   RemoveOptions{},
			config: &Config{},
			setupGit: func(t *testing.T, captured *[]string) *testutil.MockGitExecutor {
				t.Helper()
				return &testutil.MockGitExecutor{}
			},
			wantErr:     true,
			errContains: "worktree source directory is not configured",
		},
		{
			name:   "branch_not_in_worktree",
			branch: "orphan-branch",
			cwd:    "/other/dir",
			opts:   RemoveOptions{},
			config: &Config{WorktreeSourceDir: "/repo/main"},
			setupGit: func(t *testing.T, captured *[]string) *testutil.MockGitExecutor {
				t.Helper()
				return &testutil.MockGitExecutor{
					Worktrees: []testutil.MockWorktree{{Path: "/repo/main", Branch: "main"}},
				}
			},
			wantErr:     true,
			errContains: "is not checked out in any worktree",
		},
		{
			name:   "inside_worktree",
			branch: "feature/test",
			cwd:    "/repo/feature/test/subdir",
			opts:   RemoveOptions{},
			config: &Config{WorktreeSourceDir: "/repo/main"},
			setupGit: func(t *testing.T, captured *[]string) *testutil.MockGitExecutor {
				t.Helper()
				return &testutil.MockGitExecutor{
					Worktrees: []testutil.MockWorktree{{Path: "/repo/feature/test", Branch: "feature/test"}},
				}
			},
			wantErr:     true,
			errContains: "cannot remove: current directory",
		},
		{
			name:   "worktree_remove_fails",
			branch: "feature/test",
			cwd:    "/other/dir",
			opts:   RemoveOptions{},
			config: &Config{WorktreeSourceDir: "/repo/main"},
			setupGit: func(t *testing.T, captured *[]string) *testutil.MockGitExecutor {
				t.Helper()
				return &testutil.MockGitExecutor{
					Worktrees:         []testutil.MockWorktree{{Path: "/repo/feature/test", Branch: "feature/test"}},
					WorktreeRemoveErr: errors.New("has uncommitted changes"),
				}
			},
			wantErr:     true,
			errContains: "failed to remove worktree",
		},
		{
			name:   "branch_delete_fails",
			branch: "feature/test",
			cwd:    "/other/dir",
			opts:   RemoveOptions{},
			config: &Config{WorktreeSourceDir: "/repo/main"},
			setupGit: func(t *testing.T, captured *[]string) *testutil.MockGitExecutor {
				t.Helper()
				return &testutil.MockGitExecutor{
					Worktrees:       []testutil.MockWorktree{{Path: "/repo/feature/test", Branch: "feature/test"}},
					BranchDeleteErr: errors.New("branch not fully merged"),
				}
			},
			wantErr:     true,
			errContains: "failed to delete branch",
		},
		{
			name:   "prunable_worktree_success",
			branch: "feature/deleted",
			cwd:    "/other/dir",
			opts:   RemoveOptions{},
			config: &Config{WorktreeSourceDir: "/repo/main"},
			setupGit: func(t *testing.T, captured *[]string) *testutil.MockGitExecutor {
				t.Helper()
				return &testutil.MockGitExecutor{
					Worktrees: []testutil.MockWorktree{{
						Path:     "/repo/feature/deleted",
						Branch:   "feature/deleted",
						Prunable: true,
					}},
					CapturedArgs: captured,
				}
			},
			wantErr: false,
		},
		{
			name:   "prunable_worktree_dry_run",
			branch: "feature/deleted",
			cwd:    "/other/dir",
			opts:   RemoveOptions{Check: true},
			config: &Config{WorktreeSourceDir: "/repo/main"},
			setupGit: func(t *testing.T, captured *[]string) *testutil.MockGitExecutor {
				t.Helper()
				return &testutil.MockGitExecutor{
					Worktrees: []testutil.MockWorktree{{
						Path:     "/repo/feature/deleted",
						Branch:   "feature/deleted",
						Prunable: true,
					}},
					CapturedArgs: captured,
				}
			},
			wantErr:   false,
			wantCheck: true,
		},
		{
			name:   "prunable_worktree_with_force",
			branch: "feature/deleted",
			cwd:    "/other/dir",
			opts:   RemoveOptions{Force: WorktreeForceLevelUnclean},
			config: &Config{WorktreeSourceDir: "/repo/main"},
			setupGit: func(t *testing.T, captured *[]string) *testutil.MockGitExecutor {
				t.Helper()
				return &testutil.MockGitExecutor{
					Worktrees: []testutil.MockWorktree{{
						Path:     "/repo/feature/deleted",
						Branch:   "feature/deleted",
						Prunable: true,
					}},
					CapturedArgs: captured,
				}
			},
			wantErr: false,
		},
		{
			name:   "prunable_worktree_prune_fails",
			branch: "feature/deleted",
			cwd:    "/other/dir",
			opts:   RemoveOptions{},
			config: &Config{WorktreeSourceDir: "/repo/main"},
			setupGit: func(t *testing.T, captured *[]string) *testutil.MockGitExecutor {
				t.Helper()
				return &testutil.MockGitExecutor{
					Worktrees: []testutil.MockWorktree{{
						Path:     "/repo/feature/deleted",
						Branch:   "feature/deleted",
						Prunable: true,
					}},
					WorktreePruneErr: errors.New("prune failed"),
				}
			},
			wantErr:     true,
			errContains: "failed to prune worktrees",
		},
		{
			name:   "prunable_worktree_branch_delete_fails",
			branch: "feature/deleted",
			cwd:    "/other/dir",
			opts:   RemoveOptions{},
			config: &Config{WorktreeSourceDir: "/repo/main"},
			setupGit: func(t *testing.T, captured *[]string) *testutil.MockGitExecutor {
				t.Helper()
				return &testutil.MockGitExecutor{
					Worktrees: []testutil.MockWorktree{{
						Path:     "/repo/feature/deleted",
						Branch:   "feature/deleted",
						Prunable: true,
					}},
					BranchDeleteErr: errors.New("branch not fully merged"),
				}
			},
			wantErr:     true,
			errContains: "failed to delete branch",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var captured []string

			mockGit := tt.setupGit(t, &captured)

			cmd := &RemoveCommand{
				FS:     &testutil.MockFS{},
				Git:    &GitRunner{Executor: mockGit},
				Config: tt.config,
			}

			result, err := cmd.Run(tt.branch, tt.cwd, tt.opts)

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

			if tt.wantCheck {
				if !result.Check {
					t.Error("expected Check to be true")
				}
				return
			}

			if tt.wantForceLevel > 0 {
				// Check that -f appears the expected number of times for worktree removal
				fCount := 0
				for _, arg := range captured {
					if arg == "-f" {
						fCount++
					}
				}
				if WorktreeForceLevel(fCount) != tt.wantForceLevel {
					t.Errorf("expected -f %d time(s) for worktree removal, got %d in: %v", tt.wantForceLevel, fCount, captured)
				}
				// Also check -D for branch deletion
				if !slices.Contains(captured, "-D") {
					t.Errorf("expected -D for force branch deletion, got: %v", captured)
				}
			}
		})
	}
}

func TestRemoveCommand_CleanupEmptyParentDirs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		wtPath         string
		baseDir        string
		dirContents    map[string][]os.DirEntry
		removedDirs    []string
		wantCleanedLen int
	}{
		{
			name:           "single_level_no_cleanup",
			wtPath:         "/base/feature",
			baseDir:        "/base",
			dirContents:    map[string][]os.DirEntry{},
			wantCleanedLen: 0,
		},
		{
			name:    "multi_level_empty_parent",
			wtPath:  "/base/feat/test",
			baseDir: "/base",
			dirContents: map[string][]os.DirEntry{
				"/base/feat": {}, // empty
			},
			wantCleanedLen: 1,
		},
		{
			name:    "multi_level_non_empty_parent",
			wtPath:  "/base/feat/test",
			baseDir: "/base",
			dirContents: map[string][]os.DirEntry{
				"/base/feat": {mockDirEntry{name: "other"}},
			},
			wantCleanedLen: 0,
		},
		{
			name:    "deeply_nested_all_empty",
			wtPath:  "/base/a/b/c",
			baseDir: "/base",
			dirContents: map[string][]os.DirEntry{
				"/base/a/b": {},
				"/base/a":   {},
			},
			wantCleanedLen: 2,
		},
		{
			name:    "deeply_nested_partial_empty",
			wtPath:  "/base/a/b/c",
			baseDir: "/base",
			dirContents: map[string][]os.DirEntry{
				"/base/a/b": {},
				"/base/a":   {mockDirEntry{name: "other"}},
			},
			wantCleanedLen: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mockFS := &testutil.MockFS{
				DirContents: tt.dirContents,
			}

			cmd := &RemoveCommand{
				FS:     mockFS,
				Config: &Config{WorktreeDestBaseDir: tt.baseDir},
			}

			cleaned := cmd.cleanupEmptyParentDirs(tt.wtPath)

			if len(cleaned) != tt.wantCleanedLen {
				t.Errorf("cleaned = %v, want length %d", cleaned, tt.wantCleanedLen)
			}
		})
	}
}

func TestRemoveCommand_PredictEmptyParentDirs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		wtPath         string
		baseDir        string
		dirContents    map[string][]os.DirEntry
		wantCleanedLen int
	}{
		{
			name:    "predicts_empty_after_removal",
			wtPath:  "/base/feat/test",
			baseDir: "/base",
			dirContents: map[string][]os.DirEntry{
				"/base/feat": {mockDirEntry{name: "test"}}, // only the worktree itself
			},
			wantCleanedLen: 1,
		},
		{
			name:    "predicts_non_empty_with_sibling",
			wtPath:  "/base/feat/test",
			baseDir: "/base",
			dirContents: map[string][]os.DirEntry{
				"/base/feat": {mockDirEntry{name: "test"}, mockDirEntry{name: "other"}},
			},
			wantCleanedLen: 0,
		},
		{
			name:    "deeply_nested_prediction",
			wtPath:  "/base/a/b/c",
			baseDir: "/base",
			dirContents: map[string][]os.DirEntry{
				"/base/a/b": {mockDirEntry{name: "c"}},
				"/base/a":   {mockDirEntry{name: "b"}},
			},
			wantCleanedLen: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mockFS := &testutil.MockFS{
				DirContents: tt.dirContents,
			}

			cmd := &RemoveCommand{
				FS:     mockFS,
				Config: &Config{WorktreeDestBaseDir: tt.baseDir},
			}

			predicted := cmd.predictEmptyParentDirs(tt.wtPath)

			if len(predicted) != tt.wantCleanedLen {
				t.Errorf("predicted = %v, want length %d", predicted, tt.wantCleanedLen)
			}
		})
	}
}

func TestRemovedWorktree_Format_WithCleanedDirs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		result     RemovedWorktree
		opts       FormatOptions
		wantStdout string
	}{
		{
			name: "dry_run_with_cleaned_dirs",
			result: RemovedWorktree{
				Branch:       "feat/test",
				WorktreePath: "/base/feat/test",
				CleanedDirs:  []string{"/base/feat"},
				Check:        true,
			},
			opts: FormatOptions{},
			wantStdout: "Would remove worktree: /base/feat/test\n" +
				"Would delete branch: feat/test\n" +
				"Would remove empty directory: /base/feat\n",
		},
		{
			name: "verbose_with_cleaned_dirs",
			result: RemovedWorktree{
				Branch:       "feat/test",
				WorktreePath: "/base/feat/test",
				CleanedDirs:  []string{"/base/feat"},
				Check:        false,
			},
			opts: FormatOptions{Verbose: true},
			wantStdout: "Removed worktree and branch: feat/test\n" +
				"Removed empty directory: /base/feat\n" +
				"twig remove: feat/test\n",
		},
		{
			name: "normal_with_cleaned_dirs_not_shown",
			result: RemovedWorktree{
				Branch:       "feat/test",
				WorktreePath: "/base/feat/test",
				CleanedDirs:  []string{"/base/feat"},
				Check:        false,
			},
			opts:       FormatOptions{Verbose: false},
			wantStdout: "twig remove: feat/test\n",
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

// mockDirEntry implements os.DirEntry for testing.
type mockDirEntry struct {
	name  string
	isDir bool
}

func (m mockDirEntry) Name() string               { return m.name }
func (m mockDirEntry) IsDir() bool                { return m.isDir }
func (m mockDirEntry) Type() os.FileMode          { return 0 }
func (m mockDirEntry) Info() (os.FileInfo, error) { return nil, nil }

func TestRemoveResult_Format_WithHint(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		result     RemoveResult
		opts       FormatOptions
		wantStderr string
	}{
		{
			name: "git_error_with_hint_normal",
			result: RemoveResult{
				Removed: []RemovedWorktree{{
					Branch: "feature/a",
					Err: &GitError{
						Op:     OpWorktreeRemove,
						Stderr: "fatal: '/path' contains modified or untracked files",
					},
				}},
			},
			opts:       FormatOptions{Verbose: false},
			wantStderr: "error: feature/a: failed to remove worktree\nhint: use 'twig remove --force' to force removal\n",
		},
		{
			name: "git_error_with_hint_verbose",
			result: RemoveResult{
				Removed: []RemovedWorktree{{
					Branch: "feature/a",
					Err: &GitError{
						Op:     OpWorktreeRemove,
						Stderr: "fatal: '/path' contains modified or untracked files",
					},
				}},
			},
			opts:       FormatOptions{Verbose: true},
			wantStderr: "error: feature/a: failed to remove worktree\n       git: fatal: '/path' contains modified or untracked files\nhint: use 'twig remove --force' to force removal\n",
		},
		{
			name: "git_error_without_hint",
			result: RemoveResult{
				Removed: []RemovedWorktree{{
					Branch: "feature/a",
					Err: &GitError{
						Op:     OpWorktreeRemove,
						Stderr: "some other error",
					},
				}},
			},
			opts:       FormatOptions{Verbose: false},
			wantStderr: "error: feature/a: failed to remove worktree\n",
		},
		{
			name: "git_error_locked_worktree_hint",
			result: RemoveResult{
				Removed: []RemovedWorktree{{
					Branch: "feature/a",
					Err: &GitError{
						Op:     OpWorktreeRemove,
						Stderr: "fatal: cannot remove a locked working tree",
					},
				}},
			},
			opts:       FormatOptions{Verbose: false},
			wantStderr: "error: feature/a: failed to remove worktree\nhint: run 'git worktree unlock <path>' first, or use 'twig remove -f -f'\n",
		},
		{
			name: "non_git_error_fallback",
			result: RemoveResult{
				Removed: []RemovedWorktree{{
					Branch: "feature/a",
					Err:    errors.New("branch not found"),
				}},
			},
			opts:       FormatOptions{Verbose: false},
			wantStderr: "error: feature/a: branch not found\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tt.result.Format(tt.opts)
			if got.Stderr != tt.wantStderr {
				t.Errorf("Stderr = %q, want %q", got.Stderr, tt.wantStderr)
			}
		})
	}
}

func TestRemoveCommand_Check(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		branch        string
		opts          CheckOptions
		config        *Config
		setupGit      func() *testutil.MockGitExecutor
		wantCanRemove bool
		wantSkip      SkipReason
		wantClean     CleanReason
		wantErr       bool
		errContains   string
	}{
		// Basic success cases
		{
			name:   "can_remove_merged_branch",
			branch: "feat/a",
			opts: CheckOptions{
				Force:  WorktreeForceLevelNone,
				Target: "main",
				Cwd:    "/other/dir",
			},
			config: &Config{WorktreeSourceDir: "/repo/main"},
			setupGit: func() *testutil.MockGitExecutor {
				return &testutil.MockGitExecutor{
					Worktrees: []testutil.MockWorktree{
						{Path: "/repo/feat/a", Branch: "feat/a"},
					},
					MergedBranches: map[string][]string{
						"main": {"feat/a"},
					},
				}
			},
			wantCanRemove: true,
			wantClean:     CleanMerged,
		},
		{
			name:   "can_remove_upstream_gone_branch",
			branch: "feat/a",
			opts: CheckOptions{
				Force:  WorktreeForceLevelNone,
				Target: "main",
				Cwd:    "/other/dir",
			},
			config: &Config{WorktreeSourceDir: "/repo/main"},
			setupGit: func() *testutil.MockGitExecutor {
				return &testutil.MockGitExecutor{
					Worktrees: []testutil.MockWorktree{
						{Path: "/repo/feat/a", Branch: "feat/a"},
					},
					MergedBranches:       map[string][]string{"main": {}},
					UpstreamGoneBranches: []string{"feat/a"},
				}
			},
			wantCanRemove: true,
			wantClean:     CleanUpstreamGone,
		},
		// Skip cases
		// Note: Detached HEAD worktrees are handled directly in CleanCommand.Run
		// since they have no branch name and cannot be found by WorktreeFindByBranch.
		{
			name:   "skip_current_directory",
			branch: "feat/a",
			opts: CheckOptions{
				Force:  WorktreeForceLevelNone,
				Target: "main",
				Cwd:    "/repo/feat/a/subdir",
			},
			config: &Config{WorktreeSourceDir: "/repo/main"},
			setupGit: func() *testutil.MockGitExecutor {
				return &testutil.MockGitExecutor{
					Worktrees: []testutil.MockWorktree{
						{Path: "/repo/feat/a", Branch: "feat/a"},
					},
				}
			},
			wantCanRemove: false,
			wantSkip:      SkipCurrentDir,
		},
		{
			name:   "skip_locked",
			branch: "feat/a",
			opts: CheckOptions{
				Force:  WorktreeForceLevelNone,
				Target: "main",
				Cwd:    "/other/dir",
			},
			config: &Config{WorktreeSourceDir: "/repo/main"},
			setupGit: func() *testutil.MockGitExecutor {
				return &testutil.MockGitExecutor{
					Worktrees: []testutil.MockWorktree{
						{Path: "/repo/feat/a", Branch: "feat/a", Locked: true},
					},
				}
			},
			wantCanRemove: false,
			wantSkip:      SkipLocked,
		},
		{
			name:   "skip_has_changes",
			branch: "feat/a",
			opts: CheckOptions{
				Force:  WorktreeForceLevelNone,
				Target: "main",
				Cwd:    "/other/dir",
			},
			config: &Config{WorktreeSourceDir: "/repo/main"},
			setupGit: func() *testutil.MockGitExecutor {
				return &testutil.MockGitExecutor{
					Worktrees: []testutil.MockWorktree{
						{Path: "/repo/feat/a", Branch: "feat/a"},
					},
					HasChanges: true,
				}
			},
			wantCanRemove: false,
			wantSkip:      SkipHasChanges,
		},
		{
			name:   "skip_not_merged",
			branch: "feat/a",
			opts: CheckOptions{
				Force:  WorktreeForceLevelNone,
				Target: "main",
				Cwd:    "/other/dir",
			},
			config: &Config{WorktreeSourceDir: "/repo/main"},
			setupGit: func() *testutil.MockGitExecutor {
				return &testutil.MockGitExecutor{
					Worktrees: []testutil.MockWorktree{
						{Path: "/repo/feat/a", Branch: "feat/a"},
					},
					MergedBranches: map[string][]string{"main": {}},
				}
			},
			wantCanRemove: false,
			wantSkip:      SkipNotMerged,
		},
		{
			name:   "skip_dirty_submodule",
			branch: "feat/a",
			opts: CheckOptions{
				Force:  WorktreeForceLevelNone,
				Target: "main",
				Cwd:    "/other/dir",
			},
			config: &Config{WorktreeSourceDir: "/repo/main"},
			setupGit: func() *testutil.MockGitExecutor {
				return &testutil.MockGitExecutor{
					Worktrees: []testutil.MockWorktree{
						{Path: "/repo/feat/a", Branch: "feat/a"},
					},
					MergedBranches: map[string][]string{"main": {"feat/a"}},
					// + prefix indicates submodule is at different commit (modified)
					SubmoduleStatusOutput: "+abc123 submodule-path (v1.0.0)\n",
				}
			},
			wantCanRemove: false,
			wantSkip:      SkipDirtySubmodule,
		},
		// Force level: Unclean (-f)
		{
			name:   "force_unclean_bypasses_has_changes",
			branch: "feat/a",
			opts: CheckOptions{
				Force:  WorktreeForceLevelUnclean,
				Target: "main",
				Cwd:    "/other/dir",
			},
			config: &Config{WorktreeSourceDir: "/repo/main"},
			setupGit: func() *testutil.MockGitExecutor {
				return &testutil.MockGitExecutor{
					Worktrees: []testutil.MockWorktree{
						{Path: "/repo/feat/a", Branch: "feat/a"},
					},
					HasChanges: true,
				}
			},
			wantCanRemove: true,
		},
		{
			name:   "force_unclean_bypasses_not_merged",
			branch: "feat/a",
			opts: CheckOptions{
				Force:  WorktreeForceLevelUnclean,
				Target: "main",
				Cwd:    "/other/dir",
			},
			config: &Config{WorktreeSourceDir: "/repo/main"},
			setupGit: func() *testutil.MockGitExecutor {
				return &testutil.MockGitExecutor{
					Worktrees: []testutil.MockWorktree{
						{Path: "/repo/feat/a", Branch: "feat/a"},
					},
					MergedBranches: map[string][]string{"main": {}},
				}
			},
			wantCanRemove: true,
		},
		{
			name:   "force_unclean_bypasses_dirty_submodule",
			branch: "feat/a",
			opts: CheckOptions{
				Force:  WorktreeForceLevelUnclean,
				Target: "main",
				Cwd:    "/other/dir",
			},
			config: &Config{WorktreeSourceDir: "/repo/main"},
			setupGit: func() *testutil.MockGitExecutor {
				return &testutil.MockGitExecutor{
					Worktrees: []testutil.MockWorktree{
						{Path: "/repo/feat/a", Branch: "feat/a"},
					},
					// + prefix indicates submodule is at different commit (modified)
					SubmoduleStatusOutput: "+abc123 submodule-path (v1.0.0)\n",
				}
			},
			wantCanRemove: true,
		},
		{
			name:   "force_unclean_does_not_bypass_locked",
			branch: "feat/a",
			opts: CheckOptions{
				Force:  WorktreeForceLevelUnclean,
				Target: "main",
				Cwd:    "/other/dir",
			},
			config: &Config{WorktreeSourceDir: "/repo/main"},
			setupGit: func() *testutil.MockGitExecutor {
				return &testutil.MockGitExecutor{
					Worktrees: []testutil.MockWorktree{
						{Path: "/repo/feat/a", Branch: "feat/a", Locked: true},
					},
				}
			},
			wantCanRemove: false,
			wantSkip:      SkipLocked,
		},
		// Force level: Locked (-ff)
		{
			name:   "force_locked_bypasses_locked",
			branch: "feat/a",
			opts: CheckOptions{
				Force:  WorktreeForceLevelLocked,
				Target: "main",
				Cwd:    "/other/dir",
			},
			config: &Config{WorktreeSourceDir: "/repo/main"},
			setupGit: func() *testutil.MockGitExecutor {
				return &testutil.MockGitExecutor{
					Worktrees: []testutil.MockWorktree{
						{Path: "/repo/feat/a", Branch: "feat/a", Locked: true},
					},
				}
			},
			wantCanRemove: true,
		},
		// Never bypassed (even with -ff)
		// Note: Detached HEAD worktrees are handled directly in CleanCommand.Run.
		{
			name:   "force_locked_does_not_bypass_current_dir",
			branch: "feat/a",
			opts: CheckOptions{
				Force:  WorktreeForceLevelLocked,
				Target: "main",
				Cwd:    "/repo/feat/a/subdir",
			},
			config: &Config{WorktreeSourceDir: "/repo/main"},
			setupGit: func() *testutil.MockGitExecutor {
				return &testutil.MockGitExecutor{
					Worktrees: []testutil.MockWorktree{
						{Path: "/repo/feat/a", Branch: "feat/a"},
					},
				}
			},
			wantCanRemove: false,
			wantSkip:      SkipCurrentDir,
		},
		// Prunable worktree cases
		{
			name:   "prunable_can_remove_merged",
			branch: "feat/a",
			opts: CheckOptions{
				Force:  WorktreeForceLevelNone,
				Target: "main",
				Cwd:    "/other/dir",
			},
			config: &Config{WorktreeSourceDir: "/repo/main"},
			setupGit: func() *testutil.MockGitExecutor {
				return &testutil.MockGitExecutor{
					Worktrees: []testutil.MockWorktree{
						{Path: "/repo/feat/a", Branch: "feat/a", Prunable: true},
					},
					MergedBranches: map[string][]string{"main": {"feat/a"}},
				}
			},
			wantCanRemove: true,
			wantClean:     CleanMerged,
		},
		{
			name:   "prunable_skip_not_merged",
			branch: "feat/a",
			opts: CheckOptions{
				Force:  WorktreeForceLevelNone,
				Target: "main",
				Cwd:    "/other/dir",
			},
			config: &Config{WorktreeSourceDir: "/repo/main"},
			setupGit: func() *testutil.MockGitExecutor {
				return &testutil.MockGitExecutor{
					Worktrees: []testutil.MockWorktree{
						{Path: "/repo/feat/a", Branch: "feat/a", Prunable: true},
					},
					MergedBranches: map[string][]string{"main": {}},
				}
			},
			wantCanRemove: false,
			wantSkip:      SkipNotMerged,
		},
		{
			name:   "prunable_force_bypasses_not_merged",
			branch: "feat/a",
			opts: CheckOptions{
				Force:  WorktreeForceLevelUnclean,
				Target: "main",
				Cwd:    "/other/dir",
			},
			config: &Config{WorktreeSourceDir: "/repo/main"},
			setupGit: func() *testutil.MockGitExecutor {
				return &testutil.MockGitExecutor{
					Worktrees: []testutil.MockWorktree{
						{Path: "/repo/feat/a", Branch: "feat/a", Prunable: true},
					},
					MergedBranches: map[string][]string{"main": {}},
				}
			},
			wantCanRemove: true,
		},
		// No target specified (skip merged check)
		{
			name:   "no_target_skips_merged_check",
			branch: "feat/a",
			opts: CheckOptions{
				Force:  WorktreeForceLevelNone,
				Target: "",
				Cwd:    "/other/dir",
			},
			config: &Config{WorktreeSourceDir: "/repo/main"},
			setupGit: func() *testutil.MockGitExecutor {
				return &testutil.MockGitExecutor{
					Worktrees: []testutil.MockWorktree{
						{Path: "/repo/feat/a", Branch: "feat/a"},
					},
					MergedBranches: map[string][]string{"main": {}}, // not merged
				}
			},
			wantCanRemove: true,
			wantClean:     "", // no CleanReason when target is empty
		},
		// Error cases
		{
			name:   "empty_branch",
			branch: "",
			opts: CheckOptions{
				Force:  WorktreeForceLevelNone,
				Target: "main",
				Cwd:    "/other/dir",
			},
			config:      &Config{WorktreeSourceDir: "/repo/main"},
			setupGit:    func() *testutil.MockGitExecutor { return &testutil.MockGitExecutor{} },
			wantErr:     true,
			errContains: "branch name is required",
		},
		{
			name:   "no_source_dir",
			branch: "feat/a",
			opts: CheckOptions{
				Force:  WorktreeForceLevelNone,
				Target: "main",
				Cwd:    "/other/dir",
			},
			config:      &Config{},
			setupGit:    func() *testutil.MockGitExecutor { return &testutil.MockGitExecutor{} },
			wantErr:     true,
			errContains: "worktree source directory is not configured",
		},
		{
			name:   "branch_not_in_worktree",
			branch: "orphan-branch",
			opts: CheckOptions{
				Force:  WorktreeForceLevelNone,
				Target: "main",
				Cwd:    "/other/dir",
			},
			config: &Config{WorktreeSourceDir: "/repo/main"},
			setupGit: func() *testutil.MockGitExecutor {
				return &testutil.MockGitExecutor{
					Worktrees: []testutil.MockWorktree{
						{Path: "/repo/main", Branch: "main"},
					},
				}
			},
			wantErr:     true,
			errContains: "is not checked out in any worktree",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mockGit := tt.setupGit()

			cmd := &RemoveCommand{
				FS:     &testutil.MockFS{},
				Git:    &GitRunner{Executor: mockGit},
				Config: tt.config,
			}

			result, err := cmd.Check(tt.branch, tt.opts)

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

			if result.CanRemove != tt.wantCanRemove {
				t.Errorf("CanRemove = %v, want %v", result.CanRemove, tt.wantCanRemove)
			}
			if result.SkipReason != tt.wantSkip {
				t.Errorf("SkipReason = %q, want %q", result.SkipReason, tt.wantSkip)
			}
			if result.CleanReason != tt.wantClean {
				t.Errorf("CleanReason = %q, want %q", result.CleanReason, tt.wantClean)
			}
		})
	}
}

func TestRemovedWorktree_Format_VerboseChangedFiles(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		result     RemovedWorktree
		opts       FormatOptions
		wantStdout string
	}{
		{
			name: "check_verbose_with_changed_files",
			result: RemovedWorktree{
				Branch:       "feat/test",
				WorktreePath: "/base/feat/test",
				Check:        true,
				ChangedFiles: []FileStatus{
					{Status: " M", Path: "src/main.go"},
					{Status: "A ", Path: "src/new.go"},
					{Status: "??", Path: "tmp/debug.log"},
				},
			},
			opts: FormatOptions{Verbose: true},
			wantStdout: "Would remove worktree: /base/feat/test\n" +
				"Uncommitted changes:\n" +
				"   M src/main.go\n" +
				"  A  src/new.go\n" +
				"  ?? tmp/debug.log\n" +
				"Would delete branch: feat/test\n",
		},
		{
			name: "check_verbose_no_changed_files",
			result: RemovedWorktree{
				Branch:       "feat/test",
				WorktreePath: "/base/feat/test",
				Check:        true,
				ChangedFiles: nil,
			},
			opts: FormatOptions{Verbose: true},
			wantStdout: "Would remove worktree: /base/feat/test\n" +
				"Would delete branch: feat/test\n",
		},
		{
			name: "check_non_verbose_with_changed_files",
			result: RemovedWorktree{
				Branch:       "feat/test",
				WorktreePath: "/base/feat/test",
				Check:        true,
				ChangedFiles: []FileStatus{
					{Status: " M", Path: "src/main.go"},
				},
			},
			opts: FormatOptions{Verbose: false},
			wantStdout: "Would remove worktree: /base/feat/test\n" +
				"Would delete branch: feat/test\n",
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

func TestRemoveResult_Format_VerboseChangedFilesOnError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		result     RemoveResult
		opts       FormatOptions
		wantStderr string
	}{
		{
			name: "skip_has_changes_verbose",
			result: RemoveResult{
				Removed: []RemovedWorktree{{
					Branch: "feat/test",
					Err:    &SkipError{Reason: SkipHasChanges},
					ChangedFiles: []FileStatus{
						{Status: " M", Path: "src/main.go"},
						{Status: "??", Path: "tmp/debug.log"},
					},
				}},
			},
			opts: FormatOptions{Verbose: true},
			wantStderr: "error: feat/test: cannot remove: has uncommitted changes\n" +
				"Uncommitted changes:\n" +
				"   M src/main.go\n" +
				"  ?? tmp/debug.log\n" +
				"hint: use 'twig remove --force' to force removal\n",
		},
		{
			name: "skip_has_changes_non_verbose",
			result: RemoveResult{
				Removed: []RemovedWorktree{{
					Branch: "feat/test",
					Err:    &SkipError{Reason: SkipHasChanges},
					ChangedFiles: []FileStatus{
						{Status: " M", Path: "src/main.go"},
					},
				}},
			},
			opts: FormatOptions{Verbose: false},
			wantStderr: "error: feat/test: cannot remove: has uncommitted changes\n" +
				"hint: use 'twig remove --force' to force removal\n",
		},
		{
			name: "skip_not_merged_verbose_no_changed_files_shown",
			result: RemoveResult{
				Removed: []RemovedWorktree{{
					Branch: "feat/test",
					Err:    &SkipError{Reason: SkipNotMerged},
					ChangedFiles: []FileStatus{
						{Status: " M", Path: "src/main.go"},
					},
				}},
			},
			opts: FormatOptions{Verbose: true},
			wantStderr: "error: feat/test: cannot remove: not merged\n" +
				"hint: use 'twig remove --force' to force removal\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := tt.result.Format(tt.opts)
			if got.Stderr != tt.wantStderr {
				t.Errorf("Stderr = %q, want %q", got.Stderr, tt.wantStderr)
			}
		})
	}
}
