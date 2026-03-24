package twig

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoadConfig_SymlinksOverride(t *testing.T) {
	t.Parallel()

	t.Run("LocalOverridesProject", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		twigDir := filepath.Join(tmpDir, configDir)
		if err := os.MkdirAll(twigDir, 0755); err != nil {
			t.Fatal(err)
		}

		// Project config with symlinks
		projectSettings := `symlinks = [".envrc", ".config"]
`
		if err := os.WriteFile(filepath.Join(twigDir, configFileName), []byte(projectSettings), 0644); err != nil {
			t.Fatal(err)
		}

		// Local config overrides symlinks
		localSettings := `symlinks = [".tool-versions"]
`
		if err := os.WriteFile(filepath.Join(twigDir, localConfigFileName), []byte(localSettings), 0644); err != nil {
			t.Fatal(err)
		}

		result, err := LoadConfig(tmpDir)
		if err != nil {
			t.Fatal(err)
		}

		// Local should override project (only .tool-versions)
		expected := []string{".tool-versions"}
		if !reflect.DeepEqual(result.Config.Symlinks, expected) {
			t.Errorf("Symlinks = %v, want %v", result.Config.Symlinks, expected)
		}
	})

	t.Run("ProjectUsedWhenNoLocal", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		twigDir := filepath.Join(tmpDir, configDir)
		if err := os.MkdirAll(twigDir, 0755); err != nil {
			t.Fatal(err)
		}

		// Project config with symlinks
		projectSettings := `symlinks = [".envrc", ".config"]
`
		if err := os.WriteFile(filepath.Join(twigDir, configFileName), []byte(projectSettings), 0644); err != nil {
			t.Fatal(err)
		}

		result, err := LoadConfig(tmpDir)
		if err != nil {
			t.Fatal(err)
		}

		// Project symlinks should be used
		expected := []string{".envrc", ".config"}
		if !reflect.DeepEqual(result.Config.Symlinks, expected) {
			t.Errorf("Symlinks = %v, want %v", result.Config.Symlinks, expected)
		}
	})

	t.Run("EmptyLocalDoesNotOverride", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		twigDir := filepath.Join(tmpDir, configDir)
		if err := os.MkdirAll(twigDir, 0755); err != nil {
			t.Fatal(err)
		}

		// Project config with symlinks
		projectSettings := `symlinks = [".envrc"]
`
		if err := os.WriteFile(filepath.Join(twigDir, configFileName), []byte(projectSettings), 0644); err != nil {
			t.Fatal(err)
		}

		// Local config with empty symlinks array
		localSettings := `symlinks = []
`
		if err := os.WriteFile(filepath.Join(twigDir, localConfigFileName), []byte(localSettings), 0644); err != nil {
			t.Fatal(err)
		}

		result, err := LoadConfig(tmpDir)
		if err != nil {
			t.Fatal(err)
		}

		// Empty local should not override project
		expected := []string{".envrc"}
		if !reflect.DeepEqual(result.Config.Symlinks, expected) {
			t.Errorf("Symlinks = %v, want %v", result.Config.Symlinks, expected)
		}
	})
}

func TestLoadConfig_ExtraSymlinks(t *testing.T) {
	t.Parallel()

	t.Run("ExtraSymlinksAppended", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		twigDir := filepath.Join(tmpDir, configDir)
		if err := os.MkdirAll(twigDir, 0755); err != nil {
			t.Fatal(err)
		}

		// Project config with symlinks and extra_symlinks
		projectSettings := `symlinks = [".envrc"]
extra_symlinks = [".tool-versions"]
`
		if err := os.WriteFile(filepath.Join(twigDir, configFileName), []byte(projectSettings), 0644); err != nil {
			t.Fatal(err)
		}

		result, err := LoadConfig(tmpDir)
		if err != nil {
			t.Fatal(err)
		}

		// symlinks should include both
		expected := []string{".envrc", ".tool-versions"}
		if !reflect.DeepEqual(result.Config.Symlinks, expected) {
			t.Errorf("Symlinks = %v, want %v", result.Config.Symlinks, expected)
		}

		// ExtraSymlinks should only have the extra ones
		expectedExtra := []string{".tool-versions"}
		if !reflect.DeepEqual(result.Config.ExtraSymlinks, expectedExtra) {
			t.Errorf("ExtraSymlinks = %v, want %v", result.Config.ExtraSymlinks, expectedExtra)
		}
	})

	t.Run("ExtraSymlinksDeduplicated", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		twigDir := filepath.Join(tmpDir, configDir)
		if err := os.MkdirAll(twigDir, 0755); err != nil {
			t.Fatal(err)
		}

		// Project config with overlapping symlinks and extra_symlinks
		projectSettings := `symlinks = [".envrc", ".tool-versions"]
extra_symlinks = [".tool-versions", ".config"]
`
		if err := os.WriteFile(filepath.Join(twigDir, configFileName), []byte(projectSettings), 0644); err != nil {
			t.Fatal(err)
		}

		result, err := LoadConfig(tmpDir)
		if err != nil {
			t.Fatal(err)
		}

		// Duplicates should be removed
		expected := []string{".envrc", ".tool-versions", ".config"}
		if !reflect.DeepEqual(result.Config.Symlinks, expected) {
			t.Errorf("Symlinks = %v, want %v", result.Config.Symlinks, expected)
		}

		// ExtraSymlinks should only have non-duplicate entries
		expectedExtra := []string{".config"}
		if !reflect.DeepEqual(result.Config.ExtraSymlinks, expectedExtra) {
			t.Errorf("ExtraSymlinks = %v, want %v", result.Config.ExtraSymlinks, expectedExtra)
		}
	})

	t.Run("ExtraSymlinksFromBothConfigs", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		twigDir := filepath.Join(tmpDir, configDir)
		if err := os.MkdirAll(twigDir, 0755); err != nil {
			t.Fatal(err)
		}

		// Project config with extra_symlinks
		projectSettings := `symlinks = [".envrc"]
extra_symlinks = [".project-extra"]
`
		if err := os.WriteFile(filepath.Join(twigDir, configFileName), []byte(projectSettings), 0644); err != nil {
			t.Fatal(err)
		}

		// Local config with extra_symlinks
		localSettings := `extra_symlinks = [".local-extra"]
`
		if err := os.WriteFile(filepath.Join(twigDir, localConfigFileName), []byte(localSettings), 0644); err != nil {
			t.Fatal(err)
		}

		result, err := LoadConfig(tmpDir)
		if err != nil {
			t.Fatal(err)
		}

		// All extra_symlinks should be collected and appended
		expected := []string{".envrc", ".project-extra", ".local-extra"}
		if !reflect.DeepEqual(result.Config.Symlinks, expected) {
			t.Errorf("Symlinks = %v, want %v", result.Config.Symlinks, expected)
		}

		// ExtraSymlinks should have both
		expectedExtra := []string{".project-extra", ".local-extra"}
		if !reflect.DeepEqual(result.Config.ExtraSymlinks, expectedExtra) {
			t.Errorf("ExtraSymlinks = %v, want %v", result.Config.ExtraSymlinks, expectedExtra)
		}
	})

	t.Run("LocalSymlinksOverrideWithExtraSymlinks", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		twigDir := filepath.Join(tmpDir, configDir)
		if err := os.MkdirAll(twigDir, 0755); err != nil {
			t.Fatal(err)
		}

		// Project config
		projectSettings := `symlinks = [".envrc", ".config"]
extra_symlinks = [".project-extra"]
`
		if err := os.WriteFile(filepath.Join(twigDir, configFileName), []byte(projectSettings), 0644); err != nil {
			t.Fatal(err)
		}

		// Local config overrides symlinks and adds extra_symlinks
		localSettings := `symlinks = [".local-only"]
extra_symlinks = [".local-extra"]
`
		if err := os.WriteFile(filepath.Join(twigDir, localConfigFileName), []byte(localSettings), 0644); err != nil {
			t.Fatal(err)
		}

		result, err := LoadConfig(tmpDir)
		if err != nil {
			t.Fatal(err)
		}

		// Local symlinks override project, but extra_symlinks from both are collected
		expected := []string{".local-only", ".project-extra", ".local-extra"}
		if !reflect.DeepEqual(result.Config.Symlinks, expected) {
			t.Errorf("Symlinks = %v, want %v", result.Config.Symlinks, expected)
		}
	})
}

func TestLoadConfig_WorktreeDirs(t *testing.T) {
	t.Parallel()

	t.Run("WorktreeSourceDirIsConfigLoadDir", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		tmpDir, _ = filepath.EvalSymlinks(tmpDir)
		twigDir := filepath.Join(tmpDir, configDir)
		if err := os.MkdirAll(twigDir, 0755); err != nil {
			t.Fatal(err)
		}

		// Empty config - WorktreeSourceDir should be set to tmpDir
		if err := os.WriteFile(filepath.Join(twigDir, configFileName), []byte(""), 0644); err != nil {
			t.Fatal(err)
		}

		result, err := LoadConfig(tmpDir)
		if err != nil {
			t.Fatal(err)
		}

		if result.Config.WorktreeSourceDir != tmpDir {
			t.Errorf("WorktreeSourceDir = %q, want %q", result.Config.WorktreeSourceDir, tmpDir)
		}
	})

	t.Run("LocalOverridesDestBaseDir", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		tmpDir, _ = filepath.EvalSymlinks(tmpDir)
		twigDir := filepath.Join(tmpDir, configDir)
		if err := os.MkdirAll(twigDir, 0755); err != nil {
			t.Fatal(err)
		}

		projectDestDir := filepath.Join(tmpDir, "project-dest")
		localDestDir := filepath.Join(tmpDir, "local-dest")

		// Project config
		projectSettings := "worktree_destination_base_dir = " + `"` + projectDestDir + `"` + "\n"
		if err := os.WriteFile(filepath.Join(twigDir, configFileName), []byte(projectSettings), 0644); err != nil {
			t.Fatal(err)
		}

		// Local config overrides
		localSettings := "worktree_destination_base_dir = " + `"` + localDestDir + `"` + "\n"
		if err := os.WriteFile(filepath.Join(twigDir, localConfigFileName), []byte(localSettings), 0644); err != nil {
			t.Fatal(err)
		}

		result, err := LoadConfig(tmpDir)
		if err != nil {
			t.Fatal(err)
		}

		if result.Config.WorktreeDestBaseDir != localDestDir {
			t.Errorf("WorktreeDestBaseDir = %q, want %q", result.Config.WorktreeDestBaseDir, localDestDir)
		}
	})

	t.Run("NoWarningForLocalWorktreeDestDir", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		tmpDir, _ = filepath.EvalSymlinks(tmpDir)
		twigDir := filepath.Join(tmpDir, configDir)
		if err := os.MkdirAll(twigDir, 0755); err != nil {
			t.Fatal(err)
		}

		localDestDir := filepath.Join(tmpDir, "local-dest")

		// Project config (empty)
		projectSettings := ``
		if err := os.WriteFile(filepath.Join(twigDir, configFileName), []byte(projectSettings), 0644); err != nil {
			t.Fatal(err)
		}

		// Local config with worktree_destination_base_dir
		localSettings := "worktree_destination_base_dir = " + `"` + localDestDir + `"` + "\n"
		if err := os.WriteFile(filepath.Join(twigDir, localConfigFileName), []byte(localSettings), 0644); err != nil {
			t.Fatal(err)
		}

		result, err := LoadConfig(tmpDir)
		if err != nil {
			t.Fatal(err)
		}

		// No warnings should be generated
		if len(result.Warnings) > 0 {
			t.Errorf("expected no warnings, got: %v", result.Warnings)
		}
	})

	t.Run("RelativeDestBaseDirResolvedFromConfigDir", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		tmpDir, _ = filepath.EvalSymlinks(tmpDir)
		twigDir := filepath.Join(tmpDir, configDir)
		if err := os.MkdirAll(twigDir, 0755); err != nil {
			t.Fatal(err)
		}

		// Project config with relative path
		projectSettings := `worktree_destination_base_dir = "../workspace"
`
		if err := os.WriteFile(filepath.Join(twigDir, configFileName), []byte(projectSettings), 0644); err != nil {
			t.Fatal(err)
		}

		result, err := LoadConfig(tmpDir)
		if err != nil {
			t.Fatal(err)
		}

		// Relative path should be resolved from config directory (tmpDir)
		// "../workspace" from tmpDir should resolve to sibling "workspace" directory
		expected := filepath.Join(filepath.Dir(tmpDir), "workspace")
		if result.Config.WorktreeDestBaseDir != expected {
			t.Errorf("WorktreeDestBaseDir = %q, want %q", result.Config.WorktreeDestBaseDir, expected)
		}
	})

	t.Run("AbsoluteDestBaseDirUnchanged", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		tmpDir, _ = filepath.EvalSymlinks(tmpDir)
		twigDir := filepath.Join(tmpDir, configDir)
		if err := os.MkdirAll(twigDir, 0755); err != nil {
			t.Fatal(err)
		}

		absDestDir := "/absolute/path/to/worktrees"

		// Project config with absolute path
		projectSettings := `worktree_destination_base_dir = "` + absDestDir + `"
`
		if err := os.WriteFile(filepath.Join(twigDir, configFileName), []byte(projectSettings), 0644); err != nil {
			t.Fatal(err)
		}

		result, err := LoadConfig(tmpDir)
		if err != nil {
			t.Fatal(err)
		}

		// Absolute path should remain unchanged
		if result.Config.WorktreeDestBaseDir != absDestDir {
			t.Errorf("WorktreeDestBaseDir = %q, want %q", result.Config.WorktreeDestBaseDir, absDestDir)
		}
	})
}

func TestLoadConfig_WithMainWorktreeDir(t *testing.T) {
	t.Parallel()

	t.Run("RelativeDestBaseDirResolvedFromMainWorktree", func(t *testing.T) {
		t.Parallel()

		// Simulate: mainDir is /tmp/xxx/repo, worktreeDir is /tmp/xxx/repo-worktree/feat-a
		baseDir := t.TempDir()
		baseDir, _ = filepath.EvalSymlinks(baseDir)

		mainDir := filepath.Join(baseDir, "repo")
		worktreeDir := filepath.Join(baseDir, "repo-worktree", "feat-a")

		// Create config in worktreeDir
		twigDir := filepath.Join(worktreeDir, configDir)
		if err := os.MkdirAll(twigDir, 0755); err != nil {
			t.Fatal(err)
		}

		settings := `worktree_destination_base_dir = ".claude/worktrees"
`
		if err := os.WriteFile(filepath.Join(twigDir, configFileName), []byte(settings), 0644); err != nil {
			t.Fatal(err)
		}

		result, err := LoadConfig(worktreeDir, WithMainWorktreeDir(mainDir))
		if err != nil {
			t.Fatal(err)
		}

		// Should resolve relative to mainDir, not worktreeDir
		expected := filepath.Join(mainDir, ".claude", "worktrees")
		if result.Config.WorktreeDestBaseDir != expected {
			t.Errorf("WorktreeDestBaseDir = %q, want %q", result.Config.WorktreeDestBaseDir, expected)
		}
	})

	t.Run("DefaultDestBaseDirUsesMainWorktree", func(t *testing.T) {
		t.Parallel()

		baseDir := t.TempDir()
		baseDir, _ = filepath.EvalSymlinks(baseDir)

		mainDir := filepath.Join(baseDir, "my-project")
		worktreeDir := filepath.Join(baseDir, "my-project-worktree", "feat-a")

		// Create config in worktreeDir with no worktree_destination_base_dir
		twigDir := filepath.Join(worktreeDir, configDir)
		if err := os.MkdirAll(twigDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(twigDir, configFileName), []byte(""), 0644); err != nil {
			t.Fatal(err)
		}

		result, err := LoadConfig(worktreeDir, WithMainWorktreeDir(mainDir))
		if err != nil {
			t.Fatal(err)
		}

		// Default should be ../<mainDir-basename>-worktree resolved from mainDir
		expected := filepath.Join(baseDir, "my-project-worktree")
		if result.Config.WorktreeDestBaseDir != expected {
			t.Errorf("WorktreeDestBaseDir = %q, want %q", result.Config.WorktreeDestBaseDir, expected)
		}
	})

	t.Run("WithoutMainWorktreeDirFallsBackToSrcDir", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		tmpDir, _ = filepath.EvalSymlinks(tmpDir)
		twigDir := filepath.Join(tmpDir, configDir)
		if err := os.MkdirAll(twigDir, 0755); err != nil {
			t.Fatal(err)
		}

		settings := `worktree_destination_base_dir = "worktrees"
`
		if err := os.WriteFile(filepath.Join(twigDir, configFileName), []byte(settings), 0644); err != nil {
			t.Fatal(err)
		}

		// No WithMainWorktreeDir option - should resolve from srcDir (tmpDir)
		result, err := LoadConfig(tmpDir)
		if err != nil {
			t.Fatal(err)
		}

		expected := filepath.Join(tmpDir, "worktrees")
		if result.Config.WorktreeDestBaseDir != expected {
			t.Errorf("WorktreeDestBaseDir = %q, want %q", result.Config.WorktreeDestBaseDir, expected)
		}
	})
}

func TestConfig_ShouldInitSubmodules(t *testing.T) {
	t.Parallel()

	boolPtr := func(b bool) *bool { return &b }

	tests := []struct {
		name           string
		initSubmodules *bool
		want           bool
	}{
		{"nil returns false", nil, false},
		{"true returns true", boolPtr(true), true},
		{"false returns false", boolPtr(false), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := &Config{InitSubmodules: tt.initSubmodules}
			if got := cfg.ShouldInitSubmodules(); got != tt.want {
				t.Errorf("ShouldInitSubmodules() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConfig_ShouldCleanStale(t *testing.T) {
	t.Parallel()

	boolPtr := func(b bool) *bool { return &b }

	tests := []struct {
		name       string
		cleanStale *bool
		want       bool
	}{
		{"nil returns false", nil, false},
		{"true returns true", boolPtr(true), true},
		{"false returns false", boolPtr(false), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := &Config{CleanStale: tt.cleanStale}
			if got := cfg.ShouldCleanStale(); got != tt.want {
				t.Errorf("ShouldCleanStale() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConfig_ShouldUseSubmoduleReference(t *testing.T) {
	t.Parallel()

	boolPtr := func(b bool) *bool { return &b }

	tests := []struct {
		name               string
		submoduleReference *bool
		want               bool
	}{
		{"nil returns false", nil, false},
		{"true returns true", boolPtr(true), true},
		{"false returns false", boolPtr(false), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := &Config{SubmoduleReference: tt.submoduleReference}
			if got := cfg.ShouldUseSubmoduleReference(); got != tt.want {
				t.Errorf("ShouldUseSubmoduleReference() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLoadConfig_CleanStale(t *testing.T) {
	t.Parallel()

	t.Run("ProjectOnly", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		twigDir := filepath.Join(tmpDir, configDir)
		if err := os.MkdirAll(twigDir, 0755); err != nil {
			t.Fatal(err)
		}

		projectSettings := `clean_stale = true
`
		if err := os.WriteFile(filepath.Join(twigDir, configFileName), []byte(projectSettings), 0644); err != nil {
			t.Fatal(err)
		}

		result, err := LoadConfig(tmpDir)
		if err != nil {
			t.Fatal(err)
		}

		if result.Config.CleanStale == nil || *result.Config.CleanStale != true {
			t.Errorf("CleanStale = %v, want true", result.Config.CleanStale)
		}
	})

	t.Run("LocalOverridesProject", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		twigDir := filepath.Join(tmpDir, configDir)
		if err := os.MkdirAll(twigDir, 0755); err != nil {
			t.Fatal(err)
		}

		projectSettings := `clean_stale = true
`
		if err := os.WriteFile(filepath.Join(twigDir, configFileName), []byte(projectSettings), 0644); err != nil {
			t.Fatal(err)
		}

		localSettings := `clean_stale = false
`
		if err := os.WriteFile(filepath.Join(twigDir, localConfigFileName), []byte(localSettings), 0644); err != nil {
			t.Fatal(err)
		}

		result, err := LoadConfig(tmpDir)
		if err != nil {
			t.Fatal(err)
		}

		if result.Config.CleanStale == nil || *result.Config.CleanStale != false {
			t.Errorf("CleanStale = %v, want false", result.Config.CleanStale)
		}
	})

	t.Run("NilWhenUnset", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		twigDir := filepath.Join(tmpDir, configDir)
		if err := os.MkdirAll(twigDir, 0755); err != nil {
			t.Fatal(err)
		}

		projectSettings := ``
		if err := os.WriteFile(filepath.Join(twigDir, configFileName), []byte(projectSettings), 0644); err != nil {
			t.Fatal(err)
		}

		result, err := LoadConfig(tmpDir)
		if err != nil {
			t.Fatal(err)
		}

		if result.Config.CleanStale != nil {
			t.Errorf("CleanStale = %v, want nil", result.Config.CleanStale)
		}
	})

	t.Run("HelperMethodMatchesValue", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		twigDir := filepath.Join(tmpDir, configDir)
		if err := os.MkdirAll(twigDir, 0755); err != nil {
			t.Fatal(err)
		}

		projectSettings := `clean_stale = true
`
		if err := os.WriteFile(filepath.Join(twigDir, configFileName), []byte(projectSettings), 0644); err != nil {
			t.Fatal(err)
		}

		result, err := LoadConfig(tmpDir)
		if err != nil {
			t.Fatal(err)
		}

		if !result.Config.ShouldCleanStale() {
			t.Error("ShouldCleanStale() = false, want true")
		}
	})
}

func TestLoadConfig_Hooks(t *testing.T) {
	t.Parallel()

	t.Run("ProjectOnly", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		twigDir := filepath.Join(tmpDir, configDir)
		if err := os.MkdirAll(twigDir, 0755); err != nil {
			t.Fatal(err)
		}

		projectSettings := `hooks = ["npm install", "direnv allow"]
`
		if err := os.WriteFile(filepath.Join(twigDir, configFileName), []byte(projectSettings), 0644); err != nil {
			t.Fatal(err)
		}

		result, err := LoadConfig(tmpDir)
		if err != nil {
			t.Fatal(err)
		}

		expected := []string{"npm install", "direnv allow"}
		if !reflect.DeepEqual(result.Config.Hooks, expected) {
			t.Errorf("Hooks = %v, want %v", result.Config.Hooks, expected)
		}
	})

	t.Run("LocalOverridesProject", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		twigDir := filepath.Join(tmpDir, configDir)
		if err := os.MkdirAll(twigDir, 0755); err != nil {
			t.Fatal(err)
		}

		projectSettings := `hooks = ["npm install", "direnv allow"]
`
		if err := os.WriteFile(filepath.Join(twigDir, configFileName), []byte(projectSettings), 0644); err != nil {
			t.Fatal(err)
		}

		localSettings := `hooks = ["yarn install"]
`
		if err := os.WriteFile(filepath.Join(twigDir, localConfigFileName), []byte(localSettings), 0644); err != nil {
			t.Fatal(err)
		}

		result, err := LoadConfig(tmpDir)
		if err != nil {
			t.Fatal(err)
		}

		expected := []string{"yarn install"}
		if !reflect.DeepEqual(result.Config.Hooks, expected) {
			t.Errorf("Hooks = %v, want %v", result.Config.Hooks, expected)
		}
	})

	t.Run("EmptyLocalDoesNotOverride", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		twigDir := filepath.Join(tmpDir, configDir)
		if err := os.MkdirAll(twigDir, 0755); err != nil {
			t.Fatal(err)
		}

		projectSettings := `hooks = ["npm install"]
`
		if err := os.WriteFile(filepath.Join(twigDir, configFileName), []byte(projectSettings), 0644); err != nil {
			t.Fatal(err)
		}

		localSettings := `hooks = []
`
		if err := os.WriteFile(filepath.Join(twigDir, localConfigFileName), []byte(localSettings), 0644); err != nil {
			t.Fatal(err)
		}

		result, err := LoadConfig(tmpDir)
		if err != nil {
			t.Fatal(err)
		}

		expected := []string{"npm install"}
		if !reflect.DeepEqual(result.Config.Hooks, expected) {
			t.Errorf("Hooks = %v, want %v", result.Config.Hooks, expected)
		}
	})

	t.Run("NilWhenUnset", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		twigDir := filepath.Join(tmpDir, configDir)
		if err := os.MkdirAll(twigDir, 0755); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(filepath.Join(twigDir, configFileName), []byte(""), 0644); err != nil {
			t.Fatal(err)
		}

		result, err := LoadConfig(tmpDir)
		if err != nil {
			t.Fatal(err)
		}

		if result.Config.Hooks != nil {
			t.Errorf("Hooks = %v, want nil", result.Config.Hooks)
		}
	})
}
