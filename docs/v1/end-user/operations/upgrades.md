# Upgrade a Service

How to update a service to a new version.

## The simple way

```bash
# Publish the new version
globular pkg publish --file my-service_1.1.0.tgz

# Tell Globular to use it
globular services desired set my-service 1.1.0
```

Globular updates every machine automatically within 30 seconds.

## Verify the upgrade

```bash
globular services list-desired
```

All machines should show `match` at the new version.

## Rollback if something is wrong

```bash
globular services desired set my-service 1.0.0
```

The previous version is still in the store. Machines downgrade within 30 seconds.

## Check integrity after upgrade

```bash
globular services verify-integrity
```

This compares installed binaries against the store's checksums.
