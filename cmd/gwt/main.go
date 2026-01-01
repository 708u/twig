package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/708u/gwt"
	"github.com/spf13/cobra"
)

var (
	cfg     *gwt.Config
	cwd     string
	dirFlag string
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
	Use:   "gwt",
	Short: "Manage git worktrees and branches together",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}

		cwd, err = resolveDirectory(dirFlag, cwd)
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
	RunE: func(cmd *cobra.Command, args []string) error {
		verbose, _ := cmd.Flags().GetBool("verbose")

		result, err := gwt.NewAddCommand(cfg).Run(args[0])
		if err != nil {
			return err
		}

		formatted := result.Format(gwt.FormatOptions{Verbose: verbose})
		if formatted.Stderr != "" {
			fmt.Fprint(os.Stderr, formatted.Stderr)
		}
		fmt.Fprint(os.Stdout, formatted.Stdout)
		return nil
	},
}

var removeCmd = &cobra.Command{
	Use:   "remove <branch>",
	Short: "Remove a worktree and its branch",
	Long: `Remove a git worktree and delete its associated branch.

The branch name is used to locate the worktree.
By default, fails if there are uncommitted changes or the branch is not merged.
Use --force to override these checks.`,
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
		branches, err := git.WorktreeListBranches()
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}
		return branches, cobra.ShellCompDirectiveNoFileComp
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		verbose, _ := cmd.Flags().GetBool("verbose")
		force, _ := cmd.Flags().GetBool("force")
		dryRun, _ := cmd.Flags().GetBool("dry-run")

		result, err := gwt.NewRemoveCommand(cfg).Run(args[0], cwd, gwt.RemoveOptions{
			Force:  force,
			DryRun: dryRun,
		})
		if err != nil {
			return err
		}

		formatted := result.Format(gwt.FormatOptions{Verbose: verbose})
		if formatted.Stderr != "" {
			fmt.Fprint(os.Stderr, formatted.Stderr)
		}
		fmt.Fprint(os.Stdout, formatted.Stdout)
		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&dirFlag, "directory", "C", "", "Run as if gwt was started in <path>")
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "Enable verbose output")
	rootCmd.AddCommand(addCmd)

	removeCmd.Flags().BoolP("force", "f", false, "Force removal even with uncommitted changes or unmerged branch")
	removeCmd.Flags().Bool("dry-run", false, "Show what would be removed without making changes")
	rootCmd.AddCommand(removeCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
