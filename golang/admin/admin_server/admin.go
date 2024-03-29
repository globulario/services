package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/admin/adminpb"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/security"
	ps "github.com/shirou/gopsutil/process"

	"github.com/jackpal/gateway"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

/**
 * Test if a process with a given name is Running on the server.
 * By default that function is accessible by sa only.
 */
func (admin_server *server) HasRunningProcess(ctx context.Context, rqst *adminpb.HasRunningProcessRequest) (*adminpb.HasRunningProcessResponse, error) {
	ids, err := Utility.GetProcessIdsByName(rqst.Name)

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	if len(ids) == 0 {
		return &adminpb.HasRunningProcessResponse{
			Result: false,
		}, nil
	}

	return &adminpb.HasRunningProcessResponse{
		Result: true,
	}, nil
}

/**
 * Update Globular itself with a new version.
 */
func (admin_server *server) Update(stream adminpb.AdminService_UpdateServer) error {
	// The buffer that will receive the service executable.
	var buffer bytes.Buffer
	var platform string
	for {
		msg, err := stream.Recv()
		if err == io.EOF || msg == nil || len(msg.Data) == 0 {
			// end of stream...
			err_ := stream.SendAndClose(&adminpb.UpdateResponse{})
			if err_ != nil {
				fmt.Println("fail send response and close stream with error ", err_)
				return err_
			}
			err = nil
			break
		} else if err != nil {
			return err
		} else {
			buffer.Write(msg.Data)
		}

		if len(msg.Platform) > 0 {
			platform = msg.Platform
		}

	}

	if len(platform) == 0 {
		return errors.New("no platform was given")
	}

	platform_ := runtime.GOOS + ":" + runtime.GOARCH
	if platform != platform_ {
		return errors.New("Wrong executable platform to update from! wants " + platform_ + " not " + platform)
	}

	path := config.GetGlobularExecPath()

	existing_checksum := Utility.CreateFileChecksum(path)
	checksum := Utility.CreateDataChecksum(buffer.Bytes())
	if existing_checksum == checksum {
		return errors.New("no update needed")
	}

	// Move the actual file to other file...
	err := os.Rename(path, path+"_"+existing_checksum)
	if err != nil {
		return err
	}

	/** So here I will change the current server path and save the new executable **/
	err = ioutil.WriteFile(path, buffer.Bytes(), 0755)
	if err != nil {
		return err
	}

	// exit
	fmt.Println("stop globular made use systemctl to restart globular automaticaly")

	// TODO restart Globular exec...
	pids, err := Utility.GetProcessIdsByName("Globular")
	if err != nil {
		return err
	}

	err = Utility.TerminateProcess(pids[0], 0) // send signal to globular...
	if err != nil {
		return err
	}

	return nil
}

// Download the actual globular exec file.
func (admin_server *server) DownloadGlobular(rqst *adminpb.DownloadGlobularRequest, stream adminpb.AdminService_DownloadGlobularServer) error {
	platform := rqst.Platform

	if len(platform) == 0 {
		return errors.New("no platform was given")
	}

	platform_ := runtime.GOOS + ":" + runtime.GOARCH
	if platform != platform_ {
		return errors.New("Wrong executable platform to update from get " + platform + " want " + platform_)
	}

	path := config.GetGlobularExecPath()

	if !Utility.Exists(path) {
		return errors.New("fail to retreive exec at path: " + path)
	}

	// No I will stream the result over the networks.
	data, err := os.Open(path)
	if err != nil {
		return err
	}
	defer data.Close()

	reader := bufio.NewReader(data)
	const BufferSize = 1024 * 5 // the chunck size.

	for {
		var data [BufferSize]byte
		bytesread, err := reader.Read(data[0:BufferSize])
		if bytesread > 0 {
			rqst := &adminpb.DownloadGlobularResponse{
				Data: data[0:bytesread],
			}
			// send the data to the srv.
			err = stream.Send(rqst)
		}

		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}
	}

	return nil
}

// Kill process by id
func (admin_server *server) KillProcess(ctx context.Context, rqst *adminpb.KillProcessRequest) (*adminpb.KillProcessResponse, error) {
	pid := int(rqst.Pid)
	err := Utility.TerminateProcess(pid, 0)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &adminpb.KillProcessResponse{}, nil
}

// Kill process by name
func (admin_server *server) KillProcesses(ctx context.Context, rqst *adminpb.KillProcessesRequest) (*adminpb.KillProcessesResponse, error) {
	err := Utility.KillProcessByName(rqst.Name)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &adminpb.KillProcessesResponse{}, nil
}

// Return the list of process id with a given name.
func (admin_server *server) GetPids(ctx context.Context, rqst *adminpb.GetPidsRequest) (*adminpb.GetPidsResponse, error) {
	pids_, err := Utility.GetProcessIdsByName(rqst.Name)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	pids := make([]int32, len(pids_))
	for i := 0; i < len(pids_); i++ {
		pids[i] = int32(pids_[i])
	}

	return &adminpb.GetPidsResponse{
		Pids: pids,
	}, nil
}

// Run an external command must be use with care.
func (admin_server *server) RunCmd(rqst *adminpb.RunCmdRequest, stream adminpb.AdminService_RunCmdServer) error {

	baseCmd := rqst.Cmd
	cmdArgs := rqst.Args
	isBlocking := rqst.Blocking
	pid := -1
	cmd := exec.Command(baseCmd, cmdArgs...)
	if len(rqst.Path) > 0 {
		cmd.Dir = rqst.Path
	}

	if isBlocking {

		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
		output := make(chan string)
		done := make(chan bool)

		// Process message util the command is done.
		go func() {
			for {
				select {
				case <-done:

					stream.Send(
						&adminpb.RunCmdResponse{
							Pid:    int32(pid),
							Result: "done",
						},
					)
					break

				case result := <-output:
					if cmd.Process != nil {
						pid = cmd.Process.Pid
					}

					stream.Send(
						&adminpb.RunCmdResponse{
							Pid:    int32(pid),
							Result: result,
						},
					)
				}
			}
		}()

		// Start reading the output
		go Utility.ReadOutput(output, stdout)
		err = cmd.Run()
		if err != nil {
			fmt.Println("fail to run command ", err)
			return status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}

		cmd.Wait()

		// Close the output.
		stdout.Close()
		done <- true

	} else {
		err := cmd.Start()
		if err != nil {
			fmt.Println("fail to run command ", err)
			return status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
		if cmd.Process != nil {
			pid = cmd.Process.Pid
		}

		stream.Send(
			&adminpb.RunCmdResponse{
				Pid:    int32(pid),
				Result: "",
			},
		)

	}

	return nil
}

// Set environement variable.
func (admin_server *server) SetEnvironmentVariable(ctx context.Context, rqst *adminpb.SetEnvironmentVariableRequest) (*adminpb.SetEnvironmentVariableResponse, error) {
	err := Utility.SetEnvironmentVariable(rqst.Name, rqst.Value)

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &adminpb.SetEnvironmentVariableResponse{}, nil
}

// Get environement variable.
func (admin_server *server) GetEnvironmentVariable(ctx context.Context, rqst *adminpb.GetEnvironmentVariableRequest) (*adminpb.GetEnvironmentVariableResponse, error) {
	value, err := Utility.GetEnvironmentVariable(rqst.Name)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &adminpb.GetEnvironmentVariableResponse{
		Value: value,
	}, nil
}

// Delete environement variable.
func (admin_server *server) UnsetEnvironmentVariable(ctx context.Context, rqst *adminpb.UnsetEnvironmentVariableRequest) (*adminpb.UnsetEnvironmentVariableResponse, error) {

	err := Utility.UnsetEnvironmentVariable(rqst.Name)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &adminpb.UnsetEnvironmentVariableResponse{}, nil
}

///////////////////////////////////// API //////////////////////////////////////

// Get certificates from the server and copy them into the the a given directory.
// path: The path where to copy the certificates
// port: The server configuration port the default is 80.
//
// ex. Here is an exemple of the command run from the shell,
//
// Globular certificates -domain=globular.cloud -path=/tmp -port=80
//
// The command can return
func (admin_server *server) GetCertificates(ctx context.Context, rqst *adminpb.GetCertificatesRequest) (*adminpb.GetCertificatesResponse, error) {

	path := rqst.Path
	if len(path) == 0 {
		path = os.TempDir()
	}

	port := 80
	if rqst.Port != 0 {
		port = int(rqst.Port)
	}

	// set missing fileds...
	alternateDomains := make([]interface{}, len(rqst.AlternateDomains))
	for i := 0; i < len(rqst.AlternateDomains); i++ {
		alternateDomains[i] = rqst.AlternateDomains[i]
	}

	// Create the certificate at the given path.
	key, cert, ca, err := security.InstallClientCertificates(rqst.Domain, port, path, rqst.Country, rqst.State, rqst.City, rqst.Organization, alternateDomains)

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &adminpb.GetCertificatesResponse{
		Certkey: key,
		Cert:    cert,
		Cacert:  ca,
	}, nil
}

// Save Globular configuration.
func (admin_server *server) SaveConfig(ctx context.Context, rqst *adminpb.SaveConfigRequest) (*adminpb.SaveConfigRequest, error) {
	jsonStr, err := Utility.PrettyPrint([]byte(rqst.Config))
	if err != nil {
		return nil, err
	}

	configPath := config.GetConfigDir() + "/config.json"

	err = ioutil.WriteFile(configPath, jsonStr, 0644)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &adminpb.SaveConfigRequest{}, nil
}

// Return the list of process or the process with a given name or id
func (admin_server *server) GetProcessInfos(rqst *adminpb.GetProcessInfosRequest, stream adminpb.AdminService_GetProcessInfosServer) error {

	for {
		// if the connection is close...
		err := stream.Context().Err()
		if err != nil {
			fmt.Println("exit connection")
			break
		}

		// get the list of processes.
		process, err := ps.Processes()
		if err != nil {
			return status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}

		process_ := make([]*adminpb.ProcessInfo, 0)

		// Get the list of all process...
		for i := 0; i < len(process); i++ {
			p := process[i]
			p_ := new(adminpb.ProcessInfo)
			p_.Pid = int32(p.Pid)
			p_.Ppid, _ = p.Ppid()
			p_.Name, _ = p.Name()
			p_.Exec, _ = p.Exe()
			p_.User, _ = p.Username()
			nice, _ := p.Nice()
			if nice <= 10 {
				p_.Priority = "Very Low"
			} else if nice > 10 && nice < 20 {
				p_.Priority = "Low"
			} else if nice >= 20 && nice < 30 {
				p_.Priority = "Normal"
			} else if nice >= 30 && nice < 40 {
				p_.Priority = "High"
			} else if nice >= 40 {
				p_.Priority = "Very High"
			}

			p_.CpuUsagePercent, _ = p.CPUPercent()
			p_.MemoryUsagePercent, _ = p.MemoryPercent()

			memInfo, err := p.MemoryInfo()
			if err == nil {
				p_.MemoryUsage = memInfo.Data
			}

			// memoryInfo, _ := p.MemoryInfoEx()
			// p_.MemoryUsage = memoryInfo.Data
			if len(rqst.Name) > 0 || rqst.Pid > 0 {
				if rqst.Pid > 0 {
					if rqst.Pid == p_.Pid {
						process_ = append(process_, p_)
						break
					}

				}
				if len(rqst.Name) > 0 {
					if rqst.Name == p_.Name {
						process_ = append(process_, p_)
					}
				}
			} else {
				process_ = append(process_, p_)
			}
		}

		// Send value
		rsp := &adminpb.GetProcessInfosResponse{Infos: process_}
		stream.Send(rsp)
		if len(rqst.Name) > 0 || rqst.Pid > 0 {
			break
		}

		time.Sleep(time.Second * 1) // wait one second...
	}

	// return normaly
	return nil
}

// Retrun file info from the server (absolute path)
func (admin_server *server) GetFileInfo(ctx context.Context, rqst *adminpb.GetFileInfoRequest) (*adminpb.GetFileInfoResponse, error) {
	path := strings.ReplaceAll(rqst.Path, "\\", "/")
	if !Utility.Exists(path) {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no dir found at path "+rqst.Path)))
	}

	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	result := new(adminpb.FileInfo)
	result.IsDir = info.IsDir()
	result.ModTime = info.ModTime().Unix()
	result.Name = info.Name()
	result.Size = info.Size()
	result.Path = path[0:strings.LastIndex(path, "/")]

	files, err := ioutil.ReadDir(path)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	result.Files = make([]*adminpb.FileInfo, 0)
	for i := 0; i < len(files); i++ {
		info := files[i]
		file := new(adminpb.FileInfo)
		file.IsDir = info.IsDir()
		file.ModTime = info.ModTime().Unix()
		file.Name = info.Name()
		file.Size = info.Size()
		file.Path = path
		result.Files = append(result.Files, file)
	}

	return &adminpb.GetFileInfoResponse{Info: result}, nil
}

// Get the list of available host on the network.
func (srv *server) GetAvailableHosts(ctx context.Context, rqst *adminpb.GetAvailableHostsRequest) (*adminpb.GetAvailableHostsResponse, error) {
	// Run the arp -a command
	cmd := exec.Command("arp-scan", "--localnet")

	// Capture the command output
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("Error running arp -a: %v\n", err)
		return nil, err
	}

	gateway, err := gateway.DiscoverGateway()
	if err != nil {
		fmt.Println("fail to get gateway with error ", err)
		return nil, err
	}

	// Process the output if needed
	hostInfos := parseArpOutput(string(output), gateway.String())
	ipHostnameMap := Utility.GetHostnameIPMap(gateway.String())

	// I will append the current host to the list of host.
	for i := 0; i < len(hostInfos); i++ {
		hostInfo := hostInfos[i]
		if hostInfo.Mac == srv.Mac {
			hostInfo.Name, _ = os.Hostname()
			hostInfo.Infos, _ = getComputerModel()
		}
		
		if hostname, ok := ipHostnameMap[hostInfo.Ip]; ok {
			hostInfo.Name = hostname
		}

	}

	return &adminpb.GetAvailableHostsResponse{
		Hosts: hostInfos,
	}, nil
}



func getComputerModel() (string, error) {
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd := exec.Command("dmidecode", "-s", "baseboard-manufacturer")
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out.String()), nil

}

func parseArpOutput(output string, gateway string) []*adminpb.HostInfo {
	var arpEntries []*adminpb.HostInfo

	// Define regular expressions for IPv4, MAC address, and domain name
	ipRegex := regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)
	macRegex := regexp.MustCompile(`([0-9a-fA-F]{2}[:-]){5}([0-9a-fA-F]{2})`)

	// Split the output into lines
	lines := strings.Split(output, "\n")

	// Process each line
	for _, line := range lines {
		// Match IPv4 address
		ipMatches := ipRegex.FindAllString(line, -1)
		if len(ipMatches) > 0 {
			ip := ipMatches[0]

			// Match MAC address
			macMatches := macRegex.FindAllString(line, -1)
			var mac string
			if len(macMatches) > 0 {
				mac = macMatches[0]
			}

			// Match domain name
			infos := ""

			if len(mac) > 0 && len(ip) > 0 {
				infos = strings.ReplaceAll(line, ip, "")
				infos = strings.ReplaceAll(infos, mac, "")
				infos = strings.TrimSpace(infos)
			}

			// Append the parsed entry to the result
			if ip != gateway && len(mac) > 0  && !strings.Contains(line, "(DUP:"){
				arpEntries = append(arpEntries, &adminpb.HostInfo{Ip: ip, Mac: mac, Infos: infos, Name: ""})
			}
		}
	}

	return arpEntries
}
