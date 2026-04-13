# Deploy a Service

How to get a service running on your cluster.

## The quick version

```bash
# Build
globular pkg build --spec specs/my-service.yaml --root /tmp/payload --version 1.0.0

# Publish
globular pkg publish --file /tmp/out/my-service_1.0.0_linux_amd64.tgz

# Deploy
globular services desired set my-service 1.0.0
```

Wait 30 seconds. Done.

## What happens step by step

1. **Build** creates a `.tgz` package containing your binary and a systemd unit file
2. **Publish** uploads it to the cluster's package store and verifies its integrity
3. **Desired set** tells the coordinator "this service should run at this version"
4. The coordinator detects the gap between desired and installed
5. A workflow installs the package on each eligible machine
6. The service starts and health checks confirm it's working

## Verify the deployment

```bash
globular services list-desired
```

Look for your service — it should show `match` between desired and installed.

## Deploy to specific machines

Services are deployed based on **profiles**. If your service spec says `profiles: [compute]`, it only installs on machines with the `compute` profile.

Assign profiles:

```bash
globular cluster nodes profiles <node-id> --profile=compute
```

## Update a running service

Same process — publish a new version, set the new desired:

```bash
globular services desired set my-service 1.1.0 --build-number 1
```

Machines automatically update within 30 seconds.

## Rollback

Set the desired back to the old version:

```bash
globular services desired set my-service 1.0.0
```
