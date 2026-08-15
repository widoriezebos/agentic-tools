package host

import (
	"reflect"
	"strings"
	"testing"
)

// The recollection table: devin registers seam-locally, unknown
// runtimes return nil (the caller's no-resume signal), the list view
// serves conformance, and registration guards reject nil/duplicates.
func TestRecollectionTable(t *testing.T) {
	if DeliveryRecollector("ghostrt") != nil {
		t.Fatal("an undeclared runtime returned a recollector")
	}
	if got := DeliveryRecollectorList(); !reflect.DeepEqual(got, []string{"devin"}) {
		t.Fatalf("recollector list wrong: %v", got)
	}
	// The devin recollector runs against an empty turn dir and reports
	// the collect ladder's own failure — proof the registered function
	// is the real collector, not a stub.
	recollect := DeliveryRecollector("devin")
	if recollect == nil {
		t.Fatal("devin recollector missing")
	}
	dir := t.TempDir()
	result, err := recollect(RecollectParams{Root: dir, TurnRecordPath: dir + "/turn.json", TurnDir: dir, Workspace: dir})
	if err == nil && result.Delivered {
		t.Fatalf("an empty turn dir delivered: %+v", result)
	}
}

func TestRecollectorRegistrationGuards(t *testing.T) {
	expectPanic := func(name string, f func()) {
		defer func() {
			if r := recover(); r == nil || !strings.Contains(r.(string), "recollector") {
				t.Fatalf("%s did not panic usefully: %v", name, r)
			}
		}()
		f()
	}
	expectPanic("nil recollector", func() { RegisterDeliveryRecollector("x-nil", nil) })
	expectPanic("duplicate recollector", func() {
		RegisterDeliveryRecollector("devin", func(RecollectParams) (RecollectResult, error) { return RecollectResult{}, nil })
	})
}
