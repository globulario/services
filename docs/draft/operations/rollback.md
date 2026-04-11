# Rolling Back a Service Version

Goal: move a service back to a prior package version/build and let the controller roll it out.

## Fast path (using globular CLI)
```bash
# Set desired state to an older version already in the repository
globular services desired set <service-name> <version>
```
- Service name: use dash form (e.g., `cluster-controller`).
- Version: must exist in the repository; controller will converge nodes to it via workflow.

## If the older package is not in the repository
1. Rebuild or locate the desired `.tgz` (see `deploy-service.sh` to build/publish a single service with a lower version/build).
2. Publish it:
   ```bash
   globular pkg publish --repository <repo> --file <pkg.tgz> --force
   ```
3. Set desired state to that version as above.

## Controller target-build (cluster_controller only)
If rolling back the controller itself, also set `/globular/system/controller-target-build` in etcd with the target version/build/checksum (deploy-service.sh does this when deploying). Example:
```bash
ETCD_EP=https://<etcd-host>:2379
etcdctl --cacert ca.crt --cert service.crt --key service.key \
  put /globular/system/controller-target-build '{"version":"0.0.2","build_number":5,"checksum":"sha256:<...>","set_at":<epoch>}'
```

## Verify rollout
- MCP: `cluster_get_health` to see node counts/versions; `cluster_get_service_workflow_status` for rollout state.
- Logs: controller/workflow logs for rollout or failure.

## Notes
- Ports are dynamic; rollback doesn’t change allocation unless service restarts and allocator reassigns.
- Desired state lives in control plane; repository must contain the target artifact.
