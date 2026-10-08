package docker

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/G6kco/CyberSpace/internal/labs"
)

// fakeDaemon answers the few Docker Engine API calls the runtime makes, so
// the requests can be checked without a Docker daemon.
type fakeDaemon struct {
	mu       sync.Mutex
	created  map[string]any // decoded body of the last create
	name     string         // name of the last create
	started  []string
	removed  []string
	existing map[string]bool
	listing  string
}

var apiVersion = regexp.MustCompile(`^/v[0-9.]+`)

func (d *fakeDaemon) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	d.mu.Lock()
	defer d.mu.Unlock()

	path := apiVersion.ReplaceAllString(r.URL.Path, "")
	w.Header().Set("Api-Version", "1.47")
	switch {
	case path == "/_ping":
		_, _ = w.Write([]byte("OK"))

	case r.Method == http.MethodPost && path == "/containers/create":
		d.name = r.URL.Query().Get("name")
		_ = json.NewDecoder(r.Body).Decode(&d.created)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"Id":"abc123","Warnings":[]}`))

	case r.Method == http.MethodPost && strings.HasSuffix(path, "/start"):
		d.started = append(d.started, strings.TrimSuffix(strings.TrimPrefix(path, "/containers/"), "/start"))
		w.WriteHeader(http.StatusNoContent)

	case r.Method == http.MethodDelete && strings.HasPrefix(path, "/containers/"):
		name := strings.TrimPrefix(path, "/containers/")
		if r.URL.Query().Get("force") != "1" || r.URL.Query().Get("v") != "1" {
			http.Error(w, `{"message":"expected force and volume removal"}`, http.StatusBadRequest)
			return
		}
		if !d.existing[name] {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"No such container: ` + name + `"}`))
			return
		}
		d.removed = append(d.removed, name)
		w.WriteHeader(http.StatusNoContent)

	case r.Method == http.MethodGet && path == "/containers/json":
		if !strings.Contains(r.URL.Query().Get("filters"), "cyberspace.managed=true") {
			http.Error(w, `{"message":"expected the platform label filter"}`, http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(d.listing))

	default:
		http.Error(w, `{"message":"unexpected request"}`, http.StatusNotImplemented)
	}
}

func newTestRuntime(t *testing.T, daemon *fakeDaemon) *Runtime {
	t.Helper()
	server := httptest.NewServer(daemon)
	t.Cleanup(server.Close)

	t.Setenv("DOCKER_HOST", "tcp://"+strings.TrimPrefix(server.URL, "http://"))
	t.Setenv("DOCKER_TLS_VERIFY", "")
	t.Setenv("DOCKER_CERT_PATH", "")
	t.Setenv("DOCKER_API_VERSION", "")

	runtime, err := New(context.Background())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	t.Cleanup(func() { _ = runtime.Close() })
	return runtime
}

func TestStartCreatesContainerOnNetworkWithFixedIP(t *testing.T) {
	daemon := &fakeDaemon{}
	runtime := newTestRuntime(t, daemon)

	id, err := runtime.Start(context.Background(), labs.Container{
		Name:       "cyberspace-lab-7",
		Image:      "scenario1",
		Network:    "macvlan",
		IP:         netip.MustParseAddr("192.168.20.182"),
		InstanceID: 7,
	})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if id != "abc123" || daemon.name != "cyberspace-lab-7" || len(daemon.started) != 1 || daemon.started[0] != "abc123" {
		t.Fatalf("id = %q, name = %q, started = %v", id, daemon.name, daemon.started)
	}

	var body struct {
		Image            string
		Labels           map[string]string
		NetworkingConfig struct {
			EndpointsConfig map[string]struct {
				IPAMConfig struct{ IPv4Address string }
			}
		}
	}
	raw, _ := json.Marshal(daemon.created)
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("decode create body: %v", err)
	}
	if body.Image != "scenario1" {
		t.Fatalf("Image = %q; want scenario1", body.Image)
	}
	if body.Labels[labelManaged] != "true" || body.Labels[labelInstance] != "7" {
		t.Fatalf("Labels = %v; want the platform labels", body.Labels)
	}
	if got := body.NetworkingConfig.EndpointsConfig["macvlan"].IPAMConfig.IPv4Address; got != "192.168.20.182" {
		t.Fatalf("macvlan IPv4Address = %q; want 192.168.20.182", got)
	}
}

func TestRemoveIgnoresMissingContainer(t *testing.T) {
	daemon := &fakeDaemon{existing: map[string]bool{"cyberspace-lab-7": true}}
	runtime := newTestRuntime(t, daemon)

	if err := runtime.Remove(context.Background(), "cyberspace-lab-7"); err != nil {
		t.Fatalf("Remove(existing) error = %v", err)
	}
	if err := runtime.Remove(context.Background(), "cyberspace-lab-8"); err != nil {
		t.Fatalf("Remove(missing) error = %v; want nil", err)
	}
	if len(daemon.removed) != 1 || daemon.removed[0] != "cyberspace-lab-7" {
		t.Fatalf("removed = %v", daemon.removed)
	}
}

func TestManagedListsLabelledContainers(t *testing.T) {
	daemon := &fakeDaemon{listing: `[
		{"Id":"a","Names":["/cyberspace-lab-7"],"Labels":{"cyberspace.managed":"true","cyberspace.lab_instance_id":"7"}},
		{"Id":"b","Names":["/cyberspace-lab-x"],"Labels":{"cyberspace.managed":"true","cyberspace.lab_instance_id":"x"}}
	]`}
	runtime := newTestRuntime(t, daemon)

	managed, err := runtime.Managed(context.Background())
	if err != nil {
		t.Fatalf("Managed() error = %v", err)
	}
	if len(managed) != 1 || managed["cyberspace-lab-7"] != 7 {
		t.Fatalf("Managed() = %v; want only the well-formed container", managed)
	}
}
