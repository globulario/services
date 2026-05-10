// validate-deps walks every packages/metadata/<service>/awareness.yaml,
// extracts depends_on entries, and verifies each claim against the
// corresponding golang/<service>/ source. A claim that produces zero
// matches across all evidence patterns is reported as drift.
//
// Exit code: 0 if every contract matches code; 1 if any drift is found.
//
// Usage:
//
//	validate-deps [--services <path>] [--packages <path>] [--json]
//
// Defaults assume the standard layout:
//
//	$root/services/golang/<svc>/...
//	$root/packages/metadata/<svc>/awareness.yaml
//
// where $root is the directory shared by both repos. The tool auto-detects
// $root by walking up from the current working directory.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type dependency struct {
	Service  string `yaml:"service"`
	Phase    string `yaml:"phase"`
	Required bool   `yaml:"required"`
	Reason   string `yaml:"reason"`
}

type contract struct {
	Service     string       `yaml:"service"`
	Package     string       `yaml:"package"`
	PackageKind string       `yaml:"package_kind"`
	DependsOn   []dependency `yaml:"depends_on"`
}

type finding struct {
	Service        string `json:"service"`
	ContractPath   string `json:"contract_path"`
	Dependency     string `json:"dependency"`
	Phase          string `json:"phase"`
	Required       bool   `json:"required"`
	SourceDir      string `json:"source_dir,omitempty"`
	OutOfScope     bool   `json:"out_of_scope,omitempty"`
	NoEvidence     bool   `json:"no_evidence,omitempty"`
	UnknownPattern bool   `json:"unknown_pattern,omitempty"`
	Reason         string `json:"reason,omitempty"`
}

// evidencePatterns lists, per dependency name, regex strings that — if any
// matches in any non-test .go file under the dependent service's source dir
// — count as proof the dependency is real. nil means "implicit, always pass"
// (e.g. etcd, which every service uses transitively via globular_service).
//
// Names with both dash and underscore variants are listed twice — Globular
// awareness.yaml files are not consistent.
var evidencePatterns = map[string][]string{
	// Implicit / non-checkable.
	//
	// etcd is initialized for every service via globular_service. authentication,
	// rbac, and event are imported by interceptors/ServerInterceptors.go and
	// globular_service/services.go — every authenticated gRPC service gets them
	// through the lifecycle manager + interceptor chain, not by direct import.
	// Treating these as implicit reflects the architectural truth.
	"etcd":           nil,
	"etcdctl":        nil,
	"authentication": nil,
	"rbac":           nil,
	"event":          nil,
	"globular-cli":   nil,
	"globular_cli":   nil,
	"claude":         nil,
	"ffmpeg":         nil,
	"yt-dlp":         nil,
	"yt_dlp":         nil,
	"mc":             nil,
	"rclone":         nil,
	"restic":         nil,

	// Infrastructure services with distinct evidence.
	//
	// Patterns are intentionally broad — a dependency can be imported directly
	// (e.g. rbac → gocql) or used via a wrapper (e.g. rbac → storage_store.ScyllaStore).
	// The case-insensitive name match catches the wrapper case; the direct
	// patterns catch the import case.
	"scylladb":              {`(?i)\bscylla\w*`, `\bgocql\b`, `\b9042\b`, `scylla\.yaml`, `\bcqlsh\b`},
	"scylla":                {`(?i)\bscylla\w*`, `\bgocql\b`, `\b9042\b`, `scylla\.yaml`, `\bcqlsh\b`},
	"scylla-manager":        {`(?i)scylla[_-]?manager`, `\bsctool\b`},
	"scylla-manager-agent":  {`(?i)scylla[_-]?manager`, `\bsctool\b`},
	"minio":                 {`(?i)\bminio\w*`, `\bminio-go\b`, `\b9000\b`, `\bs3\.New\b`, `\bmadmin\b`},
	"prometheus":            {`(?i)\bprometheus\b`, `\b9090\b`, `/api/v1/query`, `prometheus_v1`, `prometheusv1`},
	"alertmanager":          {`(?i)\balertmanager\b`, `\b9093\b`},
	"envoy":                 {`(?i)\benvoy\b`, `\bxds\b`, `\b9901\b`},
	"gateway":               {`gateway_client`, `\b8443\b`, `\bgatewaypb\b`},
	"keepalived":            {`(?i)\bkeepalived\b`, `\bvrrp\b`, `globular-keepalived`},
	"node-exporter":         {`node[_-]?exporter`, `\b9100\b`},
	"sidekick":              {`(?i)\bsidekick\b`, `\b9091\b`},

	// gRPC services — each follows the <name>_client / <name>pb / <Name>Service convention.
	// (authentication/rbac/event are listed as implicit above — they come via interceptors.)
	"dns":                {`dns_client`, `\bdnspb\b`, `DNSService`},
	"resource":           {`resource_client`, `\bresourcepb\b`, `ResourceService`},
	"repository":         {`repository_client`, `\brepositorypb\b`, `PackageRepository`},
	"cluster-controller": {`cluster_controller_client`, `\bcluster_controllerpb\b`, `ClusterControllerService`},
	"cluster_controller": {`cluster_controller_client`, `\bcluster_controllerpb\b`, `ClusterControllerService`},
	"node-agent":         {`node_agent_client`, `\bnode_agentpb\b`, `NodeAgentService`},
	"node_agent":         {`node_agent_client`, `\bnode_agentpb\b`, `NodeAgentService`},
	"workflow":           {`workflow_client`, `\bworkflowpb\b`, `WorkflowService`},
	"log":                {`log_client`, `\blogpb\b`, `LogService`},
	"mail":               {`mail_client`, `\bmailpb\b`},
	"file":               {`file_client`, `\bfilepb\b`},
	"title":              {`title_client`, `\btitlepb\b`},
	"ldap":               {`ldap_client`, `\bldappb\b`},
	"storage":            {`storage_client`, `\bstoragepb\b`},
	"persistence":        {`persistence_client`, `\bpersistencepb\b`},
	"search":             {`search_client`, `\bsearchpb\b`},
	"sql":                {`sql_client`, `\bsqlpb\b`},
	"blog":               {`blog_client`, `\bblogpb\b`},
	"conversation":       {`conversation_client`, `\bconversationpb\b`},
	"catalog":            {`catalog_client`, `\bcatalogpb\b`},
	"discovery":          {`discovery_client`, `\bdiscoverypb\b`},
	"backup-manager":     {`backup_manager_client`, `\bbackup_managerpb\b`},
	"backup_manager":     {`backup_manager_client`, `\bbackup_managerpb\b`},
	"cluster-doctor":     {`cluster_doctor_client`, `\bcluster_doctorpb\b`},
	"cluster_doctor":     {`cluster_doctor_client`, `\bcluster_doctorpb\b`},
	"monitoring":         {`monitoring_client`, `\bmonitoringpb\b`, `MonitoringService`},
	"echo":               {`echo_client`, `\bechopb\b`, `EchoService`},
	"torrent":            {`torrent_client`, `\btorrentpb\b`},
	"media":              {`media_client`, `\bmediapb\b`},
	"ai-memory":          {`ai_memory_client`, `\bai_memorypb\b`},
	"ai_memory":          {`ai_memory_client`, `\bai_memorypb\b`},
	"ai-executor":        {`ai_executor_client`, `\bai_executorpb\b`},
	"ai_executor":        {`ai_executor_client`, `\bai_executorpb\b`},
	"ai-watcher":         {`ai_watcher_client`, `\bai_watcherpb\b`},
	"ai_watcher":         {`ai_watcher_client`, `\bai_watcherpb\b`},
	"ai-router":          {`ai_router_client`, `\bai_routerpb\b`},
	"ai_router":          {`ai_router_client`, `\bai_routerpb\b`},
}

func resolveSourceDir(servicesGolangRoot, name string) string {
	candidates := []string{
		name,
		strings.ReplaceAll(name, "-", "_"),
		strings.ReplaceAll(name, "-", ""),
	}
	for _, c := range candidates {
		d := filepath.Join(servicesGolangRoot, c)
		if info, err := os.Stat(d); err == nil && info.IsDir() {
			return d
		}
	}
	return ""
}

func anyMatch(dir string, patterns []*regexp.Regexp) bool {
	if len(patterns) == 0 {
		return true
	}
	matched := false
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || matched {
			return err
		}
		if d.IsDir() {
			// Skip vendor and generated proto dirs to keep the search fast.
			base := filepath.Base(path)
			if base == "vendor" || base == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		if strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		for _, p := range patterns {
			if p.Match(data) {
				matched = true
				return filepath.SkipAll
			}
		}
		return nil
	})
	return matched
}

func detectRoots(explicitServices, explicitPackages string) (servicesGolang, packagesMeta string, err error) {
	if explicitServices != "" && explicitPackages != "" {
		return filepath.Join(explicitServices, "golang"), filepath.Join(explicitPackages, "metadata"), nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", "", err
	}
	dir := cwd
	for {
		s := filepath.Join(dir, "services", "golang")
		p := filepath.Join(dir, "packages", "metadata")
		sInfo, sErr := os.Stat(s)
		pInfo, pErr := os.Stat(p)
		if sErr == nil && sInfo.IsDir() && pErr == nil && pInfo.IsDir() {
			return s, p, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	// Fallback: assume cwd is inside services/, packages/ is sibling.
	dir = cwd
	for {
		if filepath.Base(dir) == "services" {
			return filepath.Join(dir, "golang"), filepath.Join(filepath.Dir(dir), "packages", "metadata"), nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", "", fmt.Errorf("could not auto-detect repo layout; pass --services and --packages explicitly")
}

func main() {
	servicesFlag := flag.String("services", "", "Path to services repo root (default: auto-detect)")
	packagesFlag := flag.String("packages", "", "Path to packages repo root (default: auto-detect)")
	jsonFlag := flag.Bool("json", false, "Emit findings as JSON")
	requiredOnly := flag.Bool("required-only", false, "Only flag drift on required dependencies (default: all)")
	flag.Parse()

	servicesGolang, packagesMeta, err := detectRoots(*servicesFlag, *packagesFlag)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	// Compile patterns once.
	compiled := make(map[string][]*regexp.Regexp, len(evidencePatterns))
	for k, pats := range evidencePatterns {
		var rs []*regexp.Regexp
		for _, p := range pats {
			r, err := regexp.Compile(p)
			if err != nil {
				fmt.Fprintf(os.Stderr, "invalid pattern %q for dep %q: %v\n", p, k, err)
				os.Exit(2)
			}
			rs = append(rs, r)
		}
		compiled[k] = rs
	}

	contractPaths, err := filepath.Glob(filepath.Join(packagesMeta, "*", "awareness.yaml"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	sort.Strings(contractPaths)

	var findings []finding
	totalContracts, totalDeps := 0, 0

	for _, path := range contractPaths {
		data, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "read %s: %v\n", path, err)
			continue
		}
		var c contract
		if err := yaml.Unmarshal(data, &c); err != nil {
			fmt.Fprintf(os.Stderr, "parse %s: %v\n", path, err)
			continue
		}
		totalContracts++

		// Service name to look up source dir: prefer the contract's `service`
		// field; fall back to the parent dir name in packages/metadata/.
		dependentName := c.Service
		if dependentName == "" {
			dependentName = filepath.Base(filepath.Dir(path))
		}
		sourceDir := resolveSourceDir(servicesGolang, dependentName)

		for _, dep := range c.DependsOn {
			totalDeps++
			if *requiredOnly && !dep.Required {
				continue
			}
			f := finding{
				Service:      dependentName,
				ContractPath: path,
				Dependency:   dep.Service,
				Phase:        dep.Phase,
				Required:     dep.Required,
				SourceDir:    sourceDir,
			}
			if sourceDir == "" {
				// Dependent has no Globular Go source (external tool like
				// sidekick/keepalived/prometheus, or lives in a sibling repo
				// like xds in Globular/). Mark as out of scope; manual
				// verification needed but this is not contract drift.
				f.OutOfScope = true
				f.Reason = "dependent service has no golang/ source — manual verification required"
				findings = append(findings, f)
				continue
			}
			pats, known := compiled[dep.Service]
			if !known {
				f.UnknownPattern = true
				f.Reason = "no evidence rule for this dependency name in validate-deps; add one to evidencePatterns"
				findings = append(findings, f)
				continue
			}
			if len(pats) == 0 {
				continue // implicit, always pass
			}
			if !anyMatch(sourceDir, pats) {
				f.NoEvidence = true
				f.Reason = "no evidence in source: contract claims dependency that the code does not import or reference"
				findings = append(findings, f)
			}
		}
	}

	if *jsonFlag {
		out, _ := json.MarshalIndent(map[string]any{
			"contracts_scanned":     totalContracts,
			"dependencies_scanned":  totalDeps,
			"findings":              findings,
			"drift_count":           countDrift(findings),
		}, "", "  ")
		fmt.Println(string(out))
	} else {
		printText(findings, totalContracts, totalDeps)
	}

	if countDrift(findings) > 0 {
		os.Exit(1)
	}
}

func countDrift(fs []finding) int {
	n := 0
	for _, f := range fs {
		if f.NoEvidence {
			n++
		}
	}
	return n
}

func printText(findings []finding, totalContracts, totalDeps int) {
	bySvc := make(map[string][]finding)
	for _, f := range findings {
		bySvc[f.Service] = append(bySvc[f.Service], f)
	}
	keys := make([]string, 0, len(bySvc))
	for k := range bySvc {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	driftFindings := 0
	outOfScope := 0
	unknownFindings := 0
	for _, k := range keys {
		hadAny := false
		for _, f := range bySvc[k] {
			if f.NoEvidence || f.UnknownPattern || f.OutOfScope {
				hadAny = true
				break
			}
		}
		if !hadAny {
			continue
		}
		fmt.Printf("\n── %s ──\n", k)
		for _, f := range bySvc[k] {
			tag := ""
			switch {
			case f.NoEvidence:
				tag = "DRIFT"
				driftFindings++
			case f.OutOfScope:
				tag = "SKIP"
				outOfScope++
			case f.UnknownPattern:
				tag = "UNKNOWN"
				unknownFindings++
			default:
				continue
			}
			req := ""
			if f.Required {
				req = " (required)"
			}
			fmt.Printf("  [%s] depends_on: %s phase=%s%s — %s\n",
				tag, f.Dependency, f.Phase, req, f.Reason)
		}
	}

	fmt.Printf("\n──────────────────────────────────────────────\n")
	fmt.Printf("Contracts scanned:     %d\n", totalContracts)
	fmt.Printf("Dependencies scanned:  %d\n", totalDeps)
	fmt.Printf("Drift (no evidence):   %d  ← actionable\n", driftFindings)
	fmt.Printf("Out of scope (skip):   %d  ← external tools / sibling-repo services\n", outOfScope)
	fmt.Printf("Unknown rule:          %d  ← add a pattern to evidencePatterns\n", unknownFindings)
	if driftFindings == 0 {
		fmt.Println("\n✓ All checked contracts match code.")
	} else {
		fmt.Println("\n✗ Drift found. Patch the awareness.yaml files above to match the code,")
		fmt.Println("  or add the missing dependency to the source if the contract is correct.")
	}
}
