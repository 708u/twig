package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

const benchBase = "/tmp/twig-bench"

type scale struct {
	files     int
	commits   int
	worktrees int
}

type benchOpts struct {
	scale          scale
	outputDir      string
	warmup         int
	runs           int
	exportJSON     string
	exportMarkdown string
	compare        bool
}

var scales = map[string]scale{
	"small":  {500, 1000, 5},
	"medium": {2000, 5000, 10},
	"large":  {10000, 20000, 20},
}

// Shared flags for all run subcommands
var (
	runScale          string
	runFiles          int
	runCommits        int
	runWorktrees      int
	runWarmup         int
	runRuns           int
	runOutputDir      string
	runKeep           bool
	runExportJSON     string
	runExportMarkdown string
	runCompare        bool
	runTwigBin        string // twig binary path flag
	runBenchmarkBin   string // benchmark binary path flag
	twigBin           string // resolved twig binary path
	benchmarkBin      string // resolved benchmark binary path
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run benchmarks using hyperfine",
	Long: `Run twig benchmarks using hyperfine.

Use subcommands to run specific benchmarks:
  benchmark run list
  benchmark run add
  benchmark run remove
  benchmark run clean
  benchmark run all

Scale presets (--scale):
  small       500 files, 1000 commits, 5 worktrees (default)
  medium      2000 files, 5000 commits, 10 worktrees
  large       10000 files, 20000 commits, 20 worktrees

Use --scale-files, --scale-commits, --scale-worktrees to override preset values.`,
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "Benchmark twig list",
	Args:  cobra.NoArgs,
	RunE:  runBenchmark(benchList),
}

var addCmd = &cobra.Command{
	Use:   "add",
	Short: "Benchmark twig add",
	Args:  cobra.NoArgs,
	RunE:  runBenchmark(benchAdd),
}

var removeCmd = &cobra.Command{
	Use:   "remove",
	Short: "Benchmark twig remove",
	Args:  cobra.NoArgs,
	RunE:  runBenchmark(benchRemove),
}

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Benchmark twig clean --yes",
	Args:  cobra.NoArgs,
	RunE:  runBenchmark(benchClean),
}

var allCmd = &cobra.Command{
	Use:   "all",
	Short: "Run all benchmarks",
	Args:  cobra.NoArgs,
	RunE:  runBenchmark(benchAll),
}

// runBenchmark returns a RunE function that executes the given benchmark.
func runBenchmark(bench func(*benchOpts) error) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		s, ok := scales[runScale]
		if !ok {
			return fmt.Errorf("unknown scale '%s' (available: small, medium, large)", runScale)
		}

		if runFiles > 0 {
			s.files = runFiles
		}
		if runCommits > 0 {
			s.commits = runCommits
		}
		if runWorktrees > 0 {
			s.worktrees = runWorktrees
		}

		outputDir := runOutputDir
		if outputDir == "" {
			outputDir = benchBase
		}

		if err := checkDeps(); err != nil {
			return err
		}

		if !runKeep {
			defer func() {
				fmt.Println("\nCleaning up benchmark directory...")
				_ = os.RemoveAll(outputDir)
			}()
		}

		opts := &benchOpts{
			scale:          s,
			outputDir:      outputDir,
			warmup:         runWarmup,
			runs:           runRuns,
			exportJSON:     runExportJSON,
			exportMarkdown: runExportMarkdown,
			compare:        runCompare,
		}

		return bench(opts)
	}
}

func checkDeps() error {
	if _, err := exec.LookPath("hyperfine"); err != nil {
		return fmt.Errorf("hyperfine is required but not installed\n  Install with: brew install hyperfine")
	}

	if runTwigBin == "" {
		path, err := exec.LookPath("twig")
		if err != nil {
			return fmt.Errorf("twig is required but not installed\n  Install with: go install github.com/708u/twig/cmd/twig@latest")
		}
		twigBin = path
	} else {
		if _, err := os.Stat(runTwigBin); err != nil {
			return fmt.Errorf("twig binary not found: %s", runTwigBin)
		}
		twigBin = runTwigBin
	}

	// Resolve benchmark binary for --prepare commands
	if runBenchmarkBin == "" {
		binDir := "/tmp/twig-bench-bin"
		benchmarkBin = binDir + "/benchmark"
		if err := os.MkdirAll(binDir, 0755); err != nil {
			return fmt.Errorf("failed to create bin directory: %w", err)
		}
		cmd := exec.Command("go", "build", "-o", benchmarkBin, "./cmd/benchmark")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to build benchmark tool: %w", err)
		}
	} else {
		if _, err := os.Stat(runBenchmarkBin); err != nil {
			return fmt.Errorf("benchmark binary not found: %s", runBenchmarkBin)
		}
		benchmarkBin = runBenchmarkBin
	}

	return nil
}

func benchList(opts *benchOpts) error {
	fmt.Println("\n=== Benchmark: twig list ===")

	if err := setupRepo(opts.outputDir, opts.scale.files, opts.scale.commits, opts.scale.worktrees, false); err != nil {
		return err
	}

	args := opts.hyperfineArgs(3, 20)
	args = append(args, fmt.Sprintf("%s list -C %s/main", twigBin, opts.outputDir))

	if opts.compare {
		args = append(args, fmt.Sprintf("git -C %s/main worktree list", opts.outputDir))
	}

	return runHyperfine(args...)
}

func benchAdd(opts *benchOpts) error {
	fmt.Println("\n=== Benchmark: twig add ===")

	if err := setupRepo(opts.outputDir, opts.scale.files, opts.scale.commits, opts.scale.worktrees, false); err != nil {
		return err
	}

	args := opts.hyperfineArgs(1, 10)
	args = append(args,
		"--prepare", fmt.Sprintf("%s remove bench/bench-test -C %s/main --force 2>/dev/null || true", twigBin, opts.outputDir),
		fmt.Sprintf("%s add bench/bench-test -C %s/main", twigBin, opts.outputDir),
	)

	return runHyperfine(args...)
}

func benchRemove(opts *benchOpts) error {
	fmt.Println("\n=== Benchmark: twig remove ===")

	if err := setupRepo(opts.outputDir, opts.scale.files, opts.scale.commits, opts.scale.worktrees, false); err != nil {
		return err
	}

	args := opts.hyperfineArgs(1, 10)
	args = append(args,
		"--prepare", fmt.Sprintf("%s add bench/bench-test -C %s/main 2>/dev/null || true", twigBin, opts.outputDir),
		fmt.Sprintf("%s remove bench/bench-test -C %s/main --force", twigBin, opts.outputDir),
	)

	return runHyperfine(args...)
}

func benchClean(opts *benchOpts) error {
	fmt.Println("\n=== Benchmark: twig clean ===")

	if err := setupRepo(opts.outputDir, opts.scale.files, opts.scale.commits, opts.scale.worktrees, true); err != nil {
		return err
	}

	prepareCmd := fmt.Sprintf("%s setup --files=%d --commits=%d --worktrees=%d --merged %s 2>/dev/null",
		benchmarkBin, opts.scale.files, opts.scale.commits, opts.scale.worktrees, opts.outputDir)

	args := opts.hyperfineArgs(1, 5)
	args = append(args,
		"--prepare", prepareCmd,
		fmt.Sprintf("%s clean --yes -C %s/main", twigBin, opts.outputDir),
	)

	return runHyperfine(args...)
}

func benchAll(opts *benchOpts) error {
	benchmarks := []func(*benchOpts) error{benchList, benchAdd, benchRemove, benchClean}
	for _, bench := range benchmarks {
		if err := bench(opts); err != nil {
			return err
		}
	}
	fmt.Println("\nAll benchmarks completed.")
	return nil
}

func (o *benchOpts) hyperfineArgs(defaultWarmup, defaultRuns int) []string {
	warmup := defaultWarmup
	if o.warmup > 0 {
		warmup = o.warmup
	}

	runs := defaultRuns
	if o.runs > 0 {
		runs = o.runs
	}

	args := []string{
		"--warmup", fmt.Sprintf("%d", warmup),
		"--runs", fmt.Sprintf("%d", runs),
	}

	if o.exportJSON != "" {
		args = append(args, "--export-json", o.exportJSON)
	}
	if o.exportMarkdown != "" {
		args = append(args, "--export-markdown", o.exportMarkdown)
	}

	return args
}

func runHyperfine(args ...string) error {
	cmd := exec.Command("hyperfine", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func init() {
	// Register shared flags on parent command
	runCmd.PersistentFlags().StringVar(&runScale, "scale", "small", "Scale preset (small, medium, large)")
	runCmd.PersistentFlags().IntVar(&runFiles, "scale-files", 0, "Override number of files")
	runCmd.PersistentFlags().IntVar(&runCommits, "scale-commits", 0, "Override number of commits")
	runCmd.PersistentFlags().IntVar(&runWorktrees, "scale-worktrees", 0, "Override number of worktrees")
	runCmd.PersistentFlags().IntVar(&runWarmup, "warmup", 0, "Number of warmup runs (0 = use benchmark default)")
	runCmd.PersistentFlags().IntVar(&runRuns, "runs", 0, "Number of benchmark runs (0 = use benchmark default)")
	runCmd.PersistentFlags().StringVar(&runOutputDir, "output-dir", "", "Output directory for benchmark repository (default: /tmp/twig-bench)")
	runCmd.PersistentFlags().BoolVar(&runKeep, "keep", false, "Keep benchmark directory after completion")
	runCmd.PersistentFlags().StringVar(&runExportJSON, "export-json", "", "Export results to JSON file")
	runCmd.PersistentFlags().StringVar(&runExportMarkdown, "export-markdown", "", "Export results to Markdown file")
	runCmd.PersistentFlags().BoolVar(&runCompare, "compare", false, "Compare twig commands with git equivalents")
	runCmd.PersistentFlags().StringVar(&runTwigBin, "twig-bin", "", "Path to twig binary (default: use from PATH)")
	runCmd.PersistentFlags().StringVar(&runBenchmarkBin, "benchmark-bin", "", "Path to benchmark binary (default: build from source)")

	// Register subcommands
	runCmd.AddCommand(listCmd)
	runCmd.AddCommand(addCmd)
	runCmd.AddCommand(removeCmd)
	runCmd.AddCommand(cleanCmd)
	runCmd.AddCommand(allCmd)
}
