package main

import (
	"fmt"
	"os"

	"github.com/708u/gwt"
	"github.com/spf13/cobra"
)

// TODO: config呼び出しの共通化
var rootCmd = &cobra.Command{
	Use:   "gwt",
	Short: "Manage git worktrees and branches together",
}

var addCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Create a new worktree with a new branch",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}

		result, err := gwt.LoadConfig(cwd)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
		for _, w := range result.Warnings {
			fmt.Fprintln(os.Stderr, "warning:", w)
		}

		return gwt.NewAddCommand(result.Config).Run(args[0])
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
	RunE: func(cmd *cobra.Command, args []string) error {
		force, _ := cmd.Flags().GetBool("force")
		dryRun, _ := cmd.Flags().GetBool("dry-run")

		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}

		result, err := gwt.LoadConfig(cwd)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
		for _, w := range result.Warnings {
			fmt.Fprintln(os.Stderr, "warning:", w)
		}

		return gwt.NewRemoveCommand(result.Config).Run(args[0], cwd, gwt.RemoveOptions{
			Force:  force,
			DryRun: dryRun,
		})
	},
}

func init() {
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
