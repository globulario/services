# Upgrades

## Service Upgrade Flow

1. **Build** new version: `globular pkg build --spec <yaml> --version <V> --build-number <N>`
2. **Publish** to repository: `globular pkg publish --file <tgz>`
3. **Set desired state**: `globular services desired set <name> <version> --build-number <N>`
4. **Wait for convergence**: The drift reconciler (30s cycle) detects the version mismatch and triggers installation on each node

## Rolling Upgrades

For control-plane services (cluster-controller), the `release.apply.controller` workflow handles leader-aware rolling updates:

1. Discovers all controller instances (leader + followers)
2. Updates followers first, verifies health
3. Transfers leadership to an upgraded follower
4. Updates the old leader last

## Manual Upgrade (Single Service)

```bash
# Stop, replace binary, start
sudo systemctl stop globular-<service>.service
sudo cp /tmp/<new_binary> /usr/lib/globular/bin/<service>_server
sudo systemctl start globular-<service>.service
```

## Verification After Upgrade

```bash
# Check service is running
systemctl status globular-<service>.service

# Verify version
globular pkg info <service>

# Check cluster convergence
globular services list-desired

# Verify integrity
globular services verify-integrity
```

## Rollback

Set the desired state back to the previous version:

```bash
globular services desired set <name> <previous_version> --build-number <previous_build>
```

The reconciler will re-install the previous version on all nodes.

## Safety Rules

- Always verify the new binary before deploying cluster-wide
- Check that the service starts and passes health checks on one node first
- For the cluster controller, use the leader-aware rollout workflow
- Never force-push to all nodes simultaneously without testing
