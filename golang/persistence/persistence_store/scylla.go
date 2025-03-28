package persistence_store

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/authentication/authentication_client"
	"github.com/globulario/services/golang/config"
	"github.com/gocql/gocql"
)

// Connection represents a connection to a SQL database.
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
	/** the connections... */
	connections map[string]*ScyllaConnection

	/** the lock. */
	lock sync.Mutex
}

func (store *ScyllaStore) GetStoreType() string {
	return "SCYLLA"
}

/**
 * Create a new SCYLLA store.
 */
func (store *ScyllaStore) createKeyspace(connectionId, keyspace string) (*gocql.ClusterConfig, error) {
	if len(keyspace) == 0 {
		return nil, errors.New("the database is required")
	}

	keyspace = strings.ToLower(keyspace)
	keyspace = strings.ReplaceAll(keyspace, "-", "_")

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

	// Create the admin session.
	adminCluster := gocql.NewCluster(connection.Host)
	adminCluster.Keyspace = "system" // Use the 'system' keyspace for administrative tasks
	adminCluster.Consistency = gocql.Quorum
	adminSession, err := adminCluster.CreateSession()
	if err != nil {
		return nil, err
	}
	defer adminSession.Close()

	// Create the keyspace.
	if err := adminSession.Query(createKeyspaceQuery).Exec(); err != nil {
		return nil, err
	}

	// The cluster address...
	cluster := gocql.NewCluster(connection.Host)
	cluster.Keyspace = keyspace
	cluster.Consistency = gocql.Quorum
	cluster.Hosts = []string{connection.Host}
	cluster.Port = 9042

	return cluster, nil
}

/**
 * Connect to the database.
 */
func (store *ScyllaStore) Connect(id string, host string, port int32, user string, password string, keyspace string, timeout int32, options_str string) error {
	if len(id) == 0 {
		return errors.New("the connection id is required")
	}

	if store.connections != nil {
		store.lock.Lock()
		if _, ok := store.connections[id]; ok {
			if store.connections[id].sessions != nil {
				if _, ok := store.connections[id].sessions[keyspace]; ok {
					store.lock.Unlock()
					return nil
				}
			}
		}
		store.lock.Unlock()
	} else {
		store.connections = make(map[string]*ScyllaConnection)
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

	if len(keyspace) == 0 {
		return errors.New("the database is required")
	}

	// Authenticate the user and password.
	authentication_client, err := authentication_client.NewAuthenticationService_Client(host, "authentication.AuthenticationService")
	if err != nil {
		return err
	}

	// Authenticate the user, try 5 times.
	nbTry := 5
	var token string
	for nbTry > 0 {
		var err error
		token, err = authentication_client.Authenticate(user, password)
		if err != nil {
			if nbTry == 0 {
				return err
			}
			nbTry--
			time.Sleep(1 * time.Second)
		} else if err == nil {
			break
		}
	}

	var connection *ScyllaConnection
	if _, ok := store.connections[id]; ok {
		connection = store.connections[id]
	} else {
		// Save the connection.
		connection = &ScyllaConnection{
			Id:       id,
			Host:     host,
			Token:    token,
			sessions: make(map[string]*gocql.Session),
		}
	}

	// Save the connection before creating the cluster.
	store.lock.Lock()
	store.connections[id] = connection
	store.lock.Unlock()

	// Create the cluster.
	cluster, err := store.createKeyspace(id, keyspace)
	if err != nil {
		return err
	}

	session, err := cluster.CreateSession()
	if err != nil {
		return err
	}

	// Save the session for that keyspace.
	connection.sessions[keyspace] = session

	// Create the table if it does not exist.
	count, _ := store.Count(context.Background(), id, "", "user_data", `SELECT * FROM user_data WHERE id='`+user+`'`, "")
	if count == 0 && id != "local_resource" && user != "sa" {
		_, err := store.InsertOne(context.Background(), id, keyspace, "user_data", map[string]interface{}{"id": user, "first_name": "", "last_name": "", "middle_name": "", "profile_picture": "", "email": ""}, "")
		if err != nil {
			return err
		}
	}

	return nil
}

func (store *ScyllaStore) GetSession(connectionId string) *gocql.Session {
	// Get the connection.
	store.lock.Lock()
	connection := store.connections[connectionId]
	store.lock.Unlock()

	if connection == nil {
		return nil
	}

	// Get the first found session.
	for _, session := range connection.sessions {
		return session
	}

	return nil
}

/**
 * Disconnect from the database.
 */
func (store *ScyllaStore) Disconnect(connectionId string) error {
	// Close all sessions for that connection.
	if store.connections != nil {
		if _, ok := store.connections[connectionId]; ok {
			store.lock.Lock()
			if store.connections[connectionId].sessions != nil {
				for _, session := range store.connections[connectionId].sessions {
					session.Close()
				}
			}
			store.lock.Unlock()
		}
	}
	return nil
}

/**
 * Ping the database.
 */
func (store *ScyllaStore) Ping(ctx context.Context, connectionId string) error {
	// Get the connection.
	store.lock.Lock()
	connection := store.connections[connectionId]
	store.lock.Unlock()
	if connection == nil {
		return errors.New("the connection does not exist")
	}

	// Get the first found session.
	var session *gocql.Session
	for _, s := range connection.sessions {
		session = s
		break
	}

	if session == nil {
		return errors.New("the session does not exist")
	}

	// Execute a simple query to check connectivity
	if err := session.Query("SELECT release_version FROM system.local").Exec(); err != nil {
		return err
	}

	return nil
}

/**
 * Create a new database (keyspace)
 */
func (store *ScyllaStore) CreateDatabase(ctx context.Context, connectionId string, keyspace string) error {
	if len(keyspace) == 0 {
		return errors.New("the database is required")
	}

	_, err := store.createKeyspace(connectionId, keyspace)
	if err != nil {
		return err
	}

	return nil
}

func camelToSnake(input string) string {
	var result bytes.Buffer
	for i, runeValue := range input {
		if i > 0 && unicode.IsUpper(runeValue) {
			result.WriteRune('_')
		}
		result.WriteRune(unicode.ToLower(runeValue))
	}
	return result.String()
}

func snakeToCamel(input string) string {
	var result bytes.Buffer
	upper := false
	for _, runeValue := range input {
		if runeValue == '_' {
			upper = true
		} else {
			if upper {
				result.WriteRune(unicode.ToUpper(runeValue))
				upper = false
			} else {
				result.WriteRune(runeValue)
			}
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
		return "list<text>"
	case reflect.Map:
		return "map<text, text>"
	default:
		return ""
	}
}

func (store *ScyllaStore) createScyllaTable(session *gocql.Session, keyspace, tableName string, data map[string]interface{}) error {
	if data["_id"] == nil && data["id"] == nil {
		return errors.New("the _id is required")
	}

	// Prepare the CREATE TABLE query
	createTableQuery := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s.%s (", keyspace, tableName)

	// Iterate through the map's fields and deduce the data types
	for fieldName, value := range data {
		if value != nil {
			fieldType := deduceColumnType(value)
			if fieldType != "" {
				if fieldType != "list<text>" && fieldType != "map<text, text>" {
					fieldName = camelToSnake(fieldName)
					createTableQuery += fieldName + " " + fieldType + ", "
				}
			}
		}
	}

	// Scylla does not support _id, so replace it with id.
	createTableQuery = strings.ReplaceAll(createTableQuery, "_id", "id")

	if !strings.Contains(createTableQuery, "id") {
		createTableQuery += "id text, "
	}

	// Add the primary key
	createTableQuery += "PRIMARY KEY (id));"

	// Execute the CREATE TABLE query
	err := session.Query(createTableQuery).Exec()
	if err != nil {
		return err
	}

	return nil
}

func deleteKeyspace(host, keyspace string) error {
	// Create the admin session.
	adminCluster := gocql.NewCluster(host)
	adminCluster.Keyspace = "system" // Use the 'system' keyspace for administrative tasks
	adminSession, err := adminCluster.CreateSession()
	if err != nil {
		return err
	}
	defer adminSession.Close()

	query := fmt.Sprintf("DROP KEYSPACE IF EXISTS %s;", keyspace)
	return adminSession.Query(query).Exec()
}

func (store *ScyllaStore) DeleteDatabase(ctx context.Context, connectionId string, keyspace string) error {
	if len(keyspace) == 0 {
		return errors.New("the database is required")
	}

	// Get the session for that keyspace.
	store.lock.Lock()
	connection := store.connections[connectionId]
	store.lock.Unlock()

	if connection == nil {
		return errors.New("the connection does not exist")
	}

	// Drop the keyspace.
	if err := deleteKeyspace(connection.Host, keyspace); err != nil {
		return err
	}

	return nil
}

func (store *ScyllaStore) Count(ctx context.Context, connectionId string, keyspace string, table string, query string, options string) (int64, error) {
	if len(query) == 0 || query == "{}" {
		query = fmt.Sprintf("SELECT COUNT(*) FROM %s.%s", keyspace, table)
	}

	// Execute the query.
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

	// Get the session for that keyspace.
	store.lock.Lock()
	connection := store.connections[connectionId]
	store.lock.Unlock()

	if connection == nil {
		return nil, errors.New("the connection " + connectionId + " does not exist")
	}

	// Get the first found session.
	session, ok := connection.sessions[keyspace]
	if !ok {
		if connectionId == "local_resource" {
			// Return the first session.
			for _, session := range connection.sessions {
				return session, nil
			}
		}
		return nil, errors.New("connection with id " + connectionId + " does not have a session for keyspace " + keyspace)
	}

	return session, nil
}

func (store *ScyllaStore) insertData(connectionId, keyspace, tableName string, data map[string]interface{}) (map[string]interface{}, error) {
	// Get the session for that keyspace.
	session, err := store.getSession(connectionId, keyspace)
	if err != nil {
		return nil, err
	}

	var id string
	if data["id"] != nil {
		id = data["id"].(string)
	} else if data["_id"] != nil {
		id = data["_id"].(string)
	}

	if len(id) == 0 {
		return nil, errors.New("the id is required")
	}

	// Check if the data already exists.
	query := fmt.Sprintf("SELECT * FROM %s.%s WHERE id='%s'", keyspace, tableName, id)
	if data["domain"] != nil {
		query += fmt.Sprintf(" AND domain='%s'", data["domain"])
	}

	values_, err := store.FindOne(context.Background(), connectionId, keyspace, tableName, query, "")
	if err == nil {
		return values_.(map[string]interface{}), nil
	}

	// Create the table.
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
			// This is an array column, insert the values into the array table
			sliceValue := reflect.ValueOf(data[column])
			length := sliceValue.Len()

			for i := 0; i < length; i++ {
				element := sliceValue.Index(i)
				valueType := reflect.TypeOf(element.Interface())
				field := camelToSnake(column)

				if valueType.Kind() == reflect.Map {
					entity := element.Interface().(map[string]interface{})

					if entity["typeName"] != nil {
						typeName := entity["typeName"].(string)
						if !strings.HasSuffix(typeName, "s") {
							typeName += "s"
						}

						// Ensure the first letter is uppercase.
						typeName = strings.Title(typeName)

						// Set the domain if defined with localhost value.
						localDomain, _ := config.GetDomain()
						if entity["domain"] == nil {
							entity["domain"] = localDomain
						} else if entity["domain"] == "localhost" {
							entity["domain"] = localDomain
						}

						// Save the entity itself.
						var err error
						entity, err = store.insertData(connectionId, keyspace, typeName, entity)
						if err != nil {
							return nil, err
						}

						var _id string

						// Get the entity id.
						if entity["id"] != nil {
							_id = Utility.ToString(entity["id"])
						} else if entity["_id"] != nil {
							_id = Utility.ToString(entity["_id"])
						} else {
							return nil, errors.New("the entity does not have an id")
						}

						sourceCollection := tableName

						// Create the reference table if it does not already exist.
						createTable := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s_%s (source_id TEXT, target_id TEXT, PRIMARY KEY (source_id, target_id))`, sourceCollection, field)
						err = session.Query(createTable).Exec()
						if err != nil {
							return nil, err
						}

						// Insert the reference into the table.
						insertSQL := fmt.Sprintf("INSERT INTO %s_%s (source_id, target_id) VALUES (?, ?);", sourceCollection, field)
						parameters := make([]interface{}, 0)
						parameters = append(parameters, id, _id)

						err = session.Query(insertSQL, parameters...).Exec()
						if err != nil {
							return nil, err
						}
					} else if entity["$ref"] != nil {
						typeName := entity["$ref"].(string)
						if !strings.HasSuffix(typeName, "s") {
							typeName += "s"
						}

						// Ensure the first letter is uppercase.
						typeName = strings.Title(typeName)

						// Get the entity id.
						_id := Utility.ToString(entity["$id"])
						sourceCollection := tableName

						// Create the reference table if it does not already exist.
						createTable := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s_%s (source_id TEXT, target_id TEXT, PRIMARY KEY (source_id, target_id))`, sourceCollection, field)
						err = session.Query(createTable).Exec()
						if err != nil {
							return nil, err
						}

						// Insert the reference into the table.
						insertSQL := fmt.Sprintf("INSERT INTO %s_%s (source_id, target_id) VALUES (?, ?);", sourceCollection, field)
						parameters := make([]interface{}, 0)
						parameters = append(parameters, id, _id)

						err = session.Query(insertSQL, parameters...).Exec()
						if err != nil {
							return nil, err
						}
					}
				} else if !element.IsNil() && element.IsValid() {
					arrayTableName := tableName + "_" + field
					createTable := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (value %s, %s_id TEXT, PRIMARY KEY ((value, %s_id)));", arrayTableName, deduceColumnType(element.Interface()), tableName, tableName)

					// Create the table.
					err := session.Query(createTable).Exec()
					if err != nil {
						return nil, err
					}

					// Insert the value into the table.
					insertSQL := fmt.Sprintf("INSERT INTO %s.%s (value, %s_id) VALUES (?, ?);", keyspace, arrayTableName, tableName)
					parameters := make([]interface{}, 0)
					parameters = append(parameters, element.Interface(), id)

					err = session.Query(insertSQL, parameters...).Exec()
					if err != nil {
						return nil, err
					}
				}
			}
		} else if goType.Kind() == reflect.Map {
			entity := value.(map[string]interface{})

			if entity["typeName"] != nil {
				typeName := entity["typeName"].(string)

				if !strings.HasSuffix(typeName, "s") {
					typeName += "s"
				}

				// Ensure the first letter is uppercase.
				typeName = strings.Title(typeName)

				// Save the entity itself.
				var err error
				entity, err = store.insertData(connectionId, keyspace, typeName, entity)
				if err != nil {
					return nil, err
				}

				// Get the entity id.
				_id := Utility.ToString(entity["_id"])
				sourceCollection := tableName
				field := column

				// Create the reference table if it does not already exist.
				createTable := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s_%s (source_id TEXT, target_id TEXT, PRIMARY KEY (source_id, target_id))`, sourceCollection, field)
				err = session.Query(createTable).Exec()
				if err == nil {
					fmt.Println("Table created: ", sourceCollection+"_"+field)
				}

				// Insert the reference into the table.
				insertSQL := fmt.Sprintf("INSERT INTO %s_%s (source_id, target_id) VALUES (?, ?);", sourceCollection, field)
				parameters := make([]interface{}, 0)
				parameters = append(parameters, id, _id)

				err = session.Query(insertSQL, parameters...).Exec()
				if err != nil {
					return nil, err
				}
			} else if entity["$ref"] != nil {
				typeName := entity["$ref"].(string)

				if !strings.HasSuffix(typeName, "s") {
					typeName += "s"
				}

				// Ensure the first letter is uppercase.
				typeName = strings.Title(typeName)

				// Get the entity id.
				_id := Utility.ToString(entity["$id"])
				sourceCollection := tableName
				field := column

				// Create the reference table if it does not already exist.
				createTable := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s_%s (source_id TEXT, target_id TEXT, PRIMARY KEY (source_id, target_id))`, sourceCollection, field)
				err = session.Query(createTable).Exec()
				if err == nil {
					fmt.Println("Table created: ", sourceCollection+"_"+field)
				}

				// Insert the reference into the table.
				insertSQL := fmt.Sprintf("INSERT INTO %s_%s (source_id, target_id) VALUES (?, ?);", sourceCollection, field)
				parameters := make([]interface{}, 0)
				parameters = append(parameters, id, _id)

				err = session.Query(insertSQL, parameters...).Exec()
				if err != nil {
					return nil, err
				}
			}
		} else if column != "typeName" {
			column = camelToSnake(column)
			columns = append(columns, column)
			values = append(values, value)
		}
	}

	// Ensure the first letter of the table name is uppercase.
	tableName = strings.Title(tableName)

	query = fmt.Sprintf("INSERT INTO %s.%s (%s) VALUES (", keyspace, tableName, joinStrings(columns, ", "))
	placeholders := make([]string, len(columns))

	for i := range columns {
		placeholders[i] = "?"
	}

	query += joinStrings(placeholders, ", ")
	query += ");"

	// Scylla does not support _id, so replace it with id.
	query = strings.ReplaceAll(query, "_id", "id")

	// Execute the CREATE TABLE query
	err = session.Query(query, values...).Exec()
	if err != nil {
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

func (store *ScyllaStore) InsertOne(ctx context.Context, connectionId string, keyspace string, table string, data interface{}, options string) (interface{}, error) {
	entity, err := Utility.ToMap(data)
	if err != nil {
		return nil, err
	}

	// Generate the INSERT statement
	entity, err = store.insertData(connectionId, keyspace, table, entity)
	if err != nil {
		return nil, err
	}

	return entity, nil
}

func (store *ScyllaStore) InsertMany(ctx context.Context, connectionId string, keyspace string, table string, entities []interface{}, options string) ([]interface{}, error) {
	// Get the session for that keyspace.
	for _, data := range entities {
		var err error
		entity, err := Utility.ToMap(data)
		entity, err = store.insertData(connectionId, keyspace, table, entity)
		// Insert the entity.
		if err != nil {
			return nil, err
		}
	}

	return entities, nil
}

func (store *ScyllaStore) getParameters(condition string, values []interface{}) string {
	query := ""
	if condition == "$and" {
		for _, v := range values {
			value := v.(map[string]interface{})
			for key, v := range value {
				if key == "_id" {
					key = "id"
					if strings.Contains(v.(string), "@") {
						v = strings.Split(v.(string), "@")[0]
					}
				}

				key = camelToSnake(key)
				if reflect.TypeOf(v).Kind() == reflect.String {
					query += fmt.Sprintf("%s = '%v' AND ", key, v)
				}
			}
		}
		query = strings.TrimSuffix(query, " AND ")
	} else if condition == "$or" {
		for _, v := range values {
			value := v.(map[string]interface{})
			for key, v := range value {
				if key == "_id" {
					key = "id"
					if strings.Contains(v.(string), "@") {
						v = strings.Split(v.(string), "@")[0]
					}
				}
				key = camelToSnake(key)
				if reflect.TypeOf(v).Kind() == reflect.String {
					query += fmt.Sprintf("%s = '%v' OR ", key, v)
				}
			}
		}
		query = strings.TrimSuffix(query, " OR ")
	}

	return query
}

func (store *ScyllaStore) formatQuery(keyspace, table, q string) (string, error) {
	var query string

	if q == "{}" {
		query = fmt.Sprintf("SELECT * FROM %s.%s", keyspace, table)
	} else {
		parameters := make(map[string]interface{}, 0)
		err := json.Unmarshal([]byte(q), &parameters)
		if err != nil {
			return "", err
		}

		// Build the query here.
		query = fmt.Sprintf("SELECT * FROM %s.%s WHERE ", keyspace, table)
		for key, value := range parameters {
			if key == "_id" {
				key = "id"
				if strings.Contains(value.(string), "@") {
					value = strings.Split(value.(string), "@")[0]
				}
			}

			key = camelToSnake(key)

			if reflect.TypeOf(value).Kind() == reflect.String {
				query += fmt.Sprintf("%s = '%v' AND ", key, value)
			} else if reflect.TypeOf(value).Kind() == reflect.Slice {
				if key == "$and" || key == "$or" || key == "$regex" {
					query += store.getParameters(key, value.([]interface{}))
				}
			} else if reflect.TypeOf(value).Kind() == reflect.Map {
				// Not really a regex but is the only way to do it.
				for k, v := range value.(map[string]interface{}) {
					if k == "$regex" {
						query += fmt.Sprintf("%s LIKE '%v%%' AND ", key, v)
					}
				}
			}
		}

		query = strings.TrimSuffix(query, " AND ")
	}

	return query, nil
}

func (store *ScyllaStore) initArrayEntities(connectionId, keyspace, tableName string, entity map[string]interface{}) error {
	// The field name...
	field := strings.ReplaceAll(tableName, strings.Split(tableName, "_")[0]+"_", "")
	field = snakeToCamel(field)

	if entity[field] != nil {
		return nil // The array is already initialized.
	}

	session, err := store.getSession(connectionId, keyspace)
	if err != nil {
		return err
	}

	// Get the array table.
	query := fmt.Sprintf("SELECT target_id FROM %s.%s WHERE source_id = ? ALLOW FILTERING;", keyspace, tableName)
	iter := session.Query(query, entity["_id"]).Iter()
	defer iter.Close()
	array := []interface{}{}

	for {
		// New map each iteration
		row := make(map[string]interface{})
		if !iter.MapScan(row) {
			break
		}

		// Do things with row
		if targetId, ok := row["target_id"]; ok {
			tableName_ := field

			// Do things with row
			if field == "members" {
				tableName_ = "accounts"
			}

			array = append(array, map[string]interface{}{"$ref": tableName_, "$id": targetId, "$db": keyspace})
		}
	}

	// Set the entity type name.
	entity[field] = array

	if len(array) == 0 {
		return errors.New("no entities found")
	}

	return nil
}

/**
 * Initialize the array values.
 */
func (store *ScyllaStore) initArrayValues(connectionId, keyspace, tableName string, entity map[string]interface{}) error {
	// Get the session for that keyspace.
	session, err := store.getSession(connectionId, keyspace)
	if err != nil {
		return err
	}

	// Get the array table.
	query := fmt.Sprintf("SELECT value FROM %s.%s WHERE %s_id = ? ALLOW FILTERING;", keyspace, tableName, entity["typeName"])
	iter := session.Query(query, entity["_id"]).Iter()
	defer iter.Close()

	array := []interface{}{}

	for {
		// New map each iteration
		row := make(map[string]interface{})
		if !iter.MapScan(row) {
			break
		}
		// Do things with row
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

	// Convert the column names to camel case.
	for key, value := range entity {
		delete(entity, key)
		entity[snakeToCamel(key)] = value
	}

	// Replace the _id by id.
	if entity["id"] != nil {
		entity["_id"] = entity["id"]
		delete(entity, "id")
	}

	if entity["_id"] == nil {
		return nil, errors.New("the _id is required")
	}

	// Set the type name.
	entity["typeName"] = typeName

	if entity["domain"] == nil {
		// Ensure the domain is set...
		localDomain, _ := config.GetDomain()
		entity["domain"] = localDomain
	}

	session, err := store.getSession(connectionId, keyspace)
	if err != nil {
		return nil, err
	}

	// Retrieve the list of all tables in the keyspace.
	query := fmt.Sprintf("SELECT table_name FROM system_schema.tables WHERE keyspace_name = '%s'", keyspace)

	iter := session.Query(query).Iter()
	defer iter.Close()

	var tableName string
	tableNames := []string{}

	for iter.Scan(&tableName) {
		tableNames = append(tableNames, tableName)
	}

	// Now, tableNames contains a list of table names in the specified keyspace.
	for _, tableName := range tableNames {
		// Ignore the array tables.
		if strings.HasPrefix(strings.ToLower(tableName), strings.ToLower(typeName)+"_") {
			// Initialize the array of entities.
			err := store.initArrayEntities(connectionId, keyspace, tableName, entity)

			// Initialize the array values.
			if err != nil {
				store.initArrayValues(connectionId, keyspace, tableName, entity)
			}
		}
	}

	return entity, nil
}

/**
 * Find entities.
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

	// Set the table name.
	if len(query) == 0 {
		return nil, errors.New("query is empty")
	} else if strings.HasPrefix(query, "{") && strings.HasSuffix(query, "}") {
		var err error
		query, err = store.formatQuery(keyspace, table, query)
		query += " ALLOW FILTERING"
		if err != nil {
			return nil, err
		}
	}

	// Execute the query.
	iter := session.Query(query).Iter()
	defer iter.Close()

	// Initialize a slice to store the results.
	results := []map[string]interface{}{}
	for {
		row := make(map[string]interface{})
		if !iter.MapScan(row) {
			break
		}

		// Initialize the entity.
		entity, err := store.initEntity(connectionId, keyspace, table, row)
		if err == nil {
			results = append(results, entity)
		}
	}

	return results, nil
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

	results_ := make([]interface{}, len(results))
	for i, result := range results {
		results_[i] = result
	}

	return results_, nil
}

func (store *ScyllaStore) deleteEntity(connectionId string, keyspace string, table string, entity map[string]interface{}) error {
	session, err := store.getSession(connectionId, keyspace)
	if err != nil {
		return err
	}

	// Delete the entity.
	query := fmt.Sprintf("DELETE FROM %s.%s WHERE id = ?", keyspace, table)
	err = session.Query(query, entity["_id"]).Exec()
	if err != nil {
		return err
	}

	// Now delete the references.
	for column, value := range entity {
		if reflect.TypeOf(value).Kind() == reflect.Slice {
			sliceValue := reflect.ValueOf(value)
			length := sliceValue.Len()

			for i := range length {
				element := sliceValue.Index(i)
				valueType := reflect.TypeOf(element.Interface())
				field := camelToSnake(column)

				if valueType.Kind() == reflect.Map {
					entity_ := element.Interface().(map[string]interface{})
					if entity_["typeName"] != nil {
						// Delete the reference.
						query := fmt.Sprintf("DELETE FROM %s.%s_%s WHERE source_id = ? AND target_id = ?", keyspace, table, field)
						err := session.Query(query, entity["_id"], entity_["id"]).Exec()
						if err == nil {
							fmt.Println("reference deleted: ", query)
						}
					} else if entity_["$ref"] != nil {
						// Delete the reference.
						query := fmt.Sprintf("DELETE FROM %s.%s_%s WHERE source_id = ? AND target_id = ?", keyspace, table, field)
						err := session.Query(query, entity["_id"], entity_["$id"]).Exec()
						if err == nil {
							fmt.Println("reference deleted: ", query)
						}
					}
				} else {
					// Delete the reference.
					query := fmt.Sprintf("DELETE FROM %s.%s_%s WHERE %s_id = ? AND value = ?", keyspace, table, field, table)
					err := session.Query(query, entity["_id"], element.Interface()).Exec()
					if err != nil {
						return err
					}
				}
			}
		}
	}

	return nil
}

func (store *ScyllaStore) ReplaceOne(ctx context.Context, connectionId string, keyspace string, table string, query string, value string, options string) error {
	upsert := false
	if len(options) > 0 {
		options_ := make([]map[string]interface{}, 0)
		err := json.Unmarshal([]byte(options), &options_)
		if err == nil {
			if options_[0]["upsert"] != nil {
				upsert = options_[0]["upsert"].(bool)
			}
		}
	}

	// Insert the new entity.
	data := make(map[string]interface{})
	err := json.Unmarshal([]byte(value), &data)
	if err != nil {
		return err
	}

	// Get the entity.
	entities, err := store.find(connectionId, keyspace, table, query)
	if err != nil && !upsert {
		return err
	}

	// Delete the entity.
	if len(entities) > 0 {
		err = store.deleteEntity(connectionId, keyspace, table, entities[0])
		if err != nil {
			return err
		}
	}

	// Insert the entity.
	_, err = store.insertData(connectionId, keyspace, table, data)
	if err != nil {
		return err
	}

	return nil
}

func (store *ScyllaStore) Update(ctx context.Context, connectionId string, keyspace string, table string, query string, value string, options string) error {
	// Get the session for that keyspace.
	session, err := store.getSession(connectionId, keyspace)
	if err != nil {
		return err
	}

	values_ := make(map[string]interface{}, 0)
	err = json.Unmarshal([]byte(value), &values_)
	if err != nil {
		return err
	}

	if values_["$set"] == nil {
		return errors.New("no $set operator allowed in Update")
	}

	query, err = store.formatQuery(keyspace, table, query)
	if err != nil {
		return err
	}

	query += " ALLOW FILTERING"

	// Get the entities.
	entities, err := store.find(connectionId, keyspace, table, query)
	if err != nil {
		return err
	}

	if len(entities) == 0 {
		return errors.New("no entity found")
	}

	for _, entity := range entities {
		// Retrieve the fields and values to update
		fields := make([]interface{}, 0)
		values := make([]interface{}, 0)
		arrayFields := make([]string, 0)

		for key, value := range values_["$set"].(map[string]interface{}) {
			// Check if the value is an array.
			if reflect.TypeOf(value).Kind() == reflect.Slice {
				arrayFields = append(arrayFields, key) // Process the array later.
			} else {
				fields = append(fields, camelToSnake(key))
				values = append(values, value)
			}
		}

		query := "SELECT * FROM " + table + " WHERE id = ?"
		values = append(values, entity["_id"])

		q, err := generateUpdateTableQuery(keyspace+"."+table, fields, query)
		if err != nil {
			return err
		}

		// Execute the query
		err = session.Query(q, values...).Exec()
		if err != nil {
			return err
		}

		// Update the array fields.
		for _, field := range arrayFields {
			// Get the values
			values := values_["$set"].(map[string]interface{})[field].([]interface{})

			// Get the array table.
			arrayTableName := table + "_" + field

			// Delete existing values one by one because ALLOW FILTERING does not work...
			for _, value := range entity[field].([]interface{}) {
				deleteQuery := fmt.Sprintf("DELETE FROM %s.%s WHERE %s_id = ? AND value = ?", keyspace, arrayTableName, table)
				err = session.Query(deleteQuery, entity["_id"], value).Exec()
				if err != nil {
					return err
				}
			}

			// Insert the new values.
			for _, value := range values {
				insertQuery := fmt.Sprintf("INSERT INTO %s.%s (value, %s_id) VALUES (?, ?)", keyspace, arrayTableName, table)
				err = session.Query(insertQuery, value, entity["_id"]).Exec()
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func (store *ScyllaStore) UpdateOne(ctx context.Context, connectionId string, keyspace string, table string, query string, value string, options string) error {
	values_ := make(map[string]interface{}, 0)
	err := json.Unmarshal([]byte(value), &values_)
	if err != nil {
		return err
	}

	if values_["$set"] == nil {
		return errors.New("no $set operator allowed in UpdateOne")
	}

	query, err = store.formatQuery(keyspace, table, query)
	if err != nil {
		return err
	}

	// Retrieve the fields and values to update
	fields := make([]interface{}, 0)
	values := make([]interface{}, 0)
	arrayFields := make([]string, 0)

	for key, value := range values_["$set"].(map[string]interface{}) {
		// Check if the value is an array.
		if reflect.TypeOf(value).Kind() == reflect.Slice {
			arrayFields = append(arrayFields, key) // Process the array later.
		} else {
			fields = append(fields, camelToSnake(key))
			values = append(values, value)
		}
	}

	q, err := generateUpdateTableQuery(keyspace+"."+table, fields, query)
	if err != nil {
		return err
	}

	session, err := store.getSession(connectionId, keyspace)
	if err != nil {
		return err
	}

	// Execute the query
	err = session.Query(q, values...).Exec()
	if err != nil {
		return err
	}

	// Get the entity.
	entities, err := store.find(connectionId, keyspace, table, query)
	if err != nil {
		return err
	}

	if len(entities) == 0 {
		return errors.New("no entity found")
	}

	entity := entities[0]

	// Update the array fields.
	for _, field := range arrayFields {
		// Get the values
		values := values_["$set"].(map[string]interface{})[field].([]interface{})

		// Get the array table.
		arrayTableName := table + "_" + field

		// Delete existing values one by one because ALLOW FILTERING does not work...
		for _, value := range entity[field].([]interface{}) {
			deleteQuery := fmt.Sprintf("DELETE FROM %s.%s WHERE %s_id = ? AND value = ?", keyspace, arrayTableName, table)
			err = session.Query(deleteQuery, entity["_id"], value).Exec()
			if err != nil {
				return err
			}
		}

		// Insert the new values.
		for _, value := range values {
			insertQuery := fmt.Sprintf("INSERT INTO %s.%s (value, %s_id) VALUES (?, ?)", keyspace, arrayTableName, table)
			err = session.Query(insertQuery, value, entity["_id"]).Exec()
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (store *ScyllaStore) Delete(ctx context.Context, connectionId string, keyspace string, table string, query string, options string) error {
	// Get the entity.
	entity, err := store.find(connectionId, keyspace, table, query)
	if err != nil {
		return err
	}

	// Delete the entity.
	for _, entity := range entity {
		err = store.deleteEntity(connectionId, keyspace, table, entity)
		if err != nil {
			return err
		}
	}

	return nil
}

func (store *ScyllaStore) DeleteOne(ctx context.Context, connectionId string, keyspace string, table string, query string, options string) error {
	// Get the entity.
	entity, err := store.find(connectionId, keyspace, table, query)
	if err != nil {
		return err
	}

	// Delete the entity.
	if len(entity) > 0 {
		err = store.deleteEntity(connectionId, keyspace, table, entity[0])
		if err != nil {
			return err
		}
	}

	return nil
}

func (store *ScyllaStore) Aggregate(ctx context.Context, connectionId string, keyspace string, table string, pipeline string, optionsStr string) ([]interface{}, error) {
	return nil, errors.New("not implemented")
}

func (store *ScyllaStore) CreateTable(ctx context.Context, connectionId string, db string, table string, fields []string) error {
	// Get the session for that keyspace.
	session, err := store.getSession(connectionId, db)
	if err != nil {
		return err
	}

	// Create the keyspace if it does not already exist.
	_, err = store.createKeyspace(connectionId, db)
	if err != nil {
		return err
	}

	// Create the table
	createTable := fmt.Sprintf("CREATE TABLE IF NOT EXISTS \"%s\" (id TEXT PRIMARY KEY, %s);", table, strings.Join(fields, ", "))

	// Execute the CREATE TABLE query
	err = session.Query(createTable).Exec()
	if err != nil {
		return err
	}

	return nil
}

// The collection will be created when the first entity is inserted...
func (store *ScyllaStore) CreateCollection(ctx context.Context, connectionId string, keyspace string, collection string, options string) error {
	return errors.New("not implemented")
}

func dropTable(session *gocql.Session, keyspace, tableName string) error {
	query := fmt.Sprintf("DROP TABLE IF EXISTS %s.%s;", keyspace, tableName)
	return session.Query(query).Exec()
}

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

	// Drop the table.
	if err := dropTable(session, keyspace, collection); err != nil {
		return err
	}

	return nil
}

func splitCQLScript(script string) []string {
	var statements []string
	var currentStatement strings.Builder

	inString := false
	for _, runeValue := range script {
		char := string(runeValue)

		// Toggle the inString flag if we encounter a single quote
		if char == "'" {
			inString = !inString
		}

		// Add the character to the current statement
		currentStatement.WriteString(char)

		// If we encounter a semicolon and we're not inside a string, end the current statement
		if char == ";" && !inString {
			statements = append(statements, strings.TrimSpace(currentStatement.String()))
			currentStatement.Reset()
		}
	}

	// Add any remaining part of the script as the last statement
	if currentStatement.Len() > 0 {
		statements = append(statements, strings.TrimSpace(currentStatement.String()))
	}

	return statements
}

func (store *ScyllaStore) RunAdminCmd(ctx context.Context, connectionId string, user string, password string, script string) error {
	// Get the host.
	store.lock.Lock()
	connection := store.connections[connectionId]
	store.lock.Unlock()
	if connection == nil {
		return errors.New("the connection does not exist")
	}

	host := connection.Host

	// Validate the user and password.
	authentication_client, err := authentication_client.NewAuthenticationService_Client(host, "authentication.AuthenticationService")
	if err != nil {
		return err
	}

	// Authenticate the user, try 5 times.
	nbTry := 5
	for nbTry > 0 {
		var err error
		// Authenticate the user.
		_, err = authentication_client.Authenticate(user, password)
		if err != nil && nbTry == 0 {
			return err
		} else if err == nil {
			break
		}

		time.Sleep(1 * time.Second)
	}

	// Create the admin session.
	adminCluster := gocql.NewCluster(connection.Host)
	adminCluster.Keyspace = "system" // Use the 'system' keyspace for administrative tasks
	adminCluster.Consistency = gocql.Quorum
	adminSession, err := adminCluster.CreateSession()
	if err != nil {
		return err
	}
	defer adminSession.Close()

	// Split the script into individual statements
	statements := splitCQLScript(script)
	for _, statement := range statements {
		// Remove leading/trailing spaces and execute the statement
		statement = strings.TrimSpace(statement)
		if statement != "" {
			if err := adminSession.Query(statement).Exec(); err != nil {
				return err
			}
		}
	}

	return nil
}
