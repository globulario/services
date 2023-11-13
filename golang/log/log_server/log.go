package main

import (
	"context"
	"encoding/json"
	"errors"
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

func (server *server) log(info *logpb.LogInfo) error {

	if info == nil {
		return errors.New("no log info was given")
	}

	if len(info.Application) == 0 {
		return errors.New("no application name was given")
	}

	if len(info.Method) == 0 {
		return errors.New("no method name was given")
	}

	if len(info.Line) == 0 {
		return errors.New("no line number was given")
	}

	var level string
	if info.GetLevel() == logpb.LogLevel_INFO_MESSAGE {
		level = "info"
	} else if info.GetLevel() == logpb.LogLevel_DEBUG_MESSAGE {
		level = "debug"
	} else if info.GetLevel() == logpb.LogLevel_ERROR_MESSAGE {
		level = "error"
	} else if info.GetLevel() == logpb.LogLevel_FATAL_MESSAGE {
		level = "fatal"
	} else if info.GetLevel() == logpb.LogLevel_TRACE_MESSAGE {
		level = "trace"
	} else if info.GetLevel() == logpb.LogLevel_WARN_MESSAGE {
		level = "warning"
	}

	// Set the id of the log info.
	info.Id = Utility.GenerateUUID(level + `|` + info.Application + `|` + info.Method + `|` + info.Line)

	info.Occurences = 1

	// I will retreive the previous items...
	data, err := server.logs.GetItem(info.Id)
	if err == nil {
		jsonDecoder := json.NewDecoder(strings.NewReader(string(data)))
		for jsonDecoder.More() {

			previousInfo := logpb.LogInfo{}
			err := jsonpb.UnmarshalNext(jsonDecoder, &previousInfo)
			if err != nil {
				return err
			}

			// I will set the previous id...
			info.Occurences = previousInfo.Occurences + 1
		}
	}

	// I will index the log info...
	index := Utility.GenerateUUID(level + `|` + info.Application)
	data_, err := server.logs.GetItem(index)
	if err == nil {
		indexed := make([]string, 0)
		err = json.Unmarshal(data_, &indexed)
		if err == nil && !Utility.Contains(indexed, info.Id) {
			indexed = append(indexed, info.Id)
			data_, err = json.Marshal(indexed)
			if err == nil {
				server.logs.SetItem(index, data_)
			}
		}
	} else {
		indexed := make([]string, 0)
		indexed = append(indexed, info.Id)
		data_, err = json.Marshal(indexed)
		if err == nil {
			server.logs.SetItem(index, data_)
		}
	}

	// Marshal the log info into a json string.
	marshaler := new(jsonpb.Marshaler)
	jsonStr, err := marshaler.MarshalToString(info)
	if err != nil {
		return err
	}

	// Append the log in leveldb
	server.logs.SetItem(info.Id, []byte(jsonStr))

	// That must be use to keep all logger upto date...
	server.publish("new_log_evt", []byte(jsonStr))

	// Inc the counter
	server.logCount.WithLabelValues(level, info.Application, info.Method).Inc()

	return nil
}

// Log error or information into the data base *
func (server *server) Log(ctx context.Context, rqst *logpb.LogRqst) (*logpb.LogRsp, error) {

	_, token, err := security.GetClientId(ctx)
	if err != nil {
		return nil, err
	}

	// Valide the token...
	// The userId can be a single string or a JWT token.
	_, err = security.ValidateToken(token)
	if err != nil {
		return nil, err
	}

	// Publish event...
	server.log(rqst.Info)

	return &logpb.LogRsp{
		Result: true,
	}, nil
}

// Retreive the log informations
func (server *server) getLogs(application string, level string) ([]*logpb.LogInfo, error) {
	index := Utility.GenerateUUID(level + `|` + application)
	data, err := server.logs.GetItem(index)
	if err != nil {
		return nil, err
	}

	indexed := make([]string, 0)
	err = json.Unmarshal(data, &indexed)
	if err != nil {
		return nil, err
	}

	logs := make([]*logpb.LogInfo, 0)
	for _, id := range indexed {
		data, err := server.logs.GetItem(id)
		if err == nil {

			info := logpb.LogInfo{}
			err = jsonpb.UnmarshalString(string(data), &info)
			if err != nil {
				return nil, err
			}

			logs = append(logs, &info)
		}
	}

	return logs, nil
}

// Log error or information into the data base *
// Retreive log infos (the query must be something like /infos/'date'/'applicationName'/'userName'
func (server *server) GetLog(rqst *logpb.GetLogRqst, stream logpb.LogService_GetLogServer) error {

	// Retreive the logs...
	query := rqst.Query
	if len(query) == 0 {
		return errors.New("no query was given")
	}

	parameters := strings.Split(query, "/")
	if len(parameters) != 3 {
		return errors.New("the query must be something like /debug/application/*'")
	}

	logs, err := server.getLogs(parameters[1], parameters[0])
	if err != nil {
		return err
	}

	// send the first 100 logs...
	infos := make([]*logpb.LogInfo, 0)
	max := 100

	for _, info := range logs {
		if max == 0 {
			break
		}

		infos = append(infos, info)
		max = max - 1

	}

	return stream.Send(&logpb.GetLogRsp{
		Infos: infos,
	})
}

func (server *server) clearLogs(query string) error {

	// TODO: retreive the logs and delete them...
	// Retreive the logs...
	if len(query) == 0 {
		return errors.New("no query was given")
	}

	parameters := strings.Split(query, "/")
	if len(parameters) != 3 {
		return errors.New("the query must be something like /debug/application/*'")
	}

	// First of all I will retreive the log info with a given date.
	logs, err := server.getLogs(parameters[1], parameters[0])

	if err != nil {
		return err
	}

	// I will delete the logs...
	for _, info := range logs {
		err := server.logs.RemoveItem(info.Id)
		if err != nil {
			return err
		}
	}

	// I will delete the index...
	index := Utility.GenerateUUID(parameters[0] + `|` + parameters[1])
	err = server.logs.RemoveItem(index)
	if err != nil {
		return err
	}

	return nil
}

// * Delete a log info *
func (server *server) DeleteLog(ctx context.Context, rqst *logpb.DeleteLogRqst) (*logpb.DeleteLogRsp, error) {

	err := server.logs.RemoveItem(rqst.Log.Id)

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	var level string
	if rqst.Log.GetLevel() == logpb.LogLevel_INFO_MESSAGE {
		level = "info"
	} else if rqst.Log.GetLevel() == logpb.LogLevel_DEBUG_MESSAGE {
		level = "debug"
	} else if rqst.Log.GetLevel() == logpb.LogLevel_ERROR_MESSAGE {
		level = "error"
	} else if rqst.Log.GetLevel() == logpb.LogLevel_FATAL_MESSAGE {
		level = "fatal"
	} else if rqst.Log.GetLevel() == logpb.LogLevel_TRACE_MESSAGE {
		level = "trace"
	} else if rqst.Log.GetLevel() == logpb.LogLevel_WARN_MESSAGE {
		level = "warning"
	}

	// I will remove the log from index...
	index := Utility.GenerateUUID(level + `|` + rqst.Log.Application)
	data, err := server.logs.GetItem(index)
	if err == nil {
		indexed := make([]string, 0)
		err = json.Unmarshal(data, &indexed)
		if err == nil {
			for i, id := range indexed {
				if id == rqst.Log.Id {
					indexed = append(indexed[:i], indexed[i+1:]...)
					data, err = json.Marshal(indexed)
					if err == nil {
						server.logs.SetItem(index, data)
					}
					break
				}
			}
		}
	}
	return &logpb.DeleteLogRsp{
		Result: true,
	}, nil
}

// * Clear logs. info or errors *
func (server *server) ClearAllLog(ctx context.Context, rqst *logpb.ClearAllLogRqst) (*logpb.ClearAllLogRsp, error) {
	err := server.clearLogs(rqst.Query)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &logpb.ClearAllLogRsp{
		Result: true,
	}, nil
}
