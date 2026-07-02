# Awareness Enforcement

The `enforce` package validates that `//globular:` annotations are well-formed,
that data contracts are satisfied, and that the awareness graph stays in sync
with the source code. It backs the `globular awareness audit` family of commands.

---

## Commands

| Command | What it checks | Exits 1 on |
|---------|---------------|------------|
| `awareness audit` | All checks below | Any ERROR |
| `awareness validate-annotations` | Annotation syntax in Go source | ERROR findings |
| `awareness validate-required-tests` | tested_by targets exist in graph | ERROR findings |
| `awareness validate-contracts` | Hash schema contracts complete | ERROR findings |
| `awareness graph-drift` | Stale source_file nodes | ERROR findings |
| `awareness pr-report` | Changed files only | ERROR findings |

All commands accept `--json` to output machine-readable results.

---

## Annotation rules

| Directive | Rule |
|-----------|------|
| `//globular:state_transition` | Must contain ` -> ` with non-empty FROM and TO |
| `//globular:hash_schema` | Value must be a single identifier (no spaces) |
| `//globular:expects_hash_schema` | Value must be a single identifier (no spaces) |
| `//globular:enforces` | Value must be a dot-separated identifier (no spaces) |
| `//globular:protects` | Value must be a dot-separated identifier (no spaces) |
| `//globular:tested_by` | Value must start with `Test`, `Benchmark`, or `Example` |
| All directives | Value must be non-empty |

---

## Finding codes

| Code | Severity | Meaning |
|------|----------|---------|
| `ANNOTATION_MISSING_VALUE` | ERROR | Directive has no value |
| `ANNOTATION_BAD_IDENTIFIER` | ERROR | Value contains whitespace where identifier expected |
| `ANNOTATION_BAD_TEST_NAME` | ERROR | tested_by value doesn't start with Test/Benchmark/Example |
| `MALFORMED_STATE_TRANSITION` | ERROR | state_transition missing ` -> ` or empty side |
| `ORPHANED_HASH_SCHEMA` | ERROR | Hash schema node has no producer and no consumer |
| `MISSING_HASH_PRODUCER` | ERROR | Hash schema has consumer(s) but no producer |
| `MISSING_HASH_CONSUMER` | WARNING | Hash schema has producer but no consumer yet |
| `REQUIRED_TEST_MISSING` | ERROR | tested_by target does not exist in the graph |
| `REQUIRED_TEST_NO_PATH` | WARNING | Test node has no file path |
| `STALE_SOURCE_FILE_NODE` | WARNING | source_file node exists but file is gone on disk |
| `ORPHANED_INVARIANT_NODE` | INFO | Invariant node has no enforces/protects edge |

---

## Hash schema contracts

A hash schema contract requires **one producer** and **at least one consumer**:

```go
// Producer — the function that computes the hash
//globular:hash_schema infra_desired_hash
func ComputeInfrastructureDesiredHash() string { ... }

// Consumer — the function that reads and validates the hash
//globular:expects_hash_schema infra_desired_hash
func classifyPackageConvergence() { ... }
```

The graph links both sides through a shared `hash_schema:infra_desired_hash` node.
`validate-contracts` fails if either side is missing.

---

## CI integration

Add to your CI pipeline:

```yaml
- name: Awareness enforcement
  run: |
    globular awareness audit --json | tee audit.json
    jq -e '.pass' audit.json
```

Or use the provided script directly:

```bash
./scripts/awareness-ci.sh
```

The script exits 1 if any ERROR findings are found, making it a hard CI gate.

## Coverage ratchet (separate gate)

`awareness-ci.sh` checks annotations and audit findings. The orthogonal
question — *has coverage of failure_modes regressed?* — is handled by
a second small script:

```bash
./scripts/awareness-ci-check.sh
```

This calls `globular awareness meta-check` with thresholds that match
the current cluster state on the date the script was last updated. The
intent is a ratchet: the gate fails only when coverage drops below the
known-good floor, never when coverage is honestly limited.

Floors live as constants at the top of the script:

| Variable           | Meaning                                                |
|--------------------|--------------------------------------------------------|
| `MIN_WELL_COVERED` | Minimum well_covered failure_mode count (3 legs)       |
| `MIN_DETECTED`     | Minimum DETECTED+ENFORCED count (runtime-observable)   |
| `BASELINE_ORPHANS` | Maximum orphan count tolerated                         |

When real coverage improves (new mitigation + test + detector lands for
a previously-partial failure_mode), raise the corresponding floor in
the same PR that adds the new enforcement. The script ratchets, it does
not aspire — never set a floor higher than the current count.

This gate is wired into services CI in `.github/workflows/ci.yml`:

```yaml
- name: Awareness coverage ratchet
  run: ./scripts/awareness-ci-check.sh
```

`--strict` additionally fails on any orphan failure_mode. Default mode
fails only on regressions below the pinned floors. Pair with
`awareness-ci.sh` for a complete annotation + audit + coverage gate.
