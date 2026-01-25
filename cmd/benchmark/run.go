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
	"small":  {1000, 100, 10},
	"medium": {5000, 500, 50},
	"large":  {10000, 1000, 100},
}

var (
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
)

var runCmd = &cobra.Command{
	Use:   "run <benchmark> [scale]",
	Short: "Run benchmarks using hyperfine",
	Long: `Run twig benchmarks using hyperfine.

Benchmarks:
  list        Benchmark twig list
  add         Benchmark twig add
  remove      Benchmark twig remove
  clean       Benchmark twig clean --yes
  all         Run all benchmarks

Scales (preset):
  small       1000 files, 100 commits, 10 worktrees
  medium      5000 files, 500 commits, 50 worktrees
  large       10000 files, 1000 commits, 100 worktrees

Custom scale flags override preset values when specified.

Examples:
  benchmark run list small
  benchmark run clean large
  benchmark run all medium
  benchmark run list small --files=2000
  benchmark run add small --worktrees=20
  benchmark run all small --files=500 --commits=50 --worktrees=5

  # Export results
  benchmark run list small --export-json=results.json
  benchmark run all small --export-markdown=results.md

  # Compare twig vs git
  benchmark run list small --compare

  # Keep benchmark directory for inspection
  benchmark run list small --keep --output-dir=/tmp/my-bench`,
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		benchmark := args[0]

		var s scale
		if len(args) >= 2 {
			scaleName := args[1]
			var ok bool
			s, ok = scales[scaleName]
			if !ok {
				return fmt.Errorf("unknown scale '%s' (available: small, medium, large)", scaleName)
			}
		} else {
			s = scales["small"]
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

		if err := checkDeps(); err != nil {
			return err
		}

		outputDir := runOutputDir
		if outputDir == "" {
			outputDir = benchBase
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

		switch benchmark {
		case "list":
			return benchList(opts)
		case "add":
			return benchAdd(opts)
		case "remove":
			return benchRemove(opts)
		case "clean":
			return benchClean(opts)
		case "all":
			return benchAll(opts)
		default:
			return fmt.Errorf("unknown benchmark '%s' (available: list, add, remove, clean, all)", benchmark)
		}
	},
}

func checkDeps() error {
	if _, err := exec.LookPath("hyperfine"); err != nil {
		return fmt.Errorf("hyperfine is required but not installed\n  Install with: brew install hyperfine")
	}
	if _, err := exec.LookPath("twig"); err != nil {
		return fmt.Errorf("twig is required but not installed\n  Install with: go install github.com/708u/twig/cmd/twig@latest")
	}
	return nil
}

func benchList(opts *benchOpts) error {
	fmt.Println("\n=== Benchmark: twig list ===")

	if err := setupRepo(opts.outputDir, opts.scale.files, opts.scale.commits, opts.scale.worktrees, false); err != nil {
		return err
	}

	args := opts.hyperfineArgs(3, 20)
	args = append(args, fmt.Sprintf("twig list -C %s/main", opts.outputDir))

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
		"--prepare", fmt.Sprintf("twig remove bench/bench-test -C %s/main --force 2>/dev/null || true", opts.outputDir),
		fmt.Sprintf("twig add bench/bench-test -C %s/main", opts.outputDir),
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
		"--prepare", fmt.Sprintf("twig add bench/bench-test -C %s/main 2>/dev/null || true", opts.outputDir),
		fmt.Sprintf("twig remove bench/bench-test -C %s/main --force", opts.outputDir),
	)

	return runHyperfine(args...)
}

func benchClean(opts *benchOpts) error {
	fmt.Println("\n=== Benchmark: twig clean ===")

	if err := setupRepo(opts.outputDir, opts.scale.files, opts.scale.commits, opts.scale.worktrees, true); err != nil {
		return err
	}

	prepareCmd := fmt.Sprintf("go run ./cmd/benchmark setup --files=%d --commits=%d --worktrees=%d --merged %s 2>/dev/null",
		opts.scale.files, opts.scale.commits, opts.scale.worktrees, opts.outputDir)

	args := opts.hyperfineArgs(1, 5)
	args = append(args,
		"--prepare", prepareCmd,
		fmt.Sprintf("twig clean --yes -C %s/main", opts.outputDir),
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
	runCmd.Flags().IntVar(&runFiles, "files", 0, "Override number of files (0 = use scale default)")
	runCmd.Flags().IntVar(&runCommits, "commits", 0, "Override number of commits (0 = use scale default)")
	runCmd.Flags().IntVar(&runWorktrees, "worktrees", 0, "Override number of worktrees (0 = use scale default)")
	runCmd.Flags().IntVar(&runWarmup, "warmup", 0, "Number of warmup runs (0 = use benchmark default)")
	runCmd.Flags().IntVar(&runRuns, "runs", 0, "Number of benchmark runs (0 = use benchmark default)")
	runCmd.Flags().StringVar(&runOutputDir, "output-dir", "", "Output directory for benchmark repository (default: /tmp/twig-bench)")
	runCmd.Flags().BoolVar(&runKeep, "keep", false, "Keep benchmark directory after completion")
	runCmd.Flags().StringVar(&runExportJSON, "export-json", "", "Export results to JSON file")
	runCmd.Flags().StringVar(&runExportMarkdown, "export-markdown", "", "Export results to Markdown file")
	runCmd.Flags().BoolVar(&runCompare, "compare", false, "Compare twig commands with git equivalents")
}
