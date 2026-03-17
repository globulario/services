package policy

import "time"

// AuthzMode controls runtime authorization behavior.
type AuthzMode string

const (
	// AuthzModeRbacStrict is production mode: RBAC gRPC is authoritative.
	// If RBAC is unavailable, protected operations fail closed.
	AuthzModeRbacStrict AuthzMode = "RbacStrict"

	// AuthzModeBootstrap is Day-0 mode: allows fallback to local manifest
	// when RBAC is not yet available. Logs loudly.
	AuthzModeBootstrap AuthzMode = "Bootstrap"

	// AuthzModeDevelopment is dev mode: RBAC not required, local only.
	AuthzModeDevelopment AuthzMode = "Development"
)

// ServiceAuthzRegistration is the authorization metadata published to etcd
// as part of service registration. Enables Globular to transparently manage
// and inspect service authorization state across the cluster.
type ServiceAuthzRegistration struct {
	// Service identity
	ServiceName    string `json:"service_name"`
	ServiceVersion string `json:"service_version,omitempty"`
	NodeID         string `json:"node_id,omitempty"`

	// Network
	GrpcAddress string `json:"grpc_address,omitempty"`
	HttpAddress string `json:"http_address,omitempty"`

	// Authorization state
	AuthzMode      AuthzMode `json:"authz_mode"`
	RbacActive     bool      `json:"rbac_active"`
	FallbackActive bool      `json:"fallback_active"`
	ManifestsLoaded bool     `json:"manifests_loaded"`

	// Manifest metadata
	PermissionsSource    string `json:"permissions_source,omitempty"`
	ManifestSchemaVersion string `json:"manifest_schema_version,omitempty"`
	PermissionCount      int    `json:"permission_count"`
	ActionMappingCount   int    `json:"action_mapping_count"`

	// Role seeding
	RolesSource    string `json:"roles_source,omitempty"`
	RoleSeedStatus string `json:"role_seed_status,omitempty"`

	// Timestamps
	RegisteredAt time.Time `json:"registered_at"`
}

// NewServiceAuthzRegistration creates a registration with defaults.
func NewServiceAuthzRegistration(serviceName string) *ServiceAuthzRegistration {
	return &ServiceAuthzRegistration{
		ServiceName:  serviceName,
		AuthzMode:    AuthzModeRbacStrict,
		RegisteredAt: time.Now().UTC(),
	}
}
