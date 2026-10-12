package launch

import (
	"reflect"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

func TestEffectiveListObservesDeathWithoutWriting(t *testing.T) {
	t.Parallel()
	m, _, probe, _ := manager(t)
	seed(t, m, "dead", Running)
	before, err := m.Store.Read("dead")
	if err != nil {
		t.Fatal(err)
	}
	probe.states[10], probe.states[20] = identity.Dead, identity.Dead
	records, err := m.EffectiveList()
	if err != nil || len(records) != 1 || records[0].State != Failed || records[0].Reason != "supervisor-lost" {
		t.Fatalf("effective records=%+v err=%v", records, err)
	}
	after, err := m.Store.Read("dead")
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("read changed record: %+v %v", after, err)
	}
	probe.states[20] = identity.Alive
	records, err = m.EffectiveList()
	if err != nil || records[0].State != Running {
		t.Fatal(records, err)
	}
}
