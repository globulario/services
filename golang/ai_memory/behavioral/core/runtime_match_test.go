package core

import (
	"testing"

	"github.com/globulario/services/golang/ai_memory/behavioral/api"
	"github.com/globulario/services/golang/ai_memory/behavioral/domain"
)

// forbiddenRefMatches is the pure matcher behind CheckAction's forbidden-move
// detection. It must match by exact id/target (the original contract) AND by any
// alias declared for the ref — without that, a naturally-named action can never
// match a forbidden.<domain>.* id and the gate has no reach.
func TestForbiddenRefMatches(t *testing.T) {
	const ref = api.ForbiddenMoveRef("forbidden.cluster.hot_swap_binary_outside_release_pipeline")
	aliasIdx := map[string][]string{
		string(ref): {"replace_binary_in_place", "cp_swap_binary"},
	}
	cases := []struct {
		name       string
		actionType string
		target     string
		idx        map[string][]string
		want       bool
	}{
		{"exact id", string(ref), "", aliasIdx, true},
		{"alias match", "replace_binary_in_place", "", aliasIdx, true},
		{"second alias", "cp_swap_binary", "", aliasIdx, true},
		{"target exact id", "something_else", string(ref), aliasIdx, true},
		{"unrelated action", "restart_service", "", aliasIdx, false},
		{"alias unknown when no index", "replace_binary_in_place", "", nil, false},
		{"exact id still matches with nil index", string(ref), "", nil, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := forbiddenRefMatches(ref, tc.actionType, tc.target, tc.idx); got != tc.want {
				t.Errorf("forbiddenRefMatches(%q, action=%q, target=%q) = %v, want %v",
					ref, tc.actionType, tc.target, got, tc.want)
			}
		})
	}
}

// fakeDomain is a minimal domain.Domain carrying one forbidden move with aliases.
type fakeDomain struct {
	name string
	cats domain.Catalogs
}

func (f fakeDomain) Name() string             { return f.name }
func (f fakeDomain) Catalogs() domain.Catalogs { return f.cats }

// forbiddenAliasIndex must read action_aliases (comma-separated) from the domain
// pack via the registry, trim whitespace, and return nil when the registry or the
// domain is absent (so callers fall back to exact match).
func TestForbiddenAliasIndex(t *testing.T) {
	reg := domain.NewRegistry()
	reg.Register(fakeDomain{
		name: "cluster_operator",
		cats: domain.Catalogs{ForbiddenMoves: []domain.CatalogEntry{
			{ID: "forbidden.x", Fields: map[string]string{"action_aliases": "a1, a2 ,  a3"}},
			{ID: "forbidden.y", Fields: map[string]string{}}, // no aliases → omitted
		}},
	})
	s := &Service{registry: reg}

	idx := s.forbiddenAliasIndex(api.DomainRef("cluster_operator"))
	if got := idx["forbidden.x"]; len(got) != 3 || got[0] != "a1" || got[1] != "a2" || got[2] != "a3" {
		t.Errorf("aliases for forbidden.x = %v, want [a1 a2 a3] (trimmed)", got)
	}
	if _, ok := idx["forbidden.y"]; ok {
		t.Errorf("forbidden.y has no aliases and must be absent from the index")
	}

	// Unknown domain → nil (exact-match fallback).
	if idx := s.forbiddenAliasIndex(api.DomainRef("nope")); idx != nil {
		t.Errorf("unknown domain must yield nil alias index, got %v", idx)
	}
	// Nil registry → nil.
	if idx := (&Service{}).forbiddenAliasIndex(api.DomainRef("cluster_operator")); idx != nil {
		t.Errorf("nil registry must yield nil alias index, got %v", idx)
	}
}
