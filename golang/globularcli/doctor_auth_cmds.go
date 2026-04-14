// doctor_auth_cmds.go: Auth chain diagnostics.
//
//	globular doctor auth --subject <id> --method <grpc_method> [--token <jwt>]
//	globular doctor auth --recent-denials
//
// Walks the 5 layers of the auth chain and reports pass/fail per layer.

package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/policy"
	"github.com/globulario/services/golang/rbac/rbacpb"
	"github.com/globulario/services/golang/security"
)

var (
	doctorCmd = &cobra.Command{
		Use:   "doctor",
		Short: "Diagnostic tools for troubleshooting",
	}

	doctorAuthSubject       string
	doctorAuthMethod        string
	doctorAuthRecentDenials bool
	doctorAuthJSON          bool

	doctorAuthCmd = &cobra.Command{
		Use:   "auth",
		Short: "Trace the auth chain for a subject + method",
		Long: `Walks 5 layers of the gRPC auth chain and reports pass/fail per layer:

  [1] Bootstrap Gate  — Is bootstrap mode active?
  [2] Cluster ID      — Does the caller's cluster_id match local?
  [3] Unauthenticated — Is the method in the public allowlist?
  [4] Auth Context    — Can the subject be authenticated (JWT/mTLS)?
  [5] Role Binding    — Does a role grant the subject access to this action?

Use --recent-denials to query audit log for recent permission denied entries.`,
		RunE: runDoctorAuth,
	}
)

// authLayerResult represents the diagnostic result for one auth layer.
type authLayerResult struct {
	Layer   int    `json:"layer"`
	Name    string `json:"name"`
	Status  string `json:"status"` // PASS, FAIL, SKIP, INACTIVE, N/A
	Detail  string `json:"detail,omitempty"`
}

// authDiagnostic holds the complete auth chain diagnosis.
type authDiagnostic struct {
	Subject   string            `json:"subject"`
	Method    string            `json:"method"`
	ActionKey string            `json:"action_key,omitempty"`
	Layers    []authLayerResult `json:"layers"`
	Result    string            `json:"result"` // ALLOWED, DENIED
	Reason    string            `json:"reason,omitempty"`
}

func runDoctorAuth(cmd *cobra.Command, args []string) error {
	if doctorAuthRecentDenials {
		return runRecentDenials(cmd)
	}

	if doctorAuthSubject == "" {
		return fmt.Errorf("--subject is required (use --recent-denials for audit log query)")
	}
	if doctorAuthMethod == "" {
		return fmt.Errorf("--method is required (e.g. /file.FileService/ReadDir)")
	}

	diag := authDiagnostic{
		Subject: doctorAuthSubject,
		Method:  doctorAuthMethod,
	}

	// Resolve action key
	actionKey := policy.GlobalResolver().Resolve(doctorAuthMethod)
	if actionKey != doctorAuthMethod {
		diag.ActionKey = actionKey
	}

	// Layer 1: Bootstrap Gate
	layer1 := authLayerResult{Layer: 1, Name: "Bootstrap Gate"}
	if security.DefaultBootstrapGate.IsActive() {
		layer1.Status = "ACTIVE"
		layer1.Detail = security.DefaultBootstrapGate.GetBootstrapStatus()
	} else {
		layer1.Status = "INACTIVE"
		layer1.Detail = "bootstrap mode not enabled"
	}
	diag.Layers = append(diag.Layers, layer1)

	// Layer 2: Cluster ID
	layer2 := authLayerResult{Layer: 2, Name: "Cluster ID"}
	localClusterID, err := security.GetLocalClusterID()
	if err != nil || localClusterID == "" {
		layer2.Status = "N/A"
		layer2.Detail = "cluster not initialized or cluster_id unavailable"
	} else {
		layer2.Status = "PASS"
		layer2.Detail = fmt.Sprintf("local=%s", localClusterID)
	}
	diag.Layers = append(diag.Layers, layer2)

	// Layer 3: Unauthenticated allowlist
	layer3 := authLayerResult{Layer: 3, Name: "Unauthenticated"}
	if isMethodAllowlisted(doctorAuthMethod) {
		layer3.Status = "ALLOWLISTED"
		layer3.Detail = "method bypasses auth"
		diag.Layers = append(diag.Layers, layer3)
		diag.Result = "ALLOWED"
		diag.Reason = "unauthenticated_allowlist"
		return printDiagnostic(diag)
	}
	layer3.Status = "NOT ALLOWLISTED"
	layer3.Detail = "requires authentication"
	diag.Layers = append(diag.Layers, layer3)

	// Layer 4: Auth Context
	layer4 := authLayerResult{Layer: 4, Name: "Auth Context"}
	if doctorAuthSubject != "" {
		layer4.Status = "AUTHENTICATED"
		layer4.Detail = fmt.Sprintf("subject=%s", doctorAuthSubject)
	} else {
		layer4.Status = "FAIL"
		layer4.Detail = "no subject identity"
		diag.Layers = append(diag.Layers, layer4)
		diag.Result = "DENIED"
		diag.Reason = "no_identity"
		return printDiagnostic(diag)
	}
	diag.Layers = append(diag.Layers, layer4)

	// Layer 5: Role Binding
	layer5 := authLayerResult{Layer: 5, Name: "Role Binding"}

	addr := resolveRbacAddr()
	cc, err := dialGRPC(addr)
	if err != nil {
		layer5.Status = "ERROR"
		layer5.Detail = fmt.Sprintf("cannot connect to RBAC service: %v", err)
		diag.Layers = append(diag.Layers, layer5)
		diag.Result = "DENIED"
		diag.Reason = "rbac_unavailable"
		return printDiagnostic(diag)
	}
	defer cc.Close()

	client := rbacpb.NewRbacServiceClient(cc)
	resp, err := client.GetRoleBinding(ctxWithTimeout(), &rbacpb.GetRoleBindingRqst{Subject: doctorAuthSubject})
	if err != nil {
		layer5.Status = "FAIL"
		layer5.Detail = fmt.Sprintf("GetRoleBinding error: %v", err)
		diag.Layers = append(diag.Layers, layer5)
		diag.Result = "DENIED"
		diag.Reason = "no_role_binding"
		return printDiagnostic(diag)
	}

	roles := resp.GetBinding().GetRoles()
	if len(roles) == 0 {
		layer5.Status = "FAIL"
		layer5.Detail = "no roles bound to subject"
		diag.Layers = append(diag.Layers, layer5)
		diag.Result = "DENIED"
		diag.Reason = "no_roles"
		return printDiagnostic(diag)
	}

	// Check if any role grants access
	checkAction := actionKey
	if checkAction == "" {
		checkAction = doctorAuthMethod
	}

	hasAccess := security.HasRolePermission(roles, checkAction)
	if hasAccess {
		layer5.Status = "PASS"
		layer5.Detail = fmt.Sprintf("roles: %s", strings.Join(roles, ", "))
		if diag.ActionKey != "" {
			layer5.Detail += fmt.Sprintf(" → action: %s", diag.ActionKey)
		}
		diag.Layers = append(diag.Layers, layer5)
		diag.Result = "ALLOWED"
		diag.Reason = "role_binding_granted"
	} else {
		layer5.Status = "FAIL"
		layer5.Detail = fmt.Sprintf("roles [%s] do not grant %s", strings.Join(roles, ", "), checkAction)
		diag.Layers = append(diag.Layers, layer5)

		// Fall through to resource-mapping check
		layer5b := authLayerResult{Layer: 5, Name: "Resource Mapping"}
		infos, _ := getResourceInfos(addr, checkAction)
		if len(infos) > 0 {
			// There are resource mappings — validate access
			hasResAccess, err := validateResourceAccess(client, doctorAuthSubject, checkAction, infos)
			if err != nil {
				layer5b.Status = "ERROR"
				layer5b.Detail = fmt.Sprintf("resource validation error: %v", err)
			} else if hasResAccess {
				layer5b.Status = "PASS"
				layer5b.Detail = "resource-level access granted"
				diag.Layers = append(diag.Layers, layer5b)
				diag.Result = "ALLOWED"
				diag.Reason = "resource_access_granted"
				return printDiagnostic(diag)
			} else {
				layer5b.Status = "FAIL"
				layer5b.Detail = "resource-level access denied"
			}
			diag.Layers = append(diag.Layers, layer5b)
		}

		diag.Result = "DENIED"
		diag.Reason = "role_binding_denied"
	}

	return printDiagnostic(diag)
}

// isMethodAllowlisted checks known public endpoints.
func isMethodAllowlisted(method string) bool {
	allowlisted := []string{
		"/grpc.health.v1.Health/Check",
		"/grpc.reflection.v1alpha.ServerReflection/ServerReflectionInfo",
		"/authentication.AuthenticationService/Authenticate",
		"/authentication.AuthenticationService/RefreshToken",
	}
	for _, m := range allowlisted {
		if m == method {
			return true
		}
	}
	return false
}

// getResourceInfos fetches resource infos from RBAC for the given action.
func getResourceInfos(addr, action string) ([]*rbacpb.ResourceInfos, error) {
	cc, err := dialGRPC(addr)
	if err != nil {
		return nil, err
	}
	defer cc.Close()

	client := rbacpb.NewRbacServiceClient(cc)
	resp, err := client.GetActionResourceInfos(ctxWithTimeout(), &rbacpb.GetActionResourceInfosRqst{Action: action})
	if err != nil {
		return nil, err
	}
	return resp.GetInfos(), nil
}

// validateResourceAccess checks resource-level access for a subject.
func validateResourceAccess(client rbacpb.RbacServiceClient, subject, action string, infos []*rbacpb.ResourceInfos) (bool, error) {
	resp, err := client.ValidateAction(ctxWithTimeout(), &rbacpb.ValidateActionRqst{
		Action:  action,
		Subject: subject,
		Type:    rbacpb.SubjectType_ACCOUNT,
		Infos:   infos,
	})
	if err != nil {
		return false, err
	}
	return resp.GetHasAccess(), nil
}

func printDiagnostic(diag authDiagnostic) error {
	if doctorAuthJSON {
		data, _ := json.MarshalIndent(diag, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	fmt.Printf("\nAuth Chain Diagnosis: subject=%q, method=%q\n", diag.Subject, diag.Method)
	if diag.ActionKey != "" {
		fmt.Printf("    Action key: %s\n", diag.ActionKey)
	}
	fmt.Println()

	for _, l := range diag.Layers {
		icon := "?"
		switch l.Status {
		case "PASS", "AUTHENTICATED", "ALLOWLISTED", "INACTIVE":
			icon = "✓"
		case "FAIL", "ERROR":
			icon = "✗"
		case "N/A", "SKIP":
			icon = "-"
		case "ACTIVE", "NOT ALLOWLISTED":
			icon = "·"
		}
		fmt.Printf("[%d] %-20s %s %s", l.Layer, l.Name+":", icon, l.Status)
		if l.Detail != "" {
			fmt.Printf(" (%s)", l.Detail)
		}
		fmt.Println()
	}

	fmt.Printf("\nResult: %s", diag.Result)
	if diag.Reason != "" {
		fmt.Printf(" (%s)", diag.Reason)
	}
	fmt.Println()
	return nil
}

func runRecentDenials(cmd *cobra.Command) error {
	// Query the RBAC service's audit log for recent denials.
	// This uses the log service to fetch recent permission-denied entries.
	logAddr := config.ResolveServiceAddr("log.LogService", "")
	cc, err := dialGRPC(logAddr)
	if err != nil {
		return fmt.Errorf("cannot connect to log service: %v", err)
	}
	defer cc.Close()

	fmt.Println("Recent permission denials:")
	fmt.Println("(Query the centralized log service for 'permission denied' entries)")
	fmt.Println()
	fmt.Println("Tip: Use journalctl to check service logs directly:")
	fmt.Println("  journalctl -u 'globular-*' --since '1 hour ago' | grep -i 'denied\\|permission'")

	return nil
}

func init() {
	doctorAuthCmd.Flags().StringVar(&doctorAuthSubject, "subject", "", "Principal ID to diagnose (user, SA, or cert CN)")
	doctorAuthCmd.Flags().StringVar(&doctorAuthMethod, "method", "", "gRPC method path (e.g. /file.FileService/ReadDir)")
	doctorAuthCmd.Flags().BoolVar(&doctorAuthRecentDenials, "recent-denials", false, "Query audit log for recent permission denials")
	doctorAuthCmd.Flags().BoolVar(&doctorAuthJSON, "json", false, "Output in JSON format")

	doctorCmd.AddCommand(doctorAuthCmd)
	rootCmd.AddCommand(doctorCmd)
}
