# PR3 Control-Plane Endpoints via DNS - Verification Checklist

## Changes Implemented

**PR3 Scope:** FQDN-based endpoints for control-plane communication (A record lookups only, no SRV)

### 1. Controller DNS A Record (dns_state.go)
- Added `controller.<clusterDomain>` A record pointing to all control-plane nodes
- Generated alongside gateway and node records during reconciliation

### 2. Node-Agent DNS Discovery (server.go)
- Node-agent automatically discovers controller via `controller.<clusterDomain>:12000`
- Only activates in cluster mode when `CLUSTER_DOMAIN` is set
- Falls back to `NODE_AGENT_CONTROLLER_ENDPOINT` or persisted state if DNS not available

### 3. Endpoint Validation (server.go)
- Warns if controller endpoint uses localhost in cluster mode
- Uses existing `ValidateAdvertiseEndpoint` from PR1

---

## Expected Behavior

### On Node-Agent Startup (Cluster Mode with CLUSTER_DOMAIN set):

```
node-agent: using DNS-based controller discovery: controller.cluster.local:12000
```

### If Controller Endpoint is Localhost in Cluster Mode:
```
node-agent: WARNING - controller endpoint uses localhost in cluster mode: localhost:12000 (this may prevent multi-node operation)
```

---

## Verification Commands

### 1. Check Controller DNS A Record

```bash
dig @localhost -p 10053 controller.cluster.local A
```

**Expected Output:**
- Answer section should show: `controller.cluster.local. 60 IN A <controller-node-ip>`
- If multiple control-plane nodes: multiple A records (round-robin DNS)

### 2. Check Node-Agent Logs on Startup

```bash
journalctl -u node-agent -n 50 | grep "controller discovery\|controller endpoint"
```

**Expected Patterns:**
- In cluster mode with domain: `"using DNS-based controller discovery: controller.cluster.local:12000"`
- Not in cluster mode: Uses `NODE_AGENT_CONTROLLER_ENDPOINT` or persisted endpoint

### 3. Verify Controller Reconciler Logs

```bash
journalctl -u cluster-controller | grep "dns reconciler" | tail -5
```

**Expected:**
- `"dns reconciler: applied A=X"` where X includes controller record
- Example: If 1 control-plane node + 1 other node + 1 gateway = at least 3 A records

---

## Test Scenarios

### Scenario A: Single-Node Cluster (Control-Plane)
**Setup:** 1 node with "control-plane" profile, `CLUSTER_DOMAIN=cluster.local`

**Tests:**
1. Start cluster-controller
2. Query: `dig @localhost -p 10053 controller.cluster.local A`
3. **Expected:** Returns controller node's IP
4. Start node-agent on same node
5. **Expected:** Log shows `"using DNS-based controller discovery"`

### Scenario B: Two-Node Cluster Join
**Setup:** Node1 (control-plane) running, Node2 joining

**Tests:**
1. Node1: Verify `controller.cluster.local` resolves to Node1 IP
2. Node2: Set `CLUSTER_DOMAIN=cluster.local` and `NODE_AGENT_CLUSTER_MODE=true`
3. Node2: Start node-agent (without explicit `NODE_AGENT_CONTROLLER_ENDPOINT`)
4. **Expected:** Node2 discovers controller via DNS and connects
5. Node2: Run `RequestJoin` with join token
6. Controller: Approve join
7. **Expected:** Node2 successfully joins using DNS-only discovery

### Scenario C: Multi-Controller (Multiple Control-Plane Nodes)
**Setup:** 2+ nodes with "control-plane" profile

**Tests:**
1. Query: `dig @localhost -p 10053 controller.cluster.local A`
2. **Expected:** Returns multiple A records (one per control-plane node)
3. Node-agent should round-robin or connect to any returned IP

### Scenario D: Fallback to Explicit Endpoint
**Setup:** Node with `NODE_AGENT_CONTROLLER_ENDPOINT=192.168.1.10:12000` set

**Tests:**
1. Start node-agent
2. **Expected:** Uses explicit endpoint (not DNS)
3. Check logs: Should NOT show "using DNS-based controller discovery"

### Scenario E: No CLUSTER_DOMAIN Set
**Setup:** Node without `CLUSTER_DOMAIN` environment variable

**Tests:**
1. Start node-agent
2. **Expected:** Falls back to persisted state or explicit endpoint
3. **Expected:** No DNS discovery attempt

---

## PR3 Acceptance Criteria

✅ **PASS if:**
1. In cluster mode with `CLUSTER_DOMAIN` set:
   - Node-agent uses `controller.<clusterDomain>:12000` endpoint
   - Log shows "using DNS-based controller discovery"
   - No localhost/IP literals in controller endpoint (except fallback)

2. DNS query returns correct controller A record:
   - `dig @localhost -p 10053 controller.cluster.local A` returns control-plane node IP(s)

3. 2-node join works using DNS names only:
   - Node2 can discover and connect to Node1 controller via DNS
   - No explicit IP configuration needed

4. Validation warns on localhost:
   - If controller endpoint is localhost in cluster mode, warning is logged

❌ **FAIL if:**
- Node-agent still uses localhost/IP literals in cluster mode (when DNS available)
- DNS query for `controller.cluster.local` fails or returns no records
- 2-node join fails due to controller discovery issues
- Node-agent crashes when DNS unavailable (should fallback gracefully)

---

## Environment Variables

### New in PR3:
- `CLUSTER_CONTROLLER_PORT` - Controller port (default: 12000)

### Existing (relevant to PR3):
- `CLUSTER_DOMAIN` - Cluster DNS domain (e.g., "cluster.local")
- `NODE_AGENT_CLUSTER_MODE` - Enable cluster mode (default: true)
- `NODE_AGENT_CONTROLLER_ENDPOINT` - Explicit controller endpoint (overrides DNS)

---

## Known Limitations

1. **SRV Records:** Not implemented in PR3 (deferred to PR3.1)
   - Uses A records + port from `CLUSTER_CONTROLLER_PORT`
   - SRV would allow dynamic port discovery

2. **Failover:** Basic round-robin DNS only
   - No health-aware failover between multiple controllers
   - Client retries on connection failure

3. **Bootstrap:** Still requires initial controller reachability
   - First node needs controller running before DNS reconciler starts
   - Subsequent nodes use DNS discovery

---

## Debugging Tips

### Controller Not Resolving via DNS:
```bash
# Check DNS reconciler is running
journalctl -u cluster-controller | grep "dns reconciler: ENABLED"

# Check reconciliation succeeded
journalctl -u cluster-controller | grep "dns reconciler: SUCCESS"

# Verify control-plane profile assigned
# (Controller only creates controller.domain record for nodes with "control-plane" profile)
```

### Node-Agent Not Using DNS:
```bash
# Verify cluster mode enabled
echo $NODE_AGENT_CLUSTER_MODE  # Should be empty or "true"

# Verify cluster domain set
echo $CLUSTER_DOMAIN  # Should be non-empty (e.g., "cluster.local")

# Check for explicit endpoint override
echo $NODE_AGENT_CONTROLLER_ENDPOINT  # Should be empty for DNS discovery
```

---

## Next Steps After PR3 Verification

Once PR3 is verified:
1. Consider **PR3.1** (SRV-based discovery) if needed
2. Proceed to **PR4** (Gateway routing via FQDN)
3. Later: **PR5** (xDS/Envoy DNS integration)
