package grpcclient

import (
	"context"
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/dynamicpb"

	"roundtrip/internal/model"
)

const templateProto = `syntax = "proto3";
package shop.v1;

import "google/protobuf/timestamp.proto";
import "google/protobuf/duration.proto";
import "google/protobuf/wrappers.proto";
import "google/protobuf/struct.proto";
import "google/protobuf/any.proto";
import "google/protobuf/field_mask.proto";
import "google/protobuf/empty.proto";

enum Status {
  STATUS_UNSPECIFIED = 0;
  STATUS_PAID = 1;
}

message Money {
  string currency = 1;
  int64 units = 2;
}

message Line {
  string sku = 1;
  uint32 quantity = 2;
  Money unit_price = 3;
}

message Category {
  string name = 1;
  Category parent = 2;
  repeated Category children = 3;
}

message Order {
  string id = 1;
  bool gift = 2;
  double weight = 3;
  bytes signature = 4;
  Status status = 5;
  repeated Line lines = 6;
  repeated string tags = 7;
  map<string, Money> totals = 8;
  map<int32, bool> flags = 9;
  oneof payment {
    string card_token = 10;
    Money cash = 11;
  }
  optional string note = 12;
  google.protobuf.Timestamp created_at = 13;
  google.protobuf.Duration ttl = 14;
  google.protobuf.StringValue coupon = 15;
  google.protobuf.Int64Value points = 16;
  google.protobuf.Struct metadata = 17;
  google.protobuf.Value extra = 18;
  google.protobuf.Any attachment = 19;
  google.protobuf.FieldMask mask = 20;
  google.protobuf.Empty nothing = 21;
  Category category = 22;
  fixed64 serial = 23;
}

message Ack {}

service Orders {
  rpc Place(Order) returns (Ack);
  rpc Ping(Ack) returns (Ack);
  rpc Watch(Ack) returns (stream Order);
}
`

func describe(t *testing.T, method string) (model.GRPCMethod, error) {
	t.Helper()
	dir := t.TempDir()
	write(t, dir, map[string]string{"protos/shop/v1/shop.proto": templateProto})
	p := Protos{Config: model.ProtoConfig{Roots: []string{"protos"}}, Workspace: dir}
	return Describe(context.Background(), model.Request{Protocol: "grpc", Method: method}, nil, p)
}

func TestDescribeTemplate(t *testing.T) {
	got, err := describe(t, "shop.v1.Orders/Place")
	if err != nil {
		t.Fatal(err)
	}
	if got.Input != "shop.v1.Order" || got.Output != "shop.v1.Ack" || got.Streaming {
		t.Fatalf("method: %#v", got)
	}

	want := `{
  "id": "",
  "gift": false,
  "weight": 0,
  "signature": "",
  "status": "STATUS_UNSPECIFIED",
  "lines": [
    {
      "sku": "",
      "quantity": 0,
      "unitPrice": {
        "currency": "",
        "units": "0"
      }
    }
  ],
  "tags": [
    ""
  ],
  "totals": {
    "key": {
      "currency": "",
      "units": "0"
    }
  },
  "flags": {
    "0": false
  },
  "cardToken": "",
  "note": "",
  "createdAt": "1970-01-01T00:00:00Z",
  "ttl": "0s",
  "coupon": "",
  "points": "0",
  "metadata": {},
  "extra": null,
  "attachment": null,
  "mask": "",
  "nothing": {},
  "category": {
    "name": "",
    "parent": null,
    "children": []
  },
  "serial": "0"
}`
	if got.Template != want {
		t.Fatalf("template:\n%s\nwant:\n%s", got.Template, want)
	}
}

// Whatever the generator writes must be accepted as the message it describes,
// or "insert example" would hand the user a message that cannot be sent.
func TestTemplateParsesBackIntoItsMessage(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, map[string]string{"protos/shop/v1/shop.proto": templateProto})
	p := Protos{Config: model.ProtoConfig{Roots: []string{"protos"}}, Workspace: dir}
	set, err := p.load(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	svc, err := set.service("shop.v1.Orders")
	if err != nil {
		t.Fatal(err)
	}
	methods := svc.Methods()
	for i := range methods.Len() {
		md := methods.Get(i).Input()
		text := Template(md)
		if err := protojson.Unmarshal([]byte(text), dynamicpb.NewMessage(md)); err != nil {
			t.Errorf("%s: %v\n%s", md.FullName(), err, text)
		}
	}
}

func TestDescribeEmptyMessageAndStreaming(t *testing.T) {
	got, err := describe(t, "/shop.v1.Orders/Ping")
	if err != nil || got.Template != "{}" {
		t.Fatalf("empty message: %#v %v", got, err)
	}
	got, err = describe(t, "shop.v1.Orders/Watch")
	if err != nil || !got.Streaming {
		t.Fatalf("streaming: %#v %v", got, err)
	}
}

func TestDescribeErrors(t *testing.T) {
	for method, want := range map[string]string{
		"Place":                 "must look like",
		"shop.v1.Orders/Nope":   "it defines: ",
		"shop.v1.Missing/Place": "shop.v1.Missing",
	} {
		if _, err := describe(t, method); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: want error containing %q, got %v", method, want, err)
		}
	}
}
