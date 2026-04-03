package twig

import (
	"context"
	"strings"
	"testing"

	"github.com/708u/twig/internal/testutil"
)

func TestOverlayCommand_Apply(t *testing.T) {
	t.Parallel()

	t.Run("BasicApply", func(t *testing.T) {
		t.Parallel()

		mockGit := &testutil.MockGitExecutor{
			Worktrees: []testutil.MockWorktree{
				{Path: "/repo/main", Branch: "main", HEAD: "main-commit-abc"},
				{Path: "/repo/feat-x", Branch: "feat/x", HEAD: "feat-x-commit-def"},
			},
			BranchHEADs: map[string]string{
				"feat/x": "feat-x-commit-def",
			},
			DiffNameOnlyOutput: map[string]string{
				"D:HEAD:feat-x-commit-def": "old-file.txt\n",
				"A:HEAD:feat-x-commit-def": "new-file.go\n",
				":HEAD:feat-x-commit-def":  "main.go\nold-file.txt\nnew-file.go\n",
			},
		}
		mockFS := &testutil.MockFS{
			WrittenFiles: make(map[string][]byte),
		}

		git := &GitRunner{Executor: mockGit, Dir: "/repo/main", Log: NewNopLogger()}
		cmd := NewOverlayCommand(mockFS, git, nil)

		result, err := cmd.Run(t.Context(), "feat/x", "/repo/main", OverlayOptions{
			Target: "main",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.SourceBranch != "feat/x" {
			t.Errorf("SourceBranch = %q, want %q", result.SourceBranch, "feat/x")
		}
		if result.TargetBranch != "main" {
			t.Errorf("TargetBranch = %q, want %q", result.TargetBranch, "main")
		}
		if result.ModifiedFiles != 3 {
			t.Errorf("ModifiedFiles = %d, want 3", result.ModifiedFiles)
		}
		if len(result.DeletedFiles) != 1 || result.DeletedFiles[0] != "old-file.txt" {
			t.Errorf("DeletedFiles = %v, want [old-file.txt]", result.DeletedFiles)
		}
		if len(result.AddedFiles) != 1 || result.AddedFiles[0] != "new-file.go" {
			t.Errorf("AddedFiles = %v, want [new-file.go]", result.AddedFiles)
		}

		// State file should be written
		statePath := "/repo/main/.git/twig-overlay"
		if _, ok := mockFS.WrittenFiles[statePath]; !ok {
			t.Error("state file was not written")
		}
	})

	t.Run("TargetHasChanges_Refuse", func(t *testing.T) {
		t.Parallel()

		mockGit := &testutil.MockGitExecutor{
			Worktrees: []testutil.MockWorktree{
				{Path: "/repo/main", Branch: "main", HEAD: "main-commit"},
			},
			HasChanges: true,
		}
		mockFS := &testutil.MockFS{}

		git := &GitRunner{Executor: mockGit, Dir: "/repo/main", Log: NewNopLogger()}
		cmd := NewOverlayCommand(mockFS, git, nil)

		_, err := cmd.Run(t.Context(), "feat/x", "/repo/main", OverlayOptions{
			Target: "main",
		})
		if err == nil {
			t.Fatal("expected error for dirty target")
		}
		if !strings.Contains(err.Error(), "uncommitted changes") {
			t.Errorf("error = %q, want to contain 'uncommitted changes'", err.Error())
		}
	})

	t.Run("TargetHasChanges_ForceProceeds", func(t *testing.T) {
		t.Parallel()

		mockGit := &testutil.MockGitExecutor{
			Worktrees: []testutil.MockWorktree{
				{Path: "/repo/main", Branch: "main", HEAD: "main-commit"},
				{Path: "/repo/feat-x", Branch: "feat/x", HEAD: "feat-x-commit"},
			},
			HasChanges: true,
			BranchHEADs: map[string]string{
				"feat/x": "feat-x-commit",
			},
			DiffNameOnlyOutput: map[string]string{
				"D:HEAD:feat-x-commit": "",
				"A:HEAD:feat-x-commit": "",
				":HEAD:feat-x-commit":  "file.go\n",
			},
		}
		mockFS := &testutil.MockFS{
			WrittenFiles: make(map[string][]byte),
		}

		git := &GitRunner{Executor: mockGit, Dir: "/repo/main", Log: NewNopLogger()}
		cmd := NewOverlayCommand(mockFS, git, nil)

		_, err := cmd.Run(t.Context(), "feat/x", "/repo/main", OverlayOptions{
			Target: "main",
			Force:  true,
		})
		if err != nil {
			t.Fatalf("unexpected error with --force: %v", err)
		}
	})

	t.Run("OverlayAlreadyActive_Refuse", func(t *testing.T) {
		t.Parallel()

		mockGit := &testutil.MockGitExecutor{
			Worktrees: []testutil.MockWorktree{
				{Path: "/repo/main", Branch: "main", HEAD: "main-commit"},
			},
		}
		mockFS := &testutil.MockFS{
			ExistingPaths: []string{"/repo/main/.git/twig-overlay"},
		}

		git := &GitRunner{Executor: mockGit, Dir: "/repo/main", Log: NewNopLogger()}
		cmd := NewOverlayCommand(mockFS, git, nil)

		_, err := cmd.Run(t.Context(), "feat/x", "/repo/main", OverlayOptions{
			Target: "main",
		})
		if err == nil {
			t.Fatal("expected error for active overlay")
		}
		if !strings.Contains(err.Error(), "overlay already active") {
			t.Errorf("error = %q, want to contain 'overlay already active'", err.Error())
		}
	})

	t.Run("SourceNotFound", func(t *testing.T) {
		t.Parallel()

		// The default mock returns "default-<branch>" for unknown branches
		// without error. We need a custom RunFunc for all commands.
		mockGit := &testutil.MockGitExecutor{}
		var worktreeListOutput = "worktree /repo/main\nHEAD main-commit\nbranch refs/heads/main\n\n"
		mockGit.RunFunc = func(_ context.Context, args ...string) ([]byte, error) {
			for len(args) >= 2 && args[0] == "-C" {
				args = args[2:]
			}
			switch {
			case len(args) >= 2 && args[0] == "rev-parse":
				for _, a := range args {
					if a == "--git-dir" {
						return []byte("/repo/main/.git\n"), nil
					}
				}
				if args[1] == "nonexistent" {
					return nil, &testutil.MockExitError{Code: 128}
				}
				if args[1] == "HEAD" {
					return []byte("main-commit\n"), nil
				}
				return []byte("some-commit\n"), nil
			case len(args) >= 2 && args[0] == "worktree" && args[1] == "list":
				return []byte(worktreeListOutput), nil
			case len(args) >= 2 && args[0] == "status":
				return []byte{}, nil
			}
			return nil, nil
		}

		mockFS := &testutil.MockFS{}
		git := &GitRunner{Executor: mockGit, Dir: "/repo/main", Log: NewNopLogger()}
		cmd := NewOverlayCommand(mockFS, git, nil)

		_, err := cmd.Run(t.Context(), "nonexistent", "/repo/main", OverlayOptions{
			Target: "main",
		})
		if err == nil {
			t.Fatal("expected error for nonexistent source")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("error = %q, want to contain 'not found'", err.Error())
		}
	})

	t.Run("SourceEqualsTarget_SameCommit", func(t *testing.T) {
		t.Parallel()

		mockGit := &testutil.MockGitExecutor{
			Worktrees: []testutil.MockWorktree{
				{Path: "/repo/main", Branch: "main", HEAD: "same-commit-abc"},
			},
			BranchHEADs: map[string]string{
				"feat/x": "same-commit-abc",
			},
		}
		mockFS := &testutil.MockFS{}

		git := &GitRunner{Executor: mockGit, Dir: "/repo/main", Log: NewNopLogger()}
		cmd := NewOverlayCommand(mockFS, git, nil)

		_, err := cmd.Run(t.Context(), "feat/x", "/repo/main", OverlayOptions{
			Target: "main",
		})
		if err == nil {
			t.Fatal("expected error for same commit")
		}
		if !strings.Contains(err.Error(), "same commit") {
			t.Errorf("error = %q, want to contain 'same commit'", err.Error())
		}
	})

	t.Run("CheckMode_NoStateFile", func(t *testing.T) {
		t.Parallel()

		mockGit := &testutil.MockGitExecutor{
			Worktrees: []testutil.MockWorktree{
				{Path: "/repo/main", Branch: "main", HEAD: "main-commit"},
				{Path: "/repo/feat-x", Branch: "feat/x", HEAD: "feat-x-commit"},
			},
			BranchHEADs: map[string]string{
				"feat/x": "feat-x-commit",
			},
			DiffNameOnlyOutput: map[string]string{
				"D:HEAD:feat-x-commit": "",
				"A:HEAD:feat-x-commit": "new.go\n",
				":HEAD:feat-x-commit":  "file.go\nnew.go\n",
			},
		}
		mockFS := &testutil.MockFS{
			WrittenFiles: make(map[string][]byte),
		}

		git := &GitRunner{Executor: mockGit, Dir: "/repo/main", Log: NewNopLogger()}
		cmd := NewOverlayCommand(mockFS, git, nil)

		result, err := cmd.Run(t.Context(), "feat/x", "/repo/main", OverlayOptions{
			Target: "main",
			Check:  true,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Check {
			t.Error("Check should be true")
		}
		// State file should NOT be written in check mode
		if _, ok := mockFS.WrittenFiles["/repo/main/.git/twig-overlay"]; ok {
			t.Error("state file should not be written in check mode")
		}
	})
}

func TestOverlayCommand_Restore(t *testing.T) {
	t.Parallel()

	t.Run("BasicRestore", func(t *testing.T) {
		t.Parallel()

		mockGit := &testutil.MockGitExecutor{
			Worktrees: []testutil.MockWorktree{
				{Path: "/repo/main", Branch: "main", HEAD: "main-commit"},
			},
		}
		mockFS := &testutil.MockFS{
			ReadFileResults: map[string][]byte{
				"/repo/main/.git/twig-overlay": []byte(`{
					"source_branch": "feat/x",
					"source_commit": "feat-x-commit",
					"target_branch": "main",
					"target_commit": "main-commit",
					"added_files": ["new-file.go"]
				}`),
			},
			WrittenFiles: make(map[string][]byte),
		}

		// Track removes
		var removedFiles []string
		mockFS.RemoveFunc = func(name string) error {
			removedFiles = append(removedFiles, name)
			return nil
		}

		git := &GitRunner{Executor: mockGit, Dir: "/repo/main", Log: NewNopLogger()}
		cmd := NewOverlayCommand(mockFS, git, nil)

		result, err := cmd.Run(t.Context(), "", "/repo/main", OverlayOptions{
			Restore: true,
			Target:  "main",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Restored {
			t.Error("Restored should be true")
		}
		if result.SourceBranch != "feat/x" {
			t.Errorf("SourceBranch = %q, want %q", result.SourceBranch, "feat/x")
		}

		// Check that added files were removed
		foundAddedFile := false
		foundStateFile := false
		for _, f := range removedFiles {
			if f == "/repo/main/new-file.go" {
				foundAddedFile = true
			}
			if f == "/repo/main/.git/twig-overlay" {
				foundStateFile = true
			}
		}
		if !foundAddedFile {
			t.Error("added file should have been removed")
		}
		if !foundStateFile {
			t.Error("state file should have been removed")
		}
	})

	t.Run("NoOverlayActive", func(t *testing.T) {
		t.Parallel()

		mockGit := &testutil.MockGitExecutor{
			Worktrees: []testutil.MockWorktree{
				{Path: "/repo/main", Branch: "main", HEAD: "main-commit"},
			},
		}
		mockFS := &testutil.MockFS{}

		git := &GitRunner{Executor: mockGit, Dir: "/repo/main", Log: NewNopLogger()}
		cmd := NewOverlayCommand(mockFS, git, nil)

		_, err := cmd.Run(t.Context(), "", "/repo/main", OverlayOptions{
			Restore: true,
			Target:  "main",
		})
		if err == nil {
			t.Fatal("expected error for no active overlay")
		}
		if !strings.Contains(err.Error(), "no overlay active") {
			t.Errorf("error = %q, want to contain 'no overlay active'", err.Error())
		}
	})

	t.Run("HEADMoved_Refuse", func(t *testing.T) {
		t.Parallel()

		mockGit := &testutil.MockGitExecutor{
			Worktrees: []testutil.MockWorktree{
				{Path: "/repo/main", Branch: "main", HEAD: "new-head-commit"},
			},
		}
		mockFS := &testutil.MockFS{
			ReadFileResults: map[string][]byte{
				"/repo/main/.git/twig-overlay": []byte(`{
					"source_branch": "feat/x",
					"source_commit": "feat-x-commit",
					"target_branch": "main",
					"target_commit": "original-head-commit"
				}`),
			},
		}

		git := &GitRunner{Executor: mockGit, Dir: "/repo/main", Log: NewNopLogger()}
		cmd := NewOverlayCommand(mockFS, git, nil)

		_, err := cmd.Run(t.Context(), "", "/repo/main", OverlayOptions{
			Restore: true,
			Target:  "main",
		})
		if err == nil {
			t.Fatal("expected error for HEAD movement")
		}
		if !strings.Contains(err.Error(), "HEAD has moved") {
			t.Errorf("error = %q, want to contain 'HEAD has moved'", err.Error())
		}
	})

	t.Run("HEADMoved_ForceProceeds", func(t *testing.T) {
		t.Parallel()

		mockGit := &testutil.MockGitExecutor{
			Worktrees: []testutil.MockWorktree{
				{Path: "/repo/main", Branch: "main", HEAD: "new-head-commit"},
			},
		}
		mockFS := &testutil.MockFS{
			ReadFileResults: map[string][]byte{
				"/repo/main/.git/twig-overlay": []byte(`{
					"source_branch": "feat/x",
					"source_commit": "feat-x-commit",
					"target_branch": "main",
					"target_commit": "original-head-commit",
					"added_files": []
				}`),
			},
		}
		mockFS.RemoveFunc = func(name string) error { return nil }

		git := &GitRunner{Executor: mockGit, Dir: "/repo/main", Log: NewNopLogger()}
		cmd := NewOverlayCommand(mockFS, git, nil)

		_, err := cmd.Run(t.Context(), "", "/repo/main", OverlayOptions{
			Restore: true,
			Target:  "main",
			Force:   true,
		})
		if err != nil {
			t.Fatalf("unexpected error with --force: %v", err)
		}
	})

	t.Run("CheckMode_NoStateFileDeleted", func(t *testing.T) {
		t.Parallel()

		mockGit := &testutil.MockGitExecutor{
			Worktrees: []testutil.MockWorktree{
				{Path: "/repo/main", Branch: "main", HEAD: "main-commit"},
			},
		}
		mockFS := &testutil.MockFS{
			ReadFileResults: map[string][]byte{
				"/repo/main/.git/twig-overlay": []byte(`{
					"source_branch": "feat/x",
					"source_commit": "feat-x-commit",
					"target_branch": "main",
					"target_commit": "main-commit",
					"added_files": ["new.go"]
				}`),
			},
		}
		var removeCalled bool
		mockFS.RemoveFunc = func(name string) error {
			removeCalled = true
			return nil
		}

		git := &GitRunner{Executor: mockGit, Dir: "/repo/main", Log: NewNopLogger()}
		cmd := NewOverlayCommand(mockFS, git, nil)

		result, err := cmd.Run(t.Context(), "", "/repo/main", OverlayOptions{
			Restore: true,
			Target:  "main",
			Check:   true,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Restored || !result.Check {
			t.Error("expected Restored=true, Check=true")
		}
		if removeCalled {
			t.Error("Remove should not be called in check mode")
		}
	})
}

func TestOverlayResult_Format(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		result     OverlayResult
		opts       OverlayFormatOptions
		wantStdout string
		wantStderr string
	}{
		{
			name: "apply_default",
			result: OverlayResult{
				TargetBranch:  "main",
				SourceBranch:  "feat/x",
				ModifiedFiles: 42,
				DeletedFiles:  []string{"a.txt", "b.txt"},
				AddedFiles:    []string{"c.go"},
			},
			opts:       OverlayFormatOptions{},
			wantStdout: "Overlaid main with feat/x (42 files changed, 2 deleted, 1 added)\n",
			wantStderr: "warning: do not commit in the overlaid worktree.\n         Use 'twig overlay --restore' when done.\n",
		},
		{
			name: "apply_no_deletes_no_adds",
			result: OverlayResult{
				TargetBranch:  "main",
				SourceBranch:  "feat/x",
				ModifiedFiles: 5,
			},
			opts:       OverlayFormatOptions{},
			wantStdout: "Overlaid main with feat/x (5 files changed)\n",
			wantStderr: "warning: do not commit in the overlaid worktree.\n         Use 'twig overlay --restore' when done.\n",
		},
		{
			name: "restore_default",
			result: OverlayResult{
				Restored:     true,
				TargetBranch: "main",
				SourceBranch: "feat/x",
			},
			opts:       OverlayFormatOptions{},
			wantStdout: "Restored main (removed overlay from feat/x)\n",
			wantStderr: "",
		},
		{
			name: "check_apply",
			result: OverlayResult{
				Check:         true,
				TargetBranch:  "main",
				SourceBranch:  "feat/x",
				ModifiedFiles: 10,
				DeletedFiles:  []string{"old.txt"},
				AddedFiles:    []string{"new.go"},
			},
			opts: OverlayFormatOptions{},
			wantStdout: "Would overlay main with feat/x:\n" +
				"  10 file(s) would change\n" +
				"  1 file(s) would be deleted\n" +
				"  1 file(s) would be added\n",
		},
		{
			name: "check_restore",
			result: OverlayResult{
				Check:        true,
				Restored:     true,
				TargetBranch: "main",
				SourceBranch: "feat/x",
				AddedFiles:   []string{"new.go"},
			},
			opts: OverlayFormatOptions{},
			wantStdout: "Would restore main (remove overlay from feat/x)\n" +
				"  1 overlay-added file(s) would be removed\n",
		},
		{
			name: "apply_with_dirty",
			result: OverlayResult{
				TargetBranch:  "main",
				SourceBranch:  "feat/x",
				ModifiedFiles: 10,
				DirtyFiles:    3,
			},
			opts:       OverlayFormatOptions{},
			wantStdout: "Overlaid main with feat/x (10 files changed, 3 dirty)\n",
			wantStderr: "warning: do not commit in the overlaid worktree.\n         Use 'twig overlay --restore' when done.\n",
		},
		{
			name: "check_apply_with_dirty",
			result: OverlayResult{
				Check:         true,
				TargetBranch:  "main",
				SourceBranch:  "feat/x",
				ModifiedFiles: 5,
				DirtyFiles:    2,
			},
			opts: OverlayFormatOptions{},
			wantStdout: "Would overlay main with feat/x:\n" +
				"  5 file(s) would change\n" +
				"  2 dirty file(s) would be applied\n",
		},
		{
			name: "quiet_suppresses_output",
			result: OverlayResult{
				TargetBranch:  "main",
				SourceBranch:  "feat/x",
				ModifiedFiles: 5,
			},
			opts:       OverlayFormatOptions{Quiet: true},
			wantStdout: "",
			wantStderr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			formatted := tt.result.Format(tt.opts)
			if formatted.Stdout != tt.wantStdout {
				t.Errorf("Stdout:\ngot:  %q\nwant: %q", formatted.Stdout, tt.wantStdout)
			}
			if formatted.Stderr != tt.wantStderr {
				t.Errorf("Stderr:\ngot:  %q\nwant: %q", formatted.Stderr, tt.wantStderr)
			}
		})
	}
}

func TestOverlayCommand_DirtyApply(t *testing.T) {
	t.Parallel()

	t.Run("Basic", func(t *testing.T) {
		t.Parallel()

		mockGit := &testutil.MockGitExecutor{
			Worktrees: []testutil.MockWorktree{
				{Path: "/repo/main", Branch: "main", HEAD: "main-commit"},
				{Path: "/repo/feat-x", Branch: "feat/x", HEAD: "feat-x-commit"},
			},
			BranchHEADs: map[string]string{
				"feat/x": "feat-x-commit",
			},
			DiffNameOnlyOutput: map[string]string{
				"D:HEAD:feat-x-commit": "",
				"A:HEAD:feat-x-commit": "new.go\n",
				":HEAD:feat-x-commit":  "file.go\nnew.go\n",
			},
			// Dirty files in source worktree
			StatusOutputMap: map[string]string{
				"/repo/feat-x": " M file.go\n?? untracked.txt\n",
			},
		}
		mockFS := &testutil.MockFS{
			WrittenFiles: make(map[string][]byte),
			ExistingPaths: []string{
				"/repo/feat-x/file.go",
				"/repo/feat-x/untracked.txt",
			},
			ReadFileResults: map[string][]byte{
				"/repo/feat-x/file.go":       []byte("dirty content"),
				"/repo/feat-x/untracked.txt": []byte("new file"),
			},
		}

		git := &GitRunner{Executor: mockGit, Dir: "/repo/main", Log: NewNopLogger()}
		cmd := NewOverlayCommand(mockFS, git, nil)

		result, err := cmd.Run(t.Context(), "feat/x", "/repo/main", OverlayOptions{
			Target: "main",
			Dirty:  true,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.DirtyFiles != 2 {
			t.Errorf("DirtyFiles = %d, want 2", result.DirtyFiles)
		}
		// untracked.txt should be in AddedFiles (dirty untracked)
		found := false
		for _, f := range result.AddedFiles {
			if f == "untracked.txt" {
				found = true
			}
		}
		if !found {
			t.Errorf("AddedFiles = %v, want to contain untracked.txt", result.AddedFiles)
		}
		// Dirty files should be written to target
		if _, ok := mockFS.WrittenFiles["/repo/main/file.go"]; !ok {
			t.Error("dirty file.go was not written to target")
		}
		if _, ok := mockFS.WrittenFiles["/repo/main/untracked.txt"]; !ok {
			t.Error("dirty untracked.txt was not written to target")
		}
	})

	t.Run("SameCommit_WithDirtyFiles", func(t *testing.T) {
		t.Parallel()

		mockGit := &testutil.MockGitExecutor{
			Worktrees: []testutil.MockWorktree{
				{Path: "/repo/main", Branch: "main", HEAD: "same-commit"},
				{Path: "/repo/feat-x", Branch: "feat/x", HEAD: "same-commit"},
			},
			BranchHEADs: map[string]string{
				"feat/x": "same-commit",
			},
			StatusOutputMap: map[string]string{
				"/repo/feat-x": " M dirty.go\n",
			},
		}
		mockFS := &testutil.MockFS{
			WrittenFiles: make(map[string][]byte),
			ExistingPaths: []string{
				"/repo/feat-x/dirty.go",
			},
			ReadFileResults: map[string][]byte{
				"/repo/feat-x/dirty.go": []byte("dirty"),
			},
		}

		git := &GitRunner{Executor: mockGit, Dir: "/repo/main", Log: NewNopLogger()}
		cmd := NewOverlayCommand(mockFS, git, nil)

		result, err := cmd.Run(t.Context(), "feat/x", "/repo/main", OverlayOptions{
			Target: "main",
			Dirty:  true,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.DirtyFiles != 1 {
			t.Errorf("DirtyFiles = %d, want 1", result.DirtyFiles)
		}
		// Committed diff should be 0 (same commit)
		if result.ModifiedFiles != 0 {
			t.Errorf("ModifiedFiles = %d, want 0", result.ModifiedFiles)
		}
	})

	t.Run("SameCommit_NoDirtyFiles", func(t *testing.T) {
		t.Parallel()

		mockGit := &testutil.MockGitExecutor{
			Worktrees: []testutil.MockWorktree{
				{Path: "/repo/main", Branch: "main", HEAD: "same-commit"},
				{Path: "/repo/feat-x", Branch: "feat/x", HEAD: "same-commit"},
			},
			BranchHEADs: map[string]string{
				"feat/x": "same-commit",
			},
			StatusOutputMap: map[string]string{
				"/repo/feat-x": "",
			},
		}
		mockFS := &testutil.MockFS{}

		git := &GitRunner{Executor: mockGit, Dir: "/repo/main", Log: NewNopLogger()}
		cmd := NewOverlayCommand(mockFS, git, nil)

		_, err := cmd.Run(t.Context(), "feat/x", "/repo/main", OverlayOptions{
			Target: "main",
			Dirty:  true,
		})
		if err == nil {
			t.Fatal("expected error for same commit with no dirty files")
		}
		if !strings.Contains(err.Error(), "same commit") || !strings.Contains(err.Error(), "no uncommitted") {
			t.Errorf("error = %q, want to contain 'same commit' and 'no uncommitted'", err.Error())
		}
	})

	t.Run("CheckMode", func(t *testing.T) {
		t.Parallel()

		mockGit := &testutil.MockGitExecutor{
			Worktrees: []testutil.MockWorktree{
				{Path: "/repo/main", Branch: "main", HEAD: "main-commit"},
				{Path: "/repo/feat-x", Branch: "feat/x", HEAD: "feat-x-commit"},
			},
			BranchHEADs: map[string]string{
				"feat/x": "feat-x-commit",
			},
			DiffNameOnlyOutput: map[string]string{
				"D:HEAD:feat-x-commit": "",
				"A:HEAD:feat-x-commit": "",
				":HEAD:feat-x-commit":  "file.go\n",
			},
			StatusOutputMap: map[string]string{
				"/repo/feat-x": " M file.go\n?? new.txt\n",
			},
		}
		mockFS := &testutil.MockFS{
			WrittenFiles: make(map[string][]byte),
		}

		git := &GitRunner{Executor: mockGit, Dir: "/repo/main", Log: NewNopLogger()}
		cmd := NewOverlayCommand(mockFS, git, nil)

		result, err := cmd.Run(t.Context(), "feat/x", "/repo/main", OverlayOptions{
			Target: "main",
			Dirty:  true,
			Check:  true,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.DirtyFiles != 2 {
			t.Errorf("DirtyFiles = %d, want 2", result.DirtyFiles)
		}
		// No files should be written in check mode
		// (state file and dirty files should not be written)
		for path := range mockFS.WrittenFiles {
			if !strings.Contains(path, "twig-overlay") {
				continue
			}
			t.Errorf("state file should not be written in check mode: %s", path)
		}
	})
}

func TestMergeUnique(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		a    []string
		b    []string
		want []string
	}{
		{"both_empty", nil, nil, nil},
		{"b_empty", []string{"a", "b"}, nil, []string{"a", "b"}},
		{"a_empty", nil, []string{"x"}, []string{"x"}},
		{"no_overlap", []string{"a"}, []string{"b"}, []string{"a", "b"}},
		{"with_overlap", []string{"a", "b"}, []string{"b", "c"}, []string{"a", "b", "c"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := mergeUnique(tt.a, tt.b)
			if len(got) != len(tt.want) {
				t.Errorf("mergeUnique(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.want)
				return
			}
			for i, v := range got {
				if v != tt.want[i] {
					t.Errorf("mergeUnique(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.want)
					return
				}
			}
		})
	}
}
