package persistence_store

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/globulario/services/golang/authentication/authentication_client"
	"github.com/globulario/services/golang/config"
	Utility "github.com/globulario/utility"
	"github.com/gocql/gocql"
)

// ScyllaConnection represents a connection to a Scylla/Cassandra-compatible database.
type ScyllaConnection struct {
	Id       string
	Host     string
	Token    string
	sessions map[string]*gocql.Session
}

/**
 * The SCYLLA store.
 */
type ScyllaStore struct {
	connections map[string]*ScyllaConnection // live connections keyed by connection id
	lock        sync.Mutex                   // guards connections
}

// GetStoreType returns the constant store type name used by the service.
func (store *ScyllaStore) GetStoreType() string { return "SCYLLA" }

// ucFirst uppercases the first rune of a string (replacement for deprecated strings.Title use-cases).
func ucFirst(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

/**
 * createKeyspace creates (if needed) and returns a configured cluster for the given keyspace.
 * It also ensures a session to "system" to run administrative queries.
 */
func (store *ScyllaStore) createKeyspace(connectionId, keyspace string) (*gocql.ClusterConfig, error) {
	if len(keyspace) == 0 {
		return nil, errors.New("the database is required")
	}
	keyspace = strings.ToLower(strings.ReplaceAll(keyspace, "-", "_"))

	// Get the connection.
	store.lock.Lock()
	connection := store.connections[connectionId]
	store.lock.Unlock()
	if connection == nil {
		return nil, errors.New("the connection does not exist")
	}

	createKeyspaceQuery := `
	CREATE KEYSPACE IF NOT EXISTS ` + keyspace + `
	WITH replication = {
		'class': 'SimpleStrategy',
		'replication_factor': 3
	}`

	// Create an admin session against the system keyspace.
	adminCluster := gocql.NewCluster(connection.Host, "127.0.0.1")
	adminCluster.Keyspace = "system"
	adminCluster.Port = 9042
	adminSession, err := adminCluster.CreateSession()
	if err != nil {
		slog.Error("scylla: create admin session failed", "hosts", adminCluster.Hosts, "err", err)
		return nil, err
	}
	defer adminSession.Close()

	if err := adminSession.Query(createKeyspaceQuery).Exec(); err != nil {
		slog.Error("scylla: create keyspace failed", "keyspace", keyspace, "err", err)
		return nil, err
	}

	// Cluster configured for the created/existing keyspace.
	cluster := gocql.NewCluster(connection.Host, "127.0.0.1")
	cluster.Keyspace = keyspace
	cluster.Consistency = gocql.Quorum
	cluster.Port = 9042
	return cluster, nil
}

/**
 * Connect establishes (or reuses) a session to the target keyspace.
 */
func (store *ScyllaStore) Connect(id string, host string, port int32, user string, password string, keyspace string, timeout int32, options_str string) error {
	if len(id) == 0 {
		return errors.New("the connection id is required")
	}
	if len(host) == 0 {
		return errors.New("the host is required")
	}
	if strings.Contains(host, ":") {
		host = strings.Split(host, ":")[0]
	}
	if len(user) == 0 {
		return errors.New("the user is required")
	}
	if len(password) == 0 {
		return errors.New("the password is required")
	}
	if len(keyspace) == 0 {
		return errors.New("the database is required")
	}

	// Reuse an existing session if already present for this keyspace.
	if store.connections != nil {
		store.lock.Lock()
		if c, ok := store.connections[id]; ok && c.sessions != nil {
			if _, ok := c.sessions[keyspace]; ok {
				store.lock.Unlock()
				return nil
			}
		}
		store.lock.Unlock()
	} else {
		store.connections = make(map[string]*ScyllaConnection)
	}

	// Authenticate through the authentication service (retry up to 5 times).
	authClient, err := authentication_client.NewAuthenticationService_Client(host, "authentication.AuthenticationService")
	if err != nil {
		slog.Error("scylla: create auth client failed", "host", host, "err", err)
		return err
	}
	var token string
	for tries := 5; tries > 0; tries-- {
		token, err = authClient.Authenticate(user, password)
		if err == nil {
			break
		}
		if tries == 1 {
			slog.Error("scylla: authenticate failed", "user", user, "err", err)
			return err
		}
		time.Sleep(time.Second)
	}

	// Create/remember the connection object.
	store.lock.Lock()
	connection := store.connections[id]
	if connection == nil {
		connection = &ScyllaConnection{
			Id:       id,
			Host:     host,
			Token:    token,
			sessions: make(map[string]*gocql.Session),
		}
		store.connections[id] = connection
	}
	store.lock.Unlock()

	// Ensure keyspace exists and open a session.
	cluster, err := store.createKeyspace(id, keyspace)
	if err != nil {
		slog.Error("scylla: create keyspace failed", "keyspace", keyspace, "err", err)
		return err
	}
	session, err := cluster.CreateSession()
	if err != nil {
		slog.Error("scylla: create session failed", "keyspace", keyspace, "err", err)
		return err
	}
	connection.sessions[keyspace] = session

	// Ensure user_data row exists (for non-SA/local_resource).
	count, _ := store.Count(context.Background(), id, keyspace, "user_data", `SELECT * FROM user_data WHERE id='`+user+`'`, "")
	if count == 0 && id != "local_resource" && user != "sa" {
		if _, err := store.InsertOne(context.Background(), id, keyspace, "user_data",
			map[string]interface{}{"id": user, "first_name": "", "last_name": "", "middle_name": "", "profile_picture": "", "email": ""}, "",
		); err != nil {
			return err
		}
	}

	slog.Info("scylla: connected", "id", id, "host", host, "keyspace", keyspace)
	return nil
}

// GetSession returns any available session for a given connection.
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

/**
 * Disconnect closes all sessions for the given connection id.
 */
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

/**
 * Ping runs a simple query to validate connectivity.
 */
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

/**
 * CreateDatabase ensures the keyspace exists.
 */
func (store *ScyllaStore) CreateDatabase(ctx context.Context, connectionId string, keyspace string) error {
	if len(keyspace) == 0 {
		return errors.New("the database is required")
	}
	_, err := store.createKeyspace(connectionId, keyspace)
	return err
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

	// Replace _id by id (Scylla uses id here).
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

func deleteKeyspace(host, keyspace string) error {
	adminCluster := gocql.NewCluster(host, "127.0.0.1")
	adminCluster.Keyspace = "system"
	adminCluster.Port = 9042
	adminSession, err := adminCluster.CreateSession()
	if err != nil {
		return err
	}
	defer adminSession.Close()

	query := fmt.Sprintf("DROP KEYSPACE IF EXISTS %s;", keyspace)
	return adminSession.Query(query).Exec()
}

/**
 * DeleteDatabase drops a keyspace.
 */
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
	if err := deleteKeyspace(connection.Host, keyspace); err != nil {
		slog.Error("scylla: drop keyspace failed", "keyspace", keyspace, "err", err)
		return err
	}
	return nil
}

/**
 * Count executes a query (or a full table scan if query is empty) and returns the number of rows.
 */
func (store *ScyllaStore) Count(ctx context.Context, connectionId string, keyspace string, table string, query string, options string) (int64, error) {
	if len(query) == 0 || query == "{}" {
		query = fmt.Sprintf("SELECT * FROM %s.%s", keyspace, table)
	}
	entities, err := store.find(connectionId, keyspace, table, query)
	if err != nil {
		return 0, err
	}
	return int64(len(entities)), nil
}

func (store *ScyllaStore) getSession(connectionId, keyspace string) (*gocql.Session, error) {
	if len(keyspace) == 0 {
		return nil, errors.New("the database is required")
	}
	if len(connectionId) == 0 {
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
	// Fallback for local_resource: return any session.
	if connectionId == "local_resource" {
		for _, session := range connection.sessions {
			return session, nil
		}
	}
	return nil, errors.New("connection with id " + connectionId + " does not have a session for keyspace " + keyspace)
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
	if len(id) == 0 {
		return nil, errors.New("the id is required")
	}

	// Exists?
	query := fmt.Sprintf("SELECT * FROM %s.%s WHERE id='%s'", keyspace, tableName, id)
	if data["domain"] != nil {
		query += fmt.Sprintf(" AND domain='%s'", data["domain"])
	}
	if values_, err := store.FindOne(context.Background(), connectionId, keyspace, tableName, query, ""); err == nil {
		return values_.(map[string]interface{}), nil
	}

	// Ensure table exists with appropriate columns.
	if err := store.createScyllaTable(session, keyspace, tableName, data); err != nil {
		return nil, err
	}

	columns := make([]string, 0)
	values := make([]interface{}, 0)

	for column, value := range data {
		if value == nil {
			continue
		}
		goType := reflect.TypeOf(value)

		if goType.Kind() == reflect.Slice {
			// Array handling: either entity refs or scalar arrays.
			sliceValue := reflect.ValueOf(value)
			length := sliceValue.Len()
			field := camelToSnake(column)

			for i := 0; i < length; i++ {
				element := sliceValue.Index(i)
				valueType := reflect.TypeOf(element.Interface())

				if valueType.Kind() == reflect.Map {
					entity := element.Interface().(map[string]interface{})

					if entity["typeName"] != nil || entity["$ref"] != nil {
						// Determine target type and id.
						typeName := Utility.ToString(entity["typeName"])
						if typeName == "" {
							typeName = Utility.ToString(entity["$ref"])
						}
						if !strings.HasSuffix(typeName, "s") {
							typeName += "s"
						}
						typeName = ucFirst(typeName)

						// Domain hygiene.
						if entity["domain"] == nil {
							if localDomain, _ := config.GetDomain(); localDomain != "" {
								entity["domain"] = localDomain
							}
						} else if entity["domain"] == "localhost" {
							if localDomain, _ := config.GetDomain(); localDomain != "" {
								entity["domain"] = localDomain
							}
						}

						// Insert/ensure target entity if full doc provided.
						if entity["typeName"] != nil {
							var err error
							entity, err = store.insertData(connectionId, keyspace, typeName, entity)
							if err != nil {
								slog.Error("scylla: insert nested entity failed", "table", keyspace+"."+typeName, "err", err)
								continue
							}
						}

						// Target id from either persisted doc or $id.
						_id := Utility.ToString(entity["id"])
						if _id == "" {
							_id = Utility.ToString(entity["_id"])
						}
						if _id == "" {
							_id = Utility.ToString(entity["$id"])
						}
						if _id == "" {
							return nil, errors.New("the entity does not have an id")
						}

						// Reference table: <source>_<field>(source_id, target_id)
						sourceCollection := tableName
						createTable := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.%s_%s (source_id TEXT, target_id TEXT, PRIMARY KEY (source_id, target_id))`, keyspace, sourceCollection, field)
						if err := session.Query(createTable).Exec(); err != nil {
							slog.Error("scylla: create ref table failed", "table", sourceCollection+"_"+field, "err", err)
						}

						insertSQL := fmt.Sprintf("INSERT INTO %s.%s_%s (source_id, target_id) VALUES (?, ?);", keyspace, sourceCollection, field)
						if err := session.Query(insertSQL, id, _id).Exec(); err != nil {
							slog.Error("scylla: insert ref failed", "table", sourceCollection+"_"+field, "err", err)
						}
					}
				} else if element.IsValid() && !element.IsNil() {
					// Scalar array -> create <table>_<field>(value <type>, <table>_id text, PRIMARY KEY ((value, <table>_id)))
					arrayTable := tableName + "_" + field
					createTable := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s.%s (value %s, %s_id TEXT, PRIMARY KEY ((value, %s_id)));",
						keyspace, arrayTable, deduceColumnType(element.Interface()), tableName, tableName)
					if err := session.Query(createTable).Exec(); err != nil {
						slog.Error("scylla: create array table failed", "table", keyspace+"."+arrayTable, "err", err)
					}
					insertSQL := fmt.Sprintf("INSERT INTO %s.%s (value, %s_id) VALUES (?, ?);", keyspace, arrayTable, tableName)
					if err := session.Query(insertSQL, element.Interface(), id).Exec(); err != nil {
						slog.Error("scylla: insert array value failed", "table", keyspace+"."+arrayTable, "err", err)
					}
				}
			}

		} else if goType.Kind() == reflect.Map {
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

				_id := Utility.ToString(entity["id"])
				if _id == "" {
					_id = Utility.ToString(entity["_id"])
				}
				if _id == "" {
					_id = Utility.ToString(entity["$id"])
				}

				sourceCollection := tableName
				field := camelToSnake(column)
				createTable := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.%s_%s (source_id TEXT, target_id TEXT, PRIMARY KEY (source_id, target_id))`, keyspace, sourceCollection, field)
				if err := session.Query(createTable).Exec(); err == nil {
					slog.Info("scylla: ref table ensured", "table", sourceCollection+"_"+field)
				}
				insertSQL := fmt.Sprintf("INSERT INTO %s.%s_%s (source_id, target_id) VALUES (?, ?);", keyspace, sourceCollection, field)
				if err := session.Query(insertSQL, id, _id).Exec(); err != nil {
					slog.Error("scylla: insert ref failed", "table", sourceCollection+"_"+field, "err", err)
				}
			}
		} else if column != "typeName" {
			columns = append(columns, camelToSnake(column))
			values = append(values, value)
		}
	}

	// Make table name initial uppercase (previous code used strings.Title).
	tableName = ucFirst(tableName)

	// Build and run INSERT for scalar columns.
	query = fmt.Sprintf("INSERT INTO %s.%s (%s) VALUES (", keyspace, tableName, joinStrings(columns, ", "))
	placeholders := make([]string, len(columns))
	for i := range columns {
		placeholders[i] = "?"
	}
	query += joinStrings(placeholders, ", ") + ");"
	query = strings.ReplaceAll(query, "_id", "id")

	if err := session.Query(query, values...).Exec(); err != nil {
		slog.Error("scylla: insert entity failed", "table", keyspace+"."+tableName, "err", err)
		return nil, err
	}
	return data, nil
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

/**
 * InsertOne inserts a single entity (creating tables/references as needed) and returns it.
 */
func (store *ScyllaStore) InsertOne(ctx context.Context, connectionId string, keyspace string, table string, data interface{}, options string) (interface{}, error) {
	entity, err := Utility.ToMap(data)
	if err != nil {
		return nil, err
	}
	return store.insertData(connectionId, keyspace, table, entity)
}

/**
 * InsertMany inserts multiple entities.
 */
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

func (store *ScyllaStore) initArrayEntities(connectionId, keyspace, tableName string, entity map[string]interface{}) error {
	field := strings.ReplaceAll(tableName, strings.Split(tableName, "_")[0]+"_", "")
	field = snakeToCamel(field)

	if entity[field] != nil {
		return nil
	}

	session, err := store.getSession(connectionId, keyspace)
	if err != nil {
		return err
	}

	query := fmt.Sprintf("SELECT target_id FROM %s.%s WHERE source_id = ? ALLOW FILTERING;", keyspace, tableName)
	iter := session.Query(query, entity["_id"]).Iter()
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
	if len(array) == 0 {
		return errors.New("no entities found")
	}
	return nil
}

/**
 * initArrayValues initializes scalar array fields from their side tables.
 */
func (store *ScyllaStore) initArrayValues(connectionId, keyspace, tableName string, entity map[string]interface{}) error {
	session, err := store.getSession(connectionId, keyspace)
	if err != nil {
		return err
	}

	query := fmt.Sprintf("SELECT value FROM %s.%s WHERE %s_id = ? ALLOW FILTERING;", keyspace, tableName, entity["typeName"])
	iter := session.Query(query, entity["_id"]).Iter()
	defer iter.Close()

	array := []interface{}{}
	for {
		row := make(map[string]interface{})
		if !iter.MapScan(row) {
			break
		}
		if value, ok := row["value"]; ok {
			array = append(array, value)
		}
	}

	field := strings.ReplaceAll(tableName, strings.Split(tableName, "_")[0]+"_", "")
	field = snakeToCamel(field)
	entity[field] = array
	return nil
}

func (store *ScyllaStore) initEntity(connectionId, keyspace, typeName string, entity map[string]interface{}) (map[string]interface{}, error) {
	if len(typeName) == 0 {
		return nil, errors.New("the type name is required")
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
		return nil, errors.New("the _id is required")
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

	// Tables in keyspace for initializing arrays.
	iter := session.Query(fmt.Sprintf("SELECT table_name FROM system_schema.tables WHERE keyspace_name = '%s'", keyspace)).Iter()
	defer iter.Close()

	var tableName string
	for iter.Scan(&tableName) {
		if strings.HasPrefix(strings.ToLower(tableName), strings.ToLower(typeName)+"_") {
			// entities first, then scalar fallback
			if err := store.initArrayEntities(connectionId, keyspace, tableName, entity); err != nil {
				_ = store.initArrayValues(connectionId, keyspace, tableName, entity)
			}
		}
	}
	return entity, nil
}

/**
 * find runs a SELECT (raw or JSON-based) and returns fully-initialized entities.
 */
func (store *ScyllaStore) find(connectionId, keyspace, table, query string) ([]map[string]interface{}, error) {
	if len(keyspace) == 0 {
		return nil, errors.New("the database is required")
	}
	if len(connectionId) == 0 {
		return nil, errors.New("the connection id is required")
	}
	if len(table) == 0 {
		return nil, errors.New("the table is required")
	}

	session, err := store.getSession(connectionId, keyspace)
	if err != nil {
		return nil, err
	}

	if len(query) == 0 {
		return nil, errors.New("query is empty")
	}
	if strings.HasPrefix(query, "{") && strings.HasSuffix(query, "}") {
		if query, err = store.formatQuery(keyspace, table, query); err != nil {
			return nil, err
		}
		query += " ALLOW FILTERING"
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

/**
 * FindOne returns a single entity matching the query.
 */
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

/**
 * Find returns all entities matching the query.
 */
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

func (store *ScyllaStore) deleteEntity(connectionId string, keyspace string, table string, entity map[string]interface{}) error {
	session, err := store.getSession(connectionId, keyspace)
	if err != nil {
		return err
	}

	// Delete entity row.
	if err := session.Query(fmt.Sprintf("DELETE FROM %s.%s WHERE id = ?", keyspace, table), entity["_id"]).Exec(); err != nil {
		slog.Error("scylla: delete entity failed", "table", keyspace+"."+table, "err", err)
		return err
	}

	// Delete references.
	for column, value := range entity {
		if reflect.TypeOf(value).Kind() != reflect.Slice {
			continue
		}

		sliceValue := reflect.ValueOf(value)
		field := camelToSnake(column)
		for i := 0; i < sliceValue.Len(); i++ {
			element := sliceValue.Index(i)
			valueType := reflect.TypeOf(element.Interface())

			if valueType.Kind() == reflect.Map {
				entity_ := element.Interface().(map[string]interface{})
				var tid interface{}
				if entity_["id"] != nil {
					tid = entity_["id"]
				} else if entity_["$id"] != nil {
					tid = entity_["$id"]
				}
				if tid != nil {
					q := fmt.Sprintf("DELETE FROM %s.%s_%s WHERE source_id = ? AND target_id = ?", keyspace, table, field)
					if err := session.Query(q, entity["_id"], tid).Exec(); err == nil {
						slog.Info("scylla: ref deleted", "table", table+"_"+field)
					}
				}
			} else {
				q := fmt.Sprintf("DELETE FROM %s.%s_%s WHERE %s_id = ? AND value = ?", keyspace, table, field, table)
				if err := session.Query(q, entity["_id"], element.Interface()).Exec(); err != nil {
					slog.Error("scylla: delete array value failed", "table", table+"_"+field, "err", err)
				}
			}
		}
	}
	return nil
}

/**
 * ReplaceOne replaces the first entity matching query with value (JSON).
 * If options contains {"upsert":true}, inserts when not found.
 */
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
		slog.Error("scylla: replace find failed", "err", err)
		return err
	}

	if len(entities) > 0 {
		if err := store.deleteEntity(connectionId, keyspace, table, entities[0]); err != nil {
			slog.Error("scylla: replace delete failed", "err", err)
			return err
		}
	}

	_, err = store.insertData(connectionId, keyspace, table, data)
	if err != nil {
		slog.Error("scylla: replace insert failed", "err", err)
	}
	return err
}

/**
 * Update modifies fields using a {$set:{...}} document for all entities matching query.
 */
func (store *ScyllaStore) Update(ctx context.Context, connectionId string, keyspace string, table string, query string, value string, options string) error {
	session, err := store.getSession(connectionId, keyspace)
	if err != nil {
		return err
	}

	values_ := make(map[string]interface{})
	if err := json.Unmarshal([]byte(value), &values_); err != nil {
		slog.Error("scylla: update unmarshal failed", "err", err)
		return err
	}
	if values_["$set"] == nil {
		return errors.New("no $set operator in Update")
	}

	if query, err = store.formatQuery(keyspace, table, query); err != nil {
		return err
	}
	query += " ALLOW FILTERING"

	entities, err := store.find(connectionId, keyspace, table, query)
	if err != nil {
		slog.Error("scylla: update find failed", "err", err)
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

		q, err := generateUpdateTableQuery(table, fields, baseQuery) // preserved external helper
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

			// Delete current values
			for _, v := range entity[field].([]interface{}) {
				delQ := fmt.Sprintf("DELETE FROM %s.%s WHERE %s_id = ? AND value = ?", keyspace, arrayTable, table)
				if err := session.Query(delQ, entity["_id"], v).Exec(); err != nil {
					slog.Error("scylla: delete array value failed", "table", arrayTable, "err", err)
				}
			}
			// Insert new values
			for _, v := range values {
				insQ := fmt.Sprintf("INSERT INTO %s.%s (value, %s_id) VALUES (?, ?)", keyspace, arrayTable, table)
				if err := session.Query(insQ, v, entity["_id"]).Exec(); err != nil {
					slog.Error("scylla: insert array value failed", "table", arrayTable, "err", err)
				}
			}
		}
	}
	return nil
}

/**
 * UpdateOne modifies fields using a {$set:{...}} document for the first entity matching query.
 */
func (store *ScyllaStore) UpdateOne(ctx context.Context, connectionId string, keyspace string, table string, query string, value string, options string) error {
	values_ := make(map[string]interface{})
	if err := json.Unmarshal([]byte(value), &values_); err != nil {
		slog.Error("scylla: updateOne unmarshal failed", "err", err)
		return err
	}
	if values_["$set"] == nil {
		return errors.New("no $set operator in UpdateOne")
	}

	var err error
	if query, err = store.formatQuery(keyspace, table, query); err != nil {
		return err
	}

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

	q, err := generateUpdateTableQuery(keyspace+"."+table, fields, query) // preserved external helper
	if err != nil {
		return err
	}

	session, err := store.getSession(connectionId, keyspace)
	if err != nil {
		return err
	}
	if err := session.Query(q, vals...).Exec(); err != nil {
		slog.Error("scylla: updateOne exec failed", "q", q, "err", err)
	}

	entities, err := store.find(connectionId, keyspace, table, query)
	if err != nil {
		return err
	}
	if len(entities) == 0 {
		return errors.New("no entity found")
	}
	entity := entities[0]

	for _, field := range arrayFields {
		values := values_["$set"].(map[string]interface{})[field].([]interface{})
		arrayTable := table + "_" + field

		for _, v := range entity[field].([]interface{}) {
			delQ := fmt.Sprintf("DELETE FROM %s.%s WHERE %s_id = ? AND value = ?", keyspace, arrayTable, table)
			if err := session.Query(delQ, entity["_id"], v).Exec(); err != nil {
				slog.Error("scylla: delete array value failed", "table", arrayTable, "err", err)
			}
		}
		for _, v := range values {
			insQ := fmt.Sprintf("INSERT INTO %s.%s (value, %s_id) VALUES (?, ?)", keyspace, arrayTable, table)
			if err := session.Query(insQ, v, entity["_id"]).Exec(); err != nil {
				slog.Error("scylla: insert array value failed", "table", arrayTable, "err", err)
			}
		}
	}
	return nil
}

/**
 * Delete removes all entities that match the query (and their references).
 */
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

/**
 * DeleteOne removes the first entity that matches the query (and its references).
 */
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

/**
 * Aggregate is not implemented for Scylla store.
 */
func (store *ScyllaStore) Aggregate(ctx context.Context, connectionId string, keyspace string, table string, pipeline string, optionsStr string) ([]interface{}, error) {
	return nil, errors.New("not implemented")
}

/**
 * CreateTable creates a table with a given set of extra fields (id TEXT is implicit).
 */
func (store *ScyllaStore) CreateTable(ctx context.Context, connectionId string, db string, table string, fields []string) error {
	session, err := store.getSession(connectionId, db)
	if err != nil {
		return err
	}
	if _, err := store.createKeyspace(connectionId, db); err != nil {
		return err
	}
	createTable := fmt.Sprintf("CREATE TABLE IF NOT EXISTS \"%s\" (id TEXT PRIMARY KEY, %s);", table, strings.Join(fields, ", "))
	if err := session.Query(createTable).Exec(); err != nil {
		slog.Error("scylla: create table failed", "table", table, "err", err)
		return err
	}
	return nil
}

// CreateCollection is not used; collections (tables) are created on first insert or via CreateTable.
func (store *ScyllaStore) CreateCollection(ctx context.Context, connectionId string, keyspace string, collection string, options string) error {
	return errors.New("not implemented")
}

func dropTable(session *gocql.Session, keyspace, tableName string) error {
	return session.Query(fmt.Sprintf("DROP TABLE IF EXISTS %s.%s;", keyspace, tableName)).Exec()
}

/**
 * DeleteCollection drops a table in the provided keyspace.
 */
func (store *ScyllaStore) DeleteCollection(ctx context.Context, connectionId string, keyspace string, collection string) error {
	if len(keyspace) == 0 {
		return errors.New("the database is required")
	}
	if len(collection) == 0 {
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

/**
 * RunAdminCmd authenticates the user-password pair and executes CQL script statements as admin.
 */
func (store *ScyllaStore) RunAdminCmd(ctx context.Context, connectionId string, user string, password string, script string) error {
	store.lock.Lock()
	connection := store.connections[connectionId]
	store.lock.Unlock()
	if connection == nil {
		return errors.New("the connection does not exist")
	}
	host := connection.Host

	authClient, err := authentication_client.NewAuthenticationService_Client(host, "authentication.AuthenticationService")
	if err != nil {
		return err
	}
	for tries := 5; tries > 0; tries-- {
		if _, err = authClient.Authenticate(user, password); err == nil {
			break
		}
		if tries == 1 {
			slog.Error("scylla: admin auth failed", "user", user, "err", err)
			return err
		}
		time.Sleep(time.Second)
	}

	adminCluster := gocql.NewCluster(connection.Host, "127.0.0.1")
	adminCluster.Keyspace = "system"
	adminCluster.Port = 9042
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
