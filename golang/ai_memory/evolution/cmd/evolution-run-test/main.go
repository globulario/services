package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/globulario/services/golang/ai_memory/evolution"
)

func main() {
	var (
		envelope  = flag.String("envelope", "", "ChangeEnvelope YAML/JSON path")
		workspace = flag.String("workspace-dir", "", "candidate repository checkout")
		testName  = flag.String("test", "", "declared test name")
	)
	runnerKey := flag.String("runner-key", "", "trusted proof-runner signing key; without it a run is recorded but cannot certify")
	flag.Parse()
	if *envelope == "" || *workspace == "" || *testName == "" {
		fmt.Fprintln(os.Stderr, "evolution-run-test: --envelope, --workspace-dir, and --test are required")
		os.Exit(2)
	}
	result, err := evolution.RunDeclaredTest(context.Background(), evolution.TestRunOptions{
		RunnerKeyPath: *runnerKey,
		EnvelopePath:  *envelope,
		WorkspaceDir:  *workspace,
		TestName:      *testName,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "evolution-run-test: %v\n", err)
		os.Exit(1)
	}
	_ = json.NewEncoder(os.Stdout).Encode(result)
	if result.ExitCode != 0 {
		os.Exit(result.ExitCode)
	}
}
