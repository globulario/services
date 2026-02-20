// auth_cmds.go: authentication helpers for the Globular CLI.
//
//   globular auth login --user <email> --password <pass>
//   globular auth install-certs
//
// On login success, the token is written to ~/.config/globular/token so that
// subsequent CLI invocations can auto-load it without repeating --token.
//
// install-certs calls IssueClientCertificate on the auth service (requires a
// valid token from 'auth login') and saves the resulting cluster CA, client
// certificate and client key to ~/.config/globular/pki/.

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/globulario/services/golang/authentication/authenticationpb"
	"github.com/globulario/services/golang/config"
)

// defaultAuthPort is the fallback when all discovery methods are unavailable.
// Must match the authentication_server binary's own default port.
const defaultAuthPort = 10004

// resolveAuthAddr discovers the authentication service endpoint.
// Discovery order:
//  1. etcd (authoritative registry in a running cluster; returns the best instance)
//  2. Local service config files in GetServicesConfigDir() (standalone / Day-0)
//  3. Hardcoded fallback
//
// In a cluster with multiple authentication instances, a random one is chosen
// to distribute load (see config.ResolveServiceAddr).
func resolveAuthAddr() string {
	return config.ResolveServiceAddr(
		"authentication.AuthenticationService",
		fmt.Sprintf("localhost:%d", defaultAuthPort),
	)
}

// tokenFilePath returns the canonical path for the cached token.
func tokenFilePath() string {
	home := os.Getenv("HOME")
	if home == "" {
		home, _ = os.UserHomeDir()
	}
	return filepath.Join(home, ".config", "globular", "token")
}

var (
	authCmd = &cobra.Command{
		Use:   "auth",
		Short: "Authentication helpers",
	}

	authLoginUser     string
	authLoginPassword string
	rootOld           string
	rootNew           string
	rootConfirm       string

	authLoginCmd = &cobra.Command{
		Use:   "login",
		Short: "Authenticate and cache a token for CLI use",
		RunE: func(cmd *cobra.Command, args []string) error {
			if authLoginUser == "" {
				return errors.New("--user is required")
			}
			if authLoginPassword == "" {
				return errors.New("--password is required")
			}

			if isDefaultRoot(authLoginUser, authLoginPassword) {
				fmt.Fprintln(os.Stderr, "WARNING: You are using the factory default root password. Run `globular auth root-passwd` to change it.")
			}

			conn, closeFn, err := authConnFactory()
			if err != nil {
				return err
			}
			if closeFn != nil {
				defer closeFn()
			}

			client := authClientFactory(conn)
			resp, err := client.Authenticate(ctxWithTimeout(), &authenticationpb.AuthenticateRqst{
				Name:     authLoginUser,
				Password: authLoginPassword,
			})
			if err != nil {
				return fmt.Errorf("authentication failed: %w", err)
			}

			token := resp.GetToken()
			if token == "" {
				return errors.New("server returned an empty token")
			}

			// Write token to well-known file (best-effort; warn on failure).
			tokenPath := tokenFilePath()
			savedMsg := ""
			if err := os.MkdirAll(filepath.Dir(tokenPath), 0700); err != nil {
				fmt.Fprintf(os.Stderr, "WARNING: could not create token directory: %v\n", err)
			} else if err := os.WriteFile(tokenPath, []byte(token), 0600); err != nil {
				fmt.Fprintf(os.Stderr, "WARNING: could not write token file: %v\n", err)
			} else {
				savedMsg = fmt.Sprintf("\nToken saved to %s", tokenPath)
			}

			fmt.Printf("Authenticated as %q%s\nToken: %s\n", authLoginUser, savedMsg, token)
			return nil
		},
	}

	authRootPassCmd = &cobra.Command{
		Use:   "root-passwd",
		Short: "Change the root (sa) account password",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(rootOld) == "" {
				return errors.New("--old is required")
			}
			if strings.TrimSpace(rootNew) == "" {
				return errors.New("--new is required")
			}
			if rootConfirm != "" && rootNew != rootConfirm {
				return errors.New("--confirm does not match --new")
			}
			if err := validatePasswordPolicyCLI(rootNew); err != nil {
				return err
			}

			conn, closeFn, err := authConnFactory()
			if err != nil {
				return err
			}
			if closeFn != nil {
				defer closeFn()
			}

			client := authClientFactory(conn)
			resp, err := client.SetRootPassword(ctxWithTimeout(), &authenticationpb.SetRootPasswordRequest{
				OldPassword: rootOld,
				NewPassword: rootNew,
			})
			if err != nil {
				return fmt.Errorf("set root password: %w", err)
			}
			fmt.Println("Root password updated successfully.")
			if tok := resp.GetToken(); tok != "" {
				fmt.Println("New token:", tok)
			}
			return nil
		},
	}
)

var authInstallCertsCmd = &cobra.Command{
	Use:   "install-certs",
	Short: "Obtain and install user client certificates from the cluster CA",
	Long: `Call IssueClientCertificate on the authentication service to obtain a
fresh client certificate signed by the cluster CA.  A valid authentication
token is required (run 'globular auth login' first).

Certificates are written to:
  ~/.config/globular/pki/ca.crt      (cluster CA)
  ~/.config/globular/pki/client.crt  (client certificate, 30-day validity)
  ~/.config/globular/pki/client.key  (client private key, mode 0600)

After running this command, 'globular pkg publish' and other commands that
require mTLS will work without a --ca flag or manual certificate setup.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if rootCfg.token == "" {
			return errors.New("authentication required: run 'globular auth login' first to obtain a token")
		}

		conn, closeFn, err := authConnFactory()
		if err != nil {
			return err
		}
		if closeFn != nil {
			defer closeFn()
		}

		client := authInstallCertsClientFactory(conn)
		resp, err := client.IssueClientCertificate(ctxWithTimeout(), &emptypb.Empty{})
		if err != nil {
			return fmt.Errorf("IssueClientCertificate: %w", err)
		}

		if len(resp.GetCaCrtPem()) == 0 || len(resp.GetClientCrtPem()) == 0 || len(resp.GetClientKeyPem()) == 0 {
			return errors.New("server returned incomplete certificate data")
		}

		// Resolve PKI directory
		pkiDir, err := userPKIPath(".")
		if err != nil {
			return fmt.Errorf("resolve PKI dir: %w", err)
		}
		if err := os.MkdirAll(pkiDir, 0700); err != nil {
			return fmt.Errorf("create PKI dir %s: %w", pkiDir, err)
		}

		type fileEntry struct {
			name string
			data []byte
			mode os.FileMode
		}
		files := []fileEntry{
			{"ca.crt", resp.GetCaCrtPem(), 0644},
			{"client.crt", resp.GetClientCrtPem(), 0644},
			{"client.key", resp.GetClientKeyPem(), 0600},
		}
		for _, f := range files {
			path := filepath.Join(pkiDir, f.name)
			if err := os.WriteFile(path, f.data, f.mode); err != nil {
				return fmt.Errorf("write %s: %w", path, err)
			}
		}

		fmt.Printf("Certificates installed to %s\n", pkiDir)
		fmt.Printf("  ca.crt     – cluster CA certificate\n")
		fmt.Printf("  client.crt – client certificate (30-day validity)\n")
		fmt.Printf("  client.key – client private key (mode 0600)\n")
		return nil
	},
}

func init() {
	authLoginCmd.Flags().StringVar(&authLoginUser, "user", "", "User email or name")
	authLoginCmd.Flags().StringVar(&authLoginPassword, "password", "", "User password")

	authRootPassCmd.Flags().StringVar(&rootOld, "old", "", "Current root password")
	authRootPassCmd.Flags().StringVar(&rootNew, "new", "", "New root password")
	authRootPassCmd.Flags().StringVar(&rootConfirm, "confirm", "", "Confirm new root password")

	authCmd.AddCommand(authLoginCmd, authRootPassCmd, authInstallCertsCmd)
	rootCmd.AddCommand(authCmd)
}

// isDefaultRoot returns true when user looks like root and password is factory default.
func isDefaultRoot(user, password string) bool {
	lu := strings.ToLower(strings.TrimSpace(user))
	return (lu == "sa" || strings.HasPrefix(lu, "sa@")) && password == "adminadmin"
}

func validatePasswordPolicyCLI(pw string) error {
	if len(pw) < 12 {
		return errors.New("password must be at least 12 characters")
	}
	hasLower := strings.IndexFunc(pw, func(r rune) bool { return r >= 'a' && r <= 'z' }) >= 0
	hasUpper := strings.IndexFunc(pw, func(r rune) bool { return r >= 'A' && r <= 'Z' }) >= 0
	hasDigit := strings.IndexFunc(pw, func(r rune) bool { return r >= '0' && r <= '9' }) >= 0
	hasSpecial := strings.IndexFunc(pw, func(r rune) bool {
		return r >= 33 && r <= 126 && !(r >= '0' && r <= '9') && !(r >= 'a' && r <= 'z') && !(r >= 'A' && r <= 'Z')
	}) >= 0
	classes := 0
	for _, ok := range []bool{hasLower, hasUpper, hasDigit, hasSpecial} {
		if ok {
			classes++
		}
	}
	if classes < 3 {
		return errors.New("password must include at least 3 of: lowercase, uppercase, digit, special")
	}
	if strings.IndexFunc(pw, func(r rune) bool { return r <= 32 }) >= 0 {
		return errors.New("password may not contain spaces or control characters")
	}
	return nil
}

// seams for testing
var authConnFactory = func() (grpc.ClientConnInterface, func(), error) {
	addr := resolveAuthAddr()
	conn, err := dialGRPC(addr)
	if err != nil {
		return nil, nil, err
	}
	return conn, func() {
		if conn != nil {
			conn.Close()
		}
	}, nil
}

type authServiceClient interface {
	Authenticate(ctx context.Context, in *authenticationpb.AuthenticateRqst, opts ...grpc.CallOption) (*authenticationpb.AuthenticateRsp, error)
	SetRootPassword(ctx context.Context, in *authenticationpb.SetRootPasswordRequest, opts ...grpc.CallOption) (*authenticationpb.SetRootPasswordResponse, error)
}

var authClientFactory = func(conn grpc.ClientConnInterface) authServiceClient {
	return authenticationpb.NewAuthenticationServiceClient(conn)
}

// authInstallCertsClient is the interface for the install-certs command.
// Kept separate from authServiceClient so existing test fakes are unaffected.
type authInstallCertsClient interface {
	IssueClientCertificate(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*authenticationpb.IssueClientCertificateResponse, error)
}

var authInstallCertsClientFactory = func(conn grpc.ClientConnInterface) authInstallCertsClient {
	return authenticationpb.NewAuthenticationServiceClient(conn)
}
