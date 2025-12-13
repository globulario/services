package main

import "encoding/json"

func asString(m map[string]any, key string) string {
    if v, ok := m[key]; ok && v != nil {
        if s, ok := v.(string); ok {
            return s
        }
    }
    return ""
}

func asInt64(m map[string]any, key string) int64 {
    if v, ok := m[key]; ok && v != nil {
        switch t := v.(type) {
        case float64:
            return int64(t)
        case int64:
            return t
        case int32:
            return int64(t)
        case json.Number:
            i, _ := t.Int64()
            return i
        }
    }
    return 0
}

func asInt32(m map[string]any, key string) int32 {
    return int32(asInt64(m, key))
}
