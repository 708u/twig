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
				Git: &GitRunner{Executor: mock},
			}

			result, err := cmd.Run()

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

func TestListResult_Format(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		worktrees  []Worktree
		opts       ListFormatOptions
		wantStdout string
	}{
		{
			name: "git worktree list compatible format",
			worktrees: []Worktree{
				{Path: "/repo/main", Branch: "main", HEAD: "abc1234567890"},
				{Path: "/repo/worktree/feat-a", Branch: "feat/a", HEAD: "def5678901234"},
			},
			wantStdout: "/repo/main             abc1234 [main]\n/repo/worktree/feat-a  def5678 [feat/a]\n",
		},
		{
			name: "detached HEAD",
			worktrees: []Worktree{
				{Path: "/repo/worktree/detached", HEAD: "abc1234567890", Detached: true},
			},
			wantStdout: "/repo/worktree/detached  abc1234 (detached HEAD)\n",
		},
		{
			name: "locked worktree",
			worktrees: []Worktree{
				{Path: "/repo/worktree/locked", Branch: "locked-branch", HEAD: "abc1234567890", Locked: true},
			},
			wantStdout: "/repo/worktree/locked  abc1234 [locked-branch] locked\n",
		},
		{
			name: "prunable worktree",
			worktrees: []Worktree{
				{Path: "/repo/worktree/prunable", HEAD: "abc1234567890", Detached: true, Prunable: true},
			},
			wantStdout: "/repo/worktree/prunable  abc1234 (detached HEAD) prunable\n",
		},
		{
			name: "bare repository",
			worktrees: []Worktree{
				{Path: "/repo/bare", HEAD: "abc1234567890", Bare: true},
			},
			wantStdout: "/repo/bare  abc1234 (bare)\n",
		},
		{
			name:       "empty list",
			worktrees:  []Worktree{},
			wantStdout: "",
		},
		{
			name: "quiet format outputs paths only",
			worktrees: []Worktree{
				{Path: "/repo/main", Branch: "main", HEAD: "abc1234567890"},
				{Path: "/repo/worktree/feat-a", Branch: "feat/a", HEAD: "def5678901234"},
			},
			opts:       ListFormatOptions{Quiet: true},
			wantStdout: "/repo/main\n/repo/worktree/feat-a\n",
		},
		{
			name:       "quiet format with empty list",
			worktrees:  []Worktree{},
			opts:       ListFormatOptions{Quiet: true},
			wantStdout: "",
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
