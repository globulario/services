package main

func registerAllTools(s *server) {
	g := s.cfg.ToolGroups
	if g.Cluster {
		registerClusterTools(s)
		registerConvergenceTools(s)
	}
	if g.Doctor {
		registerDoctorTools(s)
	}
	if g.NodeAgent {
		registerNodeAgentTools(s)
		registerServiceConfigTools(s)
	}
	if g.Repository {
		registerRepositoryTools(s)
	}
	if g.Backup {
		registerBackupTools(s)
	}
	if g.Composed {
		registerComposedTools(s)
	}
	if g.RBAC {
		registerRbacTools(s)
		registerRbacExplainTools(s)
	}
	if g.Resource {
		registerResourceTools(s)
	}
	if g.File {
		registerFileTools(s)
	}
	if g.Persistence {
		registerPersistenceTools(s)
	}
	if g.Storage {
		registerStorageTools(s)
	}
	if g.CLI {
		registerCLITools(s)
	}
	if g.Governor {
		registerGovernorTools(s)
	}
	if g.Governor {
		registerPlanTools(s)
	}
	if g.Memory {
		registerMemoryTools(s)
	}
	if g.Skills {
		registerSkillsTools(s)
	}
	if g.Workflow {
		registerWorkflowTools(s)
	}
	if g.Etcd {
		registerEtcdTools(s)
	}
	if g.Title {
		registerTitleTools(s)
	}
	if g.Frontend {
		registerFrontendTools(s)
	}
	if g.Proto {
		registerProtoTools(s)
		registerGrpcCallTools(s)
	}
	if g.HTTPDiag {
		registerHTTPDiagTools(s)
	}
	if g.Monitoring {
		registerMonitoringTools(s)
	}
	if g.Browser {
		registerBrowserTools(s)
	}
	// Log ring tools are always available (in-process ring buffer).
	registerLogRingTools(s)
	// Package lifecycle tools are always available.
	registerPackageTools(s)
	// Auth and DNS deferred to phase 2.
}
