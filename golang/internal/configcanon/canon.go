package configcanon

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

// NormalizeConfig returns a canonical JSON byte slice for v and its SHA256 hex digest.
//
// Rules:
//   - Maps (map[string]T) are encoded with keys sorted lexicographically.
//   - Arrays/slices preserve order.
//   - Strings are JSON-escaped.
//   - Numbers use deterministic JSON forms (ints in base10, floats via strconv.FormatFloat).
//   - NaN/±Inf are rejected with an error.
//   - Invalid or unsupported types cause an error.
//
// Input may be a map[string]string or a generic JSON-like structure (map[string]any, []any, bool,
// string, numeric types, nil).
//
// Usage:
//
//	canon, digest, err := configcanon.NormalizeConfig(cfg)
//	// digest is lower-case SHA256 hex of canon.
func NormalizeConfig(v any) ([]byte, string, error) {
	var buf bytes.Buffer
	if err := encodeValue(&buf, v); err != nil {
		return nil, "", err
	}
	b := buf.Bytes()
	sum := sha256.Sum256(b)
	return b, hex.EncodeToString(sum[:]), nil
}

func encodeValue(buf *bytes.Buffer, v any) error {
	if v == nil {
		buf.WriteString("null")
		return nil
	}

	// Special case map[string]string -> map[string]any
	if m, ok := v.(map[string]string); ok {
		converted := make(map[string]any, len(m))
		for k, val := range m {
			converted[k] = val
		}
		return encodeObject(buf, converted)
	}

	switch val := v.(type) {
	case map[string]any:
		return encodeObject(buf, val)
	case []any:
		s := reflect.ValueOf(val)
		return encodeSlice(buf, s)
	case json.RawMessage:
		// Validate then write raw (assumed canonical already).
		if !json.Valid(val) {
			return fmt.Errorf("invalid raw JSON")
		}
		buf.Write(val)
		return nil
	case string:
		return encodeString(buf, val)
	case []byte:
		// treat as string
		return encodeString(buf, string(val))
	case bool:
		if val {
			buf.WriteString("true")
		} else {
			buf.WriteString("false")
		}
		return nil
	case json.Number:
		return encodeJSONNumber(buf, val)
	}

	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Map:
		if rv.Type().Key().Kind() != reflect.String {
			return fmt.Errorf("unsupported map key type: %s", rv.Type().Key())
		}
		iter := rv.MapRange()
		tmp := make(map[string]any, rv.Len())
		for iter.Next() {
			tmp[iter.Key().String()] = iter.Value().Interface()
		}
		return encodeObject(buf, tmp)
	case reflect.Slice, reflect.Array:
		return encodeSlice(buf, rv)
	case reflect.String:
		return encodeString(buf, rv.String())
	case reflect.Bool:
		if rv.Bool() {
			buf.WriteString("true")
		} else {
			buf.WriteString("false")
		}
		return nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		buf.WriteString(strconv.FormatInt(rv.Int(), 10))
		return nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		buf.WriteString(strconv.FormatUint(rv.Uint(), 10))
		return nil
	case reflect.Float32, reflect.Float64:
		f := rv.Convert(reflect.TypeOf(float64(0))).Float()
		if math.IsNaN(f) || math.IsInf(f, 0) {
			return fmt.Errorf("invalid float value %v", f)
		}
		buf.WriteString(strconv.FormatFloat(f, 'g', -1, 64))
		return nil
	case reflect.Interface:
		return encodeValue(buf, rv.Interface())
	case reflect.Pointer:
		if rv.IsNil() {
			buf.WriteString("null")
			return nil
		}
		return encodeValue(buf, rv.Elem().Interface())
	}

	return fmt.Errorf("unsupported type %T", v)
}

func encodeObject(buf *bytes.Buffer, m map[string]any) error {
	buf.WriteByte('{')
	if len(m) == 0 {
		buf.WriteByte('}')
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for i, k := range keys {
		if i > 0 {
			buf.WriteByte(',')
		}
		if err := encodeString(buf, k); err != nil {
			return err
		}
		buf.WriteByte(':')
		if err := encodeValue(buf, m[k]); err != nil {
			return err
		}
	}
	buf.WriteByte('}')
	return nil
}

func encodeSlice(buf *bytes.Buffer, rv reflect.Value) error {
	buf.WriteByte('[')
	n := rv.Len()
	for i := 0; i < n; i++ {
		if i > 0 {
			buf.WriteByte(',')
		}
		if err := encodeValue(buf, rv.Index(i).Interface()); err != nil {
			return err
		}
	}
	buf.WriteByte(']')
	return nil
}

func encodeString(buf *bytes.Buffer, s string) error {
	enc, err := json.Marshal(s)
	if err != nil {
		return err
	}
	buf.Write(enc)
	return nil
}

func encodeJSONNumber(buf *bytes.Buffer, num json.Number) error {
	// Validate numeric string using strconv.ParseFloat.
	if _, err := strconv.ParseFloat(num.String(), 64); err != nil {
		return fmt.Errorf("invalid number %q: %w", num.String(), err)
	}
	if strings.ContainsAny(num.String(), "NnIi") {
		return fmt.Errorf("invalid number %q", num.String())
	}
	buf.WriteString(num.String())
	return nil
}
