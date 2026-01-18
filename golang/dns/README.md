# DNS Service

<p align="center">
  <img src="../logo.png" alt="Globular Logo" width="100"/>
</p>

The DNS Service provides a complete authoritative DNS server with record management capabilities, allowing Globular clusters to manage domain name resolution for services and applications.

## Overview

This service implements a full DNS server that listens on UDP port 53 for DNS queries while providing a gRPC API for managing DNS records. It supports all common record types and can be used to host your own authoritative nameservers for complete domain control.

## Features

- **Authoritative DNS Server** - UDP server on port 53
- **Multiple Record Types** - A, AAAA, CNAME, MX, SOA, NS, TXT, CAA, URI, AFSDB
- **gRPC Management API** - Programmatic record management
- **Persistent Storage** - BadgerDB-backed record storage
- **TTL Support** - Configurable time-to-live for all records
- **Cluster Integration** - Automatic service discovery records

---

## Cluster Integration (Automated Setup)

When running as part of a Globular cluster, DNS configuration is **largely automated** through the cluster controller and node agent.

### What's Automated

| Component | Automated | Manual |
|-----------|-----------|--------|
| Service deployment (`globular-dns.service`) | Yes | - |
| Domain registration (`SetDomains`) | Yes | - |
| Node A records (`<hostname>.<domain>`) | Yes | - |
| Gateway A records (`gateway.<domain>`) | Yes | - |
| SOA records | Yes (via plan) | - |
| NS records | Yes (via plan) | - |
| Glue records (A for NS) | Yes (via plan) | - |
| Linux port 53 setup | - | Yes |
| Registrar glue records | - | Yes |

### DNS Profile

Nodes with the following profiles run the DNS service:
- `core`
- `compute`
- `control-plane`
- `dns` (dedicated DNS nodes)

### Automated Configuration Flow

```
┌──────────────────────┐
│  Cluster Controller  │
│                      │
│  1. Compute node plan│
│  2. Render DNS config│
│     - SOA record     │
│     - NS records     │
│     - Glue records   │
└──────────┬───────────┘
           │
           ▼
┌──────────────────────┐
│    Node Agent        │
│                      │
│  3. Receive plan     │
│  4. Write dns_init.json
│  5. Start DNS service│
│  6. syncDNS()        │
│     - SetDomains     │
│     - Node A records │
│     - Apply init cfg │
└──────────────────────┘
```

### Configuration Files

The cluster controller renders:

**`/var/lib/globular/dns/dns_init.json`** - DNS initialization config:
```json
{
  "domain": "example.com",
  "soa": {
    "domain": "example.com",
    "ns": "ns1.example.com.",
    "mbox": "admin.example.com.",
    "serial": 2024011800,
    "refresh": 7200,
    "retry": 3600,
    "expire": 1209600,
    "minttl": 3600,
    "ttl": 3600
  },
  "ns_records": [
    {"ns": "ns1.example.com", "ttl": 3600},
    {"ns": "ns2.example.com", "ttl": 3600}
  ],
  "glue_records": [
    {"hostname": "ns1.example.com", "ip": "192.168.1.10", "ttl": 3600},
    {"hostname": "ns2.example.com", "ip": "192.168.1.11", "ttl": 3600}
  ],
  "is_primary": true
}
```

### CLI Commands for DNS Setup

Using `globularcli`, you can bootstrap a DNS-enabled cluster:

```bash
# Step 1: Create cluster with DNS domain (do this AFTER Linux setup)
globularcli cluster create \
  --domain yourdomain.com \
  --admin-email admin@yourdomain.com

# Step 2: Bootstrap first node with DNS profile
globularcli cluster bootstrap \
  --profiles core,dns

# Step 3: Join additional DNS nodes
globularcli cluster join \
  --controller <controller-address>:10010 \
  --profiles dns

# Step 4: Verify DNS records
globularcli dns list --domain yourdomain.com
```

### What You Still Need to Do Manually

1. **Linux OS Setup** (before cluster bootstrap):
   - Free port 53 (disable systemd-resolved stub)
   - Configure hostname
   - Open firewall ports

2. **Domain Registrar Setup** (after cluster is running):
   - Create glue records at registrar

### Related Documentation

- [Cluster Controller README](../clustercontroller/README.md) - DNS config rendering details
- [Node Agent README](../nodeagent/README.md) - DNS synchronization details
   - Point nameservers to your DNS nodes

See sections below for detailed manual setup instructions.

---

## Linux Setup Guide (Manual Prerequisites)

Running a DNS server on Linux requires some system configuration to free up port 53, which is typically used by `systemd-resolved`.

### Prerequisites

- Linux system (Ubuntu/Debian recommended)
- Root or sudo access
- Static public IP address (for authoritative DNS)
- Domain name with access to registrar settings

### Step 1: Set the Hostname

Configure your server's hostname to match your DNS server name:

```bash
# Set hostname (e.g., ns1)
sudo hostnamectl set-hostname ns1

# Or edit directly
sudo nano /etc/hostname
# Enter: ns1
```

Update `/etc/hosts` to include your hostname:

```bash
sudo nano /etc/hosts
```

Add:
```
127.0.0.1   localhost
127.0.1.1   ns1.yourdomain.com ns1
YOUR_PUBLIC_IP   ns1.yourdomain.com ns1
```

### Step 2: Free Up Port 53

By default, `systemd-resolved` uses port 53. You must disable its DNS stub listener:

```bash
# Edit systemd-resolved configuration
sudo nano /etc/systemd/resolved.conf
```

Update the file to:

```ini
[Resolve]
DNS=1.1.1.1 8.8.8.8
#FallbackDNS=
#Domains=
#LLMNR=no
#MulticastDNS=no
#DNSSEC=no
#DNSOverTLS=no
#Cache=no
DNSStubListener=no
#ReadEtcHosts=yes
```

Create a symbolic link for resolv.conf:

```bash
sudo ln -sf /run/systemd/resolve/resolv.conf /etc/resolv.conf
```

Restart systemd-resolved:

```bash
sudo systemctl restart systemd-resolved
```

Verify port 53 is free:

```bash
sudo ss -tulnp | grep :53
# Should show no output if port is free
```

### Step 3: Configure Firewall

Open port 53 for both UDP and TCP:

```bash
# UFW (Ubuntu)
sudo ufw allow 53/udp
sudo ufw allow 53/tcp

# Or iptables
sudo iptables -A INPUT -p udp --dport 53 -j ACCEPT
sudo iptables -A INPUT -p tcp --dport 53 -j ACCEPT
```

If behind a NAT/router, configure port forwarding:
- Forward external port 53 (UDP/TCP) to your server's internal IP on port 53

### Step 4: Reboot

```bash
sudo reboot
```

After reboot, verify the configuration:

```bash
# Check port 53 is free
sudo ss -tulnp | grep :53

# Verify DNS resolution still works
dig @1.1.1.1 google.com
```

---

## DNS Service Configuration

### Starting the DNS Service

The DNS service is typically started as part of a Globular node or can be run standalone:

```bash
# Run the DNS service
./dns.DnsService dns-1

# With debug logging
./dns.DnsService --debug dns-1

# With custom config path
./dns.DnsService dns-1 /etc/globular/dns/config.json
```

### Initial Configuration

After starting the service, configure the domains it will manage:

```go
import (
    dns "github.com/globulario/services/golang/dns/dns_client"
)

client, _ := dns.NewDnsService_Client("localhost:10033", "dns.DnsService")
defer client.Close()

// Set the domains this DNS server will be authoritative for
err := client.SetDomains("", []string{"yourdomain.com"})
```

Or via grpcurl:

```bash
grpcurl -plaintext -d '{
  "domains": ["yourdomain.com"]
}' localhost:10033 dns.DnsService/SetDomains
```

### Setting Up SOA Record

Every authoritative zone needs an SOA (Start of Authority) record:

```go
// Set SOA record for your domain
err := client.SetSoa(
    "",                        // token (empty for local)
    "yourdomain.com",          // domain
    "ns1.yourdomain.com.",     // primary nameserver
    "admin.yourdomain.com.",   // admin email (use . instead of @)
    2024011801,                // serial (YYYYMMDDNN format)
    7200,                      // refresh (2 hours)
    3600,                      // retry (1 hour)
    1209600,                   // expire (2 weeks)
    3600,                      // minimum TTL
    3600,                      // TTL
)
```

### Setting Up NS Records

Register your nameservers:

```go
// Set NS records
err := client.SetNs("", "yourdomain.com", "ns1.yourdomain.com", 3600)
err = client.SetNs("", "yourdomain.com", "ns2.yourdomain.com", 3600)
```

### Setting Up Glue Records (A Records for NS)

Your nameservers need A records (glue records):

```go
// Set A records for nameservers
_, err := client.SetA("", "ns1.yourdomain.com", "YOUR_NS1_PUBLIC_IP", 3600)
_, err = client.SetA("", "ns2.yourdomain.com", "YOUR_NS2_PUBLIC_IP", 3600)

// Set A record for main domain
_, err = client.SetA("", "yourdomain.com", "YOUR_WEBSERVER_IP", 300)

// Set A record for www subdomain
_, err = client.SetA("", "www.yourdomain.com", "YOUR_WEBSERVER_IP", 300)
```

---

## Domain Registrar Configuration

To use your own DNS servers, you must configure "custom nameservers" at your domain registrar.

### GoDaddy Setup

1. **Log into GoDaddy** and go to your domain's DNS Management

2. **Add Glue Records (Host Names)**:
   - Go to **My Domains** > Select your domain > **DNS** > **Nameservers** > **Change**
   - Click **Enter my own nameservers (advanced)**
   - Click **Add Nameserver**
   - For **Host**: enter `ns1` (without the domain)
   - For **IP Address**: enter your first DNS server's public IP
   - Repeat for `ns2` with your second DNS server's IP

3. **Set Custom Nameservers**:
   - In the nameservers section, replace the default nameservers with:
     - `ns1.yourdomain.com`
     - `ns2.yourdomain.com`
   - Save changes

4. **Wait for Propagation**: Changes can take 24-48 hours to propagate globally.

### HostGator Setup

1. **Log into HostGator** cPanel or client portal

2. **Register Nameservers**:
   - Go to **Domains** > **Register a Nameserver** (or use WHM)
   - Register `ns1.yourdomain.com` with IP `YOUR_NS1_IP`
   - Register `ns2.yourdomain.com` with IP `YOUR_NS2_IP`

3. **Update Domain Nameservers**:
   - Go to **Domains** > **Manage**
   - Select your domain
   - Update nameservers to:
     - `ns1.yourdomain.com`
     - `ns2.yourdomain.com`

### Namecheap Setup

1. **Log into Namecheap** > **Domain List** > **Manage**

2. **Add Personal DNS Servers**:
   - Scroll to **Personal DNS Servers**
   - Add `ns1` with your IP
   - Add `ns2` with your IP

3. **Set Custom DNS**:
   - In **Nameservers** section, select **Custom DNS**
   - Enter:
     - `ns1.yourdomain.com`
     - `ns2.yourdomain.com`

### Cloudflare (Transfer Out Required)

Cloudflare domains require their nameservers. To use custom nameservers:
1. Transfer your domain to another registrar
2. Or use Cloudflare as a secondary DNS

### Google Domains (Now Squarespace)

1. Go to **DNS** settings
2. Select **Custom name servers**
3. Add your nameservers: `ns1.yourdomain.com`, `ns2.yourdomain.com`
4. Create glue records in the **Custom records** section

---

## Testing Your DNS Server

### Local Testing

Test from your DNS server:

```bash
# Query your local DNS server
dig @localhost yourdomain.com

# Query specific record type
dig @localhost yourdomain.com A
dig @localhost yourdomain.com NS
dig @localhost yourdomain.com SOA
```

### Remote Testing

Test from another machine:

```bash
# Query using your server's IP
dig @YOUR_SERVER_IP yourdomain.com

# Test via public DNS (after propagation)
dig @8.8.8.8 yourdomain.com
```

### Check Propagation

Use online tools to verify global propagation:
- https://www.whatsmydns.net/
- https://dnschecker.org/
- https://mxtoolbox.com/

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         DNS Service                              │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │                  UDP DNS Server (Port 53)                  │ │
│  │                                                            │ │
│  │   DNS Query ──▶ Lookup Records ──▶ Build Response         │ │
│  │       │                                   │                │ │
│  │       ▼                                   ▼                │ │
│  │   A, AAAA, CNAME, MX, NS, SOA, TXT, CAA, URI, AFSDB       │ │
│  │                                                            │ │
│  └────────────────────────────────────────────────────────────┘ │
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │                  gRPC Management API                       │ │
│  │                                                            │ │
│  │   Set/Get/Remove for all record types                     │ │
│  │   Domain management (SetDomains, GetDomains)              │ │
│  │                                                            │ │
│  └────────────────────────────────────────────────────────────┘ │
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │                    BadgerDB Storage                        │ │
│  │                                                            │ │
│  │   Persistent storage for all DNS records and TTLs         │ │
│  │   Location: /var/lib/globular/data/dns/                   │ │
│  │                                                            │ │
│  └────────────────────────────────────────────────────────────┘ │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

## Supported Record Types

| Type | Description | Example |
|------|-------------|---------|
| A | IPv4 address | `192.168.1.10` |
| AAAA | IPv6 address | `2001:db8::1` |
| CNAME | Canonical name (alias) | `www -> example.com` |
| MX | Mail exchange | `mail.example.com (priority: 10)` |
| TXT | Text record | `v=spf1 include:_spf.google.com ~all` |
| SOA | Start of authority | Primary NS, admin email, timers |
| NS | Name server | `ns1.example.com` |
| CAA | Certificate authority | `0 issue "letsencrypt.org"` |
| URI | URI record | Service endpoints |
| AFSDB | AFS database | AFS cell database servers |

## API Reference

### Domain Management

| Method | Description | Parameters |
|--------|-------------|------------|
| `SetDomains` | Set managed domains | `domains[]` |
| `GetDomains` | List managed domains | - |

### A Records (IPv4)

| Method | Description | Parameters |
|--------|-------------|------------|
| `SetA` | Create/update A record | `domain`, `a`, `ttl` |
| `GetA` | Retrieve A records | `domain` |
| `RemoveA` | Delete A record | `domain`, `a` |

### AAAA Records (IPv6)

| Method | Description | Parameters |
|--------|-------------|------------|
| `SetAAAA` | Create/update AAAA record | `domain`, `aaaa`, `ttl` |
| `GetAAAA` | Retrieve AAAA records | `domain` |
| `RemoveAAAA` | Delete AAAA record | `domain`, `aaaa` |

### NS Records

| Method | Description | Parameters |
|--------|-------------|------------|
| `SetNs` | Create/update NS record | `id`, `ns`, `ttl` |
| `GetNs` | Retrieve NS records | `id` |
| `RemoveNs` | Delete NS record | `id`, `ns` |

### SOA Records

| Method | Description | Parameters |
|--------|-------------|------------|
| `SetSoa` | Create/update SOA record | `id`, `soa`, `ttl` |
| `GetSoa` | Retrieve SOA records | `id` |
| `RemoveSoa` | Delete SOA record | `id`, `ns` |

### MX Records

| Method | Description | Parameters |
|--------|-------------|------------|
| `SetMx` | Create/update MX record | `id`, `mx`, `ttl` |
| `GetMx` | Retrieve MX records | `id` |
| `RemoveMx` | Delete MX record | `id`, `mx` |

### CNAME Records

| Method | Description | Parameters |
|--------|-------------|------------|
| `SetCName` | Create/update CNAME | `id`, `cname`, `ttl` |
| `GetCName` | Retrieve CNAME | `id` |
| `RemoveCName` | Delete CNAME | `id` |

### TXT Records

| Method | Description | Parameters |
|--------|-------------|------------|
| `SetText` | Create/update TXT record | `id`, `values[]`, `ttl` |
| `GetText` | Retrieve TXT records | `id` |
| `RemoveText` | Delete TXT record | `id` |

### CAA Records

| Method | Description | Parameters |
|--------|-------------|------------|
| `SetCaa` | Create/update CAA record | `id`, `caa`, `ttl` |
| `GetCaa` | Retrieve CAA records | `id` |
| `RemoveCaa` | Delete CAA record | `id`, `domain` |

## Usage Examples

### Complete Domain Setup

```go
import (
    dns "github.com/globulario/services/golang/dns/dns_client"
)

func setupDomain() error {
    client, err := dns.NewDnsService_Client("localhost:10033", "dns.DnsService")
    if err != nil {
        return err
    }
    defer client.Close()

    domain := "example.com"
    ns1IP := "203.0.113.10"
    ns2IP := "203.0.113.11"
    webIP := "203.0.113.20"

    // 1. Register the domain
    if err := client.SetDomains("", []string{domain}); err != nil {
        return err
    }

    // 2. Set SOA record
    if err := client.SetSoa("", domain,
        "ns1."+domain+".",
        "admin."+domain+".",
        2024011801, 7200, 3600, 1209600, 3600, 3600); err != nil {
        return err
    }

    // 3. Set NS records
    client.SetNs("", domain, "ns1."+domain, 3600)
    client.SetNs("", domain, "ns2."+domain, 3600)

    // 4. Set glue records (A records for nameservers)
    client.SetA("", "ns1."+domain, ns1IP, 3600)
    client.SetA("", "ns2."+domain, ns2IP, 3600)

    // 5. Set main domain A record
    client.SetA("", domain, webIP, 300)

    // 6. Set www subdomain
    client.SetA("", "www."+domain, webIP, 300)

    // 7. Set MX record for email
    client.SetMx("", domain, 10, "mail."+domain, 3600)
    client.SetA("", "mail."+domain, webIP, 3600)

    // 8. Set SPF record
    client.SetText("", domain, []string{
        "v=spf1 mx a ip4:"+webIP+" ~all",
    }, 3600)

    // 9. Set CAA record (allow Let's Encrypt)
    client.SetCaa("", domain, 0, "issue", "letsencrypt.org", 3600)

    return nil
}
```

### Command Line Examples

```bash
# Set managed domains
grpcurl -plaintext -d '{"domains": ["example.com"]}' \
  localhost:10033 dns.DnsService/SetDomains

# Set A record
grpcurl -plaintext -d '{
  "domain": "api.example.com",
  "a": "192.168.1.10",
  "ttl": 300
}' localhost:10033 dns.DnsService/SetA

# Get A record
grpcurl -plaintext -d '{"domain": "api.example.com"}' \
  localhost:10033 dns.DnsService/GetA

# Set MX record
grpcurl -plaintext -d '{
  "id": "example.com",
  "mx": {"preference": 10, "mx": "mail.example.com."},
  "ttl": 3600
}' localhost:10033 dns.DnsService/SetMx

# Set TXT record (SPF)
grpcurl -plaintext -d '{
  "id": "example.com",
  "values": ["v=spf1 mx a ~all"],
  "ttl": 3600
}' localhost:10033 dns.DnsService/SetText
```

## High Availability Setup

For production environments, run at least two DNS servers:

### Primary DNS (ns1)

```bash
# On ns1.yourdomain.com
./dns.DnsService dns-primary

# Configure as primary
client.SetDomains("", []string{"yourdomain.com"})
client.SetSoa("", "yourdomain.com", "ns1.yourdomain.com.", ...)
```

### Secondary DNS (ns2)

For a secondary DNS, you can either:
1. Run another Globular DNS service with replicated records
2. Use zone transfer to a traditional DNS server (BIND, etc.)

Both nameservers should have identical records for redundancy.

## Troubleshooting

### Port 53 Already in Use

```bash
# Check what's using port 53
sudo lsof -i :53
sudo ss -tulnp | grep :53

# If systemd-resolved is using it
sudo systemctl stop systemd-resolved
# Then follow the setup guide above
```

### DNS Queries Not Reaching Server

1. Check firewall rules
2. Verify port forwarding (if behind NAT)
3. Test locally first: `dig @127.0.0.1 yourdomain.com`

### Records Not Found

1. Verify domain is in managed domains: `GetDomains()`
2. Check record exists: `GetA("subdomain.yourdomain.com")`
3. Ensure FQDN ends with `.` in some cases

### Propagation Issues

- Registrar changes can take 24-48 hours
- Check with multiple DNS checkers
- Verify glue records are set at registrar

## Configuration Reference

### Service Configuration

| Field | Description | Default |
|-------|-------------|---------|
| `Port` | gRPC API port | `10033` |
| `Proxy` | HTTP proxy port | `10034` |
| `DnsPort` | UDP DNS port | `53` |
| `Root` | Storage directory | `/var/lib/globular/data` |

### Environment Variables

| Variable | Description |
|----------|-------------|
| `GLOBULAR_DOMAIN` | Default domain for the service |
| `GLOBULAR_ADDRESS` | Service listen address |

## Dependencies

- Storage: BadgerDB (embedded, no external dependencies)
- DNS library: `github.com/miekg/dns`

## Cluster Integration

When deployed in a Globular cluster:

- [Cluster Controller](../clustercontroller/README.md) - Renders DNS init config (`/var/lib/globular/dns/dns_init.json`)
- [Node Agent](../nodeagent/README.md) - Applies DNS init config and syncs node records

---

[Back to Services Overview](../README.md)
