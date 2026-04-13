# CLI Commands

The `globular` command is how you interact with your cluster.

## Most common commands

### Check cluster health

```bash
globular cluster health
```

Shows: node count, health status, convergence.

### List services

```bash
globular services list-desired
```

Shows: every service, its desired version, installed version, and whether they match.

### Deploy a service

```bash
# 1. Publish the package
globular pkg publish --file my-service.tgz

# 2. Set the desired version
globular services desired set my-service 1.0.0
```

What happens: Globular installs the service on all eligible machines within 30 seconds.

### Build a package

```bash
globular pkg build --spec specs/my-service.yaml --root /tmp/payload --version 1.0.0
```

What happens: Creates a `.tgz` package ready to publish.

## Cluster management

| Command | What it does |
|---------|-------------|
| `globular cluster bootstrap` | Initialize first machine |
| `globular cluster join --token T` | Add a machine to the cluster |
| `globular cluster token create` | Generate a join token |
| `globular cluster nodes list` | List all machines |
| `globular cluster nodes profiles NODE --profile=compute` | Assign a role to a machine |

## Service management

| Command | What it does |
|---------|-------------|
| `globular services desired set NAME VERSION` | Set which version should run |
| `globular services list-desired` | Compare desired vs actual |
| `globular services apply-desired` | Force install on this machine |
| `globular services repair` | Fix mismatches automatically |
| `globular services verify-integrity` | Check package checksums |

## Package management

| Command | What it does |
|---------|-------------|
| `globular pkg build --spec YAML --root DIR` | Create a package |
| `globular pkg publish --file TGZ` | Upload to the cluster store |
| `globular pkg info NAME` | Show package details |

## Diagnostics

| Command | What it does |
|---------|-------------|
| `globular doctor report` | Diagnose cluster issues |
| `globular doctor heal` | Auto-fix known problems |
| `globular doctor heal history` | View past auto-fixes |
| `globular support bundle create` | Collect debug data |

## Common flags

| Flag | Meaning |
|------|---------|
| `--controller ADDR` | Talk to a specific coordinator |
| `--output json` | Get machine-readable output |
| `--timeout 10s` | Set request timeout |
| `--insecure` | Skip certificate checks (dev only) |
