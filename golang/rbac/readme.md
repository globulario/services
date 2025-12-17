# RBAC Service

The **RBAC Service** is a core Globular microservice that provides **Role-Based Access Control**.  
It defines and enforces permissions for resources, actions, and subjects (accounts, groups, organizations, node identities, roles, applications).

---

## Features

- Fine-grained access control for resources and actions
- Resource-level permissions (read/write/execute/delete)
- Action-level permissions tied to gRPC service methods
- Support for **Allow**, **Deny**, and **Owner** rules
- Inheritance of permissions from parent resources
 - Share/unshare resources with accounts, groups, organizations, node identities, or applications
- Validation of both **actions** and **resource access**
- Public resource handling
- Full integration with Resource and Authentication services

---

## Core Concepts

- **Resource**: Any entity (file, object, service) that can have permissions.
- **Permission**: Action allowed, denied, or ownership defined for a subject.
- **Subject**: An **Account**, **Group**, **Organization**, **Node Identity**, **Role**, or **Application**.
- **Owner**: Owners of a resource have implicit full rights.
- **Inheritance**: Permissions cascade from parent paths.
- **Shares**: Resources can be shared and listed by subject.

---

## gRPC API

The gRPC API is defined in [`rbac.proto`](./rbac.proto).  

Key RPCs:

### Resource Permissions
- **SetResourcePermissions** – Set multiple permissions on a resource.
- **GetResourcePermissions** – Retrieve all permissions for a resource.
- **SetResourcePermission** – Add/update a single permission.
- **GetResourcePermission** – Retrieve one specific permission.
- **DeleteResourcePermission** – Remove one permission.
- **DeleteResourcePermissions** – Remove all permissions from a resource.

### Access Validation
- **ValidateAccess** – Check if a subject can perform an action on a resource.
- **ValidateAction** – Check if a subject can call a gRPC action (method-level validation).

### Resource Sharing
- **GetSharedResource** – List resources shared with a subject.
- **RemoveSubjectFromShare** – Remove a subject from a resource share.
- **DeleteSubjectShare** – Remove all shares involving a subject.

---

## Example Usage

### Go Client

```go
package main

import (
    "fmt"
    "time"
    rbac_client "github.com/globulario/services/golang/rbac/rbac_client"
    "github.com/globulario/services/golang/rbac/rbacpb"
)

func main() {
    client, err := rbac_client.NewRbacService_Client("localhost:10002", "rbac.RbacService")
    if err != nil {
        panic(err)
    }
    defer client.Close()
    client.SetTimeout(5 * time.Second)

    // Define a resource path
    resourcePath := "file:/tmp/test.txt"

    // Set permission: allow read for account_1
    perm := &rbacpb.Permission{
        Name:     "read",
        Accounts: []string{"account_1@globular.io"},
    }
    if err := client.SetResourcePermission("sa@globular.io", resourcePath, "file", perm, rbacpb.PermissionType_ALLOWED); err != nil {
        panic(err)
    }

    // Validate access
    allowed, denied, err := client.ValidateAccess("account_1@globular.io", rbacpb.SubjectType_ACCOUNT, "read", resourcePath)
    if err != nil {
        panic(err)
    }
    fmt.Println("account_1 read allowed:", allowed, "denied:", denied)
}
```

---

## Running Tests

The RBAC service includes a comprehensive test suite in [`rbac_test.go`](./rbac_test.go).  
Run it with:

```bash
go test ./rbac/rbac_client -v
```

Tests include:
- Resource permission CRUD
- Access validation (allow, deny, owner)
- Idempotency of deletes
- Parent-child inheritance
- Sharing and unsharing resources
- Default deny when no rules

---

## Example Scenarios

1. **Default Deny**  
   If no rules are defined on a resource, all access is denied by default.

2. **Parent Inheritance**  
   If `/home/alice` has READ allowed for `alice@globular.io`, then `/home/alice/file.txt` inherits that READ unless overridden.

3. **Deny Overrides Allow**  
   Explicit denies always override allows.

4. **Ownership**  
   Owners always retain rights, even after DeleteAllAccess, unless ownership is removed.

---

## Integration in Globular

- Managed as a Globular microservice
- Service ID: `rbac.RbacService`
- Used by **Authentication**, **Resource**, **File**, and other services for access control
- Configurable with Globular’s configuration system
- Supports TLS and RBAC-driven isolation

---

## License

Apache 2.0
