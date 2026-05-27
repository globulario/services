# Awareness Intent Audit v1.1

This document freezes the Awareness Intent Audit v1.1 baseline and defines how operators and agents should use it.

## What Intent Audit Does

Intent audit checks whether code and runtime behavior still match declared architecture intent.

It has two independent parts:

- Source audit: static checks against intent docs, allowed exceptions, and test contracts.
- Runtime evidence (optional): read-only checks against live cluster state.

Core commands:

```bash
go run ./globularcli awareness intent-audit --format text --fail-on none
go run ./globularcli awareness intent-audit --runtime --runtime-timeout 2s --format text --fail-on none
```

## Source Audit Semantics

Source audit classifies findings as:

- `pass`: check passed.
- `candidate_violation`: likely intent breach in code.
- `accepted_exception`: explicitly approved exception with traceability.
- `test_coverage_gap`: intent exists but `required_tests` mapping is missing.
- `missing_test`: a required test mapping exists but no matching test evidence exists.

### `required_tests`

`required_tests` is the contract between intent and verification. When an intent says a behavior must hold, `required_tests` lists the tests that prove it.

### `accepted_exception`

`accepted_exception` is for narrow, documented, intentional deviations. It is not a way to silence broad failures.

### `test_coverage_gap`

A gap means intent has no required proof mapping yet. The code may still be correct, but audit cannot enforce it strongly.

## Runtime Evidence Checks (v1.1)

Current checks:

- `desired.build_id_immutable_after_resolution`
- `installed_state.owned_by_node_agent`
- `runtime_observation_must_not_mutate_desired`

Runtime evidence is optional by design:

- It depends on live etcd/runtime availability.
- It can return `UNKNOWN` for incomplete evidence without blocking source-only workflows.
- CI remains source-only to avoid environment flakiness in baseline enforcement.

## Desired-Write Provenance (v1.1)

Desired-state mutations emit provenance records under:

- `/globular/audit/desired_writes/<record>`

Record schema:

- `service`
- `actor`
- `source`
- `action`
- `reason`
- `timestamp`
- optional `workflow_run_id`
- optional `etcd_revision`

Runtime causality check behavior:

- `PASS`: provenance exists for desired services and no forbidden runtime actors/sources are observed.
- `FAIL`: provenance indicates runtime/heartbeat/observer/node-agent/verifier mutation authority.
- `UNKNOWN`: provenance missing/incomplete or evidence unreadable.
- `NOT_APPLICABLE`: no desired-state records.

## Agent Rules

1. Run source audit before risky architecture edits.
2. Run scoped audit when touching known sensitive files.
3. Run runtime audit after changing desired/installed/reconciliation/canonicalize paths.
4. Never fix an intent violation by broad exception.
5. If code and intent disagree, classify as one of:
   - code bug
   - intent drift
   - accepted exception
   - test coverage gap
6. Runtime evidence is read-only.
7. `UNKNOWN` is not `PASS`.
8. Do not fabricate provenance.

## Baseline Freeze (v1.1)

Verified baseline in this freeze pass (2026-05-26):

- Source audit:
  - `pass=49`
  - `candidate_violation=0`
  - `accepted_exception=1`
  - `test_coverage_gap=19`
  - `missing_test=0`
- Runtime evidence:
  - `desired.build_id_immutable_after_resolution`: `PASS`
  - `installed_state.owned_by_node_agent`: `PASS`
  - `runtime_observation_must_not_mutate_desired`: `PASS`

## Next Runtime Evidence Candidate: `repository.metadata_is_authority`

Goal: verify desired/installed artifact identities actually resolve to repository authority metadata.

### Data sources

Desired-state identity fields:

- `/globular/resources/ServiceDesiredVersion/*`
  - `spec.service_name`
  - `spec.version`
  - `spec.build_number`
  - `spec.build_id`

Installed-state identity fields:

- `/globular/nodes/{node}/packages/{kind}/{name}`
  - `name`
  - `version`
  - `build_number`
  - `build_id`

Repository metadata candidates:

- repository manifest/catalog authority used by resolve/reachability guards
- artifact manifest identity fields (`publisher_id`, `name`, `version`, `build_number`, `build_id`, checksum)
- any repository index tables used to resolve build_id to manifest

### Classification design

- `PASS`:
  - qualifying desired/installed references resolve to repository metadata entries with matching identity.
- `FAIL`:
  - desired/installed references contain `build_id`/`build_number` that repository metadata cannot resolve.
- `UNKNOWN`:
  - repository metadata unavailable, unreadable, or ambiguous for definitive matching.
- `NOT_APPLICABLE`:
  - no qualifying desired/installed records with artifact identity fields.

### Provider interface impact

Current runtime provider supports key reads/listing. For repository-authority checks, we likely need one of:

- extend provider with repository manifest lookup helpers, or
- add a dedicated repository evidence provider passed only to this check.

Prefer dedicated provider to keep existing runtime evidence API stable.

### Required mock tests

Minimum test set:

- `PASS`: desired+installed identities match repository metadata.
- `FAIL`: desired build_id missing in repository metadata.
- `FAIL`: installed build_id missing in repository metadata.
- `UNKNOWN`: repository metadata read error/unavailable.
- `UNKNOWN`: ambiguous duplicate metadata resolution.
- `NOT_APPLICABLE`: no desired/installed qualifying entries.

Guardrail: this check verifies identity only. It must not re-resolve desired state to latest and must not mutate desired/runtime state.
