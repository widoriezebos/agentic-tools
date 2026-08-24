package delegate_test

// The seam owns the contract; the protocol package conforms to it.
// These pins hold the two vocabularies together IN BOTH DIRECTIONS:
// a row, an envelope field, or an option field added on either side
// without the other fails here, not in a live turn.

import (
	"reflect"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/acp"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/delegate"
)

// The row vocabularies are ONE set, compared both ways through each
// side's own enumerator — a row added to either list alone fails.
func TestRowParityWithACP(t *testing.T) {
	seam := map[string]bool{}
	for _, row := range delegate.Rows() {
		seam[string(row)] = true
	}
	wire := map[string]bool{}
	for _, row := range acp.Rows() {
		wire[string(row)] = true
	}
	for row := range seam {
		if !wire[row] {
			t.Fatalf("seam row %q has no acp counterpart", row)
		}
	}
	for row := range wire {
		if !seam[row] {
			t.Fatalf("acp row %q has no seam counterpart", row)
		}
	}
}

// The envelope structs match field-for-field by name and type in
// both directions, checked by reflection so an addition on either
// side fails without anyone remembering to update a literal.
func TestEnvelopeParityWithACP(t *testing.T) {
	assertSameFields(t, reflect.TypeOf(delegate.Envelope{}), reflect.TypeOf(acp.Envelope{}))
}

// The ask option mirrors the protocol's permission option the same
// way.
func TestAskOptionParityWithACP(t *testing.T) {
	assertSameFields(t, reflect.TypeOf(delegate.AskOption{}), reflect.TypeOf(acp.PermissionOption{}))
}

func assertSameFields(t *testing.T, a, b reflect.Type) {
	t.Helper()
	if a.NumField() != b.NumField() {
		t.Fatalf("%s has %d fields, %s has %d — the shapes must move together",
			a, a.NumField(), b, b.NumField())
	}
	bFields := map[string]reflect.Type{}
	for i := 0; i < b.NumField(); i++ {
		bFields[b.Field(i).Name] = b.Field(i).Type
	}
	for i := 0; i < a.NumField(); i++ {
		field := a.Field(i)
		other, ok := bFields[field.Name]
		if !ok {
			t.Fatalf("%s.%s has no counterpart in %s", a, field.Name, b)
		}
		if other != field.Type {
			t.Fatalf("%s.%s is %s but %s's is %s", a, field.Name, field.Type, b, other)
		}
	}
}
