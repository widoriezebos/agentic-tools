package missionrunner

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPriorContext(t *testing.T) {
	entry := func(outcome string, session any) map[string]any {
		return map[string]any{"outcome": outcome, "sessionId": session}
	}
	type priorCase struct {
		name          string
		log           []any
		wantSession   any
		wantReconcile bool
		wantFailures  int
	}
	cases := []priorCase{
		{"empty log", nil, nil, false, 0},
		{"completed turn resumes its session",
			[]any{entry("completed", "s1")}, "s1", false, 0},
		{"failure after completion counts one",
			[]any{entry("completed", "s1"), entry("failed", "s2")}, "s2", true, 1},
		{"failures accumulate until a completion",
			[]any{entry("failed", "a"), entry("failed", "b")}, "b", true, 2},
		{"unresumable drops the session and is not a failure",
			[]any{entry("completed", "s1"), entry("unresumable", "s2")}, nil, true, 0},
		{"unresumable between failures is skipped",
			[]any{entry("failed", "a"), entry("unresumable", "b"), entry("failed", "c")}, "c", true, 2},
		{"return-ok counts as completion",
			[]any{entry("return-ok", "s1")}, "s1", false, 0},
		{"non-string session reads as none",
			[]any{entry("failed", 7)}, nil, true, 1},
	}
	// The breaker decoupling: an entry recorded as feeding no host-failure
	// blame (a no-witness session mismatch) increments nothing and resets
	// nothing, while witnessed rejections and capped turns count as failures
	// and an accepted return resets the count.
	unwitnessed := func(session any) map[string]any {
		return map[string]any{"outcome": "failed", "sessionId": session, "feedsBreaker": false}
	}
	witnessed := func(session any) map[string]any {
		return map[string]any{"outcome": "failed", "sessionId": session, "feedsBreaker": true}
	}
	cases = append(cases,
		priorCase{"no-witness rejection increments nothing",
			[]any{entry("completed", "s1"), unwitnessed("s1")}, "s1", true, 0},
		priorCase{"no-witness rejection between failures is skipped, not reset",
			[]any{entry("failed", "a"), unwitnessed("b"), entry("failed", "c")}, "c", true, 2},
		priorCase{"witnessed rejection counts as a failure",
			[]any{entry("completed", "s1"), witnessed("s2")}, "s2", true, 1},
		priorCase{"capped turn counts as a failure",
			[]any{entry("completed", "s1"), entry("capped", "s2")}, "s2", true, 1},
	)
	for _, tc := range cases {
		session, reconcile, failures := PriorContext(tc.log)
		if session != tc.wantSession || reconcile != tc.wantReconcile || failures != tc.wantFailures {
			t.Fatalf("%s: got (%v, %v, %d), want (%v, %v, %d)",
				tc.name, session, reconcile, failures, tc.wantSession, tc.wantReconcile, tc.wantFailures)
		}
	}
}

func TestPreviousMetrics(t *testing.T) {
	log := []any{
		map[string]any{"measurement": map[string]any{"metrics": map[string]any{"score": "1", "size": "9"}}},
		map[string]any{"measurement": map[string]any{"metrics": map[string]any{"score": json.Number("2")}}},
		map[string]any{"measurement": nil},
	}
	// The newest turn carrying every declared metric wins; a turn missing one
	// of them is passed over.
	got := PreviousMetrics(log, []string{"score", "size"})
	if got == nil || got["score"] != "1" || got["size"] != "9" {
		t.Fatalf("full metric set: got %v", got)
	}
	got = PreviousMetrics(log, []string{"score"})
	if got == nil || got["score"] != "2" {
		t.Fatalf("newest carrying turn wins with numbers preserved: got %v", got)
	}
	if got := PreviousMetrics(log, []string{"latency"}); got != nil {
		t.Fatalf("absent metric must fall back to the sealed baseline (nil), got %v", got)
	}
}

func TestParseContractText(t *testing.T) {
	text := "# Intent\n\n```mission\na=1\nb=x=y\n\n```\n\n```mission-seal\nsealed.baseline.score=0\n```\n"
	authored, seal, err := parseContractText(text)
	if err != nil {
		t.Fatal(err)
	}
	if authored["a"] != "1" || authored["b"] != "x=y" {
		t.Fatalf("authored values: %v", authored)
	}
	if seal["sealed.baseline.score"] != "0" {
		t.Fatalf("seal values: %v", seal)
	}
	for name, bad := range map[string]string{
		"missing seal":     "```mission\na=1\n```\n",
		"two authored":     "```mission\na=1\n```\n```mission\nb=2\n```\n```mission-seal\nc=3\n```\n",
		"duplicate key":    "```mission\na=1\na=2\n```\n```mission-seal\nc=3\n```\n",
		"separator absent": "```mission\nbare\n```\n```mission-seal\nc=3\n```\n",
	} {
		if _, _, err := parseContractText(bad); err == nil {
			t.Fatalf("%s: want error", name)
		}
	}
}

func TestFenceReachedAt(t *testing.T) {
	now := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)
	fences := func(started string, cycles int, reservations ...string) map[string]any {
		reserved := map[string]any{}
		for _, job := range reservations {
			reserved[job] = map[string]any{}
		}
		return map[string]any{"startedAt": started, "cycles": json.Number("0"), "reservations": reserved,
			"cyclesInt": cycles}
	}
	values := map[string]string{
		"fence.wall-clock-hours": "2",
		"fence.cycles":           "3",
		"fence.jobs":             "2",
		"fence.concurrency":      "1",
	}
	base := fences(now.Add(-time.Hour).Format(time.RFC3339), 0)
	base["cycles"] = json.Number("1")
	reached, _, err := fenceReachedAt(base, values, nil, now)
	if err != nil || reached {
		t.Fatalf("under every fence: got %v, %v", reached, err)
	}

	old := fences(now.Add(-3*time.Hour).Format(time.RFC3339), 0)
	old["cycles"] = json.Number("0")
	if reached, _, err = fenceReachedAt(old, values, nil, now); err != nil || !reached {
		t.Fatalf("wall clock fence: got %v, %v", reached, err)
	}

	spent := fences(now.Format(time.RFC3339), 0)
	spent["cycles"] = json.Number("3")
	if reached, _, err = fenceReachedAt(spent, values, nil, now); err != nil || !reached {
		t.Fatalf("cycle fence: got %v, %v", reached, err)
	}

	jobs := fences(now.Format(time.RFC3339), 0, "j1", "j2")
	jobs["cycles"] = json.Number("0")
	status := map[string]string{"j1": "completed", "j2": "failed"}
	if reached, _, err = fenceReachedAt(jobs, values, status, now); err != nil || !reached {
		t.Fatalf("job-count fence: got %v, %v", reached, err)
	}

	// A reservation with no readable record counts as active: losing sight
	// of a job must never relax the concurrency fence.
	lost := fences(now.Format(time.RFC3339), 0, "ghost")
	lost["cycles"] = json.Number("0")
	looseValues := map[string]string{
		"fence.wall-clock-hours": "2", "fence.cycles": "3",
		"fence.jobs": "5", "fence.concurrency": "1",
	}
	if reached, _, err = fenceReachedAt(lost, looseValues, map[string]string{}, now); err != nil || !reached {
		t.Fatalf("lost-record concurrency fence: got %v, %v", reached, err)
	}
	settled := map[string]string{"ghost": "completed"}
	if reached, _, err = fenceReachedAt(lost, looseValues, settled, now); err != nil || reached {
		t.Fatalf("terminal job clears concurrency: got %v, %v", reached, err)
	}

	bad := fences("not-a-time", 0)
	if _, _, err := fenceReachedAt(bad, values, nil, now); err == nil {
		t.Fatal("invalid startedAt: want error")
	}
}

func TestStaleTurnOpen(t *testing.T) {
	cases := []struct {
		turn map[string]any
		want bool
	}{
		{map[string]any{"status": "pending"}, true},
		{map[string]any{"status": "running"}, true},
		{map[string]any{"status": "failed", "outcome": "running"}, true},
		{map[string]any{"status": "completed", "outcome": "completed"}, false},
		{map[string]any{"status": "failed", "outcome": "failed"}, false},
		{map[string]any{}, false},
	}
	for _, tc := range cases {
		if got := staleTurnOpen(tc.turn); got != tc.want {
			t.Fatalf("turn %v: got %v, want %v", tc.turn, got, tc.want)
		}
	}
}

func TestMarkerHoldsOnlyOwner(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "lease.d")
	if err := os.Mkdir(marker, 0o755); err != nil {
		t.Fatal(err)
	}
	read := func() []os.DirEntry {
		entries, err := os.ReadDir(marker)
		if err != nil {
			t.Fatal(err)
		}
		return entries
	}
	if !markerHoldsOnlyOwner(marker, read()) {
		t.Fatal("empty marker must be removable")
	}
	if err := os.WriteFile(filepath.Join(marker, "owner.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !markerHoldsOnlyOwner(marker, read()) {
		t.Fatal("owner.json alone must be removable")
	}
	if err := os.WriteFile(filepath.Join(marker, "stray"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if markerHoldsOnlyOwner(marker, read()) {
		t.Fatal("a stray file must refuse the cleanup")
	}
	if err := os.Remove(filepath.Join(marker, "stray")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(marker, "owner.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(marker, "owner.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	if markerHoldsOnlyOwner(marker, read()) {
		t.Fatal("a directory named owner.json must refuse the cleanup")
	}
}

func TestCleanupStaleLeaseMarker(t *testing.T) {
	root := t.TempDir()
	engine := NewEngine(root, "m1")
	marker := filepath.Join(engine.missionDir(), "lease.d")
	if err := os.MkdirAll(marker, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(marker, "owner.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := engine.cleanupStaleLease(); err != nil {
		t.Fatalf("clean marker: %v", err)
	}
	if pathExists(marker) {
		t.Fatal("stale marker was not removed")
	}

	if err := os.MkdirAll(marker, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(marker, "notes.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := engine.cleanupStaleLease()
	if err == nil || !strings.Contains(err.Error(), "unexpected files") {
		t.Fatalf("stray marker content: got %v", err)
	}
	if !pathExists(marker) {
		t.Fatal("refused cleanup must leave the marker for a human")
	}
}

func TestTurnCapFromDoc(t *testing.T) {
	if cap, err := turnCapFromDoc(map[string]any{"turnCapMin": json.Number("3")}); err != nil || cap != 3*time.Minute {
		t.Fatalf("number literal: got %v, %v", cap, err)
	}
	if cap, err := turnCapFromDoc(map[string]any{"turnCapMin": 2}); err != nil || cap != 2*time.Minute {
		t.Fatalf("int: got %v, %v", cap, err)
	}
	if _, err := turnCapFromDoc(map[string]any{}); err == nil {
		t.Fatal("missing cap: want error")
	}
	if _, err := turnCapFromDoc(map[string]any{"turnCapMin": "soon"}); err == nil {
		t.Fatal("non-numeric cap: want error")
	}
}

func TestHostStartVerified(t *testing.T) {
	tag := "metasystem-host-m1-t1-abcd"
	command := "hosts/fake.sh start-turn --instance-tag " + tag
	if !hostStartVerified(42, 42, command, tag, false) {
		t.Fatal("own group with tag must verify")
	}
	if hostStartVerified(42, 41, command, tag, false) {
		t.Fatal("a host that does not lead its group must not verify")
	}
	if hostStartVerified(42, 42, "hosts/fake.sh start-turn", tag, false) {
		t.Fatal("a command without the minted tag must not verify")
	}
	if hostStartVerified(42, 42, command, tag, true) {
		t.Fatal("the forced-unverified fixture path must never verify")
	}
}

func TestStartSignalShape(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runner-start-test.json")
	if err := writeStartSignal(path, false, nil, "refused"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := "{\n  \"error\": \"refused\",\n  \"turnId\": null,\n  \"verified\": false\n}\n"
	if string(data) != want {
		t.Fatalf("start signal bytes changed:\n%s", data)
	}
}

func TestRunnerRecordAndHeartbeatShape(t *testing.T) {
	engine := NewEngine(t.TempDir(), "m1")
	recordPath, heartbeatPath, _ := engine.runnerPaths()
	if err := atomicWriteJSON(recordPath, engine.runnerRecord(41, 41, 1700000000, "tag-x")); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(recordPath)
	if err != nil {
		t.Fatal(err)
	}
	// Drivers grep these exact spellings out of the record.
	for _, want := range []string{`"status": "running"`, `"missionId": "m1"`, `"pidStartedAt": 1700000000`, `"endedAt": null`} {
		if !strings.Contains(string(data), want) {
			t.Fatalf("runner record lost %s:\n%s", want, data)
		}
	}
	if err := engine.heartbeat(nil); err != nil {
		t.Fatal(err)
	}
	heartbeat, err := readJSONDoc(heartbeatPath)
	if err != nil {
		t.Fatal(err)
	}
	if heartbeat["function"] != "mission-runner" || heartbeat["turnId"] != nil || heartbeat["instanceTag"] != "tag-x" {
		t.Fatalf("heartbeat shape changed: %v", heartbeat)
	}
}

func TestStatusMissingState(t *testing.T) {
	if code := NewEngine(t.TempDir(), "m-status").Status(); code != 7 {
		t.Fatalf("missing state must map to exit 7, got %d", code)
	}
}
