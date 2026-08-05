package docker

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/globulario/services/golang/oci"
)

func TestRuntimePullAndCreateUseImmutableReferenceAndNoRestart(t *testing.T) {
	const ref = "registry.example.com/team/demo@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	var gotAuth string
	var gotCreate createRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch {
		case req.URL.Path == "/version":
			json.NewEncoder(w).Encode(map[string]any{"ApiVersion": "1.44", "Version": "26.1"})
		case req.URL.Path == "/v1.44/images/create":
			gotAuth = req.Header.Get("X-Registry-Auth")
			if req.URL.Query().Get("fromImage") != ref {
				t.Errorf("fromImage = %q", req.URL.Query().Get("fromImage"))
			}
			io.WriteString(w, "{\"status\":\"pulling\"}\n{\"status\":\"done\"}\n")
		case strings.HasPrefix(req.URL.Path, "/v1.44/images/"):
			json.NewEncoder(w).Encode(map[string]any{"Id": "sha256:image", "RepoDigests": []string{ref}})
		case req.URL.Path == "/v1.44/containers/create":
			if err := json.NewDecoder(req.Body).Decode(&gotCreate); err != nil {
				t.Fatal(err)
			}
			json.NewEncoder(w).Encode(map[string]any{"Id": "c1"})
		case req.URL.Path == "/v1.44/containers/c1/json":
			json.NewEncoder(w).Encode(map[string]any{
				"Id": "c1", "Name": "/globular-demo", "Image": "sha256:image",
				"Config": map[string]any{"Image": ref, "Labels": map[string]string{"io.globular.managed": "true"}},
				"State":  map[string]any{"Status": "created", "Running": false, "ExitCode": 0},
			})
		default:
			http.Error(w, req.URL.Path, http.StatusNotFound)
		}
	}))
	defer server.Close()

	runtime, err := NewRuntime(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	creds := oci.RegistryCredentials{ServerAddress: "registry.example.com", Username: "user", Password: "secret"}
	if _, err := runtime.PullImage(context.Background(), ref, creds, true); err != nil {
		t.Fatal(err)
	}
	decoded, err := base64.RawURLEncoding.DecodeString(gotAuth)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(decoded, []byte(`"password":"secret"`)) {
		t.Fatalf("registry auth = %s", decoded)
	}

	_, err = runtime.CreateContainer(context.Background(), oci.ContainerCreateSpec{
		Name:      "globular-demo",
		Image:     ref,
		Resources: oci.ResourceSpec{GPU: oci.GPURequest{Count: 1}},
		Security:  oci.SecuritySpec{NoNewPrivileges: true, ReadOnlyRootFilesystem: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotCreate.Image != ref {
		t.Fatalf("create image = %q", gotCreate.Image)
	}
	if gotCreate.HostConfig.RestartPolicy.Name != "no" {
		t.Fatalf("restart policy = %q, want no", gotCreate.HostConfig.RestartPolicy.Name)
	}
	if len(gotCreate.HostConfig.DeviceRequests) != 1 || gotCreate.HostConfig.DeviceRequests[0].Driver != "nvidia" {
		t.Fatalf("device requests = %+v", gotCreate.HostConfig.DeviceRequests)
	}
	if len(gotCreate.HostConfig.SecurityOpt) != 1 || gotCreate.HostConfig.SecurityOpt[0] != "no-new-privileges:true" {
		t.Fatalf("security opts = %#v", gotCreate.HostConfig.SecurityOpt)
	}
}

func TestInspectContainerNotFoundReturnsEmptyState(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/version" {
			json.NewEncoder(w).Encode(map[string]string{"ApiVersion": "1.44"})
			return
		}
		http.Error(w, `{"message":"No such container"}`, http.StatusNotFound)
	}))
	defer server.Close()
	runtime, _ := NewRuntime(server.URL)
	state, err := runtime.InspectContainer(context.Background(), "missing")
	if err != nil {
		t.Fatal(err)
	}
	if state.Exists() {
		t.Fatalf("state = %+v, want empty", state)
	}
}

func TestDemuxDockerStream(t *testing.T) {
	var stream bytes.Buffer
	writeFrame := func(kind byte, text string) {
		header := make([]byte, 8)
		header[0] = kind
		binary.BigEndian.PutUint32(header[4:], uint32(len(text)))
		stream.Write(header)
		stream.WriteString(text)
	}
	writeFrame(1, "hello\n")
	writeFrame(2, "bad\n")
	var stdout, stderr bytes.Buffer
	if err := demuxDockerStream(&stream, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if stdout.String() != "hello\n" || stderr.String() != "bad\n" {
		t.Fatalf("stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestStopUsesBoundedTimeout(t *testing.T) {
	seen := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/version" {
			json.NewEncoder(w).Encode(map[string]string{"ApiVersion": "1.44"})
			return
		}
		seen <- req.URL.Query().Get("t")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	runtime, _ := NewRuntime(server.URL)
	if err := runtime.StopContainer(context.Background(), "c1", 12*time.Second); err != nil {
		t.Fatal(err)
	}
	if got := <-seen; got != "12" {
		t.Fatalf("timeout = %q, want 12", got)
	}
}
