package identity

import (
	"fmt"
	"net"
	"strings"
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
