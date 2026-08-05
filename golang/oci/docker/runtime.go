package docker

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/globulario/services/golang/oci"
)

type Runtime struct {
	client *Client
}

func NewRuntime(endpoint string) (*Runtime, error) {
	client, err := NewClient(endpoint)
	if err != nil {
		return nil, err
	}
	return &Runtime{client: client}, nil
}

func (r *Runtime) Ping(ctx context.Context) error { return r.client.Ping(ctx) }

func (r *Runtime) Capabilities(ctx context.Context) (oci.RuntimeCapabilities, error) {
	versionResp, err := r.client.request(ctx, http.MethodGet, "/version", nil, nil, false)
	if err != nil {
		return oci.RuntimeCapabilities{}, err
	}
	var version struct {
		Version       string `json:"Version"`
		APIVersion    string `json:"ApiVersion"`
		MinAPIVersion string `json:"MinAPIVersion"`
		OS            string `json:"Os"`
		Arch          string `json:"Arch"`
	}
	if err := json.NewDecoder(versionResp.Body).Decode(&version); err != nil {
		closeQuietly(versionResp.Body)
		return oci.RuntimeCapabilities{}, err
	}
	closeQuietly(versionResp.Body)

	infoResp, err := r.client.Do(ctx, http.MethodGet, "/info", nil, nil)
	if err != nil {
		return oci.RuntimeCapabilities{}, err
	}
	defer closeQuietly(infoResp.Body)
	var info struct {
		Name            string                     `json:"Name"`
		NCPU            int                        `json:"NCPU"`
		MemTotal        int64                      `json:"MemTotal"`
		Driver          string                     `json:"Driver"`
		DockerRootDir   string                     `json:"DockerRootDir"`
		OperatingSystem string                     `json:"OperatingSystem"`
		OSType          string                     `json:"OSType"`
		Architecture    string                     `json:"Architecture"`
		Runtimes        map[string]json.RawMessage `json:"Runtimes"`
		SecurityOptions []string                   `json:"SecurityOptions"`
	}
	if err := json.NewDecoder(infoResp.Body).Decode(&info); err != nil {
		return oci.RuntimeCapabilities{}, err
	}
	runtimes := make([]string, 0, len(info.Runtimes))
	for name := range info.Runtimes {
		runtimes = append(runtimes, name)
	}
	sort.Strings(runtimes)
	_, nvidia := info.Runtimes["nvidia"]
	osName := info.OperatingSystem
	if osName == "" {
		osName = info.OSType
	}
	arch := info.Architecture
	if arch == "" {
		arch = version.Arch
	}
	return oci.RuntimeCapabilities{
		Provider:               "docker",
		ServerVersion:          version.Version,
		APIVersion:             version.APIVersion,
		MinAPIVersion:          version.MinAPIVersion,
		OperatingSystem:        osName,
		Architecture:           arch,
		StorageDriver:          info.Driver,
		DockerRootDir:          info.DockerRootDir,
		NodeName:               info.Name,
		CPUs:                   info.NCPU,
		MemoryBytes:            info.MemTotal,
		Runtimes:               runtimes,
		NVIDIARuntimeAvailable: nvidia,
		SecurityOptions:        info.SecurityOptions,
	}, nil
}

func (r *Runtime) PullImage(ctx context.Context, ref string, creds oci.RegistryCredentials, force bool) (oci.ImageState, error) {
	if !force {
		if image, err := r.InspectImage(ctx, ref); err == nil && image.ID != "" {
			return image, nil
		}
	}
	auth, err := registryAuthHeader(creds)
	if err != nil {
		return oci.ImageState{}, err
	}
	values := url.Values{"fromImage": []string{ref}}
	resp, err := r.client.Do(ctx, http.MethodPost, queryPath("/images/create", values), nil, map[string]string{"X-Registry-Auth": auth})
	if err != nil {
		return oci.ImageState{}, err
	}
	defer closeQuietly(resp.Body)
	decoder := json.NewDecoder(resp.Body)
	for {
		var event struct {
			Error       string `json:"error"`
			ErrorDetail struct {
				Message string `json:"message"`
			} `json:"errorDetail"`
		}
		if err := decoder.Decode(&event); err != nil {
			if err == io.EOF {
				break
			}
			return oci.ImageState{}, fmt.Errorf("decode Docker pull stream: %w", err)
		}
		if event.Error != "" || event.ErrorDetail.Message != "" {
			message := event.Error
			if message == "" {
				message = event.ErrorDetail.Message
			}
			return oci.ImageState{}, fmt.Errorf("docker image pull failed: %s", message)
		}
	}
	return r.InspectImage(ctx, ref)
}

func (r *Runtime) InspectImage(ctx context.Context, ref string) (oci.ImageState, error) {
	resp, err := r.client.Do(ctx, http.MethodGet, "/images/"+url.PathEscape(ref)+"/json", nil, nil)
	if err != nil {
		return oci.ImageState{}, err
	}
	defer closeQuietly(resp.Body)
	var image struct {
		ID          string   `json:"Id"`
		RepoTags    []string `json:"RepoTags"`
		RepoDigests []string `json:"RepoDigests"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&image); err != nil {
		return oci.ImageState{}, err
	}
	return oci.ImageState{ID: image.ID, RepoTags: image.RepoTags, Digests: image.RepoDigests}, nil
}

func (r *Runtime) InspectContainer(ctx context.Context, idOrName string) (oci.ContainerState, error) {
	resp, err := r.client.Do(ctx, http.MethodGet, "/containers/"+url.PathEscape(idOrName)+"/json", nil, nil)
	if IsNotFound(err) {
		return oci.ContainerState{}, nil
	}
	if err != nil {
		return oci.ContainerState{}, err
	}
	defer closeQuietly(resp.Body)
	return decodeContainer(resp.Body)
}

func (r *Runtime) CreateContainer(ctx context.Context, spec oci.ContainerCreateSpec) (oci.ContainerState, error) {
	request := buildCreateRequest(spec)
	resp, err := r.client.Do(ctx, http.MethodPost, queryPath("/containers/create", url.Values{"name": []string{spec.Name}}), request, nil)
	if err != nil {
		return oci.ContainerState{}, err
	}
	defer closeQuietly(resp.Body)
	var created struct {
		ID       string   `json:"Id"`
		Warnings []string `json:"Warnings"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		return oci.ContainerState{}, err
	}
	if created.ID == "" {
		return oci.ContainerState{}, fmt.Errorf("docker create response omitted container ID")
	}
	return r.InspectContainer(ctx, created.ID)
}

func (r *Runtime) StartContainer(ctx context.Context, id string) error {
	resp, err := r.client.Do(ctx, http.MethodPost, "/containers/"+url.PathEscape(id)+"/start", nil, nil)
	if resp != nil {
		closeQuietly(resp.Body)
	}
	return err
}

func (r *Runtime) StopContainer(ctx context.Context, id string, timeout time.Duration) error {
	seconds := int(timeout.Seconds())
	if seconds < 0 {
		seconds = 0
	}
	path := queryPath("/containers/"+url.PathEscape(id)+"/stop", url.Values{"t": []string{strconv.Itoa(seconds)}})
	resp, err := r.client.Do(ctx, http.MethodPost, path, nil, nil)
	if IsNotFound(err) {
		return nil
	}
	if resp != nil {
		closeQuietly(resp.Body)
	}
	return err
}

func (r *Runtime) WaitContainer(ctx context.Context, id string) (int, error) {
	path := queryPath("/containers/"+url.PathEscape(id)+"/wait", url.Values{"condition": []string{"not-running"}})
	resp, err := r.client.Do(ctx, http.MethodPost, path, nil, nil)
	if err != nil {
		return 0, err
	}
	defer closeQuietly(resp.Body)
	var result struct {
		StatusCode int `json:"StatusCode"`
		Error      *struct {
			Message string `json:"Message"`
		} `json:"Error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}
	if result.Error != nil && result.Error.Message != "" {
		return result.StatusCode, fmt.Errorf("docker wait: %s", result.Error.Message)
	}
	return result.StatusCode, nil
}

func (r *Runtime) StreamLogs(ctx context.Context, id string, stdout, stderr io.Writer) error {
	values := url.Values{
		"follow":     []string{"1"},
		"stdout":     []string{"1"},
		"stderr":     []string{"1"},
		"timestamps": []string{"0"},
	}
	resp, err := r.client.Do(ctx, http.MethodGet, queryPath("/containers/"+url.PathEscape(id)+"/logs", values), nil, nil)
	if err != nil {
		return err
	}
	defer closeQuietly(resp.Body)
	return demuxDockerStream(resp.Body, stdout, stderr)
}

func (r *Runtime) RemoveContainer(ctx context.Context, id string, force, removeVolumes bool) error {
	values := url.Values{
		"force": []string{strconv.FormatBool(force)},
		"v":     []string{strconv.FormatBool(removeVolumes)},
	}
	resp, err := r.client.Do(ctx, http.MethodDelete, queryPath("/containers/"+url.PathEscape(id), values), nil, nil)
	if IsNotFound(err) {
		return nil
	}
	if resp != nil {
		closeQuietly(resp.Body)
	}
	return err
}

func registryAuthHeader(creds oci.RegistryCredentials) (string, error) {
	payload := struct {
		Username      string `json:"username,omitempty"`
		Password      string `json:"password,omitempty"`
		ServerAddress string `json:"serveraddress,omitempty"`
		IdentityToken string `json:"identitytoken,omitempty"`
	}{creds.Username, creds.Password, creds.ServerAddress, creds.IdentityToken}
	b, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

type createRequest struct {
	Hostname     string              `json:"Hostname,omitempty"`
	User         string              `json:"User,omitempty"`
	Env          []string            `json:"Env,omitempty"`
	Cmd          []string            `json:"Cmd,omitempty"`
	Image        string              `json:"Image"`
	Entrypoint   []string            `json:"Entrypoint,omitempty"`
	Labels       map[string]string   `json:"Labels,omitempty"`
	ExposedPorts map[string]struct{} `json:"ExposedPorts,omitempty"`
	HostConfig   hostConfig          `json:"HostConfig"`
}

type hostConfig struct {
	Binds          []string                 `json:"Binds,omitempty"`
	NetworkMode    string                   `json:"NetworkMode,omitempty"`
	PortBindings   map[string][]portBinding `json:"PortBindings,omitempty"`
	Memory         int64                    `json:"Memory,omitempty"`
	NanoCPUs       int64                    `json:"NanoCpus,omitempty"`
	ShmSize        int64                    `json:"ShmSize,omitempty"`
	ReadonlyRootfs bool                     `json:"ReadonlyRootfs,omitempty"`
	Privileged     bool                     `json:"Privileged,omitempty"`
	CapAdd         []string                 `json:"CapAdd,omitempty"`
	CapDrop        []string                 `json:"CapDrop,omitempty"`
	SecurityOpt    []string                 `json:"SecurityOpt,omitempty"`
	RestartPolicy  struct {
		Name string `json:"Name"`
	} `json:"RestartPolicy"`
	DeviceRequests []deviceRequest `json:"DeviceRequests,omitempty"`
}

type portBinding struct {
	HostIP   string `json:"HostIp,omitempty"`
	HostPort string `json:"HostPort,omitempty"`
}

type deviceRequest struct {
	Driver       string            `json:"Driver,omitempty"`
	Count        int               `json:"Count,omitempty"`
	DeviceIDs    []string          `json:"DeviceIDs,omitempty"`
	Capabilities [][]string        `json:"Capabilities,omitempty"`
	Options      map[string]string `json:"Options,omitempty"`
}

func buildCreateRequest(spec oci.ContainerCreateSpec) createRequest {
	user := strings.TrimSpace(spec.Security.User)
	if group := strings.TrimSpace(spec.Security.Group); group != "" {
		user += ":" + group
	}
	req := createRequest{
		Hostname:   spec.Hostname,
		User:       user,
		Env:        spec.Environment,
		Cmd:        spec.Command,
		Image:      spec.Image,
		Entrypoint: spec.Entrypoint,
		Labels:     spec.Labels,
		HostConfig: hostConfig{
			NetworkMode:    defaultNetwork(spec.Network.Mode),
			Memory:         uint64ToInt64(spec.Resources.MemoryBytes),
			NanoCPUs:       spec.Resources.NanoCPUs,
			ShmSize:        uint64ToInt64(spec.Resources.ShmBytes),
			ReadonlyRootfs: spec.Security.ReadOnlyRootFilesystem,
			Privileged:     spec.Security.Privileged,
			CapAdd:         append([]string(nil), spec.Security.AddCapabilities...),
			CapDrop:        append([]string(nil), spec.Security.DropCapabilities...),
		},
	}
	req.HostConfig.RestartPolicy.Name = "no"
	if spec.Security.NoNewPrivileges {
		req.HostConfig.SecurityOpt = []string{"no-new-privileges:true"}
	}
	for _, m := range spec.Mounts {
		bind := m.Source + ":" + m.Target
		if m.ReadOnly {
			bind += ":ro"
		}
		req.HostConfig.Binds = append(req.HostConfig.Binds, bind)
	}
	if len(spec.Network.Ports) > 0 && defaultNetwork(spec.Network.Mode) != "host" {
		req.ExposedPorts = make(map[string]struct{})
		req.HostConfig.PortBindings = make(map[string][]portBinding)
		for _, p := range spec.Network.Ports {
			proto := strings.ToLower(strings.TrimSpace(p.Protocol))
			if proto == "" {
				proto = "tcp"
			}
			key := fmt.Sprintf("%d/%s", p.ContainerPort, proto)
			req.ExposedPorts[key] = struct{}{}
			if p.HostPort != 0 {
				req.HostConfig.PortBindings[key] = []portBinding{{HostIP: p.HostIP, HostPort: strconv.Itoa(int(p.HostPort))}}
			}
		}
	}
	gpu := spec.Resources.GPU
	if gpu.Count > 0 || len(gpu.DeviceIDs) > 0 {
		driver := strings.TrimSpace(gpu.Driver)
		if driver == "" {
			driver = "nvidia"
		}
		count := gpu.Count
		if len(gpu.DeviceIDs) > 0 {
			count = 0
		}
		req.HostConfig.DeviceRequests = []deviceRequest{{
			Driver:       driver,
			Count:        count,
			DeviceIDs:    append([]string(nil), gpu.DeviceIDs...),
			Capabilities: [][]string{{"gpu"}},
		}}
	}
	return req
}

func decodeContainer(reader io.Reader) (oci.ContainerState, error) {
	var payload struct {
		ID     string `json:"Id"`
		Name   string `json:"Name"`
		Image  string `json:"Image"`
		Config struct {
			Image  string            `json:"Image"`
			Labels map[string]string `json:"Labels"`
		} `json:"Config"`
		State struct {
			Status     string `json:"Status"`
			Running    bool   `json:"Running"`
			ExitCode   int    `json:"ExitCode"`
			Error      string `json:"Error"`
			StartedAt  string `json:"StartedAt"`
			FinishedAt string `json:"FinishedAt"`
		} `json:"State"`
	}
	if err := json.NewDecoder(reader).Decode(&payload); err != nil {
		return oci.ContainerState{}, err
	}
	return oci.ContainerState{
		ID:         payload.ID,
		Name:       strings.TrimPrefix(payload.Name, "/"),
		Image:      payload.Config.Image,
		ImageID:    payload.Image,
		Labels:     payload.Config.Labels,
		Status:     payload.State.Status,
		Running:    payload.State.Running,
		ExitCode:   payload.State.ExitCode,
		Error:      payload.State.Error,
		StartedAt:  parseDockerTime(payload.State.StartedAt),
		FinishedAt: parseDockerTime(payload.State.FinishedAt),
	}, nil
}

func parseDockerTime(value string) time.Time {
	if value == "" || strings.HasPrefix(value, "0001-") {
		return time.Time{}
	}
	t, _ := time.Parse(time.RFC3339Nano, value)
	return t
}

func demuxDockerStream(reader io.Reader, stdout, stderr io.Writer) error {
	buf := bufio.NewReader(reader)
	for {
		header, err := buf.Peek(8)
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if (header[0] != 1 && header[0] != 2) || header[1] != 0 || header[2] != 0 || header[3] != 0 {
			_, err := io.Copy(stdout, buf)
			return err
		}
		header = make([]byte, 8)
		if _, err := io.ReadFull(buf, header); err != nil {
			return err
		}
		size := binary.BigEndian.Uint32(header[4:8])
		out := stdout
		if header[0] == 2 {
			out = stderr
		}
		if _, err := io.CopyN(out, buf, int64(size)); err != nil {
			return err
		}
	}
}

func defaultNetwork(mode string) string {
	mode = strings.TrimSpace(mode)
	if mode == "" {
		return "bridge"
	}
	return mode
}

func uint64ToInt64(v uint64) int64 {
	if v > uint64(^uint64(0)>>1) {
		return int64(^uint64(0) >> 1)
	}
	return int64(v)
}
