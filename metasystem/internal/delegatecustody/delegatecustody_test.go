package delegatecustody_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/delegatecustody"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
)

func TestMain(m *testing.M) { os.Exit(testenv.Main(m)) }

func writeJob(t *testing.T, root, name, body string) {
	t.Helper()
	dir := filepath.Join(root, "artifacts", "agents", "jobs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name+".json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// Read returns the supervisor and every custody process a record names,
// for one job or for every job, and refuses a record it cannot trust.
func TestReadReturnsEveryOwnedIdentityAndRefusesAnUntrustedRecord(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeJob(t, root, "a", `{"jobId":"a","pid":11,"pidStartedAt":100,"pidStartedAtExactMicro":100000001,
		"custodyProcesses":[{"pid":12,"pidStartedAt":101,"pidStartTicks":7,"bootId":"boot"}]}`)
	writeJob(t, root, "b", `{"jobId":"b","custodyProcesses":[{"pid":21,"pidStartedAt":200}]}`)

	all, err := delegatecustody.Read(root, "", nil)
	if err != nil || len(all) != 3 {
		t.Fatalf("Read all = %+v, %v; want three owned identities", all, err)
	}
	if all[0].JobID != "a" || all[0].Ref.Mode() != identity.CompareDarwinMicroseconds ||
		all[1].Ref.Mode() != identity.CompareLinuxTicksBootID || all[2].JobID != "b" || all[2].Ref.Mode() != identity.CompareLegacySeconds {
		t.Fatalf("Read all = %+v", all)
	}
	one, err := delegatecustody.Read(root, "b", nil)
	if err != nil || len(one) != 1 || one[0].Ref.Pid != 21 {
		t.Fatalf("Read b = %+v, %v", one, err)
	}
	if _, err := delegatecustody.Read(root, "missing", nil); err == nil {
		t.Fatal("a named job without a record was not refused")
	}
	if got, err := delegatecustody.Read(t.TempDir(), "", nil); err != nil || len(got) != 0 {
		t.Fatalf("Read with no job records = %+v, %v", got, err)
	}
	// A record removed between the listing and the read is skipped when
	// every job is read, and refused when it was the one named.
	vanished := func(path string) ([]byte, error) {
		if strings.HasSuffix(path, "a.json") {
			return nil, os.ErrNotExist
		}
		return os.ReadFile(path)
	}
	if got, err := delegatecustody.Read(root, "", vanished); err != nil || len(got) != 1 {
		t.Fatalf("Read with a vanished record = %+v, %v", got, err)
	}
	if _, err := delegatecustody.Read(root, "", func(string) ([]byte, error) { return nil, errors.New("denied") }); err == nil {
		t.Fatal("an unreadable record was not refused")
	}

	for name, body := range map[string]string{
		"corrupt":    `{`,
		"mismatched": `{"jobId":"other"}`,
		"partial":    `{"jobId":"partial","pid":5}`,
		"invalid":    `{"jobId":"invalid","pid":5,"pidStartedAt":9,"pidStartedAtExactMicro":1,"pidStartTicks":2}`,
		"custody":    `{"jobId":"custody","custodyProcesses":[{"pid":0,"pidStartedAt":9}]}`,
	} {
		bad := t.TempDir()
		writeJob(t, bad, name, body)
		if _, err := delegatecustody.Read(bad, name, nil); err == nil {
			t.Errorf("record %s (%s) was not refused", name, body)
		}
	}
}

// Walk finds the first process on the ancestry that a record owns by exact
// identity, refuses an ancestry it cannot authenticate, and ends on a loop
// or at the root without a match.
func TestWalkMatchesTheAncestryByExactIdentity(t *testing.T) {
	t.Parallel()
	started := time.Unix(100, 1000)
	owned := []delegatecustody.Owned{{JobID: "job", Ref: identity.Exact{Pid: 2, StartedAt: started}.Ref()}}
	parents := map[int64]int64{4: 3, 3: 2, 2: 1}
	parent := func(pid int64) (int64, bool) { next, ok := parents[pid]; return next, ok }
	probe := func(pid int64) (identity.Exact, bool) { return identity.Exact{Pid: pid, StartedAt: started}, true }

	match, at, mode, found, err := delegatecustody.Walk(owned, 4, probe, parent)
	if err != nil || !found || at != 2 || match.JobID != "job" || mode != identity.CompareDarwinMicroseconds {
		t.Fatalf("Walk = %+v at %d (%s) found=%v err=%v", match, at, mode, found, err)
	}
	// A reused pid with another start time is not the owned process.
	reused := func(pid int64) (identity.Exact, bool) {
		return identity.Exact{Pid: pid, StartedAt: started.Add(time.Second)}, true
	}
	if _, _, _, found, err := delegatecustody.Walk(owned, 4, reused, parent); found || err != nil {
		t.Fatalf("a reused pid matched: found=%v err=%v", found, err)
	}
	if _, _, _, found, err := delegatecustody.Walk(owned, 4, func(int64) (identity.Exact, bool) { return identity.Exact{}, false }, parent); found || err == nil {
		t.Fatalf("an unreadable ancestor was not refused: found=%v err=%v", found, err)
	}
	loop := func(pid int64) (int64, bool) { return pid, true }
	if _, _, _, found, err := delegatecustody.Walk(owned, 9, probe, loop); found || err != nil {
		t.Fatalf("a self-parented process: found=%v err=%v", found, err)
	}
}
