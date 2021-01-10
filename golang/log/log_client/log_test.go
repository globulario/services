package log_client

import (
	//"encoding/json"
	"log"
	"testing"

	"github.com/globulario/services/golang/log/logpb"
)

var (
	client, _ = NewLogService_Client("localhost", "log.LogService")
)

// Test various function here.
func TestLogMessage(t *testing.T) {
	log.Println("Test log message")
	client.Log("Test", "golang", "/LogService/Log", logpb.LogLevel_INFO_MESSAGE, "This is a test message!")
}

// Test get message infos.
func TestGetLog(t *testing.T) {
	// Test get message infos for
	infos, err := client.GetLog("/info/echo.EchoService*")
	if err != nil {
		log.Println(err)
		return
	}
	log.Println(infos)
}

func TestClearLogs(t *testing.T) {
	// Test get message infos for
	err := client.ClearLog("/info*")
	if err != nil {
		log.Println(err)
		return
	}
	log.Println("Log's /info* are clear!")
}
