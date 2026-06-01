---
id: inference_v0_direct_anchors_only
type: architecture_decision
status: accepted
summary: v0 awareness uses direct file-level and symbol-level anchors only. Inferred-edge coverage is intentionally unimplemented. Unannotated files returning EMPTY is expected behavior. EMPTY does not mean safe.
---

## v0 Inference Policy: Direct Anchors Only

### Decision

The v0 awareness system uses **direct file-level and symbol-level anchors only**.
A file is covered when it carries `@awareness` annotations or when an invariant/
failure-mode YAML explicitly lists that file in `source_files`. Coverage flows from
annotation to graph; it does not flow through proximity.

### What "EMPTY" means

A briefing or impact result with `status: EMPTY` means **no direct awareness anchor
exists for that file**. It does not mean the file is safe to edit freely. It means
the file has no explicitly declared invariant, failure-mode, or intent attachment.

High-risk files returning EMPTY must be treated as opaque: read the code, check
`high_risk_files.yaml`, and add file-level annotations before making non-trivial
changes.

### Inferred fields are reserved, not implemented

The `ImpactResponse` proto message declares `inferred_invariants`,
`inferred_failure_modes`, `inferred_incident_patterns`, and `inferred_intents`
fields. These are **allocated capacity for a future phase**. In v0 they are always
empty. No code path in the scanner, importer, store, or server populates them.

This is intentional. Populating inferred fields requires a four-layer
implementation:

1. **Scanner** — emit typed `inComponent` IRI edge from SourceFile to Component node
2. **Importer** — mint `Component` IRI nodes; emit `Component --implements-->` edges
3. **Store / SPARQL** — add `InferredForFile` query with 2-hop path
4. **Server** — populate inferred response fields, apply cap, label in prose

Until all four layers are complete and tested, inferred fields remain empty.

### Why component-level inference is deferred

Inferring awareness context from neighboring annotated files within the same
component risks false or noisy context:

- A read-path file does not inherit write-path invariants because they share a component.
- A large component with 10 annotated symbols would assert all their invariants on
  every unannotated file in that component — even files with no relation to those
  invariants.
- Incorrect inferred context is worse than EMPTY: it trains AI agents to treat rules
  as applying where they do not.

### Correct fix for unannotated high-risk files

Add a 5-line file-level `@awareness` annotation block:

```go
// @awareness namespace=globular.platform
// @awareness component=my_component
// @awareness file_role=<role>
// @awareness enforces=globular.platform:invariant.<relevant>
// @awareness risk=high
package foo
```

This produces a direct anchor, emits a CodeSymbol node, and creates a SourceFile
`implements` edge — all verified by `build-awareness-graph.sh --check`.

### Future inference constraints (if implemented)

Any future inferred-edge implementation must satisfy all of the following:

| Constraint | Requirement |
|---|---|
| Namespace boundary | Same namespace only — never cross namespace |
| Component boundary | Same explicitly declared component only — no directory or package fuzzy fallback |
| Max inferred nodes | ≤ 5 per response (prevents noise in large components) |
| Ordering | Deterministic: severity-rank then ID |
| Prose presentation | Clearly labelled "Inferred" in a separate section from "Direct" |
| Status | `EMPTY` only if both direct AND inferred are empty |
| Test coverage | Must include: cap enforcement, namespace isolation, component isolation, direct-unchanged |
