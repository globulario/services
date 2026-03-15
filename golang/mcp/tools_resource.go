package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	resourcepb "github.com/globulario/services/golang/resource/resourcepb"
)

// ── Resource service endpoint ───────────────────────────────────────────────

func resourceEndpoint() string {
	if ep := os.Getenv("GLOBULAR_RESOURCE_ENDPOINT"); ep != "" {
		return ep
	}
	return gatewayEndpoint()
}

// ── Streaming collection helpers ────────────────────────────────────────────

func collectRoles(ctx context.Context, client resourcepb.ResourceServiceClient) ([]*resourcepb.Role, error) {
	stream, err := client.GetRoles(authCtx(ctx), &resourcepb.GetRolesRqst{Query: ""})
	if err != nil {
		return nil, err
	}
	var roles []*resourcepb.Role
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		roles = append(roles, resp.GetRoles()...)
	}
	return roles, nil
}

func collectGroups(ctx context.Context, client resourcepb.ResourceServiceClient) ([]*resourcepb.Group, error) {
	stream, err := client.GetGroups(authCtx(ctx), &resourcepb.GetGroupsRqst{Query: ""})
	if err != nil {
		return nil, err
	}
	var groups []*resourcepb.Group
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		groups = append(groups, resp.GetGroups()...)
	}
	return groups, nil
}

func collectOrganizations(ctx context.Context, client resourcepb.ResourceServiceClient) ([]*resourcepb.Organization, error) {
	stream, err := client.GetOrganizations(authCtx(ctx), &resourcepb.GetOrganizationsRqst{Query: ""})
	if err != nil {
		return nil, err
	}
	var orgs []*resourcepb.Organization
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		orgs = append(orgs, resp.GetOrganizations()...)
	}
	return orgs, nil
}

// ── Role lookup helpers ─────────────────────────────────────────────────────

// buildRoleIndex returns a map from role ID/name to the Role object.
func buildRoleIndex(roles []*resourcepb.Role) map[string]*resourcepb.Role {
	idx := make(map[string]*resourcepb.Role, len(roles)*2)
	for _, r := range roles {
		idx[r.GetId()] = r
		idx[r.GetName()] = r
	}
	return idx
}

// resolveRoleActions returns deduplicated actions for the given role IDs.
func resolveRoleActions(roleIDs []string, roleIdx map[string]*resourcepb.Role) ([]string, []string) {
	seen := make(map[string]struct{})
	var actions []string
	var warnings []string

	for _, rid := range roleIDs {
		r, ok := roleIdx[rid]
		if !ok {
			warnings = append(warnings, fmt.Sprintf("Role '%s' referenced but not found in role definitions", rid))
			continue
		}
		for _, a := range r.GetActions() {
			if _, dup := seen[a]; !dup {
				seen[a] = struct{}{}
				actions = append(actions, a)
			}
		}
	}
	return actions, warnings
}

// uniqueStrings deduplicates a slice of strings preserving order.
func uniqueStrings(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	var out []string
	for _, s := range in {
		if _, ok := seen[s]; !ok {
			seen[s] = struct{}{}
			out = append(out, s)
		}
	}
	return out
}

// ── Tool registration ───────────────────────────────────────────────────────

func registerResourceTools(s *server) {

	// ── resource_get_account_identity_context ────────────────────────────
	s.register(toolDef{
		Name:        "resource_get_account_identity_context",
		Description: "Builds a complete identity view for an account: direct roles, group memberships, organization memberships, inherited roles from groups and organizations, all effective actions. Security-redacted (no password/refreshToken). Start here when diagnosing RBAC or permission issues for a specific user.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"account_id": {Type: "string", Description: "The account ID to inspect"},
			},
			Required: []string{"account_id"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		accountID := getStr(args, "account_id")
		if accountID == "" {
			return nil, fmt.Errorf("account_id is required")
		}

		callCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()

		conn, err := s.clients.get(callCtx, resourceEndpoint())
		if err != nil {
			return nil, fmt.Errorf("resource service unavailable: %w", err)
		}
		client := resourcepb.NewResourceServiceClient(conn)

		// 1. Get the account.
		acctResp, err := client.GetAccount(authCtx(callCtx), &resourcepb.GetAccountRqst{
			AccountId: accountID,
		})
		if err != nil {
			return nil, fmt.Errorf("GetAccount(%s): %w", accountID, err)
		}
		acct := acctResp.GetAccount()
		if acct == nil {
			return nil, fmt.Errorf("account '%s' not found", accountID)
		}

		// 2. Collect all roles, groups, organizations for cross-referencing.
		allRoles, err := collectRoles(callCtx, client)
		if err != nil {
			return nil, fmt.Errorf("GetRoles: %w", err)
		}
		roleIdx := buildRoleIndex(allRoles)

		allGroups, groupsErr := collectGroups(callCtx, client)
		allOrgs, orgsErr := collectOrganizations(callCtx, client)

		// Build group index.
		groupIdx := make(map[string]*resourcepb.Group, len(allGroups))
		for _, g := range allGroups {
			groupIdx[g.GetId()] = g
			groupIdx[g.GetName()] = g
		}

		// Build org index.
		orgIdx := make(map[string]*resourcepb.Organization, len(allOrgs))
		for _, o := range allOrgs {
			orgIdx[o.GetId()] = o
			orgIdx[o.GetName()] = o
		}

		var warnings []string

		// 3. Direct roles and their actions.
		directRoles := acct.GetRoles()
		directActions, roleWarnings := resolveRoleActions(directRoles, roleIdx)
		warnings = append(warnings, roleWarnings...)

		// 4. Inherited roles from groups.
		type groupRoles struct {
			Group string   `json:"group"`
			Roles []string `json:"roles"`
		}
		var inheritedFromGroups []groupRoles
		var allInheritedRoleIDs []string

		for _, gid := range acct.GetGroups() {
			g, ok := groupIdx[gid]
			if !ok {
				if groupsErr != nil {
					warnings = append(warnings, fmt.Sprintf("Could not fetch groups: %v", groupsErr))
				} else {
					warnings = append(warnings, fmt.Sprintf("Group '%s' referenced by account but not found", gid))
				}
				continue
			}
			if len(g.GetRoles()) > 0 {
				inheritedFromGroups = append(inheritedFromGroups, groupRoles{
					Group: gid,
					Roles: g.GetRoles(),
				})
				allInheritedRoleIDs = append(allInheritedRoleIDs, g.GetRoles()...)
			}
		}

		// 5. Inherited roles from organizations.
		type orgRoles struct {
			Org   string   `json:"org"`
			Roles []string `json:"roles"`
		}
		var inheritedFromOrgs []orgRoles

		for _, oid := range acct.GetOrganizations() {
			o, ok := orgIdx[oid]
			if !ok {
				if orgsErr != nil {
					warnings = append(warnings, fmt.Sprintf("Could not fetch organizations: %v", orgsErr))
				} else {
					warnings = append(warnings, fmt.Sprintf("Organization '%s' referenced by account but not found", oid))
				}
				continue
			}
			if len(o.GetRoles()) > 0 {
				inheritedFromOrgs = append(inheritedFromOrgs, orgRoles{
					Org:   oid,
					Roles: o.GetRoles(),
				})
				allInheritedRoleIDs = append(allInheritedRoleIDs, o.GetRoles()...)
			}
		}

		// 6. All unique roles.
		var allRoleIDs []string
		allRoleIDs = append(allRoleIDs, directRoles...)
		allRoleIDs = append(allRoleIDs, allInheritedRoleIDs...)
		allUniqueRoles := uniqueStrings(allRoleIDs)

		// 7. All unique actions.
		allActions, moreWarnings := resolveRoleActions(allUniqueRoles, roleIdx)
		// Merge warnings without duplicating the ones from direct roles.
		for _, w := range moreWarnings {
			found := false
			for _, existing := range warnings {
				if existing == w {
					found = true
					break
				}
			}
			if !found {
				warnings = append(warnings, w)
			}
		}
		allUniqueActions := uniqueStrings(append(directActions, allActions...))

		if warnings == nil {
			warnings = []string{}
		}

		return map[string]interface{}{
			"account_id":                       acct.GetId(),
			"name":                             acct.GetName(),
			"email":                            acct.GetEmail(),
			"direct_roles":                     directRoles,
			"groups":                           acct.GetGroups(),
			"organizations":                    acct.GetOrganizations(),
			"inherited_roles_from_groups":       inheritedFromGroups,
			"inherited_roles_from_organizations": inheritedFromOrgs,
			"all_unique_roles":                 allUniqueRoles,
			"all_unique_actions":               allUniqueActions,
			"warnings":                         warnings,
		}, nil
	})

	// ── resource_get_group_identity_context ──────────────────────────────
	s.register(toolDef{
		Name:        "resource_get_group_identity_context",
		Description: "Returns the identity context for a group: member accounts, associated organizations, direct roles, and effective actions (permissions). Use this to understand what permissions a group grants to its members.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"group_id": {Type: "string", Description: "The group ID to inspect"},
			},
			Required: []string{"group_id"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		groupID := getStr(args, "group_id")
		if groupID == "" {
			return nil, fmt.Errorf("group_id is required")
		}

		callCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()

		conn, err := s.clients.get(callCtx, resourceEndpoint())
		if err != nil {
			return nil, fmt.Errorf("resource service unavailable: %w", err)
		}
		client := resourcepb.NewResourceServiceClient(conn)

		// Collect all groups and find the target.
		allGroups, err := collectGroups(callCtx, client)
		if err != nil {
			return nil, fmt.Errorf("GetGroups: %w", err)
		}

		var target *resourcepb.Group
		for _, g := range allGroups {
			if g.GetId() == groupID || g.GetName() == groupID {
				target = g
				break
			}
		}
		if target == nil {
			return nil, fmt.Errorf("group '%s' not found", groupID)
		}

		// Collect all roles for action resolution.
		allRoles, err := collectRoles(callCtx, client)
		if err != nil {
			return nil, fmt.Errorf("GetRoles: %w", err)
		}
		roleIdx := buildRoleIndex(allRoles)

		var warnings []string
		directRoles := target.GetRoles()
		effectiveActions, roleWarnings := resolveRoleActions(directRoles, roleIdx)
		warnings = append(warnings, roleWarnings...)

		if warnings == nil {
			warnings = []string{}
		}

		return map[string]interface{}{
			"group_id":          target.GetId(),
			"name":              target.GetName(),
			"member_accounts":   target.GetAccounts(),
			"organizations":     target.GetOrganizations(),
			"direct_roles":      directRoles,
			"effective_actions": effectiveActions,
			"warnings":          warnings,
		}, nil
	})

	// ── resource_get_organization_identity_context ───────────────────────
	s.register(toolDef{
		Name:        "resource_get_organization_identity_context",
		Description: "Returns the identity context for an organization: member accounts, member groups, direct roles, and effective actions (permissions). Use this to understand the full permission surface of an organization.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"organization_id": {Type: "string", Description: "The organization ID to inspect"},
			},
			Required: []string{"organization_id"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		orgID := getStr(args, "organization_id")
		if orgID == "" {
			return nil, fmt.Errorf("organization_id is required")
		}

		callCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()

		conn, err := s.clients.get(callCtx, resourceEndpoint())
		if err != nil {
			return nil, fmt.Errorf("resource service unavailable: %w", err)
		}
		client := resourcepb.NewResourceServiceClient(conn)

		// Collect all organizations and find the target.
		allOrgs, err := collectOrganizations(callCtx, client)
		if err != nil {
			return nil, fmt.Errorf("GetOrganizations: %w", err)
		}

		var target *resourcepb.Organization
		for _, o := range allOrgs {
			if o.GetId() == orgID || o.GetName() == orgID {
				target = o
				break
			}
		}
		if target == nil {
			return nil, fmt.Errorf("organization '%s' not found", orgID)
		}

		// Collect all roles for action resolution.
		allRoles, err := collectRoles(callCtx, client)
		if err != nil {
			return nil, fmt.Errorf("GetRoles: %w", err)
		}
		roleIdx := buildRoleIndex(allRoles)

		var warnings []string
		directRoles := target.GetRoles()
		effectiveActions, roleWarnings := resolveRoleActions(directRoles, roleIdx)
		warnings = append(warnings, roleWarnings...)

		if warnings == nil {
			warnings = []string{}
		}

		return map[string]interface{}{
			"organization_id":  target.GetId(),
			"name":             target.GetName(),
			"member_accounts":  target.GetAccounts(),
			"member_groups":    target.GetGroups(),
			"direct_roles":     directRoles,
			"effective_actions": effectiveActions,
			"warnings":         warnings,
		}, nil
	})
}
