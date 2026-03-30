# Task: Propose an implementation plan to reuse `globular-installer` as the shared infra install engine for Day 1, without breaking Day 0

You now have access to both codebases:
- `globular-installer`
- `node-agent`

I want you to analyze both and write a concrete implementation plan.

## What we want

We want to stop duplicating infrastructure installation logic between `globular-installer` and `node-agent`.

The target direction is:

- `globular-installer` remains the canonical implementation of infra package installation behavior
- `node-agent` remains the Day 1 executor used by the cluster controller
- the cluster controller still decides intent, phases, and gating
- `node-agent` should reuse `globular-installer` for infra apply, instead of reimplementing the same install logic separately

## Important context

Current reality:
- Day 0 bootstrap already exists and works through `globular-installer`
- Day 1 join is partially working
- etcd join/membership appears to be working already
- we must be careful not to destabilize Day 0 or break current etcd join behavior
- Day 1 currently suffers from infra not being installed/recognized correctly, wrong infra version fallback, and workloads installing before infra is truly converged

## Non-negotiable constraints

1. **Do not redesign Day 0**
   - `globular-installer` is already the canonical Day 0 bootstrap path
   - do not propose rewriting or replacing it

2. **Do not break current etcd join behavior**
   - if existing Day 1 etcd join/membership logic already works, preserve it
   - if etcd package setup and etcd cluster-join logic are separable, that is acceptable
   - but do not casually replace working join flow

3. **Do not introduce a second planner**
   - the controller must still decide node intent and Day 1 phases
   - `node-agent` must still be the executor
   - `globular-installer` must not become a second orchestration brain

4. **Do not duplicate infra logic further**
   - the plan should reduce duplication, not move it around

## Design intent

The architecture we want is:

- **Cluster controller**
  - resolves profiles/capabilities/packages
  - computes Day 1 phases
  - builds infra-first plan
  - blocks workloads until infra is installed and healthy

- **Node-agent**
  - receives plan from controller
  - executes package apply
  - reports installed state / health / result
  - does not invent cluster policy

- **Globular-installer**
  - becomes the shared infra install engine used by node-agent
  - owns the concrete spec-driven installation behavior for infra packages
  - should ideally be reused as a Go package/library, not just shelling out to a CLI, unless a short-term bridge is required

## What I want from you

Please inspect both codebases and produce a detailed implementation plan answering these questions:

### 1. Reuse strategy
What is the best technical reuse model?

Evaluate and compare:
- importing installer logic as a Go package/library into node-agent
- extracting shared install runner/package from `globular-installer`
- subprocess/CLI invocation as a temporary bridge
- any hybrid approach

I want your recommendation, with reasons.

### 2. Boundary design
Show the exact responsibility boundary between:
- controller
- node-agent
- installer

Be explicit about:
- who resolves package metadata
- who chooses install order
- who chooses install mode
- who executes install steps
- who reports installed/healthy state

### 3. etcd handling
Given that etcd join appears to work already:
- should etcd remain on the current Day 1 join path?
- should only package setup be delegated to installer?
- should etcd stay fully special-cased?
- what is the safest minimal-change approach?

I want a recommendation that minimizes risk.

### 4. Infra package routing
Explain how non-etcd infra should work after the change, especially:
- scylladb
- envoy
- minio
- xds
- any other infra packages you find relevant

Describe how node-agent should call into shared installer logic for these.

### 5. Required refactors
List the exact refactors needed in:
- `globular-installer`
- `node-agent`
- plan/apply contract between controller and node-agent
- installed-state reporting
- infra health reporting

Be concrete:
- new package/module boundaries
- interfaces to introduce
- types/structs to add
- old code paths to delete or keep temporarily

### 6. Migration strategy
I do **not** want a big-bang rewrite.

Propose a phased migration that is low-risk.

For each phase, specify:
- code changes
- behavior change
- test coverage
- rollback safety

### 7. Acceptance criteria
Define the exact acceptance criteria for the new design.

At minimum include:
- Day 0 still works unchanged
- etcd join behavior still works
- infra install logic is no longer duplicated
- non-etcd infra can be installed through shared installer logic
- infra becomes visible to reconciliation
- workloads do not install before required infra is healthy

## Specific things to check while analyzing the code

Please explicitly inspect and comment on:

1. Whether `globular-installer` already has reusable spec execution primitives that node-agent should call directly
2. Whether node-agent currently has duplicated install logic that should be replaced
3. Whether the installer code is already packageable as a reusable Go module or needs extraction
4. Whether current etcd handling is already isolated enough to preserve as-is
5. Whether installed-state reporting is coupled to the old node-agent-only install path
6. Whether there are hidden assumptions in Day 0 installer that would make reuse risky

## Output format I want

Write the response as a structured implementation plan with these sections:

1. **Summary Recommendation**
2. **Current State Analysis**
3. **Target Architecture**
4. **Recommended Reuse Model**
5. **Exact Code Changes**
6. **Phased Migration Plan**
7. **Test Plan**
8. **Risks and Safeguards**
9. **Final Recommendation**

## Important instruction

Do not jump directly into coding.

First, based on your knowledge of both codebases, produce the implementation plan.

I want the plan to be:
- technically grounded in the actual code
- conservative with Day 0
- conservative with working etcd join behavior
- aggressive only in removing duplicated infra install logic
- explicit about how to avoid making the system messier