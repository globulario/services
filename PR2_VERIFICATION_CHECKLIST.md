# PR2 DNS Reconciliation - Runtime Verification Checklist

## 1. Expected Log Lines on Controller Startup

When cluster-controller starts, you should see:

### If cluster_domain is configured:
```
dns reconciler: ENABLED (domain=cluster.local, endpoint=127.0.0.1:10033)
dns reconciler: starting loop (interval=30s)
dns reconciler: generation changed 0 -> 1, reconciling...
dns reconciler: applying N records to DNS service
dns reconciler: applied A=X, AAAA=Y records
dns reconciler: SUCCESS - applied generation 1 (N records for domain cluster.local)
```

### If cluster_domain is NOT configured:
```
dns reconciler: DISABLED (no cluster_domain configured)
```

### On reconciliation errors (non-fatal):
```
dns reconciler: reconciliation error: <error details> (will retry in 30s)
```

### On every 30 seconds (if generation hasn't changed):
```
(No log output - silent when no changes detected)
```

---

## 2. Expected DNS Records After One Reconcile Loop

After the first successful reconciliation, the following records should exist in the DNS service:

### For each node in the cluster:
- **A Record**: `<node-name>.<cluster-domain>` → `<node-ip>`
  - Example: `node-01.cluster.local` → `192.168.1.10`

### For gateway nodes (nodes with "gateway" profile):
- **A Record**: `gateway.<cluster-domain>` → `<gateway-node-ip>` (one per gateway node)
  - Example: `gateway.cluster.local` → `192.168.1.10`
  - If multiple gateways: multiple A records returned (round-robin)

### For control-plane nodes (nodes with "control-plane" profile):
- **SRV Record**: `_cluster-controller._tcp.<cluster-domain>` → `<node-fqdn>:12000`
  - Example: `_cluster-controller._tcp.cluster.local` → `node-01.cluster.local:12000`
  - Priority: 10, Weight: 100
  - **Status**: Currently logged as "implementation pending" (SRV not fully implemented)

---

## 3. Verification Commands

### Using `dig` (DNS query tool):

#### Test node A record:
```bash
dig @localhost -p 10053 node-01.cluster.local A
```
**Expected output:**
- Answer section should show: `node-01.cluster.local. 60 IN A 192.168.1.10`

#### Test gateway A record:
```bash
dig @localhost -p 10053 gateway.cluster.local A
```
**Expected output:**
- Answer section should show one or more A records for gateway nodes
- Example: `gateway.cluster.local. 60 IN A 192.168.1.10`

#### Test SRV record (once implemented):
```bash
dig @localhost -p 10053 _cluster-controller._tcp.cluster.local SRV
```
**Expected output:**
- Answer section should show SRV records pointing to controller nodes

### Using DNS gRPC API (via globularcli or direct gRPC):

#### Query A record via API:
```bash
# Assuming globularcli has DNS query commands
globularcli dns get-a node-01.cluster.local
```

#### List all managed domains:
```bash
globularcli dns domains get
```
**Expected output:**
- Should include `cluster.local` (or your configured cluster_domain)

### Check cluster-controller logs:
```bash
journalctl -u cluster-controller -f | grep "dns reconciler"
```
**Expected patterns:**
- Startup message showing ENABLED or DISABLED
- SUCCESS messages every time generation changes
- ERROR messages if DNS service unreachable (but controller keeps running)

---

## 4. Test Scenarios

### Scenario A: Add a new node to the cluster
1. Add a node via `RequestJoin` + `ApproveJoin`
2. Wait up to 30 seconds for next reconciliation loop
3. Run: `dig @localhost -p 10053 <new-node-name>.cluster.local A`
4. **Expected**: New node's A record should exist

### Scenario B: Mark a node as gateway
1. Assign "gateway" profile to a node
2. Wait up to 30 seconds
3. Run: `dig @localhost -p 10053 gateway.cluster.local A`
4. **Expected**: Gateway A record includes the node's IP

### Scenario C: Cluster with no cluster_domain
1. Start cluster-controller without `CLUSTER_DOMAIN` env var
2. Check logs
3. **Expected**: "dns reconciler: DISABLED" message
4. **Expected**: No DNS reconciliation occurs

### Scenario D: DNS service unavailable
1. Stop DNS service
2. Wait for reconciliation cycle
3. **Expected**: Error logged but controller continues running
4. **Expected**: "dns reconciler: reconciliation error: connect dns: ..." message

---

## 5. Success Criteria

✅ **Pass** if:
- Log shows "dns reconciler: ENABLED" when cluster_domain is set
- Log shows "dns reconciler: SUCCESS" after first reconciliation
- `dig` queries return correct A records for nodes
- `dig` queries return correct A records for gateway
- Adding a node triggers new DNS records within 30 seconds
- DNS service errors don't crash the controller

❌ **Fail** if:
- Controller crashes on DNS reconciliation errors
- No DNS records created after 30 seconds
- Localhost endpoints appear in DNS records
- Log shows "dns reconciler: DISABLED" when cluster_domain IS set

---

## 6. Known Limitations (Current Implementation)

1. **SRV records**: Logged as "implementation pending" - not fully wired to DNS API
2. **Token generation**: Uses placeholder "cluster-controller-token" - needs proper auth integration
3. **IPv6**: AAAA records may not be populated if nodes don't report IPv6
4. **Record deletion**: Reconciler doesn't remove stale records when nodes leave (future enhancement)

---

## 7. Next Steps After Verification

Once PR2 is verified working:
1. Proceed to **PR3** (Control-plane endpoints via DNS)
2. Consider adding:
   - SRV record implementation
   - Token generation integration with security package
   - Record deletion for removed nodes
