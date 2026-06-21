// @awareness namespace=globular.platform
// @awareness component=platform_mcp.register
// @awareness file_role=tool_group_registration_dispatcher_gated_by_mcpconfig_toolgroups
// @awareness implements=globular.platform:intent.awareness.mcp_bridge_exposes_safe_tools_only
// @awareness risk=high
package main

// register.go — single dispatch point that registers every tool
// group. Each group is gated by an MCPConfig.ToolGroups flag so
// operators can ship a minimal MCP surface in restricted
// environments. Adding a new tool group means adding a flag here
// AND in MCPConfig — silently enabling a group bypasses operator
// intent.
//
// Tool registration is finalized at server startup; there is no
// runtime add path (see server.go). An agent cannot enable a
// tool group that the operator disabled.

func registerAllTools(s *server) {
	g := s.cfg.ToolGroups
	if g.Cluster {
		registerClusterTools(s)
		registerConvergenceTools(s)
		registerReleaseTools(s)
	}
	if g.Doctor {
		registerDoctorTools(s)
		registerInfraTools(s)
	}
	if g.NodeAgent {
		registerNodeAgentTools(s)
		registerServiceConfigTools(s)
	}
	if g.Repository {
		registerRepositoryTools(s)
		registerUpstreamTools(s)
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
	if g.Behavioral {
		registerBehavioralTools(s)
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
	if g.AIExecutor {
		registerAIExecutorTools(s)
	}
	if g.Aggregator {
		registerAggregatorTools(s)
	}
	// Always-available tools.
	registerLogRingTools(s)
	registerPackageTools(s)
	registerClusterConfigTools(s)
	registerSchemaTools(s)
	// Auth and DNS deferred to phase 2.
}
