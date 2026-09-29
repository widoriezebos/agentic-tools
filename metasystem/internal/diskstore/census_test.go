package diskstore

import (
	"context"
	"errors"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// censusTable is a fake process table: uid, executable and whether the
// descriptors read (EPERM otherwise).
type censusRow struct {
	uid        uint32
	executable string
	readable   bool
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
			if row := rows[pid]; row.readable {
				return identity.ProcessUse{Cwd: "/", Executable: row.executable}, nil
			}
			return identity.ProcessUse{}, errors.New("descriptor list unreadable: operation not permitted")
		},
		Executable: func(pid int64) (string, bool) { return rows[pid].executable, rows[pid].executable != "" },
		Command:    func(int64) string { return "" },
	}
}

// Design owner 2026-09-29: an unreadable process of another uid whose
// executable lies under /System/, /usr/libexec/ or /usr/sbin/ is a system
// process, not a holder, and leaves the census complete; an unreadable
// process of the stores' own uid, or with an executable elsewhere, still
// makes it incomplete.
func TestSystemProcessesAreNotHolders(t *testing.T) {
	t.Parallel()
	const owner = 501
	base := map[int64]censusRow{
		10: {uid: owner, executable: "/usr/bin/vim", readable: true},
		20: {uid: 0, executable: "/usr/libexec/securityd"},
		21: {uid: 0, executable: "/System/Library/CoreServices/launchservicesd"},
		22: {uid: 213, executable: "/usr/sbin/cfprefsd"},
	}
	census := TakeUseCensus(context.Background(), censusReaderOf(owner, base))
	if !census.Complete() || len(census.SystemNonHolders) != 3 || census.Count != 1 {
		t.Fatalf("system processes of other uids = complete %v, non-holders %v, gaps %v", census.Complete(), census.SystemNonHolders, census.Unreadable)
	}
	for name, row := range map[string]censusRow{
		"same uid under /usr/libexec": {uid: owner, executable: "/usr/libexec/lsd"},
		"other uid elsewhere":         {uid: 0, executable: "/Library/PrivilegedHelperTools/helper"},
		"other uid, path unreadable":  {uid: 0},
		"a lookalike prefix":          {uid: 0, executable: "/usr/libexecutable/tool"},
	} {
		rows := map[int64]censusRow{30: row}
		for pid, known := range base {
			rows[pid] = known
		}
		census := TakeUseCensus(context.Background(), censusReaderOf(owner, rows))
		if census.Complete() || len(census.Unreadable) != 1 || census.Unreadable[0].Pid != 30 {
			t.Errorf("%s = complete %v, gaps %v; want incomplete on pid 30", name, census.Complete(), census.Unreadable)
		}
	}
}
