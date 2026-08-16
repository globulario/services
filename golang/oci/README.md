# Globular OCI Runtime

This package implements the node-local execution boundary for persistent
OCI-backed Globular services.

Globular remains a native, systemd-supervised platform. The
`globular-oci-runner` binary is the native process supervised by systemd; it
owns exactly one Docker container and reconciles that container from a typed,
admitted specification.

## Authority chain

```text
Repository package + desired release
              |
              v
Node Agent package apply
              |
              v
systemd: globular-<service>.service
              |
              v
globular-oci-runner
              |
              v
Docker Engine API
              |
              v
one OCI container
```

Docker is never the desired-state authority. Every created container uses
restart policy `no`. systemd and Globular own restart and recovery.

## Build and test

From `golang/`:

```bash
go test ./oci/...
go build -o ../dist/.staging/bin/globular-oci-runner ./oci/cmd/globular-oci-runner
```

## Commands

```bash
globular-oci-runner validate --spec service.json --policy policy.json
globular-oci-runner capabilities --socket unix:///var/run/docker.sock
globular-oci-runner run --spec service.json --policy policy.json
globular-oci-runner status --spec service.json
```

See `docs/developers/oci-service-packaging.md` for packaging and operation.
