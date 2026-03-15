package main

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	rbacpb "github.com/globulario/services/golang/rbac/rbacpb"
	resourcepb "github.com/globulario/services/golang/resource/resourcepb"
)

// ── Account context helper ──────────────────────────────────────────────────

func getAccountContext(ctx context.Context, pool *clientPool, accountID string) (map[string]interface{}, error) {
	conn, err := pool.get(ctx, resourceEndpoint())
	if err != nil {
		return nil, err
	}
	client := resourcepb.NewResourceServiceClient(conn)
	callCtx, cancel := context.WithTimeout(authCtx(ctx), 10*time.Second)
	defer cancel()

	resp, err := client.GetAccount(callCtx, &resourcepb.GetAccountRqst{AccountId: accountID})
	if err != nil {
		return nil, err
	}
	acc := resp.GetAccount()
	return map[string]interface{}{
		"groups":        acc.GetGroups(),
		"organizations": acc.GetOrganizations(),
		"direct_roles":  acc.GetRoles(),
	}, nil
}

// ── Permission serialization helpers ────────────────────────────────────────

func permissionToMap(p *rbacpb.Permission) map[string]interface{} {
	if p == nil {
		return nil
	}
	return map[string]interface{}{
		"name":            p.GetName(),
		"accounts":        p.GetAccounts(),
		"groups":          p.GetGroups(),
		"organizations":   p.GetOrganizations(),
		"applications":    p.GetApplications(),
		"node_identities": p.GetNodeIdentities(),
	}
}

func permissionsToMap(p *rbacpb.Permissions) map[string]interface{} {
	if p == nil {
		return nil
	}
	allowed := make([]map[string]interface{}, 0, len(p.GetAllowed()))
	for _, a := range p.GetAllowed() {
		allowed = append(allowed, permissionToMap(a))
	}
	denied := make([]map[string]interface{}, 0, len(p.GetDenied()))
	for _, d := range p.GetDenied() {
		denied = append(denied, permissionToMap(d))
	}
	return map[string]interface{}{
		"path":          p.GetPath(),
		"resource_type": p.GetResourceType(),
		"owners":        permissionToMap(p.GetOwners()),
		"allowed":       allowed,
		"denied":        denied,
	}
}

// resourceTypeFromPath extracts a heuristic resource type from a path.
// e.g. "/applications/myapp" -> "application", "/files/foo" -> "file"
func resourceTypeFromPath(path string) string {
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		return ""
	}
	// Strip trailing 's' for a rough singular form.
	t := parts[0]
	t = strings.TrimSuffix(t, "s")
	return t
}

// subjectInPermission checks whether the given subject appears in a Permission entry
// according to its subject type.
func subjectInPermission(p *rbacpb.Permission, subject, subjectType string) bool {
	if p == nil {
		return false
	}
	switch subjectType {
	case "account":
		for _, a := range p.GetAccounts() {
			if a == subject {
				return true
			}
		}
	case "group":
		for _, g := range p.GetGroups() {
			if g == subject {
				return true
			}
		}
	case "organization":
		for _, o := range p.GetOrganizations() {
			if o == subject {
				return true
			}
		}
	case "application":
		for _, a := range p.GetApplications() {
			if a == subject {
				return true
			}
		}
	case "node_identity":
		for _, n := range p.GetNodeIdentities() {
			if n == subject {
				return true
			}
		}
	}
	return false
}

// ── Heuristic analysis ──────────────────────────────────────────────────────

func analyzeAccessDecision(
	hasAccess bool,
	accessDenied bool,
	subject string,
	permission string,
	path string,
	resourcePerms *rbacpb.Permissions,
	subjectPerms []*rbacpb.Permissions,
	roles []string,
	subjectType string,
) (summary string, causes []string, suggestions []string) {

	if accessDenied {
		causes = append(causes, fmt.Sprintf("Explicit deny rule found for subject '%s' on path '%s'", subject, path))
		suggestions = append(suggestions, "Check denied rules with: rbac_get_resource_permissions")
		suggestions = append(suggestions, "Remove the deny rule if it was set in error")
	}

	if !hasAccess && !accessDenied {
		causes = append(causes, fmt.Sprintf("No matching allow rule found for subject '%s' with permission '%s' on path '%s'", subject, permission, path))

		if len(subjectPerms) == 0 {
			causes = append(causes, fmt.Sprintf("Subject '%s' has no permissions on any resource of this type", subject))
			suggestions = append(suggestions, fmt.Sprintf("Add subject '%s' to an allow rule or a group/role that has access", subject))
		}

		// Check if the subject appears in any allow rule at all for this resource.
		if resourcePerms != nil {
			foundInAllow := false
			for _, a := range resourcePerms.GetAllowed() {
				if subjectInPermission(a, subject, subjectType) {
					foundInAllow = true
					break
				}
			}
			if foundInAllow {
				causes = append(causes, fmt.Sprintf("Subject appears in an allow rule on this path but not for permission '%s'", permission))
			}
		}

		// Check if role binding exists but roles may lack the permission.
		if len(roles) > 0 {
			causes = append(causes, fmt.Sprintf("Roles assigned (%s) but none grant '%s' permission on '%s'", strings.Join(roles, ", "), permission, path))
			suggestions = append(suggestions, fmt.Sprintf("Inspect role actions with: rbac_get_role_binding for subject '%s'", subject))
		}
	}

	if hasAccess {
		// Determine why access was granted.
		if resourcePerms != nil {
			// Check owner.
			if subjectInPermission(resourcePerms.GetOwners(), subject, subjectType) {
				causes = append(causes, fmt.Sprintf("Subject '%s' is an owner of resource '%s'", subject, path))
			}
			// Check allow rules.
			for _, a := range resourcePerms.GetAllowed() {
				if a.GetName() == permission && subjectInPermission(a, subject, subjectType) {
					causes = append(causes, fmt.Sprintf("Subject '%s' has an explicit allow rule for '%s' on '%s'", subject, permission, path))
					break
				}
			}
		}
		if len(roles) > 0 {
			causes = append(causes, fmt.Sprintf("Subject has roles: %s (one of these may grant access)", strings.Join(roles, ", ")))
		}
	}

	if len(causes) == 0 {
		causes = append(causes, "Unable to determine specific root cause from available evidence")
		suggestions = append(suggestions, "Manually inspect RBAC rules for this path and subject")
	}

	// Build summary.
	decision := "denied"
	if hasAccess {
		decision = "allowed"
	}
	summary = fmt.Sprintf("Access %s for subject '%s' on path '%s' with permission '%s'. ", decision, subject, path, permission)
	if len(causes) > 0 {
		summary += causes[0]
	}

	return summary, causes, suggestions
}

// ── Tool registration ───────────────────────────────────────────────────────

func registerRbacExplainTools(s *server) {

	// ── rbac_explain_access_snapshot ─────────────────────────────────────
	s.register(toolDef{
		Name: "rbac_explain_access_snapshot",
		Description: "The highest-value RBAC diagnostic tool: explains WHY a subject has or doesn't have access to a resource. " +
			"Calls ValidateAccess, GetResourcePermissions, GetResourcePermissionsBySubject, GetRoleBinding, and (for accounts) GetAccount in parallel, " +
			"then performs heuristic analysis to produce a human-readable diagnosis with likely root causes and suggested fixes.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"subject":      {Type: "string", Description: "Subject identifier (e.g. account ID, group name)"},
				"subject_type": {Type: "string", Description: "Type of the subject", Enum: []string{"account", "node_identity", "group", "organization", "application", "role"}},
				"path":         {Type: "string", Description: "Resource path to check access on (e.g. '/files/private')"},
				"permission":   {Type: "string", Description: "Permission to check (e.g. 'read', 'write', 'delete')"},
			},
			Required: []string{"subject", "subject_type", "path", "permission"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		subject := getStr(args, "subject")
		subjectType := getStr(args, "subject_type")
		path := getStr(args, "path")
		permission := getStr(args, "permission")

		if subject == "" || subjectType == "" || path == "" || permission == "" {
			return nil, fmt.Errorf("subject, subject_type, path, and permission are all required")
		}

		outerCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		var (
			mu              sync.Mutex
			errors          []string
			hasAccess       bool
			accessDenied    bool
			resourcePerms   *rbacpb.Permissions
			subjectPerms    []*rbacpb.Permissions
			roles           []string
			identityGraph   map[string]interface{}
			validateResult  map[string]interface{}
		)

		stEnum := rbacpb.SubjectType(subjectTypeFromString(subjectType))
		resType := resourceTypeFromPath(path)

		var wg sync.WaitGroup

		// a) ValidateAccess
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn, err := s.clients.get(outerCtx, rbacEndpoint())
			if err != nil {
				mu.Lock()
				errors = append(errors, fmt.Sprintf("RBAC service unavailable: %v", err))
				mu.Unlock()
				return
			}
			client := rbacpb.NewRbacServiceClient(conn)
			callCtx, callCancel := context.WithTimeout(authCtx(outerCtx), 10*time.Second)
			defer callCancel()

			resp, err := client.ValidateAccess(callCtx, &rbacpb.ValidateAccessRqst{
				Subject:    subject,
				Type:       stEnum,
				Path:       path,
				Permission: permission,
			})
			if err != nil {
				mu.Lock()
				errors = append(errors, fmt.Sprintf("ValidateAccess: %v", err))
				mu.Unlock()
				return
			}

			mu.Lock()
			hasAccess = resp.GetHasAccess()
			accessDenied = resp.GetAccessDenied()
			validateResult = map[string]interface{}{
				"has_access":    resp.GetHasAccess(),
				"access_denied": resp.GetAccessDenied(),
			}
			mu.Unlock()
		}()

		// b) GetResourcePermissions
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn, err := s.clients.get(outerCtx, rbacEndpoint())
			if err != nil {
				return // error captured by (a)
			}
			client := rbacpb.NewRbacServiceClient(conn)
			callCtx, callCancel := context.WithTimeout(authCtx(outerCtx), 10*time.Second)
			defer callCancel()

			resp, err := client.GetResourcePermissions(callCtx, &rbacpb.GetResourcePermissionsRqst{
				Path: path,
			})
			if err != nil {
				mu.Lock()
				errors = append(errors, fmt.Sprintf("GetResourcePermissions: %v", err))
				mu.Unlock()
				return
			}

			mu.Lock()
			resourcePerms = resp.GetPermissions()
			mu.Unlock()
		}()

		// c) GetResourcePermissionsBySubject (streaming)
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn, err := s.clients.get(outerCtx, rbacEndpoint())
			if err != nil {
				return
			}
			client := rbacpb.NewRbacServiceClient(conn)
			callCtx, callCancel := context.WithTimeout(authCtx(outerCtx), 10*time.Second)
			defer callCancel()

			stream, err := client.GetResourcePermissionsBySubject(callCtx, &rbacpb.GetResourcePermissionsBySubjectRqst{
				Subject:      subject,
				SubjectType:  stEnum,
				ResourceType: resType,
			})
			if err != nil {
				mu.Lock()
				errors = append(errors, fmt.Sprintf("GetResourcePermissionsBySubject: %v", err))
				mu.Unlock()
				return
			}

			var collected []*rbacpb.Permissions
			for {
				resp, err := stream.Recv()
				if err == io.EOF {
					break
				}
				if err != nil {
					mu.Lock()
					errors = append(errors, fmt.Sprintf("GetResourcePermissionsBySubject stream: %v", err))
					mu.Unlock()
					break
				}
				collected = append(collected, resp.GetPermissions()...)
			}

			mu.Lock()
			subjectPerms = collected
			mu.Unlock()
		}()

		// d) GetRoleBinding
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn, err := s.clients.get(outerCtx, rbacEndpoint())
			if err != nil {
				return
			}
			client := rbacpb.NewRbacServiceClient(conn)
			callCtx, callCancel := context.WithTimeout(authCtx(outerCtx), 10*time.Second)
			defer callCancel()

			resp, err := client.GetRoleBinding(callCtx, &rbacpb.GetRoleBindingRqst{
				Subject: subject,
			})
			if err != nil {
				mu.Lock()
				errors = append(errors, fmt.Sprintf("GetRoleBinding: %v", err))
				mu.Unlock()
				return
			}

			if b := resp.GetBinding(); b != nil {
				mu.Lock()
				roles = b.GetRoles()
				mu.Unlock()
			}
		}()

		// e) GetAccount (only for account subject type)
		if subjectType == "account" {
			wg.Add(1)
			go func() {
				defer wg.Done()
				accCtx, err := getAccountContext(outerCtx, s.clients, subject)
				if err != nil {
					mu.Lock()
					errors = append(errors, fmt.Sprintf("GetAccount: %v", err))
					mu.Unlock()
					return
				}

				mu.Lock()
				identityGraph = accCtx
				mu.Unlock()
			}()
		}

		wg.Wait()

		// Build evidence.
		evidence := map[string]interface{}{}

		if validateResult != nil {
			evidence["validate_access_result"] = validateResult
		}
		if resourcePerms != nil {
			evidence["resource_permissions"] = permissionsToMap(resourcePerms)
		}

		subjectPermsList := make([]map[string]interface{}, 0, len(subjectPerms))
		for _, sp := range subjectPerms {
			subjectPermsList = append(subjectPermsList, permissionsToMap(sp))
		}
		evidence["subject_permissions"] = subjectPermsList

		roleBindingEvidence := map[string]interface{}{
			"roles": roles,
		}
		evidence["role_binding"] = roleBindingEvidence

		if identityGraph != nil {
			// Merge inherited roles from groups/orgs into the identity graph.
			inheritedRoles := []string{}
			if groups, ok := identityGraph["groups"].([]string); ok {
				for _, g := range groups {
					inheritedRoles = append(inheritedRoles, fmt.Sprintf("(via group '%s')", g))
				}
			}
			if orgs, ok := identityGraph["organizations"].([]string); ok {
				for _, o := range orgs {
					inheritedRoles = append(inheritedRoles, fmt.Sprintf("(via organization '%s')", o))
				}
			}
			identityGraph["inherited_roles"] = inheritedRoles
			evidence["identity_graph"] = identityGraph
		}

		// Analyze.
		allRoles := make([]string, 0, len(roles))
		allRoles = append(allRoles, roles...)
		if identityGraph != nil {
			if directRoles, ok := identityGraph["direct_roles"].([]string); ok {
				for _, r := range directRoles {
					found := false
					for _, existing := range allRoles {
						if existing == r {
							found = true
							break
						}
					}
					if !found {
						allRoles = append(allRoles, r)
					}
				}
			}
		}

		summary, causes, suggestions := analyzeAccessDecision(
			hasAccess,
			accessDenied,
			subject,
			permission,
			path,
			resourcePerms,
			subjectPerms,
			allRoles,
			subjectType,
		)

		decision := "denied"
		if hasAccess {
			decision = "allowed"
		}

		return map[string]interface{}{
			"decision":              decision,
			"has_access":            hasAccess,
			"explicit_deny":         accessDenied,
			"summary":               summary,
			"evidence":              evidence,
			"likely_root_causes":    causes,
			"suggested_next_checks": suggestions,
			"errors":                errors,
		}, nil
	})

	// ── rbac_explain_action_snapshot ─────────────────────────────────────
	s.register(toolDef{
		Name: "rbac_explain_action_snapshot",
		Description: "Explains WHY a subject can or cannot perform a gRPC action (which may require multiple resource checks). " +
			"Retrieves the action's resource infos, validates the action overall, then validates each individual resource to find " +
			"which specific check failed. Includes role binding and identity graph for accounts.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"subject":      {Type: "string", Description: "Subject identifier (e.g. account ID)"},
				"subject_type": {Type: "string", Description: "Type of the subject", Enum: []string{"account", "node_identity", "group", "organization", "application", "role"}},
				"action":       {Type: "string", Description: "The action path (e.g. '/file.FileService/Upload')"},
			},
			Required: []string{"subject", "subject_type", "action"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		subject := getStr(args, "subject")
		subjectType := getStr(args, "subject_type")
		action := getStr(args, "action")

		if subject == "" || subjectType == "" || action == "" {
			return nil, fmt.Errorf("subject, subject_type, and action are all required")
		}

		outerCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		stEnum := rbacpb.SubjectType(subjectTypeFromString(subjectType))

		// Phase 1: Get action resource infos (needed before we can do per-resource checks).
		var actionInfos []*rbacpb.ResourceInfos
		var actionInfosErr error
		{
			conn, err := s.clients.get(outerCtx, rbacEndpoint())
			if err != nil {
				return nil, fmt.Errorf("RBAC service unavailable: %w", err)
			}
			client := rbacpb.NewRbacServiceClient(conn)
			callCtx, callCancel := context.WithTimeout(authCtx(outerCtx), 10*time.Second)
			defer callCancel()

			resp, err := client.GetActionResourceInfos(callCtx, &rbacpb.GetActionResourceInfosRqst{
				Action: action,
			})
			if err != nil {
				actionInfosErr = err
			} else {
				actionInfos = resp.GetInfos()
			}
		}

		// Phase 2: Parallel calls — ValidateAction, per-resource ValidateAccess, GetRoleBinding, GetAccount.
		var (
			mu             sync.Mutex
			errors         []string
			actionHasAccess    bool
			actionAccessDenied bool
			resourceChecks     []map[string]interface{}
			roles              []string
			identityGraph      map[string]interface{}
		)

		if actionInfosErr != nil {
			errors = append(errors, fmt.Sprintf("GetActionResourceInfos: %v", actionInfosErr))
		}

		var wg sync.WaitGroup

		// a) ValidateAction
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn, err := s.clients.get(outerCtx, rbacEndpoint())
			if err != nil {
				return
			}
			client := rbacpb.NewRbacServiceClient(conn)
			callCtx, callCancel := context.WithTimeout(authCtx(outerCtx), 10*time.Second)
			defer callCancel()

			resp, err := client.ValidateAction(callCtx, &rbacpb.ValidateActionRqst{
				Subject: subject,
				Type:    stEnum,
				Action:  action,
				Infos:   actionInfos,
			})
			if err != nil {
				mu.Lock()
				errors = append(errors, fmt.Sprintf("ValidateAction: %v", err))
				mu.Unlock()
				return
			}

			mu.Lock()
			actionHasAccess = resp.GetHasAccess()
			actionAccessDenied = resp.GetAccessDenied()
			mu.Unlock()
		}()

		// b) Per-resource ValidateAccess for each resource info
		if actionInfos != nil {
			for _, info := range actionInfos {
				info := info // capture
				wg.Add(1)
				go func() {
					defer wg.Done()
					conn, err := s.clients.get(outerCtx, rbacEndpoint())
					if err != nil {
						return
					}
					client := rbacpb.NewRbacServiceClient(conn)
					callCtx, callCancel := context.WithTimeout(authCtx(outerCtx), 10*time.Second)
					defer callCancel()

					resp, err := client.ValidateAccess(callCtx, &rbacpb.ValidateAccessRqst{
						Subject:    subject,
						Type:       stEnum,
						Path:       info.GetPath(),
						Permission: info.GetPermission(),
					})

					check := map[string]interface{}{
						"index":      info.GetIndex(),
						"path":       info.GetPath(),
						"permission": info.GetPermission(),
						"field":      info.GetField(),
					}

					if err != nil {
						check["result"] = "error"
						check["reason"] = err.Error()
						mu.Lock()
						errors = append(errors, fmt.Sprintf("ValidateAccess for path '%s': %v", info.GetPath(), err))
						resourceChecks = append(resourceChecks, check)
						mu.Unlock()
						return
					}

					if resp.GetAccessDenied() {
						check["result"] = "denied"
						check["reason"] = "explicit deny rule"
					} else if resp.GetHasAccess() {
						check["result"] = "allowed"
						check["reason"] = "allow rule matched"
					} else {
						check["result"] = "denied"
						check["reason"] = "no allow rule"
					}

					mu.Lock()
					resourceChecks = append(resourceChecks, check)
					mu.Unlock()
				}()
			}
		}

		// c) GetRoleBinding
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn, err := s.clients.get(outerCtx, rbacEndpoint())
			if err != nil {
				return
			}
			client := rbacpb.NewRbacServiceClient(conn)
			callCtx, callCancel := context.WithTimeout(authCtx(outerCtx), 10*time.Second)
			defer callCancel()

			resp, err := client.GetRoleBinding(callCtx, &rbacpb.GetRoleBindingRqst{
				Subject: subject,
			})
			if err != nil {
				mu.Lock()
				errors = append(errors, fmt.Sprintf("GetRoleBinding: %v", err))
				mu.Unlock()
				return
			}

			if b := resp.GetBinding(); b != nil {
				mu.Lock()
				roles = b.GetRoles()
				mu.Unlock()
			}
		}()

		// d) GetAccount (only for accounts)
		if subjectType == "account" {
			wg.Add(1)
			go func() {
				defer wg.Done()
				accCtx, err := getAccountContext(outerCtx, s.clients, subject)
				if err != nil {
					mu.Lock()
					errors = append(errors, fmt.Sprintf("GetAccount: %v", err))
					mu.Unlock()
					return
				}

				mu.Lock()
				identityGraph = accCtx
				mu.Unlock()
			}()
		}

		wg.Wait()

		// Build per-resource analysis.
		failingResources := []string{}
		missingPermissions := []string{}
		for _, rc := range resourceChecks {
			if rc["result"] != "allowed" {
				if p, ok := rc["path"].(string); ok {
					failingResources = append(failingResources, p)
				}
				perm := ""
				if p, ok := rc["permission"].(string); ok {
					perm = p
				}
				path := ""
				if p, ok := rc["path"].(string); ok {
					path = p
				}
				if perm != "" && path != "" {
					missingPermissions = append(missingPermissions, fmt.Sprintf("%s on %s", perm, path))
				}
			}
		}

		// Build evidence.
		actionInfosList := make([]map[string]interface{}, 0, len(actionInfos))
		for _, info := range actionInfos {
			actionInfosList = append(actionInfosList, map[string]interface{}{
				"index":      info.GetIndex(),
				"path":       info.GetPath(),
				"permission": info.GetPermission(),
				"field":      info.GetField(),
			})
		}

		evidence := map[string]interface{}{
			"action_resource_infos": actionInfosList,
			"role_binding": map[string]interface{}{
				"roles": roles,
			},
		}
		if identityGraph != nil {
			evidence["identity_graph"] = identityGraph
		}

		// Build causes and suggestions.
		var causes []string
		var suggestions []string

		decision := "denied"
		if actionHasAccess {
			decision = "allowed"
		}

		if actionAccessDenied {
			causes = append(causes, fmt.Sprintf("Explicit deny on one or more resources required by action '%s'", action))
			suggestions = append(suggestions, "Check denied rules on the failing resource paths")
		}

		if !actionHasAccess && !actionAccessDenied {
			if len(failingResources) > 0 {
				for _, fr := range failingResources {
					causes = append(causes, fmt.Sprintf("Subject '%s' lacks required permission on '%s'", subject, fr))
				}
			} else if actionInfosErr != nil {
				causes = append(causes, fmt.Sprintf("Could not retrieve action resource infos: %v", actionInfosErr))
			} else {
				causes = append(causes, fmt.Sprintf("No allow rule matched for action '%s'", action))
			}

			totalResources := len(actionInfos)
			failedCount := len(failingResources)
			if totalResources > 0 && failedCount > 0 {
				causes = append(causes, fmt.Sprintf("Action requires access to %d resources; %d failed", totalResources, failedCount))
			}

			for _, mp := range missingPermissions {
				suggestions = append(suggestions, fmt.Sprintf("Add '%s' for subject '%s'", mp, subject))
			}
			if len(suggestions) == 0 {
				suggestions = append(suggestions, fmt.Sprintf("Add subject '%s' to a group/role with the required permissions", subject))
			}
		}

		if actionHasAccess {
			causes = append(causes, fmt.Sprintf("All resource checks passed for action '%s'", action))
		}

		// Build summary.
		summary := fmt.Sprintf("Action %s for subject '%s' on '%s'. ", decision, subject, action)
		if len(failingResources) > 0 {
			summary += fmt.Sprintf("Failed resource checks: %s.", strings.Join(failingResources, ", "))
		} else if actionHasAccess {
			summary += "All resource checks passed."
		} else if len(causes) > 0 {
			summary += causes[0]
		}

		return map[string]interface{}{
			"decision":              decision,
			"action":                action,
			"summary":               summary,
			"resource_checks":       resourceChecks,
			"failing_resources":     failingResources,
			"missing_permissions":   missingPermissions,
			"evidence":              evidence,
			"likely_root_causes":    causes,
			"suggested_next_checks": suggestions,
			"errors":                errors,
		}, nil
	})
}
