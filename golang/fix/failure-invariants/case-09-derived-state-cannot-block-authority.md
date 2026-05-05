# Case 09: Derived State Cannot Block Authority

## Status In Code
- DONE: authority-first execution ordering is enforced and derived/projection failures do not gate authority publication.

## Target Invariant
- Failure in derived/projection pipelines cannot block authoritative desired-state publication.

## Implemented
- Define lane dependency contract:
  - projections are consumers of authority, never a gate for authority.
- Enforce publish-first order for critical desired state each cycle.
- If projections fail:
  - mark projections degraded
  - continue authority writes and repair workflows.

## Residual Hardening (Optional)
- Add a regression guard test for any future lane dependency change that could reintroduce gating.

## Tests
- Integration: projection scan hang + confirm ingress/objectstore republish still occurs.
- Unit: derived-lane error never flips authority lane to blocked.

## DoD
- Authority lanes stay active under derived-state failures.
