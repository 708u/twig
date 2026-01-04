package gwt

import (
	"errors"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/708u/gwt/internal/testutil"
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
			wantStdout: "gwt remove: feature/a\n",
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
			wantStdout: "gwt remove: feature/a\ngwt remove: feature/b\n",
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
			wantStdout: "gwt remove: feature/a\n",
			wantStderr: "error: feature/b: failed\n",
		},
		{
			name: "dry_run",
			result: RemoveResult{
				Removed: []RemovedWorktree{{Branch: "feature/a", WorktreePath: "/repo/feature/a", DryRun: true}},
			},
			opts:       FormatOptions{},
			wantStdout: "Would remove worktree: /repo/feature/a\nWould delete branch: feature/a\n",
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
		name        string
		branch      string
		cwd         string
		opts        RemoveOptions
		config      *Config
		setupGit    func(t *testing.T, captured *[]string) *testutil.MockGitExecutor
		wantErr     bool
		errContains string
		wantForce   bool
		wantDryRun  bool
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
			opts:   RemoveOptions{DryRun: true},
			config: &Config{WorktreeSourceDir: "/repo/main"},
			setupGit: func(t *testing.T, captured *[]string) *testutil.MockGitExecutor {
				t.Helper()
				return &testutil.MockGitExecutor{
					Worktrees:    []testutil.MockWorktree{{Path: "/repo/feature/test", Branch: "feature/test"}},
					CapturedArgs: captured,
				}
			},
			wantErr:    false,
			wantDryRun: true,
		},
		{
			name:   "force",
			branch: "feature/test",
			cwd:    "/other/dir",
			opts:   RemoveOptions{Force: true},
			config: &Config{WorktreeSourceDir: "/repo/main"},
			setupGit: func(t *testing.T, captured *[]string) *testutil.MockGitExecutor {
				t.Helper()
				return &testutil.MockGitExecutor{
					Worktrees:    []testutil.MockWorktree{{Path: "/repo/feature/test", Branch: "feature/test"}},
					CapturedArgs: captured,
				}
			},
			wantErr:   false,
			wantForce: true,
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
			errContains: "cannot remove: current directory is inside worktree",
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

			if tt.wantDryRun {
				if !result.DryRun {
					t.Error("expected DryRun to be true")
				}
				return
			}

			if tt.wantForce && !slices.Contains(captured, "-f") && !slices.Contains(captured, "-D") {
				t.Errorf("expected force flag (-f or -D), got: %v", captured)
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
				DryRun:       true,
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
				DryRun:       false,
			},
			opts: FormatOptions{Verbose: true},
			wantStdout: "Removed worktree and branch: feat/test\n" +
				"Removed empty directory: /base/feat\n" +
				"gwt remove: feat/test\n",
		},
		{
			name: "normal_with_cleaned_dirs_not_shown",
			result: RemovedWorktree{
				Branch:       "feat/test",
				WorktreePath: "/base/feat/test",
				CleanedDirs:  []string{"/base/feat"},
				DryRun:       false,
			},
			opts:       FormatOptions{Verbose: false},
			wantStdout: "gwt remove: feat/test\n",
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

func TestGitError_Hint(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		gitErr *GitError
		want   string
	}{
		{
			name:   "nil_error",
			gitErr: nil,
			want:   "",
		},
		{
			name: "modified_or_untracked_files",
			gitErr: &GitError{
				Op:     OpWorktreeRemove,
				Stderr: "fatal: '/path' contains modified or untracked files",
			},
			want: "use 'gwt remove --force' to force removal",
		},
		{
			name: "locked_worktree",
			gitErr: &GitError{
				Op:     OpWorktreeRemove,
				Stderr: "fatal: cannot remove a locked working tree",
			},
			want: "run 'git worktree unlock <path>' first, or use 'gwt remove --force'",
		},
		{
			name: "unknown_error",
			gitErr: &GitError{
				Op:     OpWorktreeRemove,
				Stderr: "some other error",
			},
			want: "",
		},
		{
			name: "empty_stderr",
			gitErr: &GitError{
				Op:     OpWorktreeRemove,
				Stderr: "",
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var got string
			if tt.gitErr != nil {
				got = tt.gitErr.Hint()
			}
			if got != tt.want {
				t.Errorf("Hint() = %q, want %q", got, tt.want)
			}
		})
	}
}

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
			wantStderr: "error: feature/a: failed to remove worktree\nhint: use 'gwt remove --force' to force removal\n",
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
			wantStderr: "error: feature/a: failed to remove worktree\n       git: fatal: '/path' contains modified or untracked files\nhint: use 'gwt remove --force' to force removal\n",
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
			wantStderr: "error: feature/a: failed to remove worktree\nhint: run 'git worktree unlock <path>' first, or use 'gwt remove --force'\n",
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
