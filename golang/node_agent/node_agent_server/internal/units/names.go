package units

import "github.com/globulario/services/golang/identity"

// UnitForService returns the expected systemd unit name for a service identifier.
// Accepts bundle names, gRPC FQNs, systemd unit names, and binary names.
func UnitForService(serviceName string) string {
	return identity.UnitForService(serviceName)
}
