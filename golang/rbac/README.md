# RBAC Service

<p align="center">
  <img src="../logo.png" alt="Globular Logo" width="100"/>
</p>

The RBAC (Role-Based Access Control) Service provides fine-grained permission management for resources across the Globular platform.

## Overview

This service manages permissions on resources, controlling who can access what. It supports multiple subject types (accounts, groups, organizations, applications) and multiple resource types (files, databases, applications).

## Features

- **Resource Permissions** - Owner, Allowed, Denied lists
- **Multiple Subject Types** - Accounts, groups, organizations, applications
- **Multiple Resource Types** - Files, databases, applications
- **Hierarchical Permissions** - Inherit from groups/organizations
- **Query by Subject** - Find all permissions for a user

## Permission Model

```
Permission = (Subject, Resource, Access Level)

Subject Types:
  - Account (individual user)
  - Group (collection of accounts)
  - Organization (business entity)
  - Application (service identity)
  - Node Identity (machine identity)

Access Levels:
  - Owner (full control)
  - Allowed (specific permissions)
  - Denied (explicit denial)
```

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        RBAC Service                              │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │                  Permission Store                          │ │
│  │                                                            │ │
│  │   Resource Path ──▶ Permission Set                        │ │
│  │                     ├── Owners: [accounts]                │ │
│  │                     ├── Allowed: [(subject, actions)]     │ │
│  │                     └── Denied: [(subject, actions)]      │ │
│  │                                                            │ │
│  └────────────────────────────────────────────────────────────┘ │
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │                   Access Evaluator                         │ │
│  │                                                            │ │
│  │   (Subject, Resource, Action) ──▶ Allow/Deny              │ │
│  │                                                            │ │
│  │   1. Check explicit Denied ──▶ DENY                       │ │
│  │   2. Check Owner ──▶ ALLOW                                │ │
│  │   3. Check Allowed ──▶ ALLOW                              │ │
│  │   4. Check group/org membership ──▶ recurse               │ │
│  │   5. Default ──▶ DENY                                     │ │
│  │                                                            │ │
│  └────────────────────────────────────────────────────────────┘ │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

## API Reference

### Permission Operations

| Method | Description | Parameters |
|--------|-------------|------------|
| `GetResourcePermissions` | Get permissions for resource | `resourcePath` |
| `GetResourcePermissionsBySubject` | Get permissions by user/group | `subjectType`, `subjectId` |
| `GetResourcePermissionsByResourceType` | Get by resource type | `resourceType` |
| `SetResourcePermissions` | Set permissions | `resourcePath`, `permissions` |
| `DeleteResourcePermissions` | Remove all permissions | `resourcePath` |
| `DeleteResourcePermission` | Remove specific permission | `resourcePath`, `subject` |

### Permission Structure

```protobuf
message ResourcePermissions {
    string path = 1;              // Resource path
    string resourceType = 2;      // file, database, application
    repeated string owners = 3;   // Full control
    repeated Permission allowed = 4;
    repeated Permission denied = 5;
}

message Permission {
    string subjectType = 1;       // account, group, organization
    string subject = 2;           // Subject ID
    repeated string actions = 3;  // read, write, delete, etc.
}
```

## Usage Examples

### Go Client

```go
import (
    rbac "github.com/globulario/services/golang/rbac/rbac_client"
)

client, _ := rbac.NewRbacService_Client("localhost:10104", "rbac.RbacService")
defer client.Close()

// Set file permissions
permissions := &rbacpb.ResourcePermissions{
    Path:         "/data/reports/sales.xlsx",
    ResourceType: "file",
    Owners:       []string{"admin@example.com"},
    Allowed: []*rbacpb.Permission{
        {
            SubjectType: "group",
            Subject:     "sales-team",
            Actions:     []string{"read", "write"},
        },
        {
            SubjectType: "account",
            Subject:     "analyst@example.com",
            Actions:     []string{"read"},
        },
    },
    Denied: []*rbacpb.Permission{
        {
            SubjectType: "account",
            Subject:     "intern@example.com",
            Actions:     []string{"read", "write", "delete"},
        },
    },
}
err := client.SetResourcePermissions(permissions)

// Get permissions for a resource
perms, err := client.GetResourcePermissions("/data/reports/sales.xlsx")
fmt.Printf("Owners: %v\n", perms.Owners)
for _, p := range perms.Allowed {
    fmt.Printf("Allowed: %s (%s) can %v\n", p.Subject, p.SubjectType, p.Actions)
}

// Get all permissions for a user
userPerms, err := client.GetResourcePermissionsBySubject("account", "analyst@example.com")
for _, p := range userPerms {
    fmt.Printf("Resource: %s, Actions: %v\n", p.Path, p.Allowed[0].Actions)
}

// Delete specific permission
err = client.DeleteResourcePermission(
    "/data/reports/sales.xlsx",
    "account",
    "analyst@example.com",
)
```

### Access Check Pattern

```go
func canAccess(client rbac.Client, user, resource, action string) bool {
    perms, err := client.GetResourcePermissions(resource)
    if err != nil {
        return false
    }

    // Check if denied
    for _, denied := range perms.Denied {
        if denied.Subject == user && contains(denied.Actions, action) {
            return false
        }
    }

    // Check if owner
    if contains(perms.Owners, user) {
        return true
    }

    // Check if allowed
    for _, allowed := range perms.Allowed {
        if allowed.Subject == user && contains(allowed.Actions, action) {
            return true
        }
    }

    return false // Default deny
}
```

## Configuration

```json
{
  "port": 10104,
  "defaultDeny": true,
  "inheritFromGroups": true,
  "inheritFromOrgs": true
}
```

## Integration

Used by:
- [File Service](../file/README.md) - File access control
- [Persistence Service](../persistence/README.md) - Database permissions
- All services for access control

## Dependencies

- [Resource Service](../resource/README.md) - Account/group lookups

---

[Back to Services Overview](../README.md)
