package goal

import (
	"path/filepath"
	"reflect"
	"testing"
	"time"
	"unsafe"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
)

// testTemporaryGoalProof is confined to test code so engine tests can drive
// an already-granted proof without giving production callers an injected
// authority clock.
func testTemporaryGoalProof(t *testing.T, root, word, reviewBy string) humanauthority.Proof {
	t.Helper()
	abs, err := filepath.Abs(root)
	if err != nil {
		t.Fatal(err)
	}
	proof := humanauthority.Proof{
		Schema: 1, CheckedAt: time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC),
		Outcome: humanauthority.OutcomeTemporary, TemporaryHumanWord: word,
		ReviewBy: reviewBy, Departure: humanauthority.TemporaryWordRuling,
	}
	value := reflect.ValueOf(&proof).Elem()
	setPrivateTestField(value.FieldByName("observedRoot"), reflect.ValueOf(filepath.Clean(abs)))
	setPrivateTestField(value.FieldByName("observed"), reflect.ValueOf(true))
	return proof
}

func setPrivateTestField(field, value reflect.Value) {
	reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem().Set(value)
}
