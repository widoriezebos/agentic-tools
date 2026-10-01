package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

// stopLossBed is a registered nested lane whose steward's keeper wakes the
// landing agent through the real launch manager (its starts recorded by a
// stand-in supervisor) and reaps it with the production reaps: the outage,
// and the usage reconciled from the launch's measure or its transcript.
type stopLossBed struct {
	home, checkout, module, projects string
	store                            launch.Store
	keeper                           lane.AgentKeeper
	notified                         []string
}

func newStopLossBed(t *testing.T) *stopLossBed {
	t.Helper()
	base := t.TempDir()
	bed := &stopLossBed{home: filepath.Join(base, "home"), checkout: filepath.Join(base, "landing"), projects: filepath.Join(base, "projects")}
	bed.module = filepath.Join(bed.checkout, "metasystem")
	for _, dir := range []string{bed.home, bed.module, bed.projects} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for path, content := range map[string]string{
		filepath.Join(bed.module, "go.mod"):                "module fixture\n",
		filepath.Join(bed.module, "metasystem.conf"):       "# overrides only\n",
		filepath.Join(bed.module, "metasystem.conf.local"): "launch.landing.runtime=claude\nlaunch.landing.model=claude-roster-model\n",
	} {
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	registerLane(t, bed.home, bed.checkout, "a-person", time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC))
	bed.checkout, bed.module = resolvedPath(bed.checkout), resolvedPath(bed.module)
	now := time.Date(2026, 9, 30, 13, 0, 0, 0, time.UTC)
	bed.store = launch.Store{Root: filepath.Join(base, "launches")}
	manager := &launch.Manager{Store: bed.store, Adapters: map[string]launch.Adapter{"claude-headless": launch.ClaudeHeadless{Binary: "/fixture/bin/claude", ProjectsRoot: bed.projects}},
		Supervisor: recordingSupervisor{bed.store}, Now: func() time.Time { return now }, Sleep: func(time.Duration) {}, Poll: time.Second, StartCap: time.Minute,
		Lane: landingLaneCheckout(func() (string, error) { return bed.home, nil })}
	nonces := 0
	agent := landingAgent{manager: func() *launch.Manager { return manager }, settings: installationSettings, now: func() time.Time { return now },
		nonce: func() (string, error) { nonces++; return "00112233445566" + itoa2(nonces), nil },
		gateSettings: func(launch.Store, string, string, string) (string, error) {
			return filepath.Join(base, "gate", "landing-settings.json"), nil
		}}
	bed.keeper = newLandingAgentKeeper(bed.module, bed.home, agent)
	bed.keeper.Sources.Validation = func(string, time.Time) (bool, error) { return true, nil }
	bed.keeper.Alert = openLaneAlert(bed.home, agent.now, func(_, message string) error { bed.notified = append(bed.notified, message); return nil })
	return bed
}

func itoa2(n int) string { return string(rune('0'+n/10)) + string(rune('0'+n%10)) }

func (bed *stopLossBed) launches(t *testing.T) []launch.Record {
	t.Helper()
	records, err := bed.store.List()
	if err != nil {
		t.Fatal(err)
	}
	return records
}

// end marks a launch ended as state, as its supervisor or a cancel where no
// supervisor ran would leave it: no measurement.
func (bed *stopLossBed) end(t *testing.T, id string, state launch.State) {
	t.Helper()
	if _, err := bed.store.Update(id, func(record *launch.Record) error { record.State = state; return nil }); err != nil {
		t.Fatal(err)
	}
}

func (bed *stopLossBed) session(t *testing.T, record launch.Record) string {
	t.Helper()
	var session string
	if err := json.Unmarshal(record.AdapterData["sessionID"], &session); err != nil || len(session) != 36 {
		t.Fatalf("launch %s fixed no session id: %q %v", record.ID, session, err)
	}
	return session
}

// TestUnknownUsageHoldsLaunch (K10, R8-06, rehearsal step 11): the usage of
// every ended landing launch is reconciled once by its id, a cancelled one
// too. A launch whose usage can't be read (no measure, no transcript) holds
// the next launch: the lane stops for a person and an alert opens through
// the steward's OpenAlert. A person's resume goes past it with a fresh
// allowance; the history keeps it.
func TestUnknownUsageHoldsLaunch(t *testing.T) {
	t.Parallel()
	bed := newStopLossBed(t)
	bed.keeper.Step()
	records := bed.launches(t)
	if len(records) != 1 {
		t.Fatalf("launches = %d; want the first session", len(records))
	}
	bed.session(t, records[0])
	bed.end(t, records[0].ID, launch.Cancelled)

	line := bed.keeper.Step()
	if records := bed.launches(t); len(records) != 1 {
		t.Fatalf("a session with unknown usage did not hold the next: %d launches, line %q", len(records), line)
	}
	pause, paused := lane.ReadPause(bed.home)
	store, err := lane.ReadStopLoss(bed.home)
	if !paused || pause.By != lane.StopLossBy || err != nil || len(store.Hits) != 1 || store.Hits[0].Kind != lane.HitUsageUnknown {
		t.Fatalf("after the unknown usage: pause %+v %t, hits %+v %v; want the lane stopped by its stop-loss", pause, paused, store.Hits, err)
	}
	episodes, err := steward.AlertEpisodes(bed.module)
	if err != nil || len(episodes) != 1 || episodes[0].Owner != laneStopLossOwner || !strings.Contains(episodes[0].Message, records[0].ID) || len(bed.notified) != 1 {
		t.Fatalf("alerts = %+v %v, notified %v; want one stop-loss alert naming the session", episodes, err, bed.notified)
	}
	if line := bed.keeper.Step(); len(bed.launches(t)) != 1 || !strings.Contains(line, "paused") {
		t.Fatalf("still held: %q", line)
	}

	// A person resumes: the unknown usage is gone past, the history kept.
	if err := lane.Grant(bed.home, "Wido", time.Date(2026, 9, 30, 13, 5, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	if _, err := lane.ClearPause(bed.home); err != nil {
		t.Fatal(err)
	}
	if err := lane.ClearAgentCooldown(bed.home); err != nil {
		t.Fatal(err)
	}
	if line := bed.keeper.Step(); len(bed.launches(t)) != 2 {
		t.Fatalf("after a person's resume: %q; want the next session", line)
	}
	if after, _ := lane.ReadStopLoss(bed.home); after.Grant != 1 || after.Launches[0].AcceptedBy != "Wido" || len(after.History) <= len(store.History) {
		t.Fatalf("after the resume: %+v; want the unknown usage accepted by Wido and the history kept", after)
	}
}

// TestCancelledLaunchReconciledFromItsTranscript (K10, R8-06, rehearsal
// step 11): a session cancelled where no supervisor measured it is
// reconciled from its transcript, found by the session id its launch fixed
// before it started; the next session starts.
func TestCancelledLaunchReconciledFromItsTranscript(t *testing.T) {
	t.Parallel()
	bed := newStopLossBed(t)
	bed.keeper.Step()
	first := bed.launches(t)[0]
	dir := filepath.Join(bed.projects, "-landing")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	rows := `{"type":"assistant","message":{"id":"m1","usage":{"input_tokens":120,"output_tokens":30}}}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, bed.session(t, first)+".jsonl"), []byte(rows), 0o600); err != nil {
		t.Fatal(err)
	}
	bed.end(t, first.ID, launch.Cancelled)
	if err := lane.ClearAgentCooldown(bed.home); err != nil {
		t.Fatal(err)
	}
	line := bed.keeper.Step()
	store, err := lane.ReadStopLoss(bed.home)
	if err != nil || len(store.Launches) < 1 || store.Launches[0].Usage == nil || *store.Launches[0].Usage != (lane.Usage{Known: true, Tokens: 150, Source: "transcript"}) {
		t.Fatalf("the cancelled session's usage = %+v %v; want its transcript's 150 tokens", store.Launches, err)
	}
	if _, paused := lane.ReadPause(bed.home); paused {
		t.Fatalf("a reconciled session stopped the lane: %q", line)
	}
}

// TestPersonsResumeGrantsAFreshAllowance (K10, R8-08): a batch that spent
// its allowance waits for a person (the lane is not paused, so green work
// still publishes); a person's landing start grants a fresh allowance and
// keeps the history. A start with no allowance spent grants nothing.
func TestPersonsResumeGrantsAFreshAllowance(t *testing.T) {
	t.Parallel()
	bed := newLaneVerbBed(t)
	if code, _, stderr := bed.run(t, "landing", "set", bed.landingA); code != 0 {
		t.Fatalf("landing set = %d %q", code, stderr)
	}
	charge := func() error {
		return lane.Gate(bed.home, lane.OpProve, lane.AuthorityAgent, func(lane.Record) error { return lane.ChargeHeld(bed.home, "b-one", laneTestNow) })
	}
	for index := 0; index <= lane.AllowanceExecutions; index++ {
		_ = charge()
	}
	before, err := lane.ReadStopLoss(bed.home)
	if _, spent, _ := lane.AllowanceSpent(bed.home); err != nil || !spent {
		t.Fatalf("a spent allowance reads unspent: %v", err)
	}
	if _, paused := lane.ReadPause(bed.home); paused {
		t.Fatal("a spent allowance paused the lane")
	}
	if code, stdout, stderr := bed.run(t, "landing", "start"); code != 0 {
		t.Fatalf("landing start = %d %q %q", code, stdout, stderr)
	}
	after, err := lane.ReadStopLoss(bed.home)
	if err != nil || after.Grant != before.Grant+1 || len(after.History) <= len(before.History) || after.GrantedBy == "" {
		t.Fatalf("after a person's resume: %+v %v; want a fresh grant over the kept history", after, err)
	}
	if err := charge(); err != nil {
		t.Fatalf("an execution after the resume: %v", err)
	}
	if code, _, stderr := bed.run(t, "landing", "start"); code != 0 {
		t.Fatalf("landing start again = %d %q", code, stderr)
	}
	if again, _ := lane.ReadStopLoss(bed.home); again.Grant != after.Grant {
		t.Fatalf("a start of a running lane granted again: %d -> %d", after.Grant, again.Grant)
	}
}
