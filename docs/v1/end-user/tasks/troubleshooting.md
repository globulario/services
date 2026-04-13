# Troubleshooting

Quick fixes for common problems.

## "Service is not running"

```bash
# Check if it's installed
globular services list-desired

# Check the systemd unit
sudo systemctl status globular-<service>.service

# View recent logs
journalctl -u globular-<service>.service --since "10m ago"
```

## "Cluster health shows unhealthy"

```bash
# See what's wrong
globular doctor report

# Try auto-fix
globular doctor heal
```

## "Desired and installed don't match"

```bash
# Force install on this machine
globular services apply-desired

# Or repair across the cluster
globular services repair
```

## "Service won't start"

Common causes:

| Symptom | Likely cause | Fix |
|---------|-------------|-----|
| "TLS cert not ready" | PKI not provisioned yet | Wait 60s, it retries |
| "port already in use" | Another instance running | `sudo systemctl stop globular-<service>` |
| "etcd unavailable" | etcd not reachable | Check `systemctl status etcd` |
| Binary not found | Package not installed | Re-run `globular services apply-desired` |

## "Package publish failed"

```bash
# Check repository service
sudo systemctl status globular-repository.service

# Check if MinIO is available
globular cluster health
```

## "Node not joining"

```bash
# Verify the token is valid
globular cluster token create  # Generate a fresh one

# Check connectivity
curl -k https://<coordinator>:443/  # Should respond
```

## Collect debug info

```bash
globular support bundle create
```

This creates a tarball with logs, configs, and cluster state for troubleshooting.
