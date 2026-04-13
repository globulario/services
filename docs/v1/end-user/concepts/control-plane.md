# The Coordinator (Control Plane)

The control plane is the brain of your cluster. It knows what should be running and makes sure it happens.

## What does it do?

- Accepts your commands ("deploy this service")
- Validates them ("does this package exist? is this version valid?")
- Triggers workflows to carry out the work
- Tracks which machines are healthy and available

## What doesn't it do?

It does **not** install packages or start services directly. That's the job of the machines (node agents). The coordinator only decides and delegates.

## How many coordinators?

In a multi-machine cluster, two or more machines run the coordinator service. One is the active leader; the others are standby. If the leader goes down, a standby takes over automatically.

## How do I interact with it?

Through the `globular` CLI:

```bash
globular cluster health            # Ask the coordinator about cluster status
globular services desired set ...  # Tell it what should run
globular cluster nodes list        # Ask it about machines
```

## What happens behind the scenes

The coordinator stores the desired state in etcd (a distributed key-value store). When drift is detected between desired and installed, it starts a workflow to close the gap. The workflow engine then coordinates the actual work across machines.
