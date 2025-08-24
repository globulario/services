package admin_client

import (
	//"encoding/json"
	"log"
	"testing"

	//"time"

	"github.com/globulario/services/golang/authentication/authentication_client"
	"github.com/globulario/services/golang/log/log_client"
	"github.com/globulario/services/golang/log/logpb"
	Utility "github.com/globulario/utility"
)

var (
	// Connect to the admin client.
	address                   = "globule-ryzen.globular.cloud"
	client, _                 = NewAdminService_Client(address, "admin.AdminService")
	authentication_client_, _ = authentication_client.NewAuthenticationService_Client(address, "authentication.AuthenticationService")
	log_client_, _            = log_client.NewLogService_Client(address, "log.LogService")
)

func TestRunCmd(t *testing.T) {
	log.Println("call authenticate")
	token, err := authentication_client_.Authenticate("sa", "adminadmin")
	if err != nil {
		log.Println("---> ", err)
	} else {
		log.Println("---> ", token)
	}

	// Ther's various kind of command that can be run by this command...
	// results, err := client.RunCmd(token, "cmd", []string{"/C", "dir", "C:\\"}, true)
	// results, err := client.RunCmd(token, "taskkill", []string{"/F", "/PID", "15024"}, true)
	// results, err := client.RunCmd(token, "set", []string{"PATH=%PATH%;C:/applications_services/exec/ffmpeg-N-101994-g84ac1440b2-win64-gpl-shared/bin"}, true)
	// results, err := client.RunCmd(token, "setx", []string{"/M", "PATH ", "C:/applications_services/exec/ffmpeg-N-101994-g84ac1440b2-win64-gpl-shared/bin"}, true)
	// results, err := client.RunCmd(token, "ffmpeg", []string{"-version"}, true)

	// Here I will get the content of the tmp directory.
	results, err := client.RunCmd(token, "ls", "", []string{"-a", "/tmp"}, true)

	if err != nil {
		log.Println(err)
		log_client_.Log("admin_test", "test", "TestRunCmd", logpb.LogLevel_ERROR_MESSAGE, err.Error(), Utility.FileLine(), Utility.FunctionName())
		t.FailNow()
	}

	// So here I will set message into the logger...
	log.Println(results)
}

func TestGetAvailableHosts(t *testing.T) {
	log.Println("call authenticate")
	results, err := client.GetAvailableHosts()
	if err != nil {
		log.Println(err)
		t.FailNow()
	}

	log.Println(results)
}

/*
func TestGetVariable(t *testing.T) {
	var err error
	client_, err := resource_client.NewResourceService_Client("globular.cloud", "resource.ResourceService")
	if err != nil {
		log.Println("----> fail to connect with error ", err)
		return
	}
	token, err := client_.Authenticate("sa", "adminadmin")
	if err != nil {
		log.Println("---> ", err)
	} else {
		log.Println("---> ", token)
	}

	val, err := client.GetEnvironmentVariable(token, "path")

	if err != nil {
		log.Println(err)
		t.FailNow()
	}

	log.Println(val)

	val += ";C:\\applications_services\\exec\\ffmpeg-N-101994-g84ac1440b2-win64-gpl-shared\\bin"
	err = client.SetEnvironmentVariable(token, "path", val)
	if err != nil {
		log.Println(err)
		t.FailNow()
	}

}
*/
/*
func TestGeneratePost(t *testing.T) {
	var err error
	client_, err := resource_client.NewResourceService_Client("mon-iis-01:10003", "resource.ResourceService")
	if err != nil {
		log.Println("----> fail to connect with error ", err)
		return
	}
	token, err := client_.Authenticate("sa", "adminadmin")
	if err != nil {
		log.Println("---> ", err)
	} else {
		log.Println("---> ", token)
	}

	results, err := client.RunCmd(token, `\\mon-filer-01\Manufacturing\Transfert\_00mr\post\dave\____POST_2701.cmd`, []string{})
	if err != nil {
		log.Println(err)
		t.FailNow()
	}

	log.Println(results)
}
*/
