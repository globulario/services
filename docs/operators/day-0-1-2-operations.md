# Day-0, Day-1, Day-2 Operations

Globular operations are organized into three phases. Day-0 gets the cluster running. Day-1 makes it production-ready. Day-2 keeps it healthy over time. This page provides a complete timeline from first boot to ongoing maintenance.

## Day-0: First Boot

Day-0 is about going from bare metal to a running cluster. At the end of Day-0, you have a single-node Globular cluster with core services operational.

### What Happens on Day-0

```
Bare machine
    │
    ▼
Install binaries (/usr/local/bin/, /var/lib/globular/packages/)
    │
    ▼
Start Node Agent (port 11000)
    │
    ▼
Bootstrap cluster:
    ├── Create etcd cluster (single node)
    ├── Generate internal CA + node certificates
    ├── Generate Ed25519 signing keys
    ├── Start Cluster Controller (port 12000)
    ├── Install core services (auth, RBAC, event, discovery, repository)
    ├── Install gateway (Envoy, xDS)
    ├── Initialize RBAC (root admin, service accounts)
    ├── Publish packages to repository
    └── Seed desired state
    │
    ▼
Running single-node cluster
```

### Day-0 Commands

```bash
# 1. Install Globular binaries
sudo ./install.sh

# 2. Start the Node Agent
sudo systemctl start globular-node-agent

# 3. Bootstrap
globular cluster bootstrap \
  --node localhost:11000 \
  --domain mycluster.local \
  --profile core \
  --profile gateway

# 4. Set root password
globular auth root-passwd --password <strong-password>

# 5. Verify
globular cluster health
globular services desired list
```

### Day-0 Checklist

- [ ] Binaries installed on first node
- [ ] Node Agent running
- [ ] Bootstrap completed without errors
- [ ] `globular cluster health` shows HEALTHY
- [ ] `globular services desired list` shows all services INSTALLED
- [ ] Root admin password set
- [ ] Can authenticate: `globular auth login --username admin`

### What You Have After Day-0

- Single-node cluster running all core services
- Internal PKI (Globular Root CA, node certificates, Ed25519 keys)
- etcd with cluster state
- Package repository with all service packages
- Envoy gateway accepting traffic on ports 443 and 8443
- gRPC service discovery working via etcd
- RBAC enforcing permissions on all RPCs

### What You Don't Have Yet

- High availability (single node = single point of failure)
- External DNS or public certificates
- Backup configuration
- Monitoring alerts
- Additional compute nodes

See [Installation](installation.md) for the detailed bootstrap walkthrough.

---

## Day-1: Production Ready

Day-1 transforms a single-node cluster into a production-ready system. It adds nodes for HA, configures external access, sets up backups, and enables monitoring.

### Day-1 Timeline

```
Day-0 complete (single node)
    │
    ▼
Phase 1: Add nodes for HA
    ├── Join node-2 with core + gateway profiles
    ├── Join node-3 with core + compute profiles
    ├── etcd expands to 3-member quorum
    ├── MinIO expands with erasure coding
    └── Controller HA (leader + standby)
    │
    ▼
Phase 2: External access
    ├── Install keepalived for VIP failover
    ├── Configure router DMZ → VIP
    ├── Register external domain (globular domain add)
    ├── Obtain Let's Encrypt wildcard certificate
    └── Set up split-horizon DNS (/etc/hosts for hairpin NAT)
    │
    ▼
Phase 3: Backup and monitoring
    ├── Configure backup destinations
    ├── Run first backup
    ├── Save recovery seed
    ├── Verify Prometheus is scraping
    └── Configure Alertmanager notifications
    │
    ▼
Phase 4: Security hardening
    ├── Create operator accounts (not just root admin)
    ├── Bind RBAC roles (publisher for CI, operator for SREs)
    ├── Verify bootstrap flag expired
    └── Audit service account permissions
    │
    ▼
Production-ready cluster
```

### Phase 1: Add Nodes

```bash
# Create join token
globular cluster token create --expires 4h

# On node-2: join
globular cluster join \
  --node node-2:11000 \
  --controller node-1:12000 \
  --join-token <token>

# Approve with profiles
globular cluster requests approve <req-id> \
  --profile core --profile gateway

# Repeat for node-3
globular cluster join \
  --node node-3:11000 \
  --controller node-1:12000 \
  --join-token <token>

globular cluster requests approve <req-id> \
  --profile core --profile compute

# Verify HA
globular cluster health
# 3/3 nodes healthy
# etcd: 3 members (quorum = 2, tolerates 1 failure)
```

See [Adding Nodes](adding-nodes.md) for the full procedure.

### Phase 2: External Access

```bash
# Install keepalived on gateway nodes
sudo apt-get install -y keepalived  # on each gateway node

# Write ingress spec (keepalived auto-configures via node agent)
etcdctl put /globular/ingress/v1/spec '{
  "version": "v1",
  "mode": "vip_failover",
  "vip_failover": {
    "vip": "10.0.0.100/24",
    "interface": "eth0",
    "interface_override": {"<node-2-id>": "eno1", "<node-1-id>": "wlp5s0"},
    "virtual_router_id": 51,
    "advert_interval_ms": 1000,
    "auth_pass": "yoursecret",
    "participants": ["<node-1-id>", "<node-2-id>"],
    "priority": {"<node-1-id>": 120, "<node-2-id>": 110},
    "preempt": true,
    "check_tcp_ports": [443, 8443]
  }
}'

# Configure router DMZ → VIP (10.0.0.100)

# Register domain with ACME wildcard cert
globular domain provider add --name local-dns --type local --zone yourdomain.com
globular domain add \
  --fqdn yourdomain.com \
  --zone yourdomain.com \
  --provider local-dns \
  --target-ip <your-public-ip> \
  --enable-acme \
  --acme-email admin@yourdomain.com \
  --use-wildcard-cert

# Wait for cert (check status)
globular domain status

# Add /etc/hosts on each node for hairpin NAT
echo "10.0.0.100 yourdomain.com www.yourdomain.com" | sudo tee -a /etc/hosts
```

See [Keepalived and Ingress](keepalived-and-ingress.md) and [DNS and PKI](dns-and-pki.md) for details.

### Phase 3: Backup and Monitoring

```bash
# Verify backup tools
globular backup preflight-check

# Run first cluster backup
globular backup create --mode cluster

# Verify backup
globular backup list
globular backup validate <backup-id>

# Save recovery seed for disaster recovery
globular backup apply-recovery-seed

# Verify Prometheus
globular metrics targets
# All targets should show "up"

# Verify alerts
globular metrics alerts
```

See [Backup and Restore](backup-and-restore.md) and [Observability](observability.md).

### Phase 4: Security Hardening

```bash
# Create operator account
globular auth create-account --username sre-operator --type user
globular rbac bind --subject sre-operator --role globular-operator

# Create CI publisher account
globular auth create-account --username ci-publisher --type application
globular rbac bind --subject ci-publisher --role globular-publisher

# Verify bootstrap window expired
ls /var/lib/globular/bootstrap.enabled
# Should not exist (expired automatically after 30 minutes)

# Audit RBAC bindings
globular rbac bindings --subject admin
globular rbac bindings --subject sre-operator
globular rbac bindings --subject ci-publisher
```

See [Security](security.md) for the complete security model.

### Day-1 Checklist

- [ ] 3+ nodes joined and healthy
- [ ] etcd quorum across 3 nodes
- [ ] Controller HA (leader + standby confirmed)
- [ ] keepalived VIP responding (ping VIP)
- [ ] Router DMZ configured → VIP
- [ ] Let's Encrypt wildcard cert serving via Envoy
- [ ] External HTTPS working from internet
- [ ] `/etc/hosts` set on all nodes for hairpin NAT
- [ ] First backup completed and validated
- [ ] Recovery seed saved
- [ ] Prometheus scraping all targets
- [ ] Operator accounts created with proper RBAC roles
- [ ] Bootstrap flag expired
- [ ] DNS zones registered on all DNS instances

---

## Day-2: Ongoing Operations

Day-2 is everything after the cluster is production-ready. It covers routine maintenance, upgrades, monitoring, incident response, and capacity management.

### Routine Operations

#### Daily

```bash
# Quick health check
globular cluster health

# Check for failed workflows
globular workflow list --status FAILED

# Check doctor for findings
globular doctor report
```

#### Weekly

```bash
# Full diagnostic scan
globular doctor report --fresh

# Verify backups are running
globular backup list --limit 7
# Expect 7 daily backups (or however many your schedule produces)

# Check certificate expiry
globular node certificate-status --node <node>:11000
# All certs should have > 30 days remaining

# Check Let's Encrypt cert
openssl x509 -in /var/lib/globular/domains/<domain>/fullchain.pem -noout -dates

# Check AI executor (if enabled)
globular ai executor status
globular ai executor jobs --state FAILED --limit 10
```

#### Monthly

```bash
# Validate a backup (deep integrity check)
globular backup validate <recent-backup-id> --deep

# Review drift across all 4 layers
globular cluster get-drift-report

# Check disk usage on all nodes
ssh node-1 df -h /var/lib/globular /var/lib/etcd
ssh node-2 df -h /var/lib/globular /var/lib/etcd
ssh node-3 df -h /var/lib/globular /var/lib/etcd

# Review RBAC bindings
globular rbac bindings --subject admin
# Remove any stale accounts

# Test failover (planned)
# On the master node:
sudo systemctl stop globular-cluster-controller
# Verify standby takes over:
globular cluster health
# Restart:
sudo systemctl start globular-cluster-controller
```

#### Quarterly

```bash
# Test disaster recovery
# 1. Restore a backup to a test environment
# 2. Verify services start correctly
# 3. Validate data integrity

# Review and update desired state
globular services desired list
# Check for deprecated packages, orphaned artifacts
globular cluster get-drift-report

# Compact etcd
etcdctl compact $(etcdctl endpoint status --write-out="json" | jq '.header.revision')
etcdctl defrag

# Review AI memory for stale entries
globular ai memory list --type SCRATCH
# Clean up expired scratch memories
```

### Service Upgrades

```bash
# Publish new version
globular pkg publish <new-package.tgz>

# Set desired state
globular services desired set <service> <new-version>

# Monitor
globular services desired list
globular workflow list --service <service>

# If problems: roll back
globular services desired set <service> <previous-version>
```

See [Updating the Cluster](updating-the-cluster.md) for the full upgrade guide.

### Adding Capacity

```bash
# Create token, join new node
globular cluster token create --expires 4h
globular cluster join --node <new-node>:11000 --controller <controller>:12000 --join-token <token>
globular cluster requests approve <req-id> --profile worker --profile compute

# Update keepalived if the new node is a gateway
# Edit the ingress spec in etcd to add the new participant

# Update /etc/hosts on the new node for hairpin NAT
echo "10.0.0.100 yourdomain.com www.yourdomain.com" | sudo tee -a /etc/hosts
```

### Incident Response

```bash
# 1. Assess scope
globular cluster health
globular doctor report --fresh

# 2. Identify root cause
globular workflow list --status FAILED
globular doctor explain <finding-id>

# 3. Check AI diagnosis (if enabled)
globular ai executor jobs --limit 5

# 4. Fix
# Follow troubleshooting in Debugging Failures or Failure Scenarios docs

# 5. Verify recovery
globular cluster health
globular cluster get-drift-report
```

See [Debugging Failures](debugging-failures.md) and [Failure Scenarios](failure-scenarios.md).

### DNS Zone Recovery

After DNS service restart, managed zones may be lost from memory. Re-register on all DNS instances:

```bash
# Check current zones
globular dns domains list

# Re-add if missing (must be done on each DNS instance)
globular dns domains set globular.internal. yourdomain.com.
```

**Known issue**: Managed domain lists are in-memory. A code fix to persist zones to ScyllaDB is recommended.

### Certificate Renewal

**Internal certificates** (Globular CA): Renewed automatically on node restart. Doctor warns 30 days before expiry.

**External certificates** (Let's Encrypt): The domain reconciler auto-renews 30 days before expiry. Verify:

```bash
globular domain status
# CERT column should show ✓

# Force renewal if needed
sudo touch /var/lib/globular/domains/<domain>/.renew-requested
```

### Day-2 Checklist (Print and Keep)

| Frequency | Task | Command |
|-----------|------|---------|
| Daily | Health check | `globular cluster health` |
| Daily | Failed workflows | `globular workflow list --status FAILED` |
| Weekly | Doctor report | `globular doctor report --fresh` |
| Weekly | Backup verification | `globular backup list --limit 7` |
| Weekly | Cert expiry | `globular node certificate-status --node <node>:11000` |
| Monthly | Deep backup validation | `globular backup validate <id> --deep` |
| Monthly | Drift check | `globular cluster get-drift-report` |
| Monthly | Disk usage | `df -h /var/lib/globular /var/lib/etcd` on each node |
| Monthly | Test failover | Stop controller → verify HA → restart |
| Quarterly | Disaster recovery test | Restore backup to test env |
| Quarterly | etcd compaction | `etcdctl compact` + `etcdctl defrag` |
| Quarterly | RBAC audit | Review role bindings, remove stale accounts |

## What's Next

- [Installation](installation.md) — Detailed Day-0 bootstrap walkthrough
- [Adding Nodes](adding-nodes.md) — Day-1 cluster expansion
- [High Availability](high-availability.md) — HA architecture and failover
- [Backup and Restore](backup-and-restore.md) — Backup configuration and disaster recovery
