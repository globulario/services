package main

import "github.com/globulario/services/golang/ai_router/ai_routerpb"

// defaultClassifications returns the initial service class map.
// Per-service classification is intentionally conservative for mixed
// unary/stream services — classified by most sensitive behavior.
// Overridable via etcd: /globular/config/ai_router/service_classes
func defaultClassifications() map[string]ai_routerpb.ServiceClass {
	return map[string]ai_routerpb.ServiceClass{
		// STREAM_HEAVY: long-lived server-streaming RPCs, slow drain
		"event.EventService": ai_routerpb.ServiceClass_STREAM_HEAVY,
		"log.LogService":     ai_routerpb.ServiceClass_STREAM_HEAVY,

		// CONTROL_PLANE: must always be reachable, minimum weight enforced
		"clustercontroller.ClusterControllerService": ai_routerpb.ServiceClass_CONTROL_PLANE,
		"nodeagent.NodeAgentService":                 ai_routerpb.ServiceClass_CONTROL_PLANE,
		"discovery.DiscoveryService":                 ai_routerpb.ServiceClass_CONTROL_PLANE,
		"dns.DnsService":                             ai_routerpb.ServiceClass_CONTROL_PLANE,

		// DEPLOYMENT_SENSITIVE: needs warm-up after restart
		"repository.PackageRepository": ai_routerpb.ServiceClass_DEPLOYMENT_SENSITIVE,

		// STATELESS_UNARY: everything else — per-request balancing, fast drain
		"authentication.AuthenticationService": ai_routerpb.ServiceClass_STATELESS_UNARY,
		"rbac.RbacService":                     ai_routerpb.ServiceClass_STATELESS_UNARY,
		"resource.ResourceService":             ai_routerpb.ServiceClass_STATELESS_UNARY,
		"file.FileService":                     ai_routerpb.ServiceClass_STATELESS_UNARY,
		"media.MediaService":                   ai_routerpb.ServiceClass_STATELESS_UNARY,
		"search.SearchService":                 ai_routerpb.ServiceClass_STATELESS_UNARY,
		"persistence.PersistenceService":       ai_routerpb.ServiceClass_STATELESS_UNARY,
		"monitoring.MonitoringService":         ai_routerpb.ServiceClass_STATELESS_UNARY,
		"title.TitleService":                   ai_routerpb.ServiceClass_STATELESS_UNARY,
		"torrent.TorrentService":               ai_routerpb.ServiceClass_STATELESS_UNARY,
		"backup_manager.BackupManagerService":  ai_routerpb.ServiceClass_STATELESS_UNARY,
		"ai_memory.AiMemoryService":            ai_routerpb.ServiceClass_STATELESS_UNARY,
		"ai_watcher.AiWatcherService":          ai_routerpb.ServiceClass_STATELESS_UNARY,
		"ai_router.AiRouterService":            ai_routerpb.ServiceClass_STATELESS_UNARY,
	}
}
