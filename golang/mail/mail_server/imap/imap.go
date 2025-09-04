package imap

import (
	"crypto/tls"
	"errors"
	"time"

	imap_server "github.com/emersion/go-imap/server"
	"github.com/globulario/services/golang/persistence/persistence_client"
	Utility "github.com/globulario/utility"
)


// -----------------------------------------------------------------------------
// Global backend configuration (populated by StartImap)
// -----------------------------------------------------------------------------

var (
	Store            *persistence_client.Persistence_Client
	Backend_address  string
	Backend_port     int
	Backend_password string
)

// saveMessage stores an IMAP message into the user's mailbox collection.
//
// Parameters:
//   - user:     logical user name (used to derive the database name: user + "_db")
//   - mailBox:  target collection (mailbox) name
//   - body:     raw message bytes
//   - flags:    IMAP flags to persist with the message
//   - date:     message internal date (also used to derive a simple UID)
//
// Returns:
//   - error: on serialization or persistence failure.
func saveMessage(user string, mailBox string, body []byte, flags []string, date time.Time) error {
	if Store == nil {
		err := errors.New("saveMessage: Store not initialized")
		logger.Error("imap saveMessage: missing Store",
			"user", user, "mailbox", mailBox, "err", err)
		return err
	}
	if user == "" || mailBox == "" {
		err := errors.New("saveMessage: empty user or mailbox")
		logger.Warn("imap saveMessage: invalid params",
			"user_empty", user == "", "mailbox_empty", mailBox == "")
		return err
	}

	data := map[string]interface{}{
		"Date":  date,
		"Flags": flags,
		"Size":  uint32(len(body)),
		"Body":  body,
		"Uid":   date.Unix(), // naive UID: internaldate seconds
	}

	jsonStr, err := Utility.ToJson(data)
	if err != nil {
		logger.Error("imap saveMessage: marshal failed",
			"user", user, "mailbox", mailBox, "err", err)
		return err
	}

	db := user + "_db"
	if _, err = Store.InsertOne("local_resource", db, mailBox, jsonStr, ""); err != nil {
		logger.Error("imap saveMessage: insert failed",
			"user", user, "mailbox", mailBox, "db", db, "size", len(body), "err", err)
		return errors.New("saveMessage: insert into database failed")
	}

	logger.Info("imap saveMessage: stored",
		"user", user, "mailbox", mailBox, "db", db, "bytes", len(body))
	return nil
}

// renameCollection renames a collection in the backend database.
//
// Parameters:
//   - database: source database name
//   - name:     current collection name
//   - rename:   new collection name
//
// Returns:
//   - error: on admin command failure.
func renameCollection(database string, name string, rename string) error {
	if Store == nil {
		err := errors.New("renameCollection: Store not initialized")
		logger.Error("imap renameCollection: missing Store",
			"db", database, "from", name, "to", rename, "err", err)
		return err
	}
	if database == "" || name == "" || rename == "" {
		err := errors.New("renameCollection: empty database/name/rename")
		logger.Warn("imap renameCollection: invalid params",
			"db_empty", database == "", "from_empty", name == "", "to_empty", rename == "")
		return err
	}

	// MongoDB admin renameCollection command
	script := "db=db.getSiblingDB('admin');" +
		"db.adminCommand({renameCollection:'" + database + "." + name + "', to:'" + database + "." + rename + "'})"

	if err := Store.RunAdminCmd("local_resource", "sa", Backend_password, script); err != nil {
		logger.Error("imap renameCollection: admin command failed",
			"db", database, "from", name, "to", rename, "err", err)
		return errors.New("renameCollection: backend admin command failed")
	}

	logger.Info("imap renameCollection: renamed",
		"db", database, "from", name, "to", rename)
	return nil
}

// startImap initializes and starts an IMAP server on the given port.
// If certFile/keyFile are provided, TLS is enabled; otherwise the server
// allows plaintext auth (for dev/test) on that port.
//
// This is unexported; use StartImap to configure and launch all variants.
func startImap(port int, keyFile string, certFile string) {
	be := new(Backend_impl)

	s := imap_server.New(be)
	s.Addr = "0.0.0.0:" + Utility.ToString(port)

	go func() {
		if certFile != "" && keyFile != "" {
			cer, err := tls.LoadX509KeyPair(certFile, keyFile)
			if err != nil {
				logger.Error("imap start: tls keypair load failed",
					"port", port, "cert", certFile, "key", keyFile, "err", err)
				return
			}
			s.TLSConfig = &tls.Config{Certificates: []tls.Certificate{cer}}

			logger.Info("imap start: serving TLS", "addr", s.Addr)
			if err := s.ListenAndServeTLS(); err != nil {
				logger.Error("imap start: tls serve failed", "addr", s.Addr, "err", err)
			}
			return
		}

		// Non-TLS: allow plaintext auth for dev/test environments only.
		s.AllowInsecureAuth = true
		logger.Info("imap start: serving plaintext (insecure)", "addr", s.Addr)
		if err := s.ListenAndServe(); err != nil {
			logger.Error("imap start: serve failed", "addr", s.Addr, "err", err)
		}
	}()
}

// StartImap initializes global backend configuration and launches IMAP servers
// on the specified ports (one plaintext and up to two TLS endpoints).
//
// Parameters:
//   - store:            persistence client used by the IMAP backend
//   - backendAddress:   datastore host/ip used by the backend
//   - backendPort:      datastore port used by the backend
//   - backendPassword:  admin password used for privileged operations
//   - keyFile:          TLS private key path (used for TLS ports)
//   - certFile:         TLS certificate path (used for TLS ports)
//   - port:             plaintext IMAP port
//   - tlsPort:          primary TLS IMAP port
//   - altPort:          alternate TLS IMAP port
func StartImap(
	store *persistence_client.Persistence_Client,
	backendAddress string,
	backendPort int,
	backendPassword string,
	keyFile string,
	certFile string,
	port int,
	tlsPort int,
	altPort int,
) {
	// Configure globals for backend usage by IMAP handlers.
	Store = store
	Backend_address = backendAddress
	Backend_port = backendPort
	Backend_password = backendPassword

	logger.Info("imap bootstrap",
		"backend_addr", Backend_address, "backend_port", Backend_port,
		"plain_port", port, "tls_port", tlsPort, "alt_tls_port", altPort)

	// Launch servers
	if port > 0 {
		startImap(port, "", "")
	}
	if tlsPort > 0 {
		startImap(tlsPort, keyFile, certFile)
	}
	if altPort > 0 {
		startImap(altPort, keyFile, certFile)
	}
}
