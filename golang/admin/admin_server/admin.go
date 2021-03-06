package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"io/ioutil"
	"log"
	"os"
	"runtime"
	// "golang.org/x/sys/windows/registry"
	"path/filepath"

	"os/exec"
	"github.com/globulario/services/golang/security"

	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/admin/adminpb"

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
		if err == io.EOF || msg == nil {
			// end of stream...
			stream.SendAndClose(&adminpb.UpdateResponse{})
			err = nil
			break
		} else if err != nil {
			return err
		} else if len(msg.Data) == 0 {
			break
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

	ex, err := os.Executable()
	if err != nil {
		return err
	}

	path := filepath.Dir(ex)

	path += "/Globular"
	if runtime.GOOS == "windows" {
		path += ".exe"
	}

	existing_checksum := Utility.CreateFileChecksum(path)
	checksum := Utility.CreateDataChecksum(buffer.Bytes())
	if existing_checksum == checksum {
		return errors.New("no update needed")
	}

	// Move the actual file to other file...
	err = os.Rename(path, path+"_"+checksum)
	if err != nil {
		return err
	}

	/** So here I will change the current server path and save the new executable **/
	err = ioutil.WriteFile(path, buffer.Bytes(), 0755)
	if err != nil {
		return err
	}

	// exit
	log.Println("stop globular made use systemctl to restart globular automaticaly")

	// TODO restart Globular exec...
	// Utility.TerminateProcess(pid, 0); // send signal to globular...

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

	ex, err := os.Executable()
	if err != nil {
		return err
	}

	path := filepath.Dir(ex)
	path += "/Globular"
	if runtime.GOOS == "windows" {
		path += ".exe"
	}

	// No I will stream the result over the networks.
	data, err := os.Open(path)
	if err != nil {
		log.Fatal(err)
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
			// send the data to the server.
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

		cmd.Run()

		cmd.Wait()

		// Close the output.
		stdout.Close()
		done <- true

	} else {
		err := cmd.Start()
		if err != nil {
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

	// Create the certificate at the given path.
	key, cert, ca, err := security.InstallCertificates(rqst.Domain, port, path)

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
