package netutil

import (
	"errors"
	"net"
	"regexp"
	"strings"
)

// DefaultClusterDomain returns the canonical non-localhost default domain.
func DefaultClusterDomain() string {
	return "globular.internal"
}

// NormalizeDomain lowercases and strips a trailing dot.
func NormalizeDomain(domain string) string {
	d := strings.TrimSpace(strings.ToLower(domain))
	d = strings.TrimSuffix(d, ".")
	return d
}

var invalidDomainPattern = regexp.MustCompile(`^(localhost|localhost\.localdomain)$`)

// ValidateClusterDomain ensures the domain is non-empty, not localhost and not an IP literal.
func ValidateClusterDomain(domain string) error {
	domain = NormalizeDomain(domain)
	if domain == "" {
		return errors.New("domain is empty")
	}
	if invalidDomainPattern.MatchString(domain) {
		return errors.New("domain must not be localhost")
	}
	if ip := net.ParseIP(domain); ip != nil {
		return errors.New("domain must not be an IP literal")
	}
	if strings.Contains(domain, "..") || strings.HasPrefix(domain, ".") {
		return errors.New("domain is malformed")
	}
	return nil
}

// ResolveAdvertiseIP selects a non-loopback IPv4 address for node advertising.
// Priority:
//  1. explicitIP (validated non-loopback)
//  2. preferredIface if set and usable
//  3. first UP, non-loopback interface with a private or global unicast IPv4
func ResolveAdvertiseIP(preferredIface, explicitIP string) (net.IP, error) {
	if explicitIP = strings.TrimSpace(explicitIP); explicitIP != "" {
		ip := net.ParseIP(explicitIP)
		if ip == nil || ip.IsLoopback() || ip.IsUnspecified() {
			return nil, errors.New("explicit advertise address is invalid or loopback")
		}
		if v4 := ip.To4(); v4 != nil {
			return v4, nil
		}
		return ip, nil
	}

	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	pick := func(iface net.Interface) net.IP {
		if (iface.Flags&net.FlagUp) == 0 || (iface.Flags&net.FlagLoopback) != 0 {
			return nil
		}
		addrs, _ := iface.Addrs()
		for _, a := range addrs {
			var ip net.IP
			switch v := a.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil {
				continue
			}
			if ip = ip.To4(); ip == nil {
				continue
			}
			if ip.IsLoopback() || ip.IsUnspecified() {
				continue
			}
			if ip.IsPrivate() || ip.IsGlobalUnicast() {
				return ip
			}
		}
		return nil
	}

	if preferredIface = strings.TrimSpace(preferredIface); preferredIface != "" {
		if iface, err := net.InterfaceByName(preferredIface); err == nil {
			if ip := pick(*iface); ip != nil {
				return ip, nil
			}
		}
	}

	for _, iface := range ifaces {
		if ip := pick(iface); ip != nil {
			return ip, nil
		}
	}
	return nil, errors.New("no suitable advertise address found")
}
