// Command behavioral-export-rdf projects Scylla-backed behavioral-memory rows
// into a deterministic N-Triples RDF document for semantic inspection and AWG
// alignment.
//
// It is READ-ONLY: full-table scans, no writes to Scylla and no writes to
// Oxigraph. ScyllaDB stays authoritative; this is a derived projection (rebuild
// from Scylla to repair). It never runs CheckAction / ResolveGovernedContext and
// is never part of service startup.
//
// Usage:
//
//	behavioral-export-rdf -project globular-services -domain cluster_operator -out behavioral.nt
//	behavioral-export-rdf -project globular-services > behavioral.nt
//
// Oxigraph loading is intentionally NOT part of this command (deferred to PR-7B);
// load the emitted file with the operator's own throwaway Oxigraph if desired.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/globulario/services/golang/ai_memory/behavioral/rdf"
	behavioral_rdf "github.com/globulario/services/golang/ai_memory/behavioral_rdf"
	"github.com/globulario/services/golang/config"
	"github.com/gocql/gocql"
)

func main() {
	var (
		project     = flag.String("project", "", "behavioral-memory project (required)")
		domain      = flag.String("domain", "", "optional domain filter, e.g. cluster_operator")
		out         = flag.String("out", "", "output file; empty = stdout")
		format      = flag.String("format", "ntriples", "output format (only 'ntriples' in PR-7)")
		since       = flag.Int64("since", 0, "optional: only rows at/after this unix time")
		inclBackfil = flag.Bool("include-backfilled", true, "include ai-memory-backfilled rows")
		inclGen     = flag.Bool("include-generated", true, "include compiler-generated rows")
	)
	flag.Parse()
	if *project == "" {
		fmt.Fprintln(os.Stderr, "behavioral-export-rdf: -project is required")
		os.Exit(2)
	}
	if *format != "ntriples" {
		fmt.Fprintln(os.Stderr, "behavioral-export-rdf: only -format ntriples is supported in PR-7")
		os.Exit(2)
	}

	session, err := openSession()
	if err != nil {
		fmt.Fprintln(os.Stderr, "behavioral-export-rdf: open scylla:", err)
		os.Exit(1)
	}
	defer session.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	reader := behavioral_rdf.NewScyllaReader(session)
	bundle, err := reader.Read(ctx, rdf.ReadOptions{
		Project: *project, Domain: *domain, Since: *since,
		IncludeBackfilled: *inclBackfil, IncludeGenerated: *inclGen,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "behavioral-export-rdf: read:", err)
		os.Exit(1)
	}

	doc, triples := rdf.ProjectTriples(bundle)
	if *out == "" {
		os.Stdout.Write(doc)
	} else if err := os.WriteFile(*out, doc, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "behavioral-export-rdf: write:", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "behavioral-export-rdf: %d triples projected (read-only; Scylla unchanged)\n", triples)
}

func openSession() (*gocql.Session, error) {
	hosts, err := config.GetScyllaHosts()
	if err != nil || len(hosts) == 0 {
		return nil, fmt.Errorf("scylla hosts unavailable: %w", err)
	}
	cluster := gocql.NewCluster(hosts...)
	cluster.Port = 9042
	cluster.Keyspace = "ai_memory" // behavioral queries are fully-qualified
	cluster.Consistency = gocql.Quorum
	if len(hosts) < 2 {
		cluster.Consistency = gocql.One
	}
	cluster.Timeout = 30 * time.Second
	cluster.ConnectTimeout = 10 * time.Second
	return cluster.CreateSession()
}
