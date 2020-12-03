package log_client

import (
	"github.com/globulario/services/golang/resource/resourcepb"
)

// Append a new log information.
func (self *Log_Client) Log(application string, user string, method string, err_ error) error {

	// Here I set a log information.
	rqst := new(resourcepb.LogRqst)
	info := new(resourcepb.LogInfo)
	info.Application = application
	info.UserName = user
	info.Method = method
	info.Date = time.Now().Unix()
	if err_ != nil {
		info.Message = err_.Error()
		info.Type = resourcepb.LogType_ERROR_MESSAGE
	} else {
		info.Type = resourcepb.LogType_INFO_MESSAGE
	}
	rqst.Info = info

	_, err := self.c.Log(globular.GetClientContext(self), rqst)

	return err
}
