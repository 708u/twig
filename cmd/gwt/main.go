package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/708u/gwt"
	"github.com/spf13/cobra"
)

// AddCommander is the interface for AddCommand execution.
type AddCommander interface {
	Run(name string) (gwt.AddResult, error)
}

// CleanCommander defines the interface for clean operations.
type CleanCommander interface {
	Run(cwd string, opts gwt.CleanOptions) (gwt.CleanResult, error)
}

// NewCleanCommander is the factory function type for creating CleanCommander instances.
type NewCleanCommander func(cfg *gwt.Config) CleanCommander

func defaultNewCleanCommander(cfg *gwt.Config) CleanCommander {
	return gwt.NewDefaultCleanCommand(cfg)
}

// ListCommander defines the interface for list operations.
type ListCommander interface {
	Run() (gwt.ListResult, error)
}

// NewListCommander is the factory function type for creating ListCommander instances.
type NewListCommander func(dir string) ListCommander

func defaultNewListCommander(dir string) ListCommander {
	return gwt.NewDefaultListCommand(dir)
}

// RemoveCommander defines the interface for remove operations.
type RemoveCommander interface {
	Run(branch string, cwd string, opts gwt.RemoveOptions) (gwt.RemovedWorktree, error)
}

// NewRemoveCommander is the factory function type for creating RemoveCommander instances.
type NewRemoveCommander func(cfg *gwt.Config) RemoveCommander

func defaultNewRemoveCommander(cfg *gwt.Config) RemoveCommander {
	return gwt.NewDefaultRemoveCommand(cfg)
}

type options struct {
	addCommander       AddCommander // nil = use default
	newCleanCommander  NewCleanCommander
	newListCommander   NewListCommander
	newRemoveCommander NewRemoveCommander
}

// Option configures newRootCmd.
type Option func(*options)

// WithAddCommander sets the AddCommander instance for testing.
func WithAddCommander(cmd AddCommander) Option {
	return func(o *options) {
		o.addCommander = cmd
	}
}

// WithNewCleanCommander sets the factory function for creating CleanCommander instances.
func WithNewCleanCommander(ncc NewCleanCommander) Option {
	return func(o *options) {
		o.newCleanCommander = ncc
	}
}

// WithNewListCommander sets the factory function for creating ListCommander instances.
func WithNewListCommander(nlc NewListCommander) Option {
	return func(o *options) {
		o.newListCommander = nlc
	}
}

// WithNewRemoveCommander sets the factory function for creating RemoveCommander instances.
func WithNewRemoveCommander(nrc NewRemoveCommander) Option {
	return func(o *options) {
		o.newRemoveCommander = nrc
	}
}

// resolveCarryFrom resolves the --carry flag value to a worktree path.
func resolveCarryFrom(carryValue, cwd, originalCwd string, git *gwt.GitRunner) (string, error) {
	switch carryValue {
	case "", "<source>":
		return cwd, nil
	case "@":
		return originalCwd, nil
	default:
		path, err := git.WorktreeFindByBranch(carryValue)
		if err != nil {
			return "", fmt.Errorf("failed to find worktree for branch %q: %w", carryValue, err)
		}
		return path, nil
	}
}

func resolveDirectory(dirFlag, baseCwd string) (string, error) {
	if dirFlag == "" {
		return baseCwd, nil
	}

	var resolved string
	if !filepath.IsAbs(dirFlag) {
		resolved = filepath.Join(baseCwd, dirFlag)
	} else {
		resolved = dirFlag
	}

	resolved, err := filepath.Abs(resolved)
	if err != nil {
		return "", fmt.Errorf("failed to resolve path: %w", err)
	}

	info, err := os.Stat(resolved)
	if err != nil {
		return "", fmt.Errorf("cannot change to '%s': %w", dirFlag, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("cannot change to '%s': not a directory", dirFlag)
	}

	return resolved, nil
}

func newRootCmd(opts ...Option) *cobra.Command {
	o := &options{
		newCleanCommander:  defaultNewCleanCommander,
		newListCommander:   defaultNewListCommander,
		newRemoveCommander: defaultNewRemoveCommander,
	}
	for _, opt := range opts {
		opt(o)
	}

	var (
		cfg         *gwt.Config
		cwd         string
		originalCwd string
		dirFlag     string
	)

	resolveCompletionDirectory := func(cmd *cobra.Command) (string, error) {
		currentCwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		flag, _ := cmd.Root().PersistentFlags().GetString("directory")
		return resolveDirectory(flag, currentCwd)
	}

	rootCmd := &cobra.Command{
		Use:           "gwt",
		Short:         "Manage git worktrees and branches together",
		SilenceErrors: true,
		SilenceUsage:  true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			var err error
			originalCwd, err = os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get current directory: %w", err)
			}

			cwd, err = resolveDirectory(dirFlag, originalCwd)
			if err != nil {
				return err
			}

			result, err := gwt.LoadConfig(cwd)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}
			for _, w := range result.Warnings {
				fmt.Fprintln(cmd.ErrOrStderr(), "warning:", w)
			}
			cfg = result.Config
			return nil
		},
	}

	addCmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Create a new worktree with a new branch",
		Long: `Create a new worktree with a new branch.

Creates worktree at WorktreeDestBaseDir/<name> and sets up symlinks
based on configuration.

Use --sync to copy uncommitted changes (both worktrees keep them).
Use --carry to move uncommitted changes (only new worktree has them).

With --carry, use --file to carry only matching files:

  gwt add feat/new --carry --file "*.go"
  gwt add feat/new --carry --file "*.go" --file "cmd/**"`,
		Args: cobra.ExactArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) >= 1 {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			dir, err := resolveCompletionDirectory(cmd)
			if err != nil {
				return nil, cobra.ShellCompDirectiveError
			}
			git := gwt.NewGitRunner(dir)
			branches, err := git.BranchList()
			if err != nil {
				return nil, cobra.ShellCompDirectiveError
			}
			return branches, cobra.ShellCompDirectiveNoFileComp
		},
		PreRunE: func(cmd *cobra.Command, args []string) error {
			source, _ := cmd.Flags().GetString("source")
			sync, _ := cmd.Flags().GetBool("sync")
			carryEnabled := cmd.Flags().Changed("carry")

			// --sync and --carry are mutually exclusive
			if sync && carryEnabled {
				return fmt.Errorf("cannot use --sync and --carry together")
			}

			// Resolve effective source: CLI --source > config default_source
			if source == "" {
				source = cfg.DefaultSource
			}

			if source == "" {
				return nil
			}

			// Resolve branch to worktree path
			git := gwt.NewGitRunner(cwd)
			sourcePath, err := git.WorktreeFindByBranch(source)
			if err != nil {
				return fmt.Errorf("failed to find worktree for branch %q: %w", source, err)
			}

			// Update cwd and reload config
			cwd = sourcePath
			result, err := gwt.LoadConfig(cwd)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}
			for _, w := range result.Warnings {
				fmt.Fprintln(cmd.ErrOrStderr(), "warning:", w)
			}
			cfg = result.Config
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			verbose, _ := cmd.Flags().GetBool("verbose")
			sync, _ := cmd.Flags().GetBool("sync")
			quiet, _ := cmd.Flags().GetBool("quiet")
			lock, _ := cmd.Flags().GetBool("lock")
			lockReason, _ := cmd.Flags().GetString("reason")
			carryEnabled := cmd.Flags().Changed("carry")

			// Get file patterns from --file flag
			carryFiles, _ := cmd.Flags().GetStringArray("file")

			// --file requires --carry
			if len(carryFiles) > 0 && !carryEnabled {
				return fmt.Errorf("--file requires --carry flag")
			}

			// --reason requires --lock
			if lockReason != "" && !lock {
				return fmt.Errorf("--reason requires --lock")
			}

			// Resolve CarryFrom path
			var carryFrom string
			if carryEnabled {
				carryValue, _ := cmd.Flags().GetString("carry")
				git := gwt.NewGitRunner(cwd)
				var err error
				carryFrom, err = resolveCarryFrom(carryValue, cwd, originalCwd, git)
				if err != nil {
					return err
				}
			}

			var addCmd AddCommander
			if o.addCommander != nil {
				addCmd = o.addCommander
			} else {
				addCmd = gwt.NewDefaultAddCommand(cfg, gwt.AddOptions{
					Sync:       sync,
					CarryFrom:  carryFrom,
					CarryFiles: carryFiles,
					Lock:       lock,
					LockReason: lockReason,
				})
			}
			result, err := addCmd.Run(args[0])
			if err != nil {
				return err
			}

			formatted := result.Format(gwt.AddFormatOptions{
				Verbose: verbose,
				Quiet:   quiet,
			})
			if formatted.Stderr != "" {
				fmt.Fprint(cmd.ErrOrStderr(), formatted.Stderr)
			}
			fmt.Fprint(cmd.OutOrStdout(), formatted.Stdout)
			return nil
		},
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List all worktrees",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			quiet, _ := cmd.Flags().GetBool("quiet")

			result, err := o.newListCommander(cwd).Run()
			if err != nil {
				return err
			}

			formatted := result.Format(gwt.ListFormatOptions{Quiet: quiet})
			fmt.Fprint(cmd.OutOrStdout(), formatted.Stdout)
			return nil
		},
	}

	cleanCmd := &cobra.Command{
		Use:   "clean",
		Short: "Remove merged worktrees that are no longer needed",
		Long: `Remove worktrees that have been merged to the target branch.

By default, shows candidates and prompts for confirmation.
Use --yes to skip confirmation and remove immediately.
Use --check to only show candidates without prompting.

Safety checks (all must pass):
  - Branch is merged to target
  - No uncommitted changes
  - Worktree is not locked
  - Not the current directory
  - Not the main worktree`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			verbose, _ := cmd.Flags().GetBool("verbose")
			yes, _ := cmd.Flags().GetBool("yes")
			check, _ := cmd.Flags().GetBool("check")
			target, _ := cmd.Flags().GetString("target")
			forceCount, _ := cmd.Flags().GetCount("force")

			cleanCmd := o.newCleanCommander(cfg)

			// First pass: analyze candidates (always in check mode first)
			result, err := cleanCmd.Run(cwd, gwt.CleanOptions{
				Check:   true,
				Target:  target,
				Verbose: verbose,
				Force:   gwt.WorktreeForceLevel(forceCount),
			})
			if err != nil {
				return err
			}

			// If check mode or no candidates, just show output and exit
			if check || result.CleanableCount() == 0 {
				formatted := result.Format(gwt.FormatOptions{Verbose: verbose})
				if formatted.Stderr != "" {
					fmt.Fprint(cmd.ErrOrStderr(), formatted.Stderr)
				}
				fmt.Fprint(cmd.OutOrStdout(), formatted.Stdout)
				return nil
			}

			// Show candidates
			formatted := result.Format(gwt.FormatOptions{Verbose: verbose})
			if formatted.Stderr != "" {
				fmt.Fprint(cmd.ErrOrStderr(), formatted.Stderr)
			}
			fmt.Fprint(cmd.OutOrStdout(), formatted.Stdout)

			// If not --yes, prompt for confirmation
			if !yes {
				fmt.Fprint(cmd.OutOrStdout(), "\nProceed? [y/N]: ")
				reader := bufio.NewReader(cmd.InOrStdin())
				input, err := reader.ReadString('\n')
				if err != nil {
					return err
				}
				input = strings.TrimSpace(strings.ToLower(input))
				if input != "y" && input != "yes" {
					return nil
				}
			}

			// Second pass: execute removal
			result, err = cleanCmd.Run(cwd, gwt.CleanOptions{
				Check:   false,
				Target:  target,
				Verbose: verbose,
				Force:   gwt.WorktreeForceLevel(forceCount),
			})
			if err != nil {
				return err
			}

			formatted = result.Format(gwt.FormatOptions{Verbose: verbose})
			if formatted.Stderr != "" {
				fmt.Fprint(cmd.ErrOrStderr(), formatted.Stderr)
			}
			fmt.Fprint(cmd.OutOrStdout(), formatted.Stdout)
			return nil
		},
	}

	removeCmd := &cobra.Command{
		Use:   "remove <branch>...",
		Short: "Remove worktrees and their branches",
		Long: `Remove git worktrees and delete their associated branches.

The branch names are used to locate the worktrees.
By default, fails if there are uncommitted changes or the branch is not merged.
Use --force to override these checks.

Multiple branches can be specified. Errors on individual branches will not
stop processing of remaining branches.`,
		Args: cobra.MinimumNArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			dir, err := resolveCompletionDirectory(cmd)
			if err != nil {
				return nil, cobra.ShellCompDirectiveError
			}
			git := gwt.NewGitRunner(dir)
			branches, err := git.WorktreeListBranches()
			if err != nil {
				return nil, cobra.ShellCompDirectiveError
			}
			// Exclude already-specified branches from suggestions
			available := make([]string, 0, len(branches))
			for _, b := range branches {
				if !slices.Contains(args, b) {
					available = append(available, b)
				}
			}
			return available, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			verbose, _ := cmd.Flags().GetBool("verbose")
			forceCount, _ := cmd.Flags().GetCount("force")
			dryRun, _ := cmd.Flags().GetBool("dry-run")

			removeCmd := o.newRemoveCommander(cfg)
			var result gwt.RemoveResult

			for _, branch := range args {
				wt, err := removeCmd.Run(branch, cwd, gwt.RemoveOptions{
					Force:  gwt.WorktreeForceLevel(forceCount),
					DryRun: dryRun,
				})
				if err != nil {
					wt.Branch = branch
					wt.Err = err
				}
				result.Removed = append(result.Removed, wt)
			}

			formatted := result.Format(gwt.FormatOptions{Verbose: verbose})
			if formatted.Stderr != "" {
				fmt.Fprint(cmd.ErrOrStderr(), formatted.Stderr)
			}
			fmt.Fprint(cmd.OutOrStdout(), formatted.Stdout)

			if result.HasErrors() {
				return fmt.Errorf("failed to remove %d branch(es)", result.ErrorCount())
			}
			return nil
		},
	}

	// Register flags
	rootCmd.PersistentFlags().StringVarP(&dirFlag, "directory", "C", "", "Run as if gwt was started in <path>")
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "Enable verbose output")

	addCmd.Flags().BoolP("sync", "s", false, "Sync uncommitted changes to new worktree")
	addCmd.Flags().StringP("carry", "c", "", "Move uncommitted changes from source worktree (@: from current, <branch>: from specified)")
	addCmd.Flags().Lookup("carry").NoOptDefVal = "<source>"
	addCmd.Flags().BoolP("quiet", "q", false, "Output only the worktree path")
	addCmd.Flags().String("source", "", "Source branch's worktree to use")
	addCmd.Flags().Bool("lock", false, "Lock the worktree after creation")
	addCmd.Flags().String("reason", "", "Reason for locking (requires --lock)")
	addCmd.Flags().StringArrayP("file", "F", nil, "File patterns to carry (requires --carry)")
	addCmd.RegisterFlagCompletionFunc("file", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		// Resolve target directory from -C flag
		dir, err := resolveCompletionDirectory(cmd)
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}

		// If --source is specified, resolve to that worktree
		if source, _ := cmd.Flags().GetString("source"); source != "" {
			git := gwt.NewGitRunner(dir)
			if sourcePath, err := git.WorktreeFindByBranch(source); err == nil {
				dir = sourcePath
			}
		}

		// Get changed files from the target directory
		git := gwt.NewGitRunner(dir)
		files, err := git.ChangedFiles()
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}

		// Filter by prefix
		var completions []string
		for _, file := range files {
			if strings.HasPrefix(file, toComplete) {
				completions = append(completions, file)
			}
		}

		return completions, cobra.ShellCompDirectiveNoSpace
	})
	rootCmd.AddCommand(addCmd)

	listCmd.Flags().BoolP("quiet", "q", false, "Output only worktree paths")
	rootCmd.AddCommand(listCmd)

	cleanCmd.Flags().BoolP("yes", "y", false, "Execute removal without confirmation")
	cleanCmd.Flags().Bool("check", false, "Show candidates without prompting or removing")
	cleanCmd.Flags().String("target", "", "Target branch for merge check (default: auto-detect)")
	cleanCmd.Flags().CountP("force", "f", "Force clean (-f: unmerged/uncommitted, -ff: also locked)")
	rootCmd.AddCommand(cleanCmd)

	removeCmd.Flags().CountP("force", "f", "Force removal (-f: uncommitted/unmerged, -ff: also locked)")
	removeCmd.Flags().Bool("dry-run", false, "Show what would be removed without making changes")
	rootCmd.AddCommand(removeCmd)

	initCmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize gwt configuration",
		Long:  `Create a .gwt/settings.toml configuration file in the current directory.`,
		Args:  cobra.NoArgs,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Override parent's PersistentPreRunE to skip config loading
			// since init creates the config file
			var err error
			originalCwd, err = os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get current directory: %w", err)
			}

			cwd, err = resolveDirectory(dirFlag, originalCwd)
			if err != nil {
				return err
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			force, _ := cmd.Flags().GetBool("force")

			initCommand := gwt.NewInitCommand()
			result, err := initCommand.Run(cwd, gwt.InitOptions{Force: force})
			if err != nil {
				return err
			}

			formatted := result.Format(gwt.InitFormatOptions{})
			fmt.Fprint(cmd.OutOrStdout(), formatted.Stdout)
			return nil
		},
	}
	initCmd.Flags().BoolP("force", "f", false, "Overwrite existing configuration file")
	rootCmd.AddCommand(initCmd)

	return rootCmd
}

var rootCmd = newRootCmd()

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(rootCmd.ErrOrStderr(), "gwt:", err)
		os.Exit(1)
	}
}
