package imap

// logger is the package-level structured logger for IMAP internals.
import (
	"errors"
	"log/slog"
	"os"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/backend"
)

// logger is the package-level structured logger for IMAP internals.
var logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

// Backend_impl implements go-imap's backend.Backend hooks used during IMAP authentication.
// It relies on a persistence Store (set by the enclosing service) to validate users,
// and it constructs a User_impl with account info on successful login.
//
// External dependencies expected to be set by the service before use:
//   - Store            : persistence client exposing CreateConnection / FindOne
//   - Backend_address  : string host/ip of the datastore
//   - Backend_port     : int port of the datastore
//
// Note: Names and exported prototypes are preserved for compatibility.
type Backend_impl struct{}

// Login authenticates an IMAP user using the persistence Store.
// It creates (or reuses) a datastore connection tied to the username,
// fetches the user's account document, and returns a User_impl.
//
// Parameters:
//   - connInfo: IMAP connection info (remote addr, TLS, etc.)
//   - username: login username
//   - password: login password
//
// Returns:
//   - backend.User on success
//   - error if authentication or account lookup fails
func (b *Backend_impl) Login(connInfo *imap.ConnInfo, username, password string) (backend.User, error) {
	// Basic validation
	if username == "" || password == "" {
		err := errors.New("imap login: missing username or password")
		logger.Warn("imap login validation failed", "username", username == "", "password_empty", password == "")
		return nil, err
	}

	// Build a per-user connection id for the datastore.
	connectionID := username + "_db"

	// Establish (or refresh) a connection to the persistence layer using user creds.
	if err := Store.CreateConnection(
		connectionID,              // id
		connectionID,              // name
		Backend_address,           // host
		float64(Backend_port),     // port
		0,                         // database index / tenant (service-specific)
		username,                  // user
		password,                  // password
		5000,                      // timeout ms
		"",                        // options
		false,                     // readonly
	); err != nil {
		logger.Error("imap login: create persistence connection failed",
			"user", username, "addr", Backend_address, "port", Backend_port, "err", err)
		// Return the original error for upstream handling.
		return nil, err
	}

	// Retrieve account info for this user.
	query := `{"name":"` + username + `"}`
	info, err := Store.FindOne("local_resource", "local_resource", "Accounts", query, "")
	if err != nil {
		logger.Error("imap login: account lookup failed",
			"user", username, "collection", "Accounts", "query", query, "err", err)
		return nil, err
	}
	if info == nil {
		err = errors.New("imap login: account not found")
		logger.Warn("imap login: account document not found",
			"user", username, "collection", "Accounts", "query", query)
		return nil, err
	}

	// Bind the retrieved account info to the IMAP user implementation.
	user := new(User_impl)
	user.info = info

	logger.Info("imap login success", "user", username,
		"remote", func() string {
			if connInfo != nil {
				return connInfo.RemoteAddr.String()
			}
			return ""
		}(),
		"tls", func() bool {
			if connInfo != nil && connInfo.TLS != nil {
				return true
			}
			return false
		}(),
	)

	return user, nil
}
