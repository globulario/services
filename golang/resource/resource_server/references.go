package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/persistence/persistence_store"
	"github.com/globulario/services/golang/resource/resourcepb"
	Utility "github.com/globulario/utility"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)


func (srv *server) deleteReference(p persistence_store.Store, refId, targetId, targetField, targetCollection string) error {

	if strings.Contains(targetId, "@") {
		domain := strings.Split(targetId, "@")[1]
		targetId = strings.Split(targetId, "@")[0]

		localDomain, err := config.GetDomain()
		if err != nil {
			return err
		}

		if localDomain != domain {
			// so here I will redirect the call to the resource server at remote location.
			client, err := getResourceClient(domain)
			if err != nil {
				return err
			}

			err = client.DeleteReference(refId, targetId, targetField, targetCollection)
			if err != nil {
				return err
			}

			return nil
		}
	}

	if strings.Contains(refId, "@") {
		domain := strings.Split(refId, "@")[1]
		refId = strings.Split(refId, "@")[0]

		localDomain, err := config.GetDomain()
		if err != nil {
			return err
		}

		if localDomain != domain {
			// so here I will redirect the call to the resource server at remote location.
			client, err := getResourceClient(domain)
			if err != nil {
				return err
			}

			err = client.DeleteReference(refId, targetId, targetField, targetCollection)
			if err != nil {
				return err
			}

			return nil
		}
	}

	q := `{"_id":"` + targetId + `"}`
	values, err := p.FindOne(context.Background(), "local_resource", "local_resource", targetCollection, q, ``)
	if err != nil {
		return err
	}

	target := values.(map[string]interface{})

	if target[targetField] == nil {
		return errors.New("No field named " + targetField + " was found in object with id " + targetId + "!")
	}

	var references []interface{}
	switch target[targetField].(type) {
	case primitive.A:
		references = []interface{}(target[targetField].(primitive.A))
	case []interface{}:
		references = target[targetField].([]interface{})
	}

	references_ := make([]interface{}, 0)
	for j := 0; j < len(references); j++ {
		if references[j].(map[string]interface{})["$id"] != refId {
			references_ = append(references_, references[j])
		}
	}

	target[targetField] = references_

	jsonStr := serialyseObject(target)

	err = p.ReplaceOne(context.Background(), "local_resource", "local_resource", targetCollection, q, jsonStr, ``)
	if err != nil {
		return err
	}

	return nil
}

func (srv *server) createCrossReferences(sourceId, sourceCollection, sourceField, targetId, targetCollection, targetField string) error {
	p, err := srv.getPersistenceStore()
	if err != nil {
		return err
	}

	err = srv.createReference(p, targetId, targetCollection, targetField, sourceId, sourceCollection)
	if err != nil {
		return err
	}

	err = srv.createReference(p, sourceId, sourceCollection, sourceField, targetId, targetCollection)

	return err

}

func (srv *server) createReference(p persistence_store.Store, id, sourceCollection, field, targetId, targetCollection string) error {

	var err error
	var source map[string]interface{}

	// the must contain the domain in the id.
	if !strings.Contains(targetId, "@") {
		return errors.New("target id must be a valid id with domain")
	}

	// Here I will check if the target id is on the same domain as the source id.
	if strings.Split(targetId, "@")[1] != srv.Domain {

		// TODO create a remote reference... (not implemented yet)
		return errors.New("target id must be on the same domain as the source id")
	}

	// TODO see how to handle the case where the target id is not on the same domain as the source id.
	targetId = strings.Split(targetId, "@")[0] // remove the domain from the id.

	if strings.Contains(id, "@") {
		domain := strings.Split(id, "@")[1]
		id = strings.Split(id, "@")[0]

		// if the domain is not the same as the local domain then I will redirect the call to the remote resource srv.
		if srv.Domain != domain {
			// so here I will redirect the call to the resource server at remote location.
			client, err := getResourceClient(domain)
			if err != nil {
				return err
			}

			err = client.CreateReference(id, sourceCollection, field, targetId, targetCollection)
			if err != nil {
				return err
			}
			return nil // exit...
		}
	}

	// I will first check if the reference already exist.
	q := `{"_id":"` + id + `"}`

	// Get the source object.
	source_values, err := p.FindOne(context.Background(), "local_resource", "local_resource", sourceCollection, q, ``)
	if err != nil {
		return errors.New("fail to find object with id " + id + " in collection " + sourceCollection + " at address " + srv.Address + " err: " + err.Error())
	}

	// append the account.
	source = source_values.(map[string]interface{})
	// be sure that the target id is a valid id.
	if source["_id"] == nil {
		return errors.New("No _id field was found in object with id " + id + "!")
	}

	// append the domain to the id.
	if p.GetStoreType() == "MONGO" {
		var references []interface{}
		if source[field] != nil {
			switch source[field].(type) {
			case primitive.A:
				references = []interface{}(source[field].(primitive.A))
			case []interface{}:
				references = source[field].([]interface{})
			}
		}

		for j := 0; j < len(references); j++ {
			if references[j].(map[string]interface{})["$id"] == targetId {
				return errors.New(" named " + targetId + " already exist in  " + field + "!")
			}
		}

		source[field] = append(references, map[string]interface{}{"$ref": targetCollection, "$id": targetId, "$db": "local_resource"})
		jsonStr := serialyseObject(source)

		err = p.ReplaceOne(context.Background(), "local_resource", "local_resource", sourceCollection, q, jsonStr, ``)
		if err != nil {
			return err
		}
	} else if p.GetStoreType() == "SQL" || p.GetStoreType() == "SCYLLA" {

		// I will create the table if not already exist.
		if p.GetStoreType() == "SQL" {
			createTable := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS `+sourceCollection+`_`+field+` (source_id TEXT, target_id TEXT, FOREIGN KEY (source_id) REFERENCES %s(_id) ON DELETE CASCADE, FOREIGN KEY (target_id) REFERENCES %s(_id) ON DELETE CASCADE)`, sourceCollection, targetCollection)
			_, err := p.(*persistence_store.SqlStore).ExecContext("local_resource", "local_resource", createTable, nil, 0)
			if err != nil {
				return err
			}

		} else if p.GetStoreType() == "SCYLLA" {
			// the foreign key is not supported by SCYLLA.
			createTable := `CREATE TABLE IF NOT EXISTS ` + sourceCollection + `_` + field + ` (source_id TEXT, target_id TEXT, PRIMARY KEY (source_id, target_id))`
			session := p.(*persistence_store.ScyllaStore).GetSession("local_resource")

			if session == nil {
				return errors.New("fail to get session for local_resource")
			}

			err = session.Query(createTable).Exec()
			if err != nil {
				return err
			}
		}

		// Here I will insert the reference in the database.

		// I will first check if the reference already exist.
		q = `SELECT * FROM ` + sourceCollection + `_` + field + ` WHERE source_id='` + Utility.ToString(source["_id"]) + `' AND target_id='` + targetId + `'`
		if p.GetStoreType() == "SCYLLA" {
			q += ` ALLOW FILTERING`
		}

		count, _ := p.Count(context.Background(), "local_resource", "local_resource", sourceCollection+`_`+field, q, ``)

		if count == 0 {
			q = `INSERT INTO ` + sourceCollection + `_` + field + ` (source_id, target_id) VALUES (?,?)`

			if p.GetStoreType() == "SCYLLA" {

				session := p.(*persistence_store.ScyllaStore).GetSession("local_resource")
				if session == nil {
					return errors.New("fail to get session for local_resource")
				}

				err = session.Query(q, source["_id"], targetId).Exec()
				if err != nil {
					return err
				}

			} else if p.GetStoreType() == "SQL" {
				_, err = p.(*persistence_store.SqlStore).ExecContext("local_resource", "local_resource", q, []interface{}{source["_id"], targetId}, 0)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// CreateReference creates a reference between a source and target resource.
// It uses the persistence service to store the reference information.
// Returns a CreateReferenceRsp on success, or an error if the operation fails.
//
// Parameters:
//   ctx - the context for the request.
//   rqst - the request containing source and target details.
//
// Returns:
//   *resourcepb.CreateReferenceRsp - response indicating success.
//   error - error if the reference could not be created.
func (srv *server) CreateReference(ctx context.Context, rqst *resourcepb.CreateReferenceRqst) (*resourcepb.CreateReferenceRsp, error) {
	// That service made user of persistence service.
	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	err = srv.createReference(p, rqst.SourceId, rqst.SourceCollection, rqst.FieldName, rqst.TargetId, rqst.TargetCollection)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// reference was created...
	return &resourcepb.CreateReferenceRsp{}, nil
}

// DeleteReference deletes a reference from a resource.
// It retrieves the persistence store and attempts to remove the specified reference
// identified by RefId, TargetId, TargetField, and TargetCollection from the store.
// Returns a DeleteReferenceRsp on success, or an error with appropriate status code
// if the operation fails.
func (srv *server) DeleteReference(ctx context.Context, rqst *resourcepb.DeleteReferenceRqst) (*resourcepb.DeleteReferenceRsp, error) {
	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	err = srv.deleteReference(p, rqst.RefId, rqst.TargetId, rqst.TargetField, rqst.TargetCollection)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &resourcepb.DeleteReferenceRsp{}, nil
}
