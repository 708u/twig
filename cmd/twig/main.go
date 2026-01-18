package main

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"text/tabwriter"

	"github.com/708u/twig"
	"github.com/spf13/cobra"
)

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

// AddCommander is the interface for AddCommand execution.
type AddCommander interface {
	Run(ctx context.Context, name string) (twig.AddResult, error)
}

// CleanCommander defines the interface for clean operations.
type CleanCommander interface {
	Run(ctx context.Context, cwd string, opts twig.CleanOptions) (twig.CleanResult, error)
}

// ListCommander defines the interface for list operations.
type ListCommander interface {
	Run(ctx context.Context) (twig.ListResult, error)
}

// RemoveCommander defines the interface for remove operations.
type RemoveCommander interface {
	Run(ctx context.Context, branch string, cwd string, opts twig.RemoveOptions) (twig.RemovedWorktree, error)
}

// InitCommander defines the interface for init operations.
type InitCommander interface {
	Run(ctx context.Context, dir string, opts twig.InitOptions) (twig.InitResult, error)
}

type options struct {
	addCommander       AddCommander    // nil = use default
	cleanCommander     CleanCommander  // nil = use default
	listCommander      ListCommander   // nil = use default
	removeCommander    RemoveCommander // nil = use default
	initCommander      InitCommander   // nil = use default
	commandIDGenerator func() string   // nil = use twig.GenerateCommandID
}

// Option configures newRootCmd.
type Option func(*options)

// WithAddCommander sets the AddCommander instance for testing.
func WithAddCommander(cmd AddCommander) Option {
	return func(o *options) {
		o.addCommander = cmd
	}
}

// WithCleanCommander sets the CleanCommander instance for testing.
func WithCleanCommander(cmd CleanCommander) Option {
	return func(o *options) {
		o.cleanCommander = cmd
	}
}

// WithListCommander sets the ListCommander instance for testing.
func WithListCommander(cmd ListCommander) Option {
	return func(o *options) {
		o.listCommander = cmd
	}
}

// WithRemoveCommander sets the RemoveCommander instance for testing.
func WithRemoveCommander(cmd RemoveCommander) Option {
	return func(o *options) {
		o.removeCommander = cmd
	}
}

// WithInitCommander sets the InitCommander instance for testing.
func WithInitCommander(cmd InitCommander) Option {
	return func(o *options) {
		o.initCommander = cmd
	}
}

// WithCommandIDGenerator sets the command ID generator for testing.
func WithCommandIDGenerator(gen func() string) Option {
	return func(o *options) {
		o.commandIDGenerator = gen
	}
}

// carryFromCurrent is the sentinel value for --carry flag to use current worktree.
const carryFromCurrent = "<current>"

// resolveCarryFrom resolves the --carry flag value to a worktree path.
func resolveCarryFrom(ctx context.Context, carryValue, originalCwd string, git *twig.GitRunner) (string, error) {
	switch carryValue {
	case carryFromCurrent:
		return originalCwd, nil
	case "":
		return "", fmt.Errorf("carry value cannot be empty")
	default:
		wt, err := git.WorktreeFindByBranch(ctx, carryValue)
		if err != nil {
			return "", fmt.Errorf("failed to find worktree for branch %q: %w", carryValue, err)
		}
		return wt.Path, nil
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
	o := &options{}
	for _, opt := range opts {
		opt(o)
	}

	var (
		cfg         *twig.Config
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
		Use:           "twig",
		Short:         "Manage git worktrees and branches together",
		Version:       version,
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

			result, err := twig.LoadConfig(cwd)
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
	rootCmd.SetVersionTemplate("{{.Version}}\n")

	addCmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Create a new worktree with a new branch",
		Long: `Create a new worktree with a new branch.

Creates worktree at WorktreeDestBaseDir/<name> and sets up symlinks
based on configuration.

Use --sync to copy uncommitted changes (both worktrees keep them).
Use --carry to move uncommitted changes (only new worktree has them).

Use --file with --sync or --carry to target specific files:

  twig add feat/new --sync --file "*.go"
  twig add feat/new --carry --file "*.go" --file "cmd/**"`,
		Args: cobra.ExactArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) >= 1 {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			dir, err := resolveCompletionDirectory(cmd)
			if err != nil {
				return nil, cobra.ShellCompDirectiveError
			}
			git := twig.NewGitRunner(dir)
			branches, err := git.BranchList(cmd.Context())
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
			git := twig.NewGitRunner(cwd)
			sourceWT, err := git.WorktreeFindByBranch(cmd.Context(), source)
			if err != nil {
				return fmt.Errorf("failed to find worktree for branch %q: %w", source, err)
			}

			// Load config from source worktree
			cwd = sourceWT.Path
			result, err := twig.LoadConfig(cwd)
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
			verbosity, _ := cmd.Flags().GetCount("verbose")
			verbose := verbosity >= 1
			sync, _ := cmd.Flags().GetBool("sync")
			quiet, _ := cmd.Flags().GetBool("quiet")
			lock, _ := cmd.Flags().GetBool("lock")
			lockReason, _ := cmd.Flags().GetString("reason")
			carryEnabled := cmd.Flags().Changed("carry")

			// Get file patterns from --file flag
			filePatterns, _ := cmd.Flags().GetStringArray("file")

			// --file requires --carry or --sync
			if len(filePatterns) > 0 && !carryEnabled && !sync {
				return fmt.Errorf("--file requires --carry or --sync flag")
			}

			// --init-submodules forces enable, otherwise use config
			initSubmodules := cmd.Flags().Changed("init-submodules")

			// --reason requires --lock
			if lockReason != "" && !lock {
				return fmt.Errorf("--reason requires --lock")
			}

			// Resolve CarryFrom path
			var carryFrom string
			if carryEnabled {
				carryValue, _ := cmd.Flags().GetString("carry")
				git := twig.NewGitRunner(cwd)
				var err error
				carryFrom, err = resolveCarryFrom(cmd.Context(), carryValue, originalCwd, git)
				if err != nil {
					return err
				}
			}

			var addCmd AddCommander
			if o.addCommander != nil {
				addCmd = o.addCommander
			} else {
				addCmd = twig.NewDefaultAddCommand(cfg, twig.AddOptions{
					Sync:           sync,
					CarryFrom:      carryFrom,
					FilePatterns:   filePatterns,
					Lock:           lock,
					LockReason:     lockReason,
					InitSubmodules: initSubmodules,
				})
			}
			result, err := addCmd.Run(cmd.Context(), args[0])
			if err != nil {
				return err
			}

			formatted := result.Format(twig.AddFormatOptions{
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
			verbosity, _ := cmd.Flags().GetCount("verbose")

			// Create logger based on verbosity level
			log := twig.NewNopLogger()
			if verbosity >= 2 {
				handler := twig.NewCLIHandler(cmd.ErrOrStderr(), twig.VerbosityToLevel(verbosity))
				// Add command ID for log grouping
				idGen := twig.GenerateCommandID
				if o.commandIDGenerator != nil {
					idGen = o.commandIDGenerator
				}
				handlerWithID := handler.WithAttrs([]slog.Attr{
					twig.LogAttrKeyCmdID.Attr(idGen()),
				})
				log = slog.New(handlerWithID)
			}

			var listCmd ListCommander
			if o.listCommander != nil {
				listCmd = o.listCommander
			} else {
				listCmd = twig.NewDefaultListCommand(cwd, log)
			}
			result, err := listCmd.Run(cmd.Context())
			if err != nil {
				return err
			}

			formatted := result.Format(twig.ListFormatOptions{Quiet: quiet})
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
			verbosity, _ := cmd.Flags().GetCount("verbose")
			verbose := verbosity >= 1
			yes, _ := cmd.Flags().GetBool("yes")
			check, _ := cmd.Flags().GetBool("check")
			target, _ := cmd.Flags().GetString("target")
			forceCount, _ := cmd.Flags().GetCount("force")

			// Create logger based on verbosity level
			log := twig.NewNopLogger()
			if verbosity >= 2 {
				handler := twig.NewCLIHandler(cmd.ErrOrStderr(), twig.VerbosityToLevel(verbosity))
				// Add command ID for log grouping
				idGen := twig.GenerateCommandID
				if o.commandIDGenerator != nil {
					idGen = o.commandIDGenerator
				}
				handlerWithID := handler.WithAttrs([]slog.Attr{
					twig.LogAttrKeyCmdID.Attr(idGen()),
				})
				log = slog.New(handlerWithID)
			}

			var cleanCmd CleanCommander
			if o.cleanCommander != nil {
				cleanCmd = o.cleanCommander
			} else {
				cleanCmd = twig.NewDefaultCleanCommand(cfg, log)
			}

			// First pass: analyze candidates (always in check mode first)
			result, err := cleanCmd.Run(cmd.Context(), cwd, twig.CleanOptions{
				Check:   true,
				Target:  target,
				Verbose: verbose,
				Force:   twig.WorktreeForceLevel(forceCount),
			})
			if err != nil {
				return err
			}

			// If check mode or no candidates, just show output and exit
			if check || result.CleanableCount() == 0 {
				formatted := result.Format(twig.FormatOptions{Verbose: verbose})
				if formatted.Stderr != "" {
					fmt.Fprint(cmd.ErrOrStderr(), formatted.Stderr)
				}
				fmt.Fprint(cmd.OutOrStdout(), formatted.Stdout)
				return nil
			}

			// Show candidates
			formatted := result.Format(twig.FormatOptions{Verbose: verbose})
			if formatted.Stderr != "" {
				fmt.Fprint(cmd.ErrOrStderr(), formatted.Stderr)
			}
			fmt.Fprint(cmd.OutOrStdout(), formatted.Stdout)

			// If not --yes, prompt for confirmation
			if !yes {
				fmt.Fprint(cmd.OutOrStdout(), "\nProceed? [y/N]: ")
				reader := bufio.NewReader(cmd.InOrStdin())
				input, readErr := reader.ReadString('\n')
				if readErr != nil {
					return readErr
				}
				input = strings.TrimSpace(strings.ToLower(input))
				if input != "y" && input != "yes" {
					return nil
				}
			}

			// Second pass: execute removal
			result, err = cleanCmd.Run(cmd.Context(), cwd, twig.CleanOptions{
				Check:   false,
				Target:  target,
				Verbose: verbose,
				Force:   twig.WorktreeForceLevel(forceCount),
			})
			if err != nil {
				return err
			}

			formatted = result.Format(twig.FormatOptions{Verbose: verbose})
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
			git := twig.NewGitRunner(dir)
			branches, err := git.WorktreeListBranches(cmd.Context())
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
			verbosity, _ := cmd.Flags().GetCount("verbose")
			verbose := verbosity >= 1
			forceCount, _ := cmd.Flags().GetCount("force")
			check, _ := cmd.Flags().GetBool("check")

			// Create logger based on verbosity level
			log := twig.NewNopLogger()
			if verbosity >= 2 {
				handler := twig.NewCLIHandler(cmd.ErrOrStderr(), twig.VerbosityToLevel(verbosity))
				// Add command ID for log grouping
				idGen := twig.GenerateCommandID
				if o.commandIDGenerator != nil {
					idGen = o.commandIDGenerator
				}
				handlerWithID := handler.WithAttrs([]slog.Attr{
					twig.LogAttrKeyCmdID.Attr(idGen()),
				})
				log = slog.New(handlerWithID)
			}

			var removeCmd RemoveCommander
			if o.removeCommander != nil {
				removeCmd = o.removeCommander
			} else {
				removeCmd = twig.NewDefaultRemoveCommand(cfg, log)
			}
			var result twig.RemoveResult

			for _, branch := range args {
				wt, err := removeCmd.Run(cmd.Context(), branch, cwd, twig.RemoveOptions{
					Force: twig.WorktreeForceLevel(forceCount),
					Check: check,
				})
				if err != nil {
					wt.Branch = branch
					wt.Err = err
				}
				result.Removed = append(result.Removed, wt)
			}

			formatted := result.Format(twig.FormatOptions{Verbose: verbose})
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
	rootCmd.PersistentFlags().StringVarP(&dirFlag, "directory", "C", "", "Run as if twig was started in <path>")
	rootCmd.PersistentFlags().CountP("verbose", "v", "Enable verbose output (-v for verbose, -vv for debug)")

	addCmd.Flags().BoolP("sync", "s", false, "Sync uncommitted changes to new worktree")
	addCmd.Flags().StringP("carry", "c", "", "Move uncommitted changes (<branch>: from specified worktree)")
	addCmd.Flags().Lookup("carry").NoOptDefVal = carryFromCurrent
	addCmd.Flags().BoolP("quiet", "q", false, "Output only the worktree path")
	addCmd.Flags().String("source", "", "Source branch's worktree to use")
	addCmd.Flags().Bool("lock", false, "Lock the worktree after creation")
	addCmd.Flags().String("reason", "", "Reason for locking (requires --lock)")
	addCmd.Flags().StringArrayP("file", "F", nil, "File patterns to sync/carry (requires --sync or --carry)")
	addCmd.Flags().Bool("init-submodules", false, "Initialize submodules in new worktree")
	addCmd.RegisterFlagCompletionFunc("file", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		// Resolve target directory from -C flag
		dir, err := resolveCompletionDirectory(cmd)
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}

		ctx := cmd.Context()
		// If --source is specified, resolve to that worktree
		if source, _ := cmd.Flags().GetString("source"); source != "" {
			git := twig.NewGitRunner(dir)
			if sourceWT, findErr := git.WorktreeFindByBranch(ctx, source); findErr == nil {
				dir = sourceWT.Path
			}
		}

		// Get changed files from the target directory
		git := twig.NewGitRunner(dir)
		files, err := git.ChangedFiles(ctx)
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}

		// Filter by prefix
		var completions []string
		for _, file := range files {
			if strings.HasPrefix(file.Path, toComplete) {
				completions = append(completions, file.Path)
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
	removeCmd.Flags().Bool("check", false, "Show removal eligibility without making changes")
	rootCmd.AddCommand(removeCmd)

	initCmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize twig configuration",
		Long:  `Create a .twig/settings.toml configuration file in the current directory.`,
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

			var initCommand InitCommander
			if o.initCommander != nil {
				initCommand = o.initCommander
			} else {
				initCommand = twig.NewDefaultInitCommand()
			}
			result, err := initCommand.Run(cmd.Context(), cwd, twig.InitOptions{Force: force})
			if err != nil {
				return err
			}

			formatted := result.Format(twig.InitFormatOptions{})
			fmt.Fprint(cmd.OutOrStdout(), formatted.Stdout)
			return nil
		},
	}
	initCmd.Flags().BoolP("force", "f", false, "Overwrite existing configuration file")
	rootCmd.AddCommand(initCmd)

	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 1, ' ', 0)
			fmt.Fprintf(w, "version:\t%s\n", version)
			fmt.Fprintf(w, "commit:\t%s\n", commit)
			fmt.Fprintf(w, "date:\t%s\n", date)
			w.Flush()
		},
	}
	rootCmd.AddCommand(versionCmd)

	return rootCmd
}

var rootCmd = newRootCmd()

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(rootCmd.ErrOrStderr(), "twig:", err)
		os.Exit(1)
	}
}
