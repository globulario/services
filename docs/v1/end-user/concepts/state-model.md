# How Globular Tracks State

Globular keeps four separate views of your system. Each answers a different question.

## The four layers

| Layer | Question | Example |
|-------|----------|---------|
| **Available** | What packages exist? | "dns v0.0.2 is in the store" |
| **Desired** | What should be running? | "dns v0.0.2 should run on all machines" |
| **Installed** | What was actually installed? | "dns v0.0.2 is installed on machine A" |
| **Running** | What is happening right now? | "dns is healthy on machine A" |

## Why four layers?

Because they can disagree — and that's useful.

- **Desired != Installed** means a deployment is pending or failed
- **Installed != Running** means a service crashed after install
- **Available but not Desired** means you have a package but chose not to deploy it

When layers match, your cluster is converged. When they don't, you know exactly where the gap is.

## Who writes each layer?

| Layer | Written by |
|-------|-----------|
| Available | You, when you publish a package |
| Desired | You, when you run `globular services desired set` |
| Installed | The machines, after they install a package |
| Running | The machines, from live health checks |

## How to check alignment

```bash
globular services list-desired
```

This shows desired vs installed for every service. If they match, you see `match`. If not, you see what's different.

## What happens behind the scenes

When Desired and Installed disagree, the system detects "drift". Within 30 seconds, it starts a workflow to close the gap — downloading the right package, installing it, and starting the service.

You don't need to trigger this manually. But you can always see what happened and why.
