package diskstore

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// censusRow is one fake process: uid, parent (-1: the chain cannot be read
// here), whether its descriptors read, and whether it is the metasystem's.
type censusRow struct {
	uid      uint32
	parent   int64
	readable bool
	ours     bool
}

func censusReaderOf(owner uint32, rows map[int64]censusRow) CensusReader {
	var pids []int64
	for pid := range rows {
		pids = append(pids, pid)
	}
	return CensusReader{
		UID:  owner,
		Pids: func() ([]int64, error) { return pids, nil },
		ProcessUID: func(pid int64) (uint32, bool) {
			row, ok := rows[pid]
			return row.uid, ok
		},
		Use: func(pid int64) (identity.ProcessUse, error) {
			if rows[pid].readable {
				return identity.ProcessUse{Cwd: "/", Executable: "/bin/x"}, nil
			}
			return identity.ProcessUse{}, errors.New("descriptor list unreadable: operation not permitted")
		},
		Parent: func(pid int64) (int64, bool) {
			row, ok := rows[pid]
			return row.parent, ok && row.parent >= 0
		},
		Ours:    func(pid int64) bool { return rows[pid].ours },
		Command: func(int64) string { return "" },
	}
}

// Design owner 2026-09-29: an unreadable process none of whose ancestors
// is the metasystem's is "unreadable, not ours" and leaves the census
// complete; an unreadable child of a recorded launch, an unreadable
// recorded process itself, and one whose chain cannot be walked keep it
// incomplete.
func TestUnreadableProcessesWithNoMetasystemAncestorAreNotHolders(t *testing.T) {
	t.Parallel()
	const owner = 501
	base := map[int64]censusRow{
		1:  {uid: 0, parent: 0, readable: false},
		10: {uid: owner, parent: 1, readable: true},
		20: {uid: owner, parent: 1},                             // an Apple agent of the user's own uid
		21: {uid: 0, parent: 1},                                 // a root daemon
		22: {uid: 0, parent: 21},                                // its child
		40: {uid: owner, parent: 1, readable: true, ours: true}, // a recorded launch
	}
	census := TakeUseCensus(context.Background(), censusReaderOf(owner, base))
	if !census.Complete() || len(census.NotOurs) != 4 {
		t.Fatalf("unreadable processes with no metasystem ancestor = complete %v, not ours %d, gaps %v", census.Complete(), len(census.NotOurs), census.Unreadable)
	}
	for name, row := range map[string]censusRow{
		"child of a recorded launch":    {uid: owner, parent: 40},
		"grandchild of a recorded one":  {uid: 0, parent: 41},
		"a recorded process itself":     {uid: owner, parent: 1, ours: true},
		"a chain that cannot be walked": {uid: 0, parent: -1},
	} {
		rows := map[int64]censusRow{30: row, 41: {uid: owner, parent: 40, readable: true}}
		for pid, known := range base {
			rows[pid] = known
		}
		census := TakeUseCensus(context.Background(), censusReaderOf(owner, rows))
		if census.Complete() || len(census.Unreadable) != 1 || census.Unreadable[0].Pid != 30 {
			t.Errorf("%s = complete %v, gaps %v; want incomplete on pid 30", name, census.Complete(), census.Unreadable)
		}
	}
	noSeams := censusReaderOf(owner, base)
	noSeams.Ours = nil
	if withoutSeams := TakeUseCensus(context.Background(), noSeams); withoutSeams.Complete() {
		t.Fatal("without an ancestry reader an unreadable process was judged not ours")
	}
}

// RecordedProcesses reads the pids records name, with their start times, so
// a reused pid does not match; engine binaries are ours whatever records say.
func TestRecordedProcessesReadRecordsAndStartTimes(t *testing.T) {
	t.Parallel()
	dir := realDir(t)
	jobs := filepath.Join(dir, "jobs")
	launches := filepath.Join(dir, "launch")
	for _, path := range []string{jobs, filepath.Join(launches, "running"), filepath.Join(launches, "ended")} {
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	write := func(path, body string) {
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(jobs, "a.json"), `{"pid": 100, "pidStartedAt": 5000, "custody": [{"supervisorPid": 101}]}`)
	write(filepath.Join(jobs, "notes.txt"), `{"pid": 102}`)
	write(filepath.Join(launches, "running", "record.json"), `{"Supervisor": {"Pid": 103}}`)
	write(filepath.Join(launches, "ended", "record.json"), `{"Pid": 104}`)
	write(filepath.Join(launches, "ended", "result.json"), `{}`)
	recorded := &RecordedProcesses{
		StartOf: func(pid int64) (int64, bool) { return map[int64]int64{100: 5000, 101: 1, 103: 1}[pid], true },
		Engine:  func(pid int64) bool { return pid == 200 },
	}
	recorded.ScanFiles(RecordFiles([]string{jobs}, launches))
	for pid, want := range map[int64]bool{100: true, 101: true, 102: false, 103: true, 104: false, 200: true, 300: false} {
		if got := recorded.Ours(pid); got != want {
			t.Errorf("Ours(%d) = %v, want %v", pid, got, want)
		}
	}
	reused := &RecordedProcesses{StartOf: func(int64) (int64, bool) { return 9999, true }}
	reused.Add(100, 5000)
	if reused.Ours(100) {
		t.Fatal("a reused pid matched a record with another start time")
	}
	if !EngineExecutable("/Users/x/checkout/metasystem/bin/metasystem") || EngineExecutable("/usr/libexec/lsd") {
		t.Fatal("engine executable classification")
	}
}
