// Package check evaluates a request's tests against its response and extracts
// variables from it. It does no I/O.
//
// A target names part of a response:
//
//	status          HTTP status code, or the gRPC code
//	time            milliseconds the request took
//	header.<Name>   first header with that name, case-insensitive
//	body            the whole body as text
//	body.<path>     a value inside a JSON body: data.items[0].id
package check

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"roundtrip/internal/model"
	"roundtrip/internal/sender"
)

// Run checks r.Tests against res and returns one result per test, in order,
// followed by one failed result (Op "extract") for every r.Extract entry
// whose target was not in the response. The second value holds the variables
// that were extracted. {{variables}} in test values are resolved from vars.
func Run(r model.Request, res model.Response, vars map[string]string) ([]model.TestResult, map[string]string) {
	var results []model.TestResult
	extracted := map[string]string{}
	for _, a := range r.Tests {
		out := model.TestResult{Target: a.Target, Op: a.Op, Value: a.Value}
		out.Passed, out.Message = test(a, res, vars)
		results = append(results, out)
	}
	for _, e := range r.Extract {
		got, found, err := Lookup(res, e.From)
		switch {
		case res.Error != "":
			err = fmt.Errorf("no response")
		case err == nil && !found:
			err = fmt.Errorf("not found")
		case err == nil && e.Var == "":
			err = fmt.Errorf("no variable name")
		}
		if err != nil {
			results = append(results, model.TestResult{Target: e.From, Op: "extract", Value: e.Var, Message: err.Error()})
			continue
		}
		extracted[e.Var] = got
	}
	return results, extracted
}

func test(a model.Assertion, res model.Response, vars map[string]string) (bool, string) {
	if res.Error != "" {
		return false, "no response"
	}
	got, found, err := Lookup(res, a.Target)
	if err != nil {
		return false, err.Error()
	}
	if a.Op == "exists" {
		return found, message(found, "not found")
	}
	if !found {
		return false, "not found"
	}
	want := sender.Resolve(a.Value, vars)
	g, errG := strconv.ParseFloat(strings.TrimSpace(got), 64)
	w, errW := strconv.ParseFloat(strings.TrimSpace(want), 64)
	numeric := errG == nil && errW == nil
	var ok bool
	switch a.Op {
	case "eq":
		ok = got == want || numeric && g == w
	case "ne":
		ok = got != want && !(numeric && g == w)
	case "contains":
		ok = strings.Contains(got, want)
	case "lt", "gt":
		if !numeric {
			if errG != nil {
				return false, "got " + short(got) + ", which is not a number"
			}
			return false, "value " + short(want) + " is not a number"
		}
		ok = a.Op == "lt" && g < w || a.Op == "gt" && g > w
	case "matches":
		re, err := regexp.Compile(want)
		if err != nil {
			return false, "invalid pattern: " + err.Error()
		}
		ok = re.MatchString(got)
	default:
		return false, fmt.Sprintf("unknown op %q (use eq, ne, contains, exists, lt, gt or matches)", a.Op)
	}
	return ok, message(ok, "got "+short(got))
}

func message(ok bool, msg string) string {
	if ok {
		return ""
	}
	return msg
}

// short keeps a failure message readable when the value is a whole body.
func short(s string) string {
	if rs := []rune(s); len(rs) > 200 {
		s = string(rs[:200]) + "…"
	}
	return strconv.Quote(s)
}

// Lookup returns the part of the response a target names. found is false when
// the response has no such header or JSON value; err reports a target that
// cannot be evaluated at all (unknown target, body is not JSON).
func Lookup(res model.Response, target string) (value string, found bool, err error) {
	target = strings.TrimSpace(target)
	switch {
	case target == "status":
		return strconv.Itoa(res.Status), true, nil
	case target == "time":
		return strconv.FormatInt(res.TimeMs, 10), true, nil
	case target == "body":
		return res.Body, true, nil
	case strings.HasPrefix(target, "header."):
		name := strings.TrimPrefix(target, "header.")
		for _, h := range res.Headers {
			if strings.EqualFold(h.Key, name) {
				return h.Value, true, nil
			}
		}
		return "", false, nil
	case strings.HasPrefix(target, "body.") || strings.HasPrefix(target, "body["):
		steps, err := parsePath(strings.TrimPrefix(strings.TrimPrefix(target, "body"), "."))
		if err != nil {
			return "", false, fmt.Errorf("%s: %w", target, err)
		}
		dec := json.NewDecoder(strings.NewReader(res.Body))
		dec.UseNumber()
		var doc any
		if err := dec.Decode(&doc); err != nil {
			return "", false, fmt.Errorf("body is not JSON")
		}
		for _, s := range steps {
			switch v := doc.(type) {
			case map[string]any:
				if s.key == nil {
					return "", false, nil
				}
				if doc, found = v[*s.key]; !found {
					return "", false, nil
				}
			case []any:
				if s.key != nil || s.index >= len(v) {
					return "", false, nil
				}
				doc = v[s.index]
			default:
				return "", false, nil
			}
		}
		return text(doc), true, nil
	}
	return "", false, fmt.Errorf("unknown target %q (use status, time, header.<Name>, body or body.<path>)", target)
}

// text renders a JSON value the way it would be typed into a test: strings
// without quotes, numbers as written, objects and arrays as compact JSON.
func text(v any) string {
	switch v := v.(type) {
	case string:
		return v
	case json.Number:
		return v.String()
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.Encode(v)
	return strings.TrimSuffix(buf.String(), "\n")
}

// step is an object key, or an array index when key is nil.
type step struct {
	key   *string
	index int
}

// parsePath splits "data.items[0].id" into data, items, 0, id.
func parsePath(path string) ([]step, error) {
	var steps []step
	for _, part := range strings.Split(path, ".") {
		key, rest, _ := strings.Cut(part, "[")
		if key != "" || rest == "" {
			if key == "" {
				return nil, fmt.Errorf("empty path segment")
			}
			steps = append(steps, step{key: &key})
		}
		if !strings.Contains(part, "[") {
			continue
		}
		for _, idx := range strings.Split(rest, "[") {
			n, err := strconv.Atoi(strings.TrimSuffix(idx, "]"))
			if err != nil || n < 0 || !strings.HasSuffix(idx, "]") {
				return nil, fmt.Errorf("bad array index %q", "["+idx)
			}
			steps = append(steps, step{index: n})
		}
	}
	return steps, nil
}
