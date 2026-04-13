# Ports Reference

Complete listing of all ports used by Globular services and infrastructure. Use this when configuring firewalls, router port forwarding, or network security groups.

## External Ports (Internet-Facing)

These ports must be reachable from the internet if you want external access:

| Port | Protocol | Service | Direction | Purpose |
|------|----------|---------|-----------|---------|
| 443 | TCP/HTTPS | Envoy Gateway | Inbound | External HTTPS traffic, gRPC-Web |
| 8443 | TCP/HTTPS | Envoy Gateway | Inbound | gRPC-Web (alternative port) |
| 80 | TCP/HTTP | Envoy Gateway | Inbound | HTTP → HTTPS redirect, ACME HTTP-01 |
| 53 | TCP+UDP | DNS Service | Inbound | Authoritative DNS (if zone is public) |

**With keepalived + DMZ**: Configure DMZ to the VIP address (e.g., 10.0.0.100). No individual port forwarding needed — DMZ covers all ports.

## Internal Ports (Cluster Communication)

These ports must be open between cluster nodes:

### Control Plane

| Port | Protocol | Service | Purpose |
|------|----------|---------|---------|
| 12000 | gRPC/TLS | Cluster Controller | Cluster management, desired state, node membership |
| 11000 | gRPC/TLS | Node Agent | Local executor, workflow steps, package management |
| 13000 | gRPC/TLS | Workflow Service | Workflow execution and tracking |
| 12005 | gRPC/TLS | Cluster Doctor | Health analysis, drift detection, remediation |

### Infrastructure

| Port | Protocol | Service | Purpose |
|------|----------|---------|---------|
| 2379 | HTTPS | etcd (client) | Configuration, state, service discovery |
| 2380 | HTTPS | etcd (peer) | etcd cluster replication |
| 9000 | HTTPS | MinIO | Object storage (packages, backups, artifacts) |
| 9090 | HTTP | Prometheus | Metrics scraping |
| 9093 | HTTP | Alertmanager | Alert routing and notification |
| 9100 | HTTP | Node Exporter | Host metrics (CPU, memory, disk) |
| 9042 | TCP | ScyllaDB | Database queries (AI memory, DNS storage) |
| 7000 | TCP | ScyllaDB (inter-node) | Gossip-based cluster communication |
| 10000 | TCP | ScyllaDB Manager | Backup and management |

### Core Services

| Port | Protocol | Service | Purpose |
|------|----------|---------|---------|
| 10101 | gRPC/TLS | Authentication | Token generation, validation, password management |
| 10102 | gRPC/TLS | Event | Publish-subscribe event bus |
| 10103 | gRPC/TLS | File | File management |
| 10104 | gRPC/TLS | RBAC | Role-based access control |
| 10006 | gRPC/TLS | DNS | Zone management, record CRUD |
| 10029 | gRPC/TLS | Discovery | Service and package discovery |
| 10100 | gRPC/TLS | Log | Centralized logging |

### AI Services

| Port | Protocol | Service | Purpose |
|------|----------|---------|---------|
| 10200 | gRPC/TLS | AI Memory | Persistent knowledge (ScyllaDB-backed) |
| 10210 | gRPC/TLS | AI Watcher | Event monitoring and incident detection |
| 10220 | gRPC/TLS | AI Router | Dynamic routing policy computation |
| 10230 | gRPC/TLS | AI Executor | Incident diagnosis and remediation |

### Operational Services

| Port | Protocol | Service | Purpose |
|------|----------|---------|---------|
| 10019 | gRPC/TLS | Monitoring | Prometheus API adapter |
| 10040 | gRPC/TLS | Backup Manager | Backup orchestration |
| 10260 | HTTP/TLS | MCP Server | AI agent interface (122+ diagnostic tools) |

### Application Services

| Port | Protocol | Service | Purpose |
|------|----------|---------|---------|
| 10105 | gRPC/TLS | Persistence | Database access layer |
| 10106 | gRPC/TLS | Storage | Object/blob storage |
| 10107 | gRPC/TLS | SQL | SQL database access |
| 10108 | gRPC/TLS | Search | Full-text search |
| 10109 | gRPC/TLS | Mail | SMTP email |
| 10110 | gRPC/TLS | Media | Audio/video management |
| 10111 | gRPC/TLS | Title | Title/metadata service |
| 10112 | gRPC/TLS | Blog | Blog/CMS engine |
| 10113 | gRPC/TLS | Conversation | Chat management |
| 10114 | gRPC/TLS | Catalog | Component catalog |
| 10115 | gRPC/TLS | LDAP | LDAP authentication provider |
| 10116 | gRPC/TLS | Torrent | Torrent downloads |
| 10117 | gRPC/TLS | Resource | Package descriptors, accounts, groups |

### Internal / Management

| Port | Protocol | Service | Purpose |
|------|----------|---------|---------|
| 8081 | HTTP | xDS Server | Envoy configuration streaming (ADS/SDS) |
| 9901 | HTTP | Envoy Admin | Envoy internal admin (localhost only) |

## Keepalived (VRRP)

| Port/Protocol | Purpose |
|--------------|---------|
| VRRP (IP protocol 112) | keepalived advertisements between nodes |
| Multicast 224.0.0.18 | VRRP multicast group |

## Firewall Rules Summary

### Minimal (Single Node)

```bash
# External access
ufw allow 443/tcp    # HTTPS
ufw allow 80/tcp     # HTTP redirect

# If DNS is authoritative
ufw allow 53/tcp
ufw allow 53/udp
```

### Multi-Node Cluster

```bash
# Between all cluster nodes (internal)
ufw allow from 10.0.0.0/24 to any port 2379:2380 proto tcp  # etcd
ufw allow from 10.0.0.0/24 to any port 9000 proto tcp        # MinIO
ufw allow from 10.0.0.0/24 to any port 9042 proto tcp        # ScyllaDB
ufw allow from 10.0.0.0/24 to any port 9090:9100 proto tcp   # Monitoring
ufw allow from 10.0.0.0/24 to any port 10000:13000 proto tcp # All Globular services
ufw allow from 10.0.0.0/24 proto vrrp                         # keepalived

# External access (on gateway nodes only)
ufw allow 443/tcp
ufw allow 80/tcp
ufw allow 53/tcp
ufw allow 53/udp
```
