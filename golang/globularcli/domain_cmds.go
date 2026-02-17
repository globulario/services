package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/domain"
	"github.com/globulario/services/golang/dnsprovider"
	_ "github.com/globulario/services/golang/dnsprovider/cloudflare" // Register cloudflare provider
	_ "github.com/globulario/services/golang/dnsprovider/godaddy"    // Register godaddy provider
	_ "github.com/globulario/services/golang/dnsprovider/manual"     // Register manual provider
	"github.com/spf13/cobra"
	clientv3 "go.etcd.io/etcd/client/v3"
)

var domainCmd = &cobra.Command{
	Use:   "domain",
	Short: "Manage external domains",
	Long: `Manage external domains for public FQDN registration, DNS management, and ACME certificate acquisition.

External domains enable your Globular node to be accessible via public FQDNs (e.g., globule-ryzen.globular.cloud)
with automated DNS record management and ACME certificate acquisition.`,
}

var domainAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Register a new external domain",
	Long: `Register a new external domain for DNS management and optional ACME certificate acquisition.

This creates an ExternalDomainSpec in etcd which will be reconciled to:
1. Create DNS A/AAAA record pointing to the node's public IP
2. (Optional) Acquire ACME certificate via DNS-01 challenge
3. (Optional) Configure Envoy ingress routing for this domain

Example:
  globular domain add \
    --fqdn globule-ryzen.globular.cloud \
    --zone globular.cloud \
    --provider godaddy \
    --target-ip auto \
    --enable-acme \
    --acme-email admin@globular.cloud`,
	RunE: runDomainAdd,
}

var domainStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show status of external domains",
	Long: `Show the current status of external domains including DNS records, certificates, and ingress.

Without arguments, shows all domains. Use --fqdn to show a specific domain.

Example:
  globular domain status
  globular domain status --fqdn globule-ryzen.globular.cloud`,
	RunE: runDomainStatus,
}

var domainRemoveCmd = &cobra.Command{
	Use:   "remove",
	Short: "Remove an external domain",
	Long: `Remove an external domain registration.

This deletes the ExternalDomainSpec from etcd, stopping reconciliation.
Optionally, it can also clean up DNS records and certificates.

Example:
  globular domain remove --fqdn globule-ryzen.globular.cloud
  globular domain remove --fqdn globule-ryzen.globular.cloud --cleanup-dns`,
	RunE: runDomainRemove,
}

var dnsProviderCmd = &cobra.Command{
	Use:   "provider",
	Short: "Manage DNS provider configurations",
	Long:  `Manage DNS provider configurations for external domain management.`,
}

var dnsProviderAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a DNS provider configuration",
	Long: `Add a DNS provider configuration for managing external domains.

Provider credentials are loaded from environment variables:
  - godaddy: GODADDY_API_KEY, GODADDY_API_SECRET
  - route53: AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY (or instance role)
  - cloudflare: CLOUDFLARE_API_TOKEN or CLOUDFLARE_API_KEY + CLOUDFLARE_EMAIL

Example:
  export GODADDY_API_KEY="your-key"
  export GODADDY_API_SECRET="your-secret"
  globular dns provider add \
    --name my-godaddy \
    --type godaddy \
    --zone globular.cloud`,
	RunE: runDNSProviderAdd,
}

var dnsProviderListCmd = &cobra.Command{
	Use:   "list",
	Short: "List configured DNS providers",
	Long:  `List all configured DNS provider configurations.`,
	RunE:  runDNSProviderList,
}

// Flags for domain add
var (
	domainFQDN         string
	domainZone         string
	domainProvider     string
	domainTargetIP     string
	domainTTL          int
	domainNodeID       string
	domainEnableACME   bool
	domainACMEEmail    string
	domainACMEDir      string
	domainEnableIngress bool
	domainIngressSvc   string
	domainIngressPort  int
)

// Flags for domain remove
var (
	domainCleanupDNS   bool
	domainCleanupCerts bool
)

// Flags for DNS provider add
var (
	providerName string
	providerType string
	providerZone string
	providerTTL  int
)

func init() {
	// domain add flags
	domainAddCmd.Flags().StringVar(&domainFQDN, "fqdn", "", "Fully-qualified domain name (e.g., globule-ryzen.globular.cloud)")
	domainAddCmd.Flags().StringVar(&domainZone, "zone", "", "DNS zone (e.g., globular.cloud)")
	domainAddCmd.Flags().StringVar(&domainProvider, "provider", "", "DNS provider name (must be configured via 'dns provider add')")
	domainAddCmd.Flags().StringVar(&domainTargetIP, "target-ip", "auto", "Target IP address or 'auto' for auto-detection")
	domainAddCmd.Flags().IntVar(&domainTTL, "ttl", 600, "DNS record TTL in seconds")
	domainAddCmd.Flags().StringVar(&domainNodeID, "node-id", "", "Node ID (default: auto-detect from hostname)")
	domainAddCmd.Flags().BoolVar(&domainEnableACME, "enable-acme", false, "Enable ACME certificate acquisition")
	domainAddCmd.Flags().StringVar(&domainACMEEmail, "acme-email", "", "ACME account email (required if --enable-acme)")
	domainAddCmd.Flags().StringVar(&domainACMEDir, "acme-directory", "", "ACME directory URL (empty=production, 'staging'=LE staging)")
	domainAddCmd.Flags().BoolVar(&domainEnableIngress, "enable-ingress", true, "Enable Envoy ingress routing")
	domainAddCmd.Flags().StringVar(&domainIngressSvc, "ingress-service", "gateway", "Ingress backend service")
	domainAddCmd.Flags().IntVar(&domainIngressPort, "ingress-port", 443, "Ingress backend port")

	domainAddCmd.MarkFlagRequired("fqdn")
	domainAddCmd.MarkFlagRequired("zone")
	domainAddCmd.MarkFlagRequired("provider")

	// domain status flags
	domainStatusCmd.Flags().StringVar(&domainFQDN, "fqdn", "", "Show status for specific FQDN (default: all domains)")

	// domain remove flags
	domainRemoveCmd.Flags().StringVar(&domainFQDN, "fqdn", "", "FQDN to remove")
	domainRemoveCmd.Flags().BoolVar(&domainCleanupDNS, "cleanup-dns", false, "Also delete DNS records")
	domainRemoveCmd.Flags().BoolVar(&domainCleanupCerts, "cleanup-certs", false, "Also delete certificates")
	domainRemoveCmd.MarkFlagRequired("fqdn")

	// DNS provider add flags
	dnsProviderAddCmd.Flags().StringVar(&providerName, "name", "", "Provider configuration name")
	dnsProviderAddCmd.Flags().StringVar(&providerType, "type", "", "Provider type (godaddy, route53, cloudflare, manual)")
	dnsProviderAddCmd.Flags().StringVar(&providerZone, "zone", "", "DNS zone this provider manages")
	dnsProviderAddCmd.Flags().IntVar(&providerTTL, "ttl", 600, "Default TTL for DNS records")
	dnsProviderAddCmd.MarkFlagRequired("name")
	dnsProviderAddCmd.MarkFlagRequired("type")
	dnsProviderAddCmd.MarkFlagRequired("zone")

	// Assemble command tree
	domainCmd.AddCommand(domainAddCmd)
	domainCmd.AddCommand(domainStatusCmd)
	domainCmd.AddCommand(domainRemoveCmd)
	dnsProviderCmd.AddCommand(dnsProviderAddCmd)
	dnsProviderCmd.AddCommand(dnsProviderListCmd)
	domainCmd.AddCommand(dnsProviderCmd)

	rootCmd.AddCommand(domainCmd)
}

func runDomainAdd(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), rootCfg.timeout)
	defer cancel()

	// Auto-detect node ID if not provided
	if domainNodeID == "" {
		hostname, err := os.Hostname()
		if err != nil {
			return fmt.Errorf("failed to auto-detect node ID: %w", err)
		}
		domainNodeID = hostname
	}

	// Validate ACME requirements
	if domainEnableACME && domainACMEEmail == "" {
		return fmt.Errorf("--acme-email is required when --enable-acme is set")
	}

	// Build spec
	spec := &domain.ExternalDomainSpec{
		FQDN:        domainFQDN,
		Zone:        domainZone,
		NodeID:      domainNodeID,
		TargetIP:    domainTargetIP,
		ProviderRef: domainProvider,
		TTL:         domainTTL,
		ACME: domain.ACMEConfig{
			Enabled:       domainEnableACME,
			ChallengeType: "dns-01",
			Email:         domainACMEEmail,
			Directory:     domainACMEDir,
		},
		Ingress: domain.IngressConfig{
			Enabled: domainEnableIngress,
			Service: domainIngressSvc,
			Port:    domainIngressPort,
		},
		Status: domain.ExternalDomainStatus{
			Phase:   "Pending",
			Message: "Awaiting reconciliation",
		},
	}

	// Validate spec
	if err := spec.Validate(); err != nil {
		return fmt.Errorf("invalid domain spec: %w", err)
	}

	// Serialize to JSON
	data, err := spec.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize spec: %w", err)
	}

	// Connect to etcd
	etcdClient, err := config.GetEtcdClient()
	if err != nil {
		return fmt.Errorf("failed to connect to etcd: %w", err)
	}
	defer etcdClient.Close()

	// Write to etcd
	key := domain.DomainKey(domainFQDN)
	_, err = etcdClient.Put(ctx, key, string(data))
	if err != nil {
		return fmt.Errorf("failed to save domain spec to etcd: %w", err)
	}

	// Verify persistence by reading back
	verifyResp, err := etcdClient.Get(ctx, key)
	if err != nil {
		return fmt.Errorf("domain saved but verification failed: %w\nPossible causes: etcd connectivity issues, TLS misconfiguration", err)
	}
	if verifyResp.Count == 0 {
		return fmt.Errorf("domain spec not found after write - etcd may be misconfigured\nPossible causes:\n  - etcd endpoint mismatch\n  - service discovery failure\n  - TLS certificate issues\nDebug with: globular domain status --fqdn %s", domainFQDN)
	}

	fmt.Printf("✓ External domain registered: %s\n", domainFQDN)
	fmt.Printf("  Zone:     %s\n", domainZone)
	fmt.Printf("  Provider: %s\n", domainProvider)
	fmt.Printf("  Node:     %s\n", domainNodeID)
	fmt.Printf("  Target:   %s\n", domainTargetIP)
	if domainEnableACME {
		fmt.Printf("  ACME:     enabled (email: %s)\n", domainACMEEmail)
	}
	if domainEnableIngress {
		fmt.Printf("  Ingress:  enabled → %s:%d\n", domainIngressSvc, domainIngressPort)
	}
	fmt.Println()
	fmt.Println("The reconciler will process this domain and create DNS records + certificates.")
	fmt.Printf("Check status with: globular domain status --fqdn %s\n", domainFQDN)

	return nil
}

func runDomainStatus(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), rootCfg.timeout)
	defer cancel()

	// Connect to etcd
	etcdClient, err := config.GetEtcdClient()
	if err != nil {
		return fmt.Errorf("failed to connect to etcd: %w", err)
	}
	defer etcdClient.Close()

	// Query domains
	var resp *clientv3.GetResponse
	if domainFQDN != "" {
		// Specific domain
		resp, err = etcdClient.Get(ctx, domain.DomainKey(domainFQDN))
	} else {
		// All domains
		resp, err = etcdClient.Get(ctx, domain.EtcdDomainPrefix, clientv3.WithPrefix())
	}
	if err != nil {
		return fmt.Errorf("failed to query domains: %w", err)
	}

	if resp.Count == 0 {
		if domainFQDN != "" {
			// Specific domain not found - this is an error
			if rootCfg.output == "json" {
				// Output valid JSON error for scripts
				fmt.Fprintf(os.Stderr, `{"error": "domain not found", "fqdn": %q}`+"\n", domainFQDN)
			} else {
				fmt.Fprintf(os.Stderr, "Domain %q not found.\n", domainFQDN)
			}
			return fmt.Errorf("domain %s not found", domainFQDN)
		} else {
			// No domains registered - not an error, just empty result
			if rootCfg.output == "json" {
				fmt.Println("[]")
			} else {
				fmt.Println("No external domains registered.")
			}
			return nil
		}
	}

	// Parse and display
	if rootCfg.output == "json" {
		return displayDomainsJSON(resp)
	} else if rootCfg.output == "yaml" {
		return displayDomainsYAML(resp)
	}
	return displayDomainsTable(resp)
}

func displayDomainsTable(resp *clientv3.GetResponse) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "FQDN\tPHASE\tDNS\tCERT\tINGRESS\tUPDATED")

	for _, kv := range resp.Kvs {
		spec, err := domain.FromJSON(kv.Value)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to parse domain spec: %v\n", err)
			continue
		}

		// Extract condition statuses
		dnsStatus := getConditionStatus(spec, "DNSRecordCreated")
		certStatus := getConditionStatus(spec, "CertificateValid")
		ingressStatus := getConditionStatus(spec, "IngressConfigured")

		// Format last reconcile time
		updated := "never"
		if !spec.Status.LastReconcile.IsZero() {
			updated = formatDuration(time.Since(spec.Status.LastReconcile))
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			spec.FQDN,
			spec.Status.Phase,
			formatCondition(dnsStatus),
			formatCondition(certStatus),
			formatCondition(ingressStatus),
			updated,
		)
	}

	return w.Flush()
}

func displayDomainsJSON(resp *clientv3.GetResponse) error {
	specs := make([]*domain.ExternalDomainSpec, 0, resp.Count)
	for _, kv := range resp.Kvs {
		spec, err := domain.FromJSON(kv.Value)
		if err != nil {
			return fmt.Errorf("failed to parse domain spec: %w", err)
		}
		specs = append(specs, spec)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(specs)
}

func displayDomainsYAML(resp *clientv3.GetResponse) error {
	// TODO: Implement YAML output if gopkg.in/yaml.v3 is available
	return fmt.Errorf("YAML output not implemented yet, use --output json")
}

func getConditionStatus(spec *domain.ExternalDomainSpec, condType string) string {
	for _, cond := range spec.Status.Conditions {
		if cond.Type == condType {
			return cond.Status
		}
	}
	return "Unknown"
}

func formatCondition(status string) string {
	switch status {
	case "True":
		return "✓"
	case "False":
		return "✗"
	default:
		return "-"
	}
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	} else if d < time.Hour {
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	} else if d < 24*time.Hour {
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
	return fmt.Sprintf("%dd ago", int(d.Hours()/24))
}

func runDomainRemove(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), rootCfg.timeout)
	defer cancel()

	// Connect to etcd
	etcdClient, err := config.GetEtcdClient()
	if err != nil {
		return fmt.Errorf("failed to connect to etcd: %w", err)
	}
	defer etcdClient.Close()

	// Read current spec (for cleanup operations)
	key := domain.DomainKey(domainFQDN)
	getResp, err := etcdClient.Get(ctx, key)
	if err != nil {
		return fmt.Errorf("failed to read domain spec: %w", err)
	}
	if getResp.Count == 0 {
		return fmt.Errorf("domain %q not found", domainFQDN)
	}

	spec, err := domain.FromJSON(getResp.Kvs[0].Value)
	if err != nil {
		return fmt.Errorf("failed to parse domain spec: %w", err)
	}

	// TODO: Implement DNS cleanup if requested
	if domainCleanupDNS {
		fmt.Println("⚠️  DNS cleanup not implemented yet (--cleanup-dns ignored)")
		// Would need to:
		// 1. Load provider config
		// 2. Initialize provider
		// 3. Delete DNS records
	}

	// TODO: Implement cert cleanup if requested
	if domainCleanupCerts {
		fmt.Println("⚠️  Certificate cleanup not implemented yet (--cleanup-certs ignored)")
		// Would need to delete cert files from disk
	}

	// Delete from etcd
	_, err = etcdClient.Delete(ctx, key)
	if err != nil {
		return fmt.Errorf("failed to delete domain from etcd: %w", err)
	}

	fmt.Printf("✓ Domain removed: %s\n", domainFQDN)
	if !domainCleanupDNS {
		fmt.Println("  Note: DNS records were NOT deleted. Use --cleanup-dns to remove them.")
	}
	if !domainCleanupCerts && spec.ACME.Enabled {
		fmt.Println("  Note: Certificates were NOT deleted. Use --cleanup-certs to remove them.")
	}

	return nil
}

func runDNSProviderAdd(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), rootCfg.timeout)
	defer cancel()

	// Validate provider type is registered
	if !dnsprovider.IsRegistered(providerType) {
		return fmt.Errorf("unknown provider type %q, available: %s",
			providerType, strings.Join(dnsprovider.ListProviders(), ", "))
	}

	// Load credentials from environment
	credentials := make(map[string]string)
	switch providerType {
	case "godaddy":
		apiKey := os.Getenv("GODADDY_API_KEY")
		apiSecret := os.Getenv("GODADDY_API_SECRET")
		if apiKey == "" || apiSecret == "" {
			return fmt.Errorf("GODADDY_API_KEY and GODADDY_API_SECRET environment variables required")
		}
		credentials["api_key"] = apiKey
		credentials["api_secret"] = apiSecret

	case "cloudflare":
		apiToken := os.Getenv("CLOUDFLARE_API_TOKEN")
		if apiToken != "" {
			credentials["api_token"] = apiToken
		} else {
			apiKey := os.Getenv("CLOUDFLARE_API_KEY")
			apiEmail := os.Getenv("CLOUDFLARE_EMAIL")
			if apiKey == "" || apiEmail == "" {
				return fmt.Errorf("CLOUDFLARE_API_TOKEN or (CLOUDFLARE_API_KEY + CLOUDFLARE_EMAIL) required")
			}
			credentials["api_key"] = apiKey
			credentials["api_email"] = apiEmail
		}

	case "route53":
		// Route53 uses AWS SDK credential chain, no explicit credentials needed
		// But we can store optional region
		region := os.Getenv("AWS_REGION")
		if region != "" {
			credentials["region"] = region
		}

	case "manual":
		// No credentials needed for manual provider

	default:
		fmt.Printf("⚠️  Provider type %q credentials not auto-loaded, using empty credentials.\n", providerType)
	}

	// Build provider config
	cfg := dnsprovider.Config{
		Type:        providerType,
		Zone:        providerZone,
		Credentials: credentials,
		DefaultTTL:  providerTTL,
	}

	// Serialize to JSON
	data, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to serialize provider config: %w", err)
	}

	// Connect to etcd
	etcdClient, err := config.GetEtcdClient()
	if err != nil {
		return fmt.Errorf("failed to connect to etcd: %w", err)
	}
	defer etcdClient.Close()

	// Write to etcd
	key := domain.ProviderKey(providerName)
	_, err = etcdClient.Put(ctx, key, string(data))
	if err != nil {
		return fmt.Errorf("failed to save provider config to etcd: %w", err)
	}

	fmt.Printf("✓ DNS provider configured: %s\n", providerName)
	fmt.Printf("  Type: %s\n", providerType)
	fmt.Printf("  Zone: %s\n", providerZone)
	if len(credentials) > 0 {
		fmt.Printf("  Credentials: %d keys stored\n", len(credentials))
	}
	fmt.Println()
	fmt.Printf("Use this provider with: globular domain add --provider %s\n", providerName)

	return nil
}

func runDNSProviderList(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), rootCfg.timeout)
	defer cancel()

	// Connect to etcd
	etcdClient, err := config.GetEtcdClient()
	if err != nil {
		return fmt.Errorf("failed to connect to etcd: %w", err)
	}
	defer etcdClient.Close()

	// Query providers
	resp, err := etcdClient.Get(ctx, domain.EtcdProviderPrefix, clientv3.WithPrefix())
	if err != nil {
		return fmt.Errorf("failed to query providers: %w", err)
	}

	if resp.Count == 0 {
		fmt.Println("No DNS providers configured.")
		fmt.Println("Add one with: globular dns provider add")
		return nil
	}

	// Display table
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tTYPE\tZONE\tCREDENTIALS")

	for _, kv := range resp.Kvs {
		var cfg dnsprovider.Config
		if err := json.Unmarshal(kv.Value, &cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to parse provider config: %v\n", err)
			continue
		}

		name := strings.TrimPrefix(string(kv.Key), domain.EtcdProviderPrefix)
		credsInfo := fmt.Sprintf("%d keys", len(cfg.Credentials))
		if len(cfg.Credentials) == 0 {
			credsInfo = "none"
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", name, cfg.Type, cfg.Zone, credsInfo)
	}

	return w.Flush()
}
