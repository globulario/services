# Claude / Codex Instruction: Repository Package Identity Collision Pass

## Context

The cluster is showing package/repository drift symptoms:

```text
cluster.services.drift: desired_hash != applied_hash
```

There is a suspected package identity collision: for `cluster_controller_service`, two repository records may carry different `build_id` values while sharing the same `build_number` lane. This suggests the repository may have accepted publishing/importing the same package version/build lane more than once.

Existing repository findings also include `DesiredBuildIdOrphaned`: desired state pins a `build_id`, but the repository build_id index cannot resolve it. The forbidden fix is explicit: do **not** delete desired build IDs; repair the repository index/content instead.

## Goal

Make repository/package identity collision detection explicit, auditable, and repairable.

Do not mutate desired state to hide repository corruption.

Do not re-resolve desired to latest.

Do not treat `build_number` as artifact identity.

---

## Phase 1 — Baseline and Evidence Capture

Run:

```bash
sudo globular repository doctor identity --json > /tmp/repository-identity-before.json
sudo globular doctor report cluster --fresh --json > /tmp/cluster-doctor-before.json
go run ./globularcli awareness intent-audit --format text --fail-on none
go run ./globularcli awareness intent-audit --runtime --runtime-timeout 2s --format text --fail-on none
```

Extract all package identity anomalies involving:

- `cluster-controller`
- `cluster_controller_service`
- `cluster-controller-service`
- any package in the drift hash set

For each suspicious artifact, record:

```text
publisher
name
version
platform
build_number
build_id
checksum
publish_state
kind
desired_pinned: yes/no
installed_on_nodes: yes/no
runtime_verified: yes/no
```

---

## Phase 2 — Add Repository Identity Doctor Checks

Add or update repository doctor identity checks to detect:

1. **RepositoryBuildNumberCollision**
   - same `(publisher, name, version, platform, build_number)`
   - more than one distinct `build_id`

2. **RepositoryBuildIDReuse**
   - same `build_id`
   - more than one distinct artifact tuple

3. **RepositoryChecksumDivergence**
   - same `build_id`
   - manifest/ledger/blob checksum disagreement

4. **RepositoryKindDivergence**
   - same package identity lane
   - different artifact kind values

Every finding must include:

```text
finding_id
invariant_id
publisher/name/version/platform/build_number
build_ids involved
publish states
checksums if available
desired pinned build_id if any
forbidden_fix
safe_repair_hint
```

Do not repair yet. First make the doctor report the collision clearly.

---

## Phase 3 — Add Publish/Import Collision Gates

Patch publish/sync/import paths so they cannot create ambiguous package identity:

### Rule A — same digest retry is idempotent

If the exact same bytes/digest already exist, return the existing `build_id`.

### Rule B — same immutable lane with different digest is rejected

If `(publisher, name, version, platform, build_number)` already exists with a different `build_id` or checksum, reject with `AlreadyExists` or mark as collision before writing.

### Rule C — build_id is repository-owned identity

Clients/upstream sync must not override canonical `build_id` unless it is an explicit upstream alias mapped to a repository-owned canonical build_id.

### Rule D — sync/import must share the same collision checks as upload

Upstream sync must not bypass local upload invariants.

---

## Phase 4 — Add Repair Safety

If repair sees a collision:

1. Check desired pins first.
2. If one build_id is desired-pinned, it is canonical for this cluster unless operator explicitly rolls desired forward.
3. If neither is desired-pinned:
   - identical digest duplicate may be collapsed/aliased;
   - different digest duplicate must be quarantined/archived with a finding.
4. Never archive/delete/demote a desired-pinned artifact without explicit desired migration.

Repair output must say what it did:

```text
collision lane
canonical build_id
archived/quarantined build_id
reason
safe because desired_pinned=false
```

---

## Phase 5 — Add Awareness Intent + Invariant Metadata

Install the new intent files from this artifact pack:

```text
docs/intent/repository.build_number_collision_is_corruption.yaml
docs/intent/package.identity_tuple_must_be_unique.yaml
docs/intent/repository.publish_is_idempotent_by_digest.yaml
docs/intent/repository.repair_uses_existing_build_records.yaml
docs/intent/repository.identity_doctor_reports_collisions.yaml
```

Merge the invariant and forbidden-fix YAML files into Awareness:

```text
docs/awareness/package_identity_invariants.yaml
docs/awareness/forbidden_fixes/package_identity_forbidden_fixes.yaml
docs/awareness/failuregraph_seeds/repository_duplicate_build_number_collision.yaml
```

Then run a scoped audit:

```bash
go run ./globularcli awareness intent-audit --scope repository.build_number_collision_is_corruption,package.identity_tuple_must_be_unique,repository.identity_doctor_reports_collisions --format text --fail-on none
```

---

## Phase 6 — Tests

Add focused tests. Use real existing helper names where possible.

Required coverage:

1. publish rejects same lane with different digest/build_id
2. idempotent same digest returns existing build_id
3. sync/import cannot bypass collision gate
4. repository doctor reports duplicate build_number lane
5. repository doctor reports build_id reuse
6. repair refuses to archive desired-pinned duplicate
7. repair can quarantine/archive unpinned non-canonical duplicate
8. desired build_id orphan finding still forbids deleting desired build_id

---

## Phase 7 — Verification

Run:

```bash
go test ./repository/...
go test ./awareness/intentaudit/...
go test ./globularcli/...
go run ./globularcli awareness intent-audit --format text --fail-on none
go run ./globularcli awareness intent-audit --runtime --runtime-timeout 2s --format text --fail-on none
sudo globular repository doctor identity --json > /tmp/repository-identity-after.json
sudo globular doctor report cluster --fresh --json > /tmp/cluster-doctor-after.json
```

Expected:

```text
candidate_violation=0
missing_test=0
repository doctor reports collisions explicitly if present
DesiredBuildIdOrphaned not hidden by desired mutation
cluster.services.drift either clears or has a precise repository finding explaining why
```

---

## Final Report

Return:

```text
Repository Package Identity Collision Pass — Report

1. Baseline
   - drift findings
   - repository identity findings
   - suspicious package lanes

2. Collision analysis
   - duplicate build_number lanes
   - duplicate build_id reuse
   - checksum/kind divergence
   - desired-pinned status

3. Code changes
   - doctor checks added
   - publish/sync gates added
   - repair safety added

4. Tests
   - tests added/updated
   - results

5. Awareness artifacts installed
   - intent files
   - invariants
   - forbidden fixes

6. Verification
   - repository doctor after
   - cluster doctor after
   - intent/runtime audit after

7. Remaining risks
```

---

## Guardrails

- Do not delete desired build IDs.
- Do not re-resolve desired state to latest.
- Do not clear drift by mutating desired to match repository corruption.
- Do not use `build_number` as convergence identity.
- Do not archive desired-pinned artifacts without explicit desired migration.
- Do not hide duplicate records by picking latest.
- Do not weaken repository identity checks.
- Repository metadata is the repair target.
