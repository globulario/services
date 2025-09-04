package main

import (
	"encoding/json"
	"strings"

	Utility "github.com/globulario/utility"
)

// sanitizeName returns a safe identifier for DB/user names.
func sanitizeName(s string) string {
    r := strings.NewReplacer(".", "_", "@", "_", "-", "_", " ", "_")
    return r.Replace(s)
}

// sanitizeDBRefArray iterates a roles/groups/orgs array that may be a []interface{} or primitive.A
// and ensures each element has a sanitized "$db" value.
func sanitizeDBRefArray(v interface{}) interface{} {
    arr := toAnySlice(v)
    for i := range arr {
        if m, ok := arr[i].(map[string]interface{}); ok {
            if db, ok2 := m["$db"].(string); ok2 {
                m["$db"] = sanitizeName(db)
            }
        }
    }
    return arr
}

// toAnySlice accepts []interface{} or primitive.A and returns []interface{}.
func toAnySlice(v interface{}) []interface{} {
    switch t := v.(type) {
    case []interface{}:
        return t
    default:
        // leave as-is; callers should type-assert when needed
        if aa, ok := anyToSlice(t); ok {
            return aa
        }
        return []interface{}{}
    }
}

// anyToSlice best-effort convert to []interface{}; supports primitive.A without importing it here.
func anyToSlice(v interface{}) ([]interface{}, bool) {
    // reflect way to avoid importing go.mongodb.org/mongo-driver/bson/primitive
    // since this helper is generic and we don't want extra deps here.
    // We'll try common shapes.
    if v == nil {
        return nil, false
    }
    switch vv := v.(type) {
    case []string:
        a := make([]interface{}, len(vv))
        for i := range vv { a[i] = vv[i] }
        return a, true
    }
    return nil, false
}



// That function is necessary to serialyse reference and kept field orders
func serialyseObject(obj map[string]interface{}) string {
	// Here I will save the role.
	jsonStr, _ := Utility.ToJson(obj)
	jsonStr = strings.ReplaceAll(jsonStr, `"$ref"`, `"__a__"`)
	jsonStr = strings.ReplaceAll(jsonStr, `"$id"`, `"__b__"`)
	jsonStr = strings.ReplaceAll(jsonStr, `"$db"`, `"__c__"`)

	obj_ := make(map[string]interface{}, 0)

	json.Unmarshal([]byte(jsonStr), &obj_)
	jsonStr, _ = Utility.ToJson(obj_)
	jsonStr = strings.ReplaceAll(jsonStr, `"__a__"`, `"$ref"`)
	jsonStr = strings.ReplaceAll(jsonStr, `"__b__"`, `"$id"`)
	jsonStr = strings.ReplaceAll(jsonStr, `"__c__"`, `"$db"`)

	return jsonStr
}
