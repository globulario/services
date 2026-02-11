package domain

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ExternalDomainSpec describes the desired state for an external FQDN.
// It is stored in etcd at /globular/domains/v1/<fqdn> and drives reconciliation.
type ExternalDomainSpec struct {
	// FQDN is the fully-qualified domain name (e.g., "globule-ryzen.globular.cloud")
	FQDN string `json:"fqdn"`

	// Zone is the root DNS zone (e.g., "globular.cloud")
	// The FQDN must be a subdomain of this zone.
	Zone string `json:"zone"`

	// NodeID identifies the node/gateway this domain should route to
	// (e.g., "globule-ryzen" or cluster node ID)
	NodeID string `json:"node_id"`

	// TargetIP is the public IP address for the A/AAAA record.
	// Special value "auto" means auto-detect public IP.
	TargetIP string `json:"target_ip"`

	// ProviderRef is the name/ID of the DNS provider configuration.
	// Refers to a ProviderConfig stored at /globular/providers/v1/<name>
	ProviderRef string `json:"provider_ref"`

	// TTL is the DNS record time-to-live in seconds.
	// Default: 600 (10 minutes)
	TTL int `json:"ttl"`

	// ACME configuration for automated certificate acquisition.
	ACME ACMEConfig `json:"acme"`

	// Ingress configuration for routing traffic to this domain.
	Ingress IngressConfig `json:"ingress"`

	// Status reflects the current reconciliation state (updated by reconciler).
	Status DomainStatus `json:"status"`
}

// ACMEConfig controls automated certificate acquisition via ACME (Let's Encrypt).
type ACMEConfig struct {
	// Enabled indicates if ACME certificate acquisition is enabled.
	Enabled bool `json:"enabled"`

	// ChallengeType is the ACME challenge method ("dns-01" only for now).
	// HTTP-01 not supported yet because it requires port 80 reachability.
	ChallengeType string `json:"challenge_type"`

	// Email is the contact email for the ACME account.
	Email string `json:"email"`

	// Directory is the ACME directory URL.
	// Empty/"production": Let's Encrypt production
	// "staging": Let's Encrypt staging (for testing)
	// Custom URL: For private ACME servers
	Directory string `json:"directory,omitempty"`
}

// IngressConfig controls how Envoy routes traffic to this domain.
type IngressConfig struct {
	// Enabled indicates if ingress routing should be configured.
	Enabled bool `json:"enabled"`

	// Service is the backend service to route to (e.g., "gateway").
	Service string `json:"service"`

	// Port is the backend service port (typically 443 for HTTPS, 80 for HTTP).
	Port int `json:"port"`
}

// DomainStatus reflects the current reconciliation state.
// This is updated by the reconciler and should be treated as read-only by users.
type DomainStatus struct {
	// LastReconcile is when the reconciler last processed this domain.
	LastReconcile time.Time `json:"last_reconcile"`

	// Phase indicates the overall state: "Pending", "Ready", "Error"
	Phase string `json:"phase"`

	// Conditions provides detailed status for each reconciliation step.
	Conditions []Condition `json:"conditions,omitempty"`

	// Message is a human-readable status message.
	Message string `json:"message,omitempty"`
}

// Condition represents the status of a specific reconciliation step.
type Condition struct {
	// Type identifies the condition (e.g., "DNSRecordCreated", "CertificateValid", "IngressConfigured")
	Type string `json:"type"`

	// Status is "True", "False", or "Unknown"
	Status string `json:"status"`

	// LastTransitionTime is when this condition last changed.
	LastTransitionTime time.Time `json:"last_transition_time"`

	// Reason is a machine-readable reason code (e.g., "RecordCreated", "DNSPropagationTimeout")
	Reason string `json:"reason,omitempty"`

	// Message is a human-readable message.
	Message string `json:"message,omitempty"`
}

// Validate checks if the spec is valid and returns an error if not.
func (s *ExternalDomainSpec) Validate() error {
	if s.FQDN == "" {
		return fmt.Errorf("fqdn is required")
	}
	if s.Zone == "" {
		return fmt.Errorf("zone is required")
	}

	// Validate FQDN matches zone
	if !strings.HasSuffix(s.FQDN, "."+s.Zone) && s.FQDN != s.Zone {
		return fmt.Errorf("fqdn %q is not a subdomain of zone %q", s.FQDN, s.Zone)
	}

	if s.NodeID == "" {
		return fmt.Errorf("node_id is required")
	}
	if s.TargetIP == "" {
		return fmt.Errorf("target_ip is required (use \"auto\" for auto-detection)")
	}
	if s.ProviderRef == "" {
		return fmt.Errorf("provider_ref is required")
	}

	// Validate ACME config
	if s.ACME.Enabled {
		if s.ACME.Email == "" {
			return fmt.Errorf("acme.email is required when acme is enabled")
		}
		if s.ACME.ChallengeType == "" {
			s.ACME.ChallengeType = "dns-01" // default
		}
		if s.ACME.ChallengeType != "dns-01" {
			return fmt.Errorf("acme.challenge_type must be \"dns-01\" (http-01 not supported yet)")
		}
	}

	// Validate ingress config
	if s.Ingress.Enabled {
		if s.Ingress.Service == "" {
			s.Ingress.Service = "gateway" // default
		}
		if s.Ingress.Port == 0 {
			s.Ingress.Port = 443 // default
		}
	}

	// Default TTL
	if s.TTL == 0 {
		s.TTL = 600 // 10 minutes
	}

	return nil
}

// RelativeName returns the relative DNS name (without the zone).
// Example: FQDN "globule-ryzen.globular.cloud" → "globule-ryzen"
func (s *ExternalDomainSpec) RelativeName() string {
	if s.FQDN == s.Zone {
		return "@" // apex record
	}
	return strings.TrimSuffix(s.FQDN, "."+s.Zone)
}

// ToJSON serializes the spec to JSON.
func (s *ExternalDomainSpec) ToJSON() ([]byte, error) {
	return json.Marshal(s)
}

// FromJSON deserializes a spec from JSON.
func FromJSON(data []byte) (*ExternalDomainSpec, error) {
	var spec ExternalDomainSpec
	if err := json.Unmarshal(data, &spec); err != nil {
		return nil, err
	}
	return &spec, nil
}

// etcd key helpers

const (
	// EtcdDomainPrefix is the etcd prefix for external domain specs.
	EtcdDomainPrefix = "/globular/domains/v1/"

	// EtcdProviderPrefix is the etcd prefix for DNS provider configs.
	EtcdProviderPrefix = "/globular/providers/v1/"
)

// DomainKey returns the etcd key for a given FQDN.
func DomainKey(fqdn string) string {
	return EtcdDomainPrefix + fqdn
}

// ProviderKey returns the etcd key for a given provider name.
func ProviderKey(name string) string {
	return EtcdProviderPrefix + name
}
