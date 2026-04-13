# Task: Publish a Service

## Overview

Build a service from source, package it, and publish it to the Globular Repository so it can be deployed across the cluster.

Use this when:
- You have new or updated service code to release
- You need to make a package available for deployment
- You want to add a new service to the platform

## Prerequisites

- Go 1.24+ installed
- Globular source code checked out
- Authenticated CLI session (`globular auth login`) with publisher permissions
- Repository service running (`globular pkg info` responds)

## Steps

### Step 1: Generate protobuf code (if proto files changed)

```bash
./generateCode.sh
```

### Step 2: Build the Go binary

```bash
cd golang
go build -o ../packages/payload/<service>/bin/<service>_server ./<service>/<service>_server
cd ..
```

Example:
```bash
cd golang
go build -o ../packages/payload/monitoring/bin/monitoring_server ./monitoring/monitoring_server
cd ..
```

### Step 3: Run tests

```bash
cd golang
go test ./<service>/... -v -race
cd ..
```

### Step 4: Prepare the payload directory

```bash
mkdir -p packages/payload/<service>/bin
mkdir -p packages/payload/<service>/specs

# Binary should already be there from Step 2
# Copy the spec file
cp specs/<service>_service.yaml packages/payload/<service>/specs/
```

### Step 5: Build the package

```bash
globular pkg build \
  --spec specs/<service>_service.yaml \
  --root packages/payload/<service>/ \
  --version <version> \
  --build-number <build>
```

Example:
```bash
globular pkg build \
  --spec specs/monitoring_service.yaml \
  --root packages/payload/monitoring/ \
  --version 0.0.6 \
  --build-number 1
```

Output:
```
Package built: globular-monitoring-0.0.6-linux_amd64-1.tgz
```

### Step 6: Publish to the repository

```bash
globular pkg publish globular-<service>-<version>-linux_amd64-<build>.tgz
```

Example:
```bash
globular pkg publish globular-monitoring-0.0.6-linux_amd64-1.tgz
```

### Step 7: Optionally deprecate the old version

```bash
globular pkg deprecate <service> <old-version>
```

## Verification

```bash
# Confirm the artifact is published
globular pkg info <service>
# Shows: <service> <version> PUBLISHED

# The package is now available for deployment via:
# globular services desired set <service> <version>
```

## Troubleshooting

### "publisher identity mismatch"

Your authenticated identity doesn't match the `publisher` field in the spec file. Either:
- Log in as the correct user: `globular auth login --username <publisher>`
- Update the spec file to match your identity

### "connection refused" during publish

The Repository service is not running. Check:
```bash
globular cluster health
# Look for repository in the service list
```

### "artifact already exists"

A package with the same name/version/platform/build already exists. Increment the build number:
```bash
globular pkg build --spec ... --version 0.0.6 --build-number 2
```

### Tests fail

Fix the tests before publishing. Do not publish untested code:
```bash
cd golang && go test ./<service>/... -v -race
```
