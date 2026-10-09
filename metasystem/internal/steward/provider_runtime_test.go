package steward

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/outage"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testprovider"
)

const providerRuntimeSettings = "launch.seat.runtime=codex\nmetasystem.runtimes=claude,codex\nrole.steward-continuation.runtime=claude\nrole.steward-continuation.model.claude=fixture-model\n"

func TestFleetProviderRuntimeSeatAndRevival(t *testing.T) {
	t.Parallel()
	for _, runtime := range []string{"codex", "claude"} {
		t.Run(runtime, func(t *testing.T) {
			t.Parallel()
			bed := newSeatBed(t, seatReadyGoal("alpha", "Build it."))
			bed.now = time.Date(2026, 10, 8, 10, 0, 0, 0, time.UTC)
			if err := os.WriteFile(filepath.Join(bed.root, "metasystem.conf"), []byte(providerRuntimeSettings), 0o600); err != nil {
				t.Fatal(err)
			}
			selection := bed.tick(deadWorkers).Seat
			if selection == nil {
				t.Fatal("a healthy codex seat must be selected")
			}
			if _, err := outage.Observe(testprovider.Home(bed.root), runtime, "fixture-model", outage.ProviderLimit, "HTTP 429 Too Many Requests", "limited-call", bed.now); err != nil {
				t.Fatal(err)
			}
			result := bed.tick(deadWorkers)
			if result.ProviderOutage != (runtime == "codex") || (result.Seat == nil) != (runtime == "codex") {
				t.Fatalf("the codex seat must check only its provider: %+v", result)
			}
			record, err := bed.startUnder(*selection, deadCensus())
			if runtime == "codex" {
				if err == nil || record.LaunchID != "" || len(bed.launcher.starts) != 0 {
					t.Fatalf("the locked recheck launched into a codex outage: %+v %v", record, err)
				}
			} else if err != nil || record.LaunchID == "" || len(bed.launcher.starts) != 1 {
				t.Fatalf("a claude outage held a codex seat: %+v %v", record, err)
			}

			// A retained authorization selects its runtime even when the seat
			// settings and the current continuation roster name another runtime.
			revivalRoot := t.TempDir()
			writeLedger(t, revivalRoot, "# Goals\n\n## Current goal: fix-it — Repair the thing\n- Origin: main\n- Next step: Repair it.\n")
			if err := os.WriteFile(filepath.Join(revivalRoot, "metasystem.conf"), []byte(strings.ReplaceAll(providerRuntimeSettings, "role.steward-continuation.runtime=claude", "role.steward-continuation.runtime=main")), 0o600); err != nil {
				t.Fatal(err)
			}
			intent := testIntent("7000000000000001")
			intent.Runtime = "claude"
			if err := PrepareIntent(revivalRoot, filepath.Join(revivalRoot, "receipt.log"), intent); err != nil {
				t.Fatal(err)
			}
			launched := false
			outcome, err := CompleteRevival(revivalRoot, TickConfig{Now: bed.now, ProviderHome: testprovider.Home(bed.root)}, deadCensus(), intent.Nonce, func(Intent) error {
				launched = true
				return nil
			}, nil)
			if err != nil || outcome.Launched != (runtime == "codex") || launched != outcome.Launched {
				t.Fatalf("the recorded claude revival must check only its provider: %+v %v launched=%v", outcome, err, launched)
			}
		})
	}
}

func TestFleetProviderRuntimeBrokenRosterDoesNotHoldSeat(t *testing.T) {
	t.Parallel()
	bed := newSeatBed(t, seatReadyGoal("alpha", "Build it."))
	if err := os.WriteFile(filepath.Join(bed.root, "metasystem.conf"), []byte(strings.ReplaceAll(providerRuntimeSettings, "role.steward-continuation.runtime=claude", "role.steward-continuation.runtime=main")), 0o600); err != nil {
		t.Fatal(err)
	}
	result := bed.tick(deadWorkers)
	if result.ProviderOutage || result.Seat == nil {
		t.Fatalf("a broken continuation roster must not hold a codex seat: %+v", result)
	}
	if record, err := bed.startUnder(*result.Seat, deadCensus()); err != nil || record.LaunchID == "" {
		t.Fatalf("the locked seat start consulted the continuation roster: %+v %v", record, err)
	}
}

func TestFleetProviderRuntimeTickRevival(t *testing.T) {
	t.Parallel()
	for _, runtime := range []string{"codex", "claude", "broken roster", "retained intent"} {
		t.Run(runtime, func(t *testing.T) {
			t.Parallel()
			bed := newDecisionTickRepository(t)
			settings := providerRuntimeSettings
			if runtime == "broken roster" || runtime == "retained intent" {
				settings = strings.ReplaceAll(settings, "role.steward-continuation.runtime=claude", "role.steward-continuation.runtime=main")
			}
			if err := os.WriteFile(filepath.Join(bed.root, "metasystem.conf"), []byte(settings), 0o600); err != nil {
				t.Fatal(err)
			}
			now := time.Date(2026, 10, 8, 10, 0, 0, 0, time.UTC)
			if runtime == "codex" || runtime == "claude" || runtime == "retained intent" {
				marked := runtime
				if runtime == "retained intent" {
					marked = "claude"
					intent := testIntent("7000000000000002")
					intent.Runtime = "claude"
					if err := MintIntent(bed.root, intent); err != nil {
						t.Fatal(err)
					}
				}
				if _, err := outage.Observe(testprovider.Home(bed.root), marked, "fixture-model", outage.ProviderLimit, "HTTP 429 Too Many Requests", "limited-call", now); err != nil {
					t.Fatal(err)
				}
			}
			result := bed.tickN(TickConfig{Now: now}, deadCensus(), 1)
			if result.ProviderOutage != (runtime == "claude" || runtime == "retained intent") || (result.Decision.Action == ActRevive) != (runtime == "codex") {
				t.Fatalf("the revival tick must select its actual runtime: %+v", result)
			}
			if runtime == "broken roster" {
				if result.Decision.Verdict != VerdictDegraded || result.Decision.Action != ActNotify ||
					!strings.Contains(result.Decision.Reason, "assigned to main") ||
					!strings.Contains(result.Decision.Reason, "repair role.steward-continuation in metasystem.conf") ||
					strings.Contains(result.Decision.Reason, "overloaded") {
					t.Fatalf("a revival that needs the broken roster must name its error and repair: %+v", result)
				}
				result = bed.tickN(TickConfig{Now: now}, deadCensus(), 1)
				if result.ProviderOutage || result.Evidence.TicksSinceAdvance != 1 {
					t.Fatalf("a broken roster must not freeze progress aging: %+v", result)
				}
				pending, err := PendingNotifications(bed.root)
				if err != nil || len(pending) != 1 || !strings.Contains(pending[0].Message, "assigned to main") ||
					!strings.Contains(pending[0].Message, "repair role.steward-continuation in metasystem.conf") ||
					strings.Contains(pending[0].Message, "overloaded") {
					t.Fatalf("the durable notice must explain the roster repair: %+v %v", pending, err)
				}
			}
			if runtime == "retained intent" && result.Outage.LastClass == "unknown" {
				t.Fatalf("a retained intent must not resolve the current roster: %+v", result)
			}
		})
	}
}

func TestFleetProviderRuntimeBrokenRosterPreservesOrdinaryDecision(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		workers Workers
		free    bool
	}{
		{name: "live worker", workers: Workers{Live: 1, CensusComplete: true}},
		{name: "unprovable worker", workers: Workers{Unprovable: 1, CensusComplete: true}},
		{name: "no work", workers: Workers{CensusComplete: true}, free: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			bed := newDecisionTickRepository(t)
			if tc.free {
				bed.declareGoalFree()
			}
			cfg := TickConfig{Now: time.Date(2026, 10, 8, 10, 0, 0, 0, time.UTC)}
			census := fakeCensus{workers: tc.workers}
			if err := os.WriteFile(filepath.Join(bed.root, "metasystem.conf"), []byte(providerRuntimeSettings), 0o600); err != nil {
				t.Fatal(err)
			}
			ordinary := bed.tickN(cfg, census, 1)
			if ordinary.Decision.Action == ActRevive || ordinary.Decision.Verdict == VerdictDegraded {
				t.Fatalf("the healthy roster must produce an ordinary decision: %+v", ordinary)
			}
			settings := strings.ReplaceAll(providerRuntimeSettings, "role.steward-continuation.runtime=claude", "role.steward-continuation.runtime=main")
			if err := os.WriteFile(filepath.Join(bed.root, "metasystem.conf"), []byte(settings), 0o600); err != nil {
				t.Fatal(err)
			}
			result := bed.tickN(cfg, census, 1)
			if result.Decision != ordinary.Decision || result.ProviderOutage || result.Outage != (outage.Mark{}) {
				t.Fatalf("a broken continuation roster must preserve the ordinary decision: got %+v, want %+v", result, ordinary.Decision)
			}
			if result.Evidence.TicksSinceAdvance != ordinary.Evidence.TicksSinceAdvance+1 {
				t.Fatalf("a broken roster must not freeze progress aging: before %+v, after %+v", ordinary.Evidence, result.Evidence)
			}
		})
	}
}

func TestFleetProviderRuntimePublicTick(t *testing.T) {
	t.Parallel()
	for _, runtime := range []string{"codex", "claude"} {
		t.Run(runtime, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			home := testprovider.Register(t, root)
			writeLedger(t, root, "# Goals\n\n## Current goal: fix-it — Repair the thing\n- Origin: main\n- Next step: Repair it.\n")
			if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte(providerRuntimeSettings), 0o600); err != nil {
				t.Fatal(err)
			}
			now := time.Date(2026, 10, 8, 10, 0, 0, 0, time.UTC)
			if _, err := outage.Observe(home, runtime, "fixture-model", outage.ProviderLimit, "HTTP 429 Too Many Requests", "limited-call", now); err != nil {
				t.Fatal(err)
			}
			result, err := RunTick(root, TickConfig{Now: now, ProviderHome: home, WorkStateRoot: t.TempDir()}, deadCensus())
			if err != nil || result.ProviderOutage != (runtime == "claude") || (result.Decision.Action == ActRevive) != (runtime == "codex") {
				t.Fatalf("public tick must check the continuation runtime: %+v %v", result, err)
			}
			retained, err := LoadEvidence(EvidencePath(root))
			if err != nil || retained != result.Evidence {
				t.Fatalf("public tick must persist the decision's evidence: %+v %v", retained, err)
			}
		})
	}
}

func TestFleetProviderRuntimeTickUsesOneObservation(t *testing.T) {
	t.Parallel()
	bed := newSeatBed(t, seatReadyGoal("alpha", "Build it."))
	if err := os.WriteFile(filepath.Join(bed.root, "metasystem.conf"), []byte(providerRuntimeSettings), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := outage.Observe(testprovider.Home(bed.root), "codex", "fixture-model", outage.ProviderLimit, "HTTP 429 Too Many Requests", "limited-call", bed.now); err != nil {
		t.Fatal(err)
	}
	deps := bed.dependencies()
	read := deps.Seat.Gate
	deps.Seat.Gate = func(root string) (goal.GateSettings, error) {
		// The host read precedes the seat ladder. A changed file cannot
		// change the mark or explanation within that same tick.
		if err := os.WriteFile(testprovider.Path(bed.root), []byte("{unreadable"), 0o600); err != nil {
			t.Fatal(err)
		}
		return read(root)
	}
	result, err := decideTickWithDependencies(bed.root, TickConfig{Now: bed.now, ProviderHome: testprovider.Home(bed.root)}, deadCensus(), Evidence{}, Marks{}, deps)
	if err != nil || !result.ProviderOutage || result.Outage.LastClass != outage.ProviderLimit || !strings.Contains(result.Decision.Reason, "overloaded or limited") || strings.Contains(result.Decision.Reason, "unreadable") {
		t.Fatalf("a tick must use one provider observation for the hold and its reason: %+v %v", result, err)
	}
}

func TestFleetProviderRuntimePublicHandoff(t *testing.T) {
	t.Parallel()
	for _, runtime := range []string{"codex", "claude"} {
		t.Run(runtime, func(t *testing.T) {
			t.Parallel()
			root, intent := stagedRevivalHandoff(t, "7000000000000003")
			intent.Runtime = "codex"
			if intent.Handoff.Runtime != "claude" {
				t.Fatalf("the predecessor binding must differ from the codex continuation: %+v", intent.Handoff)
			}
			if err := PrepareIntent(root, filepath.Join(root, "receipt.log"), intent); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte(providerRuntimeSettings), 0o600); err != nil {
				t.Fatal(err)
			}
			now := time.Date(2026, 10, 8, 10, 0, 0, 0, time.UTC)
			home := testprovider.Home(root)
			if _, err := outage.Observe(home, runtime, "fixture-model", outage.ProviderLimit, "HTTP 429 Too Many Requests", "limited-call", now); err != nil {
				t.Fatal(err)
			}
			launched := false
			outcome, err := CompleteRevival(root, TickConfig{Now: now, ProviderHome: home}, deadCensus(), intent.Nonce, func(authorized Intent) error {
				if authorized.Runtime != intent.Runtime {
					t.Fatalf("the launch must use the intent's runtime: %+v", authorized)
				}
				launched = true
				return nil
			}, nil)
			if err != nil || launched != (runtime == "claude") || outcome.Launched != launched || outcome.Held != (runtime == "codex") {
				t.Fatalf("handoff must check the intent's codex runtime: %+v %v launched=%v", outcome, err, launched)
			}
			live, err := LiveIntents(root)
			if err != nil || (len(live) == 1) != outcome.Held {
				t.Fatalf("a provider hold must preserve the handoff: %+v %v", live, err)
			}
		})
	}
}
