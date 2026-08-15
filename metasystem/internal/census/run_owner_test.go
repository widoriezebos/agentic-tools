package census

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/run"
)

// MON-03: run records own their process groups through the census walk —
// wrapped three-factor with the argv nonce, draining by pgid+claim with
// the honest label, pid reuse never owned, unreadable records surfacing.
func TestRunGroupCustody(t *testing.T) {
	repo := t.TempDir()
	nonce := strings.Repeat("ab", 16)
	writeRecord := func(id, status, custody string, pid, pgid int64, endedAt string) {
		ended := "null"
		if endedAt != "" {
			ended = `"` + endedAt + `"`
		}
		record := `{"schemaVersion":1,"runId":"` + id + `","kind":"suite","display":"x","custody":"` + custody + `",` +
			`"generation":1,"pid":` + itoa(pid) + `,"pidStartedAt":5000,"pgid":` + itoa(pgid) + `,` +
			`"launchNonce":"` + nonce + `","log":"/tmp/x.log","startedAt":"2026-08-15T10:00:00Z",` +
			`"sessionId":"s","goalId":"","staleAfterMin":30,"windDownMin":10,"endedAt":` + ended + `,` +
			`"evidence":{"mode":"exit-sidecar"},"expect":{"green":"","red":"","hung":"","unknown":""},` +
			`"status":"` + status + `","provisionalVerdict":` + provisional(status) + `,"acked":false}`
		os.MkdirAll(run.Dir(repo), 0o755)
		os.WriteFile(run.RecordPath(repo, id), []byte(record), 0o644)
	}
	writeRecord("wrapped-run", "running", "wrapped", 900, 900, "")
	writeRecord("drain-run", "draining", "wrapped", 901, 901, time.Now().UTC().Format("2006-01-02T15:04:05Z"))
	// Finding 6: a drain whose wind-down expired owns NOTHING anymore.
	writeRecord("expired-drain", "draining", "wrapped", 902, 902,
		time.Now().UTC().Add(-30*time.Minute).Format("2006-01-02T15:04:05Z"))
	os.WriteFile(filepath.Join(run.Dir(repo), "broken.json"), []byte("{nope"), 0o644)

	processes := []Process{
		{Pid: 900, PGID: 900, Started: 5000, Argv: "metasystem run wrap --nonce " + nonce + " -- work", Alive: true},
		{Pid: 910, PGID: 900, Started: 6000, Argv: "child worker", Alive: true},
		{Pid: 920, PGID: 901, Started: 7000, Argv: "surviving descendant", Alive: true},
		{Pid: 930, PGID: 999, Started: 8000, Argv: "unrelated", Alive: true},
		{Pid: 940, PGID: 941, Started: 9000, Argv: "reused pid", Alive: true},
	}
	var diagnostics []string
	owners := loadRunOwners(repo, processes, &diagnostics)
	if len(owners) != 2 {
		t.Fatalf("owners wrong: %+v", owners)
	}
	for _, owner := range owners {
		if owner.Id == "expired-drain" {
			t.Fatalf("an expired drain still owns its group: %+v", owner)
		}
	}

	check := func(p Process, wantClass, wantTagPrefix string) {
		class, _, tag := classifyOwnership(p, nil, nil, owners)
		if class != wantClass {
			t.Fatalf("pid %d class %s want %s", p.Pid, class, wantClass)
		}
		if wantTagPrefix != "" {
			s, _ := tag.(string)
			if !strings.HasPrefix(s, wantTagPrefix) {
				t.Fatalf("pid %d tag %v want prefix %s", p.Pid, tag, wantTagPrefix)
			}
		}
	}
	// The wrapped leader and its group member are owned.
	check(processes[0], "CUSTODY", "RUN wrapped-run")
	check(processes[1], "CUSTODY", "RUN wrapped-run")
	// The draining survivor is owned with the honest label.
	check(processes[2], "CUSTODY", "RUN drain-run (draining)")
	// Unrelated and reused-group processes stay untracked.
	check(processes[3], "UNTRACKED", "")
	check(processes[4], "UNTRACKED", "")
	// The unreadable record surfaced.
	found := false
	for _, line := range diagnostics {
		if strings.Contains(line, "RUN-RECORD-UNREADABLE") {
			found = true
		}
	}
	if !found {
		t.Fatalf("unreadable run record did not surface: %v", diagnostics)
	}
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if neg {
		return "-" + string(digits)
	}
	return string(digits)
}

func provisional(status string) string {
	if status == "draining" {
		return `"ended-unknown"`
	}
	return "null"
}
