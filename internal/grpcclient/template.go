package grpcclient

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/jhump/protoreflect/v2/grpcreflect"
	"google.golang.org/protobuf/reflect/protoreflect"

	"roundtrip/internal/model"
	"roundtrip/internal/sender"
)

// Describe reports the message types of the request's method and an example
// request message to start from. Like Invoke, it reads the descriptors from
// the proto files when p has a config and from server reflection otherwise.
func Describe(ctx context.Context, r model.Request, vars map[string]string, p Protos) (model.GRPCMethod, error) {
	r = sender.ResolveRequest(r, vars)
	svcName, methodName, ok := strings.Cut(strings.TrimPrefix(strings.TrimSpace(r.Method), "/"), "/")
	if !ok || svcName == "" || methodName == "" {
		return model.GRPCMethod{}, errors.New("method must look like package.Service/Method")
	}

	var svc protoreflect.ServiceDescriptor
	if p.Config.Empty() {
		conn, err := dial(r.URL)
		if err != nil {
			return model.GRPCMethod{}, err
		}
		defer conn.Close()
		client := grpcreflect.NewClientAuto(outgoing(ctx, r), conn)
		defer client.Reset()
		if svc, err = service(client, protoreflect.FullName(svcName)); err != nil {
			return model.GRPCMethod{}, err
		}
	} else {
		set, err := p.load(ctx, vars)
		if err != nil {
			return model.GRPCMethod{}, err
		}
		if svc, err = set.service(svcName); err != nil {
			return model.GRPCMethod{}, err
		}
	}
	method := svc.Methods().ByName(protoreflect.Name(methodName))
	if method == nil {
		return model.GRPCMethod{}, fmt.Errorf("service %s has no method %s; it defines: %s", svcName, methodName, list(methodNames(svc)))
	}
	return model.GRPCMethod{
		Input:     string(method.Input().FullName()),
		Output:    string(method.Output().FullName()),
		Streaming: method.IsStreamingClient() || method.IsStreamingServer(),
		Template:  Template(method.Input()),
		Enums:     Enums(method.Input()),
	}, nil
}

// Template writes an example of a message in the proto JSON mapping: every
// field present with its zero value, in declaration order, so the shape of
// the message can be seen and filled in. Repeated fields and maps get one
// entry to show what goes in them, and only the first member of a oneof is
// included. A message that contains itself stops at null. The result always
// parses back into the message.
func Template(md protoreflect.MessageDescriptor) string {
	var b strings.Builder
	writeMessage(&b, md, "", map[protoreflect.FullName]bool{})
	return b.String()
}

// Enums lists the enum fields reachable from md, walked like Template
// (declaration order, well-known types skipped, a message containing itself
// not entered again) except that every member of a oneof is included. A path
// segment "*" stands for every element of a list or value of a map.
func Enums(md protoreflect.MessageDescriptor) []model.GRPCEnum {
	return appendEnums(nil, md, nil, map[protoreflect.FullName]bool{})
}

func appendEnums(out []model.GRPCEnum, md protoreflect.MessageDescriptor, path []string, open map[protoreflect.FullName]bool) []model.GRPCEnum {
	if _, ok := wellKnown[md.FullName()]; ok || open[md.FullName()] {
		return out
	}
	open[md.FullName()] = true
	defer delete(open, md.FullName())

	fields := md.Fields()
	for i := range fields.Len() {
		fd := fields.Get(i)
		p := append(path[:len(path):len(path)], fd.JSONName())
		if fd.IsMap() {
			fd = fd.MapValue()
			p = append(p, "*")
		} else if fd.IsList() {
			p = append(p, "*")
		}
		switch fd.Kind() {
		case protoreflect.EnumKind:
			ed := fd.Enum()
			values := make([]string, ed.Values().Len())
			for j := range values {
				values[j] = string(ed.Values().Get(j).Name())
			}
			out = append(out, model.GRPCEnum{Path: p, Enum: string(ed.FullName()), Values: values})
		case protoreflect.MessageKind, protoreflect.GroupKind:
			out = appendEnums(out, fd.Message(), p, open)
		}
	}
	return out
}

// wellKnown are the types with their own JSON form instead of an object.
var wellKnown = map[protoreflect.FullName]string{
	"google.protobuf.Timestamp":   `"1970-01-01T00:00:00Z"`,
	"google.protobuf.Duration":    `"0s"`,
	"google.protobuf.FieldMask":   `""`,
	"google.protobuf.Struct":      `{}`,
	"google.protobuf.ListValue":   `[]`,
	"google.protobuf.Value":       `null`,
	"google.protobuf.Empty":       `{}`,
	"google.protobuf.Any":         `null`,
	"google.protobuf.DoubleValue": `0`,
	"google.protobuf.FloatValue":  `0`,
	"google.protobuf.Int32Value":  `0`,
	"google.protobuf.UInt32Value": `0`,
	"google.protobuf.Int64Value":  `"0"`,
	"google.protobuf.UInt64Value": `"0"`,
	"google.protobuf.BoolValue":   `false`,
	"google.protobuf.StringValue": `""`,
	"google.protobuf.BytesValue":  `""`,
}

// writeMessage appends md as JSON. open holds the messages being written
// further up, so a message that contains itself ends in null.
func writeMessage(b *strings.Builder, md protoreflect.MessageDescriptor, indent string, open map[protoreflect.FullName]bool) {
	if v, ok := wellKnown[md.FullName()]; ok {
		b.WriteString(v)
		return
	}
	if open[md.FullName()] {
		b.WriteString("null")
		return
	}
	open[md.FullName()] = true
	defer delete(open, md.FullName())

	inner := indent + "  "
	fields := md.Fields()
	wrote := false
	for i := range fields.Len() {
		fd := fields.Get(i)
		if oneof := fd.ContainingOneof(); oneof != nil && !oneof.IsSynthetic() && oneof.Fields().Get(0) != fd {
			continue
		}
		if wrote {
			b.WriteString(",")
		} else {
			b.WriteString("{")
			wrote = true
		}
		b.WriteString("\n" + inner + strconv.Quote(fd.JSONName()) + ": ")
		// An entry that would be null (a message containing itself, or a
		// type with no example) is not valid inside a list or map, so those
		// are left empty.
		var entry strings.Builder
		switch {
		case fd.IsMap():
			if writeValue(&entry, fd.MapValue(), inner+"  ", open); entry.String() == "null" {
				b.WriteString("{}")
				break
			}
			b.WriteString("{\n" + inner + "  " + mapKey(fd.MapKey()) + ": " + entry.String() + "\n" + inner + "}")
		case fd.IsList():
			if writeValue(&entry, fd, inner+"  ", open); entry.String() == "null" {
				b.WriteString("[]")
				break
			}
			b.WriteString("[\n" + inner + "  " + entry.String() + "\n" + inner + "]")
		default:
			writeValue(b, fd, inner, open)
		}
	}
	if !wrote {
		b.WriteString("{}")
		return
	}
	b.WriteString("\n" + indent + "}")
}

func writeValue(b *strings.Builder, fd protoreflect.FieldDescriptor, indent string, open map[protoreflect.FullName]bool) {
	switch fd.Kind() {
	case protoreflect.BoolKind:
		b.WriteString("false")
	case protoreflect.EnumKind:
		b.WriteString(strconv.Quote(string(fd.Enum().Values().Get(0).Name())))
	case protoreflect.StringKind, protoreflect.BytesKind:
		b.WriteString(`""`)
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Uint64Kind, protoreflect.Fixed64Kind, protoreflect.Sfixed64Kind:
		// 64-bit integers are strings in proto JSON.
		b.WriteString(`"0"`)
	case protoreflect.MessageKind, protoreflect.GroupKind:
		writeMessage(b, fd.Message(), indent, open)
	default:
		b.WriteString("0")
	}
}

// mapKey is an example key; JSON object keys are always strings.
func mapKey(fd protoreflect.FieldDescriptor) string {
	switch fd.Kind() {
	case protoreflect.StringKind:
		return `"key"`
	case protoreflect.BoolKind:
		return `"false"`
	default:
		return `"0"`
	}
}
