package catalog_client

import (
	"strconv"

	"context"

	"github.com/globulario/services/golang/catalog/catalogpb"
	globular "github.com/globulario/services/golang/globular_client"
	"github.com/davecourtois/Utility"
	"github.com/golang/protobuf/jsonpb"
	"google.golang.org/grpc"
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
}

// Create a connection to the service.
func NewCatalogService_Client(address string, id string) (*Catalog_Client, error) {
	client := new(Catalog_Client)
	err := globular.InitClient(client, address, id)
	if err != nil {
		return nil, err
	}
	client.cc, err = globular.GetClientConnection(client)
	if err != nil {
		return nil, err
	}

	client.c = catalogpb.NewCatalogServiceClient(client.cc)

	return client, nil
}

func (catalog_client *Catalog_Client) Invoke(method string, rqst interface{}, ctx context.Context) (interface{}, error) {
	if ctx == nil {
		ctx = globular.GetClientContext(catalog_client)
	}
	return globular.InvokeClientRequest(catalog_client.c, ctx, method, rqst)
}

// Return the domain
func (catalog_client *Catalog_Client) GetDomain() string {
	return catalog_client.domain
}

func (catalog_client *Catalog_Client) GetAddress() string {
	return catalog_client.domain + ":" + strconv.Itoa(catalog_client.port)
}

// Return the id of the service instance
func (catalog_client *Catalog_Client) GetId() string {
	return catalog_client.id
}

// Return the name of the service
func (catalog_client *Catalog_Client) GetName() string {
	return catalog_client.name
}

func (catalog_client *Catalog_Client) GetMac() string {
	return catalog_client.mac
}

// must be close when no more needed.
func (catalog_client *Catalog_Client) Close() {
	catalog_client.cc.Close()
}

// Set grpc_service port.
func (catalog_client *Catalog_Client) SetPort(port int) {
	catalog_client.port = port
}

// Set the client instance id.
func (catalog_client *Catalog_Client) SetId(id string) {
	catalog_client.id = id
}

// Set the client name.
func (catalog_client *Catalog_Client) SetName(name string) {
	catalog_client.name = name
}

func (catalog_client *Catalog_Client) SetMac(mac string) {
	catalog_client.mac = mac
}

// Set the domain.
func (catalog_client *Catalog_Client) SetDomain(domain string) {
	catalog_client.domain = domain
}

////////////////// TLS ///////////////////

// Get if the client is secure.
func (catalog_client *Catalog_Client) HasTLS() bool {
	return catalog_client.hasTLS
}

// Get the TLS certificate file path
func (catalog_client *Catalog_Client) GetCertFile() string {
	return catalog_client.certFile
}

// Get the TLS key file path
func (catalog_client *Catalog_Client) GetKeyFile() string {
	return catalog_client.keyFile
}

// Get the TLS key file path
func (catalog_client *Catalog_Client) GetCaFile() string {
	return catalog_client.caFile
}

// Set the client is a secure client.
func (catalog_client *Catalog_Client) SetTLS(hasTls bool) {
	catalog_client.hasTLS = hasTls
}

// Set TLS certificate file path
func (catalog_client *Catalog_Client) SetCertFile(certFile string) {
	catalog_client.certFile = certFile
}

// Set TLS key file path
func (catalog_client *Catalog_Client) SetKeyFile(keyFile string) {
	catalog_client.keyFile = keyFile
}

// Set TLS authority trust certificate file path
func (catalog_client *Catalog_Client) SetCaFile(caFile string) {
	catalog_client.caFile = caFile
}

////////////////////////// API ////////////////////////
// Stop the service.
func (catalog_client *Catalog_Client) StopService() {
	catalog_client.c.Stop(globular.GetClientContext(catalog_client), &catalogpb.StopRequest{})
}

// Create a new datastore connection.
func (catalog_client *Catalog_Client) CreateConnection(connectionId string, name string, host string, port float64, storeType float64, user string, pwd string, timeout float64, options string) error {
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

	_, err := catalog_client.c.CreateConnection(globular.GetClientContext(catalog_client), rqst)
	return err
}

/**
 * Create a new unit of measure
 */
func (catalog_client *Catalog_Client) SaveUnitOfMesure(connectionId string, id string, languageCode string, name string, abreviation string, description string) error {
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

	_, err := catalog_client.c.SaveUnitOfMeasure(globular.GetClientContext(catalog_client), rqst)
	if err != nil {
		return err
	}

	return nil
}

/**
 * Save item property definition.
 */
func (catalog_client *Catalog_Client) SavePropertyDefinition(connectionId string, id string, languageCode string, name string, abreviation string, description string, valueType float64) error {
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

	_, err := catalog_client.c.SavePropertyDefinition(globular.GetClientContext(catalog_client), rqst)
	if err != nil {
		return err
	}

	return nil
}

/**
 * Save item property definition.
 */
func (catalog_client *Catalog_Client) SaveItemDefinition(connectionId string, id string, languageCode string, name string, abreviation string, description string, properties_str string, properties_ids_str string) error {

	properties := new(catalogpb.References)

	jsonpb.UnmarshalString(properties_str, properties)
	jsonpb.UnmarshalString(properties_ids_str, properties)

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

	_, err := catalog_client.c.SaveItemDefinition(globular.GetClientContext(catalog_client), rqst)
	if err != nil {
		return err
	}

	return nil
}

/**
 * Save item property definition.
 */
func (catalog_client *Catalog_Client) SaveItemInstance(connectionId string, jsonStr string) error {

	instance := new(catalogpb.ItemInstance)

	err := jsonpb.UnmarshalString(jsonStr, instance)
	if err != nil {
		return err
	}

	rqst := &catalogpb.SaveItemInstanceRequest{
		ItemInstance: instance,
		ConnectionId: connectionId,
	}

	_, err = catalog_client.c.SaveItemInstance(globular.GetClientContext(catalog_client), rqst)
	if err != nil {
		return err
	}

	return nil
}

/**
 * Save a Manufacturer whitout item.
 */
func (catalog_client *Catalog_Client) SaveManufacturer(connectionId string, id string, name string) error {
	rqst := &catalogpb.SaveManufacturerRequest{
		ConnectionId: connectionId,
		Manufacturer: &catalogpb.Manufacturer{
			Id:   id,
			Name: name,
		},
	}

	_, err := catalog_client.c.SaveManufacturer(globular.GetClientContext(catalog_client), rqst)

	return err
}

/**
 * Save package, create it if it not already exist.
 * TODO append subPackage param and itemInstance...
 */
func (catalog_client *Catalog_Client) SavePackage(connectionId string, id string, name string, languageCode string, description string, inventories []*catalogpb.Inventory) error {

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

	_, err := catalog_client.c.SavePackage(globular.GetClientContext(catalog_client), rqst)

	return err
}

/**
 * Save a supplier.
 */
func (catalog_client *Catalog_Client) SaveSupplier(connectionId string, id string, name string) error {
	rqst := &catalogpb.SaveSupplierRequest{
		ConnectionId: connectionId,
		Supplier: &catalogpb.Supplier{
			Id:   id,
			Name: name,
		},
	}

	_, err := catalog_client.c.SaveSupplier(globular.GetClientContext(catalog_client), rqst)

	return err
}

/**
 * Save package supplier.
 */
func (catalog_client *Catalog_Client) SavePackageSupplier(connectionId string, id string, supplier_ref_str string, packege_ref_str string, price_str string, date int64, qty int64) error {

	// Supplier Ref.
	supplierRef := new(catalogpb.Reference)
	err := jsonpb.UnmarshalString(supplier_ref_str, supplierRef)
	if err != nil {
		return err
	}

	// Pacakge Ref.
	packageRef := new(catalogpb.Reference)
	err = jsonpb.UnmarshalString(packege_ref_str, packageRef)
	if err != nil {
		return err
	}

	price := new(catalogpb.Price)
	err = jsonpb.UnmarshalString(price_str, price)
	if err != nil {
		return err
	}

	rqst := new(catalogpb.SavePackageSupplierRequest)
	rqst.ConnectionId = connectionId
	rqst.PackageSupplier = &catalogpb.PackageSupplier{Id: id, Supplier: supplierRef, Package: packageRef, Price: price, Date: date, Quantity: qty}

	_, err = catalog_client.c.SavePackageSupplier(globular.GetClientContext(catalog_client), rqst)
	return err
}

/**
 * Save Item Manufacturer.
 */
func (catalog_client *Catalog_Client) SaveItemManufacturer(connectionId string, id string, manufacturer_ref_str string, item_ref_str string) error {

	// Supplier Ref.
	manufacturerRef := new(catalogpb.Reference)
	err := jsonpb.UnmarshalString(manufacturer_ref_str, manufacturerRef)
	if err != nil {
		return err
	}

	// Item Ref.
	itemRef := new(catalogpb.Reference)
	err = jsonpb.UnmarshalString(item_ref_str, itemRef)
	if err != nil {
		return err
	}

	rqst := new(catalogpb.SaveItemManufacturerRequest)
	rqst.ConnectionId = connectionId
	rqst.ItemManafacturer = &catalogpb.ItemManufacturer{Id: id, Manufacturer: manufacturerRef, Item: itemRef}

	_, err = catalog_client.c.SaveItemManufacturer(globular.GetClientContext(catalog_client), rqst)
	return err
}

/**
 * Save Item Manufacturer.
 */
func (catalog_client *Catalog_Client) SaveCategory(connectionId string, id string, name string, languageCode string, categories_str string) error {
	categories := new(catalogpb.References)
	jsonpb.UnmarshalString(categories_str, categories)

	rqst := &catalogpb.SaveCategoryRequest{
		ConnectionId: connectionId,
		Category: &catalogpb.Category{
			Id:           id,
			Name:         name,
			LanguageCode: languageCode,
			Categories:   categories,
		},
	}

	_, err := catalog_client.c.SaveCategory(globular.GetClientContext(catalog_client), rqst)
	return err
}

/**
 * Appen item defintion category.
 */
func (catalog_client *Catalog_Client) AppendItemDefinitionCategory(connectionId string, item_definition_ref_str string, category_ref_str string) error {
	// The item definition reference.
	itemDefinitionRef := new(catalogpb.Reference)
	err := jsonpb.UnmarshalString(item_definition_ref_str, itemDefinitionRef)
	if err != nil {
		return err
	}

	// The category reference.
	categoryRef := new(catalogpb.Reference)
	err = jsonpb.UnmarshalString(category_ref_str, categoryRef)
	if err != nil {
		return err
	}

	rqst := &catalogpb.AppendItemDefinitionCategoryRequest{
		ConnectionId:   connectionId,
		ItemDefinition: itemDefinitionRef,
		Category:       categoryRef,
	}

	_, err = catalog_client.c.AppendItemDefinitionCategory(globular.GetClientContext(catalog_client), rqst)

	return err
}

/**
 * Remove item defintion category.
 */
func (catalog_client *Catalog_Client) RemoveItemDefinitionCategory(connectionId string, item_definition_ref_str string, category_ref_str string) error {
	// The item definition reference.
	itemDefinitionRef := new(catalogpb.Reference)
	err := jsonpb.UnmarshalString(item_definition_ref_str, itemDefinitionRef)
	if err != nil {
		return err
	}

	// The category reference.
	categoryRef := new(catalogpb.Reference)
	err = jsonpb.UnmarshalString(category_ref_str, categoryRef)
	if err != nil {
		return err
	}

	rqst := &catalogpb.RemoveItemDefinitionCategoryRequest{
		ConnectionId:   connectionId,
		ItemDefinition: itemDefinitionRef,
		Category:       categoryRef,
	}

	_, err = catalog_client.c.RemoveItemDefinitionCategory(globular.GetClientContext(catalog_client), rqst)

	return err
}

/**
 * Save Item Localisation.
 */
func (catalog_client *Catalog_Client) SaveLocalisation(connectionId string, localisation *catalogpb.Localisation) error {

	rqst := &catalogpb.SaveLocalisationRequest{
		ConnectionId: connectionId,
		Localisation: localisation,
	}

	_, err := catalog_client.c.SaveLocalisation(globular.GetClientContext(catalog_client), rqst)
	return err
}
