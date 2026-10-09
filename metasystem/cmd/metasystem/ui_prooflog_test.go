package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/httpd"
)

// TestUIProofLogsReadThisComputersLane (fleet-panel-ux step 2, 2a.3): the
// interface's proof-log seam finds an attempt in the records of the lane
// this computer registered, under that lane's installation. With no lane
// registered it serves none; a lane home it cannot find, or a lane record it
// cannot place, is said as such, never as no log.
func TestUIProofLogsReadThisComputersLane(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	root := t.TempDir()
	install := filepath.Join(root, "metasystem")
	proofs := filepath.Join(plain.Dir(install), "proofs")
	if err := os.MkdirAll(proofs, 0o755); err != nil {
		t.Fatal(err)
	}
	result := `{"tree":"t1","commit":"c1","result":"red","log":"` + filepath.Join(proofs, "a1.log") + `","at":"2026-10-03T08:00:00Z","attempt":"a1"}` + "\n"
	if err := os.WriteFile(filepath.Join(plain.Dir(install), "results.jsonl"), []byte(result), 0o644); err != nil {
		t.Fatal(err)
	}
	logs := uiProofLogs(func() (string, error) { return home, nil })

	if _, err := logs("a1"); !errors.Is(err, plain.ErrNoProofLog) || !strings.Contains(err.Error(), "no landing lane is registered on this computer") {
		t.Fatalf("no lane registered: %v", err)
	}

	writeLaneRecord(t, home, `{"root":"`+root+`","install":"`+install+`","custodyEpoch":1,"registeredBy":"wido","at":"2026-10-03T07:00:00Z"}`)
	if got, err := logs("a1"); err != nil || got != filepath.Join(proofs, "a1.log") {
		t.Fatalf("the registered lane's log: %q %v", got, err)
	}

	writeLaneRecord(t, home, `{"root":"`+root+`","install":"/elsewhere","custodyEpoch":1,"registeredBy":"wido","at":"2026-10-03T07:00:00Z"}`)
	if _, err := logs("a1"); err == nil || errors.Is(err, plain.ErrNoProofLog) || !strings.Contains(err.Error(), "this computer's landing lane record can't be read") {
		t.Fatalf("a lane record it cannot place: %v", err)
	}

	writeLaneRecord(t, home, `{"root":"`+root+`","install":"`+install+`","custodyEpoch":"one","registeredBy":"wido","at":"2026-10-03T07:00:00Z"}`)
	if got, err := logs("a1"); err == nil || got != "" || errors.Is(err, plain.ErrNoProofLog) || !strings.Contains(err.Error(), "this computer's landing lane record can't be read") {
		t.Fatalf("a lane record it cannot read, whose paths still place: %q %v", got, err)
	}

	lost := uiProofLogs(func() (string, error) { return "", errors.New("no board home") })
	if _, err := lost("a1"); err == nil || errors.Is(err, plain.ErrNoProofLog) || !strings.Contains(err.Error(), "this computer's landing lane can't be found: no board home") {
		t.Fatalf("a home it cannot find: %v", err)
	}

	if withProofLogs(nil) != nil {
		t.Fatal("a build with no board reader gained one")
	}
	if source := withProofLogs(&httpd.BoardSource{}); source.ProofLog == nil || source.Capacity == nil {
		t.Fatal("the board serves no proof logs or host capacity")
	}
}

func writeLaneRecord(t *testing.T, home, record string) {
	t.Helper()
	if err := os.MkdirAll(lane.HostDir(home), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lane.RecordPath(home), []byte(record), 0o600); err != nil {
		t.Fatal(err)
	}
}
