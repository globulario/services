package main

import (
	"errors"
	"fmt"
	"net"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/globulario/services/golang/dns/dnspb"
)

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
			cc, err := dialGRPC(rootCfg.dnsAddr)
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

			cc, err := dialGRPC(rootCfg.dnsAddr)
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

			cc, err := dialGRPC(rootCfg.dnsAddr)
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

			cc, err := dialGRPC(rootCfg.dnsAddr)
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

			cc, err := dialGRPC(rootCfg.dnsAddr)
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

			cc, err := dialGRPC(rootCfg.dnsAddr)
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

			cc, err := dialGRPC(rootCfg.dnsAddr)
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

			cc, err := dialGRPC(rootCfg.dnsAddr)
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

			cc, err := dialGRPC(rootCfg.dnsAddr)
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

			cc, err := dialGRPC(rootCfg.dnsAddr)
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

			cc, err := dialGRPC(rootCfg.dnsAddr)
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

			cc, err := dialGRPC(rootCfg.dnsAddr)
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

			cc, err := dialGRPC(rootCfg.dnsAddr)
			if err != nil {
				return err
			}
			defer cc.Close()

			client := dnspb.NewDnsServiceClient(cc)
			_, err = client.RemoveTXT(ctxWithTimeout(), &dnspb.RemoveTXTRequest{Domain: name, Txt: text})
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
