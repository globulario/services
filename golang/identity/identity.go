// Package identity provides canonical service identity mapping for Globular services.
//
// The canonical key is kebab-case (e.g., "node-agent", "cluster-controller").
// All other representations (gRPC FQN, binary name, systemd unit) derive from it.
//
// Usage:
//
//	key, ok := NormalizeServiceKey("node_agent.NodeAgentService") // "node-agent", true
//	id, ok  := IdentityByKey("node-agent")
//	unit    := id.UnitName  // "globular-node-agent.service"
package identity

import "strings"

// ServiceIdentity holds all known representations of one service.
type ServiceIdentity struct {
	// Key is the canonical kebab-case name (bundle/spec name).
	Key string
	// BundleName is the repository bundle name (usually == Key).
	BundleName string
	// UnitName is the systemd unit (e.g., "globular-node-agent.service").
	UnitName string
	// GrpcFull is the fully-qualified gRPC service name (e.g., "node_agent.NodeAgentService").
	GrpcFull string
	// Binary is the server binary name (e.g., "node_agent_server").
	Binary string
	// Aliases are additional accepted input strings (lower-cased, matched before fallback).
	Aliases []string
}

// registry maps canonical key → ServiceIdentity.
var registry = func() map[string]ServiceIdentity {
	entries := []ServiceIdentity{
		{
			Key:        "node-agent",
			BundleName: "node-agent",
			UnitName:   "globular-node-agent.service",
			GrpcFull:   "node_agent.NodeAgentService",
			Binary:     "node_agent_server",
			Aliases:    []string{"nodeagent", "node_agent", "globular-node-agent", "globular-node-agent.service", "node_agent.nodeagentservice"},
		},
		{
			Key:        "cluster-controller",
			BundleName: "cluster-controller",
			UnitName:   "globular-cluster-controller.service",
			GrpcFull:   "cluster_controller.ClusterControllerService",
			Binary:     "cluster_controller_server",
			Aliases:    []string{"clustercontroller", "cluster_controller", "globular-cluster-controller", "globular-cluster-controller.service", "cluster_controller.clustercontrollerservice"},
		},
		{
			Key:        "dns",
			BundleName: "dns",
			UnitName:   "globular-dns.service",
			GrpcFull:   "dns.DnsService",
			Binary:     "dns_server",
			Aliases:    []string{"globular-dns", "globular-dns.service", "dns.dnsservice"},
		},
		{
			Key:        "discovery",
			BundleName: "discovery",
			UnitName:   "globular-discovery.service",
			GrpcFull:   "discovery.DiscoveryService",
			Binary:     "discovery_server",
			Aliases:    []string{"globular-discovery", "globular-discovery.service", "discovery.discoveryservice"},
		},
		{
			Key:        "file",
			BundleName: "file",
			UnitName:   "globular-file.service",
			GrpcFull:   "file.FileService",
			Binary:     "file_server",
			Aliases:    []string{"globular-file", "globular-file.service", "file.fileservice"},
		},
		{
			Key:        "event",
			BundleName: "event",
			UnitName:   "globular-event.service",
			GrpcFull:   "event.EventService",
			Binary:     "event_server",
			Aliases:    []string{"globular-event", "globular-event.service", "event.eventservice"},
		},
		{
			Key:        "rbac",
			BundleName: "rbac",
			UnitName:   "globular-rbac.service",
			GrpcFull:   "rbac.RbacService",
			Binary:     "rbac_server",
			Aliases:    []string{"globular-rbac", "globular-rbac.service", "rbac.rbacservice"},
		},
		{
			Key:        "resource",
			BundleName: "resource",
			UnitName:   "globular-resource.service",
			GrpcFull:   "resource.ResourceService",
			Binary:     "resource_server",
			Aliases:    []string{"globular-resource", "globular-resource.service", "resource.resourceservice"},
		},
		{
			Key:        "repository",
			BundleName: "repository",
			UnitName:   "globular-repository.service",
			GrpcFull:   "repository.PackageRepository",
			Binary:     "repository_server",
			Aliases:    []string{"globular-repository", "globular-repository.service", "repository.packagerepository"},
		},
		{
			Key:        "persistence",
			BundleName: "persistence",
			UnitName:   "globular-persistence.service",
			GrpcFull:   "persistence.PersistenceService",
			Binary:     "persistence_server",
			Aliases:    []string{"globular-persistence", "globular-persistence.service", "persistence.persistenceservice"},
		},
		{
			Key:        "media",
			BundleName: "media",
			UnitName:   "globular-media.service",
			GrpcFull:   "media.MediaService",
			Binary:     "media_server",
			Aliases:    []string{"globular-media", "globular-media.service", "media.mediaservice"},
		},
		{
			Key:        "title",
			BundleName: "title",
			UnitName:   "globular-title.service",
			GrpcFull:   "title.TitleService",
			Binary:     "title_server",
			Aliases:    []string{"globular-title", "globular-title.service", "title.titleservice"},
		},
		{
			Key:        "authentication",
			BundleName: "authentication",
			UnitName:   "globular-authentication.service",
			GrpcFull:   "authentication.AuthenticationService",
			Binary:     "authentication_server",
			Aliases:    []string{"globular-authentication", "globular-authentication.service", "authentication.authenticationservice"},
		},
		{
			Key:        "ca",
			BundleName: "ca",
			UnitName:   "globular-ca.service",
			GrpcFull:   "ca.CertificateAuthorityService",
			Binary:     "ca_server",
			Aliases:    []string{"globular-ca", "globular-ca.service", "ca.certificateauthorityservice"},
		},
		{
			Key:        "applications",
			BundleName: "applications",
			UnitName:   "globular-applications.service",
			GrpcFull:   "applications.ApplicationsService",
			Binary:     "applications_server",
			Aliases:    []string{"globular-applications", "globular-applications.service", "applications.applicationsservice"},
		},
		{
			Key:        "torrent",
			BundleName: "torrent",
			UnitName:   "globular-torrent.service",
			GrpcFull:   "torrent.TorrentService",
			Binary:     "torrent_server",
			Aliases:    []string{"globular-torrent", "globular-torrent.service", "torrent.torrentservice"},
		},
		{
			Key:        "blog",
			BundleName: "blog",
			UnitName:   "globular-blog.service",
			GrpcFull:   "blog.BlogService",
			Binary:     "blog_server",
			Aliases:    []string{"globular-blog", "globular-blog.service", "blog.blogservice"},
		},
		{
			Key:        "conversation",
			BundleName: "conversation",
			UnitName:   "globular-conversation.service",
			GrpcFull:   "conversation.ConversationService",
			Binary:     "conversation_server",
			Aliases:    []string{"globular-conversation", "globular-conversation.service", "conversation.conversationservice"},
		},
		{
			Key:        "minio",
			BundleName: "minio",
			UnitName:   "globular-minio.service",
			GrpcFull:   "minio",
			Binary:     "minio",
			Aliases:    []string{"globular-minio", "globular-minio.service"},
		},
		{
			Key:        "etcd",
			BundleName: "etcd",
			UnitName:   "globular-etcd.service",
			GrpcFull:   "etcd",
			Binary:     "etcd",
			Aliases:    []string{"globular-etcd", "globular-etcd.service"},
		},
		{
			// Bundle name is "globular-gateway" but canonical key strips "globular-" prefix
			// to match the historical canonicalServiceName("globular-gateway") = "gateway".
			Key:        "gateway",
			BundleName: "globular-gateway",
			UnitName:   "globular-gateway.service",
			GrpcFull:   "globular-gateway",
			Binary:     "gateway", // deployed binary name (build script renames from gateway_server)
			Aliases:    []string{"globular-gateway", "globular-gateway.service"},
		},
		{
			// Bundle name is "globular-xds" but canonical key strips "globular-" prefix.
			Key:        "xds",
			BundleName: "globular-xds",
			UnitName:   "globular-xds.service",
			GrpcFull:   "globular-xds",
			Binary:     "xds", // deployed binary name (build script renames from xds_server)
			Aliases:    []string{"globular-xds", "globular-xds.service"},
		},
		{
			// Globular packages envoy as globular-envoy.service (not the OS system envoy.service).
			Key:        "envoy",
			BundleName: "envoy",
			UnitName:   "globular-envoy.service",
			GrpcFull:   "envoy",
			Binary:     "envoy",
			Aliases:    []string{"envoy.service"}, // envoy.service = system envoy alias
		},
	}

	m := make(map[string]ServiceIdentity, len(entries))
	for _, e := range entries {
		m[e.Key] = e
	}
	return m
}()

// aliasMap maps every lower-cased alias/grpc/binary/unit back to a canonical Key.
var aliasMap = func() map[string]string {
	m := make(map[string]string)
	for key, id := range registry {
		// key itself
		m[strings.ToLower(key)] = key
		// explicit aliases
		for _, a := range id.Aliases {
			m[strings.ToLower(a)] = key
		}
		// GrpcFull (lower-cased)
		if id.GrpcFull != "" {
			m[strings.ToLower(id.GrpcFull)] = key
		}
		// Binary (lower-cased)
		if id.Binary != "" {
			m[strings.ToLower(id.Binary)] = key
		}
		// UnitName (lower-cased)
		if id.UnitName != "" {
			m[strings.ToLower(id.UnitName)] = key
		}
	}
	return m
}()

// NormalizeServiceKey converts any known service representation to the canonical kebab-case key.
//
// Accepts: bundle names, gRPC FQNs, systemd unit names, binary names, and aliases.
// Returns ("", false) for unknown inputs.
//
// Examples:
//
//	NormalizeServiceKey("node-agent")                → ("node-agent", true)
//	NormalizeServiceKey("node_agent.NodeAgentService") → ("node-agent", true)
//	NormalizeServiceKey("globular-node-agent.service")→ ("node-agent", true)
//	NormalizeServiceKey("node_agent_server")           → ("node-agent", true)
func NormalizeServiceKey(input string) (string, bool) {
	if input == "" {
		return "", false
	}
	norm := strings.ToLower(strings.TrimSpace(input))
	if key, ok := aliasMap[norm]; ok {
		return key, true
	}
	// Fallback: strip "globular-" prefix and ".service" suffix (handles unknown services gracefully).
	stripped := norm
	stripped = strings.TrimPrefix(stripped, "globular-")
	stripped = strings.TrimSuffix(stripped, ".service")
	if stripped != norm {
		if key, ok := aliasMap[stripped]; ok {
			return key, true
		}
		// Still unknown but at least canonicalized to stripped form.
		return stripped, true
	}
	// Strip gRPC package prefix (e.g., "foo.FooService" → "foo") and try.
	if dot := strings.LastIndex(norm, "."); dot > 0 {
		pkg := norm[:dot]
		if key, ok := aliasMap[pkg]; ok {
			return key, true
		}
	}
	// Return the input lowercased and stripped as best-effort.
	return norm, false
}

// IdentityByKey returns the ServiceIdentity for a canonical key.
func IdentityByKey(key string) (ServiceIdentity, bool) {
	id, ok := registry[key]
	return id, ok
}

// MustIdentityByKey returns the ServiceIdentity for a canonical key, or a synthesized
// fallback if the key is not in the registry (useful for unknown/dynamic services).
func MustIdentityByKey(key string) ServiceIdentity {
	if id, ok := registry[key]; ok {
		return id
	}
	// Synthesize a best-effort identity for unregistered services.
	unit := "globular-" + key + ".service"
	return ServiceIdentity{
		Key:        key,
		BundleName: key,
		UnitName:   unit,
	}
}

// UnitForService returns the systemd unit name for any service identifier.
// Accepts bundle names, gRPC FQNs, unit names, or binary names.
func UnitForService(serviceName string) string {
	if serviceName == "" {
		return ""
	}
	key, _ := NormalizeServiceKey(serviceName)
	if key == "" {
		return ""
	}
	return MustIdentityByKey(key).UnitName
}

// All returns a copy of the full registry (keyed by canonical key).
func All() map[string]ServiceIdentity {
	out := make(map[string]ServiceIdentity, len(registry))
	for k, v := range registry {
		out[k] = v
	}
	return out
}
