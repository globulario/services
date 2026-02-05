package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"gopkg.in/yaml.v3"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/dns/dnspb"
)

// resolveDnsGrpcEndpoint discovers the DNS service gRPC endpoint dynamically.
// It tries multiple discovery methods:
// 1. Query etcd for DNS service configuration (preferred)
// 2. Use --describe from service binary (if ServicesRoot configured)
// 3. Fall back to provided default
func resolveDnsGrpcEndpoint(fallback string) string {
	// Method 1: Try to resolve from etcd service configuration
	svc, err := config.ResolveService("dns.DnsService")
	if err == nil && svc != nil {
		// Extract port from service config
		var port int
		switch p := svc["Port"].(type) {
		case int:
			port = p
		case float64:
			port = int(p)
		case string:
			fmt.Sscanf(p, "%d", &port)
		}

		if port > 0 {
			host := "localhost"
			if addr, ok := svc["Address"].(string); ok && addr != "" {
				// Check if address already contains port
				if strings.Contains(addr, ":") {
					return addr
				}
				host = addr
			}
			return fmt.Sprintf("%s:%d", host, port)
		}
	}

	// Method 2: Try --describe from binary
	root := config.GetServicesRoot()
	if root != "" {
		binPath, err := config.FindServiceBinary(root, "dns")
		if err == nil {
			desc, err := config.RunDescribe(binPath, 3*time.Second, nil)
			if err == nil && desc.Port > 0 {
				host := "localhost"
				if desc.Address != "" {
					host = desc.Address
				}
				return fmt.Sprintf("%s:%d", host, desc.Port)
			}
		}
	}

	// Method 3: Fallback
	return fallback
}

// resolveDnsResolverEndpoint discovers the DNS resolver listening endpoint.
// It reads the DNS service configuration to get the actual DNS port (typically 53).
func resolveDnsResolverEndpoint() string {
	// Default fallback - standard DNS port
	fallback := "127.0.0.1:53"

	// Check environment variable first
	if dnsPort := os.Getenv("GLOB_DNS_PORT"); dnsPort != "" {
		return fmt.Sprintf("127.0.0.1:%s", dnsPort)
	}

	// Try to read DNS service configuration
	root := config.GetServicesRoot()
	if root == "" {
		return fallback
	}

	_, err := config.FindServiceBinary(root, "dns")
	if err != nil {
		return fallback
	}

	// TODO: Parse DNS service config file to get actual resolver port
	// For now, we use the standard DNS port (53) as fallback
	return fallback
}

// getEffectiveDnsGrpcAddr returns the DNS gRPC endpoint to use,
// preferring user-specified flag over dynamic discovery.
func getEffectiveDnsGrpcAddr() string {
	// If user explicitly set --dns flag, use it
	if rootCfg.dnsAddr != "localhost:10033" {
		return rootCfg.dnsAddr
	}

	// Otherwise, try to discover it
	discovered := resolveDnsGrpcEndpoint("localhost:10033")
	return discovered
}

var (
	dnsCmd = &cobra.Command{
		Use:   "dns",
		Short: "DNS service helpers",
	}

	dnsDomainsCmd = &cobra.Command{
		Use:   "domains",
		Short: "Manage DNS managed domains",
	}

	dnsDomainsGetCmd = &cobra.Command{
		Use:   "get",
		Short: "Get managed domains",
		RunE: func(cmd *cobra.Command, args []string) error {
			cc, err := dialGRPC(getEffectiveDnsGrpcAddr())
			if err != nil {
				return err
			}
			defer cc.Close()

			client := dnspb.NewDnsServiceClient(cc)
			resp, err := client.GetDomains(ctxWithTimeout(), &dnspb.GetDomainsRequest{})
			if err != nil {
				return err
			}
			printStringList(resp.Domains)
			return nil
		},
	}

	dnsDomainsSetCmd = &cobra.Command{
		Use:   "set <domain> [<domain>...]",
		Short: "Replace managed domains with the provided list",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			domains := normalizeDomains(args)
			if len(domains) == 0 {
				return errors.New("no valid domains provided")
			}

			cc, err := dialGRPC(getEffectiveDnsGrpcAddr())
			if err != nil {
				return err
			}
			defer cc.Close()

			client := dnspb.NewDnsServiceClient(cc)
			_, err = client.SetDomains(ctxWithTimeout(), &dnspb.SetDomainsRequest{Domains: domains})
			return err
		},
	}

	dnsDomainsAddCmd = &cobra.Command{
		Use:   "add <domain> [<domain>...]",
		Short: "Add domains to the managed domains list",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			toAdd := normalizeDomains(args)
			if len(toAdd) == 0 {
				return errors.New("no valid domains provided")
			}

			cc, err := dialGRPC(getEffectiveDnsGrpcAddr())
			if err != nil {
				return err
			}
			defer cc.Close()

			client := dnspb.NewDnsServiceClient(cc)

			cur, err := client.GetDomains(ctxWithTimeout(), &dnspb.GetDomainsRequest{})
			if err != nil {
				return err
			}

			merged := mergeDomains(cur.Domains, toAdd)

			_, err = client.SetDomains(ctxWithTimeout(), &dnspb.SetDomainsRequest{Domains: merged})
			return err
		},
	}

	dnsDomainsRemoveCmd = &cobra.Command{
		Use:   "remove <domain> [<domain>...]",
		Short: "Remove domains from the managed domains list",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			toRemove := normalizeDomains(args)
			if len(toRemove) == 0 {
				return errors.New("no valid domains provided")
			}

			cc, err := dialGRPC(getEffectiveDnsGrpcAddr())
			if err != nil {
				return err
			}
			defer cc.Close()

			client := dnspb.NewDnsServiceClient(cc)

			cur, err := client.GetDomains(ctxWithTimeout(), &dnspb.GetDomainsRequest{})
			if err != nil {
				return err
			}

			next := removeDomains(cur.Domains, toRemove)
			_, err = client.SetDomains(ctxWithTimeout(), &dnspb.SetDomainsRequest{Domains: next})
			return err
		},
	}

	// A record commands
	dnsACmd = &cobra.Command{
		Use:   "a",
		Short: "Manage DNS A (IPv4) records",
	}

	dnsASetCmd = &cobra.Command{
		Use:   "set <name> <ipv4>",
		Short: "Set an A record",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := strings.TrimSpace(args[0])
			ipv4 := strings.TrimSpace(args[1])

			if net.ParseIP(ipv4) == nil || strings.Contains(ipv4, ":") {
				return fmt.Errorf("invalid IPv4 address: %s", ipv4)
			}

			ttl, _ := cmd.Flags().GetUint32("ttl")

			cc, err := dialGRPC(getEffectiveDnsGrpcAddr())
			if err != nil {
				return err
			}
			defer cc.Close()

			client := dnspb.NewDnsServiceClient(cc)
			_, err = client.SetA(ctxWithTimeout(), &dnspb.SetARequest{Domain: name, A: ipv4, Ttl: ttl})
			return err
		},
	}

	dnsAGetCmd = &cobra.Command{
		Use:   "get <name>",
		Short: "Get A records for a name",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := strings.TrimSpace(args[0])

			cc, err := dialGRPC(getEffectiveDnsGrpcAddr())
			if err != nil {
				return err
			}
			defer cc.Close()

			client := dnspb.NewDnsServiceClient(cc)
			resp, err := client.GetA(ctxWithTimeout(), &dnspb.GetARequest{Domain: name})
			if err != nil {
				return err
			}
			printStringList(resp.A)
			return nil
		},
	}

	dnsARemoveCmd = &cobra.Command{
		Use:   "remove <name> [<ipv4>]",
		Short: "Remove A record(s) for a name",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := strings.TrimSpace(args[0])
			ipv4 := ""
			if len(args) == 2 {
				ipv4 = strings.TrimSpace(args[1])
				if net.ParseIP(ipv4) == nil || strings.Contains(ipv4, ":") {
					return fmt.Errorf("invalid IPv4 address: %s", ipv4)
				}
			}

			cc, err := dialGRPC(getEffectiveDnsGrpcAddr())
			if err != nil {
				return err
			}
			defer cc.Close()

			client := dnspb.NewDnsServiceClient(cc)
			_, err = client.RemoveA(ctxWithTimeout(), &dnspb.RemoveARequest{Domain: name, A: ipv4})
			return err
		},
	}

	// AAAA record commands
	dnsAAAACmd = &cobra.Command{
		Use:   "aaaa",
		Short: "Manage DNS AAAA (IPv6) records",
	}

	dnsAAAASetCmd = &cobra.Command{
		Use:   "set <name> <ipv6>",
		Short: "Set an AAAA record",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := strings.TrimSpace(args[0])
			ipv6 := strings.TrimSpace(args[1])

			if net.ParseIP(ipv6) == nil || !strings.Contains(ipv6, ":") {
				return fmt.Errorf("invalid IPv6 address: %s", ipv6)
			}

			ttl, _ := cmd.Flags().GetUint32("ttl")

			cc, err := dialGRPC(getEffectiveDnsGrpcAddr())
			if err != nil {
				return err
			}
			defer cc.Close()

			client := dnspb.NewDnsServiceClient(cc)
			_, err = client.SetAAAA(ctxWithTimeout(), &dnspb.SetAAAARequest{Domain: name, Aaaa: ipv6, Ttl: ttl})
			return err
		},
	}

	dnsAAAAGetCmd = &cobra.Command{
		Use:   "get <name>",
		Short: "Get AAAA records for a name",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := strings.TrimSpace(args[0])

			cc, err := dialGRPC(getEffectiveDnsGrpcAddr())
			if err != nil {
				return err
			}
			defer cc.Close()

			client := dnspb.NewDnsServiceClient(cc)
			resp, err := client.GetAAAA(ctxWithTimeout(), &dnspb.GetAAAARequest{Domain: name})
			if err != nil {
				return err
			}
			printStringList(resp.Aaaa)
			return nil
		},
	}

	dnsAAAARemoveCmd = &cobra.Command{
		Use:   "remove <name> [<ipv6>]",
		Short: "Remove AAAA record(s) for a name",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := strings.TrimSpace(args[0])
			ipv6 := ""
			if len(args) == 2 {
				ipv6 = strings.TrimSpace(args[1])
				if net.ParseIP(ipv6) == nil || !strings.Contains(ipv6, ":") {
					return fmt.Errorf("invalid IPv6 address: %s", ipv6)
				}
			}

			cc, err := dialGRPC(getEffectiveDnsGrpcAddr())
			if err != nil {
				return err
			}
			defer cc.Close()

			client := dnspb.NewDnsServiceClient(cc)
			_, err = client.RemoveAAAA(ctxWithTimeout(), &dnspb.RemoveAAAARequest{Domain: name, Aaaa: ipv6})
			return err
		},
	}

	// TXT record commands
	dnsTXTCmd = &cobra.Command{
		Use:   "txt",
		Short: "Manage DNS TXT records",
	}

	dnsTXTSetCmd = &cobra.Command{
		Use:   "set <name> <text>",
		Short: "Set a TXT record",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := strings.TrimSpace(args[0])
			text := args[1] // Don't trim text as whitespace might be significant

			ttl, _ := cmd.Flags().GetUint32("ttl")

			cc, err := dialGRPC(getEffectiveDnsGrpcAddr())
			if err != nil {
				return err
			}
			defer cc.Close()

			client := dnspb.NewDnsServiceClient(cc)
			_, err = client.SetTXT(ctxWithTimeout(), &dnspb.SetTXTRequest{Domain: name, Txt: text, Ttl: ttl})
			return err
		},
	}

	dnsTXTGetCmd = &cobra.Command{
		Use:   "get <name>",
		Short: "Get TXT records for a name",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := strings.TrimSpace(args[0])

			cc, err := dialGRPC(getEffectiveDnsGrpcAddr())
			if err != nil {
				return err
			}
			defer cc.Close()

			client := dnspb.NewDnsServiceClient(cc)
			resp, err := client.GetTXT(ctxWithTimeout(), &dnspb.GetTXTRequest{Domain: name})
			if err != nil {
				return err
			}
			printStringList(resp.Txt)
			return nil
		},
	}

	dnsTXTRemoveCmd = &cobra.Command{
		Use:   "remove <name> [<text>]",
		Short: "Remove TXT record(s) for a name",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := strings.TrimSpace(args[0])
			text := ""
			if len(args) == 2 {
				text = args[1]
			}

			cc, err := dialGRPC(getEffectiveDnsGrpcAddr())
			if err != nil {
				return err
			}
			defer cc.Close()

			client := dnspb.NewDnsServiceClient(cc)
			_, err = client.RemoveTXT(ctxWithTimeout(), &dnspb.RemoveTXTRequest{Domain: name, Txt: text})
			return err
		},
	}

	// DNS lookup command (resolver-based)
	dnsLookupCmd = &cobra.Command{
		Use:   "lookup <name>",
		Short: "DNS resolver-based lookup (what clients resolve)",
		Args:  cobra.ExactArgs(1),
		RunE:  runDNSLookup,
	}

	// DNS inspect command (gRPC-based)
	dnsInspectCmd = &cobra.Command{
		Use:   "inspect <name>",
		Short: "Inspect DNS records via gRPC (what DNS service stores)",
		Args:  cobra.ExactArgs(1),
		RunE:  runDNSInspect,
	}

	// DNS status command
	dnsStatusCmd = &cobra.Command{
		Use:   "status",
		Short: "Show DNS service status and health",
		RunE:  runDNSStatus,
	}

	// SRV record commands
	dnsSRVCmd = &cobra.Command{
		Use:   "srv",
		Short: "Manage DNS SRV records",
	}

	dnsSRVGetCmd = &cobra.Command{
		Use:   "get <name>",
		Short: "Get SRV records for a name",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := strings.TrimSpace(args[0])

			cc, err := dialGRPC(getEffectiveDnsGrpcAddr())
			if err != nil {
				return err
			}
			defer cc.Close()

			client := dnspb.NewDnsServiceClient(cc)
			resp, err := client.GetSrv(ctxWithTimeout(), &dnspb.GetSrvRequest{Id: name})
			if err != nil {
				return err
			}

			if len(resp.Result) == 0 {
				fmt.Println("No SRV records found")
				return nil
			}

			printSRVRecords(resp.Result)
			return nil
		},
	}

	dnsSRVSetCmd = &cobra.Command{
		Use:   "set <name> <target> <port>",
		Short: "Set an SRV record",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := strings.TrimSpace(args[0])
			target := strings.TrimSpace(args[1])
			port := args[2]

			priority, _ := cmd.Flags().GetUint32("priority")
			weight, _ := cmd.Flags().GetUint32("weight")
			ttl, _ := cmd.Flags().GetUint32("ttl")

			var portNum uint32
			if _, err := fmt.Sscanf(port, "%d", &portNum); err != nil || portNum < 1 || portNum > 65535 {
				return fmt.Errorf("invalid port: %s (must be 1-65535)", port)
			}

			cc, err := dialGRPC(getEffectiveDnsGrpcAddr())
			if err != nil {
				return err
			}
			defer cc.Close()

			client := dnspb.NewDnsServiceClient(cc)
			_, err = client.SetSrv(ctxWithTimeout(), &dnspb.SetSrvRequest{
				Id: name,
				Srv: &dnspb.SRV{
					Priority: priority,
					Weight:   weight,
					Port:     portNum,
					Target:   target,
				},
				Ttl: ttl,
			})
			return err
		},
	}

	dnsSRVRemoveCmd = &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove SRV record(s) for a name",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := strings.TrimSpace(args[0])

			cc, err := dialGRPC(getEffectiveDnsGrpcAddr())
			if err != nil {
				return err
			}
			defer cc.Close()

			client := dnspb.NewDnsServiceClient(cc)
			_, err = client.RemoveSrv(ctxWithTimeout(), &dnspb.RemoveSrvRequest{Id: name})
			return err
		},
	}
)

func init() {
	rootCmd.AddCommand(dnsCmd)
	dnsCmd.AddCommand(dnsDomainsCmd)
	dnsDomainsCmd.AddCommand(dnsDomainsGetCmd, dnsDomainsSetCmd, dnsDomainsAddCmd, dnsDomainsRemoveCmd)

	// A record commands
	dnsCmd.AddCommand(dnsACmd)
	dnsACmd.AddCommand(dnsASetCmd, dnsAGetCmd, dnsARemoveCmd)
	dnsASetCmd.Flags().Uint32("ttl", 0, "TTL for the record")

	// AAAA record commands
	dnsCmd.AddCommand(dnsAAAACmd)
	dnsAAAACmd.AddCommand(dnsAAAASetCmd, dnsAAAAGetCmd, dnsAAAARemoveCmd)
	dnsAAAASetCmd.Flags().Uint32("ttl", 0, "TTL for the record")

	// TXT record commands
	dnsCmd.AddCommand(dnsTXTCmd)
	dnsTXTCmd.AddCommand(dnsTXTSetCmd, dnsTXTGetCmd, dnsTXTRemoveCmd)
	dnsTXTSetCmd.Flags().Uint32("ttl", 0, "TTL for the record (default 300)")

	// Inspection commands
	dnsCmd.AddCommand(dnsLookupCmd, dnsInspectCmd, dnsStatusCmd)
	dnsLookupCmd.Flags().String("type", "A", "Record type (A, AAAA, TXT, SRV, ALL)")
	dnsLookupCmd.Flags().String("server", "", "DNS server for resolver mode (default: auto-discover)")
	dnsLookupCmd.Flags().Bool("tcp", false, "Use TCP instead of UDP")
	dnsInspectCmd.Flags().String("types", "A,AAAA,TXT,SRV", "Record types to inspect (comma-separated)")

	// SRV record commands
	dnsCmd.AddCommand(dnsSRVCmd)
	dnsSRVCmd.AddCommand(dnsSRVGetCmd, dnsSRVSetCmd, dnsSRVRemoveCmd)
	dnsSRVSetCmd.Flags().Uint32("priority", 10, "Priority")
	dnsSRVSetCmd.Flags().Uint32("weight", 10, "Weight")
	dnsSRVSetCmd.Flags().Uint32("ttl", 300, "TTL")
}

func normalizeDomains(in []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, d := range in {
		d = strings.TrimSpace(strings.ToLower(d))
		d = strings.TrimSuffix(d, ".")
		if d == "" {
			continue
		}
		if _, ok := seen[d]; ok {
			continue
		}
		seen[d] = struct{}{}
		out = append(out, d)
	}
	sort.Strings(out)
	return out
}

func mergeDomains(cur []string, add []string) []string {
	return normalizeDomains(append(cur, add...))
}

func removeDomains(cur []string, remove []string) []string {
	rm := map[string]struct{}{}
	for _, d := range normalizeDomains(remove) {
		rm[d] = struct{}{}
	}
	out := make([]string, 0, len(cur))
	for _, d := range normalizeDomains(cur) {
		if _, ok := rm[d]; ok {
			continue
		}
		out = append(out, d)
	}
	sort.Strings(out)
	return out
}

func printStringList(items []string) {
	if rootCfg.output == "json" {
		fmt.Printf("%q\n", items)
		return
	}
	for _, s := range items {
		fmt.Println(s)
	}
}

// runDNSLookup performs DNS resolution via resolver (PR-DNSCLI)
func runDNSLookup(cmd *cobra.Command, args []string) error {
	name := strings.TrimSpace(args[0])
	recordType, _ := cmd.Flags().GetString("type")
	server, _ := cmd.Flags().GetString("server")
	useTCP, _ := cmd.Flags().GetBool("tcp")

	// If server not specified, discover it dynamically
	if server == "" {
		server = resolveDnsResolverEndpoint()
	}

	recordType = strings.ToUpper(recordType)

	// Create resolver
	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{Timeout: rootCfg.timeout}
			proto := "udp"
			if useTCP {
				proto = "tcp"
			}
			return d.DialContext(ctx, proto, server)
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), rootCfg.timeout)
	defer cancel()

	result := &LookupResult{
		Name:   name,
		Server: server,
	}

	switch recordType {
	case "A":
		ips, err := lookupIPv4(ctx, resolver, name)
		if err != nil {
			return err
		}
		result.A = ips
	case "AAAA":
		ips, err := lookupIPv6(ctx, resolver, name)
		if err != nil {
			return err
		}
		result.AAAA = ips
	case "TXT":
		txts, err := resolver.LookupTXT(ctx, name)
		if err != nil {
			return err
		}
		result.TXT = txts
	case "SRV":
		srvs, err := lookupSRV(ctx, resolver, name)
		if err != nil {
			return err
		}
		result.SRV = srvs
	case "ALL":
		result.A, _ = lookupIPv4(ctx, resolver, name)
		result.AAAA, _ = lookupIPv6(ctx, resolver, name)
		result.TXT, _ = resolver.LookupTXT(ctx, name)
		result.SRV, _ = lookupSRV(ctx, resolver, name)
	default:
		return fmt.Errorf("unsupported record type: %s", recordType)
	}

	return printLookupResult(result)
}

// runDNSInspect inspects DNS records via gRPC (PR-DNSCLI)
func runDNSInspect(cmd *cobra.Command, args []string) error {
	name := strings.TrimSpace(args[0])
	typesStr, _ := cmd.Flags().GetString("types")
	types := strings.Split(typesStr, ",")

	cc, err := dialGRPC(getEffectiveDnsGrpcAddr())
	if err != nil {
		return err
	}
	defer cc.Close()

	client := dnspb.NewDnsServiceClient(cc)
	result := &InspectResult{
		Name:   name,
		Source: "grpc",
	}

	for _, t := range types {
		t = strings.TrimSpace(strings.ToUpper(t))
		switch t {
		case "A":
			resp, err := client.GetA(ctxWithTimeout(), &dnspb.GetARequest{Domain: name})
			if err == nil {
				result.A = resp.A
			}
		case "AAAA":
			resp, err := client.GetAAAA(ctxWithTimeout(), &dnspb.GetAAAARequest{Domain: name})
			if err == nil {
				result.AAAA = resp.Aaaa
			}
		case "TXT":
			resp, err := client.GetTXT(ctxWithTimeout(), &dnspb.GetTXTRequest{Domain: name})
			if err == nil {
				result.TXT = resp.Txt
			}
		case "SRV":
			// Note: GetSrv RPC may not exist yet in generated code
			// Skip silently if method doesn't exist
			_ = name // Use name to avoid unused warning
		}
	}

	return printInspectResult(result)
}

// runDNSStatus shows DNS service status (PR-DNSCLI)
func runDNSStatus(cmd *cobra.Command, args []string) error {
	grpcEndpoint := getEffectiveDnsGrpcAddr()
	resolverEndpoint := resolveDnsResolverEndpoint()

	fmt.Printf("DNS Service Status\n")
	fmt.Printf("==================\n\n")
	fmt.Printf("gRPC Endpoint: %s\n", grpcEndpoint)
	fmt.Printf("Resolver Endpoint: %s\n\n", resolverEndpoint)

	// gRPC check with short timeout
	fmt.Printf("Checking gRPC connectivity...\n")
	dialCtx, dialCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer dialCancel()

	cc, err := grpc.DialContext(dialCtx, grpcEndpoint,
		grpc.WithBlock(),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		fmt.Printf("❌ gRPC Check: FAILED (cannot connect to %s: %v)\n", grpcEndpoint, err)
		return nil
	}
	defer cc.Close()

	client := dnspb.NewDnsServiceClient(cc)
	ctx1, cancel1 := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel1()

	resp, err := client.GetDomains(ctx1, &dnspb.GetDomainsRequest{})
	if err != nil {
		// Check if it's a "key not found" error (DNS not initialized)
		if strings.Contains(err.Error(), "Key not found") || strings.Contains(err.Error(), "badger") {
			fmt.Printf("✓ gRPC Check: OK (connected)\n")
			fmt.Printf("  ⚠  DNS service not initialized (no domains configured yet)\n")
			return nil
		}
		fmt.Printf("❌ gRPC Check: FAILED (%v)\n", err)
		return nil
	}

	fmt.Printf("✓ gRPC Check: OK\n")
	if len(resp.Domains) == 0 {
		fmt.Printf("  Managed Domains: (none)\n")
	} else {
		fmt.Printf("  Managed Domains: %d\n", len(resp.Domains))
		for _, d := range resp.Domains {
			fmt.Printf("    - %s\n", d)
		}
	}

	// Resolver check - query Globular DNS directly, not OS resolver
	fmt.Printf("\nResolver Check:\n")

	// Create custom resolver pointing to Globular DNS
	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{Timeout: 1 * time.Second}
			return d.DialContext(ctx, "udp", resolverEndpoint)
		},
	}

	ctx2, cancel2 := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel2()

	// Test with localhost first (should always work if resolver is running)
	testName := "localhost"
	if len(resp.Domains) > 0 {
		testName = fmt.Sprintf("controller.%s", resp.Domains[0])
	}

	ips, err := resolver.LookupIP(ctx2, "ip", testName)
	if err != nil {
		// Check if it's a timeout or connection refused
		if ctx2.Err() == context.DeadlineExceeded {
			fmt.Printf("  ❌ Resolver unreachable (timeout)\n")
		} else {
			fmt.Printf("  ⚠  Lookup for %s failed: %v\n", testName, err)
			// Try a simple connectivity test
			conn, connErr := net.DialTimeout("udp", resolverEndpoint, 1*time.Second)
			if connErr != nil {
				fmt.Printf("  ❌ Cannot connect to resolver at %s\n", resolverEndpoint)
			} else {
				conn.Close()
				fmt.Printf("  ✓  Resolver reachable at %s\n", resolverEndpoint)
			}
		}
	} else {
		fmt.Printf("  ✓  Resolved %s: %v\n", testName, ips)
	}

	return nil
}

// Helper types for output
type LookupResult struct {
	Name   string      `json:"name" yaml:"name"`
	Server string      `json:"server" yaml:"server"`
	A      []string    `json:"a,omitempty" yaml:"a,omitempty"`
	AAAA   []string    `json:"aaaa,omitempty" yaml:"aaaa,omitempty"`
	TXT    []string    `json:"txt,omitempty" yaml:"txt,omitempty"`
	SRV    []SRVRecord `json:"srv,omitempty" yaml:"srv,omitempty"`
}

type InspectResult struct {
	Name   string   `json:"name" yaml:"name"`
	Source string   `json:"source" yaml:"source"`
	A      []string `json:"a,omitempty" yaml:"a,omitempty"`
	AAAA   []string `json:"aaaa,omitempty" yaml:"aaaa,omitempty"`
	TXT    []string `json:"txt,omitempty" yaml:"txt,omitempty"`
}

type SRVRecord struct {
	Priority uint16 `json:"priority" yaml:"priority"`
	Weight   uint16 `json:"weight" yaml:"weight"`
	Port     uint16 `json:"port" yaml:"port"`
	Target   string `json:"target" yaml:"target"`
}

// Helper functions
func lookupIPv4(ctx context.Context, resolver *net.Resolver, name string) ([]string, error) {
	ips, err := resolver.LookupIP(ctx, "ip4", name)
	if err != nil {
		return nil, err
	}
	result := make([]string, len(ips))
	for i, ip := range ips {
		result[i] = ip.String()
	}
	return result, nil
}

func lookupIPv6(ctx context.Context, resolver *net.Resolver, name string) ([]string, error) {
	ips, err := resolver.LookupIP(ctx, "ip6", name)
	if err != nil {
		return nil, err
	}
	result := make([]string, len(ips))
	for i, ip := range ips {
		result[i] = ip.String()
	}
	return result, nil
}

func lookupSRV(ctx context.Context, resolver *net.Resolver, name string) ([]SRVRecord, error) {
	// Parse SRV name: _service._proto.domain
	if !strings.HasPrefix(name, "_") {
		return nil, fmt.Errorf("SRV name must start with _service._proto.domain format")
	}

	parts := strings.SplitN(name, ".", 3)
	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid SRV name format (expected _service._proto.domain)")
	}

	service := strings.TrimPrefix(parts[0], "_")
	proto := strings.TrimPrefix(parts[1], "_")
	domain := parts[2]

	_, srvs, err := resolver.LookupSRV(ctx, service, proto, domain)
	if err != nil {
		return nil, err
	}

	result := make([]SRVRecord, len(srvs))
	for i, srv := range srvs {
		result[i] = SRVRecord{
			Priority: srv.Priority,
			Weight:   srv.Weight,
			Port:     srv.Port,
			Target:   srv.Target,
		}
	}
	return result, nil
}

func printLookupResult(result *LookupResult) error {
	switch rootCfg.output {
	case "json":
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	case "yaml":
		data, err := yaml.Marshal(result)
		if err != nil {
			return err
		}
		fmt.Print(string(data))
		return nil
	default:
		// Table format
		fmt.Printf("Lookup: %s (via %s)\n\n", result.Name, result.Server)
		if len(result.A) > 0 {
			fmt.Println("A Records:")
			for _, a := range result.A {
				fmt.Printf("  %s\n", a)
			}
		}
		if len(result.AAAA) > 0 {
			fmt.Println("AAAA Records:")
			for _, aaaa := range result.AAAA {
				fmt.Printf("  %s\n", aaaa)
			}
		}
		if len(result.TXT) > 0 {
			fmt.Println("TXT Records:")
			for _, txt := range result.TXT {
				fmt.Printf("  %s\n", txt)
			}
		}
		if len(result.SRV) > 0 {
			fmt.Println("SRV Records:")
			for _, srv := range result.SRV {
				fmt.Printf("  %d %d %d %s\n", srv.Priority, srv.Weight, srv.Port, srv.Target)
			}
		}
	}
	return nil
}

func printInspectResult(result *InspectResult) error {
	switch rootCfg.output {
	case "json":
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	case "yaml":
		data, err := yaml.Marshal(result)
		if err != nil {
			return err
		}
		fmt.Print(string(data))
		return nil
	default:
		// Table format
		fmt.Printf("Inspect: %s (source: %s)\n\n", result.Name, result.Source)
		if len(result.A) > 0 {
			fmt.Println("A Records:")
			for _, a := range result.A {
				fmt.Printf("  %s\n", a)
			}
		}
		if len(result.AAAA) > 0 {
			fmt.Println("AAAA Records:")
			for _, aaaa := range result.AAAA {
				fmt.Printf("  %s\n", aaaa)
			}
		}
		if len(result.TXT) > 0 {
			fmt.Println("TXT Records:")
			for _, txt := range result.TXT {
				fmt.Printf("  %s\n", txt)
			}
		}
	}
	return nil
}

func printSRVRecords(records interface{}) {
	if rootCfg.output == "json" {
		data, _ := json.MarshalIndent(records, "", "  ")
		fmt.Println(string(data))
		return
	}
	// Table format - records should be printed by caller for now
	fmt.Printf("%v\n", records)
}
