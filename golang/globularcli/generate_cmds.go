package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/globulario/services/golang/globularcli/generate"
	"github.com/spf13/cobra"
)

// ── Flags ────────────────────────────────────────────────────────────────────

var (
	genProtoFile   string
	genLang        string
	genOutDir      string
	genStore       string
	genPersistence string
	genDryRun      bool
	genPort        int
	genJSON        bool
	genNoSpec      bool
)

// ── Commands ─────────────────────────────────────────────────────────────────

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate service scaffolding from proto files",
	Long: `Generate Globular-compliant service code from .proto definitions.

Proto is the source of truth. Generated code follows Globular conventions
and should not be edited manually. Business logic goes in handwritten files.`,
}

var generateInspectCmd = &cobra.Command{
	Use:   "inspect",
	Short: "Inspect a proto file and display its structure",
	Long: `Parse a .proto file and display its services, RPCs, messages, and
authorization annotations. Use --json for machine-readable output.

This is a dry-run prerequisite — verify the proto before generating code.`,
	Example: `  globular generate inspect --proto proto/catalog.proto
  globular generate inspect --proto proto/catalog.proto --json`,
	RunE: runGenerateInspect,
}

var generateServiceCmd = &cobra.Command{
	Use:   "service",
	Short: "Generate service scaffolding only (server.go, config.go, handlers.go)",
	Long: `Generate the server-side scaffolding for a Globular service.
Produces server.go, config.go, and handler stubs following Globular conventions.

Use --dry-run to preview the file list without writing.`,
	Example: `  globular generate service --proto proto/echo.proto --lang go --out ./golang/echo
  globular generate service --proto proto/catalog.proto --lang csharp --out ./csharp/catalog`,
	RunE: runGenerateService,
}

var generateSpecCmd = &cobra.Command{
	Use:   "spec",
	Short: "Generate package.yaml spec for an existing service",
	Long: `Generate only the package.yaml deployment spec for a Globular service.
Useful for adding a spec to a service that was created before spec generation
was available, or for regenerating a spec after proto changes.`,
	Example: `  globular generate spec --proto proto/echo.proto --lang go --out ./golang/echo
  globular generate spec --proto proto/echo.proto --lang go --out ./golang/echo --dry-run`,
	RunE: runGenerateSpec,
}

var generateAllCmd = &cobra.Command{
	Use:   "all",
	Short: "Generate full service: server + client + tests",
	Long: `Generate everything needed for a Globular service:
  - Server: server.go, config.go, handlers.go
  - Client: client wrapper with reconnect and TLS
  - Tests: basic test scaffolding

For Go: generates in <package>_server/ and <package>_client/ subdirectories.
For C#: generates Program.cs, .csproj, appsettings.json, manifest.json.`,
	Example: `  globular generate all --proto proto/echo.proto --lang go --out ./golang/echo
  globular generate all --proto proto/catalog.proto --lang csharp --out ./csharp/catalog --store postgres --persistence ef`,
	RunE: runGenerateAll,
}

func init() {
	// Inspect flags
	generateInspectCmd.Flags().StringVar(&genProtoFile, "proto", "", "path to .proto file (required)")
	generateInspectCmd.Flags().BoolVar(&genJSON, "json", false, "output as JSON")
	_ = generateInspectCmd.MarkFlagRequired("proto")

	// Service flags
	generateServiceCmd.Flags().StringVar(&genProtoFile, "proto", "", "path to .proto file (required)")
	generateServiceCmd.Flags().StringVar(&genLang, "lang", "go", "target language (go|csharp)")
	generateServiceCmd.Flags().StringVar(&genOutDir, "out", ".", "output directory")
	generateServiceCmd.Flags().StringVar(&genStore, "store", "none", "storage backend (none|sqlite|postgres|mongodb)")
	generateServiceCmd.Flags().IntVar(&genPort, "port", 0, "default gRPC port (auto-allocated if 0)")
	generateServiceCmd.Flags().BoolVar(&genDryRun, "dry-run", false, "print file list without writing")
	generateServiceCmd.Flags().BoolVar(&genNoSpec, "no-spec", false, "skip package.yaml spec generation")
	_ = generateServiceCmd.MarkFlagRequired("proto")

	// All flags
	generateAllCmd.Flags().StringVar(&genProtoFile, "proto", "", "path to .proto file (required)")
	generateAllCmd.Flags().StringVar(&genLang, "lang", "go", "target language (go|csharp)")
	generateAllCmd.Flags().StringVar(&genOutDir, "out", ".", "output directory")
	generateAllCmd.Flags().StringVar(&genStore, "store", "none", "storage backend (none|sqlite|postgres|mongodb)")
	generateAllCmd.Flags().StringVar(&genPersistence, "persistence", "native", "persistence style (native|ef)")
	generateAllCmd.Flags().IntVar(&genPort, "port", 0, "default gRPC port (auto-allocated if 0)")
	generateAllCmd.Flags().BoolVar(&genDryRun, "dry-run", false, "print file list without writing")
	generateAllCmd.Flags().BoolVar(&genNoSpec, "no-spec", false, "skip package.yaml spec generation")
	_ = generateAllCmd.MarkFlagRequired("proto")

	// Spec flags
	generateSpecCmd.Flags().StringVar(&genProtoFile, "proto", "", "path to .proto file (required)")
	generateSpecCmd.Flags().StringVar(&genLang, "lang", "go", "target language (go|csharp)")
	generateSpecCmd.Flags().StringVar(&genOutDir, "out", ".", "output directory")
	generateSpecCmd.Flags().IntVar(&genPort, "port", 0, "default gRPC port (auto-allocated if 0)")
	generateSpecCmd.Flags().BoolVar(&genDryRun, "dry-run", false, "print file list without writing")
	_ = generateSpecCmd.MarkFlagRequired("proto")

	generateCmd.AddCommand(generateInspectCmd)
	generateCmd.AddCommand(generateServiceCmd)
	generateCmd.AddCommand(generateAllCmd)
	generateCmd.AddCommand(generateSpecCmd)
}

// ── Handlers ─────────────────────────────────────────────────────────────────

func runGenerateInspect(cmd *cobra.Command, args []string) error {
	result, err := generate.InspectProto(genProtoFile)
	if err != nil {
		return err
	}

	output, err := generate.FormatInspectResult(result, genJSON)
	if err != nil {
		return err
	}

	fmt.Print(output)
	return nil
}

func runGenerateService(cmd *cobra.Command, args []string) error {
	data, err := parseProtoForGeneration()
	if err != nil {
		return err
	}

	files, err := generate.GenerateServiceOnly(data, genLang, genOutDir, genDryRun)
	if err != nil {
		return err
	}

	return printGenerationResult(files, genDryRun)
}

func runGenerateAll(cmd *cobra.Command, args []string) error {
	data, err := parseProtoForGeneration()
	if err != nil {
		return err
	}

	data.Persistence = genPersistence

	files, err := generate.GenerateFiles(data, genLang, genOutDir, genDryRun)
	if err != nil {
		return err
	}

	return printGenerationResult(files, genDryRun)
}

func runGenerateSpec(cmd *cobra.Command, args []string) error {
	data, err := parseProtoForGeneration()
	if err != nil {
		return err
	}

	files, err := generate.GenerateSpecOnly(data, genLang, genOutDir, genDryRun)
	if err != nil {
		return err
	}

	return printGenerationResult(files, genDryRun)
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func parseProtoForGeneration() (*generate.ServiceData, error) {
	// Try protoc first, fall back to regex
	data, err := generate.ParseProto(genProtoFile)
	if err != nil {
		data, err = generate.ParseProtoFallback(genProtoFile)
		if err != nil {
			return nil, fmt.Errorf("failed to parse proto: %w", err)
		}
	}

	// Enrich with authz annotations via regex if protoc missed them
	if !data.HasAuth {
		fallback, ferr := generate.ParseProtoFallback(genProtoFile)
		if ferr == nil && fallback.HasAuth {
			for i := range data.RPCs {
				for _, frpc := range fallback.RPCs {
					if frpc.Name == data.RPCs[i].Name {
						data.RPCs[i].AuthzAction = frpc.AuthzAction
						data.RPCs[i].AuthzPerm = frpc.AuthzPerm
						data.RPCs[i].ResourceTemplate = frpc.ResourceTemplate
						data.RPCs[i].DefaultRoleHint = frpc.DefaultRoleHint
					}
				}
			}
			data.HasAuth = fallback.HasAuth
		}
	}

	// Apply overrides from flags
	data.Store = genStore
	data.SkipSpec = genNoSpec
	if genPort > 0 {
		data.Port = genPort
	} else if data.Port == 0 {
		data.Port = 10200 // Default for new services
	}

	return data, nil
}

func printGenerationResult(files []string, dryRun bool) error {
	sort.Strings(files)

	if rootCfg.output == "json" {
		action := "generated"
		if dryRun {
			action = "would_generate"
		}
		out := map[string]interface{}{
			"action": action,
			"files":  files,
			"count":  len(files),
		}
		data, _ := json.MarshalIndent(out, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	if dryRun {
		fmt.Fprintf(os.Stderr, "Dry run — would generate %d files:\n", len(files))
	} else {
		fmt.Fprintf(os.Stderr, "Generated %d files:\n", len(files))
	}
	for _, f := range files {
		fmt.Println("  " + f)
	}
	return nil
}
