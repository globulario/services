// @awareness namespace=globular.platform
// @awareness component=platform_node_agent
// @awareness file_role=systemd_unit_name_conventions
// @awareness risk=low
package units

import "github.com/globulario/services/golang/identity"

// UnitForService returns the expected systemd unit name for a service identifier.
// Accepts bundle names, gRPC FQNs, systemd unit names, and binary names.
func UnitForService(serviceName string) string {
	return identity.UnitForService(serviceName)
}
