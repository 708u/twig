package gwt

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/708u/gwt/internal/testutil"
)

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
