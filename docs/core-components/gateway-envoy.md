# Gateway (Envoy)

## Purpose

The gateway provides TLS termination, gRPC-Web proxying, and service mesh routing for the cluster. It uses Envoy as the data plane proxy, configured dynamically via xDS.

## Architecture

- Envoy listens on port 443 for all external and mesh traffic
- Routes are configured via xDS (gRPC-based dynamic configuration)
- TLS termination uses the cluster PKI certificates
- gRPC-Web translation enables browser clients to call gRPC services

## Routing Model

- Incoming requests are routed by gRPC service name
- Each service registers in etcd; xDS reads these registrations
- Envoy maintains upstream clusters per service with health checking
- Load balancing across service instances on different nodes

## Key Points

- All external access goes through the gateway
- Internal service-to-service calls may bypass the mesh (direct port)
- The `ComputeRunnerService` is NOT routed through Envoy (uses direct ports)
- DNS names `*.globular.internal` are resolved via the cluster DNS service

## Configuration

- xDS server provides dynamic cluster/route configuration
- No static Envoy config files — everything is discovered from etcd
- TLS certificates from `/var/lib/globular/pki/`

## Systemd Unit

`globular-gateway.service` — Manages the Envoy process
