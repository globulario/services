# Networking & Mesh (Envoy / xDS / Gateway)

Purpose: snapshot of how Globular routes traffic, discovers services, and exposes ingress.

## Components
- **Envoy proxy**: TLS termination + L7 routing for external traffic (HTTPS 443/8443). Configured dynamically via xDS.
- **xds service** (binary `xds`): Control plane for Envoy; builds clusters/listeners from service registry data (etcd). Manages secrets for TLS via SDS.
- **gateway** service (binary `gateway` / `globular-gateway`): Internal HTTP/2 + gRPC entry; handles REST→gRPC translation and front-door logic for internal services.
- **Service mesh**: All services register in etcd; xDS renders clusters/endpoints; Envoy routes to services over mTLS.
- **SDS (Secret Discovery Service)**: Envoy TLS certs delivered dynamically by xDS; hot rotation supported. Insecure xDS is blocked when SDS is enabled (see `docs/envoy-sds.md`).

## Data sources
- **Service registry**: etcd entries (service Id, Port/Proxy, health) drive cluster endpoints.
- **TLS secrets**: Provided to Envoy via xDS/SDS; cert management handled in platform (see PKI/certs).
- **Domains/routes**: Envoy virtual hosts map subdomains/path prefixes to services (e.g., `mcp.<domain>/mcp` → MCP HTTP).

## Flow (external ingress)
1. Client → Envoy (HTTPS 443) with SNI/Host header.
2. Envoy matches vhost/route → cluster from xDS.
3. Upstream to gateway or directly to service (depending on route) over mTLS.
4. Services authenticate via JWT + mTLS.

## Flow (mesh service-to-service)
1. Service discovers peer via etcd (or Envoy sidecar route).
2. Traffic routed through Envoy using xDS clusters; mTLS enforced.

## Ports
- External: 443/8443 (Envoy).
- Gateway: dynamically allocated from `PortsRange` at startup; discover via etcd/MCP.
- xDS: dynamic port; Envoy connects to it for config/SDS.

## Ops tips
- To see live listeners/clusters: check Envoy admin (if enabled) or use xDS logs; for quick view of service ports, use MCP `cluster_get_health` or etcd configs.
- If routing fails: verify service registered in etcd with correct Port/Proxy, ensure xds is reachable by Envoy, and confirm TLS secret delivery.
- For MCP ingress: ensure Envoy route `/mcp` → MCP service; MCP itself listens on loopback port from config (often 10250) if accessed locally, or through Envoy for remote.
- For TLS/SDS correctness: see `docs/envoy-sds.md`; xDS/SDS defaults to mTLS using `/var/lib/globular/config/tls/{fullchain.pem,privkey.pem,ca.pem}`; dev-only override `GLOBULAR_XDS_INSECURE=1` should not be used in prod.

## Example Envoy route (MCP)
```yaml
virtual_hosts:
  - name: mcp-admin
    domains: ["mcp.globular.internal", "mcp.*"]
    routes:
      - match: { prefix: "/mcp" }
        route:
          cluster: globular-mcp
          timeout: 60s
      - match: { prefix: "/health" }
        route:
          cluster: globular-mcp
          timeout: 5s
clusters:
  - name: globular-mcp
    type: STATIC
    load_assignment:
      cluster_name: globular-mcp
      endpoints:
        - lb_endpoints:
            - endpoint:
                address:
                  socket_address:
                    address: 127.0.0.1
                    port_value: 10250
```

## Example Envoy route (Gateway as front door)
Shared ingress listener that routes `/api/` to the gateway service cluster (gateway runs on a dynamic Port/Proxy from the allocator; replace `GATEWAY_PORT` with the live port from etcd/MCP).

```yaml
virtual_hosts:
  - name: gateway
    domains: ["*.globular.internal", "globular.internal"]
    routes:
      - match: { prefix: "/api/" }
        route:
          cluster: globular-gateway
          timeout: 30s
clusters:
  - name: globular-gateway
    type: STATIC
    load_assignment:
      cluster_name: globular-gateway
      endpoints:
        - lb_endpoints:
            - endpoint:
                address:
                  socket_address:
                    address: 127.0.0.1
                    port_value: GATEWAY_PORT
```

## Quickstart: run xDS + Envoy locally (outline)
1. Generate/obtain TLS bundle: `/var/lib/globular/config/tls/{fullchain.pem,privkey.pem,ca.pem}`.
2. Start xDS (mTLS): `globular-xds --addr :18000` (or set env to point to certs; default paths used if present).
3. Start Envoy with bootstrap pointing ADS/SDS to xDS at `:18000`, using the same certs for client mTLS.
4. Ensure service registry (etcd) has the MCP service entry (Port/Proxy). xDS builder will emit cluster/listener.
5. Hit `https://mcp.globular.internal/mcp` through Envoy; MCP should answer via local `127.0.0.1:10250`.

For gateway front door: ensure the gateway service is registered in etcd with its allocated port; update the cluster address/port accordingly in the Envoy bootstrap or xDS snapshot.

## Minimal Envoy bootstrap (ADS/SDS, mTLS)
```yaml
static_resources:
  clusters:
    - name: xds_cluster
      type: STRICT_DNS
      connect_timeout: 2s
      lb_policy: ROUND_ROBIN
      load_assignment:
        cluster_name: xds_cluster
        endpoints:
          - lb_endpoints:
              - endpoint:
                  address:
                    socket_address:
                      address: 127.0.0.1
                      port_value: 18000  # xDS server port
      http2_protocol_options: {}
      transport_socket:
        name: envoy.transport_sockets.tls
        typed_config:
          "@type": type.googleapis.com/envoy.extensions.transport_sockets.tls.v3.UpstreamTlsContext
          common_tls_context:
            tls_certificates:
              - certificate_chain: { filename: "/var/lib/globular/config/tls/fullchain.pem" }
                private_key:      { filename: "/var/lib/globular/config/tls/privkey.pem" }
            validation_context:
              trusted_ca:
                filename: "/var/lib/globular/config/tls/ca.pem"
dynamic_resources:
  ads_config:
    api_type: GRPC
    transport_api_version: V3
    grpc_services:
      - envoy_grpc:
          cluster_name: xds_cluster
  lds_config: { ads: {} }
  cds_config: { ads: {} }
  sds_config: { ads: {} }
node:
  id: envoy-node-1
  cluster: globular
admin:
  access_log_path: /tmp/admin_access.log
  address:
    socket_address: { address: 127.0.0.1, port_value: 9901 }
```
Notes:
- Uses the same cert/key/CA for client mTLS to xDS. If SDS is enabled (recommended), xDS must be TLS.
- Replace `port_value: 18000` with the live xDS port; set `node.id`/`node.cluster` to match xDS snapshots.
- To consume SDS-delivered certs for ingress listeners, set `transport_socket` on the listener to reference SDS secrets (see `docs/envoy-sds.md`); common secret names: `internal-server-cert`, `public-ingress-cert`, `internal-ca-bundle`.
