# How Changes Happen: Workflows

Every meaningful change in Globular happens through a workflow.

## What is a workflow?

A workflow is a sequence of steps that performs a specific operation. For example, deploying a service:

```
Step 1: Download the package
Step 2: Verify its checksum
Step 3: Install it on the machine
Step 4: Start the service
Step 5: Check health
```

Each step either succeeds or fails. If a step fails, the workflow stops and records what happened.

## Why workflows?

Because they make the system **predictable** and **debuggable**.

- You can see exactly what happened during a deployment
- You can see which step failed and why
- You can safely re-run a failed workflow
- Nothing changes without a recorded workflow

## Built-in workflows

Globular includes workflows for common operations:

| Workflow | What it does |
|----------|-------------|
| Package install | Download, verify, install, start, health check |
| Node join | Add a new machine to the cluster |
| Node repair | Fix a machine that's out of sync |
| Service update | Rolling upgrade across machines |
| Compute job | Execute work across multiple machines |

## Can I see workflow history?

Yes:

```bash
globular workflow list-runs
globular workflow get-run <run_id>
```

Each run shows: which steps ran, what succeeded, what failed, and how long it took.

## What happens behind the scenes

Workflows are defined as YAML files stored in the cluster. The workflow engine reads them, executes each step in dependency order, and calls the appropriate service to perform the work.

Steps can run in sequence or in parallel (for fan-out operations like installing on multiple machines at once).

If you need to understand the full workflow system, see [Internals: Workflow Engine](../core-components/workflow-service.md).
