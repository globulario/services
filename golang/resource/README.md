# Resource Service

<p align="center">
  <img src="../logo.png" alt="Globular Logo" width="100"/>
</p>

The Resource Service manages packages, roles, accounts, groups, and organizations - the foundational entities for identity and access management.

## Overview

This service serves as the reference data layer for Globular, providing structured management of users, groups, roles, and package definitions that other services depend on.

## Features

- **Package Management** - Service and application descriptors
- **Account Management** - User accounts with profiles
- **Group Management** - Collections of accounts
- **Organization Management** - Business entities
- **Role Definitions** - Actions and permissions

## Core Entities

### Package Descriptor

Defines a deployable package (service or application):

```
PackageDescriptor
├── id: unique identifier
├── name: display name
├── description: package description
├── version: semantic version
├── type: SERVICE | APPLICATION
├── actions: available operations
├── roles: default roles
├── groups: default groups
└── dependencies: required packages
```

### Account

Represents a user in the system:

```
Account
├── id: unique identifier
├── name: username
├── email: email address
├── password: hashed password
├── roles: assigned roles
├── groups: group memberships
└── profile: user metadata
```

### Role

Defines a set of allowed actions:

```
Role
├── id: unique identifier
├── name: role name
├── description: role description
├── actions: permitted operations
└── members: accounts with this role
```

### Group

A collection of accounts:

```
Group
├── id: unique identifier
├── name: group name
├── description: group description
└── members: account list
```

### Organization

A business entity:

```
Organization
├── id: unique identifier
├── name: organization name
├── description: description
├── accounts: member accounts
└── groups: organization groups
```

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                      Resource Service                            │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │                   Entity Managers                          │ │
│  │                                                            │ │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐  │ │
│  │  │ Packages │  │ Accounts │  │  Roles   │  │  Groups  │  │ │
│  │  └──────────┘  └──────────┘  └──────────┘  └──────────┘  │ │
│  │                                                            │ │
│  │                       ┌──────────────┐                     │ │
│  │                       │Organizations │                     │ │
│  │                       └──────────────┘                     │ │
│  │                                                            │ │
│  └────────────────────────────────────────────────────────────┘ │
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │                   Reference Store                          │ │
│  │                                                            │ │
│  │   Accounts ──┬── Roles ──┬── Groups                       │ │
│  │              │           │                                 │ │
│  │              └───────────┴── Organizations                │ │
│  │                                                            │ │
│  └────────────────────────────────────────────────────────────┘ │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

## API Reference

### Package Operations

| Method | Description | Parameters |
|--------|-------------|------------|
| `CreatePackageDescriptor` | Register package | `descriptor` |
| `GetPackageDescriptor` | Get package info | `id` |
| `UpdatePackageDescriptor` | Update package | `descriptor` |
| `DeletePackageDescriptor` | Remove package | `id` |
| `ListPackageDescriptors` | List all packages | - |

### Account Operations

| Method | Description | Parameters |
|--------|-------------|------------|
| `CreateAccount` | Create user account | `account` |
| `GetAccount` | Get account by ID | `id` |
| `UpdateAccount` | Update account | `account` |
| `DeleteAccount` | Remove account | `id` |
| `ListAccounts` | List all accounts | - |

### Role Operations

| Method | Description | Parameters |
|--------|-------------|------------|
| `CreateRole` | Create role | `role` |
| `GetRole` | Get role | `id` |
| `UpdateRole` | Update role | `role` |
| `DeleteRole` | Remove role | `id` |
| `ListRoles` | List all roles | - |
| `AddRoleAction` | Add action to role | `roleId`, `action` |
| `RemoveRoleAction` | Remove action | `roleId`, `action` |

### Group Operations

| Method | Description | Parameters |
|--------|-------------|------------|
| `CreateGroup` | Create group | `group` |
| `GetGroup` | Get group | `id` |
| `UpdateGroup` | Update group | `group` |
| `DeleteGroup` | Remove group | `id` |
| `AddGroupMember` | Add account to group | `groupId`, `accountId` |
| `RemoveGroupMember` | Remove from group | `groupId`, `accountId` |

## Usage Examples

### Go Client

```go
import (
    resource "github.com/globulario/services/golang/resource/resource_client"
)

client, _ := resource.NewResourceService_Client("localhost:10105", "resource.ResourceService")
defer client.Close()

// Create account
account := &resourcepb.Account{
    Id:       "user-123",
    Name:     "johndoe",
    Email:    "john@example.com",
    Roles:    []string{"user"},
    Groups:   []string{"engineering"},
}
err := client.CreateAccount(account)

// Create role
role := &resourcepb.Role{
    Id:          "editor",
    Name:        "Editor",
    Description: "Can edit content",
    Actions:     []string{"read", "write", "delete"},
}
err = client.CreateRole(role)

// Create group
group := &resourcepb.Group{
    Id:          "engineering",
    Name:        "Engineering Team",
    Description: "Software engineers",
}
err = client.CreateGroup(group)

// Add account to group
err = client.AddGroupMember("engineering", "user-123")

// Add role to account
err = client.AddAccountRole("user-123", "editor")

// Get account with roles
account, err = client.GetAccount("user-123")
fmt.Printf("User %s has roles: %v\n", account.Name, account.Roles)
```

## Configuration

### Configuration File

```json
{
  "port": 10105,
  "defaultRoles": ["guest", "user", "admin"],
  "defaultGroups": ["users"],
  "passwordMinLength": 8,
  "passwordRequireSpecial": true
}
```

## Integration

Used by:
- [Authentication Service](../authentication/README.md) - Account validation
- [RBAC Service](../rbac/README.md) - Role definitions
- [Discovery Service](../discovery/README.md) - Package descriptors

---

[Back to Services Overview](../README.md)
