// Package main — admin implementation updated to save config in etcd instead of only config.json.
package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"image/color"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	Utility "github.com/globulario/utility"
	"github.com/globulario/services/golang/admin/adminpb"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/security"
	ps "github.com/shirou/gopsutil/process"

	"github.com/jackpal/gateway"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	// NEW: etcd client (for saving global config) and grpc creds
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	// NEW: for emitting a config-updated event (optional but handy)
	"github.com/globulario/services/golang/event/event_client"
)

const chunkSize = 5 * 1024 // 5KB stream chunk

// ----------------------------------------------------------------------------
// Process management
// ----------------------------------------------------------------------------

// HasRunningProcess checks if there are any running processes with the specified name.
// It retrieves the process IDs matching the given name and returns a response indicating
// whether any such processes are currently running.
//
// Parameters:
//   ctx - The context for the request, used for cancellation and deadlines.
//   rqst - The request containing the name of the process to check.
//
// Returns:
//   *adminpb.HasRunningProcessResponse - The response indicating if any matching processes are running.
//   error - An error if the process lookup fails.
func (admin_server *server) HasRunningProcess(ctx context.Context, rqst *adminpb.HasRunningProcessRequest) (*adminpb.HasRunningProcessResponse, error) {
	ids, err := Utility.GetProcessIdsByName(rqst.Name)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%v", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err),
		)
	}
	return &adminpb.HasRunningProcessResponse{Result: len(ids) > 0}, nil
}

// Update handles a streaming update request for the server executable.
// It receives chunks of data and platform information from the client,
// verifies the platform compatibility, and checks if the update is necessary
// by comparing checksums. If an update is required, it backs up the existing
// executable, writes the new data, and attempts to terminate the running process.
// Returns an error if any step fails or if no update is needed.
func (admin_server *server) Update(stream adminpb.AdminService_UpdateServer) error {
	var (
		buf      bytes.Buffer
		platform string
	)

	for {
		msg, err := stream.Recv()
		if err == io.EOF || msg == nil || len(msg.Data) == 0 {
			if sendErr := stream.SendAndClose(&adminpb.UpdateResponse{}); sendErr != nil {
				slog.Error("update: send close failed", "err", sendErr)
				return sendErr
			}
			break
		}
		if err != nil {
			slog.Error("update: receive failed", "err", err)
			return err
		}

		if len(msg.Platform) > 0 {
			platform = msg.Platform
		}
		if len(msg.Data) > 0 {
			if _, werr := buf.Write(msg.Data); werr != nil {
				slog.Error("update: buffer write failed", "err", werr)
				return werr
			}
		}
	}

	if platform == "" {
		return errors.New("update: no platform was given")
	}

	want := runtime.GOOS + ":" + runtime.GOARCH
	if platform != want {
		return errors.New("update: wrong executable platform: want " + want + ", got " + platform)
	}

	path := config.GetGlobularExecPath()
	existingChecksum := Utility.CreateFileChecksum(path)
	newChecksum := Utility.CreateDataChecksum(buf.Bytes())

	if existingChecksum == newChecksum {
		return errors.New("update: no update needed (same checksum)")
	}

	backup := path + "_" + existingChecksum
	if err := os.Rename(path, backup); err != nil {
		return err
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o755); err != nil {
		return err
	}

	pids, err := Utility.GetProcessIdsByName(filepath.Base(path))
	if err != nil {
		return err
	}
	if len(pids) == 0 {
		slog.Info("update: no running process found to terminate")
		return nil
	}
	if err := Utility.TerminateProcess(pids[0], 0); err != nil {
		return err
	}
	return nil
}

// DownloadGlobular streams the Globular executable file to the client based on the requested platform.
// It validates the platform against the server's OS and architecture, checks for the existence of the executable,
// and reads the file in chunks, sending each chunk through the gRPC stream.
// Returns an error if the platform is invalid, the executable is missing, or any I/O or streaming error occurs.
func (admin_server *server) DownloadGlobular(rqst *adminpb.DownloadGlobularRequest, stream adminpb.AdminService_DownloadGlobularServer) error {
	platform := strings.TrimSpace(rqst.Platform)
	if platform == "" {
		return errors.New("download: no platform was given")
	}
	want := runtime.GOOS + ":" + runtime.GOARCH
	if platform != want {
		return errors.New("download: wrong platform: got " + platform + ", want " + want)
	}

	path := config.GetGlobularExecPath()
	if !Utility.Exists(path) {
		return errors.New("download: executable not found at path " + path)
	}

	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := f.Close(); cerr != nil {
			slog.Warn("download: close failed", "err", cerr)
		}
	}()

	reader := bufio.NewReader(f)
	for {
		var b [chunkSize]byte
		n, rerr := reader.Read(b[:])
		if n > 0 {
			if sendErr := stream.Send(&adminpb.DownloadGlobularResponse{Data: b[:n]}); sendErr != nil {
				return sendErr
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return rerr
		}
	}
	return nil
}

// KillProcess terminates a process with the specified PID.
// It receives a KillProcessRequest containing the PID of the process to kill.
// If the process termination fails, it returns an internal error with details.
// On success, it returns an empty KillProcessResponse.
func (admin_server *server) KillProcess(ctx context.Context, rqst *adminpb.KillProcessRequest) (*adminpb.KillProcessResponse, error) {
	pid := int(rqst.Pid)
	if err := Utility.TerminateProcess(pid, 0); err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err),
		)
	}
	return &adminpb.KillProcessResponse{}, nil
}

// KillProcesses terminates all processes matching the specified name provided in the request.
// It uses the Utility.KillProcessByName function to perform the termination.
// Returns an error if the process termination fails, otherwise returns an empty response.
func (admin_server *server) KillProcesses(ctx context.Context, rqst *adminpb.KillProcessesRequest) (*adminpb.KillProcessesResponse, error) {
	if err := Utility.KillProcessByName(rqst.Name); err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err),
		)
	}
	return &adminpb.KillProcessesResponse{}, nil
}

// GetPids retrieves the process IDs (PIDs) of all running processes that match the given name.
// It takes a context and a GetPidsRequest containing the process name to search for.
// Returns a GetPidsResponse with a list of matching PIDs, or an error if the operation fails.
func (admin_server *server) GetPids(ctx context.Context, rqst *adminpb.GetPidsRequest) (*adminpb.GetPidsResponse, error) {
	pidsRaw, err := Utility.GetProcessIdsByName(rqst.Name)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err),
		)
	}
	pids := make([]int32, len(pidsRaw))
	for i, v := range pidsRaw {
		pids[i] = int32(v)
	}
	return &adminpb.GetPidsResponse{Pids: pids}, nil
}

// RunCmd executes a system command as specified in the RunCmdRequest and streams the output back to the client.
// The command to execute, its arguments, working directory, and blocking behavior are provided in the request.
// If blocking is true, the command's output is read line by line and sent to the client until completion.
// If blocking is false, the command is started and its process ID is sent to the client immediately.
// Returns an error if the command fails to start or execute.
func (admin_server *server) RunCmd(rqst *adminpb.RunCmdRequest, stream adminpb.AdminService_RunCmdServer) error {
	baseCmd := rqst.Cmd
	cmdArgs := rqst.Args
	isBlocking := rqst.Blocking

	cmd := exec.Command(baseCmd, cmdArgs...)
	if dir := strings.TrimSpace(rqst.Path); dir != "" {
		cmd.Dir = dir
	}

	if isBlocking {
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return status.Errorf(
				codes.Internal,
				"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err),
			)
		}
		output := make(chan string)
		done := make(chan struct{})

		go func() {
			defer close(done)
			for line := range output {
				var pid int32
				if cmd.Process != nil {
					pid = int32(cmd.Process.Pid)
				}
				_ = stream.Send(&adminpb.RunCmdResponse{
					Pid:    pid,
					Result: line,
				})
			}
		}()

		go Utility.ReadOutput(output, stdout)

		if err := cmd.Start(); err != nil {
			slog.Error("runcmd: start failed", "err", err, "cmd", baseCmd, "args", cmdArgs)
			return status.Errorf(
				codes.Internal,
				"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err),
			)
		}
		if err := cmd.Wait(); err != nil {
			slog.Error("runcmd: wait failed", "err", err, "cmd", baseCmd)
			return status.Errorf(
				codes.Internal,
				"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err),
			)
		}

		_ = stdout.Close()
		close(output)
		<-done

		var pid int32
		if cmd.ProcessState != nil && cmd.Process != nil {
			pid = int32(cmd.Process.Pid)
		}
		_ = stream.Send(&adminpb.RunCmdResponse{Pid: pid, Result: "done"})
		return nil
	}

	if err := cmd.Start(); err != nil {
		slog.Error("runcmd: start failed", "err", err, "cmd", baseCmd, "args", cmdArgs)
		return status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err),
		)
	}
	var pid int32
	if cmd.Process != nil {
		pid = int32(cmd.Process.Pid)
	}
	_ = stream.Send(&adminpb.RunCmdResponse{Pid: pid, Result: ""})
	return nil
}

// ----------------------------------------------------------------------------
// Environment variables
// ----------------------------------------------------------------------------

// SetEnvironmentVariable sets an environment variable on the server.
//
// It receives a SetEnvironmentVariableRequest containing the name and value of the environment variable to set.
// If the operation is successful, it returns an empty SetEnvironmentVariableResponse.
// If an error occurs while setting the environment variable, it returns an appropriate gRPC error.
//
// Parameters:
//   ctx - The context for the request.
//   rqst - The request containing the environment variable name and value.
//
// Returns:
//   *adminpb.SetEnvironmentVariableResponse - The response object.
//   error - An error if the operation fails.
func (admin_server *server) SetEnvironmentVariable(ctx context.Context, rqst *adminpb.SetEnvironmentVariableRequest) (*adminpb.SetEnvironmentVariableResponse, error) {
	if err := Utility.SetEnvironmentVariable(rqst.Name, rqst.Value); err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err),
		)
	}
	return &adminpb.SetEnvironmentVariableResponse{}, nil
}

// GetEnvironmentVariable retrieves the value of the specified environment variable.
// It takes a context and a GetEnvironmentVariableRequest containing the variable name.
// Returns a GetEnvironmentVariableResponse with the variable's value, or an error if retrieval fails.
func (admin_server *server) GetEnvironmentVariable(ctx context.Context, rqst *adminpb.GetEnvironmentVariableRequest) (*adminpb.GetEnvironmentVariableResponse, error) {
	value, err := Utility.GetEnvironmentVariable(rqst.Name)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err),
		)
	}
	return &adminpb.GetEnvironmentVariableResponse{Value: value}, nil
}

// UnsetEnvironmentVariable handles a request to unset an environment variable on the server.
// It receives the name of the environment variable to unset from the request and attempts to remove it.
// If the operation fails, it returns an internal error with details; otherwise, it returns a successful response.
//
// Parameters:
//   - ctx: The context for the request, used for cancellation and deadlines.
//   - rqst: The request containing the name of the environment variable to unset.
//
// Returns:
//   - *adminpb.UnsetEnvironmentVariableResponse: The response indicating success.
//   - error: An error if the environment variable could not be unset.
func (admin_server *server) UnsetEnvironmentVariable(ctx context.Context, rqst *adminpb.UnsetEnvironmentVariableRequest) (*adminpb.UnsetEnvironmentVariableResponse, error) {
	if err := Utility.UnsetEnvironmentVariable(rqst.Name); err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err),
		)
	}
	return &adminpb.UnsetEnvironmentVariableResponse{}, nil
}

// ----------------------------------------------------------------------------
// Certificates & (GLOBAL) config
// ----------------------------------------------------------------------------

// GetCertificates generates and installs client certificates for a specified domain.
// It accepts parameters such as domain, port, certificate path, country, state, city,
// organization, and alternate domains from the request. If the path is not provided,
// it defaults to the system's temporary directory. The function calls
// security.InstallClientCertificates to create the certificates and returns the key,
// certificate, and CA certificate in the response. In case of an error, it returns
// an appropriate gRPC status error.
func (admin_server *server) GetCertificates(ctx context.Context, rqst *adminpb.GetCertificatesRequest) (*adminpb.GetCertificatesResponse, error) {
	path := strings.TrimSpace(rqst.Path)
	if path == "" {
		path = os.TempDir()
	}
	port := 80
	if rqst.Port != 0 {
		port = int(rqst.Port)
	}
	alternateDomains := make([]interface{}, len(rqst.AlternateDomains))
	for i := range rqst.AlternateDomains {
		alternateDomains[i] = rqst.AlternateDomains[i]
	}

	key, cert, ca, err := security.InstallClientCertificates(
		rqst.Domain, port, path,
		rqst.Country, rqst.State, rqst.City, rqst.Organization,
		alternateDomains,
	)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err),
		)
	}

	return &adminpb.GetCertificatesResponse{
		Certkey: key,
		Cert:    cert,
		Cacert:  ca,
	}, nil
}


// SaveConfig saves the provided configuration to the system.
// It performs the following steps:
// 1. Validates and pretty-prints the JSON configuration from the request.
// 2. Persists the configuration to etcd as the source of truth.
// 3. Optionally notifies listeners that the global configuration has changed.
// Returns an error if the configuration is invalid or if persistence fails.
func (admin_server *server) SaveConfig(ctx context.Context, rqst *adminpb.SaveConfigRequest) (*adminpb.SaveConfigRequest, error) {
	pretty, err := Utility.PrettyPrint([]byte(rqst.Config))
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid JSON: %v", err)
	}

	// 1) Persist to etcd (source of truth)
	if err := putSystemConfigEtcd(string(pretty)); err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err),
		)
	}

	// 3) Notify listeners that global config changed (optional)
	_ = publishEvent("update_globular_configuration_evt", pretty)

	return &adminpb.SaveConfigRequest{}, nil
}

// putSystemConfigEtcd stores the global config JSON under /globular/system/config.
func putSystemConfigEtcd(jsonStr string) error {
	// derive endpoint from local address
	addr, _ := config.GetAddress()
	host := addr
	if i := strings.Index(addr, ":"); i > 0 {
		host = addr[:i]
	}
	if strings.TrimSpace(host) == "" {
		host = "127.0.0.1"
	}

	cli, err := clientv3.New(clientv3.Config{
		Endpoints:            []string{host + ":2379"},
		DialTimeout:          3 * time.Second,
		DialKeepAliveTime:    10 * time.Second,
		DialKeepAliveTimeout: 3 * time.Second,
		DialOptions:          []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())},
	})
	if err != nil {
		return err
	}
	defer cli.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	_, err = cli.Put(ctx, "/globular/system/config", jsonStr)
	return err
}

// publishEvent posts a small notification on the local event bus.
// If the event service is unavailable, it silently returns an error.
func publishEvent(topic string, payload []byte) error {
	address, _ := config.GetAddress()
	ec, err := event_client.NewEventService_Client(address, "event.EventService")
	if err != nil {
		return err
	}
	return ec.Publish(topic, payload)
}

// ----------------------------------------------------------------------------
// Host/process info & files
// ----------------------------------------------------------------------------

// GetProcessInfos streams information about running processes to the client.
// It continuously retrieves the list of processes and sends matching process information
// based on the request filters (PID and Name) via the provided gRPC stream.
// If a specific PID or Name is requested, only matching processes are sent and the stream ends.
// Otherwise, the method streams all processes every second until the client cancels the request.
// Returns an error if process information cannot be retrieved or if sending the response fails.
func (admin_server *server) GetProcessInfos(rqst *adminpb.GetProcessInfosRequest, stream adminpb.AdminService_GetProcessInfosServer) error {
	for {
		if err := stream.Context().Err(); err != nil {
			return nil
		}

		procs, err := ps.Processes()
		if err != nil {
			return status.Errorf(
				codes.Internal,
				"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err),
			)
		}

		out := make([]*adminpb.ProcessInfo, 0, len(procs))
		for _, p := range procs {
			pi := new(adminpb.ProcessInfo)
			pi.Pid = int32(p.Pid)
			pi.Ppid, _ = p.Ppid()
			pi.Name, _ = p.Name()
			pi.Exec, _ = p.Exe()
			pi.User, _ = p.Username()

			if nice, err := p.Nice(); err == nil {
				switch {
				case nice <= 10:
					pi.Priority = "Very Low"
				case nice > 10 && nice < 20:
					pi.Priority = "Low"
				case nice >= 20 && nice < 30:
					pi.Priority = "Normal"
				case nice >= 30 && nice < 40:
					pi.Priority = "High"
				case nice >= 40:
					pi.Priority = "Very High"
				}
			}

			pi.CpuUsagePercent, _ = p.CPUPercent()
			pi.MemoryUsagePercent, _ = p.MemoryPercent()

			if memInfo, err := p.MemoryInfo(); err == nil {
				pi.MemoryUsage = memInfo.Data
			}

			match := true
			if rqst.Pid > 0 && rqst.Pid != pi.Pid {
				match = false
			}
			if rqst.Name != "" && rqst.Name != pi.Name {
				match = false
			}
			if match {
				out = append(out, pi)
				if rqst.Pid > 0 {
					break
				}
			}
		}

		if err := stream.Send(&adminpb.GetProcessInfosResponse{Infos: out}); err != nil {
			return err
		}
		if rqst.Name != "" || rqst.Pid > 0 {
			return nil
		}
		time.Sleep(time.Second)
	}
}

// GetFileInfo retrieves information about a file or directory specified by the request path.
// It normalizes the path, checks for existence, and gathers metadata such as name, size,
// modification time, and whether it is a directory. If the path is a directory, it also
// lists its immediate children and includes their metadata in the response.
// Returns a GetFileInfoResponse containing the file or directory information, or an error
// if the path does not exist or cannot be accessed.
func (admin_server *server) GetFileInfo(ctx context.Context, rqst *adminpb.GetFileInfoRequest) (*adminpb.GetFileInfoResponse, error) {
	path := strings.ReplaceAll(rqst.Path, "\\", "/")
	if !Utility.Exists(path) {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(
				Utility.FunctionName(),
				Utility.FileLine(),
				errors.New("no dir found at path "+rqst.Path),
			),
		)
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

	if info.IsDir() {
		result.Path = path
	} else if idx := strings.LastIndex(path, "/"); idx >= 0 {
		result.Path = path[:idx]
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err),
		)
	}

	result.Files = make([]*adminpb.FileInfo, 0, len(entries))
	for _, e := range entries {
		fi, err := e.Info()
		if err != nil {
			continue
		}
		child := &adminpb.FileInfo{
			IsDir:   fi.IsDir(),
			ModTime: fi.ModTime().Unix(),
			Name:    fi.Name(),
			Size:    fi.Size(),
			Path:    path,
		}
		result.Files = append(result.Files, child)
	}

	return &adminpb.GetFileInfoResponse{Info: result}, nil
}


// GetAvailableHosts scans the local network for available hosts using the "arp-scan" utility.
// It discovers the network gateway and parses the scan output to collect host information,
// including IP addresses, MAC addresses, and hostnames. The function attempts to resolve
// hostnames for each discovered IP and enriches the information for the host matching the server's MAC address.
// Returns a GetAvailableHostsResponse containing the list of discovered hosts, or an error if the scan fails.
func (srv *server) GetAvailableHosts(ctx context.Context, rqst *adminpb.GetAvailableHostsRequest) (*adminpb.GetAvailableHostsResponse, error) {
	cmd := exec.Command("arp-scan", "--localnet")
	output, err := cmd.CombinedOutput()
	if err != nil {
		slog.Error("arp-scan failed", "err", err)
		return nil, err
	}

	gw, err := gateway.DiscoverGateway()
	if err != nil {
		slog.Error("discover gateway failed", "err", err)
		return nil, err
	}

	hostInfos := parseArpOutput(string(output), gw.String())
	ipHostnameMap := Utility.GetHostnameIPMap(gw.String())

	hostname, _ := os.Hostname()

	for _, hi := range hostInfos {
		if hi.Mac == srv.Mac {
			hi.Name = hostname
			if model, e := getComputerModel(); e == nil {
				hi.Infos = model
			}
		}
		if hn, ok := ipHostnameMap[hi.Ip]; ok {
			hi.Name = hn
		}
	}

	return &adminpb.GetAvailableHostsResponse{Hosts: hostInfos}, nil
}

func getComputerModel() (string, error) {
	var out, stderr bytes.Buffer
	cmd := exec.Command("dmidecode", "-s", "baseboard-manufacturer")
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return strings.TrimSpace(out.String()), nil
}

func parseArpOutput(output string, gateway string) []*adminpb.HostInfo {
	var results []*adminpb.HostInfo

	ipRegex := regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)
	macRegex := regexp.MustCompile(`([0-9a-fA-F]{2}[:-]){5}([0-9a-fA-F]{2})`)

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		ips := ipRegex.FindAllString(line, -1)
		if len(ips) == 0 {
			continue
		}
		ip := ips[0]
		macs := macRegex.FindAllString(line, -1)

		var mac string
		if len(macs) > 0 {
			mac = macs[0]
		}

		if ip == gateway || mac == "" || strings.Contains(line, "(DUP:") {
			continue
		}

		infos := ""
		if mac != "" && ip != "" {
			infos = strings.ReplaceAll(line, ip, "")
			infos = strings.ReplaceAll(infos, mac, "")
			infos = strings.TrimSpace(infos)
		}

		results = append(results, &adminpb.HostInfo{
			Ip:    ip,
			Mac:   strings.ToLower(mac),
			Infos: infos,
			Name:  "",
		})
	}
	return results
}

// (Unused) — keeps the "image/color" import non-blank in some build setups.
var _ color.Color
