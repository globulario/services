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

	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/authentication/authentication_client"
	"github.com/globulario/services/golang/config"

	_ "github.com/mattn/go-sqlite3" // Import the sqlite3 driver
)

// Connection represent a connection to a SQL database.
type Connection struct {
	Id       string
	Host     string
	Token    string
	Database string
	Path     string
	db       *sql.DB
}

/**
 * The SQL store.
 */
type SqlStore struct {
	/** The connections */
	connections map[string]Connection
}

func (store *SqlStore) GetStoreType() string {
	return "SQL"
}

// ///////////////////////////////////// Get SQL Client //////////////////////////////////////////
func (store *SqlStore) Connect(id string, host string, port int32, user string, password string, database string, timeout int32, options_str string) error {

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

	// set the path if it is provided.
	if options["path"] != nil {
		path = options["path"].(string)
	}

	// Create the directory if it does not exist.
	Utility.CreateDirIfNotExist(path)

	// Create the connection.
	connection := Connection{
		Id:       id,
		Host:     host,
		Token:    token,
		Database: database,
		Path:     path,
	}

	if store.connections == nil {
		store.connections = make(map[string]Connection, 0)
	}

	// Save the connection.
	store.connections[id] = connection

	// Create the database if it does not exist.
	databasePath := connection.Path + "/" + database + ".db"

	// Create the database.
	db, err := sql.Open("sqlite3", databasePath)
	if err != nil {
		return err
	}

	// keep the database reference.
	connection.db = db

	// Create the table if it does not exist.
	count, _ := store.Count(context.Background(), id, "", "user_data", `SELECT * FROM user_data WHERE _id='`+user+`'`, "")
	if count == 0 && id != "local_resource" {
		_, err := store.InsertOne(context.Background(), id, database, "user_data", map[string]interface{}{"_id": user, "firstName_": "", "lastName_": "", "middleName_": "", "profilePicture_": "", "domain_": "", "email_": ""}, "")
		if err != nil {
			return err
		}
	}

	return nil
}

func (store *SqlStore) ExecContext(connectionId interface{}, query interface{}, parameters_string string, tx_ interface{}) (string, error) {
	// Type assert the connection and query to their respective types
	connID, ok := connectionId.(string)
	if !ok {
		return "", errors.New("connectionId should be of type string")
	}

	conn, exists := store.connections[connID]
	if !exists {
		return "", fmt.Errorf("connection with ID %s does not exist", connID)
	}

	if conn.db == nil {
		databasePath := conn.Path + "/" + conn.Database + ".db"

		// Create the database.
		db, err := sql.Open("sqlite3", databasePath)
		if err != nil {
			return "", err
		}
		conn.db = db
	}

	// The list of parameters
	parameters := make([]interface{}, 0)
	json.Unmarshal([]byte(parameters_string), &parameters)

	hasTx := false
	if tx_ != nil {
		hasTx = tx_.(int) == 1
	}

	// Execute the query here.
	var result sql.Result
	if hasTx {
		// with transaction
		tx, err := conn.db.BeginTx(context.Background(), &sql.TxOptions{Isolation: sql.LevelSerializable})
		if err != nil {
			return "", err
		}

		var execErr error
		result, execErr = tx.ExecContext(context.Background(), query.(string), parameters...)
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
		result, err = conn.db.ExecContext(context.Background(), query.(string), parameters...)
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

func (store *SqlStore) QueryContext(connectionId string, query string, parameters_ string) (string, error) {
	conn, exists := store.connections[connectionId]
	if !exists {
		return "", fmt.Errorf("connection with ID %s does not exist", connectionId)
	}

	if conn.db == nil {
		databasePath := conn.Path + "/" + conn.Database + ".db"

		// Create the database.
		db, err := sql.Open("sqlite3", databasePath)
		if err != nil {
			return "", err
		}
		conn.db = db
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
	rows, err := conn.db.QueryContext(context.Background(), query, parameters...)

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
	if err := store.connections[connectionId].db.Close(); err != nil {
		return err
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
func (store *SqlStore) DeleteDatabase(ctx context.Context, connectionId string, name string) error {

	fmt.Println("Delete database ", connectionId, name)
	var databasePath string

	return os.RemoveAll(databasePath)
}

func (store *SqlStore) Count(ctx context.Context, connectionId string, keyspace string, table string, query string, options string) (int64, error) {
	fmt.Println("Count ", connectionId, keyspace, table, query, options)

	if len(query) == 0 || query == "{}" {
		query = fmt.Sprintf("SELECT * FROM %s", table)
	}

	str, err := store.QueryContext(connectionId, query, "[]")
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

func (store *SqlStore) isTableExist(connectionId string, table string) bool {
	// Query to check if the table exists
	query := fmt.Sprintf("SELECT name FROM sqlite_master WHERE type='table' AND name='%s'", table)

	// Execute the query
	str, err := store.QueryContext(connectionId, query, "[]")
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

				// Format column names with special characters in double quotes
				columnNameFormatted := fmt.Sprintf("\"%s\"", columnName)

				// Add the column to the main table
				columnsSQL = append(columnsSQL, fmt.Sprintf("%s %s", columnNameFormatted, sqlType))
			}
		}
	}

	createTableSQL := fmt.Sprintf("CREATE TABLE IF NOT EXISTS \"%s\" (uid INTEGER PRIMARY KEY AUTOINCREMENT, %s);", tableName, strings.Join(columnsSQL, ", "))

	return createTableSQL, arrayTables
}

func getSQLType(goType reflect.Type) string {
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
func (store *SqlStore) insertData(connectionId string, tableName string, data map[string]interface{}) (map[string]interface{}, error) {

	// test if the data already exist.
	if store.isTableExist(connectionId, tableName) {
		var id string
		if data["id"] != nil {
			id = data["id"].(string)
		} else if data["_id"] != nil {
			id = data["_id"].(string)
		}

		if len(id) > 0 {
			// I will check if the data already exist.
			query := fmt.Sprintf("SELECT * FROM %s WHERE _id='%s'", tableName, id)
			values, err := store.FindOne(context.Background(), connectionId, "", tableName, query, "")
			if err == nil {
				return values.(map[string]interface{}), nil
			}

		}
	}

	// Insert data into the main table
	insertSQL, values := generateMainInsertSQL(tableName, data)
	values_, _ := Utility.ToJson(values)
	str, err := store.ExecContext(connectionId, insertSQL, values_, nil)
	if err != nil {
		fmt.Println("error inserting data into %s table", tableName, err)
		return nil, err
	}

	result := make(map[string]interface{}, 0)
	err = json.Unmarshal([]byte(str), &result)
	if err != nil {
		return nil, err
	}

	// set the id
	data["uid"] = Utility.ToInt(result["lastId"])

	// Insert data into the array tables
	for columnName, columnValue := range data {
		// Check if the column is an array (slice)
		if columnName != "typeName" {
			if columnValue != nil {
				isArray := reflect.Slice == reflect.TypeOf(columnValue).Kind()
				if isArray {

					// This is an array column, insert the values into the array table
					arrayTableName := tableName + "_" + columnName
					sliceValue := reflect.ValueOf(data[columnName])
					length := sliceValue.Len()
					for i := 0; i < length; i++ {
						element := sliceValue.Index(i)

						// Insert the values into the array table
						arrayInsertSQL := fmt.Sprintf("INSERT INTO %s (value, %s_uid) VALUES (?, ?);", arrayTableName, tableName)

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

							// TODO create the entity and insert it into the database....
							entity := element.Interface().(map[string]interface{})

							// So here I will insert the entity into the database.
							// I will get the entity type.
							// ** only if the entity has a typeName property.
							if entity["typeName"] != nil {
								typeName := entity["typeName"].(string)

								// I will get the entity type.
								var err error
								entity, err = store.insertData(connectionId, typeName+"s", entity)
								if err != nil {
									fmt.Println("Error inserting data into array table: ", err)
								}

								// I will get the entity id.
								uid := Utility.ToInt(entity["uid"])
								sourceCollection := tableName
								targetCollection := typeName + "s"
								field := columnName

								// He I will create the reference table.
								// I will create the table if not already exist.
								fmt.Println("------------------------------> create reference table: ", sourceCollection+"_"+field)
								createTableSQL := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS `+sourceCollection+`_`+field+` (source_uid TEXT, target_uid TEXT, FOREIGN KEY (source_uid) REFERENCES %s(uid) ON DELETE CASCADE, FOREIGN KEY (target_uid) REFERENCES %s(uid) ON DELETE CASCADE)`, sourceCollection, targetCollection)
								_, err = store.ExecContext("local_resource", createTableSQL, "[]", nil)
								if err == nil {
									fmt.Println("Table created: ", sourceCollection+"_"+field)
								}else{
									fmt.Println("-------> Error creating table: ", sourceCollection+"_"+field, err)
								}

								

								// I will insert the reference into the table.
								insertSQL := fmt.Sprintf("INSERT INTO " + sourceCollection + "_" + field + " (source_uid, target_uid) VALUES (?, ?);")
								parameters := make([]interface{}, 0)
								parameters = append(parameters, data["uid"])
								parameters = append(parameters, uid)
								parameters_, _ := Utility.ToJson(parameters)
								_, err = store.ExecContext("local_resource", insertSQL, parameters_, nil)
								if err != nil {
									fmt.Println("Error inserting data into array table: ", err)
								}

							}

						default:
							fmt.Printf("------------------> index %d: Unknown Type %s \n", i, columnName)
						}

						// append the object id...
						if len(parameters) > 0 {
						
							// Create the table if it does not exist.
							if !store.isTableExist(connectionId, arrayTableName) {
								createTableSQL, arrayTableSQL := generateCreateTableSQL(arrayTableName, map[string]interface{}{"value": 0, tableName + "_uid": 0})
								_, err := store.ExecContext(connectionId, createTableSQL, "[]", nil)
								if err != nil {
									return nil, err
								}

								for _, sql := range arrayTableSQL {
									_, err := store.ExecContext(connectionId, sql, "[]", nil)
									if err != nil {
										return nil, err
									}
								}
							}

							parameters = append(parameters, Utility.ToInt(result["lastId"]))
							parameters_, _ := Utility.ToJson(parameters)
							_, err := store.ExecContext(connectionId, arrayInsertSQL, parameters_, Utility.ToInt(result["lastId"]))
							if err != nil {
								fmt.Println("Error inserting data into array table: ", err)
								return nil, err
							}
						}
					}
				}
			}
		}
	}

	return data, nil
}

func (store *SqlStore) InsertOne(ctx context.Context, connectionId string, db string, table string, entity interface{}, options string) (interface{}, error) {

	if !store.isTableExist(connectionId, table) {
		// Create the table
		createTableSQL, arrayTableSQL := generateCreateTableSQL(table, entity.(map[string]interface{}))

		_, err := store.ExecContext(connectionId, createTableSQL, "[]", nil)
		if err != nil {
			return nil, err
		}

		for _, sql := range arrayTableSQL {
			_, err := store.ExecContext(connectionId, sql, "[]", nil)
			if err != nil {
				return nil, err
			}
		}
	}

	result, err := store.insertData(connectionId, table, entity.(map[string]interface{}))
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (store *SqlStore) InsertMany(ctx context.Context, connectionId string, keyspace string, table string, entities []interface{}, options string) ([]interface{}, error) {

	if !store.isTableExist(connectionId, table) {
		// Create the table
		createTableSQL, arrayTableSQL := generateCreateTableSQL(table, entities[0].(map[string]interface{}))
		_, err := store.ExecContext(connectionId, createTableSQL, "[]", nil)
		if err != nil {
			return nil, err
		}

		for _, sql := range arrayTableSQL {
			_, err := store.ExecContext(connectionId, sql, "[]", nil)
			if err != nil {
				return nil, err
			}
		}
	}

	var results []interface{}
	for _, entity := range entities {
		result, err := store.insertData(connectionId, table, entity.(map[string]interface{}))
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
func (store *SqlStore) recreateArrayOfObjects(connectionId, tableName string, dataHeader map[string]interface{}, options []map[string]interface{}) ([]interface{}, error) {
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
				projection["uid"] = 1 // set the uid
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
		str, err := store.QueryContext(connectionId, query, "[]")
		if err != nil {
			fmt.Println("Error executing query: ", query, err)
			return nil, err
		}

		tables := make(map[string]interface{}, 0)
		err = json.Unmarshal([]byte(str), &tables)
		if err != nil {
			fmt.Print("Error unmarshalling result: ", str, err)
			return nil, err
		}

		// Loop through the tables
		for _, values := range tables["data"].([]interface{}) {

			for _, value := range values.([]interface{}) {

				field := strings.Replace(value.(string), tableName+"_", "", 1)

				if strings.HasPrefix(value.(string), tableName+"_") && object[field] == nil {

					// Query to retrieve the data from the array table
					query := fmt.Sprintf("SELECT value FROM %s WHERE %s=?", value.(string), tableName+"_uid")
					parameters := make([]interface{}, 0)
					parameters = append(parameters, object["uid"]) // append the object id...
					parameters_, _ := Utility.ToJson(parameters)

					// Execute the query
					str, err := store.QueryContext(connectionId, query, parameters_)
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
					} else{

						query := fmt.Sprintf("SELECT * FROM %s WHERE source_uid=?", value.(string))
						parameters := make([]interface{}, 0)
						parameters = append(parameters, object["uid"]) // append the object id...
						parameters_, _ := Utility.ToJson(parameters)
	
						// Execute the query
						str, err := store.QueryContext(connectionId, query, parameters_)
						if err == nil {
							data := make(map[string]interface{}, 0)
							err = json.Unmarshal([]byte(str), &data)
							if err != nil {
								return nil, err
							}
	
							object[field] = make([]interface{}, 0)
							for _, values := range data["data"].([]interface{}) {
								ref_uid := values.([]interface{})[1] // the value is the second element of the array.
								fmt.Println("------------------------> ", ref_uid)
							}
						}
					}

				}
			}
		}
	}

	return objects, nil
}

func (store *SqlStore) FindOne(ctx context.Context, connectionId string, database string, table string, query string, options string) (interface{}, error) {
	fmt.Println("FindOne ", connectionId, database, table, query, options)
	if len(query) == 0 {
		return nil, errors.New("query is empty")
	} else if strings.HasPrefix(query, "{") && strings.HasSuffix(query, "}") {
		if query == "{}" {
			query = fmt.Sprintf("SELECT * FROM %s", table)
		} else {
			parameters := make(map[string]interface{}, 0)
			err := json.Unmarshal([]byte(query), &parameters)
			if err != nil {
				return nil, err
			}

			// I will build the query here.
			query = fmt.Sprintf("SELECT * FROM %s WHERE ", table)
			for key, value := range parameters {
				if reflect.TypeOf(value).Kind() == reflect.String {
					query += fmt.Sprintf("%s = '%v' AND ", key, value)
				} else {
					query += fmt.Sprintf("%s = %v AND ", key, value)
				}
			}

			query = strings.TrimSuffix(query, " AND ")

			fmt.Println("Query: ", query)
		}
	}

	str, err := store.QueryContext(connectionId, query, "[]")
	if err != nil {
		fmt.Println("Error executing query: ", query, err)
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

	objects, err := store.recreateArrayOfObjects(connectionId, table, data, options_)
	if err != nil {
		return nil, err
	}

	if len(objects) > 0 {
		return objects[0], nil
	}

	return nil, errors.New("not found")
}

func (store *SqlStore) Find(ctx context.Context, connectionId string, keyspace string, table string, query string, options string) ([]interface{}, error) {

	fmt.Println("Find ", connectionId, keyspace, table, query, options)

	if len(query) == 0 || query == "{}" {
		query = fmt.Sprintf("SELECT * FROM %s", table)
	} else if strings.HasPrefix(query, "{") && strings.HasSuffix(query, "}") {
		if query == "{}" {
			query = fmt.Sprintf("SELECT * FROM %s", table)
		} else {
			parameters := make(map[string]interface{}, 0)
			err := json.Unmarshal([]byte(query), &parameters)
			if err != nil {
				return nil, err
			}

			// I will build the query here.
			query = fmt.Sprintf("SELECT * FROM %s WHERE ", table)
			for key, value := range parameters {
				if reflect.TypeOf(value).Kind() == reflect.String {
					query += fmt.Sprintf("%s = '%v' AND ", key, value)
				} else {
					query += fmt.Sprintf("%s = %v AND ", key, value)
				}
			}

			query = strings.TrimSuffix(query, " AND ")

			fmt.Println("Query: ", query)
		}
	}

	str, err := store.QueryContext(connectionId, query, "[]")
	if err != nil {
		fmt.Println("Error executing query: ", query, err)
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

	objects, err := store.recreateArrayOfObjects(connectionId, table, data, options_)
	if err != nil {
		fmt.Println("Error recreating array of objects: ", err)
		return nil, err
	}

	if len(objects) > 0 {
		return objects, nil
	}

	return nil, errors.New("not found")
}

func (store *SqlStore) ReplaceOne(ctx context.Context, connectionId string, keyspace string, table string, query string, value string, options string) error {

	// insert the new entry.
	entity := make(map[string]interface{}, 0)
	err := json.Unmarshal([]byte(value), &entity)
	if err != nil {
		return err
	}

	if !store.isTableExist(connectionId, table) {
		// Create the table
		createTableSQL, arrayTableSQL := generateCreateTableSQL(table, entity)
		_, err := store.ExecContext(connectionId, createTableSQL, "[]", nil)
		if err != nil {
			return err
		}

		for _, sql := range arrayTableSQL {
			_, err := store.ExecContext(connectionId, sql, "[]", nil)
			if err != nil {
				return err
			}
		}
	}

	// delete entry if it exist.
	err = store.deleteSqlEntry(connectionId, table, query)

	if err != nil {
		fmt.Println("fail to delete data: ", query, err)

	}

	_, err = store.insertData(connectionId, table, entity)
	if err != nil {
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

func (store *SqlStore) Update(ctx context.Context, connectionId string, keyspace string, table string, query string, value string, options string) error {
	fmt.Println("Update ", connectionId, keyspace, table, query, value, options)

	values_ := make(map[string]interface{}, 0)
	err := json.Unmarshal([]byte(value), &values_)
	if err != nil {
		return err
	}

	// Ensure the number of fields and values match
	if len(values_["fields"].([]interface{})) != len(values_["values"].([]interface{})) {
		return fmt.Errorf("number of fields does not match the number of values")
	}

	q, err := generateUpdateTableQuery(table, values_["fields"].([]interface{}), query)
	if err != nil {
		return err
	}

	parameters, _ := Utility.ToJson(values_["values"])
	_, err = store.ExecContext(connectionId, q, string(parameters), nil)
	if err != nil {
		return err
	}

	return nil
}

func (store *SqlStore) UpdateOne(ctx context.Context, connectionId string, keyspace string, table string, query string, value string, options string) error {
	// fmt.Println("UpdateOne ", connectionId, keyspace, table, query, value, options)
	values_ := make(map[string]interface{}, 0)
	err := json.Unmarshal([]byte(value), &values_)
	if err != nil {
		return err
	}

	q, err := generateUpdateTableQuery(table, values_["fields"].([]interface{}), query)
	if err != nil {
		return err
	}

	parameters, _ := Utility.ToJson(values_["values"])

	_, err = store.ExecContext(connectionId, q, string(parameters), nil)
	if err != nil {
		return err
	}

	return nil
}

func (store *SqlStore) deleteSqlEntry(connectionId string, table string, query string) error {

	if strings.HasPrefix(query, "{") && strings.HasSuffix(query, "}") {
		if query == "{}" {
			query = fmt.Sprintf("SELECT * FROM %s", table)
		} else {
			parameters := make(map[string]interface{}, 0)
			err := json.Unmarshal([]byte(query), &parameters)
			if err != nil {
				return err
			}

			// I will build the query here.
			query = fmt.Sprintf("SELECT * FROM %s WHERE ", table)
			for key, value := range parameters {
				if reflect.TypeOf(value).Kind() == reflect.String {
					query += fmt.Sprintf("%s = '%v' AND ", key, value)
				} else {
					query += fmt.Sprintf("%s = %v AND ", key, value)
				}
			}

			query = strings.TrimSuffix(query, " AND ")

			fmt.Println("Query: ", query)
		}
	}

	query = strings.Replace(query, "SELECT *", "DELETE", 1)
	_, err := store.ExecContext(connectionId, query, "[]", nil)
	if err != nil {
		return err
	}
	return nil
}

func (store *SqlStore) Delete(ctx context.Context, connectionId string, keyspace string, table string, query string, options string) error {
	fmt.Println("Delete ", connectionId, keyspace, table, query, options)
	return store.deleteSqlEntry(connectionId, table, query)
}

func (store *SqlStore) DeleteOne(ctx context.Context, connectionId string, keyspace string, table string, query string, options string) error {
	fmt.Println("DeleteOne ", connectionId, keyspace, table, query, options)
	return store.deleteSqlEntry(connectionId, table, query)
}

func (store *SqlStore) Aggregate(ctx context.Context, connectionId string, keyspace string, table string, pipeline string, optionsStr string) ([]interface{}, error) {
	fmt.Println("Aggregate ", connectionId, keyspace, table, pipeline, optionsStr)
	return nil, errors.New("not implemented")
}

func (store *SqlStore) CreateTable(ctx context.Context, connectionId string, database string, table string, fields []string) error {

	fmt.Println("CreateTable ", connectionId, database, table, fields)

	// Create the table
	createTableSQL := fmt.Sprintf("CREATE TABLE IF NOT EXISTS \"%s\" (uid INTEGER PRIMARY KEY AUTOINCREMENT, %s);", table, strings.Join(fields, ", "))
	_, err := store.ExecContext(connectionId, createTableSQL, "[]", nil)
	if err != nil {
		return err
	}

	return nil
}

func (store *SqlStore) CreateCollection(ctx context.Context, connectionId string, database string, name string, optionsStr string) error {
	fmt.Println("CreateCollection ", connectionId, database, name)
	return errors.New("not implemented")
}

func (store *SqlStore) DeleteCollection(ctx context.Context, connectionId string, database string, collection string) error {
	fmt.Println("DeleteCollection ", connectionId, database, collection)
	return errors.New("not implemented")
}

func (store *SqlStore) RunAdminCmd(ctx context.Context, connectionId string, user string, password string, script string) error {
	fmt.Println("RunAdminCmd ", connectionId, user, password, script)
	// TODO: I will need to parse the script and execute the command.
	return nil
}
