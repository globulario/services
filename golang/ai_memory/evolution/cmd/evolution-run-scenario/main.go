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

func main() {
	var (
		envelopePath  = flag.String("envelope", "", "ChangeEnvelope YAML/JSON path")
		quickstartDir = flag.String("quickstart-dir", "", "globulario/globular-quickstart checkout")
		scenario      = flag.String("scenario", "", "scenario path relative to quickstart checkout")
		scenarioName  = flag.String("scenario-name", "", "required-scenario name this run answers for; defaults to the scenario file name")
		keepArtifacts = flag.Bool("keep-artifacts", true, "preserve quickstart evidence artifacts")
		verbose       = flag.Bool("verbose", false, "verbose quickstart probe output")
		ingest        = flag.Bool("ingest-learning", false, "ingest resulting learning.json into Behavioral Memory")
		clusterID     = flag.String("cluster-id", "", "cluster id/scope attached to behavioral observations")
	)
	runnerKey := flag.String("runner-key", "", "trusted proof-runner signing key; without it a run is recorded but cannot certify")
	flag.Parse()
	if *envelopePath == "" || *quickstartDir == "" || *scenario == "" {
		fmt.Fprintln(os.Stderr, "evolution-run-scenario: --envelope, --quickstart-dir, and --scenario are required")
		os.Exit(2)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result, err := evolution.RunQuickstartScenario(ctx, evolution.QuickstartRunOptions{
		RunnerKeyPath: *runnerKey,
		QuickstartDir: *quickstartDir,
		Scenario:      *scenario,
		ScenarioName:  *scenarioName,
		EnvelopePath:  *envelopePath,
		KeepArtifacts: *keepArtifacts,
		Verbose:       *verbose,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "evolution-run-scenario: proof orchestration failed: %v\n", err)
		os.Exit(1)
	}

	response := map[string]interface{}{
		"exit_code":         result.ExitCode,
		"proof":             result.Proof,
		"proof_path":        result.ProofPath,
		"learning_path":     result.LearningPath,
		"marked_proven":     result.MarkedProven,
		"learning_ingested": false,
	}

	learningDegraded := false
	if *ingest {
		data, readErr := os.ReadFile(result.LearningPath)
		if readErr != nil {
			fmt.Fprintf(os.Stderr, "evolution-run-scenario: learning artifact unavailable: %v\n", readErr)
			learningDegraded = true
		} else {
			learning, parseErr := evolution.ParseSimulationLearning(data)
			if parseErr != nil {
				fmt.Fprintf(os.Stderr, "evolution-run-scenario: learning artifact rejected: %v\n", parseErr)
				learningDegraded = true
			} else if bindErr := learning.RequireBoundTo(result.ChangeID, result.Proof); bindErr != nil {
				// Recorded as learning from this proof run, so it must be this
				// proof occurrence. Never repaired from the proof.
				fmt.Fprintf(os.Stderr, "evolution-run-scenario: learning artifact not bound to this proof: %v\n", bindErr)
				learningDegraded = true
			} else {
				recorder := evolution.NewRemoteRecorder(5 * time.Second)
				ingestCtx, ingestCancel := context.WithTimeout(context.Background(), 30*time.Second)
				ingestResult, ingestErr := (evolution.SimulationIngestor{
					Recorder:  recorder,
					Project:   "globular",
					Domain:    behavioral.DomainRef("cluster_operator"),
					AgentID:   "evolution_runner.simulation_learning",
					ClusterID: *clusterID,
				}).Ingest(ingestCtx, learning)
				ingestCancel()
				_ = recorder.Close()
				if ingestErr != nil {
					fmt.Fprintf(os.Stderr, "evolution-run-scenario: learning ingestion degraded: %v\n", ingestErr)
					learningDegraded = true
				} else {
					response["learning_ingested"] = true
					response["learning_result"] = ingestResult
				}
			}
		}
	}

	_ = json.NewEncoder(os.Stdout).Encode(response)
	if result.ExitCode != 0 {
		os.Exit(result.ExitCode)
	}
	// A harness can exit zero and still produce a record that can never certify —
	// FAIL, UNSUPPORTED, ineligible, or missing its mandatory evidence artifact.
	// Ask the same predicate validation asks, so automation cannot read success
	// from the exit status for a run that proved nothing.
	if err := result.CertifiesRequirement(*scenarioName); err != nil {
		fmt.Fprintf(os.Stderr, "evolution-run-scenario: %v\n", err)
		os.Exit(1)
	}
	if learningDegraded {
		// Proof remains valid, but autonomous learning did not complete. Keep this
		// visible to callers instead of silently treating the lifecycle as closed.
		os.Exit(3)
	}
}
