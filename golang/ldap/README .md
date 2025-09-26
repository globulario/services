# Globular LDAP Service

A lightweight LDAP bridge for Globular:

- **gRPC API** to create LDAP connections, authenticate, and search.
- **LDAP facade** that exposes Globular identities as a classic LDAP tree:
  - `ou=people` (accounts), `ou=groups`, `ou=roles`, `ou=orgs`
  - Supports **Bind**, **Search**, **Add**, **Modify**, **Delete**
  - Runs on **LDAP** `:389` and **LDAPS** `:636` with your TLS certs.

---

## Contents

- [Architecture](#architecture)
- [Build & Run](#build--run)
- [gRPC API](#grpc-api)
  - [Client Quickstart (Go)](#client-quickstart-go)
  - [API Summary](#api-summary)
- [LDAP Facade](#ldap-facade)
  - [Tree layout & objectClasses](#tree-layout--objectclasses)
  - [Bind rules](#bind-rules)
  - [LDAPS](#ldaps)
  - [LDAP Examples](#ldap-examples)
- [Testing](#testing)
- [Notes](#notes)

---

## Architecture

This service provides two complementary surfaces:

- **Server (gRPC)** — Implements operations to define connections to external LDAP servers (or the facade itself), authenticate users, and run searches. See `ldap.proto` / `ldap.go` for message and server definitions.

- **Client (Go)** — Thin wrapper around the gRPC stubs with helpers: `CreateConnection`, `DeleteConnection`, `Authenticate`, `Search`, etc. See `ldap_client.go`.

- **LDAP Facade** — Projects Globular resources as an LDAP directory with handlers for `Bind`, `Search`, `Add`, `Modify`, `Delete`. See `ldap_facade.go`.

---

## Build & Run

1. **Build** as part of your Globular deployment (make sure your `main.go` wires in the LDAP service server).
2. **Certificates**  
   Place your server keypair where the service is configured to read them (e.g., `s.CertFile`, `s.KeyFile`). The facade will start:
   - **LDAP** on `:389`
   - **LDAPS** on `:636` (TLS-wrapped listener)
3. **Config**  
   The facade computes `baseDN` from your domain (e.g., `globular.io` → `dc=globular,dc=io`). Listeners default to `:389` and `:636` and can be overridden via server fields.

---

## gRPC API

### Client Quickstart (Go)

```go
import (
    "fmt"
    client "github.com/globulario/services/golang/ldap/ldap_client"
)

func example() error {
    // Connect to service (address & ID depend on your env)
    c, err := client.NewLdapService_Client("globule-ryzen.globular.io", "ldap.LdapService")
    if err != nil { return err }
    defer c.Close()

    // Define a logical connection
    if err := c.CreateConnection("my_ldap", "bindUser", "bindPass", "ldap.my.org", 636); err != nil {
        return err
    }
    defer c.DeleteConnection("my_ldap")

    // Authenticate a user
    if err := c.Authenticate("my_ldap", "alice@my.org", "S3cret!"); err != nil {
        return err
    }

    // Search (rows as JSON [][]any)
    rows, err := c.Search("my_ldap",
        "dc=my,dc=org",
        "(&(objectClass=person)(uid=alice))",
        []string{"uid","cn","mail"})
    if err != nil { return err }

    fmt.Println("Rows:", rows)
    return nil
}
```

### API Summary

- **CreateConnection(id, login, password, host, port)** — Register a named connection used by subsequent calls.
- **DeleteConnection(id)** — Remove the named connection.
- **Authenticate(id, login, password)** — Bind against the connection.
- **Search(id, baseDN, filter, attrs[])** — Execute an LDAP search and return JSON-encoded rows.

> See `ldap.proto` / `ldap.go` for message details and server implementation.

---

## LDAP Facade

### Tree layout & objectClasses

The facade exposes a virtual directory rooted at your domain (`toBaseDN(domain)`), advertising these OUs:

```
dc=<part1>,dc=<part2>,...
 ├─ ou=people  (accounts: objectClass=inetOrgPerson,organizationalPerson,person,top)
 ├─ ou=groups  (objectClass=groupOfNames,top)           # member: uid=...,ou=people,...
 ├─ ou=roles   (objectClass=globularRole,top)           # globularAction; member: users
 └─ ou=orgs    (objectClass=organization,top)           # member: users/groups, uniqueMember: roles
```

- **people** — `uid=<account>,ou=people,<baseDN>`; attributes: `uid`, `cn`, `sn`, `mail`.
- **groups** — `cn=<group>,ou=groups,<baseDN>`; emits `member` (user DNs) when known.
- **roles** — `cn=<role>,ou=roles,<baseDN>`; emits `globularAction` and `member` (users).
- **orgs** — `o=<org>,ou=orgs,<baseDN>`; emits `member` (users/groups) and `uniqueMember` (roles).

### Bind rules

- **Anonymous bind** allowed (empty DN & password).
- Admin-style binds are accepted when DN begins with **`cn=admin,`**, **`cn=sa,`**, or **`uid=sa,`**. These authenticate via Globular’s Authentication service and cache a token per client connection.

### LDAPS

The LDAPS listener uses a normal TCP listener wrapped with `tls.NewListener` and your configured certificate/key. The same route mux handles both `:389` and `:636`.

### LDAP Examples (Go)

> Assume `baseDN = dc=globular,dc=io` and admin bind `cn=sa,dc=globular,dc=io`/.

```go
import (
  "crypto/tls"
  "github.com/go-ldap/ldap/v3"
)

cfg := &tls.Config{InsecureSkipVerify: true} // or set proper RootCAs/ServerName
l, _ := ldap.DialTLS("tcp", "globule-ryzen.globular.io:636", cfg)
defer l.Close()
_ = l.Bind("cn=sa,dc=globular,dc=io", "adminadmin")

// 1) Create a user
userDN := "uid=jdoe,ou=people,dc=globular,dc=io"
add := ldap.NewAddRequest(userDN, nil)
add.Attribute("objectClass", []string{"inetOrgPerson","organizationalPerson","person","top"})
add.Attribute("uid",  []string{"jdoe"})
add.Attribute("cn",   []string{"John Doe"})
add.Attribute("sn",   []string{"Doe"})
add.Attribute("mail", []string{"jdoe@example.com"})
add.Attribute("userPassword", []string{"s3cr3t"})
_ = l.Add(add)

// 2) Add the user to a group
grpDN := "cn=devs,ou=groups,dc=globular,dc=io"
mod := ldap.NewModifyRequest(grpDN, nil)
mod.Add("member", []string{userDN})
_ = l.Modify(mod)

// 3) Create a role and add action + user
roleDN := "cn=blog-editor,ou=roles,dc=globular,dc=io"
addR := ldap.NewAddRequest(roleDN, nil)
addR.Attribute("objectClass", []string{"top","globularRole"})
addR.Attribute("cn", []string{"blog-editor"})
addR.Attribute("globularAction", []string{"blog.post.create"})
_ = l.Add(addR)
modR := ldap.NewModifyRequest(roleDN, nil)
modR.Add("globularAction", []string{"blog.post.publish"})
modR.Add("member", []string{userDN})
_ = l.Modify(modR)

// 4) Create an org with initial members (user+group+role)
orgDN := "o=engineering,ou=orgs,dc=globular,dc=io"
addO := ldap.NewAddRequest(orgDN, nil)
addO.Attribute("objectClass", []string{"organization","top"})
addO.Attribute("o", []string{"engineering"})
addO.Attribute("member", []string{userDN, grpDN})
addO.Attribute("uniqueMember", []string{roleDN})
_ = l.Add(addO)

// 5) Search with scope and filter
sr, _ := l.Search(ldap.NewSearchRequest(
  "ou=orgs,dc=globular,dc=io",
  ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
  "(&(o=engineering)(objectClass=organization))",
  []string{"dn","member","uniqueMember"},
  nil,
))
for _, e := range sr.Entries {
  fmt.Println(e.DN, e.GetAttributeValues("member"), e.GetAttributeValues("uniqueMember"))
}
```

---

## Testing

There are table-driven tests for:
- **LDAPS bind** using the same TLS config as the gRPC client.
- **Group CRUD** and membership (create, search, add/remove user, delete).
- **Role CRUD** and actions + membership.
- **Org membership** across users, groups, and roles.
- **Search behavior** (scope, filters, and emitted `member`/`uniqueMember`).

Run a specific test:
```bash
go test -run ^TestLDAP_TLS_Bind$ ./ldap/ldap_client
```

Run the whole suite:
```bash
go test ./ldap/ldap_client -v
```

---

## Notes

- The facade emits `member`/`uniqueMember` attributes when it can discover them from the resource layer. If a particular getter isn’t available in your build, those attributes may be absent (other attributes still return).
- Simple equality filters are supported for common attributes: `(cn=...)`, `(uid=...)`, `(o=...)`, and `(&(a=b)(c=d))` conjunctions.
- Scopes: `BaseObject`, `SingleLevel`, and `WholeSubtree` are honored.
- Admin-like DNs (`cn=admin,...`, `cn=sa,...`, `uid=sa,...`) authenticate via the Authentication service; the facade caches a token per connection.
