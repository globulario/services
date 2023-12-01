package catalog_client

import (
	"context"
	"time"

	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/catalog/catalogpb"
	globular "github.com/globulario/services/golang/globular_client"
	"github.com/globulario/services/golang/security"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

////////////////////////////////////////////////////////////////////////////////
// catalog Client Service
////////////////////////////////////////////////////////////////////////////////

type Catalog_Client struct {
	cc *grpc.ClientConn
	c  catalogpb.CatalogServiceClient

	// The id of the service
	id string

	// The name of the service
	name string

	// The mac address of the server
	mac string

	// The client domain
	domain string

	// The address where connection with client can be done. ex: globule0.globular.cloud:10101
	address string

	//  keep the last connection state of the client.
	state string

	// The port of the client.
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
func NewCatalogService_Client(address string, id string) (*Catalog_Client, error) {
	client := new(Catalog_Client)
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

func (client *Catalog_Client) Reconnect() error {
	var err error
	nb_try_connect := 10

	for i := 0; i < nb_try_connect; i++ {
		client.cc, err = globular.GetClientConnection(client)
		if err == nil {
			client.c = catalogpb.NewCatalogServiceClient(client.cc)
			break
		}

		// wait 500 millisecond before next try
		time.Sleep(500 * time.Millisecond)
	}

	return err

}

// The address where the client can connect.
func (client *Catalog_Client) SetAddress(address string) {
	client.address = address
}

func (client *Catalog_Client) Invoke(method string, rqst interface{}, ctx context.Context) (interface{}, error) {
	if ctx == nil {
		ctx = client.GetCtx()
	}
	return globular.InvokeClientRequest(client.c, ctx, method, rqst)
}

func (client *Catalog_Client) GetCtx() context.Context {
	if client.ctx == nil {
		client.ctx = globular.GetClientContext(client)
	}

	// refresh the client as needed...
	token, err := security.GetLocalToken(client.GetMac())
	if err == nil {
		md := metadata.New(map[string]string{"token": string(token), "domain": client.domain, "mac": client.GetMac(), "address": client.GetAddress()})
		client.ctx = metadata.NewOutgoingContext(context.Background(), md)
	}

	return client.ctx
}

// Return the last know connection state
func (client *Catalog_Client) GetState() string {
	return client.state
}

// Return the domain
func (client *Catalog_Client) GetDomain() string {
	return client.domain
}

func (client *Catalog_Client) GetAddress() string {
	return client.address
}

// Return the id of the service instance
func (client *Catalog_Client) GetId() string {
	return client.id
}

// Return the name of the service
func (client *Catalog_Client) GetName() string {
	return client.name
}

func (client *Catalog_Client) GetMac() string {
	return client.mac
}

// must be close when no more needed.
func (client *Catalog_Client) Close() {
	client.cc.Close()
}

// Set grpc_service port.
func (client *Catalog_Client) SetPort(port int) {
	client.port = port
}

// Return the grpc port number
func (client *Catalog_Client) GetPort() int {
	return client.port
}

// Set the client instance id.
func (client *Catalog_Client) SetId(id string) {
	client.id = id
}

// Set the client name.
func (client *Catalog_Client) SetName(name string) {
	client.name = name
}

func (client *Catalog_Client) SetMac(mac string) {
	client.mac = mac
}

func (client *Catalog_Client) SetState(state string) {
	client.state = state
}

// Set the domain.
func (client *Catalog_Client) SetDomain(domain string) {
	client.domain = domain
}

////////////////// TLS ///////////////////

// Get if the client is secure.
func (client *Catalog_Client) HasTLS() bool {
	return client.hasTLS
}

// Get the TLS certificate file path
func (client *Catalog_Client) GetCertFile() string {
	return client.certFile
}

// Get the TLS key file path
func (client *Catalog_Client) GetKeyFile() string {
	return client.keyFile
}

// Get the TLS key file path
func (client *Catalog_Client) GetCaFile() string {
	return client.caFile
}

// Set the client is a secure client.
func (client *Catalog_Client) SetTLS(hasTls bool) {
	client.hasTLS = hasTls
}

// Set TLS certificate file path
func (client *Catalog_Client) SetCertFile(certFile string) {
	client.certFile = certFile
}

// Set TLS key file path
func (client *Catalog_Client) SetKeyFile(keyFile string) {
	client.keyFile = keyFile
}

// Set TLS authority trust certificate file path
func (client *Catalog_Client) SetCaFile(caFile string) {
	client.caFile = caFile
}

// //////////////////////// API ////////////////////////
// Stop the service.
func (client *Catalog_Client) StopService() {
	client.c.Stop(client.GetCtx(), &catalogpb.StopRequest{})
}

// Create a new datastore connection.
func (client *Catalog_Client) CreateConnection(connectionId string, name string, host string, port float64, storeType float64, user string, pwd string, timeout float64, options string) error {
	rqst := &catalogpb.CreateConnectionRqst{
		Connection: &catalogpb.Connection{
			Id:       connectionId,
			Name:     name,
			Host:     host,
			Port:     int32(Utility.ToInt(port)),
			Store:    catalogpb.StoreType(storeType),
			User:     user,
			Password: pwd,
			Timeout:  int32(Utility.ToInt(timeout)),
			Options:  options,
		},
	}

	_, err := client.c.CreateConnection(client.GetCtx(), rqst)
	return err
}

/**
 * Create a new unit of measure
 */
func (client *Catalog_Client) SaveUnitOfMesure(connectionId string, id string, languageCode string, name string, abreviation string, description string) error {
	rqst := &catalogpb.SaveUnitOfMeasureRequest{
		ConnectionId: connectionId,
		UnitOfMeasure: &catalogpb.UnitOfMeasure{
			Id:           id,
			LanguageCode: languageCode,
			Name:         name,
			Description:  description,
			Abreviation:  abreviation,
		},
	}

	_, err := client.c.SaveUnitOfMeasure(client.GetCtx(), rqst)
	if err != nil {
		return err
	}

	return nil
}

/**
 * Save item property definition.
 */
func (client *Catalog_Client) SavePropertyDefinition(connectionId string, id string, languageCode string, name string, abreviation string, description string, valueType float64) error {
	rqst := &catalogpb.SavePropertyDefinitionRequest{
		ConnectionId: connectionId,
		PropertyDefinition: &catalogpb.PropertyDefinition{
			Id:           id,
			LanguageCode: languageCode,
			Name:         name,
			Description:  description,
			Abreviation:  abreviation,
			Type:         catalogpb.PropertyDefinition_Type(int32(Utility.ToInt(valueType))),
		},
	}

	_, err := client.c.SavePropertyDefinition(client.GetCtx(), rqst)
	if err != nil {
		return err
	}

	return nil
}

/**
 * Save item property definition.
 */
func (client *Catalog_Client) SaveItemDefinition(connectionId string, id string, languageCode string, name string, abreviation string, description string, properties_str string, properties_ids_str string) error {

	properties := new(catalogpb.References)

	protojson.Unmarshal([]byte(properties_str), properties)
	protojson.Unmarshal([]byte(properties_ids_str), properties)

	rqst := &catalogpb.SaveItemDefinitionRequest{
		ConnectionId: connectionId,
		ItemDefinition: &catalogpb.ItemDefinition{
			Id:           id,
			LanguageCode: languageCode,
			Name:         name,
			Description:  description,
			Abreviation:  abreviation,
			Properties:   properties,
		},
	}

	_, err := client.c.SaveItemDefinition(client.GetCtx(), rqst)
	if err != nil {
		return err
	}

	return nil
}

/**
 * Save item property definition.
 */
func (client *Catalog_Client) SaveItemInstance(connectionId string, jsonStr string) error {

	instance := new(catalogpb.ItemInstance)

	err := protojson.Unmarshal([]byte(jsonStr), instance)
	if err != nil {
		return err
	}

	rqst := &catalogpb.SaveItemInstanceRequest{
		ItemInstance: instance,
		ConnectionId: connectionId,
	}

	_, err = client.c.SaveItemInstance(client.GetCtx(), rqst)
	if err != nil {
		return err
	}

	return nil
}

/**
 * Save a Manufacturer whitout item.
 */
func (client *Catalog_Client) SaveManufacturer(connectionId string, id string, name string) error {
	rqst := &catalogpb.SaveManufacturerRequest{
		ConnectionId: connectionId,
		Manufacturer: &catalogpb.Manufacturer{
			Id:   id,
			Name: name,
		},
	}

	_, err := client.c.SaveManufacturer(client.GetCtx(), rqst)

	return err
}

/**
 * Save package, create it if it not already exist.
 * TODO append subPackage param and itemInstance...
 */
func (client *Catalog_Client) SavePackage(connectionId string, id string, name string, languageCode string, description string, inventories []*catalogpb.Inventory) error {

	// The request.
	rqst := &catalogpb.SavePackageRequest{
		Package: &catalogpb.Package{
			Id:            id,
			Name:          name,
			LanguageCode:  languageCode,
			Description:   description,
			Subpackages:   make([]*catalogpb.SubPackage, 0),
			ItemInstances: make([]*catalogpb.ItemInstancePackage, 0),
		},
		ConnectionId: connectionId,
	}

	_, err := client.c.SavePackage(client.GetCtx(), rqst)

	return err
}

/**
 * Save a supplier.
 */
func (client *Catalog_Client) SaveSupplier(connectionId string, id string, name string) error {
	rqst := &catalogpb.SaveSupplierRequest{
		ConnectionId: connectionId,
		Supplier: &catalogpb.Supplier{
			Id:   id,
			Name: name,
		},
	}

	_, err := client.c.SaveSupplier(client.GetCtx(), rqst)

	return err
}

/**
 * Save package supplier.
 */
func (client *Catalog_Client) SavePackageSupplier(connectionId string, id string, supplier_ref_str string, packege_ref_str string, price_str string, date int64, qty int64) error {

	// Supplier Ref.
	supplierRef := new(catalogpb.Reference)
	err := protojson.Unmarshal([]byte(supplier_ref_str), supplierRef)
	if err != nil {
		return err
	}

	// Pacakge Ref.
	packageRef := new(catalogpb.Reference)
	err = protojson.Unmarshal([]byte(packege_ref_str), packageRef)
	if err != nil {
		return err
	}

	price := new(catalogpb.Price)
	err = protojson.Unmarshal([]byte(price_str), price)
	if err != nil {
		return err
	}

	rqst := new(catalogpb.SavePackageSupplierRequest)
	rqst.ConnectionId = connectionId
	rqst.PackageSupplier = &catalogpb.PackageSupplier{Id: id, Supplier: supplierRef, Package: packageRef, Price: price, Date: date, Quantity: qty}

	_, err = client.c.SavePackageSupplier(client.GetCtx(), rqst)
	return err
}

/**
 * Save Item Manufacturer.
 */
func (client *Catalog_Client) SaveItemManufacturer(connectionId string, id string, manufacturer_ref_str string, item_ref_str string) error {

	// Supplier Ref.
	manufacturerRef := new(catalogpb.Reference)
	err := protojson.Unmarshal([]byte(manufacturer_ref_str), manufacturerRef)
	if err != nil {
		return err
	}

	// Item Ref.
	itemRef := new(catalogpb.Reference)
	err = protojson.Unmarshal([]byte(item_ref_str), itemRef)
	if err != nil {
		return err
	}

	rqst := new(catalogpb.SaveItemManufacturerRequest)
	rqst.ConnectionId = connectionId
	rqst.ItemManafacturer = &catalogpb.ItemManufacturer{Id: id, Manufacturer: manufacturerRef, Item: itemRef}

	_, err = client.c.SaveItemManufacturer(client.GetCtx(), rqst)
	return err
}

/**
 * Save Item Manufacturer.
 */
func (client *Catalog_Client) SaveCategory(connectionId string, id string, name string, languageCode string, categories_str string) error {
	categories := new(catalogpb.References)
	protojson.Unmarshal([]byte(categories_str), categories)

	rqst := &catalogpb.SaveCategoryRequest{
		ConnectionId: connectionId,
		Category: &catalogpb.Category{
			Id:           id,
			Name:         name,
			LanguageCode: languageCode,
			Categories:   categories,
		},
	}

	_, err := client.c.SaveCategory(client.GetCtx(), rqst)
	return err
}

/**
 * Appen item defintion category.
 */
func (client *Catalog_Client) AppendItemDefinitionCategory(connectionId string, item_definition_ref_str string, category_ref_str string) error {
	// The item definition reference.
	itemDefinitionRef := new(catalogpb.Reference)
	err := protojson.Unmarshal([]byte(item_definition_ref_str), itemDefinitionRef)
	if err != nil {
		return err
	}

	// The category reference.
	categoryRef := new(catalogpb.Reference)
	err = protojson.Unmarshal([]byte(category_ref_str), categoryRef)
	if err != nil {
		return err
	}

	rqst := &catalogpb.AppendItemDefinitionCategoryRequest{
		ConnectionId:   connectionId,
		ItemDefinition: itemDefinitionRef,
		Category:       categoryRef,
	}

	_, err = client.c.AppendItemDefinitionCategory(client.GetCtx(), rqst)

	return err
}

/**
 * Remove item defintion category.
 */
func (client *Catalog_Client) RemoveItemDefinitionCategory(connectionId string, item_definition_ref_str string, category_ref_str string) error {
	// The item definition reference.
	itemDefinitionRef := new(catalogpb.Reference)
	err :=protojson.Unmarshal([]byte(item_definition_ref_str), itemDefinitionRef)
	if err != nil {
		return err
	}

	// The category reference.
	categoryRef := new(catalogpb.Reference)
	err = protojson.Unmarshal([]byte(category_ref_str), categoryRef)
	if err != nil {
		return err
	}

	rqst := &catalogpb.RemoveItemDefinitionCategoryRequest{
		ConnectionId:   connectionId,
		ItemDefinition: itemDefinitionRef,
		Category:       categoryRef,
	}

	_, err = client.c.RemoveItemDefinitionCategory(client.GetCtx(), rqst)

	return err
}

/**
 * Save Item Localisation.
 */
func (client *Catalog_Client) SaveLocalisation(connectionId string, localisation *catalogpb.Localisation) error {

	rqst := &catalogpb.SaveLocalisationRequest{
		ConnectionId: connectionId,
		Localisation: localisation,
	}

	_, err := client.c.SaveLocalisation(client.GetCtx(), rqst)
	return err
}
