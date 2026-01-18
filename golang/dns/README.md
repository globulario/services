# DNS Service

<p align="center">
  <img src="../logo.png" alt="Globular Logo" width="100"/>
</p>

The DNS Service provides DNS record management capabilities, allowing Globular to manage domain name resolution for cluster services.

## Overview

This service manages DNS records including A, AAAA, CNAME, MX, TXT, and other record types. It enables dynamic DNS updates for service discovery and load balancing.

## Features

- **Multiple Record Types** - A, AAAA, CNAME, MX, SOA, NS, TXT, AFSDB, URI, CAA
- **TTL Support** - Configurable time-to-live for all records
- **Bulk Operations** - Manage multiple domains simultaneously
- **Dynamic Updates** - Real-time DNS record changes

## Supported Record Types

| Type | Description | Example |
|------|-------------|---------|
| A | IPv4 address | `192.168.1.10` |
| AAAA | IPv6 address | `2001:db8::1` |
| CNAME | Canonical name (alias) | `www → example.com` |
| MX | Mail exchange | `mail.example.com (priority: 10)` |
| TXT | Text record | `v=spf1 include:_spf.google.com ~all` |
| SOA | Start of authority | Primary NS, admin email |
| NS | Name server | `ns1.example.com` |
| CAA | Certificate authority | `0 issue "letsencrypt.org"` |
| URI | URI record | Service endpoints |
| AFSDB | AFS database | AFS cell database servers |

## API Reference

### A Records (IPv4)

| Method | Description | Parameters |
|--------|-------------|------------|
| `SetA` | Create/update A record | `domain`, `ip`, `ttl` |
| `GetA` | Retrieve A record | `domain` |
| `RemoveA` | Delete A record | `domain`, `ip` |

### AAAA Records (IPv6)

| Method | Description | Parameters |
|--------|-------------|------------|
| `SetAAAA` | Create/update AAAA record | `domain`, `ip`, `ttl` |
| `GetAAAA` | Retrieve AAAA record | `domain` |
| `RemoveAAAA` | Delete AAAA record | `domain`, `ip` |

### CNAME Records

| Method | Description | Parameters |
|--------|-------------|------------|
| `SetCName` | Create/update CNAME | `name`, `target`, `ttl` |
| `GetCName` | Retrieve CNAME | `name` |
| `RemoveCName` | Delete CNAME | `name` |

### MX Records

| Method | Description | Parameters |
|--------|-------------|------------|
| `SetMx` | Create/update MX record | `domain`, `host`, `preference`, `ttl` |
| `GetMx` | Retrieve MX records | `domain` |
| `RemoveMx` | Delete MX record | `domain`, `host` |

### Domain Management

| Method | Description | Parameters |
|--------|-------------|------------|
| `SetDomains` | Bulk domain configuration | `domains[]` |
| `GetDomains` | List all domains | - |

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         DNS Service                              │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │                    Record Manager                          │ │
│  │                                                            │ │
│  │   ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐ │ │
│  │   │    A     │  │   AAAA   │  │  CNAME   │  │    MX    │ │ │
│  │   │ Records  │  │ Records  │  │ Records  │  │ Records  │ │ │
│  │   └──────────┘  └──────────┘  └──────────┘  └──────────┘ │ │
│  │                                                            │ │
│  │   ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐ │ │
│  │   │   TXT    │  │   SOA    │  │    NS    │  │   CAA    │ │ │
│  │   │ Records  │  │ Records  │  │ Records  │  │ Records  │ │ │
│  │   └──────────┘  └──────────┘  └──────────┘  └──────────┘ │ │
│  │                                                            │ │
│  └────────────────────────────────────────────────────────────┘ │
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │                    Zone Storage                            │ │
│  │                                                            │ │
│  │   example.com                                              │ │
│  │   ├── A:     192.168.1.10 (TTL: 300)                      │ │
│  │   ├── MX:    mail.example.com (pref: 10)                  │ │
│  │   └── TXT:   v=spf1 ...                                   │ │
│  │                                                            │ │
│  └────────────────────────────────────────────────────────────┘ │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

## Usage Examples

### Go Client

```go
import (
    dns "github.com/globulario/services/golang/dns/dns_client"
)

client, _ := dns.NewDnsService_Client("localhost:10107", "dns.DnsService")
defer client.Close()

// Set A record
err := client.SetA("api.example.com", "192.168.1.10", 300)
if err != nil {
    log.Fatal(err)
}

// Set MX record
err = client.SetMx("example.com", "mail.example.com", 10, 3600)

// Get A record
ip, ttl, err := client.GetA("api.example.com")
fmt.Printf("A record: %s (TTL: %d)\n", ip, ttl)

// Set CNAME
err = client.SetCName("www.example.com", "example.com", 3600)
```

### Command Line

```bash
# Set A record
grpcurl -plaintext -d '{
  "domain": "api.example.com",
  "ip": "192.168.1.10",
  "ttl": 300
}' localhost:10107 dns.DnsService/SetA

# Get A record
grpcurl -plaintext -d '{"domain": "api.example.com"}' \
  localhost:10107 dns.DnsService/GetA

# Set MX record
grpcurl -plaintext -d '{
  "domain": "example.com",
  "host": "mail.example.com",
  "preference": 10,
  "ttl": 3600
}' localhost:10107 dns.DnsService/SetMx
```

## Service Discovery Pattern

```
┌─────────────────────┐      ┌──────────────────┐
│  Service Registrar  │      │   DNS Service    │
└─────────┬───────────┘      └────────┬─────────┘
          │                           │
          │ 1. Register service       │
          │   (api.cluster.local)     │
          │──────────────────────────▶│
          │                           │
          │                           │ SetA("api.cluster.local",
          │                           │       "192.168.1.10")
          │                           │
          │                           │
┌─────────┴───────────┐               │
│      Client         │               │
└─────────┬───────────┘               │
          │                           │
          │ 2. DNS Lookup             │
          │   api.cluster.local       │
          │──────────────────────────▶│
          │                           │
          │ 3. 192.168.1.10          │
          │◀──────────────────────────│
          │                           │
          │ 4. Connect to service     │
          │──────────────────────────▶│ (Service at 192.168.1.10)
```

## Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `DNS_DEFAULT_TTL` | Default TTL for records | `300` |
| `DNS_ZONE_FILE` | Zone file path | `/var/lib/globular/dns/zones` |

### Configuration File

```json
{
  "port": 10107,
  "defaultTTL": 300,
  "zoneFile": "/var/lib/globular/dns/zones",
  "upstreamDNS": ["8.8.8.8", "8.8.4.4"]
}
```

## Integration with Cluster

When nodes join the cluster, DNS records are automatically created:

```
┌─────────────────────┐
│ Cluster Controller  │
└─────────┬───────────┘
          │
          │ Node Joined: host1 (192.168.1.10)
          │
          ▼
┌─────────────────────┐
│    DNS Service      │
└─────────┬───────────┘
          │
          │ SetA("host1.cluster.local", "192.168.1.10")
          │ SetA("etcd.cluster.local", "192.168.1.10")
          │ SetA("minio.cluster.local", "192.168.1.10")
          │
          ▼
     DNS Zone Updated
```

## Dependencies

- [Storage Service](../storage/README.md) - Zone data persistence

---

[Back to Services Overview](../README.md)
