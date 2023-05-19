package persistence_client

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"reflect"
	"time"

	//"log"

	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/config/config_client"
	"github.com/globulario/services/golang/globular_client"
	globular "github.com/globulario/services/golang/globular_client"
	"github.com/globulario/services/golang/persistence/persistencepb"
	"github.com/globulario/services/golang/security"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// //////////////////////////////////////////////////////////////////////////////
// Persitence Client Service
// //////////////////////////////////////////////////////////////////////////////
type Persistence_Client struct {
	cc *grpc.ClientConn
	c  persistencepb.PersistenceServiceClient

	// The id of the service
	id string

	// The mac address of the server
	mac string

	// The name of the service
	name string

	// The client domain
	domain string

	// The address where connection with client can be done. ex: globule0.globular.cloud:10101
	address string

	//  keep the last connection state of the client.
	state string

	// The port
	port int

	// is the connection is secure?
	hasTLS bool

	// Link to client key file
	keyFile string

	// Link to client certificate file.
	certFile string

	// certificate authority file
	caFile string

	// The client context
	ctx context.Context
}

// Create a connection to the service.
func NewPersistenceService_Client(address string, id string) (*Persistence_Client, error) {

	client := new(Persistence_Client)
	err := globular.InitClient(client, address, id)
	if err != nil {
		return nil, err
	}

	err = client.Reconnect()
	if err != nil {
		return nil, err
	}

	return client, nil
}

func (client *Persistence_Client) Reconnect() error {
	var err error
	nb_try_connect := 10
	
	for i:=0; i <nb_try_connect; i++ {
		client.cc, err = globular.GetClientConnection(client)
		if err == nil {
			client.c = persistencepb.NewPersistenceServiceClient(client.cc)
			break
		}
		
		// wait 500 millisecond before next try
		time.Sleep(500 * time.Millisecond)
	}

	return err
}

// The address where the client can connect.
func (client *Persistence_Client) SetAddress(address string) {
	client.address = address
}

func (client *Persistence_Client) GetConfiguration(address, id string) (map[string]interface{}, error) {
	Utility.RegisterFunction("NewConfigService_Client", config_client.NewConfigService_Client)
	client_, err := globular_client.GetClient(address, "config.ConfigService", "NewConfigService_Client")
	if err != nil {
		return nil, err
	}
	return client_.(*config_client.Config_Client).GetServiceConfiguration(id)
}

func (client *Persistence_Client) Invoke(method string, rqst interface{}, ctx context.Context) (interface{}, error) {
	if ctx == nil {
		ctx = client.GetCtx()
	}
	return globular.InvokeClientRequest(client.c, ctx, method, rqst)
}

func (client *Persistence_Client) GetCtx() context.Context {
	if client.ctx == nil {
		client.ctx = globular.GetClientContext(client)
	}
	token, err := security.GetLocalToken(client.GetMac())
	if err == nil {
		md := metadata.New(map[string]string{"token": string(token), "domain": client.domain, "mac": client.GetMac()})
		client.ctx = metadata.NewOutgoingContext(context.Background(), md)
	}
	return client.ctx
}

// Return the last know connection state
func (client *Persistence_Client) GetState() string {
	return client.state
}

// Return the domain
func (client *Persistence_Client) GetDomain() string {
	return client.domain
}

// Return the address
func (client *Persistence_Client) GetAddress() string {
	return client.address
}

// Return the id of the service instance
func (client *Persistence_Client) GetId() string {
	return client.id
}

// Return the name of the service
func (client *Persistence_Client) GetName() string {
	return client.name
}

func (client *Persistence_Client) GetMac() string {
	return client.mac
}

// must be close when no more needed.
func (client *Persistence_Client) Close() {
	if client.cc != nil {
		client.cc.Close()
	}
}

// Set grpc_service port.
func (client *Persistence_Client) SetPort(port int) {
	client.port = port
}

// Return the grpc port number
func (client *Persistence_Client) GetPort() int {
	return client.port
}

// Set the client instance id.
func (client *Persistence_Client) SetId(id string) {
	client.id = id
}

// Set the client name.
func (client *Persistence_Client) SetName(name string) {
	client.name = name
}

func (client *Persistence_Client) SetMac(mac string) {
	client.mac = mac
}

// Set the domain.
func (client *Persistence_Client) SetDomain(domain string) {
	client.domain = domain
}

func (client *Persistence_Client) SetState(state string) {
	client.state = state
}

////////////////// TLS ///////////////////

// Get if the client is secure.
func (client *Persistence_Client) HasTLS() bool {
	return client.hasTLS
}

// Get the TLS certificate file path
func (client *Persistence_Client) GetCertFile() string {
	return client.certFile
}

// Get the TLS key file path
func (client *Persistence_Client) GetKeyFile() string {
	return client.keyFile
}

// Get the TLS key file path
func (client *Persistence_Client) GetCaFile() string {
	return client.caFile
}

// Set the client is a secure client.
func (client *Persistence_Client) SetTLS(hasTls bool) {
	client.hasTLS = hasTls
}

// Set TLS certificate file path
func (client *Persistence_Client) SetCertFile(certFile string) {
	client.certFile = certFile
}

// Set TLS key file path
func (client *Persistence_Client) SetKeyFile(keyFile string) {
	client.keyFile = keyFile
}

// Set TLS authority trust certificate file path
func (client *Persistence_Client) SetCaFile(caFile string) {
	client.caFile = caFile
}

///////////////////////// API /////////////////////

// Stop the service.
func (client *Persistence_Client) StopService() {
	client.c.Stop(client.GetCtx(), &persistencepb.StopRequest{})
}

// Create a new datastore connection.
func (client *Persistence_Client) CreateConnection(connectionId string, name string, host string, port float64, storeType float64, user string, pwd string, timeout float64, options string, save bool) error {

	rqst := &persistencepb.CreateConnectionRqst{
		Connection: &persistencepb.Connection{
			Id:       connectionId,
			Name:     name,
			Host:     host,
			Port:     int32(Utility.ToInt(port)),
			Store:    persistencepb.StoreType(storeType),
			User:     user,
			Password: pwd,
			Timeout:  int32(Utility.ToInt(timeout)),
			Options:  options,
		},
		Save: save,
	}

	_, err := client.c.CreateConnection(client.GetCtx(), rqst)
	return err
}

func (client *Persistence_Client) DeleteConnection(connectionId string) error {
	rqst := &persistencepb.DeleteConnectionRqst{
		Id: connectionId,
	}

	_, err := client.c.DeleteConnection(client.GetCtx(), rqst)
	return err
}

func (client *Persistence_Client) CreateDatabase(connectionId string, database string) error {
	rqst := &persistencepb.CreateDatabaseRqst{
		Id:       connectionId,
		Database: database,
	}

	_, err := client.c.CreateDatabase(client.GetCtx(), rqst)
	return err
}

func (client *Persistence_Client) Connect(id string, password string) error {
	rqst := &persistencepb.ConnectRqst{
		ConnectionId: id,
		Password:     password,
	}

	_, err := client.c.Connect(client.GetCtx(), rqst)
	return err
}

func (client *Persistence_Client) Disconnect(connectionId string) error {

	rqst := &persistencepb.DisconnectRqst{
		ConnectionId: connectionId,
	}

	_, err := client.c.Disconnect(client.GetCtx(), rqst)

	return err
}

func (client *Persistence_Client) Ping(connectionId string) error {

	rqst := &persistencepb.PingConnectionRqst{
		Id: connectionId,
	}

	_, err := client.c.Ping(client.GetCtx(), rqst)

	return err
}

func (client *Persistence_Client) FindOne(connectionId string, database string, collection string, jsonStr string, options string) (map[string]interface{}, error) {

	// Retreive a single value...
	rqst := &persistencepb.FindOneRqst{
		Id:         connectionId,
		Database:   database,
		Collection: collection,
		Query:      jsonStr,
		Options:    options,
	}

	rsp, err := client.c.FindOne(client.GetCtx(), rqst)
	if err != nil {
		return nil, err
	}

	obj, err := Utility.ToMap(rsp.GetResult())
	if err != nil {
		return nil, err
	}

	return obj, err
}

func (client *Persistence_Client) Find(connectionId string, database string, collection string, query string, options string) ([]interface{}, error) {

	// Retreive a single value...
	rqst := &persistencepb.FindRqst{
		Id:         connectionId,
		Database:   database,
		Collection: collection,
		Query:      query,
		Options:    options,
	}

	stream, err := client.c.Find(client.GetCtx(), rqst)
	if err != nil {
		return nil, err
	}

	// Here I will create the final array
	var buffer bytes.Buffer
	for {
		msg, err := stream.Recv()
		if err == io.EOF || len(msg.Data) == 0 {
			// end of stream...
			break
		} else if err != nil {
			return nil, err
		} else {
			_, err = buffer.Write(msg.Data)
			if err != nil {
				return nil, err
			}
		}
	}

	data := make([]interface{}, 0)
	err = json.Unmarshal(buffer.Bytes(), &data)
	if err != nil {
		return nil, err
	}

	return data, nil
}

/**
 * Usefull function to query and transform document.
 */
func (client *Persistence_Client) Aggregate(connectionId, database string, collection string, pipeline string, options string) ([]interface{}, error) {
	// Retreive a single value...
	rqst := &persistencepb.AggregateRqst{
		Id:         connectionId,
		Database:   database,
		Collection: collection,
		Pipeline:   pipeline,
		Options:    options,
	}

	stream, err := client.c.Aggregate(client.GetCtx(), rqst)

	if err != nil {
		return nil, err
	}

	// Here I will create the final array
	var buffer bytes.Buffer
	for {
		msg, err := stream.Recv()
		if err == io.EOF || len(msg.Data) == 0 {
			// end of stream...
			break
		} else if err != nil {
			return nil, err
		} else {

			_, err = buffer.Write(msg.Data)
			if err != nil {
				return nil, err
			}
		}
	}

	// The buffer that contain the
	data := make([]interface{}, 0)
	err = json.Unmarshal(buffer.Bytes(), &data)
	if err != nil {
		return nil, err
	}

	return data, nil
}

/**
 * Count the number of document that match the query.
 */
func (client *Persistence_Client) Count(connectionId string, database string, collection string, query string, options string) (int, error) {

	rqst := &persistencepb.CountRqst{
		Id:         connectionId,
		Database:   database,
		Collection: collection,
		Query:      query,
		Options:    options,
	}

	rsp, err := client.c.Count(client.GetCtx(), rqst)

	if err != nil {
		return 0, err
	}

	return int(rsp.Result), err
}

/**
 * Insert one value in the database.
 */
func (client *Persistence_Client) InsertOne(connectionId string, database string, collection string, entity interface{}, options string) (string, error) {

	// Try to marshal object...
	data, err := json.Marshal(entity)
	if err != nil {
		return "", err
	}

	rqst := &persistencepb.InsertOneRqst{
		Id:         connectionId,
		Database:   database,
		Collection: collection,
		Data:       string(data),
		Options:    options,
	}

	rsp, err := client.c.InsertOne(client.GetCtx(), rqst)

	if err != nil {
		return "", err
	}

	return rsp.GetId(), err
}

func (client *Persistence_Client) InsertMany(connectionId string, database string, collection string, entities []interface{}, options string) error {

	stream, err := client.c.InsertMany(client.GetCtx())
	if err != nil {
		return err
	}

	// here you must run the sql service test before runing this test in order
	// to generate the file Employees.json
	const BufferSize = 1024 // the chunck size.
	var buffer bytes.Buffer
	enc := json.NewEncoder(&buffer) // Will write to network.
	err = enc.Encode(entities)

	if err != nil {
		return err
	}

	for {
		var data [BufferSize]byte
		bytesread, err := buffer.Read(data[0:BufferSize])
		if err == io.EOF {
			err = nil
			break
		} else if err != nil {
			return err
		} else if bytesread > 0 {
			rqst := &persistencepb.InsertManyRqst{
				Id:         connectionId,
				Database:   database,
				Collection: collection,
				Data:       data[0:bytesread],
			}
			// send the data to the server.
			err = stream.Send(rqst)
			if err != nil {
				break
			}
		} else {
			break
		}

	}

	_, err = stream.CloseAndRecv()
	if err != nil && err != io.EOF {
		return err
	}

	return nil
}

/**
 * Insert one value in the database.
 */
func (client *Persistence_Client) ReplaceOne(connectionId string, database string, collection string, query string, entity interface{}, options string) error {

	var value string
	if reflect.TypeOf(entity).Kind() == reflect.String {
		value = entity.(string)
	} else {
		data, err := json.Marshal(entity)
		if err != nil {
			return err
		}
		value = string(data)
	}

	rqst := &persistencepb.ReplaceOneRqst{
		Id:         connectionId,
		Database:   database,
		Collection: collection,
		Query:      query,
		Value:      value,
		Options:    options,
	}

	_, err := client.c.ReplaceOne(client.GetCtx(), rqst)

	return err
}

func (client *Persistence_Client) UpdateOne(connectionId string, database string, collection string, query string, entity interface{}, options string) error {

	var value string
	if reflect.TypeOf(entity).Kind() == reflect.String {
		value = entity.(string)
	} else {
		data, err := json.Marshal(entity)
		if err != nil {
			return err
		}
		value = string(data)
	}

	rqst := &persistencepb.UpdateOneRqst{
		Id:         connectionId,
		Database:   database,
		Collection: collection,
		Query:      query,
		Value:      value,
		Options:    options,
	}

	_, err := client.c.UpdateOne(client.GetCtx(), rqst)

	return err
}

/**
 * Update one or more document.
 */
func (client *Persistence_Client) Update(connectionId string, database string, collection string, query string, value string, options string) error {

	rqst := &persistencepb.UpdateRqst{
		Id:         connectionId,
		Database:   database,
		Collection: collection,
		Query:      query,
		Value:      value,
		Options:    options,
	}

	_, err := client.c.Update(client.GetCtx(), rqst)

	return err
}

/**
 * Delete one document from the db
 */
func (client *Persistence_Client) DeleteOne(connectionId string, database string, collection string, query string, options string) error {

	rqst := &persistencepb.DeleteOneRqst{
		Id:         connectionId,
		Database:   database,
		Collection: collection,
		Query:      query,
		Options:    options,
	}

	_, err := client.c.DeleteOne(client.GetCtx(), rqst)

	if err != nil {
		return err
	}

	return err
}

/**
 * Delete many document from the db.
 */
func (client *Persistence_Client) Delete(connectionId string, database string, collection string, query string, options string) error {

	rqst := &persistencepb.DeleteRqst{
		Id:         connectionId,
		Database:   database,
		Collection: collection,
		Query:      query,
		Options:    options,
	}

	_, err := client.c.Delete(client.GetCtx(), rqst)

	if err != nil {
		return err
	}

	return err
}

/**
 * Drop a collection.
 */
func (client *Persistence_Client) DeleteCollection(connectionId string, database string, collection string) error {
	// Test drop collection.
	rqst_drop_collection := &persistencepb.DeleteCollectionRqst{
		Id:         connectionId,
		Database:   database,
		Collection: collection,
	}
	_, err := client.c.DeleteCollection(client.GetCtx(), rqst_drop_collection)

	return err
}

/**
 * Drop a database.
 */
func (client *Persistence_Client) DeleteDatabase(connectionId string, database string) error {
	// Test drop collection.
	rqst_drop_db := &persistencepb.DeleteDatabaseRqst{
		Id:       connectionId,
		Database: database,
	}

	_, err := client.c.DeleteDatabase(client.GetCtx(), rqst_drop_db)

	return err
}

/**
 * Admin function, that must be protected.
 */
func (client *Persistence_Client) RunAdminCmd(connectionId string, user string, pwd string, script string) error {
	// Test drop collection.
	rqst_drop_db := &persistencepb.RunAdminCmdRqst{
		ConnectionId: connectionId,
		Script:       script,
		User:         user,
		Password:     pwd,
	}

	_, err := client.c.RunAdminCmd(client.GetCtx(), rqst_drop_db)

	return err
}
