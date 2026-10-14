package model

import (
	"reflect"
	"strings"
	"testing"
)

// The types written to workspace files must name their fields the same in
// YAML as in the JSON API, and omit the same empty ones.
func TestYAMLTagsMirrorJSONTags(t *testing.T) {
	for _, v := range []any{KV{}, Auth{}, Body{}, Request{}, Assertion{}, Extract{}, ProtoConfig{}} {
		typ := reflect.TypeOf(v)
		for i := range typ.NumField() {
			f := typ.Field(i)
			j := strings.Replace(f.Tag.Get("json"), "omitzero", "omitempty", 1)
			if y := f.Tag.Get("yaml"); y != j || y == "" {
				t.Errorf("%s.%s: json %q, yaml %q", typ.Name(), f.Name, f.Tag.Get("json"), y)
			}
		}
	}
}
