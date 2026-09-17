package identity

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

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
	fixtureSurvivorScope = func(pid int64) fixtureProcessScope {
		return fixtureProcessScope{pgid: 900, sid: 901, signalable: true}
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
	fixtureSurvivorScope = func(pid int64) fixtureProcessScope {
		return fixtureProcessScope{pgid: 701, sid: pid + 100, signalable: true}
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
	fixtureSurvivorScope = func(pid int64) fixtureProcessScope {
		return fixtureProcessScope{pgid: pid, sid: pid + 100, signalable: true}
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
	fixtureSurvivorScope = func(pid int64) fixtureProcessScope {
		switch pid {
		case 701:
			return fixtureProcessScope{pgid: 900, sid: 1001, signalable: true}
		case 702:
			return fixtureProcessScope{pgid: 701, sid: 1002, signalable: true}
		case 703:
			return fixtureProcessScope{pgid: 900, sid: 1003, signalable: true}
		default:
			return fixtureProcessScope{pgid: pid, sid: pid + 100, signalable: true}
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
