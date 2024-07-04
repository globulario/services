package persistence_store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"time"
	"unicode"

	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/authentication/authentication_client"
	"github.com/globulario/services/golang/config"
	_ "github.com/mattn/go-sqlite3" // Import the sqlite3 driver
)

// Connection represent a connection to a SQL database.
type SqlConnection struct {
	Id        string
	Host      string
	Token     string
	Path      string
	databases map[string]*sql.DB
}

/**
 * The SQL store.
 */
type SqlStore struct {
	/** The connections */
	connections map[string]SqlConnection
}

func (store *SqlStore) GetStoreType() string {
	return "SQL"
}

// ///////////////////////////////////// Get SQL Client //////////////////////////////////////////

// Connect to the SQL database.
func (store *SqlStore) Connect(id string, host string, port int32, user string, password string, database string, timeout int32, options_str string) error {

	if len(id) == 0 {
		return errors.New("the connection id is required")
	}

	if store.connections != nil {
		if _, ok := store.connections[id]; ok {
			if store.connections[id].databases != nil {
				if _, ok := store.connections[id].databases[database]; ok {
					return nil
				}
			}
		}
	} else {
		store.connections = make(map[string]SqlConnection)
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

	if len(database) == 0 {
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
			
			fmt.Println("--------------> 99 ", err)
			return err
		} else if err == nil {
			break
		}

		nbTry--
		time.Sleep(1 * time.Second)
	}

	// be sure that the data path exist.
	options := make(map[string]interface{})
	if len(options_str) > 0 {
		err := json.Unmarshal([]byte(options_str), &options)
		if err != nil {
			//fmt.Println("Fail to parse options ", err)
			return err
		}
	}

	// set default path
	path := config.GetDataDir() + "/sql-data"

	fmt.Println("----------------> 120")

	// set the path if it is provided.
	if options["path"] != nil {
		path = options["path"].(string)
	}

	// Create the directory if it does not exist.
	Utility.CreateDirIfNotExist(path)

	// Create the connection.
	var connection SqlConnection
	if _, ok := store.connections[id]; ok {
		connection = store.connections[id]
	} else {
		connection = SqlConnection{
			Id:        id,
			Host:      host,
			Token:     token,
			Path:      path,
			databases: make(map[string]*sql.DB, 0),
		}

		// Save the connection.
		store.connections[id] = connection
	}


	// Create the database if it does not exist.
	databasePath := connection.Path + "/" + database + ".db"

	fmt.Println("Database path: ", databasePath)
	fmt.Println("connection: ", id)
	fmt.Println("database: ", store.connections[id])

	// Create the database.
	db, err := sql.Open("sqlite3", databasePath)
	if err != nil {
		return err
	}

	connection.databases[database] = db

	// Create the table if it does not exist.
	count, _ := store.Count(context.Background(), id, "", "user_data", `SELECT * FROM user_data WHERE _id='`+user+`'`, "")
	if count == 0 && id != "local_resource" {
		store.InsertOne(context.Background(), id, database, "user_data", map[string]interface{}{"_id": user, "first_name": "", "last_name": "", "middle_name": "", "profile_picture": "", "email": ""}, "")
	}

	return nil
}

func (store *SqlStore) ExecContext(connectionId string, database string, query string, parameters []interface{}, tx_ int) (string, error) {
	// Type assert the connection and query to their respective types
	if len(connectionId) == 0 {
		return "", errors.New("the connection id is required")
	}

	if len(database) == 0 {
		return "", errors.New("the database is required")
	}

	conn, exists := store.connections[connectionId]
	if !exists {
		return "", fmt.Errorf("connection with ID %s does not exist", connectionId)
	}

	if conn.databases == nil {
		conn.databases = make(map[string]*sql.DB, 0)
	}

	if conn.databases[database] == nil {
		databasePath := conn.Path + "/" + database + ".db"

		// Create the database.
		db, err := sql.Open("sqlite3", databasePath)
		if err != nil {
			return "", err
		}
		conn.databases[database] = db
	}

	hasTx := tx_ == 1

	// Execute the query here.
	var result sql.Result
	if hasTx {
		// with transaction
		tx, err := conn.databases[database].BeginTx(context.Background(), &sql.TxOptions{Isolation: sql.LevelSerializable})
		if err != nil {
			return "", err
		}

		var execErr error
		result, execErr = tx.ExecContext(context.Background(), query, parameters...)
		if execErr != nil {
			if rollbackErr := tx.Rollback(); rollbackErr != nil {
				err = errors.New(fmt.Sprint("update failed: %v, unable to rollback: %v\n", execErr, rollbackErr))
				return "", err
			}

			err = errors.New(fmt.Sprint("update failed: %v", execErr))
			return "", err
		}
		if err := tx.Commit(); err != nil {
			return "", err
		}
	} else {
		// without transaction
		var err error
		result, err = conn.databases[database].ExecContext(context.Background(), query, parameters...)
		if err != nil {
			return "", err
		}
	}

	// So here I will stream affected row if there one.
	numRowsAffected, err := result.RowsAffected()
	if err != nil {
		return "", err
	}

	// I will send back the last id and the number of affected rows to the caller.
	lastId, _ := result.LastInsertId()

	return fmt.Sprintf("{\"lastId\": %d, \"rowsAffected\": %d}", lastId, numRowsAffected), nil
}

func (store *SqlStore) QueryContext(connectionId string, database string, query string, parameters_ string) (string, error) {

	if len(connectionId) == 0 {
		return "", errors.New("the connection id is required")
	}

	if len(database) == 0 {
		return "", errors.New("the database is required")
	}

	conn, exists := store.connections[connectionId]
	if !exists {
		return "", fmt.Errorf("connection with ID %s does not exist", connectionId)
	}

	if conn.databases == nil {
		conn.databases = make(map[string]*sql.DB, 0)
	}

	if conn.databases[database] == nil {
		databasePath := conn.Path + "/" + database + ".db"

		// Create the database.
		db, err := sql.Open("sqlite3", databasePath)
		if err != nil {
			return "", err
		}

		conn.databases[database] = db
	}

	// The list of parameters
	parameters := make([]interface{}, 0)
	if len(parameters_) > 0 {
		err := json.Unmarshal([]byte(parameters_), &parameters)
		if err != nil {
			return "", err
		}
	}

	// Here I the sql works.
	rows, err := conn.databases[database].QueryContext(context.Background(), query, parameters...)

	if err != nil {
		return "", err
	}

	defer rows.Close()

	// First of all I will get the information about columns
	columns, err := rows.Columns()
	if err != nil {
		return "", err
	}

	// The columns type.
	columnsType, err := rows.ColumnTypes()
	if err != nil {
		return "", err
	}

	// In header is not guaranty to contain a column type.
	header := make([]interface{}, len(columns))

	for i := 0; i < len(columnsType); i++ {
		column := columns[i]

		// So here I will extract type information.
		typeInfo := make(map[string]interface{})
		typeInfo["DatabaseTypeName"] = columnsType[i].DatabaseTypeName()
		typeInfo["Name"] = columnsType[i].DatabaseTypeName()

		// If the type is decimal.
		precision, scale, isDecimal := columnsType[i].DecimalSize()
		if isDecimal {
			typeInfo["Scale"] = scale
			typeInfo["Precision"] = precision
		}

		length, hasLength := columnsType[i].Length()
		if hasLength {
			typeInfo["Precision"] = length
		}

		isNull, isNullable := columnsType[i].Nullable()
		typeInfo["IsNullable"] = isNullable
		if isNullable {
			typeInfo["IsNull"] = isNull
		}

		header[i] = map[string]interface{}{"name": column, "typeInfo": typeInfo}
	}

	count := len(columns)
	values := make([]interface{}, count)
	scanArgs := make([]interface{}, count)
	for i := range values {
		scanArgs[i] = &values[i]
	}

	// Here I will get the rows.
	rows_ := make([]interface{}, 0)

	for rows.Next() {
		row := make([]interface{}, count)
		err := rows.Scan(scanArgs...)
		if err != nil {
			return "", err
		}

		for i, v := range values {
			// So here I will convert the values to Number, Boolean or String
			if v == nil {
				row[i] = nil // NULL value.
			} else {
				if Utility.IsNumeric(v) {
					row[i] = Utility.ToNumeric(v)
				} else if Utility.IsBool(v) {
					row[i] = Utility.ToBool(v)
				} else {
					str := Utility.ToString(v)
					row[i] = str
				}
			}
		}

		rows_ = append(rows_, row)
	}

	result := make(map[string]interface{}, 0)
	result["header"] = header
	result["data"] = rows_
	// I will send back the result to the caller.
	result_, _ := Utility.ToJson(result)
	return result_, nil
}

func (store *SqlStore) Disconnect(connectionId string) error {
	// Check if the connection exists
	_, exists := store.connections[connectionId]
	if !exists {
		return fmt.Errorf("connection with ID %s does not exist", connectionId)
	}

	// Close the database connection
	for _, db := range store.connections[connectionId].databases {
		err := db.Close()
		if err != nil {
			return err
		}
	}

	// Remove the connection from the map
	delete(store.connections, connectionId)

	fmt.Println("Disconnected from SQL server", connectionId)
	return nil
}

func (store *SqlStore) Ping(ctx context.Context, connectionId string) error {

	/** Nothing here **/
	return nil
}

// Create the database.
func (store *SqlStore) CreateDatabase(ctx context.Context, connectionId string, name string) error {
	return errors.New("not implemented") /** Not implemented */
}

// Delete the database.
func (store *SqlStore) DeleteDatabase(ctx context.Context, connectionId string, db string) error {

	var databasePath string
	if len(db) == 0 {
		return errors.New("the database name is required")
	} else {
		databasePath = store.connections[connectionId].Path + "/" + db + ".db"
	}

	fmt.Println("Delete database files: ", databasePath)

	return os.RemoveAll(databasePath)
}

func (store *SqlStore) Count(ctx context.Context, connectionId string, db string, table string, query string, options string) (int64, error) {

	if len(query) == 0 || query == "{}" {
		query = fmt.Sprintf("SELECT * FROM %s", table)
	} else if strings.HasPrefix(query, "{") && strings.HasSuffix(query, "}") {
		var err error
		query, err = store.formatQuery(table, query)
		if err != nil {
			return 0, err
		}
	}

	str, err := store.QueryContext(connectionId, db, query, "[]")
	if err != nil {
		return 0, err
	}

	data := make(map[string]interface{}, 0)
	err = json.Unmarshal([]byte(str), &data)
	if err != nil {
		fmt.Print("Error unmarshalling result: ", str, err)
		return 0, err
	}

	count := int64(len(data["data"].([]interface{})))
	return count, nil
}

func (store *SqlStore) isTableExist(connectionId string, db string, table string) bool {
	// Query to check if the table exists
	query := fmt.Sprintf("SELECT name FROM sqlite_master WHERE type='table' AND name='%s'", table)

	// Execute the query
	str, err := store.QueryContext(connectionId, db, query, "[]")
	if err != nil {
		return false
	}

	result := make(map[string]interface{}, 0)
	err = json.Unmarshal([]byte(str), &result)
	if err != nil {
		return false
	}

	return len(result["data"].([]interface{})) > 0
}

/**
 * Generate the SQL query to create the main table and the array tables.
 */
func generateCreateTableSQL(tableName string, columns map[string]interface{}) (string, []string) {
	var columnsSQL []string
	var arrayTables []string

	for columnName, columnType := range columns {

		if columnType != nil {
			// Determine the SQL data type based on the Go data type
			sqlType := getSQLType(reflect.TypeOf(columnType))

			// Check if the column is an array (slice)
			isArray := reflect.Slice == reflect.TypeOf(columnType).Kind()

			if !isArray {

				if columnName != "typeName" {
					// This is not an array column, include it in the main table
					if columnName == "id" {
						// rename it to _id
						columnName = "_id"
					}

					if columnName != "_id" {
						// Format column names with special characters in double quotes
						columnNameFormatted := fmt.Sprintf("\"%s\"", columnName)

						// Add the column to the main table
						columnsSQL = append(columnsSQL, fmt.Sprintf("%s %s", columnNameFormatted, sqlType))
					}
				}
			}
		}
	}

	createTableSQL := fmt.Sprintf("CREATE TABLE IF NOT EXISTS \"%s\" (_id TEXT PRIMARY KEY, %s);", tableName, strings.Join(columnsSQL, ", "))

	return createTableSQL, arrayTables
}

func getSQLType(goType reflect.Type) string {
	if goType == nil {
		return ""
	}

	switch goType.Kind() {
	case reflect.Int, reflect.Int32, reflect.Int64:
		return "INTEGER"
	case reflect.Float32, reflect.Float64:
		return "REAL"
	case reflect.String:
		return "TEXT"
	default:
		//fmt.Println("unsupported data type: %s", goType.String())
		return ""
	}
}

/**
 * Generate the SQL query to insert data into the main table.
 */
func generateMainInsertSQL(tableName string, data map[string]interface{}) (string, []interface{}) {
	var mainColumns []string
	var mainPlaceholders []string
	var mainValues []interface{}

	for columnName, columnValue := range data {

		// Check if the column is an array (slice)
		if columnValue != nil {
			isArray := reflect.Slice == reflect.TypeOf(columnValue).Kind()

			if !isArray && columnName != "typeName" {
				if columnName == "id" {
					// rename it to _id
					columnName = "_id"
				}

				// This is not an array column, include it in the main table insert
				mainColumns = append(mainColumns, fmt.Sprintf("\"%s\"", columnName))
				mainPlaceholders = append(mainPlaceholders, "?")
				mainValues = append(mainValues, columnValue)
			}
		}
	}

	mainInsertSQL := fmt.Sprintf("INSERT INTO \"%s\" (%s) VALUES (%s);", tableName, strings.Join(mainColumns, ", "), strings.Join(mainPlaceholders, ", "))

	return mainInsertSQL, mainValues
}

/**
 * Insert data into the database. This function will insert the data into the main table and the array tables.
 */
func (store *SqlStore) insertData(connectionId string, db string, tableName string, data map[string]interface{}) (map[string]interface{}, error) {

	var id string

	if data["id"] != nil {
		id = data["id"].(string)
	} else if data["_id"] != nil {
		id = data["_id"].(string)
	}

	if len(id) == 0 {
		fmt.Println("the id is required to insert data into the database", data)
		return nil, errors.New("the id is required to insert data into the database")
	}

	// test if the data already exist.
	if store.isTableExist(connectionId, db, tableName) {
		// I will check if the data already exist.
		query := fmt.Sprintf("SELECT * FROM %s WHERE id='%s'", tableName, id)
		values, err := store.FindOne(context.Background(), connectionId, db, tableName, query, "")
		if err == nil {
			return values.(map[string]interface{}), nil
		}
	}

	// Insert data into the main table
	insertSQL, values := generateMainInsertSQL(tableName, data)
	str, err := store.ExecContext(connectionId, db, insertSQL, values, 0)
	if err != nil {
		fmt.Printf("error inserting data into %s table with error: %s", tableName, err.Error())
		return nil, err
	}

	result := make(map[string]interface{}, 0)
	err = json.Unmarshal([]byte(str), &result)
	if err != nil {
		return nil, err
	}

	// Insert data into the array tables
	for columnName, columnValue := range data {
		// Check if the column is an array (slice)
		if columnName != "typeName" {
			if columnValue != nil {
				if reflect.Slice == reflect.TypeOf(columnValue).Kind() {
					// This is an array column, insert the values into the array table
					arrayTableName := tableName + "_" + columnName
					sliceValue := reflect.ValueOf(data[columnName])
					length := sliceValue.Len()
					for i := 0; i < length; i++ {
						element := sliceValue.Index(i)

						// Insert the values into the array table
						arrayInsertSQL := fmt.Sprintf("INSERT INTO %s (value, %s_id) VALUES (?, ?);", arrayTableName, tableName)

						parameters := make([]interface{}, 0)
						switch element.Interface().(type) {
						case int:
							intValue := element.Interface().(int)
							parameters = append(parameters, intValue)
						case float64:
							floatValue := element.Interface().(float64)
							parameters = append(parameters, floatValue)
						case string:
							stringValue := element.Interface().(string)
							parameters = append(parameters, stringValue)
						case map[string]interface{}:

							entity := element.Interface().(map[string]interface{})

							// So here I will insert the entity into the database.
							// I will get the entity type.
							// ** only if the entity has a typeName property.
							if entity["typeName"] != nil {
								typeName := entity["typeName"].(string)

								// set the domain in case is define with localhost value.
								localDomain, _ := config.GetDomain()
								if entity["domain"] == nil {
									entity["domain"] = localDomain
								} else if entity["domain"] == "localhost" {
									entity["domain"] = localDomain
								}

								// I will get the entity type.
								var err error
								entity, err = store.insertData(connectionId, db, typeName+"s", entity)
								if err != nil {
									fmt.Printf("error inserting data into %s table with error: %s", typeName+"s", err.Error())
								}

								// I will get the entity id.
								_id := Utility.ToInt(entity["_id"])
								sourceCollection := tableName
								targetCollection := typeName + "s"
								field := columnName

								// He I will create the reference table.
								// I will create the table if not already exist.
								createTableSQL := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS `+sourceCollection+`_`+field+` (source_id TEXT, target_id TEXT, FOREIGN KEY (source_id) REFERENCES %s(_id) ON DELETE CASCADE, FOREIGN KEY (target_id) REFERENCES %s(_id) ON DELETE CASCADE)`, sourceCollection, targetCollection)
								_, err = store.ExecContext("local_resource", db, createTableSQL, nil, 0)
								if err == nil {
									fmt.Println("Table created: ", sourceCollection+"_"+field)
								} else {
									fmt.Printf("error creating table: %s with error %s", sourceCollection+"_"+field, err.Error())
								}

								// I will insert the reference into the table.
								insertSQL := fmt.Sprintf("INSERT INTO " + sourceCollection + "_" + field + " (source_id, target_id) VALUES (?, ?);")
								parameters := make([]interface{}, 0)
								parameters = append(parameters, id)
								parameters = append(parameters, _id)
								_, err = store.ExecContext("local_resource", db, insertSQL, parameters, 0)
								if err != nil {
									fmt.Printf("error inserting data into %s table with error: %s", sourceCollection+"_"+field+"s", err.Error())
								}

							} else if entity["$ref"] != nil {

								// I will get the entity id.

								sourceCollection := tableName
								targetCollection := entity["$ref"].(string)
								_id := entity["$id"].(string)

								field := columnName
								// He I will create the reference table.
								// I will create the table if not already exist.
								createTableSQL := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS `+sourceCollection+`_`+field+` (source_id TEXT, target_id TEXT, FOREIGN KEY (source_id) REFERENCES %s(_id) ON DELETE CASCADE, FOREIGN KEY (target_id) REFERENCES %s(_id) ON DELETE CASCADE)`, sourceCollection, targetCollection)
								fmt.Println("createTableSQL: ", createTableSQL)

								_, err = store.ExecContext("local_resource", db, createTableSQL, nil, 0)
								if err == nil {
									fmt.Println("Table created: ", sourceCollection+"_"+field)
								} else {
									fmt.Printf("error creating table: %s with error %s", sourceCollection+"_"+field, err.Error())
								}

								// I will insert the reference into the table.
								insertSQL := fmt.Sprintf("INSERT INTO " + sourceCollection + "_" + field + " (source_id, target_id) VALUES (?, ?);")
								parameters := make([]interface{}, 0)
								parameters = append(parameters, id)
								parameters = append(parameters, _id)
								_, err = store.ExecContext("local_resource", db, insertSQL, parameters, 0)
								if err != nil {
									fmt.Printf("error inserting data into %s table with error: %s", sourceCollection+"_"+field+"s", err.Error())
								}
							}

						default:
							fmt.Printf("index %d: Unknown Type %s \n", i, columnName)
						}

						// append the object id...
						if len(parameters) > 0 {

							// Create the table if it does not exist.
							if !store.isTableExist(connectionId, db, arrayTableName) {
								createTableSQL := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (value %s, %s_id TEXT, FOREIGN KEY (%s_id) REFERENCES %s(_id) ON DELETE CASCADE)", arrayTableName, getSQLType(reflect.TypeOf(columnValue)), tableName, tableName, tableName)
								_, err := store.ExecContext(connectionId, db, createTableSQL, nil, 0)
								if err != nil {
									return nil, err
								}
							}

							parameters = append(parameters, id)
							_, err := store.ExecContext(connectionId, db, arrayInsertSQL, parameters, 0)
							if err != nil {
								fmt.Printf("error inserting data into %s table with error: %s", arrayTableName, err.Error())
								return nil, err
							}
						}
					}
				} else if reflect.Map == reflect.TypeOf(columnValue).Kind() {

					entity := columnValue.(map[string]interface{})

					// So here I will insert the entity into the database.
					// I will get the entity type.
					// ** only if the entity has a typeName property.
					if entity["typeName"] != nil {
						typeName := entity["typeName"].(string)

						if entity["domain"] == nil {
							localDomain, _ := config.GetDomain()
							entity["domain"] = localDomain
						}

						// I will get the entity type.
						var err error
						entity, err = store.insertData(connectionId, db, typeName+"s", entity)
						if err != nil {
							fmt.Printf("error inserting data into %s table with error: %s", typeName+"s", err.Error())
						}

						// I will get the entity id.
						_id := Utility.ToInt(entity["_id"])
						sourceCollection := tableName
						targetCollection := typeName + "s"
						field := columnName

						// He I will create the reference table.
						// I will create the table if not already exist.
						createTableSQL := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS `+sourceCollection+`_`+field+` (source_id TEXT, target_id TEXT, FOREIGN KEY (source_id) REFERENCES %s(_id) ON DELETE CASCADE, FOREIGN KEY (target_id) REFERENCES %s(_id) ON DELETE CASCADE)`, sourceCollection, targetCollection)
						_, err = store.ExecContext("local_resource", db, createTableSQL, nil, 0)
						if err == nil {
							fmt.Println("Table created: ", sourceCollection+"_"+field)
						} else {
							fmt.Printf("error creating table: %s with error %s ", sourceCollection+"_"+field, err.Error())
						}

						// I will insert the reference into the table.
						insertSQL := fmt.Sprintf("INSERT INTO " + sourceCollection + "_" + field + " (source_id, target_id) VALUES (?, ?);")
						parameters := make([]interface{}, 0)
						parameters = append(parameters, id)
						parameters = append(parameters, _id)
						_, err = store.ExecContext("local_resource", db, insertSQL, parameters, 0)
						if err != nil {
							fmt.Printf("error inserting data into %s table with error: %s", sourceCollection+"_"+field+"s", err.Error())
						}

					} else if entity["$ref"] != nil {

						// I will get the entity id.
						sourceCollection := tableName
						targetCollection := entity["$ref"].(string)
						_id := entity["$id"].(string)

						field := columnName
						// He I will create the reference table.
						// I will create the table if not already exist.
						createTableSQL := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS `+sourceCollection+`_`+field+` (source_id TEXT, target_id TEXT, FOREIGN KEY (source_id) REFERENCES %s(_id) ON DELETE CASCADE, FOREIGN KEY (target_id) REFERENCES %s(_id) ON DELETE CASCADE)`, sourceCollection, targetCollection)
						_, err = store.ExecContext("local_resource", db, createTableSQL, nil, 0)
						if err == nil {
							fmt.Println("Table created: ", sourceCollection+"_"+field)
						} else {
							fmt.Printf("error creating table: %s with error %s", sourceCollection+"_"+field, err)
						}

						// I will insert the reference into the table.
						insertSQL := fmt.Sprintf("INSERT INTO " + sourceCollection + "_" + field + " (source_id, target_id) VALUES (?, ?);")
						parameters := make([]interface{}, 0)
						parameters = append(parameters, id)
						parameters = append(parameters, _id)
						_, err = store.ExecContext("local_resource", db, insertSQL, parameters, 0)
						if err != nil {
							fmt.Printf("error inserting data into %s table with error: %s", sourceCollection+"_"+field+"s", err.Error())
						}
					}
				}
			}
		}
	}

	return data, nil
}

func (store *SqlStore) InsertOne(ctx context.Context, connectionId string, db string, table string, entity interface{}, options string) (interface{}, error) {

	entity_, err := Utility.ToMap(entity)
	if err != nil {
		return nil, err
	}

	if !store.isTableExist(connectionId, db, table) {

		// Create the table
		createTableSQL, arrayTableSQL := generateCreateTableSQL(table, entity_)

		_, err = store.ExecContext(connectionId, db, createTableSQL, nil, 0)
		if err != nil {
			return nil, err
		}

		for _, sql := range arrayTableSQL {
			_, err := store.ExecContext(connectionId, db, sql, nil, 0)
			if err != nil {
				return nil, err
			}
		}
	}

	result, err := store.insertData(connectionId, db, table, entity_)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (store *SqlStore) InsertMany(ctx context.Context, connectionId string, db string, table string, entities []interface{}, options string) ([]interface{}, error) {

	if !store.isTableExist(connectionId, db, table) {

		entity_, err := Utility.ToMap(entities[0])
		if err != nil {
			return nil, err
		}

		// Create the table
		createTableSQL, arrayTableSQL := generateCreateTableSQL(table, entity_)
		_, err = store.ExecContext(connectionId, db, createTableSQL, nil, 0)
		if err != nil {
			return nil, err
		}

		for _, sql := range arrayTableSQL {
			_, err := store.ExecContext(connectionId, db, sql, nil, 0)
			if err != nil {
				return nil, err
			}
		}
	}

	var results []interface{}
	for _, entity := range entities {

		entity_, err := Utility.ToMap(entity)
		if err != nil {
			return nil, err
		}

		result, err := store.insertData(connectionId, db, table, entity_)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}

	return results, nil
}

// Helper function to check if the SQL type is INTEGER
func isIntegerType(sqlType string) bool {
	return sqlType == "INTEGER"
}

// Helper function to cast value to int32 if it's INTEGER
func getIntValue(value interface{}) (int32, error) {
	v := Utility.ToInt(value)

	return int32(v), nil
}

// RecreateArrayOfObjects recreates an array of objects from the given data and header.
func (store *SqlStore) recreateArrayOfObjects(connectionId, db, tableName string, dataHeader map[string]interface{}, options []map[string]interface{}) ([]interface{}, error) {
	data := dataHeader["data"]
	header := dataHeader["header"]

	// Create a slice to hold the recreated objects
	var objects []interface{}

	// Get the projection option
	var projection map[string]interface{}
	if len(options) > 0 {
		for _, option := range options {
			if option["Projection"] != nil {
				projection = option["Projection"].(map[string]interface{})
				projection["_id"] = 1 // set the _id
			}
		}
	}

	// Get the header as a slice of maps
	for _, dataRow := range data.([]interface{}) {
		dataRow := dataRow.([]interface{})
		object := make(map[string]interface{}, 0)
		object["typeName"] = tableName // keep typename infos...

		for index, fieldInfos := range header.([]interface{}) {
			fieldInfos := fieldInfos.(map[string]interface{})
			fieldName := fieldInfos["name"].(string)
			value := dataRow[index]
			typeInfos := fieldInfos["typeInfo"].(map[string]interface{})

			if projection != nil {
				if projection[fieldName] != nil {
					if isIntegerType(typeInfos["DatabaseTypeName"].(string)) {
						intValue, err := getIntValue(value)
						if err != nil {
							return nil, err
						}
						object[fieldName] = intValue
					} else {
						object[fieldName] = value
					}
				}
			} else {
				if isIntegerType(typeInfos["DatabaseTypeName"].(string)) {
					intValue, err := getIntValue(value)
					if err != nil {
						return nil, err
					}
					object[fieldName] = intValue
				} else {
					object[fieldName] = value
				}
			}
		}

		objects = append(objects, object)

		// Now I will check if there is any array in the object.

		// I will get the list of tables and test if there table with name starting with table name.
		// If there is, I will get the data from the table and add it to the object.
		// Query to retrieve table names
		query := "SELECT name FROM sqlite_master WHERE type='table'"

		// Execute the query
		str, err := store.QueryContext(connectionId, db, query, "[]")
		if err != nil {
			fmt.Printf("error executing query: %s with error %s", query, err)
			return nil, err
		}

		tables := make(map[string]interface{}, 0)
		err = json.Unmarshal([]byte(str), &tables)
		if err != nil {
			fmt.Print("Error unmarshalling result: ", str, err)
			return nil, err
		}

		// Get the domain.
		domain, _ := config.GetDomain()

		// Loop through the tables
		for _, values := range tables["data"].([]interface{}) {

			for _, value := range values.([]interface{}) {

				field := strings.Replace(value.(string), tableName+"_", "", 1)

				if strings.HasPrefix(value.(string), tableName+"_") && object[field] == nil {

					// Query to retrieve the data from the array table
					query := fmt.Sprintf("SELECT value FROM %s WHERE %s=?", value.(string), tableName+"_id")

					parameters := make([]interface{}, 0)
					parameters = append(parameters, object["_id"]) // append the object id...
					parameters_, _ := Utility.ToJson(parameters)

					// Execute the query
					str, err := store.QueryContext(connectionId, db, query, parameters_)
					if err == nil {

						data := make(map[string]interface{}, 0)
						err = json.Unmarshal([]byte(str), &data)
						if err != nil {
							return nil, err
						}

						object[field] = make([]interface{}, 0)
						for _, values := range data["data"].([]interface{}) {
							value := values.([]interface{})[0] // the value is the second element of the array.
							object[field] = append(object[field].([]interface{}), value)
						}

					} else {

						// Query to retrieve the data from the array table
						query := fmt.Sprintf("SELECT * FROM %s WHERE source_id=?", value.(string))
						parameters := make([]interface{}, 0)
						parameters = append(parameters, object["_id"]) // append the object id...
						parameters_, _ := Utility.ToJson(parameters)

						// Execute the query
						str, err := store.QueryContext(connectionId, db, query, parameters_)
						if err == nil {

							data := make(map[string]interface{}, 0)
							err = json.Unmarshal([]byte(str), &data)
							if err == nil {

								// I will create the array.
								for _, values := range data["data"].([]interface{}) {

									ref_id := Utility.ToString(values.([]interface{})[1]) // the value is the second element of the array.

									if strings.Contains(ref_id, "@") {
										// Only if the domain is the same.
										ref_id = strings.Split(ref_id, "@")[0]
										if strings.Split(ref_id, "@")[0] != domain {
											continue
										}
									}

									// The type name will be the field name with the first letter in upper case.
									bytes := []byte(field)
									bytes[0] = byte(unicode.ToUpper(rune(bytes[0])))
									typeName := string(bytes)

									// Here a little exception... I will replace Members by Accounts, because the table name is Accounts.
									if typeName == "Members" {
										typeName = "Accounts"
									}

									if object[field] == nil {
										object[field] = make([]interface{}, 0)
									}

									object[field] = append(object[field].([]interface{}), map[string]interface{}{"$ref": typeName, "$id": ref_id, "$db": db})
								}
							}
						}
					}
				}
			}
		}
	}

	return objects, nil
}

func (store *SqlStore) formatQuery(table, query string) (string, error) {
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

			if key == "_id" {
				if strings.Contains(value.(string), "@") {
					value = strings.Split(value.(string), "@")[0]
				}
			}

			if reflect.TypeOf(value).Kind() == reflect.String {
				query += fmt.Sprintf("%s = '%v' AND ", key, value)
			} else if reflect.TypeOf(value).Kind() == reflect.Slice {
				if key == "$and" || key == "$or" {
					query += store.getParameters(key, value.([]interface{}))
				}
			} else if reflect.TypeOf(value).Kind() == reflect.Map {
				// is not really a regex but is the only way to do it.
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

func (store *SqlStore) FindOne(ctx context.Context, connectionId string, database string, table string, query string, options string) (interface{}, error) {

	if len(query) == 0 {
		return nil, errors.New("query is empty")
	} else if strings.HasPrefix(query, "{") && strings.HasSuffix(query, "}") {
		var err error
		query, err = store.formatQuery(table, query)
		if err != nil {
			return nil, err
		}
	}

	str, err := store.QueryContext(connectionId, database, query, "[]")
	if err != nil {
		return nil, err
	}

	data := make(map[string]interface{}, 0)
	err = json.Unmarshal([]byte(str), &data)

	if err != nil {
		fmt.Print("Error unmarshalling result: ", str, err)
		return nil, err
	}

	options_ := make([]map[string]interface{}, 0)
	if len(options) > 0 {
		err = json.Unmarshal([]byte(options), &options_)
		if err != nil {
			return nil, err
		}
	}

	objects, err := store.recreateArrayOfObjects(connectionId, database, table, data, options_)
	if err != nil {
		return nil, err
	}

	if len(objects) > 0 {
		return objects[0], nil
	}

	return nil, errors.New("not found")
}

func (store *SqlStore) getParameters(condition string, values []interface{}) string {
	query := ""

	if condition == "$and" {
		query += "("
		for _, v := range values {
			value := v.(map[string]interface{})
			for key, v := range value {
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

func (store *SqlStore) Find(ctx context.Context, connectionId string, db string, table string, query string, options string) ([]interface{}, error) {

	if len(query) == 0 || query == "{}" {
		query = fmt.Sprintf("SELECT * FROM %s", table)
	} else if strings.HasPrefix(query, "{") && strings.HasSuffix(query, "}") {
		var err error
		query, err = store.formatQuery(table, query)
		if err != nil {
			return nil, err
		}
	}

	str, err := store.QueryContext(connectionId, db, query, "[]")
	if err != nil {
		fmt.Printf("error executing query: %s with error %s ", query, err.Error())
		return nil, err
	}

	data := make(map[string]interface{}, 0)
	err = json.Unmarshal([]byte(str), &data)
	if err != nil {
		fmt.Print("Error unmarshalling result: ", str, err)
		return nil, err
	}

	options_ := make([]map[string]interface{}, 0)
	if len(options) > 0 {
		err = json.Unmarshal([]byte(options), &options_)
		if err != nil {
			return nil, err
		}
	}

	objects, err := store.recreateArrayOfObjects(connectionId, db, table, data, options_)
	if err != nil {
		fmt.Printf("error recreating array of objects %s", err)
		return nil, err
	}

	if len(objects) > 0 {
		return objects, nil
	}

	return []interface{}{}, nil
}

func (store *SqlStore) ReplaceOne(ctx context.Context, connectionId string, db string, table string, query string, value string, options string) error {

	// insert the new entry.
	entity := make(map[string]interface{}, 0)
	err := json.Unmarshal([]byte(value), &entity)
	if err != nil {
		fmt.Printf("error unmarshalling %s", err)
		return err
	}

	if !store.isTableExist(connectionId, db, table) {

		// Create the table
		createTableSQL, arrayTableSQL := generateCreateTableSQL(table, entity)
		_, err := store.ExecContext(connectionId, db, createTableSQL, nil, 0)
		if err != nil {
			fmt.Printf("error creating table: %s with error %s ", table, err.Error())
			return err
		}

		// Create the array tables, like list of strings etc...
		for _, sql := range arrayTableSQL {
			_, err := store.ExecContext(connectionId, db, sql, nil, 0)
			if err != nil {
				fmt.Printf("error creating table: %s with error %s", table, err.Error())
				return err
			}
		}
	}

	// Parse the query to check if it is a valid JSON.
	if strings.HasPrefix(query, "{") && strings.HasSuffix(query, "}") {
		var err error
		query, err = store.formatQuery(table, query)
		if err != nil {
			fmt.Printf("error formatting query: %s with error %s", query, err.Error())
			return err
		}
	}

	// delete entry if it exist.
	store.deleteOneSqlEntry(connectionId, db, table, query)

	_, err = store.insertData(connectionId, db, table, entity)
	if err != nil {
		fmt.Printf("error inserting data into %s table with error: %s", table, err.Error())
		return err
	}

	return nil
}

func generateUpdateTableQuery(tableName string, fields []interface{}, whereClause string) (string, error) {

	// Build the SQL query
	updateQuery := "UPDATE " + tableName + " SET "
	for i, field := range fields {
		updateQuery += field.(string) + " = ?"
		if i < len(fields)-1 {
			updateQuery += ", "
		}
	}

	if whereClause != "" {

		if strings.Contains(whereClause, "WHERE") {
			whereClause = strings.Split(whereClause, "WHERE")[1]
		}

		updateQuery += " WHERE " + whereClause

	}

	return updateQuery, nil
}

func (store *SqlStore) Update(ctx context.Context, connectionId string, db string, table string, query string, value string, options string) error {

	values_ := make(map[string]interface{}, 0)
	err := json.Unmarshal([]byte(value), &values_)
	if err != nil {
		return err
	}

	if values_["$set"] == nil {
		return errors.New("no $set operator allowed in Update")
	}

	query, err = store.formatQuery(table, query)
	if err != nil {
		return err
	}

	// Here I will retreive the fiedls
	fields := make([]interface{}, 0)
	values := make([]interface{}, 0)

	for key, value := range values_["$set"].(map[string]interface{}) {
		fields = append(fields, key)
		values = append(values, value)
	}

	q, err := generateUpdateTableQuery(table, fields, query)
	if err != nil {
		return err
	}

	_, err = store.ExecContext(connectionId, db, q, values, 0)
	if err != nil {
		return err
	}

	return nil
}

func (store *SqlStore) UpdateOne(ctx context.Context, connectionId string, db string, table string, query string, value string, options string) error {

	values_ := make(map[string]interface{}, 0)
	err := json.Unmarshal([]byte(value), &values_)
	if err != nil {
		fmt.Printf("error unmarshalling entity values with error: %s", err.Error())
		return err
	}

	if values_["$set"] == nil {
		return errors.New("no $set operator allowed in UpdateOne")
	}

	query, err = store.formatQuery(table, query)
	if err != nil {
		return err
	}

	// Here I will retreive the current entity.
	_, err = store.FindOne(context.Background(), connectionId, db, table, query, "")
	if err != nil {
		return err
	}

	// Here I will retreive the fiedls
	fields := make([]interface{}, 0)
	values := make([]interface{}, 0)

	for key, value := range values_["$set"].(map[string]interface{}) {
		fields = append(fields, key)
		values = append(values, value)
	}

	q, err := generateUpdateTableQuery(table, fields, query)

	if err != nil {
		return err
	}

	_, err = store.ExecContext(connectionId, db, q, values, 0)
	if err != nil {
		return err
	}

	return nil
}

func (store *SqlStore) deleteSqlEntries(connectionId string, db string, table string, query string) error {
	entities, err := store.Find(context.Background(), connectionId, db, table, query, "")
	if err != nil {
		return err
	}

	for _, entity := range entities {
		if entity.(map[string]interface{})["_id"] != nil {
			query := fmt.Sprintf("SELECT * FROM '%s' WHERE _id='%s'", table, entity.(map[string]interface{})["_id"].(string))
			// Execute the query
			err := store.deleteOneSqlEntry(connectionId, db, table, query)
			if err != nil {
				fmt.Println(query, err)
			}
		}
	}

	return nil
}

func (store *SqlStore) deleteOneSqlEntry(connectionId string, db string, table string, query string) error {

	// I will retreive the entity with the query.
	entity, err := store.FindOne(context.Background(), connectionId, db, table, query, "")
	if err != nil {
		return err
	}

	if strings.HasPrefix(query, "{") && strings.HasSuffix(query, "}") {
		var err error
		query, err = store.formatQuery(table, query)
		if err != nil {
			return err
		}
	}

	// Here I will delete the entity from the database.
	query = strings.Replace(query, "SELECT *", "DELETE", 1)
	_, err = store.ExecContext(connectionId, db, query, nil, 0)
	if err != nil {
		return err
	}

	// Now I will delete the data from the array tables.
	for columnName, columnValue := range entity.(map[string]interface{}) {
		// Check if the column is an array (slice)
		if columnName != "typeName" {
			if columnValue != nil {
				isArray := reflect.Slice == reflect.TypeOf(columnValue).Kind()
				if isArray {

					// This is an array column, insert the values into the array table
					arrayTableName := table + "_" + columnName

					// I will delete the data from the array table.
					query := fmt.Sprintf("DELETE FROM %s WHERE %s_id=?", arrayTableName, table)

					parameters := make([]interface{}, 0)

					if entity.(map[string]interface{})["_id"] != nil {
						parameters = append(parameters, entity.(map[string]interface{})["_id"]) // append the object id...
					} else if entity.(map[string]interface{})["$id"] != nil {
						parameters = append(parameters, entity.(map[string]interface{})["$id"]) // append the object id...
					}

					// Execute the query
					_, err := store.ExecContext(connectionId, db, query, parameters, 0)
					if err != nil {

						query := fmt.Sprintf("DELETE FROM %s WHERE source_id=?", arrayTableName)

						parameters := make([]interface{}, 0)
						parameters = append(parameters, entity.(map[string]interface{})["_id"]) // append the object id...

						// Execute the query
						_, err := store.ExecContext(connectionId, db, query, parameters, 0)
						if err != nil {
							fmt.Printf("error deleting data from array table: %s with error %s", arrayTableName, err.Error())

							return err
						}
					}
				}
			}
		}
	}

	return nil
}

func (store *SqlStore) Delete(ctx context.Context, connectionId string, db string, table string, query string, options string) error {
	return store.deleteSqlEntries(connectionId, db, table, query)
}

func (store *SqlStore) DeleteOne(ctx context.Context, connectionId string, db string, table string, query string, options string) error {
	return store.deleteOneSqlEntry(connectionId, db, table, query)
}

func (store *SqlStore) Aggregate(ctx context.Context, connectionId string, keyspace string, table string, pipeline string, optionsStr string) ([]interface{}, error) {

	return nil, errors.New("not implemented")
}

func (store *SqlStore) CreateTable(ctx context.Context, connectionId string, db string, table string, fields []string) error {

	// Create the table
	createTableSQL := fmt.Sprintf("CREATE TABLE IF NOT EXISTS \"%s\" (_id TEXT PRIMARY KEY, %s);", table, strings.Join(fields, ", "))
	_, err := store.ExecContext(connectionId, db, createTableSQL, nil, 0)
	if err != nil {
		return err
	}

	return nil
}

func (store *SqlStore) CreateCollection(ctx context.Context, connectionId string, database string, name string, optionsStr string) error {

	return errors.New("not implemented")
}

func (store *SqlStore) DeleteCollection(ctx context.Context, connectionId string, database string, collection string) error {

	// I will delete the table.
	query := fmt.Sprintf("DROP TABLE IF EXISTS %s", collection)
	_, err := store.ExecContext(connectionId, database, query, nil, 0)
	if err != nil {
		return err
	}

	// I will delete the reference tables.
	query = fmt.Sprintf("SELECT name FROM sqlite_master WHERE type='table'")
	str, err := store.QueryContext(connectionId, database, query, "[]")
	if err != nil {
		return err
	}

	data := make(map[string]interface{}, 0)
	err = json.Unmarshal([]byte(str), &data)
	if err != nil {
		return err
	}

	// Loop through the tables
	for _, values := range data["data"].([]interface{}) {
		for _, value := range values.([]interface{}) {
			if strings.HasPrefix(value.(string), collection+"_") {
				// I will delete the table.
				query := fmt.Sprintf("DROP TABLE IF EXISTS %s", value.(string))
				_, err := store.ExecContext(connectionId, database, query, nil, 0)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func (store *SqlStore) RunAdminCmd(ctx context.Context, connectionId string, user string, password string, script string) error {
	fmt.Println("RunAdminCmd ", connectionId, user, password, script)
	// TODO: I will need to parse the script and execute the command.
	return nil
}
