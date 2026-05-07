// go-test-to-awareness converts `go test -json` output into the awareness
// CI test results schema expected by awareness.self_review (test_results_file).
//
// Usage:
//
//	go test -json ./awareness/... | go-test-to-awareness > .awareness/test-results.json
//	go-test-to-awareness -input results.jsonl -output .awareness/test-results.json
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"
)

// goTestEvent is a single line from `go test -json` output.
type goTestEvent struct {
	Action  string  `json:"Action"`
	Package string  `json:"Package"`
	Test    string  `json:"Test"`
	Elapsed float64 `json:"Elapsed"`
	Output  string  `json:"Output"`
}

type outputTest struct {
	Name       string `json:"name"`
	Package    string `json:"package"`
	Status     string `json:"status"` // passed | failed | skipped
	DurationMs int    `json:"duration_ms"`
}

type outputFile struct {
	Command      string       `json:"command"`
	StartedAt    string       `json:"started_at"`
	FinishedAt   string       `json:"finished_at"`
	Passed       bool         `json:"passed"`
	Packages     int          `json:"packages"`
	Tests        []outputTest `json:"tests"`
	FailedTests  []string     `json:"failed_tests"`
	SkippedTests []string     `json:"skipped_tests"`
}

func main() {
	inputPath := flag.String("input", "-", "Path to go test -json output (- for stdin)")
	outputPath := flag.String("output", "-", "Path to write awareness test-results JSON (- for stdout)")
	command := flag.String("command", "go test -json ./awareness/...", "Original go test command")
	flag.Parse()

	var scanner *bufio.Scanner
	if *inputPath == "-" {
		scanner = bufio.NewScanner(os.Stdin)
	} else {
		f, err := os.Open(*inputPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error opening input: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()
		scanner = bufio.NewScanner(f)
	}

	startedAt := time.Now().UTC()
	pkgSet := make(map[string]bool)
	testResults := make(map[string]*outputTest)
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
			// Package-level event.
			if ev.Action == "fail" {
				packageFailed = true
			}
			continue
		}
		key := ev.Package + "/" + ev.Test
		switch ev.Action {
		case "run":
			testResults[key] = &outputTest{
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

	var tests []outputTest
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

	out := outputFile{
		Command:      *command,
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
		fmt.Fprintf(os.Stderr, "error marshaling output: %v\n", err)
		os.Exit(1)
	}

	if *outputPath == "-" {
		fmt.Println(string(data))
	} else {
		if err := os.MkdirAll(dirOf(*outputPath), 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "error creating output dir: %v\n", err)
			os.Exit(1)
		}
		if err := os.WriteFile(*outputPath, data, 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "error writing output: %v\n", err)
			os.Exit(1)
		}
	}
}

func dirOf(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == '\\' {
			return path[:i]
		}
	}
	return "."
}
