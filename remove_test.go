package gwt

import (
	"errors"
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
