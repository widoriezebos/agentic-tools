package dispatch

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/census"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

type stage4ProcessTable struct {
	starts      map[int64]identity.Exact
	startStates map[int64]identity.Liveness
	argv        map[int64][]string
	argvKnown   map[int64]bool
	groups      map[int64]int64
	pids        []int64
}

func (f *stage4ProcessTable) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	exact, state, err := f.ReadStart(pid)
	if err != nil {
		return exact, state, err
	}
	exact.Argv = f.argv[pid]
	exact.ArgvKnown = f.argvKnown[pid]
	return exact, state, nil
}

func (f *stage4ProcessTable) ReadStart(pid int64) (identity.Exact, identity.Liveness, error) {
	state, present := f.startStates[pid]
	if !present {
		state = identity.Dead
	}
	if state == identity.Unknown {
		return identity.Exact{}, state, errors.New("identity unreadable")
	}
	return f.starts[pid], state, nil
}

func (f *stage4ProcessTable) ReadArgv(pid int64) ([]string, bool) {
	return f.argv[pid], f.argvKnown[pid]
}

func (f *stage4ProcessTable) PIDs() ([]int64, error) {
	return append([]int64(nil), f.pids...), nil
}

func (f *stage4ProcessTable) PGID(pid int64) (int64, error) {
	group, ok := f.groups[pid]
	if !ok {
		return 0, errors.New("group unavailable")
	}
	return group, nil
}

func stage4Exact(pid, micro int64) identity.Exact {
	return nativeTestExact(pid, micro)
}

func stage4RefObject(exact identity.Exact) map[string]any {
	return exactIdentityFields(exact.Ref())
}

func stage4RecordWithPrimary(fields map[string]any, primary identity.Exact, pgid int64) map[string]any {
	for key, value := range exactIdentityFields(primary.Ref()) {
		fields[key] = value
	}
	fields["pgid"] = pgid
	return fields
}

func stage4DeathDependencies(table *stage4ProcessTable) CustodyDeathDependencies {
	return CustodyDeathDependencies{
		Reader: table,
		PIDs:   table.PIDs,
		PGID:   table.PGID,
		TaggedScan: func(tag string) census.TaggedProcessCensus {
			return census.ScanTaggedProcesses(tag, census.TaggedScanDependencies{
				PIDs: table.PIDs, Signal: func(int64) error { return nil }, PGID: table.PGID, Reader: table,
				MatchesTag: func(argv []string, wanted string) bool {
					return len(argv) == 2 && argv[0] == "owned" && argv[1] == wanted
				},
			})
		},
		MatchesTag: func(argv []string, wanted string) bool {
			return len(argv) == 2 && argv[0] == "owned" && argv[1] == wanted
		},
	}
}

func TestSupervisorDeathInForkWindowDefersWhileMarkerStands(t *testing.T) {
	root := t.TempDir()
	tag := "metasystem-job-fork-window-nonce"
	supervisor := stage4Exact(40, 100_000_001)
	child := stage4Exact(41, 101_000_001)
	record := stage4RecordWithPrimary(map[string]any{
		"jobId": "fork-window", "status": "pending", "instanceTag": tag,
		"custodyProcesses": []any{},
	}, supervisor, 40)
	writeStage4Marker(t, root, tag, supervisor, 40, 0)
	table := &stage4ProcessTable{
		starts: map[int64]identity.Exact{41: child},
		startStates: map[int64]identity.Liveness{
			40: identity.Dead, 41: identity.Alive,
		},
		argv: map[int64][]string{41: {"untagged-child"}}, argvKnown: map[int64]bool{41: true},
		groups: map[int64]int64{41: 40}, pids: []int64{41},
	}
	dependencies := stage4DeathDependencies(table)
	marker, _ := preforkMarkerPath(root, tag)
	for pass := 1; pass <= 2; pass++ {
		got := ProveCustodyDeath(root, record, dependencies)
		if got.Outcome != CustodyDeathDeferred || got.Reason != "prefork-child-unproven" {
			t.Fatalf("marker pass %d = %+v, want live fork-window child deferral", pass, got)
		}
		if _, err := os.Stat(marker); err != nil {
			t.Fatalf("marker disappeared after pass %d while its intended group was live: %v", pass, err)
		}
	}
	if !sessionRecordIsBusy(classifySessionRecord("fork-window", record)) {
		t.Fatal("a fork-window deferral freed the session")
	}
}

func TestRecycledGroupExpiresMarkerBoundToDeadSupervisor(t *testing.T) {
	root := t.TempDir()
	tag := "metasystem-job-recycled-group-nonce"
	supervisor := stage4Exact(50, 200_000_001)
	foreign := stage4Exact(51, 100_000_001)
	record := stage4RecordWithPrimary(map[string]any{
		"jobId": "recycled-group", "status": "running", "instanceTag": tag,
		"custodyProcesses": []any{},
	}, supervisor, 50)
	marker := writeStage4Marker(t, root, tag, supervisor, 50, 0)
	table := &stage4ProcessTable{
		starts: map[int64]identity.Exact{51: foreign},
		startStates: map[int64]identity.Liveness{
			50: identity.Dead, 51: identity.Alive,
		},
		argv: map[int64][]string{51: {"foreign"}}, argvKnown: map[int64]bool{51: true},
		groups: map[int64]int64{51: 50}, pids: []int64{51},
	}
	dependencies := stage4DeathDependencies(table)
	if first := ProveCustodyDeath(root, record, dependencies); first.Outcome != CustodyDeathDeferred || first.Reason != "prefork-marker-expired" {
		t.Fatalf("marker did not expire from supervisor identity: %+v", first)
	}
	got := ProveCustodyDeath(root, record, dependencies)
	if got.Outcome != CustodyDeathProven {
		t.Fatalf("recycled group death = %+v, want proven", got)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("expired marker was not swept: %v", err)
	}
	record["status"] = "failed"
	if !sessionRecordIsFree(classifySessionRecord("recycled-group", record)) {
		t.Fatal("a terminal record with an expired marker did not free the session")
	}
}

func TestDeadSupervisorMarkerExpiresWhenItsNamedGroupIsEmpty(t *testing.T) {
	root := t.TempDir()
	tag := "metasystem-job-empty-prefork-group-nonce"
	supervisor := stage4Exact(55, 350_000_001)
	record := stage4RecordWithPrimary(map[string]any{
		"jobId": "empty-prefork-group", "status": "pending", "instanceTag": tag,
		"custodyProcesses": []any{},
	}, supervisor, 55)
	marker := writeStage4Marker(t, root, tag, supervisor, 55, 0)
	table := &stage4ProcessTable{
		startStates: map[int64]identity.Liveness{55: identity.Dead},
		groups:      map[int64]int64{},
		pids:        []int64{},
	}
	got := ProveCustodyDeath(root, record, stage4DeathDependencies(table))
	if got.Outcome != CustodyDeathDeferred || got.Reason != "prefork-marker-expired" {
		t.Fatalf("empty named group expiry = %+v", got)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("empty named group left its expired marker: %v", err)
	}
}

func TestMarkerSweepHonorsCustodyWriteBeforeRemoval(t *testing.T) {
	root := t.TempDir()
	tag := "metasystem-job-marker-order-nonce"
	supervisor := stage4Exact(60, 400_000_001)
	child := stage4Exact(61, 401_000_001)
	marker := writeStage4Marker(t, root, tag, supervisor, 60, 0)
	record := map[string]any{
		"jobId": "marker-order", "status": "pending", "instanceTag": tag,
		"custodyProcesses": []any{stage4RefObject(child)},
	}
	if removed, err := SweepSatisfiedPreforkMarker(root, record); err != nil || !removed {
		t.Fatalf("marker+custody sweep = removed:%v err:%v", removed, err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("satisfied marker remains: %v", err)
	}

	marker = writeStage4Marker(t, root, tag, supervisor, 60, 1)
	if removed, err := SweepSatisfiedPreforkMarker(root, record); err != nil || removed {
		t.Fatalf("standing marker before its custody write = removed:%v err:%v", removed, err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("standing marker disappeared: %v", err)
	}
}

func TestCustodyWriteLandsBeforeMarkerRemoval(t *testing.T) {
	root := t.TempDir()
	job := "custody-order"
	tag := "metasystem-job-custody-order-nonce"
	supervisor := stage4Exact(62, 410_000_001)
	child := stage4Exact(63, 411_000_001)
	recordPath := filepath.Join(root, "artifacts", "agents", "jobs", job+".json")
	if err := writeRecord(recordPath, map[string]any{
		"jobId": job, "status": "pending", "proofLevel": "proven", "instanceTag": tag,
		"custodyProcesses": []any{},
	}); err != nil {
		t.Fatal(err)
	}
	reader := &stage4ProcessTable{
		starts:      map[int64]identity.Exact{62: supervisor, 63: child},
		startStates: map[int64]identity.Liveness{62: identity.Alive, 63: identity.Alive},
	}
	if err := WritePreforkMarker(root, job, tag, 62, 62, reader); err != nil {
		t.Fatal(err)
	}
	marker, _ := preforkMarkerPath(root, tag)
	if err := CustodyAdd(root, job, 63, reader, func(int64) (int64, error) { return 62, nil }); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("custody registration did not remove its marker: %v", err)
	}
	landed, err := readObject(recordPath)
	if err != nil {
		t.Fatal(err)
	}
	items, _ := landed["custodyProcesses"].([]any)
	if len(items) != 1 {
		t.Fatalf("marker disappeared without a custody record: %+v", landed)
	}
	entry, _ := items[0].(map[string]any)
	if !looseEqual(entry["pgid"], int64(62)) {
		t.Fatalf("custody entry did not preserve its group: %+v", entry)
	}
}

func TestCrossGroupCustodyBlocksDeathAndIsIncludedInWindDown(t *testing.T) {
	root := t.TempDir()
	tag := "metasystem-job-cross-group-nonce"
	cross := stage4Exact(71, 501_000_001)
	entry := stage4RefObject(cross)
	entry["pgid"] = int64(70)
	entry["instanceTag"] = tag
	primary := stage4Exact(69, 500_000_001)
	record := stage4RecordWithPrimary(map[string]any{
		"jobId": "cross-group", "status": "running", "instanceTag": tag,
		"custodyProcesses": []any{entry},
	}, primary, 69)
	table := &stage4ProcessTable{
		starts: map[int64]identity.Exact{71: cross},
		startStates: map[int64]identity.Liveness{
			69: identity.Dead, 71: identity.Alive,
		},
		argv: map[int64][]string{71: {"owned", tag}}, argvKnown: map[int64]bool{71: true},
		groups: map[int64]int64{71: 70}, pids: []int64{71},
	}
	if got := ProveCustodyDeath(root, record, stage4DeathDependencies(table)); got.Outcome != CustodyDeathAlive {
		t.Fatalf("cross-group custody did not block death: %+v", got)
	}
	targets, err := CustodyGroupTargets(record, table.PGID)
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 2 || targets[0] != 69 || targets[1] != 70 {
		t.Fatalf("kill targets = %v, want [69 70]", targets)
	}
}

func TestWindDownTargetsRequireARecordedPrimaryGroup(t *testing.T) {
	if _, err := CustodyGroupTargets(map[string]any{
		"pgid": nil, "custodyProcesses": []any{},
	}, nil); err == nil || !strings.Contains(err.Error(), "no primary process group") {
		t.Fatalf("identityless record entered group wind-down: %v", err)
	}
}

func TestUnreadableInGroupMemberDefersDeathWithoutMarker(t *testing.T) {
	root := t.TempDir()
	tag := "metasystem-job-unreadable-group-nonce"
	primary := stage4Exact(72, 510_000_001)
	record := stage4RecordWithPrimary(map[string]any{
		"jobId": "unreadable-group", "status": "running", "instanceTag": tag,
		"custodyProcesses": []any{},
	}, primary, 72)
	table := &stage4ProcessTable{
		startStates: map[int64]identity.Liveness{72: identity.Dead, 73: identity.Unknown},
		groups:      map[int64]int64{73: 72},
		pids:        []int64{73},
	}
	got := ProveCustodyDeath(root, record, stage4DeathDependencies(table))
	if got.Outcome != CustodyDeathDeferred || got.Reason != "in-group-member-unproven" {
		t.Fatalf("unreadable in-group member = %+v, want parent D-E deferral", got)
	}
}

func writeStage4Marker(t *testing.T, root, tag string, supervisor identity.Exact, pgid int64, custodyCount int) string {
	t.Helper()
	path := filepath.Join(root, "artifacts", "agents", "prefork", tag)
	marker := map[string]any{
		"schemaVersion": 1, "instanceTag": tag, "intendedPgid": pgid,
		"supervisor": stage4RefObject(supervisor), "custodyCountBefore": custodyCount,
	}
	if err := writeRecord(path, marker); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestInstanceTagsRequireLaunchOperationSuffix(t *testing.T) {
	if _, err := reservationInstanceTag("job-a", ""); err == nil {
		t.Fatal("a generationless tag was accepted")
	}
	got, err := reservationInstanceTag("job-a", "0123456789abcdef")
	if err != nil || got != "metasystem-job-job-a-0123456789abcdef" {
		t.Fatalf("generational tag = %q err=%v", got, err)
	}
	if _, err := reservationInstanceTag("job-a", "bad/suffix"); err == nil || !strings.Contains(err.Error(), "suffix") {
		t.Fatalf("unsafe suffix error = %v", err)
	}
}
