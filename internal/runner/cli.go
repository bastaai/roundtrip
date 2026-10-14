package runner

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"

	"roundtrip/internal/grpcclient"
	"roundtrip/internal/model"
	"roundtrip/internal/sender"
	"roundtrip/internal/store"
)

const usage = `usage: roundtrip run [flags] [path ...]

Runs the requests of a workspace one after another and checks their tests.
Without a path every collection runs. A path names a collection, folder or
request by the names shown in the app, joined with "/":

  roundtrip run "Users API"
  roundtrip run -env-file ci.yaml -var token=$TOKEN "Users API/Admin"

gRPC requests use the proto files configured on their collection, found
relative to the workspace, and server reflection when it has none.

Variables come from -env, then -env-file, then -var, then whatever earlier
requests in the run extracted; later ones win. Environments are stored per
machine, so on CI use -env-file and -var.

Exit status: 0 when every request passed, 1 when one failed, 2 when the run
could not start.

`

// A request selected to run, with the names leading to it and the proto
// config of its collection.
type entry struct {
	path  []string
	req   *model.Request
	proto model.ProtoConfig
}

// Main is the `roundtrip run` command. args are the arguments after "run";
// color says whether stdout takes ANSI colors. It returns the exit status.
func Main(args []string, stdout, stderr io.Writer, color bool) int {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(stderr)
	workspace := fs.String("workspace", ".", "workspace `directory` (the one holding collections/)")
	envName := fs.String("env", "", "`name` of an environment saved on this machine")
	envFile := fs.String("env-file", "", "YAML or JSON `file` of variables, as a flat map: name: value")
	data := fs.String("data", "", "`directory` -env reads environments from; defaults to the user config directory")
	bail := fs.Bool("bail", false, "stop at the first request that fails")
	timeout := fs.Duration("timeout", 60*time.Second, "time allowed per request")
	overrides := map[string]string{}
	fs.Func("var", "set a variable, as `name=value` (repeatable)", func(s string) error {
		k, v, ok := strings.Cut(s, "=")
		if !ok || k == "" {
			return errors.New("want name=value")
		}
		overrides[k] = v
		return nil
	})
	fs.Usage = func() {
		fmt.Fprint(stderr, usage)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	fail := func(err error) int {
		fmt.Fprintln(stderr, "roundtrip run:", err)
		return 2
	}

	dir, err := filepath.Abs(*workspace)
	if err != nil {
		return fail(err)
	}
	// Without -env the per-machine data is not even looked at.
	local := ""
	if *envName != "" {
		if *data == "" {
			config, err := os.UserConfigDir()
			if err != nil {
				return fail(err)
			}
			*data = filepath.Join(config, "roundtrip")
		}
		local = store.LocalDir(*data, dir)
	}
	st, err := store.OpenReadOnly(dir, local)
	if err != nil {
		return fail(err)
	}

	vars := map[string]string{}
	if *envName != "" {
		env, err := environment(st, *envName)
		if err != nil {
			return fail(err)
		}
		vars = sender.Vars(env)
	}
	if *envFile != "" {
		fromFile := map[string]string{}
		content, err := os.ReadFile(*envFile)
		if err == nil {
			err = yaml.Unmarshal(content, &fromFile)
		}
		if err != nil {
			return fail(fmt.Errorf("%s: %w (want a flat map of name: value)", *envFile, err))
		}
		for k, v := range fromFile {
			vars[k] = v
		}
	}
	for k, v := range overrides {
		vars[k] = v
	}

	cols, err := st.Collections()
	if err != nil {
		return fail(err)
	}
	roots := make([]*model.Item, len(cols))
	protos := map[*model.Request]model.ProtoConfig{}
	for i, c := range cols {
		roots[i] = &model.Item{Name: c.Name, Items: c.Items}
		if c.Proto != nil {
			for _, e := range flatten(nil, c.Items) {
				protos[e.req] = *c.Proto
			}
		}
	}
	var entries []entry
	if fs.NArg() == 0 {
		entries = flatten(nil, roots)
	}
	for _, path := range fs.Args() {
		picked, err := pick(roots, nil, strings.Trim(path, "/"))
		if err != nil {
			return fail(err)
		}
		entries = append(entries, picked...)
	}
	if len(entries) == 0 {
		return fail(fmt.Errorf("no requests to run in %s", dir))
	}
	for i := range entries {
		entries[i].proto = protos[entries[i].req]
	}

	if os.Getenv("NO_COLOR") != "" {
		color = false
	}
	if run(context.Background(), &http.Client{Timeout: *timeout}, dir, entries, vars, *bail, stdout, color) {
		return 0
	}
	return 1
}

func environment(st *store.Store, name string) (*model.Environment, error) {
	envs, err := st.Environments()
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range envs {
		if e.Name == name {
			return e, nil
		}
		names = append(names, strconv.Quote(e.Name))
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("no environment %q: this machine has no environments for this workspace (use -env-file or -var)", name)
	}
	return nil, fmt.Errorf("no environment %q; this machine has: %s", name, strings.Join(names, ", "))
}

// flatten lists every request under items, in store order.
func flatten(prefix []string, items []*model.Item) []entry {
	var out []entry
	for _, it := range items {
		path := append(prefix[:len(prefix):len(prefix)], it.Name)
		if it.Request != nil {
			out = append(out, entry{path: path, req: it.Request})
		} else {
			out = append(out, flatten(path, it.Items)...)
		}
	}
	return out
}

// pick resolves a path of names joined with "/" below items. Names may
// themselves contain "/", so every item whose name is a prefix is tried.
func pick(items []*model.Item, prefix []string, path string) ([]entry, error) {
	var out []entry
	var deeper error
	found := false
	for _, it := range items {
		here := append(prefix[:len(prefix):len(prefix)], it.Name)
		if it.Name == path {
			found = true
			out = append(out, flatten(prefix, []*model.Item{it})...)
		} else if it.Request == nil && strings.HasPrefix(path, it.Name+"/") {
			sub, err := pick(it.Items, here, path[len(it.Name)+1:])
			if err != nil {
				deeper = err
				continue
			}
			found = true
			out = append(out, sub...)
		}
	}
	if found {
		return out, nil
	}
	if deeper != nil {
		return nil, deeper
	}
	where := "the workspace"
	if len(prefix) > 0 {
		where = strconv.Quote(strings.Join(prefix, "/"))
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("no %q in %s, which is empty", path, where)
	}
	names := make([]string, len(items))
	for i, it := range items {
		names[i] = strconv.Quote(it.Name)
	}
	return nil, fmt.Errorf("no %q in %s, which has: %s", path, where, strings.Join(names, ", "))
}

// kind is the short protocol/method label shown for a request.
func kind(r *model.Request) string {
	switch {
	case r.Protocol == "grpc":
		return "GRPC"
	case r.Body.Mode == "graphql":
		return "GQL"
	case strings.TrimSpace(r.Method) == "":
		return "GET"
	}
	return strings.ToUpper(strings.TrimSpace(r.Method))
}

// run executes the entries in order, carrying extracted variables forward,
// and reports whether all of them passed.
func run(ctx context.Context, client *http.Client, workspace string, entries []entry, vars map[string]string, bail bool, out io.Writer, color bool) bool {
	paint := func(code, s string) string {
		if !color {
			return s
		}
		return "\x1b[" + code + "m" + s + "\x1b[0m"
	}
	kindWidth, nameWidth := 0, 0
	for _, e := range entries {
		kindWidth = max(kindWidth, len(kind(e.req)))
		nameWidth = max(nameWidth, len([]rune(strings.Join(e.path, " / "))))
	}

	start := time.Now()
	passed, failed := 0, 0
	for _, e := range entries {
		res, extracted := Do(ctx, client, *e.req, vars, grpcclient.Protos{Config: e.proto, Workspace: workspace})
		for k, v := range extracted {
			vars[k] = v
		}
		ok := Passed(*e.req, res)
		mark := paint("32", "PASS")
		if ok {
			passed++
		} else {
			failed++
			mark = paint("31", "FAIL")
		}
		name := strings.Join(e.path, " / ")
		name += strings.Repeat(" ", nameWidth-len([]rune(name)))
		outcome := res.Error
		if res.Error == "" {
			outcome = strings.TrimSpace(fmt.Sprintf("%d %s", res.Status, res.StatusText)) + fmt.Sprintf("  %dms", res.TimeMs)
		}
		fmt.Fprintf(out, "%s  %-*s  %s  %s\n", mark, kindWidth, kind(e.req), name, outcome)
		if res.Error == "" {
			for _, t := range res.Tests {
				if t.Passed {
					continue
				}
				what := strings.TrimSpace(t.Target + " " + t.Op + " " + t.Value)
				if t.Op == "extract" {
					what = "extract " + t.Value + " from " + t.Target
				}
				fmt.Fprintf(out, "      %s: %s\n", what, t.Message)
			}
			if len(e.req.Tests) == 0 && !statusOK(res) {
				if res.GRPC {
					fmt.Fprintln(out, "      no tests, so the call must return OK")
				} else {
					fmt.Fprintln(out, "      no tests, so the status must be below 400")
				}
			}
		}
		if !ok && bail {
			break
		}
	}

	summary := fmt.Sprintf("%d requests: %d passed, %d failed", passed+failed, passed, failed)
	if passed+failed == 1 {
		summary = strings.Replace(summary, "requests", "request", 1)
	}
	if rest := len(entries) - passed - failed; rest > 0 {
		summary += fmt.Sprintf(", %d not run", rest)
	}
	fmt.Fprintf(out, "\n%s in %dms\n", summary, time.Since(start).Milliseconds())
	return failed == 0
}
