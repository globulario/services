package main

// awareness_test_results_cmd.go: convert `go test -json` output to awareness CI schema.
//
// Usage:
//
//	go test -json ./awareness/... | globular awareness test-results
//	globular awareness test-results --input results.jsonl --output .awareness/test-results.json

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
)

var testResultsCfg = struct {
	inputPath  string
	outputPath string
	command    string
}{}

// goTestEvent is a single line from `go test -json` output.
type goTestEvent struct {
	Action  string  `json:"Action"`
	Package string  `json:"Package"`
	Test    string  `json:"Test"`
	Elapsed float64 `json:"Elapsed"`
	Output  string  `json:"Output"`
}

type awarenessTestOutput struct {
	Name       string `json:"name"`
	Package    string `json:"package"`
	Status     string `json:"status"` // passed | failed | skipped
	DurationMs int    `json:"duration_ms"`
}

type awarenessTestResultsFile struct {
	Command      string                `json:"command"`
	StartedAt    string                `json:"started_at"`
	FinishedAt   string                `json:"finished_at"`
	Passed       bool                  `json:"passed"`
	Packages     int                   `json:"packages"`
	Tests        []awarenessTestOutput `json:"tests"`
	FailedTests  []string              `json:"failed_tests"`
	SkippedTests []string              `json:"skipped_tests"`
}

var awarenessTestResultsCmd = &cobra.Command{
	Use:   "test-results",
	Short: "Convert `go test -json` output to the awareness CI test-results schema",
	Long: `Reads go test -json output and writes .awareness/test-results.json,
which is consumed by 'globular awareness ci-check' to upgrade verification status.

Example:
  go test -json ./awareness/... | globular awareness test-results`,
	SilenceErrors: true,
	SilenceUsage:  true,
	RunE: func(cmd *cobra.Command, args []string) error {
		var scanner *bufio.Scanner
		if testResultsCfg.inputPath == "-" {
			scanner = bufio.NewScanner(os.Stdin)
		} else {
			f, err := os.Open(testResultsCfg.inputPath)
			if err != nil {
				return fmt.Errorf("open input: %w", err)
			}
			defer f.Close()
			scanner = bufio.NewScanner(f)
		}

		startedAt := time.Now().UTC()
		pkgSet := make(map[string]bool)
		testResults := make(map[string]*awarenessTestOutput)
		packageFailed := false

		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}
			var ev goTestEvent
			if err := json.Unmarshal(line, &ev); err != nil {
				continue
			}
			if ev.Package != "" {
				pkgSet[ev.Package] = true
			}
			if ev.Test == "" {
				if ev.Action == "fail" {
					packageFailed = true
				}
				continue
			}
			key := ev.Package + "/" + ev.Test
			switch ev.Action {
			case "run":
				testResults[key] = &awarenessTestOutput{
					Name:    ev.Test,
					Package: ev.Package,
					Status:  "running",
				}
			case "pass":
				if r := testResults[key]; r != nil {
					r.Status = "passed"
					r.DurationMs = int(ev.Elapsed * 1000)
				}
			case "fail":
				if r := testResults[key]; r != nil {
					r.Status = "failed"
					r.DurationMs = int(ev.Elapsed * 1000)
				}
			case "skip":
				if r := testResults[key]; r != nil {
					r.Status = "skipped"
					r.DurationMs = int(ev.Elapsed * 1000)
				}
			}
		}

		finishedAt := time.Now().UTC()

		var tests []awarenessTestOutput
		var failed, skipped []string
		for _, t := range testResults {
			tests = append(tests, *t)
			switch t.Status {
			case "failed":
				failed = append(failed, t.Name)
			case "skipped":
				skipped = append(skipped, t.Name)
			}
		}
		if failed == nil {
			failed = []string{}
		}
		if skipped == nil {
			skipped = []string{}
		}

		out := awarenessTestResultsFile{
			Command:      testResultsCfg.command,
			StartedAt:    startedAt.Format(time.RFC3339),
			FinishedAt:   finishedAt.Format(time.RFC3339),
			Passed:       !packageFailed && len(failed) == 0,
			Packages:     len(pkgSet),
			Tests:        tests,
			FailedTests:  failed,
			SkippedTests: skipped,
		}

		data, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal output: %w", err)
		}

		if testResultsCfg.outputPath == "-" {
			fmt.Println(string(data))
			return nil
		}

		dir := dirOf(testResultsCfg.outputPath)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create output dir: %w", err)
		}
		return os.WriteFile(testResultsCfg.outputPath, data, 0o644)
	},
}

func dirOf(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == '\\' {
			return path[:i]
		}
	}
	return "."
}

func init() {
	awarenessTestResultsCmd.Flags().StringVar(&testResultsCfg.inputPath, "input", "-", "Path to go test -json output (- for stdin)")
	awarenessTestResultsCmd.Flags().StringVar(&testResultsCfg.outputPath, "output", "-", "Path to write awareness test-results JSON (- for stdout)")
	awarenessTestResultsCmd.Flags().StringVar(&testResultsCfg.command, "command", "go test -json ./awareness/...", "Original go test command")

	awarenessCmd.AddCommand(awarenessTestResultsCmd)
}
