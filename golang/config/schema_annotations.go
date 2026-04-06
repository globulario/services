package config

// This file contains schema annotations for etcd keys owned by the config
// package. These are Tier-0 infrastructure keys that most services read
// but only specific writers own.
//
// See docs/schema.md (generated) and golang/schema_reference/ for the
// extractor and registry.

// +globular:schema:key="/globular/cluster/dns/hosts"
// +globular:schema:writer="cluster-controller"
// +globular:schema:readers="node-agent,cluster-doctor,resource,dns,discovery,config"
// +globular:schema:description="Tier-0 DNS resolver IP list. JSON array of IPv4 addresses. Cannot use DNS to discover DNS."
// +globular:schema:invariants="No loopback addresses; must contain at least one reachable IP; written during bootstrap and node-join"
// +globular:schema:since_version="0.0.1"
type ClusterDNSHosts struct{}

// +globular:schema:key="/globular/cluster/scylla/hosts"
// +globular:schema:writer="cluster-controller"
// +globular:schema:readers="node-agent,workflow,ai-memory,backup-manager,config"
// +globular:schema:description="Tier-0 ScyllaDB seed IP list. JSON array of IPv4 addresses. Used by all ScyllaDB clients to bootstrap CQL connections."
// +globular:schema:invariants="No loopback addresses; must contain at least one reachable seed; written during bootstrap and scylla-join"
// +globular:schema:since_version="0.0.1"
type ClusterScyllaHosts struct{}

// +globular:schema:key="/globular/cluster/minio/config"
// +globular:schema:writer="cluster-controller"
// +globular:schema:readers="node-agent,repository,backup-manager,workflow,config"
// +globular:schema:description="MinIO connection config. JSON with endpoint, access_key, secret_key, secure, bucket, prefix. Single source of truth for all MinIO clients."
// +globular:schema:invariants="Endpoint is a DNS name (never IP); credentials must be valid; written during bootstrap and minio-pool changes"
// +globular:schema:since_version="0.0.1"
type ClusterMinIOConfig struct{}

// +globular:schema:key="/globular/services/{service_id}/config"
// +globular:schema:writer="node-agent"
// +globular:schema:readers="gateway,xds,discovery,config"
// +globular:schema:description="Per-service configuration. JSON with Name, Port, Protocol, TLS, Address, etc. Written by node-agent during install; read by gateway/xDS for routing."
// +globular:schema:invariants="One config per service per cluster; Address must be routable; Port must be in range"
// +globular:schema:since_version="0.0.1"
type ServiceConfig struct{}

// +globular:schema:key="/globular/services/{service_id}/instances/{node_key}"
// +globular:schema:writer="node-agent"
// +globular:schema:readers="gateway,xds,discovery,config"
// +globular:schema:description="Per-node service instance registration. JSON with node-specific address, port, process ID. Written by node-agent on service start; read by gateway/xDS for load balancing."
// +globular:schema:invariants="One instance per (service, node); stale instances cleaned up on node removal"
// +globular:schema:since_version="0.0.1"
type ServiceInstance struct{}

// +globular:schema:key="/globular/services/{service_id}/runtime"
// +globular:schema:writer="node-agent"
// +globular:schema:readers="cluster-controller,monitoring,config"
// +globular:schema:description="Service runtime state. JSON with process status, last restart, health check results. Watchable key for live status updates."
// +globular:schema:invariants="Updated on service start/stop/restart; stale entries indicate node-agent failure"
// +globular:schema:since_version="0.0.1"
type ServiceRuntime struct{}

// +globular:schema:key="/globular/cluster/public-dirs/{path}"
// +globular:schema:writer="file"
// +globular:schema:readers="gateway,authentication"
// +globular:schema:description="Public directory registry. Marks filesystem paths as publicly accessible without authentication. Read by gateway for access control."
// +globular:schema:invariants="Paths must be absolute; only the file service writes entries"
// +globular:schema:since_version="0.0.1"
type PublicDirRegistryEntry struct{}
