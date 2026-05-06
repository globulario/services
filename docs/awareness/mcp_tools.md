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
