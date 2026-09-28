package census

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// The live table carries each process's argv as the kernel's vector. A
// quote inside one argument is an ordinary character there: re-tokenizing
// the space-joined string as a shell line would read an apostrophe as an
// unbalanced quote and fail the whole census for every checkout on the host
// (argv-unreadable). An out-of-scope process with such an argument stays
// out of the verdict, and an in-scope path argument containing a quote or a
// space is found whole.
func TestProductionCensusReadsTheArgvVectorNotAShellLine(t *testing.T) {
	t.Parallel()
	bed := supCNewBed(t)
	quoted := filepath.Join(bed.repo, "it's here")
	if err := os.MkdirAll(quoted, 0o755); err != nil {
		t.Fatal(err)
	}
	outside, inside := int64(os.Getpid())+200_001, int64(os.Getpid())+200_002
	exact := func(pid int64, argv ...string) productionProbeResult {
		return productionProbeResult{state: identity.Alive, exact: identity.Exact{
			Pid: pid, StartedAt: supCNow.Add(-time.Minute), Argv: argv, ArgvKnown: true, EnvironKnown: true,
		}}
	}
	source := productionProcessSource{
		pids: func() ([]int64, error) { return []int64{outside, inside}, nil },
		prober: productionProbe{
			outside: exact(outside, "metasystem-fake-agent", "-p", "review the run's reading"),
			inside:  exact(inside, "metasystem-fake-agent", "--workspace", quoted),
		},
		processGroup:  func(pid int) (int, error) { return pid, nil },
		parentProcess: func(int64) (int64, bool) { return 1, true },
	}
	resolve := func(pids []int64) map[int64]cwdResult {
		out := map[int64]cwdResult{}
		for _, pid := range pids {
			out[pid] = cwdResult{Cwd: bed.peer}
		}
		return out
	}
	verdict, err := runCensusVerifying(bed.repo, bed.repo, bed.repo, "fixture-fingerprint", 1, supCNow,
		func(string) ([]Process, error) { return source.enumerate() }, resolve,
		func(map[string]identityRecord, identity.FixtureProbe, *[]string) {})
	if err != nil {
		t.Fatal(err)
	}
	if verdict.Verdict != "SUCCESS" || len(verdict.Errors) != 0 {
		t.Fatalf("census over a quoted argv = %s %v, want SUCCESS", verdict.Verdict, verdict.Errors)
	}
	if len(verdict.Inventory) != 1 || verdict.Inventory[0].Pid != inside || verdict.Inventory[0].Scope != "argv" {
		t.Fatalf("inventory = %+v, want only pid %d in scope by its quoted workspace argument", verdict.Inventory, inside)
	}
}
