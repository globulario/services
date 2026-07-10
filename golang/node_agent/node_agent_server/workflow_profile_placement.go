package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/globulario/services/golang/component_catalog"
)

func workflowProfilesFromInput(raw any) []string {
	switch v := raw.(type) {
	case nil:
		return nil
	case []string:
		return append([]string(nil), v...)
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			out = append(out, workflowProfilesFromInput(fmt.Sprint(item))...)
		}
		return out
	case string:
		return splitWorkflowProfiles(v)
	default:
		return splitWorkflowProfiles(fmt.Sprint(v))
	}
}

func splitWorkflowProfiles(raw string) []string {
	s := strings.TrimSpace(raw)
	if s == "" || s == "<nil>" {
		return nil
	}
	if strings.HasPrefix(s, "[") {
		var arr []string
		if err := json.Unmarshal([]byte(s), &arr); err == nil {
			return arr
		}
		var anyArr []any
		if err := json.Unmarshal([]byte(s), &anyArr); err == nil {
			return workflowProfilesFromInput(anyArr)
		}
	}
	fields := strings.FieldsFunc(s, func(r rune) bool {
		switch r {
		case ',', ';', ' ', '\t', '\n', '\r':
			return true
		default:
			return false
		}
	})
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		if p := strings.TrimSpace(field); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func workflowProfilesAny(profiles []string) []any {
	out := make([]any, 0, len(profiles))
	for _, p := range profiles {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func workflowJoinPlanProfiles(planJSON []byte) []string {
	if len(planJSON) == 0 {
		return nil
	}
	var plan nodeJoinPlan
	if err := json.Unmarshal(planJSON, &plan); err != nil {
		return nil
	}
	return append([]string(nil), plan.AssignedProfiles...)
}

func (srv *NodeAgentServer) ensureWorkflowNodeProfiles(workflowName string, inputs map[string]any) ([]string, error) {
	if inputs == nil {
		inputs = make(map[string]any)
	}

	raw := workflowProfilesFromInput(inputs["node_profiles"])
	if len(raw) == 0 && srv != nil && srv.state != nil {
		raw = workflowJoinPlanProfiles(srv.state.JoinPlanJSON)
	}
	profiles := component_catalog.NormalizeProfiles(raw)
	if len(profiles) > 0 {
		inputs["node_profiles"] = workflowProfilesAny(profiles)
	}

	if workflowName != "node.join" {
		return profiles, nil
	}
	if len(raw) == 0 {
		return nil, fmt.Errorf("node.join requires assigned node profiles")
	}
	if unknown := component_catalog.UnknownProfiles(raw); len(unknown) > 0 {
		return nil, fmt.Errorf("node.join has unknown assigned profiles %v (known profiles: %v)", unknown, component_catalog.ProfileNames())
	}
	if len(profiles) == 0 {
		return nil, fmt.Errorf("node.join resolved no installable profiles from %v", raw)
	}
	inputs["node_profiles"] = workflowProfilesAny(profiles)
	return profiles, nil
}

func workflowPackageAllowedForProfiles(packageName string, profiles []string) bool {
	name := strings.ToLower(strings.TrimSpace(packageName))
	if name == "" {
		return false
	}
	allowed := component_catalog.PackagesForProfiles(profiles)
	for _, candidate := range allowed {
		if candidate == name {
			return true
		}
	}
	return false
}
