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

	"golang.org/x/sys/unix"
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
	t.Parallel()
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

func TestHomeSettlementReportsThePendingCondition(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name   string
		change func(*testing.T, string)
		want   string
	}{
		{name: "settled"},
		{name: "owner lock", want: "take registry owner lock", change: func(t *testing.T, home string) {
			owner, err := openRegistryOwner(home)
			checkTestenv(t, err)
			checkTestenv(t, unix.Flock(int(owner.Fd()), unix.LOCK_EX|unix.LOCK_NB))
			t.Cleanup(func() { _ = unlockAndClose(owner) })
		}},
		{name: "missing owner", want: "open registry owner", change: func(t *testing.T, home string) {
			checkTestenv(t, os.Remove(filepath.Join(home, registryOwnerFile)))
		}},
		{name: "absent home", want: "inspect registry home", change: func(t *testing.T, home string) {
			checkTestenv(t, os.RemoveAll(home))
		}},
		{name: "unsafe home", want: "registry home is not an owned directory", change: func(t *testing.T, home string) {
			checkTestenv(t, os.RemoveAll(home))
			checkTestenv(t, os.Symlink(t.TempDir(), home))
		}},
		{name: "log lock", want: "take custodian log lock", change: func(t *testing.T, home string) {
			log, err := os.OpenFile(home+".custodian-4242.log", os.O_CREATE|os.O_RDWR, 0o600)
			checkTestenv(t, err)
			checkTestenv(t, unix.Flock(int(log.Fd()), unix.LOCK_EX|unix.LOCK_NB))
			t.Cleanup(func() { _ = unlockAndClose(log) })
		}},
		{name: "records without log", want: "custodian records remain", change: func(t *testing.T, home string) {
			checkTestenv(t, os.WriteFile(home+".fixture-refs-4242", []byte("synthetic record"), 0o600))
		}},
		{name: "records with unlocked log", want: "custodian records remain", change: func(t *testing.T, home string) {
			checkTestenv(t, os.WriteFile(home+".fixture-refs-4242", []byte("synthetic record"), 0o600))
			checkTestenv(t, os.WriteFile(home+".custodian-4242.log", []byte("cleanup failed"), 0o600))
		}},
		{name: "unsafe log", want: "open custodian log", change: func(t *testing.T, home string) {
			checkTestenv(t, os.Symlink(filepath.Join(home, registryOwnerFile), home+".custodian-4242.log"))
		}},
		{name: "nonregular log", want: "custodian log is not regular", change: func(t *testing.T, home string) {
			checkTestenv(t, os.Mkdir(home+".custodian-4242.log", 0o700))
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			home := deadRegistryHomeUnder(t, t.TempDir()).path
			if test.change != nil {
				test.change(t, home)
			}
			before := snapshotRoot(t, filepath.Dir(home))
			var report bytes.Buffer
			settled := homeSettled(home, &report)
			if settled != (test.want == "") || (test.want == "" && report.Len() != 0) || !strings.Contains(report.String(), test.want) {
				t.Fatalf("settled=%t condition=%q, want %q", settled, report.String(), test.want)
			}
			if test.want != "" && !strings.Contains(report.String(), home) {
				t.Fatalf("condition omitted its registry: %q", report.String())
			}
			if got := HomeSettled(home); got != settled {
				t.Fatalf("HomeSettled=%t, reporting observation=%t", got, settled)
			}
			if after := snapshotRoot(t, filepath.Dir(home)); after != before {
				t.Fatalf("settlement observation changed the fixture:\nbefore %s\nafter %s", before, after)
			}
		})
	}
}
