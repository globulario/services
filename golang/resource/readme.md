# Resource Service

The **Resource Service** is a core Globular microservice that manages **accounts, groups, organizations, roles, peers, applications, and sessions**.  
It acts as the authoritative registry for all entities in a Globular domain and provides APIs to manage identity, relationships, and membership.

---

## Features

- **Accounts**: Create, delete, update user accounts
- **Organizations**: Manage organizations and their groups, roles, and accounts
- **Groups**: Add/remove members, manage group ownership and roles
- **Roles**: Create roles, assign/remove actions, associate accounts and organizations
- **Applications**: Register applications, manage versions, actions, and ownership
- **Peers**: Manage distributed peers in a cluster, with approval workflows
- **Calls**: Track call history and sessions
- **Sessions**: Manage account login sessions, state, and expiration
- Full integration with **RBAC Service** for access control
- Event publishing on changes (`create`, `update`, `delete`)

---

## Core Concepts

- **Account**: Represents an individual user in the system.
- **Organization**: Logical grouping of accounts, groups, and roles.
- **Group**: Collection of accounts inside an organization.
- **Role**: Defines actions (permissions) that can be granted to accounts or organizations.
- **Application**: Software entity registered in the domain with its own access control.
- **Peer**: A remote Globular node in a distributed system.
- **Session**: Active authenticated presence of an account.

---

## gRPC API

The gRPC API is defined in [`resource.proto`](./resource.proto).  

### Accounts
- `RegisterAccount`, `DeleteAccount`
- `GetAccounts`, `UpdateAccount`
- `AddAccountRole`, `RemoveAccountRole`

### Organizations
- `CreateOrganization`, `DeleteOrganization`
- `AddOrganizationAccount`, `RemoveOrganizationAccount`
- `AddOrganizationRole`, `RemoveOrganizationRole`
- `AddOrganizationGroup`, `RemoveOrganizationGroup`

### Groups
- `CreateGroup`, `DeleteGroup`
- `AddGroupMemberAccount`, `RemoveGroupMemberAccount`

### Roles
- `CreateRole`, `DeleteRole`
- `AddRoleActions`, `RemoveRoleAction`
- `GetRoles`

### Applications
- `CreateApplication`, `DeleteApplication`, `UpdateApplication`
- `AddApplicationActions`, `RemoveApplicationAction`
- `GetApplications`

### Peers
- `RegisterPeer`, `DeletePeer`, `AcceptPeer`, `RejectPeer`
- `GetPeers`, `GetPeerPublicKey`, `GetPeerApprovalState`

### Sessions
- `GetSession`, `GetSessions`
- `UpdateSession`, `RemoveSession`

### Calls
- `SetCall`, `GetCallHistory`, `DeleteCall`, `ClearCalls`

---

## Example Usage

### Go Client

```go
package main

import (
    "fmt"
    "time"
    resource_client "github.com/globulario/services/golang/resource/resource_client"
)

func main() {
    client, err := resource_client.NewResourceService_Client("localhost:10003", "resource.ResourceService")
    if err != nil {
        panic(err)
    }
    defer client.Close()
    client.SetTimeout(5 * time.Second)

    // Create account
    err = client.RegisterAccount("globular.io", "user1", "User One", "user1@example.com", "pass", "pass")
    if err != nil {
        panic(err)
    }
    fmt.Println("Account created: user1")

    // Create group and add account
    err = client.CreateGroup("sa_token", "devs", "Developers", "Dev group")
    if err != nil {
        panic(err)
    }
    err = client.AddGroupMemberAccount("sa_token", "devs", "user1@globular.io")
    if err != nil {
        panic(err)
    }
    fmt.Println("Account added to group")
}
```

---

## Running Tests

The Resource service includes a complete test suite in [`resource_test.go`](./resource_test.go).  

Run them with:

```bash
go test ./resource/resource_client -v
```

Covers:
- Account, group, organization lifecycle
- Role creation and action assignment
- Application management
- Peer registration and approval
- Call history and sessions

---

## Integration in Globular

- Managed as a Globular microservice
- Service ID: `resource.ResourceService`
- Works in conjunction with **RBAC Service** for permission validation
- Used by **Authentication Service** for account lookups
- Persists to `Persistence Service` (MongoDB, ScyllaDB, SQL)

---

## License

Apache 2.0
