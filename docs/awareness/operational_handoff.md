# Awareness — Operational Handoff

Phase 9 moves awareness from a conversation-driven tool to a schedulable operational loop.
This document is the runbook for an operator wiring it up.

**What Phase 9 adds:**
- `awareness.health_pulse` — one schedulable command that aggregates all self-health checks
- CI test results import — `go-test-to-awareness` converter + `test_results_file` param on `self_review`
- `awareness.runtime_config_bootstrap` — detect and generate runtime source config
- `awareness.proposal_review_plan` + `awareness.validate_proposal_batch` — proposal drain helpers

**What Phase 9 does NOT add:**
- No daemon. No autonomous remediation. No auto-promotion.
- `health_pulse` reports status and exits. A scheduler calls it.
- Human approval is required before any knowledge change is promoted.

---

## 1. Scheduling awareness.health_pulse

### 1a. cron

Add to the operator's crontab (`crontab -e`) on the node running the MCP server:

```cron
# Run awareness health pulse every 30 minutes.
# Exit 0 = healthy, 1 = warning, 2 = critical, 3 = check failed.
# Redirect output to a log file for review.
*/30 * * * * globular awareness mcp-call awareness.health_pulse \
  --arg stale_proposal_hours=24 \
  >> /var/log/globular/awareness-health.log 2>&1 \
  || echo "$(date -Iseconds) awareness.health_pulse exit $?" \
     >> /var/log/globular/awareness-health-alerts.log
```

If the MCP server is accessed via the network rather than a local binary:

```cron
*/30 * * * * curl -sf -X POST https://globular.internal:10260/mcp \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"awareness.health_pulse","arguments":{"stale_proposal_hours":24}}}' \
  | jq -r '.result.content[0].text | fromjson | "\(.checked_at) status=\(.status) exit=\(.exit_code)"' \
  >> /var/log/globular/awareness-health.log 2>&1
```

### 1b. systemd timer

Create two files:

**`/etc/systemd/system/awareness-health-pulse.service`**
```ini
[Unit]
Description=Awareness health pulse check
After=network.target

[Service]
Type=oneshot
User=globular
# Exit codes: 0=healthy 1=warning 2=critical 3=check_failed
ExecStart=/usr/local/bin/globular awareness mcp-call awareness.health_pulse \
  --arg stale_proposal_hours=24 \
  --arg include_graph_check=true
StandardOutput=journal
StandardError=journal
# Do not restart on failure — the timer handles re-scheduling.
# A non-zero exit surfaces in journalctl and can be picked up by alerting.
```

**`/etc/systemd/system/awareness-health-pulse.timer`**
```ini
[Unit]
Description=Run awareness health pulse every 30 minutes
Requires=awareness-health-pulse.service

[Timer]
OnBootSec=5min
OnUnitActiveSec=30min
AccuracySec=1min
Persistent=true

[Install]
WantedBy=timers.target
```

Enable and start:
```bash
systemctl daemon-reload
systemctl enable --now awareness-health-pulse.timer
# Verify
systemctl list-timers awareness-health-pulse.timer
# Check last run
journalctl -u awareness-health-pulse.service -n 50
```

**Exit code interpretation:**

| Code | Meaning | Action |
|------|---------|--------|
| 0 | healthy | Nothing required |
| 1 | warning | Review `awareness.proposal_queue_health` or `awareness.coverage_report` |
| 2 | critical | Graph stale or core invariant violated — rebuild graph, check failure modes |
| 3 | check failed / invalid config | MCP server unreachable or docs dir not configured |

---

## 2. CI workflow: go test → strict_verified

This wires `awareness.self_review` so that implemented gaps can reach `strict_verified` (existence + execution evidence), not just `tests_found` (existence only).

### Step-by-step

**Build the converter once (add to CI build step or repo binary):**
```bash
go build -o bin/go-test-to-awareness \
  ./awareness/cmd/go-test-to-awareness
```

**CI workflow step (GitHub Actions example):**

```yaml
- name: Run awareness tests and generate CI evidence
  run: |
    mkdir -p .awareness

    # 1. Run tests with JSON output
    go test -json ./awareness/... \
      -timeout 120s \
      2>&1 | ./bin/go-test-to-awareness \
               --command "go test -json ./awareness/..." \
               --output .awareness/test-results.json

    # 2. Check if the suite passed overall
    PASSED=$(jq '.passed' .awareness/test-results.json)
    if [ "$PASSED" != "true" ]; then
      echo "::error::Awareness test suite failed — see .awareness/test-results.json"
      exit 1
    fi

- name: Check awareness self_review strict_verified
  run: |
    # 3. Call self_review with the CI evidence file.
    #    This upgrades tests_found → strict_verified for gaps whose
    #    required tests appear in the passed list.
    RESULT=$(curl -sf -X POST https://globular.internal:10260/mcp \
      -H 'Content-Type: application/json' \
      -d "{
        \"jsonrpc\":\"2.0\",\"id\":1,
        \"method\":\"tools/call\",
        \"params\":{
          \"name\":\"awareness.self_review\",
          \"arguments\":{
            \"feedback\":\"CI verification pass\",
            \"test_results_file\":\".awareness/test-results.json\"
          }
        }
      }" | jq -r '.result.content[0].text | fromjson')

    # 4. Fail CI if any implemented gap regresses below tests_found.
    NOT_FOUND=$(echo "$RESULT" | jq '[.closed_gaps[] | select(.verification_status == "tests_not_found" or .verification_status == "invalid_metadata")] | length')
    if [ "$NOT_FOUND" -gt 0 ]; then
      echo "::error::$NOT_FOUND implemented gap(s) have missing or invalid test evidence"
      echo "$RESULT" | jq '.closed_gaps[] | select(.verification_status == "tests_not_found" or .verification_status == "invalid_metadata")'
      exit 1
    fi

    echo "Self-review complete. Gaps at strict_verified or tests_found. No regression."
```

### What the test-results.json file looks like

```json
{
  "command": "go test -json ./awareness/...",
  "started_at": "2026-05-07T18:00:00Z",
  "finished_at": "2026-05-07T18:01:23Z",
  "passed": true,
  "packages": 21,
  "tests": [
    {"name": "TestOfflineDiagnose_EtcdNspace", "package": "golang/awareness/mcp", "status": "passed", "duration_ms": 42},
    {"name": "TestHealthPulse_Healthy",        "package": "golang/awareness/mcp", "status": "passed", "duration_ms": 8}
  ],
  "failed_tests": [],
  "skipped_tests": []
}
```

**Verification level ladder** (ascending strength):

```
invalid_metadata        ← tests_required contains non-TestXxx entries
tests_not_found         ← required tests don't exist in codebase
tests_partial           ← some required tests found, others missing
tests_found             ← all required tests found (existence only, no CI evidence)
tests_found_but_skipped ← tests exist but t.Skip was called
tests_failed            ← required test(s) appeared in failed_tests
tests_passed            ← CI evidence present but passed=false globally
strict_verified         ← all required tests found AND CI evidence confirms pass
```

CI gates should fail on `tests_not_found` and `invalid_metadata`. `tests_found` is acceptable if CI evidence is not wired. `strict_verified` is the gold standard.

---

## 3. Runtime sources config

By default the awareness runtime bridge is **noop** — it reads static YAML, not live cluster state. To wire live sources, the awareness block in `/var/lib/globular/mcp/config.json` must include cluster addresses and TLS credentials.

### 3a. Bootstrap (detect existing config)

Run on a cluster node:
```bash
# Dry-run: detect and show what would be generated
globular awareness mcp-call awareness.runtime_config_bootstrap \
  --arg globular_config_dir=/var/lib/globular/config \
  --arg output_config_path=.awareness/runtime_sources.yaml \
  --arg write=false

# Write the sample config
globular awareness mcp-call awareness.runtime_config_bootstrap \
  --arg globular_config_dir=/var/lib/globular/config \
  --arg output_config_path=.awareness/runtime_sources.yaml \
  --arg write=true
```

### 3b. Sample runtime_sources.yaml (generated output)

```yaml
# Awareness runtime sources config — generated by awareness.runtime_config_bootstrap
# Review and adjust addresses before use.
# Never commit credentials to source control.
awareness:
  runtime_sources:
    controller_addr: "globular.internal:12000"
    doctor_addr:     "globular.internal:12005"
    workflow_addr:   "globular.internal:10004"
    prometheus_addr: "http://globular.internal:9090"
    ca_cert:         "/var/lib/globular/pki/ca.crt"
    client_cert:     "/var/lib/globular/pki/issued/services/service.crt"
    # client_key: /path/to/service.key  # set this manually
```

### 3c. Apply to MCP server config

Merge the generated values into `/var/lib/globular/mcp/config.json`:

```json
{
  "tool_groups": { "awareness": true },
  "awareness": {
    "db_path":          "/path/to/.globular/awareness/graph.db",
    "repo_path":        "/path/to/globulario/services",
    "docs_dir":         "/path/to/docs/awareness",
    "controller_addr":  "globular.internal:12000",
    "doctor_addr":      "globular.internal:12005",
    "workflow_addr":    "globular.internal:10004",
    "prometheus_addr":  "http://globular.internal:9090",
    "ca_cert":          "/var/lib/globular/pki/ca.crt",
    "client_cert":      "/var/lib/globular/pki/issued/services/service.crt",
    "client_key":       "/var/lib/globular/pki/issued/services/service.key"
  }
}
```

Restart the MCP server after editing, then verify:
```bash
globular awareness mcp-call awareness.runtime_activation_check \
  --arg check_connectivity=true
```

Expected output when live:
```json
{
  "runtime_awareness_status": "live",
  "sources": [
    {"source": "controller", "configured": true, "connectivity": "ok"},
    {"source": "doctor",     "configured": true, "connectivity": "ok"},
    {"source": "workflow",   "configured": true, "connectivity": "ok"},
    {"source": "prometheus", "configured": true, "connectivity": "ok"}
  ]
}
```

**Safety notes:**
- `runtime_config_bootstrap` never prints private key contents — only reports "present" or "missing"
- Never commit `client_key` paths or values to source control
- `client_key` must be set manually in the MCP config — the bootstrap tool intentionally omits it from the written file

---

## 4. Proposal queue runbook

Run this sequence when the health pulse reports `proposal_queue.stale` alerts.

### Step 1 — Check queue health

```bash
globular awareness mcp-call awareness.proposal_queue_health \
  --arg draft_sla_hours=24 \
  --arg validated_sla_hours=24 \
  --arg approved_sla_hours=24
```

Look at `queue_status`:
- `healthy` — nothing needed
- `stale` — proposals past SLA, see `stale_proposals[]`
- `needs_review` — at least one proposal is in `NEEDS_REVIEW` state — act immediately
- `blocked` — duplicate proposal IDs detected — resolve before proceeding

### Step 2 — Get the review plan

```bash
globular awareness mcp-call awareness.proposal_review_plan
```

Output groups proposals into:
```json
{
  "validate_now":               ["proposal-id-a"],
  "needs_human_review":         ["proposal-id-b"],
  "approved_waiting_promotion": ["proposal-id-c"],
  "safe_to_reject_duplicates":  [],
  "invalid_schema":             []
}
```

Act on each bucket in this order:
1. `invalid_schema` — fix or delete the broken YAML files
2. `safe_to_reject_duplicates` — delete all but one copy
3. `validate_now` — run batch validation (Step 3)
4. `needs_human_review` — read the YAML and make a decision (Step 4)
5. `approved_waiting_promotion` — promote (Step 5)

### Step 3 — Batch validate DRAFT proposals

```bash
globular awareness mcp-call awareness.validate_proposal_batch
```

This checks proposal schema (id, status, created_at present) without approving anything. Review the `entries[]` list. Fix any `invalid` proposals manually.

**This tool never approves. It never modifies files.**

### Step 4 — Human review and approval

Open each proposal file under `docs/awareness/proposals/`:

```bash
ls docs/awareness/proposals/*.yaml
cat docs/awareness/proposals/<proposal-id>.yaml
```

If the proposal is correct:
1. Edit the file: change `status: DRAFT` → `status: VALIDATED`
2. Have a second operator review: change `status: VALIDATED` → `status: APPROVED`

If the proposal is wrong or duplicate:
1. Change `status: DRAFT` → `status: REJECTED`
2. Add a `rejected_reason:` field explaining why

**There is no tool that approves proposals. Approval is always a human edit.**

### Step 5 — Promote approved proposals (CLI only)

Once a proposal has `status: APPROVED`, merge its content into the appropriate knowledge YAML file manually, then mark it promoted:

```bash
# Edit the knowledge YAML (failure_modes.yaml, invariants.yaml, etc.)
# then mark the proposal done:
sed -i 's/status: APPROVED/status: PROMOTED/' \
  docs/awareness/proposals/<proposal-id>.yaml
```

Or use the `markProposalPromoted` helper if integrated into a CLI command:
```bash
globular awareness promote-proposal <proposal-id>
```

**`awareness.promote_approved_proposals` is NOT available via MCP.**
Promotion is intentionally CLI-only. The MCP selfcheck invariant
`awareness.mcp_must_not_expose_promotion` enforces this at startup.

---

## 5. Safety notes

### No auto-promotion

Awareness never modifies knowledge YAML files autonomously. The full lifecycle is:

```
AI proposes → DRAFT file created
Operator validates → DRAFT → VALIDATED
Second operator approves → VALIDATED → APPROVED
Operator manually merges content → APPROVED → PROMOTED
```

No step is skipped. No tool shortcircuits this chain.

### health_pulse is scheduled reporting, not remediation

`awareness.health_pulse` reads state and reports it. It does not:
- Restart services
- Modify proposals
- Rebuild the graph
- Apply fixes

The exit code is for the scheduler to act on (e.g., send an alert). The operator decides what to do.

### Runtime config must not expose private keys

`awareness.runtime_config_bootstrap` reports whether the client key file is "present" or "missing". It never prints the key path in the sample config. Set `client_key` manually in `/var/lib/globular/mcp/config.json` after reading the bootstrap output.

Never commit `config.json` with real `client_key` values to source control.

### Runtime bridge is opt-in

If `controller_addr`, `doctor_addr`, `workflow_addr`, and `prometheus_addr` are all empty in the MCP config, every runtime source degrades gracefully to noop. The awareness system continues to work with static YAML knowledge. Run `awareness.runtime_activation_check` to confirm which mode is active.

---

## 6. How to know awareness is operationally wired

Run `awareness.health_pulse` and verify:

```
status: "healthy"
exit_code: 0
sections.runtime_sources.runtime_awareness_status: "live"   (not "noop" or "partial")
sections.proposal_queue.queue_status: "healthy"
sections.graph_freshness.stale: false
sections.self_review_verification.tests_not_found: 0
sections.self_review_verification.invalid_metadata: 0
sections.coverage.components_without_failure_modes: 0        (aspirational)
alerts: []
```

**Minimum viable operational state:**

| Check | Minimum acceptable |
|-------|--------------------|
| `health_pulse` exit code | 0 or 1 (not 2) |
| `runtime_sources.status` | `ok` (live) or `warn` (noop, acknowledged) |
| `proposal_queue.stale_proposals` | 0 |
| `self_review_verification.tests_not_found` | 0 |
| `self_review_verification.invalid_metadata` | 0 |
| `graph_freshness.stale` | false |
| CI step present | `go-test-to-awareness` wired, `.awareness/test-results.json` produced |
| Scheduler present | cron or systemd timer running `health_pulse` at ≤60 min interval |

**If `runtime_sources.status` is `warn` (noop) and that is acceptable** (e.g., local dev or no cluster), document it explicitly so future reviews don't repeat the same criticism.

---

*Generated: Phase 9 operational handoff. No new awareness features — scheduling, CI evidence, runtime config, and proposal drain helpers only.*
