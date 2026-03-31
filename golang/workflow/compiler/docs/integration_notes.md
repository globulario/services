# Integration Notes for Existing Globular Code

## Where this module belongs

Best home:
- workflow service package
- or a shared internal package imported by workflow service

Suggested path:

```text
services/golang/workflow/
  v1alpha1/
  compiler/
  runtime/
```

## Expected call flow after integration

### Definition registration
1. Workflow service loads YAML files from disk or repository.
2. `v1alpha1` loader parses them.
3. compiler compiles them to `CompiledWorkflow`.
4. compiled definitions are stored in a registry keyed by workflow name and version.

### Run creation
1. cluster-controller requests workflow run:
   - workflow name
   - workflow version or latest compatible
   - input payload
   - target node or cluster scope
2. workflow service fetches compiled definition from registry.
3. runtime evaluates expressions against inputs and executes graph.

## How this replaces current hidden workflow logic

### cluster-controller
Current role:
- resolve release
- manage phases manually
- generate plans imperatively
- poll/apply/wait with embedded state machine logic

After integration:
- select workflow definition
- build workflow input
- submit run
- react to workflow run state

### node-agent
Current role:
- partially interprets multi-step plans
- retries and tracks execution progression
- contains orchestration fragments

After integration:
- expose actions only
- execute action requests from workflow runtime
- report step status and outputs

### installer
Current role:
- contains procedural install sequences in Go and scripts

After integration:
- expose install/uninstall/configure actions
- let workflow definitions determine sequencing

## Recommended first integrations

1. `node.bootstrap`
2. `release.apply.infrastructure`
3. `day0.bootstrap`

Those are the best targets because they contain large amounts of embedded orchestration and high debugging value.

## Safe migration strategy

### Phase 1
- add compiler package
- compile definitions at startup
- do not execute yet in production path
- validate compiled output only

### Phase 2
- execute one non-critical workflow through new runtime path
- compare resulting workflow events against current implementation

### Phase 3
- move cluster-controller bootstrap/release flows to workflow-driven path
- keep old path behind feature flag

### Phase 4
- collapse NodePlan semantics into workflow-run semantics
- reduce bespoke orchestration structs and reconcile code

## What Claude should not do during implementation

- do not mix runtime execution into compiler package
- do not let compiler depend on node-agent or installer clients
- do not evaluate `$.` expressions during compile
- do not add cluster-specific logic into compiler
- do not make compiler mutate persisted workflow runs

## What Claude should aim for

- pure compiler package
- deterministic compiled artifacts
- clear diagnostics
- easy unit testing
- easy future addition of optimizer passes

