# Build Packages

## Package Structure

A Globular package is a `.tgz` archive containing:
- `bin/` — Service binary
- `package.json` — Package manifest (auto-generated)
- Optional: `workflows/`, config files, scripts

## Package Spec

Every service has a spec YAML in `generated/specs/`:

```yaml
version: 1
metadata:
  name: <service>
  profiles: [core]      # Which node profiles install this
  priority: 1000         # Install order (lower = earlier)
service:
  name: <service>
  exec: <binary_name>
steps:
  - id: ensure-dirs
    type: ensure_dirs
    dirs: [...]
  - id: install-payload
    type: install_package_payload
    install_bins: true
  - id: install-service
    type: install_services
    units: [...]
  - id: health-check
    type: health_checks
    services: [...]
```

## Build Commands

```bash
# Build all services
cd golang && go build ./...

# Build a single service binary
cd golang && GOOS=linux GOARCH=amd64 go build -o /tmp/<name>_server ./<name>/<name>_server/

# Package it
globular pkg build \
  --spec generated/specs/<name>_service.yaml \
  --root /tmp/payload \
  --version 0.0.1 \
  --build-number 1 \
  --publisher core@globular.io \
  --out /tmp/out

# Publish to repository
globular pkg publish --file /tmp/out/<name>_0.0.1_linux_amd64.tgz
```

## Proto Contracts

After modifying a `.proto` file:

```bash
./generateCode.sh
```

This regenerates Go and TypeScript code from all proto files in `/proto/`.
