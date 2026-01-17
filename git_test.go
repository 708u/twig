package twig

import (
	"testing"

	"github.com/708u/twig/internal/testutil"
)

func TestGitRunner_IsBranchUpstreamGone(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		branch       string
		upstreamGone []string
		want         bool
	}{
		{
			name:         "upstream is gone",
			branch:       "feat/squashed",
			upstreamGone: []string{"feat/squashed"},
			want:         true,
		},
		{
			name:         "upstream exists",
			branch:       "feat/new",
			upstreamGone: []string{},
			want:         false,
		},
		{
			name:         "multiple gone branches",
			branch:       "feat/b",
			upstreamGone: []string{"feat/a", "feat/b", "feat/c"},
			want:         true,
		},
		{
			name:         "different branch gone",
			branch:       "feat/x",
			upstreamGone: []string{"feat/y"},
			want:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mockGit := &testutil.MockGitExecutor{
				UpstreamGoneBranches: tt.upstreamGone,
			}
			runner := &GitRunner{Executor: mockGit, Log: NewNopLogger()}

			got, err := runner.IsBranchUpstreamGone(tt.branch)

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGitRunner_IsBranchMerged_WithSquashMerge(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		branch       string
		target       string
		merged       map[string][]string
		upstreamGone []string
		want         bool
	}{
		{
			name:   "traditional merge detected",
			branch: "feat/merged",
			target: "main",
			merged: map[string][]string{
				"main": {"feat/merged"},
			},
			upstreamGone: []string{},
			want:         true,
		},
		{
			name:   "squash merge detected via upstream gone",
			branch: "feat/squashed",
			target: "main",
			merged: map[string][]string{
				"main": {}, // Not in --merged output
			},
			upstreamGone: []string{"feat/squashed"}, // But upstream is gone
			want:         true,
		},
		{
			name:   "not merged at all",
			branch: "feat/new",
			target: "main",
			merged: map[string][]string{
				"main": {},
			},
			upstreamGone: []string{},
			want:         false,
		},
		{
			name:   "traditional merge takes precedence",
			branch: "feat/both",
			target: "main",
			merged: map[string][]string{
				"main": {"feat/both"},
			},
			upstreamGone: []string{"feat/both"},
			want:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mockGit := &testutil.MockGitExecutor{
				MergedBranches:       tt.merged,
				UpstreamGoneBranches: tt.upstreamGone,
			}
			runner := &GitRunner{Executor: mockGit, Log: NewNopLogger()}

			got, err := runner.IsBranchMerged(tt.branch, tt.target)

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}
