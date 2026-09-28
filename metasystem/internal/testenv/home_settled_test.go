package testenv

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// snapshotRoot is every path, mode and content digest under root.
func snapshotRoot(t *testing.T, root string) string {
	t.Helper()
	var lines []string
	checkTestenv(t, filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		line := fmt.Sprintf("%s %s", strings.TrimPrefix(path, root), info.Mode())
		if info.Mode().IsRegular() {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			sum := sha256.Sum256(data)
			line += " " + hex.EncodeToString(sum[:])
		}
		lines = append(lines, line)
		return nil
	}))
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

// HomeSettled observes the dead-home predicate and changes nothing (DL2-13):
// a live owner, a running custodian and unsettled records each read not
// settled; a dead, settled home reads settled; the root is byte-identical
// after every observation, and RemoveSettledHome then removes what the old
// sweep removed.
func TestHomeSettledObservesAndRemoveSettledHomeApplies(t *testing.T) {
	root := t.TempDir()

	live, err := createRegistryHomeUnder(os.MkdirTemp, root)
	checkTestenv(t, err)
	t.Cleanup(func() { _ = live.lock.Close() })
	dead := deadRegistryHomeUnder(t, root)
	unsettled := deadRegistryHomeUnder(t, root)
	running := deadRegistryHomeUnder(t, root)

	finishedLog := dead.path + ".custodian-4343.log"
	checkTestenv(t, os.WriteFile(finishedLog, []byte("finished custodian diagnostic\n"), 0o600))
	checkTestenv(t, os.WriteFile(unsettled.path+".fixture-refs-4242", []byte("recorded fixture\n"), 0o600))
	checkTestenv(t, os.WriteFile(unsettled.path+".custodian-4242.log", []byte("exceeded\n"), 0o600))
	runningRecords, runningLog := running.path+".fixture-refs-4444", running.path+".custodian-4444.log"
	checkTestenv(t, os.WriteFile(runningRecords, nil, 0o600))
	stopCustodian := startSidecarCustodian(t, runningLog)

	before := snapshotRoot(t, root)
	for name, want := range map[string]bool{live.path: false, dead.path: true, unsettled.path: false, running.path: false} {
		if got := HomeSettled(name); got != want {
			t.Errorf("HomeSettled(%s) = %v, want %v", filepath.Base(name), got, want)
		}
	}
	if HomeSettled(filepath.Join(root, "absent")) {
		t.Error("an absent home reads settled")
	}
	if after := snapshotRoot(t, root); after != before {
		t.Fatalf("an observation changed the root:\nbefore %s\nafter  %s", before, after)
	}

	var report bytes.Buffer
	for _, home := range []string{live.path, unsettled.path, running.path} {
		checkTestenv(t, RemoveSettledHome(home, &report))
	}
	requireSidecars(t, true, live.path, unsettled.path, running.path, runningRecords, runningLog)
	checkTestenv(t, RemoveSettledHome(dead.path, &report))
	requireSidecars(t, false, dead.path, finishedLog)
	if !strings.Contains(report.String(), "finished custodian diagnostic") {
		t.Fatalf("the settled log was removed without its report: %q", report.String())
	}

	// The running custodian settles: records gone, then it exits.
	checkTestenv(t, os.Remove(runningRecords))
	if HomeSettled(running.path) {
		t.Fatal("a home whose custodian still holds its log reads settled")
	}
	stopCustodian()
	if !HomeSettled(running.path) {
		t.Fatal("a settled home reads unsettled")
	}
	checkTestenv(t, RemoveSettledHome(running.path, &report))
	requireSidecars(t, false, running.path, runningLog)
}
