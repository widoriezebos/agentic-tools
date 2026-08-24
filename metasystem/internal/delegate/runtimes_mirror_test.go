package delegate

import (
	"reflect"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/runtimes"
)

// The runtimes registry mirrors this package's Declaration as pure
// data (runtimes.ACPCapabilities) so it can stay a dependency leaf.
// The duplication is deliberate — registry expectation vs driver
// claim, joined at registration — and THIS pin is what makes the
// duplication safe: the two structs must carry exactly the same
// field names and types, both ways.
func TestRuntimesCapabilityMirrorParity(t *testing.T) {
	decl := reflect.TypeOf(Declaration{})
	mirror := reflect.TypeOf(runtimes.ACPCapabilities{})
	declFields := map[string]reflect.Type{}
	for i := 0; i < decl.NumField(); i++ {
		declFields[decl.Field(i).Name] = decl.Field(i).Type
	}
	mirrorFields := map[string]reflect.Type{}
	for i := 0; i < mirror.NumField(); i++ {
		mirrorFields[mirror.Field(i).Name] = mirror.Field(i).Type
	}
	for name, typ := range declFields {
		got, ok := mirrorFields[name]
		if !ok {
			t.Errorf("runtimes.ACPCapabilities lacks %s", name)
		} else if got != typ {
			t.Errorf("field %s: delegate %v vs runtimes %v", name, typ, got)
		}
	}
	for name := range mirrorFields {
		if _, ok := declFields[name]; !ok {
			t.Errorf("runtimes.ACPCapabilities carries %s that delegate.Declaration lacks", name)
		}
	}
}
