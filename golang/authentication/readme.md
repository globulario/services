# Authentication Service

The **Authentication Service** is a Globular microservice that handles **user authentication, JWT token lifecycle, and root credential management**. It integrates tightly with the Resource and RBAC services to manage sessions and enforce permissions.

---

## Features

- **Authenticate accounts** (root `sa` or user accounts).
- **Issue JWTs** with `issuer`, `userId`, `email`, and `domain`.
- **Validate & refresh tokens** (refresh allowed up to 7 days past expiry).
- **Manage passwords**
  - `SetPassword` – self-service or root (`sa@domain`).
  - `SetRootPassword` – secure root password rotation (safe no-op if unchanged).
- **Manage root email** via `SetRootEmail`.
- **Generate peer tokens** for inter-node communication (by MAC).
- **RBAC integration** with curated roles:
  - Password Self-Service
  - Peer Token Issuer
  - Root Credential Manager
  - Authentication Admin

---

## Service overview

- **Protocol:** gRPC (`authentication.AuthenticationService`)
- **Default ports:** service `:10000`, proxy `:10001`
- **Dependencies:** Resource, RBAC (and optional LDAP)
- **Runtime:** maintains user sessions and background janitor to expire sessions

---

## API

| Method            | Description                                                                 |
|-------------------|-----------------------------------------------------------------------------|
| `Authenticate`    | Login with user/password → returns JWT                                       |
| `ValidateToken`   | Verify a token and return clientId + expiry                                 |
| `RefreshToken`    | Renew token if valid or expired <7 days                                     |
| `SetPassword`     | Change own password (or root changes any)                                   |
| `SetRootPassword` | Rotate root (`sa`) password (with no-op short-circuit)                      |
| `SetRootEmail`    | Update root administrator email                                             |
| `GeneratePeerToken` | Issue token for a peer node identified by MAC                             |

---

## Usage

### Running the service

```bash
authentication_service --describe   # print service description as JSON
authentication_service --health     # check service health
```

### Go client example

```go
package main

import (
  "fmt"
  authcli "github.com/globulario/services/golang/authentication/authentication_client"
)

func main() {
  // Connect to the service
  client, err := authcli.NewAuthenticationService_Client("globular.io", "authentication.AuthenticationService")
  if err != nil { panic(err) }
  defer client.Close()

  // Authenticate as root (sa)
  token, err := client.Authenticate("sa", "adminadmin")
  if err != nil { panic(err) }

  // Attach token to metadata
  _ = client.SetToken(token)

  // Validate token
  clientId, exp, err := client.ValidateToken(token)
  fmt.Println("clientId:", clientId, "exp:", exp)

  // Refresh token
  newTok, err := client.RefreshToken(token)
  if err == nil && newTok != "" {
    _ = client.SetToken(newTok)
  }
}
```

---

## Example: change password

```go
// After SetToken(...)
newToken, err := client.SetPassword("alice", "oldSecret!", "newSecret!")
if err != nil {
  panic(err)
}
_ = client.SetToken(newToken) // use new token
```

---

## Example: rotate root password

```go
// Safe no-op if old == new
tok, err := client.SetRootPassword("adminadmin", "adminadmin")
if err != nil {
  panic(err)
}
fmt.Println("new sa token:", tok)
```

---

## Testing

The repo includes lifecycle tests:

```bash
cd authentication/authentication_client
GLOBULAR_DOMAIN=globular.io GLOBULAR_SA_USER=sa GLOBULAR_SA_PWD=adminadmin AUTH_TEST_ALLOW_ROTATE=false go test -v
```

Covers: authenticate → validate/refresh → root password no-op → (optional) rotation → logout.

---

## Security notes

- Tokens are signed JWTs, validated server-side.
- Refresh is limited to 7 days after expiry.
- Root ops restricted to `sa@<domain>`.
- No-op password changes short-circuit without side effects.

---

## License

Part of the **Globular** microservices suite.  
See repository license for details.
