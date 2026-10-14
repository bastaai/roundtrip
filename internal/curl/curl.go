// Package curl converts between curl command lines and requests.
package curl

import (
	"errors"
	"net/url"
	"strings"
	"unicode"

	"roundtrip/internal/model"
	"roundtrip/internal/sender"
)

var takesValue = map[string]bool{
	"-X": true, "--request": true, "-H": true, "--header": true,
	"-d": true, "--data": true, "--data-raw": true, "--data-binary": true, "--data-ascii": true, "--data-urlencode": true, "--json": true,
	"-F": true, "--form": true, "-u": true, "--user": true, "-A": true, "--user-agent": true,
	"-b": true, "--cookie": true, "-e": true, "--referer": true, "--url": true,
	"-o": true, "--output": true, "-m": true, "--max-time": true, "--connect-timeout": true,
	"-w": true, "--write-out": true, "-x": true, "--proxy": true, "-c": true, "--cookie-jar": true,
	"--retry": true, "--cacert": true, "-E": true, "--cert": true, "--key": true,
	"-T": true, "--upload-file": true, "--resolve": true, "-r": true, "--range": true,
}

// Parse understands the subset of curl that "Copy as cURL" in browsers and API docs produce.
func Parse(cmd string) (model.Request, error) {
	r := model.Request{Auth: model.Auth{Type: "none"}, Body: model.Body{Mode: "none"}}
	args, err := tokenize(cmd)
	if err != nil {
		return r, err
	}
	if len(args) == 0 || args[0] != "curl" {
		return r, errors.New("not a curl command")
	}
	args = args[1:]

	var data []string
	var form []model.KV
	method := ""
	get, jsonFlag := false, false
	header := func(k, v string) {
		r.Headers = append(r.Headers, model.KV{Key: strings.TrimSpace(k), Value: strings.TrimSpace(v), Enabled: true})
	}

	for i := 0; i < len(args); i++ {
		a := args[i]
		if !strings.HasPrefix(a, "-") || a == "-" {
			if r.URL == "" {
				r.URL = a
			}
			continue
		}
		name, val, hasVal := a, "", false
		if strings.HasPrefix(a, "--") {
			if k, v, ok := strings.Cut(a, "="); ok {
				name, val, hasVal = k, v, true
			}
		} else if len(a) > 2 {
			name = a[:2]
			if takesValue[name] {
				// -XPOST
				val, hasVal = a[2:], true
			} else {
				// -sSL: handle the first flag now, the rest on the next pass
				args[i] = "-" + a[2:]
				i--
			}
		}
		next := func() string {
			if hasVal {
				return val
			}
			if i+1 < len(args) {
				i++
				return args[i]
			}
			return ""
		}

		switch name {
		case "-X", "--request":
			method = strings.ToUpper(next())
		case "-H", "--header":
			k, v, _ := strings.Cut(next(), ":")
			header(k, v)
		case "-d", "--data", "--data-raw", "--data-binary", "--data-ascii":
			data = append(data, next())
		case "--data-urlencode":
			v := next()
			if k, content, ok := strings.Cut(v, "="); ok {
				data = append(data, k+"="+url.QueryEscape(content))
			} else {
				data = append(data, url.QueryEscape(v))
			}
		case "--json":
			data = append(data, next())
			jsonFlag = true
		case "-F", "--form":
			k, v, _ := strings.Cut(next(), "=")
			form = append(form, model.KV{Key: k, Value: v, Enabled: true})
		case "-u", "--user":
			user, pass, _ := strings.Cut(next(), ":")
			r.Auth = model.Auth{Type: "basic", Username: user, Password: pass}
		case "-A", "--user-agent":
			header("User-Agent", next())
		case "-b", "--cookie":
			header("Cookie", next())
		case "-e", "--referer":
			header("Referer", next())
		case "--url":
			r.URL = next()
		case "-G", "--get":
			get = true
		case "-I", "--head":
			method = "HEAD"
		default:
			if takesValue[name] {
				next()
			}
		}
	}

	if r.URL == "" {
		return r, errors.New("no URL found in curl command")
	}

	body := strings.Join(data, "&")
	hasBody := false
	switch {
	case len(form) > 0:
		r.Body.Mode, r.Body.Form = "form", form
		hasBody = true
	case len(data) > 0 && get:
		sep := "?"
		if strings.Contains(r.URL, "?") {
			sep = "&"
		}
		r.URL += sep + body
	case len(data) > 0:
		hasBody = true
		if jsonFlag && !sender.HasHeader(r.Headers, "Content-Type") {
			header("Content-Type", "application/json")
			if !sender.HasHeader(r.Headers, "Accept") {
				header("Accept", "application/json")
			}
		}
		mime := "application/x-www-form-urlencoded"
		for _, h := range r.Headers {
			if strings.EqualFold(h.Key, "Content-Type") {
				mime, _, _ = strings.Cut(h.Value, ";")
				mime = strings.ToLower(strings.TrimSpace(mime))
			}
		}
		if fields, ok := parseForm(body); ok && mime == "application/x-www-form-urlencoded" {
			r.Body.Mode, r.Body.Form = "urlencoded", fields
		} else {
			r.Body.Mode, r.Body.Raw, r.Body.ContentType = "raw", body, mime
		}
	}

	if method == "" {
		method = "GET"
		if hasBody {
			method = "POST"
		}
	}
	r.Method = method
	return r, nil
}

// Export expects a request with variables already resolved.
func Export(r model.Request) string {
	r = sender.ApplyAuth(r)
	method := strings.ToUpper(strings.TrimSpace(r.Method))
	if method == "" {
		method = "GET"
	}
	parts := []string{"curl -X " + method + " " + quote(r.URL)}
	for _, h := range r.Headers {
		if h.Enabled && h.Key != "" {
			parts = append(parts, "-H "+quote(h.Key+": "+h.Value))
		}
	}
	if r.Body.Mode == "graphql" {
		// Invalid variables are dropped here; sending the request reports them.
		payload, err := sender.GraphQLBody(r.Body)
		if err != nil {
			payload, _ = sender.GraphQLBody(model.Body{Raw: r.Body.Raw})
		}
		r.Body = model.Body{Mode: "raw", Raw: payload, ContentType: "application/json"}
	}
	switch r.Body.Mode {
	case "raw":
		if r.Body.ContentType != "" && !sender.HasHeader(r.Headers, "Content-Type") {
			parts = append(parts, "-H "+quote("Content-Type: "+r.Body.ContentType))
		}
		parts = append(parts, "--data-raw "+quote(r.Body.Raw))
	case "urlencoded":
		for _, kv := range r.Body.Form {
			if kv.Enabled && kv.Key != "" {
				parts = append(parts, "--data-urlencode "+quote(kv.Key+"="+kv.Value))
			}
		}
	case "form":
		for _, kv := range r.Body.Form {
			if kv.Enabled && kv.Key != "" {
				parts = append(parts, "-F "+quote(kv.Key+"="+kv.Value))
			}
		}
	}
	return strings.Join(parts, " \\\n  ")
}

func quote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func parseForm(s string) ([]model.KV, bool) {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" || trimmed[0] == '{' || trimmed[0] == '[' {
		return nil, false
	}
	var out []model.KV
	for _, pair := range strings.Split(s, "&") {
		k, v, ok := strings.Cut(pair, "=")
		if !ok {
			return nil, false
		}
		k, err1 := url.QueryUnescape(k)
		v, err2 := url.QueryUnescape(v)
		if err1 != nil || err2 != nil {
			return nil, false
		}
		out = append(out, model.KV{Key: k, Value: v, Enabled: true})
	}
	return out, true
}

// tokenize splits a command line following POSIX shell quoting rules,
// including $'...' strings and backslash-newline continuations.
func tokenize(s string) ([]string, error) {
	var toks []string
	var cur strings.Builder
	inTok := false
	rs := []rune(s)
	for i := 0; i < len(rs); i++ {
		c := rs[i]
		switch {
		case c == '\\':
			if i+1 >= len(rs) {
				break
			}
			i++
			if rs[i] == '\r' && i+1 < len(rs) && rs[i+1] == '\n' {
				i++
			}
			if rs[i] == '\n' {
				break
			}
			cur.WriteRune(rs[i])
			inTok = true
		case c == '\'':
			inTok = true
			for i++; i < len(rs) && rs[i] != '\''; i++ {
				cur.WriteRune(rs[i])
			}
			if i >= len(rs) {
				return nil, errors.New("unterminated single quote")
			}
		case c == '$' && i+1 < len(rs) && rs[i+1] == '\'':
			inTok = true
			for i += 2; i < len(rs) && rs[i] != '\''; i++ {
				if rs[i] == '\\' && i+1 < len(rs) {
					i++
					switch rs[i] {
					case 'n':
						cur.WriteRune('\n')
					case 't':
						cur.WriteRune('\t')
					case 'r':
						cur.WriteRune('\r')
					default:
						cur.WriteRune(rs[i])
					}
					continue
				}
				cur.WriteRune(rs[i])
			}
			if i >= len(rs) {
				return nil, errors.New("unterminated $' quote")
			}
		case c == '"':
			inTok = true
			for i++; i < len(rs) && rs[i] != '"'; i++ {
				if rs[i] == '\\' && i+1 < len(rs) && strings.ContainsRune("\"\\$`\n", rs[i+1]) {
					i++
					if rs[i] == '\n' {
						continue
					}
				}
				cur.WriteRune(rs[i])
			}
			if i >= len(rs) {
				return nil, errors.New("unterminated double quote")
			}
		case unicode.IsSpace(c):
			if inTok {
				toks = append(toks, cur.String())
				cur.Reset()
				inTok = false
			}
		default:
			cur.WriteRune(c)
			inTok = true
		}
	}
	if inTok {
		toks = append(toks, cur.String())
	}
	return toks, nil
}
