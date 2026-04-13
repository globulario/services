# How Globular Keeps Things In Sync

When you tell Globular what should run, it works to make reality match your intent.

## The basic idea

1. You declare: "Service X version 2 should run everywhere"
2. Globular checks: "Is service X version 2 installed everywhere?"
3. If not: a workflow installs it on the machines that need it

This is called **convergence** — the system converges toward the desired state.

## How often does it check?

Every 30 seconds, the coordinator compares desired vs installed. If there's a difference, it acts.

## What triggers convergence?

- You set a new desired version
- A machine joins the cluster
- A service crashes and needs reinstallation
- The coordinator detects a mismatch during its regular check

## Is it a background loop?

No. The check runs periodically, but the actual work is always a recorded workflow. You can see every action in the workflow history.

## How do I know if my cluster is converged?

```bash
globular services list-desired
```

If every service shows `match`, your cluster is converged.

## What if convergence fails?

The workflow records what went wrong. You can:

```bash
globular doctor report       # See what's wrong
globular services repair     # Try to fix it
```
