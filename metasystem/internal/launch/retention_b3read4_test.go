package launch

// Witnesses of the fourth B3 read (the reader's probes; each failed on
// 5d7057808).

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

// L1: a launch named by a retained design attempt (launch/.design/<key>/
// request.json) or by a standalone read attempt (unit/.reads/<ref>/
// attempt-N/attempt.json) is no retention root: both are released. Their
// consumers read a missing launch as "never started" and start a new
// launch under the same id (RequestDesign :186-187; stepDriver.startStep
// :45-53 through the read attempt's deterministic id <ref>-a<N>-s<i>).
func TestB3Read4LaunchNamedByDesignOrReadAttemptReleased(t *testing.T) {
	t.Parallel()
	f := newRetentionFixture(t)
	f.launch("design-l", Completed, 40*day, 64<<10, 100)
	f.launch("rd-ref-a1-s1", Completed, 40*day, 64<<10, 110)
	f.launch("filler", Completed, 50*day, 64<<10, 120)
	destination := "/repo/docs/design/x.md"
	entry := designEntry{Goal: "g", RecordID: "r", Destination: destination,
		Attempts: []DesignAttempt{{Attempt: 1, BriefSHA256: "b", LaunchID: "design-l", Draft: "/repo/artifacts/agents/intent-design/r-1/draft.md"}}}
	data, _ := json.Marshal(entry)
	dir := f.manager.designDir(destination)
	os.MkdirAll(dir, 0o700)
	os.WriteFile(filepath.Join(dir, "request.json"), data, 0o600)
	attempt := ReadAttempt{Number: 1, State: readAttemptRunning, Round: UnitRound{Number: 1,
		Steps: []UnitStep{{Name: "read", LaunchID: "rd-ref-a1-s1", State: StepRunning}}}}
	data, _ = json.Marshal(attempt)
	adir := filepath.Join(f.units, ".reads", "rd-ref", "attempt-1")
	os.MkdirAll(adir, 0o700)
	os.WriteFile(filepath.Join(adir, "attempt.json"), data, 0o600)
	report := f.pass(f.retention(1))
	t.Logf("actions %+v kept %+v", report.Actions, report.Kept)
	_, designErr := f.manager.Store.Read("design-l")
	_, readErr := f.manager.Store.Read("rd-ref-a1-s1")
	if errors.Is(designErr, fs.ErrNotExist) || errors.Is(readErr, fs.ErrNotExist) {
		t.Fatalf("launches a retained design attempt (%v) and a running read attempt (%v) name were released; both consumers restart a missing launch under the same id",
			designErr, readErr)
	}
}
