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

/************* helpers for canonical link tables *************/

// canonicalize two collection names (lowercased) and tell if a is first
func canonicalPair(a, b string) (left, right string, aIsFirst bool) {
	la, lb := strings.ToLower(strings.TrimSpace(a)), strings.ToLower(strings.TrimSpace(b))
	if la <= lb {
		return la, lb, true
	}
	return lb, la, false
}

// derive the “other” collection name from a field (roles -> roles, roleIds -> roles)
func inferCollectionFromField(field string) string {
	f := strings.ToLower(strings.TrimSpace(field))
	for _, suf := range []string{"ids", "id", "_ids", "_id"} {
		f = strings.TrimSuffix(f, suf)
	}
	if !strings.HasSuffix(f, "s") {
		f += "s"
	}
	return f
}

/************************************************************/

func (srv *server) deleteReference(p persistence_store.Store, refId, targetId, targetField, targetCollection string) error {
	// Remote routing based on ids (keep current behavior)
	if strings.Contains(targetId, "@") {
		domain := strings.Split(targetId, "@")[1]
		targetId = strings.Split(targetId, "@")[0]

		localDomain, err := config.GetDomain()
		if err != nil {
			return err
		}
		if localDomain != domain {
			client, err := getResourceClient(domain)
			if err != nil {
				return err
			}
			return client.DeleteReference(refId, targetId, targetField, targetCollection)
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
			client, err := getResourceClient(domain)
			if err != nil {
				return err
			}
			return client.DeleteReference(refId, targetId, targetField, targetCollection)
		}
	}

	// Branch per store type
	switch p.GetStoreType() {
	case "MONGO":
		// Original array-in-document logic stays the same
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
		references_ := make([]interface{}, 0, len(references))
		for j := 0; j < len(references); j++ {
			if references[j].(map[string]interface{})["$id"] != refId {
				references_ = append(references_, references[j])
			}
		}
		target[targetField] = references_
		jsonStr := serialyseObject(target)
		return p.ReplaceOne(context.Background(), "local_resource", "local_resource", targetCollection, q, jsonStr, ``)

	case "SQL", "SCYLLA":
		// Canonical link table deletion
		other := inferCollectionFromField(targetField)
		left, right, aIsFirst := canonicalPair(targetCollection, other)
		linkTable := left + "_" + right

		tid := targetId
		rid := refId
		// put (source_id, target_id) in canonical orientation
		src, dst := tid, rid
		if !aIsFirst { // canonical (other, targetCollection)
			src, dst = rid, tid
		}

		if p.GetStoreType() == "SCYLLA" {
			session := p.(*persistence_store.ScyllaStore).GetSession("local_resource")
			if session == nil {
				return errors.New("fail to get session for local_resource")
			}
			del := fmt.Sprintf("DELETE FROM %s WHERE source_id=? AND target_id=?", linkTable)
			return session.Query(del, src, dst).Exec()
		}

		// SQL
		del := fmt.Sprintf("DELETE FROM %s WHERE source_id=? AND target_id=?", linkTable)
		_, err := p.(*persistence_store.SqlStore).ExecContext("local_resource", "local_resource", del, []interface{}{src, dst}, 0)
		return err
	}

	// Unknown store type — no-op to keep parity with existing behavior
	return nil
}

func (srv *server) createCrossReferences(sourceId, sourceCollection, sourceField, targetId, targetCollection, targetField string) error {
	p, err := srv.getPersistenceStore()
	if err != nil {
		return err
	}
	if err = srv.createReference(p, targetId, targetCollection, targetField, sourceId, sourceCollection); err != nil {
		return err
	}
	return srv.createReference(p, sourceId, sourceCollection, sourceField, targetId, targetCollection)
}

func (srv *server) createReference(p persistence_store.Store, id, sourceCollection, field, targetId, targetCollection string) error {
	var err error
	var source map[string]interface{}

	// the target must contain the domain in the id.
	if !strings.Contains(targetId, "@") {
		return errors.New("target id must be a valid id with domain")
	}

	// Ensure same domain (current behavior)
	if strings.Split(targetId, "@")[1] != srv.Domain {
		return errors.New("target id must be on the same domain as the source id")
	}
	targetId = strings.Split(targetId, "@")[0] // remove domain

	// route if source id is remote
	if strings.Contains(id, "@") {
		domain := strings.Split(id, "@")[1]
		id = strings.Split(id, "@")[0]
		if srv.Domain != domain {
			client, err := getResourceClient(domain)
			if err != nil {
				return err
			}
			return client.CreateReference(id, sourceCollection, field, targetId, targetCollection)
		}
	}

	// load source
	q := `{"_id":"` + id + `"}`
	source_values, err := p.FindOne(context.Background(), "local_resource", "local_resource", sourceCollection, q, ``)
	if err != nil {
		return errors.New("fail to find object with id " + id + " in collection " + sourceCollection + " at address " + srv.Address + " err: " + err.Error())
	}
	source = source_values.(map[string]interface{})
	if source["_id"] == nil {
		return errors.New("No _id field was found in object with id " + id + "!")
	}

	// Mongo: keep array-in-document behavior
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
		return p.ReplaceOne(context.Background(), "local_resource", "local_resource", sourceCollection, q, jsonStr, ``)
	}

	// SQL / SCYLLA: single canonical link table per pair
	if p.GetStoreType() == "SQL" || p.GetStoreType() == "SCYLLA" {
		left, right, aIsFirst := canonicalPair(sourceCollection, targetCollection)
		linkTable := left + "_" + right

		// ensure link table exists
		create := fmt.Sprintf(
			`CREATE TABLE IF NOT EXISTS %s (source_id TEXT, target_id TEXT, PRIMARY KEY (source_id, target_id))`,
			linkTable,
		)

		if p.GetStoreType() == "SCYLLA" {
			session := p.(*persistence_store.ScyllaStore).GetSession("local_resource")
			if session == nil {
				return errors.New("fail to get session for local_resource")
			}
			if err := session.Query(create).Exec(); err != nil {
				return err
			}
		} else {
			if _, err := p.(*persistence_store.SqlStore).ExecContext("local_resource", "local_resource", create, nil, 0); err != nil {
				return err
			}
		}

		// choose (source_id,target_id) according to canonical order
		src, dst := id, targetId
		if !aIsFirst {
			src, dst = targetId, id
		}

		// idempotent insert (Scylla INSERT is upsert; SQL has PK to avoid dup rows)
		if p.GetStoreType() == "SCYLLA" {
			session := p.(*persistence_store.ScyllaStore).GetSession("local_resource")
			if session == nil {
				return errors.New("fail to get session for local_resource")
			}
			q := fmt.Sprintf("INSERT INTO %s (source_id, target_id) VALUES (?,?)", linkTable)
			return session.Query(q, src, dst).Exec()
		}

		// SQL
		q := fmt.Sprintf("INSERT OR IGNORE INTO %s (source_id, target_id) VALUES (?,?)", linkTable)
		_, err := p.(*persistence_store.SqlStore).ExecContext("local_resource", "local_resource", q, []interface{}{src, dst}, 0)
		return err
	}

	return nil
}

// CreateReference API
func (srv *server) CreateReference(ctx context.Context, rqst *resourcepb.CreateReferenceRqst) (*resourcepb.CreateReferenceRsp, error) {
	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	if err = srv.createReference(p, rqst.SourceId, rqst.SourceCollection, rqst.FieldName, rqst.TargetId, rqst.TargetCollection); err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &resourcepb.CreateReferenceRsp{}, nil
}

// DeleteReference API
func (srv *server) DeleteReference(ctx context.Context, rqst *resourcepb.DeleteReferenceRqst) (*resourcepb.DeleteReferenceRsp, error) {
	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	if err = srv.deleteReference(p, rqst.RefId, rqst.TargetId, rqst.TargetField, rqst.TargetCollection); err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &resourcepb.DeleteReferenceRsp{}, nil
}
