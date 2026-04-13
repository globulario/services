# What Is Globular?

Globular is a platform for running services across multiple machines.

You describe what you want running. Globular makes it happen.

## What can you do with it?

- **Deploy services** to one or many machines
- **Keep them running** — if something crashes, it gets fixed
- **Update them** without downtime
- **Monitor them** from a single place
- **Run compute jobs** across your machines (transcode video, process data, create backups)

## How is it different?

You don't write deployment scripts. You don't SSH into machines to install things.

Instead:

1. You **publish** a package
2. You **tell Globular** where it should run
3. Globular **installs and runs it** on the right machines

That's it. Globular handles the rest — certificates, health checks, restarts, updates.

## What does a cluster look like?

A Globular cluster is a group of machines working together. Each machine runs a small agent that receives instructions. One machine acts as the coordinator.

```
Machine A (coordinator)  ←→  Machine B  ←→  Machine C
     ↓                           ↓               ↓
  30+ services              30+ services     30+ services
```

All machines share the same view of what should be running.

## What's next?

- [Install Globular](installation.md) on your first machine
- [Run your first service](quick-start.md)
