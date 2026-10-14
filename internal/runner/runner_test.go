package runner

import (
	"bytes"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	"roundtrip/internal/model"
	"roundtrip/internal/store"
)

// api is the server the workspace's requests talk to.
func api(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"access_token": "tok-1", "user": {"id": 7}}`))
		case "/me":
			if r.Header.Get("Authorization") != "Bearer tok-1" {
				http.Error(w, "who are you", http.StatusUnauthorized)
				return
			}
			w.Write([]byte(`{"id": 7, "name": "Ada"}`))
		case "/graphql":
			body, _ := io.ReadAll(r.Body)
			var in struct{ Query string }
			json.Unmarshal(body, &in)
			w.Write([]byte(`{"data": {"echo": ` + string(mustJSON(in.Query)) + `}}`))
		case "/echo":
			w.Write([]byte(r.URL.RawQuery))
		case "/broken":
			http.Error(w, "boom", http.StatusInternalServerError)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func mustJSON(v any) []byte {
	out, _ := json.Marshal(v)
	return out
}

func get(url string, tests ...model.Assertion) *model.Request {
	return &model.Request{Method: "GET", URL: url, Tests: tests}
}

func is(target, op, value string) model.Assertion {
	return model.Assertion{Target: target, Op: op, Value: value}
}

// workspace writes collections the way the app does and returns the directory.
func workspace(t *testing.T, cols ...*model.Collection) string {
	t.Helper()
	dir := t.TempDir()
	st, err := store.Open(dir, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cols {
		if c.ID == "" {
			c.ID = model.NewID()
		}
		if err := st.SaveCollection(c); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

var elapsed = regexp.MustCompile(`\d+ms`)

// roundtrip runs the command and returns its exit status and output, with
// durations replaced by "Nms" and trailing spaces trimmed.
func roundtrip(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := Main(args, &stdout, &stderr, false)
	lines := strings.Split(elapsed.ReplaceAllString(stdout.String(), "Nms"), "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " ")
	}
	return code, strings.Join(lines, "\n"), stderr.String()
}

func users(base string) *model.Collection {
	return &model.Collection{Name: "Users", Items: []*model.Item{
		{ID: "f1", Name: "Admin", Items: []*model.Item{
			{ID: "r3", Name: "Echo", Request: get(base+"/echo?who={{who}}", is("body", "eq", "who=ci"))},
		}},
		{ID: "r1", Name: "1 Login", Request: &model.Request{
			Method:  "POST",
			URL:     "{{base}}/login",
			Body:    model.Body{Mode: "raw", ContentType: "application/json", Raw: "{\n  \"user\": \"ada\"\n}"},
			Tests:   []model.Assertion{is("status", "eq", "200"), is("header.content-type", "contains", "json")},
			Extract: []model.Extract{{Var: "token", From: "body.access_token"}, {Var: "id", From: "body.user.id"}},
		}},
		{ID: "r2", Name: "2 Me", Request: &model.Request{
			Method: "GET",
			URL:    "{{base}}/me",
			Auth:   model.Auth{Type: "bearer", Token: "{{token}}"},
			Tests:  []model.Assertion{is("body.id", "eq", "{{id}}"), is("body.name", "eq", "Ada")},
		}},
	}}
}

func TestRunPassesAndChainsExtractedVariables(t *testing.T) {
	base := api(t)
	dir := workspace(t, users(base))
	code, out, errOut := roundtrip(t, "-workspace", dir, "-var", "base="+base, "-var", "who=ci")
	want := `PASS  GET   Users / Admin / Echo  200 OK  Nms
PASS  POST  Users / 1 Login       200 OK  Nms
PASS  GET   Users / 2 Me          200 OK  Nms

3 requests: 3 passed, 0 failed in Nms
`
	if code != 0 || out != want || errOut != "" {
		t.Fatalf("exit %d\n%s\nstderr: %s", code, out, errOut)
	}
}

func TestRunReportsFailures(t *testing.T) {
	base := api(t)
	dir := workspace(t, &model.Collection{Name: "Mixed", Items: []*model.Item{
		{ID: "1", Name: "a wrong name", Request: get(base+"/login", is("status", "eq", "200"), is("body.user.name", "eq", "Ada"), is("body.user.id", "gt", "10"))},
		{ID: "2", Name: "b no tests, 404", Request: get(base + "/nope")},
		{ID: "3", Name: "c no tests, 200", Request: get(base + "/echo")},
		{ID: "4", Name: "d unreachable", Request: get("http://127.0.0.1:1/x", is("status", "eq", "200"))},
		{ID: "5", Name: "e 500 is wanted", Request: get(base+"/broken", is("status", "eq", "500"))},
		{ID: "6", Name: "f missing extract", Request: &model.Request{URL: base + "/login", Extract: []model.Extract{{Var: "r", From: "body.refresh_token"}}}},
	}})
	code, out, _ := roundtrip(t, "-workspace", dir)
	// The transport error text differs per platform; keep the line's start.
	out = regexp.MustCompile(`(d unreachable) .*`).ReplaceAllString(out, "$1  <error>")
	want := `FAIL  GET  Mixed / a wrong name       200 OK  Nms
      body.user.name eq Ada: not found
      body.user.id gt 10: got "7"
FAIL  GET  Mixed / b no tests, 404    404 Not Found  Nms
      no tests, so the status must be below 400
PASS  GET  Mixed / c no tests, 200    200 OK  Nms
FAIL  GET  Mixed / d unreachable  <error>
PASS  GET  Mixed / e 500 is wanted    500 Internal Server Error  Nms
FAIL  GET  Mixed / f missing extract  200 OK  Nms
      extract r from body.refresh_token: not found

6 requests: 2 passed, 4 failed in Nms
`
	if code != 1 || out != want {
		t.Fatalf("exit %d\n%s", code, out)
	}

	code, out, _ = roundtrip(t, "-workspace", dir, "-bail")
	if code != 1 || strings.Count(out, "\n") != 5 || !strings.Contains(out, "1 request: 0 passed, 1 failed, 5 not run in Nms") {
		t.Fatalf("-bail: exit %d\n%s", code, out)
	}
	// -bail does not stop a run that passes.
	if code, out, _ = roundtrip(t, "-workspace", dir, "-bail", "Mixed/c no tests, 200", "Mixed/e 500 is wanted"); code != 0 || !strings.Contains(out, "2 requests: 2 passed, 0 failed in") {
		t.Fatalf("-bail on success: exit %d\n%s", code, out)
	}
}

func TestPathSelection(t *testing.T) {
	base := api(t)
	echo := func(id, name string) *model.Item {
		return &model.Item{ID: id, Name: name, Request: get(base + "/echo")}
	}
	dir := workspace(t,
		&model.Collection{Name: "Users API", Items: []*model.Item{
			{ID: "f1", Name: "Admin", Items: []*model.Item{
				{ID: "f2", Name: "Deep", Items: []*model.Item{echo("r1", "Purge")}},
				echo("r2", "Delete user"),
			}},
			{ID: "f3", Name: "Empty"},
			echo("r3", "List users"),
			echo("r4", "v1/v2 compare"),
		}},
		&model.Collection{Name: "Billing", Items: []*model.Item{echo("r5", "Invoices")}},
	)
	names := func(out string) string {
		var got []string
		for _, l := range strings.Split(out, "\n") {
			if strings.HasPrefix(l, "PASS") {
				got = append(got, strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(l, "PASS  GET  "), "200 OK  Nms")))
			}
		}
		return strings.Join(got, "|")
	}
	cases := []struct {
		paths []string
		want  string
	}{
		{nil, "Billing / Invoices|Users API / Admin / Deep / Purge|Users API / Admin / Delete user|Users API / List users|Users API / v1/v2 compare"},
		{[]string{"Billing"}, "Billing / Invoices"},
		{[]string{"Users API/Admin"}, "Users API / Admin / Deep / Purge|Users API / Admin / Delete user"},
		{[]string{"Users API/Admin/"}, "Users API / Admin / Deep / Purge|Users API / Admin / Delete user"},
		{[]string{"Users API/Admin/Deep/Purge"}, "Users API / Admin / Deep / Purge"},
		{[]string{"Users API/List users"}, "Users API / List users"},
		{[]string{"Users API/v1/v2 compare"}, "Users API / v1/v2 compare"},
		{[]string{"Users API/List users", "Billing"}, "Users API / List users|Billing / Invoices"},
	}
	for _, tc := range cases {
		code, out, errOut := roundtrip(t, append([]string{"-workspace", dir}, tc.paths...)...)
		if code != 0 || names(out) != tc.want {
			t.Errorf("%q: exit %d, ran %q, want %q\n%s", tc.paths, code, names(out), tc.want, errOut)
		}
	}

	unknown := map[string]string{
		"Nope":                  `no "Nope" in the workspace, which has: "Billing", "Users API"`,
		"Users API/Admn":        `no "Admn" in "Users API", which has: "Admin", "Empty", "List users", "v1/v2 compare"`,
		"Users API/Admin/Nope":  `no "Nope" in "Users API/Admin", which has: "Deep", "Delete user"`,
		"Users API/Empty/x":     `no "x" in "Users API/Empty", which is empty`,
		"Users API/List users/": "",
		"users api":             `no "users api" in the workspace`,
		"Users API/Empty":       "no requests to run",
		"Billing/Invoices/more": `no "Invoices/more" in "Billing", which has: "Invoices"`,
	}
	for path, want := range unknown {
		code, out, errOut := roundtrip(t, "-workspace", dir, path)
		if want == "" {
			if code != 0 {
				t.Errorf("%q: exit %d: %s", path, code, errOut)
			}
			continue
		}
		if code != 2 || out != "" || !strings.Contains(errOut, want) {
			t.Errorf("%q: exit %d, stdout %q, stderr %q, want %q", path, code, out, errOut, want)
		}
	}
}

func TestVariableSources(t *testing.T) {
	base := api(t)
	dir := workspace(t, &model.Collection{Name: "C", Items: []*model.Item{
		{ID: "1", Name: "Echo", Request: get(base+"/echo?{{a}}-{{b}}-{{c}}-{{port}}-{{off}}-{{unset}}", is("body", "eq", "{{want}}"))},
	}})
	data := t.TempDir()
	abs, _ := filepath.Abs(dir)
	st, err := store.Open(dir, store.LocalDir(data, abs))
	if err != nil {
		t.Fatal(err)
	}
	st.SaveEnvironment(&model.Environment{ID: "e1", Name: "Staging", Variables: []model.KV{
		{Key: "a", Value: "env", Enabled: true},
		{Key: "b", Value: "env", Enabled: true, Secret: true},
		{Key: "c", Value: "env", Enabled: true},
		{Key: "off", Value: "env"},
	}})
	file := filepath.Join(t.TempDir(), "ci.yaml")
	os.WriteFile(file, []byte("# CI values\nb: file\nc: file\nport: 8080\n"), 0o644)
	jsonFile := filepath.Join(t.TempDir(), "ci.json")
	os.WriteFile(jsonFile, []byte(`{"b": "json", "port": "1"}`), 0o644)
	before := snapshot(t, data)

	cases := []struct {
		args []string
		want string
	}{
		{nil, "{{a}}-{{b}}-{{c}}-{{port}}-{{off}}-{{unset}}"},
		{[]string{"-data", data, "-env", "Staging"}, "env-env-env-{{port}}-{{off}}-{{unset}}"},
		{[]string{"-env-file", file}, "{{a}}-file-file-8080-{{off}}-{{unset}}"},
		{[]string{"-env-file", jsonFile}, "{{a}}-json-{{c}}-1-{{off}}-{{unset}}"},
		{[]string{"-data", data, "-env", "Staging", "-env-file", file}, "env-file-file-8080-{{off}}-{{unset}}"},
		{[]string{"-data", data, "-env", "Staging", "-env-file", file, "-var", "c=flag", "-var", "unset=a=b"}, "env-file-flag-8080-{{off}}-a=b"},
	}
	for _, tc := range cases {
		args := append([]string{"-workspace", dir, "-var", "want=" + tc.want}, tc.args...)
		if code, out, errOut := roundtrip(t, args...); code != 0 {
			t.Errorf("%q: exit %d\n%s%s", tc.args, code, out, errOut)
		}
	}
	if after := snapshot(t, data); after != before {
		t.Fatalf("the run changed the per-machine data:\n%s\nwas:\n%s", after, before)
	}

	// Without -env the data directory is not touched at all.
	missing := filepath.Join(t.TempDir(), "never")
	if code, _, errOut := roundtrip(t, "-workspace", dir, "-data", missing, "-var", "want={{a}}-{{b}}-{{c}}-{{port}}-{{off}}-{{unset}}"); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	if _, err := os.Stat(missing); err == nil {
		t.Fatal("data directory was created")
	}

	bad := filepath.Join(t.TempDir(), "bad.yaml")
	os.WriteFile(bad, []byte("name: x\nvariables:\n  - key: a\n"), 0o644)
	setup := map[string][]string{
		`no environment "Prod"; this machine has: "Staging"`:  {"-data", data, "-env", "Prod"},
		`this machine has no environments for this workspace`: {"-data", missing, "-env", "Staging"},
		"want a flat map of name: value":                      {"-env-file", bad},
		"nope.yaml":                                           {"-env-file", filepath.Join(dir, "nope.yaml")},
		"want name=value":                                     {"-var", "novalue"},
		"flag provided but not defined":                       {"-nonsense"},
	}
	for want, args := range setup {
		code, out, errOut := roundtrip(t, append([]string{"-workspace", dir}, args...)...)
		if code != 2 || out != "" || !strings.Contains(errOut, want) {
			t.Errorf("%q: exit %d, stdout %q, stderr %q", args, code, out, errOut)
		}
	}
	if _, err := os.Stat(missing); err == nil {
		t.Fatal("data directory was created")
	}
}

// snapshot lists the files under dir with their contents.
func snapshot(t *testing.T, dir string) string {
	t.Helper()
	var b strings.Builder
	filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err == nil {
			data, _ := os.ReadFile(path)
			b.WriteString(path + "\n" + string(data) + "\n")
		}
		return nil
	})
	return b.String()
}

func TestSetupErrors(t *testing.T) {
	empty := t.TempDir()
	if code, _, errOut := roundtrip(t, "-workspace", empty); code != 2 || !strings.Contains(errOut, "no requests to run in") {
		t.Fatalf("empty workspace: exit %d %s", code, errOut)
	}
	if entries, _ := os.ReadDir(empty); len(entries) != 0 {
		t.Fatalf("the run created %v", entries)
	}
	if code, _, errOut := roundtrip(t, "-workspace", filepath.Join(empty, "missing")); code != 2 || errOut == "" {
		t.Fatalf("missing workspace: exit %d %s", code, errOut)
	}
	if code, out, errOut := roundtrip(t, "-h"); code != 0 || out != "" || !strings.Contains(errOut, "usage: roundtrip run") || !strings.Contains(errOut, "-env-file") {
		t.Fatalf("help: exit %d %s", code, errOut)
	}

	broken := t.TempDir()
	os.MkdirAll(filepath.Join(broken, "collections", "C"), 0o755)
	os.WriteFile(filepath.Join(broken, "collections", "C", "bad.yaml"), []byte("method: [GET\n"), 0o644)
	if code, _, errOut := roundtrip(t, "-workspace", broken); code != 2 || !strings.Contains(errOut, "bad.yaml") {
		t.Fatalf("broken file: exit %d %s", code, errOut)
	}
}

func TestRunsHandWrittenGraphQLAndGRPC(t *testing.T) {
	base := api(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := grpc.NewServer()
	hs := health.NewServer()
	hs.SetServingStatus("db", healthpb.HealthCheckResponse_NOT_SERVING)
	healthpb.RegisterHealthServer(srv, hs)
	reflection.Register(srv)
	go srv.Serve(ln)
	t.Cleanup(srv.Stop)

	dir := t.TempDir()
	files := map[string]string{
		"1 query.yaml": `method: POST
url: '{{base}}/graphql'
body:
  mode: graphql
  raw: |
    query {
      echo
    }
tests:
  - {target: status, op: eq, value: 200}
  - {target: body.data.echo, op: contains, value: echo}
extract:
  - {var: service, from: header.x-nothing}
`,
		"2 health.yaml": `protocol: grpc
method: grpc.health.v1.Health/Check
url: grpc://{{addr}}
body:
  mode: raw
  raw: |
    {"service": "db"}
tests:
  - {target: status, op: eq, value: 0}
  - {target: body.status, op: eq, value: SERVING}
`,
		"3 unknown service.yaml": `protocol: grpc
method: grpc.health.v1.Health/Check
url: grpc://{{addr}}
body:
  mode: raw
  raw: '{"service": "nope"}'
`,
		"4 no such method.yaml": "protocol: grpc\nmethod: grpc.health.v1.Health/Nope\nurl: grpc://{{addr}}\n",
	}
	os.MkdirAll(filepath.Join(dir, "collections", "Mixed"), 0o755)
	for name, content := range files {
		os.WriteFile(filepath.Join(dir, "collections", "Mixed", name), []byte(content), 0o644)
	}
	code, out, errOut := roundtrip(t, "-workspace", dir, "-var", "base="+base, "-var", "addr="+ln.Addr().String())
	want := `FAIL  GQL   Mixed / 1 query            200 OK  Nms
      extract service from header.x-nothing: not found
FAIL  GRPC  Mixed / 2 health           0 OK  Nms
      body.status eq SERVING: got "NOT_SERVING"
FAIL  GRPC  Mixed / 3 unknown service  5 NotFound  Nms
      no tests, so the call must return OK
FAIL  GRPC  Mixed / 4 no such method   service grpc.health.v1.Health has no method Nope

4 requests: 0 passed, 4 failed in Nms
`
	if code != 1 || out != want {
		t.Fatalf("exit %d\n%s\n%s", code, out, errOut)
	}
}

func TestColorAndTimeout(t *testing.T) {
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-make(chan struct{}):
		}
	}))
	t.Cleanup(slow.Close)
	base := api(t)
	dir := workspace(t, &model.Collection{Name: "C", Items: []*model.Item{
		{ID: "1", Name: "a ok", Request: get(base + "/echo")},
		{ID: "2", Name: "b slow", Request: get(slow.URL)},
	}})
	var out bytes.Buffer
	if code := Main([]string{"-workspace", dir, "-timeout", "50ms"}, &out, io.Discard, true); code != 1 {
		t.Fatalf("exit %d\n%s", code, out.String())
	}
	if got := out.String(); !strings.Contains(got, "\x1b[32mPASS\x1b[0m  GET  C / a ok") || !strings.Contains(got, "\x1b[31mFAIL\x1b[0m  GET  C / b slow") || !strings.Contains(got, "Timeout") {
		t.Fatalf("got %q", got)
	}

	t.Setenv("NO_COLOR", "1")
	out.Reset()
	Main([]string{"-workspace", dir, "-timeout", "50ms"}, &out, io.Discard, true)
	if strings.Contains(out.String(), "\x1b") {
		t.Fatalf("NO_COLOR ignored: %q", out.String())
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
}
`

// The server has no reflection: a gRPC request only works through the proto
// config of its collection.
func TestRunUsesTheProtoConfigOfTheCollection(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := grpc.NewServer()
	healthpb.RegisterHealthServer(srv, health.NewServer())
	go srv.Serve(ln)
	t.Cleanup(srv.Stop)

	check := func(name string) *model.Item {
		return &model.Item{ID: model.NewID(), Name: name, Request: &model.Request{
			Protocol: "grpc", URL: "grpc://{{addr}}", Method: "grpc.health.v1.Health/Check",
			Tests: []model.Assertion{is("body.status", "eq", "SERVING")},
		}}
	}
	dir := workspace(t,
		&model.Collection{Name: "A protos", Proto: &model.ProtoConfig{Roots: []string{"{{protoDir}}"}, Files: []string{"grpc/health.proto"}},
			Items: []*model.Item{{ID: "f1", Name: "Folder", Items: []*model.Item{check("Check")}}}},
		&model.Collection{Name: "B reflection", Items: []*model.Item{check("Check")}},
		&model.Collection{Name: "C broken", Proto: &model.ProtoConfig{Roots: []string{"nope"}}, Items: []*model.Item{check("Check")}},
	)
	os.MkdirAll(filepath.Join(dir, "protos", "grpc"), 0o755)
	os.WriteFile(filepath.Join(dir, "protos", "grpc", "health.proto"), []byte(healthProto), 0o644)
	before := snapshot(t, dir)

	code, out, errOut := roundtrip(t, "-workspace", dir, "-var", "addr="+ln.Addr().String(), "-var", "protoDir=protos")
	out = strings.ReplaceAll(out, dir, "<workspace>")
	want := `PASS  GRPC  A protos / Folder / Check  0 OK  Nms
FAIL  GRPC  B reflection / Check       server reflection failed (is it enabled on the server?): rpc error: code = Unimplemented desc = unknown service grpc.reflection.v1alpha.ServerReflection
FAIL  GRPC  C broken / Check           proto root "nope" does not exist: <workspace>` + string(filepath.Separator) + `nope

3 requests: 1 passed, 2 failed in Nms
`
	if code != 1 || out != want {
		t.Fatalf("exit %d\n%s\n%s", code, out, errOut)
	}
	if snapshot(t, dir) != before {
		t.Fatal("the run wrote to the workspace")
	}
	// A path selects the request together with its collection's config.
	if code, out, _ := roundtrip(t, "-workspace", dir, "-var", "addr="+ln.Addr().String(), "-var", "protoDir=protos", "A protos/Folder/Check"); code != 0 || !strings.Contains(out, "1 request: 1 passed") {
		t.Fatalf("selected: exit %d\n%s", code, out)
	}
}
