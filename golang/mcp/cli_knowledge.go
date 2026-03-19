package main

// ── CLI Knowledge Base ───────────────────────────────────────────────────────
// Embedded knowledge base for the Globular CLI. This allows the MCP server
// to serve structured help without external files.

// CLICommand describes a CLI command for the help system.
type CLICommand struct {
	Path        string     `json:"path"`
	Description string     `json:"description"`
	Long        string     `json:"long,omitempty"`
	Flags       []CLIFlag  `json:"flags,omitempty"`
	Examples    []string   `json:"examples,omitempty"`
	Rules       []string   `json:"rules,omitempty"`
	FollowUp    []string   `json:"follow_up,omitempty"`
}

// CLIFlag describes a command flag.
type CLIFlag struct {
	Name     string   `json:"name"`
	Type     string   `json:"type"`
	Required bool     `json:"required"`
	Default  string   `json:"default,omitempty"`
	Allowed  []string `json:"allowed,omitempty"`
	Help     string   `json:"help"`
}

// CLIWorkflow describes a multi-step task workflow.
type CLIWorkflow struct {
	Task        string         `json:"task"`
	Description string         `json:"description"`
	Steps       []WorkflowStep `json:"steps"`
}

// WorkflowStep describes a single step in a workflow.
type WorkflowStep struct {
	Order       int    `json:"order"`
	Action      string `json:"action"`
	Command     string `json:"command,omitempty"`
	Description string `json:"description"`
	Expected    string `json:"expected,omitempty"`
}

// CLIRule is a core rule for AI agents operating the CLI.
type CLIRule struct {
	ID          string `json:"id"`
	Rule        string `json:"rule"`
	Scope       string `json:"scope,omitempty"`
	Explanation string `json:"explanation,omitempty"`
}

// CLIExample is a validated CLI example.
type CLIExample struct {
	Command     string `json:"command"`
	Description string `json:"description"`
	Category    string `json:"category"`
}

// ── Command Registry ─────────────────────────────────────────────────────────

var cliCommands = map[string]CLICommand{
	// Generate commands
	"generate": {
		Path:        "generate",
		Description: "Generate service scaffolding from proto files",
		Long:        "Generate Globular-compliant service code from .proto definitions. Proto is the source of truth.",
		FollowUp:    []string{"generate inspect", "generate service", "generate all"},
	},
	"generate inspect": {
		Path:        "generate inspect",
		Description: "Inspect a proto file and display its structure",
		Long:        "Parse a .proto file and display services, RPCs, messages, and authorization annotations. Use as dry-run prerequisite before generation.",
		Flags: []CLIFlag{
			{Name: "proto", Type: "string", Required: true, Help: "path to .proto file"},
			{Name: "json", Type: "bool", Required: false, Default: "false", Help: "output as JSON"},
		},
		Examples: []string{
			"globular generate inspect --proto proto/catalog.proto",
			"globular generate inspect --proto proto/catalog.proto --json",
		},
		Rules:    []string{"Always inspect proto before generating", "Verify service and RPC names match expectations"},
		FollowUp: []string{"generate service", "generate all"},
	},
	"generate service": {
		Path:        "generate service",
		Description: "Generate server scaffolding only (server.go, config.go, handlers.go)",
		Flags: []CLIFlag{
			{Name: "proto", Type: "string", Required: true, Help: "path to .proto file"},
			{Name: "lang", Type: "string", Required: false, Default: "go", Allowed: []string{"go", "csharp"}, Help: "target language"},
			{Name: "out", Type: "string", Required: false, Default: ".", Help: "output directory"},
			{Name: "store", Type: "string", Required: false, Default: "none", Allowed: []string{"none", "sqlite", "postgres", "mongodb"}, Help: "storage backend"},
			{Name: "port", Type: "int", Required: false, Default: "0", Help: "default gRPC port (auto-allocated if 0)"},
			{Name: "dry-run", Type: "bool", Required: false, Default: "false", Help: "print file list without writing"},
		},
		Examples: []string{
			"globular generate service --proto proto/echo.proto --lang go --out ./golang/echo",
			"globular generate service --proto proto/catalog.proto --lang csharp --out ./csharp/catalog",
			"globular generate service --proto proto/echo.proto --lang go --out ./golang/echo --dry-run",
		},
		Rules:    []string{"Never edit generated files", "Business logic goes in handlers.go only"},
		FollowUp: []string{"go build ./...", "generate all"},
	},
	"generate all": {
		Path:        "generate all",
		Description: "Generate full service: server + client + tests",
		Flags: []CLIFlag{
			{Name: "proto", Type: "string", Required: true, Help: "path to .proto file"},
			{Name: "lang", Type: "string", Required: false, Default: "go", Allowed: []string{"go", "csharp"}, Help: "target language"},
			{Name: "out", Type: "string", Required: false, Default: ".", Help: "output directory"},
			{Name: "store", Type: "string", Required: false, Default: "none", Allowed: []string{"none", "sqlite", "postgres", "mongodb"}, Help: "storage backend"},
			{Name: "persistence", Type: "string", Required: false, Default: "native", Allowed: []string{"native", "ef"}, Help: "persistence style (ef is C# only)"},
			{Name: "port", Type: "int", Required: false, Default: "0", Help: "default gRPC port (auto-allocated if 0)"},
			{Name: "dry-run", Type: "bool", Required: false, Default: "false", Help: "print file list without writing"},
		},
		Examples: []string{
			"globular generate all --proto proto/echo.proto --lang go --out ./golang/echo",
			"globular generate all --proto proto/catalog.proto --lang csharp --out ./csharp/catalog --store postgres --persistence ef",
			"globular generate all --proto proto/market.proto --lang go --store none --out ./golang/market --dry-run",
		},
		Rules:    []string{"Never edit generated files", "EF is persistence only, not architecture", "Proto is the source of truth"},
		FollowUp: []string{"go build ./...", "go test ./...", "globular pkg build"},
	},
	// Cluster commands
	"cluster bootstrap": {
		Path:        "cluster bootstrap",
		Description: "Initialize the first node of a new cluster",
		Flags: []CLIFlag{
			{Name: "domain", Type: "string", Required: true, Help: "cluster domain (e.g. globular.internal)"},
		},
		Examples: []string{"globular cluster bootstrap --domain globular.internal"},
		Rules:    []string{"Run only once per cluster", "Requires root access"},
		FollowUp: []string{"cluster token create", "cluster join"},
	},
	"cluster join": {
		Path:        "cluster join",
		Description: "Add a node to an existing cluster",
		Flags: []CLIFlag{
			{Name: "token", Type: "string", Required: true, Help: "join token from cluster token create"},
			{Name: "controller", Type: "string", Required: true, Help: "controller address"},
		},
		Examples: []string{"globular cluster join --token <token> --controller host:12000"},
		FollowUp: []string{"cluster list-nodes"},
	},
	"cluster token create": {
		Path:        "cluster token create",
		Description: "Create a join token for adding nodes",
		Examples:    []string{"globular cluster token create"},
		FollowUp:    []string{"cluster join"},
	},
	// Package commands
	"pkg build": {
		Path:        "pkg build",
		Description: "Build service packages from payload roots",
		Flags: []CLIFlag{
			{Name: "spec", Type: "string", Required: false, Help: "path to a single spec YAML"},
			{Name: "spec-dir", Type: "string", Required: false, Help: "directory containing spec YAMLs"},
			{Name: "version", Type: "string", Required: false, Help: "package version"},
			{Name: "root", Type: "string", Required: false, Help: "payload root containing bin/ and config/"},
		},
		Examples: []string{
			"globular pkg build --spec packages/specs/echo.yaml --version 1.0.0",
			"globular pkg build --spec-dir packages/specs/ --version 1.0.0",
		},
		Rules:    []string{"Set either --spec or --spec-dir, not both"},
		FollowUp: []string{"pkg publish"},
	},
	"pkg publish": {
		Path:        "pkg publish",
		Description: "Publish a package to the repository service",
		Flags: []CLIFlag{
			{Name: "file", Type: "string", Required: true, Help: "path to a package .tgz"},
			{Name: "repository", Type: "string", Required: true, Help: "repository service address"},
		},
		Examples: []string{"globular pkg publish --file echo-1.0.0.tgz --repository localhost:10200"},
		FollowUp: []string{"services desired set"},
	},
	// Services commands
	"services desired set": {
		Path:        "services desired set",
		Description: "Set the desired release for a service",
		Examples:    []string{"globular services desired set --name echo --version 1.0.0"},
		FollowUp:    []string{"services repair"},
	},
	"services repair": {
		Path:        "services repair",
		Description: "Reconcile installed services with desired state",
		Flags: []CLIFlag{
			{Name: "dry-run", Type: "bool", Required: false, Default: "false", Help: "show what would change without applying"},
		},
		Examples: []string{
			"globular services repair --dry-run",
			"globular services repair",
		},
	},
	// DNS commands
	"dns a set": {
		Path:        "dns a set",
		Description: "Create or update a DNS A record",
		Flags: []CLIFlag{
			{Name: "name", Type: "string", Required: true, Help: "record name (must end with trailing dot)"},
			{Name: "ip", Type: "string", Required: true, Help: "IP address"},
		},
		Examples: []string{"globular dns a set --name host.globular.internal. --ip 10.0.0.1"},
		Rules:    []string{"DNS records MUST have trailing dot"},
	},
}

// ── Workflow Registry ────────────────────────────────────────────────────────

var cliWorkflows = map[string]CLIWorkflow{
	"create_service": {
		Task:        "create_service",
		Description: "Create a new Globular service from scratch",
		Steps: []WorkflowStep{
			{Order: 1, Action: "call_mcp", Command: "globular_cli.rules", Description: "Retrieve core rules to follow"},
			{Order: 2, Action: "propose", Description: "Propose service name, purpose, language, storage, and proto definition. Wait for user approval."},
			{Order: 3, Action: "run_command", Command: "globular generate inspect --proto <file> --json", Description: "Inspect proto to verify structure", Expected: "JSON output with services, RPCs, messages"},
			{Order: 4, Action: "call_mcp", Command: "globular_cli.help {\"command_path\": \"generate all\"}", Description: "Get exact flags and examples for generation"},
			{Order: 5, Action: "run_command", Command: "globular generate all --proto <file> --lang <lang> --out <dir>", Description: "Generate the service scaffolding", Expected: "List of generated files"},
			{Order: 6, Action: "implement", Description: "Implement business logic in handwritten files only (e.g. handlers.go). Do NOT edit generated files."},
			{Order: 7, Action: "run_command", Command: "go build ./... && go test ./...", Description: "Build and validate", Expected: "No errors"},
			{Order: 8, Action: "wait_approval", Description: "Confirm with user before publishing"},
			{Order: 9, Action: "run_command", Command: "globular pkg build --spec <spec>", Description: "Build the package"},
			{Order: 10, Action: "run_command", Command: "globular pkg publish --file <tgz> --repository <addr>", Description: "Publish to repository"},
			{Order: 11, Action: "observe", Description: "Use MCP tools (cluster_get_health, nodeagent_get_inventory) to verify deployment"},
		},
	},
	"publish_package": {
		Task:        "publish_package",
		Description: "Build and publish a service package",
		Steps: []WorkflowStep{
			{Order: 1, Action: "run_command", Command: "go build ./...", Description: "Ensure code compiles"},
			{Order: 2, Action: "run_command", Command: "go test ./...", Description: "Run tests"},
			{Order: 3, Action: "run_command", Command: "globular pkg build --spec <spec> --version <ver>", Description: "Build the package"},
			{Order: 4, Action: "run_command", Command: "globular pkg publish --file <tgz> --repository <addr>", Description: "Publish to repository"},
			{Order: 5, Action: "run_command", Command: "globular services desired set --name <name> --version <ver>", Description: "Set desired release"},
			{Order: 6, Action: "run_command", Command: "globular services repair", Description: "Trigger reconciliation"},
		},
	},
	"bootstrap_cluster": {
		Task:        "bootstrap_cluster",
		Description: "Initialize a new Globular cluster",
		Steps: []WorkflowStep{
			{Order: 1, Action: "run_command", Command: "globular cluster bootstrap --domain <domain>", Description: "Initialize first node", Expected: "Cluster ID and join token"},
			{Order: 2, Action: "run_command", Command: "globular cluster token create", Description: "Create join token for additional nodes"},
			{Order: 3, Action: "run_command", Command: "globular cluster join --token <token> --controller <addr>", Description: "Add worker nodes (repeat per node)"},
			{Order: 4, Action: "observe", Command: "cluster_get_health", Description: "Verify cluster health via MCP"},
		},
	},
}

// ── Rules Registry ───────────────────────────────────────────────────────────

var cliRules = []CLIRule{
	{ID: "proto_is_truth", Rule: "Proto is the source of truth for service contracts", Scope: "generation", Explanation: "All service definitions, RPC signatures, and message types are defined in .proto files. Generated code reflects proto. Handwritten code implements behavior."},
	{ID: "use_cli", Rule: "Use CLI for code generation, not manual scaffolding", Scope: "generation", Explanation: "Never create service structure manually if the CLI generate command exists. This ensures consistency across all services."},
	{ID: "no_edit_generated", Rule: "Never edit generated files manually", Scope: "generation", Explanation: "Generated files have a 'Code generated by globular-cli; DO NOT EDIT' header. They can be regenerated at any time. Business logic goes in handwritten files."},
	{ID: "ef_persistence_only", Rule: "EF is a persistence layer only, not an architecture", Scope: "csharp", Explanation: "When using Entity Framework in C# services, it should be used strictly as a data access layer. It does not define the service architecture."},
	{ID: "propose_before_execute", Rule: "Propose before executing", Scope: "workflow", Explanation: "AI must propose service name, language, storage, and proto definition before generating. Wait for explicit user approval before executing."},
	{ID: "check_help_first", Rule: "Always check help before using a command", Scope: "safety", Explanation: "Use globular_cli.help to verify flags, allowed values, and examples before running any CLI command. Never invent commands."},
	{ID: "no_hallucinate_commands", Rule: "Only use commands returned by the help system", Scope: "safety", Explanation: "Never fabricate CLI commands or flags. If a command is not in the help system, it does not exist."},
	{ID: "confirm_destructive", Rule: "Never execute destructive commands without explicit confirmation", Scope: "safety", Explanation: "Commands that modify cluster state, delete data, or deploy to production require explicit user approval."},
	{ID: "dns_trailing_dot", Rule: "DNS records must have trailing dot", Scope: "dns", Explanation: "All DNS record names must end with a trailing dot (e.g. hostname.globular.internal.)"},
	{ID: "global_flags_first", Rule: "Global CLI flags must come BEFORE subcommands", Scope: "usage", Explanation: "Flags like --timeout, --controller must precede the subcommand: globular --timeout 10s dns a set ..."},
}

// ── Examples Registry ────────────────────────────────────────────────────────

var cliExamples = []CLIExample{
	// Generate
	{Command: "globular generate inspect --proto proto/catalog.proto --json", Description: "Inspect a proto file as JSON", Category: "generate"},
	{Command: "globular generate service --proto proto/echo.proto --lang go --out ./golang/echo", Description: "Generate Go server scaffolding", Category: "generate"},
	{Command: "globular generate all --proto proto/echo.proto --lang go --out ./golang/echo", Description: "Generate full Go service (server + client + tests)", Category: "generate"},
	{Command: "globular generate all --proto proto/catalog.proto --lang csharp --out ./csharp/catalog --store postgres --persistence ef", Description: "Generate C# service with EF persistence", Category: "generate"},
	{Command: "globular generate all --proto proto/market.proto --lang go --store none --out ./golang/market --dry-run", Description: "Preview file list without writing", Category: "generate"},

	// Package
	{Command: "globular pkg build --spec packages/specs/echo.yaml --version 1.0.0", Description: "Build a single service package", Category: "pkg"},
	{Command: "globular pkg build --spec-dir packages/specs/ --version 1.0.0", Description: "Build all service packages", Category: "pkg"},
	{Command: "globular pkg publish --file echo-1.0.0.tgz --repository localhost:10200", Description: "Publish a package", Category: "pkg"},

	// Cluster
	{Command: "globular cluster bootstrap --domain globular.internal", Description: "Initialize first cluster node", Category: "cluster"},
	{Command: "globular cluster token create", Description: "Create a join token", Category: "cluster"},
	{Command: "globular cluster join --token <token> --controller host:12000", Description: "Join a node to the cluster", Category: "cluster"},

	// Services
	{Command: "globular services repair --dry-run", Description: "Preview reconciliation changes", Category: "services"},
	{Command: "globular services repair", Description: "Reconcile installed services with desired state", Category: "services"},

	// DNS
	{Command: "globular --timeout 10s dns a set --name host.globular.internal. --ip 10.0.0.1", Description: "Set DNS A record (note: global flags before subcommand)", Category: "dns"},
}

// ── Lookup Functions ─────────────────────────────────────────────────────────

func lookupCommand(path string) (CLICommand, bool) {
	cmd, ok := cliCommands[path]
	return cmd, ok
}

func lookupWorkflow(task string) (CLIWorkflow, bool) {
	wf, ok := cliWorkflows[task]
	return wf, ok
}

func filterExamples(category string) []CLIExample {
	if category == "" {
		return cliExamples
	}
	var filtered []CLIExample
	for _, ex := range cliExamples {
		if ex.Category == category {
			filtered = append(filtered, ex)
		}
	}
	return filtered
}

func filterRules(scope string) []CLIRule {
	if scope == "" {
		return cliRules
	}
	var filtered []CLIRule
	for _, r := range cliRules {
		if r.Scope == scope || r.Scope == "" {
			filtered = append(filtered, r)
		}
	}
	return filtered
}
