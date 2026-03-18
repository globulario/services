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
	// Auth and DNS deferred to phase 2.
}
