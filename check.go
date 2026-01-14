package twig

import (
	"fmt"
	"path/filepath"
	"strings"
)

// CheckCommand validates twig configuration and symlink patterns.
type CheckCommand struct {
	FS     FileSystem
	Git    *GitRunner
	Config *Config
}

// CheckOptions holds options for the check command.
type CheckOptions struct {
	Verbose bool
	Quiet   bool
}

// CheckSeverity represents the severity level of a check item.
type CheckSeverity string

const (
	SeverityOK    CheckSeverity = "ok"
	SeverityInfo  CheckSeverity = "info"
	SeverityWarn  CheckSeverity = "warn"
	SeverityError CheckSeverity = "error"
)

// CheckCategory represents the category of a check item.
type CheckCategory string

const (
	CategoryConfig   CheckCategory = "config"
	CategorySymlinks CheckCategory = "symlinks"
)

// CheckItem represents a single check result.
type CheckItem struct {
	Category   CheckCategory
	Severity   CheckSeverity
	Message    string
	Suggestion string
}

// CheckResult holds the result of all checks.
type CheckResult struct {
	Items      []CheckItem
	ConfigPath string
}

// NewCheckCommand creates a CheckCommand with explicit dependencies (for testing).
func NewCheckCommand(fs FileSystem, git *GitRunner, cfg *Config) *CheckCommand {
	return &CheckCommand{
		FS:     fs,
		Git:    git,
		Config: cfg,
	}
}

// NewDefaultCheckCommand creates a CheckCommand with production defaults.
func NewDefaultCheckCommand(cfg *Config) *CheckCommand {
	return NewCheckCommand(osFS{}, NewGitRunner(cfg.WorktreeSourceDir), cfg)
}

// CheckFormatOptions holds formatting options for CheckResult.
type CheckFormatOptions struct {
	Verbose bool
	Quiet   bool
}

// ErrorCount returns the number of errors.
func (r CheckResult) ErrorCount() int {
	count := 0
	for _, item := range r.Items {
		if item.Severity == SeverityError {
			count++
		}
	}
	return count
}

// WarningCount returns the number of warnings.
func (r CheckResult) WarningCount() int {
	count := 0
	for _, item := range r.Items {
		if item.Severity == SeverityWarn {
			count++
		}
	}
	return count
}

// InfoCount returns the number of info items.
func (r CheckResult) InfoCount() int {
	count := 0
	for _, item := range r.Items {
		if item.Severity == SeverityInfo {
			count++
		}
	}
	return count
}

// Format formats the CheckResult for display.
func (r CheckResult) Format(opts CheckFormatOptions) FormatResult {
	var stdout strings.Builder

	// Group items by category
	configItems := r.filterByCategory(CategoryConfig)
	symlinkItems := r.filterByCategory(CategorySymlinks)

	if opts.Quiet {
		// Quiet mode: only show errors
		for _, item := range r.Items {
			if item.Severity == SeverityError {
				fmt.Fprintf(&stdout, "[error] %s\n", item.Message)
			}
		}
		return FormatResult{Stdout: stdout.String()}
	}

	// Default or verbose mode
	if len(configItems) > 0 {
		r.formatCategory(&stdout, "config:", configItems, opts.Verbose)
	}

	if len(symlinkItems) > 0 {
		if len(configItems) > 0 {
			fmt.Fprintln(&stdout)
		}
		r.formatCategory(&stdout, "symlinks:", symlinkItems, opts.Verbose)
	}

	// Summary
	fmt.Fprintf(&stdout, "\nSummary: %d errors, %d warnings, %d info\n",
		r.ErrorCount(), r.WarningCount(), r.InfoCount())

	return FormatResult{Stdout: stdout.String()}
}

func (r CheckResult) filterByCategory(cat CheckCategory) []CheckItem {
	var items []CheckItem
	for _, item := range r.Items {
		if item.Category == cat {
			items = append(items, item)
		}
	}
	return items
}

func (r CheckResult) formatCategory(w *strings.Builder, header string, items []CheckItem, verbose bool) {
	fmt.Fprintln(w, header)

	for _, item := range items {
		// Skip ok items unless verbose
		if item.Severity == SeverityOK && !verbose {
			continue
		}

		fmt.Fprintf(w, "  [%s] %s\n", item.Severity, item.Message)
		if item.Suggestion != "" {
			fmt.Fprintf(w, "         suggestion: %s\n", item.Suggestion)
		}
	}
}

// Run executes all checks and returns the result.
func (c *CheckCommand) Run() (CheckResult, error) {
	var result CheckResult

	// Set config path for reference
	result.ConfigPath = filepath.Join(c.Config.WorktreeSourceDir, configDir, configFileName)

	// Config checks
	c.checkConfig(&result)

	// Symlink pattern checks
	c.checkSymlinks(&result)

	return result, nil
}

func (c *CheckCommand) checkConfig(result *CheckResult) {
	// Check TOML syntax validity (already validated by LoadConfig)
	result.Items = append(result.Items, CheckItem{
		Category: CategoryConfig,
		Severity: SeverityOK,
		Message:  "TOML syntax valid",
	})

	// Check worktree_destination_base_dir existence
	destDir := c.Config.WorktreeDestBaseDir
	if _, err := c.FS.Stat(destDir); err != nil {
		if c.FS.IsNotExist(err) {
			result.Items = append(result.Items, CheckItem{
				Category:   CategoryConfig,
				Severity:   SeverityWarn,
				Message:    fmt.Sprintf("worktree_destination_base_dir does not exist: %s", destDir),
				Suggestion: fmt.Sprintf("run 'mkdir -p %s'", destDir),
			})
		} else {
			result.Items = append(result.Items, CheckItem{
				Category: CategoryConfig,
				Severity: SeverityError,
				Message:  fmt.Sprintf("cannot access worktree_destination_base_dir: %v", err),
			})
		}
	} else {
		// Check write permission by attempting to create a temp file
		if writable := c.checkWritable(destDir); !writable {
			result.Items = append(result.Items, CheckItem{
				Category:   CategoryConfig,
				Severity:   SeverityWarn,
				Message:    fmt.Sprintf("worktree_destination_base_dir is not writable: %s", destDir),
				Suggestion: "check directory permissions",
			})
		} else {
			result.Items = append(result.Items, CheckItem{
				Category: CategoryConfig,
				Severity: SeverityOK,
				Message:  "worktree_destination_base_dir exists and is writable",
			})
		}
	}
}

func (c *CheckCommand) checkWritable(dir string) bool {
	// Try to write a temp file to check writability
	tmpFile := filepath.Join(dir, ".twig-check-write-test")
	if err := c.FS.WriteFile(tmpFile, []byte{}, 0644); err != nil {
		return false
	}
	c.FS.Remove(tmpFile)
	return true
}

func (c *CheckCommand) checkSymlinks(result *CheckResult) {
	srcDir := c.Config.WorktreeSourceDir

	for _, pattern := range c.Config.Symlinks {
		// Check for invalid glob pattern
		matches, err := c.FS.Glob(srcDir, pattern)
		if err != nil {
			result.Items = append(result.Items, CheckItem{
				Category: CategorySymlinks,
				Severity: SeverityError,
				Message:  fmt.Sprintf("invalid glob pattern %q: %v", pattern, err),
			})
			continue
		}

		// Check if pattern matches any files
		if len(matches) == 0 {
			result.Items = append(result.Items, CheckItem{
				Category:   CategorySymlinks,
				Severity:   SeverityWarn,
				Message:    fmt.Sprintf("pattern %q matches no files", pattern),
				Suggestion: "remove from symlinks or create the file",
			})
			continue
		}

		// Check if matched files are gitignored
		for _, match := range matches {
			ignored, err := c.Git.CheckIgnore(match)
			if err != nil {
				// git check-ignore might fail, skip silently
				continue
			}
			if ignored {
				result.Items = append(result.Items, CheckItem{
					Category: CategorySymlinks,
					Severity: SeverityInfo,
					Message:  fmt.Sprintf("%q is gitignored (symlink may not work as expected)", match),
				})
			}
		}

		// Add success item for verbose output
		result.Items = append(result.Items, CheckItem{
			Category: CategorySymlinks,
			Severity: SeverityOK,
			Message:  fmt.Sprintf("pattern %q matches %d file(s)", pattern, len(matches)),
		})
	}
}
