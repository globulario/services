package storage_store

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/gocql/gocql"
)

// SetScyllaLogger allows tests/callers to override the internal logger.
func SetScyllaLogger(l *slog.Logger) { if l != nil { bcLogger = l } }

// ScyllaStore is a simple key/value store backed by Scylla/Cassandra.
type ScyllaStore struct {
	cluster *gocql.ClusterConfig
	session *gocql.Session

	// serialization loop (optional, used by scylla_store_sync.go)
	actions chan action

	// resolved config
	keyspace string
	table    string
}

// action is used by the serialized loop in scylla_store_sync.go.
type action struct {
	name  string
	args  []any
	resCh chan any
	errCh chan error
}

// OpenOptions are the JSON-serializable options accepted by Open.
type OpenOptions struct {
	Hosts                     []string `json:"hosts"`
	Port                      int      `json:"port"`
	Username                  string   `json:"username"`
	Password                  string   `json:"password"`
	Keyspace                  string   `json:"keyspace"`
	Table                     string   `json:"table"`
	ReplicationFactor         int      `json:"replication_factor"`
	TimeoutMS                 int      `json:"timeout_ms"`
	ConnectTimeoutMS          int      `json:"connect_timeout_ms"`
	Consistency               string   `json:"consistency"`
	DisableInitialHostLookup  bool     `json:"disable_initial_host_lookup"`
	// TLS
	TLS                bool   `json:"tls"`
	CAFile             string `json:"ca_file"`   // aliases: "ca"
	CertFile           string `json:"cert_file"` // aliases: "cert"
	KeyFile            string `json:"key_file"`  // aliases: "key"
	InsecureSkipVerify bool   `json:"insecure_skip_verify"`
	ServerName         string `json:"server_name"`
	// Some deployments expose TLS on a different port.
	SSLPort            int    `json:"ssl_port"`
}



// parseOpenOptions reads options from r (JSON) and seeds missing values from
// address/keyspace/table fallback parameters.
//
// Supports aliases to match persistence_store/scylla.go:
//  - "ca" -> CAFile, "cert" -> CertFile, "key" -> KeyFile
func parseOpenOptions(r io.Reader, address, keyspaceFallback, tableFallback string) (OpenOptions, error) {
	opts := OpenOptions{}
	if r != nil {
		// decode into a generic map first to support key aliases
		raw := map[string]any{}
		dec := json.NewDecoder(r)
		if err := dec.Decode(&raw); err != nil && !errors.Is(err, io.EOF) {
			return opts, fmt.Errorf("decode options: %w", err)
		}
		// normalize keys
		if v, ok := raw["ca"]; ok && raw["ca_file"] == nil { raw["ca_file"] = v }
		if v, ok := raw["cert"]; ok && raw["cert_file"] == nil { raw["cert_file"] = v }
		if v, ok := raw["key"]; ok && raw["key_file"] == nil { raw["key_file"] = v }
		// marshal back into struct
		b, _ := json.Marshal(raw)
		if err := json.Unmarshal(b, &opts); err != nil {
			return opts, fmt.Errorf("map options: %w", err)
		}
	}
	// If user provided an address string in legacy path, seed Hosts from it.
	if len(opts.Hosts) == 0 && address != "" {
		for _, part := range strings.Split(address, ",") {
			part = strings.TrimSpace(part)
			if part == "" { continue }
			opts.Hosts = append(opts.Hosts, part)
		}
	}
	if opts.Port == 0 { opts.Port = 9042 }
	if opts.SSLPort == 0 { opts.SSLPort = 9142 } // common default for Scylla TLS
	if opts.Keyspace == "" {
		if keyspaceFallback != "" { opts.Keyspace = keyspaceFallback } else { opts.Keyspace = "cache" }
	}
	if opts.Table == "" {
		if tableFallback != "" { opts.Table = tableFallback } else { opts.Table = "kv" }
	}
	if opts.ReplicationFactor <= 0 { opts.ReplicationFactor = 1 }
	if opts.TimeoutMS <= 0 { opts.TimeoutMS = 5000 }
	if opts.ConnectTimeoutMS <= 0 { opts.ConnectTimeoutMS = 5000 }
	if opts.Consistency == "" { opts.Consistency = "quorum" }
	return opts, nil
}

func consistencyFromString(s string) gocql.Consistency {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "any": return gocql.Any
	case "one": return gocql.One
	case "two": return gocql.Two
	case "three": return gocql.Three
	case "quorum": return gocql.Quorum
	case "all": return gocql.All
	case "localquorum", "local_quorum", "local-quorum": return gocql.LocalQuorum
	default:
		return gocql.Quorum
	}
}

func (s *ScyllaStore) buildCluster(opts OpenOptions) (*gocql.ClusterConfig, error) {
	if len(opts.Hosts) == 0 {
		opts.Hosts = []string{"127.0.0.1"}
	}

	hosts := make([]string, 0, len(opts.Hosts))
	for _, h := range opts.Hosts {
		if strings.Contains(h, ":") {
			hosts = append(hosts, h) // already host:port
		} else {
			port := opts.Port
			if opts.TLS && opts.SSLPort > 0 {
				port = opts.SSLPort
			}
			hosts = append(hosts, fmt.Sprintf("%s:%d", h, port))
		}
	}

	cluster := gocql.NewCluster(hosts...)
	cluster.Timeout = time.Duration(opts.TimeoutMS) * time.Millisecond
	cluster.ConnectTimeout = time.Duration(opts.ConnectTimeoutMS) * time.Millisecond
	cluster.Consistency = consistencyFromString(opts.Consistency)
	cluster.DisableInitialHostLookup = opts.DisableInitialHostLookup
	if opts.Username != "" {
		cluster.Authenticator = gocql.PasswordAuthenticator{
			Username: opts.Username,
			Password: opts.Password,
		}
	}
	// TLS if requested.
	if opts.TLS {
		tlsCfg := &tls.Config{
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: opts.InsecureSkipVerify,
			ServerName:         opts.ServerName,
		}
		// CA
		if opts.CAFile != "" {
			caPEM, err := os.ReadFile(opts.CAFile)
			if err != nil {
				return nil, fmt.Errorf("read CA file: %w", err)
			}
			cp := x509.NewCertPool()
			if ok := cp.AppendCertsFromPEM(caPEM); !ok {
				return nil, errors.New("failed to parse CA file")
			}
			tlsCfg.RootCAs = cp
		}
		// Optional client cert
		if opts.CertFile != "" && opts.KeyFile != "" {
			crt, err := tls.LoadX509KeyPair(opts.CertFile, opts.KeyFile)
			if err != nil {
				return nil, fmt.Errorf("load client cert/key: %w", err)
			}
			tlsCfg.Certificates = []tls.Certificate{crt}
		}
		cluster.SslOpts = &gocql.SslOptions{Config: tlsCfg}
	}

	return cluster, nil
}

// ensureKeyspaceAndTable makes sure keyspace and table exist.
func (s *ScyllaStore) ensureKeyspaceAndTable(opts OpenOptions) error {
	sysCluster := *s.cluster // shallow copy
	sysCluster.Keyspace = "" // connect to system to create ks if needed
	sysSession, err := sysCluster.CreateSession()
	if err != nil {
		return fmt.Errorf("scylla connect (system): %w", err)
	}
	defer sysSession.Close()

	// Replication strategy: SimpleStrategy
	cql := fmt.Sprintf(`CREATE KEYSPACE IF NOT EXISTS "%s" WITH replication = {'class': 'SimpleStrategy', 'replication_factor': %d}`,
		opts.Keyspace, opts.ReplicationFactor)
	if err := sysSession.Query(cql).Consistency(gocql.All).Exec(); err != nil {
		return fmt.Errorf("create keyspace: %w", err)
	}
	// now create table
	tableCQL := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS "%s"."%s" (
		k text PRIMARY KEY,
		v blob,
		updated_at timestamp
	)`, opts.Keyspace, opts.Table)
	if err := sysSession.Query(tableCQL).Consistency(gocql.All).Exec(); err != nil {
		return fmt.Errorf("create table: %w", err)
	}

	return nil
}

// open creates the session and initializes schema.
func (s *ScyllaStore) open(r io.Reader, address, keyspace, table string) error {
	opts, err := parseOpenOptions(r, address, keyspace, table)
	if err != nil {
		return err
	}
	cluster, err := s.buildCluster(opts)
	if err != nil {
		return err
	}
	s.cluster = cluster
	s.keyspace = opts.Keyspace
	s.table = opts.Table

	if err := s.ensureKeyspaceAndTable(opts); err != nil {
		return err
	}
	// set ks on main cluster
	s.cluster.Keyspace = s.keyspace
	s.session, err = s.cluster.CreateSession()
	if err != nil {
		return fmt.Errorf("scylla connect (keyspace): %w", err)
	}
	bcLogger.Info("scylla connected", "hosts", s.cluster.Hosts, "keyspace", s.keyspace, "table", s.table, "tls", opts.TLS)
	return nil
}

func (s *ScyllaStore) close() error {
	if s.session != nil {
		s.session.Close()
		s.session = nil
	}
	return nil
}

// SetItem stores a value by key. If ttlSeconds > 0, it is applied.
func (s *ScyllaStore) setItem(key string, val []byte, ttlSeconds ...int) error {
	if s.session == nil {
		return errors.New("scylla not open")
	}
	cql := fmt.Sprintf(`INSERT INTO "%s"."%s" (k, v, updated_at) VALUES (?, ?, toTimestamp(now()))`, s.keyspace, s.table)
	var q *gocql.Query
	if len(ttlSeconds) > 0 && ttlSeconds[0] > 0 {
		cql = cql + " USING TTL ?"
		q = s.session.Query(cql, key, val, ttlSeconds[0])
	} else {
		q = s.session.Query(cql, key, val)
	}
	return q.Consistency(gocql.Quorum).Exec()
}

// GetItem loads a value by key. Returns (nil, gocql.ErrNotFound) if missing.
func (s *ScyllaStore) getItem(key string) ([]byte, error) {
	if s.session == nil {
		return nil, errors.New("scylla not open")
	}
	var val []byte
	cql := fmt.Sprintf(`SELECT v FROM "%s"."%s" WHERE k = ?`, s.keyspace, s.table)
	if err := s.session.Query(cql, key).Consistency(gocql.Quorum).Scan(&val); err != nil {
		return nil, err
	}
	return val, nil
}

// RemoveItem deletes a key.
func (s *ScyllaStore) removeItem(key string) error {
	if s.session == nil {
		return errors.New("scylla not open")
	}
	cql := fmt.Sprintf(`DELETE FROM "%s"."%s" WHERE k = ?`, s.keyspace, s.table)
	return s.session.Query(cql, key).Consistency(gocql.Quorum).Exec()
}

// Clear truncates the table.
func (s *ScyllaStore) clear() error {
	if s.session == nil {
		return errors.New("scylla not open")
	}
	cql := fmt.Sprintf(`TRUNCATE "%s"."%s"`, s.keyspace, s.table)
	return s.session.Query(cql).Consistency(gocql.All).Exec()
}

// Drop drops the table.
func (s *ScyllaStore) drop() error {
	if s.session == nil {
		return errors.New("scylla not open")
	}
	cql := fmt.Sprintf(`DROP TABLE IF EXISTS "%s"."%s"`, s.keyspace, s.table)
	return s.session.Query(cql).Consistency(gocql.All).Exec()
}

// Run starts the serialized action loop if actions channel is initialized.
func (s *ScyllaStore) Run(ctx context.Context) {
	if s.actions == nil { return }
	for {
		select {
		case <-ctx.Done():
			return
		case act := <-s.actions:
			var res any
			var err error
			switch strings.ToLower(act.name) {
			case "open":
				// Accept either a JSON options string or an io.Reader for options.
				var r io.Reader
				var address, ks, tbl string
				if len(act.args) > 0 {
					switch v := act.args[0].(type) {
					case string:
						if v != "" { r = strings.NewReader(v) } // <-- initialize reader from optionsStr
					case io.Reader:
						r = v
					default:
						// nil or unknown -> leave r nil; parseOpenOptions can handle that
					}
				}
				// Back-compat: allow address, keyspace, table in subsequent args if provided.
				if len(act.args) > 1 { if v, _ := act.args[1].(string); v != "" { address = v } }
				if len(act.args) > 2 { if v, _ := act.args[2].(string); v != "" { ks = v } }
				if len(act.args) > 3 { if v, _ := act.args[3].(string); v != "" { tbl = v } }
				err = s.open(r, address, ks, tbl)
			case "close":
				err = s.close()
			case "setitem":
				key := act.args[0].(string)
				val := act.args[1].([]byte)
				var ttl int
				if len(act.args) > 2 {
					if v, ok := act.args[2].(int); ok { ttl = v }
				}
				err = s.setItem(key, val, ttl)
			case "getitem":
				key := act.args[0].(string)
				res, err = s.getItem(key)
			case "removeitem":
				key := act.args[0].(string)
				err = s.removeItem(key)
			case "clear":
				err = s.clear()
			case "drop":
				err = s.drop()
			default:
				err = fmt.Errorf("unknown action: %s", act.name)
			}
			if err != nil {
				if act.errCh != nil { act.errCh <- err }
			} else {
				if act.resCh != nil { act.resCh <- res }
				if act.errCh != nil { act.errCh <- nil }
			}
		}
	}
}
