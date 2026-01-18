# LDAP Service

<p align="center">
  <img src="../logo.png" alt="Globular Logo" width="100"/>
</p>

The LDAP Service provides integration with LDAP directory servers for user and group synchronization.

## Overview

This service enables Globular to integrate with enterprise LDAP directories (Active Directory, OpenLDAP, etc.) for centralized user authentication and group management.

## Features

- **Directory Connection** - Connect to LDAP servers
- **User Authentication** - Validate credentials against LDAP
- **Search Queries** - Search directory with LDAP filters
- **Synchronization** - Sync users and groups to Globular
- **Periodic Refresh** - Auto-sync at configurable intervals

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        LDAP Service                              │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │                 Connection Manager                         │ │
│  │                                                            │ │
│  │  LDAP Server ◄──► Connection Pool ◄──► Bind/Unbind        │ │
│  │                                                            │ │
│  └────────────────────────────────────────────────────────────┘ │
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │                   Sync Manager                             │ │
│  │                                                            │ │
│  │  LDAP Users ──▶ Transform ──▶ Globular Accounts           │ │
│  │  LDAP Groups ──▶ Transform ──▶ Globular Groups            │ │
│  │                                                            │ │
│  └────────────────────────────────────────────────────────────┘ │
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │                  Query Executor                            │ │
│  │                                                            │ │
│  │  Search Filter ──▶ Execute ──▶ Parse Entries              │ │
│  │                                                            │ │
│  └────────────────────────────────────────────────────────────┘ │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

## API Reference

### Connection Management

| Method | Description | Parameters |
|--------|-------------|------------|
| `CreateConnection` | Configure LDAP connection | `id`, `host`, `port`, `baseDN`, `bindDN`, `password` |
| `DeleteConnection` | Remove connection | `id` |

### Directory Operations

| Method | Description | Parameters |
|--------|-------------|------------|
| `Search` | Search directory | `connectionId`, `filter`, `attributes[]` |
| `Authenticate` | Verify credentials | `connectionId`, `username`, `password` |

### Synchronization

| Method | Description | Parameters |
|--------|-------------|------------|
| `Synchronize` | Sync directory to Globular | `connectionId` |
| `SetLdapSyncInfo` | Configure sync settings | `connectionId`, `syncInfo` |
| `GetLdapSyncInfo` | Get sync configuration | `connectionId` |
| `DeleteLdapSyncInfo` | Remove sync config | `connectionId` |

## Usage Examples

### Go Client

```go
import (
    ldap "github.com/globulario/services/golang/ldap/ldap_client"
)

client, _ := ldap.NewLdapService_Client("localhost:10117", "ldap.LdapService")
defer client.Close()

// Create connection
err := client.CreateConnection(
    "corp-ad",                          // connection ID
    "ldap.company.com",                 // host
    389,                                // port
    "dc=company,dc=com",                // base DN
    "cn=admin,dc=company,dc=com",       // bind DN
    "password",                         // password
)

// Search for users
entries, err := client.Search(
    "corp-ad",
    "(&(objectClass=user)(department=Engineering))",
    []string{"cn", "mail", "memberOf"},
)
for _, entry := range entries {
    fmt.Printf("User: %s, Email: %s\n",
        entry.GetAttribute("cn"),
        entry.GetAttribute("mail"))
}

// Authenticate user
valid, err := client.Authenticate("corp-ad", "jdoe", "userpassword")
if valid {
    fmt.Println("Authentication successful")
}

// Configure sync
syncInfo := &ldappb.SyncInfo{
    UserFilter:    "(&(objectClass=user)(!(disabled=TRUE)))",
    GroupFilter:   "(objectClass=group)",
    RefreshInterval: 3600, // 1 hour
    UserMapping: &ldappb.AttributeMapping{
        Username: "sAMAccountName",
        Email:    "mail",
        Name:     "displayName",
    },
}
err = client.SetLdapSyncInfo("corp-ad", syncInfo)

// Run sync
err = client.Synchronize("corp-ad")
```

## LDAP Filter Examples

| Purpose | Filter |
|---------|--------|
| All users | `(objectClass=user)` |
| Active users | `(&(objectClass=user)(!(userAccountControl:1.2.840.113556.1.4.803:=2)))` |
| Users in group | `(&(objectClass=user)(memberOf=cn=engineers,ou=groups,dc=company,dc=com))` |
| All groups | `(objectClass=group)` |
| By email | `(mail=user@example.com)` |

## Configuration

```json
{
  "port": 10117,
  "defaultTimeout": "30s",
  "connections": [
    {
      "id": "corp-ad",
      "host": "ldap.company.com",
      "port": 389,
      "baseDN": "dc=company,dc=com",
      "useTLS": true
    }
  ]
}
```

## Dependencies

- [Resource Service](../resource/README.md) - Account/group storage

---

[Back to Services Overview](../README.md)
