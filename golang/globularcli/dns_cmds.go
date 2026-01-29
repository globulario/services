package main

import (
	"errors"
	"fmt"
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
)

func init() {
	rootCmd.AddCommand(dnsCmd)
	dnsCmd.AddCommand(dnsDomainsCmd)
	dnsDomainsCmd.AddCommand(dnsDomainsGetCmd, dnsDomainsSetCmd, dnsDomainsAddCmd, dnsDomainsRemoveCmd)
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
