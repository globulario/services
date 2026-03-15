package identity

import (
	"fmt"
	"net"
	"os"
	"sort"
	"strings"

	"github.com/google/uuid"
)

// ValidateAdvertiseEndpoint ensures endpoint is not localhost/loopback in cluster mode
func ValidateAdvertiseEndpoint(endpoint string, clusterMode bool) error {
	host, _, err := net.SplitHostPort(endpoint)
	if err != nil {
		return fmt.Errorf("invalid endpoint format: %w", err)
	}

	if !clusterMode {
		return nil // Single-node mode allows localhost
	}

	// In cluster mode, reject localhost variants
	if isLocalhost(host) {
		return fmt.Errorf("localhost/loopback not allowed in cluster mode: %s", host)
	}

	return nil
}

// isLocalhost checks for localhost, 127.0.0.1, ::1 variants
func isLocalhost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))

	// Check string literals
	if host == "localhost" || host == "localhost." {
		return true
	}

	// Check IP addresses
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}

	return ip.IsLoopback()
}

// SelectAdvertiseIP picks best non-loopback IP from available interfaces
// It prefers private IPs over public IPs and returns the first suitable IPv4 address
func SelectAdvertiseIP(envOverride string) (string, error) {
	// 1. Respect explicit override
	if envOverride != "" {
		ip := net.ParseIP(envOverride)
		if ip == nil {
			return "", fmt.Errorf("invalid IP in override: %s", envOverride)
		}
		if ip.IsLoopback() {
			return "", fmt.Errorf("loopback IP not allowed: %s", envOverride)
		}
		return ip.String(), nil
	}

	// 2. Gather IPs from interfaces (reusing existing logic pattern)
	ips, err := gatherNonLoopbackIPs()
	if err != nil {
		return "", fmt.Errorf("gather IPs: %w", err)
	}
	if len(ips) == 0 {
		return "", fmt.Errorf("no non-loopback IPs found")
	}

	return ips[0], nil // Already sorted with private IPs first
}

// gatherNonLoopbackIPs collects non-loopback IPv4 addresses from active interfaces
// Returns IPs sorted with private addresses first
func gatherNonLoopbackIPs() ([]string, error) {
	var ips []string
	seen := make(map[string]struct{})

	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("list interfaces: %w", err)
	}

	for _, iface := range ifaces {
		// Skip down or loopback interfaces
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			// Skip nil, loopback, or IPv6 addresses
			if ip == nil || ip.IsLoopback() {
				continue
			}

			// Convert to IPv4
			ip = ip.To4()
			if ip == nil {
				continue // IPv6 address
			}

			text := ip.String()
			if _, ok := seen[text]; ok {
				continue // Duplicate
			}
			seen[text] = struct{}{}
			ips = append(ips, text)
		}
	}

	// Sort IPs: prefer private network addresses first
	sortIPsByPrivacy(ips)

	return ips, nil
}

// sortIPsByPrivacy sorts IPs in-place with private IPs first, then public
func sortIPsByPrivacy(ips []string) {
	// Separate private and public IPs
	var private, public []string
	for _, ipStr := range ips {
		ip := net.ParseIP(ipStr)
		if ip == nil {
			continue
		}
		if isPrivate(ip) {
			private = append(private, ipStr)
		} else {
			public = append(public, ipStr)
		}
	}

	// Rebuild slice with private first, then public
	copy(ips[:len(private)], private)
	copy(ips[len(private):], public)
}

// isPrivate checks if an IP is in private address space
func isPrivate(ip net.IP) bool {
	if ip == nil {
		return false
	}

	// Private IPv4 ranges:
	// 10.0.0.0/8
	// 172.16.0.0/12
	// 192.168.0.0/16
	// 169.254.0.0/16 (link-local)

	if ip.To4() != nil {
		// 10.0.0.0/8
		if ip[0] == 10 {
			return true
		}
		// 172.16.0.0/12
		if ip[0] == 172 && ip[1] >= 16 && ip[1] <= 31 {
			return true
		}
		// 192.168.0.0/16
		if ip[0] == 192 && ip[1] == 168 {
			return true
		}
		// 169.254.0.0/16 (link-local)
		if ip[0] == 169 && ip[1] == 254 {
			return true
		}
	}

	return false
}

// sanitizeNodeName converts hostname to valid DNS name
// Only allows lowercase letters, digits, and hyphens
func sanitizeNodeName(hostname string) string {
	if hostname == "" {
		return "node"
	}

	// Convert to lowercase
	name := strings.ToLower(hostname)

	// Replace invalid DNS characters with hyphens
	name = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			return r
		}
		return '-'
	}, name)

	// Trim leading/trailing hyphens
	name = strings.Trim(name, "-")

	// Ensure it's not empty after sanitization
	if name == "" {
		return "node"
	}

	return name
}

// SanitizeNodeName is the exported version of sanitizeNodeName
func SanitizeNodeName(hostname string) string {
	return sanitizeNodeName(hostname)
}

// globularNodeIDNamespace is a fixed UUID v5 namespace for Globular node IDs.
var globularNodeIDNamespace = uuid.MustParse("a1b2c3d4-e5f6-4a7b-8c9d-0e1f2a3b4c5d")

// StableNodeID returns a deterministic UUID derived from the best available
// hardware identifier on this machine. It picks the MAC address of the
// highest-priority healthy network interface:
//
//  1. Up, non-loopback, non-virtual, with a routable IP and valid MAC
//  2. Prefer physical interfaces (no veth, docker, br-, virbr, vnet, tun, tap)
//  3. Among candidates prefer those with a private IP
//  4. Tie-break: sort by interface name for stability
//
// If no suitable MAC is found, falls back to hostname + sorted IPs.
func StableNodeID() (string, error) {
	mac, err := SelectBestMAC()
	if err == nil && mac != "" {
		return uuid.NewSHA1(globularNodeIDNamespace, []byte("mac:"+mac)).String(), nil
	}

	// Fallback: hostname + IPs
	hostname := ""
	if h, herr := hostnameSafe(); herr == nil {
		hostname = h
	}
	ips, _ := gatherNonLoopbackIPs()
	if hostname == "" && len(ips) == 0 {
		return "", fmt.Errorf("stable node ID: no MAC, hostname, or IP available")
	}
	sort.Strings(ips)
	key := hostname + "|" + strings.Join(ips, "|")
	return uuid.NewSHA1(globularNodeIDNamespace, []byte("host:"+key)).String(), nil
}

// SelectBestMAC picks the MAC address from the best available interface.
func SelectBestMAC() (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}

	type candidate struct {
		name      string
		mac       string
		hasPrivIP bool
		physical  bool
	}
	var candidates []candidate

	for _, iface := range ifaces {
		// Must be up and not loopback.
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		// Must have a valid MAC (not all-zero).
		mac := iface.HardwareAddr.String()
		if mac == "" || mac == "00:00:00:00:00:00" {
			continue
		}
		// Must have at least one IPv4 address (proves it's a working interface).
		addrs, err := iface.Addrs()
		if err != nil || len(addrs) == 0 {
			continue
		}
		hasIPv4 := false
		hasPriv := false
		for _, addr := range addrs {
			ipnet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}
			ip4 := ipnet.IP.To4()
			if ip4 != nil && !ip4.IsLoopback() {
				hasIPv4 = true
				if isPrivate(ip4) {
					hasPriv = true
				}
			}
		}
		if !hasIPv4 {
			continue
		}

		candidates = append(candidates, candidate{
			name:      iface.Name,
			mac:       mac,
			hasPrivIP: hasPriv,
			physical:  isPhysicalInterface(iface.Name),
		})
	}

	if len(candidates) == 0 {
		return "", fmt.Errorf("no suitable interfaces found")
	}

	// Sort: physical > virtual, private IP > public, then by name.
	sort.Slice(candidates, func(i, j int) bool {
		a, b := candidates[i], candidates[j]
		if a.physical != b.physical {
			return a.physical
		}
		if a.hasPrivIP != b.hasPrivIP {
			return a.hasPrivIP
		}
		return a.name < b.name
	})

	return candidates[0].mac, nil
}

// isPhysicalInterface returns false for known virtual interface name prefixes.
func isPhysicalInterface(name string) bool {
	name = strings.ToLower(name)
	virtual := []string{
		"veth", "docker", "br-", "virbr", "vnet",
		"tun", "tap", "flannel", "cni", "calico",
		"wg", "tailscale", "zt",
	}
	for _, prefix := range virtual {
		if strings.HasPrefix(name, prefix) {
			return false
		}
	}
	return true
}

func hostnameSafe() (string, error) {
	return os.Hostname()
}
