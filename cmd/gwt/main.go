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

var (
	cfg         *gwt.Config
	cwd         string
	originalCwd string
	dirFlag     string
)

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

func resolveCompletionDirectory(cmd *cobra.Command) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dirFlag, _ := cmd.Root().PersistentFlags().GetString("directory")
	return resolveDirectory(dirFlag, cwd)
}

var rootCmd = &cobra.Command{
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
			fmt.Fprintln(os.Stderr, "warning:", w)
		}
		cfg = result.Config
		return nil
	},
}

var addCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Create a new worktree with a new branch",
	Args:  cobra.ExactArgs(1),
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
			fmt.Fprintln(os.Stderr, "warning:", w)
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

		// --reason requires --lock
		if lockReason != "" && !lock {
			return fmt.Errorf("--reason requires --lock")
		}

		// Resolve CarryFrom path
		var carryFrom string
		if cmd.Flags().Changed("carry") {
			carryValue, _ := cmd.Flags().GetString("carry")
			switch carryValue {
			case "":
				// --carry without value: use source worktree (cwd)
				carryFrom = cwd
			case "@":
				// --carry=@: use original worktree (where command was executed)
				carryFrom = originalCwd
			default:
				// --carry=<branch>: resolve branch to worktree path
				git := gwt.NewGitRunner(cwd)
				path, err := git.WorktreeFindByBranch(carryValue)
				if err != nil {
					return fmt.Errorf("failed to find worktree for branch %q: %w", carryValue, err)
				}
				carryFrom = path
			}
		}

		addCmd := gwt.NewAddCommand(cfg, gwt.AddOptions{
			Sync:       sync,
			CarryFrom:  carryFrom,
			Lock:       lock,
			LockReason: lockReason,
		})
		result, err := addCmd.Run(args[0])
		if err != nil {
			return err
		}

		formatted := result.Format(gwt.AddFormatOptions{
			Verbose: verbose,
			Quiet:   quiet,
		})
		if formatted.Stderr != "" {
			fmt.Fprint(os.Stderr, formatted.Stderr)
		}
		fmt.Fprint(os.Stdout, formatted.Stdout)
		return nil
	},
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all worktrees",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		quiet, _ := cmd.Flags().GetBool("quiet")

		result, err := gwt.NewListCommand(cwd).Run()
		if err != nil {
			return err
		}

		formatted := result.Format(gwt.ListFormatOptions{Quiet: quiet})
		fmt.Fprint(os.Stdout, formatted.Stdout)
		return nil
	},
}

var cleanCmd = &cobra.Command{
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

		cleanCommand := gwt.NewCleanCommand(cfg)

		// First pass: analyze candidates (always in check mode first)
		result, err := cleanCommand.Run(cwd, gwt.CleanOptions{
			Check:   true,
			Target:  target,
			Verbose: verbose,
		})
		if err != nil {
			return err
		}

		// If check mode or no candidates, just show output and exit
		if check || result.CleanableCount() == 0 {
			formatted := result.Format(gwt.FormatOptions{Verbose: verbose})
			if formatted.Stderr != "" {
				fmt.Fprint(os.Stderr, formatted.Stderr)
			}
			fmt.Fprint(os.Stdout, formatted.Stdout)
			return nil
		}

		// Show candidates
		formatted := result.Format(gwt.FormatOptions{Verbose: verbose})
		if formatted.Stderr != "" {
			fmt.Fprint(os.Stderr, formatted.Stderr)
		}
		fmt.Fprint(os.Stdout, formatted.Stdout)

		// If not --yes, prompt for confirmation
		if !yes {
			fmt.Print("\nProceed? [y/N]: ")
			reader := bufio.NewReader(os.Stdin)
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
		result, err = cleanCommand.Run(cwd, gwt.CleanOptions{
			Check:   false,
			Target:  target,
			Verbose: verbose,
		})
		if err != nil {
			return err
		}

		formatted = result.Format(gwt.FormatOptions{Verbose: verbose})
		if formatted.Stderr != "" {
			fmt.Fprint(os.Stderr, formatted.Stderr)
		}
		fmt.Fprint(os.Stdout, formatted.Stdout)
		return nil
	},
}

var removeCmd = &cobra.Command{
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
		force, _ := cmd.Flags().GetBool("force")
		dryRun, _ := cmd.Flags().GetBool("dry-run")

		removeCmd := gwt.NewRemoveCommand(cfg)
		var result gwt.RemoveResult

		for _, branch := range args {
			wt, err := removeCmd.Run(branch, cwd, gwt.RemoveOptions{
				Force:  force,
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
			fmt.Fprint(os.Stderr, formatted.Stderr)
		}
		fmt.Fprint(os.Stdout, formatted.Stdout)

		if result.HasErrors() {
			return fmt.Errorf("failed to remove %d branch(es)", result.ErrorCount())
		}
		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&dirFlag, "directory", "C", "", "Run as if gwt was started in <path>")
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "Enable verbose output")

	addCmd.Flags().BoolP("sync", "s", false, "Sync uncommitted changes to new worktree")
	addCmd.Flags().StringP("carry", "c", "", "Move uncommitted changes to new worktree (no value: from source, @: from current, <branch>: from branch)")
	addCmd.Flags().BoolP("quiet", "q", false, "Output only the worktree path")
	addCmd.Flags().String("source", "", "Source branch's worktree to use")
	addCmd.Flags().Bool("lock", false, "Lock the worktree after creation")
	addCmd.Flags().String("reason", "", "Reason for locking (requires --lock)")
	rootCmd.AddCommand(addCmd)

	listCmd.Flags().BoolP("quiet", "q", false, "Output only worktree paths")
	rootCmd.AddCommand(listCmd)

	cleanCmd.Flags().BoolP("yes", "y", false, "Execute removal without confirmation")
	cleanCmd.Flags().Bool("check", false, "Show candidates without prompting or removing")
	cleanCmd.Flags().String("target", "", "Target branch for merge check (default: auto-detect)")
	rootCmd.AddCommand(cleanCmd)

	removeCmd.Flags().BoolP("force", "f", false, "Force removal even with uncommitted changes or unmerged branch")
	removeCmd.Flags().Bool("dry-run", false, "Show what would be removed without making changes")
	rootCmd.AddCommand(removeCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "gwt:", err)
		os.Exit(1)
	}
}
