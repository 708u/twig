package gwt

import (
	"bytes"
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
		setupFS     func(t *testing.T) *testutil.MockFS
		setupGit    func(t *testing.T, captured *[]string) *testutil.MockGitExecutor
		wantErr     bool
		errContains string
		wantBFlag   bool
		checkPath   string
	}{
		{
			name:   "new_branch",
			branch: "feature/test",
			config: &Config{Symlinks: []string{".envrc"}},
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
			config: &Config{Symlinks: []string{".envrc"}},
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
			config: &Config{},
			setupFS: func(t *testing.T) *testutil.MockFS {
				t.Helper()
				return &testutil.MockFS{
					ExistingPaths: []string{"/repo/feature-test"},
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
			name:   "getwd_error",
			branch: "feature/test",
			config: &Config{},
			setupFS: func(t *testing.T) *testutil.MockFS {
				t.Helper()
				return &testutil.MockFS{
					GetwdErr: errors.New("getwd failed"),
				}
			},
			setupGit: func(t *testing.T, captured *[]string) *testutil.MockGitExecutor {
				t.Helper()
				return &testutil.MockGitExecutor{}
			},
			wantErr:     true,
			errContains: "failed to get current directory",
		},
		{
			name:   "branch_checked_out",
			branch: "already-used",
			config: &Config{},
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
			config: &Config{},
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
			config:    &Config{WorktreeDestBaseDir: "/worktrees"},
			checkPath: "/worktrees/feature-foo",
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var stdout, stderr bytes.Buffer
			var captured []string

			mockFS := tt.setupFS(t)
			mockGit := tt.setupGit(t, &captured)

			cmd := &AddCommand{
				FS:     mockFS,
				Git:    &GitRunner{Executor: mockGit, Stdout: &stdout},
				Config: tt.config,
				Stdout: &stdout,
				Stderr: &stderr,
			}

			err := cmd.Run(tt.branch)

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
		})
	}
}

func TestAddCommand_createSymlinks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		targets     []string
		setupFS     func(t *testing.T) *testutil.MockFS
		wantErr     bool
		errContains string
		wantStderr  string
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
			wantErr: false,
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
			wantErr:    false,
			wantStderr: "warning: .envrc does not match any files, skipping",
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var stdout, stderr bytes.Buffer

			mockFS := tt.setupFS(t)

			cmd := &AddCommand{
				FS:     mockFS,
				Stdout: &stdout,
				Stderr: &stderr,
			}

			err := cmd.createSymlinks("/src", "/dst", tt.targets)

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

			if tt.wantStderr != "" && !strings.Contains(stderr.String(), tt.wantStderr) {
				t.Errorf("stderr %q should contain %q", stderr.String(), tt.wantStderr)
			}
		})
	}
}
