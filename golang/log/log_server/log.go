package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/log/logpb"
	"github.com/globulario/services/golang/security"
	"github.com/golang/protobuf/jsonpb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

////////////////////////////////////////////////////////////////////////////////
// Api
////////////////////////////////////////////////////////////////////////////////

func (server *server) getLogInfoKeyValue(info *logpb.LogInfo) (string, error) {

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

	if len(info.Method) > 0 {
		key += "/" + info.Method
	}

	key += "/" + Utility.GenerateUUID(info.FunctionName + info.Line)

	// I will try to retreive previous item...
	return key, nil
}

func (server *server) log(info *logpb.LogInfo, occurence *logpb.Occurence) error {

	// The userId can be a single string or a JWT token.
	if len(occurence.UserId) > 0 {
		id, name, _, _, _, err := security.ValidateToken(occurence.UserId)
		if err == nil {
			occurence.UserId = id
		}

		occurence.UserId = id
		occurence.UserName = name // keep only the user name
	}

	// Return the log information.
	key, err := server.getLogInfoKeyValue(info)
	if err != nil {
		return err
	}

	// Here I will get the previous info and append it new offucrence before saved it...
	previousInstance := new(logpb.LogInfo)
	data, err := server.logs.GetItem(key)
	if err == nil {
		err = jsonpb.UnmarshalString(string(data), previousInstance)
		if err == nil {
			if previousInstance.Occurences == nil {
				previousInstance.Occurences = make([]*logpb.Occurence, 0)
			}
			previousInstance.Occurences = append(previousInstance.Occurences, occurence)
		}
		info.Occurences = previousInstance.Occurences
	} else {
		info.Occurences = make([]*logpb.Occurence, 0)
		info.Occurences = append(info.Occurences, occurence)
	}

	marshaler := new(jsonpb.Marshaler)
	jsonStr, err := marshaler.MarshalToString(info)
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
	fmt.Println("log occurence ", rqst.Occurence)
	server.log(rqst.Info, rqst.Occurence)
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

		key, err := server.getLogInfoKeyValue(&info)
		if err != nil {
			return err
		}

		server.logs.RemoveItem(key)
	}

	return nil
}

//* Delete a log info *
func (server *server) DeleteLog(ctx context.Context, rqst *logpb.DeleteLogRqst) (*logpb.DeleteLogRsp, error) {

	key, _ := server.getLogInfoKeyValue(rqst.Log)
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
