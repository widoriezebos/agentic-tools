package testutil

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// scriptedExitProber answers probes from a script: the first answers in
// order, then the last one forever.
type scriptedExitProber struct {
	mu      sync.Mutex
	answers []identity.Exact
	states  []identity.Liveness
	errs    []error
	calls   int
}

func (p *scriptedExitProber) Probe(int64) (identity.Exact, identity.Liveness, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	index := min(p.calls, len(p.states)-1)
	p.calls++
	return p.answers[index], p.states[index], p.errs[index]
}

// TestAwaitExactExitJoinsTheExactIdentity: the join returns once the exact
// identity is gone or its pid belongs to another process, and fails at its
// bound while that same identity lives or its liveness is unknown.
func TestAwaitExactExitJoinsTheExactIdentity(t *testing.T) {
	t.Parallel()
	started := time.Unix(1_700_000_000, 0)
	ref := identity.Exact{Pid: 4242, StartedAt: started}.Ref()
	same := identity.Exact{Pid: 4242, StartedAt: started}
	other := identity.Exact{Pid: 4242, StartedAt: started.Add(time.Hour)}
	for _, test := range []struct {
		name   string
		prober *scriptedExitProber
		fails  string
	}{
		{"exits after a while", &scriptedExitProber{answers: []identity.Exact{same, same, {}}, states: []identity.Liveness{identity.Alive, identity.Alive, identity.Dead}, errs: []error{nil, nil, nil}}, ""},
		{"the pid now belongs to another process", &scriptedExitProber{answers: []identity.Exact{same, other}, states: []identity.Liveness{identity.Alive, identity.Alive}, errs: []error{nil, nil}}, ""},
		{"the same identity outlives the bound", &scriptedExitProber{answers: []identity.Exact{same}, states: []identity.Liveness{identity.Alive}, errs: []error{nil}}, "same-identity=true"},
		{"unknown liveness is not an exit", &scriptedExitProber{answers: []identity.Exact{{}}, states: []identity.Liveness{identity.Unknown}, errs: []error{errors.New("probe denied")}}, "probe denied"},
	} {
		err := awaitExactExitWithin(test.prober, ref, 200*time.Millisecond)
		if test.fails == "" && err != nil || test.fails != "" && (err == nil || !strings.Contains(err.Error(), test.fails)) {
			t.Fatalf("%s: err=%v", test.name, err)
		}
	}
}
