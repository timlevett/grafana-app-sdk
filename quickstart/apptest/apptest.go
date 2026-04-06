// Package apptest provides helpers for testing Grafana app-platform apps
// built with the local SDK. It spins up an in-memory API server backed by
// SQLite so tests run without Kubernetes, Docker, or any external process.
//
// Typical usage:
//
//	func TestMyApp(t *testing.T) {
//	    ts := apptest.New(t, apptest.BookmarkResource)
//	    ts.MustCreate(t, "default", "grafana", map[string]any{
//	        "url":   "https://grafana.com",
//	        "title": "Grafana",
//	    })
//	    ts.AssertExists(t, "default", "grafana")
//	}
package apptest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/grafana/grafana-app-sdk/quickstart/local"
)

// TestServer is an in-memory API server for use in tests.
// It wraps local.Server with helpers that fail the test immediately on error.
type TestServer struct {
	srv *local.Server
	ts  *httptest.Server
	gvr local.GroupVersionResource // primary resource (first registered)
}

// New creates a TestServer with the given resources registered.
// The server is closed automatically when the test ends.
func New(t testing.TB, resources ...local.GroupVersionResource) *TestServer {
	t.Helper()
	srv, err := local.NewServer(":memory:")
	if err != nil {
		t.Fatalf("apptest.New: %v", err)
	}
	for _, r := range resources {
		srv.RegisterResource(r)
	}

	ts := httptest.NewServer(local.CORSMiddleware(srv))

	var primary local.GroupVersionResource
	if len(resources) > 0 {
		primary = resources[0]
	}

	s := &TestServer{srv: srv, ts: ts, gvr: primary}
	t.Cleanup(func() {
		ts.Close()
		srv.Close()
	})
	return s
}

// URL returns the base URL of the test server (e.g. "http://127.0.0.1:PORT").
func (ts *TestServer) URL() string { return ts.ts.URL }

// Server returns the underlying local.Server so tests can register additional
// resources, admission hooks, or reconcilers.
func (ts *TestServer) Server() *local.Server { return ts.srv }

// resourcePath returns the collection path for the given resource.
func (ts *TestServer) resourcePath(gvr local.GroupVersionResource, ns string) string {
	return fmt.Sprintf("%s/apis/%s/%s/namespaces/%s/%s",
		ts.ts.URL, gvr.Group, gvr.Version, ns, strings.ToLower(gvr.Resource))
}

// MustCreate creates a resource, failing the test if the server returns an error.
// spec must be JSON-serialisable.
func (ts *TestServer) MustCreate(t testing.TB, ns, name string, spec interface{}) local.Resource {
	t.Helper()
	return ts.MustCreateWith(t, ts.gvr, ns, name, spec)
}

// MustCreateWith creates a resource of the given GVR.
func (ts *TestServer) MustCreateWith(t testing.TB, gvr local.GroupVersionResource, ns, name string, spec interface{}) local.Resource {
	t.Helper()
	specJSON, err := json.Marshal(spec)
	if err != nil {
		t.Fatalf("MustCreate: marshal spec: %v", err)
	}
	body := fmt.Sprintf(`{"metadata":{"name":%q},"spec":%s}`, name, specJSON)
	resp, err := http.Post(ts.resourcePath(gvr, ns), "application/json", bytes.NewBufferString(body))
	if err != nil {
		t.Fatalf("MustCreate %s/%s: %v", ns, name, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		var errBody map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errBody)
		t.Fatalf("MustCreate %s/%s: expected 201, got %d: %v", ns, name, resp.StatusCode, errBody["message"])
	}
	var res local.Resource
	json.NewDecoder(resp.Body).Decode(&res)
	return res
}

// MustGet fetches a resource, failing the test if not found.
func (ts *TestServer) MustGet(t testing.TB, ns, name string) local.Resource {
	t.Helper()
	return ts.MustGetWith(t, ts.gvr, ns, name)
}

// MustGetWith fetches a resource of the given GVR.
func (ts *TestServer) MustGetWith(t testing.TB, gvr local.GroupVersionResource, ns, name string) local.Resource {
	t.Helper()
	resp, err := http.Get(ts.resourcePath(gvr, ns) + "/" + name)
	if err != nil {
		t.Fatalf("MustGet %s/%s: %v", ns, name, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		t.Fatalf("MustGet %s/%s: not found (404)", ns, name)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("MustGet %s/%s: expected 200, got %d", ns, name, resp.StatusCode)
	}
	var res local.Resource
	json.NewDecoder(resp.Body).Decode(&res)
	return res
}

// MustList returns all resources in a namespace, failing the test on error.
func (ts *TestServer) MustList(t testing.TB, ns string) []local.Resource {
	t.Helper()
	return ts.MustListWith(t, ts.gvr, ns)
}

// MustListWith returns all resources of the given GVR.
func (ts *TestServer) MustListWith(t testing.TB, gvr local.GroupVersionResource, ns string) []local.Resource {
	t.Helper()
	resp, err := http.Get(ts.resourcePath(gvr, ns))
	if err != nil {
		t.Fatalf("MustList %s: %v", ns, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("MustList %s: expected 200, got %d", ns, resp.StatusCode)
	}
	var list local.ResourceList
	json.NewDecoder(resp.Body).Decode(&list)
	return list.Items
}

// MustUpdate updates a resource, failing the test on error.
func (ts *TestServer) MustUpdate(t testing.TB, ns, name string, spec interface{}) local.Resource {
	t.Helper()
	return ts.MustUpdateWith(t, ts.gvr, ns, name, spec)
}

// MustUpdateWith updates a resource of the given GVR.
func (ts *TestServer) MustUpdateWith(t testing.TB, gvr local.GroupVersionResource, ns, name string, spec interface{}) local.Resource {
	t.Helper()
	specJSON, err := json.Marshal(spec)
	if err != nil {
		t.Fatalf("MustUpdate: marshal spec: %v", err)
	}
	body := fmt.Sprintf(`{"metadata":{"name":%q},"spec":%s}`, name, specJSON)
	req, _ := http.NewRequest(http.MethodPut, ts.resourcePath(gvr, ns)+"/"+name, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("MustUpdate %s/%s: %v", ns, name, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		var errBody map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errBody)
		t.Fatalf("MustUpdate %s/%s: expected 200, got %d: %v", ns, name, resp.StatusCode, errBody["message"])
	}
	var res local.Resource
	json.NewDecoder(resp.Body).Decode(&res)
	return res
}

// MustDelete deletes a resource, failing the test if not found.
func (ts *TestServer) MustDelete(t testing.TB, ns, name string) {
	t.Helper()
	ts.MustDeleteWith(t, ts.gvr, ns, name)
}

// MustDeleteWith deletes a resource of the given GVR.
func (ts *TestServer) MustDeleteWith(t testing.TB, gvr local.GroupVersionResource, ns, name string) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodDelete, ts.resourcePath(gvr, ns)+"/"+name, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("MustDelete %s/%s: %v", ns, name, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("MustDelete %s/%s: expected 200, got %d", ns, name, resp.StatusCode)
	}
}

// AssertExists fails the test if the resource does not exist.
func (ts *TestServer) AssertExists(t testing.TB, ns, name string) {
	t.Helper()
	resp, err := http.Get(ts.resourcePath(ts.gvr, ns) + "/" + name)
	if err != nil {
		t.Fatalf("AssertExists %s/%s: %v", ns, name, err)
	}
	resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		t.Errorf("AssertExists: %s/%s not found", ns, name)
	} else if resp.StatusCode != http.StatusOK {
		t.Errorf("AssertExists: %s/%s unexpected status %d", ns, name, resp.StatusCode)
	}
}

// AssertNotExists fails the test if the resource exists.
func (ts *TestServer) AssertNotExists(t testing.TB, ns, name string) {
	t.Helper()
	resp, err := http.Get(ts.resourcePath(ts.gvr, ns) + "/" + name)
	if err != nil {
		t.Fatalf("AssertNotExists %s/%s: %v", ns, name, err)
	}
	resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		t.Errorf("AssertNotExists: %s/%s exists but should not", ns, name)
	}
}

// AssertCount fails the test if the number of resources in ns differs from n.
func (ts *TestServer) AssertCount(t testing.TB, ns string, n int) {
	t.Helper()
	items := ts.MustList(t, ns)
	if len(items) != n {
		t.Errorf("AssertCount %s: expected %d items, got %d", ns, n, len(items))
	}
}

// PostRaw sends a raw HTTP request to the server and returns the response.
// Useful for testing edge cases not covered by the helpers above.
func (ts *TestServer) PostRaw(t testing.TB, path, contentType, body string) *http.Response {
	t.Helper()
	resp, err := http.Post(ts.ts.URL+path, contentType, strings.NewReader(body))
	if err != nil {
		t.Fatalf("PostRaw %s: %v", path, err)
	}
	return resp
}

// GetRaw sends a GET to an arbitrary path and returns the response.
func (ts *TestServer) GetRaw(t testing.TB, path string) *http.Response {
	t.Helper()
	resp, err := http.Get(ts.ts.URL + path)
	if err != nil {
		t.Fatalf("GetRaw %s: %v", path, err)
	}
	return resp
}
