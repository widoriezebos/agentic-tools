package steward

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
	"golang.org/x/sys/unix"
)

var alertClearBedRoot = flag.String("alert-clear-bed-root", "", "isolated alert clear health bed")
var alertClearBedAction = flag.String("alert-clear-bed-action", "tick", "tick, preview, restart or concurrent")
var alertClearBedOutput = flag.String("alert-clear-bed-output", "", "health verdict output")
var alertClearBedCount = flag.Int("alert-clear-bed-count", 1, "observations at the injected instant")

// The command child runs the tick's health pass with real record writers,
// checks and remedies. Its runtime login fails at the prober boundary;
// ledger examination sees the actual state, without a repository diff.
func TestAlertClearHealthBed(t *testing.T) {
	t.Parallel()
	root := *alertClearBedRoot
	if root == "" {
		root = canonicalPath(t.TempDir())
	}
	b := newHealthBedAt(t, root, EnrollmentFixture, "")
	b.writeFile("metasystem.conf", []byte("metasystem.runtimes=claude\nsteward.ledger-attention-stale-minutes=20\n"))
	b.lookPath = func(string) (string, error) { return "/fixture/claude", nil }
	evaluate := func(root, installation string, at time.Time, probe identity.Prober, hook bool) ([]RoleVerdict, SpendObservation) {
		roles, spend := b.evaluate(root, installation, at, probe, hook)
		for i, role := range roles {
			if role.Role == RoleLedgerAttention {
				roles[i] = checkLedgerAttention(root, at)
			}
		}
		return roles, spend
	}
	if _, exists, err := loadLedgerAttentionState(root); err != nil {
		t.Fatal(err)
	} else if !exists {
		if err := saveLedgerAttentionState(root, ledgerAttentionState{RemoteTip: "remote-tip", DiffedTip: "examined-tip", ExaminedTip: "examined-tip", MovedAt: b.base.Add(-time.Hour).Format(time.RFC3339), LastFailure: "fixture fetch failed"}); err != nil {
			t.Fatal(err)
		}
	}
	if *alertClearBedAction == "preview" {
		verdict := previewHealthAtWithEvaluation(root, root, b.base, b.probe, evaluate)
		data, _ := json.Marshal(verdict)
		if err := os.WriteFile(*alertClearBedOutput, data, 0600); err != nil {
			t.Fatal(err)
		}
		return
	}
	if *alertClearBedAction == "restart" {
		if err := examineLedgerMove(root, b.base); err != nil {
			t.Fatalf("startup must preserve the acknowledged tip: %v", err)
		}
	}
	arbitration, err := AcquireArbitration(root)
	if err != nil {
		t.Fatal(err)
	}
	defer arbitration.Release()
	if *alertClearBedAction == "concurrent" {
		fmt.Println("tick holds arbitration")
		testenv.Await(t, "the clear's queued arbitration hold", func() bool {
			file, err := os.OpenFile(arbitrationWantPath(root), os.O_RDONLY, 0)
			if err == nil {
				err = unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB)
				unlockAndClose(file)
				if errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN) {
					return true
				}
			}
			return false
		})
	}
	deps := tickHealthDependencies{evaluate: evaluate, now: func() time.Time { return b.base }, lookPath: b.lookPath,
		deliver: func(string, string) error { return nil }, examineLedger: func(root string, now time.Time) error {
			return examineLedgerMoveWithRepositoryAndWriter(root, now, defaultLedgerAttentionRepository(), atomicfile.WriteText)
		},
		probeRuntime: func(string, string) error { return errors.New("fixture login unavailable") },
	}
	var result TickResult
	for i := 0; i < *alertClearBedCount; i++ {
		if err := completeTickHealthWithDependencies(root, &result, b.generation, b.runner, b.base, deps); err != nil {
			t.Fatal(err)
		}
	}
	record, err := loadComponentEvidence(ComponentEvidencePath(root, "ledger-examination"))
	if err != nil || record.Outcome != "FAILED" || (!strings.Contains(record.LastFailure, "nothing to examine: remote remote-tip, examined examined-tip") || !strings.Contains(record.LastFailure, "fixture fetch failed")) {
		t.Fatalf("no-op examination must fail and name both tips: %+v %v", record, err)
	}
}
