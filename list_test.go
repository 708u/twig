package twig

import (
	"testing"

	"github.com/708u/twig/internal/testutil"
)

func TestListCommand_Run(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		worktrees []testutil.MockWorktree
		wantCount int
		wantErr   bool
	}{
		{
			name: "multiple worktrees",
			worktrees: []testutil.MockWorktree{
				{Path: "/repo/main", Branch: "main"},
				{Path: "/repo/worktree/feat-a", Branch: "feat/a"},
				{Path: "/repo/worktree/feat-b", Branch: "feat/b"},
			},
			wantCount: 3,
		},
		{
			name: "single worktree",
			worktrees: []testutil.MockWorktree{
				{Path: "/repo/main", Branch: "main"},
			},
			wantCount: 1,
		},
		{
			name:      "empty worktrees",
			worktrees: []testutil.MockWorktree{},
			wantCount: 0,
		},
		{
			name: "detached HEAD",
			worktrees: []testutil.MockWorktree{
				{Path: "/repo/main", Branch: "main"},
				{Path: "/repo/worktree/detached", Detached: true, HEAD: "abc1234567890"},
			},
			wantCount: 2,
		},
		{
			name: "locked worktree",
			worktrees: []testutil.MockWorktree{
				{Path: "/repo/main", Branch: "main"},
				{Path: "/repo/worktree/locked", Branch: "locked-branch", Locked: true, LockReason: "in use"},
			},
			wantCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mock := &testutil.MockGitExecutor{
				Worktrees: tt.worktrees,
			}
			cmd := &ListCommand{
				Git: &GitRunner{Executor: mock, Log: NewNopLogger()},
				Log: NewNopLogger(),
			}

			result, err := cmd.Run(t.Context(), ListOptions{})

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(result.Worktrees) != tt.wantCount {
				t.Errorf("got %d worktrees, want %d", len(result.Worktrees), tt.wantCount)
			}

			for i, wt := range result.Worktrees {
				if wt.Path != tt.worktrees[i].Path {
					t.Errorf("worktree[%d].Path = %q, want %q", i, wt.Path, tt.worktrees[i].Path)
				}
				if wt.Detached != tt.worktrees[i].Detached {
					t.Errorf("worktree[%d].Detached = %v, want %v", i, wt.Detached, tt.worktrees[i].Detached)
				}
				if wt.Locked != tt.worktrees[i].Locked {
					t.Errorf("worktree[%d].Locked = %v, want %v", i, wt.Locked, tt.worktrees[i].Locked)
				}
			}
		})
	}
}

func TestListCommand_Run_Verbose(t *testing.T) {
	t.Parallel()

	mock := &testutil.MockGitExecutor{
		Worktrees: []testutil.MockWorktree{
			{Path: "/repo/main", Branch: "main"},
			{Path: "/repo/worktree/feat-a", Branch: "feat/a"},
		},
		StatusByDir: map[string]string{
			"/repo/worktree/feat-a": " M src/main.go\n?? tmp/debug.log\n",
		},
	}
	cmd := &ListCommand{
		Git: &GitRunner{Executor: mock, Log: NewNopLogger()},
		Log: NewNopLogger(),
	}

	result, err := cmd.Run(t.Context(), ListOptions{Verbose: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Worktrees) != 2 {
		t.Fatalf("got %d worktrees, want 2", len(result.Worktrees))
	}

	// Main worktree should have no changed files
	if len(result.Worktrees[0].ChangedFiles) != 0 {
		t.Errorf("main worktree should have no changed files, got %d", len(result.Worktrees[0].ChangedFiles))
	}

	// feat/a should have 2 changed files
	if len(result.Worktrees[1].ChangedFiles) != 2 {
		t.Errorf("feat/a worktree should have 2 changed files, got %d", len(result.Worktrees[1].ChangedFiles))
	}
}

func TestListCommand_Run_VerboseSkipsBare(t *testing.T) {
	t.Parallel()

	mock := &testutil.MockGitExecutor{
		Worktrees: []testutil.MockWorktree{
			{Path: "/repo/bare", Bare: true},
			{Path: "/repo/main", Branch: "main"},
		},
	}
	cmd := &ListCommand{
		Git: &GitRunner{Executor: mock, Log: NewNopLogger()},
		Log: NewNopLogger(),
	}

	result, err := cmd.Run(t.Context(), ListOptions{Verbose: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Bare worktree should have no changed files (skipped)
	if len(result.Worktrees[0].ChangedFiles) != 0 {
		t.Errorf("bare worktree should have no changed files, got %d", len(result.Worktrees[0].ChangedFiles))
	}
}

func TestNewListCommand_NilLogger(t *testing.T) {
	t.Parallel()

	mock := &testutil.MockGitExecutor{
		Worktrees: []testutil.MockWorktree{
			{Path: "/repo/main", Branch: "main"},
		},
	}
	git := &GitRunner{Executor: mock, Log: NewNopLogger()}

	// Should not panic when log is nil
	cmd := NewListCommand(git, nil)
	if cmd.Log == nil {
		t.Error("Log should not be nil after NewListCommand")
	}

	// Should be able to run without panic
	_, err := cmd.Run(t.Context(), ListOptions{})
	if err != nil {
		t.Errorf("Run() error = %v", err)
	}
}

func TestListResult_Format(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		worktrees  []ListWorktreeInfo
		opts       FormatOptions
		wantStdout string
	}{
		{
			name: "git worktree list compatible format",
			worktrees: []ListWorktreeInfo{
				{Worktree: Worktree{Path: "/repo/main", Branch: "main", HEAD: "abc1234567890"}},
				{Worktree: Worktree{Path: "/repo/worktree/feat-a", Branch: "feat/a", HEAD: "def5678901234"}},
			},
			wantStdout: "/repo/main             abc1234 [main]\n/repo/worktree/feat-a  def5678 [feat/a]\n",
		},
		{
			name: "detached HEAD",
			worktrees: []ListWorktreeInfo{
				{Worktree: Worktree{Path: "/repo/worktree/detached", HEAD: "abc1234567890", Detached: true}},
			},
			wantStdout: "/repo/worktree/detached  abc1234 (detached HEAD)\n",
		},
		{
			name: "locked worktree",
			worktrees: []ListWorktreeInfo{
				{Worktree: Worktree{Path: "/repo/worktree/locked", Branch: "locked-branch", HEAD: "abc1234567890", Locked: true}},
			},
			wantStdout: "/repo/worktree/locked  abc1234 [locked-branch] locked\n",
		},
		{
			name: "prunable worktree",
			worktrees: []ListWorktreeInfo{
				{Worktree: Worktree{Path: "/repo/worktree/prunable", HEAD: "abc1234567890", Detached: true, Prunable: true}},
			},
			wantStdout: "/repo/worktree/prunable  abc1234 (detached HEAD) prunable\n",
		},
		{
			name: "bare repository",
			worktrees: []ListWorktreeInfo{
				{Worktree: Worktree{Path: "/repo/bare", HEAD: "abc1234567890", Bare: true}},
			},
			wantStdout: "/repo/bare  abc1234 (bare)\n",
		},
		{
			name:       "empty list",
			worktrees:  []ListWorktreeInfo{},
			wantStdout: "",
		},
		{
			name: "quiet format outputs paths only",
			worktrees: []ListWorktreeInfo{
				{Worktree: Worktree{Path: "/repo/main", Branch: "main", HEAD: "abc1234567890"}},
				{Worktree: Worktree{Path: "/repo/worktree/feat-a", Branch: "feat/a", HEAD: "def5678901234"}},
			},
			opts:       FormatOptions{Quiet: true},
			wantStdout: "/repo/main\n/repo/worktree/feat-a\n",
		},
		{
			name:       "quiet format with empty list",
			worktrees:  []ListWorktreeInfo{},
			opts:       FormatOptions{Quiet: true},
			wantStdout: "",
		},
		{
			name: "quiet overrides verbose",
			worktrees: []ListWorktreeInfo{
				{Worktree: Worktree{Path: "/repo/main", Branch: "main", HEAD: "abc1234567890"}},
			},
			opts:       FormatOptions{Quiet: true, Verbose: true},
			wantStdout: "/repo/main\n",
		},
		{
			name: "verbose no changes same as default",
			worktrees: []ListWorktreeInfo{
				{Worktree: Worktree{Path: "/repo/main", Branch: "main", HEAD: "abc1234567890"}},
				{Worktree: Worktree{Path: "/repo/worktree/feat-a", Branch: "feat/a", HEAD: "def5678901234"}},
			},
			opts:       FormatOptions{Verbose: true},
			wantStdout: "/repo/main             abc1234 [main]\n/repo/worktree/feat-a  def5678 [feat/a]\n",
		},
		{
			name: "verbose with changes",
			worktrees: []ListWorktreeInfo{
				{Worktree: Worktree{Path: "/repo/main", Branch: "main", HEAD: "abc1234567890"}},
				{
					Worktree: Worktree{Path: "/repo/worktree/feat-a", Branch: "feat/a", HEAD: "def5678901234"},
					ChangedFiles: []FileStatus{
						{Status: " M", Path: "src/main.go"},
						{Status: "??", Path: "tmp/debug.log"},
					},
				},
			},
			opts: FormatOptions{Verbose: true},
			wantStdout: "/repo/main             abc1234 [main]\n" +
				"/repo/worktree/feat-a  def5678 [feat/a]\n" +
				"   M src/main.go\n" +
				"  ?? tmp/debug.log\n",
		},
		{
			name: "verbose with locked and lock reason",
			worktrees: []ListWorktreeInfo{
				{Worktree: Worktree{Path: "/repo/worktree/usb", Branch: "feat/usb", HEAD: "abc1234567890", Locked: true, LockReason: "USB drive work"}},
			},
			opts: FormatOptions{Verbose: true},
			wantStdout: "/repo/worktree/usb  abc1234 [feat/usb] locked\n" +
				"  lock reason: USB drive work\n",
		},
		{
			name: "verbose with locked without reason",
			worktrees: []ListWorktreeInfo{
				{Worktree: Worktree{Path: "/repo/worktree/locked", Branch: "feat/locked", HEAD: "abc1234567890", Locked: true}},
			},
			opts:       FormatOptions{Verbose: true},
			wantStdout: "/repo/worktree/locked  abc1234 [feat/locked] locked\n",
		},
		{
			name: "verbose with changes and lock reason",
			worktrees: []ListWorktreeInfo{
				{
					Worktree: Worktree{Path: "/repo/worktree/usb", Branch: "feat/usb", HEAD: "abc1234567890", Locked: true, LockReason: "USB drive work"},
					ChangedFiles: []FileStatus{
						{Status: " M", Path: "config.toml"},
					},
				},
			},
			opts: FormatOptions{Verbose: true},
			wantStdout: "/repo/worktree/usb  abc1234 [feat/usb] locked\n" +
				"  lock reason: USB drive work\n" +
				"   M config.toml\n",
		},
		{
			name: "verbose bare and prunable have no changes",
			worktrees: []ListWorktreeInfo{
				{Worktree: Worktree{Path: "/repo/bare", HEAD: "abc1234567890", Bare: true}},
				{Worktree: Worktree{Path: "/repo/prunable", HEAD: "def5678901234", Detached: true, Prunable: true}},
			},
			opts: FormatOptions{Verbose: true},
			wantStdout: "/repo/bare      abc1234 (bare)\n" +
				"/repo/prunable  def5678 (detached HEAD) prunable\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := ListResult{Worktrees: tt.worktrees}
			formatted := result.Format(tt.opts)

			if formatted.Stdout != tt.wantStdout {
				t.Errorf("Stdout = %q, want %q", formatted.Stdout, tt.wantStdout)
			}
		})
	}
}

func TestWorktree_ShortHEAD(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		head string
		want string
	}{
		{
			name: "40 character hash",
			head: "abc1234567890abcdef1234567890abcdef1234",
			want: "abc1234",
		},
		{
			name: "short hash",
			head: "abc12",
			want: "abc12",
		},
		{
			name: "empty hash",
			head: "",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			wt := Worktree{HEAD: tt.head}
			if got := wt.ShortHEAD(); got != tt.want {
				t.Errorf("ShortHEAD() = %q, want %q", got, tt.want)
			}
		})
	}
}
