# Globular Version Control — Tier 1 Execution Checklist

## Pre-flight

```bash
mkdir -p /tmp/phase1-test
globular cluster info 2>&1 | head -3
globular pkg info echo 2>&1 | head -3
globular services desired get 2>&1 | grep echo > /tmp/phase1-test/baseline-desired.txt
globular pkg info echo > /tmp/phase1-test/baseline-pkg-info.txt 2>&1
cat /tmp/phase1-test/baseline-desired.txt
```

If any command fails with a connection error, do NOT proceed.

---

## Test 1.1 — Non-existent desired version rejected

**Preconditions:** Controller leader reachable. Repository reachable.

**Commands:**

```bash
globular services desired set echo 9.9.9 --controller 10.0.0.8:12000 2>&1 | tee /tmp/phase1-test/t1.1.txt
globular services desired get 2>&1 | grep echo > /tmp/phase1-test/t1.1-desired-after.txt
diff /tmp/phase1-test/baseline-desired.txt /tmp/phase1-test/t1.1-desired-after.txt
```

**Pass:** First command output contains `not found in repository` OR `NotFound`. Diff shows no changes.

**Stop if:** Diff shows changes (desired state was written for 9.9.9).

**Evidence:** `t1.1.txt`, `t1.1-desired-after.txt`

---

## Test 1.2 — Second non-existent desired version rejected

**Preconditions:** Test 1.1 passed.

**Commands:**

```bash
globular services desired set authentication 99.99.99 --controller 10.0.0.8:12000 2>&1 | tee /tmp/phase1-test/t1.2.txt
```

**Pass:** Output contains `not found in repository` OR `NotFound`.

**Stop if:** Output contains `OK` or `desired state updated`.

**Evidence:** `t1.2.txt`

---

## Test 1.3 — Valid desired version accepted

**Preconditions:** Test 1.2 passed. Know the current echo desired version.

**Commands:**

```bash
CURRENT_VER=$(cat /tmp/phase1-test/baseline-desired.txt | awk '{print $2}' | head -1)
echo "Re-setting echo to current version: $CURRENT_VER"
globular services desired set echo $CURRENT_VER --controller 10.0.0.8:12000 2>&1 | tee /tmp/phase1-test/t1.3.txt
```

If `CURRENT_VER` is empty or parsing fails, read the baseline file manually and substitute the version.

**Pass:** Output contains `OK` or `desired state updated`.

**Stop if:** Output contains `not found in repository` or `Unavailable` (would mean the validation guard is rejecting valid artifacts).

**Evidence:** `t1.3.txt`

---

## Test 1.4 — Idempotent publish

**Preconditions:** Test 1.3 passed. An echo .tgz file exists.

**Commands:**

```bash
# Find echo package
ECHO_TGZ=$(ls -t /var/lib/globular/packages/out/echo_*.tgz 2>/dev/null | head -1)
if [ -z "$ECHO_TGZ" ]; then
    ECHO_TGZ=$(ls -t /home/dave/Documents/github.com/globulario/services/generated/echo_*.tgz 2>/dev/null | head -1)
fi
echo "Using: $ECHO_TGZ"

# Record digest before
globular pkg info echo 2>&1 | grep -i -E "digest|checksum" | tee /tmp/phase1-test/t1.4-digest-before.txt

# Publish same package
globular pkg publish --file "$ECHO_TGZ" --repository 10.0.0.8:443 2>&1 | tee /tmp/phase1-test/t1.4.txt

# Record digest after
globular pkg info echo 2>&1 | grep -i -E "digest|checksum" | tee /tmp/phase1-test/t1.4-digest-after.txt

# Verify unchanged
diff /tmp/phase1-test/t1.4-digest-before.txt /tmp/phase1-test/t1.4-digest-after.txt

# Check repo log
journalctl -u globular-repository.service --no-pager -n 20 2>&1 | grep -E "idempotent|re-upload" | tee /tmp/phase1-test/t1.4-log.txt
```

**Pass:** Publish returns success. Diff shows no changes. Log contains `idempotent`.

**Stop if:** Digest changed (artifact was modified by re-upload).

**Evidence:** `t1.4.txt`, `t1.4-digest-before.txt`, `t1.4-digest-after.txt`, `t1.4-log.txt`

---

## Test 1.5 — Overwrite prevention (critical)

**Preconditions:** Test 1.4 passed. Know the current echo version and build number.

**Commands:**

```bash
# Read current version/build from baseline
cat /tmp/phase1-test/baseline-pkg-info.txt
# Set these manually from the output:
ECHO_VER="0.0.8"    # <-- adjust
ECHO_BUILD=1         # <-- adjust

# Record artifact state before
globular pkg info echo 2>&1 | tee /tmp/phase1-test/t1.5-before.txt

# Build a different binary
cd /home/dave/Documents/github.com/globulario/services/golang
echo '// phase1-test-marker' >> echo/echo_server/main.go
go build -buildvcs=false -o /tmp/phase1-test/echo_server ./echo/echo_server
git checkout -- echo/echo_server/main.go

# Package at same version+build
mkdir -p /tmp/phase1-test/payload/bin
cp /tmp/phase1-test/echo_server /tmp/phase1-test/payload/bin/echo_server
cd /home/dave/Documents/github.com/globulario/services
globular pkg build \
    --spec generated/specs/echo_service.yaml \
    --root /tmp/phase1-test/payload \
    --version $ECHO_VER --build-number $ECHO_BUILD \
    --out /tmp/phase1-test/ 2>&1 | tee /tmp/phase1-test/t1.5-build.txt

# Attempt publish — should FAIL
globular pkg publish \
    --file /tmp/phase1-test/echo_${ECHO_VER}_linux_amd64.tgz \
    --repository 10.0.0.8:443 2>&1 | tee /tmp/phase1-test/t1.5-publish.txt

# Verify artifact unchanged
globular pkg info echo 2>&1 | tee /tmp/phase1-test/t1.5-after.txt
diff /tmp/phase1-test/t1.5-before.txt /tmp/phase1-test/t1.5-after.txt

# Check repo log
journalctl -u globular-repository.service --no-pager -n 10 2>&1 | grep -E "forbidden|AlreadyExists" | tee /tmp/phase1-test/t1.5-log.txt
```

**Pass:** Publish output contains `forbidden` or `AlreadyExists` or `FAILED`. Diff shows no changes. Repo log contains rejection.

**Stop if:** Publish succeeds (output contains `SUCCESS`) — means overwrite protection is NOT working. **STOP ALL TESTING. ROLLBACK.**

**Evidence:** `t1.5-build.txt`, `t1.5-publish.txt`, `t1.5-before.txt`, `t1.5-after.txt`, `t1.5-log.txt`

---

## Test 1.6 — Deploy collision via manual publish

**Preconditions:** Test 1.5 passed. Test 1.5 artifacts still available.

**Commands:**

```bash
# Same artifact from Test 1.5, same version/build — attempt again
# (Confirms rejection is consistent, not a one-time fluke)
globular pkg publish \
    --file /tmp/phase1-test/echo_${ECHO_VER}_linux_amd64.tgz \
    --repository 10.0.0.8:443 2>&1 | tee /tmp/phase1-test/t1.6.txt
```

**Pass:** Output contains `forbidden` or `AlreadyExists` or `FAILED`.

**Stop if:** Output contains `SUCCESS`.

**Evidence:** `t1.6.txt`

---

## Cleanup

```bash
# Remove test artifacts (do NOT remove evidence)
rm -f /tmp/phase1-test/echo_server
rm -rf /tmp/phase1-test/payload
rm -f /tmp/phase1-test/echo_*.tgz

# Bundle evidence
tar czf /tmp/phase1-tier1-evidence.tar.gz /tmp/phase1-test/
echo "Evidence saved to /tmp/phase1-tier1-evidence.tar.gz"
```

---

## Result Table

| Test | Description | Pass/Fail | Evidence File | Notes |
|------|-------------|-----------|---------------|-------|
| 1.1 | Non-existent desired rejected | | t1.1.txt | |
| 1.2 | Second non-existent rejected | | t1.2.txt | |
| 1.3 | Valid desired accepted | | t1.3.txt | |
| 1.4 | Idempotent publish | | t1.4.txt | |
| 1.5 | Overwrite prevention | | t1.5-publish.txt | CRITICAL |
| 1.6 | Collision rejection | | t1.6.txt | |

**Tier 1 verdict:** All 6 pass → proceed to Tier 2. Any fail → stop, investigate, consider rollback.
