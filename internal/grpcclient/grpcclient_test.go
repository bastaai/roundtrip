package grpcclient

import (
	"context"
	"net"
	"reflect"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"

	"roundtrip/internal/model"
)

// serve starts a server exposing the standard health service plus reflection.
// Calls must carry "authorization: Bearer tok".
func serve(t *testing.T, withReflection bool) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := grpc.NewServer(grpc.UnaryInterceptor(func(ctx context.Context, req any, info *grpc.UnaryServerInfo, next grpc.UnaryHandler) (any, error) {
		md, _ := metadata.FromIncomingContext(ctx)
		if got := md.Get("authorization"); len(got) != 1 || got[0] != "Bearer tok" {
			return nil, status.Error(codes.Unauthenticated, "missing token")
		}
		grpc.SetHeader(ctx, metadata.Pairs("x-served-by", "test"))
		return next(ctx, req)
	}))
	h := health.NewServer()
	h.SetServingStatus("db", healthpb.HealthCheckResponse_NOT_SERVING)
	healthpb.RegisterHealthServer(srv, h)
	if withReflection {
		reflection.Register(srv)
	}
	go srv.Serve(ln)
	t.Cleanup(srv.Stop)
	return ln.Addr().String()
}

func request(addr, method, msg string) model.Request {
	return model.Request{
		Protocol: "grpc",
		URL:      "grpc://" + addr,
		Method:   method,
		Auth:     model.Auth{Type: "bearer", Token: "tok"},
		Body:     model.Body{Mode: "raw", Raw: msg},
	}
}

func TestMethods(t *testing.T) {
	addr := serve(t, true)
	got, err := Methods(context.Background(), request(addr, "", ""), nil, Protos{})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"grpc.health.v1.Health/Check", "grpc.health.v1.Health/List", "grpc.health.v1.Health/Watch"}
	if !reflect.DeepEqual(got, want) {
		// The health service gains methods over time; Check and Watch are the stable ones.
		joined := strings.Join(got, " ")
		if !strings.Contains(joined, want[0]) || !strings.Contains(joined, want[2]) || strings.Contains(joined, "reflection") {
			t.Fatalf("got %q", got)
		}
	}
}

func TestInvoke(t *testing.T) {
	addr := serve(t, true)
	ctx := context.Background()

	res := Invoke(ctx, request(addr, "grpc.health.v1.Health/Check", `{"service": "db"}`), nil, Protos{})
	if res.Error != "" {
		t.Fatal(res.Error)
	}
	if !res.GRPC || res.Status != 0 || res.StatusText != "OK" || !strings.Contains(res.Body, `"NOT_SERVING"`) {
		t.Errorf("unexpected response: %#v", res)
	}
	found := false
	for _, h := range res.Headers {
		found = found || (h.Key == "x-served-by" && h.Value == "test")
	}
	if !found {
		t.Errorf("response metadata missing: %#v", res.Headers)
	}

	// Empty message, leading slash and a bare host:port all work.
	r := request(addr, "/grpc.health.v1.Health/Check", "")
	r.URL = addr
	if res := Invoke(ctx, r, nil, Protos{}); res.Error != "" || !strings.Contains(res.Body, `"SERVING"`) {
		t.Errorf("defaults: %#v", res)
	}

	// A non-OK status from the server is a response, not a transport failure.
	res = Invoke(ctx, request(addr, "grpc.health.v1.Health/Check", `{"service": "unknown"}`), nil, Protos{})
	if res.Error != "" || res.Status != int(codes.NotFound) || res.StatusText != "NotFound" || res.Body == "" {
		t.Errorf("not found: %#v", res)
	}
}

func TestInvokeErrors(t *testing.T) {
	addr := serve(t, true)
	ctx := context.Background()
	cases := map[string]model.Request{
		"must look like":      request(addr, "Check", ""),
		"no method":           request(addr, "grpc.health.v1.Health/Nope", ""),
		"does not match":      request(addr, "grpc.health.v1.Health/Check", `{"nope": 1}`),
		"streaming":           request(addr, "grpc.health.v1.Health/Watch", ""),
		"unsupported scheme":  {URL: "ftp://" + addr, Method: "a.B/C"},
		"address is empty":    {Method: "a.B/C"},
		"reflection failed":   request(serve(t, false), "grpc.health.v1.Health/Check", ""),
		"grpc.health.v1.Nope": request(addr, "grpc.health.v1.Nope/Check", ""),
	}
	for want, r := range cases {
		res := Invoke(ctx, r, nil, Protos{})
		if !strings.Contains(res.Error, want) {
			t.Errorf("want error containing %q, got %q", want, res.Error)
		}
	}
}
