# Install Globular

Get Globular running on your first machine.

## What you need

- A Linux machine (Ubuntu 22.04+ recommended)
- Internet access (to download packages)

## Install

```bash
# Download and run the installer
curl -sL https://get.globular.io | bash
```

Or build from source:

```bash
git clone https://github.com/globulario/services.git
cd services/golang
go build ./...
./install-day0.sh
```

## Verify it works

```bash
globular cluster health
```

You should see:

```
Cluster: healthy
Nodes: 1/1 ready
Services: 48 converged
```

## Add more machines

On your first machine, create a join token:

```bash
globular cluster token create
```

On each new machine:

```bash
globular cluster join --token <TOKEN> --controller <FIRST_MACHINE_IP>:443
```

## What happens after install?

Globular automatically:

- Sets up encrypted communication between machines
- Starts the core services (storage, networking, monitoring)
- Creates a shared package repository
- Begins health monitoring

You don't need to configure any of this manually.

## What's next?

- [Deploy your first service](quick-start.md)
