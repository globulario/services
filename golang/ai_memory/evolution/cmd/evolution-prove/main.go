package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	behavioral "github.com/globulario/services/golang/ai_memory/behavioral/api"
	"github.com/globulario/services/golang/ai_memory/evolution"
)

type output struct {
	ProofPlan      evolution.ProofPlanResult          `json:"proof_plan"`
	Learning       []evolution.SimulationIngestResult `json:"learning,omitempty"`
	LearningErrors []string                           `json:"learning_errors,omitempty"`
}

func main() {
	var (
		envelope       = flag.String("envelope", "", "ChangeEnvelope YAML/JSON path")
		workspace      = flag.String("workspace-dir", "", "exact candidate repository checkout")
		quickstart     = flag.String("quickstart-dir", "", "globulario/globular-quickstart checkout")
		keepArtifacts  = flag.Bool("keep-artifacts", true, "preserve quickstart proof artifacts")
		verbose        = flag.Bool("verbose", false, "verbose quickstart probe output")
		ingestLearning = flag.Bool("ingest-learning", false, "ingest learning from every scenario actually executed")
		clusterID      = flag.String("cluster-id", "", "cluster id/scope attached to behavioral observations")
	)
	runnerKey := flag.String("runner-key", "", "trusted proof-runner signing key; without it a run is recorded but cannot certify")
	flag.Parse()
	if *envelope == "" || *workspace == "" || *quickstart == "" {
		fmt.Fprintln(os.Stderr, "evolution-prove: --envelope, --workspace-dir, and --quickstart-dir are required")
		os.Exit(2)
	}

	proofResult, proofErr := evolution.RunProofPlan(context.Background(), evolution.ProofPlanOptions{
		RunnerKeyPath: *runnerKey,
		EnvelopePath:  *envelope,
		WorkspaceDir:  *workspace,
		QuickstartDir: *quickstart,
		KeepArtifacts: *keepArtifacts,
		Verbose:       *verbose,
	})
	response := output{ProofPlan: proofResult}

	if *ingestLearning && len(proofResult.Scenarios) > 0 {
		recorder := evolution.NewRemoteRecorder(5 * time.Second)
		for _, scenario := range proofResult.Scenarios {
			if scenario.LearningPath == "" {
				continue
			}
			data, err := os.ReadFile(scenario.LearningPath)
			if err != nil {
				response.LearningErrors = append(response.LearningErrors,
					fmt.Sprintf("%s: read learning: %v", scenario.Proof.Scenario, err))
				continue
			}
			learning, err := evolution.ParseSimulationLearning(data)
			if err != nil {
				response.LearningErrors = append(response.LearningErrors,
					fmt.Sprintf("%s: reject learning: %v", scenario.Proof.Scenario, err))
				continue
			}
			// The learning artifact is about to be recorded as learning from
			// this proof run, so it must be this proof occurrence. An artifact
			// that cannot say so is refused rather than adopted.
			if err := learning.RequireBoundTo(scenario.ChangeID, scenario.Proof); err != nil {
				response.LearningErrors = append(response.LearningErrors,
					fmt.Sprintf("%s: reject learning: %v", scenario.Proof.Scenario, err))
				continue
			}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			ingested, err := (evolution.SimulationIngestor{
				Recorder:  recorder,
				Project:   "globular",
				Domain:    behavioral.DomainRef("cluster_operator"),
				AgentID:   "evolution_prove.simulation_learning",
				ClusterID: *clusterID,
			}).Ingest(ctx, learning)
			cancel()
			if err != nil {
				response.LearningErrors = append(response.LearningErrors,
					fmt.Sprintf("%s: ingest learning: %v", scenario.Proof.Scenario, err))
				continue
			}
			response.Learning = append(response.Learning, ingested)
		}
		_ = recorder.Close()
	}

	_ = json.NewEncoder(os.Stdout).Encode(response)
	if proofErr != nil {
		fmt.Fprintf(os.Stderr, "evolution-prove: %v\n", proofErr)
		os.Exit(1)
	}
	if len(response.LearningErrors) > 0 {
		// Proof and learning are separate verdicts. A proof may remain valid while
		// background learning is visibly degraded and retriable.
		os.Exit(3)
	}
}
