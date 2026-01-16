package twig

import (
	"errors"
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
				Git:          &GitRunner{Executor: mockGit},
				Config:       tt.config,
				Sync:         tt.sync,
				CarryFrom:    tt.carryFrom,
				FilePatterns: tt.filePatterns,
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
				Git:        &GitRunner{Executor: mockGit},
				Config:     tt.config,
				Lock:       tt.lock,
				LockReason: tt.lockReason,
			}

			_, err := cmd.Run(tt.branch)
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
		initSubmodules            *bool
		setupFS                   func(t *testing.T) *testutil.MockFS
		setupGit                  func(t *testing.T, captured *[]string) *testutil.MockGitExecutor
		wantSubmodulesInited      bool
		wantSubmoduleCount        int
		wantSubmoduleUpdateCalled bool
		wantSubmoduleInitError    bool
	}{
		{
			name:           "init_submodules_enabled_with_submodules",
			branch:         "feature/submod",
			config:         &Config{WorktreeSourceDir: "/repo/main", WorktreeDestBaseDir: "/repo/main-worktree"},
			initSubmodules: boolPtr(true),
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
			name:           "init_submodules_enabled_no_submodules",
			branch:         "feature/no-submod",
			config:         &Config{WorktreeSourceDir: "/repo/main", WorktreeDestBaseDir: "/repo/main-worktree"},
			initSubmodules: boolPtr(true),
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
			wantSubmoduleUpdateCalled: false,
		},
		{
			name:           "init_submodules_disabled",
			branch:         "feature/disabled",
			config:         &Config{WorktreeSourceDir: "/repo/main", WorktreeDestBaseDir: "/repo/main-worktree"},
			initSubmodules: boolPtr(false),
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
			name:           "init_submodules_from_config",
			branch:         "feature/from-config",
			config:         &Config{WorktreeSourceDir: "/repo/main", WorktreeDestBaseDir: "/repo/main-worktree", InitSubmodules: boolPtr(true)},
			initSubmodules: nil, // Use config
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
			name:           "cli_flag_overrides_config",
			branch:         "feature/override",
			config:         &Config{WorktreeSourceDir: "/repo/main", WorktreeDestBaseDir: "/repo/main-worktree", InitSubmodules: boolPtr(true)},
			initSubmodules: boolPtr(false), // CLI flag overrides config
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
			name:           "init_submodules_error_is_warning",
			branch:         "feature/init-error",
			config:         &Config{WorktreeSourceDir: "/repo/main", WorktreeDestBaseDir: "/repo/main-worktree"},
			initSubmodules: boolPtr(true),
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
				Git:            &GitRunner{Executor: mockGit},
				Config:         tt.config,
				InitSubmodules: tt.initSubmodules,
			}

			result, err := cmd.Run(tt.branch)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.SubmodulesInited != tt.wantSubmodulesInited {
				t.Errorf("SubmodulesInited = %v, want %v", result.SubmodulesInited, tt.wantSubmodulesInited)
			}

			if result.SubmoduleCount != tt.wantSubmoduleCount {
				t.Errorf("SubmoduleCount = %v, want %v", result.SubmoduleCount, tt.wantSubmoduleCount)
			}

			if mockGit.SubmoduleUpdateCalled != tt.wantSubmoduleUpdateCalled {
				t.Errorf("SubmoduleUpdateCalled = %v, want %v", mockGit.SubmoduleUpdateCalled, tt.wantSubmoduleUpdateCalled)
			}

			if tt.wantSubmoduleInitError && result.SubmoduleInitError == nil {
				t.Error("expected SubmoduleInitError, got nil")
			}
			if !tt.wantSubmoduleInitError && result.SubmoduleInitError != nil {
				t.Errorf("unexpected SubmoduleInitError: %v", result.SubmoduleInitError)
			}
		})
	}
}

func TestAddResult_Format_Submodules(t *testing.T) {
	t.Parallel()

	t.Run("default_output_with_submodules", func(t *testing.T) {
		t.Parallel()

		result := AddResult{
			Branch:           "feature/test",
			WorktreePath:     "/worktrees/feature/test",
			Symlinks:         []SymlinkResult{},
			SubmodulesInited: true,
			SubmoduleCount:   2,
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
			Branch:           "feature/test",
			WorktreePath:     "/worktrees/feature/test",
			Symlinks:         []SymlinkResult{},
			SubmodulesInited: true,
			SubmoduleCount:   3,
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
			Branch:             "feature/test",
			WorktreePath:       "/worktrees/feature/test",
			Symlinks:           []SymlinkResult{},
			SubmodulesInited:   true,
			SubmoduleInitError: errors.New("failed to initialize submodules"),
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
			Branch:           "feature/test",
			WorktreePath:     "/worktrees/feature/test",
			Symlinks:         []SymlinkResult{},
			SubmodulesInited: true,
			SubmoduleCount:   0,
		}

		got := result.Format(AddFormatOptions{})
		want := "twig add: feature/test (0 symlinks)\n"

		if got.Stdout != want {
			t.Errorf("Stdout = %q, want %q", got.Stdout, want)
		}
	})
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
