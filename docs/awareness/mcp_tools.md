# Awareness MCP Tools

This document defines the MCP tool interface for the Globular awareness system.
These tools wrap the `globular awareness` CLI commands into typed, agent-consumable
JSON APIs.

The primary tool is `awareness.preflight`. All other tools are surfaced for
fine-grained queries when the agent needs a specific sub-result.

---

## awareness.preflight

The front door for any AI agent before editing Globular code.

Runs alias matching, agent context, did-we-fix, impact analysis, package admission,
cycle detection, and fix-ledger lookup — returns a single structured report.

### Input

```json
{
  "task": "string (required)",
  "files": ["string"],
  "package_path": "string",
  "phase": "string",
  "format": "json"
}
```

| Field | Type | Description |
|-------|------|-------------|
| `task` | string | What the agent intends to do |
| `files` | []string | Files the agent plans to edit |
| `package_path` | string | Path to package dir with `awareness.yaml` |
| `phase` | string | Dependency phase: `recovery`, `bootstrap`, `package_install`, `reconcile` |
| `format` | string | Always `"json"` for MCP callers |

### Output

```json
{
  "task": "string",
  "classification": ["ARCHITECTURE_SENSITIVE", "CONVERGENCE_RISK"],
  "matched_aliases": ["infra.desired_hash_consistency"],
  "services": ["envoy", "cluster-controller"],
  "packages": [],
  "files": ["golang/cluster_controller/convergence.go"],
  "invariants": ["infra.desired_hash_consistency"],
  "failure_modes": ["failure_mode.desired_hash_restart_storm"],
  "forbidden_fixes": ["use raw artifact digest as desired_hash"],
  "did_we_fix": {
    "status": "PARTIAL",
    "matched_patterns": ["desired_hash"],
    "fix_cases": ["desired_hash_consistency"],
    "remaining_gaps": ["golang/awareness/analysis/hash.go"],
    "next_action": "complete partial fix before closing"
  },
  "package_admission": null,
  "cycles": [],
  "required_tests": ["TestDriftWorkflowUsesDesiredHash"],
  "required_searches": ["ComputeInfrastructureDesiredHash"],
  "recommended_investigation_order": [
    "Check desired-hash computation",
    "Verify installed-state stamping"
  ],
  "agent_instruction": "This task is architecture-sensitive. ...",
  "warnings": []
}
```

### Classification values

| Class | Meaning |
|-------|---------|
| `LOCAL_CODE_CHANGE` | Safe local change with no architecture signals |
| `ARCHITECTURE_SENSITIVE` | Touches convergence, desired state, or runtime |
| `CONVERGENCE_RISK` | May break the convergence proof |
| `PACKAGE_ADMISSION` | Involves package install or awareness contract |
| `RUNTIME_INCIDENT` | Active crash, OOM, or panic |
| `RETRY_LOOP` | Infinite retry or loop detected |
| `RESTART_STORM` | SIGTERM storm or start-limit-hit |
| `STATE_MISMATCH` | Desired hash or build_id mismatch |
| `DEPENDENCY_CYCLE` | Circular dependency in graph |
| `UNKNOWN_IMPACT` | No invariants or failure modes matched |

---

## awareness.agent_context

Returns the architectural context for a task — invariants, failure modes, forbidden
fixes, and required tests — without the full preflight overhead.

### Input

```json
{
  "task": "string",
  "files": ["string"],
  "services": ["string"]
}
```

### Output

```json
{
  "invariants": [],
  "failure_modes": [],
  "forbidden_fixes": [],
  "required_tests": [],
  "required_searches": [],
  "services": []
}
```

---

## awareness.impact_file

Returns all graph nodes impacted by edits to a specific file.

### Input

```json
{
  "file": "golang/cluster_controller/convergence.go"
}
```

### Output

```json
{
  "source_file": "string",
  "services": [],
  "invariants": [],
  "failure_modes": [],
  "forbidden_fixes": [],
  "tests": [],
  "other": []
}
```

---

## awareness.did_we_fix

Queries whether a given task is covered by known fix cases.

### Input

```json
{
  "task": "string"
}
```

### Output

```json
{
  "status": "PARTIAL",
  "matched_patterns": [],
  "fix_cases": [],
  "remaining_gaps": [],
  "next_action": "string"
}
```

---

## awareness.validate_package

Validates a package's `awareness.yaml` against the admission rules and the main graph.

### Input

```json
{
  "package_path": "string"
}
```

### Output

```json
{
  "status": "ADMIT | WARN | BLOCK",
  "reasons": ["string"],
  "impacted_invariants": [],
  "dependency_cycles": [],
  "forbidden_fixes_found": [],
  "required_tests": [],
  "missing_tests": []
}
```

---

## awareness.propose_from_incident

Generates a draft proposal from an incident bundle YAML.

### Input

```json
{
  "incident_id": "string",
  "incident_path": "string"
}
```

### Output

```json
{
  "proposal_path": "string",
  "proposal_id": "string",
  "status": "DRAFT"
}
```

---

## awareness.validate_proposal

Validates a draft proposal against the 12 admission rules.

### Input

```json
{
  "proposal_path": "string"
}
```

### Output

```json
{
  "status": "PASS | FAIL | WARN",
  "findings": [
    { "rule": 1, "status": "PASS", "message": "..." }
  ]
}
```

---

## awareness.fix_status

Returns the fix-ledger status for a given fix case ID or pattern.

### Input

```json
{
  "pattern": "desired_hash"
}
```

### Output

```json
{
  "fix_cases": [
    {
      "id": "desired_hash_consistency",
      "status": "PARTIAL",
      "fixed_files": [],
      "remaining_files": [],
      "required_tests": []
    }
  ]
}
```

---

## Usage pattern for orchestrating agents

```
1. Call awareness.preflight with the task + files + phase
2. Check classification array for ARCHITECTURE_SENSITIVE, CONVERGENCE_RISK, RESTART_STORM
3. If forbidden_fixes is non-empty — refuse to implement those approaches
4. If did_we_fix.status is REGRESSED — report regression, do not add new workaround
5. If did_we_fix.status is PARTIAL — complete the existing fix, do not start over
6. Run required_tests before submitting any change
7. Report agent_instruction verbatim as the final constraint summary
```

All tools are read-only. They do not modify the graph or any YAML file.
Use the CLI commands (`promote-proposal`, `admit-package --commit`) for mutations.

---

## Closed-Loop Learning

These tools complete the awareness feedback cycle: a verified fix becomes a
knowledge proposal that enters the normal approval and promotion pipeline.

### awareness.learn_from_fix

**Purpose:** Convert a verified fix into a draft awareness proposal.

**Flow:**
```
verified fix → learn_from_fix → proposals/learned-fix-<ts>.yaml → approve → promote → graph rebuild
```

**Input fields:** `symptom_text`, `root_cause`, `fix_summary` (required);
`incident_id`, `verification`, `changed_files`, `tests_added`, `known_bad_fix`,
`related_failure_mode`, `related_invariant` (optional).

**What it proposes:**
- A **failure mode** entry derived from the symptom/root-cause/fix
- A **forbidden fix** entry when `known_bad_fix` is provided
- A **scan rule** entry when Go files are changed and a detectable bad pattern is in the symptom
- An **invariant** entry when the root cause implies an architectural constraint

**Safety rules:**
- All proposals have `requires_human_approval: true`
- Proposals are saved only to `docs/awareness/proposals/` — never directly to knowledge YAML files
- Compatible with `pending_proposals` and `approve_proposal` tools

**NO_MATCH is not safe:** If the failure mode isn't in the knowledge base yet,
the tool synthesizes a proposal. Review it — don't skip.

---

### awareness.offline_diagnose

**Purpose:** Diagnose failures when gRPC sources are unavailable (cluster down,
node unreachable, early bootstrap).

**Input:** Any combination of `journalctl_text`, `systemd_status`, `etcdctl_output`,
`docker_compose_logs`, `logs_dir`, `workflow_receipts_dir`, `doctor_report_file`.

**Behavior:**
- Extracts events matching 15 known failure patterns (etcd NOSPACE, leader changes,
  port squatting, restart storms, TLS problems, MinIO disk failures, workflow stuck, etc.)
- Scores events against `failure_modes.yaml` and `invariants.yaml` using the same
  keyword scoring as the raw knowledge fallback
- Builds a time-ordered timeline when timestamps are present
- Returns `confidence` and `blind_spots` so you know what's missing

**Confidence is honest:** Low/unknown confidence means you should get runtime sources
before acting.

---

### awareness.causal_chain

**Purpose:** Identify multi-step failure chains from log evidence.

**Input:** `events` array or `offline_evidence` text. Optionally `live=true`
(requires cluster access).

**Behavior:**
- Loads `docs/awareness/knowledge/causal_rules.yaml`
- For each rule, scores how many sequence steps have matching evidence
- Returns chains where 50%+ of steps match
- Chains are sorted by confidence then step coverage

**Output fields:** `chain_id`, `confidence`, `root_signal`, `matched_steps`,
`total_steps`, `events` (per-step with evidence snippet), `explanation`,
`recommended_fix_order`, `blind_spots`.

**Heuristic — always check blind_spots.** The chain tool reports what matches
the known rules; it cannot detect what it doesn't know. An empty `chains` array
does NOT mean the cluster is healthy.

---

### awareness.scan_violations (AST mode)

The existing `scan_violations` tool now runs both regex and AST-based checks.

**AST patterns detected:**
- `loopback_string_literal` — string literal with `127.0.0.1` or `localhost`
- `loopback_in_const_or_var` — const/var initialized to loopback address
- `loopback_in_grpc_dial` — `grpc.Dial/NewClient/DialContext` with loopback first arg
- `loopback_in_http_call` — `http.Get/Post/NewRequest` with loopback URL
- `os_getenv_runtime_config` — `os.Getenv` in non-test file
- `exec_import_in_controller` — `os/exec` import in `cluster_controller` path
- `exec_command_in_high_risk` — `exec.Command` in `cluster_controller` or `workflow` path
- `retry_without_terminal` — `for` loop with `time.Sleep` and no terminal break (heuristic, low confidence)

AST findings have `scanner: "go_ast"`. Regex findings have `scanner: "regex"`.
Duplicate (file, line) findings are deduplicated — one finding per source line.

---

### Safety model summary

```
awareness proposes → humans approve → CLI promotes → graph rebuilds
```

1. NO tool directly edits `failure_modes.yaml`, `invariants.yaml`, `forbidden_fixes.yaml`, or any knowledge YAML.
2. Every learn_from_fix output goes to `proposals/` and is `DRAFT` status.
3. NO_MATCH in preflight ≠ safe. Always grep the raw YAML files if the graph misses.
4. Causal chain confidence is heuristic. Use with offline_diagnose and explain_symptom together.

---

## Closed-Loop Review Mode

Two tools convert criticism/feedback into structured, testable requirements so that every gap has a closure condition and prevents repeat critique.

### awareness.self_review

Converts a block of feedback or criticism into structured capability gap requirements. Each matched criticism becomes a gap with requirement, implementation plan, tests, closure condition, and `prevents_repeat_criticism`. Already-implemented gaps go into `closed_gaps`. Vague feedback that matches no known pattern is marked `incomplete` — never invented.

**Algorithm:** Pure keyword matching against `docs/awareness/knowledge/agent_playbooks.yaml`. No LLM, no external calls.

#### Input

```json
{
  "goal": "make awareness prevent known dangerous code patterns",
  "feedback": "scan_violations is grep-based, not AST-based. There is no causal chain tool.",
  "context": {
    "current_tools": true,
    "include_runtime_snapshot": false,
    "include_graph_freshness": true,
    "include_pending_proposals": true,
    "include_test_results": false
  },
  "scope": ["scan_violations", "awareness"],
  "strict": false
}
```

#### Output

```json
{
  "summary": "2 open gap(s) identified; 1 already-implemented gap(s).",
  "capability_gaps": [
    {
      "gap_id": "awareness.ast_scan",
      "priority": "P1",
      "status": "open",
      "criticism": "scan_violations is grep-based ...",
      "why_it_matters": "...",
      "requirement": "Add Go AST scanning ...",
      "implementation_plan": ["..."],
      "tests_required": ["..."],
      "closure_condition": "A fixture with const target = '127.0.0.1' must produce a violation.",
      "knowledge_updates": [{"target_file": "scan_rules.yaml", "entry": "...", "operation": "add"}],
      "prevents_repeat_criticism": "Future reviews cannot say 'scan is only grep-based' once ...",
      "already_proposed": false,
      "duplicate_of": ""
    }
  ],
  "closed_gaps": [
    {
      "gap_id": "awareness.causal_chain",
      "status": "implemented",
      "closure_condition": "...",
      "prevents_repeat_criticism": "..."
    }
  ],
  "incomplete_criticisms": [
    {
      "text": "Awareness is bad.",
      "status": "incomplete",
      "missing_evidence": "Criticism is too vague to map to a specific capability gap."
    }
  ],
  "global_closure_condition": "Every criticism in the feedback is either converted into a testable requirement or marked as incomplete with missing evidence.",
  "confidence": "high",
  "confidence_reason": "3 gaps identified with keyword matching",
  "blind_spots": [],
  "recommended_next_action": "implement_p0_gaps"
}
```

#### Rules

- **Open gaps** → `capability_gaps[]` with full requirement structure
- **Implemented gaps** → `closed_gaps[]` with closure condition and prevents_repeat_criticism
- **Vague feedback** → `incomplete_criticisms[]` with missing_evidence — never a fabricated gap
- **Duplicate check** → if a pending proposal already covers the gap, `already_proposed=true` and `duplicate_of=<filename>`
- **Confidence** → high (3+ gaps), medium (1-2 gaps), low (no matches or vague)

---

### awareness.requirement_from_critique

Converts a single criticism string into a single structured requirement. Used standalone or internally by `self_review`. Same keyword matching, operates on one string.

#### Input

```json
{
  "criticism": "scan_violations is grep-based, not AST-based",
  "goal": "make awareness prevent known dangerous code patterns",
  "scope": "scan_violations"
}
```

#### Output

```json
{
  "gap_id": "awareness.ast_scan",
  "priority": "P1",
  "criticism": "...",
  "why_it_matters": "...",
  "requirement": "Add Go AST scanning ...",
  "implementation_plan": ["..."],
  "tests_required": ["..."],
  "closure_condition": "...",
  "knowledge_updates": [],
  "prevents_repeat_criticism": "...",
  "confidence": "medium",
  "blind_spots": []
}
```

---

### agent_playbooks.yaml — the knowledge base of known gaps

Located at `docs/awareness/knowledge/agent_playbooks.yaml`. Contains two sections:

**Section 1: `playbooks`** — review workflow definitions with required output fields and forbidden behaviors.

**Section 2: `capability_gap_patterns`** — each known awareness capability gap. Fields:
- `id` — unique gap identifier (e.g. `awareness.ast_scan`)
- `priority` — P0 (false confidence risk) | P1 (high value) | P2 (nice to have)
- `keywords` — phrases used for keyword matching against feedback text
- `status` — `open` | `implemented`
- `requirement` — single-sentence testable requirement
- `implementation_plan` — ordered list of implementation steps
- `tests_required` — specific test names/descriptions that close the gap
- `closure_condition` — what must be true for the gap to be considered closed
- `prevents_repeat_criticism` — explicit statement of what future reviews cannot say

To add a new gap pattern, add an entry to `capability_gap_patterns` with `status: open`. Mark as `status: implemented` once the gap is closed. The tools will automatically route implemented gaps to `closed_gaps` in review output.

---

### How to use

1. After receiving criticism of the awareness system, paste it into `awareness.self_review` as `feedback`.
2. Review `capability_gaps` — implement P0 gaps first.
3. For each gap, use `implementation_plan`, `tests_required`, and `closure_condition` to guide implementation.
4. When a gap is implemented, set `status: implemented` in `agent_playbooks.yaml`.
5. Use `awareness.requirement_from_critique` for single-sentence criticisms or when building a requirement incrementally.
5. AST scan findings are higher-fidelity than regex but still require judgment — review before blocking.
