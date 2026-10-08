package outage

import (
	"fmt"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

var t0 = time.Date(2026, 8, 24, 8, 0, 0, 0, time.UTC)

// The mark's lifecycle: absent → recorded → fed → cleared, with the
// count growing only while the outage keeps being fed.
func TestMarkLifecycle(t *testing.T) {
	root := t.TempDir()
	if _, ok := fixtureRead(root); ok {
		t.Fatal("an absent mark must read as no outage")
	}
	m, err := fixtureObserve(root, "overloaded", "API Error: 529", "mission-runner", t0)
	if err != nil {
		t.Fatal(err)
	}
	if m.ConsecutiveFailures != 1 || m.LastClass != "overloaded" || m.Since != m.LastAt {
		t.Fatalf("first failure starts the outage: %+v", m)
	}
	m, err = fixtureObserve(root, "http-529", "still down", "delegate-adapter", t0.Add(2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if m.ConsecutiveFailures != 2 || m.Since == m.LastAt || m.Source != "delegate-adapter" {
		t.Fatalf("a fed outage keeps its start and grows its count: %+v", m)
	}
	if _, ok := fixtureStanding(root, t0.Add(3*time.Minute)); !ok {
		t.Fatal("a fed mark stands")
	}
	if err := fixtureClear(root); err != nil {
		t.Fatal(err)
	}
	if _, ok := fixtureRead(root); ok {
		t.Fatal("a cleared mark is gone")
	}
	if err := fixtureClear(root); err != nil {
		t.Fatal("clearing an absent mark is success")
	}
}

// The horizon: a mark nobody feeds lapses, so a forgotten mark can
// never permanently pause the steward's clocks; and a NEW failure
// after the lapse starts a fresh outage rather than resuming the old.
func TestHorizonLapse(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if _, err := fixtureObserve(root, "overloaded", "529", "mission-runner", t0); err != nil {
		t.Fatal(err)
	}
	if _, ok := fixtureStanding(root, t0.Add(Horizon)); !ok {
		t.Fatal("a mark inside the horizon stands")
	}
	if _, ok := fixtureStanding(root, t0.Add(Horizon+time.Second)); ok {
		t.Fatal("a mark past the horizon has lapsed")
	}
	if _, ok := fixtureStanding(root, t0.Add(-Horizon-time.Second)); ok {
		t.Fatal("a future-dated mark lapses on the same bound; a clock correction must not pause the clocks indefinitely")
	}
	m, err := fixtureObserve(root, "overloaded", "529 again", "mission-runner", t0.Add(2*Horizon))
	if err != nil {
		t.Fatal(err)
	}
	if m.ConsecutiveFailures != 1 || m.Since != t0.Add(2*Horizon).UTC().Format(time.RFC3339) || m.LastAt != m.Since {
		t.Fatalf("a failure after expiry starts a fresh outage: %+v", m)
	}
	state, err := ReadProviders(filepath.Join(root, ".metasystem"))
	if err != nil {
		t.Fatal(err)
	}
	intervals := state.Current["anthropic"].Intervals
	if len(intervals) != 1 || intervals[0].Since != t0.Format(time.RFC3339) ||
		intervals[0].Until != t0.Add(Horizon).Format(time.RFC3339) || !intervals[0].Stale || intervals[0].FirstSuccessAt != "" {
		t.Fatalf("expiry must retain the bounded wait without claiming provider success: %+v", intervals)
	}
}

// Unreadable current evidence stays unknown and cannot be overwritten by an observation.
func TestUnreadableProviderStateRefusesObservation(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if _, err := fixtureObserve(root, "overloaded", "529", "fixture", t0); err != nil {
		t.Fatal(err)
	}
	home := filepath.Join(root, ".metasystem")
	state, err := ReadProviders(home)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "artifacts", "agents", fmt.Sprintf("providers-%d.json", state.Owner.CustodyEpoch))
	if err := os.WriteFile(path, []byte("{torn"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadProviders(home); err == nil || !strings.Contains(err.Error(), path) {
		t.Fatalf("unreadable state was healthy or its file was hidden: %v", err)
	}
	if _, err := Observe(home, "claude", "fixture-model", "overloaded", "529", "fixture", t0.Add(time.Second)); err == nil {
		t.Fatal("observation overwrote unreadable state")
	}
}

// The classifier's line rule: the provider's own words and status codes
// hit; ordinary diagnostics and near-miss numbers do not.
func TestClassifyLogs(t *testing.T) {
	cases := []struct {
		line  string
		class string
	}{
		{`API Error: 529 {"type":"error","error":{"type":"overloaded_error","message":"Overloaded"}}`, "overloaded"},
		{"api error: status 503 service unavailable", "http-503"},
		{"HTTP 503 Service Unavailable", "http-503"},
		{"upstream returned error 502", "http-502"},
		{"request failed with status 500", "http-500"},
		{"API status 504 gateway timeout", "http-504"},
		{"server error (501)", "http-501"},
		{"stream error: code 529", "http-529"},
		{"unexpected http 507 from upstream", "http-507"},
		{"error after 500ms", ""},
		{"took 503 ms retrying", ""},
		{"processed 529 records", ""},
		{"error: took 5030ms", ""},
		{"error at line 15290", ""},
		{"failed to decode 500 records", ""},
		{"the local scheduler is overloaded", ""},
		{"the pipeline is overloadedly verbose", ""},
		{"all good", ""},
	}
	for _, c := range cases {
		dir := t.TempDir()
		log := filepath.Join(dir, "host.log")
		if err := os.WriteFile(log, []byte("benign line\n"+c.line+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		class, evidence, ok := ClassifyLogs(log)
		if c.class == "" {
			if ok {
				t.Fatalf("%q must not classify (got %s: %s)", c.line, class, evidence)
			}
			continue
		}
		if !ok || class != c.class {
			t.Fatalf("%q: want %s, got %s (ok=%v)", c.line, c.class, class, ok)
		}
	}
	if _, _, ok := ClassifyLogs("", filepath.Join(t.TempDir(), "absent.log")); ok {
		t.Fatal("absent logs classify nothing")
	}
}

// The structured gate: a provider result classifies only when it
// declares itself an error. A successful result whose model output
// merely DISCUSSES a 529 must never mark an outage.
func TestClassifyProviderResultGate(t *testing.T) {
	dir := t.TempDir()
	talking := filepath.Join(dir, "success.json")
	if err := os.WriteFile(talking, []byte(
		`{"is_error":false,"result":"we fixed the 529 overloaded error handling"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, ok := ClassifyProviderResult(talking); ok {
		t.Fatal("model output discussing an outage must not mark one")
	}
	erring := filepath.Join(dir, "error.json")
	if err := os.WriteFile(erring, []byte(
		`{"is_error":true,"result":"API Error: 529 Overloaded"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	class, _, ok := ClassifyProviderResult(erring)
	if !ok || class != "overloaded" {
		t.Fatalf("a declared error with overload words classifies: %s %v", class, ok)
	}
	if _, _, ok := ClassifyProviderResult(filepath.Join(dir, "absent.json")); ok {
		t.Fatal("an absent result classifies nothing")
	}
}

// Long evidence is clipped; a long log is tail-read without error.
func TestEvidenceClipAndTail(t *testing.T) {
	root := t.TempDir()
	m, err := fixtureObserve(root, "overloaded", strings.Repeat("x", 1000), "mission-runner", t0)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.LastDetail) != evidenceClip {
		t.Fatalf("evidence must clip to %d, got %d", evidenceClip, len(m.LastDetail))
	}
	log := filepath.Join(t.TempDir(), "big.log")
	big := strings.Repeat("filler line\n", 10000) + "api error: status 529\n"
	if err := os.WriteFile(log, []byte(big), 0o644); err != nil {
		t.Fatal(err)
	}
	if class, _, ok := ClassifyLogs(log); !ok || class != "http-529" {
		t.Fatalf("the tail of a large log still classifies: %s %v", class, ok)
	}
}

// TestUsageLimitFeedsTheOutageMark (seat-works-without-a-person, test 18):
// the provider's usage limit is weather like an overload. The Claude CLI's
// usage-limit line, the API's rate_limit_error and an HTTP 429 line classify
// as provider-limit; the same framing rules keep local prose and numbers
// out; and a result file still classifies only when it declares is_error.
func TestUsageLimitFeedsTheOutageMark(t *testing.T) {
	t.Parallel()
	cases := []struct {
		line  string
		class string
	}{
		{"Claude AI usage limit reached|1759262400", ProviderLimit},
		{"5-hour limit reached ∙ resets 3pm", ProviderLimit},
		{"You've hit your usage limit · resets 11pm (Europe/Amsterdam)", ProviderLimit},
		{`API Error: 429 {"type":"error","error":{"type":"rate_limit_error","message":"Number of request tokens has exceeded your per-minute rate limit"}}`, ProviderLimit},
		{`{"type":"error","error":{"type":"rate_limit_error","message":"slow down"}}`, ProviderLimit},
		{"HTTP 429 Too Many Requests", ProviderLimit},
		{"request failed with status 429", ProviderLimit},
		{"processed 429 records", ""},
		{"error after 429ms", ""},
		{"set rate_limit_per_minute in the config", ""},
		{"the usage limit setting is documented below", ""},
		// An overload stays an overload.
		{"API Error: 529 Overloaded", "overloaded"},
	}
	for _, c := range cases {
		log := filepath.Join(t.TempDir(), "host.log")
		if err := os.WriteFile(log, []byte("benign line\n"+c.line+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		class, _, ok := ClassifyLogs(log)
		if c.class == "" {
			if ok {
				t.Fatalf("%q must not classify (got %s)", c.line, class)
			}
			continue
		}
		if !ok || class != c.class {
			t.Fatalf("%q: want %s, got %s (ok=%v)", c.line, c.class, class, ok)
		}
	}
	dir := t.TempDir()
	limited := filepath.Join(dir, "result.json")
	if err := os.WriteFile(limited, []byte(`{"type":"result","subtype":"success","is_error":true,"result":"Claude AI usage limit reached|1759262400"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if class, evidence, ok := ClassifyProviderResult(limited); !ok || class != ProviderLimit || !strings.Contains(evidence, "usage limit") {
		t.Fatalf("a declared error carrying the usage-limit line = %s %q %v", class, evidence, ok)
	}
	// The wording a delegate's terminal result carried on 2026-09-06
	// (records/goals/delegate-death-names-the-account-limit.md).
	session := filepath.Join(dir, "session.json")
	if err := os.WriteFile(session, []byte(`{"type":"result","subtype":"success","is_error":true,"api_error_status":429,"result":"You've hit your session limit, resets 12:10am (Europe/Amsterdam)"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if class, evidence, ok := ClassifyProviderResult(session); !ok || class != ProviderLimit || !strings.Contains(evidence, "session limit") {
		t.Fatalf("a declared error carrying the session-limit line = %s %q %v", class, evidence, ok)
	}
	talking := filepath.Join(dir, "success.json")
	if err := os.WriteFile(talking, []byte(`{"is_error":false,"result":"Claude AI usage limit reached|1759262400"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, ok := ClassifyProviderResult(talking); ok {
		t.Fatal("a result that is not an error must not mark an outage, whatever it says")
	}
	// The class feeds the mark like any other provider failure.
	root := t.TempDir()
	if _, err := fixtureObserve(root, ProviderLimit, "Claude AI usage limit reached", "steward", t0); err != nil {
		t.Fatal(err)
	}
	if mark, ok := fixtureStanding(root, t0.Add(time.Minute)); !ok || mark.LastClass != ProviderLimit {
		t.Fatalf("the usage limit did not stand as an outage: %+v %v", mark, ok)
	}
}

// A limit line that names its reset ends the mark then: on 2026-10-03 a
// session limit that reset at 3:30 held every seat start until the horizon
// ran out at 3:47. An invalid zone, an epoch already past and an overload
// keep the horizon alone; a reset over five hours ahead caps the hold, and a
// later failure's own line replaces an earlier reset.
func TestLimitMarkLapsesAtItsNamedReset(t *testing.T) {
	t.Parallel()
	seen := time.Date(2026, 10, 3, 1, 17, 52, 0, time.UTC)
	stands := func(t *testing.T, root string, at time.Time) bool {
		t.Helper()
		_, ok := fixtureStanding(root, at)
		return ok
	}
	root := t.TempDir()
	m, err := fixtureObserve(root, ProviderLimit, "You've hit your session limit · resets 3:30am (Europe/Amsterdam)", "steward-seat", seen)
	if err != nil {
		t.Fatal(err)
	}
	if m.ResetAt != "2026-10-03T01:30:00Z" {
		t.Fatalf("reset = %q, want 3:30 in Amsterdam", m.ResetAt)
	}
	if !stands(t, root, time.Date(2026, 10, 3, 1, 29, 59, 0, time.UTC)) || stands(t, root, time.Date(2026, 10, 3, 1, 30, 0, 0, time.UTC)) {
		t.Fatal("the mark must stand until the named reset and lapse at it")
	}

	epoch := t.TempDir()
	if m, err := fixtureObserve(epoch, ProviderLimit, "Claude AI usage limit reached|"+strconv.FormatInt(seen.Add(5*time.Minute).Unix(), 10), "mission-runner", seen); err != nil || m.ResetAt != "2026-10-03T01:22:52Z" {
		t.Fatalf("epoch reset = %+v, %v", m, err)
	}

	local := t.TempDir()
	m, err = fixtureObserve(local, ProviderLimit, "5-hour limit reached ∙ resets 3pm", "mission-runner", seen)
	reset, parseErr := time.Parse(time.RFC3339, m.ResetAt)
	localSeen := seen.In(time.Local)
	want := time.Date(localSeen.Year(), localSeen.Month(), localSeen.Day(), 15, 0, 0, 0, time.Local)
	if !want.After(seen) {
		want = want.AddDate(0, 0, 1)
	}

	if err != nil || parseErr != nil || !reset.Equal(want) {
		t.Fatalf("a zoneless reset is the next local 3pm with the observed reset retained: %+v, %v, want %s", m, err, want)
	}

	for _, line := range []string{
		"You've hit your usage limit · resets 3:30am (Nowhere/Atlantis)",
		"Claude AI usage limit reached|" + strconv.FormatInt(seen.Add(-time.Hour).Unix(), 10),
	} {
		later := t.TempDir()
		if _, err := fixtureObserve(later, ProviderLimit, line, "steward-seat", seen); err != nil {
			t.Fatal(err)
		}
		if !stands(t, later, seen.Add(Horizon)) || stands(t, later, seen.Add(Horizon+time.Second)) {
			t.Fatalf("%q must keep the horizon alone", line)
		}
	}
	distant := t.TempDir()
	m, err = fixtureObserve(distant, ProviderLimit, "You've hit your usage limit · resets 11pm (Europe/Amsterdam)", "steward-seat", seen)
	if err != nil || m.ResetAt != "2026-10-03T21:00:00Z" {
		t.Fatalf("a distant reset must retain the provider observation: %+v, %v", m, err)
	}
	if !stands(t, distant, seen) || !stands(t, distant, seen.Add(MaxLimitHold-time.Second)) || stands(t, distant, seen.Add(MaxLimitHold)) {
		t.Fatal("a distant reset must hold for five hours and lapse at the cap")
	}
	overload := t.TempDir()
	if m, err := fixtureObserve(overload, "overloaded", "API Error: 529 Overloaded · resets 1am", "mission-runner", seen); err != nil || m.ResetAt != "" {
		t.Fatalf("an overload names no reset: %+v, %v", m, err)
	}

	if m, err := fixtureObserve(root, "overloaded", "API Error: 529", "mission-runner", seen.Add(time.Minute)); err != nil || m.ResetAt != "" {
		t.Fatalf("a later failure's line replaces the reset: %+v, %v", m, err)
	}
	if !stands(t, root, time.Date(2026, 10, 3, 1, 31, 0, 0, time.UTC)) {
		t.Fatal("a mark fed by an overload after the limit stands on its horizon")
	}
}

func TestSessionResetWindow(t *testing.T) {
	t.Parallel()
	zone, err := time.LoadLocation("Europe/Amsterdam")
	if err != nil {
		t.Fatal(err)
	}
	line := "You've hit your session limit · resets 8:50pm (Europe/Amsterdam)"
	for _, minute := range []int{48, 49, 50, 51, 52, 53} {
		seen := time.Date(2026, 10, 4, 20, minute, 0, 0, zone)
		reset, ok := limitReset(ProviderLimit, line, seen)
		want := time.Date(2026, 10, 4, 20, 50, 0, 0, zone)
		if minute >= 50 {
			want = want.AddDate(0, 0, 1)
		}
		if !ok || !reset.Equal(want) {
			t.Fatalf("message at %s: reset=%s, want %s", seen, reset, want)
		}
		mark, err := fixtureObserve(t.TempDir(), ProviderLimit, line, "fixture", seen)

		if err != nil || mark.ResetAt != want.UTC().Format(time.RFC3339) {
			t.Fatalf("message at %s: mark=%+v, error=%v, want %s", seen, mark, err, want)
		}
		retryAt, retry := ResetRetryAt(ProviderLimit, line, seen)
		if retry != (minute <= 52) || retry && !retryAt.Equal(time.Date(2026, 10, 4, 20, 50, 0, 0, zone)) {
			t.Fatalf("message at %s: retry=%v at %s", seen, retry, retryAt)
		}
	}
}

func fixtureObserve(root, class, detail, source string, at time.Time) (Mark, error) {
	home := filepath.Join(root, ".metasystem")
	if _, _, err := lane.Register(home, lane.Layout{Checkout: lane.CheckoutRoot(root), Install: lane.InstallRoot(root)}, "fixture", at); err != nil {
		return Mark{}, err
	}
	return Observe(home, "claude", "fixture-model", class, detail, source, at)
}
func fixtureRead(root string) (Mark, bool) {
	s, err := ReadProviders(filepath.Join(root, ".metasystem"))
	if err != nil {
		return Mark{}, false
	}
	m := s.Current["anthropic"].Mark
	return m, m.ConsecutiveFailures > 0
}
func fixtureStanding(root string, at time.Time) (Mark, bool) {
	s, err := ReadProviders(filepath.Join(root, ".metasystem"))
	if err != nil {
		return Mark{}, false
	}
	return s.Standing("claude", at)
}
func fixtureClear(root string) error {
	m, _ := fixtureRead(root)
	at, _ := time.Parse(time.RFC3339Nano, m.LastAt)
	_, err := Observe(filepath.Join(root, ".metasystem"), "claude", "fixture-model", "", "", "fixture-success", at.Add(time.Nanosecond))
	return err
}
