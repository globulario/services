# Globular Version Control — Phase 1 Validation Runbook

## Pre-Flight Checklist

Run ALL checks before any test. If any check fails, do NOT proceed.

```bash
# 1. Controller leader healthy
globular cluster info 2>&1 | grep -i leader
# Expected: leader on one of the 3 nodes

# 2. Repository reachable
globular pkg info echo 2>&1 | head -5
# Expected: package info returned (not connection error)

# 3. All 3 nodes healthy
globular cluster list-nodes 2>&1
# Expected: 3 nodes, all status=ready or converging, last_seen < 1 min ago

# 4. Record baseline for echo service
echo "=== BASELINE ==="
globular pkg info echo > /tmp/phase1-test/baseline-pkg-info.txt 2>&1
globular services desired get 2>&1 | grep echo > /tmp/phase1-test/baseline-desired.txt
```

```bash
# Create test working directory
mkdir -p /tmp/phase1-test
```

### Rollback command (prepared, not executed)

```bash
# If anything goes wrong, revert Phase 1:
cd /home/dave/Documents/github.com/globulario/services
git checkout -- golang/repository/repository_server/artifact_handlers.go
git checkout -- golang/deploy/deploy.go
git checkout -- golang/cluster_controller/cluster_controller_server/desired_state_handlers.go
# Then rebuild and redeploy affected services
```

---

## STOP CONDITIONS

**Immediately stop all testing if:**

- An artifact digest changes when it should not (compare before/after)
- `etcd` contains a `ServiceDesiredVersion` entry for version `9.9.9` or `99.99.99`
- Controller starts reconciling or dispatching workflows for test versions
- Any non-echo service is impacted (check `journalctl -u globular-cluster-controller.service -n 20` for unexpected activity)
- Repository crashes or restarts unexpectedly

---

## Part 1 — Validation Runbook (Safe, no service interruption)

Run these immediately. No services are stopped or disrupted.

---

### Test 1.1 — Desired state: non-existent version rejected

**Risk:** None. Read-only on repo, write is expected to be blocked.

**Commands:**

```bash
# Attempt to set desired to a version that cannot exist
globular services desired set echo 9.9.9 --controller 10.0.0.8:12000 2>&1 | tee /tmp/phase1-test/t1.1-output.txt
```

**Expected output:** Error containing `"not found in repository"` or `"NotFound"`

**Verify etcd not modified:**

```bash
globular services desired get 2>&1 | grep echo | tee /tmp/phase1-test/t1.1-desired-after.txt
diff /tmp/phase1-test/baseline-desired.txt /tmp/phase1-test/t1.1-desired-after.txt
```

**Expected:** No diff. Echo desired state unchanged.

**Evidence:** Save `t1.1-output.txt` and `t1.1-desired-after.txt`.

**Pass criteria:** Error returned AND etcd unchanged.

---

### Test 1.2 — Desired state: second non-existent version rejected

**Risk:** None. Confirms the guard works for different service names.

**Commands:**

```bash
globular services desired set authentication 99.99.99 --controller 10.0.0.8:12000 2>&1 | tee /tmp/phase1-test/t1.2-output.txt
```

**Expected output:** Error containing `"not found in repository"`

**Pass criteria:** Error returned.

---

### Test 1.3 — Desired state: valid existing version accepted

**Risk:** None if using the currently desired version. This is a no-op write.

**Commands:**

```bash
# Read current desired version for echo
CURRENT_VER=$(globular services desired get 2>&1 | grep "echo" | awk '{print $2}' | cut -d@ -f1)
echo "Current echo version: $CURRENT_VER"

# Re-set to the same version (should succeed, no-op)
globular services desired set echo $CURRENT_VER --controller 10.0.0.8:12000 2>&1 | tee /tmp/phase1-test/t1.3-output.txt
```

**Expected output:** Success (`"OK: desired state updated"`)

**Pass criteria:** Command succeeds.

---

### Test 1.4 — Idempotent publish

**Risk:** None. Re-uploading identical bytes is a no-op.

**Commands:**

```bash
# Find the most recent echo package
ECHO_TGZ=$(ls -t /home/dave/Documents/github.com/globulario/services/generated/echo_*.tgz 2>/dev/null | head -1)
if [ -z "$ECHO_TGZ" ]; then
    ECHO_TGZ=$(ls -t /var/lib/globular/packages/out/echo_*.tgz 2>/dev/null | head -1)
fi
echo "Using: $ECHO_TGZ"

# Record digest before
globular pkg info echo 2>&1 | grep -i digest | tee /tmp/phase1-test/t1.4-digest-before.txt

# Publish same package again
globular pkg publish --file "$ECHO_TGZ" --repository 10.0.0.8:443 2>&1 | tee /tmp/phase1-test/t1.4-output.txt

# Record digest after
globular pkg info echo 2>&1 | grep -i digest | tee /tmp/phase1-test/t1.4-digest-after.txt

# Verify unchanged
diff /tmp/phase1-test/t1.4-digest-before.txt /tmp/phase1-test/t1.4-digest-after.txt
```

**Expected:**
- Publish returns success (idempotent)
- Digest before == digest after (no diff)

**Check repository log for idempotent path:**

```bash
journalctl -u globular-repository.service --no-pager -n 20 2>&1 | grep -E "idempotent|re-upload" | tee /tmp/phase1-test/t1.4-repo-log.txt
```

**Expected:** Line containing `"idempotent, same checksum"`

**Pass criteria:** Success return AND digest unchanged AND idempotent log line present.

---

### Test 1.5 — Overwrite prevention (critical)

**Risk:** Low. We build a test artifact but the upload is expected to be rejected.

**Commands:**

```bash
# Step 1: Record current echo artifact info
globular pkg info echo 2>&1 | tee /tmp/phase1-test/t1.5-before.txt

# Step 2: Get current version and build number
# (parse from pkg info output or use known values)
ECHO_VER="0.0.8"   # adjust to actual current version
ECHO_BUILD=1        # adjust to actual current build number

# Step 3: Build a DIFFERENT binary
cd /home/dave/Documents/github.com/globulario/services/golang
# Add a harmless change to force different checksum
echo "// phase1-test-marker" >> echo/echo_server/main.go
go build -buildvcs=false -o /tmp/phase1-test/echo_server ./echo/echo_server
# Revert the change immediately
git checkout -- echo/echo_server/main.go

# Step 4: Stage and build package at SAME version+build
mkdir -p /tmp/phase1-test/payload/bin
cp /tmp/phase1-test/echo_server /tmp/phase1-test/payload/bin/echo_server
globular pkg build \
    --spec ../generated/specs/echo_service.yaml \
    --root /tmp/phase1-test/payload \
    --version $ECHO_VER --build-number $ECHO_BUILD \
    --out /tmp/phase1-test/ 2>&1 | tee /tmp/phase1-test/t1.5-build.txt

# Step 5: Attempt publish (should FAIL)
globular pkg publish \
    --file /tmp/phase1-test/echo_${ECHO_VER}_linux_amd64.tgz \
    --repository 10.0.0.8:443 2>&1 | tee /tmp/phase1-test/t1.5-publish.txt

# Step 6: Verify artifact unchanged
globular pkg info echo 2>&1 | tee /tmp/phase1-test/t1.5-after.txt
diff /tmp/phase1-test/t1.5-before.txt /tmp/phase1-test/t1.5-after.txt
```

**Expected:**
- Step 5: Error containing `"overwrite of published artifacts is forbidden"` or `"AlreadyExists"`
- Step 6: No diff — artifact digest unchanged

**Check repository log:**

```bash
journalctl -u globular-repository.service --no-pager -n 10 2>&1 | grep -E "forbidden|AlreadyExists|terminal" | tee /tmp/phase1-test/t1.5-repo-log.txt
```

**Expected:** Rejection log line present.

**Pass criteria:** Publish rejected AND artifact unchanged AND rejection logged.

---

### Test 1.6 — Deploy collision via manual publish

**Risk:** Low. We attempt a manual publish at a known build number. No deploy command used.

**Commands:**

```bash
# Find the latest published build number for echo
globular pkg info echo 2>&1 | grep -i build | tee /tmp/phase1-test/t1.6-current.txt
# Note the build number (e.g., 2)

# Attempt to publish the test artifact from Test 1.5 at that build number
# (already built at /tmp/phase1-test/echo_*.tgz with different content)
globular pkg publish \
    --file /tmp/phase1-test/echo_${ECHO_VER}_linux_amd64.tgz \
    --repository 10.0.0.8:443 2>&1 | tee /tmp/phase1-test/t1.6-output.txt
```

**Expected:** `AlreadyExists` error (same as Test 1.5 — the artifact is PUBLISHED).

**Pass criteria:** Rejected.

---

**Tier 1 checkpoint:** If all tests 1.1–1.6 pass, proceed to Tier 2.

---

### Test 2.1 — Normal deploy (new build number)

**Risk:** Medium. This actually deploys a new echo binary. The service will restart.

**Commands:**

```bash
# Record state before
globular pkg info echo 2>&1 | tee /tmp/phase1-test/t2.1-before.txt

# Deploy echo (builds, publishes at new build number, sets desired)
cd /home/dave/Documents/github.com/globulario/services
globular deploy echo_server 2>&1 | tee /tmp/phase1-test/t2.1-deploy.txt

# Record state after
globular pkg info echo 2>&1 | tee /tmp/phase1-test/t2.1-after.txt
```

**Expected:**
- Deploy succeeds
- Build number incremented (compare before/after)
- New artifact created (different digest if binary changed, or skip if identical)
- `desired set` portion succeeds (new version now exists in repo)

**Pass criteria:** Deploy completes without errors. New build number visible.

---

### Test 2.2 — Pre-terminal overwrite allowed

**Risk:** Low. Uploads to VERIFIED state only (not PUBLISHED).

This test requires direct repository interaction to create a VERIFIED-only artifact. If the MCP `package_build` + `package_publish` tools auto-promote to PUBLISHED, this test may need to be deferred or done with a custom gRPC call.

**Alternative verification:** Confirm that `isTerminalState(VERIFIED)` returns false by code inspection (already verified during implementation). Mark as **verified by code review** if live testing is impractical.

---

**Tier 2 checkpoint:** If all Tier 2 tests pass, Phase 1 validation is complete for normal operations. Tier 3 is optional.

---

## Part 2 — Failure Injection Runbook (Disruptive, requires approval)

**WARNING:** These tests stop services on the live cluster. Run only with explicit operator approval and during a maintenance window.

---

### Test 3.1 — Repository unavailable during desired set

**Risk:** High — stopping repository affects all nodes.

**Commands:**

```bash
# Step 1: Stop repository
sudo systemctl stop globular-repository.service

# Step 2: Attempt desired set (should fail)
globular services desired set echo 0.0.8 --controller 10.0.0.8:12000 2>&1 | tee /tmp/phase1-test/t3.1-output.txt

# Step 3: Restart repository IMMEDIATELY
sudo systemctl start globular-repository.service

# Step 4: Verify etcd unchanged
globular services desired get 2>&1 | grep echo | tee /tmp/phase1-test/t3.1-desired-after.txt
```

**Expected:** Step 2 returns `"Unavailable"`. etcd unchanged.

**Pass criteria:** Error returned AND etcd unchanged AND repository restarts cleanly.

---

### Test 3.2 — Concurrent deploy

**Risk:** Medium — two deploys running simultaneously. Echo service may restart twice.

**Commands:**

```bash
# Run two deploys simultaneously
globular deploy echo_server > /tmp/phase1-test/t3.2-deploy1.txt 2>&1 &
PID1=$!
globular deploy echo_server > /tmp/phase1-test/t3.2-deploy2.txt 2>&1 &
PID2=$!

wait $PID1
echo "Deploy 1 exit: $?"
wait $PID2
echo "Deploy 2 exit: $?"

cat /tmp/phase1-test/t3.2-deploy1.txt
cat /tmp/phase1-test/t3.2-deploy2.txt
```

**Expected:** At least one succeeds. If both get different build numbers, both succeed. If they collide, one fails with `AlreadyExists` — that's correct behavior.

**Pass criteria:** No silent overwrite. At least one deploy succeeds. Failed deploy shows clear error.

---

## Evidence Capture Summary

After all tests, collect:

```bash
# Bundle all evidence
tar czf /tmp/phase1-validation-evidence.tar.gz /tmp/phase1-test/
ls -la /tmp/phase1-test/
```

Each test produces:
- Command output (`t*.txt`)
- Relevant log excerpts (`*-log.txt`)
- Before/after comparisons (`*-before.txt`, `*-after.txt`)

---

## Final Signoff Criteria

### Must pass (blocks rollout if any fail)

| # | Criterion | Test |
|---|-----------|------|
| 1 | Non-existent desired state rejected | Test 1.1, 1.2 |
| 2 | Valid desired state accepted | Test 1.3 |
| 3 | Idempotent publish works | Test 1.4 |
| 4 | Overwrite of PUBLISHED artifact blocked | Test 1.5 |
| 5 | Artifact unchanged after rejected overwrite | Test 1.5 (diff) |
| 6 | Normal deploy works | Test 2.1 |

### Should pass (defer to Phase 2 if problematic)

| # | Criterion | Test |
|---|-----------|------|
| 7 | Concurrent deploy handles collision cleanly | Test 3.2 |
| 8 | Repository unavailable returns Unavailable | Test 3.1 |

### Findings deferrable to Phase 2

- Build number regression within same version (requires `build_id`)
- Pre-terminal overwrite verification (requires direct gRPC testing)
- Desired state publish-state enforcement (node agent is current guard)
