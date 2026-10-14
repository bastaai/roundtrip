package curl

import (
	"reflect"
	"testing"

	"roundtrip/internal/model"
)

func kv(k, v string) model.KV { return model.KV{Key: k, Value: v, Enabled: true} }

func TestTokenize(t *testing.T) {
	got, err := tokenize("curl 'a b' \"c \\\"d\\\"\" e\\ f \\\n  $'g\\'h\\n' ''")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"curl", "a b", `c "d"`, "e f", "g'h\n", ""}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
	if _, err := tokenize("curl 'oops"); err == nil {
		t.Fatal("unterminated quote should fail")
	}
}

func TestParse(t *testing.T) {
	t.Run("json post with headers", func(t *testing.T) {
		r, err := Parse(`curl -sSL -X POST 'https://api.test/users?x=1' \
  -H 'Content-Type: application/json; charset=utf-8' \
  -H "Authorization: Bearer abc" \
  --compressed \
  --data-raw '{"name":"it''s"}'`)
		if err != nil {
			t.Fatal(err)
		}
		if r.Method != "POST" || r.URL != "https://api.test/users?x=1" {
			t.Errorf("method/url: %s %s", r.Method, r.URL)
		}
		wantHeaders := []model.KV{kv("Content-Type", "application/json; charset=utf-8"), kv("Authorization", "Bearer abc")}
		if !reflect.DeepEqual(r.Headers, wantHeaders) {
			t.Errorf("headers: %#v", r.Headers)
		}
		if r.Body.Mode != "raw" || r.Body.Raw != `{"name":"its"}` || r.Body.ContentType != "application/json" {
			t.Errorf("body: %#v", r.Body)
		}
	})

	t.Run("data without content type is a form and implies POST", func(t *testing.T) {
		r, err := Parse(`curl https://api.test/login -d user=bob -d 'pass=a%20b'`)
		if err != nil {
			t.Fatal(err)
		}
		if r.Method != "POST" || r.Body.Mode != "urlencoded" {
			t.Errorf("got %s %s", r.Method, r.Body.Mode)
		}
		if want := []model.KV{kv("user", "bob"), kv("pass", "a b")}; !reflect.DeepEqual(r.Body.Form, want) {
			t.Errorf("form: %#v", r.Body.Form)
		}
	})

	t.Run("json body without header stays raw", func(t *testing.T) {
		r, _ := Parse(`curl https://api.test -d '{"q":"a=b"}'`)
		if r.Body.Mode != "raw" || r.Body.ContentType != "application/x-www-form-urlencoded" {
			t.Errorf("body: %#v", r.Body)
		}
	})

	t.Run("attached values, basic auth, multipart", func(t *testing.T) {
		r, err := Parse(`curl -XPUT --url=https://api.test/up -u alice:s3:cret -F name=x -F file=@a.txt -o out.txt -A agent`)
		if err != nil {
			t.Fatal(err)
		}
		if r.Method != "PUT" || r.URL != "https://api.test/up" {
			t.Errorf("method/url: %s %s", r.Method, r.URL)
		}
		if r.Auth.Type != "basic" || r.Auth.Username != "alice" || r.Auth.Password != "s3:cret" {
			t.Errorf("auth: %#v", r.Auth)
		}
		if r.Body.Mode != "form" || len(r.Body.Form) != 2 {
			t.Errorf("body: %#v", r.Body)
		}
		if !reflect.DeepEqual(r.Headers, []model.KV{kv("User-Agent", "agent")}) {
			t.Errorf("headers: %#v", r.Headers)
		}
	})

	t.Run("-G moves data to the query string", func(t *testing.T) {
		r, _ := Parse(`curl -G https://api.test/search -d q=go --data-urlencode 'tag=a b'`)
		if r.Method != "GET" || r.URL != "https://api.test/search?q=go&tag=a+b" || r.Body.Mode != "none" {
			t.Errorf("got %s %s %s", r.Method, r.URL, r.Body.Mode)
		}
	})

	t.Run("--json", func(t *testing.T) {
		r, _ := Parse(`curl --json '{"a":1}' https://api.test`)
		if r.Method != "POST" || r.Body.Mode != "raw" || r.Body.ContentType != "application/json" || len(r.Headers) != 2 {
			t.Errorf("got %#v", r)
		}
	})

	t.Run("errors", func(t *testing.T) {
		if _, err := Parse("wget http://x"); err == nil {
			t.Error("non-curl should fail")
		}
		if _, err := Parse("curl -L"); err == nil {
			t.Error("missing URL should fail")
		}
	})
}

func TestExport(t *testing.T) {
	got := Export(model.Request{
		Method:  "post",
		URL:     "https://api.test/a?b=c",
		Headers: []model.KV{kv("X-One", "1"), {Key: "X-Off", Value: "0"}},
		Auth:    model.Auth{Type: "bearer", Token: "tok"},
		Body:    model.Body{Mode: "raw", Raw: `{"say":"it's"}`, ContentType: "application/json"},
	})
	want := `curl -X POST 'https://api.test/a?b=c' \
  -H 'X-One: 1' \
  -H 'Authorization: Bearer tok' \
  -H 'Content-Type: application/json' \
  --data-raw '{"say":"it'\''s"}'`
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestExportGraphQL(t *testing.T) {
	got := Export(model.Request{Method: "POST", URL: "https://x/graphql", Body: model.Body{Mode: "graphql", Raw: "{ me }", Variables: `{"a": 1}`}})
	want := `curl -X POST 'https://x/graphql' \
  -H 'Content-Type: application/json' \
  --data-raw '{"query":"{ me }","variables":{"a":1}}'`
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestExportParseRoundTrip(t *testing.T) {
	bodies := []model.Body{
		{Mode: "none"},
		{Mode: "raw", Raw: "{\n  \"a\": \"it's $HOME\"\n}", ContentType: "application/json"},
		{Mode: "urlencoded", Form: []model.KV{kv("a", "1 & 2"), kv("b", "100%")}},
		{Mode: "form", Form: []model.KV{kv("a", "1"), kv("b", "x y")}},
	}
	for _, body := range bodies {
		in := model.Request{
			Method:  "PATCH",
			URL:     "https://api.test/x?y=1&z=a%20b",
			Headers: []model.KV{kv("Accept", "*/*")},
			Auth:    model.Auth{Type: "none"},
			Body:    body,
		}
		out, err := Parse(Export(in))
		if err != nil {
			t.Fatalf("%s: %v", body.Mode, err)
		}
		// Export makes the raw content type explicit as a header.
		if body.Mode == "raw" {
			in.Headers = append(in.Headers, kv("Content-Type", body.ContentType))
		}
		if !reflect.DeepEqual(in, out) {
			t.Errorf("%s round trip:\n in: %#v\nout: %#v", body.Mode, in, out)
		}
	}
}
