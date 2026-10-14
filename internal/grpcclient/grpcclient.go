// Package grpcclient calls unary gRPC methods. The descriptors come from
// local .proto files when a proto config is given (see Protos) and from
// server reflection otherwise.
package grpcclient

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jhump/protoreflect/v2/grpcreflect"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/dynamicpb"

	"roundtrip/internal/model"
	"roundtrip/internal/sender"
)

// dial accepts host:port, grpc://host:port (plaintext) and grpcs://host:port
// (TLS). http:// and https:// are treated the same way.
func dial(addr string) (*grpc.ClientConn, error) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return nil, errors.New("server address is empty")
	}
	creds := insecure.NewCredentials()
	if scheme, rest, ok := strings.Cut(addr, "://"); ok {
		addr = rest
		switch strings.ToLower(scheme) {
		case "grpc", "http":
		case "grpcs", "https":
			creds = credentials.NewTLS(&tls.Config{})
		default:
			return nil, fmt.Errorf("unsupported scheme %q: use grpc:// or grpcs://", scheme)
		}
	}
	addr = strings.TrimRight(addr, "/")
	return grpc.NewClient(addr, grpc.WithTransportCredentials(creds))
}

// outgoing turns the request's headers and auth into gRPC metadata.
func outgoing(ctx context.Context, r model.Request) context.Context {
	if r.Auth.Type == "apikey" {
		r.Auth.In = "header"
	}
	md := metadata.MD{}
	for _, h := range sender.ApplyAuth(r).Headers {
		if h.Enabled && h.Key != "" {
			md.Append(h.Key, h.Value)
		}
	}
	return metadata.NewOutgoingContext(ctx, md)
}

// Methods lists what can be called as "package.Service/Method": every method
// of the proto files when p has a config (the server is not contacted), every
// method the server exposes through reflection otherwise.
func Methods(ctx context.Context, r model.Request, vars map[string]string, p Protos) ([]string, error) {
	if !p.Config.Empty() {
		set, err := p.load(ctx, vars)
		if err != nil {
			return nil, err
		}
		return set.check().Methods, nil
	}
	r = sender.ResolveRequest(r, vars)
	conn, err := dial(r.URL)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	client := grpcreflect.NewClientAuto(outgoing(ctx, r), conn)
	defer client.Reset()

	services, err := client.ListServices()
	if err != nil {
		return nil, reflectionError(err)
	}
	out := []string{}
	for _, name := range services {
		if strings.HasPrefix(string(name), "grpc.reflection.") {
			continue
		}
		svc, err := service(client, name)
		if err != nil {
			return nil, err
		}
		out = append(out, methodNames(svc)...)
	}
	sort.Strings(out)
	return out, nil
}

func methodNames(svc protoreflect.ServiceDescriptor) []string {
	methods := svc.Methods()
	out := make([]string, methods.Len())
	for i := range out {
		out[i] = string(svc.FullName()) + "/" + string(methods.Get(i).Name())
	}
	return out
}

// Invoke never returns a Go error: failures are reported in Response.Error.
// A call the server answered with a non-OK status is not a failure: it comes
// back as Status (the gRPC code) with the status message as body.
// With a proto config in p the server is never asked for reflection.
func Invoke(ctx context.Context, r model.Request, vars map[string]string, p Protos) model.Response {
	start := time.Now()
	fail := func(err error) model.Response {
		return model.Response{Error: err.Error(), TimeMs: time.Since(start).Milliseconds(), Headers: []model.Header{}, GRPC: true}
	}
	r = sender.ResolveRequest(r, vars)

	svcName, methodName, ok := strings.Cut(strings.TrimPrefix(strings.TrimSpace(r.Method), "/"), "/")
	if !ok || svcName == "" || methodName == "" {
		return fail(errors.New("method must look like package.Service/Method"))
	}
	conn, err := dial(r.URL)
	if err != nil {
		return fail(err)
	}
	defer conn.Close()
	ctx = outgoing(ctx, r)

	// types resolves google.protobuf.Any; nil means the compiled-in types.
	var svc protoreflect.ServiceDescriptor
	var types typeResolver
	if p.Config.Empty() {
		client := grpcreflect.NewClientAuto(ctx, conn)
		defer client.Reset()
		svc, err = service(client, protoreflect.FullName(svcName))
	} else {
		var set *protoSet
		if set, err = p.load(ctx, vars); err == nil {
			svc, err = set.service(svcName)
			types = set.types
		}
	}
	if err != nil {
		return fail(err)
	}
	method := svc.Methods().ByName(protoreflect.Name(methodName))
	if method == nil {
		if types == nil {
			return fail(fmt.Errorf("service %s has no method %s", svcName, methodName))
		}
		return fail(fmt.Errorf("service %s has no method %s in the loaded proto files, which define: %s", svcName, methodName, list(methodNames(svc))))
	}
	if method.IsStreamingClient() || method.IsStreamingServer() {
		return fail(errors.New("streaming methods are not supported yet"))
	}

	in := dynamicpb.NewMessage(method.Input())
	if msg := strings.TrimSpace(r.Body.Raw); msg != "" {
		if err := (protojson.UnmarshalOptions{Resolver: types}).Unmarshal([]byte(msg), in); err != nil {
			return fail(fmt.Errorf("message does not match %s: %w", method.Input().FullName(), err))
		}
	}
	out := dynamicpb.NewMessage(method.Output())
	var header, trailer metadata.MD
	err = conn.Invoke(ctx, "/"+svcName+"/"+methodName, in, out, grpc.Header(&header), grpc.Trailer(&trailer))

	st := status.Convert(err)
	res := model.Response{
		GRPC:       true,
		Status:     int(st.Code()),
		StatusText: st.Code().String(),
		Headers:    []model.Header{},
		TimeMs:     time.Since(start).Milliseconds(),
	}
	for _, md := range []metadata.MD{header, trailer} {
		keys := make([]string, 0, len(md))
		for k := range md {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			for _, v := range md[k] {
				res.Headers = append(res.Headers, model.Header{Key: k, Value: v})
			}
		}
	}
	if err != nil {
		res.Body = st.Message()
	} else {
		body, err := protojson.MarshalOptions{Multiline: true, Indent: "  ", EmitUnpopulated: true, Resolver: types}.Marshal(out)
		if err != nil {
			return fail(err)
		}
		res.Body = string(body)
	}
	res.Size = len(res.Body)
	return res
}

// typeResolver is what protojson needs to expand google.protobuf.Any.
type typeResolver interface {
	protoregistry.MessageTypeResolver
	protoregistry.ExtensionTypeResolver
}

func list(names []string) string {
	if len(names) == 0 {
		return "nothing"
	}
	return strings.Join(names, ", ")
}

func service(client *grpcreflect.Client, name protoreflect.FullName) (protoreflect.ServiceDescriptor, error) {
	file, err := client.FileContainingSymbol(name)
	if err != nil {
		return nil, reflectionError(err)
	}
	services := file.Services()
	for i := range services.Len() {
		if services.Get(i).FullName() == name {
			return services.Get(i), nil
		}
	}
	return nil, fmt.Errorf("service %s not found", name)
}

func reflectionError(err error) error {
	if grpcreflect.IsElementNotFoundError(err) {
		return err
	}
	return fmt.Errorf("server reflection failed (is it enabled on the server?): %w", err)
}
