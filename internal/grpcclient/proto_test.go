package grpcclient

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/dynamicpb"

	"roundtrip/internal/model"
)

const shopProto = `syntax = "proto3";
package shop.v1;

import "google/protobuf/any.proto";
import "google/protobuf/timestamp.proto";
import "common/money.proto";

service Shop {
  rpc GetItem(GetItemRequest) returns (Item);
  rpc Watch(GetItemRequest) returns (stream Item);
}

message GetItemRequest {
  string id = 1;
}

message Item {
  string id = 1;
  common.Money price = 2;
  google.protobuf.Timestamp created = 3;
  google.protobuf.Any detail = 4;
}

message Detail {
  string note = 1;
}
`

const moneyProto = `syntax = "proto3";
package common;

message Money {
  string currency = 1;
  int64 units = 2;
}
`

const pingProto = `syntax = "proto3";
package ping.v1;

import "google/protobuf/empty.proto";

service Ping {
  rpc Ping(google.protobuf.Empty) returns (Pong);
}

message Pong {
  string message = 1;
}
`

func write(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// tree lays out a workspace with the shop service under protos/, the file it
// imports under a second root outside the workspace (and again under the home
// directory), a file that does not parse in a dot-directory, and a service on
// its own under solo/.
type tree struct {
	workspace, ext, home string
}

func layout(t *testing.T) tree {
	t.Helper()
	tr := tree{workspace: t.TempDir(), ext: t.TempDir(), home: t.TempDir()}
	write(t, tr.workspace, map[string]string{
		"protos/shop/v1/shop.proto":  shopProto,
		"protos/.git/broken.proto":   "syntax = \"proto3\";\n\nmessage {\n",
		"protos/shop/v1/README.md":   "not a proto",
		"solo/ping.proto":            pingProto,
		"empty/keep.txt":             "",
		"collections/placeholder.md": "",
	})
	write(t, tr.ext, map[string]string{"common/money.proto": moneyProto})
	write(t, tr.home, map[string]string{"checkout/common/money.proto": moneyProto})
	t.Setenv("HOME", tr.home)
	t.Setenv("USERPROFILE", tr.home)
	return tr
}

// serveProtos starts a server WITHOUT reflection that implements the unary
// methods of the given proto config with dynamic messages. Calls must carry
// "authorization: Bearer tok". reply returns the response message as JSON.
func serveProtos(t *testing.T, p Protos, reply func(method string, in *dynamicpb.Message) (string, error)) string {
	t.Helper()
	set, err := p.load(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := grpc.NewServer()
	for _, svc := range set.services() {
		desc := grpc.ServiceDesc{ServiceName: string(svc.FullName()), HandlerType: (*any)(nil)}
		for i := range svc.Methods().Len() {
			m := svc.Methods().Get(i)
			if m.IsStreamingClient() || m.IsStreamingServer() {
				continue
			}
			desc.Methods = append(desc.Methods, grpc.MethodDesc{
				MethodName: string(m.Name()),
				Handler: func(_ any, ctx context.Context, dec func(any) error, _ grpc.UnaryServerInterceptor) (any, error) {
					md, _ := metadata.FromIncomingContext(ctx)
					if got := md.Get("authorization"); len(got) != 1 || got[0] != "Bearer tok" {
						return nil, status.Error(codes.Unauthenticated, "missing token")
					}
					grpc.SetHeader(ctx, metadata.Pairs("x-served-by", "test"))
					in := dynamicpb.NewMessage(m.Input())
					if err := dec(in); err != nil {
						return nil, err
					}
					body, err := reply(string(m.FullName()), in)
					if err != nil {
						return nil, err
					}
					out := dynamicpb.NewMessage(m.Output())
					return out, protojson.UnmarshalOptions{Resolver: set.types}.Unmarshal([]byte(body), out)
				},
			})
		}
		srv.RegisterService(&desc, struct{}{})
	}
	go srv.Serve(ln)
	t.Cleanup(srv.Stop)
	return ln.Addr().String()
}

func shop(t *testing.T, tr tree) string {
	t.Helper()
	all := Protos{Workspace: tr.workspace, Config: model.ProtoConfig{Roots: []string{"protos", tr.ext, "solo"}}}
	return serveProtos(t, all, func(method string, in *dynamicpb.Message) (string, error) {
		if method == "ping.v1.Ping.Ping" {
			return `{"message": "pong"}`, nil
		}
		id := in.Get(in.Descriptor().Fields().ByName("id")).String()
		if id == "gone" {
			return "", status.Error(codes.NotFound, "no item gone")
		}
		return `{
			"id": "` + id + `",
			"price": {"currency": "EUR", "units": 12},
			"created": "2026-01-02T03:04:05Z",
			"detail": {"@type": "type.googleapis.com/shop.v1.Detail", "note": "hi"}
		}`, nil
	})
}

func TestInvokeWithProtoFiles(t *testing.T) {
	tr := layout(t)
	addr := shop(t, tr)
	vars := map[string]string{"protoRoot": filepath.Join(tr.workspace, "protos"), "ext": tr.ext, "dir": "protos", "id": "42"}
	item := map[string]any{
		"id":      "42",
		"price":   map[string]any{"currency": "EUR", "units": "12"},
		"created": "2026-01-02T03:04:05Z",
		"detail":  map[string]any{"@type": "type.googleapis.com/shop.v1.Detail", "note": "hi"},
	}
	getItem, ping := "shop.v1.Shop/GetItem", "ping.v1.Ping/Ping"
	cases := []struct {
		name   string
		config model.ProtoConfig
		method string
		want   map[string]any
	}{
		{"roots and a file", model.ProtoConfig{Roots: []string{"protos", tr.ext}, Files: []string{"shop/v1/shop.proto"}}, getItem, item},
		{"roots only", model.ProtoConfig{Roots: []string{"protos", tr.ext}}, getItem, item},
		{"file without roots", model.ProtoConfig{Files: []string{"solo/ping.proto"}}, ping, map[string]any{"message": "pong"}},
		{"absolute file without roots", model.ProtoConfig{Files: []string{filepath.Join(tr.workspace, "solo", "ping.proto")}}, ping, map[string]any{"message": "pong"}},
		{"variables", model.ProtoConfig{Roots: []string{"{{protoRoot}}", "{{ ext }}"}, Files: []string{"shop/{{version}}/shop.proto"}}, getItem, item},
		{"variable in a workspace-relative root", model.ProtoConfig{Roots: []string{"./{{dir}}/", "{{ext}}"}}, getItem, item},
		{"home directory", model.ProtoConfig{Roots: []string{"protos", "~/checkout"}, Files: []string{"shop/v1/shop.proto"}}, getItem, item},
		{"absolute file under a root", model.ProtoConfig{Roots: []string{"protos", tr.ext}, Files: []string{filepath.Join(tr.workspace, "protos", "shop", "v1", "shop.proto")}}, getItem, item},
		{"workspace-relative file under a root", model.ProtoConfig{Roots: []string{"protos", tr.ext}, Files: []string{"protos/shop/v1/shop.proto"}}, getItem, item},
		{"backslashes", model.ProtoConfig{Roots: []string{`.\protos`, tr.ext}, Files: []string{`shop\v1/shop.proto`}}, getItem, item},
		{"blank and repeated entries", model.ProtoConfig{Roots: []string{"", "protos", "protos", tr.ext}, Files: []string{" ", "shop/v1/shop.proto", "shop/v1/shop.proto"}}, getItem, item},
	}
	vars["version"] = "v1"
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := request(addr, c.method, `{"id": "{{id}}"}`)
			if c.method == ping {
				r.Body.Raw = ""
			}
			res := Invoke(context.Background(), r, vars, Protos{Config: c.config, Workspace: tr.workspace})
			if res.Error != "" {
				t.Fatal(res.Error)
			}
			var got map[string]any
			if err := json.Unmarshal([]byte(res.Body), &got); err != nil {
				t.Fatal(err, res.Body)
			}
			if !res.GRPC || res.Status != 0 || res.StatusText != "OK" || !reflect.DeepEqual(got, c.want) {
				t.Errorf("unexpected response: %#v", res)
			}
		})
	}

	p := Protos{Workspace: tr.workspace, Config: model.ProtoConfig{Roots: []string{"protos", tr.ext}}}
	// A non-OK status and response metadata come back as on the reflection path.
	res := Invoke(context.Background(), request(addr, getItem, `{"id": "gone"}`), nil, p)
	if res.Error != "" || res.Status != int(codes.NotFound) || res.StatusText != "NotFound" || res.Body != "no item gone" {
		t.Errorf("not found: %#v", res)
	}
	res = Invoke(context.Background(), request(addr, getItem, ""), nil, p)
	served := false
	for _, h := range res.Headers {
		served = served || (h.Key == "x-served-by" && h.Value == "test")
	}
	if !served || res.Error != "" {
		t.Errorf("response metadata missing: %#v", res)
	}
	if res := Invoke(context.Background(), request(addr, getItem, `{"id": 1}`), nil, p); !strings.Contains(res.Error, "does not match shop.v1.GetItemRequest") {
		t.Errorf("bad message: %#v", res)
	}
	// The server has no reflection: without the files the call cannot be made.
	if res := Invoke(context.Background(), request(addr, getItem, ""), nil, Protos{Workspace: tr.workspace}); !strings.Contains(res.Error, "reflection failed") {
		t.Errorf("without protos: %#v", res)
	}
}

func TestProtoErrors(t *testing.T) {
	tr := layout(t)
	addr := shop(t, tr)
	protos := filepath.Join(tr.workspace, "protos")
	shopFile := filepath.Join(protos, "shop", "v1", "shop.proto")
	write(t, tr.workspace, map[string]string{"bad/a.proto": "syntax = \"proto3\";\n\nmessage A {\n  strin name = 1;\n  B b = 2\n}\n"})
	getItem := "shop.v1.Shop/GetItem"
	cases := []struct {
		name   string
		config model.ProtoConfig
		method string
		want   []string
	}{
		{"root does not exist", model.ProtoConfig{Roots: []string{"nope"}}, getItem,
			[]string{`proto root "nope" does not exist`, filepath.Join(tr.workspace, "nope")}},
		{"root is a file", model.ProtoConfig{Roots: []string{"solo/ping.proto"}}, getItem,
			[]string{"is not a directory", filepath.Join(tr.workspace, "solo", "ping.proto")}},
		{"file not under any root", model.ProtoConfig{Roots: []string{"protos", tr.ext}, Files: []string{"shop/v2/shop.proto"}}, getItem,
			[]string{`proto file "shop/v2/shop.proto" not found under any import root`, protos, tr.ext}},
		{"file outside the roots", model.ProtoConfig{Roots: []string{"protos"}, Files: []string{"solo/ping.proto"}}, getItem,
			[]string{"is not under any import root", filepath.Join(tr.workspace, "solo", "ping.proto"), protos}},
		{"file without roots does not exist", model.ProtoConfig{Files: []string{"solo/pong.proto"}}, getItem,
			[]string{`proto file "solo/pong.proto" does not exist`, filepath.Join(tr.workspace, "solo", "pong.proto")}},
		{"missing import", model.ProtoConfig{Roots: []string{"protos"}, Files: []string{"shop/v1/shop.proto"}}, getItem,
			[]string{shopFile + ":6:8: ", `import "common/money.proto" not found`, "searched: " + protos}},
		{"missing import with two roots", model.ProtoConfig{Roots: []string{"protos", "solo"}}, getItem,
			[]string{shopFile + ":6:8: ", `"common/money.proto"`, protos + ", " + filepath.Join(tr.workspace, "solo")}},
		{"parse errors", model.ProtoConfig{Roots: []string{"bad"}}, getItem,
			[]string{filepath.Join(tr.workspace, "bad", "a.proto") + ":6:1: syntax error"}},
		{"file in a skipped directory named explicitly", model.ProtoConfig{Roots: []string{"protos/.git"}}, getItem,
			[]string{filepath.Join(protos, ".git", "broken.proto") + ":3:9: syntax error"}},
		{"no proto files", model.ProtoConfig{Roots: []string{"empty"}}, getItem,
			[]string{"no .proto files found", filepath.Join(tr.workspace, "empty")}},
		{"unknown service", model.ProtoConfig{Roots: []string{"protos", tr.ext, "solo"}}, "shop.v1.Nope/GetItem",
			[]string{"service shop.v1.Nope is not defined in the loaded proto files, which define: ping.v1.Ping, shop.v1.Shop"}},
		{"unknown method", model.ProtoConfig{Roots: []string{"protos", tr.ext}}, "shop.v1.Shop/Nope",
			[]string{"service shop.v1.Shop has no method Nope", "which define: shop.v1.Shop/GetItem, shop.v1.Shop/Watch"}},
		{"streaming", model.ProtoConfig{Roots: []string{"protos", tr.ext}}, "shop.v1.Shop/Watch",
			[]string{"streaming methods are not supported yet"}},
		{"unresolved variable in a root", model.ProtoConfig{Roots: []string{"{{protoRoot}}/third_party"}}, getItem,
			[]string{`proto root "{{protoRoot}}/third_party": variable protoRoot is not set`}},
		{"unresolved variable in a file", model.ProtoConfig{Roots: []string{"protos"}, Files: []string{"{{name}}.proto"}}, getItem,
			[]string{`proto file "{{name}}.proto": variable name is not set`}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := Protos{Config: c.config, Workspace: tr.workspace}
			res := Invoke(context.Background(), request(addr, c.method, ""), nil, p)
			for _, want := range c.want {
				if !strings.Contains(res.Error, want) {
					t.Errorf("want error containing %q, got:\n%s", want, res.Error)
				}
			}
			if res.Error == "" || !res.GRPC {
				t.Errorf("unexpected response: %#v", res)
			}
			// Everything but an unknown or streaming method is a problem with the config.
			if c.method == getItem {
				if _, err := Check(context.Background(), p, nil); err == nil || err.Error() != res.Error {
					t.Errorf("Check: %v", err)
				}
			}
		})
	}
}

func TestMethodsAndCheckFromProtoFilesNeedNoServer(t *testing.T) {
	tr := layout(t)
	ctx := context.Background()
	p := Protos{Workspace: tr.workspace, Config: model.ProtoConfig{Roots: []string{"protos", "{{ext}}"}}}
	vars := map[string]string{"ext": tr.ext}
	down := model.Request{Protocol: "grpc", URL: "grpc://127.0.0.1:1"}

	got, err := Methods(ctx, down, vars, p)
	want := []string{"shop.v1.Shop/GetItem", "shop.v1.Shop/Watch"}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("methods: %q %v", got, err)
	}
	// Nothing is cached: an edit shows up on the next call.
	write(t, tr.workspace, map[string]string{"protos/shop/v1/shop.proto": strings.Replace(shopProto, "rpc Watch", "rpc Buy(GetItemRequest) returns (Item);\n  rpc Watch", 1)})
	if got, err := Methods(ctx, model.Request{}, vars, p); err != nil || !reflect.DeepEqual(got, []string{"shop.v1.Shop/Buy", "shop.v1.Shop/GetItem", "shop.v1.Shop/Watch"}) {
		t.Fatalf("after an edit: %q %v", got, err)
	}

	// Roots only loads every file below them, but not dot-directories.
	found, err := Check(ctx, p, vars)
	if err != nil || found.Files != 2 || !reflect.DeepEqual(found.Services, []string{"shop.v1.Shop"}) || len(found.Methods) != 3 {
		t.Fatalf("check: %#v %v", found, err)
	}
	// An explicit file counts alone, whatever it imports, and files without
	// services are fine.
	p.Config = model.ProtoConfig{Roots: []string{tr.ext}, Files: []string{"common/money.proto"}}
	found, err = Check(ctx, p, nil)
	if err != nil || !reflect.DeepEqual(found, model.ProtoCheck{Files: 1, Services: []string{}, Methods: []string{}}) {
		t.Fatalf("check without services: %#v %v", found, err)
	}
	// Overlapping roots do not load a file twice.
	p.Config = model.ProtoConfig{Roots: []string{tr.ext, filepath.Join(tr.ext, "common")}}
	if found, err = Check(ctx, p, nil); err != nil || found.Files != 1 {
		t.Fatalf("overlapping roots: %#v %v", found, err)
	}

	if _, err := Methods(ctx, down, nil, Protos{Workspace: tr.workspace}); err == nil || !strings.Contains(err.Error(), "reflection failed") {
		t.Fatalf("without protos the server is asked: %v", err)
	}
}
