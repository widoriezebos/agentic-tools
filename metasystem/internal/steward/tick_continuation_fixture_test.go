package steward

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/spend"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

type tickContinuationFixture struct {
	bed          *attentionPolicyBed
	cfg          TickConfig
	dependencies tickContinuationDependencies
	writes       int
	markReads    int
	machineReads int
	healthReads  int
	checkWork    func()
}

func newTickContinuationFixture(t *testing.T, bed *attentionPolicyBed, refuseBaseline bool) *tickContinuationFixture {
	t.Helper()
	f := &tickContinuationFixture{bed: bed, cfg: (TickConfig{Now: bed.now}).withDefaults()}
	decision := &decisionTickRepository{t: t, root: bed.root, head: strings.Repeat("a", 40), files: bed.worlds["base"]}
	f.dependencies.ledgerRepository = bed.repository()
	f.dependencies.ledgerWriter = func(path, contents, anchor string) (bool, error) {
		f.writes++
		if path != ledgerAttentionStatePath(bed.root) || anchor != bed.root {
			t.Fatalf("ledger writer path=%q anchor=%q", path, anchor)
		}
		if refuseBaseline {
			if f.writes != 1 {
				t.Fatalf("ledger baseline writer called %d times", f.writes)
			}
			return false, nil
		}
		return atomicfile.WriteText(path, contents, anchor)
	}
	f.dependencies.readMarks = func(root string, args ...string) ([]byte, error) {
		want := [][]string{{"rev-parse", "HEAD"}, {"rev-parse", "--verify", "--quiet", "refs/metasystem/goals/accepted"}}
		values := []string{decision.head, decision.acceptedTip()}
		if root != bed.root || f.markReads >= len(want) || !reflect.DeepEqual(args, want[f.markReads]) {
			t.Fatalf("unexpected mark read: root=%q args=%v index=%d", root, args, f.markReads)
		}
		answer := values[f.markReads]
		f.markReads++
		return []byte(answer + "\n"), nil
	}
	f.dependencies.openWork, f.checkWork = decision.openWorkDependencies()
	f.dependencies.resolveMachine = func(root string) (string, error) {
		if root != bed.root {
			t.Fatalf("machine root = %q, want %q", root, bed.root)
		}
		f.machineReads++
		return "mac-a", nil
	}
	f.dependencies.resolveLayout = func(root string) (stateroot.Layout, error) {
		if root != bed.root {
			t.Fatalf("digest layout root = %q, want %q", root, bed.root)
		}
		return stateroot.Layout{GitRoot: root, RepositoryRoot: root, InstallationRoot: root}, nil
	}
	stable := tickHealthRoles(t, bed.root, "bed-m1", func(string, string, time.Time) (spend.Ledger, error) {
		return fixtureSpendLedger(), nil
	})
	f.dependencies.health = tickHealthDependencies{
		evaluate: func(root, metasystemRoot string, now time.Time, prober identity.Prober, preview bool) ([]RoleVerdict, SpendObservation) {
			f.healthReads++
			if root != bed.root || metasystemRoot != bed.root || preview {
				t.Fatalf("health inputs: root=%q metasystem=%q preview=%t", root, metasystemRoot, preview)
			}
			record, err := loadComponentEvidence(ComponentEvidencePath(root, "ledger-attention"))
			if err != nil {
				t.Fatal(err)
			}
			wantOutcome := "PASS_COMPLETE"
			if refuseBaseline {
				wantOutcome = ledgerAttentionStateWriteFailed
			}
			if record.Result != map[bool]ComponentResult{true: ComponentError, false: ComponentOK}[refuseBaseline] || record.Outcome != wantOutcome {
				t.Fatalf("health ran before durable ledger completion: %+v", record)
			}
			return stable(root, metasystemRoot, now, prober, preview)
		},
		now:     f.cfg.now,
		deliver: func(string, string) error { return fmt.Errorf("unexpected health delivery") },
	}
	return f
}

func (f *tickContinuationFixture) run(t *testing.T) (TickResult, bool, error) {
	t.Helper()
	lock, err := AcquireArbitration(f.bed.root)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Release()
	exact, state, err := (identity.KernelProber{}).Probe(int64(os.Getpid()))
	if err != nil || state != identity.Alive {
		t.Fatalf("self process identity: state=%v err=%v", state, err)
	}
	const generation = 1
	attempt, err := beginComponentAttempt(f.bed.root, "steward-tick", generation, exact.Ref(), f.cfg.now())
	if err != nil {
		t.Fatal(err)
	}
	return runTickAfterCustodial(f.bed.root, f.cfg, fakeCensus{}, generation, exact.Ref(), attempt.AttemptSeq, nil, f.dependencies)
}

func TestTickAfterCustodialDegradedReturnRemainsIncomplete(t *testing.T) {
	bed := newAttentionPolicyBed(t)
	f := newTickContinuationFixture(t, bed, false)
	path := EvidencePath(bed.root)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{torn"), 0o644); err != nil {
		t.Fatal(err)
	}
	result, completed, err := f.run(t)
	if err != nil || completed || result.Decision.Verdict != VerdictDegraded {
		t.Fatalf("torn evidence continuation: result=%+v completed=%t err=%v", result, completed, err)
	}
	if want := strings.Fields(attentionBaselineCalls()); !reflect.DeepEqual(bed.calls, want) {
		t.Fatalf("ledger pass calls = %v, want %v", bed.calls, want)
	}
	ledger, ledgerErr := loadComponentEvidence(ComponentEvidencePath(bed.root, "ledger-attention"))
	tick, tickErr := loadComponentEvidence(ComponentEvidencePath(bed.root, "steward-tick"))
	if ledgerErr != nil || ledger.Result != ComponentOK || ledger.Outcome != "PASS_COMPLETE" || tickErr != nil || tick.Result == ComponentOK || !tick.LastSuccess.IsZero() {
		t.Fatalf("degraded return claimed successful tick: ledger=%+v ledgerErr=%v tick=%+v tickErr=%v", ledger, ledgerErr, tick, tickErr)
	}
	if f.markReads != 0 || f.healthReads != 0 {
		t.Fatalf("duties ran after torn evidence: marks=%d health=%d", f.markReads, f.healthReads)
	}
}
