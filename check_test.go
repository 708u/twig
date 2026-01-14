package twig

import (
	"strings"
	"testing"

	"github.com/708u/twig/internal/testutil"
)

func TestCheckResult_Counts(t *testing.T) {
	t.Parallel()

	result := CheckResult{
		Items: []CheckItem{
			{Severity: SeverityError, Message: "error 1"},
			{Severity: SeverityError, Message: "error 2"},
			{Severity: SeverityWarn, Message: "warning 1"},
			{Severity: SeverityInfo, Message: "info 1"},
			{Severity: SeverityOK, Message: "ok 1"},
		},
	}

	if got := result.ErrorCount(); got != 2 {
		t.Errorf("ErrorCount() = %d, want 2", got)
	}
	if got := result.WarningCount(); got != 1 {
		t.Errorf("WarningCount() = %d, want 1", got)
	}
	if got := result.InfoCount(); got != 1 {
		t.Errorf("InfoCount() = %d, want 1", got)
	}
}

func TestCheckResult_Format(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		result         CheckResult
		opts           CheckFormatOptions
		wantContains   []string
		wantNotContain []string
	}{
		{
			name: "default_hides_ok_items",
			result: CheckResult{
				Items: []CheckItem{
					{Category: CategoryConfig, Severity: SeverityOK, Message: "ok item"},
					{Category: CategoryConfig, Severity: SeverityWarn, Message: "warning item"},
				},
			},
			opts:           CheckFormatOptions{},
			wantContains:   []string{"[warn] warning item", "0 errors, 1 warnings, 0 info"},
			wantNotContain: []string{"[ok] ok item"},
		},
		{
			name: "verbose_shows_ok_items",
			result: CheckResult{
				Items: []CheckItem{
					{Category: CategoryConfig, Severity: SeverityOK, Message: "ok item"},
					{Category: CategoryConfig, Severity: SeverityWarn, Message: "warning item"},
				},
			},
			opts:         CheckFormatOptions{Verbose: true},
			wantContains: []string{"[ok] ok item", "[warn] warning item"},
		},
		{
			name: "quiet_only_shows_errors",
			result: CheckResult{
				Items: []CheckItem{
					{Category: CategoryConfig, Severity: SeverityError, Message: "error item"},
					{Category: CategoryConfig, Severity: SeverityWarn, Message: "warning item"},
					{Category: CategoryConfig, Severity: SeverityInfo, Message: "info item"},
				},
			},
			opts:           CheckFormatOptions{Quiet: true},
			wantContains:   []string{"[error] error item"},
			wantNotContain: []string{"[warn]", "[info]", "Summary"},
		},
		{
			name: "shows_suggestions",
			result: CheckResult{
				Items: []CheckItem{
					{
						Category:   CategoryConfig,
						Severity:   SeverityWarn,
						Message:    "dir does not exist",
						Suggestion: "run mkdir -p",
					},
				},
			},
			opts:         CheckFormatOptions{},
			wantContains: []string{"suggestion: run mkdir -p"},
		},
		{
			name: "groups_by_category",
			result: CheckResult{
				Items: []CheckItem{
					{Category: CategoryConfig, Severity: SeverityWarn, Message: "config warning"},
					{Category: CategorySymlinks, Severity: SeverityWarn, Message: "symlink warning"},
				},
			},
			opts:         CheckFormatOptions{},
			wantContains: []string{"config:", "symlinks:"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := tt.result.Format(tt.opts)

			for _, want := range tt.wantContains {
				if !strings.Contains(got.Stdout, want) {
					t.Errorf("Stdout should contain %q, got:\n%s", want, got.Stdout)
				}
			}

			for _, notWant := range tt.wantNotContain {
				if strings.Contains(got.Stdout, notWant) {
					t.Errorf("Stdout should not contain %q, got:\n%s", notWant, got.Stdout)
				}
			}
		})
	}
}

func TestCheckCommand_Run(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		config      *Config
		setupFS     func() *testutil.MockFS
		setupGit    func() *testutil.MockGitExecutor
		wantErrors  int
		wantWarns   int
		wantInfos   int
		checkStdout func(t *testing.T, result CheckResult)
	}{
		{
			name: "valid_config_and_symlinks",
			config: &Config{
				WorktreeSourceDir:   "/repo",
				WorktreeDestBaseDir: "/worktrees",
				Symlinks:            []string{".envrc"},
			},
			setupFS: func() *testutil.MockFS {
				return &testutil.MockFS{
					ExistingPaths: []string{"/worktrees"},
					WritableDirs:  []string{"/worktrees"},
					GlobResults: map[string][]string{
						".envrc": {".envrc"},
					},
				}
			},
			setupGit: func() *testutil.MockGitExecutor {
				return &testutil.MockGitExecutor{}
			},
			wantErrors: 0,
			wantWarns:  0,
			wantInfos:  0,
		},
		{
			name: "worktree_dest_dir_not_exists",
			config: &Config{
				WorktreeSourceDir:   "/repo",
				WorktreeDestBaseDir: "/missing-dir",
				Symlinks:            []string{},
			},
			setupFS: func() *testutil.MockFS {
				return &testutil.MockFS{
					ExistingPaths: []string{}, // dir does not exist
				}
			},
			setupGit: func() *testutil.MockGitExecutor {
				return &testutil.MockGitExecutor{}
			},
			wantErrors: 0,
			wantWarns:  1,
			checkStdout: func(t *testing.T, result CheckResult) {
				t.Helper()
				found := false
				for _, item := range result.Items {
					if item.Severity == SeverityWarn &&
						strings.Contains(item.Message, "does not exist") {
						found = true
						if item.Suggestion == "" {
							t.Error("should have suggestion for missing dir")
						}
					}
				}
				if !found {
					t.Error("should have warning about missing dir")
				}
			},
		},
		{
			name: "symlink_pattern_no_match",
			config: &Config{
				WorktreeSourceDir:   "/repo",
				WorktreeDestBaseDir: "/worktrees",
				Symlinks:            []string{".envrc", "missing/**"},
			},
			setupFS: func() *testutil.MockFS {
				return &testutil.MockFS{
					ExistingPaths: []string{"/worktrees"},
					WritableDirs:  []string{"/worktrees"},
					GlobResults: map[string][]string{
						".envrc":     {".envrc"},
						"missing/**": {}, // no matches
					},
				}
			},
			setupGit: func() *testutil.MockGitExecutor {
				return &testutil.MockGitExecutor{}
			},
			wantErrors: 0,
			wantWarns:  1,
			checkStdout: func(t *testing.T, result CheckResult) {
				t.Helper()
				found := false
				for _, item := range result.Items {
					if item.Severity == SeverityWarn &&
						strings.Contains(item.Message, "missing/**") &&
						strings.Contains(item.Message, "matches no files") {
						found = true
					}
				}
				if !found {
					t.Error("should have warning about no matching files")
				}
			},
		},
		{
			name: "gitignored_symlink_target",
			config: &Config{
				WorktreeSourceDir:   "/repo",
				WorktreeDestBaseDir: "/worktrees",
				Symlinks:            []string{".claude"},
			},
			setupFS: func() *testutil.MockFS {
				return &testutil.MockFS{
					ExistingPaths: []string{"/worktrees"},
					WritableDirs:  []string{"/worktrees"},
					GlobResults: map[string][]string{
						".claude": {".claude"},
					},
				}
			},
			setupGit: func() *testutil.MockGitExecutor {
				return &testutil.MockGitExecutor{
					IgnoredPaths: []string{".claude"},
				}
			},
			wantErrors: 0,
			wantWarns:  0,
			wantInfos:  1,
			checkStdout: func(t *testing.T, result CheckResult) {
				t.Helper()
				found := false
				for _, item := range result.Items {
					if item.Severity == SeverityInfo &&
						strings.Contains(item.Message, "gitignored") {
						found = true
					}
				}
				if !found {
					t.Error("should have info about gitignored file")
				}
			},
		},
		{
			name: "invalid_glob_pattern",
			config: &Config{
				WorktreeSourceDir:   "/repo",
				WorktreeDestBaseDir: "/worktrees",
				Symlinks:            []string{"[invalid"},
			},
			setupFS: func() *testutil.MockFS {
				return &testutil.MockFS{
					ExistingPaths: []string{"/worktrees"},
					WritableDirs:  []string{"/worktrees"},
					GlobErr:       &testutil.GlobPatternError{Pattern: "[invalid"},
				}
			},
			setupGit: func() *testutil.MockGitExecutor {
				return &testutil.MockGitExecutor{}
			},
			wantErrors: 1,
			checkStdout: func(t *testing.T, result CheckResult) {
				t.Helper()
				found := false
				for _, item := range result.Items {
					if item.Severity == SeverityError &&
						strings.Contains(item.Message, "invalid glob pattern") {
						found = true
					}
				}
				if !found {
					t.Error("should have error about invalid glob pattern")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mockFS := tt.setupFS()
			mockGit := tt.setupGit()

			cmd := &CheckCommand{
				FS:     mockFS,
				Git:    &GitRunner{Executor: mockGit, Dir: tt.config.WorktreeSourceDir},
				Config: tt.config,
			}

			result, err := cmd.Run()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got := result.ErrorCount(); got != tt.wantErrors {
				t.Errorf("ErrorCount() = %d, want %d", got, tt.wantErrors)
			}
			if got := result.WarningCount(); got != tt.wantWarns {
				t.Errorf("WarningCount() = %d, want %d", got, tt.wantWarns)
			}
			if tt.wantInfos > 0 {
				if got := result.InfoCount(); got != tt.wantInfos {
					t.Errorf("InfoCount() = %d, want %d", got, tt.wantInfos)
				}
			}

			if tt.checkStdout != nil {
				tt.checkStdout(t, result)
			}
		})
	}
}
