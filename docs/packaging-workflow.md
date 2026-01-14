# Globular Service Packaging Workflow

This document describes the standard workflow to:
1) build staged service executables
2) generate service specs + default configs
3) generate `.tgz` service packages with `globular pkg build`

## Prerequisites
- Go toolchain installed
- `python3` installed
- You have built `globularcli` (or `globular`) with the `pkg build` command available

From `services/golang/globularcli`:
```bash
go build -o ./globularcli .
./globularcli pkg --help
```

## Directory conventions
### Staged binaries (input)
By default, scripts look for service executables at:

```
/home/dave/Documents/github.com/globulario/services/golang/tools/stage/linux-amd64/usr/local/bin
```

That directory must contain binaries like:

- event_server
- discovery_server
- torrent_server
- etc.

### Generated artifacts (output)
By default, scripts write into:

```
./generated/
```

Structure:

```
generated/
  specs/        # generated YAML specs (one per service)
  config/       # generated default config.json (one per service)
  payload/      # per-service payload roots assembled for pkg build
  packages/     # resulting .tgz packages
```

## Step 1 — Build staged executables
Run your normal build that produces staged binaries under the stage directory.

Verify:
```bash
ls -1 /home/dave/Documents/github.com/globulario/services/golang/tools/stage/linux-amd64/usr/local/bin/*_server | head
```

## Step 2 — Generate specs + default configs
From `services/golang/globularcli`:
```bash
make specgen
```

Or directly:
```bash
./tools/specgen/specgen.sh \
  /home/dave/Documents/github.com/globulario/services/golang/tools/stage/linux-amd64/usr/local/bin \
  ./generated
```

Outputs:
- `generated/specs/<svc>_service.yaml`
- `generated/config/<svc>/config.json`

## Step 3 — Generate packages
From `services/golang/globularcli`:
```bash
make pkggen PKG_VERSION=0.0.1
```

Or directly:
```bash
./tools/pkggen/pkggen.sh \
  --globular ./globularcli \
  --bin-dir /home/dave/Documents/github.com/globulario/services/golang/tools/stage/linux-amd64/usr/local/bin \
  --gen-root ./generated \
  --out ./generated/packages \
  --version 0.0.1
```

## Verify a package
```bash
./globularcli pkg verify --file ./generated/packages/service.event_0.0.1_linux_amd64.tgz
```

## Troubleshooting
- **“command not found: globular”**  
  Use `--globular ./globularcli` (path to the built CLI binary), or ensure it is on your `PATH`.

- **“no executable found in bin among candidates”**  
  Ensure specs declare the executable explicitly (`service.exec`) **or** the payload bin directory contains the expected candidate name. Re-run:
  ```bash
  make specgen
  make pkggen PKG_VERSION=0.0.1
  ```

## One-shot build
```bash
make all-packages PKG_VERSION=0.0.1
```
