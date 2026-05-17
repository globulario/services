# awareness

Awareness is the AI-assisted observability layer for Globular. It builds a knowledge graph
from static analysis, YAML documentation, and live cluster signals, then exposes that graph
through MCP tools so AI agents can reason about cluster state, detect drift, and learn from
past failures.

---

## Two-module boundary

The code lives in **two separate Go modules** with different dependency profiles:

| Module | Path | Purpose |
|--------|------|---------|
| `github.com/globulario/awareness` | `github.com/globulario/awareness/` (standalone repo) | Lean core: graph, preflight, learning, session oracle, failure graph. **Zero Globular dependencies.** Usable outside Globular. |
| `github.com/globulario/services/golang/awareness` | this directory | Heavy integration: extractors (cluster state, gRPC, ScyllaDB, MinIO), live cluster signals, MCP tool wiring. Imports the standalone module. |

**Rule**: logic that only needs the graph and YAML fixtures belongs in the standalone module.
Logic that needs a running Globular cluster (etcd, gRPC, node-agent protos) belongs here.

---

## Package map

```
awareness/
├── graph/           Core knowledge graph — nodes, edges, invariants, incidents
├── extractors/      Populate the graph from external sources
│   ├── manual/      Invariants and failure modes from YAML docs
│   ├── metrics/     Prometheus metric coverage annotations
│   ├── dns/         DNS zone knowledge
│   ├── goast/       Go AST source analysis
│   ├── packages/    Package catalog classification
│   ├── pki/         Certificate and PKI state
│   ├── rbac/        RBAC policy graph
│   ├── scripts/     Shell script analysis
│   ├── clusterspec/ Cluster spec / desired-state extraction
│   ├── clusterstate/Live cluster state signals
│   ├── doctor/      Cluster doctor report integration
│   ├── docs/        YAML documentation index
│   ├── workflowstate/ Workflow runtime signals
│   └── tests/       Test coverage mapping
├── preflight/       Pre-edit safety check (invariants, forbidden fixes)
├── learning/        Failure pattern learning and signature normalization
├── failuregraph/    Categorized failure knowledge (seeded + learned)
├── failurelearning/ Learning loop (propose → review → apply)
├── fixledger/       Fix case registry and DOD tracking
├── sessionoracle/   Session resumption memory
├── incidentpattern/ Incident pattern matching
├── integrity/       Graph integrity verification
├── assurance/       Coverage and quality checks
├── semantic/        Semantic diff interpretation
├── livecluster/     Live cluster signal integration
├── debugsession/    Debug session tracking
├── bundlesync/      Awareness bundle versioning and sync
├── checkedit/       Pre-edit context enrichment
├── context/         Context alias resolution
├── contextfreshness/ Stale-context detection
├── enforce/         Invariant enforcement
├── analysis/        Impact and causal analysis
├── runtime/         Runtime adapter (live vs offline)
└── docs/awareness/  YAML knowledge files (invariants, fix_cases, aliases, ...)
```

---

## Docs directory

`docs/awareness/` contains the YAML files that seed the graph at build time:

| File | Purpose |
|------|---------|
| `invariants.yaml` | Hard invariants (forbidden fixes, required tests) |
| `fix_cases.yaml` | Historical fix cases and their DOD |
| `context_aliases.yaml` | Natural-language aliases mapped to invariant IDs |
| `learning_rules.yaml` | Failure learning promotion rules |
| `knowledge/` | Domain knowledge (DNS zones, metric queries/thresholds, …) |

---

## Building the graph

```bash
# From the services root:
globular awareness build

# Or via the standalone CLI:
awareness build --docs docs/awareness --repo .
```

The graph is written to `/var/lib/globular/awareness/runtime/graph.json` (on cluster nodes)
or `~/.cache/globular/awareness/graph.json` (local dev).

---

## MCP tools

The MCP server (`golang/mcp/`) exposes ~40 awareness tools. Key entry points:

| Tool | When to use |
|------|-------------|
| `awareness.session_start` | Start of every dev session |
| `awareness.preflight` | Before editing any file |
| `awareness.scan_violations` | After editing, before commit |
| `awareness.health_pulse` | Scheduled integrity check |
| `awareness.impact_file` | Blast-radius for a single file |
| `awareness.explain_invariant` | Deep-dive on one invariant |
