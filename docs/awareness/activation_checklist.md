# Awareness Activation Checklist

**Purpose:** Verify that awareness is operationally wired, not just installed.
Each section is a discrete deployment step with exact proof commands.
Do not mark a section done until the proof commands produce the expected output.

**Cross-reference:** `docs/awareness/operational_handoff.md` contains full explanations.
**Status tracking:** Copy `activation_status.example.json` to `.awareness/activation_status.json` and fill in as you complete each section.

---

## A. Schedule health_pulse

**Criticism closed:** "Awareness is only reactive — tools only run when someone calls them."
**Requirement:** `awareness.health_pulse` runs on a schedule without any human or Claude invocation.

### A.1 Install the scheduler

**Option 1 — systemd timer (preferred on Linux cluster nodes)**

```bash
# Copy unit files from operational_handoff.md section 1b, then:
sudo systemctl daemon-reload
sudo systemctl enable --now awareness-health-pulse.timer
```

**Option 2 — cron (fallback)**

```bash
# Add to crontab — run as the globular service user:
sudo -u globular crontab -e
# Paste the cron line from operational_handoff.md section 1a
```

### A.2 Verify the timer is active

```bash
systemctl list-timers awareness-health-pulse.timer --no-pager
# Expected: one row with NEXT and LAST timestamps populated
# LAST must not be empty after the first firing (5 min after boot for systemd)
```

```bash
# Check that the service ran without error:
journalctl -u awareness-health-pulse.service -n 20 --no-pager
# Expected: output containing checked_at and status fields
# No lines containing "Failed" or "error"
```

### A.3 Verify the pulse log exists and has content

```bash
# If using the log-file cron variant:
ls -lh /var/log/globular/awareness-health.log
tail -5 /var/log/globular/awareness-health.log
# Expected: lines with ISO timestamps and status=healthy/warning/critical

# If using journald (systemd timer variant):
journalctl -u awareness-health-pulse.service --since "1 hour ago" --no-pager \
  | grep -E "status=|exit_code="
# Expected: at least one line per timer firing
```

### A.4 Verify exit codes are interpreted correctly

```bash
# Manually invoke once and check the exit code:
globular awareness mcp-call awareness.health_pulse \
  --arg stale_proposal_hours=24 \
  --arg include_graph_check=true
echo "exit: $?"
# Expected exit codes:
#   0 = healthy
#   1 = warning  (stale proposals, partial runtime, unverified gaps)
#   2 = critical (graph stale, core invariant violated)
#   3 = check failed (MCP unreachable, docs dir not set)
```

### Closure condition

> `awareness.health_pulse` runs without Claude or manual chat invocation.

### Proof commands

```bash
# All three must pass:

# 1. Timer is listed and has fired at least once:
systemctl list-timers awareness-health-pulse.timer --no-pager \
  | grep awareness-health-pulse

# 2. A pulse report exists with a checked_at timestamp:
journalctl -u awareness-health-pulse.service -n 5 --no-pager \
  | grep -c "checked_at"
# Expected: integer ≥ 1

# 3. The most recent exit code is 0 or 1 (not 2 or 3):
journalctl -u awareness-health-pulse.service -n 1 --no-pager \
  | grep -E "exit_code.: [01]"
# Expected: one matching line
```

---

## B. Wire CI strict verification

**Criticism closed:** "`strict_verified` is operationally unreachable — no CI pipeline feeds test results into self_review."
**Requirement:** At least one implemented gap reaches `strict_verified` in CI output. CI fails on `tests_not_found` and `invalid_metadata`.

### B.1 Build the converter

```bash
cd golang
go build -o bin/go-test-to-awareness \
  ./awareness/cmd/go-test-to-awareness
# Binary should exist:
ls -lh bin/go-test-to-awareness
```

### B.2 Run tests with JSON output and convert

```bash
mkdir -p .awareness
go test -json ./awareness/... -timeout 120s \
  | ./golang/bin/go-test-to-awareness \
      --command "go test -json ./awareness/..." \
      --output .awareness/test-results.json
# File must exist and be non-empty:
ls -lh .awareness/test-results.json
jq '.passed, .packages, (.tests | length)' .awareness/test-results.json
# Expected: true, <integer ≥ 1>, <integer ≥ 1>
```

### B.3 Run self_review with test results file

```bash
globular awareness mcp-call awareness.self_review \
  --arg feedback="CI verification pass" \
  --arg test_results_file=".awareness/test-results.json" \
  | jq '.closed_gaps[] | {gap_id, verification_status}' \
  | head -40
# At least one gap must show: "verification_status": "strict_verified"
```

### B.4 Add to CI pipeline

Add the following steps to `.github/workflows/ci.yml` (or equivalent):

```yaml
- name: Build go-test-to-awareness
  run: go build -o golang/bin/go-test-to-awareness ./golang/awareness/cmd/go-test-to-awareness

- name: Run awareness tests and generate CI evidence
  working-directory: golang
  run: |
    mkdir -p ../.awareness
    go test -json ./awareness/... -timeout 120s \
      | ./bin/go-test-to-awareness \
          --command "go test -json ./awareness/..." \
          --output ../.awareness/test-results.json
    jq -e '.passed == true' ../.awareness/test-results.json \
      || { echo "::error::Awareness test suite failed"; exit 1; }

- name: Check awareness strict_verified (no regression)
  run: |
    RESULT=$(globular awareness mcp-call awareness.self_review \
      --arg feedback="CI verification pass" \
      --arg test_results_file=".awareness/test-results.json")
    NOT_OK=$(echo "$RESULT" | jq \
      '[.closed_gaps[] | select(.verification_status == "tests_not_found"
         or .verification_status == "invalid_metadata"
         or .verification_status == "tests_failed")] | length')
    if [ "$NOT_OK" -gt 0 ]; then
      echo "::error::$NOT_OK gap(s) have missing, invalid, or failed test evidence"
      echo "$RESULT" | jq '.closed_gaps[] | select(.verification_status == "tests_not_found" or .verification_status == "invalid_metadata" or .verification_status == "tests_failed")'
      exit 1
    fi
    STRICT=$(echo "$RESULT" | jq '[.closed_gaps[] | select(.verification_status == "strict_verified")] | length')
    echo "strict_verified gaps: $STRICT"

- name: Upload awareness test evidence
  uses: actions/upload-artifact@v4
  with:
    name: awareness-test-results
    path: .awareness/test-results.json
```

### Closure condition

> At least one implemented gap reports `strict_verified`.
> CI fails on `tests_not_found`, `invalid_metadata`, or `tests_failed` for required tests.

### Proof commands

```bash
# 1. test-results.json exists and reports passed=true:
jq -e '.passed == true' .awareness/test-results.json && echo "PASS"

# 2. At least one gap is strict_verified:
globular awareness mcp-call awareness.self_review \
  --arg feedback="proof check" \
  --arg test_results_file=".awareness/test-results.json" \
  | jq '[.closed_gaps[] | select(.verification_status == "strict_verified")] | length'
# Expected: integer ≥ 1

# 3. No gaps are tests_not_found or invalid_metadata:
globular awareness mcp-call awareness.self_review \
  --arg feedback="proof check" \
  --arg test_results_file=".awareness/test-results.json" \
  | jq '[.closed_gaps[] | select(.verification_status == "tests_not_found" or .verification_status == "invalid_metadata")] | length'
# Expected: 0
```

---

## C. Activate runtime sources

**Criticism closed:** "Runtime awareness is static unless cluster addresses and credentials are wired."
**Requirement:** `awareness.runtime_activation_check` reports `live` or `partial` — not `noop`.

### C.1 Bootstrap the config (dry-run first)

Run on a cluster node where Globular is installed:

```bash
globular awareness mcp-call awareness.runtime_config_bootstrap \
  --arg globular_config_dir=/var/lib/globular/config \
  --arg output_config_path=.awareness/runtime_sources.yaml \
  --arg write=false
# Review the detected values and missing fields in the output.
# Do not proceed if CACert or ControllerAddr is missing.
```

### C.2 Generate the sample config

```bash
globular awareness mcp-call awareness.runtime_config_bootstrap \
  --arg globular_config_dir=/var/lib/globular/config \
  --arg output_config_path=.awareness/runtime_sources.yaml \
  --arg write=true
cat .awareness/runtime_sources.yaml
# Verify addresses look correct for this cluster.
# client_key is intentionally absent — add it manually.
```

### C.3 Apply to MCP server config

```bash
# Edit /var/lib/globular/mcp/config.json
# Merge the awareness.runtime_sources values from step C.2
# Add client_key manually — it was NOT written by the bootstrap tool:
sudo nano /var/lib/globular/mcp/config.json

# Example target state in config.json:
# "awareness": {
#   "controller_addr": "globular.internal:12000",
#   "doctor_addr":     "globular.internal:12005",
#   "workflow_addr":   "globular.internal:10004",
#   "prometheus_addr": "http://globular.internal:9090",
#   "ca_cert":         "/var/lib/globular/pki/ca.crt",
#   "client_cert":     "/var/lib/globular/pki/issued/services/service.crt",
#   "client_key":      "/var/lib/globular/pki/issued/services/service.key"
# }
```

### C.4 Restart the MCP server

```bash
sudo systemctl restart globular-mcp
sudo systemctl is-active globular-mcp
# Expected: active
```

### C.5 Verify sources are live

```bash
globular awareness mcp-call awareness.runtime_activation_check \
  --arg check_connectivity=true \
  --arg check_credentials=true
# Check runtime_awareness_status field:
#   "live"         — all 4 sources configured and reachable
#   "partial"      — some configured — acceptable if intentional
#   "noop"         — still not wired — go back to C.3
#   "misconfigured"— addresses set but TLS broken — check cert paths
```

### Closure condition

> `awareness.runtime_activation_check` reports `live` or `partial`.
> At least one source has `configured: true` and `connectivity: "ok"`.

### Proof commands

```bash
# 1. Status is not noop:
globular awareness mcp-call awareness.runtime_activation_check \
  | jq -e '.runtime_awareness_status != "noop"' && echo "PASS"

# 2. At least one source is configured and reachable:
globular awareness mcp-call awareness.runtime_activation_check \
  --arg check_connectivity=true \
  | jq '[.sources[] | select(.configured == true and .connectivity == "ok")] | length'
# Expected: integer ≥ 1

# 3. No source has an unreadable credential error:
globular awareness mcp-call awareness.runtime_activation_check \
  --arg check_credentials=true \
  | jq '[.sources[] | select(.last_error != null and .last_error != "")] | length'
# Expected: 0
```

**If noop is intentional** (local dev, no cluster): document it here and in `.awareness/activation_status.json` with `acknowledged: true`. Do not revisit this section until a cluster is available.

---

## D. Drain proposal queue

**Criticism closed:** "Proposal pipeline has no drain — proposals accumulate silently."
**Requirement:** No stale DRAFT/VALIDATED/APPROVED proposals remain unless explicitly acknowledged. The MCP server does not expose a promotion tool.

### D.1 Check queue health

```bash
globular awareness mcp-call awareness.proposal_queue_health \
  --arg draft_sla_hours=24
# Check queue_status:
#   "healthy"      — done, nothing to do
#   "stale"        — continue to D.2
#   "needs_review" — urgent, continue to D.2 immediately
#   "blocked"      — duplicate IDs, resolve before D.2
```

### D.2 Get the review plan

```bash
globular awareness mcp-call awareness.proposal_review_plan
# Act on each bucket in order:
# 1. invalid_schema       → fix or delete broken YAML
# 2. safe_to_reject_duplicates → delete all but one copy of each ID
# 3. validate_now         → continue to D.3
# 4. needs_human_review   → continue to D.4
# 5. approved_waiting_promotion → continue to D.5
```

### D.3 Batch validate DRAFT proposals

```bash
globular awareness mcp-call awareness.validate_proposal_batch
# Review entries[]. Fix any "status": "invalid" proposals manually.
# This command never approves or modifies proposals.
```

### D.4 Human review and approval (no tool — manual edit)

```bash
# List proposals awaiting review:
ls docs/awareness/proposals/*.yaml

# For each DRAFT or VALIDATED proposal, read it:
cat docs/awareness/proposals/<proposal-id>.yaml

# Approve: change status field in the YAML file
# DRAFT → VALIDATED (first review)
# VALIDATED → APPROVED (second reviewer sign-off)
sed -i 's/^  status: DRAFT/  status: VALIDATED/' \
  docs/awareness/proposals/<proposal-id>.yaml

# Reject if wrong or duplicate:
sed -i 's/^  status: DRAFT/  status: REJECTED/' \
  docs/awareness/proposals/<proposal-id>.yaml
echo "  rejected_reason: \"<reason>\"" \
  >> docs/awareness/proposals/<proposal-id>.yaml
```

**There is no tool that approves proposals. Approval is always a human edit.**

### D.5 Promote approved proposals (CLI only — not MCP)

```bash
# 1. Merge the proposal content into the target knowledge YAML manually.
#    (failure_modes.yaml, invariants.yaml, forbidden_fixes.yaml, etc.)

# 2. Mark the proposal as promoted:
sed -i 's/^  status: APPROVED/  status: PROMOTED/' \
  docs/awareness/proposals/<proposal-id>.yaml

# Or, if the globular CLI has promote-proposal integrated:
globular awareness promote-proposal <proposal-id>
```

### D.6 Verify queue is healthy

```bash
globular awareness mcp-call awareness.proposal_queue_health
# Expected: queue_status = "healthy", stale_proposals = []

# Also verify MCP does NOT expose a promotion tool:
globular awareness mcp-call tools/list \
  | jq '[.tools[] | select(.name | contains("promote"))] | length'
# Expected: 0
```

### Closure condition

> `proposal_queue_health` reports `healthy` or all remaining stale proposals are explicitly acknowledged.
> No MCP promotion tool is exposed.

### Proof commands

```bash
# 1. Queue is healthy:
globular awareness mcp-call awareness.proposal_queue_health \
  | jq -e '.queue_status == "healthy"' && echo "PASS"

# 2. Stale count is zero:
globular awareness mcp-call awareness.proposal_queue_health \
  | jq '.counts.stale'
# Expected: 0

# 3. MCP does not expose a promotion tool:
globular awareness mcp-call tools/list \
  | jq -e '[.tools[] | select(.name | contains("promote"))] | length == 0' \
  && echo "PASS: no promotion tool exposed via MCP"
```

---

## Full activation proof (run all four)

When all four sections are complete, this compound check must pass without error:

```bash
#!/usr/bin/env bash
set -euo pipefail

echo "=== A. Scheduler ==="
systemctl list-timers awareness-health-pulse.timer --no-pager | grep awareness-health-pulse
journalctl -u awareness-health-pulse.service -n 1 --no-pager | grep -E "checked_at|status="

echo "=== B. CI strict_verified ==="
jq -e '.passed == true' .awareness/test-results.json
globular awareness mcp-call awareness.self_review \
  --arg feedback="activation proof" \
  --arg test_results_file=".awareness/test-results.json" \
  | jq -e '[.closed_gaps[] | select(.verification_status == "strict_verified")] | length > 0'

echo "=== C. Runtime sources ==="
globular awareness mcp-call awareness.runtime_activation_check \
  | jq -e '.runtime_awareness_status != "noop"'

echo "=== D. Proposal queue ==="
globular awareness mcp-call awareness.proposal_queue_health \
  | jq -e '.queue_status == "healthy"'
globular awareness mcp-call tools/list \
  | jq -e '[.tools[] | select(.name | contains("promote"))] | length == 0'

echo ""
echo "All activation checks passed. Awareness is operationally wired."
```

---

## Sections that may be acknowledged as noop

Some sections may not apply to every deployment. When a section is intentionally skipped, record it in `.awareness/activation_status.json`:

- **C (runtime sources):** Acceptable as noop for local dev environments with no cluster. Set `acknowledged: true` and `reason`.
- **B (CI):** Acceptable to skip if the repo has no CI pipeline yet. Tests still run locally. Set `acknowledged: true`.

**Sections A and D are not acknowledgeable** — a stale proposal queue or an unscheduled health pulse is always an active gap.
