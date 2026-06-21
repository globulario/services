package rdf

import (
	"sort"
	"strings"
)

// tripleSet accumulates unique N-Triples, giving deterministic, duplicate-free
// output regardless of emission order.
type tripleSet struct {
	seen  map[string]struct{}
	lines []string
}

func newTripleSet() *tripleSet { return &tripleSet{seen: map[string]struct{}{}} }

// add records "subject predicate object ." once. All three arguments must be
// pre-rendered (IRIs in <…>, literals in "…").
func (t *tripleSet) add(subject, predicate, object string) {
	line := subject + " " + predicate + " " + object + " ."
	if _, ok := t.seen[line]; ok {
		return
	}
	t.seen[line] = struct{}{}
	t.lines = append(t.lines, line)
}

// ntriples returns the sorted, deduplicated N-Triples document.
func (t *tripleSet) ntriples() []byte {
	sort.Strings(t.lines)
	if len(t.lines) == 0 {
		return []byte{}
	}
	return []byte(strings.Join(t.lines, "\n") + "\n")
}

// count returns the number of distinct triples.
func (t *tripleSet) count() int { return len(t.lines) }
