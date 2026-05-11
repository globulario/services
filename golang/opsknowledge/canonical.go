package opsknowledge

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"

	"gopkg.in/yaml.v3"
)

// CanonicalizeEntry produces the deterministic byte sequence used to compute
// an entry's seed_sha256. The bytes are the JSON serialization of the entry
// with:
//
//   - provenance.seed_version REMOVED (ignored — would otherwise affect every
//     entry's hash on every release)
//   - provenance.seed_sha256 REMOVED (the hash CANNOT include itself)
//   - all map keys sorted alphabetically (recursively)
//   - no insignificant whitespace (compact JSON)
//
// The result is identical regardless of YAML key ordering or whitespace in
// the source file. Two semantically equal entries produce the same hash.
//
// Why JSON instead of canonical YAML: JSON's object semantics (sorted keys,
// no comments, no anchors, no styles) are simpler and more widely available.
// The hash is opaque anyway — only the build pipeline and the doctor verify it.
func CanonicalizeEntry(e Entry) ([]byte, error) {
	// Serialize to YAML first to flatten Extra (inline) into a generic map.
	yamlBytes, err := yaml.Marshal(e)
	if err != nil {
		return nil, fmt.Errorf("yaml marshal: %w", err)
	}
	var generic map[string]any
	if err := yaml.Unmarshal(yamlBytes, &generic); err != nil {
		return nil, fmt.Errorf("yaml roundtrip: %w", err)
	}

	// Strip the two provenance fields that must not affect the hash.
	if prov, ok := generic["provenance"].(map[string]any); ok {
		delete(prov, "seed_version")
		delete(prov, "seed_sha256")
	}

	// Walk the tree sorting all map keys so JSON output is deterministic.
	sortedTree := normalizeSortedKeys(generic)

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(sortedTree); err != nil {
		return nil, fmt.Errorf("json encode: %w", err)
	}
	// json.Encoder appends a trailing newline; strip it for a stable hash.
	out := bytes.TrimRight(buf.Bytes(), "\n")
	return out, nil
}

// HashEntry returns the hex-encoded SHA256 of the entry's canonical form.
// This is what the build pipeline stamps into provenance.seed_sha256 and
// what cluster-doctor verifies at runtime.
func HashEntry(e Entry) (string, error) {
	canon, err := CanonicalizeEntry(e)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(canon)
	return hex.EncodeToString(sum[:]), nil
}

// normalizeSortedKeys recursively converts every map[string]any (and its
// generic siblings) into one whose JSON encoding has sorted keys. Used for
// canonical hashing only.
func normalizeSortedKeys(v any) any {
	switch t := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		out := make(map[string]any, len(t))
		for _, k := range keys {
			out[k] = normalizeSortedKeys(t[k])
		}
		// Wrap in a sortedMap so the JSON encoder writes keys in our chosen
		// order. Plain map[string]any already serializes alphabetically with
		// encoding/json, but make it explicit for forward safety.
		return sortedMap{keys: keys, m: out}
	case map[any]any:
		// yaml.v3 may produce map[any]any for maps with non-string keys;
		// coerce to string keys for JSON.
		strMap := make(map[string]any, len(t))
		for k, v := range t {
			strMap[fmt.Sprintf("%v", k)] = v
		}
		return normalizeSortedKeys(strMap)
	case []any:
		out := make([]any, len(t))
		for i, v := range t {
			out[i] = normalizeSortedKeys(v)
		}
		return out
	default:
		return v
	}
}

// sortedMap is a map[string]any wrapper that preserves a chosen key order
// when JSON-encoded. encoding/json honors a custom MarshalJSON.
type sortedMap struct {
	keys []string
	m    map[string]any
}

func (s sortedMap) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, k := range s.keys {
		if i > 0 {
			buf.WriteByte(',')
		}
		kb, err := json.Marshal(k)
		if err != nil {
			return nil, err
		}
		buf.Write(kb)
		buf.WriteByte(':')
		vb, err := json.Marshal(s.m[k])
		if err != nil {
			return nil, err
		}
		buf.Write(vb)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}
