package evolution

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	behavioral "github.com/globulario/services/golang/ai_memory/behavioral/api"
)

// TestRemoteRecorderExposesNoRoutingSurface pins the shape of the type rather
// than a behaviour, because the defect is structural: any exported way to name
// an endpoint is a second routing authority, regardless of who calls it today.
func TestRemoteRecorderExposesNoRoutingSurface(t *testing.T) {
	rt := reflect.TypeOf(RemoteRecorder{})
	for i := 0; i < rt.NumField(); i++ {
		name := strings.ToLower(rt.Field(i).Name)
		if strings.Contains(name, "addr") || strings.Contains(name, "endpoint") ||
			strings.Contains(name, "target") || strings.Contains(name, "host") {
			t.Fatalf(
				"RemoteRecorder.%s lets a caller choose the Behavioral Memory owner; "+
					"the endpoint must come from service discovery alone",
				rt.Field(i).Name,
			)
		}
	}
	// The constructor takes a timeout and nothing else.
	ctor := reflect.TypeOf(NewRemoteRecorder)
	if ctor.NumIn() != 1 || ctor.In(0).Kind() != reflect.Int64 {
		t.Fatalf("NewRemoteRecorder must take only a timeout, got %v", ctor)
	}
}

// TestRuntimeResolvesBehavioralEndpointThroughDiscovery asserts the production
// path has exactly one way to find the service. With no discovery answer, it
// refuses rather than falling back to some other address.
func TestRuntimeResolvesBehavioralEndpointThroughDiscovery(t *testing.T) {
	r := NewRemoteRecorder(0)
	_, err := r.conn()
	if err == nil {
		// Discovery answered in this environment; that is the sanctioned path.
		_ = r.Close()
		return
	}
	if !strings.Contains(err.Error(), "service discovery") {
		t.Fatalf("expected a discovery-scoped refusal, got %v", err)
	}
}

// TestNoCLIOffersABehavioralAddressOverride guards the actual regression: the
// flags that let an operator point production ingestion at an instance of their
// choosing. A dependency-injected fake is the supported test seam instead.
func TestNoCLIOffersABehavioralAddressOverride(t *testing.T) {
	cmdRoot := "cmd"
	entries, err := os.ReadDir(cmdRoot)
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	checked := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		main := filepath.Join(cmdRoot, entry.Name(), "main.go")
		src, err := os.ReadFile(main)
		if err != nil {
			continue
		}
		checked++
		file, err := parser.ParseFile(fset, main, src, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", main, err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) == 0 {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "String" {
				return true
			}
			pkg, ok := sel.X.(*ast.Ident)
			if !ok || pkg.Name != "flag" {
				return true
			}
			lit, ok := call.Args[0].(*ast.BasicLit)
			if !ok {
				return true
			}
			name := strings.ToLower(strings.Trim(lit.Value, `"`))
			if name == "addr" || strings.Contains(name, "behavioral-addr") ||
				strings.HasSuffix(name, "-addr") || strings.HasSuffix(name, "-endpoint") {
				t.Errorf(
					"%s defines flag %s: a caller-supplied Behavioral Memory endpoint "+
						"would become a second production routing authority",
					main, lit.Value,
				)
			}
			return true
		})
	}
	if checked == 0 {
		t.Fatal("found no evolution commands to inspect")
	}
}

// TestIngestionStillAcceptsAnInjectedTransport keeps the supported seam honest:
// tests isolate the transport through the interface, which carries no routing.
func TestIngestionStillAcceptsAnInjectedTransport(t *testing.T) {
	fake := &fakeRecorder{}
	ingestor := SimulationIngestor{
		Recorder: fake,
		Project:  "globular",
		Domain:   behavioral.DomainRef("cluster_operator"),
		AgentID:  "test",
	}
	if _, err := ingestor.Ingest(context.Background(), validLearning()); err != nil {
		t.Fatalf("injected transport rejected: %v", err)
	}
	if fake.signals == 0 {
		t.Fatal("injected transport was not used")
	}
}
