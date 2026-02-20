// auth_cmds.go: authentication helpers for the Globular CLI.
//
//   globular auth login --user <email> --password <pass>
//
// On success, the token is written to ~/.config/globular/token so that
// subsequent CLI invocations can auto-load it without repeating --token.

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

func init() {
	authLoginCmd.Flags().StringVar(&authLoginUser, "user", "", "User email or name")
	authLoginCmd.Flags().StringVar(&authLoginPassword, "password", "", "User password")

	authRootPassCmd.Flags().StringVar(&rootOld, "old", "", "Current root password")
	authRootPassCmd.Flags().StringVar(&rootNew, "new", "", "New root password")
	authRootPassCmd.Flags().StringVar(&rootConfirm, "confirm", "", "Confirm new root password")

	authCmd.AddCommand(authLoginCmd, authRootPassCmd)
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
