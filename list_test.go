package gwt

import (
	"testing"

	"github.com/708u/gwt/internal/testutil"
)

func TestListCommand_Run(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		worktrees  []testutil.MockWorktree
		wantCount  int
		wantErr    bool
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
				if wt.Branch != tt.worktrees[i].Branch {
					t.Errorf("worktree[%d].Branch = %q, want %q", i, wt.Branch, tt.worktrees[i].Branch)
				}
			}
		})
	}
}

func TestListResult_Format(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		worktrees  []WorktreeInfo
		opts       ListFormatOptions
		wantStdout string
	}{
		{
			name: "default shows branch names",
			worktrees: []WorktreeInfo{
				{Path: "/repo/main", Branch: "main"},
				{Path: "/repo/worktree/feat-a", Branch: "feat/a"},
			},
			opts:       ListFormatOptions{ShowPath: false},
			wantStdout: "main\nfeat/a\n",
		},
		{
			name: "with path shows full paths",
			worktrees: []WorktreeInfo{
				{Path: "/repo/main", Branch: "main"},
				{Path: "/repo/worktree/feat-a", Branch: "feat/a"},
			},
			opts:       ListFormatOptions{ShowPath: true},
			wantStdout: "/repo/main\n/repo/worktree/feat-a\n",
		},
		{
			name:       "empty list",
			worktrees:  []WorktreeInfo{},
			opts:       ListFormatOptions{ShowPath: false},
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
