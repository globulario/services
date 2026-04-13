# Resource Service

## Purpose

The Resource Service manages identity, access, and organizational metadata for the cluster. It is the authority for accounts, groups, organizations, and package bundle descriptors.

## Responsibilities

- Account management (create, update, delete users)
- Group management (membership, hierarchy)
- Organization management (multi-tenancy)
- Package bundle descriptor storage (used by repository during publish)
- Identity context resolution for RBAC decisions

## gRPC Service

`resource.ResourceService` — Port 10011

## Key RPCs

- `GetAccountIdentityContext` — Resolve identity for access decisions
- `GetGroupIdentityContext` — Resolve group membership
- `GetOrganizationIdentityContext` — Resolve org-level identity
- `SetPackageBundle` — Register package metadata during publish
- `GetPackageBundleChecksum` — Verify bundle integrity

## Dependencies

- etcd (state storage)
- Authentication service (token validation)
- MongoDB (persistent storage for account data)
