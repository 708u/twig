package twig

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/708u/twig/internal/testutil"
)

func TestAddCommand_Run(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		branch       string
		config       *Config
		sync         bool
		carryFrom    string
		filePatterns []string
		setupFS      func(t *testing.T) *testutil.MockFS
		setupGit     func(t *testing.T, captured *[]string) *testutil.MockGitExecutor
		wantErr      bool
		errContains  string
		wantBFlag    bool
		checkPath    string
		wantSynced   bool
		wantCarried  bool
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
			name:         "sync_with_file_pattern",
			branch:       "feature/sync-file",
			config:       &Config{WorktreeSourceDir: "/repo/main", WorktreeDestBaseDir: "/repo/main-worktree", Symlinks: []string{".envrc"}},
			sync:         true,
			filePatterns: []string{"*.go"},
			setupFS: func(t *testing.T) *testutil.MockFS {
				t.Helper()
				return &testutil.MockFS{
					GlobResults: map[string][]string{
						"*.go": {"main.go", "util.go"},
					},
				}
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
		{
			name:      "carry_with_changes",
			branch:    "feature/carry",
			config:    &Config{WorktreeSourceDir: "/repo/main", WorktreeDestBaseDir: "/repo/main-worktree", Symlinks: []string{".envrc"}},
			carryFrom: "/repo/main",
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
			wantErr:     false,
			wantBFlag:   true,
			wantCarried: true,
		},
		{
			name:      "carry_no_changes",
			branch:    "feature/carry-no-changes",
			config:    &Config{WorktreeSourceDir: "/repo/main", WorktreeDestBaseDir: "/repo/main-worktree", Symlinks: []string{".envrc"}},
			carryFrom: "/repo/main",
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
			wantErr:     false,
			wantBFlag:   true,
			wantCarried: false,
		},
		{
			name:      "carry_stash_apply_error",
			branch:    "feature/carry-apply-err",
			config:    &Config{WorktreeSourceDir: "/repo/main", WorktreeDestBaseDir: "/repo/main-worktree"},
			carryFrom: "/repo/main",
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
			name:   "remote_branch_single_remote",
			branch: "feature/remote-only",
			config: &Config{WorktreeSourceDir: "/repo/main", WorktreeDestBaseDir: "/repo/main-worktree", Symlinks: []string{".envrc"}},
			setupFS: func(t *testing.T) *testutil.MockFS {
				t.Helper()
				return &testutil.MockFS{}
			},
			setupGit: func(t *testing.T, captured *[]string) *testutil.MockGitExecutor {
				t.Helper()
				return &testutil.MockGitExecutor{
					CapturedArgs: captured,
					Remotes:      []string{"origin"},
					RemoteBranches: map[string][]string{
						"origin": {"feature/remote-only"},
					},
				}
			},
			wantErr:   false,
			wantBFlag: false, // Should not use -b flag since it fetches from remote
		},
		{
			name:   "remote_branch_multiple_remotes_one_has_it",
			branch: "feature/on-upstream",
			config: &Config{WorktreeSourceDir: "/repo/main", WorktreeDestBaseDir: "/repo/main-worktree", Symlinks: []string{".envrc"}},
			setupFS: func(t *testing.T) *testutil.MockFS {
				t.Helper()
				return &testutil.MockFS{}
			},
			setupGit: func(t *testing.T, captured *[]string) *testutil.MockGitExecutor {
				t.Helper()
				return &testutil.MockGitExecutor{
					CapturedArgs: captured,
					Remotes:      []string{"origin", "upstream"},
					RemoteBranches: map[string][]string{
						"origin":   {},
						"upstream": {"feature/on-upstream"},
					},
				}
			},
			wantErr:   false,
			wantBFlag: false,
		},
		{
			name:   "remote_branch_ambiguous",
			branch: "feature/ambiguous",
			config: &Config{WorktreeSourceDir: "/repo/main", WorktreeDestBaseDir: "/repo/main-worktree"},
			setupFS: func(t *testing.T) *testutil.MockFS {
				t.Helper()
				return &testutil.MockFS{}
			},
			setupGit: func(t *testing.T, captured *[]string) *testutil.MockGitExecutor {
				t.Helper()
				return &testutil.MockGitExecutor{
					Remotes: []string{"origin", "upstream"},
					RemoteBranches: map[string][]string{
						"origin":   {"feature/ambiguous"},
						"upstream": {"feature/ambiguous"},
					},
				}
			},
			wantErr:     true,
			errContains: "exists on multiple remotes",
		},
		{
			name:   "remote_branch_fetch_error",
			branch: "feature/fetch-fail",
			config: &Config{WorktreeSourceDir: "/repo/main", WorktreeDestBaseDir: "/repo/main-worktree"},
			setupFS: func(t *testing.T) *testutil.MockFS {
				t.Helper()
				return &testutil.MockFS{}
			},
			setupGit: func(t *testing.T, captured *[]string) *testutil.MockGitExecutor {
				t.Helper()
				return &testutil.MockGitExecutor{
					Remotes: []string{"origin"},
					RemoteBranches: map[string][]string{
						"origin": {"feature/fetch-fail"},
					},
					FetchErr: errors.New("network error"),
				}
			},
			wantErr:     true,
			errContains: "failed to fetch",
		},
		{
			name:   "no_remote_no_local_creates_new_branch",
			branch: "feature/brand-new",
			config: &Config{WorktreeSourceDir: "/repo/main", WorktreeDestBaseDir: "/repo/main-worktree", Symlinks: []string{".envrc"}},
			setupFS: func(t *testing.T) *testutil.MockFS {
				t.Helper()
				return &testutil.MockFS{}
			},
			setupGit: func(t *testing.T, captured *[]string) *testutil.MockGitExecutor {
				t.Helper()
				return &testutil.MockGitExecutor{
					CapturedArgs:   captured,
					Remotes:        []string{"origin"},
					RemoteBranches: map[string][]string{},
				}
			},
			wantErr:   false,
			wantBFlag: true, // Should use -b flag since branch doesn't exist anywhere
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var captured []string

			mockFS := tt.setupFS(t)
			mockGit := tt.setupGit(t, &captured)

			cmd := &AddCommand{
				FS:           mockFS,
				Git:          &GitRunner{Executor: mockGit, Log: NewNopLogger()},
				Config:       tt.config,
				Sync:         tt.sync,
				CarryFrom:    tt.carryFrom,
				FilePatterns: tt.filePatterns,
			}

			result, err := cmd.Run(t.Context(), tt.branch)

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

			if result.ChangesCarried != tt.wantCarried {
				t.Errorf("ChangesCarried = %v, want %v", result.ChangesCarried, tt.wantCarried)
			}
		})
	}
}

func TestAddCommand_Run_Lock(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		branch         string
		config         *Config
		lock           bool
		lockReason     string
		setupFS        func(t *testing.T) *testutil.MockFS
		setupGit       func(t *testing.T, captured *[]string) *testutil.MockGitExecutor
		wantLockFlag   bool
		wantReasonFlag bool
	}{
		{
			name:   "lock_flag",
			branch: "feature/locked",
			config: &Config{WorktreeSourceDir: "/repo/main", WorktreeDestBaseDir: "/repo/main-worktree", Symlinks: []string{".envrc"}},
			lock:   true,
			setupFS: func(t *testing.T) *testutil.MockFS {
				t.Helper()
				return &testutil.MockFS{}
			},
			setupGit: func(t *testing.T, captured *[]string) *testutil.MockGitExecutor {
				t.Helper()
				return &testutil.MockGitExecutor{CapturedArgs: captured}
			},
			wantLockFlag:   true,
			wantReasonFlag: false,
		},
		{
			name:       "lock_with_reason",
			branch:     "feature/locked-reason",
			config:     &Config{WorktreeSourceDir: "/repo/main", WorktreeDestBaseDir: "/repo/main-worktree", Symlinks: []string{".envrc"}},
			lock:       true,
			lockReason: "USB drive",
			setupFS: func(t *testing.T) *testutil.MockFS {
				t.Helper()
				return &testutil.MockFS{}
			},
			setupGit: func(t *testing.T, captured *[]string) *testutil.MockGitExecutor {
				t.Helper()
				return &testutil.MockGitExecutor{CapturedArgs: captured}
			},
			wantLockFlag:   true,
			wantReasonFlag: true,
		},
		{
			name:   "no_lock",
			branch: "feature/unlocked",
			config: &Config{WorktreeSourceDir: "/repo/main", WorktreeDestBaseDir: "/repo/main-worktree", Symlinks: []string{".envrc"}},
			lock:   false,
			setupFS: func(t *testing.T) *testutil.MockFS {
				t.Helper()
				return &testutil.MockFS{}
			},
			setupGit: func(t *testing.T, captured *[]string) *testutil.MockGitExecutor {
				t.Helper()
				return &testutil.MockGitExecutor{CapturedArgs: captured}
			},
			wantLockFlag:   false,
			wantReasonFlag: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var captured []string

			mockFS := tt.setupFS(t)
			mockGit := tt.setupGit(t, &captured)

			cmd := &AddCommand{
				FS:         mockFS,
				Git:        &GitRunner{Executor: mockGit, Log: NewNopLogger()},
				Config:     tt.config,
				Lock:       tt.lock,
				LockReason: tt.lockReason,
			}

			_, err := cmd.Run(t.Context(), tt.branch)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			hasLockFlag := slices.Contains(captured, "--lock")
			if tt.wantLockFlag != hasLockFlag {
				t.Errorf("--lock flag: got %v, want %v; args: %v", hasLockFlag, tt.wantLockFlag, captured)
			}

			hasReasonFlag := slices.Contains(captured, "--reason")
			if tt.wantReasonFlag != hasReasonFlag {
				t.Errorf("--reason flag: got %v, want %v; args: %v", hasReasonFlag, tt.wantReasonFlag, captured)
			}

			if tt.wantReasonFlag && tt.lockReason != "" {
				if !slices.Contains(captured, tt.lockReason) {
					t.Errorf("expected reason %q in args, got: %v", tt.lockReason, captured)
				}
			}
		})
	}
}

func TestAddResult_Format(t *testing.T) {
	t.Parallel()

	result := AddResult{
		Branch:       "feature/test",
		WorktreePath: "/worktrees/feature/test",
		Symlinks: []SymlinkResult{
			{Src: "/repo/.envrc", Dst: "/worktrees/feature/test/.envrc"},
		},
		ChangesSynced: false,
	}

	tests := []struct {
		name       string
		opts       AddFormatOptions
		wantStdout string
		wantStderr string
	}{
		{
			name:       "default_output",
			opts:       AddFormatOptions{},
			wantStdout: "twig add: feature/test (1 symlinks)\n",
			wantStderr: "",
		},
		{
			name:       "quiet",
			opts:       AddFormatOptions{Quiet: true},
			wantStdout: "/worktrees/feature/test\n",
			wantStderr: "",
		},
		{
			name:       "quiet_ignores_verbose",
			opts:       AddFormatOptions{Verbose: true, Quiet: true},
			wantStdout: "/worktrees/feature/test\n",
			wantStderr: "",
		},
		{
			name:       "verbose_output",
			opts:       AddFormatOptions{Verbose: true},
			wantStdout: "Created worktree at /worktrees/feature/test\nCreated symlink: /worktrees/feature/test/.envrc -> /repo/.envrc\ntwig add: feature/test (1 symlinks)\n",
			wantStderr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := result.Format(tt.opts)

			if got.Stdout != tt.wantStdout {
				t.Errorf("Stdout = %q, want %q", got.Stdout, tt.wantStdout)
			}
			if got.Stderr != tt.wantStderr {
				t.Errorf("Stderr = %q, want %q", got.Stderr, tt.wantStderr)
			}
		})
	}

	// Test carried output
	t.Run("default_output_carried", func(t *testing.T) {
		t.Parallel()

		carriedResult := AddResult{
			Branch:       "feature/test",
			WorktreePath: "/worktrees/feature/test",
			Symlinks: []SymlinkResult{
				{Src: "/repo/.envrc", Dst: "/worktrees/feature/test/.envrc"},
			},
			ChangesCarried: true,
		}

		got := carriedResult.Format(AddFormatOptions{})
		want := "twig add: feature/test (1 symlinks, carried)\n"

		if got.Stdout != want {
			t.Errorf("Stdout = %q, want %q", got.Stdout, want)
		}
	})

	t.Run("verbose_output_carried", func(t *testing.T) {
		t.Parallel()

		carriedResult := AddResult{
			Branch:       "feature/test",
			WorktreePath: "/worktrees/feature/test",
			Symlinks: []SymlinkResult{
				{Src: "/repo/.envrc", Dst: "/worktrees/feature/test/.envrc"},
			},
			ChangesCarried: true,
		}

		got := carriedResult.Format(AddFormatOptions{Verbose: true})
		wantContains := "Carried uncommitted changes (source is now clean)"

		if !strings.Contains(got.Stdout, wantContains) {
			t.Errorf("Stdout = %q, should contain %q", got.Stdout, wantContains)
		}
	})
}

func TestAddCommand_Run_InitSubmodules(t *testing.T) {
	t.Parallel()

	boolPtr := func(b bool) *bool { return &b }

	tests := []struct {
		name                      string
		branch                    string
		config                    *Config
		initSubmodules            bool // true forces enable, false uses config
		setupFS                   func(t *testing.T) *testutil.MockFS
		setupGit                  func(t *testing.T, captured *[]string) *testutil.MockGitExecutor
		wantSubmodulesInited      bool
		wantSubmoduleCount        int
		wantSubmoduleUpdateCalled bool
		wantSubmoduleInitError    bool
	}{
		{
			name:           "cli_flag_forces_enable",
			branch:         "feature/submod",
			config:         &Config{WorktreeSourceDir: "/repo/main", WorktreeDestBaseDir: "/repo/main-worktree"},
			initSubmodules: true,
			setupFS: func(t *testing.T) *testutil.MockFS {
				t.Helper()
				return &testutil.MockFS{}
			},
			setupGit: func(t *testing.T, captured *[]string) *testutil.MockGitExecutor {
				t.Helper()
				return &testutil.MockGitExecutor{
					CapturedArgs:          captured,
					SubmoduleStatusOutput: " abc123 submodule1 (v1.0.0)\n",
				}
			},
			wantSubmodulesInited:      true,
			wantSubmoduleCount:        1,
			wantSubmoduleUpdateCalled: true,
		},
		{
			name:           "cli_flag_enabled_no_submodules",
			branch:         "feature/no-submod",
			config:         &Config{WorktreeSourceDir: "/repo/main", WorktreeDestBaseDir: "/repo/main-worktree"},
			initSubmodules: true,
			setupFS: func(t *testing.T) *testutil.MockFS {
				t.Helper()
				return &testutil.MockFS{}
			},
			setupGit: func(t *testing.T, captured *[]string) *testutil.MockGitExecutor {
				t.Helper()
				return &testutil.MockGitExecutor{
					CapturedArgs:          captured,
					SubmoduleStatusOutput: "", // No submodules
				}
			},
			wantSubmodulesInited:      false,
			wantSubmoduleCount:        0,
			wantSubmoduleUpdateCalled: true, // SubmoduleUpdate is always called when enabled
		},
		{
			name:           "config_disabled_no_flag",
			branch:         "feature/disabled",
			config:         &Config{WorktreeSourceDir: "/repo/main", WorktreeDestBaseDir: "/repo/main-worktree", InitSubmodules: boolPtr(false)},
			initSubmodules: false, // Use config
			setupFS: func(t *testing.T) *testutil.MockFS {
				t.Helper()
				return &testutil.MockFS{}
			},
			setupGit: func(t *testing.T, captured *[]string) *testutil.MockGitExecutor {
				t.Helper()
				return &testutil.MockGitExecutor{
					CapturedArgs:          captured,
					SubmoduleStatusOutput: " abc123 submodule1 (v1.0.0)\n",
				}
			},
			wantSubmodulesInited:      false,
			wantSubmoduleCount:        0,
			wantSubmoduleUpdateCalled: false,
		},
		{
			name:           "config_enabled_no_flag",
			branch:         "feature/from-config",
			config:         &Config{WorktreeSourceDir: "/repo/main", WorktreeDestBaseDir: "/repo/main-worktree", InitSubmodules: boolPtr(true)},
			initSubmodules: false, // Use config
			setupFS: func(t *testing.T) *testutil.MockFS {
				t.Helper()
				return &testutil.MockFS{}
			},
			setupGit: func(t *testing.T, captured *[]string) *testutil.MockGitExecutor {
				t.Helper()
				return &testutil.MockGitExecutor{
					CapturedArgs:          captured,
					SubmoduleStatusOutput: " abc123 submodule1 (v1.0.0)\n",
				}
			},
			wantSubmodulesInited:      true,
			wantSubmoduleCount:        1,
			wantSubmoduleUpdateCalled: true,
		},
		{
			name:           "default_disabled_no_config",
			branch:         "feature/default",
			config:         &Config{WorktreeSourceDir: "/repo/main", WorktreeDestBaseDir: "/repo/main-worktree"},
			initSubmodules: false, // Use config (nil = default false)
			setupFS: func(t *testing.T) *testutil.MockFS {
				t.Helper()
				return &testutil.MockFS{}
			},
			setupGit: func(t *testing.T, captured *[]string) *testutil.MockGitExecutor {
				t.Helper()
				return &testutil.MockGitExecutor{
					CapturedArgs:          captured,
					SubmoduleStatusOutput: " abc123 submodule1 (v1.0.0)\n",
				}
			},
			wantSubmodulesInited:      false,
			wantSubmoduleCount:        0,
			wantSubmoduleUpdateCalled: false,
		},
		{
			name:           "init_error_is_warning",
			branch:         "feature/init-error",
			config:         &Config{WorktreeSourceDir: "/repo/main", WorktreeDestBaseDir: "/repo/main-worktree"},
			initSubmodules: true,
			setupFS: func(t *testing.T) *testutil.MockFS {
				t.Helper()
				return &testutil.MockFS{}
			},
			setupGit: func(t *testing.T, captured *[]string) *testutil.MockGitExecutor {
				t.Helper()
				return &testutil.MockGitExecutor{
					CapturedArgs:          captured,
					SubmoduleStatusOutput: " abc123 submodule1 (v1.0.0)\n",
					SubmoduleUpdateErr:    errors.New("submodule update failed"),
				}
			},
			wantSubmodulesInited:      true,
			wantSubmoduleCount:        0,
			wantSubmoduleUpdateCalled: true,
			wantSubmoduleInitError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var captured []string

			mockFS := tt.setupFS(t)
			mockGit := tt.setupGit(t, &captured)

			cmd := &AddCommand{
				FS:             mockFS,
				Git:            &GitRunner{Executor: mockGit, Log: NewNopLogger()},
				Config:         tt.config,
				InitSubmodules: tt.initSubmodules,
			}

			result, err := cmd.Run(t.Context(), tt.branch)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.SubmoduleInit.Attempted != tt.wantSubmodulesInited {
				t.Errorf("SubmoduleInit.Attempted = %v, want %v", result.SubmoduleInit.Attempted, tt.wantSubmodulesInited)
			}

			if result.SubmoduleInit.Count != tt.wantSubmoduleCount {
				t.Errorf("SubmoduleInit.Count = %v, want %v", result.SubmoduleInit.Count, tt.wantSubmoduleCount)
			}

			if mockGit.SubmoduleUpdateCalled != tt.wantSubmoduleUpdateCalled {
				t.Errorf("SubmoduleUpdateCalled = %v, want %v", mockGit.SubmoduleUpdateCalled, tt.wantSubmoduleUpdateCalled)
			}

			if tt.wantSubmoduleInitError && !result.SubmoduleInit.Skipped {
				t.Error("expected SubmoduleInit.Skipped = true, got false")
			}
			if !tt.wantSubmoduleInitError && result.SubmoduleInit.Skipped {
				t.Errorf("unexpected SubmoduleInit.Skipped: %s", result.SubmoduleInit.Reason)
			}
		})
	}
}

func TestAddResult_Format_Submodules(t *testing.T) {
	t.Parallel()

	t.Run("default_output_with_submodules", func(t *testing.T) {
		t.Parallel()

		result := AddResult{
			Branch:        "feature/test",
			WorktreePath:  "/worktrees/feature/test",
			Symlinks:      []SymlinkResult{},
			SubmoduleInit: SubmoduleInitResult{Attempted: true, Count: 2},
		}

		got := result.Format(AddFormatOptions{})
		want := "twig add: feature/test (0 symlinks, 2 submodules)\n"

		if got.Stdout != want {
			t.Errorf("Stdout = %q, want %q", got.Stdout, want)
		}
	})

	t.Run("verbose_output_with_submodules", func(t *testing.T) {
		t.Parallel()

		result := AddResult{
			Branch:        "feature/test",
			WorktreePath:  "/worktrees/feature/test",
			Symlinks:      []SymlinkResult{},
			SubmoduleInit: SubmoduleInitResult{Attempted: true, Count: 3},
		}

		got := result.Format(AddFormatOptions{Verbose: true})
		wantContains := "Initialized 3 submodule(s)"

		if !strings.Contains(got.Stdout, wantContains) {
			t.Errorf("Stdout = %q, should contain %q", got.Stdout, wantContains)
		}
	})

	t.Run("submodule_init_error_as_warning", func(t *testing.T) {
		t.Parallel()

		result := AddResult{
			Branch:        "feature/test",
			WorktreePath:  "/worktrees/feature/test",
			Symlinks:      []SymlinkResult{},
			SubmoduleInit: SubmoduleInitResult{Attempted: true, Skipped: true, Reason: "failed to initialize submodules"},
		}

		got := result.Format(AddFormatOptions{})

		if !strings.Contains(got.Stderr, "warning:") {
			t.Errorf("Stderr = %q, should contain 'warning:'", got.Stderr)
		}
		if !strings.Contains(got.Stderr, "failed to initialize submodules") {
			t.Errorf("Stderr = %q, should contain error message", got.Stderr)
		}
	})

	t.Run("no_submodule_info_when_count_is_zero", func(t *testing.T) {
		t.Parallel()

		result := AddResult{
			Branch:        "feature/test",
			WorktreePath:  "/worktrees/feature/test",
			Symlinks:      []SymlinkResult{},
			SubmoduleInit: SubmoduleInitResult{Attempted: true, Count: 0},
		}

		got := result.Format(AddFormatOptions{})
		want := "twig add: feature/test (0 symlinks)\n"

		if got.Stdout != want {
			t.Errorf("Stdout = %q, want %q", got.Stdout, want)
		}
	})
}

func TestCreateSymlinks(t *testing.T) {
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
					LstatFunc: func(name string) (fs.FileInfo, error) {
						if name == "/dst/.claude" {
							return &testutil.MockFileInfo{NameVal: ".claude"}, nil
						}
						return nil, fs.ErrNotExist
					},
				}
			},
			wantErr:        false,
			wantSkipped:    1,
			wantReasonLike: "regular file exists",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mockFS := tt.setupFS(t)

			results, err := createSymlinks(mockFS, "/src", "/dst", tt.targets)

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

func TestCreateSymlinks_RelativePath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		pattern     string
		match       string
		wantRelBase string // filepath.Dir(dst) for computing expected relative path
		wantAbsSrc  string // expected absolute SymlinkResult.Src
	}{
		{
			name:        "root_level",
			pattern:     ".envrc",
			match:       ".envrc",
			wantRelBase: "/dst",
			wantAbsSrc:  "/src/.envrc",
		},
		{
			name:        "nested_glob",
			pattern:     "config/**/*.toml",
			match:       "config/app.toml",
			wantRelBase: "/dst/config",
			wantAbsSrc:  "/src/config/app.toml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var capturedOldname string
			mockFS := &testutil.MockFS{
				GlobResults: map[string][]string{
					tt.pattern: {tt.match},
				},
				SymlinkFunc: func(oldname, newname string) error {
					capturedOldname = oldname
					return nil
				},
			}

			results, err := createSymlinks(mockFS, "/src", "/dst", []string{tt.pattern})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			wantRel, _ := filepath.Rel(tt.wantRelBase, tt.wantAbsSrc)
			if capturedOldname != wantRel {
				t.Errorf("Symlink oldname = %q, want relative %q", capturedOldname, wantRel)
			}

			if len(results) != 1 {
				t.Fatalf("expected 1 result, got %d", len(results))
			}
			if results[0].Src != tt.wantAbsSrc {
				t.Errorf("SymlinkResult.Src = %q, want absolute %q", results[0].Src, tt.wantAbsSrc)
			}
		})
	}
}

func TestAddResult_Format_Hooks(t *testing.T) {
	t.Parallel()

	t.Run("default_output_with_hooks", func(t *testing.T) {
		t.Parallel()

		result := AddResult{
			Branch:       "feature/test",
			WorktreePath: "/worktrees/feature/test",
			Symlinks:     []SymlinkResult{},
			HookResults: []HookResult{
				{Command: "npm install"},
				{Command: "direnv allow"},
			},
		}

		got := result.Format(AddFormatOptions{})
		want := "twig add: feature/test (0 symlinks, 2 hooks ran)\n"

		if got.Stdout != want {
			t.Errorf("Stdout = %q, want %q", got.Stdout, want)
		}
	})

	t.Run("hook_failure_warning", func(t *testing.T) {
		t.Parallel()

		result := AddResult{
			Branch:       "feature/test",
			WorktreePath: "/worktrees/feature/test",
			Symlinks:     []SymlinkResult{},
			HookResults: []HookResult{
				{Command: "npm install", Err: errors.New("exit status 1"), Output: []byte("npm ERR!\n")},
			},
		}

		got := result.Format(AddFormatOptions{})

		if !strings.Contains(got.Stderr, "warning:") {
			t.Errorf("Stderr = %q, should contain 'warning:'", got.Stderr)
		}
		if !strings.Contains(got.Stderr, "npm install") {
			t.Errorf("Stderr = %q, should contain hook command", got.Stderr)
		}
		if !strings.Contains(got.Stderr, "npm ERR!") {
			t.Errorf("Stderr = %q, should contain hook output", got.Stderr)
		}
	})

	t.Run("verbose_output_with_hooks", func(t *testing.T) {
		t.Parallel()

		result := AddResult{
			Branch:       "feature/test",
			WorktreePath: "/worktrees/feature/test",
			Symlinks:     []SymlinkResult{},
			HookResults: []HookResult{
				{Command: "npm install", Output: []byte("added 100 packages\n")},
			},
		}

		got := result.Format(AddFormatOptions{Verbose: true})

		if !strings.Contains(got.Stdout, "Ran hook: npm install") {
			t.Errorf("Stdout = %q, should contain 'Ran hook: npm install'", got.Stdout)
		}
		if !strings.Contains(got.Stdout, "added 100 packages") {
			t.Errorf("Stdout = %q, should contain hook output", got.Stdout)
		}
	})

	t.Run("no_hook_info_when_empty", func(t *testing.T) {
		t.Parallel()

		result := AddResult{
			Branch:       "feature/test",
			WorktreePath: "/worktrees/feature/test",
			Symlinks:     []SymlinkResult{},
			HookResults:  []HookResult{},
		}

		got := result.Format(AddFormatOptions{})
		want := "twig add: feature/test (0 symlinks)\n"

		if got.Stdout != want {
			t.Errorf("Stdout = %q, want %q", got.Stdout, want)
		}
	})

	t.Run("failed_hooks_not_counted_in_summary", func(t *testing.T) {
		t.Parallel()

		result := AddResult{
			Branch:       "feature/test",
			WorktreePath: "/worktrees/feature/test",
			Symlinks:     []SymlinkResult{},
			HookResults: []HookResult{
				{Command: "npm install"},
				{Command: "bad-command", Err: errors.New("exit status 1")},
			},
		}

		got := result.Format(AddFormatOptions{})
		want := "twig add: feature/test (0 symlinks, 1 hooks ran)\n"

		if got.Stdout != want {
			t.Errorf("Stdout = %q, want %q", got.Stdout, want)
		}
	})
}

// checkoutSettingsExecutor records every git command (without -C) and answers
// the commands that MockGitExecutor does not model.
type checkoutSettingsExecutor struct {
	inner             *testutil.MockGitExecutor
	version           string
	worktreeConfig    bool
	inheritedConfig   string // output of git config --worktree --get-regexp
	sparseCheckoutErr error
	calls             [][]string
}

func (e *checkoutSettingsExecutor) Run(ctx context.Context, args ...string) ([]byte, error) {
	cmd := args
	if len(cmd) >= 2 && cmd[0] == "-C" {
		cmd = cmd[2:]
	}
	e.calls = append(e.calls, cmd)

	sub := cmd
	for len(sub) >= 2 && sub[0] == "-c" {
		sub = sub[2:]
	}
	switch {
	case len(sub) > 0 && sub[0] == GitCmdVersion:
		return []byte("git version " + e.version + "\n"), nil
	case slices.Equal(sub, []string{GitCmdConfig, "--local", "--bool", "--get", "extensions.worktreeConfig"}):
		if !e.worktreeConfig {
			return nil, &testutil.MockExitError{Code: 1}
		}
		return []byte("true\n"), nil
	case len(sub) >= 3 && sub[0] == GitCmdConfig && sub[2] == "--get-regexp":
		if e.inheritedConfig == "" {
			return nil, &testutil.MockExitError{Code: 1}
		}
		return []byte(e.inheritedConfig), nil
	case len(sub) > 0 && sub[0] == GitCmdSparseCheckout:
		return nil, e.sparseCheckoutErr
	}
	return e.inner.Run(ctx, args...)
}

// find returns the index of the first recorded call that starts with prefix
// after skipping -c options, or -1.
func (e *checkoutSettingsExecutor) find(prefix ...string) int {
	for i, call := range e.calls {
		sub := call
		for len(sub) >= 2 && sub[0] == "-c" {
			sub = sub[2:]
		}
		if len(sub) >= len(prefix) && slices.Equal(sub[:len(prefix)], prefix) {
			return i
		}
	}
	return -1
}

func TestAddCommand_Run_CheckoutSettings(t *testing.T) {
	t.Parallel()

	const (
		lfsArg       = "lfs.fetchexclude=assets/**,images/**"
		inheritedCfg = "core.sparsecheckout true\nlfs.fetchexclude big/**\n"
	)
	setLFS := []string{GitCmdConfig, "--worktree", "--replace-all", "lfs.fetchexclude"}

	tests := []struct {
		name            string
		lfsFetchExclude []string
		sparseCheckout  []string
		full            bool
		version         string
		worktreeConfig  bool
		inheritedConfig string
		// wantCalls lists command prefixes that must appear in this order.
		wantCalls [][]string
		// wantAbsent lists command prefixes that must not appear.
		wantAbsent [][]string
		wantErr    string
	}{
		{
			name:            "lfs_fetch_exclude_only",
			lfsFetchExclude: []string{"assets/**", "images/**"},
			wantCalls: [][]string{
				{"-c", lfsArg, GitCmdWorktree, GitWorktreeAdd, "-b"},
			},
			wantAbsent: [][]string{
				{GitCmdWorktree, GitWorktreeAdd, "--no-checkout"},
				{GitCmdSparseCheckout},
				setLFS,
				{GitCmdCheckout},
			},
		},
		{
			name:            "lfs_fetch_exclude_kept_when_worktree_config_enabled",
			lfsFetchExclude: []string{"assets/**", "images/**"},
			worktreeConfig:  true,
			wantCalls: [][]string{
				{"-c", lfsArg, GitCmdWorktree, GitWorktreeAdd, "-b"},
				{"-c", lfsArg, GitCmdConfig, "--worktree", "--replace-all", "lfs.fetchexclude", "assets/**,images/**"},
			},
			wantAbsent: [][]string{
				{GitCmdSparseCheckout},
			},
		},
		{
			name:           "sparse_checkout_only",
			sparseCheckout: []string{"/*", "!/plans/"},
			wantCalls: [][]string{
				{GitCmdVersion},
				{GitCmdWorktree, GitWorktreeAdd, "--no-checkout", "-b"},
				{GitCmdSparseCheckout, "set", "--no-cone", "/*", "!/plans/"},
				{GitCmdCheckout, "--no-recurse-submodules"},
			},
			wantAbsent: [][]string{
				{"-c"},
				setLFS,
			},
		},
		{
			name:            "both",
			lfsFetchExclude: []string{"assets/**", "images/**"},
			sparseCheckout:  []string{"/*", "!/plans/"},
			wantCalls: [][]string{
				{"-c", lfsArg, GitCmdWorktree, GitWorktreeAdd, "--no-checkout", "-b"},
				{"-c", lfsArg, GitCmdSparseCheckout, "set", "--no-cone", "/*", "!/plans/"},
				{"-c", lfsArg, GitCmdConfig, "--worktree", "--replace-all", "lfs.fetchexclude", "assets/**,images/**"},
				{"-c", lfsArg, GitCmdCheckout},
			},
		},
		{
			name:            "full_ignores_both",
			lfsFetchExclude: []string{"assets/**", "images/**"},
			sparseCheckout:  []string{"/*", "!/plans/"},
			worktreeConfig:  true,
			full:            true,
			wantCalls: [][]string{
				{GitCmdWorktree, GitWorktreeAdd, "-b"},
			},
			wantAbsent: [][]string{
				{"-c"},
				{GitCmdVersion},
				{GitCmdWorktree, GitWorktreeAdd, "--no-checkout"},
				{GitCmdSparseCheckout},
				setLFS,
			},
		},
		{
			name:            "full_resets_settings_inherited_from_source",
			lfsFetchExclude: []string{"assets/**", "images/**"},
			sparseCheckout:  []string{"/*", "!/plans/"},
			worktreeConfig:  true,
			inheritedConfig: inheritedCfg,
			full:            true,
			wantCalls: [][]string{
				{GitCmdWorktree, GitWorktreeAdd, "--no-checkout", "-b"},
				{GitCmdSparseCheckout, "disable"},
				{GitCmdConfig, "--worktree", "--unset-all", "lfs.fetchexclude"},
				{GitCmdCheckout},
			},
			wantAbsent: [][]string{
				{"-c"},
				{GitCmdSparseCheckout, "set"},
			},
		},
		{
			name:            "settings_override_inherited_values",
			lfsFetchExclude: []string{"assets/**", "images/**"},
			sparseCheckout:  []string{"/*", "!/plans/"},
			worktreeConfig:  true,
			inheritedConfig: inheritedCfg,
			wantCalls: [][]string{
				{"-c", lfsArg, GitCmdWorktree, GitWorktreeAdd, "--no-checkout", "-b"},
				{"-c", lfsArg, GitCmdSparseCheckout, "set", "--no-cone", "/*", "!/plans/"},
				{"-c", lfsArg, GitCmdConfig, "--worktree", "--replace-all", "lfs.fetchexclude", "assets/**,images/**"},
				{"-c", lfsArg, GitCmdCheckout},
			},
			wantAbsent: [][]string{
				{GitCmdSparseCheckout, "disable"},
				{GitCmdConfig, "--worktree", "--unset-all"},
			},
		},
		{
			name:            "inherited_settings_kept_without_full",
			worktreeConfig:  true,
			inheritedConfig: inheritedCfg,
			wantCalls: [][]string{
				{GitCmdWorktree, GitWorktreeAdd, "-b"},
			},
			wantAbsent: [][]string{
				{GitCmdConfig},
				{GitCmdWorktree, GitWorktreeAdd, "--no-checkout"},
				{GitCmdSparseCheckout},
				{GitCmdCheckout},
			},
		},
		{
			name:            "full_resets_implicit_true_sparse_checkout",
			worktreeConfig:  true,
			inheritedConfig: "core.sparsecheckout\n",
			full:            true,
			wantCalls: [][]string{
				{GitCmdWorktree, GitWorktreeAdd, "--no-checkout", "-b"},
				{GitCmdSparseCheckout, "disable"},
				{GitCmdCheckout},
			},
			wantAbsent: [][]string{
				{GitCmdConfig, "--worktree", "--unset-all"},
			},
		},
		{
			name:            "full_keeps_disabled_sparse_checkout",
			worktreeConfig:  true,
			inheritedConfig: "core.sparsecheckout false\n",
			full:            true,
			wantCalls: [][]string{
				{GitCmdWorktree, GitWorktreeAdd, "-b"},
			},
			wantAbsent: [][]string{
				{GitCmdWorktree, GitWorktreeAdd, "--no-checkout"},
				{GitCmdSparseCheckout},
				{GitCmdCheckout},
			},
		},
		{
			name:           "sparse_checkout_requires_git_2_36",
			sparseCheckout: []string{"/*", "!/plans/"},
			version:        "2.35.1",
			wantErr:        "sparse_checkout requires git 2.36 or later (found 2.35)",
			wantAbsent: [][]string{
				{GitCmdWorktree, GitWorktreeAdd},
			},
		},
		{
			name:            "lfs_fetch_exclude_rejects_comma",
			lfsFetchExclude: []string{"assets/{png,jpg}/**"},
			wantErr:         "must not contain a comma",
			wantAbsent: [][]string{
				{GitCmdWorktree, GitWorktreeAdd},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			version := tt.version
			if version == "" {
				version = "2.39.5 (Apple Git-154)"
			}
			exec := &checkoutSettingsExecutor{
				inner:           &testutil.MockGitExecutor{},
				version:         version,
				worktreeConfig:  tt.worktreeConfig,
				inheritedConfig: tt.inheritedConfig,
			}
			cmd := &AddCommand{
				FS:  &testutil.MockFS{},
				Git: &GitRunner{Executor: exec, Log: NewNopLogger()},
				Config: &Config{
					WorktreeSourceDir:   "/repo/main",
					WorktreeDestBaseDir: "/repo/main-worktree",
					LFSFetchExclude:     tt.lfsFetchExclude,
					SparseCheckout:      tt.sparseCheckout,
				},
				Log:  NewNopLogger(),
				Full: tt.full,
			}

			result, err := cmd.Run(t.Context(), "feature/checkout")
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want containing %q", err, tt.wantErr)
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			last := -1
			for _, want := range tt.wantCalls {
				idx := -1
				for i := last + 1; i < len(exec.calls); i++ {
					call := exec.calls[i]
					if len(call) >= len(want) && slices.Equal(call[:len(want)], want) {
						idx = i
						break
					}
				}
				if idx < 0 {
					t.Fatalf("call %v not found after index %d in %v", want, last, exec.calls)
				}
				last = idx
			}
			for _, absent := range tt.wantAbsent {
				if absent[0] == "-c" {
					for _, call := range exec.calls {
						if slices.Contains(call, "-c") {
							t.Errorf("unexpected -c in %v", call)
						}
					}
					continue
				}
				if i := exec.find(absent...); i >= 0 {
					t.Errorf("unexpected call %v", exec.calls[i])
				}
			}

			if tt.wantErr == "" && !tt.full {
				if !slices.Equal(result.LFSFetchExclude, tt.lfsFetchExclude) {
					t.Errorf("LFSFetchExclude = %v, want %v", result.LFSFetchExclude, tt.lfsFetchExclude)
				}
				if !slices.Equal(result.SparseCheckout, tt.sparseCheckout) {
					t.Errorf("SparseCheckout = %v, want %v", result.SparseCheckout, tt.sparseCheckout)
				}
			}
			if tt.full && (result.LFSFetchExclude != nil || result.SparseCheckout != nil) {
				t.Errorf("full: result = %v / %v, want nil", result.LFSFetchExclude, result.SparseCheckout)
			}
		})
	}
}

func TestAddCommand_Run_SparseCheckoutFailureRemovesWorktree(t *testing.T) {
	t.Parallel()

	exec := &checkoutSettingsExecutor{
		inner:             &testutil.MockGitExecutor{HasChanges: true, StashHash: "abc123"},
		version:           "2.53.0",
		sparseCheckoutErr: errors.New("sparse-checkout failed"),
	}
	cmd := &AddCommand{
		FS:  &testutil.MockFS{},
		Git: &GitRunner{Executor: exec, Log: NewNopLogger()},
		Config: &Config{
			WorktreeSourceDir:   "/repo/main",
			WorktreeDestBaseDir: "/repo/main-worktree",
			SparseCheckout:      []string{"/*"},
		},
		Log:  NewNopLogger(),
		Lock: true,
		Sync: true,
	}

	_, err := cmd.Run(t.Context(), "feature/sparse")
	if err == nil || !strings.Contains(err.Error(), "failed to set sparse checkout") {
		t.Fatalf("err = %v, want sparse checkout failure", err)
	}
	// -f -f is required to remove a worktree created with --lock
	if exec.find(GitCmdWorktree, GitWorktreeRemove, "-f", "-f") < 0 {
		t.Errorf("locked worktree was not force removed: %v", exec.calls)
	}
	if exec.find(GitCmdBranch, "-D", "feature/sparse") < 0 {
		t.Errorf("created branch was not deleted: %v", exec.calls)
	}
	if exec.find(GitCmdStash, GitStashApply, "abc123") < 0 {
		t.Errorf("stash was not restored: %v", exec.calls)
	}
}

func TestAddResult_Format_CheckoutSettings(t *testing.T) {
	t.Parallel()

	result := AddResult{
		Branch:          "feat/x",
		WorktreePath:    "/repo/feat/x",
		LFSFetchExclude: []string{"assets/**"},
		SparseCheckout:  []string{"/*", "!/plans/"},
	}

	got := result.Format(AddFormatOptions{}).Stdout
	want := "twig add: feat/x (0 symlinks, 1 lfs excludes, 2 sparse patterns)\n"
	if got != want {
		t.Errorf("default stdout = %q, want %q", got, want)
	}

	verbose := result.Format(AddFormatOptions{Verbose: true}).Stdout
	for _, line := range []string{
		"Excluded LFS files: assets/**\n",
		"Applied sparse checkout: /*, !/plans/\n",
	} {
		if !strings.Contains(verbose, line) {
			t.Errorf("verbose stdout = %q, want to contain %q", verbose, line)
		}
	}
}
