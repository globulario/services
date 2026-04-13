# Quick Start: Deploy Your First Service

Deploy a service to your cluster in under 5 minutes.

## Step 1: Build a package

```bash
# Build the echo service (a simple test service)
cd services/golang
go build -o /tmp/echo_server ./echo/echo_server/

# Package it
mkdir -p /tmp/echo-payload/bin
cp /tmp/echo_server /tmp/echo-payload/bin/
globular pkg build --spec generated/specs/echo_service.yaml \
  --root /tmp/echo-payload --version 0.0.1 --build-number 1
```

## Step 2: Publish it

```bash
globular pkg publish --file /tmp/out/echo_0.0.1_linux_amd64.tgz
```

## Step 3: Tell Globular to run it

```bash
globular services desired set echo 0.0.1 --build-number 1
```

## Step 4: Watch it deploy

```bash
globular services list-desired
```

Within 30 seconds, you should see:

```
echo    0.0.1    0.0.1    match
```

The service is now running on your cluster.

## Step 5: Check health

```bash
globular cluster health
```

## What just happened?

1. You **built** a package from source code
2. You **published** it to the cluster's package store
3. You **declared** that it should run
4. Globular **installed** it on the right machines and **started** it

You didn't SSH anywhere. You didn't write systemd units. You didn't manage certificates.

## Try more things

- [Deploy a compute job](../../draft/ffmpeg_transcode_demo.md) (transcode audio with ffmpeg)
- [See all CLI commands](../cli/commands.md)
- [Learn how Globular works](../architecture/overview.md)
