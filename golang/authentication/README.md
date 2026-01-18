# Authentication Service

<p align="center">
  <img src="../logo.png" alt="Globular Logo" width="100"/>
</p>

The Authentication Service is the cornerstone of Globular's security infrastructure, providing user identity management, credential verification, and token-based authentication for all platform services.

## Overview

Every authenticated operation in Globular flows through this service. It manages user credentials, generates and validates JWT tokens, and provides the foundation for role-based access control.

## Features

- **Token-Based Authentication** - JWT tokens for stateless authentication
- **Password Management** - Secure credential storage with bcrypt hashing
- **Root Account Management** - Special administrative account handling
- **Peer Device Tokens** - MAC address-based device authentication
- **Token Refresh** - Automatic token renewal before expiration

## Architecture

```
┌──────────────────────────────────────────────────────────────┐
│                   Authentication Service                     │
├──────────────────────────────────────────────────────────────┤
│                                                              │
│   ┌─────────────────┐      ┌──────────────────────────────┐ │
│   │   Credentials   │      │       Token Manager          │ │
│   │     Store       │      │                              │ │
│   │                 │      │  • JWT Generation            │ │
│   │  • bcrypt hash  │      │  • Validation                │ │
│   │  • salt mgmt    │      │  • Refresh                   │ │
│   └─────────────────┘      │  • Expiration tracking       │ │
│                            └──────────────────────────────┘ │
│                                                              │
│   ┌─────────────────────────────────────────────────────┐   │
│   │              Peer Device Manager                     │   │
│   │                                                      │   │
│   │  • MAC address registration                          │   │
│   │  • Device token generation                           │   │
│   │  • Multi-device support                              │   │
│   └─────────────────────────────────────────────────────┘   │
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

## API Reference

### Core Methods

| Method | Description | Request | Response |
|--------|-------------|---------|----------|
| `Authenticate` | Validate credentials and get token | `name`, `password` | `token` |
| `ValidateToken` | Verify token and get client info | `token` | `clientId`, `expires` |
| `RefreshToken` | Renew an existing token | `token` | `newToken` |

### Password Management

| Method | Description | Request | Response |
|--------|-------------|---------|----------|
| `SetPassword` | Change user password | `oldPassword`, `newPassword` | `success` |
| `SetRootPassword` | Change root account password | `oldPassword`, `newPassword` | `success` |
| `SetRootEmail` | Update root email address | `oldEmail`, `newEmail` | `success` |

### Device Management

| Method | Description | Request | Response |
|--------|-------------|---------|----------|
| `GeneratePeerToken` | Create token for peer device | `macAddress` | `token` |

## Authentication Flow

```
┌────────┐                         ┌──────────────────┐
│ Client │                         │  Authentication  │
└───┬────┘                         │     Service      │
    │                              └────────┬─────────┘
    │                                       │
    │  1. Authenticate(username, password)  │
    │──────────────────────────────────────▶│
    │                                       │
    │       ┌───────────────────────────────┤
    │       │ 2. Verify credentials         │
    │       │    - Load user record         │
    │       │    - bcrypt.Compare()         │
    │       └───────────────────────────────┤
    │                                       │
    │  3. JWT Token (signed)                │
    │◀──────────────────────────────────────│
    │                                       │
    │  4. API Request + Authorization: Bearer <token>
    │──────────────────────────────────────▶│ (Other Service)
    │                                       │
    │       ┌───────────────────────────────┤
    │       │ 5. ValidateToken(token)       │
    │       │    - Verify signature         │
    │       │    - Check expiration         │
    │       │    - Return clientId          │
    │       └───────────────────────────────┤
    │                                       │
    │  6. Response                          │
    │◀──────────────────────────────────────│
```

## Token Structure

Tokens contain:

| Field | Description |
|-------|-------------|
| `sub` | Subject (username) |
| `iss` | Issuer (service ID) |
| `exp` | Expiration timestamp |
| `iat` | Issued at timestamp |
| `domain` | User's domain |
| `state` | Session state |

## Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `AUTH_TOKEN_EXPIRY` | Token validity duration | `24h` |
| `AUTH_REFRESH_WINDOW` | Time before expiry to allow refresh | `1h` |
| `AUTH_SECRET_KEY` | JWT signing secret | Required |
| `AUTH_BCRYPT_COST` | Password hashing cost | `12` |

### Configuration File

```json
{
  "port": 10101,
  "tokenExpiry": "24h",
  "refreshWindow": "1h",
  "bcryptCost": 12,
  "allowedOrigins": ["*"]
}
```

## Usage Examples

### Go Client

```go
import (
    "context"
    auth "github.com/globulario/services/golang/authentication/authentication_client"
)

// Create client
client, err := auth.NewAuthenticationService_Client("localhost:10101", "auth.AuthenticationService")
if err != nil {
    log.Fatal(err)
}
defer client.Close()

// Authenticate
token, err := client.Authenticate("admin", "password123")
if err != nil {
    log.Fatal("Authentication failed:", err)
}

// Validate token
clientID, expires, err := client.ValidateToken(token)
if err != nil {
    log.Fatal("Token validation failed:", err)
}

fmt.Printf("Authenticated as: %s (expires: %v)\n", clientID, expires)
```

### Command Line

```bash
# Authenticate and get token
grpcurl -plaintext -d '{"name": "admin", "password": "secret"}' \
  localhost:10101 authentication.AuthenticationService/Authenticate

# Validate a token
grpcurl -plaintext -d '{"token": "<jwt-token>"}' \
  localhost:10101 authentication.AuthenticationService/ValidateToken
```

## Security Considerations

1. **Password Storage** - All passwords are hashed using bcrypt with configurable cost factor
2. **Token Security** - JWT tokens are signed with HMAC-SHA256
3. **Transport Security** - Always use TLS in production
4. **Token Expiration** - Tokens have limited lifetime to reduce exposure risk
5. **Root Account** - Special handling for administrative credentials

## Integration with Other Services

The Authentication Service integrates with:

- **RBAC Service** - For permission-based access control
- **Resource Service** - For user account management
- **LDAP Service** - For external directory authentication
- **All Services** - Every authenticated RPC validates tokens

## Dependencies

- [Resource Service](../resource/README.md) - Account storage
- [Storage Service](../storage/README.md) - Token persistence

---

[Back to Services Overview](../README.md)
