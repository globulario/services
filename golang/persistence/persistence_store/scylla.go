// scylla.go
package persistence_store

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/globulario/services/golang/config"
	Utility "github.com/globulario/utility"
	"github.com/gocql/gocql"
)

// ---------- Connection & Store ----------

type ScyllaConnection struct {
	Id       string
	Hosts    []string
	Port     int
	Options  map[string]string
	sessions map[string]*gocql.Session
}

type ScyllaStore struct {
	connections map[string]*ScyllaConnection // live connections keyed by connection id
	lock        sync.Mutex                   // guards connections
}

func (store *ScyllaStore) GetStoreType() string { return "SCYLLA" }

// ---------- Connection Retry ----------

// retryCreateSession attempts to create a session with exponential backoff.
// Retries: 1s, 2s, 4s, 8s, 16s, 32s (total ~63 seconds, stays under 90s timeout)
func retryCreateSession(cluster *gocql.ClusterConfig, keyspace string) (*gocql.Session, error) {
	maxRetries := 6
	backoff := time.Second

	for attempt := 0; attempt <= maxRetries; attempt++ {
		session, err := cluster.CreateSession()
		if err == nil {
			if attempt > 0 {
				slog.Info("scylla: connection established after retries", "keyspace", keyspace, "attempts", attempt+1)
			}
			return session, nil
		}

		if attempt < maxRetries {
			slog.Warn("scylla: connection attempt failed, retrying",
				"keyspace", keyspace,
				"attempt", attempt+1,
				"next_retry_in", backoff.String(),
				"err", err)
			time.Sleep(backoff)
			backoff *= 2
		} else {
			slog.Error("scylla: all connection attempts exhausted",
				"keyspace", keyspace,
				"total_attempts", attempt+1,
				"err", err)
			return nil, fmt.Errorf("failed to connect after %d attempts: %w", attempt+1, err)
		}
	}

	return nil, errors.New("unexpected retry loop exit")
}

// ---------- Helpers ----------

func camelToSnake(input string) string {
	runes := []rune(input)
	n := len(runes)
	var result bytes.Buffer
	for i, r := range runes {
		if i > 0 && unicode.IsUpper(r) {
			prevLower := unicode.IsLower(runes[i-1])
			nextLower := i+1 < n && unicode.IsLower(runes[i+1])
			if prevLower || nextLower {
				result.WriteRune('_')
			}
		}
		result.WriteRune(unicode.ToLower(r))
	}
	return result.String()
}

func snakeToCamel(input string) string {
	var result bytes.Buffer
	upper := false
	for _, r := range input {
		if r == '_' {
			upper = true
			continue
		}
		if upper {
			result.WriteRune(unicode.ToUpper(r))
			upper = false
		} else {
			result.WriteRune(r)
		}
	}
	return result.String()
}

func joinStrings(slice []string, separator string) string {
	if len(slice) == 0 {
		return ""
	}
	result := slice[0]
	for i := 1; i < len(slice); i++ {
		result += separator + slice[i]
	}
	return result
}

func parseOptions(s string) map[string]string {
	m := map[string]string{}
	for _, part := range strings.Split(s, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		kv := strings.SplitN(part, "=", 2)
		if len(kv) == 2 {
			m[strings.ToLower(strings.TrimSpace(kv[0]))] = strings.TrimSpace(kv[1])
		}
	}
	return m
}

// accept JSON options too (e.g. {"replication_factor":1,"dc":"dc1","consistency":"ONE","tls":true})
func mergeJSONOptions(opts map[string]string, s string) {
	trim := strings.TrimSpace(s)
	if !strings.HasPrefix(trim, "{") {
		return
	}
	var mj map[string]interface{}
	if json.Unmarshal([]byte(trim), &mj) != nil {
		return
	}
	for k, v := range mj {
		ks := strings.ToLower(k)
		vs := fmt.Sprint(v)
		switch ks {
		case "replication_factor", "rf":
			opts["rf"] = vs
		case "datacenter", "dc":
			opts["dc"] = vs
		case "consistency":
			opts["consistency"] = strings.ToUpper(vs)
		case "tls":
			opts["tls"] = vs
		case "ca":
			opts["ca"] = vs
		case "cert":
			opts["cert"] = vs
		case "key":
			opts["key"] = vs
		case "insecure_skip_verify":
			opts["insecure_skip_verify"] = vs
		case "server_name":
			opts["server_name"] = vs
		case "ssl_port":
			opts["ssl_port"] = vs
		}
	}
}

func deduceColumnType(value interface{}) string {
	goType := reflect.TypeOf(value)
	switch goType.Kind() {
	case reflect.Bool:
		return "boolean"
	case reflect.Int64:
		return "bigint"
	case reflect.Int, reflect.Int32:
		return "int"
	case reflect.Float64:
		return "double"
	case reflect.String:
		return "text"
	case reflect.Slice:
		return "array"
	case reflect.Map:
		return "map"
	default:
		return ""
	}
}

// ---- New helpers for reference inference ----
// ListLinkedIDs returns, for baseCollection/id, a map like:
//
//	{"roles": ["r1","r2"], "groups": ["g3"] }
func (store *ScyllaStore) ListLinkedIDs(connectionId, keyspace, baseCollection, id string) (map[string][]string, error) {
	session, err := store.getSession(connectionId, keyspace)
	if err != nil {
		return nil, err
	}

	base := strings.ToLower(strings.TrimSpace(baseCollection))
	if !strings.HasSuffix(base, "s") {
		base += "s"
	}

	out := map[string][]string{}

	iter := session.Query(`SELECT table_name FROM system_schema.tables WHERE keyspace_name = ?`, keyspace).Iter()
	defer iter.Close()

	var t string
	for iter.Scan(&t) {
		parts := strings.SplitN(strings.ToLower(t), "_", 2)
		if len(parts) != 2 {
			continue
		}
		a, b := parts[0], parts[1]

		// canonical link table: has (source_id,target_id)
		cols, _ := store.getTableColumns(session, keyspace, t)
		if _, ok := cols["source_id"]; !ok {
			continue
		}
		if _, ok := cols["target_id"]; !ok {
			continue
		}

		var q string
		var other string
		if a == base {
			q = fmt.Sprintf("SELECT target_id FROM %s.%s WHERE source_id = ?", keyspace, t)
			other = b
		} else if b == base {
			q = fmt.Sprintf("SELECT source_id FROM %s.%s WHERE target_id = ?", keyspace, t)
			other = a
		} else {
			continue
		}

		it := session.Query(q, id).Iter()
		row := map[string]interface{}{}
		for it.MapScan(row) {
			if a == base {
				out[other] = append(out[other], Utility.ToString(row["target_id"]))
			} else {
				out[other] = append(out[other], Utility.ToString(row["source_id"]))
			}
			row = map[string]interface{}{}
		}
		_ = it.Close()
	}
	if err := iter.Close(); err != nil {
		return nil, err
	}
	return out, nil
}

// extract a best-effort id from a reference/entity map (more aggressive)
func extractID(m map[string]interface{}) string {
	candidates := []string{
		"id", "_id", "$id",
		"account", "accountId", "account_id",
		"name", "email", "path",
	}
	for _, k := range candidates {
		if s := strings.TrimSpace(Utility.ToString(m[k])); s != "" {
			return s
		}
	}
	return ""
}

// ---------- Canonical link-table helpers (NEW) ----------

func (store *ScyllaStore) buildCluster(hosts []string, port int, keyspace string, opts map[string]string) *gocql.ClusterConfig {
	cluster := gocql.NewCluster(hosts...)

	// Keyspace
	if keyspace != "" {
		cluster.Keyspace = keyspace
	}

	// Consistency
	switch strings.ToUpper(opts["consistency"]) {
	case "ONE":
		cluster.Consistency = gocql.One
	case "LOCAL_ONE":
		cluster.Consistency = gocql.LocalOne
	case "QUORUM":
		cluster.Consistency = gocql.Quorum
	case "LOCAL_QUORUM":
		cluster.Consistency = gocql.LocalQuorum
	case "ALL":
		cluster.Consistency = gocql.All
	default:
		cluster.Consistency = gocql.One // default safe on single node
	}

	// Auth
	if u, uok := opts["username"]; uok {
		if p, pok := opts["password"]; pok {
			cluster.Authenticator = gocql.PasswordAuthenticator{Username: u, Password: p}
		}
	}

	// Timeouts
	if toStr := opts["timeout_ms"]; toStr != "" {
		if ms, _ := strconv.Atoi(toStr); ms > 0 {
			cluster.Timeout = time.Duration(ms) * time.Millisecond
		}
	}

	// ----- TLS (only if requested) -----
	tlsEnabled := strings.EqualFold(opts["tls"], "true")
	if tlsEnabled {
		cfg := &tls.Config{
			MinVersion: tls.VersionTLS12,
		}

		// CA trust
		if caPath := strings.TrimSpace(opts["ca"]); caPath != "" {
			if pem, err := os.ReadFile(caPath); err == nil {
				pool := x509.NewCertPool()
				if pool.AppendCertsFromPEM(pem) {
					cfg.RootCAs = pool
				}
			}
		}

		// Client cert (mutual TLS)
		if certPath, keyPath := strings.TrimSpace(opts["cert"]), strings.TrimSpace(opts["key"]); certPath != "" && keyPath != "" {
			if crt, err := tls.LoadX509KeyPair(certPath, keyPath); err == nil {
				cfg.Certificates = []tls.Certificate{crt}
			}
		}

		// Hostname verification
		insecure := strings.EqualFold(opts["insecure_skip_verify"], "true")
		if insecure {
			cfg.InsecureSkipVerify = true
		} else if sni := strings.TrimSpace(opts["server_name"]); sni != "" {
			cfg.ServerName = sni
		}

		cluster.SslOpts = &gocql.SslOptions{
			Config:                 cfg,
			EnableHostVerification: !insecure,
		}

		if p := strings.TrimSpace(opts["ssl_port"]); p != "" {
			if sslPort, err := strconv.Atoi(p); err == nil && sslPort > 0 {
				cluster.Port = sslPort
			}
		}
	}

	// ----- Port (set last so TLS block can override with ssl_port) -----
	if cluster.Port == 0 {
		if port > 0 {
			cluster.Port = port
		} else {
			cluster.Port = 9042
		}
	}

	return cluster
}

// ---------- Keyspace (Database) management ----------

func (store *ScyllaStore) createKeyspace(connectionId, keyspace string) (*gocql.ClusterConfig, error) {
	if len(keyspace) == 0 {
		return nil, errors.New("the database is required")
	}
	keyspace = strings.ToLower(strings.ReplaceAll(keyspace, "-", "_"))

	store.lock.Lock()
	connection := store.connections[connectionId]
	store.lock.Unlock()
	if connection == nil {
		return nil, errors.New("the connection does not exist")
	}

	// Admin session on "system" with retry logic
	adminCluster := store.buildCluster(connection.Hosts, connection.Port, "system", connection.Options)
	adminSession, err := retryCreateSession(adminCluster, "system")
	if err != nil {
		return nil, fmt.Errorf("create admin session: %w", err)
	}
	defer adminSession.Close()

	// Strategy: default RF=1 unless options specify otherwise
	rf := connection.Options["rf"]
	dc := connection.Options["dc"]
	if rf == "" {
		rf = "1"
	}
	var cql string
	if rf != "" && dc != "" {
		cql = fmt.Sprintf(
			`CREATE KEYSPACE IF NOT EXISTS %s WITH replication = {'class': 'NetworkTopologyStrategy','%s': %s}`,
			keyspace, dc, rf,
		)
	} else {
		cql = fmt.Sprintf(
			`CREATE KEYSPACE IF NOT EXISTS %s WITH replication = {'class': 'SimpleStrategy','replication_factor': %s}`,
			keyspace, rf,
		)
	}

	if err := adminSession.Query(cql).Exec(); err != nil {
		slog.Error("scylla: create keyspace failed", "keyspace", keyspace, "err", err)
		return nil, err
	}

	return store.buildCluster(connection.Hosts, connection.Port, keyspace, connection.Options), nil
}

func (store *ScyllaStore) CreateDatabase(ctx context.Context, connectionId string, keyspace string) error {
	_, err := store.createKeyspace(connectionId, keyspace)
	return err
}

func dropKeyspace(session *gocql.Session, keyspace string) error {
	return session.Query(fmt.Sprintf("DROP KEYSPACE IF EXISTS %s;", keyspace)).Exec()
}

func (store *ScyllaStore) DeleteDatabase(ctx context.Context, connectionId string, keyspace string) error {
	if len(keyspace) == 0 {
		return errors.New("the database is required")
	}
	store.lock.Lock()
	connection := store.connections[connectionId]
	store.lock.Unlock()
	if connection == nil {
		return errors.New("the connection does not exist")
	}
	adminCluster := store.buildCluster(connection.Hosts, connection.Port, "system", connection.Options)
	adminSession, err := adminCluster.CreateSession()
	if err != nil {
		return err
	}
	defer adminSession.Close()
	if err := dropKeyspace(adminSession, keyspace); err != nil {
		slog.Error("scylla: drop keyspace failed", "keyspace", keyspace, "err", err)
		return err
	}
	return nil
}

// ---------- Connection lifecycle ----------

func (store *ScyllaStore) Connect(id string, host string, port int32, user string, password string, keyspace string, timeout int32, options_str string) error {
	if id == "" {
		return errors.New("the connection id is required")
	}
	if strings.TrimSpace(host) == "" && strings.TrimSpace(options_str) != "" {
		opts := parseOptions(options_str)
		mergeJSONOptions(opts, options_str)
		for _, key := range []string{"host", "hosts", "contact_points"} {
			if v, ok := opts[key]; ok && strings.TrimSpace(v) != "" {
				host = strings.TrimSpace(strings.Split(v, ",")[0])
				break
			}
		}
	}
	if strings.TrimSpace(host) == "" {
		return errors.New("the host is required")
	}
	if keyspace == "" {
		return errors.New("the database is required")
	}

	// Normalize hosts & options
	var hosts []string
	for _, h := range strings.Split(host, ",") {
		h = strings.TrimSpace(h)
		if h == "" {
			continue
		}
		hosts = append(hosts, strings.Split(h, ":")[0])
	}
	if len(hosts) == 0 {
		return errors.New("no valid host provided")
	}

	opts := parseOptions(options_str)   // k=v;k2=v2
	mergeJSONOptions(opts, options_str) // and/or JSON

	if user != "" {
		opts["username"] = user
	}
	if password != "" {
		opts["password"] = password
	}
	effectivePort := 9042
	if port > 0 {
		effectivePort = int(port)
	}
	if _, ok := opts["consistency"]; !ok {
		opts["consistency"] = "ONE"
	}
	if timeout > 0 {
		opts["timeout_ms"] = fmt.Sprintf("%d", timeout)
	}

	if store.connections == nil {
		store.connections = make(map[string]*ScyllaConnection)
	}

	store.lock.Lock()
	if c, ok := store.connections[id]; ok && c.sessions != nil {
		if _, ok := c.sessions[keyspace]; ok {
			for k, v := range opts {
				c.Options[k] = v
			}
			store.lock.Unlock()
			return nil
		}
	}
	store.lock.Unlock()

	store.lock.Lock()
	connection := store.connections[id]
	if connection == nil {
		connection = &ScyllaConnection{
			Id:       id,
			Hosts:    hosts,
			Port:     effectivePort,
			Options:  opts,
			sessions: make(map[string]*gocql.Session),
		}
		store.connections[id] = connection
	} else {
		connection.Hosts = hosts
		connection.Port = effectivePort
		if connection.Options == nil {
			connection.Options = map[string]string{}
		}
		for k, v := range opts {
			connection.Options[k] = v
		}
	}
	store.lock.Unlock()

	cluster, err := store.createKeyspace(id, keyspace)
	if err != nil {
		return err
	}
	session, err := retryCreateSession(cluster, keyspace)
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	connection.sessions[keyspace] = session

	// Best-effort canonical migration so legacy <b>_<a> tables are collapsed into <a>_<b>
	if err := store.MigrateLinkTables(id, keyspace); err != nil {
		slog.Warn("scylla: link-table migration skipped/failed", "keyspace", keyspace, "err", err)
	}

	slog.Info("scylla: connected", "id", id, "hosts", hosts, "port", effectivePort, "keyspace", keyspace)
	return nil
}

func (store *ScyllaStore) GetSession(connectionId string) *gocql.Session {
	store.lock.Lock()
	connection := store.connections[connectionId]
	store.lock.Unlock()
	if connection == nil {
		return nil
	}
	for _, session := range connection.sessions {
		return session
	}
	return nil
}

func (store *ScyllaStore) Disconnect(connectionId string) error {
	if store.connections != nil {
		store.lock.Lock()
		if c, ok := store.connections[connectionId]; ok && c.sessions != nil {
			for _, session := range c.sessions {
				session.Close()
			}
		}
		store.lock.Unlock()
	}
	slog.Info("scylla: disconnected", "id", connectionId)
	return nil
}

func (store *ScyllaStore) Ping(ctx context.Context, connectionId string) error {
	store.lock.Lock()
	connection := store.connections[connectionId]
	store.lock.Unlock()
	if connection == nil {
		return errors.New("the connection does not exist")
	}
	var session *gocql.Session
	for _, s := range connection.sessions {
		session = s
		break
	}
	if session == nil {
		return errors.New("the session does not exist")
	}
	if err := session.Query("SELECT release_version FROM system.local").WithContext(ctx).Exec(); err != nil {
		slog.Error("scylla: ping failed", "err", err)
		return err
	}
	return nil
}

// ---------- DDL helpers ----------

func (store *ScyllaStore) createScyllaTable(session *gocql.Session, keyspace, tableName string, data map[string]interface{}) error {
	if data["_id"] == nil && data["id"] == nil {
		return errors.New("the _id is required")
	}

	createTableQuery := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s.%s (", keyspace, tableName)
	haveID := false

	for fieldName, value := range data {
		if value == nil {
			continue
		}
		fieldType := deduceColumnType(value)
		if fieldType == "" || fieldType == "array" || fieldType == "map" {
			// arrays & maps live in side tables
			continue
		}

		col := camelToSnake(fieldName)
		if col == "_id" || col == "id" {
			col = "id"
			haveID = true
		}
		createTableQuery += col + " " + fieldType + ", "
	}

	if !haveID {
		createTableQuery += "id text, "
	}
	createTableQuery += "PRIMARY KEY (id));"

	if err := session.Query(createTableQuery).Exec(); err != nil {
		slog.Error("scylla: create table failed", "table", tableName, "err", err)
		return err
	}

	return nil
}

// ---------- Query formatting ----------

func (store *ScyllaStore) getParameters(condition string, values []interface{}) string {
	query := ""
	switch condition {
	case "$and":
		for _, v := range values {
			value := v.(map[string]interface{})
			for key, vv := range value {
				if key == "_id" {
					key = "id"
					if s, ok := vv.(string); ok && strings.Contains(s, "@") {
						vv = strings.Split(s, "@")[0]
					}
				}
				key = camelToSnake(key)
				if reflect.TypeOf(vv).Kind() == reflect.String {
					query += fmt.Sprintf("%s = '%v' AND ", key, vv)
				}
			}
		}
		query = strings.TrimSuffix(query, " AND ")
	case "$or":
		for _, v := range values {
			value := v.(map[string]interface{})
			for key, vv := range value {
				if key == "_id" {
					key = "id"
					if s, ok := vv.(string); ok && strings.Contains(s, "@") {
						vv = strings.Split(s, "@")[0]
					}
				}
				key = camelToSnake(key)
				if reflect.TypeOf(vv).Kind() == reflect.String {
					query += fmt.Sprintf("%s = '%v' OR ", key, vv)
				}
			}
		}
		query = strings.TrimSuffix(query, " OR ")
	}
	return query
}

func (store *ScyllaStore) formatQuery(keyspace, table, q string) (string, error) {
	if q == "{}" {
		return fmt.Sprintf("SELECT * FROM %s.%s", keyspace, table), nil
	}
	params := make(map[string]interface{})
	if err := json.Unmarshal([]byte(q), &params); err != nil {
		slog.Error("scylla: unmarshal query failed", "q", q, "err", err)
		return "", err
	}
	query := fmt.Sprintf("SELECT * FROM %s.%s WHERE ", keyspace, table)
	for key, value := range params {
		if key == "_id" {
			key = "id"
			if s, ok := value.(string); ok && strings.Contains(s, "@") {
				value = strings.Split(s, "@")[0]
			}
		}
		key = camelToSnake(key)
		switch reflect.TypeOf(value).Kind() {
		case reflect.String:
			query += fmt.Sprintf("%s = '%v' AND ", key, value)
		case reflect.Slice:
			if key == "$and" || key == "$or" || key == "$regex" {
				query += store.getParameters(key, value.([]interface{}))
			}
		case reflect.Map:
			for k, v := range value.(map[string]interface{}) {
				if k == "$regex" {
					query += fmt.Sprintf("%s LIKE '%v%%' AND ", key, v)
				}
			}
		}
	}
	return strings.TrimSuffix(query, " AND "), nil
}

// ---------- Read paths ----------

func (store *ScyllaStore) getSession(connectionId, keyspace string) (*gocql.Session, error) {
	if keyspace == "" {
		return nil, errors.New("the database is required")
	}
	if connectionId == "" {
		return nil, errors.New("the connection id is required")
	}
	store.lock.Lock()
	connection := store.connections[connectionId]
	store.lock.Unlock()
	if connection == nil {
		return nil, errors.New("the connection " + connectionId + " does not exist")
	}
	if session, ok := connection.sessions[keyspace]; ok {
		return session, nil
	}
	return nil, errors.New("connection with id " + connectionId + " does not have a session for keyspace " + keyspace)
}

// Reworked: bi-directional array entity initialization using canonical link tables
func (store *ScyllaStore) initArrayEntities(connectionId, keyspace, linkTable string, entity map[string]interface{}) error {
	session, err := store.getSession(connectionId, keyspace)
	if err != nil {
		return err
	}
	typeName, _ := entity["typeName"].(string)
	if typeName == "" {
		return fmt.Errorf("initArrayEntities: missing typeName in entity")
	}
	base := strings.ToLower(typeName)
	if !strings.HasSuffix(base, "s") {
		base += "s"
	}

	id := Utility.ToString(entity["_id"])
	if id == "" {
		id = Utility.ToString(entity["id"])
	}
	if id == "" {
		return fmt.Errorf("initArrayEntities: missing id")
	}

	parts := strings.SplitN(strings.ToLower(linkTable), "_", 2)
	if len(parts) != 2 {
		return nil
	}

	// only follow canonical link tables
	lc, rc := canonicalPair(parts[0], parts[1])
	if strings.ToLower(linkTable) != lc+"_"+rc {
		return nil // ignore legacy non-canonical table
	}

	a, b := parts[0], parts[1]
	baseIsA := (base == a)
	baseIsB := (base == b)
	if !baseIsA && !baseIsB {
		return nil
	}

	other := b
	if baseIsB {
		other = a
	}

	// query: if base is first token, id is in source_id; else in target_id
	var q string
	if baseIsA {
		q = fmt.Sprintf("SELECT target_id FROM %s.%s WHERE source_id = ?", keyspace, linkTable)
	} else {
		q = fmt.Sprintf("SELECT source_id FROM %s.%s WHERE target_id = ?", keyspace, linkTable)
	}

	iter := session.Query(q, id).Iter()
	defer iter.Close()

	array := []interface{}{}
	col := "target_id"
	if baseIsB {
		col = "source_id"
	}
	for {
		row := make(map[string]interface{})
		if !iter.MapScan(row) {
			break
		}
		if tid, ok := row[col]; ok {
			refTable := other
			array = append(array, map[string]interface{}{"$ref": refTable, "$id": tid, "$db": keyspace})
		}
	}
	if len(array) > 0 {
		entity[snakeToCamel(other)] = array
	}

	return nil
}

func (store *ScyllaStore) initArrayValues(connectionId, keyspace, tableName string, entity map[string]interface{}) error {
	session, err := store.getSession(connectionId, keyspace)
	if err != nil {
		return err
	}
	parts := strings.SplitN(tableName, "_", 2)
	if len(parts) != 2 {
		return nil
	}
	base := parts[0]
	fk := fmt.Sprintf("%s_id", base)
	q := fmt.Sprintf("SELECT value FROM %s.%s WHERE %s = ?", keyspace, tableName, fk)
	iter := session.Query(q, entity["_id"]).Iter()
	defer iter.Close()

	var array []interface{}
	for {
		row := make(map[string]interface{})
		if !iter.MapScan(row) {
			break
		}
		if v, ok := row["value"]; ok {
			array = append(array, v)
		}
	}
	field := snakeToCamel(parts[1])
	entity[field] = array
	return nil
}

func (store *ScyllaStore) getTableColumns(session *gocql.Session, keyspace, table string) (map[string]struct{}, error) {
	cols := map[string]struct{}{}
	iter := session.Query(
		"SELECT column_name FROM system_schema.columns WHERE keyspace_name = ? AND table_name = ?",
		keyspace, table,
	).Iter()
	defer iter.Close()
	var name string
	for iter.Scan(&name) {
		cols[strings.ToLower(name)] = struct{}{}
	}
	return cols, nil
}

func (store *ScyllaStore) initEntity(connectionId, keyspace, typeName string, entity map[string]interface{}) (map[string]interface{}, error) {
	if typeName == "" {
		return nil, fmt.Errorf("the type name is required")
	}
	// Convert column names to camelCase.
	for key, value := range entity {
		delete(entity, key)
		entity[snakeToCamel(key)] = value
	}
	// Normalize id/_id
	if entity["id"] != nil {
		entity["_id"] = entity["id"]
		delete(entity, "id")
	}
	if entity["_id"] == nil {
		return nil, fmt.Errorf("the _id is required")
	}

	entity["typeName"] = typeName
	if entity["domain"] == nil {
		if localDomain, _ := config.GetDomain(); localDomain != "" {
			entity["domain"] = localDomain
		}
	}

	session, err := store.getSession(connectionId, keyspace)
	if err != nil {
		return nil, err
	}

	// Discover side tables and load arrays.
	iter := session.Query(
		"SELECT table_name FROM system_schema.tables WHERE keyspace_name = ?",
		keyspace,
	).Iter()
	defer iter.Close()

	base := strings.ToLower(typeName)
	if !strings.HasSuffix(base, "s") {
		base += "s"
	}

	var tName string
	for iter.Scan(&tName) {
		lower := strings.ToLower(tName)
		parts := strings.SplitN(lower, "_", 2)
		if len(parts) != 2 {
			continue
		}

		cols, _ := store.getTableColumns(session, keyspace, tName)

		// ---- LINK TABLES (refs) ----
		if _, ok := cols["source_id"]; ok {
			if _, ok2 := cols["target_id"]; ok2 {
				// Apply canonicalization ONLY to link tables
				lc, rc := canonicalPair(parts[0], parts[1])
				if lower != lc+"_"+rc {
					// non-canonical, skip to avoid dup loading
					continue
				}
				// follow links only if base matches one side
				if parts[0] == base || parts[1] == base {
					_ = store.initArrayEntities(connectionId, keyspace, tName, entity)
				}
				continue
			}
		}

		// ---- SCALAR ARRAY TABLES (<base>_<field> with <base>_id, value) ----
		// DO NOT canonicalize here — <base>_<field> is the actual table name

		pkCol := parts[0] + "_id"

		if _, ok := cols[pkCol]; ok {
			if _, ok2 := cols["value"]; ok2 {

				_ = store.initArrayValues(connectionId, keyspace, tName, entity)
				continue
			}
		}
	}

	return entity, nil
}

func (store *ScyllaStore) find(connectionId, keyspace, table, query string) ([]map[string]interface{}, error) {
	if keyspace == "" {
		return nil, errors.New("the database is required")
	}
	if connectionId == "" {
		return nil, errors.New("the connection id is required")
	}
	if table == "" {
		return nil, errors.New("the table is required")
	}
	if query == "" {
		return nil, errors.New("query is empty")
	}
	session, err := store.getSession(connectionId, keyspace)
	if err != nil {
		return nil, err
	}
	// JSON → CQL
	if strings.HasPrefix(query, "{") && strings.HasSuffix(query, "}") {
		if query, err = store.formatQuery(keyspace, table, query); err != nil {
			return nil, err
		}
	}

	iter := session.Query(query).Iter()
	defer iter.Close()

	results := []map[string]interface{}{}
	for {
		row := make(map[string]interface{})
		if !iter.MapScan(row) {
			break
		}
		entity, err := store.initEntity(connectionId, keyspace, table, row)
		if err == nil {
			results = append(results, entity)
		}
	}
	return results, nil
}

// ---------- CRUD surface (Store interface) ----------

// Count returns:
// - Exact count when WHERE targets a single partition (all PK cols with equality)
// - Estimated count (from system.size_estimates) when no WHERE
// - Error (or estimate) when WHERE is not partition-key-complete
func (store *ScyllaStore) Count(ctx context.Context, connectionId, keyspace, table, query, options string) (int64, error) {
	session, err := store.getSession(connectionId, keyspace)
	if err != nil {
		return 0, err
	}

	normalize := func(s string) string {
		// strip optional quotes/backticks and lowercase
		s = strings.TrimSpace(s)
		s = strings.Trim(s, "`\"")
		return strings.ToLower(s)
	}
	keyspace = normalize(keyspace)
	table = normalize(table)

	// Get table metadata to know partition key columns
	km, err := session.KeyspaceMetadata(keyspace)
	if err != nil {
		return 0, fmt.Errorf("keyspace metadata error: %v", err)
	}
	if km == nil {
		return 0, fmt.Errorf("keyspace not found: %s", keyspace)
	}

	// Tables map is keyed by lowercase names.
	tm, ok := km.Tables[table]
	if !ok || tm == nil {
		// extra safety: case-insensitive scan if something odd slipped through
		for name, meta := range km.Tables {
			if strings.EqualFold(name, table) && meta != nil {
				tm, ok = meta, true
				break
			}
		}
		if !ok || tm == nil {
			return 0, fmt.Errorf("table not found: %s.%s", keyspace, table)
		}
	}

	pkCols := make([]string, 0, len(tm.PartitionKey))
	for _, col := range tm.PartitionKey {
		pkCols = append(pkCols, strings.ToLower(col.Name))
	}

	// 1) No query -> return an estimate (safe & fast)
	if query == "" || query == "{}" {
		est, err := estimateTableCount(ctx, session, keyspace, table)
		if err != nil {
			slog.Warn("scylla: estimate failed; fallback to scan length", "table", keyspace+"."+table, "err", err)
			entities, ferr := store.find(connectionId, keyspace, table, query)
			if ferr != nil {
				return 0, ferr
			}
			return int64(len(entities)), nil
		}
		return est, nil
	}

	// 2) Query provided
	qt := strings.TrimSpace(query)
	up := strings.ToUpper(qt)
	if strings.HasPrefix(up, "SELECT ") {
		where := extractWhereClause(qt)
		cleanWhere := where
		// remove trailing semicolon
		cleanWhere = strings.TrimSuffix(strings.TrimSpace(cleanWhere), ";")
		// remove ALLOW FILTERING (case-insensitive)
		reAF := regexp.MustCompile(`(?i)\s+ALLOW\s+FILTERING\s*$`)
		cleanWhere = reAF.ReplaceAllString(cleanWhere, "")

		if cleanWhere == "" {
			return estimateTableCount(ctx, session, keyspace, table)
		}
		if whereHasAllPKEquals(cleanWhere, pkCols) {
			cql := fmt.Sprintf("SELECT COUNT(*) FROM %s.%s %s", keyspace, table, cleanWhere)
			var cnt int64
			if err := session.Query(cql).WithContext(ctx).Scan(&cnt); err != nil {
				return 0, err
			}
			return cnt, nil
		}
		slog.Warn("scylla: COUNT would scan multiple partitions; returning estimate",
			"table", keyspace+"."+table, "where", cleanWhere)
		return estimateTableCount(ctx, session, keyspace, table)
	}

	// 3) Legacy/JSON filter path
	slog.Warn("scylla: Count fallback to scan; consider raw CQL with PK WHERE", "table", keyspace+"."+table)
	entities, err := store.find(connectionId, keyspace, table, query)
	if err != nil {
		return 0, err
	}
	return int64(len(entities)), nil
}

// --- helpers -----------------------------------------------------------------

// estimateTableCount uses system.size_estimates to approximate row count.
// It sums partitions_count across token ranges.
func estimateTableCount(ctx context.Context, session *gocql.Session, keyspace, table string) (int64, error) {
	iter := session.Query(
		`SELECT partitions_count FROM system.size_estimates WHERE keyspace_name=? AND table_name=?`,
		keyspace, table,
	).WithContext(ctx).Iter()
	var pcnt, sum int64
	for iter.Scan(&pcnt) {
		sum += pcnt
	}
	if err := iter.Close(); err != nil {
		return 0, err
	}
	return sum, nil
}

// extractWhereClause returns "WHERE ..." (including WHERE) if present.
func extractWhereClause(selectQuery string) string {
	up := strings.ToUpper(selectQuery)
	fromIdx := strings.Index(up, " FROM ")
	if fromIdx < 0 {
		return ""
	}
	// try WHERE
	whereIdx := strings.Index(up[fromIdx:], " WHERE ")
	if whereIdx < 0 {
		return ""
	}
	return selectQuery[fromIdx+whereIdx:] // original case-preserving slice
}

// whereHasAllPKEquals checks that WHERE contains equality conditions for all PK cols.
// This is a simple heuristic; for robust parsing, consider a real CQL parser.
func whereHasAllPKEquals(where string, pkCols []string) bool {
	w := strings.ToLower(where)
	// normalize whitespace
	space := regexp.MustCompile(`\s+`)
	w = space.ReplaceAllString(w, " ")

	for _, col := range pkCols {
		// naive token check: "<col> ="
		if !strings.Contains(w, " "+col+" =") && !strings.HasPrefix(w, "where "+col+" =") {
			return false
		}
	}
	// also ensure there's no IN/>, etc. that could widen range (optional)
	return true
}

// local CQL version (so we don't rely on the sqlite helper)
func generateCqlUpdateTableQuery(tableName string, fields []interface{}, whereClause string) (string, error) {
	if len(fields) == 0 {
		return "", nil
	}
	// If a SELECT was passed in as whereClause, extract its WHERE part.
	w := whereClause
	up := strings.ToUpper(w)
	if strings.HasPrefix(up, "SELECT ") {
		w = extractWhereClause(w)
	}
	w = strings.TrimSpace(w)
	if !strings.HasPrefix(strings.ToUpper(w), "WHERE ") {
		if w != "" {
			w = "WHERE " + w
		}
	}
	setParts := make([]string, 0, len(fields))
	for _, f := range fields {
		setParts = append(setParts, fmt.Sprintf("%s = ?", f.(string)))
	}
	return fmt.Sprintf("UPDATE %s SET %s %s", tableName, strings.Join(setParts, ", "), w), nil
}

func (store *ScyllaStore) backfillArrays(connectionId, keyspace, tableName, id string, data map[string]interface{}) {
	session, err := store.getSession(connectionId, keyspace)
	if err != nil {
		return
	}
	for column, value := range data {
		rv := reflect.ValueOf(value)
		if rv.Kind() != reflect.Slice {
			continue
		}
		field := camelToSnake(column)
		for i := 0; i < rv.Len(); i++ {
			el := rv.Index(i)
			if el.Kind() == reflect.Map {
				continue
			}
			arrayTable := tableName + "_" + field
			createArray := fmt.Sprintf(
				"CREATE TABLE IF NOT EXISTS %s.%s ( %s_id TEXT, value %s, PRIMARY KEY (%s_id, value));",
				keyspace, arrayTable, tableName, deduceColumnType(el.Interface()), tableName,
			)
			if err := session.Query(createArray).Exec(); err != nil {
				slog.Error("scylla: backfill create array table failed", "table", keyspace+"."+arrayTable, "err", err)
				continue
			}
			insArray := fmt.Sprintf("INSERT INTO %s.%s (%s_id, value) VALUES (?, ?);", keyspace, arrayTable, tableName)
			if err := session.Query(insArray, id, el.Interface()).Exec(); err != nil {
				slog.Error("scylla: backfill insert array value failed", "table", keyspace+"."+arrayTable, "err", err)
			}
		}
	}
}

func (store *ScyllaStore) insertData(connectionId, keyspace, tableName string, data map[string]interface{}) (map[string]interface{}, error) {
	session, err := store.getSession(connectionId, keyspace)
	if err != nil {
		return nil, err
	}

	var id string
	if data["id"] != nil {
		id = Utility.ToString(data["id"])
	} else if data["_id"] != nil {
		id = Utility.ToString(data["_id"])
	}
	if id == "" {
		return nil, errors.New("the id is required")
	}

	// Exists?
	checkQ := fmt.Sprintf("SELECT * FROM %s.%s WHERE id = ?", keyspace, tableName)
	if v, err := store.FindOne(context.Background(), connectionId, keyspace, tableName, checkQ, ""); err == nil && v != nil {
		store.backfillArrays(connectionId, keyspace, tableName, id, data)
		return v.(map[string]interface{}), nil
	}

	// Ensure base table exists
	if err := store.createScyllaTable(session, keyspace, tableName, data); err != nil {
		return nil, err
	}

	// Schema migration: add any columns that exist in the data but not yet in the table.
	// This handles renamed columns (e.g. publisher_i_d → publisher_id after camelToSnake fix)
	// without requiring a manual DROP TABLE.
	if existingCols, err := store.getTableColumns(session, keyspace, tableName); err == nil {
		for column, value := range data {
			if value == nil || column == "typeName" {
				continue
			}
			fieldType := deduceColumnType(value)
			if fieldType == "" || fieldType == "array" || fieldType == "map" {
				continue
			}
			col := camelToSnake(column)
			if col == "_id" || col == "id" {
				continue
			}
			if _, exists := existingCols[col]; !exists {
				alterQ := fmt.Sprintf("ALTER TABLE %s.%s ADD %s %s", keyspace, tableName, col, fieldType)
				if err := session.Query(alterQ).Exec(); err != nil {
					slog.Warn("scylla: alter table add column skipped", "table", tableName, "col", col, "err", err)
				}
			}
		}
	}

	columns := make([]string, 0)
	values := make([]interface{}, 0)

	for column, value := range data {
		if value == nil {
			continue
		}
		kind := reflect.TypeOf(value).Kind()

		switch kind {
		case reflect.Slice:
			// Array handling: either entity refs or scalar arrays.
			sv := reflect.ValueOf(value)
			length := sv.Len()
			field := camelToSnake(column)

			for i := 0; i < length; i++ {
				el := sv.Index(i)

				if el.Kind() == reflect.Interface && !el.IsNil() {
					el = el.Elem()
				}

				elKind := el.Kind()
				if elKind == reflect.Chan || elKind == reflect.Func || elKind == reflect.Map ||
					elKind == reflect.Pointer || elKind == reflect.Interface || elKind == reflect.Slice {
					if el.IsNil() {
						continue
					}
				}

				switch el.Kind() {
				case reflect.Map:
					entity, _ := el.Interface().(map[string]interface{})

					// **New behavior**: aggressively extract an id; if present, treat as REF ONLY.
					_tid := extractID(entity)
					if _tid != "" {
						// canonical ref table & direction
						refTable, baseIsFirst := canonicalRefTable(tableName, field)
						createRef := fmt.Sprintf(
							`CREATE TABLE IF NOT EXISTS %s.%s (source_id TEXT, target_id TEXT, PRIMARY KEY (source_id, target_id))`,
							keyspace, refTable,
						)
						if err := session.Query(createRef).Exec(); err != nil {
							slog.Error("scylla: create ref table failed", "table", keyspace+"."+refTable, "err", err)
							break
						}
						src, dst := id, _tid
						if !baseIsFirst {
							src, dst = _tid, id
						}
						insRef := fmt.Sprintf("INSERT INTO %s.%s (source_id, target_id) VALUES (?, ?);", keyspace, refTable)
						if err := session.Query(insRef, src, dst).Exec(); err != nil {
							slog.Error("scylla: insert ref failed", "table", keyspace+"."+refTable, "err", err)
						}
						break
					}

					// No id found; as a safety, do NOT attempt to upsert nested entity without id.
					slog.Warn("scylla: array entity has no identifiable id; skipping upsert and link",
						"table", keyspace+"."+tableName, "field", field)
					break

				default:
					// Scalar arrays only.
					valType := deduceColumnType(el.Interface())
					if valType == "map" || valType == "" {
						slog.Warn("scylla: refusing to create scalar array table with map/empty", "table", keyspace+"."+tableName, "field", field)
						break
					}

					arrayTable := tableName + "_" + field
					createArray := fmt.Sprintf(
						"CREATE TABLE IF NOT EXISTS %s.%s (%s_id TEXT, value %s, PRIMARY KEY (%s_id, value));",
						keyspace, arrayTable, tableName, valType, tableName,
					)
					if err := session.Query(createArray).Exec(); err != nil {
						slog.Error("scylla: create array table failed", "table", keyspace+"."+arrayTable, "err", err)
						break
					}
					insArray := fmt.Sprintf("INSERT INTO %s.%s (%s_id, value) VALUES (?, ?);", keyspace, arrayTable, tableName)
					if err := session.Query(insArray, id, el.Interface()).Exec(); err != nil {
						slog.Error("scylla: insert array value failed", "table", keyspace+"."+arrayTable, "err", err)
					}
				}
			}

		case reflect.Map:
			// Single embedded entity -> treat as reference if we can find an id.
			m := value.(map[string]interface{})

			_tid := extractID(m)
			field := camelToSnake(column)

			if _tid != "" {
				refTable, baseIsFirst := canonicalRefTable(tableName, field)
				createRef := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.%s (source_id TEXT, target_id TEXT, PRIMARY KEY (source_id, target_id))`, keyspace, refTable)
				if err := session.Query(createRef).Exec(); err != nil {
					slog.Error("scylla: create ref table failed", "table", keyspace+"."+refTable, "err", err)
					break
				}
				src, dst := id, _tid
				if !baseIsFirst {
					src, dst = _tid, id
				}
				insRef := fmt.Sprintf("INSERT INTO %s.%s (source_id, target_id) VALUES (?, ?);", keyspace, refTable)
				if err := session.Query(insRef, src, dst).Exec(); err != nil {
					slog.Error("scylla: insert ref failed", "table", keyspace+"."+refTable, "err", err)
				}
				break
			}

			// No id -> skip upsert, warn.
			slog.Warn("scylla: single embedded entity has no identifiable id; skipping upsert and link",
				"table", keyspace+"."+tableName, "field", field)

		default:
			if column != "typeName" {
				col := camelToSnake(column)
				if col == "_id" || col == "id" {
					col = "id"
				}

				// Normalize known numeric columns by column name
				switch col {
				case "expire_at", "last_state_time":
					switch n := value.(type) {
					case float64:
						value = int64(n)
					case json.Number:
						if i, err := n.Int64(); err == nil {
							value = i
						}
					}
				case "state":
					switch n := value.(type) {
					case float64:
						value = int32(n)
					case json.Number:
						if i, err := n.Int64(); err == nil {
							value = int32(i)
						}
					case int:
						value = int32(n)
					case int64:
						value = int32(n)
					}
				}

				columns = append(columns, col)
				values = append(values, value)
			}
		}
	}

	// Build INSERT for scalar columns
	if len(columns) > 0 {
		insertCols := joinStrings(columns, ", ")
		ph := make([]string, len(columns))
		for i := range columns {
			ph[i] = "?"
		}
		query := fmt.Sprintf("INSERT INTO %s.%s (%s) VALUES (%s);", keyspace, tableName, insertCols, joinStrings(ph, ", "))
		if err := session.Query(query, values...).Exec(); err != nil {
			slog.Error("scylla: insert entity failed", "table", keyspace+"."+tableName, "err", err)
			return nil, err
		}
	}
	return data, nil
}

func (store *ScyllaStore) InsertOne(
	ctx context.Context,
	connectionId, keyspace, table string,
	data interface{},
	options string,
) (interface{}, error) {
	var entity map[string]interface{}
	switch v := data.(type) {
	case map[string]interface{}:
		entity = v
	default:
		var err error
		entity, err = Utility.ToMap(v)
		if err != nil {
			return nil, err
		}
	}
	return store.insertData(connectionId, keyspace, table, entity)
}

func (store *ScyllaStore) InsertMany(ctx context.Context, connectionId string, keyspace string, table string, entities []interface{}, options string) ([]interface{}, error) {
	for _, data := range entities {
		entity, err := Utility.ToMap(data)
		if err != nil {
			return nil, err
		}
		if _, err := store.insertData(connectionId, keyspace, table, entity); err != nil {
			return nil, err
		}
	}
	return entities, nil
}

func (store *ScyllaStore) FindOne(ctx context.Context, connectionId string, keyspace string, table string, query string, options string) (interface{}, error) {
	results, err := store.find(connectionId, keyspace, table, query)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, errors.New("no matching document found for query " + query)
	}
	return results[0], nil
}

func (store *ScyllaStore) Find(ctx context.Context, connectionId string, keyspace string, table string, query string, options string) ([]interface{}, error) {
	results, err := store.find(connectionId, keyspace, table, query)
	if err != nil {
		return nil, err
	}
	out := make([]interface{}, len(results))
	for i := range results {
		out[i] = results[i]
	}
	return out, nil
}

// ---------- Delete / Update ----------

func (store *ScyllaStore) deleteSideTables(connectionId, keyspace, baseTable string, id any) {
	session, err := store.getSession(connectionId, keyspace)
	if err != nil {
		slog.Error("scylla: deleteSideTables get session", "err", err)
		return
	}

	base := strings.ToLower(baseTable)
	if !strings.HasSuffix(base, "s") {
		base += "s"
	}

	iter := session.Query(
		"SELECT table_name FROM system_schema.tables WHERE keyspace_name = ?",
		keyspace,
	).Iter()
	defer iter.Close()

	var tname string
	for iter.Scan(&tname) {
		lower := strings.ToLower(tname)
		parts := strings.SplitN(lower, "_", 2)
		if len(parts) != 2 {
			continue
		}

		cols := map[string]struct{}{}
		citer := session.Query(
			"SELECT column_name FROM system_schema.columns WHERE keyspace_name = ? AND table_name = ?",
			keyspace, tname,
		).Iter()
		var col string
		for citer.Scan(&col) {
			cols[strings.ToLower(col)] = struct{}{}
		}
		citer.Close()

		// ref link table?
		if _, ok := cols["source_id"]; ok {
			if _, ok2 := cols["target_id"]; ok2 {
				// skip non-canonical names
				lc, rc := canonicalPair(parts[0], parts[1])
				if lower != lc+"_"+rc {
					continue
				}
				if parts[0] == base {
					q := fmt.Sprintf("DELETE FROM %s.%s WHERE source_id = ?", keyspace, tname)
					if err := session.Query(q, id).Exec(); err != nil {
						slog.Warn("scylla: side delete by source_id failed", "table", tname, "err", err)
					}
					continue
				}
				if parts[1] == base {
					q := fmt.Sprintf("DELETE FROM %s.%s WHERE target_id = ?", keyspace, tname)
					if err := session.Query(q, id).Exec(); err != nil {
						slog.Warn("scylla: side delete by target_id failed", "table", tname, "err", err)
					}
					continue
				}
			}
		}

		// scalar array cleanup (table_<field> with table_id)
		keyCol := base + "_id"
		if _, ok := cols[keyCol]; ok {
			q := fmt.Sprintf("DELETE FROM %s.%s WHERE %s = ?", keyspace, tname, keyCol)
			if err := session.Query(q, id).Exec(); err != nil {
				slog.Warn("scylla: side delete by base_id failed", "table", tname, "pk", keyCol, "err", err)
			}
			continue
		}
	}
}

func (store *ScyllaStore) deleteEntity(connectionId, keyspace, table string, entity map[string]interface{}) error {
	session, err := store.getSession(connectionId, keyspace)
	if err != nil {
		return err
	}

	id := entity["_id"]
	if id == nil {
		id = entity["id"]
	}
	if id == nil {
		return fmt.Errorf("deleteEntity: missing id for %s", table)
	}

	store.deleteSideTables(connectionId, keyspace, table, id)

	if err := session.Query(fmt.Sprintf("DELETE FROM %s.%s WHERE id = ?", keyspace, table), id).Exec(); err != nil {
		slog.Error("scylla: delete entity failed", "table", keyspace+"."+table, "err", err)
		return err
	}
	return nil
}

func (store *ScyllaStore) ReplaceOne(ctx context.Context, connectionId string, keyspace string, table string, query string, value string, options string) error {
	upsert := false
	if len(options) > 0 {
		var opts []map[string]interface{}
		if err := json.Unmarshal([]byte(options), &opts); err == nil {
			if v, ok := opts[0]["upsert"].(bool); ok {
				upsert = v
			}
		}
	}
	data := make(map[string]interface{})
	if err := json.Unmarshal([]byte(value), &data); err != nil {
		return err
	}
	entities, err := store.find(connectionId, keyspace, table, query)
	if err != nil && !upsert {
		return err
	}
	if len(entities) > 0 {
		if err := store.deleteEntity(connectionId, keyspace, table, entities[0]); err != nil {
			return err
		}
	}
	_, err = store.insertData(connectionId, keyspace, table, data)
	return err
}

func (store *ScyllaStore) Update(ctx context.Context, connectionId string, keyspace string, table string, query string, value string, options string) error {
	session, err := store.getSession(connectionId, keyspace)
	if err != nil {
		return err
	}
	values_ := make(map[string]interface{})
	if err := json.Unmarshal([]byte(value), &values_); err != nil {
		return err
	}
	if values_["$set"] == nil {
		return errors.New("no $set operator in Update")
	}
	if query, err = store.formatQuery(keyspace, table, query); err != nil {
		return err
	}
	entities, err := store.find(connectionId, keyspace, table, query)
	if err != nil {
		return err
	}
	if len(entities) == 0 {
		return errors.New("no matching document found for query " + query)
	}
	for _, entity := range entities {
		fields := make([]interface{}, 0)
		vals := make([]interface{}, 0)
		arrayFields := make([]string, 0)

		for k, v := range values_["$set"].(map[string]interface{}) {
			if reflect.TypeOf(v).Kind() == reflect.Slice {
				arrayFields = append(arrayFields, k)
			} else {
				fields = append(fields, camelToSnake(k))
				vals = append(vals, v)
			}
		}

		baseQuery := "SELECT * FROM " + table + " WHERE id = ?"
		vals = append(vals, entity["_id"])

		q, err := generateCqlUpdateTableQuery(table, fields, baseQuery) // now local to this file
		if err != nil {
			return err
		}
		if q != "" {
			if err := session.Query(q, vals...).Exec(); err != nil {
				return err
			}
		}

		for _, field := range arrayFields {
			values := values_["$set"].(map[string]interface{})[field].([]interface{})
			fieldSnake := camelToSnake(field)
			arrayTable, baseIsFirst := canonicalRefTable(table, fieldSnake)

			cols, _ := store.getTableColumns(session, keyspace, arrayTable)
			isRefTable := false
			if _, ok := cols["source_id"]; ok {
				if _, ok2 := cols["target_id"]; ok2 {
					isRefTable = true
				}
			}
			keyCol := table + "_id"

			if isRefTable {
				// delete where the base id sits (source if base is first, else target)
				if baseIsFirst {
					delQ := fmt.Sprintf("DELETE FROM %s.%s WHERE source_id = ?", keyspace, arrayTable)
					_ = session.Query(delQ, entity["_id"]).Exec()
				} else {
					delQ := fmt.Sprintf("DELETE FROM %s.%s WHERE target_id = ?", keyspace, arrayTable)
					_ = session.Query(delQ, entity["_id"]).Exec()
				}
			} else if _, ok := cols[strings.ToLower(keyCol)]; ok {
				delQ := fmt.Sprintf("DELETE FROM %s.%s WHERE %s = ?", keyspace, arrayTable, keyCol)
				_ = session.Query(delQ, entity["_id"]).Exec()
			}

			for _, raw := range values {
				if m, ok := raw.(map[string]interface{}); ok {
					if !isRefTable {
						createRef := fmt.Sprintf(
							`CREATE TABLE IF NOT EXISTS %s.%s (source_id TEXT, target_id TEXT, PRIMARY KEY (source_id, target_id))`,
							keyspace, arrayTable,
						)
						if err := session.Query(createRef).Exec(); err != nil {
							slog.Error("scylla: create ref table failed", "table", keyspace+"."+arrayTable, "err", err)
							continue
						}
						isRefTable = true
					}
					tid := extractID(m)
					if tid == "" {
						slog.Warn("scylla: array ref update skipped (missing target id)", "table", arrayTable, "field", field)
						continue
					}
					var src, dst interface{}
					if baseIsFirst {
						src, dst = entity["_id"], tid
					} else {
						src, dst = tid, entity["_id"]
					}
					ins := fmt.Sprintf("INSERT INTO %s.%s (source_id, target_id) VALUES (?, ?)", keyspace, arrayTable)
					if err := session.Query(ins, src, dst).WithContext(ctx).Exec(); err != nil {
						slog.Error("scylla: insert ref failed", "table", arrayTable, "err", err)
					}
					continue
				}

				// scalar row
				if isRefTable {
					valType := deduceColumnType(raw)
					if valType == "map" || valType == "" {
						slog.Warn("scylla: refusing to insert scalar array map/empty", "table", arrayTable)
						continue
					}
					createArray := fmt.Sprintf(
						"CREATE TABLE IF NOT EXISTS %s.%s (%s_id TEXT, value %s, PRIMARY KEY (%s_id, value));",
						keyspace, arrayTable, table, valType, table,
					)
					if err := session.Query(createArray).Exec(); err != nil {
						slog.Error("scylla: create scalar array table failed", "table", keyspace+"."+arrayTable, "err", err)
						continue
					}
					isRefTable = false
				}

				insQ := fmt.Sprintf("INSERT INTO %s.%s (%s_id, value) VALUES (?, ?)", keyspace, arrayTable, table)
				if err := session.Query(insQ, entity["_id"], raw).WithContext(ctx).Exec(); err != nil {
					slog.Error("scylla: insert scalar array value failed", "table", arrayTable, "err", err)
				}
			}
		}
	}
	return nil
}

func (store *ScyllaStore) UpdateOne(
	ctx context.Context,
	connectionId string,
	keyspace string,
	table string,
	query string,
	value string,
	options string,
) error {
	session, err := store.getSession(connectionId, keyspace)
	if err != nil {
		return err
	}

	values_ := make(map[string]interface{})
	if err := json.Unmarshal([]byte(value), &values_); err != nil {
		return err
	}
	rawSet, ok := values_["$set"]
	if !ok {
		return errors.New("no $set operator in UpdateOne")
	}
	setMap, ok := rawSet.(map[string]interface{})
	if !ok {
		return errors.New("$set must be an object")
	}

	formatted, err := store.formatQuery(keyspace, table, query)
	if err != nil {
		return err
	}

	entities, err := store.find(connectionId, keyspace, table, formatted)
	if err != nil {
		return err
	}
	if len(entities) == 0 {
		return errors.New("no matching document found for query " + query)
	}
	entity := entities[0]
	entityID := entity["_id"]
	if entityID == nil {
		entityID = entity["id"]
	}
	if entityID == nil {
		return fmt.Errorf("UpdateOne: entity missing id in table %s", table)
	}

	scalarSet := make([]string, 0)
	scalarVals := make([]interface{}, 0)
	arrayFields := make([]string, 0)
	arrayValues := make(map[string][]interface{})

	for k, v := range setMap {
		if v == nil {
			scalarSet = append(scalarSet, fmt.Sprintf("%s = ?", camelToSnake(k)))
			scalarVals = append(scalarVals, nil)
			continue
		}
		if arr, ok := v.([]interface{}); ok {
			arrayFields = append(arrayFields, k)
			arrayValues[k] = arr
			continue
		}
		scalarSet = append(scalarSet, fmt.Sprintf("%s = ?", camelToSnake(k)))
		scalarVals = append(scalarVals, v)
	}

	if len(scalarSet) > 0 {
		cql := fmt.Sprintf("UPDATE %s.%s SET %s WHERE id = ?", keyspace, table, strings.Join(scalarSet, ", "))
		args := append([]interface{}{}, scalarVals...)
		args = append(args, entityID)
		if err := session.Query(cql, args...).WithContext(ctx).Exec(); err != nil {
			return err
		}
	}

	for _, field := range arrayFields {
		fieldSnake := camelToSnake(field)
		arrayTable, baseIsFirst := canonicalRefTable(table, fieldSnake)

		cols, _ := store.getTableColumns(session, keyspace, arrayTable)
		isRefTable := false
		if _, ok := cols["source_id"]; ok {
			if _, ok2 := cols["target_id"]; ok2 {
				isRefTable = true
			}
		}
		keyCol := table + "_id"

		if isRefTable {
			if baseIsFirst {
				delQ := fmt.Sprintf("DELETE FROM %s.%s WHERE source_id = ?", keyspace, arrayTable)
				_ = session.Query(delQ, entityID).WithContext(ctx).Exec()
			} else {
				delQ := fmt.Sprintf("DELETE FROM %s.%s WHERE target_id = ?", keyspace, arrayTable)
				_ = session.Query(delQ, entityID).WithContext(ctx).Exec()
			}
		} else if _, ok := cols[strings.ToLower(keyCol)]; ok {
			delQ := fmt.Sprintf("DELETE FROM %s.%s WHERE %s = ?", keyspace, arrayTable, keyCol)
			_ = session.Query(delQ, entityID).WithContext(ctx).Exec()
		}

		for _, raw := range arrayValues[field] {
			if m, ok := raw.(map[string]interface{}); ok {
				if !isRefTable {
					createRef := fmt.Sprintf(
						`CREATE TABLE IF NOT EXISTS %s.%s (source_id TEXT, target_id TEXT, PRIMARY KEY (source_id, target_id))`,
						keyspace, arrayTable,
					)
					if err := session.Query(createRef).Exec(); err != nil {
						slog.Error("scylla: create ref table failed", "table", keyspace+"."+arrayTable, "err", err)
						continue
					}
					isRefTable = true
				}
				tid := extractID(m)
				if tid == "" {
					slog.Warn("scylla: array ref insert skipped (missing target id)", "table", arrayTable, "field", field)
					continue
				}
				var src, dst interface{}
				if baseIsFirst {
					src, dst = entityID, tid
				} else {
					src, dst = tid, entityID
				}
				ins := fmt.Sprintf("INSERT INTO %s.%s (source_id, target_id) VALUES (?, ?)", keyspace, arrayTable)
				if err := session.Query(ins, src, dst).WithContext(ctx).Exec(); err != nil {
					slog.Error("scylla: insert ref failed", "table", arrayTable, "err", err)
				}
				continue
			}

			// scalar
			if isRefTable {
				valType := deduceColumnType(raw)
				if valType == "map" || valType == "" {
					slog.Warn("scylla: refusing to insert scalar array map/empty", "table", arrayTable)
					continue
				}
				createArray := fmt.Sprintf(
					"CREATE TABLE IF NOT EXISTS %s.%s (%s_id TEXT, value %s, PRIMARY KEY (%s_id, value));",
					keyspace, arrayTable, table, valType, table,
				)
				if err := session.Query(createArray).Exec(); err != nil {
					slog.Error("scylla: create scalar array table failed", "table", keyspace+"."+arrayTable, "err", err)
					continue
				}
				isRefTable = false
			}

			insQ := fmt.Sprintf("INSERT INTO %s.%s (%s_id, value) VALUES (?, ?)", keyspace, arrayTable, table)
			if err := session.Query(insQ, entityID, raw).WithContext(ctx).Exec(); err != nil {
				slog.Error("scylla: insert scalar array value failed", "table", arrayTable, "err", err)
			}
		}
	}

	return nil
}

func (store *ScyllaStore) Delete(ctx context.Context, connectionId string, keyspace string, table string, query string, options string) error {
	entities, err := store.find(connectionId, keyspace, table, query)
	if err != nil {
		return err
	}
	for _, e := range entities {
		if err := store.deleteEntity(connectionId, keyspace, table, e); err != nil {
			return err
		}
	}
	return nil
}

func (store *ScyllaStore) DeleteOne(ctx context.Context, connectionId string, keyspace string, table string, query string, options string) error {
	entities, err := store.find(connectionId, keyspace, table, query)
	if err != nil {
		return err
	}
	if len(entities) > 0 {
		if err := store.deleteEntity(connectionId, keyspace, table, entities[0]); err != nil {
			return err
		}
	}
	return nil
}

// ---------- Collections (Tables) & Admin ----------

func (store *ScyllaStore) CreateTable(ctx context.Context, connectionId string, db string, table string, fields []string) error {
	session, err := store.getSession(connectionId, db)
	if err != nil {
		return err
	}
	if _, err := store.createKeyspace(connectionId, db); err != nil {
		return err
	}
	createTable := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s.%s (id TEXT PRIMARY KEY, %s);", db, table, strings.Join(fields, ", "))
	if err := session.Query(createTable).Exec(); err != nil {
		slog.Error("scylla: create table failed", "table", table, "err", err)
		return err
	}
	return nil
}

func (store *ScyllaStore) CreateCollection(ctx context.Context, connectionId string, keyspace string, collection string, options string) error {
	return errors.New("not implemented")
}

func dropTable(session *gocql.Session, keyspace, tableName string) error {
	return session.Query(fmt.Sprintf("DROP TABLE IF EXISTS %s.%s;", keyspace, tableName)).Exec()
}

func (store *ScyllaStore) DeleteCollection(ctx context.Context, connectionId string, keyspace string, collection string) error {
	if keyspace == "" {
		return errors.New("the database is required")
	}
	if collection == "" {
		return errors.New("the collection is required")
	}
	session, err := store.getSession(connectionId, keyspace)
	if err != nil {
		return err
	}
	if err := dropTable(session, keyspace, collection); err != nil {
		slog.Error("scylla: drop table failed", "table", collection, "err", err)
		return err
	}
	return nil
}

func splitCQLScript(script string) []string {
	var statements []string
	var current strings.Builder
	inString := false
	for _, r := range script {
		ch := string(r)
		if ch == "'" {
			inString = !inString
		}
		current.WriteString(ch)
		if ch == ";" && !inString {
			statements = append(statements, strings.TrimSpace(current.String()))
			current.Reset()
		}
	}
	if current.Len() > 0 {
		statements = append(statements, strings.TrimSpace(current.String()))
	}
	return statements
}

func (store *ScyllaStore) RunAdminCmd(ctx context.Context, connectionId, user, password, script string) error {
	store.lock.Lock()
	connection := store.connections[connectionId]
	store.lock.Unlock()
	if connection == nil {
		return errors.New("the connection does not exist")
	}

	// Prefer explicit args, else fall back to connection.Options["username"/"password"]
	opts := map[string]string{}
	for k, v := range connection.Options {
		opts[k] = v
	}
	if user == "" {
		user = opts["username"]
	}
	if password == "" {
		password = opts["password"]
	}
	if user == "" || password == "" {
		return errors.New("RunAdminCmd: no admin username/password available for admin script")
	}

	opts["username"] = user
	opts["password"] = password

	adminCluster := store.buildCluster(connection.Hosts, connection.Port, "system", opts)
	adminSession, err := adminCluster.CreateSession()
	if err != nil {
		return err
	}
	defer adminSession.Close()

	for _, stmt := range splitCQLScript(script) {
		if stmt == "" {
			continue
		}
		if err := adminSession.Query(stmt).Exec(); err != nil {
			slog.Error("scylla: admin script exec failed", "stmt", stmt, "err", err)
			return err
		}
	}
	return nil
}

// ---------- Migration of legacy link tables ----------

// MigrateLinkTables scans for non-canonical "<x>_<y>" link tables (with source_id/target_id),
// moves rows into the canonical, sorted "<min>_<max>" table (swapping source/target when needed),
// and drops the legacy tables.
func (store *ScyllaStore) MigrateLinkTables(connectionId, keyspace string) error {
	session, err := store.getSession(connectionId, keyspace)
	if err != nil {
		return err
	}

	iter := session.Query(
		"SELECT table_name FROM system_schema.tables WHERE keyspace_name = ?",
		keyspace,
	).Iter()
	defer iter.Close()

	var tname string
	for iter.Scan(&tname) {
		lower := strings.ToLower(tname)
		parts := strings.SplitN(lower, "_", 2)
		if len(parts) != 2 {
			continue
		}

		cols, _ := store.getTableColumns(session, keyspace, tname)
		if _, ok := cols["source_id"]; !ok {
			continue
		}
		if _, ok := cols["target_id"]; !ok {
			continue
		}

		left, right := canonicalPair(parts[0], parts[1])
		canonical := left + "_" + right
		if lower == canonical {
			continue // already canonical
		}

		// ensure canonical table exists
		createRef := fmt.Sprintf(
			`CREATE TABLE IF NOT EXISTS %s.%s (source_id TEXT, target_id TEXT, PRIMARY KEY (source_id, target_id))`,
			keyspace, canonical,
		)
		if err := session.Query(createRef).Exec(); err != nil {
			slog.Error("migrate: create canonical failed", "table", canonical, "err", err)
			continue
		}

		// copy rows: since table name is reversed, swap source/target on insert
		q := fmt.Sprintf("SELECT source_id, target_id FROM %s.%s", keyspace, tname)
		it := session.Query(q).Iter()
		row := map[string]interface{}{}
		for it.MapScan(row) {
			src := row["source_id"]
			dst := row["target_id"]
			ins := fmt.Sprintf("INSERT INTO %s.%s (source_id, target_id) VALUES (?, ?)", keyspace, canonical)
			if err := session.Query(ins, dst, src).Exec(); err != nil {
				slog.Warn("migrate: insert into canonical failed", "table", canonical, "err", err)
			}
			for k := range row {
				delete(row, k)
			}
		}
		_ = it.Close()

		// drop legacy table
		if err := dropTable(session, keyspace, tname); err != nil {
			slog.Warn("migrate: drop legacy link table failed", "table", tname, "err", err)
		} else {
			slog.Info("migrate: dropped legacy link table", "table", tname)
		}
	}
	return nil
}

// ---------- Not implemented (explicit) ----------

func (store *ScyllaStore) Aggregate(ctx context.Context, connectionId string, keyspace string, table string, pipeline string, optionsStr string) ([]interface{}, error) {
	return nil, errors.New("not implemented")
}
