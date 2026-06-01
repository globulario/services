package main

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	rbacpb "github.com/globulario/services/golang/rbac/rbacpb"
)

// ── Endpoint resolution ─────────────────────────────────────────────────────

func rbacEndpoint() string {
	return gatewayEndpoint()
}

// ── Subject-type helpers ────────────────────────────────────────────────────

var subjectTypeMap = map[string]int32{
	"account":       0,
	"node_identity": 1,
	"group":         2,
	"organization":  3,
	"application":   4,
	"role":          5,
}

var subjectTypeNames = map[int32]string{
	0: "account",
	1: "node_identity",
	2: "group",
	3: "organization",
	4: "application",
	5: "role",
}

func subjectTypeFromString(s string) int32 {
	if v, ok := subjectTypeMap[strings.ToLower(s)]; ok {
		return v
	}
	return 0
}

func subjectTypeToString(t int32) string {
	if s, ok := subjectTypeNames[t]; ok {
		return s
	}
	return "account"
}

// ── Permission normalization ────────────────────────────────────────────────

func normalizePermission(p *rbacpb.Permission) map[string]interface{} {
	if p == nil {
		return map[string]interface{}{}
	}
	return map[string]interface{}{
		"permission":      p.GetName(),
		"accounts":        p.GetAccounts(),
		"groups":          p.GetGroups(),
		"organizations":   p.GetOrganizations(),
		"applications":    p.GetApplications(),
		"node_identities": p.GetNodeIdentities(),
	}
}

func normalizePermissions(ps *rbacpb.Permissions) map[string]interface{} {
	if ps == nil {
		return map[string]interface{}{}
	}

	allowed := make([]map[string]interface{}, 0, len(ps.GetAllowed()))
	for _, p := range ps.GetAllowed() {
		allowed = append(allowed, normalizePermission(p))
	}

	denied := make([]map[string]interface{}, 0, len(ps.GetDenied()))
	for _, p := range ps.GetDenied() {
		denied = append(denied, normalizePermission(p))
	}

	owners := map[string]interface{}{}
	if o := ps.GetOwners(); o != nil {
		owners = map[string]interface{}{
			"accounts":        o.GetAccounts(),
			"groups":          o.GetGroups(),
			"organizations":   o.GetOrganizations(),
			"applications":    o.GetApplications(),
			"node_identities": o.GetNodeIdentities(),
		}
	}

	return map[string]interface{}{
		"path":          ps.GetPath(),
		"resource_type": ps.GetResourceType(),
		"owners":        owners,
		"allowed":       allowed,
		"denied":        denied,
	}
}

// ── Tool registration ───────────────────────────────────────────────────────

func registerRbacTools(s *server) {

	// ── 1. rbac_validate_access ─────────────────────────────────────────────
	s.register(toolDef{
		Name:        "rbac_validate_access",
		Description: "Check whether a subject (account, group, etc.) has a specific permission on a resource path. Returns allowed/denied status with details.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"subject": {
					Type:        "string",
					Description: "Subject identifier (e.g. 'sa', 'admin', a user ID).",
				},
				"subject_type": {
					Type:        "string",
					Description: "Type of the subject.",
					Enum:        []string{"account", "node_identity", "group", "organization", "application", "role"},
				},
				"path": {
					Type:        "string",
					Description: "Resource path to validate access for (e.g. '/users/admin').",
				},
				"permission": {
					Type:        "string",
					Description: "Permission name to check (e.g. 'read', 'write', 'delete').",
				},
			},
			Required: []string{"subject", "subject_type", "path", "permission"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		subject := getStr(args, "subject")
		subjectType := getStr(args, "subject_type")
		path := getStr(args, "path")
		permission := getStr(args, "permission")
		if subject == "" || path == "" || permission == "" {
			return nil, fmt.Errorf("missing required arguments: subject, path, and permission")
		}

		conn, err := s.clients.get(ctx, rbacEndpoint())
		if err != nil {
			return nil, err
		}
		client := rbacpb.NewRbacServiceClient(conn)

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 10*time.Second)
		defer cancel()

		resp, err := client.ValidateAccess(callCtx, &rbacpb.ValidateAccessRqst{
			Subject:    subject,
			Type:       rbacpb.SubjectType(subjectTypeFromString(subjectType)),
			Path:       path,
			Permission: permission,
		})
		if err != nil {
			return nil, fmt.Errorf("ValidateAccess: %w", err)
		}

		hasAccess := resp.GetHasAccess()
		accessDenied := resp.GetAccessDenied()

		status := "denied"
		if hasAccess {
			status = "allowed"
		}

		summary := fmt.Sprintf("Access ALLOWED for %s '%s' to path '%s' with permission '%s'",
			subjectType, subject, path, permission)
		if !hasAccess {
			if accessDenied {
				summary = fmt.Sprintf("Access DENIED (explicit deny present) for %s '%s' to path '%s' with permission '%s'",
					subjectType, subject, path, permission)
			} else {
				summary = fmt.Sprintf("Access DENIED for %s '%s' to path '%s' with permission '%s'",
					subjectType, subject, path, permission)
			}
		}

		return map[string]interface{}{
			"status":        status,
			"has_access":    hasAccess,
			"access_denied": accessDenied,
			"summary":       summary,
			"recommended_followup": []string{
				"rbac_get_resource_permissions",
				"rbac_explain_access_snapshot",
			},
		}, nil
	})

	// ── 2. rbac_validate_action ─────────────────────────────────────────────
	s.register(toolDef{
		Name:        "rbac_validate_action",
		Description: "Validate whether a subject can perform an action involving multiple resources. Returns allowed/denied status.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"subject": {
					Type:        "string",
					Description: "Subject identifier.",
				},
				"subject_type": {
					Type:        "string",
					Description: "Type of the subject.",
					Enum:        []string{"account", "node_identity", "group", "organization", "application", "role"},
				},
				"action": {
					Type:        "string",
					Description: "The action path (e.g. gRPC method path).",
				},
				"infos": {
					Type:        "array",
					Description: "Optional array of resource info objects with fields: index (int), permission (string), path (string), field (string).",
				},
			},
			Required: []string{"subject", "subject_type", "action"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		subject := getStr(args, "subject")
		subjectType := getStr(args, "subject_type")
		action := getStr(args, "action")
		if subject == "" || action == "" {
			return nil, fmt.Errorf("missing required arguments: subject and action")
		}

		conn, err := s.clients.get(ctx, rbacEndpoint())
		if err != nil {
			return nil, err
		}
		client := rbacpb.NewRbacServiceClient(conn)

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 10*time.Second)
		defer cancel()

		var infos []*rbacpb.ResourceInfos
		if raw, ok := args["infos"]; ok {
			if arr, ok := raw.([]interface{}); ok {
				for _, item := range arr {
					if m, ok := item.(map[string]interface{}); ok {
						info := &rbacpb.ResourceInfos{
							Index:      int32(getInt(m, "index", 0)),
							Permission: getStr(m, "permission"),
							Path:       getStr(m, "path"),
							Field:      getStr(m, "field"),
						}
						infos = append(infos, info)
					}
				}
			}
		}

		resp, err := client.ValidateAction(callCtx, &rbacpb.ValidateActionRqst{
			Subject: subject,
			Type:    rbacpb.SubjectType(subjectTypeFromString(subjectType)),
			Action:  action,
			Infos:   infos,
		})
		if err != nil {
			return nil, fmt.Errorf("ValidateAction: %w", err)
		}

		hasAccess := resp.GetHasAccess()
		status := "denied"
		if hasAccess {
			status = "allowed"
		}

		summary := fmt.Sprintf("Action '%s' is %s for %s '%s'", action, strings.ToUpper(status), subjectType, subject)

		return map[string]interface{}{
			"status":  status,
			"allowed": hasAccess,
			"subject": subject,
			"action":  action,
			"summary": summary,
			"recommended_followup": []string{
				"rbac_get_action_resource_infos",
				"rbac_get_resource_permissions",
			},
		}, nil
	})

	// ── 3. rbac_get_action_resource_infos ───────────────────────────────────
	s.register(toolDef{
		Name:        "rbac_get_action_resource_infos",
		Description: "Get the resource information (permissions, paths, fields) required by an action. Useful for understanding what resources an action checks.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"action": {
					Type:        "string",
					Description: "The action path (e.g. gRPC method path).",
				},
			},
			Required: []string{"action"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		action := getStr(args, "action")
		if action == "" {
			return nil, fmt.Errorf("missing required argument: action")
		}

		conn, err := s.clients.get(ctx, rbacEndpoint())
		if err != nil {
			return nil, err
		}
		client := rbacpb.NewRbacServiceClient(conn)

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 10*time.Second)
		defer cancel()

		resp, err := client.GetActionResourceInfos(callCtx, &rbacpb.GetActionResourceInfosRqst{
			Action: action,
		})
		if err != nil {
			return nil, fmt.Errorf("GetActionResourceInfos: %w", err)
		}

		infos := make([]map[string]interface{}, 0, len(resp.GetInfos()))
		for _, ri := range resp.GetInfos() {
			infos = append(infos, map[string]interface{}{
				"index":      ri.GetIndex(),
				"permission": ri.GetPermission(),
				"path":       ri.GetPath(),
				"field":      ri.GetField(),
			})
		}

		return map[string]interface{}{
			"action":         action,
			"resource_infos": infos,
			"summary":        fmt.Sprintf("Action '%s' requires %d resource checks", action, len(infos)),
		}, nil
	})

	// ── 4. rbac_get_resource_permissions ────────────────────────────────────
	s.register(toolDef{
		Name:        "rbac_get_resource_permissions",
		Description: "Get the full permissions (owners, allowed, denied) for a specific resource path.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"path": {
					Type:        "string",
					Description: "Resource path to query (e.g. '/users/admin', '/applications/myapp').",
				},
			},
			Required: []string{"path"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		path := getStr(args, "path")
		if path == "" {
			return nil, fmt.Errorf("missing required argument: path")
		}

		conn, err := s.clients.get(ctx, rbacEndpoint())
		if err != nil {
			return nil, err
		}
		client := rbacpb.NewRbacServiceClient(conn)

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 10*time.Second)
		defer cancel()

		resp, err := client.GetResourcePermissions(callCtx, &rbacpb.GetResourcePermissionsRqst{
			Path: path,
		})
		if err != nil {
			return nil, fmt.Errorf("GetResourcePermissions: %w", err)
		}

		perms := resp.GetPermissions()
		result := normalizePermissions(perms)

		// Add summary.
		allowCount := 0
		denyCount := 0
		ownerCount := 0
		if perms != nil {
			allowCount = len(perms.GetAllowed())
			denyCount = len(perms.GetDenied())
			if o := perms.GetOwners(); o != nil {
				ownerCount = len(o.GetAccounts()) + len(o.GetGroups()) + len(o.GetOrganizations()) +
					len(o.GetApplications()) + len(o.GetNodeIdentities())
			}
		}
		result["summary"] = fmt.Sprintf("Resource has %d owners, %d allow rules, %d deny rules",
			ownerCount, allowCount, denyCount)

		return result, nil
	})

	// ── 5. rbac_get_permissions_by_subject ──────────────────────────────────
	s.register(toolDef{
		Name:        "rbac_get_permissions_by_subject",
		Description: "Get all resource permissions for a specific subject (account, group, etc.). Streams results from the RBAC service.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"subject": {
					Type:        "string",
					Description: "Subject identifier.",
				},
				"subject_type": {
					Type:        "string",
					Description: "Type of the subject.",
					Enum:        []string{"account", "node_identity", "group", "organization", "application", "role"},
				},
				"resource_type": {
					Type:        "string",
					Description: "Filter by resource type.",
				},
			},
			Required: []string{"subject", "subject_type", "resource_type"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		subject := getStr(args, "subject")
		subjectType := getStr(args, "subject_type")
		resourceType := getStr(args, "resource_type")
		if subject == "" || resourceType == "" {
			return nil, fmt.Errorf("missing required arguments: subject and resource_type")
		}

		conn, err := s.clients.get(ctx, rbacEndpoint())
		if err != nil {
			return nil, err
		}
		client := rbacpb.NewRbacServiceClient(conn)

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 10*time.Second)
		defer cancel()

		stream, err := client.GetResourcePermissionsBySubject(callCtx, &rbacpb.GetResourcePermissionsBySubjectRqst{
			Subject:      subject,
			SubjectType:  rbacpb.SubjectType(subjectTypeFromString(subjectType)),
			ResourceType: resourceType,
		})
		if err != nil {
			return nil, fmt.Errorf("GetResourcePermissionsBySubject: %w", err)
		}

		var results []map[string]interface{}
		for {
			resp, err := stream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, fmt.Errorf("GetResourcePermissionsBySubject stream: %w", err)
			}
			for _, p := range resp.GetPermissions() {
				results = append(results, normalizePermissions(p))
			}
		}

		return map[string]interface{}{
			"subject":       subject,
			"subject_type":  subjectType,
			"resource_type": resourceType,
			"count":         len(results),
			"permissions":   results,
		}, nil
	})

	// ── 6. rbac_get_permissions_by_resource_type ────────────────────────────
	s.register(toolDef{
		Name:        "rbac_get_permissions_by_resource_type",
		Description: "Get all resource permissions for a given resource type. Streams results from the RBAC service.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"resource_type": {
					Type:        "string",
					Description: "The resource type name to query.",
				},
			},
			Required: []string{"resource_type"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		resourceType := getStr(args, "resource_type")
		if resourceType == "" {
			return nil, fmt.Errorf("missing required argument: resource_type")
		}

		conn, err := s.clients.get(ctx, rbacEndpoint())
		if err != nil {
			return nil, err
		}
		client := rbacpb.NewRbacServiceClient(conn)

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 10*time.Second)
		defer cancel()

		stream, err := client.GetResourcePermissionsByResourceType(callCtx, &rbacpb.GetResourcePermissionsByResourceTypeRqst{
			ResourceType: resourceType,
		})
		if err != nil {
			return nil, fmt.Errorf("GetResourcePermissionsByResourceType: %w", err)
		}

		var results []map[string]interface{}
		for {
			resp, err := stream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, fmt.Errorf("GetResourcePermissionsByResourceType stream: %w", err)
			}
			for _, p := range resp.GetPermissions() {
				results = append(results, normalizePermissions(p))
			}
		}

		return map[string]interface{}{
			"resource_type": resourceType,
			"count":         len(results),
			"permissions":   results,
		}, nil
	})

	// ── 7. rbac_get_role_binding ────────────────────────────────────────────
	s.register(toolDef{
		Name:        "rbac_get_role_binding",
		Description: "Get the role binding for a specific subject. Returns the list of roles assigned to the subject.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"subject": {
					Type:        "string",
					Description: "Subject identifier (user, service account, etc.).",
				},
			},
			Required: []string{"subject"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		subject := getStr(args, "subject")
		if subject == "" {
			return nil, fmt.Errorf("missing required argument: subject")
		}

		conn, err := s.clients.get(ctx, rbacEndpoint())
		if err != nil {
			return nil, err
		}
		client := rbacpb.NewRbacServiceClient(conn)

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 10*time.Second)
		defer cancel()

		resp, err := client.GetRoleBinding(callCtx, &rbacpb.GetRoleBindingRqst{
			Subject: subject,
		})
		if err != nil {
			return nil, fmt.Errorf("GetRoleBinding: %w", err)
		}

		binding := resp.GetBinding()
		roles := []string{}
		if binding != nil {
			roles = binding.GetRoles()
		}

		return map[string]interface{}{
			"subject":    subject,
			"roles":      roles,
			"role_count": len(roles),
			"summary":    fmt.Sprintf("Subject '%s' has %d role(s): %s", subject, len(roles), strings.Join(roles, ", ")),
		}, nil
	})

	// ── 8. rbac_list_role_bindings ──────────────────────────────────────────
	s.register(toolDef{
		Name:        "rbac_list_role_bindings",
		Description: "List all role bindings, optionally filtered by subject. Streams results from the RBAC service.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"subject": {
					Type:        "string",
					Description: "Optional subject filter. If empty, lists all bindings.",
				},
			},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		conn, err := s.clients.get(ctx, rbacEndpoint())
		if err != nil {
			return nil, err
		}
		client := rbacpb.NewRbacServiceClient(conn)

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 10*time.Second)
		defer cancel()

		stream, err := client.ListRoleBindings(callCtx, &rbacpb.ListRoleBindingsRqst{})
		if err != nil {
			return nil, fmt.Errorf("ListRoleBindings: %w", err)
		}

		subjectFilter := getStr(args, "subject")

		var bindings []map[string]interface{}
		for {
			resp, err := stream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, fmt.Errorf("ListRoleBindings stream: %w", err)
			}
			b := resp.GetBinding()
			if b == nil {
				continue
			}
			// Apply optional client-side filter.
			if subjectFilter != "" && !strings.Contains(b.GetSubject(), subjectFilter) {
				continue
			}
			bindings = append(bindings, map[string]interface{}{
				"subject": b.GetSubject(),
				"roles":   b.GetRoles(),
			})
		}

		return map[string]interface{}{
			"count":    len(bindings),
			"bindings": bindings,
		}, nil
	})
}
