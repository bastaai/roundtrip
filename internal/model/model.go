package model

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
	"time"
)

// The json tags are the API contract with the UI and, written as YAML, the
// workspace file format (see internal/store). The yaml tags are used to read
// those files back and must use the same names.

// Secret only applies to environment variables and only tells the UI to mask
// the value: environments are stored on this machine, never in the workspace.
type KV struct {
	Key     string `json:"key" yaml:"key"`
	Value   string `json:"value" yaml:"value"`
	Enabled bool   `json:"enabled" yaml:"enabled"`
	Secret  bool   `json:"secret,omitempty" yaml:"secret,omitempty"`
}

// Auth.Type is one of: none, bearer, basic, apikey.
// Auth.In is one of: header, query (apikey only).
type Auth struct {
	Type     string `json:"type" yaml:"type"`
	Token    string `json:"token,omitempty" yaml:"token,omitempty"`
	Username string `json:"username,omitempty" yaml:"username,omitempty"`
	Password string `json:"password,omitempty" yaml:"password,omitempty"`
	Key      string `json:"key,omitempty" yaml:"key,omitempty"`
	Value    string `json:"value,omitempty" yaml:"value,omitempty"`
	In       string `json:"in,omitempty" yaml:"in,omitempty"`
}

// Body.Mode is one of: none, raw, urlencoded, form, graphql.
// ContentType is the MIME type sent with a raw body. In graphql mode Raw is
// the query and Variables a JSON object.
type Body struct {
	Mode        string `json:"mode" yaml:"mode"`
	Raw         string `json:"raw,omitempty" yaml:"raw,omitempty"`
	ContentType string `json:"contentType,omitempty" yaml:"contentType,omitempty"`
	Form        []KV   `json:"form,omitempty" yaml:"form,omitempty"`
	Variables   string `json:"variables,omitempty" yaml:"variables,omitempty"`
}

// Protocol is "" for HTTP (GraphQL is HTTP with a graphql body) or "grpc".
// For gRPC, URL is the server address (grpc://host:port, or grpcs:// for
// TLS), Method the full method name (package.Service/Method), Headers the
// metadata and Body.Raw the request message as JSON.
type Request struct {
	Protocol string `json:"protocol,omitempty" yaml:"protocol,omitempty"`
	Method   string `json:"method" yaml:"method"`
	URL      string `json:"url" yaml:"url"`
	Headers  []KV   `json:"headers,omitempty" yaml:"headers,omitempty"`
	Auth     Auth   `json:"auth,omitzero" yaml:"auth,omitempty"`
	Body     Body   `json:"body,omitzero" yaml:"body,omitempty"`
	// Tests are checked against the response; Extract turns parts of the
	// response into variables for later requests. See internal/check.
	Tests   []Assertion `json:"tests,omitempty" yaml:"tests,omitempty"`
	Extract []Extract   `json:"extract,omitempty" yaml:"extract,omitempty"`
}

// Assertion.Target is one of: status, time (ms), header.<Name>, body,
// body.<path> (a dotted path into a JSON body, e.g. data.items[0].id).
// Assertion.Op is one of: eq, ne, contains, exists, lt, gt, matches.
type Assertion struct {
	Target string `json:"target" yaml:"target"`
	Op     string `json:"op" yaml:"op"`
	Value  string `json:"value,omitempty" yaml:"value,omitempty"`
}

// Extract.From uses the same syntax as Assertion.Target.
type Extract struct {
	Var  string `json:"var" yaml:"var"`
	From string `json:"from" yaml:"from"`
}

// TestResult is an Assertion after it was checked. Message says what was
// found instead when it did not pass. An extraction that found nothing is
// reported the same way, with Op "extract" and the variable name as Value.
type TestResult struct {
	Target  string `json:"target"`
	Op      string `json:"op"`
	Value   string `json:"value,omitempty"`
	Passed  bool   `json:"passed"`
	Message string `json:"message,omitempty"`
}

// Item is a folder when Request is nil.
type Item struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Request *Request `json:"request,omitempty"`
	Items   []*Item  `json:"items,omitempty"`
}

// Proto is nil when the collection has no proto config; its gRPC requests
// then use server reflection.
type Collection struct {
	ID    string       `json:"id"`
	Name  string       `json:"name"`
	Proto *ProtoConfig `json:"proto,omitempty"`
	Items []*Item      `json:"items"`
}

// ProtoConfig says which .proto files describe the gRPC services of a
// collection. Roots are import root directories (like protoc -I), relative to
// the workspace unless absolute; Files are the files to load, relative to a
// root. Both may use {{variables}} and a leading ~. With Roots only, every
// .proto file below them is loaded. See internal/grpcclient.
type ProtoConfig struct {
	Roots []string `json:"roots,omitempty" yaml:"roots,omitempty"`
	Files []string `json:"files,omitempty" yaml:"files,omitempty"`
}

// Empty reports whether there is nothing to load: a nil config, or one with
// only blank entries.
func (p *ProtoConfig) Empty() bool {
	if p == nil {
		return true
	}
	for _, list := range [][]string{p.Roots, p.Files} {
		for _, s := range list {
			if strings.TrimSpace(s) != "" {
				return false
			}
		}
	}
	return true
}

// ProtoCheck is what loading a ProtoConfig found. Files counts the loaded
// files, not their imports; Methods are "package.Service/Method".
type ProtoCheck struct {
	Files    int      `json:"files"`
	Services []string `json:"services"`
	Methods  []string `json:"methods"`
}

type Environment struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Variables []KV   `json:"variables"`
}

type HistoryEntry struct {
	ID      string    `json:"id"`
	At      time.Time `json:"at"`
	Request Request   `json:"request"`
	Status  int       `json:"status"`
	TimeMs  int64     `json:"timeMs"`
	Error   string    `json:"error,omitempty"`
	// Passed and Failed count the request's tests.
	Passed int `json:"passed,omitempty"`
	Failed int `json:"failed,omitempty"`
}

type Header struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type Response struct {
	Status     int      `json:"status"`
	StatusText string   `json:"statusText"`
	Headers    []Header `json:"headers"`
	Body       string   `json:"body"`
	Base64     bool     `json:"base64"`
	Size       int      `json:"size"`
	TimeMs     int64    `json:"timeMs"`
	Error      string   `json:"error,omitempty"`
	// GRPC marks Status as a gRPC status code (0 is OK) rather than an HTTP one.
	GRPC bool `json:"grpc,omitempty"`
	// Tests holds the results of the request's tests, in order.
	Tests []TestResult `json:"tests,omitempty"`
}

func NewID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// GRPCMethod describes one gRPC method for the request editor.
type GRPCMethod struct {
	// Input and Output are the full names of the request and response messages.
	Input     string `json:"input"`
	Output    string `json:"output"`
	Streaming bool   `json:"streaming"`
	// Template is an example request message as JSON, every field present.
	Template string `json:"template"`
}
