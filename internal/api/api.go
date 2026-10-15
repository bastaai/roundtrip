// Package api serves the JSON API and the embedded UI.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"maps"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"roundtrip/internal/curl"
	"roundtrip/internal/gitsync"
	"roundtrip/internal/grpcclient"
	"roundtrip/internal/model"
	"roundtrip/internal/runner"
	"roundtrip/internal/sender"
	"roundtrip/internal/store"
)

type Config struct {
	// Workspace is the directory opened at start.
	Workspace string
	// DataDir holds everything per-machine: the last opened workspace,
	// and the environments and history of each workspace.
	DataDir string
	Static  fs.FS
}

type server struct {
	cfg    Config
	client *http.Client

	mu        sync.RWMutex
	workspace string
	store     *store.Store
	// runtime holds the variables extracted from responses in the current
	// workspace. They are layered over the environment and never saved.
	runtime map[string]string
}

type workspaceInfo struct {
	Path         string         `json:"path"`
	GitAvailable bool           `json:"gitAvailable"`
	Git          gitsync.Status `json:"git"`
	// Recent lists other workspaces opened on this machine, newest first.
	Recent []string `json:"recent"`
}

// recentLimit is how many workspaces are remembered for quick switching.
const recentLimit = 10

// config is the per-machine config.json: the workspace to reopen and the
// ones opened before it, newest first (the current one included).
type config struct {
	Workspace string   `json:"workspace"`
	Recent    []string `json:"recent,omitempty"`
}

func readConfig(dataDir string) config {
	var cfg config
	data, _ := os.ReadFile(filepath.Join(dataDir, "config.json"))
	json.Unmarshal(data, &cfg)
	return cfg
}

// LastWorkspace is the workspace that was open when the app last ran, or "".
func LastWorkspace(dataDir string) string {
	return readConfig(dataDir).Workspace
}

// open switches the server to a workspace directory and remembers it.
func (s *server) open(dir string) error {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	st, err := store.Open(dir, store.LocalDir(s.cfg.DataDir, dir))
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.workspace, s.store, s.runtime = dir, st, map[string]string{}
	s.mu.Unlock()
	cfg := config{Workspace: dir, Recent: []string{dir}}
	for _, p := range readConfig(s.cfg.DataDir).Recent {
		if p != dir && len(cfg.Recent) < recentLimit {
			cfg.Recent = append(cfg.Recent, p)
		}
	}
	data, _ := json.Marshal(cfg)
	return os.WriteFile(filepath.Join(s.cfg.DataDir, "config.json"), data, 0o600)
}

func (s *server) st() *store.Store {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.store
}

func (s *server) dir() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.workspace
}

func (s *server) info(ctx context.Context) (workspaceInfo, error) {
	st, err := gitsync.GetStatus(ctx, s.dir())
	// Folders that have since been moved or deleted are not offered.
	recent := []string{}
	for _, p := range readConfig(s.cfg.DataDir).Recent {
		if info, statErr := os.Stat(p); p != s.dir() && statErr == nil && info.IsDir() {
			recent = append(recent, p)
		}
	}
	return workspaceInfo{Path: s.dir(), GitAvailable: gitsync.Available(), Git: st, Recent: recent}, err
}

// openWorkspace switches to dir. With carry, a folder that has no collections
// yet receives a copy of the current ones, and the current environments
// follow on this machine when the new workspace has none; this is how
// requests started elsewhere are moved into a repository folder.
func (s *server) openWorkspace(dir string, carry bool) error {
	prev, prevStore := s.dir(), s.st()
	if carry {
		if _, err := os.Stat(filepath.Join(dir, "collections")); err == nil {
			carry = false
		}
	}
	if carry {
		if _, err := os.Stat(filepath.Join(prev, "collections")); err == nil {
			if err := os.CopyFS(filepath.Join(dir, "collections"), os.DirFS(filepath.Join(prev, "collections"))); err != nil {
				return err
			}
		}
	}
	if err := s.open(dir); err != nil {
		return err
	}
	if !carry {
		return nil
	}
	if envs, err := s.st().Environments(); err != nil || len(envs) > 0 {
		return err
	}
	envs, err := prevStore.Environments()
	for _, e := range envs {
		if err := s.st().SaveEnvironment(e); err != nil {
			return err
		}
	}
	return err
}

// localGRPC is where GET /api/grpc/local looks; a variable so tests can point it elsewhere.
var localGRPC = "localhost:50051"

func New(cfg Config) (http.Handler, error) {
	s := &server{cfg: cfg, client: &http.Client{Timeout: 60 * time.Second}}
	if err := os.MkdirAll(cfg.DataDir, 0o700); err != nil {
		return nil, err
	}
	if err := s.open(cfg.Workspace); err != nil {
		return nil, err
	}
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/fs/list", s.listDir)
	mux.HandleFunc("GET /api/workspace", func(w http.ResponseWriter, r *http.Request) {
		respond(w)(s.info(r.Context()))
	})
	mux.HandleFunc("POST /api/workspace/open", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Path  string `json:"path"`
			Carry bool   `json:"carry"`
		}
		if !decode(w, r, &in) {
			return
		}
		if !filepath.IsAbs(in.Path) {
			http.Error(w, "enter the full path of a folder", http.StatusBadRequest)
			return
		}
		if err := s.openWorkspace(filepath.Clean(in.Path), in.Carry); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		respond(w)(s.info(r.Context()))
	})
	// git talks to the network, so these get a generous timeout and report
	// git's own message (auth failures, conflicts) to the UI.
	gitAction := func(run func(ctx context.Context, r *http.Request) error) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if !gitsync.Available() {
				http.Error(w, "git is not installed", http.StatusBadRequest)
				return
			}
			ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
			defer cancel()
			if err := run(ctx, r); err != nil {
				http.Error(w, err.Error(), http.StatusBadGateway)
				return
			}
			respond(w)(s.info(ctx))
		}
	}
	mux.HandleFunc("POST /api/git/sync", gitAction(func(ctx context.Context, r *http.Request) error {
		var in struct {
			Message string `json:"message"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			return err
		}
		return gitsync.Sync(ctx, s.dir(), in.Message)
	}))
	mux.HandleFunc("POST /api/git/pull", gitAction(func(ctx context.Context, r *http.Request) error {
		return gitsync.Pull(ctx, s.dir())
	}))

	mux.HandleFunc("GET /api/collections", func(w http.ResponseWriter, r *http.Request) {
		respond(w)(s.st().Collections())
	})
	mux.HandleFunc("PUT /api/collections/{id}", func(w http.ResponseWriter, r *http.Request) {
		var c model.Collection
		if !decode(w, r, &c) {
			return
		}
		c.ID = r.PathValue("id")
		respond(w)(&c, s.st().SaveCollection(&c))
	})
	mux.HandleFunc("DELETE /api/collections/{id}", func(w http.ResponseWriter, r *http.Request) {
		respond(w)(nil, s.st().DeleteCollection(r.PathValue("id")))
	})

	mux.HandleFunc("GET /api/environments", func(w http.ResponseWriter, r *http.Request) {
		respond(w)(s.st().Environments())
	})
	mux.HandleFunc("PUT /api/environments/{id}", func(w http.ResponseWriter, r *http.Request) {
		var e model.Environment
		if !decode(w, r, &e) {
			return
		}
		e.ID = r.PathValue("id")
		respond(w)(&e, s.st().SaveEnvironment(&e))
	})
	mux.HandleFunc("DELETE /api/environments/{id}", func(w http.ResponseWriter, r *http.Request) {
		respond(w)(nil, s.st().DeleteEnvironment(r.PathValue("id")))
	})

	mux.HandleFunc("GET /api/history", func(w http.ResponseWriter, r *http.Request) {
		respond(w)(s.st().History())
	})
	mux.HandleFunc("GET /api/history/{id}/response", func(w http.ResponseWriter, r *http.Request) {
		res, err := s.st().HistoryResponse(r.PathValue("id"))
		if errors.Is(err, fs.ErrNotExist) {
			http.Error(w, "this response was not kept", http.StatusNotFound)
			return
		}
		respond(w)(res, err)
	})
	mux.HandleFunc("DELETE /api/history", func(w http.ResponseWriter, r *http.Request) {
		respond(w)(nil, s.st().ClearHistory())
	})

	mux.HandleFunc("POST /api/send", s.send)
	mux.HandleFunc("GET /api/variables/runtime", func(w http.ResponseWriter, r *http.Request) {
		s.mu.RLock()
		defer s.mu.RUnlock()
		respond(w)(s.runtime, nil)
	})
	mux.HandleFunc("DELETE /api/variables/runtime", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.runtime = map[string]string{}
		s.mu.Unlock()
		respond(w)(nil, nil)
	})
	mux.HandleFunc("POST /api/curl/parse", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Command string `json:"command"`
		}
		if !decode(w, r, &in) {
			return
		}
		req, err := curl.Parse(in.Command)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		respond(w)(req, nil)
	})
	mux.HandleFunc("POST /api/curl/export", func(w http.ResponseWriter, r *http.Request) {
		var in sendInput
		if !decode(w, r, &in) {
			return
		}
		vars, err := s.vars(in.EnvironmentID)
		respond(w)(map[string]string{"command": curl.Export(sender.ResolveRequest(in.Request, vars))}, err)
	})

	// The usual address of a gRPC server under development, offered for a gRPC
	// request with no address yet when something is listening there.
	mux.HandleFunc("GET /api/grpc/local", func(w http.ResponseWriter, r *http.Request) {
		address := ""
		if conn, err := net.DialTimeout("tcp", localGRPC, 300*time.Millisecond); err == nil {
			conn.Close()
			address = "grpc://" + localGRPC
		}
		respond(w)(map[string]string{"address": address}, nil)
	})
	mux.HandleFunc("POST /api/grpc/describe", func(w http.ResponseWriter, r *http.Request) {
		var in sendInput
		if !decode(w, r, &in) {
			return
		}
		dir := s.dir()
		vars, err := s.vars(in.EnvironmentID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()
		method, err := grpcclient.Describe(ctx, in.Request, vars, grpcclient.Protos{Config: in.Proto, Workspace: dir})
		if err != nil {
			// With proto files the server is not involved: the config or the method name is wrong.
			code := http.StatusBadGateway
			if !in.Proto.Empty() {
				code = http.StatusBadRequest
			}
			http.Error(w, err.Error(), code)
			return
		}
		respond(w)(method, nil)
	})
	mux.HandleFunc("POST /api/grpc/methods", func(w http.ResponseWriter, r *http.Request) {
		var in sendInput
		if !decode(w, r, &in) {
			return
		}
		dir := s.dir()
		vars, err := s.vars(in.EnvironmentID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()
		methods, err := grpcclient.Methods(ctx, in.Request, vars, grpcclient.Protos{Config: in.Proto, Workspace: dir})
		if err != nil {
			// With proto files the server is not involved: the config is wrong.
			code := http.StatusBadGateway
			if !in.Proto.Empty() {
				code = http.StatusBadRequest
			}
			http.Error(w, err.Error(), code)
			return
		}
		respond(w)(methods, nil)
	})
	mux.HandleFunc("POST /api/proto/check", func(w http.ResponseWriter, r *http.Request) {
		var in sendInput
		if !decode(w, r, &in) {
			return
		}
		dir := s.dir()
		vars, err := s.vars(in.EnvironmentID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if in.Proto.Empty() {
			http.Error(w, "add an import root or a proto file first", http.StatusBadRequest)
			return
		}
		found, err := grpcclient.Check(r.Context(), grpcclient.Protos{Config: in.Proto, Workspace: dir}, vars)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		respond(w)(found, nil)
	})

	mux.Handle("/", http.FileServerFS(cfg.Static))
	return guard(mux), nil
}

// Proto is the proto config of the collection the request belongs to; it
// only matters for gRPC.
type sendInput struct {
	Request       model.Request     `json:"request"`
	EnvironmentID string            `json:"environmentId"`
	Proto         model.ProtoConfig `json:"proto"`
	// ItemID is the saved request being sent, so its earlier responses can be found.
	ItemID string `json:"itemId"`
}

// vars returns the variables of an environment with the runtime variables
// layered on top.
func (s *server) vars(environmentID string) (map[string]string, error) {
	env, err := s.st().Environment(environmentID)
	vars := sender.Vars(env)
	s.mu.RLock()
	defer s.mu.RUnlock()
	maps.Copy(vars, s.runtime)
	return vars, err
}

func (s *server) send(w http.ResponseWriter, r *http.Request) {
	var in sendInput
	if !decode(w, r, &in) {
		return
	}
	s.mu.RLock()
	st, dir := s.store, s.workspace
	s.mu.RUnlock()
	vars, err := s.vars(in.EnvironmentID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	res, extracted := runner.Do(r.Context(), s.client, in.Request, vars, grpcclient.Protos{Config: in.Proto, Workspace: dir})
	s.mu.Lock()
	// The workspace may have been switched while the request was out.
	if s.store == st {
		maps.Copy(s.runtime, extracted)
	}
	s.mu.Unlock()

	entry := &model.HistoryEntry{
		ID:      model.NewID(),
		At:      time.Now(),
		Request: in.Request,
		ItemID:  in.ItemID,
		Status:  res.Status,
		TimeMs:  res.TimeMs,
		Error:   res.Error,
	}
	for _, t := range res.Tests {
		if t.Passed {
			entry.Passed++
		} else {
			entry.Failed++
		}
	}
	respond(w)(res, st.AddHistory(entry, &res))
}

// guard rejects requests that don't come from this machine's own UI. Without
// it any web page could use the local server to send requests into the user's
// network (cross-site POST, DNS rebinding).
func guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ok := loopback(r.Host)
		if origin := r.Header.Get("Origin"); ok && origin != "" {
			u, err := url.Parse(origin)
			ok = err == nil && loopback(u.Host)
		}
		if !ok {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func loopback(hostport string) bool {
	host := hostport
	if h, _, err := net.SplitHostPort(hostport); err == nil {
		host = h
	}
	host = strings.Trim(host, "[]")
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return false
	}
	return true
}

// respond returns a func so handlers can pass a (value, error) pair straight through.
func respond(w http.ResponseWriter) func(v any, err error) {
	return func(v any, err error) {
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if v == nil {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(v)
	}
}
