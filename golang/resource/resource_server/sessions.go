package main

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"time"

	"github.com/globulario/services/golang/resource/resourcepb"
	Utility "github.com/globulario/utility"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (srv *server) getSession(accountId string) (*resourcepb.Session, error) {
	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, err
	}

	var q string
	if p.GetStoreType() == "MONGO" {
		q = `{"accountId":"` + accountId + `"}`
	} else if p.GetStoreType() == "SCYLLA" {
		q = `SELECT * FROM Sessions WHERE accountId='` + accountId + `' ALLOW FILTERING`
	} else if p.GetStoreType() == "SQL" {
		q = `SELECT * FROM Sessions WHERE accountId='` + accountId + `'`
	} else {
		return nil, errors.New("unknown database type " + p.GetStoreType())
	}

	// Now I will remove the token...
	session_, err := p.FindOne(context.Background(), "local_resource", "local_resource", "Sessions", q, "")
	if err != nil {
		return nil, err
	}

	session := session_.(map[string]interface{})

	expireAt := Utility.ToInt(session["expire_at"])
	lastStateTime := Utility.ToInt(session["last_state_time"])

	if expireAt == 0 || lastStateTime == 0 {
		return nil, errors.New("invalid session with id " + accountId + " expire_at has value " + time.Unix(int64(expireAt), 0).Local().String() + " last_state_time " + time.Unix(int64(lastStateTime), 0).Local().String())
	}

	var state resourcepb.SessionState
	// Default state is offline
	state = resourcepb.SessionState_OFFLINE

	if session["state"] != nil {
		state = resourcepb.SessionState(int32(Utility.ToInt(session["state"])))
	}

	return &resourcepb.Session{AccountId: session["accountId"].(string), ExpireAt: int64(expireAt), LastStateTime: int64(lastStateTime), State: state}, nil
}

// GetSession retrieves the session information for a given account ID.
// It logs the request, attempts to fetch the session, and returns the session data
// in the response. If an error occurs during retrieval, it returns an appropriate
// gRPC error status.
//
// Parameters:
//   ctx - The context for the request, used for cancellation and deadlines.
//   rqst - The request containing the AccountId for which the session is to be retrieved.
//
// Returns:
//   *resourcepb.GetSessionResponse - The response containing the session information.
//   error - An error if the session could not be retrieved.
func (srv *server) GetSession(ctx context.Context, rqst *resourcepb.GetSessionRequest) (*resourcepb.GetSessionResponse, error) {

	logger.Info("log", "args", []interface{}{"get session for ", rqst.AccountId})

	// Now I will remove the token...
	session, err := srv.getSession(rqst.AccountId)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &resourcepb.GetSessionResponse{
		Session: session,
	}, nil
}

// GetSessions retrieves a list of session objects based on the provided query and options.
// It supports both SQL and non-SQL persistence stores. If the query is empty, it defaults to an empty filter.
// For SQL stores, it parses the query as JSON and constructs an SQL statement accordingly.
// Returns a GetSessionsResponse containing the matching sessions or an error if the operation fails.
//
// Parameters:
//   - ctx: The context for the request.
//   - rqst: The GetSessionsRequest containing query and options.
//
// Returns:
//   - *resourcepb.GetSessionsResponse: The response containing the list of sessions.
//   - error: An error if the retrieval fails.
func (srv *server) GetSessions(ctx context.Context, rqst *resourcepb.GetSessionsRequest) (*resourcepb.GetSessionsResponse, error) {
	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	query := rqst.Query
	if len(query) == 0 {
		query = "{}"
	} else {
		if p.GetStoreType() == "SQL" {
			paremeters := make(map[string]interface{})
			err := json.Unmarshal([]byte(query), &paremeters)
			if err != nil {
				return nil, status.Errorf(
					codes.Internal,
					"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
			}

			query = `SELECT * FROM Sessions WHERE `
			if paremeters["state"] != nil {
				query += ` state=` + Utility.ToString(paremeters["state"])
			}

		}
	}

	sessions, err := p.Find(context.Background(), "local_resource", "local_resource", "Sessions", query, rqst.Options)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	sessions_ := make([]*resourcepb.Session, 0)
	for i := range sessions {
		session := sessions[i].(map[string]interface{})
		expireAt := Utility.ToInt(session["expire_at"])
		lastStateTime := Utility.ToInt(session["last_state_time"])
		state := int32(Utility.ToInt(session["state"]))
		sessions_ = append(sessions_, &resourcepb.Session{AccountId: session["accountId"].(string), ExpireAt: int64(expireAt), LastStateTime: int64(lastStateTime), State: resourcepb.SessionState(state)})
	}

	return &resourcepb.GetSessionsResponse{
		Sessions: sessions_,
	}, nil
}

// RemoveSession removes a session associated with the specified account ID from the persistence store.
// The method determines the query format based on the underlying store type (MONGO, SCYLLA, or SQL).
// It returns a RemoveSessionResponse on success, or an error if the operation fails or the store type is unknown.
//
// Parameters:
//   ctx - The context for the request.
//   rqst - The request containing the account ID whose session should be removed.
//
// Returns:
//   *resourcepb.RemoveSessionResponse - The response indicating successful removal.
//   error - An error if the session could not be removed or if the store type is unknown.
func (srv *server) RemoveSession(ctx context.Context, rqst *resourcepb.RemoveSessionRequest) (*resourcepb.RemoveSessionResponse, error) {
	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	var q string
	if p.GetStoreType() == "MONGO" {
		q = `{"accountId":"` + rqst.AccountId + `"}`
	} else if p.GetStoreType() == "SCYLLA" {
		q = `SELECT * FROM Sessions WHERE accountId='` + rqst.AccountId + `' ALLOW FILTERING`
	} else if p.GetStoreType() == "SQL" {
		q = `SELECT * FROM Sessions WHERE accountId='` + rqst.AccountId + `'`
	} else {
		return nil, errors.New("unknown database type " + p.GetStoreType())
	}

	// Now I will remove the token...
	err = p.Delete(context.Background(), "local_resource", "local_resource", "Sessions", q, "")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &resourcepb.RemoveSessionResponse{}, nil
}

func (srv *server) updateSession(accountId string, state resourcepb.SessionState, last_session_time, expire_at int64) error {

	expiration := time.Unix(expire_at, 0)
	delay := time.Until(expiration)
	if state != resourcepb.SessionState_OFFLINE {
		if expiration.Before(time.Now()) {
			return errors.New("session is already expired " + expiration.Local().String() + " " + Utility.ToString(math.Floor(delay.Minutes())) + ` minutes ago`)
		}
	}

	p, err := srv.getPersistenceStore()
	if err != nil {
		return err
	}

	// Log a message to display update session...
	//srv.logServiceInfo("updateSession", Utility.FileLine(), Utility.FunctionName(), "update session for user "+accountId+" last_session_time: "+time.Unix(last_session_time, 0).Local().String()+" expire_at: "+time.Unix(expire_at, 0).Local().String())
	session := map[string]interface{}{"_id": Utility.ToString(last_session_time), "domain": srv.Domain, "accountId": accountId, "expire_at": expire_at, "last_state_time": last_session_time, "state": state}

	// send update_session event
	var q string
	if p.GetStoreType() == "MONGO" {
		q = `{"accountId":"` + accountId + `"}`
	} else if p.GetStoreType() == "SCYLLA" {
		q = `SELECT * FROM Sessions WHERE accountId='` + accountId + `' ALLOW FILTERING`
	} else if p.GetStoreType() == "SQL" {
		session["_id"] = Utility.RandomUUID() // set a random id for sql db.
		q = `SELECT * FROM Sessions WHERE accountId='` + accountId + `'`
	} else {
		return errors.New("unknown database type " + p.GetStoreType())
	}

	// Be sure to remove the old session...
	p.Delete(context.Background(), "local_resource", "local_resource", "Sessions", q, "")

	_, err = p.InsertOne(context.Background(), "local_resource", "local_resource", "Sessions", session, "")

	return err

	//return p.ReplaceOne(context.Background(), "local_resource", "local_resource", "Sessions", q, jsonStr, `[{"upsert":true}]`)

}

// UpdateSession updates the session information for a given account.
// It validates the request to ensure the session is not nil, then calls the internal
// updateSession method to persist the changes. Returns an UpdateSessionResponse on success,
// or an error if the update fails or the request is invalid.
//
// Parameters:
//   ctx - The context for the request, used for cancellation and deadlines.
//   rqst - The UpdateSessionRequest containing the session details to update.
//
// Returns:
//   *resourcepb.UpdateSessionResponse - The response indicating success.
//   error - An error if the session is nil or the update operation fails.
func (srv *server) UpdateSession(ctx context.Context, rqst *resourcepb.UpdateSessionRequest) (*resourcepb.UpdateSessionResponse, error) {

	if rqst.Session == nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("session is empty")))
	}

	err := srv.updateSession(rqst.Session.AccountId, rqst.Session.State, rqst.Session.LastStateTime, rqst.Session.ExpireAt)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &resourcepb.UpdateSessionResponse{}, nil
}
