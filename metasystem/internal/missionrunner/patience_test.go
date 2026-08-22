package missionrunner

import (
	"strings"
	"testing"
)

// Test fixtures build jobRecords directly: the derivation is pure over
// (floors, records, turn log, in-flight certified), which is exactly what
// the design demands of it.

func patienceRecord(id string, fields map[string]any) jobRecord {
	doc := map[string]any{"jobId": id, "status": "completed", "role": "implementer",
		"runtime": "codex", "effectiveModel": "gpt-5.6-sol"}
	for key, value := range fields {
		doc[key] = value
	}
	return jobRecord{path: "/jobs/" + id + ".json", doc: doc}
}

func certEntry(jobID string) any {
	return map[string]any{"jobId": jobID, "verdict": "accepted", "evidence": "ran: reviewed the return"}
}

var solFloors = patienceFloors{"implementer|codex|gpt-5-6-sol": 2}

// Unconfigured missions evaluate nothing: byte-identical structurally.
func TestPatienceUnconfiguredIsSilent(t *testing.T) {
	records := []jobRecord{patienceRecord("a", nil)}
	if got := patienceEvaluate(patienceFloors{}, records, nil, nil); got != nil {
		t.Fatalf("unconfigured mission produced annotations: %v", got)
	}
}

// A breach books when the count strictly exceeds the floor; at the floor it
// stays silent.
func TestPatienceThresholdStrictlyExceeds(t *testing.T) {
	records := []jobRecord{
		patienceRecord("root-1", map[string]any{"endedAt": "2026-08-12T10:00:00Z"}),
		patienceRecord("root-1-r2", map[string]any{"parentJob": "root-1", "endedAt": "2026-08-12T11:00:00Z"}),
	}
	if got := patienceEvaluate(solFloors, records, nil, nil); len(got) != 0 {
		t.Fatalf("count at floor must stay silent: %v", got)
	}
	records = append(records, patienceRecord("root-1-r3",
		map[string]any{"parentJob": "root-1-r2", "endedAt": "2026-08-12T12:00:00Z"}))
	got := patienceEvaluate(solFloors, records, nil, nil)
	if len(got) != 1 || got[0] != "Patience: chain=root-1 rounds=3 floor=2" {
		t.Fatalf("breach not booked: %v", got)
	}
}

// Certification resets the streak: alternating witness/barren never
// breaches, and a certification in the CURRENT conclusion
// suppresses the breach in the SAME booking.
func TestPatienceCertificationResetsStreak(t *testing.T) {
	records := []jobRecord{
		patienceRecord("root-1", map[string]any{"endedAt": "2026-08-12T10:00:00Z"}),
		patienceRecord("root-1-r2", map[string]any{"parentJob": "root-1", "endedAt": "2026-08-12T11:00:00Z"}),
		patienceRecord("root-1-r3", map[string]any{"parentJob": "root-1-r2", "endedAt": "2026-08-12T12:00:00Z"}),
	}
	turnLog := []any{map[string]any{"certified": []any{certEntry("root-1-r2")}}}
	if got := patienceEvaluate(solFloors, records, turnLog, nil); len(got) != 0 {
		t.Fatalf("witnessed round did not reset the streak: %v", got)
	}
	// Same suppression through the in-flight conclusion alone.
	if got := patienceEvaluate(solFloors, records, nil, []any{certEntry("root-1-r3")}); len(got) != 0 {
		t.Fatalf("in-flight certification did not suppress: %v", got)
	}
	// Rejected and empty-evidence certifications witness nothing.
	rejected := []any{map[string]any{"jobId": "root-1-r2", "verdict": "rejected", "evidence": "x"}}
	empty := []any{map[string]any{"jobId": "root-1-r2", "verdict": "accepted", "evidence": "   "}}
	records = append(records, patienceRecord("root-1-r4",
		map[string]any{"parentJob": "root-1-r3", "endedAt": "2026-08-12T13:00:00Z"}))
	for name, certified := range map[string][]any{"rejected": rejected, "empty": empty} {
		got := patienceEvaluate(solFloors, records, []any{map[string]any{"certified": certified}}, nil)
		if len(got) != 1 || !strings.HasPrefix(got[0], "Patience: chain=root-1 rounds=4") {
			t.Fatalf("%s certification counted as witness: %v", name, got)
		}
	}
}

// A certification of a job that never started cannot erase a drought,
// and foreign jobIds are ignored.
func TestPatienceWitnessMustBeStartedAndOwned(t *testing.T) {
	records := []jobRecord{
		patienceRecord("root-1", map[string]any{"endedAt": "2026-08-12T10:00:00Z"}),
		patienceRecord("root-1-r2", map[string]any{"parentJob": "root-1", "endedAt": "2026-08-12T11:00:00Z"}),
		patienceRecord("root-1-r3", map[string]any{"parentJob": "root-1-r2", "endedAt": "2026-08-12T12:00:00Z",
			"status": "failed", "error": "handshake_timeout", "effectiveModel": ""}),
	}
	// The husk (r3) is newest but never started; certifying it must not
	// reset the two started barren rounds... which are exactly at the floor,
	// so add one more started round to make a real breach.
	records = append(records, patienceRecord("root-1-r4",
		map[string]any{"parentJob": "root-1-r3", "endedAt": "2026-08-12T13:00:00Z"}))
	certified := []any{certEntry("root-1-r3"), certEntry("ghost-job")}
	got := patienceEvaluate(solFloors, records, []any{map[string]any{"certified": certified}}, nil)
	if len(got) != 1 || !strings.HasPrefix(got[0], "Patience: chain=root-1 rounds=3") {
		t.Fatalf("husk or foreign certification erased the drought: %v", got)
	}
}

// Started-work proof: husks and
// pending-cancelled never count; started cancellations count; the
// never-started vocabulary is trumped by spend-proving usage; a
// failed-handshake record with a patched effectiveModel never counts.
func TestPatienceStartedPredicate(t *testing.T) {
	cases := []struct {
		name    string
		fields  map[string]any
		started bool
	}{
		{"completed structural", map[string]any{"status": "completed"}, true},
		{"timeout structural", map[string]any{"status": "timeout"}, true},
		{"pending-cancelled husk", map[string]any{"status": "cancelled", "effectiveModel": ""}, false},
		{"cancelled after running", map[string]any{"status": "cancelled"}, true},
		{"failed handshake with patched model", map[string]any{"status": "failed",
			"error": "handshake_missing_session_id"}, false},
		{"failed permissions mismatch", map[string]any{"status": "failed",
			"error": "permissions_mismatch:network"}, false},
		{"post-run mismatch with spend", map[string]any{"status": "failed",
			"error": "handshake_missing_session_id",
			"usage": map[string]any{"outputTokens": float64(12)}}, true},
		{"post-running resume collision", map[string]any{"status": "failed",
			"error": "resume_collision"}, true},
		{"abandoned setup", map[string]any{"status": "failed", "error": "abandoned-setup",
			"effectiveModel": ""}, false},
		{"launch failed", map[string]any{"status": "failed", "error": "launch_failed",
			"effectiveModel": ""}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			record := patienceRecord("job-x", tc.fields)
			if got := patienceStarted(record); got != tc.started {
				t.Fatalf("started=%v want %v for %v", got, tc.started, tc.fields)
			}
		})
	}
}

// Spend proof: presence and
// zero prove nothing, nested positives prove spend, availability plays no
// part.
func TestPatienceUsageProvesSpend(t *testing.T) {
	cases := []struct {
		name  string
		usage any
		spend bool
	}{
		{"all null", map[string]any{"availability": "native", "inputTokens": nil, "cost": nil, "providerUnits": nil}, false},
		{"all zero", map[string]any{"inputTokens": float64(0), "outputTokens": float64(0), "cost": nil}, false},
		{"nested zero cost", map[string]any{"cost": map[string]any{"amount": float64(0)}}, false},
		{"nested zero unit", map[string]any{"providerUnits": []any{map[string]any{"name": "acu", "value": float64(0)}}}, false},
		{"positive tokens", map[string]any{"outputTokens": float64(3)}, true},
		{"positive nested cost", map[string]any{"cost": map[string]any{"amount": float64(0.02)}}, true},
		{"tokens unavailable, positive units", map[string]any{"availability": "unavailable",
			"providerUnits": []any{map[string]any{"name": "acu", "value": float64(1.5)}}}, true},
		{"not an object", "12 tokens", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := patienceUsageProvesSpend(tc.usage); got != tc.spend {
				t.Fatalf("spend=%v want %v for %v", got, tc.spend, tc.usage)
			}
		})
	}
}

// Participation boundary: identity
// mismatches and unknown statuses are excluded and counted in the aggregate
// line; lawful running jobs participate but never count.
func TestPatienceParticipationBoundary(t *testing.T) {
	records := []jobRecord{
		patienceRecord("root-1", map[string]any{"endedAt": "2026-08-12T10:00:00Z"}),
		patienceRecord("root-1-r2", map[string]any{"parentJob": "root-1", "endedAt": "2026-08-12T11:00:00Z"}),
		patienceRecord("root-1-r3", map[string]any{"parentJob": "root-1-r2", "endedAt": "2026-08-12T12:00:00Z"}),
		// Identity mismatch: recorded id differs from the filename stem.
		{path: "/jobs/other-name.json", doc: map[string]any{"jobId": "job-b", "status": "completed"}},
		// Unknown status.
		{path: "/jobs/weird.json", doc: map[string]any{"jobId": "weird", "status": "vanished"}},
		// Lawful running job in the same chain: in flight, never counted.
		patienceRecord("root-1-r5", map[string]any{"parentJob": "root-1-r3", "status": "running"}),
	}
	got := patienceEvaluate(solFloors, records, nil, nil)
	want := map[string]bool{
		"Patience: chain=root-1 rounds=3 floor=2": true,
		"Patience: excluded=2":                    true,
	}
	if len(got) != 2 || !want[got[0]] || !want[got[1]] {
		t.Fatalf("boundary misdrawn: %v", got)
	}
	// The running job later terminates uncertified: it counts.
	records[5].doc["status"] = "completed"
	records[5].doc["endedAt"] = "2026-08-12T13:00:00Z"
	got = patienceEvaluate(solFloors, records, nil, nil)
	if len(got) != 2 || got[0] != "Patience: chain=root-1 rounds=4 floor=2" {
		t.Fatalf("terminal transition did not count: %v", got)
	}
}

// Branch tolerance and round numbers: sibling follow-ups aggregate
// into one set; duplicate round
// numbers are harmless.
func TestPatienceBranchesAndRoundNumbers(t *testing.T) {
	records := []jobRecord{
		patienceRecord("root-1", map[string]any{"endedAt": "2026-08-12T10:00:00Z", "round": float64(1)}),
		patienceRecord("sib-a", map[string]any{"parentJob": "root-1", "endedAt": "2026-08-12T11:00:00Z", "round": float64(2)}),
		patienceRecord("sib-b", map[string]any{"parentJob": "root-1", "endedAt": "2026-08-12T12:00:00Z", "round": float64(2)}),
	}
	got := patienceEvaluate(solFloors, records, nil, nil)
	if len(got) != 1 || got[0] != "Patience: chain=root-1 rounds=3 floor=2" {
		t.Fatalf("branch set miscounted: %v", got)
	}
}

// Orphans: broken lineage becomes a floor-independent
// damage report, emitted despite any positive floor — in configured missions
// only.
func TestPatienceOrphanReports(t *testing.T) {
	records := []jobRecord{
		patienceRecord("lost-child", map[string]any{"parentJob": "never-existed",
			"endedAt": "2026-08-12T10:00:00Z"}),
	}
	got := patienceEvaluate(solFloors, records, nil, nil)
	if len(got) != 1 || got[0] != "Patience: orphan=lost-child rounds=1" {
		t.Fatalf("orphan not reported: %v", got)
	}
	if got := patienceEvaluate(patienceFloors{}, records, nil, nil); got != nil {
		t.Fatalf("orphan reported in an unconfigured mission: %v", got)
	}
}

// Two-chain isolation: certifying a job in chain B resets B and
// leaves chain A's breach intact.
func TestPatienceTwoChainIsolation(t *testing.T) {
	records := []jobRecord{
		patienceRecord("chain-a", map[string]any{"endedAt": "2026-08-12T10:00:00Z"}),
		patienceRecord("chain-a-r2", map[string]any{"parentJob": "chain-a", "endedAt": "2026-08-12T11:00:00Z"}),
		patienceRecord("chain-a-r3", map[string]any{"parentJob": "chain-a-r2", "endedAt": "2026-08-12T12:00:00Z"}),
		patienceRecord("chain-b", map[string]any{"endedAt": "2026-08-12T10:30:00Z"}),
		patienceRecord("chain-b-r2", map[string]any{"parentJob": "chain-b", "endedAt": "2026-08-12T13:00:00Z"}),
	}
	turnLog := []any{map[string]any{"certified": []any{certEntry("chain-b-r2")}}}
	got := patienceEvaluate(solFloors, records, turnLog, nil)
	if len(got) != 1 || got[0] != "Patience: chain=chain-a rounds=3 floor=2" {
		t.Fatalf("chain isolation broken: %v", got)
	}
}

// Closed chains leave evaluation at derivation time.
func TestPatienceClosedChainExcluded(t *testing.T) {
	records := []jobRecord{
		patienceRecord("root-1", map[string]any{"endedAt": "2026-08-12T10:00:00Z", "chainClosed": true}),
		patienceRecord("root-1-r2", map[string]any{"parentJob": "root-1", "endedAt": "2026-08-12T11:00:00Z"}),
		patienceRecord("root-1-r3", map[string]any{"parentJob": "root-1-r2", "endedAt": "2026-08-12T12:00:00Z"}),
	}
	if got := patienceEvaluate(solFloors, records, nil, nil); len(got) != 0 {
		t.Fatalf("closed chain still evaluated: %v", got)
	}
}

// Selection:
// sentinels are not evidence; the triple comes from one qualifying record;
// pre-witness history selects nothing; requested-model fallback is
// streak-scoped.
func TestPatienceFloorSelection(t *testing.T) {
	floors := patienceFloors{
		"implementer|codex|gpt-5-6-sol": 1,
		"verifier|codex|gpt-5-6-sol":    9,
	}
	// The newest streak job carries a sentinel: it does not qualify, the
	// next-newest real model drives rows 1-2.
	records := []jobRecord{
		patienceRecord("root-1", map[string]any{"endedAt": "2026-08-12T10:00:00Z"}),
		patienceRecord("root-1-r2", map[string]any{"parentJob": "root-1", "endedAt": "2026-08-12T11:00:00Z",
			"effectiveModel": "unobserved"}),
	}
	got := patienceEvaluate(floors, records, nil, nil)
	if len(got) != 1 || got[0] != "Patience: chain=root-1 rounds=2 floor=1" {
		t.Fatalf("sentinel handling wrong: %v", got)
	}
	// An invalid role on the newest model-bearing record falls through to the
	// next qualifying record, never assembling a cross-record triple.
	records = []jobRecord{
		patienceRecord("root-1", map[string]any{"endedAt": "2026-08-12T10:00:00Z"}),
		patienceRecord("root-1-r2", map[string]any{"parentJob": "root-1", "endedAt": "2026-08-12T11:00:00Z",
			"role": "Broken Role"}),
	}
	got = patienceEvaluate(floors, records, nil, nil)
	if len(got) != 1 || got[0] != "Patience: chain=root-1 rounds=2 floor=1" {
		t.Fatalf("invalid-role fallthrough wrong: %v", got)
	}
	// No effective evidence anywhere in the streak: requestedModel on a
	// streak record drives rows 3-4.
	records = []jobRecord{
		patienceRecord("root-1", map[string]any{"endedAt": "2026-08-12T10:00:00Z",
			"effectiveModel": "", "requestedModel": "gpt-5.6-sol",
			"status": "failed", "error": "worktree-dirty",
			"usage": map[string]any{"outputTokens": float64(5)}}),
		patienceRecord("root-1-r2", map[string]any{"parentJob": "root-1", "endedAt": "2026-08-12T11:00:00Z",
			"effectiveModel": "", "requestedModel": "gpt-5.6-sol",
			"status": "failed", "error": "worktree-dirty",
			"usage": map[string]any{"outputTokens": float64(5)}}),
	}
	got = patienceEvaluate(floors, records, nil, nil)
	if len(got) != 1 || got[0] != "Patience: chain=root-1 rounds=2 floor=1" {
		t.Fatalf("requested-model fallback wrong: %v", got)
	}
	// A clean model with no entry is configured-nothing: infinite, silent.
	records = []jobRecord{
		patienceRecord("root-1", map[string]any{"endedAt": "2026-08-12T10:00:00Z",
			"effectiveModel": "gpt-5.3-codex"}),
		patienceRecord("root-1-r2", map[string]any{"parentJob": "root-1", "endedAt": "2026-08-12T11:00:00Z",
			"effectiveModel": "gpt-5.3-codex"}),
	}
	if got := patienceEvaluate(floors, records, nil, nil); len(got) != 0 {
		t.Fatalf("configured-nothing pair breached: %v", got)
	}
}

// Ranking and bound: breach
// distance beats raw count, orphans rank after breaches, and the combined
// set bounds at 19 detail + 1 overflow.
func TestPatienceRankingAndBound(t *testing.T) {
	floors := patienceFloors{
		"implementer|codex|gpt-5-6-sol": 2,
		"verifier|codex|gpt-5-6-sol":    100,
	}
	var records []jobRecord
	// Chain far past a small floor: 6 rounds against floor 2 (distance 4).
	prev := ""
	for i := 0; i < 6; i++ {
		id := "deep-" + string(rune('a'+i))
		fields := map[string]any{"endedAt": "2026-08-12T10:0" + string(rune('0'+i)) + ":00Z"}
		if prev != "" {
			fields["parentJob"] = prev
		}
		records = append(records, patienceRecord(id, fields))
		prev = id
	}
	// Chain barely past a big floor: 101 rounds against floor 100
	// (distance 1) — a count-descending comparator would rank it first.
	prev = ""
	for i := 0; i < 101; i++ {
		id := "wide-" + string(rune('a'+i/26)) + string(rune('a'+i%26))
		fields := map[string]any{"role": "verifier", "endedAt": "2026-08-12T11:00:00Z"}
		if prev != "" {
			fields["parentJob"] = prev
		}
		records = append(records, patienceRecord(id, fields))
		prev = id
	}
	got := patienceEvaluate(floors, records, nil, nil)
	if len(got) != 2 {
		t.Fatalf("expected two breach lines: %v", got)
	}
	if !strings.HasPrefix(got[0], "Patience: chain=deep-a rounds=6") ||
		!strings.HasPrefix(got[1], "Patience: chain=wide-aa rounds=101") {
		t.Fatalf("distance ranking wrong: %v", got)
	}

	// Bound: 25 orphans → 19 detail + 1 overflow counting the remainder.
	records = nil
	for i := 0; i < 25; i++ {
		id := "orphan-" + string(rune('a'+i/5)) + string(rune('a'+i%5))
		records = append(records, patienceRecord(id, map[string]any{
			"parentJob": "gone-" + id, "endedAt": "2026-08-12T10:00:00Z"}))
	}
	got = patienceEvaluate(solFloors, records, nil, nil)
	if len(got) != 20 {
		t.Fatalf("bound not enforced: %d lines", len(got))
	}
	if got[19] != "Patience overflow: chains=6" {
		t.Fatalf("overflow wrong: %v", got[19])
	}
}

// Ordering totality over damaged timestamps: missing
// and unparseable stamps share the oldest bucket; jobId breaks the tie
// deterministically.
func TestPatienceOrderingDeterministic(t *testing.T) {
	records := []jobRecord{
		patienceRecord("b-job", map[string]any{"endedAt": "not-a-time"}),
		patienceRecord("a-job", nil),
		patienceRecord("c-job", map[string]any{"endedAt": "2026-08-12T10:00:00Z"}),
	}
	sortJobsNewestFirst(records)
	ids := []string{}
	for _, record := range records {
		id, _ := record.doc["jobId"].(string)
		ids = append(ids, id)
	}
	if ids[0] != "c-job" || ids[1] != "b-job" || ids[2] != "a-job" {
		t.Fatalf("ordering not total: %v", ids)
	}
}

// Floor parsing reads only well-formed patience.rounds entries.
func TestParsePatienceFloors(t *testing.T) {
	floors := parsePatienceFloors(map[string]string{
		"patience.rounds.implementer.codex.gpt-5-6-sol": "4",
		"patience.rounds.bad":                           "4",
		"patience.rounds.a.b.c":                         "0",
		"cap.min.codex.gpt-5-6-sol":                     "30",
	})
	if len(floors) != 1 || floors["implementer|codex|gpt-5-6-sol"] != 4 {
		t.Fatalf("floors misparsed: %v", floors)
	}
}
