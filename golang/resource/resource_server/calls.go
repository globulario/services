package main

import (
	"context"
	"errors"
	"strings"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/resource/resourcepb"
	Utility "github.com/globulario/utility"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ClearCalls removes call records for a specified account from the persistence store.
// It validates the account ID, ensuring it belongs to the local domain, and constructs
// the appropriate database name. The function applies an optional filter to select
// specific calls to delete. If no filter is provided, all calls are targeted.
// Returns an empty ClearCallsRsp on success, or an error if any operation fails.
//
// Parameters:
//   ctx  - The context for the request.
//   rqst - The ClearCallsRqst containing the account ID and optional filter.
//
// Returns:
//   *resourcepb.ClearCallsRsp - The response object (empty on success).
//   error                     - An error if the operation fails.
func (srv *server) ClearCalls(ctx context.Context, rqst *resourcepb.ClearCallsRqst) (*resourcepb.ClearCallsRsp, error) {

	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}


	query := rqst.Filter
	if len(query) == 0 {
		query = "{}"
	}

	results, err := p.Find(context.Background(), "local_resource", "local_resource", "Calls", query, "")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Delete the call.
	for i := range results {
		call := results[i].(map[string]interface{})
		srv.deleteCall(rqst.AccountId, call["_id"].(string))
	}

	return &resourcepb.ClearCallsRsp{}, nil
}

func (srv *server) deleteCall(account_id, uuid string) error {
	p, err := srv.getPersistenceStore()
	if err != nil {
		return err
	}

	// Keep the id portion only...
	accountId := account_id
	if strings.Contains(accountId, "@") {
		domain := strings.Split(accountId, "@")[1]
		localDomain, _ := config.GetDomain()
		if domain != localDomain {
			return err
		}
		accountId = strings.Split(accountId, "@")[0]
	}

	q := `{"_id":"` + uuid + `"}`

	err = p.DeleteOne(context.Background(), "local_resource", "local_resource", "Calls", q, "")
	if err != nil {
		return err
	}

	return nil
}

// DeleteCall handles the deletion of a call resource identified by its UUID and associated account ID.
// It receives a DeleteCallRqst containing the AccountId and Uuid, attempts to delete the call,
// and returns a DeleteCallRsp on success. If an error occurs during deletion, it returns an appropriate gRPC error status.
func (srv *server) DeleteCall(ctx context.Context, rqst *resourcepb.DeleteCallRqst) (*resourcepb.DeleteCallRsp, error) {

	err := srv.deleteCall(rqst.AccountId, rqst.Uuid)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &resourcepb.DeleteCallRsp{}, nil
}

// GetCallHistory retrieves the call history for a given account ID.
// It validates the account ID, determines the appropriate database name,
// constructs a query based on the underlying persistence store type (MONGO, SCYLLA, SQL),
// executes the query to fetch call records, and returns them in the response.
// Returns an error if the account ID is invalid, the database type is unknown,
// or if there is a failure during query execution.
//
// Parameters:
//   ctx - The context for the request.
//   rqst - The request containing the account ID.
//
// Returns:
//   *resourcepb.GetCallHistoryRsp - The response containing the list of calls.
//   error - An error if the operation fails.
func (srv *server) GetCallHistory(ctx context.Context, rqst *resourcepb.GetCallHistoryRqst) (*resourcepb.GetCallHistoryRsp, error) {
	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Keep the id portion only...
	accountId := rqst.AccountId
	if strings.Contains(accountId, "@") {
		domain := strings.Split(accountId, "@")[1]
		localDomain, _ := config.GetDomain()
		if domain != localDomain {
			return nil, status.Errorf(
				codes.Internal,
				"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no account found with id "+accountId)))

		}
		accountId = strings.Split(accountId, "@")[0]
	}

	var query string
	if p.GetStoreType() == "MONGO" {
		query = `{"$or":[{"caller":"` + rqst.AccountId + `"},{"callee":"` + rqst.AccountId + `"} ]}`
	} else if p.GetStoreType() == "SCYLLA" {
		query = `SELECT * FROM Calls WHERE caller='` + rqst.AccountId + `' OR callee='` + rqst.AccountId + `' ALLOW FILTERING`
	} else if p.GetStoreType() == "SQL" {
		query = `SELECT * FROM Calls WHERE caller='` + rqst.AccountId + `' OR callee='` + rqst.AccountId + `'`
	} else {
		return nil, errors.New("unknown database type " + p.GetStoreType())
	}

	results, err := p.Find(context.Background(), "local_resource", "local_resource", "Calls", query, "")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	calls := make([]*resourcepb.Call, len(results))
	for i := range results {
		call := results[i].(map[string]interface{})
		startTime := Utility.ToInt(call["start_time"])
		endTime := Utility.ToInt(call["end_time"])

		calls[i] = &resourcepb.Call{Caller: call["caller"].(string), Callee: call["callee"].(string), Uuid: call["_id"].(string), StartTime: int64(startTime), EndTime: int64(endTime)}
	}

	return &resourcepb.GetCallHistoryRsp{Calls: calls}, nil
}

func (srv *server) setCall(accountId string, call *resourcepb.Call) error {

	// Get the persistence connection
	p, err := srv.getPersistenceStore()
	if err != nil {
		return err
	}

	// rename the uuid to _id (for mongo identifier)
	call_ := map[string]interface{}{"caller": call.Caller, "callee": call.Callee, "_id": call.Uuid, "start_time": call.StartTime, "end_time": call.EndTime}
	jsonStr, _ := Utility.ToJson(call_)

	q := `{"_id":"` + call.Uuid + `"}`
	err = p.ReplaceOne(context.Background(), "local_resource", "local_resource", "Calls", q, jsonStr, `[{"upsert":true}]`)
	if err != nil {
		return err
	}

	return nil
}

// SetCall handles the setting of a call resource for both caller and callee.
// It checks if the caller or callee contains a domain (identified by '@') and,
// if the domain matches the local domain, it sets the call for the local user.
// Otherwise, it sets the call using the provided caller or callee identifier.
// Returns a SetCallRsp response on success, or an error if the operation fails.
//
// Parameters:
//   ctx - The context for the request.
//   rqst - The SetCallRqst containing call information.
//
// Returns:
//   *resourcepb.SetCallRsp - The response object.
//   error - An error if the call could not be set.
func (srv *server) SetCall(ctx context.Context, rqst *resourcepb.SetCallRqst) (*resourcepb.SetCallRsp, error) {

	// Get the persistence connection
	if strings.Contains(rqst.Call.Caller, "@") {
		domain := strings.Split(rqst.Call.Caller, "@")[1]
		localDomain, _ := config.GetDomain()
		if domain == localDomain {
			err := srv.setCall(strings.Split(rqst.Call.Caller, "@")[0], rqst.Call)
			if err != nil {
				return nil, status.Errorf(
					codes.Internal,
					"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
			}
		}
	} else {
		err := srv.setCall(rqst.Call.Caller, rqst.Call)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	if strings.Contains(rqst.Call.Callee, "@") {
		domain := strings.Split(rqst.Call.Callee, "@")[1]
		localDomain, _ := config.GetDomain()
		if domain == localDomain {
			err := srv.setCall(strings.Split(rqst.Call.Callee, "@")[0], rqst.Call)
			if err != nil {
				return nil, status.Errorf(
					codes.Internal,
					"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
			}
		}
	} else {
		err := srv.setCall(rqst.Call.Callee, rqst.Call)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	return &resourcepb.SetCallRsp{}, nil
}
