# Awareness

Awareness is Globular's *self-knowledge* layer. It is a graph of invariants,
failure modes, decisions, and learned lessons that the platform — and the AI
agents working on it — consult before editing code, before running migrations,
and before declaring a fix done.

It exists because the platform has 4 truth layers (Repository → Desired →
Installed → Runtime) and many subsystems that depend on each other in subtle
ways. A naive change in one place silently breaks an invariant somewhere else.
Awareness makes those invariants first-class, queryable, and enforceable.

If you have ever asked "is it safe to change this file?" — awareness answers.

---

## TL;DR for developers

Before editing anything in `golang/awareness_graph_client/`, `cluster_controller`,
`workflow`, `node_agent`, `repository`, `xds`, `runtime`, or `mcp`:

```bash
globular awareness preflight --task "<what you are about to do>" --format agent
```

Or in an MCP-aware client, call `awareness.preflight`. The result tells you:

- **Top decision paths** — what to obey before editing
- **Forbidden actions** — patterns that have caused incidents
- **Required tests** — what closes the loop
- **Blind spots** — where coverage is incomplete

`NO_MATCH` is *not* safe. Always read `coverage` and `blind_spots`.

---

## Concepts

| Doc | What it covers |
|-----|----------------|
| [Daily Workflow](daily_workflow.md) | The default loop: preflight → edit → learn |
| [Agent Workflow](agent_workflow.md) | How AI agents are expected to use awareness in a session |
| [Agent Usage](agent_usage.md) | Concrete agent prompts and patterns |
| [Operational Handoff](operational_handoff.md) | Resuming a session safely after a break |
| [Daily Operator Cockpit](operator_cockpit.md) | What operators check each day |
| [Composed-Path Failures](composed_path_failures.md) | The repeat-bug log — read before touching shared concepts |

## Reference

| Doc | What it covers |
|-----|----------------|
| [MCP Tools](mcp_tools.md) | The full MCP tool surface (`awareness.*`) |
| [MCP Server](mcp_server.md) | Server architecture and configuration |
| [Intent Audit v1.1](intent-audit-v1.1.md) | Source/runtime audit contract, provenance causality, and agent rules |
| [Live Overlay](live_overlay.md) | Combining static graph + live runtime evidence |
| [Graph Coverage](graph_coverage.md) | What percentage of code is covered by knowledge |
| [Semantic Navigation](semantic_navigation.md) | Pivot/falsifier model used by `finding_context` |
| [Annotation Coverage](annotation_coverage.md) | Awareness annotations on protos / Go decls |
| [No Match and Confidence](no_match_and_confidence.md) | How to read `NO_MATCH` and scores |
| [Proposal Queue](proposal_queue.md) | How new knowledge enters the graph |
| [Enforcement](enforcement.md) | Where awareness rules are enforced (CLI, MCP, CI) |
| [Test Quality Gates](test_quality_gates.md) | Required-test contracts |
| [Activation Checklist](activation_checklist.md) | What "awareness active" means on a node |
| [Claude Agent Footer](claude_agent_footer.md) | Required-by-policy session footer |

## Theory

| Doc | What it covers |
|-----|----------------|
| [Design Decisions](design_decisions.md) | Why awareness is shaped the way it is |
| [Error-Fix Contract](error_fix_contract.md) | The two-stage contract every bug fix must follow |
| [Awareness Improvement Plan](awareness_improvement_plan.md) | Roadmap |
| [Container Training Loop](container_training_loop.md) | How awareness learns from sandbox incidents |

## Decisions log

Architectural decisions that shaped the graph model. Read these to understand
why awareness draws the lines it does.

- [Awareness graph is *compiled context*, not authority](decisions/awareness-graph-is-compiled-context-not-authority.md)
- [Desired hash is convergence identity](decisions/desired-hash-is-convergence-identity.md)
- [Local success is not global convergence](decisions/local-success-is-not-global-convergence.md)
- [Missing state is not delete intent](decisions/missing-state-is-not-delete-intent.md)
- [Runtime observation is not desired authority](decisions/runtime-observation-is-not-desired-authority.md)

---

## When to consult awareness

| Situation | What to run |
|-----------|-------------|
| Starting a session | `awareness.briefing` with `file` or `task` |
| About to edit a high-risk file | `awareness.decision_context` with the goal + files |
| Got an error you do not recognize | `awareness.failure_match_error` |
| Wondering if a bug is already known | `awareness.did_we_fix` |
| After fixing something | `awareness.learn_from_fix` to propose new knowledge |
| Resuming after a break | `awareness.session_resume_latest` |

The full surface is in [MCP Tools](mcp_tools.md).
