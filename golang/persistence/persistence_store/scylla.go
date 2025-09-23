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

// ---------- Helpers ----------

func ucFirst(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

func camelToSnake(input string) string {
	var result bytes.Buffer
	for i, r := range input {
		if i > 0 && unicode.IsUpper(r) {
			result.WriteRune('_')
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
			cfg.ServerName = sni // should match node cert SAN/CN
		}

		cluster.SslOpts = &gocql.SslOptions{
			Config:                 cfg,
			EnableHostVerification: !insecure,
		}

		// Pick TLS port if provided (e.g., 9142). If not, use the given/default port.
		if p := strings.TrimSpace(opts["ssl_port"]); p != "" {
			if sslPort, err := strconv.Atoi(p); err == nil && sslPort > 0 {
				cluster.Port = sslPort
			}
		}
	}

	// ----- Port (set last so TLS block can override with ssl_port) -----
	if cluster.Port == 0 { // not set by TLS override
		if port > 0 {
			cluster.Port = port
		} else {
			cluster.Port = 9042 // default plaintext
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

	// Admin session on "system"
	adminCluster := store.buildCluster(connection.Hosts, connection.Port, "system", connection.Options)
	adminSession, err := adminCluster.CreateSession()
	if err != nil {
		slog.Error("scylla: create admin session failed", "hosts", adminCluster.Hosts, "err", err)
		return nil, err
	}
	defer adminSession.Close()

	// Strategy: default RF=1 (single-node friendly) unless options specify otherwise
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

	// Return a cluster bound to the created/existing keyspace
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
	if host == "" {
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

	// Pass username/password to Scylla auth if provided
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
	// safer defaults on single node
	if _, ok := opts["consistency"]; !ok {
		opts["consistency"] = "ONE"
	}
	if timeout > 0 {
		opts["timeout_ms"] = fmt.Sprintf("%d", timeout)
	}

	// Init store map
	if store.connections == nil {
		store.connections = make(map[string]*ScyllaConnection)
	}

	// If already connected to this keyspace, return
	store.lock.Lock()
	if c, ok := store.connections[id]; ok && c.sessions != nil {
		if _, ok := c.sessions[keyspace]; ok {
			// refresh options in case caller changed RF/etc.
			for k, v := range opts {
				c.Options[k] = v
			}
			store.lock.Unlock()
			return nil
		}
	}
	store.lock.Unlock()

	// Create/remember the connection object
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
		// Update connection info on subsequent calls
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

	// Ensure keyspace exists and open a session
	cluster, err := store.createKeyspace(id, keyspace)
	if err != nil {
		return err
	}
	session, err := cluster.CreateSession()
	if err != nil {
		slog.Error("scylla: create session failed", "keyspace", keyspace, "err", err)
		return err
	}
	connection.sessions[keyspace] = session

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
	for fieldName, value := range data {
		if value == nil {
			continue
		}
		fieldType := deduceColumnType(value)
		if fieldType == "" || fieldType == "array" {
			continue
		}
		createTableQuery += camelToSnake(fieldName) + " " + fieldType + ", "
	}
	// Replace _id by id
	createTableQuery = strings.ReplaceAll(createTableQuery, "_id", "id")
	if !strings.Contains(createTableQuery, "id ") {
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
	// No silent fallback; fail fast (prevents masking config errors)
	return nil, errors.New("connection with id " + connectionId + " does not have a session for keyspace " + keyspace)
}

func (store *ScyllaStore) initArrayEntities(connectionId, keyspace, tableName string, entity map[string]interface{}) error {
	// tableName is "<base>_<field>"
	parts := strings.SplitN(tableName, "_", 2)
	if len(parts) != 2 {
		return nil
	}
	field := snakeToCamel(parts[1])
	if entity[field] != nil {
		return nil
	}
	session, err := store.getSession(connectionId, keyspace)
	if err != nil {
		return err
	}
	q := fmt.Sprintf("SELECT target_id FROM %s.%s WHERE source_id = ?", keyspace, tableName)
	iter := session.Query(q, entity["_id"]).Iter()
	defer iter.Close()

	array := []interface{}{}
	for {
		row := make(map[string]interface{})
		if !iter.MapScan(row) {
			break
		}
		if targetId, ok := row["target_id"]; ok {
			tableName_ := field
			if field == "members" {
				tableName_ = "accounts"
			}
			array = append(array, map[string]interface{}{"$ref": tableName_, "$id": targetId, "$db": keyspace})
		}
	}
	entity[field] = array
	return nil
}

func (store *ScyllaStore) initArrayValues(connectionId, keyspace, tableName string, entity map[string]interface{}) error {
	session, err := store.getSession(connectionId, keyspace)
	if err != nil {
		return err
	}
	// tableName "<base>_<field>" -> fk "<base>_id"
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

// getTableColumns returns a lowercase set of column names for keyspace.table.
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

// initEntity populates computed fields and hydrates array fields from side tables.
// It now detects whether side tables are reference tables (source_id/target_id)
// or scalar-array tables (<base>_id/value) and loads accordingly.
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
	var tName string
	for iter.Scan(&tName) {
		lower := strings.ToLower(tName)
		if !strings.HasPrefix(lower, base+"_") {
			continue
		}

		// Which shape is this side table?
		cols, _ := store.getTableColumns(session, keyspace, tName)
		// entity-ref: source_id + target_id
		if _, ok := cols["source_id"]; ok {
			if _, ok2 := cols["target_id"]; ok2 {
				if err := store.initArrayEntities(connectionId, keyspace, tName, entity); err != nil {
					slog.Debug("initArrayEntities failed; continuing", "table", tName, "err", err)
				}
				continue
			}
		}
		// scalar-array: <base>_id + value
		pkCol := base + "_id"
		if _, ok := cols[pkCol]; ok {
			if _, ok2 := cols["value"]; ok2 {
				if err := store.initArrayValues(connectionId, keyspace, tName, entity); err != nil {
					slog.Debug("initArrayValues failed; continuing", "table", tName, "err", err)
				}
				continue
			}
		}
		// Unknown shape — skip quietly.
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
		// Avoid unconditional ALLOW FILTERING. Let caller/schema decide.
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

func (store *ScyllaStore) Count(ctx context.Context, connectionId string, keyspace string, table string, query string, options string) (int64, error) {
	session, err := store.getSession(connectionId, keyspace)
	if err != nil {
		return 0, err
	}
	if query == "" || query == "{}" {
		q := fmt.Sprintf("SELECT COUNT(*) FROM %s.%s", keyspace, table)
		var cnt int64
		if err := session.Query(q).WithContext(ctx).Scan(&cnt); err != nil {
			return 0, err
		}
		return cnt, nil
	}
	// If query is raw CQL SELECT, try to convert to COUNT(*)
	qt := strings.TrimSpace(strings.ToUpper(query))
	if strings.HasPrefix(qt, "SELECT ") {
		// naive transform: take FROM ... [WHERE ...]
		up := strings.ToUpper(query)
		fromIdx := strings.Index(up, " FROM ")
		var where string
		if fromIdx >= 0 {
			whereIdx := strings.Index(up[fromIdx:], " WHERE ")
			if whereIdx >= 0 {
				where = query[fromIdx+whereIdx:]
			}
		}
		q := fmt.Sprintf("SELECT COUNT(*) FROM %s.%s%s", keyspace, table, where)
		var cnt int64
		if err := session.Query(q).WithContext(ctx).Scan(&cnt); err != nil {
			return 0, err
		}
		return cnt, nil
	}
	// JSON path fallback (expensive)
	slog.Warn("scylla: Count fallback to scan; consider raw CQL", "table", keyspace+"."+table)
	entities, err := store.find(connectionId, keyspace, table, query)
	if err != nil {
		return 0, err
	}
	return int64(len(entities)), nil
}

// backfill arrays when base row already exists (e.g., roles_actions)
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
			// Only scalar arrays here; entity references are handled elsewhere.
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
		// ensure side-tables for scalar arrays exist and are populated
		store.backfillArrays(connectionId, keyspace, tableName, id, data)
		return v.(map[string]interface{}), nil
	}

	// Ensure base table exists
	if err := store.createScyllaTable(session, keyspace, tableName, data); err != nil {
		return nil, err
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

			for i := range length {
				el := sv.Index(i)

				// unwrap interface{} elements so maps inside []interface{} are handled as maps
				if el.Kind() == reflect.Interface && !el.IsNil() {
					el = el.Elem()
				}

				// Safety: only call IsNil on nilable kinds
				elKind := el.Kind()
				if elKind == reflect.Chan || elKind == reflect.Func || elKind == reflect.Map ||
					elKind == reflect.Pointer || elKind == reflect.Interface || elKind == reflect.Slice {
					if el.IsNil() {
						continue
					}
				}

				switch el.Kind() {
				case reflect.Map:
					// entity reference in an array ({"$ref": "...", "$id": "..."} or {"typeName": ...})
					entity, _ := el.Interface().(map[string]interface{})
					if entity["typeName"] == nil && entity["$ref"] == nil {
						// Unsupported: array of arbitrary maps (not refs) → skip
						slog.Warn("scylla: skipping unsupported array-of-map (no $ref/typeName)",
							"table", keyspace+"."+tableName, "field", field)
						break
					}

					typeName := Utility.ToString(entity["typeName"])
					if typeName == "" {
						typeName = Utility.ToString(entity["$ref"])
					}
					if !strings.HasSuffix(typeName, "s") {
						typeName += "s"
					}
					typeName = ucFirst(typeName)

					// Domain hygiene
					if entity["domain"] == nil || entity["domain"] == "localhost" {
						if localDomain, _ := config.GetDomain(); localDomain != "" {
							entity["domain"] = localDomain
						}
					}

					// Insert/ensure target entity if full doc provided
					if entity["typeName"] != nil {
						var err error
						entity, err = store.insertData(connectionId, keyspace, typeName, entity)
						if err != nil {
							slog.Error("scylla: insert nested entity failed", "table", keyspace+"."+typeName, "err", err)
							break
						}
					}

					// Target id
					_tid := Utility.ToString(entity["id"])
					if _tid == "" {
						_tid = Utility.ToString(entity["_id"])
					}
					if _tid == "" {
						_tid = Utility.ToString(entity["$id"])
					}
					if _tid == "" {
						slog.Warn("scylla: missing $id/id/_id for array entity ref",
							"table", keyspace+"."+tableName, "field", field)
						break
					}

					// Reference table: <source>_<field>(source_id, target_id)
					refTable := fmt.Sprintf("%s_%s", tableName, field)
					createRef := fmt.Sprintf(
						`CREATE TABLE IF NOT EXISTS %s.%s (source_id TEXT, target_id TEXT, PRIMARY KEY (source_id, target_id))`,
						keyspace, refTable,
					)
					if err := session.Query(createRef).Exec(); err != nil {
						slog.Error("scylla: create ref table failed", "table", keyspace+"."+refTable, "err", err)
						break
					}
					insRef := fmt.Sprintf("INSERT INTO %s.%s (source_id, target_id) VALUES (?, ?);", keyspace, refTable)
					if err := session.Query(insRef, id, _tid).Exec(); err != nil {
						slog.Error("scylla: insert ref failed", "table", keyspace+"."+refTable, "err", err)
					}

				default:
					// Scalar arrays only. Refuse to create scalar array tables with 'map' type.
					valType := deduceColumnType(el.Interface())
					if valType == "map" {
						slog.Warn("scylla: refusing to create scalar array table with map type; expected entity ref",
							"table", keyspace+"."+tableName, "field", field)
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
			// Embedded single entity => reference table too.
			entity := value.(map[string]interface{})
			if entity["typeName"] != nil || entity["$ref"] != nil {
				typeName := Utility.ToString(entity["typeName"])
				if typeName == "" {
					typeName = Utility.ToString(entity["$ref"])
				}
				if !strings.HasSuffix(typeName, "s") {
					typeName += "s"
				}
				typeName = ucFirst(typeName)
				if entity["typeName"] != nil {
					var err error
					entity, err = store.insertData(connectionId, keyspace, typeName, entity)
					if err != nil {
						slog.Error("scylla: insert nested entity failed", "table", keyspace+"."+typeName, "err", err)
					}
				}
				_tid := Utility.ToString(entity["id"])
				if _tid == "" {
					_tid = Utility.ToString(entity["_id"])
				}
				if _tid == "" {
					_tid = Utility.ToString(entity["$id"])
				}
				field := camelToSnake(column)
				refTable := fmt.Sprintf("%s_%s", tableName, field)
				createRef := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.%s (source_id TEXT, target_id TEXT, PRIMARY KEY (source_id, target_id))`, keyspace, refTable)
				if err := session.Query(createRef).Exec(); err != nil {
					slog.Error("scylla: create ref table failed", "table", keyspace+"."+refTable, "err", err)
				}
				insRef := fmt.Sprintf("INSERT INTO %s.%s (source_id, target_id) VALUES (?, ?);", keyspace, refTable)
				if err := session.Query(insRef, id, _tid).Exec(); err != nil {
					slog.Error("scylla: insert ref failed", "table", keyspace+"."+refTable, "err", err)
				}
			}

		default:
			if column != "typeName" {
				col := camelToSnake(column)

				// Normalize well-known numeric columns by *column name*
				switch col {
				case "expire_at", "last_state_time": // CQL: BIGINT
					switch n := value.(type) {
					case float64:
						value = int64(n)
					case json.Number:
						if i, err := n.Int64(); err == nil {
							value = i
						}
					}
				case "state": // CQL: INT
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

	// Build INSERT for scalar columns (keep original tableName case)
	if len(columns) > 0 {
		insertCols := strings.ReplaceAll(joinStrings(columns, ", "), "_id", "id")
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
		entity = v // keep original int64/int32 types
	default:
		var err error
		entity, err = Utility.ToMap(v) // fallback for structs, etc.
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
		return nil, errors.New("no entity found")
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

// deleteSideTables removes all rows in side tables that reference the given base-row id.
// It is resilient to missing tables/columns and does not assume the entity payload
// contains the array values (actions, owners, members, etc.).
func (store *ScyllaStore) deleteSideTables(connectionId, keyspace, baseTable string, id any) {
	session, err := store.getSession(connectionId, keyspace)
	if err != nil {
		slog.Error("scylla: deleteSideTables get session", "err", err)
		return
	}

	base := strings.ToLower(baseTable)

	// 1) discover candidate side tables: <base>_*
	iter := session.Query(
		"SELECT table_name FROM system_schema.tables WHERE keyspace_name = ?",
		keyspace,
	).Iter()
	defer iter.Close()

	var tname string
	for iter.Scan(&tname) {
		if !strings.HasPrefix(strings.ToLower(tname), base+"_") {
			continue
		}

		// 2) discover columns of that table so we can decide which primary key to use
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

		// Two shapes are supported:
		// (a) entity references: source_id + target_id
		// (b) scalar arrays: <base>_id + value
		deleted := false

		if _, ok := cols["source_id"]; ok {
			q := fmt.Sprintf("DELETE FROM %s.%s WHERE source_id = ?", keyspace, tname)
			if err := session.Query(q, id).Exec(); err != nil {
				slog.Warn("scylla: side delete by source_id failed", "table", tname, "err", err)
			} else {
				deleted = true
			}
		}

		keyCol := base + "_id"
		if _, ok := cols[keyCol]; ok {
			q := fmt.Sprintf("DELETE FROM %s.%s WHERE %s = ?", keyspace, tname, keyCol)
			if err := session.Query(q, id).Exec(); err != nil {
				slog.Warn("scylla: side delete by base_id failed", "table", tname, "pk", keyCol, "err", err)
			} else {
				deleted = true
			}
		}

		if !deleted {
			slog.Debug("scylla: skip table (no matching PK columns)", "table", tname)
		}
	}
}

// deleteEntity removes a base row and *all* its side-table rows (arrays and references).
// It does not rely on array values being present in the provided entity document.
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

	// First, nuke all side tables that reference this id (covers both scalar arrays and refs).
	store.deleteSideTables(connectionId, keyspace, table, id)

	// Then, remove the base row itself.
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
		return errors.New("no entity found")
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

		q, err := generateUpdateTableQuery(table, fields, baseQuery) // external helper
		if err != nil {
			return err
		}
		if err := session.Query(q, vals...).Exec(); err != nil {
			return err
		}

		// Update array fields by re-writing side tables.
		for _, field := range arrayFields {
			values := values_["$set"].(map[string]interface{})[field].([]interface{})
			arrayTable := table + "_" + field

			// delete current values
			if arr, ok := entity[field].([]interface{}); ok {
				for _, v := range arr {
					delQ := fmt.Sprintf("DELETE FROM %s.%s WHERE %s_id = ? AND value = ?", keyspace, arrayTable, table)
					if err := session.Query(delQ, entity["_id"], v).Exec(); err != nil {
						slog.Error("scylla: delete array value failed", "table", arrayTable, "err", err)
					}
				}
			}
			// insert new values
			for _, v := range values {
				insQ := fmt.Sprintf("INSERT INTO %s.%s (%s_id, value) VALUES (?, ?)", keyspace, arrayTable, table)
				if err := session.Query(insQ, entity["_id"], v).Exec(); err != nil {
					slog.Error("scylla: insert array value failed", "table", arrayTable, "err", err)
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
	// Parse payload and validate
	values_ := make(map[string]interface{})
	if err := json.Unmarshal([]byte(value), &values_); err != nil {
		return err
	}
	setMap, ok := values_["$set"].(map[string]interface{})
	if !ok {
		return errors.New("no $set operator in UpdateOne")
	}

	// Turn JSON query into: SELECT * FROM <ks>.<table> WHERE ...
	formatted, err := store.formatQuery(keyspace, table, query)
	if err != nil {
		return err
	}
	up := strings.ToUpper(formatted)
	idx := strings.Index(up, " WHERE ")
	if idx < 0 {
		return errors.New("UpdateOne requires a WHERE clause")
	}
	whereClause := formatted[idx+len(" WHERE "):] // only the WHERE condition (no leading "WHERE ")

	// Split scalar vs array fields
	setParts := make([]string, 0)
	vals := make([]interface{}, 0)
	arrayFields := make([]string, 0)

	for k, v := range setMap {
		if reflect.TypeOf(v).Kind() == reflect.Slice {
			arrayFields = append(arrayFields, k) // handle after updating the base row
			continue
		}
		setParts = append(setParts, fmt.Sprintf("%s = ?", camelToSnake(k)))
		vals = append(vals, v)
	}

	// Exec scalar UPDATE first (if any)
	session, err := store.getSession(connectionId, keyspace)
	if err != nil {
		return err
	}
	if len(setParts) > 0 {
		cql := fmt.Sprintf("UPDATE %s.%s SET %s WHERE %s",
			keyspace, table, strings.Join(setParts, ", "), whereClause)
		if err := session.Query(cql, vals...).Exec(); err != nil {
			return err
		}
	}

	// Read the (single) entity to get its _id and current arrays
	entities, err := store.find(connectionId, keyspace, table, formatted) // formatted is a SELECT
	if err != nil {
		return err
	}
	if len(entities) == 0 {
		return errors.New("no entity found")
	}
	entity := entities[0]

	// Overwrite array side-tables
	for _, field := range arrayFields {
		values := setMap[field].([]interface{})
		arrayTable := table + "_" + camelToSnake(field)

		// delete current values
		if arr, ok := entity[field].([]interface{}); ok {
			for _, v := range arr {
				delQ := fmt.Sprintf("DELETE FROM %s.%s WHERE %s_id = ? AND value = ?",
					keyspace, arrayTable, table)
				if err := session.Query(delQ, entity["_id"], v).Exec(); err != nil {
					slog.Error("scylla: delete array value failed", "table", arrayTable, "err", err)
				}
			}
		}
		// insert new values
		for _, v := range values {
			insQ := fmt.Sprintf("INSERT INTO %s.%s (%s_id, value) VALUES (?, ?)",
				keyspace, arrayTable, table)
			if err := session.Query(insQ, entity["_id"], v).Exec(); err != nil {
				slog.Error("scylla: insert array value failed", "table", arrayTable, "err", err)
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

func (store *ScyllaStore) RunAdminCmd(ctx context.Context, connectionId string, user string, password string, script string) error {

	store.lock.Lock()
	connection := store.connections[connectionId]
	store.lock.Unlock()
	if connection == nil {
		return errors.New("the connection does not exist")
	}
	// Build admin cluster with provided credentials (if any)
	opts := map[string]string{}
	for k, v := range connection.Options {
		opts[k] = v
	}
	if user != "" {
		opts["username"] = user
	}
	if password != "" {
		opts["password"] = password
	}
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

// ---------- Not implemented (explicit) ----------

func (store *ScyllaStore) Aggregate(ctx context.Context, connectionId string, keyspace string, table string, pipeline string, optionsStr string) ([]interface{}, error) {
	return nil, errors.New("not implemented")
}
