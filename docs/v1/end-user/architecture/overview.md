# How Globular Works

Globular has a simple design: you say what you want, and it makes it happen through a series of clear steps.

## The big picture

```
You → "Run service X version 2"
  → Coordinator checks if it's valid
    → Workflow executes the steps
      → Machines install and start the service
        → Health checks confirm it's working
```

Every change follows this path. Nothing happens in the background without you knowing.

## Five parts working together

### 1. The Coordinator

One machine acts as the brain. It knows what should be running and where.

When you say "deploy service X", the coordinator decides if it makes sense, then hands it off to the execution system.

### 2. The Workflow Engine

This is where work actually gets done. The coordinator doesn't install packages directly — it starts a workflow that performs each step in order:

1. Download the package
2. Verify its integrity
3. Install it on the target machine
4. Start the service
5. Check health

If any step fails, the workflow records what happened and stops.

### 3. The Machines (Node Agents)

Each machine in your cluster runs a small agent. The agent receives instructions from workflows and carries them out: install packages, start services, report health.

The agent doesn't decide what to do. It only does what it's told.

### 4. The Package Store

All your services live in a shared store. When you publish a package, it goes here. When a machine needs to install something, it downloads from here.

Every package has a version, a checksum, and a manifest. Nothing gets installed without verification.

### 5. Monitoring

Globular watches everything: service health, machine status, resource usage. If something goes wrong, it's visible immediately.

## Why this design?

**Predictability.** Every change is a sequence of recorded steps. If something breaks, you can see exactly what happened and why.

**Safety.** Nothing changes without a workflow. No background loops silently modifying your system.

**Simplicity.** You interact with one command (`globular`). The system handles the complexity.

## Learn more

- [What is the state model?](state-model.md) — How Globular tracks what should run vs what is running
- [What are workflows?](workflows.md) — How changes are executed
- [What does the coordinator do?](control-plane.md) — The brain of the cluster
