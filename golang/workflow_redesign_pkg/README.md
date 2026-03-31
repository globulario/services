# Globular workflow externalization starter pack

This package turns the hidden orchestration in Globular into explicit workflow definitions plus the Go interfaces an executable workflow engine would need.

## Included workflow definitions

- `definitions/node.bootstrap.yaml`
- `definitions/release.apply.infrastructure.yaml`
- `definitions/day0.bootstrap.yaml`

These are not just pretty diagrams. They are shaped so the workflow service can execute them by dispatching actions to actors:

- `cluster-controller`
- `node-agent`
- `installer`
- `repository`

## Included Go interfaces

- `go/workflow_engine_interfaces.go`

This file defines:

- workflow definition model
- run/step state model
- actor/action dispatch contracts
- condition evaluation contracts
- persistence contracts
- node command bridge contracts
- controller callback contracts
- installer callback contracts
- repository callback contracts

## Design rules

1. Package is the payload unit.
2. Plan is the node-scoped executable fragment.
3. Workflow is the cross-actor orchestration story.
4. The workflow service becomes the durable runtime.
5. The cluster-controller becomes the planner/compiler, not the phase machine.
6. The node-agent stays the executor of atomic actions.
7. The installer stays a primitive library and local actor.

## Intended migration order

1. `node.bootstrap`
2. `release.apply.infrastructure`
3. `day0.bootstrap`

That sequence gives the best code shrink with the least blast radius.

## Added in this revision

A new Go package is now included under `go/v1alpha1/` with:

- `types.go` — v1alpha1 authoring schema structs for YAML/JSON workflow definitions
- `loader.go` — loader + validator for single files or whole definition directories

### Why a separate v1alpha1 package?

The runtime interfaces in `workflow_engine_interfaces.go` model execution-time state.
The new `v1alpha1` package models **authoring-time definitions**, which need to accept:

- literal values like `30m` or `3`
- expression values like `$.execute_timeout` or `$.max_parallel_nodes`

That is why fields such as concurrency and timeout use flexible scalar wrappers rather than plain `int` or `string`.

### Example

```go
loader := v1alpha1.NewLoader()
def, err := loader.LoadFile("definitions/day0.bootstrap.yaml")
if err != nil {
    return err
}

fmt.Println(def.Metadata.Name)
```

### Validation performed

The validator checks:

- `apiVersion` and `kind`
- `metadata.name`
- supported strategy mode
- required foreach fields
- supported actors
- non-empty actions
- positive retry counts
- valid durations or expression references
- unique step IDs
- missing dependencies
- self-dependencies
- dependency cycles
