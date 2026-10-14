package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	"roundtrip/internal/gitsync"
	"roundtrip/internal/model"
)

type client struct {
	t       *testing.T
	base    string
	dataDir string
}

func (c client) do(method, path string, body any, out any) int {
	c.t.Helper()
	var r io.Reader
	switch b := body.(type) {
	case nil:
	case string:
		r = strings.NewReader(b)
	default:
		data, _ := json.Marshal(b)
		r = bytes.NewReader(data)
	}
	req, _ := http.NewRequest(method, c.base+path, r)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer res.Body.Close()
	data, _ := io.ReadAll(res.Body)
	if out != nil {
		if err := json.Unmarshal(data, out); err != nil {
			c.t.Fatalf("%s %s: %v: %s", method, path, err, data)
		}
	}
	return res.StatusCode
}

func setup(t *testing.T) client {
	t.Helper()
	static := fstest.MapFS{"index.html": {Data: []byte("<html>ui</html>")}}
	dataDir := t.TempDir()
	h, err := New(Config{Workspace: t.TempDir(), DataDir: dataDir, Static: static})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return client{t, srv.URL, dataDir}
}

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestOpenRepositoryFolderAndSync(t *testing.T) {
	if !gitsync.Available() {
		t.Skip("git not installed")
	}
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "gitconfig"))
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	for _, k := range []string{"GIT_AUTHOR_NAME", "GIT_COMMITTER_NAME", "GIT_AUTHOR_EMAIL", "GIT_COMMITTER_EMAIL"} {
		t.Setenv(k, "test@example.com")
	}
	remote := filepath.Join(t.TempDir(), "acme", "requests.git")
	os.MkdirAll(filepath.Dir(remote), 0o755)
	git(t, "", "init", "--bare", "-b", "main", remote)

	c := setup(t)
	var info workspaceInfo
	c.do("GET", "/api/workspace", nil, &info)
	if info.Git.Repo || info.Path == "" {
		t.Fatalf("fresh workspace: %#v", info)
	}

	// A collection made elsewhere is carried into the repository folder, and
	// the environments follow on this machine without entering it.
	c.do("PUT", "/api/collections/c1", model.Collection{Name: "API", Items: []*model.Item{{ID: "r1", Name: "Ping", Request: &model.Request{Method: "GET", URL: "http://x"}}}}, nil)
	c.do("PUT", "/api/environments/e1", model.Environment{Name: "dev", Variables: []model.KV{{Key: "token", Value: "hunter2", Enabled: true, Secret: true}}}, nil)

	repo := filepath.Join(t.TempDir(), "requests")
	git(t, "", "clone", remote, repo)
	if code := c.do("POST", "/api/workspace/open", map[string]any{"path": repo, "carry": true}, &info); code != 200 {
		t.Fatalf("open: %d", code)
	}
	if !info.Git.Repo || info.Git.Remote != remote || mustEval(t, info.Path) != mustEval(t, repo) {
		t.Fatalf("after open: %#v", info)
	}
	if len(info.Git.Changes) != 2 {
		t.Fatalf("carried-over collection should show as uncommitted: %#v", info.Git.Changes)
	}
	var cols []model.Collection
	c.do("GET", "/api/collections", nil, &cols)
	if len(cols) != 1 || cols[0].Name != "API" {
		t.Fatalf("collections after open: %#v", cols)
	}
	var envs []model.Environment
	c.do("GET", "/api/environments", nil, &envs)
	if len(envs) != 1 || envs[0].ID != "e1" || envs[0].Variables[0].Value != "hunter2" {
		t.Fatalf("environments after open: %#v", envs)
	}
	if _, err := os.Stat(filepath.Join(info.Path, "environments")); err == nil {
		t.Fatal("environments were written into the repository")
	}

	if code := c.do("POST", "/api/git/sync", map[string]string{"message": "Add API collection"}, &info); code != 200 || len(info.Git.Changes) != 0 || !info.Git.Upstream {
		t.Fatalf("sync: %d %#v", code, info)
	}
	if log := git(t, remote, "log", "--format=%s", "main"); log != "Add API collection" {
		t.Fatalf("remote log: %q", log)
	}
	if files := git(t, remote, "ls-tree", "-r", "--name-only", "main"); files != "collections/API/Ping.yaml\ncollections/API/_collection.yaml" {
		t.Fatalf("remote files: %q", files)
	}

	// A teammate pushes; pull brings it in.
	mate := filepath.Join(t.TempDir(), "mate")
	git(t, "", "clone", remote, mate)
	os.WriteFile(filepath.Join(mate, "collections", "API", "Pong.yaml"), []byte("method: GET\nurl: http://y\n"), 0o644)
	git(t, mate, "add", "-A")
	git(t, mate, "commit", "-m", "Add pong")
	git(t, mate, "push")
	if code := c.do("POST", "/api/git/pull", nil, nil); code != 200 {
		t.Fatalf("pull: %d", code)
	}
	c.do("GET", "/api/collections", nil, &cols)
	if len(cols[0].Items) != 2 {
		t.Fatalf("pulled request missing: %#v", cols[0].Items)
	}

	// Opening a folder that already has collections never overwrites them.
	other := setup(t)
	other.do("PUT", "/api/collections/c9", model.Collection{Name: "Other"}, nil)
	if code := other.do("POST", "/api/workspace/open", map[string]any{"path": repo, "carry": true}, nil); code != 200 {
		t.Fatalf("open existing: %d", code)
	}
	other.do("GET", "/api/collections", nil, &cols)
	if len(cols) != 1 || cols[0].Name != "API" {
		t.Fatalf("existing collections were replaced: %#v", cols)
	}
	if code := c.do("POST", "/api/workspace/open", map[string]string{"path": "relative"}, nil); code != 400 {
		t.Fatalf("relative path: %d", code)
	}
}

func TestOpenWorkspaceIsRemembered(t *testing.T) {
	static := fstest.MapFS{"index.html": {Data: []byte("ui")}}
	data, other := t.TempDir(), t.TempDir()
	h, err := New(Config{Workspace: t.TempDir(), DataDir: data, Static: static})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	defer srv.Close()
	c := client{t, srv.URL, data}

	c.do("PUT", "/api/collections/c1", model.Collection{Name: "First"}, nil)
	if code := c.do("POST", "/api/workspace/open", map[string]string{"path": other}, nil); code != 200 {
		t.Fatalf("open: %d", code)
	}
	var cols []model.Collection
	c.do("GET", "/api/collections", nil, &cols)
	if len(cols) != 0 {
		t.Fatalf("new workspace should be empty: %#v", cols)
	}
	if got, _ := filepath.EvalSymlinks(LastWorkspace(data)); got != mustEval(t, other) {
		t.Fatalf("LastWorkspace = %q, want %q", got, other)
	}
}

func TestRecentWorkspaces(t *testing.T) {
	static := fstest.MapFS{"index.html": {Data: []byte("ui")}}
	first, second, third := t.TempDir(), t.TempDir(), t.TempDir()
	data := t.TempDir()
	h, err := New(Config{Workspace: first, DataDir: data, Static: static})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	defer srv.Close()
	c := client{t, srv.URL, data}

	var info workspaceInfo
	c.do("GET", "/api/workspace", nil, &info)
	if info.Recent == nil || len(info.Recent) != 0 {
		t.Fatalf("a first workspace has no others to switch to: %#v", info.Recent)
	}

	for _, dir := range []string{second, third, second} {
		if code := c.do("POST", "/api/workspace/open", map[string]string{"path": dir}, &info); code != 200 {
			t.Fatalf("open %s: %d", dir, code)
		}
	}
	// Newest first, the current workspace excluded, no duplicates.
	want := []string{mustEval(t, third), mustEval(t, first)}
	got := []string{}
	for _, p := range info.Recent {
		got = append(got, mustEval(t, p))
	}
	if !slices.Equal(got, want) {
		t.Fatalf("recent = %q, want %q", got, want)
	}

	// A folder that no longer exists is not offered.
	os.RemoveAll(third)
	c.do("GET", "/api/workspace", nil, &info)
	if len(info.Recent) != 1 || mustEval(t, info.Recent[0]) != mustEval(t, first) {
		t.Fatalf("recent after delete = %q", info.Recent)
	}
}

func mustEval(t *testing.T, p string) string {
	t.Helper()
	out, err := filepath.EvalSymlinks(p)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestCollectionAndEnvironmentCRUD(t *testing.T) {
	c := setup(t)

	var cols []model.Collection
	if code := c.do("GET", "/api/collections", nil, &cols); code != 200 || cols == nil || len(cols) != 0 {
		t.Fatalf("empty list: %d %#v", code, cols)
	}

	col := model.Collection{Name: "API", Items: []*model.Item{{ID: "r1", Name: "Ping", Request: &model.Request{Method: "GET", URL: "http://x"}}}}
	if code := c.do("PUT", "/api/collections/c1", col, nil); code != 200 {
		t.Fatalf("put: %d", code)
	}
	c.do("GET", "/api/collections", nil, &cols)
	if len(cols) != 1 || cols[0].ID != "c1" || cols[0].Items[0].Request.URL != "http://x" {
		t.Fatalf("after put: %#v", cols)
	}
	if code := c.do("DELETE", "/api/collections/c1", nil, nil); code != 204 {
		t.Fatalf("delete: %d", code)
	}
	c.do("GET", "/api/collections", nil, &cols)
	if len(cols) != 0 {
		t.Fatalf("after delete: %#v", cols)
	}

	var envs []model.Environment
	c.do("PUT", "/api/environments/e1", model.Environment{Name: "dev"}, nil)
	c.do("GET", "/api/environments", nil, &envs)
	if len(envs) != 1 || envs[0].ID != "e1" {
		t.Fatalf("envs: %#v", envs)
	}

	if code := c.do("PUT", "/api/collections/c2", "{not json", nil); code != 400 {
		t.Fatalf("bad json: %d", code)
	}
}

func TestSendResolvesEnvironmentAndRecordsHistory(t *testing.T) {
	c := setup(t)
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(r.URL.Path + " " + r.Header.Get("Authorization")))
	}))
	defer target.Close()

	c.do("PUT", "/api/environments/e1", model.Environment{Name: "dev", Variables: []model.KV{
		{Key: "base", Value: target.URL, Enabled: true},
		{Key: "token", Value: "s3cret", Enabled: true, Secret: true},
	}}, nil)

	in := map[string]any{
		"environmentId": "e1",
		"request":       model.Request{Method: "GET", URL: "{{base}}/hello", Auth: model.Auth{Type: "bearer", Token: "{{token}}"}},
	}
	var res model.Response
	if code := c.do("POST", "/api/send", in, &res); code != 200 || res.Status != 200 || res.Body != "/hello Bearer s3cret" {
		t.Fatalf("send: %d %#v", code, res)
	}

	var hist []model.HistoryEntry
	c.do("GET", "/api/history", nil, &hist)
	if len(hist) != 1 || hist[0].Request.URL != "{{base}}/hello" || hist[0].Status != 200 {
		t.Fatalf("history should keep the unresolved request: %#v", hist)
	}

	var exported struct{ Command string }
	c.do("POST", "/api/curl/export", in, &exported)
	if !strings.Contains(exported.Command, target.URL+"/hello") || !strings.Contains(exported.Command, "Bearer s3cret") {
		t.Fatalf("export: %s", exported.Command)
	}

	// A failing request is still a 200 from the API, with the error in the body.
	in["request"] = model.Request{URL: "http://127.0.0.1:1/"}
	if code := c.do("POST", "/api/send", in, &res); code != 200 || res.Error == "" {
		t.Fatalf("failed send: %d %#v", code, res)
	}

	c.do("DELETE", "/api/history", nil, nil)
	c.do("GET", "/api/history", nil, &hist)
	if len(hist) != 0 {
		t.Fatal("history not cleared")
	}
}

func TestSendRunsTestsAndRemembersExtractedVariables(t *testing.T) {
	c := setup(t)
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login" {
			w.Header().Set("X-Session", "s1")
			w.Write([]byte(`{"access_token": "tok-1", "user": {"id": 7}}`))
			return
		}
		w.Write([]byte(r.URL.Path + " " + r.Header.Get("Authorization")))
	}))
	defer target.Close()
	c.do("PUT", "/api/environments/e1", model.Environment{Name: "dev", Variables: []model.KV{
		{Key: "base", Value: target.URL, Enabled: true},
		{Key: "token", Value: "from-env", Enabled: true},
		{Key: "expectedId", Value: "7", Enabled: true},
	}}, nil)

	var vars map[string]string
	if code := c.do("GET", "/api/variables/runtime", nil, &vars); code != 200 || vars == nil || len(vars) != 0 {
		t.Fatalf("runtime variables at start: %d %#v", code, vars)
	}

	login := map[string]any{"environmentId": "e1", "request": model.Request{
		Method: "POST", URL: "{{base}}/login",
		Tests: []model.Assertion{
			{Target: "status", Op: "eq", Value: "200"},
			{Target: "body.user.id", Op: "eq", Value: "{{expectedId}}"},
			{Target: "header.x-session", Op: "eq", Value: "nope"},
		},
		Extract: []model.Extract{{Var: "token", From: "body.access_token"}, {Var: "userId", From: "body.user.id"}, {Var: "gone", From: "body.gone"}},
	}}
	var res model.Response
	c.do("POST", "/api/send", login, &res)
	want := []model.TestResult{
		{Target: "status", Op: "eq", Value: "200", Passed: true},
		{Target: "body.user.id", Op: "eq", Value: "{{expectedId}}", Passed: true},
		{Target: "header.x-session", Op: "eq", Value: "nope", Message: `got "s1"`},
		{Target: "body.gone", Op: "extract", Value: "gone", Message: "not found"},
	}
	if !reflect.DeepEqual(res.Tests, want) {
		t.Fatalf("tests: %#v", res.Tests)
	}
	c.do("GET", "/api/variables/runtime", nil, &vars)
	if !reflect.DeepEqual(vars, map[string]string{"token": "tok-1", "userId": "7"}) {
		t.Fatalf("runtime variables: %#v", vars)
	}

	// Later sends, cURL exports and requests without tests see them, over the environment.
	next := map[string]any{"environmentId": "e1", "request": model.Request{Method: "GET", URL: "{{base}}/users/{{userId}}", Auth: model.Auth{Type: "bearer", Token: "{{token}}"}}}
	var raw map[string]any
	c.do("POST", "/api/send", next, &raw)
	if _, has := raw["tests"]; has || raw["body"] != "/users/7 Bearer tok-1" {
		t.Fatalf("second send: %#v", raw)
	}
	var exported struct{ Command string }
	c.do("POST", "/api/curl/export", next, &exported)
	if !strings.Contains(exported.Command, "/users/7") || !strings.Contains(exported.Command, "Bearer tok-1") {
		t.Fatalf("export: %s", exported.Command)
	}

	var hist []model.HistoryEntry
	c.do("GET", "/api/history", nil, &hist)
	if len(hist) != 2 || hist[1].Passed != 2 || hist[1].Failed != 2 || hist[0].Passed != 0 || hist[0].Failed != 0 {
		t.Fatalf("history: %#v", hist)
	}

	// A request that never got a response fails its tests and extracts nothing.
	login["request"] = model.Request{URL: "http://127.0.0.1:1/", Tests: []model.Assertion{{Target: "status", Op: "eq", Value: "200"}}, Extract: []model.Extract{{Var: "token", From: "status"}}}
	c.do("POST", "/api/send", login, &res)
	if res.Error == "" || len(res.Tests) != 2 || res.Tests[0].Passed || res.Tests[0].Message != "no response" {
		t.Fatalf("failed send: %#v", res)
	}
	c.do("GET", "/api/variables/runtime", nil, &vars)
	if vars["token"] != "tok-1" {
		t.Fatalf("runtime variables after failed send: %#v", vars)
	}

	if code := c.do("DELETE", "/api/variables/runtime", nil, nil); code != 204 {
		t.Fatalf("clear: %d", code)
	}
	c.do("POST", "/api/send", next, &res)
	if res.Body != "/users/{{userId}} Bearer from-env" {
		t.Fatalf("after clear: %q", res.Body)
	}

	// Switching workspace forgets them, and nothing was written to disk.
	c.do("POST", "/api/send", map[string]any{"environmentId": "e1", "request": model.Request{URL: target.URL + "/login", Extract: []model.Extract{{Var: "token", From: "body.access_token"}}}}, &res)
	var info workspaceInfo
	c.do("GET", "/api/workspace", nil, &info)
	if got := grep(t, c.dataDir, "tok-1") + grep(t, info.Path, "tok-1"); got != "" {
		t.Fatalf("extracted value on disk: %s", got)
	}
	c.do("POST", "/api/workspace/open", map[string]string{"path": t.TempDir()}, nil)
	clear(vars)
	c.do("GET", "/api/variables/runtime", nil, &vars)
	if len(vars) != 0 {
		t.Fatalf("runtime variables after switching workspace: %#v", vars)
	}
}

// grep returns the files under dir that contain text.
func grep(t *testing.T, dir, text string) string {
	t.Helper()
	found := ""
	filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			if data, _ := os.ReadFile(path); strings.Contains(string(data), text) {
				found += path + " "
			}
		}
		return nil
	})
	return found
}

func TestGRPC(t *testing.T) {
	c := setup(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := grpc.NewServer()
	healthpb.RegisterHealthServer(srv, health.NewServer())
	reflection.Register(srv)
	go srv.Serve(ln)
	defer srv.Stop()

	c.do("PUT", "/api/environments/e1", model.Environment{Name: "dev", Variables: []model.KV{{Key: "addr", Value: ln.Addr().String(), Enabled: true}}}, nil)
	in := map[string]any{
		"environmentId": "e1",
		"request": model.Request{Protocol: "grpc", URL: "grpc://{{addr}}", Method: "grpc.health.v1.Health/Check", Body: model.Body{Mode: "raw", Raw: "{}"},
			Tests: []model.Assertion{{Target: "status", Op: "eq", Value: "0"}, {Target: "body.status", Op: "eq", Value: "SERVING"}}},
	}

	var methods []string
	if code := c.do("POST", "/api/grpc/methods", in, &methods); code != 200 || !slices.Contains(methods, "grpc.health.v1.Health/Check") {
		t.Fatalf("methods: %d %q", code, methods)
	}

	var res model.Response
	if code := c.do("POST", "/api/send", in, &res); code != 200 || !res.GRPC || res.Status != 0 || !strings.Contains(res.Body, "SERVING") || len(res.Tests) != 2 || !res.Tests[0].Passed || !res.Tests[1].Passed {
		t.Fatalf("send: %d %#v", code, res)
	}

	in["request"] = model.Request{Protocol: "grpc", URL: "grpc://127.0.0.1:1"}
	if code := c.do("POST", "/api/grpc/methods", in, nil); code != 502 {
		t.Fatalf("unreachable server: %d", code)
	}
}

func TestCurlParse(t *testing.T) {
	c := setup(t)

	var req model.Request
	if code := c.do("POST", "/api/curl/parse", map[string]string{"command": "curl -X DELETE https://x/1"}, &req); code != 200 || req.Method != "DELETE" {
		t.Fatalf("curl parse: %d %#v", code, req)
	}
	if code := c.do("POST", "/api/curl/parse", map[string]string{"command": "ls"}, nil); code != 400 {
		t.Fatalf("bad curl: %d", code)
	}

}

func TestGuardAndStatic(t *testing.T) {
	c := setup(t)

	res, _ := http.Get(c.base + "/")
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != 200 || !strings.Contains(string(body), "ui") {
		t.Fatalf("static: %d %s", res.StatusCode, body)
	}

	cases := []struct {
		name, host, origin string
		want               int
	}{
		{"same machine", "", "", 200},
		{"vite dev origin", "", "http://localhost:5173", 200},
		{"foreign origin", "", "https://evil.test", 403},
		{"dns rebinding", "evil.test", "", 403},
	}
	for _, tc := range cases {
		req, _ := http.NewRequest("GET", c.base+"/api/history", nil)
		if tc.host != "" {
			req.Host = tc.host
		}
		if tc.origin != "" {
			req.Header.Set("Origin", tc.origin)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != tc.want {
			t.Errorf("%s: got %d, want %d", tc.name, res.StatusCode, tc.want)
		}
	}
}

const healthProto = `syntax = "proto3";
package grpc.health.v1;

message HealthCheckRequest {
  string service = 1;
}

message HealthCheckResponse {
  enum ServingStatus {
    UNKNOWN = 0;
    SERVING = 1;
    NOT_SERVING = 2;
    SERVICE_UNKNOWN = 3;
  }
  ServingStatus status = 1;
}

service Health {
  rpc Check(HealthCheckRequest) returns (HealthCheckResponse);
  rpc Watch(HealthCheckRequest) returns (stream HealthCheckResponse);
}
`

// text returns the status and the plain body of a POST.
func (c client) text(path string, body any) (int, string) {
	c.t.Helper()
	data, _ := json.Marshal(body)
	res, err := http.Post(c.base+path, "application/json", bytes.NewReader(data))
	if err != nil {
		c.t.Fatal(err)
	}
	defer res.Body.Close()
	out, _ := io.ReadAll(res.Body)
	return res.StatusCode, string(out)
}

// The server has no reflection, so everything here depends on the proto files.
func TestGRPCWithProtoFiles(t *testing.T) {
	c := setup(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := grpc.NewServer()
	healthpb.RegisterHealthServer(srv, health.NewServer())
	go srv.Serve(ln)
	defer srv.Stop()

	var ws struct{ Path string }
	c.do("GET", "/api/workspace", nil, &ws)
	checkout := t.TempDir()
	os.MkdirAll(filepath.Join(checkout, "grpc", "health", "v1"), 0o755)
	os.WriteFile(filepath.Join(checkout, "grpc", "health", "v1", "health.proto"), []byte(healthProto), 0o644)
	os.MkdirAll(filepath.Join(ws.Path, "protos"), 0o755)
	os.WriteFile(filepath.Join(ws.Path, "protos", "health.proto"), []byte(healthProto), 0o644)

	// The collection carries the config; an empty one is left out.
	proto := map[string]any{"roots": []string{"{{protoRoot}}"}, "files": []string{"grpc/health/v1/health.proto"}}
	c.do("PUT", "/api/collections/c1", map[string]any{"name": "Health", "items": []any{}, "proto": proto}, nil)
	c.do("PUT", "/api/collections/c2", map[string]any{"name": "Other", "items": []any{}, "proto": map[string]any{"roots": []string{}}}, nil)
	var cols []map[string]any
	c.do("GET", "/api/collections", nil, &cols)
	if _, has := cols[1]["proto"]; has || len(cols) != 2 || !reflect.DeepEqual(cols[0]["proto"], map[string]any{"roots": []any{"{{protoRoot}}"}, "files": []any{"grpc/health/v1/health.proto"}}) {
		t.Fatalf("collections: %#v", cols)
	}
	if file, _ := os.ReadFile(filepath.Join(ws.Path, "collections", "Health", "_collection.yaml")); !strings.Contains(string(file), "proto:\n  roots:\n    - '{{protoRoot}}'\n  files:\n    - grpc/health/v1/health.proto\n") {
		t.Fatalf("_collection.yaml:\n%s", file)
	}

	c.do("PUT", "/api/environments/e1", model.Environment{Name: "dev", Variables: []model.KV{
		{Key: "addr", Value: ln.Addr().String(), Enabled: true},
		{Key: "protoRoot", Value: checkout, Enabled: true},
	}}, nil)
	req := model.Request{Protocol: "grpc", URL: "grpc://{{addr}}", Method: "grpc.health.v1.Health/Check", Body: model.Body{Mode: "raw", Raw: "{}"},
		Tests: []model.Assertion{{Target: "body.status", Op: "eq", Value: "SERVING"}}}
	in := map[string]any{"environmentId": "e1", "request": req, "proto": proto}
	want := []string{"grpc.health.v1.Health/Check", "grpc.health.v1.Health/Watch"}

	var res model.Response
	if code := c.do("POST", "/api/send", in, &res); code != 200 || res.Error != "" || !res.GRPC || res.Status != 0 || len(res.Tests) != 1 || !res.Tests[0].Passed {
		t.Fatalf("send: %d %#v", code, res)
	}
	var methods []string
	if code := c.do("POST", "/api/grpc/methods", in, &methods); code != 200 || !reflect.DeepEqual(methods, want) {
		t.Fatalf("methods: %d %q", code, methods)
	}
	// Workspace-relative roots work too, and listing needs no server.
	local := map[string]any{"roots": []string{"protos"}}
	down := map[string]any{"request": model.Request{Protocol: "grpc", URL: "grpc://127.0.0.1:1"}, "proto": local}
	if code := c.do("POST", "/api/grpc/methods", down, &methods); code != 200 || !reflect.DeepEqual(methods, want) {
		t.Fatalf("methods of a server that is down: %d %q", code, methods)
	}
	in["proto"] = local
	if code := c.do("POST", "/api/send", in, &res); code != 200 || res.Error != "" || !strings.Contains(res.Body, "SERVING") {
		t.Fatalf("send with a workspace root: %d %#v", code, res)
	}

	// Without the config it is the reflection path, as before.
	for _, none := range []any{nil, map[string]any{}, map[string]any{"roots": []string{}, "files": []string{}}} {
		in["proto"] = none
		if code := c.do("POST", "/api/send", in, &res); code != 200 || !strings.Contains(res.Error, "reflection failed") {
			t.Fatalf("send without proto: %d %#v", code, res)
		}
		if code, _ := c.text("/api/grpc/methods", in); code != 502 {
			t.Fatalf("methods without proto: %d", code)
		}
	}
	delete(in, "proto")
	if code := c.do("POST", "/api/send", in, &res); code != 200 || !strings.Contains(res.Error, "reflection failed") {
		t.Fatalf("send without proto: %d %#v", code, res)
	}

	// A config that does not load is reported in the response of a send and
	// as a 400 when listing methods.
	bad := map[string]any{"roots": []string{"nope"}}
	in["proto"] = bad
	missing := `proto root "nope" does not exist: ` + filepath.Join(ws.Path, "nope")
	if code := c.do("POST", "/api/send", in, &res); code != 200 || res.Error != missing {
		t.Fatalf("send with a bad config: %d %#v", code, res)
	}
	if code, body := c.text("/api/grpc/methods", in); code != 400 || strings.TrimSpace(body) != missing {
		t.Fatalf("methods with a bad config: %d %s", code, body)
	}

	// Check.
	var raw map[string]any
	if code := c.do("POST", "/api/proto/check", map[string]any{"proto": proto, "environmentId": "e1"}, &raw); code != 200 ||
		!reflect.DeepEqual(raw, map[string]any{"files": 1.0, "services": []any{"grpc.health.v1.Health"}, "methods": []any{want[0], want[1]}}) {
		t.Fatalf("check: %d %#v", code, raw)
	}
	if code, body := c.text("/api/proto/check", map[string]any{"proto": bad}); code != 400 || strings.TrimSpace(body) != missing {
		t.Fatalf("check of a bad config: %d %s", code, body)
	}
	// Without the environment the variable is unknown.
	if code, body := c.text("/api/proto/check", map[string]any{"proto": proto}); code != 400 || !strings.Contains(body, "variable protoRoot is not set") {
		t.Fatalf("check without environment: %d %s", code, body)
	}
	os.WriteFile(filepath.Join(ws.Path, "protos", "health.proto"), []byte(healthProto+"\nmessage Broken {\n  Nope nope = 1;\n}\n"), 0o644)
	if code, body := c.text("/api/proto/check", map[string]any{"proto": local}); code != 400 || !strings.Contains(body, filepath.Join(ws.Path, "protos", "health.proto")+":24:3: field grpc.health.v1.Broken.nope: unknown type Nope") {
		t.Fatalf("check of a file that does not compile: %d %s", code, body)
	}
	if code, body := c.text("/api/proto/check", map[string]any{"proto": map[string]any{}}); code != 400 || strings.TrimSpace(body) == "" {
		t.Fatalf("check of an empty config: %d %s", code, body)
	}
}
