package twig

import (
	"io/fs"
	"testing"

	"github.com/708u/twig/internal/testutil"
)

func Test_resolveSource(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		source        string
		defaultSource string
		want          string
		wantErr       bool
		errContains   string
	}{
		{
			name:    "explicit_source",
			source:  "main",
			want:    "main",
			wantErr: false,
		},
		{
			name:          "default_source_from_config",
			source:        "",
			defaultSource: "develop",
			want:          "develop",
			wantErr:       false,
		},
		{
			name:        "no_source_no_default",
			source:      "",
			wantErr:     true,
			errContains: "source branch not specified",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := resolveSource(tt.source, tt.defaultSource)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errContains)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSyncResult_Format(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		result     SyncResult
		opts       SyncFormatOptions
		wantStdout string
		wantStderr string
	}{
		{
			name: "nothing_to_sync",
			result: SyncResult{
				NothingToSync: true,
			},
			opts:       SyncFormatOptions{},
			wantStdout: "nothing to sync (no symlinks or submodules configured)\n",
		},
		{
			name: "check_mode_single_target",
			result: SyncResult{
				Check:        true,
				SourceBranch: "main",
				Targets: []SyncTargetResult{
					{
						Branch:       "feat/a",
						WorktreePath: "/repo/feat/a",
						Symlinks: []SymlinkResult{
							{Src: "/repo/main/.envrc", Dst: "/repo/feat/a/.envrc"},
						},
						SubmoduleInit: SubmoduleInitResult{Attempted: true},
					},
				},
			},
			opts: SyncFormatOptions{},
			wantStdout: `Would sync from main:

feat/a:
  Would create symlink: /repo/feat/a/.envrc
  Would initialize submodules

`,
		},
		{
			name: "check_mode_skipped_target_verbose",
			result: SyncResult{
				Check:        true,
				SourceBranch: "main",
				Targets: []SyncTargetResult{
					{
						Branch:     "feat/a",
						Skipped:    true,
						SkipReason: "up to date",
					},
				},
			},
			opts: SyncFormatOptions{Verbose: true},
			wantStdout: `Would sync from main:

feat/a:
  (skipped: up to date)

`,
		},
		{
			name: "normal_mode_single_target",
			result: SyncResult{
				Check:        false,
				SourceBranch: "main",
				Targets: []SyncTargetResult{
					{
						Branch:       "feat/a",
						WorktreePath: "/repo/feat/a",
						Symlinks: []SymlinkResult{
							{Src: "/repo/main/.envrc", Dst: "/repo/feat/a/.envrc"},
							{Src: "/repo/main/.tool-versions", Dst: "/repo/feat/a/.tool-versions"},
						},
						SubmoduleInit: SubmoduleInitResult{Attempted: true, Count: 1},
					},
				},
			},
			opts:       SyncFormatOptions{},
			wantStdout: "twig sync: feat/a (2 symlinks, 1 submodule(s))\n",
		},
		{
			name: "normal_mode_verbose",
			result: SyncResult{
				Check:        false,
				SourceBranch: "main",
				Targets: []SyncTargetResult{
					{
						Branch:       "feat/a",
						WorktreePath: "/repo/feat/a",
						Symlinks: []SymlinkResult{
							{Src: "/repo/main/.envrc", Dst: "/repo/feat/a/.envrc"},
						},
						SubmoduleInit: SubmoduleInitResult{Attempted: true, Count: 2},
					},
				},
			},
			opts: SyncFormatOptions{Verbose: true},
			wantStdout: `Syncing from main to feat/a
Created symlink: /repo/feat/a/.envrc -> /repo/main/.envrc
Initialized 2 submodule(s)
twig sync: feat/a (1 symlinks, 2 submodule(s))
`,
		},
		{
			name: "skipped_target",
			result: SyncResult{
				Check:        false,
				SourceBranch: "main",
				Targets: []SyncTargetResult{
					{
						Branch:     "feat/a",
						Skipped:    true,
						SkipReason: "up to date",
					},
				},
			},
			opts:       SyncFormatOptions{},
			wantStdout: "twig sync: feat/a (skipped: up to date)\n",
		},
		{
			name: "error_target",
			result: SyncResult{
				Check:        false,
				SourceBranch: "main",
				Targets: []SyncTargetResult{
					{
						Branch: "feat/a",
						Err:    testutil.NewError("failed to create symlink"),
					},
				},
			},
			opts:       SyncFormatOptions{},
			wantStdout: "",
			wantStderr: "error: feat/a: failed to create symlink\n",
		},
		{
			name: "warning_symlink_skipped",
			result: SyncResult{
				Check:        false,
				SourceBranch: "main",
				Targets: []SyncTargetResult{
					{
						Branch:       "feat/a",
						WorktreePath: "/repo/feat/a",
						Symlinks: []SymlinkResult{
							{Src: "/repo/main/.envrc", Dst: "/repo/feat/a/.envrc", Skipped: true, Reason: "already exists"},
						},
						Skipped:    true,
						SkipReason: "up to date",
					},
				},
			},
			opts:       SyncFormatOptions{},
			wantStdout: "twig sync: feat/a (skipped: up to date)\n",
			wantStderr: "warning: already exists\n",
		},
		{
			name: "quiet_mode",
			result: SyncResult{
				Check:        false,
				SourceBranch: "main",
				Targets: []SyncTargetResult{
					{
						Branch:       "feat/a",
						WorktreePath: "/repo/feat/a",
						Symlinks: []SymlinkResult{
							{Src: "/repo/main/.envrc", Dst: "/repo/feat/a/.envrc"},
						},
					},
					{
						Branch:     "feat/b",
						Skipped:    true,
						SkipReason: "up to date",
					},
				},
			},
			opts:       SyncFormatOptions{Quiet: true},
			wantStdout: "/repo/feat/a\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := tt.result.Format(tt.opts)

			if got.Stdout != tt.wantStdout {
				t.Errorf("Stdout:\ngot:\n%s\nwant:\n%s", got.Stdout, tt.wantStdout)
			}
			if got.Stderr != tt.wantStderr {
				t.Errorf("Stderr:\ngot:\n%s\nwant:\n%s", got.Stderr, tt.wantStderr)
			}
		})
	}
}

func TestSyncResult_Counts(t *testing.T) {
	t.Parallel()

	result := SyncResult{
		Targets: []SyncTargetResult{
			{Branch: "feat/a", Err: nil, Skipped: false},
			{Branch: "feat/b", Err: testutil.NewError("error"), Skipped: false},
			{Branch: "feat/c", Err: nil, Skipped: true},
			{Branch: "feat/d", Err: nil, Skipped: false},
		},
	}

	t.Run("HasErrors", func(t *testing.T) {
		if !result.HasErrors() {
			t.Error("expected HasErrors() to be true")
		}
	})

	t.Run("ErrorCount", func(t *testing.T) {
		if got := result.ErrorCount(); got != 1 {
			t.Errorf("ErrorCount() = %d, want 1", got)
		}
	})

	t.Run("SuccessCount", func(t *testing.T) {
		if got := result.SuccessCount(); got != 2 {
			t.Errorf("SuccessCount() = %d, want 2", got)
		}
	})

	t.Run("SkippedCount", func(t *testing.T) {
		if got := result.SkippedCount(); got != 1 {
			t.Errorf("SkippedCount() = %d, want 1", got)
		}
	})

	t.Run("SyncedBranches", func(t *testing.T) {
		got := result.SyncedBranches()
		want := []string{"feat/a", "feat/d"}
		if len(got) != len(want) {
			t.Errorf("SyncedBranches() = %v, want %v", got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("SyncedBranches()[%d] = %q, want %q", i, got[i], want[i])
			}
		}
	})
}

func TestSyncCommand_predictSymlinks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		patterns    []string
		setupFS     func() *testutil.MockFS
		wantCreated int
		wantSkipped int
		wantErr     bool
	}{
		{
			name:     "new_symlink",
			patterns: []string{".envrc"},
			setupFS: func() *testutil.MockFS {
				return &testutil.MockFS{
					GlobResults: map[string][]string{
						".envrc": {".envrc"},
					},
				}
			},
			wantCreated: 1,
		},
		{
			name:     "existing_symlink_replaced",
			patterns: []string{".envrc"},
			setupFS: func() *testutil.MockFS {
				return &testutil.MockFS{
					GlobResults: map[string][]string{
						".envrc": {".envrc"},
					},
					ExistingPaths: []string{"/dst/.envrc"},
					LstatFunc: func(name string) (fs.FileInfo, error) {
						return &testutil.MockFileInfo{
							ModeVal: fs.ModeSymlink,
						}, nil
					},
				}
			},
			wantCreated: 1,
		},
		{
			name:     "existing_regular_file_skipped",
			patterns: []string{".envrc"},
			setupFS: func() *testutil.MockFS {
				return &testutil.MockFS{
					GlobResults: map[string][]string{
						".envrc": {".envrc"},
					},
					ExistingPaths: []string{"/dst/.envrc"},
					LstatFunc: func(name string) (fs.FileInfo, error) {
						return &testutil.MockFileInfo{
							ModeVal: 0, // regular file (no special mode bits)
						}, nil
					},
				}
			},
			wantSkipped: 1,
		},
		{
			name:     "no_matches",
			patterns: []string{".envrc"},
			setupFS: func() *testutil.MockFS {
				return &testutil.MockFS{
					GlobResults: map[string][]string{},
				}
			},
			wantSkipped: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mockFS := tt.setupFS()
			cmd := &SyncCommand{FS: mockFS}

			results, err := cmd.predictSymlinks("/src", "/dst", tt.patterns)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			var created, skipped int
			for _, r := range results {
				if r.Skipped {
					skipped++
				} else {
					created++
				}
			}

			if created != tt.wantCreated {
				t.Errorf("created = %d, want %d", created, tt.wantCreated)
			}
			if skipped != tt.wantSkipped {
				t.Errorf("skipped = %d, want %d", skipped, tt.wantSkipped)
			}
		})
	}
}

// contains checks if s contains substr.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
