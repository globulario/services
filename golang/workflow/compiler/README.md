# Workflow Compiler Package

This package defines the **workflow compiler module** that sits between:

- `workflow/v1alpha1` authoring definitions
- the workflow engine runtime

Its job is to turn a user-authored workflow definition into a **compiled execution graph** that can be executed by the workflow service without re-parsing YAML, re-solving dependencies, or re-normalizing options at runtime.

## Goal

Convert this:

- YAML or JSON workflow definitions
- loose fields
- shorthand values
- unresolved dependency graph
- mixed static values and runtime expressions

Into this:

- validated, normalized `CompiledWorkflow`
- topologically sorted steps
- entry points and reverse dependencies precomputed
- static values resolved up front
- expressions preserved for runtime evaluation
- deterministic output safe to cache, persist, diff, and execute

## Compiler responsibilities

1. Load `v1alpha1.WorkflowDefinition`
2. Normalize defaults and shorthand forms
3. Validate structure and graph invariants
4. Resolve static values
5. Preserve runtime expressions like `$.node.id`
6. Build DAG indexes
7. Emit `CompiledWorkflow`

## Module boundaries

### Input
- `go/v1alpha1/types.go`
- `go/v1alpha1/loader.go`

### Output
- `go/compiler/types.go`
- `go/compiler/compiler.go`

### Consumed by
- workflow service runtime
- run scheduler
- dispatcher / actor router
- persistence layer

## Important design rule

The compiler must be **pure**.

It must not:
- execute actions
- call node-agent
- call installer
- mutate cluster state
- decide runtime availability

It only transforms definition into executable structure.

## Suggested integration path

1. Keep existing `v1alpha1` definitions as authoring format.
2. Add this compiler package under the workflow service.
3. Make workflow service load and compile definitions at startup.
4. Store compiled definitions in memory, optionally cache by hash.
5. Change cluster-controller so it submits:
   - workflow name
   - input payload
   - target scope
   instead of hand-built orchestration logic.

## Files in this package

- `docs/compiler_module_definition.md` — precise design document
- `docs/integration_notes.md` — how to wire into existing code
- `go/compiler/types.go` — compiled data model
- `go/compiler/compiler.go` — compiler skeleton
- `go/compiler/errors.go` — compiler error model
- `go/compiler/expressions.go` — expression classification helpers
- `examples/compiled_workflow_example.json` — concrete output shape

