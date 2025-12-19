package units

import "strings"

var knownServices = map[string]string{
	"dns.dnsservice":             "globular-dns.service",
	"discovery.discoveryservice": "globular-discovery.service",
	"file.fileservice":           "globular-file.service",
	"event.eventservice":         "globular-event.service",
	"rbac.rbacservice":           "globular-rbac.service",
	"minio":                      "globular-minio.service",
	"etcd":                       "globular-etcd.service",
	"globular-gateway":           "globular-gateway.service",
	"globular-xds":               "globular-xds.service",
	"envoy":                      "envoy.service",
}

// UnitForService returns the expected systemd unit name for a service identifier.
func UnitForService(serviceName string) string {
	if serviceName == "" {
		return ""
	}
	key := strings.ToLower(strings.TrimSpace(serviceName))
	if unit, ok := knownServices[key]; ok {
		return unit
	}
	return "globular-" + strings.ReplaceAll(key, ".", "-") + ".service"
}
