// rbac_actions.go: action-level permissioning and resource validation.

package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/rbac/rbacpb"
	Utility "github.com/globulario/utility"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// -----------------------------------------------------------------------------
// Phase 3: Wildcard action matching for globular-admin role
// Security Fix #6: Path normalization to prevent bypass attacks
// -----------------------------------------------------------------------------

// canonicalizeAction normalizes a gRPC action/method name to prevent bypass attacks.
// Security requirements:
// - Remove duplicate slashes (e.g., "//rbac" → "/rbac")
// - Reject relative path components (".", "..")
// - Ensure starts with "/"
// - Reject null bytes, newlines, and other control characters
//
// gRPC method names follow format: "/package.ServiceName/MethodName"
// Any deviation from this format is suspicious and rejected.
func canonicalizeAction(action string) (string, error) {
	// Empty check
	if action == "" {
		return "", errors.New("empty action")
	}

	// Reject control characters and null bytes
	for i := 0; i < len(action); i++ {
		c := action[i]
		if c < 0x20 || c == 0x7F { // Control characters including null, tab, newline
			return "", errors.New("action contains control characters")
		}
	}

	// Must start with "/"
	if !strings.HasPrefix(action, "/") {
		return "", errors.New("action must start with /")
	}

	// Reject relative path components (shouldn't exist in gRPC method names)
	if strings.Contains(action, "/.") {
		return "", errors.New("action contains relative path components")
	}

	// Normalize: remove duplicate slashes
	// "/rbac.RbacService//CreateAccount" → "/rbac.RbacService/CreateAccount"
	normalized := action
	for strings.Contains(normalized, "//") {
		normalized = strings.ReplaceAll(normalized, "//", "/")
	}

	// Ensure exactly one slash divider (format: /package.Service/Method)
	// Global wildcard "/*" is special case
	if normalized != "/*" {
		parts := strings.Split(normalized[1:], "/") // Remove leading "/" and split
		if len(parts) != 2 {
			// Valid formats: "/package.Service/Method" or "/package.Service/*"
			// Invalid: "/", "/service", "/a/b/c"
			if !(len(parts) == 2 && parts[1] == "*") {
				return "", errors.New("invalid action format (expected /package.Service/Method)")
			}
		}
	}

	return normalized, nil
}

// matchesAction checks if a permission pattern matches a requested action.
// Supports:
// - Exact match: "/rbac.RbacService/CreateAccount" matches only that method
// - Wildcard: "/*" matches ALL methods (for globular-admin role)
// - Service wildcard: "/rbac.RbacService/*" matches all methods in that service
//
// Security Fix #6: Both pattern and action are canonicalized before comparison
// to prevent bypass attacks using path normalization tricks.
//
// Examples:
//   - pattern="/*", action="/rbac.RbacService/CreateAccount" → true (global admin)
//   - pattern="/rbac.RbacService/*", action="/rbac.RbacService/CreateAccount" → true
//   - pattern="/rbac.RbacService/CreateAccount", action="/rbac.RbacService/CreateAccount" → true
//   - pattern="/rbac.RbacService/CreateAccount", action="/dns.DnsService/CreateZone" → false
//
// Bypass prevention:
//   - pattern="/rbac.RbacService/CreateAccount", action="/rbac.RbacService//CreateAccount" → true (normalized)
//   - pattern="/rbac.RbacService/CreateAccount", action="/rbac.RbacService/./CreateAccount" → ERROR (rejected)
func matchesAction(pattern, action string) bool {
	// Security Fix #6: Canonicalize both sides to prevent bypass
	canonPattern, err := canonicalizeAction(pattern)
	if err != nil {
		// Invalid pattern - should not happen with valid RBAC configuration
		// Log and reject to fail closed
		return false
	}

	canonAction, err := canonicalizeAction(action)
	if err != nil {
		// Invalid action - suspicious, reject
		return false
	}

	// Exact match (fast path)
	if canonPattern == canonAction {
		return true
	}

	// Global wildcard: "/*" matches everything
	if canonPattern == "/*" {
		return true
	}

	// Service-level wildcard: "/service.ServiceName/*"
	if strings.HasSuffix(canonPattern, "/*") {
		prefix := canonPattern[:len(canonPattern)-1] // Remove trailing '*', keep '/'
		return strings.HasPrefix(canonAction, prefix)
	}

	// No match
	return false
}

/*
* Set action permissions.
When gRPC service methode are called they must validate the resource pass in parameters.
So each service is reponsible to give access permissions requirement.
*/
func (srv *server) setActionResourcesPermissions(permissions map[string]interface{}) error {

	// So here I will keep values in local storage.cap()
	data, err := json.Marshal(permissions["resources"])
	if err != nil {
		return err
	}

	return srv.setItem(permissions["action"].(string), data)
}

// SetActionResourcesPermissions sets the permissions for resources associated with a specific action.
// It receives a request containing a map of permissions and applies them using the internal
// setActionResourcesPermissions method. Returns an empty response on success or an error if the operation fails.
//
// Parameters:
//
//	ctx - The context for the request, used for cancellation and deadlines.
//	rqst - The request containing the permissions to set.
//
// Returns:
//
//	*rbacpb.SetActionResourcesPermissionsRsp - The response indicating success.
//	error - An error if the operation fails.
func (srv *server) SetActionResourcesPermissions(ctx context.Context, rqst *rbacpb.SetActionResourcesPermissionsRqst) (*rbacpb.SetActionResourcesPermissionsRsp, error) {

	err := srv.setActionResourcesPermissions(rqst.Permissions.AsMap())
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &rbacpb.SetActionResourcesPermissionsRsp{}, nil
}

func (srv *server) getActionResourcesPermissions(action string) ([]*rbacpb.ResourceInfos, error) {

	if len(action) == 0 {
		return nil, errors.New("no action given")
	}
	data, err := srv.getItem(action)
	infos_ := make([]*rbacpb.ResourceInfos, 0)
	if err != nil {
		if !strings.Contains(err.Error(), "item not found") || strings.Contains(err.Error(), "Key not found") {
			return nil, err
		} else {
			// no infos_ found...
			return infos_, nil
		}
	}
	infos := make([]interface{}, 0)
	err = json.Unmarshal(data, &infos)

	for i := range infos {
		info := infos[i].(map[string]interface{})
		field := ""
		if info["field"] != nil {
			field = info["field"].(string)
		}
		infos_ = append(infos_, &rbacpb.ResourceInfos{Index: int32(Utility.ToInt(info["index"])), Permission: info["permission"].(string), Field: field})
	}

	return infos_, err
}

// GetActionResourceInfos retrieves information about resources and their permissions associated with a specific action.
// It takes a context and a request containing the action name, and returns a response with resource info or an error.
// In case of failure, it returns a gRPC internal error with detailed information.
func (srv *server) GetActionResourceInfos(ctx context.Context, rqst *rbacpb.GetActionResourceInfosRqst) (*rbacpb.GetActionResourceInfosRsp, error) {
	infos, err := srv.getActionResourcesPermissions(rqst.Action)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &rbacpb.GetActionResourceInfosRsp{Infos: infos}, nil
}

func (srv *server) validateAction(action string, subject string, subjectType rbacpb.SubjectType, resources []*rbacpb.ResourceInfos) (bool, bool, error) {
	// ---------------------------------------------------------------
	// Exceptions: allow a few infra calls when no resources provided
	// ---------------------------------------------------------------
	if len(resources) == 0 {
		if strings.HasPrefix(action, "/echo.EchoService") ||
			strings.HasPrefix(action, "/resource.ResourceService") ||
			strings.HasPrefix(action, "/event.EventService") ||
			action == "/file.FileService/GetFileInfo" {
			return true, false, nil
		}
	}

	// Validate/normalize subject (e.g., ensure it exists; may return canonical id)
	subject, err := srv.validateSubject(subject, subjectType)
	if err != nil {
		return false, false, err
	}

	// Helper: ensure id is fully-qualified with domain if missing
	withDomain := func(id string, domain string) string {
		if id == "" || strings.Contains(id, "@") {
			return id
		}
		return id + "@" + domain
	}

	var actions []string
	hasAccess := false

	switch subjectType {
	case rbacpb.SubjectType_APPLICATION:
		app, err := srv.getApplication(subject)
		if err != nil {
			return false, false, err
		}
		actions = app.Actions

	case rbacpb.SubjectType_NODE_IDENTITY:
		if _, err := srv.getNodeIdentityByMac(subject); err != nil {
			return false, false, err
		}
		actions = nil

	case rbacpb.SubjectType_ROLE:
		role, err := srv.getRole(subject)
		if err != nil {
			return false, false, err
		}
		// Local "admin" role → full access
		domain, _ := config.GetDomain()
		if role.Domain == domain && role.Name == "admin" {
			return true, false, nil
		}
		actions = role.Actions

	case rbacpb.SubjectType_ACCOUNT:
		// Phase 3: Removed hardcoded "sa@domain" bypass
		// Admin access now enforced via RBAC globular-admin role

		account, err := srv.getAccount(subject)
		if err != nil {
			return false, false, err
		}

		// -----------------------------------------------------------
		// (A) Direct Account → Role assignments (unchanged)
		// -----------------------------------------------------------
		if account.Roles != nil {
			for _, rid := range account.Roles {
				roleId := withDomain(rid, srv.Domain)

				// Local admin role → full access
				if roleId == "admin@"+srv.Domain {
					return true, false, nil
				}

				// Only recurse for local roles (keep previous semantics)
				if strings.HasSuffix(roleId, "@"+srv.Domain) {
					ok, _, _ := srv.validateAction(action, roleId, rbacpb.SubjectType_ROLE, resources)
					if ok {
						hasAccess = true
						break
					}
				}
			}
		}

		// -----------------------------------------------------------
		// (B) External account that local roles list in Role.Accounts
		//     (existing behavior retained)
		// -----------------------------------------------------------
		if !hasAccess {
			if roles, err := srv.getRoles(); err == nil {
				for i := range roles {
					roleFQN := roles[i].Id + "@" + roles[i].Domain
					if Utility.Contains(roles[i].Accounts, subject) {
						ok, _, _ := srv.validateAction(action, roleFQN, rbacpb.SubjectType_ROLE, resources)
						if ok {
							hasAccess = true
							break
						}
					}
				}
			}
		}

		// -----------------------------------------------------------
		// (C) NEW: Account → Groups → Roles
		//     Group now contains Roles []string; if the account is in
		//     a group, grant via any group role.
		// -----------------------------------------------------------
		if !hasAccess {
			// 1) Collect groups for this account
			var accountGroups []struct {
				Id     string
				Domain string
				Roles  []string
			}

			// Prefer a direct membership list if your Account model has it (account.Groups).
			// If not, derive via srv.getGroups().
			haveGroups := false
			if len(account.Groups) > 0 {
				haveGroups = true
			}

			if haveGroups {
				// We need group domains & roles; fetch groups and filter to those ids.
				if groups, err := srv.getGroups(); err == nil {
					for _, g := range groups {
						// accept both raw id and fqdn
						if Utility.Contains(account.Groups, g.Id) ||
							Utility.Contains(account.Groups, withDomain(g.Id, g.Domain)) {
							accountGroups = append(accountGroups, struct {
								Id     string
								Domain string
								Roles  []string
							}{Id: g.Id, Domain: g.Domain, Roles: g.Roles})
						}
					}
				}
			} else {
				// Derive by scanning groups to find membership (Members/Accounts fields)
				if groups, err := srv.getGroups(); err == nil {
					for _, g := range groups {
						if Utility.Contains(g.Accounts, subject) {
							accountGroups = append(accountGroups, struct {
								Id     string
								Domain string
								Roles  []string
							}{Id: g.Id, Domain: g.Domain, Roles: g.Roles})
						}
					}
				}
			}

			// 2) From groups, walk their roles and validate
			if len(accountGroups) > 0 {
				seen := make(map[string]struct{})
				for _, ag := range accountGroups {
					for _, rid := range ag.Roles {
						roleFQN := withDomain(rid, ag.Domain)
						if _, done := seen[roleFQN]; done {
							continue
						}
						seen[roleFQN] = struct{}{}

						ok, _, _ := srv.validateAction(action, roleFQN, rbacpb.SubjectType_ROLE, resources)
						if ok {
							hasAccess = true
							break
						}
					}
					if hasAccess {
						break
					}
				}
			}
		}
	}

	// If still not granted, check the subject's own action list
	// Phase 3: Added wildcard matching support for globular-admin role
	if !hasAccess && actions != nil {
		for _, a := range actions {
			if matchesAction(a, action) {
				hasAccess = true
				break
			}
		}
	}

	// If method not granted, return early (resource checks not needed)
	if !hasAccess {
		return false, true, nil
	}

	// Roles: method-level only (resource checks happen at the caller’s subject)
	if subjectType == rbacpb.SubjectType_ROLE {
		return true, false, nil
	}

	// -----------------------------------------------------------
	// Resource-level checks (if resources provided)
	// -----------------------------------------------------------
	permissions_, _ := srv.getActionResourcesPermissions(action)
	if len(resources) > 0 {
		if permissions_ == nil {
			return false, false, errors.New("no resources path are given for validations")
		}
		for i := range resources {
			if len(resources[i].Path) > 0 { // empty path => skip validation
				ok, accessDenied, err := srv.validateAccess(subject, subjectType, resources[i].Permission, resources[i].Path)
				if err != nil {
					return false, false, err
				}
				return ok, accessDenied, nil
			}
		}
	}

	// Granted: method ok, no resource constraints to enforce
	return true, false, nil
}

// ValidateAction validates whether a given subject is allowed to perform a specified action.
// It checks the request for a valid action, and then delegates the permission check to validateAction.
// Returns a response indicating if access is granted and any access denial reasons.
// In case of errors (e.g., missing action or internal validation failure), returns a gRPC error.
func (srv *server) ValidateAction(ctx context.Context, rqst *rbacpb.ValidateActionRqst) (*rbacpb.ValidateActionRsp, error) {

	// So here From the context I will validate if the application can execute the action...
	var err error
	if len(rqst.Action) == 0 {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no action was given to validate")))
	}

	// If the address is local I will give the permission.
	hasAccess, accessDenied, err := srv.validateAction(rqst.Action, rqst.Subject, rqst.Type, rqst.Infos)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &rbacpb.ValidateActionRsp{
		HasAccess:    hasAccess,
		AccessDenied: accessDenied,
	}, nil
}
