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
)

// server is assumed to be defined elsewhere in the same package.
// type server struct { Mac string /* ... other fields ... */ }

const chunkSize = 5 * 1024 // 5KB stream chunk

// ----------------------------------------------------------------------------
// Process management
// ----------------------------------------------------------------------------

// HasRunningProcess reports whether a process with the given name is running.
// Accessible to 'sa' by default (enforced upstream).
func (admin_server *server) HasRunningProcess(ctx context.Context, rqst *adminpb.HasRunningProcessRequest) (*adminpb.HasRunningProcessResponse, error) {
	ids, err := Utility.GetProcessIdsByName(rqst.Name)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err),
		)
	}
	return &adminpb.HasRunningProcessResponse{Result: len(ids) > 0}, nil
}

// Update replaces the current Globular executable with a new version streamed
// over gRPC and then terminates the running process to allow an external
// supervisor to restart it.
func (admin_server *server) Update(stream adminpb.AdminService_UpdateServer) error {
	var (
		buf      bytes.Buffer
		platform string
	)

	for {
		msg, err := stream.Recv()
		if err == io.EOF || msg == nil || len(msg.Data) == 0 {
			// End of stream: close with an empty response.
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

	// Backup current executable alongside with checksum suffix.
	backup := path + "_" + existingChecksum
	if err := os.Rename(path, backup); err != nil {
		return err
	}

	// Write the new executable (0755).
	if err := os.WriteFile(path, buf.Bytes(), 0o755); err != nil {
		return err
	}

	// Attempt to terminate the running Globular so the supervisor can restart.
	pids, err := Utility.GetProcessIdsByName(filepath.Base(path))
	if err != nil {
		return err
	}
	if len(pids) == 0 {
		// Nothing to kill — return success; external supervisor may start it.
		slog.Info("update: no running process found to terminate")
		return nil
	}

	if err := Utility.TerminateProcess(pids[0], 0); err != nil {
		return err
	}
	return nil
}

// DownloadGlobular streams the current Globular executable to the caller
// for the given platform (GOOS:GOARCH).
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
		var buf [chunkSize]byte
		n, rerr := reader.Read(buf[:])
		if n > 0 {
			if sendErr := stream.Send(&adminpb.DownloadGlobularResponse{Data: buf[:n]}); sendErr != nil {
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

// KillProcess terminates a process by PID with a best-effort signal.
func (admin_server *server) KillProcess(ctx context.Context, rqst *adminpb.KillProcessRequest) (*adminpb.KillProcessResponse, error) {
	pid := int(rqst.Pid)
	if err := Utility.TerminateProcess(pid, 0); err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err),
		)
	}
	return &adminpb.KillProcessResponse{}, nil
}

// KillProcesses terminates all processes matching a given name.
func (admin_server *server) KillProcesses(ctx context.Context, rqst *adminpb.KillProcessesRequest) (*adminpb.KillProcessesResponse, error) {
	if err := Utility.KillProcessByName(rqst.Name); err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err),
		)
	}
	return &adminpb.KillProcessesResponse{}, nil
}

// GetPids returns the list of PIDs for processes matching the given name.
func (admin_server *server) GetPids(ctx context.Context, rqst *adminpb.GetPidsRequest) (*adminpb.GetPidsResponse, error) {
	pidsRaw, err := Utility.GetProcessIdsByName(rqst.Name)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err),
		)
	}
	pids := make([]int32, len(pidsRaw))
	for i, v := range pidsRaw {
		pids[i] = int32(v)
	}
	return &adminpb.GetPidsResponse{Pids: pids}, nil
}

// RunCmd executes an external command. If blocking=true, stdout is streamed back
// to the client until completion. If blocking=false, the process is started and
// the PID is returned immediately.
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
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err),
			)
		}
		output := make(chan string)
		done := make(chan struct{})

		// Stream lines to client
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

		// Start reading stdout
		go Utility.ReadOutput(output, stdout)

		if err := cmd.Start(); err != nil {
			slog.Error("runcmd: start failed", "err", err, "cmd", baseCmd, "args", cmdArgs)
			return status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err),
			)
		}
		if err := cmd.Wait(); err != nil {
			slog.Error("runcmd: wait failed", "err", err, "cmd", baseCmd)
			return status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err),
			)
		}

		// Close reader and signal completion to the sender goroutine.
		_ = stdout.Close()
		close(output)
		<-done

		// Final "done" marker
		var pid int32
		if cmd.ProcessState != nil && cmd.Process != nil {
			pid = int32(cmd.Process.Pid)
		}
		_ = stream.Send(&adminpb.RunCmdResponse{
			Pid:    pid,
			Result: "done",
		})
		return nil
	}

	// Non-blocking mode
	if err := cmd.Start(); err != nil {
		slog.Error("runcmd: start failed", "err", err, "cmd", baseCmd, "args", cmdArgs)
		return status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err),
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

// SetEnvironmentVariable sets an environment variable on the host.
func (admin_server *server) SetEnvironmentVariable(ctx context.Context, rqst *adminpb.SetEnvironmentVariableRequest) (*adminpb.SetEnvironmentVariableResponse, error) {
	if err := Utility.SetEnvironmentVariable(rqst.Name, rqst.Value); err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err),
		)
	}
	return &adminpb.SetEnvironmentVariableResponse{}, nil
}

// GetEnvironmentVariable returns the value of an environment variable.
func (admin_server *server) GetEnvironmentVariable(ctx context.Context, rqst *adminpb.GetEnvironmentVariableRequest) (*adminpb.GetEnvironmentVariableResponse, error) {
	value, err := Utility.GetEnvironmentVariable(rqst.Name)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err),
		)
	}
	return &adminpb.GetEnvironmentVariableResponse{Value: value}, nil
}

// UnsetEnvironmentVariable removes an environment variable.
func (admin_server *server) UnsetEnvironmentVariable(ctx context.Context, rqst *adminpb.UnsetEnvironmentVariableRequest) (*adminpb.UnsetEnvironmentVariableResponse, error) {
	if err := Utility.UnsetEnvironmentVariable(rqst.Name); err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err),
		)
	}
	return &adminpb.UnsetEnvironmentVariableResponse{}, nil
}

// ----------------------------------------------------------------------------
// Certificates & config
// ----------------------------------------------------------------------------

// GetCertificates generates client certificates for the caller and stores them
// at the requested path. The server’s config port (default 80) is used for ACME.
func (admin_server *server) GetCertificates(ctx context.Context, rqst *adminpb.GetCertificatesRequest) (*adminpb.GetCertificatesResponse, error) {
	path := strings.TrimSpace(rqst.Path)
	if path == "" {
		path = os.TempDir()
	}

	port := 80
	if rqst.Port != 0 {
		port = int(rqst.Port)
	}

	// Convert repeated string -> []interface{} expected by security API
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
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err),
		)
	}

	return &adminpb.GetCertificatesResponse{
		Certkey: key,
		Cert:    cert,
		Cacert:  ca,
	}, nil
}

// SaveConfig writes the provided Globular configuration JSON to config.json.
func (admin_server *server) SaveConfig(ctx context.Context, rqst *adminpb.SaveConfigRequest) (*adminpb.SaveConfigRequest, error) {
	jsonStr, err := Utility.PrettyPrint([]byte(rqst.Config))
	if err != nil {
		return nil, err
	}

	configPath := config.GetConfigDir() + "/config.json"
	if err := os.WriteFile(configPath, jsonStr, 0o644); err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err),
		)
	}
	return &adminpb.SaveConfigRequest{}, nil
}

// ----------------------------------------------------------------------------
// Host/process info & files
// ----------------------------------------------------------------------------

// GetProcessInfos streams information about processes. If rqst.Name or rqst.Pid
// is set, only matching processes are returned and the stream ends after one send.
func (admin_server *server) GetProcessInfos(rqst *adminpb.GetProcessInfosRequest, stream adminpb.AdminService_GetProcessInfosServer) error {
	for {
		// Stop if client closed the stream.
		if err := stream.Context().Err(); err != nil {
			return nil
		}

		procs, err := ps.Processes()
		if err != nil {
			return status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err),
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
					// Note: "nice" semantics vary by OS; preserving your original mapping.
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
					// When requesting a single PID, we can short-circuit.
					break
				}
			}
		}

		if err := stream.Send(&adminpb.GetProcessInfosResponse{Infos: out}); err != nil {
			return err
		}

		// If filtered by name or pid, send once and exit.
		if rqst.Name != "" || rqst.Pid > 0 {
			return nil
		}

		time.Sleep(time.Second)
	}
}

// GetFileInfo returns information about the file/directory at rqst.Path,
// alongside listing immediate children when it is a directory.
func (admin_server *server) GetFileInfo(ctx context.Context, rqst *adminpb.GetFileInfoRequest) (*adminpb.GetFileInfoResponse, error) {
	path := strings.ReplaceAll(rqst.Path, "\\", "/")
	if !Utility.Exists(path) {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(
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
		result.Path = path // directory path
	} else if idx := strings.LastIndex(path, "/"); idx >= 0 {
		result.Path = path[:idx]
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err),
		)
	}

	result.Files = make([]*adminpb.FileInfo, 0, len(entries))
	for _, e := range entries {
		fi, err := e.Info()
		if err != nil {
			// Skip entries that cannot be stat'ed.
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

// GetAvailableHosts scans the local network and returns discovered hosts,
// including the current host and gateway. Uses `arp-scan --localnet`.
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

	// Enrich entries
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

// getComputerModel returns a short string for the computer model (Linux dmidecode).
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

// parseArpOutput extracts IP/MAC pairs and vendor/info strings from arp-scan
// output, skipping the gateway and duplicate markers.
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

// (Unused) — keep import of "image/color" happy if needed by your build pipeline.
var _ color.Color
