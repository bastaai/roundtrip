// Package sender turns a model.Request into a real HTTP call.
package sender

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"roundtrip/internal/model"
)

const maxBody = 50 << 20

var varRe = regexp.MustCompile(`\{\{\s*([^{}\s]+)\s*\}\}`)

func Vars(env *model.Environment) map[string]string {
	vars := map[string]string{}
	if env == nil {
		return vars
	}
	for _, v := range env.Variables {
		if v.Enabled && v.Key != "" {
			vars[v.Key] = v.Value
		}
	}
	return vars
}

// Resolve replaces {{name}} placeholders. Unknown names are left untouched.
func Resolve(s string, vars map[string]string) string {
	return varRe.ReplaceAllStringFunc(s, func(m string) string {
		if v, ok := vars[varRe.FindStringSubmatch(m)[1]]; ok {
			return v
		}
		return m
	})
}

// Unresolved lists the names of the {{placeholders}} left in s.
func Unresolved(s string) []string {
	var names []string
	for _, m := range varRe.FindAllStringSubmatch(s, -1) {
		names = append(names, m[1])
	}
	return names
}

func ResolveRequest(r model.Request, vars map[string]string) model.Request {
	res := func(s string) string { return Resolve(s, vars) }
	kvs := func(in []model.KV) []model.KV {
		out := make([]model.KV, len(in))
		for i, kv := range in {
			out[i] = model.KV{Key: res(kv.Key), Value: res(kv.Value), Enabled: kv.Enabled}
		}
		return out
	}
	r.URL = res(r.URL)
	r.Headers = kvs(r.Headers)
	r.Auth.Token = res(r.Auth.Token)
	r.Auth.Username = res(r.Auth.Username)
	r.Auth.Password = res(r.Auth.Password)
	r.Auth.Key = res(r.Auth.Key)
	r.Auth.Value = res(r.Auth.Value)
	r.Body.Raw = res(r.Body.Raw)
	r.Body.Variables = res(r.Body.Variables)
	r.Body.Form = kvs(r.Body.Form)
	return r
}

func HasHeader(headers []model.KV, name string) bool {
	for _, h := range headers {
		if h.Enabled && strings.EqualFold(h.Key, name) {
			return true
		}
	}
	return false
}

// ApplyAuth folds the auth settings into headers / query string.
// An explicit header with the same name wins over the auth helper.
func ApplyAuth(r model.Request) model.Request {
	a := r.Auth
	r.Auth = model.Auth{Type: "none"}
	header := func(k, v string) {
		if k == "" || HasHeader(r.Headers, k) {
			return
		}
		r.Headers = append(append([]model.KV{}, r.Headers...), model.KV{Key: k, Value: v, Enabled: true})
	}
	switch a.Type {
	case "bearer":
		header("Authorization", "Bearer "+a.Token)
	case "basic":
		header("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(a.Username+":"+a.Password)))
	case "apikey":
		if a.In == "query" {
			r.URL = appendQuery(r.URL, a.Key, a.Value)
		} else {
			header(a.Key, a.Value)
		}
	}
	return r
}

func appendQuery(raw, k, v string) string {
	frag := ""
	if i := strings.IndexByte(raw, '#'); i >= 0 {
		raw, frag = raw[:i], raw[i:]
	}
	sep := "?"
	if strings.Contains(raw, "?") {
		sep = "&"
	}
	return raw + sep + url.QueryEscape(k) + "=" + url.QueryEscape(v) + frag
}

// EncodeForm encodes enabled pairs as application/x-www-form-urlencoded, preserving order.
func EncodeForm(form []model.KV) string {
	var parts []string
	for _, kv := range form {
		if kv.Enabled && kv.Key != "" {
			parts = append(parts, url.QueryEscape(kv.Key)+"="+url.QueryEscape(kv.Value))
		}
	}
	return strings.Join(parts, "&")
}

// GraphQLBody is the JSON payload for a graphql-mode body.
func GraphQLBody(b model.Body) (string, error) {
	payload := map[string]any{"query": b.Raw}
	if vars := strings.TrimSpace(b.Variables); vars != "" {
		if !json.Valid([]byte(vars)) {
			return "", errors.New("GraphQL variables are not valid JSON")
		}
		payload["variables"] = json.RawMessage(vars)
	}
	out, err := json.Marshal(payload)
	return string(out), err
}

// Build expects a request that is already resolved and has auth applied.
func Build(ctx context.Context, r model.Request) (*http.Request, error) {
	raw := strings.TrimSpace(r.URL)
	if raw == "" {
		return nil, errors.New("URL is empty")
	}
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	u.RawQuery = strings.ReplaceAll(u.RawQuery, " ", "%20")

	method := strings.ToUpper(strings.TrimSpace(r.Method))
	if method == "" {
		method = http.MethodGet
	}

	var body io.Reader
	contentType := ""
	switch r.Body.Mode {
	case "raw":
		body = strings.NewReader(r.Body.Raw)
		contentType = r.Body.ContentType
	case "urlencoded":
		body = strings.NewReader(EncodeForm(r.Body.Form))
		contentType = "application/x-www-form-urlencoded"
	case "graphql":
		payload, err := GraphQLBody(r.Body)
		if err != nil {
			return nil, err
		}
		body = strings.NewReader(payload)
		contentType = "application/json"
	case "form":
		var buf bytes.Buffer
		w := multipart.NewWriter(&buf)
		for _, kv := range r.Body.Form {
			if kv.Enabled && kv.Key != "" {
				w.WriteField(kv.Key, kv.Value)
			}
		}
		w.Close()
		body = &buf
		contentType = w.FormDataContentType()
	}

	req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return nil, err
	}
	for _, h := range r.Headers {
		if !h.Enabled || h.Key == "" {
			continue
		}
		if strings.EqualFold(h.Key, "Host") {
			req.Host = h.Value
			continue
		}
		req.Header.Add(h.Key, h.Value)
	}
	if contentType != "" && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", contentType)
	}
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "roundtrip")
	}
	return req, nil
}

// Send never returns a Go error: failures are reported in Response.Error.
func Send(ctx context.Context, client *http.Client, r model.Request, vars map[string]string) model.Response {
	req, err := Build(ctx, ApplyAuth(ResolveRequest(r, vars)))
	if err != nil {
		return model.Response{Error: err.Error(), Headers: []model.Header{}}
	}
	start := time.Now()
	fail := func(err error) model.Response {
		return model.Response{Error: err.Error(), TimeMs: time.Since(start).Milliseconds(), Headers: []model.Header{}}
	}
	resp, err := client.Do(req)
	if err != nil {
		return fail(err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return fail(err)
	}

	out := model.Response{
		Status:     resp.StatusCode,
		StatusText: strings.TrimSpace(strings.TrimPrefix(resp.Status, strconv.Itoa(resp.StatusCode))),
		Headers:    []model.Header{},
		Size:       len(data),
		TimeMs:     time.Since(start).Milliseconds(),
	}
	keys := make([]string, 0, len(resp.Header))
	for k := range resp.Header {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		for _, v := range resp.Header[k] {
			out.Headers = append(out.Headers, model.Header{Key: k, Value: v})
		}
	}
	if utf8.Valid(data) {
		out.Body = string(data)
	} else {
		out.Body = base64.StdEncoding.EncodeToString(data)
		out.Base64 = true
	}
	return out
}
