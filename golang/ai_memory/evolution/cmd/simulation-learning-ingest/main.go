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
		file      = flag.String("file", "", "quickstart learning.json to ingest")
		project   = flag.String("project", "globular", "behavioral project")
		domain    = flag.String("domain", "cluster_operator", "behavioral domain")
		agentID   = flag.String("agent-id", "simulation_learning_ingestor", "audit actor id")
		clusterID = flag.String("cluster-id", "", "cluster id/scope when known")
		timeout   = flag.Duration("timeout", 5*time.Second, "per Behavioral Memory call timeout")
		dryRun    = flag.Bool("dry-run", false, "validate and print normalized learning without writing Behavioral Memory")
	)
	flag.Parse()
	if *file == "" {
		fmt.Fprintln(os.Stderr, "simulation-learning-ingest: --file is required")
		os.Exit(2)
	}

	data, err := os.ReadFile(*file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "simulation-learning-ingest: read %s: %v\n", *file, err)
		os.Exit(2)
	}
	learning, err := evolution.ParseSimulationLearning(data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "simulation-learning-ingest: rejected learning artifact: %v\n", err)
		os.Exit(2)
	}
	// Live ingestion mutates the behavioral store, so an artifact that cannot
	// name its proof occurrence is refused here. Dry-run stays inspectable so an
	// operator can still see what an unbound artifact contains.
	if !*dryRun {
		if err := learning.RequireOccurrenceBinding(); err != nil {
			fmt.Fprintf(os.Stderr, "simulation-learning-ingest: refusing unevidenced artifact: %v\n", err)
			os.Exit(2)
		}
	}
	if *dryRun {
		if err := json.NewEncoder(os.Stdout).Encode(learning); err != nil {
			fmt.Fprintf(os.Stderr, "simulation-learning-ingest: encode dry-run: %v\n", err)
			os.Exit(1)
		}
		return
	}

	recorder := evolution.NewRemoteRecorder(*timeout)
	defer func() { _ = recorder.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 4**timeout)
	defer cancel()
	result, err := (evolution.SimulationIngestor{
		Recorder:  recorder,
		Project:   *project,
		Domain:    behavioral.DomainRef(*domain),
		AgentID:   *agentID,
		ClusterID: *clusterID,
	}).Ingest(ctx, learning)
	if err != nil {
		fmt.Fprintf(os.Stderr, "simulation-learning-ingest: ingest failed: %v\n", err)
		os.Exit(1)
	}
	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		fmt.Fprintf(os.Stderr, "simulation-learning-ingest: encode result: %v\n", err)
		os.Exit(1)
	}
}
