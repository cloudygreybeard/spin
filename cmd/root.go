// Copyright 2026
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/cloudygreybeard/spin/internal/wheel"
	"github.com/spf13/cobra"
)

// Version information set via ldflags at build time.
var (
	Version = "dev"
	Commit  = "unknown"
	Date    = "unknown"
)

var rootCmd = &cobra.Command{
	Use:   "spin [items...]",
	Short: "Randomly pick an item from a list with a spinning text display",
	Long: `spin reads items from positional arguments, stdin, or a file and randomly
selects one, displaying the result with a spinning wheel that decelerates to a stop.

The animation models a physical prize wheel with Coulomb friction and
aerodynamic drag. The winner is determined by crypto/rand before the
display begins.

Use --monte-carlo to run repeated trials and display a live histogram of
the empirical distribution.`,
	Example: `  spin apple banana cherry
  echo "apple banana cherry" | spin
  spin -f items.txt
  spin -f items.txt -s ","
  spin --force 2.0 --friction 0.1 red green blue
  spin --start 3 a b c d e
  spin --fast-forward a b c d e
  spin --monte-carlo 1000 a b c d e
  spin --monte-carlo 500 --fast-forward a b c d e`,
	Args:         cobra.ArbitraryArgs,
	RunE:         runSpin,
	SilenceUsage: true,
}

func init() {
	rootCmd.Flags().StringP("file", "f", "", "path to input file")
	rootCmd.Flags().StringP("separator", "s", "", "item separator regex (default: whitespace)")
	rootCmd.Flags().String("start", "random", `starting position (1-indexed, wraps via modulo) or "random"`)
	rootCmd.Flags().Float64("force", 1.0, "spin force (higher = faster initial spin)")
	rootCmd.Flags().Float64("mass", 1.0, "wheel mass (higher = more inertia)")
	rootCmd.Flags().Float64("friction", 0.2, "coefficient of kinetic friction (higher = faster stop)")
	rootCmd.Flags().Float64("drag", 0.1, "aerodynamic drag coefficient (0 = pure Coulomb friction)")
	rootCmd.Flags().DurationP("max-delay", "m", 500*time.Millisecond, "delay threshold at which the wheel stops")
	rootCmd.Flags().IntP("monte-carlo", "n", 0, "run N trials and display a frequency histogram")
	rootCmd.Flags().BoolP("fast-forward", "q", false, "skip the wheel animation (print result only)")
	rootCmd.Flags().String("sort", "original", "histogram sort order: original, count[-desc|-asc], name[-asc|-desc]")
	rootCmd.Flags().Bool("rank", false, "shorthand for --sort=count (most frequent first)")
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

func runSpin(cmd *cobra.Command, args []string) error {
	items, err := readItems(cmd, args)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		return cmd.Help()
	}

	trials, _ := cmd.Flags().GetInt("monte-carlo")
	fastForward, _ := cmd.Flags().GetBool("fast-forward")

	if trials > 0 {
		return runMonteCarlo(cmd, items, trials, fastForward)
	}

	return runSingle(cmd, items, fastForward)
}

func runSingle(cmd *cobra.Command, items []string, fastForward bool) error {
	if fastForward {
		winner, err := wheel.Select(items)
		if err != nil {
			return err
		}
		fmt.Println(winner)
		return nil
	}

	cfg, err := buildConfig(cmd)
	if err != nil {
		return err
	}

	winner, err := wheel.Spin(os.Stderr, items, cfg)
	if err != nil {
		return err
	}

	fmt.Println(winner)
	return nil
}

func runMonteCarlo(cmd *cobra.Command, items []string, trials int, _ bool) error {
	sortOrder, err := parseSortOrder(cmd)
	if err != nil {
		return err
	}

	rendered := false
	progress := func(h *wheel.Histogram) {
		h.SetSort(sortOrder)
		_ = h.Render(os.Stderr, 70, rendered)
		rendered = true
	}

	hist, err := wheel.MonteCarlo(items, trials, progress)
	if err != nil {
		return err
	}

	hist.SetSort(sortOrder)
	if err := hist.Render(os.Stderr, 70, rendered); err != nil {
		return fmt.Errorf("rendering histogram: %w", err)
	}
	if err := hist.WriteTSV(os.Stdout); err != nil {
		return fmt.Errorf("writing results: %w", err)
	}
	return nil
}

func parseSortOrder(cmd *cobra.Command) (wheel.SortOrder, error) {
	rank, _ := cmd.Flags().GetBool("rank")
	if rank {
		return wheel.SortCountDesc, nil
	}
	s, _ := cmd.Flags().GetString("sort")
	return wheel.ParseSortOrder(s)
}

func buildConfig(cmd *cobra.Command) (wheel.Config, error) {
	startIdx, err := parseStart(cmd)
	if err != nil {
		return wheel.Config{}, err
	}

	force, _ := cmd.Flags().GetFloat64("force")
	mass, _ := cmd.Flags().GetFloat64("mass")
	friction, _ := cmd.Flags().GetFloat64("friction")
	drag, _ := cmd.Flags().GetFloat64("drag")
	maxDelay, _ := cmd.Flags().GetDuration("max-delay")

	return wheel.Config{
		Force:    force,
		Mass:     mass,
		Friction: friction,
		Drag:     drag,
		MaxDelay: maxDelay,
		Start:    startIdx,
	}, nil
}

func parseStart(cmd *cobra.Command) (int, error) {
	s, _ := cmd.Flags().GetString("start")
	if s == "random" {
		return -1, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("invalid start position %q: must be a positive integer or \"random\"", s)
	}
	if n < 1 {
		return 0, fmt.Errorf("start position must be >= 1, got %d", n)
	}
	return n - 1, nil
}

func readItems(cmd *cobra.Command, args []string) ([]string, error) {
	filePath, _ := cmd.Flags().GetString("file")
	separator, _ := cmd.Flags().GetString("separator")

	if filePath != "" {
		data, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("reading file: %w", err)
		}
		return wheel.ParseItems(string(data), separator)
	}

	if len(args) > 0 {
		return args, nil
	}

	stat, err := os.Stdin.Stat()
	if err != nil {
		return nil, fmt.Errorf("checking stdin: %w", err)
	}
	if (stat.Mode() & os.ModeCharDevice) != 0 {
		return nil, nil
	}

	scanner := bufio.NewScanner(os.Stdin)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading stdin: %w", err)
	}
	return wheel.ParseItems(strings.Join(lines, "\n"), separator)
}
