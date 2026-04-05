You are finishing the **last compliance gap** in Globular’s workflow migration.

The system is already mostly correct. Do **not** restart the migration. Do **not** redesign unrelated parts.
Your job is to remove the final two contradictions so the code fully conforms to the workflow requirement.

# Remaining blockers

There are only two remaining issues to fix:

## Blocker 1: APPLYING still exists as a live execution phase

Current problem:

* APPLYING no longer drives reconciler branching, which is good.
* But workflow still writes `ReleasePhaseApplying` during execution.

Why this is still wrong:

* The requirement says APPLYING may exist **only as legacy compatibility**, not as a real internal orchestration concept.
* If workflow writes APPLYING as part of live execution, then APPLYING is still part of the internal state model instead of being a boundary mapping.

Required outcome:

* Workflow must stop using APPLYING as a real live execution phase.
* Internal truth must be represented by workflow-native run/step state only.
* If external consumers still require APPLYING, derive it at the boundary from workflow-native state.

What to do:

1. Find all workflow callbacks and execution paths that write `ReleasePhaseApplying`.
2. Remove those writes from internal execution logic.
3. Replace them with workflow-native state only:

   * queued
   * waiting_on_dependency
   * waiting_on_node
   * running
   * retrying
   * blocked
   * failed
   * succeeded
   * cancelled
   * superseded
4. If an external API, UI, or compatibility surface still expects APPLYING:

   * compute it from workflow state at read/serialization boundary
   * do not persist or branch on APPLYING internally
5. Ensure release state written by workflow is either:

   * explicit workflow-native execution state
   * or final release outcome
     but not APPLYING as a live mixed concept

Definition of done:

* No workflow callback writes APPLYING as internal live state
* No internal decision-making depends on APPLYING
* APPLYING, if still exposed externally, is derived compatibility only

---

## Blocker 2: controller still contains too much orchestration

Current problem:

* Controller is thinner than before, but still holds meaningful orchestration logic in reconcile paths:

  * drift classification branching
  * remediation flow decisions
  * aggregation/finalization behavior
  * step sequencing logic that belongs in workflow

Why this is still wrong:

* The requirement says controller should not hide orchestration in scattered imperative code.
* Controller should primarily:

  * detect drift
  * choose workflow
  * admit/start workflow
* Workflow should express:

  * sequencing
  * wait reasons
  * retries
  * dependencies
  * aggregation
  * finalize logic

Required outcome:

* Controller becomes an admission/selection layer, not an orchestration engine.
* Workflow definitions, callbacks, and run/step state become the readable convergence spine.

What to do:

1. Audit reconcile controller code and classify each block as one of:

   * drift detection
   * workflow selection/admission
   * orchestration logic
2. Keep only:

   * drift detection
   * workflow selection
   * workflow admission/start
3. Move orchestration logic into workflow-native structures wherever possible:

   * step graph
   * workflow definitions
   * workflow callbacks
   * explicit run/step state transitions
4. Specifically target logic like:

   * dependency ordering
   * remediation sequencing
   * per-node aggregation
   * success/failure finalization
   * retry transitions
   * blocked/wait reasoning
5. If a reconcile function contains logic that answers “what happens next?”, that logic probably belongs in workflow, not controller.

Definition of done:

* Controller mostly detects, selects, and starts
* Workflow mostly orchestrates
* A new engineer can understand convergence primarily from workflow definition + callbacks + run state, not from controller branching

---

# Hard rules

* Do not reintroduce plan-based reasoning
* Do not keep APPLYING as internal truth
* Do not solve this with extra compatibility glue in core logic
* Prefer moving orchestration into workflow over preserving imperative controller logic
* Preserve existing working execution paths while refactoring

# Final acceptance test

The work is complete only if all of the following are true:

1. Workflow callbacks do not write APPLYING as live internal execution state
2. APPLYING exists only as optional boundary compatibility mapping
3. Internal execution truth comes from workflow-native run/step state
4. Controller no longer contains significant hidden orchestration
5. Controller mainly detects drift, selects workflow, and starts it
6. Workflow definitions/callbacks/state explain the orchestration story
7. A new engineer can understand convergence without reverse-engineering scattered controller reconcile code

# Working method

Follow this order:

1. Remove internal APPLYING writes
2. Add boundary-only compatibility mapping if needed
3. Audit reconcile controller for orchestration logic
4. Move orchestration behavior into workflow structure/callbacks/state
5. Delete stale branches/comments/names left behind
6. Re-read the code and verify there is only one mental model left

# Final question

Ask this before declaring success:

“Is APPLYING now only a compatibility label, and is controller now mostly an admission layer while workflow is the actual orchestration layer?”

If the answer is no, the requirement is still not fully met.
