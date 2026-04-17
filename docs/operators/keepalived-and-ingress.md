# Keepalived and Ingress Networking

This page covers how Globular uses keepalived for high-availability network ingress: floating VIPs, automatic failover between gateway nodes, health-gated VIP promotion, and integration with consumer/ISP routers via DMZ.

## Why Keepalived

In a multi-node Globular cluster, the Envoy gateway runs on two or more nodes. External traffic enters through one of these gateways. Without keepalived, you must point your router's port forwarding at a specific node IP. If that node goes down, external traffic is dead until you manually reconfigure the router.

Keepalived solves this by managing a **Virtual IP (VIP)** — a floating IP address that automatically moves between gateway nodes. The router points at the VIP, and keepalived ensures it's always on a healthy node.

```
Before keepalived:
  Router → 10.0.0.63 (globule-ryzen)
  If ryzen dies → external traffic dead, manual intervention required

After keepalived:
  Router DMZ → 10.0.0.100 (VIP, managed by keepalived)
  If ryzen dies → VIP moves to globule-nuc in ~3 seconds → traffic continues
  Router config never changes
```

## How It Works

### VRRP Protocol

Keepalived implements VRRP (Virtual Router Redundancy Protocol). Gateway nodes form a VRRP group where one is MASTER (holds the VIP) and others are BACKUP. The MASTER sends advertisements every second. If backups stop hearing advertisements, the highest-priority backup promotes itself to MASTER and takes the VIP.

### Health-Gated Failover

Globular configures keepalived with TCP health checks on the gateway ports (443, 8443). If the Envoy gateway process crashes on the MASTER node — even if the node itself is still running — the health check fails and the VIP moves to a healthy BACKUP. This provides service-level failover, not just node-level.

### Integration with Globular

Keepalived is not managed through manual config files. It is integrated into the Node Agent's **ingress reconciliation loop**:

1. The ingress spec is stored in etcd at `/globular/ingress/v1/spec`
2. Every Node Agent watches this key (every 30 seconds + instantly on change)
3. Nodes listed as participants get keepalived configured and started automatically
4. Non-participant nodes have keepalived disabled
5. Each node reports its VRRP state to etcd at `/globular/ingress/v1/status/{node_id}`

This means: you change the ingress spec in etcd, and every node auto-configures within 30 seconds. No SSH, no manual config files.

## Architecture

<img src="/docs/assets/diagrams/network-topology.svg" alt="Keepalived VIP network topology" style="width:100%;max-width:800px">

### Failover Scenarios

| Scenario | What Happens | Failover Time |
|----------|-------------|---------------|
| MASTER node powers off | BACKUP stops hearing advertisements, promotes to MASTER, takes VIP | ~3 seconds |
| Gateway process crashes on MASTER | TCP health check fails (ports 443/8443 not listening), VIP moves to BACKUP | ~4-6 seconds (2 failed checks) |
| Network partition (MASTER loses uplink) | BACKUP doesn't hear advertisements, takes VIP; MASTER also keeps VIP (split-brain, resolved when partition heals) | ~3 seconds |
| MASTER recovers after failover | With preempt=true, MASTER reclaims VIP (higher priority); with preempt=false, BACKUP keeps VIP until it fails | ~1 second (preempt) |

## Prerequisites

### Keepalived Binary

Keepalived must be installed on every node that participates in VIP failover. There are two ways to install it:

**Via the Globular package** (preferred — managed by the convergence model):
```bash
# The keepalived infrastructure package is already in the repository
globular pkg info keepalived
# Shows: keepalived 0.0.1 PUBLISHED

# Deploy to gateway nodes
globular services desired set keepalived 0.0.1
```

**Via apt** (quick setup, not managed by Globular):
```bash
# On each gateway node
sudo apt-get install -y keepalived
```

The Node Agent checks for the keepalived binary at `/usr/sbin/keepalived` or via `which keepalived`. If it's not installed, the reconciliation reports an error in etcd status.

### Network Interfaces

Each participating node needs a network interface on the same subnet as the VIP. The interface names can differ between nodes — the ingress spec supports per-node interface overrides.

## Configuration

### The Ingress Spec

The ingress configuration is a single JSON document stored in etcd:

```json
{
  "version": "v1",
  "mode": "vip_failover",
  "vip_failover": {
    "vip": "10.0.0.100/24",
    "interface": "eth0",
    "interface_override": {
      "<nuc-node-id>": "eno1",
      "<ryzen-node-id>": "wlp5s0"
    },
    "virtual_router_id": 51,
    "advert_interval_ms": 1000,
    "auth_pass": "globvrrp",
    "participants": [
      "<ryzen-node-id>",
      "<nuc-node-id>"
    ],
    "priority": {
      "<ryzen-node-id>": 120,
      "<nuc-node-id>": 110
    },
    "preempt": true,
    "check_tcp_ports": [443, 8443]
  }
}
```

### Spec Fields

| Field | Required | Description |
|-------|----------|-------------|
| `vip` | Yes | The floating IP address with CIDR notation (e.g., `10.0.0.100/24`). This IP must not be assigned to any machine. |
| `interface` | Yes | Default network interface for the VIP. Used for nodes not in `interface_override`. |
| `interface_override` | No | Map of node ID → interface name. Use when nodes have different interface names (e.g., WiFi `wlp5s0` vs wired `eno1`). |
| `virtual_router_id` | Yes | VRRP router ID (1-255). Must be unique on the network segment. Use 51 unless you have other VRRP groups. |
| `advert_interval_ms` | Yes | How often the MASTER sends advertisements, in milliseconds. 1000 (1 second) is the standard value. |
| `auth_pass` | No | VRRP authentication password. All participants must use the same password. Prevents rogue keepalived instances from joining. |
| `participants` | Yes | List of node IDs that participate in VIP election. Only these nodes run keepalived. |
| `priority` | Yes | Map of node ID → keepalived priority (1-254). Higher priority = more likely to become MASTER. |
| `preempt` | Yes | If `true`, a higher-priority node reclaims MASTER when it recovers. If `false`, the current MASTER keeps the VIP until it fails. |
| `check_tcp_ports` | No | TCP ports to health-check. If any port stops listening, the node gives up the VIP. Use `[443, 8443]` for gateway health. |

### Writing the Spec to etcd

Using etcdctl:
```bash
etcdctl put /globular/ingress/v1/spec '{
  "version": "v1",
  "mode": "vip_failover",
  "vip_failover": {
    "vip": "10.0.0.100/24",
    "interface": "eth0",
    "interface_override": {
      "814fbbb9-607f-5144-be1a-a863a0bea1e1": "eno1",
      "eb9a2dac-05b0-52ac-9002-99d8ffd35902": "wlp5s0"
    },
    "virtual_router_id": 51,
    "advert_interval_ms": 1000,
    "auth_pass": "globvrrp",
    "participants": [
      "eb9a2dac-05b0-52ac-9002-99d8ffd35902",
      "814fbbb9-607f-5144-be1a-a863a0bea1e1"
    ],
    "priority": {
      "eb9a2dac-05b0-52ac-9002-99d8ffd35902": 120,
      "814fbbb9-607f-5144-be1a-a863a0bea1e1": 110
    },
    "preempt": true,
    "check_tcp_ports": [443, 8443]
  }
}'
```

Within 30 seconds, all node agents reconcile: participants start keepalived, non-participants disable it.

### Disabling Keepalived

To disable VIP failover across the cluster:

```bash
etcdctl put /globular/ingress/v1/spec '{"version":"v1","mode":"disabled"}'
```

All node agents will stop and disable keepalived within 30 seconds.

## Router Configuration with DMZ

### The Problem with Port Forwarding

Consumer and ISP routers (like Videotron Helix) often have simplified interfaces that only allow port forwarding to **discovered devices** — you can't type a custom IP address. Since the VIP (10.0.0.100) is a virtual address managed by keepalived, the router may not discover it as a "device."

### The DMZ Solution

DMZ (Demilitarized Zone) on a consumer router means: **forward ALL incoming traffic to this one IP address.** This is simpler and more reliable than port forwarding rules:

1. In your router's admin panel, find the DMZ setting (usually under Advanced or Security)
2. Set the DMZ host to the VIP address (e.g., `10.0.0.100`)
3. Save — all external traffic now reaches the VIP

**Why DMZ is safe here:**
- The Envoy gateway only listens on configured ports (443, 8443)
- All other ports are closed — traffic to unopened ports is rejected
- The gateway handles TLS termination and authentication
- The gateway is specifically designed to be the internet-facing entry point

### Why DMZ Is Better Than Port Forwarding

| Aspect | Port Forwarding | DMZ |
|--------|----------------|-----|
| Configuration | One rule per port (443, 8443, etc.) | Single setting |
| New ports | Must add a new rule | Automatic |
| Maintenance | Rules can break or conflict | Set once, never change |
| VIP support | May not allow custom IPs | Usually accepts any IP |
| Failover | Rules point to a specific device | DMZ points to VIP, failover is transparent |

### Complete Network Flow

```
User browser (internet)
    │
    ▼
ISP Router (public IP, e.g., 96.20.133.54)
    │ DMZ → 10.0.0.100
    ▼
VIP 10.0.0.100 (keepalived, on active gateway node)
    │
    ▼
Envoy Gateway (ports 443, 8443)
    │ TLS termination, xDS routing
    ▼
Internal gRPC services (authentication, RBAC, repository, etc.)
```

If the active gateway node fails, keepalived moves the VIP to the backup node within 3 seconds. The router's DMZ setting doesn't change. External users experience at most a 3-second interruption.

## Monitoring

### Check VRRP Status

Query the ingress status from etcd:

```bash
etcdctl get /globular/ingress/v1/status/ --prefix
```

Each participating node reports:
```json
{
  "node_id": "eb9a2dac-...",
  "phase": "Ready",
  "vrrp_state": "MASTER",
  "has_vip": true,
  "vip": "10.0.0.100/24",
  "updated_at_unix": 1712937600
}
```

### Check Keepalived Service

```bash
# On any node
sudo systemctl status keepalived

# Check which node holds the VIP
ip addr show | grep 10.0.0.100
# If output appears, this node is MASTER
```

### Check VIP Reachability

```bash
# From any machine on the network
ping 10.0.0.100

# Test gateway through VIP
curl -sk https://10.0.0.100:443
```

### Check External Access

```bash
# Test from outside the network (or use your public IP)
curl -sk https://<your-public-ip>:443
```

## Choosing the VIP Address

The VIP must be:
- On the same subnet as the participating nodes (e.g., 10.0.0.x/24)
- Not assigned to any machine (not via DHCP or static)
- Not in the DHCP pool range (to prevent the router from assigning it to a new device)

**Good choices**: Pick an address at the high end of the subnet that's outside the typical DHCP range. Common DHCP pools use .2 through .200, so addresses like `.100`, `.250`, or `.251` are usually safe.

**Important**: Do not use a node's primary IP as the VIP. Changing a node's primary IP breaks TLS certificates (SANs), etcd cluster membership, and all service registrations. The VIP must be a separate address that keepalived adds as a secondary IP.

## Handling Different Interface Names

In real clusters, nodes often have different network interface names:
- Wired: `eth0`, `eno1`, `enp6s0` (varies by hardware)
- WiFi: `wlan0`, `wlp3s0`, `wlp5s0` (varies by hardware)

The ingress spec's `interface_override` field handles this:

```json
{
  "interface": "eth0",
  "interface_override": {
    "node-id-with-wifi": "wlp5s0",
    "node-id-with-eno1": "eno1"
  }
}
```

Each node uses its override if present, falling back to the default `interface` value. The Node Agent validates that the resolved interface exists on the local system before configuring keepalived.

## Practical Scenarios

### Scenario 1: Gateway Node Crashes

```
T+0s:    globule-ryzen (MASTER) loses power
T+1s:    globule-nuc stops hearing VRRP advertisements
T+3s:    globule-nuc promotes to MASTER, takes VIP 10.0.0.100
T+3s:    External traffic routes to globule-nuc via DMZ → VIP
T+3s:    Node Agent on nuc writes MASTER status to etcd

         Users experience: ~3 second interruption, then service resumes

T+10m:   globule-ryzen boots back up
T+10m:   keepalived starts, detects higher priority (120 > 110)
T+10m:   Preempt=true: ryzen reclaims MASTER, VIP moves back
T+10m:   nuc writes BACKUP status to etcd
```

### Scenario 2: Gateway Process Crashes (Node Still Up)

```
T+0s:    Envoy on globule-ryzen crashes (port 443 stops listening)
T+2s:    keepalived health check: ss -lnt | grep :443 → fails
T+4s:    Second health check fails (fall=2 configured)
T+4s:    globule-ryzen enters FAULT state, releases VIP
T+4s:    globule-nuc takes VIP
T+4s:    External traffic flows through nuc

         Meanwhile: systemd restarts Envoy on ryzen
T+10s:   Envoy back, health check passes on ryzen
T+12s:   Two consecutive passes (rise=2): ryzen leaves FAULT
T+12s:   Preempt: ryzen reclaims MASTER
```

### Scenario 3: Adding a Third Gateway Node

```bash
# 1. Install keepalived on the new node
ssh new-node "sudo apt-get install -y keepalived"

# 2. Update the ingress spec to include the new node
# Add the new node ID to participants, priority, and interface_override
etcdctl put /globular/ingress/v1/spec '{
  "version": "v1",
  "mode": "vip_failover",
  "vip_failover": {
    "vip": "10.0.0.100/24",
    "interface": "eth0",
    "interface_override": {
      "node-1-id": "wlp5s0",
      "node-2-id": "eno1",
      "node-3-id": "enp6s0"
    },
    "virtual_router_id": 51,
    "advert_interval_ms": 1000,
    "auth_pass": "globvrrp",
    "participants": ["node-1-id", "node-2-id", "node-3-id"],
    "priority": {
      "node-1-id": 120,
      "node-2-id": 110,
      "node-3-id": 100
    },
    "preempt": true,
    "check_tcp_ports": [443, 8443]
  }
}'

# 3. All three node agents auto-configure within 30 seconds
# The new node becomes the lowest-priority BACKUP
```

## Troubleshooting

### VIP Not Appearing on Any Node

```bash
# Check keepalived service
sudo systemctl status keepalived

# Check keepalived logs
sudo journalctl -u keepalived -n 50

# Check etcd status
etcdctl get /globular/ingress/v1/status/<node-id>
# Look for phase="Error" and last_error
```

Common causes:
- keepalived binary not installed
- Interface name wrong (check `ip link show` for actual names)
- VIP conflicts with an existing IP
- VRRP multicast blocked by network (rare on home networks)

### Both Nodes Claim MASTER (Split-Brain)

This can happen briefly during network partition or keepalived restart. It resolves automatically when nodes can communicate again. The `auth_pass` prevents rogue instances from interfering.

If persistent:
- Check that both nodes can reach each other on the VRRP interface
- Verify `virtual_router_id` is the same on both nodes
- Verify `auth_pass` matches

### VIP Moves Too Frequently (Flapping)

The health check might be too sensitive:
- Increase `fall` count (number of failures before FAULT) in the check script
- Increase `advert_interval_ms` to reduce chatter
- Check if the monitored ports are genuinely unstable

### Router Doesn't See the VIP

If the DMZ setting doesn't accept the VIP address:
- The VIP responds to ARP — it should appear in the router's device list after a few minutes
- Try pinging the VIP from another device to force ARP resolution
- As a last resort, set DMZ to the MASTER node's real IP (less ideal — manual change needed on failover)

## What's Next

- [High Availability](high-availability.md) — Controller leader election, etcd quorum, MinIO erasure coding
- [Network and Routing](network-and-routing.md) — Envoy gateway, xDS, DNS, service discovery
- [Failure Scenarios](failure-scenarios.md) — Infrastructure failure catalog and recovery procedures
