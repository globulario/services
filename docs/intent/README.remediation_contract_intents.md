# Remediation and Evidence Contract Intents

This bundle adds eight proposed intent files that close the gaps around remediation authority, weak evidence, auditability, operator override, dependency degradation, workflow truth, and runtime identity.

## Added intent files

1. `remediation.token_contract.yaml`
2. `evidence.provenance_trust_levels.yaml`
3. `audit.retention_and_correlation_policy.yaml`
4. `remediation.failure_rate_policy.yaml`
5. `operator.override_intent.yaml`
6. `workflow.remediation_truth_consistency.yaml`
7. `service.dependency_degradation_modes.yaml`
8. `runtime.identity_attestation.yaml`

## Intended enforcement theme

These should be promoted from `status: proposed` to the normal accepted status only after matching invariants, schema fields, and tests exist. Until then, agents should treat them as design contracts and use them during preflight review.
