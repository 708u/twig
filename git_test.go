package twig

import (
	"reflect"
	"testing"

	"github.com/708u/twig/internal/testutil"
)

func TestNewGitRunner_DefaultLogger(t *testing.T) {
	t.Parallel()

	// Should use nop logger by default
	runner := NewGitRunner("/tmp")
	if runner.Log == nil {
		t.Error("Log should not be nil after NewGitRunner")
	}
}

func TestGitRunner_ChangedFiles(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		statusOutput string
		want         []FileStatus
	}{
		{
			name:         "no changes",
			statusOutput: "",
			want:         make([]FileStatus, 0),
		},
		{
			name:         "modified file",
			statusOutput: " M src/main.go\n",
			want: []FileStatus{
				{Status: " M", Path: "src/main.go"},
			},
		},
		{
			name:         "staged new file",
			statusOutput: "A  src/new.go\n",
			want: []FileStatus{
				{Status: "A ", Path: "src/new.go"},
			},
		},
		{
			name:         "untracked file",
			statusOutput: "?? tmp/debug.log\n",
			want: []FileStatus{
				{Status: "??", Path: "tmp/debug.log"},
			},
		},
		{
			name:         "multiple files",
			statusOutput: " M src/main.go\nA  src/new.go\n?? tmp/debug.log\n",
			want: []FileStatus{
				{Status: " M", Path: "src/main.go"},
				{Status: "A ", Path: "src/new.go"},
				{Status: "??", Path: "tmp/debug.log"},
			},
		},
		{
			name:         "renamed file",
			statusOutput: "R  old.go -> new.go\n",
			want: []FileStatus{
				{Status: "R ", Path: "new.go"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mockGit := &testutil.MockGitExecutor{
				StatusOutput: tt.statusOutput,
			}
			runner := &GitRunner{Executor: mockGit, Log: NewNopLogger()}

			got, err := runner.ChangedFiles(t.Context())
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

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

			got, err := runner.IsBranchUpstreamGone(t.Context(), tt.branch)

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
		branchHEADs  map[string]string
		merged       map[string][]string
		upstreamGone []string
		want         bool
	}{
		{
			name:   "traditional merge detected",
			branch: "feat/merged",
			target: "main",
			branchHEADs: map[string]string{
				"feat/merged": "commit123",
				"main":        "commit456",
			},
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
			branchHEADs: map[string]string{
				"feat/squashed": "commit123",
				"main":          "commit456",
			},
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
			branchHEADs: map[string]string{
				"feat/new": "commit123",
				"main":     "commit456",
			},
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
			branchHEADs: map[string]string{
				"feat/both": "commit123",
				"main":      "commit456",
			},
			merged: map[string][]string{
				"main": {"feat/both"},
			},
			upstreamGone: []string{"feat/both"},
			want:         true,
		},
		{
			name:   "same commit is not considered merged",
			branch: "feat/new-branch",
			target: "main",
			branchHEADs: map[string]string{
				"feat/new-branch": "same-commit-abc123",
				"main":            "same-commit-abc123",
			},
			merged: map[string][]string{
				"main": {"feat/new-branch"}, // git branch --merged would return this
			},
			upstreamGone: []string{},
			want:         false, // but we should return false because same commit
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mockGit := &testutil.MockGitExecutor{
				MergedBranches:       tt.merged,
				UpstreamGoneBranches: tt.upstreamGone,
				BranchHEADs:          tt.branchHEADs,
			}
			runner := &GitRunner{Executor: mockGit, Log: NewNopLogger()}

			got, err := runner.IsBranchMerged(t.Context(), tt.branch, tt.target)

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}
