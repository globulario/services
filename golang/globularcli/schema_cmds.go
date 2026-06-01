// schema_cmds.go: Phase 4a schema discovery CLI.
//
//	globular schema list
//	globular schema describe <query>
//
// Answers: "What does this etcd/scylla key mean, who writes it, who
// reads it, what invariants does it carry?" The data is embedded into
// the CLI binary via go:embed — no server round-trip, works offline.

package main

import (
	"fmt"
	"strings"

	"github.com/globulario/services/golang/schema_reference"
	"github.com/spf13/cobra"
)

var (
	schemaJSON bool

	schemaCmd = &cobra.Command{
		Use:   "schema",
		Short: "Discover Globular's etcd/scylla key schema",
		Long: `The schema surface answers "what does this key mean?" for every etcd
and scylla key that carries a +globular:schema: pragma in the Go source.

Commands are offline — the schema index is embedded in the CLI binary
at build time, so lookups work with or without a running controller.`,
	}

	schemaListCmd = &cobra.Command{
		Use:   "list",
		Short: "List every annotated key in the schema index",
		RunE:  runSchemaList,
	}

	schemaDescribeCmd = &cobra.Command{
		Use:   "describe <query>",
		Short: "Look up a schema entry by key pattern, Go type name, or substring",
		Args:  cobra.ExactArgs(1),
		RunE:  runSchemaDescribe,
	}
)

func init() {
	rootCmd.AddCommand(schemaCmd)
	schemaCmd.AddCommand(schemaListCmd)
	schemaCmd.AddCommand(schemaDescribeCmd)
	for _, c := range []*cobra.Command{schemaListCmd, schemaDescribeCmd} {
		c.Flags().BoolVar(&schemaJSON, "json", false, "Output as JSON")
	}
}

func runSchemaList(cmd *cobra.Command, _ []string) error {
	reg := schema_reference.DefaultRegistry()
	entries := reg.Entries()
	if schemaJSON {
		return writeJSON(map[string]interface{}{
			"count":        len(entries),
			"entries":      entries,
			"source":       reg.Source(),
			"generated_at": reg.GeneratedAtUnix(),
		})
	}
	printFreshnessHeader(reg)
	fmt.Printf("%d entries:\n\n", len(entries))
	for _, e := range entries {
		fmt.Printf("  %s\n", e.KeyPattern)
		fmt.Printf("    type: %s  writer: %s\n", e.TypeName, e.Writer)
		if e.Description != "" {
			fmt.Printf("    %s\n", e.Description)
		}
		fmt.Println()
	}
	return nil
}

func runSchemaDescribe(cmd *cobra.Command, args []string) error {
	q := strings.TrimSpace(args[0])
	reg := schema_reference.DefaultRegistry()

	var hits []schema_reference.Entry
	var matchKind string
	if e := reg.LookupByKey(q); e != nil {
		hits = []schema_reference.Entry{*e}
		matchKind = "exact_key"
	} else if e := reg.LookupByType(q); e != nil {
		hits = []schema_reference.Entry{*e}
		matchKind = "exact_type"
	} else {
		hits = reg.Search(q)
		matchKind = "substring"
	}
	if len(hits) == 0 {
		return fmt.Errorf("no schema entry matches %q", q)
	}
	if schemaJSON {
		return writeJSON(map[string]interface{}{
			"count":        len(hits),
			"entries":      hits,
			"match_kind":   matchKind,
			"source":       reg.Source(),
			"generated_at": reg.GeneratedAtUnix(),
		})
	}
	printFreshnessHeader(reg)
	fmt.Printf("match: %s (%d result%s)\n\n", matchKind, len(hits), pluralS(len(hits)))
	for _, e := range hits {
		printSchemaEntry(e)
		fmt.Println()
	}
	return nil
}

func printSchemaEntry(e schema_reference.Entry) {
	fmt.Printf("key_pattern:   %s\n", e.KeyPattern)
	fmt.Printf("type:          %s\n", e.TypeName)
	fmt.Printf("writer:        %s\n", e.Writer)
	if len(e.Readers) > 0 {
		fmt.Printf("readers:       %s\n", strings.Join(e.Readers, ", "))
	}
	if e.Description != "" {
		fmt.Printf("description:   %s\n", e.Description)
	}
	if e.Invariants != "" {
		fmt.Printf("invariants:    %s\n", e.Invariants)
	}
	if e.SinceVersion != "" {
		fmt.Printf("since:         %s\n", e.SinceVersion)
	}
	if e.SourceFile != "" {
		fmt.Printf("source:        %s:%d\n", e.SourceFile, e.SourceLine)
	}
}

func printFreshnessHeader(reg *schema_reference.Registry) {
	fmt.Printf("── schema index ── source=%s  generated_at=%d\n\n", reg.Source(), reg.GeneratedAtUnix())
}

func pluralS(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// writeJSON is defined in doctor_report_cmd.go — shared across the
// doctor and schema subcommands. Kept there to avoid a duplicate.
