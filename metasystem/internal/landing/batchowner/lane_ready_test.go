package batchowner

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
)

// A lane is ready when its checkout has a machine nickname and its
// supervision runs; each missing precondition is its own refusal naming the
// one command that fixes it, the nickname first.
func TestLandingLaneReadyNamesEachMissingPrecondition(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	named := func(string) (string, error) { return "landing", nil }
	unnamed := func(string) (string, error) { return "", errors.New("no machine nickname is enrolled") }
	armed := func(string) (bool, error) { return true, nil }
	unarmed := func(string) (bool, error) { return false, nil }
	var refusal *lane.Refusal
	if err := landingLaneReady(root, unnamed, unarmed); !errors.As(err, &refusal) || refusal.Code != lane.CodeNoMachine ||
		strings.Join(refusal.Argv, " ") != "git -C "+root+" config metasystem.goal.machine landing" {
		t.Fatalf("no nickname = %v", err)
	}
	if err := landingLaneReady(root, named, unarmed); !errors.As(err, &refusal) || refusal.Code != lane.CodeUnarmed ||
		strings.Join(refusal.Argv, " ") != "metasystem system start --repo "+root {
		t.Fatalf("unarmed = %v", err)
	}
	if err := landingLaneReady(root, named, func(string) (bool, error) { return false, errors.New("unreadable") }); err == nil || errors.As(err, &refusal) {
		t.Fatalf("an unknown supervision must be an error, not a refusal: %v", err)
	}
	if err := landingLaneReady(root, named, armed); err != nil {
		t.Fatalf("ready = %v", err)
	}
}

// A checkout whose supervision was never armed, or whose recorded
// supervision owner is gone, is not armed; one nesting the module is read
// in its installation.
func TestLandingLaneArmedReadsTheInstallationsSupervision(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if ok, err := LandingLaneArmed(root); ok || err != nil {
		t.Fatalf("never armed = %v %v", ok, err)
	}
	module := filepath.Join(root, "metasystem")
	lockDir := filepath.Join(module, "artifacts", "agents", "supervision", "lock.d")
	if err := os.MkdirAll(lockDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(module, "go.mod"), []byte("module example\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A dead owner: a pid that cannot be a live process's with this start.
	gone := `{"pid": 999999, "pidStartedAt": 1, "instanceTag": "gone", "fenceGeneration": 0}`
	if err := os.WriteFile(filepath.Join(lockDir, "owner.json"), []byte(gone), 0o644); err != nil {
		t.Fatal(err)
	}
	if ok, err := LandingLaneArmed(root); ok || err != nil {
		t.Fatalf("a gone supervision owner = %v %v", ok, err)
	}
	if err := os.WriteFile(filepath.Join(lockDir, "owner.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if ok, err := LandingLaneArmed(root); ok || err == nil {
		t.Fatalf("an unreadable supervision owner record = %v %v; want an error", ok, err)
	}
}

// Every production reader of the lane asks whether its owner can run: the
// steward's keeper and the board's view (a nil Ready would ask nothing).
func TestProductionLaneReadersAskWhetherTheOwnerCanRun(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	if keeper := landingLaneKeeper(home); keeper.Ready == nil || keeper.Start == nil || keeper.Inspect == nil {
		t.Fatalf("keeper = %+v", keeper)
	}
	if sources := LandingLaneViewSources(home, time.Now()); sources.Ready == nil || sources.Owner == nil {
		t.Fatalf("view sources = %+v", sources)
	}
}
