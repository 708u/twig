//go:build integration

package twig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitCommand_Integration(t *testing.T) {
	t.Parallel()

	t.Run("CreatesConfigDirectory", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()

		cmd := NewDefaultInitCommand()
		result, err := cmd.Run(tmpDir, InitOptions{})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		if !result.Created {
			t.Error("expected Created to be true")
		}

		// Verify .twig directory was created
		configDir := filepath.Join(tmpDir, ".twig")
		if _, err := os.Stat(configDir); os.IsNotExist(err) {
			t.Error(".twig directory should exist")
		}

		// Verify settings.toml was created
		settingsPath := filepath.Join(configDir, "settings.toml")
		if _, err := os.Stat(settingsPath); os.IsNotExist(err) {
			t.Error("settings.toml should exist")
		}
	})

	t.Run("SettingsFileHasCorrectContent", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()

		cmd := NewDefaultInitCommand()
		_, err := cmd.Run(tmpDir, InitOptions{})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		settingsPath := filepath.Join(tmpDir, ".twig", "settings.toml")
		content, err := os.ReadFile(settingsPath)
		if err != nil {
			t.Fatalf("failed to read settings file: %v", err)
		}

		// Verify content has expected elements
		contentStr := string(content)
		if !strings.Contains(contentStr, `default_source = "main"`) {
			t.Error("settings should contain default_source = main")
		}
		if !strings.Contains(contentStr, "symlinks = []") {
			t.Error("settings should contain symlinks = []")
		}
		if !strings.Contains(contentStr, "# worktree_destination_base_dir") {
			t.Error("settings should contain commented worktree_destination_base_dir")
		}
		if !strings.Contains(contentStr, "# init_submodules") {
			t.Error("settings should contain commented init_submodules")
		}
	})

	t.Run("SkipsExistingFile", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()

		// Create config directory and file first
		configDir := filepath.Join(tmpDir, ".twig")
		if err := os.MkdirAll(configDir, 0755); err != nil {
			t.Fatal(err)
		}
		existingContent := []byte("existing content")
		settingsPath := filepath.Join(configDir, "settings.toml")
		if err := os.WriteFile(settingsPath, existingContent, 0644); err != nil {
			t.Fatal(err)
		}

		cmd := NewDefaultInitCommand()
		result, err := cmd.Run(tmpDir, InitOptions{})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		if !result.Skipped {
			t.Error("expected Skipped to be true")
		}

		// Verify content was not changed
		content, err := os.ReadFile(settingsPath)
		if err != nil {
			t.Fatal(err)
		}
		if string(content) != string(existingContent) {
			t.Error("existing content should not be changed")
		}
	})

	t.Run("OverwritesWithForce", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()

		// Create config directory and file first
		configDir := filepath.Join(tmpDir, ".twig")
		if err := os.MkdirAll(configDir, 0755); err != nil {
			t.Fatal(err)
		}
		existingContent := []byte("existing content")
		settingsPath := filepath.Join(configDir, "settings.toml")
		if err := os.WriteFile(settingsPath, existingContent, 0644); err != nil {
			t.Fatal(err)
		}

		cmd := NewDefaultInitCommand()
		result, err := cmd.Run(tmpDir, InitOptions{Force: true})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		if !result.Created {
			t.Error("expected Created to be true")
		}
		if !result.Overwritten {
			t.Error("expected Overwritten to be true")
		}

		// Verify content was changed
		content, err := os.ReadFile(settingsPath)
		if err != nil {
			t.Fatal(err)
		}
		if string(content) == string(existingContent) {
			t.Error("content should be overwritten")
		}
	})

	t.Run("FormatOutputCreated", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()

		cmd := NewDefaultInitCommand()
		result, err := cmd.Run(tmpDir, InitOptions{})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		formatted := result.Format(InitFormatOptions{})
		if !strings.Contains(formatted.Stdout, "Created") {
			t.Errorf("output should contain 'Created': %s", formatted.Stdout)
		}
		if !strings.Contains(formatted.Stdout, ".twig/settings.toml") {
			t.Errorf("output should contain '.twig/settings.toml': %s", formatted.Stdout)
		}
	})

	t.Run("FormatOutputSkipped", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()

		// Create settings file first
		configDir := filepath.Join(tmpDir, ".twig")
		if err := os.MkdirAll(configDir, 0755); err != nil {
			t.Fatal(err)
		}
		settingsPath := filepath.Join(configDir, "settings.toml")
		if err := os.WriteFile(settingsPath, []byte(""), 0644); err != nil {
			t.Fatal(err)
		}

		cmd := NewDefaultInitCommand()
		result, err := cmd.Run(tmpDir, InitOptions{})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		formatted := result.Format(InitFormatOptions{})
		if !strings.Contains(formatted.Stdout, "Skipped") {
			t.Errorf("output should contain 'Skipped': %s", formatted.Stdout)
		}
		if !strings.Contains(formatted.Stdout, "already exists") {
			t.Errorf("output should contain 'already exists': %s", formatted.Stdout)
		}
	})
}
