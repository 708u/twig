package gwt

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/708u/gwt/internal/testutil"
)

func TestAddCommand_Run(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		branch      string
		config      *Config
		sync        bool
		setupFS     func(t *testing.T) *testutil.MockFS
		setupGit    func(t *testing.T, captured *[]string) *testutil.MockGitExecutor
		wantErr     bool
		errContains string
		wantBFlag   bool
		checkPath   string
		wantSynced  bool
	}{
		{
			name:   "new_branch",
			branch: "feature/test",
			config: &Config{WorktreeSourceDir: "/repo/main", WorktreeDestBaseDir: "/repo/main-worktree", Symlinks: []string{".envrc"}},
			setupFS: func(t *testing.T) *testutil.MockFS {
				t.Helper()
				return &testutil.MockFS{}
			},
			setupGit: func(t *testing.T, captured *[]string) *testutil.MockGitExecutor {
				t.Helper()
				return &testutil.MockGitExecutor{CapturedArgs: captured}
			},
			wantErr:   false,
			wantBFlag: true,
		},
		{
			name:   "existing_branch",
			branch: "existing",
			config: &Config{WorktreeSourceDir: "/repo/main", WorktreeDestBaseDir: "/repo/main-worktree", Symlinks: []string{".envrc"}},
			setupFS: func(t *testing.T) *testutil.MockFS {
				t.Helper()
				return &testutil.MockFS{}
			},
			setupGit: func(t *testing.T, captured *[]string) *testutil.MockGitExecutor {
				t.Helper()
				return &testutil.MockGitExecutor{
					ExistingBranches: []string{"existing"},
					Worktrees:        []testutil.MockWorktree{{Path: "/repo/main", Branch: "main"}},
					CapturedArgs:     captured,
				}
			},
			wantErr:   false,
			wantBFlag: false,
		},
		{
			name:   "directory_exists",
			branch: "feature/test",
			config: &Config{WorktreeSourceDir: "/repo/main", WorktreeDestBaseDir: "/repo/main-worktree"},
			setupFS: func(t *testing.T) *testutil.MockFS {
				t.Helper()
				return &testutil.MockFS{
					ExistingPaths: []string{"/repo/main-worktree/feature/test"},
				}
			},
			setupGit: func(t *testing.T, captured *[]string) *testutil.MockGitExecutor {
				t.Helper()
				return &testutil.MockGitExecutor{}
			},
			wantErr:     true,
			errContains: "directory already exists",
		},
		{
			name:   "empty_name",
			branch: "",
			config: &Config{WorktreeSourceDir: "/repo/main"},
			setupFS: func(t *testing.T) *testutil.MockFS {
				t.Helper()
				return &testutil.MockFS{}
			},
			setupGit: func(t *testing.T, captured *[]string) *testutil.MockGitExecutor {
				t.Helper()
				return &testutil.MockGitExecutor{}
			},
			wantErr:     true,
			errContains: "branch name is required",
		},
		{
			name:   "branch_checked_out",
			branch: "already-used",
			config: &Config{WorktreeSourceDir: "/repo/main", WorktreeDestBaseDir: "/repo/main-worktree"},
			setupFS: func(t *testing.T) *testutil.MockFS {
				t.Helper()
				return &testutil.MockFS{}
			},
			setupGit: func(t *testing.T, captured *[]string) *testutil.MockGitExecutor {
				t.Helper()
				return &testutil.MockGitExecutor{
					Worktrees: []testutil.MockWorktree{{Path: "/repo/already-used", Branch: "already-used"}},
				}
			},
			wantErr:     true,
			errContains: "already checked out",
		},
		{
			name:   "worktree_add_error",
			branch: "feature/test",
			config: &Config{WorktreeSourceDir: "/repo/main", WorktreeDestBaseDir: "/repo/main-worktree"},
			setupFS: func(t *testing.T) *testutil.MockFS {
				t.Helper()
				return &testutil.MockFS{}
			},
			setupGit: func(t *testing.T, captured *[]string) *testutil.MockGitExecutor {
				t.Helper()
				return &testutil.MockGitExecutor{
					WorktreeAddErr: errors.New("worktree add failed"),
				}
			},
			wantErr:     true,
			errContains: "failed to create worktree",
		},
		{
			name:      "slash_in_branch_name",
			branch:    "feature/foo",
			config:    &Config{WorktreeSourceDir: "/repo/main", WorktreeDestBaseDir: "/worktrees"},
			checkPath: "/worktrees/feature/foo",
			setupFS: func(t *testing.T) *testutil.MockFS {
				t.Helper()
				return &testutil.MockFS{}
			},
			setupGit: func(t *testing.T, captured *[]string) *testutil.MockGitExecutor {
				t.Helper()
				return &testutil.MockGitExecutor{CapturedArgs: captured}
			},
			wantErr:   false,
			wantBFlag: true,
		},
		{
			name:   "sync_with_changes",
			branch: "feature/sync",
			config: &Config{WorktreeSourceDir: "/repo/main", WorktreeDestBaseDir: "/repo/main-worktree", Symlinks: []string{".envrc"}},
			sync:   true,
			setupFS: func(t *testing.T) *testutil.MockFS {
				t.Helper()
				return &testutil.MockFS{}
			},
			setupGit: func(t *testing.T, captured *[]string) *testutil.MockGitExecutor {
				t.Helper()
				return &testutil.MockGitExecutor{
					CapturedArgs: captured,
					HasChanges:   true,
				}
			},
			wantErr:    false,
			wantBFlag:  true,
			wantSynced: true,
		},
		{
			name:   "sync_no_changes",
			branch: "feature/sync-no-changes",
			config: &Config{WorktreeSourceDir: "/repo/main", WorktreeDestBaseDir: "/repo/main-worktree", Symlinks: []string{".envrc"}},
			sync:   true,
			setupFS: func(t *testing.T) *testutil.MockFS {
				t.Helper()
				return &testutil.MockFS{}
			},
			setupGit: func(t *testing.T, captured *[]string) *testutil.MockGitExecutor {
				t.Helper()
				return &testutil.MockGitExecutor{
					CapturedArgs: captured,
					HasChanges:   false,
				}
			},
			wantErr:    false,
			wantBFlag:  true,
			wantSynced: false,
		},
		{
			name:   "sync_stash_push_error",
			branch: "feature/sync-push-err",
			config: &Config{WorktreeSourceDir: "/repo/main", WorktreeDestBaseDir: "/repo/main-worktree"},
			sync:   true,
			setupFS: func(t *testing.T) *testutil.MockFS {
				t.Helper()
				return &testutil.MockFS{}
			},
			setupGit: func(t *testing.T, captured *[]string) *testutil.MockGitExecutor {
				t.Helper()
				return &testutil.MockGitExecutor{
					HasChanges:   true,
					StashPushErr: errors.New("stash push failed"),
				}
			},
			wantErr:     true,
			errContains: "failed to stash changes",
		},
		{
			name:   "sync_stash_apply_error",
			branch: "feature/sync-apply-err",
			config: &Config{WorktreeSourceDir: "/repo/main", WorktreeDestBaseDir: "/repo/main-worktree"},
			sync:   true,
			setupFS: func(t *testing.T) *testutil.MockFS {
				t.Helper()
				return &testutil.MockFS{}
			},
			setupGit: func(t *testing.T, captured *[]string) *testutil.MockGitExecutor {
				t.Helper()
				return &testutil.MockGitExecutor{
					HasChanges:    true,
					StashApplyErr: errors.New("stash apply failed"),
				}
			},
			wantErr:     true,
			errContains: "failed to apply changes",
		},
		{
			name:   "sync_disabled_with_changes",
			branch: "feature/no-sync",
			config: &Config{WorktreeSourceDir: "/repo/main", WorktreeDestBaseDir: "/repo/main-worktree", Symlinks: []string{".envrc"}},
			sync:   false,
			setupFS: func(t *testing.T) *testutil.MockFS {
				t.Helper()
				return &testutil.MockFS{}
			},
			setupGit: func(t *testing.T, captured *[]string) *testutil.MockGitExecutor {
				t.Helper()
				return &testutil.MockGitExecutor{
					CapturedArgs: captured,
					HasChanges:   true,
				}
			},
			wantErr:    false,
			wantBFlag:  true,
			wantSynced: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var captured []string

			mockFS := tt.setupFS(t)
			mockGit := tt.setupGit(t, &captured)

			cmd := &AddCommand{
				FS:     mockFS,
				Git:    &GitRunner{Executor: mockGit},
				Config: tt.config,
				Sync:   tt.sync,
			}

			result, err := cmd.Run(tt.branch)

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

			if tt.wantBFlag && !slices.Contains(captured, "-b") {
				t.Errorf("expected -b flag, got: %v", captured)
			}

			if !tt.wantBFlag && len(captured) > 0 && slices.Contains(captured, "-b") {
				t.Errorf("unexpected -b flag, got: %v", captured)
			}

			if tt.checkPath != "" && !slices.Contains(captured, tt.checkPath) {
				t.Errorf("expected path %q in args, got: %v", tt.checkPath, captured)
			}

			if result.ChangesSynced != tt.wantSynced {
				t.Errorf("ChangesSynced = %v, want %v", result.ChangesSynced, tt.wantSynced)
			}
		})
	}
}

func TestAddCommand_createSymlinks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		targets        []string
		setupFS        func(t *testing.T) *testutil.MockFS
		wantErr        bool
		errContains    string
		wantSkipped    int
		wantCreated    int
		wantReasonLike string
	}{
		{
			name:    "success",
			targets: []string{".envrc", ".tool-versions"},
			setupFS: func(t *testing.T) *testutil.MockFS {
				t.Helper()
				return &testutil.MockFS{
					GlobResults: map[string][]string{
						".envrc":         {".envrc"},
						".tool-versions": {".tool-versions"},
					},
				}
			},
			wantErr:     false,
			wantCreated: 2,
		},
		{
			name:    "source_not_exist",
			targets: []string{".envrc"},
			setupFS: func(t *testing.T) *testutil.MockFS {
				t.Helper()
				return &testutil.MockFS{
					GlobResults: map[string][]string{},
				}
			},
			wantErr:        false,
			wantSkipped:    1,
			wantReasonLike: "does not match any files",
		},
		{
			name:    "symlink_error",
			targets: []string{".envrc"},
			setupFS: func(t *testing.T) *testutil.MockFS {
				t.Helper()
				return &testutil.MockFS{
					GlobResults: map[string][]string{
						".envrc": {".envrc"},
					},
					SymlinkErr: errors.New("symlink failed"),
				}
			},
			wantErr:     true,
			errContains: "failed to create symlink",
		},
		{
			name:    "destination_already_exists",
			targets: []string{".claude"},
			setupFS: func(t *testing.T) *testutil.MockFS {
				t.Helper()
				return &testutil.MockFS{
					GlobResults: map[string][]string{
						".claude": {".claude"},
					},
					ExistingPaths: []string{"/dst/.claude"},
				}
			},
			wantErr:        false,
			wantSkipped:    1,
			wantReasonLike: "already exists",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mockFS := tt.setupFS(t)

			cmd := &AddCommand{
				FS: mockFS,
			}

			results, err := cmd.createSymlinks("/src", "/dst", tt.targets)

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

			var skipped, created int
			for _, r := range results {
				if r.Skipped {
					skipped++
					if tt.wantReasonLike != "" && !strings.Contains(r.Reason, tt.wantReasonLike) {
						t.Errorf("reason %q should contain %q", r.Reason, tt.wantReasonLike)
					}
				} else {
					created++
				}
			}

			if skipped != tt.wantSkipped {
				t.Errorf("got %d skipped, want %d", skipped, tt.wantSkipped)
			}
			if created != tt.wantCreated {
				t.Errorf("got %d created, want %d", created, tt.wantCreated)
			}
		})
	}
}
