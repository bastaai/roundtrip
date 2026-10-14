package sender

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"roundtrip/internal/model"
)

func kv(k, v string) model.KV { return model.KV{Key: k, Value: v, Enabled: true} }

func TestResolve(t *testing.T) {
	vars := map[string]string{"host": "api.test", "id": "42"}
	cases := map[string]string{
		"https://{{host}}/users/{{ id }}": "https://api.test/users/42",
		"{{missing}} stays":               "{{missing}} stays",
		"no vars":                         "no vars",
		"{{host}}{{host}}":                "api.testapi.test",
	}
	for in, want := range cases {
		if got := Resolve(in, vars); got != want {
			t.Errorf("Resolve(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestVarsSkipsDisabled(t *testing.T) {
	vars := Vars(&model.Environment{Variables: []model.KV{kv("a", "1"), {Key: "b", Value: "2"}}})
	if vars["a"] != "1" || len(vars) != 1 {
		t.Fatalf("got %v", vars)
	}
	if len(Vars(nil)) != 0 {
		t.Fatal("nil env should give no vars")
	}
}

func TestResolveRequestDoesNotMutateInput(t *testing.T) {
	in := model.Request{URL: "{{u}}", Headers: []model.KV{kv("X", "{{u}}")}}
	out := ResolveRequest(in, map[string]string{"u": "v"})
	if out.URL != "v" || out.Headers[0].Value != "v" {
		t.Fatalf("not resolved: %#v", out)
	}
	if in.Headers[0].Value != "{{u}}" {
		t.Fatal("input mutated")
	}
}

func TestApplyAuth(t *testing.T) {
	header := func(r model.Request, name string) string {
		for _, h := range r.Headers {
			if h.Key == name {
				return h.Value
			}
		}
		return ""
	}

	r := ApplyAuth(model.Request{Auth: model.Auth{Type: "bearer", Token: "tok"}})
	if got := header(r, "Authorization"); got != "Bearer tok" {
		t.Errorf("bearer: %q", got)
	}

	r = ApplyAuth(model.Request{Auth: model.Auth{Type: "basic", Username: "user", Password: "pass"}})
	if got := header(r, "Authorization"); got != "Basic dXNlcjpwYXNz" {
		t.Errorf("basic: %q", got)
	}

	r = ApplyAuth(model.Request{Auth: model.Auth{Type: "apikey", Key: "X-Key", Value: "k", In: "header"}})
	if got := header(r, "X-Key"); got != "k" {
		t.Errorf("apikey header: %q", got)
	}

	r = ApplyAuth(model.Request{URL: "http://a/b?x=1#frag", Auth: model.Auth{Type: "apikey", Key: "api key", Value: "a&b", In: "query"}})
	if r.URL != "http://a/b?x=1&api+key=a%26b#frag" {
		t.Errorf("apikey query: %q", r.URL)
	}

	r = ApplyAuth(model.Request{
		Headers: []model.KV{kv("authorization", "mine")},
		Auth:    model.Auth{Type: "bearer", Token: "tok"},
	})
	if len(r.Headers) != 1 || r.Headers[0].Value != "mine" {
		t.Errorf("explicit header should win: %#v", r.Headers)
	}
}

func TestBuildBodies(t *testing.T) {
	read := func(r model.Request) (*http.Request, string) {
		t.Helper()
		req, err := Build(context.Background(), r)
		if err != nil {
			t.Fatal(err)
		}
		if req.Body == nil {
			return req, ""
		}
		b, _ := io.ReadAll(req.Body)
		return req, string(b)
	}

	req, body := read(model.Request{URL: "example.com/a?q=hello world", Method: "post", Body: model.Body{Mode: "raw", Raw: `{"a":1}`, ContentType: "application/json"}})
	if req.Method != "POST" || req.URL.String() != "http://example.com/a?q=hello%20world" {
		t.Errorf("method/url: %s %s", req.Method, req.URL)
	}
	if body != `{"a":1}` || req.Header.Get("Content-Type") != "application/json" {
		t.Errorf("raw: %q %q", body, req.Header.Get("Content-Type"))
	}

	req, body = read(model.Request{URL: "http://x", Body: model.Body{Mode: "urlencoded", Form: []model.KV{kv("b", "2 3"), kv("a", "1"), {Key: "off", Value: "x"}}}})
	if body != "b=2+3&a=1" || req.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
		t.Errorf("urlencoded: %q %q", body, req.Header.Get("Content-Type"))
	}

	req, body = read(model.Request{URL: "http://x", Body: model.Body{Mode: "form", Form: []model.KV{kv("name", "value")}}})
	if !strings.HasPrefix(req.Header.Get("Content-Type"), "multipart/form-data; boundary=") || !strings.Contains(body, `name="name"`) {
		t.Errorf("form: %q %q", body, req.Header.Get("Content-Type"))
	}

	req, _ = read(model.Request{URL: "http://x", Headers: []model.KV{kv("Content-Type", "text/csv"), kv("Host", "other"), {Key: "Off", Value: "1"}}, Body: model.Body{Mode: "raw", ContentType: "application/json"}})
	if req.Header.Get("Content-Type") != "text/csv" || req.Host != "other" || req.Header.Get("Off") != "" {
		t.Errorf("headers: %#v host=%s", req.Header, req.Host)
	}

	req, body = read(model.Request{URL: "http://x", Method: "POST", Body: model.Body{Mode: "graphql", Raw: "query($id: ID!) { user(id: $id) { name } }", Variables: `{"id": "7"}`}})
	if body != `{"query":"query($id: ID!) { user(id: $id) { name } }","variables":{"id":"7"}}` || req.Header.Get("Content-Type") != "application/json" {
		t.Errorf("graphql: %q %q", body, req.Header.Get("Content-Type"))
	}
	_, body = read(model.Request{URL: "http://x", Body: model.Body{Mode: "graphql", Raw: "{ me }"}})
	if body != `{"query":"{ me }"}` {
		t.Errorf("graphql without variables: %q", body)
	}
	if _, err := Build(context.Background(), model.Request{URL: "http://x", Body: model.Body{Mode: "graphql", Variables: "{oops"}}); err == nil {
		t.Error("invalid graphql variables should fail")
	}

	if _, err := Build(context.Background(), model.Request{URL: "  "}); err == nil {
		t.Error("empty URL should fail")
	}
}

func TestSend(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/binary":
			w.Write([]byte{0xff, 0xfe, 0x00})
		default:
			body, _ := io.ReadAll(r.Body)
			w.Header().Set("X-Auth", r.Header.Get("Authorization"))
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(r.Method + " " + r.URL.RequestURI() + " " + string(body)))
		}
	}))
	defer srv.Close()
	vars := map[string]string{"base": srv.URL, "token": "secret"}

	res := Send(context.Background(), srv.Client(), model.Request{
		Method: "POST",
		URL:    "{{base}}/things?x=1",
		Auth:   model.Auth{Type: "bearer", Token: "{{token}}"},
		Body:   model.Body{Mode: "raw", Raw: "hi"},
	}, vars)
	if res.Error != "" {
		t.Fatal(res.Error)
	}
	if res.Status != 201 || res.StatusText != "Created" || res.Body != "POST /things?x=1 hi" || res.Size != len(res.Body) {
		t.Errorf("unexpected response: %#v", res)
	}
	found := false
	for _, h := range res.Headers {
		if h.Key == "X-Auth" && h.Value == "Bearer secret" {
			found = true
		}
	}
	if !found {
		t.Errorf("auth header not sent: %#v", res.Headers)
	}

	res = Send(context.Background(), srv.Client(), model.Request{URL: "{{base}}/binary"}, vars)
	if !res.Base64 || res.Body != "//4A" || res.Size != 3 {
		t.Errorf("binary: %#v", res)
	}

	res = Send(context.Background(), srv.Client(), model.Request{URL: "{{nope}}/x"}, vars)
	if res.Error == "" {
		t.Error("unresolved host should produce an error")
	}
}
