# Workflow Compiler Module Definition

## Purpose

The workflow compiler converts a `v1alpha1.WorkflowDefinition` into a deterministic `CompiledWorkflow` suitable for execution by the workflow engine.

The compiler exists to remove interpretation work from runtime.

Without the compiler, the workflow service must repeatedly:
- parse YAML
- infer defaults
- validate edges
- inspect actors and action names
- parse durations
- re-check retry policy
- resolve graph ordering
- discover entry points

With the compiler, all of that is done once.

## Input contract

```go
// authoring layer
import workflowv1alpha1 ".../workflow/v1alpha1"

type WorkflowDefinition struct {
    APIVersion string
    Kind       string
    Metadata   Metadata
    Spec       WorkflowSpec
}
```

The compiler assumes the authoring definition has already been loaded, but it may still contain:
- missing defaults
- shorthand values
- static literals
- runtime expressions beginning with `$.`

## Output contract

```go
type CompiledWorkflow struct {
    Name        string
    Namespace   string
    Version     string
    SourceHash  string

    Steps       map[string]*CompiledStep
    TopoOrder   []string
    EntryPoints []string
    Dependents  map[string][]string

    Metadata    CompiledMetadata
    Defaults    RuntimeDefaults
}
```

## Core compiler phases

### 1. Normalize
Turn the user definition into an explicit internal form.

Examples:
- empty retry policy becomes inherited default
- absent timeout becomes workflow default or zero
- singleton step fields are expanded into the normalized structure
- `depends_on: null` becomes `[]`

### 2. Validate
Ensure the workflow is structurally sound.

Checks include:
- `apiVersion` present and supported
- `kind == WorkflowDefinition`
- workflow name not empty
- step IDs unique
- every dependency target exists
- no self-dependency
- no dependency cycles
- actor/action names non-empty
- `foreach` only present on valid strategy modes
- duration strings parse

### 3. Resolve static values
Convert static text into runtime-ready representations.

Examples:
- `"5m"` -> `time.Duration(5 * time.Minute)`
- `"parallel"` -> enum-like normalized mode
- literals remain static values

### 4. Preserve expressions
Expressions beginning with `$.` remain unresolved in the compiled artifact.

Examples:
- `$.node.id`
- `$.inputs.package_name`
- `$.max_parallel_nodes`

These are preserved for runtime evaluation against run inputs and context.

### 5. Build graph indexes
Precompute all graph structures required by the runtime.

Outputs:
- `Steps` map keyed by step ID
- `TopoOrder`
- `EntryPoints`
- `Dependents`

### 6. Freeze the artifact
Emit a deterministic compiled object that can be:
- cached
- hashed
- serialized
- persisted
- reused for many workflow runs

## Output types

### CompiledStep

```go
type CompiledStep struct {
    ID          string
    DisplayName string

    Actor       string
    Action      string

    With        map[string]ValueExpr

    DependsOn   []string
    Dependents  []string

    Retry       RetryPolicy
    Timeout     time.Duration

    Strategy    *CompiledStrategy

    OnSuccess   []TransitionHook
    OnFailure   []TransitionHook
}
```

### ValueExpr

```go
type ValueExpr struct {
    Raw       string
    IsExpr    bool
    Static    any
    ExprKind  ExprKind
}
```

### CompiledStrategy

```go
type CompiledStrategy struct {
    Mode           StrategyMode
    MaxConcurrency *ValueExpr
    Foreach        *ValueExpr
}
```

## Runtime responsibilities not owned by compiler

The compiler must not:
- evaluate `$.` expressions against runtime inputs
- dispatch actors
- schedule retries
- persist workflow runs
- manage locks
- decide cluster admission
- execute rollback actions

Those belong to the workflow engine runtime.

## Determinism requirements

Given the same input definition bytes, the compiler output should be functionally identical.

Recommended:
- stable map serialization when hashing
- sorted diagnostics
- sorted `EntryPoints`
- stable topological sort when multiple valid orders exist

## Error model

The compiler should return structured diagnostics.

```go
type Diagnostic struct {
    Severity string // error | warning
    Path     string // spec.steps[2].timeout
    Code     string // invalid_duration
    Message  string
}
```

Compilation should fail if any error diagnostic is present.
Warnings may be returned alongside a valid compiled artifact.

## Recommended public API

```go
type Compiler interface {
    Compile(ctx context.Context, def *workflowv1alpha1.WorkflowDefinition) (*CompiledWorkflow, []Diagnostic, error)
}
```

Optional helper APIs:

```go
func CompileDefinition(ctx context.Context, def *workflowv1alpha1.WorkflowDefinition) (*CompiledWorkflow, []Diagnostic, error)
func MustCompileDefinition(def *workflowv1alpha1.WorkflowDefinition) *CompiledWorkflow
```

## Caching

The compiled artifact is a strong candidate for caching by source hash.

Suggested cache key:
- hash of normalized definition bytes
- or hash of original bytes if normalization is deterministic

## Migration value

This module lets you progressively replace hidden orchestration logic in:
- Day-0 installer
- cluster-controller release state machine
- bootstrap sequencing
- node-agent multi-step execution logic

Instead of embedding orchestration in Go, those components emit workflow definitions and inputs.

