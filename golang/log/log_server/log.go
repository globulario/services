package main

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/security"
	"github.com/globulario/services/golang/log/logpb"
	"github.com/golang/protobuf/jsonpb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

////////////////////////////////////////////////////////////////////////////////
// Api
////////////////////////////////////////////////////////////////////////////////

func (server *server) getLogInfoKeyValue(info *logpb.LogInfo) (string, string, error) {
	marshaler := new(jsonpb.Marshaler)
	jsonStr, err := marshaler.MarshalToString(info)
	if err != nil {
		return "", "", err
	}

	key := ""

	if info.GetLevel() == logpb.LogLevel_INFO_MESSAGE {
		key += "/info"
	} else if info.GetLevel() == logpb.LogLevel_DEBUG_MESSAGE {
		key += "/debug"
	} else if info.GetLevel() == logpb.LogLevel_ERROR_MESSAGE {
		key += "/error"
	} else if info.GetLevel() == logpb.LogLevel_FATAL_MESSAGE {
		key += "/fatal"
	} else if info.GetLevel() == logpb.LogLevel_TRACE_MESSAGE {
		key += "/trace"
	} else if info.GetLevel() == logpb.LogLevel_WARN_MESSAGE {
		key += "/warning"
	}

	// Set the application in the path
	if len(info.Application) > 0 {
		key += "/" + info.Application
	}

	// Set the User Name if available.
	if len(info.UserName) > 0 {
		key += "/" + info.UserName
	}

	if len(info.Method) > 0 {
		key += "/" + info.Method
	}

	key += "/" + Utility.ToString(info.Date)

	key += "/" + Utility.GenerateUUID(jsonStr)

	return key, jsonStr, nil
}

func (server *server) log(info *logpb.LogInfo) error {

	// The userId can be a single string or a JWT token.
	if len(info.UserId) > 0 {
		id, name, _, _, _, err := security.ValidateToken(info.UserId)
		if err == nil {
			info.UserId = id
		}

		info.UserId = id
		info.UserName = name // keep only the user name
	}

	// Return the log information.
	key, jsonStr, err := server.getLogInfoKeyValue(info)
	if err != nil {
		return err
	}

	// Append the log in leveldb
	server.logs.SetItem(key, []byte(jsonStr))

	// That must be use to keep all logger upto date...
	server.publish("new_log_evt", []byte(jsonStr))

	return nil
}

// Log error or information into the data base *
func (server *server) Log(ctx context.Context, rqst *logpb.LogRqst) (*logpb.LogRsp, error) {
	// Publish event...
	server.log(rqst.Info)

	return &logpb.LogRsp{
		Result: true,
	}, nil
}

// Log error or information into the data base *
// Retreive log infos (the query must be something like /infos/'date'/'applicationName'/'userName'
func (server *server) GetLog(rqst *logpb.GetLogRqst, stream logpb.LogService_GetLogServer) error {

	query := rqst.Query
	if len(query) == 0 {
		query = "/*"
	}

	data, err := server.logs.GetItem(query)
	if err != nil {
		return status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	jsonDecoder := json.NewDecoder(strings.NewReader(string(data)))

	// read open bracket
	_, err = jsonDecoder.Token()
	if err != nil {
		return status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	infos := make([]*logpb.LogInfo, 0)
	i := 0
	max := 100
	for jsonDecoder.More() {
		info := logpb.LogInfo{}
		err := jsonpb.UnmarshalNext(jsonDecoder, &info)
		if err != nil {
			return status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
		// append the info inside the stream.
		infos = append(infos, &info)
		if i == max-1 {
			// I will send the stream at each 100 logs...
			rsp := &logpb.GetLogRsp{
				Infos: infos,
			}
			// Send the infos
			err = stream.Send(rsp)
			if err != nil {
				return status.Errorf(
					codes.Internal,
					Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
			}
			infos = make([]*logpb.LogInfo, 0)
			i = 0
		}
		i++
	}

	// Send the last infos...
	if len(infos) > 0 {
		rsp := &logpb.GetLogRsp{
			Infos: infos,
		}
		err = stream.Send(rsp)
		if err != nil {
			return status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	return nil
}

func (server *server) deleteLog(query string) error {

	// First of all I will retreive the log info with a given date.
	data, err := server.logs.GetItem(query)
	if err != nil {
		return err
	}

	jsonDecoder := json.NewDecoder(strings.NewReader(string(data)))
	// read open bracket
	_, err = jsonDecoder.Token()
	if err != nil {
		return err
	}

	for jsonDecoder.More() {
		info := logpb.LogInfo{}

		err := jsonpb.UnmarshalNext(jsonDecoder, &info)
		if err != nil {
			return err
		}

		key, _, err := server.getLogInfoKeyValue(&info)
		if err != nil {
			return err
		}

		server.logs.RemoveItem(key)
	}

	return nil
}

//* Delete a log info *
func (server *server) DeleteLog(ctx context.Context, rqst *logpb.DeleteLogRqst) (*logpb.DeleteLogRsp, error) {

	key, _, _ := server.getLogInfoKeyValue(rqst.Log)
	err := server.logs.RemoveItem(key)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &logpb.DeleteLogRsp{
		Result: true,
	}, nil
}

//* Clear logs. info or errors *
func (server *server) ClearAllLog(ctx context.Context, rqst *logpb.ClearAllLogRqst) (*logpb.ClearAllLogRsp, error) {
	err := server.deleteLog(rqst.Query)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &logpb.ClearAllLogRsp{
		Result: true,
	}, nil
}
