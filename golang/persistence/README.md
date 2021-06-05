# Persistence Service
Create Read Update and Delete entities... Persistence service give you all require functionalities to make all your applications persistent.
That microservice define a generic datastore that can be implement by almost any storage technologie. At this time only mongoDB is available,
but SQL is planned to be use to.

## Create a connection
Here is how to create a connection. The required parameters are,
* The _connection id_
* The _database_ name
* The data store _domain_ 
* The data store _port_
* The data story type 0 is _mongoDB_
* The _user_
* The _password_
* The connection _timeout_
* The options string
* Does the connection must be store in the configuration file

_Go_
```go Golang
user := "sa"
pwd := "adminadmin"
err := client.CreateConnection("mongo_db_test_connection", "mongo_db_test_connection", "localhost", 27017, 0, user, pwd, 500, "", true)
```


## Insert One
Here is how to insert an entity into the database