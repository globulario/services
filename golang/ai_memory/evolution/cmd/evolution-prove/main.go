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
		envelope     = flag.String("envelope", "", "ChangeEnvelope YAML/JSON path")
		workspace    = flag.String("workspace-dir", "", "exact candidate repository checkout")
		quickstart   = flag.String("quickstart-dir", "", "globulario/globular-quickstart checkout")
		keepArtifacts = flag.Bool("keep-artifacts", true, "preserve quickstart proof artifacts")
		verbose      = flag.Bool("verbose", false, "verbose quickstart probe output")
	)
	flag.Parse()
	if *envelope == "" || *workspace == "" || *quickstart == "" {
		fmt.Fprintln(os.Stderr, "evolution-prove: --envelope, --workspace-dir, and --quickstart-dir are required")
		os.Exit(2)
	}
	result, err := evolution.RunProofPlan(context.Background(), evolution.ProofPlanOptions{
		EnvelopePath:  *envelope,
		WorkspaceDir:  *workspace,
		QuickstartDir: *quickstart,
		KeepArtifacts: *keepArtifacts,
		Verbose:       *verbose,
	})
	_ = json.NewEncoder(os.Stdout).Encode(result)
	if err != nil {
		fmt.Fprintf(os.Stderr, "evolution-prove: %v\n", err)
		os.Exit(1)
	}
}
