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

func TestFixtureScanUsesOwnershipRecordBelowGoTmp(t *testing.T) {
	owner := fixtureExact(700, 70).Ref()
	key := FixtureKey{Owner: owner, Test: "TestRecord", Nonce: "a1b2c3d4"}
	directory := filepath.Join(t.TempDir(), "go-tmp", "TestRecord123", "bin")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	encoded, err := EncodeKey(key)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(directory), "fixture-owner"), []byte(encoded+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	child := fixtureExact(701, 71)
	child.Exe, child.ExeKnown = filepath.Join(directory, "metasystem"), true
	table := fixtureTable{701: child}
	installFixtureScanTable(t, table)
	got, err := FixtureSurvivorsOfDeadOwner(table, owner)
	if err != nil || len(got) != 1 || got[0].Class != FixtureSurvivorCertain {
		t.Fatalf("record-backed scan = %#v, %v; want one certain survivor", got, err)
	}
}
