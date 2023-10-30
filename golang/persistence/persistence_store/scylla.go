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
	"github.com/gocql/gocql"
)

// Connection represent a connection to a SQL database.
type ScyllaConnection struct {
	Id       string
	Host     string
	Token    string
	sessions map[string]*gocql.Session
}

/**
 * The ScyllaDB store.
 */
type ScyllaStore struct {

	/** the connections... */
	connections map[string]*ScyllaConnection
	lock        sync.Mutex
}

func (store *ScyllaStore) GetStoreType() string {
	return "SCYLLADB"
}

/**
 * Create a new ScyllaDB store.
 */
func (store *ScyllaStore) createKeyspace(connectionId, keyspace string) (*gocql.ClusterConfig, error) {

	if len(keyspace) == 0 {
		return nil, errors.New("the database is required")
	}

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
		'replication_factor': 1
	}`

	// Create the admin session.
	adminCluster := gocql.NewCluster()                          // Replace with your ScyllaDB cluster IP address
	adminCluster.Hosts = []string{connection.Host, "127.0.0.1"} // add local host as well.
	adminCluster.Keyspace = "system"                            // Use the 'system' keyspace for administrative tasks
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
	cluster := gocql.NewCluster() // Set your ScyllaDB cluster address here
	cluster.Keyspace = keyspace   // Set your keyspace name here
	cluster.Consistency = gocql.Quorum
	cluster.Hosts = []string{connection.Host, "127.0.0.1"}
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

	if len(user) == 0 {
		return errors.New("the user is required")

	}

	if len(password) == 0 {
		return errors.New("the password is required")
	}

	if len(keyspace) == 0 {
		return errors.New("the database is required")
	}

	// So here I will authenticate the user and password.
	authentication_client, err := authentication_client.NewAuthenticationService_Client(host, "authentication.AuthenticationService")
	if err != nil {
		return err
	}

	// Authenticate the user, I will try 5 times.
	nbTry := 5
	var token string
	for nbTry > 0 {

		var err error
		// Authenticate the user.
		token, err = authentication_client.Authenticate(user, password)
		if err != nil && nbTry == 0 {
			fmt.Println("Fail to authenticate user ", user, err)
			return err
		} else if err == nil {
			break
		}

		time.Sleep(1 * time.Second)
	}

	var connection *ScyllaConnection
	if _, ok := store.connections[id]; ok {
		connection = store.connections[id]
	} else {
		// Now I will save the connection.
		connection = &ScyllaConnection{
			Id:       id,
			Host:     host,
			Token:    token,
			sessions: make(map[string]*gocql.Session),
		}
	}

	// Save the connection.
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

	return nil
}

/**
 * Disconnect from the database.
 */
func (store *ScyllaStore) Disconnect(connectionId string) error {
	// close all sessions for that connection.
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
		fmt.Printf("Failed to execute query: %v\n", err)
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

func (store *ScyllaStore) isTableExist(connectionId string, keyspace string, table string) bool {
	// I will get the session for that keyspace.
	store.lock.Lock()
	connection := store.connections[connectionId]
	store.lock.Unlock()
	if connection == nil {
		return false
	}

	// Get the first found session.
	session, ok := connection.sessions[keyspace]
	if !ok {
		return false
	}

	// Check if the table exist.
	query := session.Query(`SELECT columnfamily_name FROM system.schema_columnfamilies WHERE keyspace_name = ? AND columnfamily_name = ?`, keyspace, table)
	iter := query.Iter()
	defer iter.Close()

	// Test if the table exist.
	return iter.NumRows() > 0
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
		//fmt.Println("unsupported data type: %s", goType.String())
		return ""
	}
}

func (store *ScyllaStore) createScyllaTable(session *gocql.Session, keyspace, tableName string, data map[string]interface{}) error {
	// Prepare the CREATE TABLE query
	createTableQuery := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s.%s (", keyspace, tableName)

	// Iterate through the map's fields and deduce the data types
	for fieldName, value := range data {
		fieldType := deduceColumnType(value)
		if fieldType != "unknow" {
			if fieldType != "array" {
				fieldName = camelToSnake(fieldName)
				createTableQuery += fieldName + " " + fieldType + ", "
			}
		}
	}

	// scylla does not support _id, so I will replace it with id.
	createTableQuery = strings.ReplaceAll(createTableQuery, "_id", "id")

	if !strings.Contains(createTableQuery, "id") {
		createTableQuery += "id text, "
	}

	// Add the primary key
	createTableQuery += "PRIMARY KEY (id));"

	// Execute the CREATE TABLE query
	err := session.Query(createTableQuery).Exec()
	if err != nil {
		fmt.Println("Failed to create table ", tableName, "with error:", err)
	}

	return err
}

func deleteKeyspace(host, keyspace string) error {

	// Create the admin session.
	adminCluster := gocql.NewCluster()               // Replace with your ScyllaDB cluster IP address
	adminCluster.Hosts = []string{host, "127.0.0.1"} // add local host as well.
	adminCluster.Keyspace = "system"                 // Use the 'system' keyspace for administrative tasks
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

	// I will get the session for that keyspace.
	store.lock.Lock()
	connection := store.connections[connectionId]
	store.lock.Unlock()

	if connection == nil {
		return errors.New("the connection does not exist")
	}

	// Drop the keyspace.
	if err := deleteKeyspace(connection.Host, keyspace); err != nil {
		fmt.Println("Fail to drop keyspace ", err)
		return err
	}

	return nil
}

func (store *ScyllaStore) Count(ctx context.Context, connectionId string, keyspace string, table string, query string, options string) (int64, error) {
	return 0, errors.New("not implemented")
}

func (store *ScyllaStore) insertData(connectionId, keyspace, tableName string, data map[string]interface{}) (map[string]interface{}, error) {

	// I will get the session for that keyspace.
	store.lock.Lock()
	connection := store.connections[connectionId]
	store.lock.Unlock()

	if connection == nil {
		return nil, errors.New("the connection does not exist")
	}

	// Get the first found session.
	session, ok := connection.sessions[keyspace]
	if !ok {
		return nil, errors.New("the session does not exist")
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

	// test if the data already exist.
	if store.isTableExist(connectionId, keyspace, tableName) {
		// I will check if the data already exist.
		query := fmt.Sprintf("SELECT * FROM %s WHERE id='%s'", tableName, id)
		values, err := store.FindOne(context.Background(), connectionId, keyspace, tableName, query, "")
		if err == nil {
			return values.(map[string]interface{}), nil
		}
	}

	columns := make([]string, 0)
	values := make([]interface{}, 0)

	for column, value := range data {
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

						// I will save the entity itself.
						var err error
						entity, err = store.insertData(connectionId, keyspace, typeName+"s", entity)
						if err != nil {
							fmt.Println("Error inserting data into array table: ", err)
						}

						// I will get the entity id.
						_id := Utility.ToInt(entity["_id"])
						sourceCollection := tableName

						// He I will create the reference table.
						// I will create the table if not already exist.
						createTable := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS ` + sourceCollection + `_` + field + ` (source_id TEXT, target_id TEXT, PRIMARY KEY (source_id, target_id))`)
						err = session.Query(createTable).Exec()
						if err != nil {
							fmt.Println("Error creating array table: ", createTable, err)
						}

						// I will insert the reference into the table.
						insertSQL := fmt.Sprintf("INSERT INTO " + sourceCollection + "_" + field + " (source_id, target_id) VALUES (?, ?);")
						parameters := make([]interface{}, 0)
						parameters = append(parameters, id)
						parameters = append(parameters, _id)

						err = session.Query(insertSQL, parameters...).Exec()

						if err != nil {
							fmt.Println("Error inserting data into array table: ", err)
						}
					}
				} else {

					arrayTableName := tableName + "_" + field
					createTable := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (value %s, %s_id TEXT, PRIMARY KEY ((value, %s_id)));", arrayTableName, deduceColumnType(element.Interface()), tableName, tableName)

					fmt.Println("--------------------> ", createTable)

					// Create the table.
					err := session.Query(createTable).Exec()
					if err != nil {
						fmt.Println("Error creating array table: ", createTable, err)
					}

					// Insert the value into the table.
					insertSQL := fmt.Sprintf("INSERT INTO %s (value, %s_id) VALUES (?, ?);", arrayTableName, tableName)
					parameters := make([]interface{}, 0)
					parameters = append(parameters, element.Interface())
					parameters = append(parameters, id)

					err = session.Query(insertSQL, parameters...).Exec()
					if err != nil {
						fmt.Println("Error inserting data into array table: ", err)
					}
				}
			}

		} else if goType.Kind() == reflect.Map {
			entity := value.(map[string]interface{})
			typeName := entity["typeName"].(string)

			// I will save the entity itself.
			var err error
			entity, err = store.insertData(connectionId, keyspace, typeName+"s", entity)
			if err != nil {
				fmt.Println("Error inserting data into array table: ", err)
			}

			// I will get the entity id.
			_id := Utility.ToInt(entity["_id"])
			sourceCollection := tableName
			field := column

			// He I will create the reference table.
			// I will create the table if not already exist.
			createTable := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS ` + sourceCollection + `_` + field + ` (source_id TEXT, target_id TEXT, PRIMARY KEY (source_id, target_id))`)
			err = session.Query(createTable).Exec()
			if err == nil {
				fmt.Println("Table created: ", sourceCollection+"_"+field)
			}

			// I will insert the reference into the table.
			insertSQL := fmt.Sprintf("INSERT INTO " + sourceCollection + "_" + field + " (source_id, target_id) VALUES (?, ?);")
			parameters := make([]interface{}, 0)
			parameters = append(parameters, id)
			parameters = append(parameters, _id)

			err = session.Query(insertSQL, parameters...).Exec()

			if err != nil {
				fmt.Println("Error inserting data into array table: ", err)
			}

		} else if column != "typeName" {
			column = camelToSnake(column)
			columns = append(columns, column)
			values = append(values, value)
		}
	}

	query := fmt.Sprintf("INSERT INTO %s.%s (%s) VALUES (", keyspace, tableName, joinStrings(columns, ", "))
	placeholders := make([]string, len(columns))

	for i := range columns {
		placeholders[i] = "?"
	}

	query += joinStrings(placeholders, ", ")
	query += ");"

	// scylla does not support _id, so I will replace it with id.
	query = strings.ReplaceAll(query, "_id", "id")

	// Execute the CREATE TABLE query
	err := session.Query(query, values...).Exec()
	if err != nil {
		fmt.Println(query)
		fmt.Println("Failed to insert entity ", err)
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

func (store *ScyllaStore) InsertOne(ctx context.Context, connectionId string, keyspace string, table string, entity interface{}, options string) (interface{}, error) {

	// I will get the session for that keyspace.
	store.lock.Lock()
	connection := store.connections[connectionId]
	store.lock.Unlock()

	if connection == nil {
		return nil, errors.New("the connection does not exist")
	}

	// Get the first found session.
	session, ok := connection.sessions[keyspace]
	if !ok {
		return nil, errors.New("the session does not exist")
	}

	// Check if the table exist.
	if !store.isTableExist(connectionId, keyspace, table) {
		// return nil, errors.New("the table does not exist")
		// Create the table.
		if err := store.createScyllaTable(session, keyspace, table, entity.(map[string]interface{})); err != nil {
			fmt.Println("Fail to create table ", err)
			return nil, err
		}

		// Wait for the table to be created.
		time.Sleep(1 * time.Second)
	}

	// Generate the INSERT statement
	var err error
	entity, err = store.insertData(connectionId, keyspace, table, entity.(map[string]interface{}))
	if err != nil {
		return nil, err
	}

	return entity, nil
}

func (store *ScyllaStore) InsertMany(ctx context.Context, connectionId string, keyspace string, table string, entities []interface{}, options string) ([]interface{}, error) {

	// I will get the session for that keyspace.
	store.lock.Lock()
	connection := store.connections[connectionId]
	store.lock.Unlock()

	if connection == nil {
		return nil, errors.New("the connection does not exist")
	}

	// Get the first found session.
	session, ok := connection.sessions[keyspace]
	if !ok {
		return nil, errors.New("the session does not exist")
	}

	// Check if the table exist.
	if !store.isTableExist(connectionId, keyspace, table) {
		// return nil, errors.New("the table does not exist")
		// Create the table.
		if err := store.createScyllaTable(session, keyspace, table, entities[0].(map[string]interface{})); err != nil {
			fmt.Println("Fail to create table ", err)
			return nil, err
		}

		// Wait for the table to be created.
		time.Sleep(1 * time.Second)
	}

	for _, entity := range entities {
		var err error

		entity, err = store.insertData(connectionId, keyspace, table, entity.(map[string]interface{}))
		// insert the entity.
		if err != nil {
			return nil, err
		}
	}

	return entities, nil
}

func (store *ScyllaStore) getParameters(condition string, values []interface{}) string {
	query := ""

	if condition == "$and" {
		query += "("
		for _, v := range values {
			value := v.(map[string]interface{})
			for key, v := range value {
				key = camelToSnake(key)
				if reflect.TypeOf(v).Kind() == reflect.String {
					query += fmt.Sprintf("%s = '%v' AND ", key, v)
				}

			}
		}
		query = strings.TrimSuffix(query, " AND ")
		query += ")"
	}

	return query
}

func (store *ScyllaStore) formatQuery(table, query string) (string, error) {

	if query == "{}" {
		query = fmt.Sprintf("SELECT * FROM %s", table)
	} else {

		parameters := make(map[string]interface{}, 0)
		err := json.Unmarshal([]byte(query), &parameters)
		if err != nil {
			return "", err
		}

		// I will build the query here.
		query = fmt.Sprintf("SELECT * FROM %s WHERE ", table)
		for key, value := range parameters {
			key = camelToSnake(key)
			if key == "_id" {
				key = "id"
			}

			if reflect.TypeOf(value).Kind() == reflect.String {
				query += fmt.Sprintf("%s = '%v' AND ", key, value)
			} else if reflect.TypeOf(value).Kind() == reflect.Slice {
				if key == "$and" || key == "$or" {
					query += store.getParameters(key, value.([]interface{}))
				}
			}

		}

		query = strings.TrimSuffix(query, " AND ")
	}

	return query, nil
}

func (store *ScyllaStore) initArrayEntities(connectionId, keyspace, tableName string, entity map[string]interface{}) error {

	field := tableName[len(entity["typeName"].(string))+1:]
	field = snakeToCamel(field)
	if entity[field] != nil {
		return nil // The array is already initialized.
	}

	// I will get the session for that keyspace.
	store.lock.Lock()
	connection := store.connections[connectionId]
	store.lock.Unlock()

	if connection == nil {
		return errors.New("the connection does not exist")
	}

	// Get the first found session.
	session, ok := connection.sessions[keyspace]
	if !ok {
		return errors.New("the session does not exist")
	}

	// Get the array table.
	query := fmt.Sprintf("SELECT target_id FROM %s WHERE source_id = ? ALLOW FILTERING;", tableName)
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
			// I will get the entity.
			query := fmt.Sprintf("SELECT * FROM %s WHERE _id = ? ALLOW FILTERING;", tableName[:len(tableName)-len(field)-1]+"s")
			iter := session.Query(query, targetId).Iter()
			defer iter.Close()

			for {
				// New map each iteration
				row := make(map[string]interface{})
				if !iter.MapScan(row) {
					break
				}

				// Do things with row
				err := store.initEntity(connectionId, keyspace, tableName[:len(tableName)-len(field)-1]+"s", row)
				if err == nil {
					array = append(array, row)
				}
			}
		}
	}

	// I will get the entity type name.
	entity[snakeToCamel(field)] = array

	if err := iter.Close(); err != nil {
		return err
	}

	return nil

}

/**
 * Initialize the array values.
 */
func (store *ScyllaStore) initArrayValues(connectionId, keyspace, tableName string, entity map[string]interface{}) error {

	// I will get the session for that keyspace.
	store.lock.Lock()
	connection := store.connections[connectionId]
	store.lock.Unlock()

	if connection == nil {
		return errors.New("the connection does not exist")
	}

	// Get the first found session.
	session, ok := connection.sessions[keyspace]
	if !ok {
		return errors.New("the session does not exist")
	}

	// Get the array table.
	query := fmt.Sprintf("SELECT value FROM %s WHERE %s_id = ? ALLOW FILTERING;", tableName, entity["typeName"])
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

	field := tableName[len(entity["typeName"].(string))+1:]
	entity[snakeToCamel(field)] = array

	if err := iter.Close(); err != nil {
		return err
	}

	return nil
}

func (store *ScyllaStore) initEntity(connectionId, keyspace, typeName string, entity map[string]interface{}) error {
	entity["typeName"] = typeName

	// I will convert the column names to camel case.
	for key, value := range entity {
		delete(entity, key)
		entity[snakeToCamel(key)] = value
	}

	// I will replace the _id by id.
	if entity["id"] != nil {
		entity["_id"] = entity["id"]
		delete(entity, "id")
	}

	// Now I will try to get tables that contains references to this entity.
	store.lock.Lock()
	connection := store.connections[connectionId]
	store.lock.Unlock()

	if connection == nil {
		return errors.New("the connection does not exist")
	}

	// Get the first found session.
	session, ok := connection.sessions[keyspace]
	if !ok {
		return errors.New("the session does not exist")
	}

	// Retreive the list of all tables in teh keyspace.
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

		// I will ignore the array tables.
		if strings.HasPrefix(strings.ToLower(tableName), strings.ToLower(typeName)+"_") {

			// I will initialize the array values.
			err := store.initArrayValues(connectionId, keyspace, tableName, entity)
			if err != nil {
				// I will initialize the array of entities.
				store.initArrayEntities(connectionId, keyspace, tableName, entity)
			}
		}

	}

	return nil

}

/**
 * Find entities.
 */
func (store *ScyllaStore) find(connectionId, keyspace, table, query string) ([]map[string]interface{}, error) {
	// I will get the session for that keyspace.
	store.lock.Lock()
	connection := store.connections[connectionId]
	store.lock.Unlock()

	if connection == nil {
		return nil, errors.New("the connection does not exist")
	}

	// Get the first found session.
	session, ok := connection.sessions[keyspace]
	if !ok {
		return nil, errors.New("the session does not exist")
	}

	if len(query) == 0 {
		return nil, errors.New("query is empty")
	} else if strings.HasPrefix(query, "{") && strings.HasSuffix(query, "}") {
		var err error
		query, err = store.formatQuery(table, query)
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

		// init
		err := store.initEntity(connectionId, keyspace, table, row)
		if err == nil {
			results = append(results, row)
		}
	}

	// Now 'results' contains an array of maps with column names and their actual values.
	for i, result := range results {
		fmt.Printf("Row %d:\n", i)
		for colName, value := range result {
			fmt.Printf("  Column: %s, Value: %v\n", colName, value)
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

	return []interface{}{results}, nil
}

func (store *ScyllaStore) deleteEntity(connectionId string, keyspace string, table string, entity map[string]interface{}) error {

	// I will get the session for that keyspace.
	store.lock.Lock()
	connection := store.connections[connectionId]
	store.lock.Unlock()

	if connection == nil {
		return errors.New("the connection does not exist")
	}

	// Get the first found session.
	session, ok := connection.sessions[keyspace]
	if !ok {
		return errors.New("the session does not exist")
	}

	// I will delete the entity.
	query := fmt.Sprintf("DELETE FROM %s WHERE id = ?", table)
	err := session.Query(query, entity["_id"]).Exec()
	if err != nil {
		fmt.Println("fail to delete entity with error: ", err)
		return err
	}

	// Now I will delete the references.
	for column, value := range entity {
		if reflect.TypeOf(value).Kind() == reflect.Slice {

			sliceValue := reflect.ValueOf(value)
			length := sliceValue.Len()

			for i := 0; i < length; i++ {
				element := sliceValue.Index(i)
				valueType := reflect.TypeOf(element.Interface())
				field := camelToSnake(column)

				if valueType.Kind() == reflect.Map {
					entity_ := element.Interface().(map[string]interface{})
					if entity_["typeName"] != nil {

						// I will delete the reference.
						query := fmt.Sprintf("DELETE FROM %s_%s WHERE source_id = ? AND target_id = ?", table, field)
						err := session.Query(query, entity["_id"], entity_["id"]).Exec()
						if err == nil {
							fmt.Println("reference deleted: ", query)
						}
					}

				} else {
					// I will delete the reference.
					query := fmt.Sprintf("DELETE FROM %s_%s WHERE %s_id = ? AND value = ?", table, field, table)
					err := session.Query(query, entity["_id"], element.Interface()).Exec()
					if err != nil {
						fmt.Println("Error deleting reference: ", err)
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

	// I will get the entity.
	entity, err := store.find(connectionId, keyspace, table, query)
	if err != nil && !upsert {
		return err
	}

	// I will delete the entity.
	if len(entity) > 0 {
		err = store.deleteEntity(connectionId, keyspace, table, entity[0])
		if err != nil {
			return err
		}
	}

	// I will insert the new entity.
	data := make(map[string]interface{})
	err = json.Unmarshal([]byte(value), &data)
	if err != nil {
		return err
	}

	// I will insert the entity.
	_, err = store.insertData(connectionId, keyspace, table, data)

	return err
}

func (store *ScyllaStore) Update(ctx context.Context, connectionId string, keyspace string, table string, query string, value string, options string) error {
	
	// I will get the session for that keyspace.
	store.lock.Lock()
	connection := store.connections[connectionId]
	store.lock.Unlock()

	if connection == nil {
		return errors.New("the connection does not exist")
	}

	// Get the first found session.
	session, ok := connection.sessions[keyspace]

	if !ok {
		return errors.New("the session does not exist")
	}

	
	values_ := make(map[string]interface{}, 0)
	err := json.Unmarshal([]byte(value), &values_)
	if err != nil {
		return err
	}

	if values_["$set"] == nil {
		return errors.New("no $set operator allowed in UpdateOne")
	}

	query, err = store.formatQuery(table, query)
	if err != nil {
		return err
	}

	query += " ALLOW FILTERING"

	// I will get the entities.
	entities, err := store.find(connectionId, keyspace, table, query)

	if err != nil {
		return err
	}

	if len(entities) == 0 {
		return errors.New("no entity found")
	}

	for _, entity := range entities {
		// Here I will retreive the fiedls
		fields := make([]interface{}, 0)
		values := make([]interface{}, 0)

		for key, value := range values_["$set"].(map[string]interface{}) {
			fields = append(fields, camelToSnake(key))
			values = append(values, value)
		}

		query := "SELECT * FROM " + table + " WHERE id = ?"
		values = append(values, entity["_id"])

		q, err := generateUpdateTableQuery(table, fields, query)
		if err != nil {
			return err
		}

		// Execute the query
		err = session.Query(q, values...).Exec()
		if err != nil {
			return err
		}
	}

	return err
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

	query, err = store.formatQuery(table, query)
	if err != nil {
		return err
	}

	// Here I will retreive the fiedls
	fields := make([]interface{}, 0)
	values := make([]interface{}, 0)

	for key, value := range values_["$set"].(map[string]interface{}) {
		fields = append(fields, camelToSnake(key))
		values = append(values, value)
	}

	q, err := generateUpdateTableQuery(table, fields, query)
	if err != nil {
		return err
	}

	// I will get the session for that keyspace.
	store.lock.Lock()
	connection := store.connections[connectionId]
	store.lock.Unlock()

	if connection == nil {
		return errors.New("the connection does not exist")
	}

	// Get the first found session.
	session, ok := connection.sessions[keyspace]

	if !ok {
		return errors.New("the session does not exist")
	}

	// Execute the query
	err = session.Query(q, values...).Exec()

	return err
}

func (store *ScyllaStore) Delete(ctx context.Context, connectionId string, keyspace string, table string, query string, options string) error {

	// I will get the entity.
	entity, err := store.find(connectionId, keyspace, table, query)
	if err != nil {
		return err
	}

	// I will delete the entity.
	for _, entity := range entity {
		err = store.deleteEntity(connectionId, keyspace, table, entity)
		if err != nil {
			return err
		}
	}

	return nil
}

func (store *ScyllaStore) DeleteOne(ctx context.Context, connectionId string, keyspace string, table string, query string, options string) error {

	// I will get the entity.
	entity, err := store.find(connectionId, keyspace, table, query)
	if err != nil {
		return err
	}

	// I will delete the entity.
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

	// I will get the session for that keyspace.
	store.lock.Lock()
	connection := store.connections[connectionId]
	store.lock.Unlock()
	if connection == nil {
		return errors.New("the connection does not exist")
	}

	// Get the first found session.
	session, ok := connection.sessions[keyspace]
	if !ok {
		return errors.New("the session does not exist")
	}

	// Drop the table.
	if err := dropTable(session, keyspace, collection); err != nil {
		fmt.Println("Fail to drop table ", err)
		return err
	}

	return nil
}

func (store *ScyllaStore) RunAdminCmd(ctx context.Context, connectionId string, user string, password string, script string) error {
	return errors.New("not implemented")
}
