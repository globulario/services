// Command behavioral-backfill imports selected existing ai_memory.memories into
// behavioral-memory candidate objects (signals, claims, outcomes, and — only when
// fully specified — PROPOSED principles).
//
// It is SAFE BY DEFAULT: dry-run unless -apply is passed; always scoped by
// -project and -domain; deterministic + idempotent; never promotes; creates no
// schema (the behavioral_memory keyspace must already exist, created by the
// running ai-memory service).
//
// Usage:
//
//	behavioral-backfill -project globular-services -domain cluster_operator        # dry-run
//	behavioral-backfill -project globular-services -domain cluster_operator -apply  # write
//
// It writes through the behavioral STORE port (not the RPC) because safe
// idempotency requires get-before-write and "never overwrite a governed
// principle" — guarantees the write RPCs cannot offer from the client side.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	ai_memorypb "github.com/globulario/services/golang/ai_memory/ai_memorypb"
	backfill "github.com/globulario/services/golang/ai_memory/behavioral_backfill"
	"github.com/globulario/services/golang/ai_memory/behavioral/store"
	"github.com/globulario/services/golang/config"
	"github.com/gocql/gocql"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func main() {
	var (
		project   = flag.String("project", "", "behavioral-memory project (required)")
		domain    = flag.String("domain", "", "behavioral-memory domain, e.g. cluster_operator (required)")
		apply     = flag.Bool("apply", false, "actually write rows; default is dry-run")
		limit     = flag.Int("limit", 0, "max memories to scan (0 = source default)")
		since     = flag.Int64("since", 0, "only memories created at/after this unix time")
		types     = flag.String("types", "", "comma-separated memory types to include (default: convertible types)")
		tags      = flag.String("tags", "", "comma-separated tags the memory must carry")
		agent     = flag.String("agent", "", "only memories from this agent id")
		overwrite = flag.Bool("overwrite", false, "re-write existing PROPOSED rows (never promoted/revoked)")
	)
	flag.Parse()
	if *project == "" || *domain == "" {
		fmt.Fprintln(os.Stderr, "behavioral-backfill: -project and -domain are required")
		os.Exit(2)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	src, closeSrc, err := dialMemorySource()
	if err != nil {
		fmt.Fprintln(os.Stderr, "behavioral-backfill: connect ai-memory:", err)
		os.Exit(1)
	}
	defer closeSrc()

	st, closeStore, err := openBehavioralStore()
	if err != nil {
		fmt.Fprintln(os.Stderr, "behavioral-backfill: open store:", err)
		os.Exit(1)
	}
	defer closeStore()

	opts := backfill.Options{
		Project: *project, Domain: *domain, DryRun: !*apply, Limit: *limit, Since: *since,
		MemoryTypes: parseTypes(*types), Tags: csv(*tags), AgentID: *agent, Overwrite: *overwrite,
	}
	rep, err := backfill.Run(ctx, src, st, opts)
	if err != nil {
		fmt.Fprintln(os.Stderr, "behavioral-backfill:", err)
		os.Exit(1)
	}
	fmt.Print(rep.String())
}

func dialMemorySource() (backfill.MemorySource, func(), error) {
	addr := config.ResolveServiceAddr("ai_memory.AiMemoryService", "")
	if addr == "" {
		return nil, nil, fmt.Errorf("could not resolve ai_memory.AiMemoryService address")
	}
	tlsCfg, err := config.GetEtcdTLS()
	if err != nil {
		return nil, nil, fmt.Errorf("cluster mTLS unavailable: %w", err)
	}
	host := addr
	if i := strings.LastIndex(addr, ":"); i > 0 {
		host = addr[:i]
	}
	tlsCfg.ServerName = host
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(credentials.NewTLS(tlsCfg)))
	if err != nil {
		return nil, nil, err
	}
	return backfill.NewRPCSource(ai_memorypb.NewAiMemoryServiceClient(conn)), func() { conn.Close() }, nil
}

func openBehavioralStore() (store.Store, func(), error) {
	hosts, err := config.GetScyllaHosts()
	if err != nil || len(hosts) == 0 {
		return nil, nil, fmt.Errorf("scylla hosts unavailable: %w", err)
	}
	cluster := gocql.NewCluster(hosts...)
	cluster.Port = 9042
	cluster.Keyspace = "ai_memory" // behavioral queries are fully-qualified
	cluster.Consistency = gocql.Quorum
	if len(hosts) < 2 {
		cluster.Consistency = gocql.One
	}
	cluster.Timeout = 10 * time.Second
	cluster.ConnectTimeout = 10 * time.Second
	session, err := cluster.CreateSession()
	if err != nil {
		return nil, nil, err
	}
	return store.NewScyllaStore(session), session.Close, nil
}

func parseTypes(s string) []ai_memorypb.MemoryType {
	var out []ai_memorypb.MemoryType
	for _, t := range csv(s) {
		if v, ok := ai_memorypb.MemoryType_value[strings.ToUpper(t)]; ok {
			out = append(out, ai_memorypb.MemoryType(v))
		}
	}
	return out
}

func csv(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
