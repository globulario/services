// authzgen reads proto descriptor sets and generates authorization policy
// manifests (permissions.generated.json and roles.generated.json) from
// globular.auth.AuthzRule annotations on RPC methods.
//
// Usage:
//
//	protoc -I proto --descriptor_set_out=descriptor.pb --include_imports proto/catalog.proto
//	go run ./globularcli/tools/authzgen -descriptor descriptor.pb -out generated/policy/catalog
//
// Or generate for all annotated services:
//
//	go run ./globularcli/tools/authzgen -descriptor descriptor.pb -out generated/policy
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/globulario/services/golang/authpb"
	"google.golang.org/protobuf/proto"
	descriptorpb "google.golang.org/protobuf/types/descriptorpb"
)

const (
	schemaVersion    = "2"
	generatorVersion = "authzgen/0.1.0"
)

// ── Output types ────────────────────────────────────────────────────────────

type PermissionsManifest struct {
	SchemaVersion    string               `json:"schema_version"`
	GeneratorVersion string               `json:"generator_version"`
	Service          string               `json:"service"`
	Permissions      []PermissionEntry    `json:"permissions"`
}

type PermissionEntry struct {
	Method             string          `json:"method"`
	Action             string          `json:"action"`
	Permission         string          `json:"permission"`
	ResourceTemplate   string          `json:"resource_template,omitempty"`
	CollectionTemplate string          `json:"collection_template,omitempty"`
	Resources          []ResourceEntry `json:"resources"`
}

type ResourceEntry struct {
	Field       string `json:"field"`
	Kind        string `json:"kind"`
	ScopeAnchor bool   `json:"scope_anchor,omitempty"`
}

type RolesManifest struct {
	SchemaVersion    string     `json:"schema_version"`
	GeneratorVersion string     `json:"generator_version"`
	Service          string     `json:"service"`
	Roles            []RoleEntry `json:"roles"`
}

type RoleEntry struct {
	Name     string   `json:"name"`
	Inherits []string `json:"inherits,omitempty"`
	Actions  []string `json:"actions"`
}

func main() {
	descriptorPath := flag.String("descriptor", "", "Path to compiled proto descriptor set (.pb)")
	outDir := flag.String("out", "", "Output directory for generated policy files")
	flag.Parse()

	if *descriptorPath == "" || *outDir == "" {
		flag.Usage()
		os.Exit(2)
	}

	data, err := os.ReadFile(*descriptorPath)
	if err != nil {
		log.Fatalf("read descriptor: %v", err)
	}

	var fds descriptorpb.FileDescriptorSet
	if err := proto.Unmarshal(data, &fds); err != nil {
		log.Fatalf("unmarshal descriptor set: %v", err)
	}

	// First pass: collect all services grouped by proto package.
	// A single proto file can define multiple services (e.g. workflow.proto
	// defines WorkflowService and WorkflowActorService in package "workflow").
	// Without grouping, the inner loop would write the same output path once
	// per service, and each write would silently overwrite the previous —
	// leaving only the last service's permissions in the file.
	type svcPerms struct {
		name      string
		perms     []PermissionEntry
		resources map[string][]ResourceEntry
	}
	pkgServices := make(map[string][]svcPerms) // lowercase-package → services
	pkgOrder := []string{}                     // preserve iteration order for determinism

	for _, fd := range fds.GetFile() {
		for _, sd := range fd.GetService() {
			serviceName := fmt.Sprintf("%s.%s", fd.GetPackage(), sd.GetName())
			perms, resources := extractPermissions(fd, sd)
			if len(perms) == 0 {
				continue // no annotated methods
			}
			pkg := strings.ToLower(fd.GetPackage())
			if _, seen := pkgServices[pkg]; !seen {
				pkgOrder = append(pkgOrder, pkg)
			}
			pkgServices[pkg] = append(pkgServices[pkg], svcPerms{
				name:      serviceName,
				perms:     perms,
				resources: resources,
			})
		}
	}

	// Second pass: write one permissions.generated.json per package directory,
	// merging all services that share the package.
	for _, pkg := range pkgOrder {
		services := pkgServices[pkg]

		// Merge permissions and resource metadata from all services in this package.
		var allPerms []PermissionEntry
		mergedResources := make(map[string][]ResourceEntry)
		var serviceNames []string
		for _, svc := range services {
			serviceNames = append(serviceNames, svc.name)
			allPerms = append(allPerms, svc.perms...)
			for k, v := range svc.resources {
				if _, exists := mergedResources[k]; !exists {
					mergedResources[k] = v
				}
			}
		}

		// Sort deterministically by method path.
		sort.Slice(allPerms, func(i, j int) bool {
			return allPerms[i].Method < allPerms[j].Method
		})

		svcDir := filepath.Join(*outDir, pkg)
		if err := os.MkdirAll(svcDir, 0755); err != nil {
			log.Fatalf("mkdir %s: %v", svcDir, err)
		}

		// Record all service names in the manifest so the file is self-describing.
		displayName := strings.Join(serviceNames, ", ")

		// Emit permissions.generated.json
		pm := PermissionsManifest{
			SchemaVersion:    schemaVersion,
			GeneratorVersion: generatorVersion,
			Service:          displayName,
			Permissions:      addResources(allPerms, mergedResources),
		}
		writeJSON(filepath.Join(svcDir, "permissions.generated.json"), pm)

		// Emit roles.generated.json (derive role prefix from the package name).
		rm := generateRoles(services[0].name, allPerms)
		writeJSON(filepath.Join(svcDir, "roles.generated.json"), rm)

		fmt.Printf("==> %s: %d permissions, %d roles\n", displayName, len(pm.Permissions), len(rm.Roles))
	}
}

// extractPermissions reads AuthzRule annotations from service methods.
func extractPermissions(fd *descriptorpb.FileDescriptorProto, sd *descriptorpb.ServiceDescriptorProto) ([]PermissionEntry, map[string][]ResourceEntry) {
	var perms []PermissionEntry
	resources := make(map[string][]ResourceEntry) // request message name → fields

	// Build field-level resource metadata from all messages in the file.
	for _, md := range fd.GetMessageType() {
		var fields []ResourceEntry
		for _, field := range md.GetField() {
			if field.GetOptions() == nil {
				continue
			}
			rf := getResourceField(field.GetOptions())
			if rf == nil {
				continue
			}
			fields = append(fields, ResourceEntry{
				Field:       field.GetName(),
				Kind:        rf.GetKind(),
				ScopeAnchor: rf.GetScopeAnchor(),
			})
		}
		if len(fields) > 0 {
			resources[md.GetName()] = fields
		}
	}

	pkg := fd.GetPackage()
	svcName := sd.GetName()
	for _, md := range sd.GetMethod() {
		if md.GetOptions() == nil {
			continue
		}
		rule := getAuthzRule(md.GetOptions())
		if rule == nil {
			continue
		}

		methodPath := fmt.Sprintf("/%s.%s/%s", pkg, svcName, md.GetName())
		perms = append(perms, PermissionEntry{
			Method:             methodPath,
			Action:             rule.GetAction(),
			Permission:         rule.GetPermission(),
			ResourceTemplate:   rule.GetResourceTemplate(),
			CollectionTemplate: rule.GetCollectionTemplate(),
		})
	}

	return perms, resources
}

// getAuthzRule extracts the AuthzRule extension from method options.
func getAuthzRule(opts *descriptorpb.MethodOptions) *authpb.AuthzRule {
	if !proto.HasExtension(opts, authpb.E_Authz) {
		return nil
	}
	ext := proto.GetExtension(opts, authpb.E_Authz)
	if rule, ok := ext.(*authpb.AuthzRule); ok {
		return rule
	}
	return nil
}

// getResourceField extracts the ResourceField extension from field options.
func getResourceField(opts *descriptorpb.FieldOptions) *authpb.ResourceField {
	if !proto.HasExtension(opts, authpb.E_Resource) {
		return nil
	}
	ext := proto.GetExtension(opts, authpb.E_Resource)
	if rf, ok := ext.(*authpb.ResourceField); ok {
		return rf
	}
	return nil
}

// addResources merges field-level resource metadata into permission entries
// by extracting {field} placeholders from templates and matching them against
// ResourceField annotations on the corresponding request message.
func addResources(perms []PermissionEntry, resources map[string][]ResourceEntry) []PermissionEntry {
	// Build a flat field→ResourceEntry index (first match wins per field name).
	fieldIndex := make(map[string]ResourceEntry)
	for _, msgFields := range resources {
		for _, rf := range msgFields {
			if _, exists := fieldIndex[rf.Field]; !exists {
				fieldIndex[rf.Field] = rf
			}
		}
	}

	for i := range perms {
		p := &perms[i]
		template := p.ResourceTemplate
		if template == "" {
			template = p.CollectionTemplate
		}
		if template == "" {
			continue
		}
		// Extract {field} placeholders from template.
		seen := make(map[string]bool)
		var fields []ResourceEntry
		for _, part := range strings.Split(template, "/") {
			if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
				fieldName := part[1 : len(part)-1]
				if seen[fieldName] {
					continue // deduplicate
				}
				seen[fieldName] = true
				if rf, ok := fieldIndex[fieldName]; ok {
					fields = append(fields, rf)
				} else {
					fields = append(fields, ResourceEntry{Field: fieldName})
				}
			}
		}
		p.Resources = fields
	}
	return perms
}

// generateRoles creates default roles from action families using role hints.
func generateRoles(serviceName string, perms []PermissionEntry) RolesManifest {
	// Group actions by role hint.
	hintActions := make(map[string][]string)
	for _, p := range perms {
		hint := p.Permission // fallback to permission kind if no role hint
		// We use the permission field as the grouping key:
		// read → viewer, write → editor, delete/admin → admin
		switch {
		case hint == "read":
			hintActions["viewer"] = appendUnique(hintActions["viewer"], p.Action)
		case hint == "write":
			hintActions["editor"] = appendUnique(hintActions["editor"], p.Action)
		case hint == "delete" || hint == "admin":
			hintActions["admin"] = appendUnique(hintActions["admin"], p.Action)
		}
	}

	// Sort actions within each role.
	for k := range hintActions {
		sort.Strings(hintActions[k])
	}

	// Build hierarchical roles.
	prefix := strings.Split(serviceName, ".")[0] // e.g., "catalog"
	var roles []RoleEntry

	if actions, ok := hintActions["viewer"]; ok {
		roles = append(roles, RoleEntry{
			Name:    fmt.Sprintf("role:%s.viewer", prefix),
			Actions: actions,
		})
	}
	if actions, ok := hintActions["editor"]; ok {
		roles = append(roles, RoleEntry{
			Name:     fmt.Sprintf("role:%s.editor", prefix),
			Inherits: []string{fmt.Sprintf("role:%s.viewer", prefix)},
			Actions:  actions,
		})
	}
	if actions, ok := hintActions["admin"]; ok {
		roles = append(roles, RoleEntry{
			Name:     fmt.Sprintf("role:%s.admin", prefix),
			Inherits: []string{fmt.Sprintf("role:%s.editor", prefix)},
			Actions:  actions,
		})
	}

	return RolesManifest{
		SchemaVersion:    schemaVersion,
		GeneratorVersion: generatorVersion,
		Service:          serviceName,
		Roles:            roles,
	}
}

func appendUnique(slice []string, val string) []string {
	for _, s := range slice {
		if s == val {
			return slice
		}
	}
	return append(slice, val)
}

func writeJSON(path string, v interface{}) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		log.Fatalf("marshal %s: %v", path, err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0644); err != nil {
		log.Fatalf("write %s: %v", path, err)
	}
	fmt.Printf("  wrote %s\n", path)
}
