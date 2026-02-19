// auth_cmds.go: authentication helpers for the Globular CLI.
//
//   globular auth login --user <email> --password <pass>
//
// On success, the token is written to ~/.config/globular/token so that
// subsequent CLI invocations can auto-load it without repeating --token.

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/authentication/authenticationpb"
)

// defaultAuthPort is the fallback when etcd service discovery is unavailable.
const defaultAuthPort = 10020

// resolveAuthAddr discovers the authentication service endpoint.
func resolveAuthAddr() string {
	svc, err := config.ResolveService("authentication.AuthenticationService")
	if err == nil && svc != nil {
		var port int
		switch p := svc["Port"].(type) {
		case int:
			port = p
		case float64:
			port = int(p)
		}
		if port > 0 {
			return fmt.Sprintf("localhost:%d", port)
		}
	}
	return fmt.Sprintf("localhost:%d", defaultAuthPort)
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

			addr := resolveAuthAddr()
			cc, err := dialGRPC(addr)
			if err != nil {
				return err
			}
			defer cc.Close()

			client := authenticationpb.NewAuthenticationServiceClient(cc)
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

			// Write token to well-known file.
			tokenPath := tokenFilePath()
			if err := os.MkdirAll(filepath.Dir(tokenPath), 0700); err != nil {
				return fmt.Errorf("create token directory: %w", err)
			}
			if err := os.WriteFile(tokenPath, []byte(token), 0600); err != nil {
				return fmt.Errorf("write token file: %w", err)
			}

			fmt.Printf("Authenticated as %q\nToken saved to %s\n", authLoginUser, tokenPath)
			return nil
		},
	}
)

func init() {
	authLoginCmd.Flags().StringVar(&authLoginUser, "user", "", "User email or name")
	authLoginCmd.Flags().StringVar(&authLoginPassword, "password", "", "User password")

	authCmd.AddCommand(authLoginCmd)
	rootCmd.AddCommand(authCmd)
}
