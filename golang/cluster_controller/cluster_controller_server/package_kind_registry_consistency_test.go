package main

// package_kind_registry_consistency_test.go — Slice 1 services-side drift gate.
//
// component_catalog.go (Component.Kind, copy #6) and the node-agent / repository
// classifiers (#7, #8, now routed through packagekind) must agree with the single
// canonical author, packages/registry.yaml — projected into the build-time generated
// packagekind table. A hand-edit to component_catalog.go that diverges from the
// registry fails HERE, in services CI, without needing the packages repo checked out
// (it compares against the committed generated table). This is the exact cross-kind
// drift class behind the xds incident; see
// docs/design/package-classification-single-source.md and ai-memory architecture/83b8f143.

import (
	"testing"

	"github.com/globulario/services/golang/packagekind"
)

// componentKindToRegistry maps the controller's ComponentKind enum to the
// registry.yaml kind string used by packagekind.
func componentKindToRegistry(k ComponentKind) string {
	switch k {
	case KindInfrastructure:
		return packagekind.KindInfrastructure
	case KindWorkload:
		return packagekind.KindService
	case KindCommand:
		return packagekind.KindCommand
	default:
		return ""
	}
}

func TestComponentCatalogKindMatchesRegistry(t *testing.T) {
	for _, comp := range buildCatalog() {
		if comp == nil || comp.Name == "" {
			continue
		}
		want, ok := packagekind.KindOf(comp.Name)
		if !ok {
			t.Errorf("component %q is in component_catalog.go but NOT in registry.yaml (packagekind projection) — registry.yaml is the single author; add it there and run `make gen-package-kinds`", comp.Name)
			continue
		}
		got := componentKindToRegistry(comp.Kind)
		if got != want {
			t.Errorf("kind drift for %q: component_catalog.go says %q, registry.yaml says %q — registry.yaml is authority; reconcile (do not hand-edit either to agree, fix the registry)", comp.Name, got, want)
		}
	}
}
