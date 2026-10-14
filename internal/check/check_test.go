package check

import (
	"reflect"
	"strings"
	"testing"

	"roundtrip/internal/model"
)

const body = `{
  "access_token": "abc.def",
  "count": 3,
  "price": 10.50,
  "big": 12345678901234567890,
  "ok": true,
  "nothing": null,
  "empty": "",
  "user": {"name": "Ada", "tags": ["a", "b"]},
  "data": {"items": [{"id": 7, "name": "<first>"}, {"id": 8}]},
  "matrix": [[1, 2], [3, 4]]
}`

var response = model.Response{
	Status:     201,
	StatusText: "Created",
	TimeMs:     42,
	Body:       body,
	Headers: []model.Header{
		{Key: "Content-Type", Value: "application/json; charset=utf-8"},
		{Key: "Location", Value: "/users/7"},
		{Key: "Set-Cookie", Value: "a=1"},
		{Key: "Set-Cookie", Value: "b=2"},
	},
}

func TestLookup(t *testing.T) {
	cases := []struct {
		target, want string
		found        bool
		err          string
	}{
		{"status", "201", true, ""},
		{"time", "42", true, ""},
		{"body", body, true, ""},
		{"header.Location", "/users/7", true, ""},
		{"header.content-type", "application/json; charset=utf-8", true, ""},
		{"header.Set-Cookie", "a=1", true, ""},
		{"header.X-Missing", "", false, ""},
		{"body.access_token", "abc.def", true, ""},
		{"body.count", "3", true, ""},
		{"body.price", "10.50", true, ""},
		{"body.big", "12345678901234567890", true, ""},
		{"body.ok", "true", true, ""},
		{"body.nothing", "null", true, ""},
		{"body.empty", "", true, ""},
		{"body.user.name", "Ada", true, ""},
		{"body.user.tags", `["a","b"]`, true, ""},
		{"body.user.tags[1]", "b", true, ""},
		{"body.data.items[0].id", "7", true, ""},
		{"body.data.items[0].name", "<first>", true, ""},
		{"body.data.items[1]", `{"id":8}`, true, ""},
		{"body.matrix[1][0]", "3", true, ""},
		{" body.count ", "3", true, ""},
		{"body.missing", "", false, ""},
		{"body.user.name.first", "", false, ""},
		{"body.data.items[2].id", "", false, ""},
		{"body.user[0]", "", false, ""},
		{"body.data.items.id", "", false, ""},
		{"body.data.items[x]", "", false, "bad array index"},
		{"body.data.items[-1]", "", false, "bad array index"},
		{"body.data.items[0", "", false, "bad array index"},
		{"body.user..name", "", false, "empty path segment"},
		{"body.", "", false, "empty path segment"},
		{"cookies.a", "", false, "unknown target"},
		{"", "", false, "unknown target"},
	}
	for _, tc := range cases {
		got, found, err := Lookup(response, tc.target)
		if got != tc.want || found != tc.found || (err == nil) != (tc.err == "") || err != nil && !strings.Contains(err.Error(), tc.err) {
			t.Errorf("Lookup(%q) = %q, %v, %v; want %q, %v, %q", tc.target, got, found, err, tc.want, tc.found, tc.err)
		}
	}

	list := model.Response{Body: `[{"id": 1}, {"id": 2}]`}
	if got, found, _ := Lookup(list, "body[1].id"); got != "2" || !found {
		t.Errorf("top-level array: %q %v", got, found)
	}
	if _, _, err := Lookup(model.Response{Body: "<html>"}, "body.a"); err == nil || err.Error() != "body is not JSON" {
		t.Errorf("non-JSON body: %v", err)
	}
}

func TestOps(t *testing.T) {
	vars := map[string]string{"id": "7", "name": "Ada"}
	cases := []struct {
		target, op, value string
		passed            bool
		message           string
	}{
		{"status", "eq", "201", true, ""},
		{"status", "eq", "200", false, `got "201"`},
		{"status", "ne", "500", true, ""},
		{"status", "ne", "201", false, `got "201"`},
		{"status", "lt", "400", true, ""},
		{"status", "gt", "400", false, `got "201"`},
		{"time", "lt", "1000", true, ""},
		{"time", "gt", "41.5", true, ""},
		// Numbers compare as numbers, everything else as text.
		{"body.price", "eq", "10.5", true, ""},
		{"body.price", "ne", "10.5", false, `got "10.50"`},
		{"body.count", "eq", "3.0", true, ""},
		{"body.count", "eq", " 3 ", true, ""},
		{"body.user.name", "eq", "ada", false, `got "Ada"`},
		{"body.user.name", "ne", "ada", true, ""},
		{"body.ok", "eq", "true", true, ""},
		{"body.nothing", "eq", "null", true, ""},
		{"body.empty", "eq", "", true, ""},
		{"body.user.name", "lt", "5", false, `got "Ada", which is not a number`},
		{"body.count", "gt", "many", false, `value "many" is not a number`},
		{"body", "contains", `"ok": true`, true, ""},
		{"header.Content-Type", "contains", "json", true, ""},
		{"header.Content-Type", "contains", "xml", false, `got "application/json; charset=utf-8"`},
		{"header.Location", "matches", `^/users/\d+$`, true, ""},
		{"header.Location", "matches", `^/orders/`, false, `got "/users/7"`},
		{"header.Location", "matches", `(`, false, "invalid pattern"},
		{"body.user", "exists", "", true, ""},
		{"body.nothing", "exists", "", true, ""},
		{"body.nope", "exists", "", false, "not found"},
		{"header.X-Missing", "exists", "", false, "not found"},
		// Anything but exists fails on a missing target, ne included.
		{"body.nope", "ne", "1", false, "not found"},
		{"body.nope", "eq", "", false, "not found"},
		{"body.data.items[0].id", "eq", "{{id}}", true, ""},
		{"body.user.name", "eq", "{{name}}", true, ""},
		{"header.Location", "eq", "/users/{{id}}", true, ""},
		{"body.user.name", "eq", "{{unknown}}", false, `got "Ada"`},
		{"status", "equals", "201", false, "unknown op"},
		{"cookie", "eq", "1", false, "unknown target"},
		{"body.user[", "eq", "1", false, "bad array index"},
	}
	req := model.Request{}
	for _, tc := range cases {
		req.Tests = append(req.Tests, model.Assertion{Target: tc.target, Op: tc.op, Value: tc.value})
	}
	results, extracted := Run(req, response, vars)
	if len(results) != len(cases) || len(extracted) != 0 {
		t.Fatalf("got %d results for %d tests, extracted %v", len(results), len(cases), extracted)
	}
	for i, tc := range cases {
		r := results[i]
		// The result echoes the test as written, variables unresolved.
		if r.Target != tc.target || r.Op != tc.op || r.Value != tc.value {
			t.Errorf("result %d does not echo its test: %#v", i, r)
		}
		if r.Passed != tc.passed || !strings.Contains(r.Message, tc.message) || tc.message == "" && r.Message != "" {
			t.Errorf("%s %s %q: passed=%v message=%q; want %v %q", tc.target, tc.op, tc.value, r.Passed, r.Message, tc.passed, tc.message)
		}
	}
}

func TestExtract(t *testing.T) {
	req := model.Request{
		Tests: []model.Assertion{{Target: "status", Op: "eq", Value: "201"}},
		Extract: []model.Extract{
			{Var: "token", From: "body.access_token"},
			{Var: "userId", From: "body.data.items[0].id"},
			{Var: "location", From: "header.location"},
			{Var: "status", From: "status"},
			{Var: "empty", From: "body.empty"},
			{Var: "missing", From: "body.refresh_token"},
			{Var: "bad", From: "nonsense"},
			{Var: "", From: "status"},
		},
	}
	results, extracted := Run(req, response, nil)
	want := map[string]string{"token": "abc.def", "userId": "7", "location": "/users/7", "status": "201", "empty": ""}
	if !reflect.DeepEqual(extracted, want) {
		t.Fatalf("extracted %#v", extracted)
	}
	// What could not be extracted is reported as a failed result after the tests.
	wantResults := []model.TestResult{
		{Target: "status", Op: "eq", Value: "201", Passed: true},
		{Target: "body.refresh_token", Op: "extract", Value: "missing", Message: "not found"},
		{Target: "nonsense", Op: "extract", Value: "bad", Message: `unknown target "nonsense" (use status, time, header.<Name>, body or body.<path>)`},
		{Target: "status", Op: "extract", Message: "no variable name"},
	}
	if !reflect.DeepEqual(results, wantResults) {
		t.Fatalf("results %#v", results)
	}
}

func TestNothingToCheck(t *testing.T) {
	results, extracted := Run(model.Request{}, response, nil)
	if results != nil || len(extracted) != 0 {
		t.Fatalf("got %#v %#v", results, extracted)
	}
}

func TestNoResponseFailsEverything(t *testing.T) {
	req := model.Request{
		Tests:   []model.Assertion{{Target: "status", Op: "ne", Value: "500"}, {Target: "body.x", Op: "exists"}},
		Extract: []model.Extract{{Var: "s", From: "status"}},
	}
	results, extracted := Run(req, model.Response{Error: "connection refused"}, nil)
	if len(results) != 3 || len(extracted) != 0 {
		t.Fatalf("got %#v %#v", results, extracted)
	}
	for _, r := range results {
		if r.Passed || r.Message != "no response" {
			t.Errorf("got %#v", r)
		}
	}
}

func TestGRPCResponse(t *testing.T) {
	res := model.Response{GRPC: true, Status: 5, StatusText: "NotFound", Body: "no such user"}
	req := model.Request{Tests: []model.Assertion{
		{Target: "status", Op: "eq", Value: "5"},
		{Target: "body", Op: "contains", Value: "no such"},
		{Target: "body.id", Op: "exists"},
	}}
	results, _ := Run(req, res, nil)
	if !results[0].Passed || !results[1].Passed || results[2].Passed || results[2].Message != "body is not JSON" {
		t.Fatalf("got %#v", results)
	}
}

func TestLongValuesAreShortenedInMessages(t *testing.T) {
	res := model.Response{Body: strings.Repeat("x", 5000)}
	results, _ := Run(model.Request{Tests: []model.Assertion{{Target: "body", Op: "eq", Value: "y"}}}, res, nil)
	if len(results[0].Message) > 300 {
		t.Fatalf("message is %d bytes", len(results[0].Message))
	}
}
