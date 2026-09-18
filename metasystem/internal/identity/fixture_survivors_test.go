package identity

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

type fixtureScanProber struct {
	exacts  map[int64]Exact
	unknown map[int64]bool
}

func (prober fixtureScanProber) Probe(pid int64) (Exact, Liveness, error) {
	if prober.unknown[pid] {
		return Exact{}, Unknown, errors.New("fixture probe denied")
	}
	if exact, ok := prober.exacts[pid]; ok {
		return exact, Alive, nil
	}
	return Exact{}, Dead, nil
}

type fixtureTable map[int64]Exact

func (table fixtureTable) Probe(pid int64) (Exact, Liveness, error) {
	if exact, ok := table[pid]; ok {
		return exact, Alive, nil
	}
	return Exact{}, Dead, nil
}

func fixtureExact(pid, token int64) Exact {
	exact := Exact{Pid: pid, StartedAt: time.Unix(100, token)}
	if runtime.GOOS == "linux" {
		exact.StartTicks, exact.BootID = token, "fixture-boot"
	}
	return exact
}

func fixtureWord(t *testing.T, key FixtureKey) string {
	encoded, err := EncodeKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return fixtureOwnerPrefix + encoded
}

func installFixtureScanTable(t *testing.T, table fixtureTable) {
	oldPids, oldProber, oldScope := survivorPids, fixtureSurvivorProber, fixtureSurvivorScope
	survivorPids = func() ([]int64, error) {
		pids := make([]int64, 0, len(table))
		for pid := range table {
			pids = append(pids, pid)
		}
		return pids, nil
	}
	fixtureSurvivorProber = table
	fixtureSurvivorScope = func(pid int64) FixtureProcessScope {
		return FixtureProcessScope{Pgid: 900, Sid: 901, Signalable: true}
	}
	t.Cleanup(func() { survivorPids, fixtureSurvivorProber, fixtureSurvivorScope = oldPids, oldProber, oldScope })
}

func TestCleanupIsScopedToTheFixtureKey(t *testing.T) {
	owner := fixtureExact(700, 70).Ref()
	keyA := FixtureKey{Owner: owner, Test: "TestOne", Nonce: "00000001"}
	keyB := FixtureKey{Owner: owner, Test: "TestTwo", Nonce: "00000002"}
	childA := fixtureExact(701, 71)
	childA.Argv, childA.ArgvKnown = []string{"/bin/sh", fixtureWord(t, keyA)}, true
	childB := fixtureExact(702, 72)
	childB.Environ, childB.EnvironKnown = []string{fixtureWord(t, keyB)}, true
	table := fixtureTable{700: fixtureExact(700, 70), 701: childA, 702: childB}
	installFixtureScanTable(t, table)
	got, err := FixtureSurvivors(keyA)
	if err != nil || len(got) != 1 || got[0].Ref.Pid != 701 {
		t.Fatalf("FixtureSurvivors(key A) = %#v, %v; want child A only", got, err)
	}
	got, err = FixtureSurvivorsOfDeadOwner(table, owner)
	if err == nil || got != nil {
		t.Fatalf("live-owner scan = %#v, %v; want no results and an error", got, err)
	}
	got, err = FixtureSurvivorsOfDeadOwner(fakeProber{state: Unknown}, owner)
	if err == nil || got != nil {
		t.Fatalf("unknown-owner scan = %#v, %v; want no results and an error", got, err)
	}
	delete(table, 700)
	got, err = FixtureSurvivorsOfDeadOwner(table, owner)
	if err != nil || len(got) != 2 || got[0].Ref.Pid != 701 || got[0].Carrier != FixtureCarrierArgvWord ||
		got[1].Ref.Pid != 702 || got[1].Carrier != FixtureCarrierEnvironment {
		t.Fatalf("dead-owner scan = %#v, %v; want argv child then environment child", got, err)
	}
}

func TestFixtureScanClassifiesScopedUnreadableProcess(t *testing.T) {
	owner := fixtureExact(700, 70).Ref()
	key := FixtureKey{Owner: owner, Test: "TestOne", Nonce: "00000001"}
	child := fixtureExact(701, 71)
	child.Argv, child.ArgvKnown = []string{"/bin/sh", fixtureWord(t, key)}, true
	unreadable := fixtureExact(702, 72)
	table := fixtureTable{701: child, 702: unreadable}
	installFixtureScanTable(t, table)
	fixtureSurvivorScope = func(pid int64) FixtureProcessScope {
		return FixtureProcessScope{Pgid: 701, Sid: pid + 100, Signalable: true}
	}
	got, err := FixtureSurvivorsOfDeadOwner(table, owner)
	if err != nil || len(got) != 2 || got[1].Class != FixtureSurvivorUnreadable {
		t.Fatalf("scan = %#v, %v; want a separate unreadable class", got, err)
	}
}

func TestFixtureScanClassifiesGoTmpUnreadableProcess(t *testing.T) {
	owner := fixtureExact(700, 70).Ref()
	key := FixtureKey{Owner: owner, Test: "TestOne", Nonce: "00000001"}
	child := fixtureExact(701, 71)
	child.Argv, child.ArgvKnown = []string{"/bin/sh", fixtureWord(t, key)}, true
	unreadable := fixtureExact(702, 72)
	unreadable.Exe, unreadable.ExeKnown = filepath.Join(t.TempDir(), "go-tmp", "TestOther99", "metasystem"), true
	table := fixtureTable{701: child, 702: unreadable}
	installFixtureScanTable(t, table)
	fixtureSurvivorScope = func(pid int64) FixtureProcessScope {
		return FixtureProcessScope{Pgid: pid, Sid: pid + 100, Signalable: true}
	}
	got, err := FixtureSurvivorsOfDeadOwner(table, owner)
	if err != nil || len(got) != 2 || got[1].Ref.Pid != 702 || got[1].Class != FixtureSurvivorUnreadable {
		t.Fatalf("scan = %#v, %v; want the unrelated go-tmp process in the unreadable class", got, err)
	}
}

func TestFixtureSurvivorsRequiresUnreadableProcessToBeLedByCertainSurvivor(t *testing.T) {
	owner := fixtureExact(700, 70).Ref()
	key := FixtureKey{Owner: owner, Test: "TestOne", Nonce: "00000001"}
	child := fixtureExact(701, 71)
	child.Argv, child.ArgvKnown = []string{"/bin/sh", fixtureWord(t, key)}, true
	led := fixtureExact(702, 72)
	shared := fixtureExact(703, 73)
	shared.Exe, shared.ExeKnown = filepath.Join(t.TempDir(), "go-tmp", "TestOther99", "metasystem"), true
	table := fixtureTable{701: child, 702: led, 703: shared, 704: fixtureExact(704, 74)}
	installFixtureScanTable(t, table)
	fixtureSurvivorScope = func(pid int64) FixtureProcessScope {
		switch pid {
		case 701:
			return FixtureProcessScope{Pgid: 900, Sid: 1001, Signalable: true}
		case 702:
			return FixtureProcessScope{Pgid: 701, Sid: 1002, Signalable: true}
		case 703:
			return FixtureProcessScope{Pgid: 900, Sid: 1003, Signalable: true}
		default:
			return FixtureProcessScope{Pgid: pid, Sid: pid + 100, Signalable: true}
		}
	}
	got, err := FixtureSurvivors(key)
	if err != nil || len(got) != 2 || got[0].Ref.Pid != 701 || got[0].Class != FixtureSurvivorCertain ||
		got[0].Carrier != FixtureCarrierArgvWord || got[1].Ref.Pid != 702 ||
		got[1].Class != FixtureSurvivorUnreadable || got[1].Carrier != "" {
		t.Fatalf("key scan = %#v, %v; want certain pid 701 and unreadable pid 702 only", got, err)
	}
}

func TestFixtureScanUsesOwnershipRecordWhereTestRan(t *testing.T) {
	owner := fixtureExact(700, 70).Ref()
	for _, test := range []struct {
		name, testName, directory string
		want                      int
	}{
		{"plain temporary root", "TestRecord/subtest", "TestRecord", 1},
		{"long top-level name", "TestOwnershipRecordWithATopLevelNameThatContinuesBeyondSixtyFourBytesForFixtureDiscovery/subtest", "TestOwnershipRecordWithATopLevelNameThatContinuesBeyondSixtyFour", 1},
		{"multi-byte letter at limit", "TestOwnershipRecordWithExactlySixtyThreeAsciiBytesBeforeALetteréSuffix/subtest", "TestOwnershipRecordWithExactlySixtyThreeAsciiBytesBeforeALetter", 1},
		{"wrong directory", "TestRecord/subtest", "WrongDirectory", 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			key := FixtureKey{Owner: owner, Test: test.testName, Nonce: "a1b2c3d4"}
			child := fixtureExact(701, 71)
			child.Exe, child.ExeKnown = fixtureRecordExecutable(t, test.directory, key), true
			table := fixtureTable{701: child}
			installFixtureScanTable(t, table)
			got, err := FixtureSurvivorsOfDeadOwner(table, owner)
			if err != nil || len(got) != test.want || test.want == 1 && (got[0].Ref.Pid != 701 || got[0].Class != FixtureSurvivorCertain || got[0].Carrier != FixtureCarrierRecord) {
				t.Fatalf("record-backed scan = %#v, %v; want %d certain record survivors", got, err, test.want)
			}
		})
	}
	t.Run("tag decides before record", func(t *testing.T) {
		recordKey := FixtureKey{Owner: owner, Test: "TestRecord/subtest", Nonce: "a1b2c3d4"}
		tagKey := FixtureKey{Owner: owner, Test: "TestTagged", Nonce: "00000002"}
		child := fixtureExact(703, 73)
		child.Exe, child.ExeKnown = fixtureRecordExecutable(t, "TestRecord", recordKey), true
		child.Environ, child.EnvironKnown = []string{fixtureWord(t, tagKey)}, true
		table := fixtureTable{703: child}
		installFixtureScanTable(t, table)
		got, err := FixtureSurvivors(recordKey)
		if err != nil || len(got) != 0 {
			t.Fatalf("record-key scan = %#v, %v; want tagged process excluded", got, err)
		}
		got, err = FixtureSurvivors(tagKey)
		if err != nil || len(got) != 1 || got[0].Ref.Pid != 703 || got[0].Class != FixtureSurvivorCertain || got[0].Carrier != FixtureCarrierEnvironment {
			t.Fatalf("tag-key scan = %#v, %v; want pid 703 as one certain environment survivor", got, err)
		}
	})
}
func fixtureRecordExecutable(t *testing.T, directoryPrefix string, key FixtureKey) string {
	t.Helper()
	directory, err := os.MkdirTemp("/tmp", directoryPrefix)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(directory) })
	encoded, err := EncodeKey(key)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "fixture-owner"), []byte(encoded+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(directory, "bin", "metasystem")
}

func TestFixtureScanReadsTheGivenSource(t *testing.T) {
	deadOwner := fixtureExact(800, 80).Ref()
	unknownOwner := fixtureExact(801, 81).Ref()
	liveOwner := fixtureExact(802, 82)
	mismatchedOwner := fixtureExact(803, 83).Ref()
	deadKey := FixtureKey{Owner: deadOwner, Test: "TestDead", Nonce: "00000001"}
	unknownKey := FixtureKey{Owner: unknownOwner, Test: "TestUnknown", Nonce: "00000002"}
	liveKey := FixtureKey{Owner: liveOwner.Ref(), Test: "TestLive", Nonce: "00000003"}
	mismatchedKey := FixtureKey{Owner: mismatchedOwner, Test: "TestMismatched", Nonce: "00000004"}
	tagged := func(pid, token int64, key FixtureKey) Exact {
		exact := fixtureExact(pid, token)
		exact.Argv, exact.ArgvKnown = []string{"/bin/sh", fixtureWord(t, key)}, true
		exact.EnvironKnown = true
		return exact
	}
	root := filepath.Join(t.TempDir(), "go-tmp", "TestOther99")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	unowned := fixtureExact(904, 94)
	unowned.ArgvKnown, unowned.EnvironKnown = true, true
	unowned.Exe, unowned.ExeKnown = filepath.Join(root, "unowned"), true
	unreadable := fixtureExact(905, 95)
	unreadable.Exe, unreadable.ExeKnown = filepath.Join(root, "unreadable"), true
	prober := fixtureScanProber{
		exacts: map[int64]Exact{
			802: liveOwner,
			803: fixtureExact(803, 1_000_000),
			900: tagged(900, 90, deadKey),
			901: tagged(901, 91, unknownKey),
			902: tagged(902, 92, liveKey),
			903: tagged(903, 93, mismatchedKey),
			904: unowned,
			905: unreadable,
		},
		unknown: map[int64]bool{801: true},
	}
	oldPids, oldProber, oldScope := survivorPids, fixtureSurvivorProber, fixtureSurvivorScope
	survivorPids = func() ([]int64, error) { return nil, errors.New("package pid source used") }
	fixtureSurvivorProber = fakeProber{state: Unknown, err: errors.New("package prober used")}
	fixtureSurvivorScope = func(int64) FixtureProcessScope { panic("package scope used") }
	t.Cleanup(func() { survivorPids, fixtureSurvivorProber, fixtureSurvivorScope = oldPids, oldProber, oldScope })

	pids := []int64{900, 901, 902, 903, 904, 905}
	got, err := ScanFixtureSurvivors(pids, prober, func(pid int64) FixtureProcessScope {
		return FixtureProcessScope{Pgid: pid, Sid: pid, Ppid: 1, Signalable: true}
	}, FixtureSurvivorSelection{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 5 || got[0].Ref.Pid != 900 || got[0].Class != FixtureSurvivorCertain ||
		got[1].Ref.Pid != 901 || got[1].Class != FixtureSurvivorUnreadable ||
		got[2].Ref.Pid != 903 || got[2].Class != FixtureSurvivorCertain ||
		got[3].Ref.Pid != 904 || got[3].Class != FixtureSurvivorUnowned ||
		got[4].Ref.Pid != 905 || got[4].Class != FixtureSurvivorUnreadable {
		t.Fatalf("whole-table scan = %#v; want dead, unknown, mismatched, unowned, and unreadable classes", got)
	}
	if got[0].Ppid != 1 || got[0].Started.IsZero() {
		t.Fatalf("scan dropped process metadata: %#v", got[0])
	}
}

func TestOwnershipRecordMustBeARegularFile(t *testing.T) {
	key := FixtureKey{Owner: fixtureExact(800, 80).Ref(), Test: "TestOwnershipRecordMustBeARegularFile", Nonce: "a1b2c3d4"}
	encoded, err := EncodeKey(key)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name  string
		build func(string) error
	}{
		{"symlink", func(path string) error {
			target := filepath.Join(t.TempDir(), "target")
			if err := os.WriteFile(target, []byte(encoded), 0o600); err != nil {
				return err
			}
			return os.Symlink(target, path)
		}},
		{"directory", func(path string) error { return os.Mkdir(path, 0o700) }},
		{"oversized", func(path string) error { return os.WriteFile(path, make([]byte, 4097), 0o600) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			directory, err := os.MkdirTemp(t.TempDir(), "TestOwnershipRecordMustBeARegularFile")
			if err != nil {
				t.Fatal(err)
			}
			if err := test.build(filepath.Join(directory, "fixture-owner")); err != nil {
				t.Fatal(err)
			}
			if got, ok := fixtureOwnershipRecord(filepath.Join(directory, "bin", "child")); ok {
				t.Fatalf("fixtureOwnershipRecord accepted %s: %#v", test.name, got)
			}
		})
	}
}
