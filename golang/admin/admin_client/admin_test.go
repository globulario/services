package admin_client

import (
	//"encoding/json"
	"log"
	"testing"

	//"time"
	"github.com/globulario/services/golang/resource/resource_client"
)

var (
	// Connect to the admin client.
	client, _   = NewAdminService_Client("mon-intranet:10097", "admin.AdminService")
	resource, _ = resource_client.NewResourceService_Client("mon-intranet:10097", "resource.ResourceService")
)

// Test various function here.

func TestGetConfig(t *testing.T) {
	config, err := client.GetConfig()
	if err != nil {
		log.Println(err)
		t.FailNow()
	}
	log.Println("Get Config succeed!", config)
}

func TestGetFullConfig(t *testing.T) {
	token, err := resource.Authenticate("sa", "adminadmin")
	if err != nil {
		log.Println("Fail to authenticate to mon-intranet:10012")
		log.Println(err.Error())
		return
	}
	log.Println(token)
	config, err := client.GetFullConfig()
	if err != nil {
		log.Println(err)
		t.FailNow()
	}
	log.Println(config)
	log.Println("GetFullConfig succeed!")
}

// Test modify the config...
/*func TestSaveConfig(t *testing.T) {
	log.Println("---> test get config.")

	configStr, err := client.GetFullConfig()
	if err != nil {
		log.Println("---> ", err)
	}

	// So here I will use a map to intialyse config...
	config := make(map[string]interface{})
	err = json.Unmarshal([]byte(configStr), &config)
	if err != nil {
		log.Println("---> ", err)
	}

	// Print the services configuration here.
	config["IdleTimeout"] = 220

	configStr_, _ := json.Marshal(config)

	// Here I will save the configuration directly...
	err = client.SaveConfig(string(configStr_))
	if err != nil {
		log.Println("---> ", err)
	}

	// Now I will try to save a single service.
	serviceConfig := config["Services"].(map[string]interface{})["echo_server"].(map[string]interface{})
	serviceConfig["Port"] = 10029 // set new port number.
	serviceConfigStr_, _ := json.Marshal(serviceConfig)
	err = client.SaveConfig(string(serviceConfigStr_))
	if err != nil {
		log.Println("---> ", err)
	}
}*/
/*
func TestInstallService(t *testing.T) {
	err := client.InstallService("localhost", "globulario", "b94d0011-39a0-4bdb-9a5c-7e9abc23b26b")
	if err != nil {
		log.Panicln(err)
	}
	time.Sleep(time.Second * 5)
}
*/
/*
func TestStartService(t *testing.T) {
	log.Println("---> test get config.")

	service_pid, proxy_pid, err := client.StartService("spc.SpcService")
	if err != nil {

		log.Println("---> ", err)
	}
	log.Println("service pid:", service_pid, " proxy pid:", proxy_pid)

}
*/

func TestStopService(t *testing.T) {
	token, err := resource.Authenticate("sa", "adminadmin")
	if err != nil {
		log.Println("Fail to authenticate to mon-intranet:10097")
		log.Println(err.Error())
		return
	}
	log.Println(token)
	err = client.StopService("efc.EntityFrameworkService")
	if err != nil {
		log.Println(err)
		return
	}
	log.Println("---> stop service succeeded")
}

/*
func TestUninstallService(t *testing.T) {
	err := client.UninstallService("globulario", "b94d0011-39a0-4bdb-9a5c-7e9abc23b26b", "0.0.1")
	if err != nil {
		log.Panicln(err)
	}
}
*/
/*
func TestRestartServices(t *testing.T) {
	err := client.RestartServices()
	if err != nil {
		log.Println(err)
		t.FailNow()
	}
	log.Println("RestartServices succeed!")
}
*/
/*func TestDeployApplication(t *testing.T) {
	err := client.DeployApplication("testApp", "/home/dave/Documents/chitchat")
	if err != nil {
		log.Panicln(err)
	}
}*/

// Test register/start external service.
/*func TestRegisterExternalService(t *testing.T) {
	// Start mongo db
	pid, err := client.RegisterExternalApplication("mongoDB_srv_win64", "E:\\MongoDB\\bin\\mongod.exe", []string{"--port", "27017", "--dbpath", "E:\\MongoDB\\data\\db"})

	if err == nil {
		log.Println("---> mongo db start at port: ", pid)
	} else {
		log.Println("---> err", err)
	}
}*/

/*func TestPublishService(t *testing.T) {
	err := client.PublishService("echo_server", "localhost:8080", "localhost:8080", "Echo is the simplest serive of all.", []string{"test", "echo"})
	if err != nil {
		log.Panicln(err)
	}
}
*/
/*
func TestRunCmd(t *testing.T) {
	var err error
	client_, err := resource_client.NewResourceService_Client("mon-iis-01:8080", "resource.ResourceService")
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

	results, err := client.RunCmd(token, "cmd", []string{"/C", "dir", "C:\\"}, true)
	if err != nil {
		log.Println(err)
		t.FailNow()
	}

	log.Println(results)
}

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
