package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/globulario/services/golang/catalog/catalogpb"
	Utility "github.com/globulario/utility"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
)

// ----- Service method implementations (RPC handlers) -----

func (srv *server) Stop(context.Context, *catalogpb.StopRequest) (*catalogpb.StopResponse, error) {
	return &catalogpb.StopResponse{}, srv.StopService()
}

// Create a new connection.
func (srv *server) CreateConnection(ctx context.Context, rqst *catalogpb.CreateConnectionRqst) (*catalogpb.CreateConnectionRsp, error) {
	if rqst.Connection == nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection information found in the request!")))
	}

	// So here I will call the function on the client.
	if srv.persistenceClient != nil {
		persistence := srv.Services["Persistence"].(map[string]interface{})
		if persistence["Connections"] == nil {
			persistence["Connections"] = make(map[string]interface{})
		}

		connections := persistence["Connections"].(map[string]interface{})

		storeType := int32(rqst.Connection.GetStore())
		err := srv.persistenceClient.CreateConnection(
			rqst.Connection.GetId(),
			rqst.Connection.GetName(),
			rqst.Connection.GetHost(),
			Utility.ToNumeric(rqst.Connection.Port),
			Utility.ToNumeric(storeType),
			rqst.Connection.GetUser(),
			rqst.Connection.GetPassword(),
			Utility.ToNumeric(rqst.Connection.GetTimeout()),
			rqst.Connection.GetOptions(),
			false)
		if err != nil {
			return nil, status.Errorf(codes.Internal,
				"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}

		connection := make(map[string]interface{})
		connection["Id"] = rqst.Connection.GetId()
		connection["Name"] = rqst.Connection.GetName()
		connection["Host"] = rqst.Connection.GetHost()
		connection["Store"] = rqst.Connection.GetStore()
		connection["User"] = rqst.Connection.GetUser()
		connection["Password"] = rqst.Connection.GetPassword()
		connection["Port"] = rqst.Connection.GetPort()
		connection["Timeout"] = rqst.Connection.GetTimeout()
		connection["Options"] = rqst.Connection.GetOptions()

		connections[rqst.Connection.GetId()] = connection

		_ = srv.Save()
	}

	return &catalogpb.CreateConnectionRsp{Result: true}, nil
}

// Delete a connection.
func (srv *server) DeleteConnection(ctx context.Context, rqst *catalogpb.DeleteConnectionRqst) (*catalogpb.DeleteConnectionRsp, error) {
	// Not implemented in original code; keep as stub.
	return nil, nil
}

// ---- Save* methods ----

func (srv *server) SaveUnitOfMeasure(ctx context.Context, rqst *catalogpb.SaveUnitOfMeasureRequest) (*catalogpb.SaveUnitOfMeasureResponse, error) {
	unitOfMeasure := rqst.GetUnitOfMeasure()

	persistence := srv.Services["Persistence"].(map[string]interface{})

	if persistence["Connections"].(map[string]interface{})[rqst.ConnectionId] == nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection found with id "+rqst.ConnectionId)))
	}

	connection := persistence["Connections"].(map[string]interface{})[rqst.ConnectionId].(map[string]interface{})

	// Here I will generate the _id key
	_id := Utility.GenerateUUID(unitOfMeasure.Id + unitOfMeasure.LanguageCode)
	srv.persistenceClient.DeleteOne(connection["Id"].(string), connection["Name"].(string), "UnitOfMeasure", `{ "_id" : "`+_id+`" }`, "")

	data, err := protojson.Marshal(unitOfMeasure)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	jsonStr := `{ "_id" : "` + _id + `",` + string(data)[1:]

	id, err := srv.persistenceClient.InsertOne(connection["Id"].(string), connection["Name"].(string), "UnitOfMeasure", jsonStr, "")
	if err != nil {
		return nil, status.Errorf(codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &catalogpb.SaveUnitOfMeasureResponse{Id: id}, nil
}

func (srv *server) SavePropertyDefinition(ctx context.Context, rqst *catalogpb.SavePropertyDefinitionRequest) (*catalogpb.SavePropertyDefinitionResponse, error) {
	propertyDefinition := rqst.PropertyDefinition

	persistence := srv.Services["Persistence"].(map[string]interface{})
	if persistence["Connections"].(map[string]interface{})[rqst.ConnectionId] == nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection found with id "+rqst.ConnectionId)))
	}
	connection := persistence["Connections"].(map[string]interface{})[rqst.ConnectionId].(map[string]interface{})

	_id := Utility.GenerateUUID(propertyDefinition.Id + propertyDefinition.LanguageCode)

	data, err := protojson.Marshal(propertyDefinition)
	if err != nil {
		return nil, status.Errorf(codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	jsonStr := `{ "_id" : "` + _id + `",` + string(data)[1:]
	err = srv.persistenceClient.ReplaceOne(connection["Id"].(string), connection["Name"].(string), "PropertyDefinition", `{ "_id" : "`+_id+`"}`, jsonStr, `[{"upsert": true}]`)
	if err != nil {
		return nil, status.Errorf(codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &catalogpb.SavePropertyDefinitionResponse{Id: _id}, nil
}

func (srv *server) SaveItemDefinition(ctx context.Context, rqst *catalogpb.SaveItemDefinitionRequest) (*catalogpb.SaveItemDefinitionResponse, error) {
	itemDefinition := rqst.ItemDefinition

	persistence := srv.Services["Persistence"].(map[string]interface{})
	if persistence["Connections"].(map[string]interface{})[rqst.ConnectionId] == nil {
		return nil, status.Errorf(codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection found with id "+rqst.ConnectionId)))
	}
	connection := persistence["Connections"].(map[string]interface{})[rqst.ConnectionId].(map[string]interface{})

	_id := Utility.GenerateUUID(itemDefinition.Id + itemDefinition.LanguageCode)

	data, err := protojson.Marshal(itemDefinition)
	if err != nil {
		return nil, status.Errorf(codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	jsonStr := `{ "_id" : "` + _id + `",` + string(data)[1:]

	// Set the db reference.
	jsonStr = strings.Replace(jsonStr, "refColId", "$ref", -1)
	jsonStr = strings.Replace(jsonStr, "refObjId", "$id", -1)
	jsonStr = strings.Replace(jsonStr, "refDbName", "$db", -1)

	err = srv.persistenceClient.ReplaceOne(connection["Id"].(string), connection["Name"].(string), "ItemDefinition", `{ "_id" : "`+_id+`"}`, jsonStr, `[{"upsert": true}]`)
	if err != nil {
		return nil, status.Errorf(codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &catalogpb.SaveItemDefinitionResponse{Id: _id}, nil
}

func (srv *server) SaveInventory(ctx context.Context, rqst *catalogpb.SaveInventoryRequest) (*catalogpb.SaveInventoryResponse, error) {
	inventory := rqst.Inventory
	persistence := srv.Services["Persistence"].(map[string]interface{})

	if persistence["Connections"].(map[string]interface{})[rqst.ConnectionId] == nil {
		return nil, status.Errorf(codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection found with id "+rqst.ConnectionId)))
	}
	connection := persistence["Connections"].(map[string]interface{})[rqst.ConnectionId].(map[string]interface{})

	data, err := protojson.Marshal(inventory)
	if err != nil {
		return nil, status.Errorf(codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	_id := Utility.GenerateUUID(inventory.LocalisationId + inventory.PacakgeId)
	jsonStr := `{ "_id" : "` + _id + `",` + string(data)[1:]

	jsonStr = strings.Replace(jsonStr, "refColId", "$ref", -1)
	jsonStr = strings.Replace(jsonStr, "refObjId", "$id", -1)
	jsonStr = strings.Replace(jsonStr, "refDbName", "$db", -1)

	err = srv.persistenceClient.ReplaceOne(connection["Id"].(string), connection["Name"].(string), "Inventory", `{ "_id" : "`+_id+`"}`, jsonStr, `[{"upsert": true}]`)
	if err != nil {
		return nil, status.Errorf(codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &catalogpb.SaveInventoryResponse{Id: _id}, nil
}

func (srv *server) SaveItemInstance(ctx context.Context, rqst *catalogpb.SaveItemInstanceRequest) (*catalogpb.SaveItemInstanceResponse, error) {
	instance := rqst.ItemInstance
	persistence := srv.Services["Persistence"].(map[string]interface{})

	if persistence["Connections"].(map[string]interface{})[rqst.ConnectionId] == nil {
		return nil, status.Errorf(codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection found with id "+rqst.ConnectionId)))
	}
	connection := persistence["Connections"].(map[string]interface{})[rqst.ConnectionId].(map[string]interface{})

	data, err := protojson.Marshal(instance)
	if err != nil {
		return nil, status.Errorf(codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	_id := Utility.GenerateUUID(instance.Id)
	jsonStr := `{ "_id" : "` + _id + `",` + string(data)[1:]

	jsonStr = strings.Replace(jsonStr, "refColId", "$ref", -1)
	jsonStr = strings.Replace(jsonStr, "refObjId", "$id", -1)
	jsonStr = strings.Replace(jsonStr, "refDbName", "$db", -1)

	err = srv.persistenceClient.ReplaceOne(connection["Id"].(string), connection["Name"].(string), "ItemInstance", `{ "_id" : "`+_id+`"}`, jsonStr, `[{"upsert": true}]`)
	if err != nil {
		return nil, status.Errorf(codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &catalogpb.SaveItemInstanceResponse{Id: _id}, nil
}

func (srv *server) SaveManufacturer(ctx context.Context, rqst *catalogpb.SaveManufacturerRequest) (*catalogpb.SaveManufacturerResponse, error) {
	persistence := srv.Services["Persistence"].(map[string]interface{})
	if persistence["Connections"].(map[string]interface{})[rqst.ConnectionId] == nil {
		return nil, status.Errorf(codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection found with id "+rqst.ConnectionId)))
	}
	connection := persistence["Connections"].(map[string]interface{})[rqst.ConnectionId].(map[string]interface{})

	manufacturer := rqst.Manufacturer
	_id := Utility.GenerateUUID(manufacturer.Id)
	data, err := protojson.Marshal(manufacturer)
	if err != nil {
		return nil, status.Errorf(codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	jsonStr := `{ "_id" : "` + _id + `",` + string(data)[1:]
	err = srv.persistenceClient.ReplaceOne(connection["Id"].(string), connection["Name"].(string), "Manufacturer", `{ "_id" : "`+_id+`"}`, jsonStr, `[{"upsert": true}]`)
	if err != nil {
		return nil, status.Errorf(codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &catalogpb.SaveManufacturerResponse{Id: _id}, nil
}

func (srv *server) SaveSupplier(ctx context.Context, rqst *catalogpb.SaveSupplierRequest) (*catalogpb.SaveSupplierResponse, error) {
	persistence := srv.Services["Persistence"].(map[string]interface{})
	if persistence["Connections"].(map[string]interface{})[rqst.ConnectionId] == nil {
		return nil, status.Errorf(codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection found with id "+rqst.ConnectionId)))
	}
	connection := persistence["Connections"].(map[string]interface{})[rqst.ConnectionId].(map[string]interface{})

	supplier := rqst.Supplier
	_id := Utility.GenerateUUID(supplier.Id)
	data, err := protojson.Marshal(supplier)
	if err != nil {
		return nil, status.Errorf(codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	jsonStr := `{ "_id" : "` + _id + `",` + string(data)[1:]
	err = srv.persistenceClient.ReplaceOne(connection["Id"].(string), connection["Name"].(string), "Supplier", `{ "_id" : "`+_id+`"}`, jsonStr, `[{"upsert": true}]`)
	if err != nil {
		return nil, status.Errorf(codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &catalogpb.SaveSupplierResponse{Id: _id}, nil
}

func (srv *server) SaveLocalisation(ctx context.Context, rqst *catalogpb.SaveLocalisationRequest) (*catalogpb.SaveLocalisationResponse, error) {
	persistence := srv.Services["Persistence"].(map[string]interface{})
	if persistence["Connections"].(map[string]interface{})[rqst.ConnectionId] == nil {
		return nil, status.Errorf(codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection found with id "+rqst.ConnectionId)))
	}
	connection := persistence["Connections"].(map[string]interface{})[rqst.ConnectionId].(map[string]interface{})

	localisation := rqst.Localisation
	if localisation.SubLocalisations != nil {
		for i := 0; i < len(localisation.SubLocalisations.Values); i++ {
			if !Utility.IsUuid(localisation.SubLocalisations.Values[i].GetRefObjId()) {
				localisation.SubLocalisations.Values[i].RefObjId = Utility.GenerateUUID(localisation.SubLocalisations.Values[i].GetRefObjId())
			}
		}
	}

	_id := Utility.GenerateUUID(localisation.Id + localisation.LanguageCode)
	data, err := protojson.Marshal(localisation)
	if err != nil {
		return nil, status.Errorf(codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	jsonStr := `{ "_id" : "` + _id + `",` + string(data)[1:]

	jsonStr = strings.Replace(jsonStr, "refObjId", "$id", -1)
	jsonStr = strings.Replace(jsonStr, "refColId", "$ref", -1)
	jsonStr = strings.Replace(jsonStr, "refDbName", "$db", -1)

	err = srv.persistenceClient.ReplaceOne(connection["Id"].(string), connection["Name"].(string), "Localisation", `{ "_id" : "`+_id+`"}`, jsonStr, `[{"upsert": true}]`)
	if err != nil {
		return nil, status.Errorf(codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &catalogpb.SaveLocalisationResponse{Id: _id}, nil
}

func (srv *server) SavePackage(ctx context.Context, rqst *catalogpb.SavePackageRequest) (*catalogpb.SavePackageResponse, error) {
	persistence := srv.Services["Persistence"].(map[string]interface{})
	if persistence["Connections"].(map[string]interface{})[rqst.ConnectionId] == nil {
		return nil, status.Errorf(codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection found with id "+rqst.ConnectionId)))
	}
	connection := persistence["Connections"].(map[string]interface{})[rqst.ConnectionId].(map[string]interface{})

	package_ := rqst.Package

	for i := 0; i < len(package_.Subpackages); i++ {
		subPackage := package_.Subpackages[i]
		if subPackage.UnitOfMeasure != nil {
			if !Utility.IsUuid(subPackage.UnitOfMeasure.RefObjId) {
				subPackage.UnitOfMeasure.RefObjId = Utility.GenerateUUID(subPackage.UnitOfMeasure.RefObjId)
			}
		}
		if subPackage.Package != nil {
			if !Utility.IsUuid(subPackage.Package.RefObjId) {
				subPackage.Package.RefObjId = Utility.GenerateUUID(subPackage.Package.RefObjId)
			}
		}
	}
	for i := 0; i < len(package_.ItemInstances); i++ {
		itemInstance := package_.ItemInstances[i]
		if itemInstance.UnitOfMeasure != nil {
			if !Utility.IsUuid(itemInstance.UnitOfMeasure.RefObjId) {
				itemInstance.UnitOfMeasure.RefObjId = Utility.GenerateUUID(itemInstance.UnitOfMeasure.RefObjId)
			}
		}
		if !Utility.IsUuid(itemInstance.ItemInstance.RefObjId) {
			itemInstance.ItemInstance.RefObjId = Utility.GenerateUUID(itemInstance.ItemInstance.RefObjId)
		}
	}

	_id := Utility.GenerateUUID(package_.Id + package_.LanguageCode)
	data, err := protojson.Marshal(package_)
	if err != nil {
		return nil, status.Errorf(codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	jsonStr := `{ "_id" : "` + _id + `",` + string(data)[1:]

	jsonStr = strings.Replace(jsonStr, "refObjId", "$id", -1)
	jsonStr = strings.Replace(jsonStr, "refColId", "$ref", -1)
	jsonStr = strings.Replace(jsonStr, "refDbName", "$db", -1)

	err = srv.persistenceClient.ReplaceOne(connection["Id"].(string), connection["Name"].(string), "Package", `{ "_id" : "`+_id+`"}`, jsonStr, `[{"upsert": true}]`)
	if err != nil {
		return nil, status.Errorf(codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &catalogpb.SavePackageResponse{Id: _id}, nil
}

func (srv *server) SavePackageSupplier(ctx context.Context, rqst *catalogpb.SavePackageSupplierRequest) (*catalogpb.SavePackageSupplierResponse, error) {
	persistence := srv.Services["Persistence"].(map[string]interface{})
	if persistence["Connections"].(map[string]interface{})[rqst.ConnectionId] == nil {
		return nil, status.Errorf(codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection found with id "+rqst.ConnectionId)))
	}
	connection := persistence["Connections"].(map[string]interface{})[rqst.ConnectionId].(map[string]interface{})

	packageSupplier := rqst.PackageSupplier
	if !Utility.IsUuid(packageSupplier.Supplier.RefObjId) {
		packageSupplier.Supplier.RefObjId = Utility.GenerateUUID(packageSupplier.Supplier.RefObjId)
	}
	if !Utility.IsUuid(packageSupplier.Package.RefObjId) {
		packageSupplier.Package.RefObjId = Utility.GenerateUUID(packageSupplier.Package.RefObjId)
	}

	// ensure both referenced docs exist
	if _, err := srv.persistenceClient.FindOne(connection["Id"].(string), rqst.PackageSupplier.Package.RefDbName, rqst.PackageSupplier.Package.RefColId, `{"_id":"`+rqst.PackageSupplier.Package.RefObjId+`"}`, ""); err != nil {
		return nil, status.Errorf(codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	if _, err := srv.persistenceClient.FindOne(connection["Id"].(string), rqst.PackageSupplier.Supplier.RefDbName, rqst.PackageSupplier.Supplier.RefColId, `{"_id":"`+rqst.PackageSupplier.Supplier.RefObjId+`"}`, ""); err != nil {
		return nil, status.Errorf(codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	_id := Utility.GenerateUUID(packageSupplier.Id)
	data, err := protojson.Marshal(packageSupplier)
	if err != nil {
		return nil, status.Errorf(codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	jsonStr := `{ "_id" : "` + _id + `",` + string(data)[1:]

	jsonStr = strings.Replace(jsonStr, "refObjId", "$id", -1)
	jsonStr = strings.Replace(jsonStr, "refColId", "$ref", -1)
	jsonStr = strings.Replace(jsonStr, "refDbName", "$db", -1)

	err = srv.persistenceClient.ReplaceOne(connection["Id"].(string), connection["Name"].(string), "PackageSupplier", `{ "_id" : "`+_id+`"}`, jsonStr, `[{"upsert": true}]`)
	if err != nil {
		return nil, status.Errorf(codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &catalogpb.SavePackageSupplierResponse{Id: _id}, nil
}

func (srv *server) SaveItemManufacturer(ctx context.Context, rqst *catalogpb.SaveItemManufacturerRequest) (*catalogpb.SaveItemManufacturerResponse, error) {
	persistence := srv.Services["Persistence"].(map[string]interface{})
	if persistence["Connections"].(map[string]interface{})[rqst.ConnectionId] == nil {
		return nil, status.Errorf(codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection found with id "+rqst.ConnectionId)))
	}
	connection := persistence["Connections"].(map[string]interface{})[rqst.ConnectionId].(map[string]interface{})

	if _, err := srv.persistenceClient.FindOne(connection["Id"].(string), rqst.ItemManafacturer.Item.RefDbName, rqst.ItemManafacturer.Item.RefColId, `{"_id":"`+rqst.ItemManafacturer.Item.RefObjId+`"}`, ""); err != nil {
		return nil, status.Errorf(codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	if _, err := srv.persistenceClient.FindOne(connection["Id"].(string), rqst.ItemManafacturer.Manufacturer.RefDbName, rqst.ItemManafacturer.Manufacturer.RefColId, `{"_id":"`+rqst.ItemManafacturer.Manufacturer.RefObjId+`"}`, ""); err != nil {
		return nil, status.Errorf(codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	itemManafacturer := rqst.ItemManafacturer
	_id := Utility.GenerateUUID(itemManafacturer.Id)
	data, err := protojson.Marshal(itemManafacturer)
	if err != nil {
		return nil, status.Errorf(codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	jsonStr := `{ "_id" : "` + _id + `",` + string(data)[1:]

	jsonStr = strings.Replace(jsonStr, "refObjId", "$id", -1)
	jsonStr = strings.Replace(jsonStr, "refColId", "$ref", -1)
	jsonStr = strings.Replace(jsonStr, "refDbName", "$db", -1)

	err = srv.persistenceClient.ReplaceOne(connection["Id"].(string), connection["Name"].(string), "ItemManufacturer", `{ "_id" : "`+_id+`"}`, jsonStr, `[{"upsert": true}]`)
	if err != nil {
		return nil, status.Errorf(codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &catalogpb.SaveItemManufacturerResponse{Id: _id}, nil
}

func (srv *server) SaveCategory(ctx context.Context, rqst *catalogpb.SaveCategoryRequest) (*catalogpb.SaveCategoryResponse, error) {
	persistence := srv.Services["Persistence"].(map[string]interface{})
	if persistence["Connections"].(map[string]interface{})[rqst.ConnectionId] == nil {
		return nil, status.Errorf(codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection found with id "+rqst.ConnectionId)))
	}
	connection := persistence["Connections"].(map[string]interface{})[rqst.ConnectionId].(map[string]interface{})

	category := rqst.Category
	_id := Utility.GenerateUUID(category.Id + category.LanguageCode)
	data, err := protojson.Marshal(category)
	if err != nil {
		return nil, status.Errorf(codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	jsonStr := `{ "_id" : "` + _id + `",` + string(data)[1:]
	err = srv.persistenceClient.ReplaceOne(connection["Id"].(string), connection["Name"].(string), "Category", `{ "_id" : "`+_id+`"}`, jsonStr, `[{"upsert": true}]`)
	if err != nil {
		return nil, status.Errorf(codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &catalogpb.SaveCategoryResponse{Id: _id}, nil
}

func (srv *server) AppendItemDefinitionCategory(ctx context.Context, rqst *catalogpb.AppendItemDefinitionCategoryRequest) (*catalogpb.AppendItemDefinitionCategoryResponse, error) {
	persistence := srv.Services["Persistence"].(map[string]interface{})
	if persistence["Connections"].(map[string]interface{})[rqst.ConnectionId] == nil {
		return nil, status.Errorf(codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection found with id "+rqst.ConnectionId)))
	}
	connection := persistence["Connections"].(map[string]interface{})[rqst.ConnectionId].(map[string]interface{})

	data, err := protojson.Marshal(rqst.Category)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	jsonStr := string(data)
	jsonStr = strings.Replace(jsonStr, "refObjId", "$id", -1)
	jsonStr = strings.Replace(jsonStr, "refColId", "$ref", -1)
	jsonStr = strings.Replace(jsonStr, "refDbName", "$db", -1)
	jsonStr = `{ "$push": { "categories":` + jsonStr + `}}`

	err = srv.persistenceClient.UpdateOne(connection["Id"].(string), connection["Name"].(string), rqst.ItemDefinition.RefColId, `{ "_id" : "`+rqst.ItemDefinition.RefObjId+`"}`, jsonStr, `[]`)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &catalogpb.AppendItemDefinitionCategoryResponse{Result: true}, nil
}

func (srv *server) RemoveItemDefinitionCategory(ctx context.Context, rqst *catalogpb.RemoveItemDefinitionCategoryRequest) (*catalogpb.RemoveItemDefinitionCategoryResponse, error) {
	persistence := srv.Services["Persistence"].(map[string]interface{})
	if persistence["Connections"].(map[string]interface{})[rqst.ConnectionId] == nil {
		return nil, status.Errorf(codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection found with id "+rqst.ConnectionId)))
	}
	connection := persistence["Connections"].(map[string]interface{})[rqst.ConnectionId].(map[string]interface{})

	data, err := protojson.Marshal(rqst.Category)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	jsonStr := string(data)
	jsonStr = strings.Replace(jsonStr, "refObjId", "$id", -1)
	jsonStr = strings.Replace(jsonStr, "refColId", "$ref", -1)
	jsonStr = strings.Replace(jsonStr, "refDbName", "$db", -1)
	jsonStr = `{ "$pull": { "categories":` + jsonStr + `}}`

	err = srv.persistenceClient.UpdateOne(connection["Id"].(string), connection["Name"].(string), rqst.ItemDefinition.RefColId, `{ "_id" : "`+rqst.ItemDefinition.RefObjId+`"}`, jsonStr, `[]`)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &catalogpb.RemoveItemDefinitionCategoryResponse{Result: true}, nil
}

// ----- Get* helpers & RPCs -----

func (srv *server) GetItemInstance(ctx context.Context, rqst *catalogpb.GetItemInstanceRequest) (*catalogpb.GetItemInstanceResponse, error) {
	persistence := srv.Services["Persistence"].(map[string]interface{})
	if persistence["Connections"].(map[string]interface{})[rqst.ConnectionId] == nil {
		return nil, status.Errorf(codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection found with id "+rqst.ConnectionId)))
	}
	connection := persistence["Connections"].(map[string]interface{})[rqst.ConnectionId].(map[string]interface{})
	var query string
	if Utility.IsUuid(rqst.ItemInstanceId) {
		query = `{"_id":"` + rqst.ItemInstanceId + `"}`
	} else {
		query = `{"_id":"` + Utility.GenerateUUID(rqst.ItemInstanceId) + `"}`
	}

	obj, err := srv.persistenceClient.FindOne(connection["Id"].(string), connection["Name"].(string), "ItemInstance", query, `[{"Projection":{"_id":0}}]`)
	if err != nil {
		return nil, err
	}
	jsonStr, _ := Utility.ToJson(obj)
	jsonStr = strings.Replace(jsonStr, "$id", "refObjId", -1)
	jsonStr = strings.Replace(jsonStr, "$ref", "refColId", -1)
	jsonStr = strings.Replace(jsonStr, "$db", "refDbName", -1)

	instance := new(catalogpb.ItemInstance)
	if err := protojson.Unmarshal([]byte(jsonStr), instance); err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &catalogpb.GetItemInstanceResponse{ItemInstance: instance}, nil
}

func (srv *server) GetItemInstances(ctx context.Context, rqst *catalogpb.GetItemInstancesRequest) (*catalogpb.GetItemInstancesResponse, error) {
	persistence := srv.Services["Persistence"].(map[string]interface{})
	if persistence["Connections"].(map[string]interface{})[rqst.ConnectionId] == nil {
		return nil, status.Errorf(codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection found with id "+rqst.ConnectionId)))
	}
	connection := persistence["Connections"].(map[string]interface{})[rqst.ConnectionId].(map[string]interface{})

	options, err := srv.getOptionsString(rqst.Options)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	if len(rqst.Query) == 0 {
		rqst.Query = ``
	}

	values, err := srv.persistenceClient.Find(connection["Id"].(string), connection["Name"].(string), "ItemInstance", rqst.Query, options)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	jsonStr, err := Utility.ToJson(&values)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	jsonStr = strings.Replace(jsonStr, "$id", "refObjId", -1)
	jsonStr = strings.Replace(jsonStr, "$ref", "refColId", -1)
	jsonStr = strings.Replace(jsonStr, "$db", "refDbName", -1)
	jsonStr = `{ "itemInstances":` + jsonStr + `}`

	instances := new(catalogpb.ItemInstances)
	if err := protojson.Unmarshal([]byte(jsonStr), instances); err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &catalogpb.GetItemInstancesResponse{ItemInstances: instances.ItemInstances}, nil
}

func (srv *server) GetItemDefinition(ctx context.Context, rqst *catalogpb.GetItemDefinitionRequest) (*catalogpb.GetItemDefinitionResponse, error) {
	persistence := srv.Services["Persistence"].(map[string]interface{})
	if persistence["Connections"].(map[string]interface{})[rqst.ConnectionId] == nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection found with id "+rqst.ConnectionId)))
	}
	connection := persistence["Connections"].(map[string]interface{})[rqst.ConnectionId].(map[string]interface{})
	var query string
	if Utility.IsUuid(rqst.ItemDefinitionId) {
		query = `{"_id":"` + rqst.ItemDefinitionId + `"}`
	} else {
		query = `{"_id":"` + Utility.GenerateUUID(rqst.ItemDefinitionId) + `"}`
	}

	obj, err := srv.persistenceClient.FindOne(connection["Id"].(string), connection["Name"].(string), "ItemDefinition", query, `[{"Projection":{"_id":0}}]`)
	if err != nil {
		return nil, err
	}
	jsonStr, _ := Utility.ToJson(obj)
	jsonStr = strings.Replace(jsonStr, "$id", "refObjId", -1)
	jsonStr = strings.Replace(jsonStr, "$ref", "refColId", -1)
	jsonStr = strings.Replace(jsonStr, "$db", "refDbName", -1)

	definition := new(catalogpb.ItemDefinition)
	if err := protojson.Unmarshal([]byte(jsonStr), definition); err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &catalogpb.GetItemDefinitionResponse{ItemDefinition: definition}, nil
}

func (srv *server) GetInventories(ctx context.Context, rqst *catalogpb.GetInventoriesRequest) (*catalogpb.GetInventoriesResponse, error) {
	persistence := srv.Services["Persistence"].(map[string]interface{})
	if persistence["Connections"].(map[string]interface{})[rqst.ConnectionId] == nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection found with id "+rqst.ConnectionId)))
	}
	connection := persistence["Connections"].(map[string]interface{})[rqst.ConnectionId].(map[string]interface{})

	options, err := srv.getOptionsString(rqst.Options)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	if len(rqst.Query) == 0 {
		rqst.Query = ``
	}
	values, err := srv.persistenceClient.Find(connection["Id"].(string), connection["Name"].(string), "Inventory", rqst.Query, options)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	jsonStr, err := Utility.ToJson(&values)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	jsonStr = strings.Replace(jsonStr, "$id", "refObjId", -1)
	jsonStr = strings.Replace(jsonStr, "$ref", "refColId", -1)
	jsonStr = strings.Replace(jsonStr, "$db", "refDbName", -1)
	jsonStr = `{ "inventories":` + jsonStr + `}`

	inventories := new(catalogpb.Inventories)
	if err := protojson.Unmarshal([]byte(jsonStr), inventories); err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &catalogpb.GetInventoriesResponse{Inventories: inventories.Inventories}, nil
}

func (srv *server) GetItemDefinitions(ctx context.Context, rqst *catalogpb.GetItemDefinitionsRequest) (*catalogpb.GetItemDefinitionsResponse, error) {
	persistence := srv.Services["Persistence"].(map[string]interface{})
	if persistence["Connections"].(map[string]interface{})[rqst.ConnectionId] == nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection found with id "+rqst.ConnectionId)))
	}
	connection := persistence["Connections"].(map[string]interface{})[rqst.ConnectionId].(map[string]interface{})

	options, err := srv.getOptionsString(rqst.Options)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	if len(rqst.Query) == 0 {
		rqst.Query = ``
	}

	values, err := srv.persistenceClient.Find(connection["Id"].(string), connection["Name"].(string), "ItemDefinition", rqst.Query, options)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	jsonStr, err := Utility.ToJson(&values)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	jsonStr = strings.Replace(jsonStr, "$id", "refObjId", -1)
	jsonStr = strings.Replace(jsonStr, "$ref", "refColId", -1)
	jsonStr = strings.Replace(jsonStr, "$db", "refDbName", -1)
	jsonStr = `{ "itemDefinitions":` + jsonStr + `}`

	definitions := new(catalogpb.ItemDefinitions)
	if err := protojson.Unmarshal([]byte(jsonStr), definitions); err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &catalogpb.GetItemDefinitionsResponse{ItemDefinitions: definitions.ItemDefinitions}, nil
}

func (srv *server) GetSupplier(ctx context.Context, rqst *catalogpb.GetSupplierRequest) (*catalogpb.GetSupplierResponse, error) {
	persistence := srv.Services["Persistence"].(map[string]interface{})
	if persistence["Connections"].(map[string]interface{})[rqst.ConnectionId] == nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection found with id "+rqst.ConnectionId)))
	}
	connection := persistence["Connections"].(map[string]interface{})[rqst.ConnectionId].(map[string]interface{})
	var query string
	if Utility.IsUuid(rqst.SupplierId) {
		query = `{"_id":"` + rqst.SupplierId + `"}`
	} else {
		query = `{"_id":"` + Utility.GenerateUUID(rqst.SupplierId) + `"}`
	}

	obj, err := srv.persistenceClient.FindOne(connection["Id"].(string), connection["Name"].(string), "Supplier", query, `[{"Projection":{"_id":0}}]`)
	if err != nil {
		return nil, err
	}
	jsonStr, _ := Utility.ToJson(obj)
	jsonStr = strings.Replace(jsonStr, "$id", "refObjId", -1)
	jsonStr = strings.Replace(jsonStr, "$ref", "refColId", -1)
	jsonStr = strings.Replace(jsonStr, "$db", "refDbName", -1)

	supplier := new(catalogpb.Supplier)
	if err := protojson.Unmarshal([]byte(jsonStr), supplier); err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &catalogpb.GetSupplierResponse{Supplier: supplier}, nil
}

func (srv *server) GetSuppliers(ctx context.Context, rqst *catalogpb.GetSuppliersRequest) (*catalogpb.GetSuppliersResponse, error) {
	persistence := srv.Services["Persistence"].(map[string]interface{})
	if persistence["Connections"].(map[string]interface{})[rqst.ConnectionId] == nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection found with id "+rqst.ConnectionId)))
	}
	connection := persistence["Connections"].(map[string]interface{})[rqst.ConnectionId].(map[string]interface{})

	options, err := srv.getOptionsString(rqst.Options)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	if len(rqst.Query) == 0 {
		rqst.Query = ``
	}
	values, err := srv.persistenceClient.Find(connection["Id"].(string), connection["Name"].(string), "Supplier", rqst.Query, options)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	jsonStr, err := Utility.ToJson(&values)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	jsonStr = strings.Replace(jsonStr, "$id", "refObjId", -1)
	jsonStr = strings.Replace(jsonStr, "$ref", "refColId", -1)
	jsonStr = strings.Replace(jsonStr, "$db", "refDbName", -1)
	jsonStr = `{ "suppliers":` + jsonStr + `}`

	suppliers := new(catalogpb.Suppliers)
	if err := protojson.Unmarshal([]byte(jsonStr), suppliers); err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &catalogpb.GetSuppliersResponse{Suppliers: suppliers.Suppliers}, nil
}

func (srv *server) GetSupplierPackages(ctx context.Context, rqst *catalogpb.GetSupplierPackagesRequest) (*catalogpb.GetSupplierPackagesResponse, error) {
	persistence := srv.Services["Persistence"].(map[string]interface{})
	if persistence["Connections"].(map[string]interface{})[rqst.ConnectionId] == nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection found with id "+rqst.ConnectionId)))
	}
	connection := persistence["Connections"].(map[string]interface{})[rqst.ConnectionId].(map[string]interface{})
	var query string
	if Utility.IsUuid(rqst.SupplierId) {
		query = `{"supplier.$id":"` + rqst.SupplierId + `"}`
	} else {
		query = `{"supplier.$id":"` + Utility.GenerateUUID(rqst.SupplierId) + `"}`
	}

	values, err := srv.persistenceClient.Find(connection["Id"].(string), connection["Name"].(string), "PackageSupplier", query, `[{"Projection":{"_id":1}}]`)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	jsonStr, err := Utility.ToJson(&values)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	results := make([]map[string]interface{}, 0)
	if err := json.Unmarshal([]byte(jsonStr), &results); err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	packagesSupplier := make([]*catalogpb.PackageSupplier, 0)
	for i := 0; i < len(results); i++ {
		obj, err := srv.persistenceClient.FindOne(connection["Id"].(string), connection["Name"].(string), "PackageSupplier", `{"_id":"`+results[i]["_id"].(string)+`"}`, `[{"Projection":{"_id":0}}]`)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}

		jsonStr, _ := Utility.ToJson(obj)
		jsonStr = strings.Replace(jsonStr, "$id", "refObjId", -1)
		jsonStr = strings.Replace(jsonStr, "$ref", "refColId", -1)
		jsonStr = strings.Replace(jsonStr, "$db", "refDbName", -1)

		ps := new(catalogpb.PackageSupplier)
		if err := protojson.Unmarshal([]byte(jsonStr), ps); err != nil {
			return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
		packagesSupplier = append(packagesSupplier, ps)
	}
	return &catalogpb.GetSupplierPackagesResponse{PackagesSupplier: packagesSupplier}, nil
}

func (srv *server) GetPackage(ctx context.Context, rqst *catalogpb.GetPackageRequest) (*catalogpb.GetPackageResponse, error) {
	persistence := srv.Services["Persistence"].(map[string]interface{})
	if persistence["Connections"].(map[string]interface{})[rqst.ConnectionId] == nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection found with id "+rqst.ConnectionId)))
	}
	connection := persistence["Connections"].(map[string]interface{})[rqst.ConnectionId].(map[string]interface{})
	var query string
	if Utility.IsUuid(rqst.PackageId) {
		query = `{"_id":"` + rqst.PackageId + `"}`
	} else {
		query = `{"_id":"` + Utility.GenerateUUID(rqst.PackageId) + `"}`
	}

	obj, err := srv.persistenceClient.FindOne(connection["Id"].(string), connection["Name"].(string), "Package", query, `[{"Projection":{"_id":0}}]`)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	jsonStr, err := Utility.ToJson(obj)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	jsonStr = strings.Replace(jsonStr, "$id", "refObjId", -1)
	jsonStr = strings.Replace(jsonStr, "$ref", "refColId", -1)
	jsonStr = strings.Replace(jsonStr, "$db", "refDbName", -1)

	package_ := new(catalogpb.Package)
	if err := protojson.Unmarshal([]byte(jsonStr), package_); err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &catalogpb.GetPackageResponse{Pacakge: package_}, nil
}

func (srv *server) GetPackages(ctx context.Context, rqst *catalogpb.GetPackagesRequest) (*catalogpb.GetPackagesResponse, error) {
	persistence := srv.Services["Persistence"].(map[string]interface{})
	if persistence["Connections"].(map[string]interface{})[rqst.ConnectionId] == nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection found with id "+rqst.ConnectionId)))
	}
	connection := persistence["Connections"].(map[string]interface{})[rqst.ConnectionId].(map[string]interface{})

	options, err := srv.getOptionsString(rqst.Options)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	values, err := srv.persistenceClient.Find(connection["Id"].(string), connection["Name"].(string), "Package", rqst.Query, options)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	jsonStr, err := Utility.ToJson(&values)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	jsonStr = strings.Replace(jsonStr, "$id", "refObjId", -1)
	jsonStr = strings.Replace(jsonStr, "$ref", "refColId", -1)
	jsonStr = strings.Replace(jsonStr, "$db", "refDbName", -1)
	jsonStr = `{ "packages":` + jsonStr + `}`

	packages := new(catalogpb.Packages)
	if err := protojson.Unmarshal([]byte(jsonStr), packages); err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &catalogpb.GetPackagesResponse{Packages: packages.Packages}, nil
}

func (srv *server) getLocalisation(localisationId string, connectionId string) (*catalogpb.Localisation, error) {
	persistence := srv.Services["Persistence"].(map[string]interface{})
	if persistence["Connections"].(map[string]interface{})[connectionId] == nil {
		return nil, errors.New("no connection found with id " + connectionId)
	}
	connection := persistence["Connections"].(map[string]interface{})[connectionId].(map[string]interface{})

	var query string
	if Utility.IsUuid(localisationId) {
		query = `{"_id":"` + localisationId + `"}`
	} else {
		query = `{"_id":"` + Utility.GenerateUUID(localisationId) + `"}`
	}

	obj, err := srv.persistenceClient.FindOne(connection["Id"].(string), connection["Name"].(string), "Localisation", query, `[{"Projection":{"_id":0}}]`)
	if err != nil {
		return nil, err
	}
	jsonStr, err := Utility.ToJson(obj)
	if err != nil {
		return nil, err
	}
	jsonStr = strings.Replace(jsonStr, "$id", "refObjId", -1)
	jsonStr = strings.Replace(jsonStr, "$ref", "refColId", -1)
	jsonStr = strings.Replace(jsonStr, "$db", "refDbName", -1)

	localisation := new(catalogpb.Localisation)
	if err := protojson.Unmarshal([]byte(jsonStr), localisation); err != nil {
		return nil, err
	}
	return localisation, nil
}

func (srv *server) GetLocalisation(ctx context.Context, rqst *catalogpb.GetLocalisationRequest) (*catalogpb.GetLocalisationResponse, error) {
	localisation, err := srv.getLocalisation(rqst.LocalisationId, rqst.ConnectionId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &catalogpb.GetLocalisationResponse{Localisation: localisation}, nil
}

func (srv *server) getLocalisations(query string, options string, connectionId string) ([]*catalogpb.Localisation, error) {
	persistence := srv.Services["Persistence"].(map[string]interface{})
	if persistence["Connections"].(map[string]interface{})[connectionId] == nil {
		return nil, errors.New("no connection found with id " + connectionId)
	}
	connection := persistence["Connections"].(map[string]interface{})[connectionId].(map[string]interface{})
	options, err := srv.getOptionsString(options)
	if err != nil {
		return nil, err
	}
	if len(query) == 0 {
		query = ``
	}
	values, err := srv.persistenceClient.Find(connection["Id"].(string), connection["Name"].(string), "Localisation", query, options)
	if err != nil {
		return nil, err
	}
	jsonStr, err := Utility.ToJson(&values)
	if err != nil {
		return nil, err
	}
	jsonStr = strings.Replace(jsonStr, "$id", "refObjId", -1)
	jsonStr = strings.Replace(jsonStr, "$ref", "refColId", -1)
	jsonStr = strings.Replace(jsonStr, "$db", "refDbName", -1)
	jsonStr = `{ "localisations":` + jsonStr + `}`

	localisations := new(catalogpb.Localisations)
	if err := protojson.Unmarshal([]byte(jsonStr), localisations); err != nil {
		return nil, err
	}
	return localisations.Localisations, nil
}

func (srv *server) GetLocalisations(ctx context.Context, rqst *catalogpb.GetLocalisationsRequest) (*catalogpb.GetLocalisationsResponse, error) {
	localisations, err := srv.getLocalisations(rqst.Query, rqst.Options, rqst.ConnectionId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &catalogpb.GetLocalisationsResponse{Localisations: localisations}, nil
}

func (srv *server) getCategory(categoryId string, connectionId string) (*catalogpb.Category, error) {
	persistence := srv.Services["Persistence"].(map[string]interface{})
	if persistence["Connections"].(map[string]interface{})[connectionId] == nil {
		return nil, errors.New("no connection found with id " + connectionId)
	}
	connection := persistence["Connections"].(map[string]interface{})[connectionId].(map[string]interface{})

	var query string
	if Utility.IsUuid(categoryId) {
		query = `{"_id":"` + categoryId + `"}`
	} else {
		query = `{"_id":"` + Utility.GenerateUUID(categoryId) + `"}`
	}

	obj, err := srv.persistenceClient.FindOne(connection["Id"].(string), connection["Name"].(string), "Category", query, `[{"Projection":{"_id":0}}]`)
	if err != nil {
		return nil, err
	}
	jsonStr, err := Utility.ToJson(obj)
	if err != nil {
		return nil, err
	}
	jsonStr = strings.Replace(jsonStr, "$id", "refObjId", -1)
	jsonStr = strings.Replace(jsonStr, "$ref", "refColId", -1)
	jsonStr = strings.Replace(jsonStr, "$db", "refDbName", -1)

	category := new(catalogpb.Category)
	if err := protojson.Unmarshal([]byte(jsonStr), category); err != nil {
		return nil, err
	}
	return category, nil
}

func (srv *server) GetCategory(ctx context.Context, rqst *catalogpb.GetCategoryRequest) (*catalogpb.GetCategoryResponse, error) {
	category, err := srv.getCategory(rqst.CategoryId, rqst.ConnectionId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &catalogpb.GetCategoryResponse{Category: category}, nil
}

func (srv *server) getCategories(query string, options string, connectionId string) ([]*catalogpb.Category, error) {
	persistence := srv.Services["Persistence"].(map[string]interface{})
	if persistence["Connections"].(map[string]interface{})[connectionId] == nil {
		return nil, errors.New("no connection found with id " + connectionId)
	}
	connection := persistence["Connections"].(map[string]interface{})[connectionId].(map[string]interface{})
	options, err := srv.getOptionsString(options)
	if err != nil {
		return nil, err
	}
	if len(query) == 0 {
		query = ``
	}
	values, err := srv.persistenceClient.Find(connection["Id"].(string), connection["Name"].(string), "Category", query, options)
	if err != nil {
		return nil, err
	}
	jsonStr, err := Utility.ToJson(&values)
	if err != nil {
		return nil, err
	}
	jsonStr = strings.Replace(jsonStr, "$id", "refObjId", -1)
	jsonStr = strings.Replace(jsonStr, "$ref", "refColId", -1)
	jsonStr = strings.Replace(jsonStr, "$db", "refDbName", -1)
	jsonStr = `{ "categories":` + jsonStr + `}`

	categories := new(catalogpb.Categories)
	if err := protojson.Unmarshal([]byte(jsonStr), categories); err != nil {
		return nil, err
	}
	return categories.Categories, nil
}

func (srv *server) GetCategories(ctx context.Context, rqst *catalogpb.GetCategoriesRequest) (*catalogpb.GetCategoriesResponse, error) {
	categories, err := srv.getCategories(rqst.Query, rqst.Options, rqst.ConnectionId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &catalogpb.GetCategoriesResponse{Categories: categories}, nil
}

func (srv *server) GetManufacturer(ctx context.Context, rqst *catalogpb.GetManufacturerRequest) (*catalogpb.GetManufacturerResponse, error) {
	persistence := srv.Services["Persistence"].(map[string]interface{})
	if persistence["Connections"].(map[string]interface{})[rqst.ConnectionId] == nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection found with id "+rqst.ConnectionId)))
	}
	connection := persistence["Connections"].(map[string]interface{})[rqst.ConnectionId].(map[string]interface{})
	var query string
	if Utility.IsUuid(rqst.ManufacturerId) {
		query = `{"_id":"` + rqst.ManufacturerId + `"}`
	} else {
		query = `{"_id":"` + Utility.GenerateUUID(rqst.ManufacturerId) + `"}`
	}

	obj, err := srv.persistenceClient.FindOne(connection["Id"].(string), connection["Name"].(string), "Package", query, `[{"Projection":{"_id":0}}]`)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	jsonStr, err := Utility.ToJson(obj)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	jsonStr = strings.Replace(jsonStr, "$id", "refObjId", -1)
	jsonStr = strings.Replace(jsonStr, "$ref", "refColId", -1)
	jsonStr = strings.Replace(jsonStr, "$db", "refDbName", -1)

	manufacturer := new(catalogpb.Manufacturer)
	if err := protojson.Unmarshal([]byte(jsonStr), manufacturer); err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &catalogpb.GetManufacturerResponse{Manufacturer: manufacturer}, nil
}

func (srv *server) getOptionsString(options string) (string, error) {
	options_ := make([]map[string]interface{}, 0)
	if len(options) > 0 {
		if err := json.Unmarshal([]byte(options), &options_); err != nil {
			return "", err
		}
		var projections map[string]interface{}
		for i := 0; i < len(options_); i++ {
			if options_[i]["Projection"] != nil {
				projections = options_[i]["Projection"].(map[string]interface{})
				break
			}
		}
		if projections != nil {
			projections["_id"] = 0
		} else {
			options_ = append(options_, map[string]interface{}{"Projection": map[string]interface{}{"_id": 0}})
		}
	} else {
		options_ = append(options_, map[string]interface{}{"Projection": map[string]interface{}{"_id": 0}})
	}
	optionsStr, err := Utility.ToJson(options_)
	return string(optionsStr), err
}

func (srv *server) GetManufacturers(ctx context.Context, rqst *catalogpb.GetManufacturersRequest) (*catalogpb.GetManufacturersResponse, error) {
	persistence := srv.Services["Persistence"].(map[string]interface{})
	if persistence["Connections"].(map[string]interface{})[rqst.ConnectionId] == nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection found with id "+rqst.ConnectionId)))
	}
	connection := persistence["Connections"].(map[string]interface{})[rqst.ConnectionId].(map[string]interface{})

	options, err := srv.getOptionsString(rqst.Options)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	if len(rqst.Query) == 0 {
		rqst.Query = ``
	}

	values, err := srv.persistenceClient.Find(connection["Id"].(string), connection["Name"].(string), "Manufacturer", rqst.Query, options)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	jsonStr, err := Utility.ToJson(&values)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	jsonStr = strings.Replace(jsonStr, "$id", "refObjId", -1)
	jsonStr = strings.Replace(jsonStr, "$ref", "refColId", -1)
	jsonStr = strings.Replace(jsonStr, "$db", "refDbName", -1)
	jsonStr = `{ "manufacturers":` + jsonStr + `}`

	manufacturers := new(catalogpb.Manufacturers)
	if err := protojson.Unmarshal([]byte(jsonStr), manufacturers); err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &catalogpb.GetManufacturersResponse{Manufacturers: manufacturers.Manufacturers}, nil
}

func (srv *server) GetUnitOfMeasures(ctx context.Context, rqst *catalogpb.GetUnitOfMeasuresRequest) (*catalogpb.GetUnitOfMeasuresResponse, error) {
	persistence := srv.Services["Persistence"].(map[string]interface{})
	if persistence["Connections"].(map[string]interface{})[rqst.ConnectionId] == nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection found with id "+rqst.ConnectionId)))
	}
	connection := persistence["Connections"].(map[string]interface{})[rqst.ConnectionId].(map[string]interface{})

	options, err := srv.getOptionsString(rqst.Options)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	if len(rqst.Query) == 0 {
		rqst.Query = ``
	}

	values, err := srv.persistenceClient.Find(connection["Id"].(string), connection["Name"].(string), "UnitOfMeasure", rqst.Query, options)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	jsonStr, err := Utility.ToJson(&values)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	jsonStr = strings.Replace(jsonStr, "$id", "refObjId", -1)
	jsonStr = strings.Replace(jsonStr, "$ref", "refColId", -1)
	jsonStr = strings.Replace(jsonStr, "$db", "refDbName", -1)
	jsonStr = `{ "unitOfMeasures":` + jsonStr + `}`

	unitOfMeasures := new(catalogpb.UnitOfMeasures)
	if err := protojson.Unmarshal([]byte(jsonStr), unitOfMeasures); err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &catalogpb.GetUnitOfMeasuresResponse{UnitOfMeasures: unitOfMeasures.UnitOfMeasures}, nil
}

func (srv *server) GetUnitOfMeasure(ctx context.Context, rqst *catalogpb.GetUnitOfMeasureRequest) (*catalogpb.GetUnitOfMeasureResponse, error) {
	persistence := srv.Services["Persistence"].(map[string]interface{})
	if persistence["Connections"].(map[string]interface{})[rqst.ConnectionId] == nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection found with id "+rqst.ConnectionId)))
	}
	connection := persistence["Connections"].(map[string]interface{})[rqst.ConnectionId].(map[string]interface{})
	var query string
	if Utility.IsUuid(rqst.UnitOfMeasureId) {
		query = `{"_id":"` + rqst.UnitOfMeasureId + `"}`
	} else {
		query = `{"_id":"` + Utility.GenerateUUID(rqst.UnitOfMeasureId) + `"}`
	}

	obj, err := srv.persistenceClient.FindOne(connection["Id"].(string), connection["Name"].(string), "UnitOfMeasure", query, `[{"Projection":{"_id":0}}]`)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	jsonStr, err := Utility.ToJson(obj)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	jsonStr = strings.Replace(jsonStr, "$id", "refObjId", -1)
	jsonStr = strings.Replace(jsonStr, "$ref", "refColId", -1)
	jsonStr = strings.Replace(jsonStr, "$db", "refDbName", -1)

	unitOfMeasure := new(catalogpb.UnitOfMeasure)
	if err := protojson.Unmarshal([]byte(jsonStr), unitOfMeasure); err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &catalogpb.GetUnitOfMeasureResponse{UnitOfMeasure: unitOfMeasure}, nil
}

// ----- Delete* methods -----

func (srv *server) DeletePackage(ctx context.Context, rqst *catalogpb.DeletePackageRequest) (*catalogpb.DeletePackageResponse, error) {
	persistence := srv.Services["Persistence"].(map[string]interface{})
	if persistence["Connections"].(map[string]interface{})[rqst.ConnectionId] == nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection found with id "+rqst.ConnectionId)))
	}
	connection := persistence["Connections"].(map[string]interface{})[rqst.ConnectionId].(map[string]interface{})

	package_ := rqst.Package
	_id := Utility.GenerateUUID(package_.Id + package_.LanguageCode)
	if err := srv.persistenceClient.DeleteOne(connection["Id"].(string), connection["Name"].(string), "Package", `{"_id":"`+_id+`"}`, ""); err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &catalogpb.DeletePackageResponse{Result: true}, nil
}

func (srv *server) DeletePackageSupplier(ctx context.Context, rqst *catalogpb.DeletePackageSupplierRequest) (*catalogpb.DeletePackageSupplierResponse, error) {
	persistence := srv.Services["Persistence"].(map[string]interface{})
	if persistence["Connections"].(map[string]interface{})[rqst.ConnectionId] == nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection found with id "+rqst.ConnectionId)))
	}
	connection := persistence["Connections"].(map[string]interface{})[rqst.ConnectionId].(map[string]interface{})

	packageSupplier := rqst.PackageSupplier
	_id := Utility.GenerateUUID(packageSupplier.Id)
	if err := srv.persistenceClient.DeleteOne(connection["Id"].(string), connection["Name"].(string), "PackageSupplier", `{"_id":"`+_id+`"}`, ""); err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &catalogpb.DeletePackageSupplierResponse{Result: true}, nil
}

func (srv *server) DeleteSupplier(ctx context.Context, rqst *catalogpb.DeleteSupplierRequest) (*catalogpb.DeleteSupplierResponse, error) {
	persistence := srv.Services["Persistence"].(map[string]interface{})
	if persistence["Connections"].(map[string]interface{})[rqst.ConnectionId] == nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection found with id "+rqst.ConnectionId)))
	}
	connection := persistence["Connections"].(map[string]interface{})[rqst.ConnectionId].(map[string]interface{})

	supplier := rqst.Supplier
	_id := Utility.GenerateUUID(supplier.Id)
	if err := srv.persistenceClient.DeleteOne(connection["Id"].(string), connection["Name"].(string), "Supplier", `{"_id":"`+_id+`"}`, ""); err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &catalogpb.DeleteSupplierResponse{Result: true}, nil
}

func (srv *server) DeletePropertyDefinition(ctx context.Context, rqst *catalogpb.DeletePropertyDefinitionRequest) (*catalogpb.DeletePropertyDefinitionResponse, error) {
	// Not implemented in original code; keep as stub.
	return nil, nil
}

func (srv *server) DeleteUnitOfMeasure(ctx context.Context, rqst *catalogpb.DeleteUnitOfMeasureRequest) (*catalogpb.DeleteUnitOfMeasureResponse, error) {
	// Not implemented in original code; keep as stub.
	return nil, nil
}

func (srv *server) DeleteItemInstance(ctx context.Context, rqst *catalogpb.DeleteItemInstanceRequest) (*catalogpb.DeleteItemInstanceResponse, error) {
	// Not implemented in original code; keep as stub.
	return nil, nil
}

func (srv *server) DeleteManufacturer(ctx context.Context, rqst *catalogpb.DeleteManufacturerRequest) (*catalogpb.DeleteManufacturerResponse, error) {
	// Not implemented in original code; keep as stub.
	return nil, nil
}

func (srv *server) DeleteItemManufacturer(ctx context.Context, rqst *catalogpb.DeleteItemManufacturerRequest) (*catalogpb.DeleteItemManufacturerResponse, error) {
	// Not implemented in original code; keep as stub.
	return nil, nil
}

func (srv *server) DeleteCategory(ctx context.Context, rqst *catalogpb.DeleteCategoryRequest) (*catalogpb.DeleteCategoryResponse, error) {
	// Not implemented in original code; keep as stub.
	return nil, nil
}

func (srv *server) deleteLocalisation(localisation *catalogpb.Localisation, connectionId string) error {
	persistence := srv.Services["Persistence"].(map[string]interface{})
	if persistence["Connections"].(map[string]interface{})[connectionId] == nil {
		return errors.New("no connection found with id " + connectionId)
	}
	connection := persistence["Connections"].(map[string]interface{})[connectionId].(map[string]interface{})

	referenced, err := srv.getLocalisations(`{"subLocalisations.values.$id":"`+Utility.GenerateUUID(localisation.GetId()+localisation.GetLanguageCode())+`"}`, "", connectionId)
	if err == nil {
		refStr := `{"$id":"` + Utility.GenerateUUID(localisation.GetId()+localisation.GetLanguageCode()) + `","$ref":"Localisation","$db":"` + connection["Name"].(string) + `"}`
		for i := 0; i < len(referenced); i++ {
			query := `{"$pull":{"subLocalisations.values":` + refStr + `}}`
			_id := Utility.GenerateUUID(referenced[i].Id + referenced[i].LanguageCode)
			if err = srv.persistenceClient.UpdateOne(connection["Id"].(string), connection["Name"].(string), "Localisation", `{"_id" : "`+_id+`"}`, query, `[]`); err != nil {
				return err
			}
		}
	}

	if localisation.GetSubLocalisations() != nil {
		for i := 0; i < len(localisation.GetSubLocalisations().GetValues()); i++ {
			subLocalisation, err := srv.getLocalisation(localisation.GetSubLocalisations().GetValues()[i].GetRefObjId(), connectionId)
			if err == nil {
				if err := srv.deleteLocalisation(subLocalisation, connectionId); err != nil {
					return err
				}
			}
		}
	}

	_id := Utility.GenerateUUID(localisation.Id + localisation.LanguageCode)
	return srv.persistenceClient.DeleteOne(connection["Id"].(string), connection["Name"].(string), "Localisation", `{"_id":"`+_id+`"}`, "")
}

func (srv *server) DeleteLocalisation(ctx context.Context, rqst *catalogpb.DeleteLocalisationRequest) (*catalogpb.DeleteLocalisationResponse, error) {
	if err := srv.deleteLocalisation(rqst.Localisation, rqst.ConnectionId); err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &catalogpb.DeleteLocalisationResponse{Result: true}, nil
}

func (srv *server) DeleteInventory(ctx context.Context, rqst *catalogpb.DeleteInventoryRequest) (*catalogpb.DeleteInventoryResponse, error) {
	persistence := srv.Services["Persistence"].(map[string]interface{})
	if persistence["Connections"].(map[string]interface{})[rqst.ConnectionId] == nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection found with id "+rqst.ConnectionId)))
	}
	connection := persistence["Connections"].(map[string]interface{})[rqst.ConnectionId].(map[string]interface{})

	inventory := rqst.Inventory
	_id := Utility.GenerateUUID(inventory.LocalisationId + inventory.PacakgeId)
	if err := srv.persistenceClient.DeleteOne(connection["Id"].(string), connection["Name"].(string), "Inventory", `{"_id":"`+_id+`"}`, ""); err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &catalogpb.DeleteInventoryResponse{Result: true}, nil
}
