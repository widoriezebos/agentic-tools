package mission

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// getOwnPgid reads this test process's own live process group, the one group
// guaranteed to probe as existing.
func getOwnPgid() (int64, error) {
	pgid, err := unix.Getpgid(0)
	return int64(pgid), err
}

var fixedNow = time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)

func withFixedClock(t *testing.T) {
	t.Helper()
	old := clock
	clock = func() time.Time { return fixedNow }
	t.Cleanup(func() { clock = old })
}

const fenceContract = "```mission\n" +
	"fence.wall-clock-hours=12\n" +
	"fence.cycles=1\n" +
	"fence.jobs=5\n" +
	"fence.concurrency=2\n" +
	"fence.job-cap-min=240\n" +
	"cap.min.codex.gpt-5-6-sol=180\n" +
	"```\n"

// fenceEnv writes a sealed contract and its fence counters, and returns the repo.
func fenceEnv(t *testing.T) (repo, mission string) {
	t.Helper()
	withFixedClock(t)
	repo = t.TempDir()
	mission = "demo"
	plans := filepath.Join(repo, "plans")
	if err := os.MkdirAll(plans, 0o755); err != nil {
		t.Fatal(err)
	}
	contractPath := filepath.Join(plans, "mission-demo.contract.md")
	writeText(t, contractPath, fenceContract)
	data, _ := os.ReadFile(contractPath)
	approved := sha256Hex(string(data))

	fencesDir := missionDir(repo, mission)
	if err := os.MkdirAll(fencesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	fences := map[string]any{
		"schemaVersion": 1, "missionId": mission, "startedAt": fenceNowISO(),
		"cycles": 0, "reservations": map[string]any{}, "approvedContractSha256": approved,
	}
	if err := atomicWriteJSON(filepath.Join(fencesDir, "fences.json"), fences); err != nil {
		t.Fatal(err)
	}
	return repo, mission
}

func TestContractValidationRejectsNonCanonicalCap(t *testing.T) {
	// A cap key whose model segment is not canonical.
	bad := "```mission\nfence.wall-clock-hours=12\nfence.cycles=1\nfence.jobs=5\nfence.concurrency=2\nfence.job-cap-min=240\ncap.min.codex.GPT_5=180\n```\n"
	if _, err := contractValuesFromBytes([]byte(bad)); err == nil ||
		!strings.Contains(err.Error(), "not canonical") {
		t.Fatalf("a non-canonical cap key must be rejected, got %v", err)
	}
	// A contract missing a universal fence.
	missing := "```mission\nfence.cycles=1\nfence.jobs=5\nfence.concurrency=2\nfence.job-cap-min=240\n```\n"
	if _, err := contractValuesFromBytes([]byte(missing)); err == nil ||
		!strings.Contains(err.Error(), "universal lifecycle fence") {
		t.Fatalf("a missing fence must be rejected, got %v", err)
	}
}

func TestAuthorizeCapUsesPairCap(t *testing.T) {
	repo, mission := fenceEnv(t)
	result, err := AuthorizeCap(repo, mission, "job-1", "codex", "gpt-5-6-sol", "", nil)
	if err != nil {
		t.Fatalf("authorize should succeed: %v", err)
	}
	if cap, _ := intValue(result["capMin"]); cap != 180 {
		t.Fatalf("cap should be the signed pair cap 180, got %v", result["capMin"])
	}
	source, _ := result["source"].(map[string]any)
	if source["rule"] != "contract-pair" {
		t.Fatalf("source rule should be contract-pair: %v", source)
	}
	// The reservation was recorded.
	fences, _ := readJSONObjectFile(filepath.Join(missionDir(repo, mission), "fences.json"))
	if _, ok := reservationsMap(fences)["job-1"]; !ok {
		t.Fatal("the job reservation should be recorded")
	}
}

func TestAuthorizeCapRefusesAboveSigned(t *testing.T) {
	repo, mission := fenceEnv(t)
	requested := 300
	if _, err := AuthorizeCap(repo, mission, "job-1", "codex", "gpt-5-6-sol", "", &requested); err == nil ||
		!strings.Contains(err.Error(), "above signed") {
		t.Fatalf("a request above the signed cap must be refused, got %v", err)
	}
}

func TestFMA_R2_MissionCapSourceBypass(t *testing.T) {
	repo, mission := fenceEnv(t)
	contract := strings.Replace(fenceContract,
		"cap.min.codex.gpt-5-6-sol=180",
		"cap.min.claude.claude-fable-5=30", 1)
	contractPath := filepath.Join(repo, "plans", "mission-demo.contract.md")
	writeText(t, contractPath, contract)
	fencesPath := filepath.Join(missionDir(repo, mission), "fences.json")
	fences, err := readJSONObjectFile(fencesPath)
	if err != nil {
		t.Fatal(err)
	}
	fences["approvedContractSha256"] = sha256Hex(contract)
	if err := atomicWriteJSON(fencesPath, fences); err != nil {
		t.Fatal(err)
	}

	result, err := AuthorizeCap(repo, mission, "job-source", "claude", "claude-fable-5-1", "claude-fable-5", nil)
	if err != nil {
		t.Fatal(err)
	}
	if capMin, _ := intValue(result["capMin"]); capMin != 30 {
		t.Fatalf("source pair authorized %d minutes, want 30 rather than the general mission cap", capMin)
	}
	source, _ := result["source"].(map[string]any)
	if source["rule"] != "contract-pair" {
		t.Fatalf("source pair rule = %v, want contract-pair", source)
	}

	conf := filepath.Join(t.TempDir(), "metasystem.conf")
	writeText(t, conf, "metasystem.runtimes=claude\n")
	t.Setenv("METASYSTEM_CAP_MIN_CLAUDE_CLAUDE_FABLE_5", "30")
	if err := dispatchcore.RefuseUnsignedMissionCap(conf, "implementer", "claude", "claude-fable-5-1", "claude-fable-5"); err == nil ||
		!strings.Contains(err.Error(), "unsigned env key cap.min.claude.claude-fable-5") {
		t.Fatalf("unsigned source-only mission cap was not refused: %v", err)
	}
}

func TestReserveCycleTripsFenceAndBatchesAsk(t *testing.T) {
	repo, mission := fenceEnv(t)
	// fence.cycles=1: the first cycle is allowed.
	if err := ReserveCycle(repo, mission); err != nil {
		t.Fatalf("the first cycle should be allowed: %v", err)
	}
	// The second trips the cycles fence and writes a batched ask.
	err := ReserveCycle(repo, mission)
	if err == nil || !strings.Contains(err.Error(), "cycles") {
		t.Fatalf("the cycle fence should refuse the second cycle, got %v", err)
	}
	asks, _ := filepath.Glob(filepath.Join(missionDir(repo, mission), "asks", "fence-bound*.json"))
	if len(asks) != 1 {
		t.Fatalf("a single batched ask should exist, got %d", len(asks))
	}
	ask, _ := readJSONObjectFile(asks[0])
	if q, _ := ask["question"].(string); !strings.Contains(q, "`cycles`") {
		t.Fatalf("the ask should name the cycles fence: %q", q)
	}
}

func TestAggregateUsageSumsTerminalJobs(t *testing.T) {
	repo, mission := fenceEnv(t)
	jobs := filepath.Join(repo, "artifacts", "agents", "jobs")
	if err := os.MkdirAll(jobs, 0o755); err != nil {
		t.Fatal(err)
	}
	writeText(t, filepath.Join(jobs, "job-a.json"),
		`{"jobId":"job-a","mission":"demo","status":"completed","runtime":"codex","usage":{"outputTokens":100}}`)
	writeText(t, filepath.Join(jobs, "job-b.json"),
		`{"jobId":"job-b","mission":"demo","status":"completed","runtime":"codex","usage":{"outputTokens":40}}`)
	// A running job is not aggregated.
	writeText(t, filepath.Join(jobs, "job-c.json"),
		`{"jobId":"job-c","mission":"demo","status":"running","runtime":"codex","usage":{"outputTokens":999}}`)

	if err := AggregateUsage(repo, mission); err != nil {
		t.Fatalf("aggregate: %v", err)
	}
	usage, _ := readJSONObjectFile(filepath.Join(missionDir(repo, mission), "usage.json"))
	units, _ := usage["units"].([]any)
	var total float64
	for _, u := range units {
		item, _ := u.(map[string]any)
		if item["unit"] == "tokens.outputTokens" {
			total, _ = floatValue(item["value"])
		}
	}
	if total != 140 {
		t.Fatalf("terminal output tokens should sum to 140, got %v (units=%v)", total, units)
	}
	// Reported jobs carry the reported provenance, sorted by (jobId, round),
	// with the exact five-key entry shape.
	rounds, _ := usage["rounds"].([]any)
	if len(rounds) != 2 {
		t.Fatalf("two terminal jobs should carry rounds entries, got %v", rounds)
	}
	firstEntry, _ := rounds[0].(map[string]any)
	if firstEntry["jobId"] != "job-a" || firstEntry["provenance"] != "reported" ||
		firstEntry["source"] != nil || firstEntry["detail"] != nil {
		t.Fatalf("reported provenance entry: %v", firstEntry)
	}
	if len(firstEntry) != 5 {
		t.Fatalf("a rounds entry must carry exactly jobId, round, provenance, source, detail: %v", firstEntry)
	}
}

func TestJobUsageAtMatchesAggregateUsage(t *testing.T) {
	repo, mission := fenceEnv(t)
	jobs := filepath.Join(repo, "artifacts", "agents", "jobs")
	if err := os.MkdirAll(jobs, 0o755); err != nil {
		t.Fatal(err)
	}
	recordPath := filepath.Join(jobs, "job-measured.json")
	writeText(t, recordPath,
		`{"jobId":"job-measured","mission":"demo","status":"completed","runtime":"codex","round":1,`+
			`"usage":{"inputTokens":10,"cachedInputTokens":20,"outputTokens":3,"reasoningTokens":2,`+
			`"cost":{"currency":"USD","amount":1.25},"providerUnits":{"name":"turns","value":1}}}`)
	writeText(t, filepath.Join(jobs, "unreadable.json"), `{`)

	measurement := JobUsageAt(repo, recordPath)
	if measurement.Provenance != usageReported || measurement.Source != nil || measurement.Detail != nil {
		t.Fatalf("reported measurement metadata changed: %#v", measurement)
	}
	if measurement.Tokens["inputTokens"] != 10 || measurement.Tokens["cachedInputTokens"] != 20 ||
		measurement.Tokens["outputTokens"] != 3 || measurement.Tokens["reasoningTokens"] != 2 {
		t.Fatalf("reported token classes were not preserved: %#v", measurement.Tokens)
	}
	if measurement.Cost == nil || measurement.Cost.Currency != "USD" || measurement.Cost.Amount != 1.25 {
		t.Fatalf("reported cost was not preserved: %#v", measurement.Cost)
	}
	if measurement.ProviderUnit == nil || measurement.ProviderUnit.Name != "turns" || measurement.ProviderUnit.Value != 1 {
		t.Fatalf("reported provider unit was not preserved: %#v", measurement.ProviderUnit)
	}
	unreadable := JobUsageAt(repo, filepath.Join(jobs, "unreadable.json"))
	if unreadable.Record != nil || unreadable.Provenance != usageUnreadable || !strings.Contains(fmt.Sprint(unreadable.Detail), "unreadable.json") {
		t.Fatalf("unreadable measurement was not explicit: %#v", unreadable)
	}

	if err := AggregateUsage(repo, mission); err != nil {
		t.Fatalf("aggregate: %v", err)
	}
	aggregate, err := readJSONObjectFile(filepath.Join(missionDir(repo, mission), "usage.json"))
	if err != nil {
		t.Fatal(err)
	}
	units, _ := aggregate["units"].([]any)
	got := map[string]float64{}
	for _, raw := range units {
		item, _ := raw.(map[string]any)
		unit, _ := item["unit"].(string)
		got[unit], _ = floatValue(item["value"])
	}
	for field, value := range measurement.Tokens {
		if got["tokens."+field] != value {
			t.Fatalf("aggregate token %s = %v, measurement = %v", field, got["tokens."+field], value)
		}
	}
	if got["cost.USD"] != measurement.Cost.Amount || got["provider.turns"] != measurement.ProviderUnit.Value {
		t.Fatalf("aggregate units do not match measurement: %v", got)
	}
}

// deadPgid is a process-group id no live system carries, so its probe is a
// definitive ESRCH. deadPid likewise proves kernel death for the custodian.
const deadPgid = 987654
const deadPid = 987653

// aggregateFixtureJob writes one terminal job record for the demo mission.
func aggregateFixtureJob(t *testing.T, repo, id, body string) {
	t.Helper()
	jobs := filepath.Join(repo, "artifacts", "agents", "jobs")
	if err := os.MkdirAll(jobs, 0o755); err != nil {
		t.Fatal(err)
	}
	writeText(t, filepath.Join(jobs, id+".json"), body)
}

func readAggregateRounds(t *testing.T, repo, mission string) map[string]map[string]any {
	t.Helper()
	usage, err := readJSONObjectFile(filepath.Join(missionDir(repo, mission), "usage.json"))
	if err != nil {
		t.Fatalf("mission usage aggregate unreadable: %v", err)
	}
	list, _ := usage["rounds"].([]any)
	entries := map[string]map[string]any{}
	for _, raw := range list {
		entry, _ := raw.(map[string]any)
		id, _ := entry["jobId"].(string)
		entries[id] = entry
	}
	return entries
}

// A budget-capped job whose record carries usage null
// derives its spend from the dead round's event stream — last valid usage
// block wins, a truncated final line is tolerated, and the derived value is
// never written back to the round.
func TestAggregateUsageDerivesFromProvablyDeadGroup(t *testing.T) {
	repo, mission := fenceEnv(t)
	aggregateFixtureJob(t, repo, "cap-1", fmt.Sprintf(
		`{"jobId":"cap-1","mission":"demo","status":"timeout","runtime":"codex","usage":null,`+
			`"round":1,"parentJob":null,"pid":%d,"pidStartedAt":100,"pgid":%d,"instanceTag":"tag-1"}`,
		deadPid, deadPgid))
	events := filepath.Join(repo, "artifacts", "agents", "cap-1", "rounds", "1", "events.jsonl")
	if err := os.MkdirAll(filepath.Dir(events), 0o755); err != nil {
		t.Fatal(err)
	}
	writeText(t, events,
		`{"type":"turn","usage":{"input_tokens":10,"output_tokens":2}}`+"\n"+
			`{"type":"turn","usage":{"input_tokens":100,"cached_input_tokens":40,"output_tokens":25,"reasoning_output_tokens":5}}`+"\n"+
			`{"type":"turn","usage":{"input_tok`)
	if err := AggregateUsage(repo, mission); err != nil {
		t.Fatalf("aggregate: %v", err)
	}
	usage, _ := readJSONObjectFile(filepath.Join(missionDir(repo, mission), "usage.json"))
	units, _ := usage["units"].([]any)
	got := map[string]float64{}
	for _, raw := range units {
		item, _ := raw.(map[string]any)
		unit, _ := item["unit"].(string)
		got[unit], _ = floatValue(item["value"])
	}
	want := map[string]float64{
		"tokens.inputTokens": 100, "tokens.cachedInputTokens": 40,
		"tokens.outputTokens": 25, "tokens.reasoningTokens": 5,
	}
	for unit, value := range want {
		if got[unit] != value {
			t.Fatalf("derived unit %s = %v, want %v (units=%v)", unit, got[unit], value, units)
		}
	}
	entry := readAggregateRounds(t, repo, mission)["cap-1"]
	if entry["provenance"] != "derived" ||
		entry["source"] != "artifacts/agents/cap-1/rounds/1/events.jsonl" || entry["detail"] != nil {
		t.Fatalf("derived provenance entry: %v", entry)
	}
	if unavailable, _ := usage["unavailableJobs"].([]any); len(unavailable) != 0 {
		t.Fatalf("a derived job is measured, not unavailable: %v", unavailable)
	}
	// Derivation never writes back: the round still has no usage.json.
	if _, err := os.Stat(filepath.Join(repo, "artifacts", "agents", "cap-1", "rounds", "1", "usage.json")); !os.IsNotExist(err) {
		t.Fatalf("derived usage must never be written back: %v", err)
	}
}

// The whole-group death gate, row by row: a live group defers, a permission
// denial blocks (existence proven), a missing pgid is permanently
// unprovable, a live custodian defers even when the group probe cannot see
// it, and adapter-reported usage always wins over the stream.
func TestAggregateUsageGroupDeathGate(t *testing.T) {
	repo, mission := fenceEnv(t)
	self := os.Getpid()
	ownGroup, err := getOwnPgid()
	if err != nil {
		t.Fatal(err)
	}
	exact, state, err := (identity.KernelProber{}).Probe(int64(self))
	if err != nil || state != identity.Alive {
		t.Fatalf("cannot probe own identity: %v %v", state, err)
	}
	ownStart := exact.StartedAt.Unix()

	// live: the recorded group is this test's own live group.
	aggregateFixtureJob(t, repo, "live-1", fmt.Sprintf(
		`{"jobId":"live-1","mission":"demo","status":"timeout","runtime":"codex","usage":null,`+
			`"round":1,"pgid":%d,"pid":%d,"pidStartedAt":%d,"instanceTag":""}`, ownGroup, deadPid, 100))
	// eperm: the probe reports existence through a permission denial.
	aggregateFixtureJob(t, repo, "eperm-1",
		`{"jobId":"eperm-1","mission":"demo","status":"failed","runtime":"codex","usage":null,"round":1,"pgid":1}`)
	// nopgid: no recorded pgid can never satisfy the whole-group gate.
	aggregateFixtureJob(t, repo, "nopgid-1", fmt.Sprintf(
		`{"jobId":"nopgid-1","mission":"demo","status":"failed","runtime":"codex","usage":null,"round":1,"pid":%d,"pidStartedAt":100}`, deadPid))
	// custodian: group probes gone, but the recorded custodian is this live
	// process - the writer may still be running, so derivation waits.
	aggregateFixtureJob(t, repo, "cust-1", fmt.Sprintf(
		`{"jobId":"cust-1","mission":"demo","status":"failed","runtime":"codex","usage":null,`+
			`"round":1,"pgid":%d,"pid":%d,"pidStartedAt":%d,"instanceTag":""}`, deadPgid, self, ownStart))
	// reported: adapter usage wins; the event stream is never consulted.
	aggregateFixtureJob(t, repo, "rep-1", fmt.Sprintf(
		`{"jobId":"rep-1","mission":"demo","status":"completed","runtime":"codex",`+
			`"usage":{"outputTokens":7},"round":1,"pgid":%d,"pid":%d,"pidStartedAt":100}`, deadPgid, deadPid))
	events := filepath.Join(repo, "artifacts", "agents", "rep-1", "rounds", "1", "events.jsonl")
	if err := os.MkdirAll(filepath.Dir(events), 0o755); err != nil {
		t.Fatal(err)
	}
	writeText(t, events, `{"usage":{"output_tokens":9999}}`+"\n")

	oldProbe := probeGroupGone
	probeGroupGone = func(pgid int64) (bool, string) {
		if pgid == 1 {
			return false, "process group 1 exists (permission denial proves existence)"
		}
		return oldProbe(pgid)
	}
	t.Cleanup(func() { probeGroupGone = oldProbe })

	if err := AggregateUsage(repo, mission); err != nil {
		t.Fatalf("aggregate: %v", err)
	}
	entries := readAggregateRounds(t, repo, mission)
	for id, wantProvenance := range map[string]string{
		"live-1":   "pending-death-proof",
		"eperm-1":  "pending-death-proof",
		"nopgid-1": "unavailable",
		"cust-1":   "pending-death-proof",
		"rep-1":    "reported",
	} {
		entry := entries[id]
		if entry == nil || entry["provenance"] != wantProvenance {
			t.Fatalf("job %s provenance = %v, want %s", id, entry, wantProvenance)
		}
	}
	if detail, _ := entries["nopgid-1"]["detail"].(string); !strings.Contains(detail, "no recorded pgid") {
		t.Fatalf("the missing-pgid entry must say why it is unprovable: %v", entries["nopgid-1"])
	}
	// Reported usage entered the units; the stream's 9999 did not.
	usage, _ := readJSONObjectFile(filepath.Join(missionDir(repo, mission), "usage.json"))
	units, _ := usage["units"].([]any)
	for _, raw := range units {
		item, _ := raw.(map[string]any)
		if item["unit"] == "tokens.outputTokens" {
			if v, _ := floatValue(item["value"]); v != 7 {
				t.Fatalf("adapter usage must win over the stream: %v", units)
			}
		}
	}
}

// A dead round whose stream carries no usage block - including the parser's
// native-with-nulls answer on unusable input - normalizes to unavailable.
func TestAggregateUsageNativeWithNullsIsUnavailable(t *testing.T) {
	repo, mission := fenceEnv(t)
	aggregateFixtureJob(t, repo, "null-1", fmt.Sprintf(
		`{"jobId":"null-1","mission":"demo","status":"timeout","runtime":"codex",`+
			`"usage":{"availability":"native","inputTokens":null,"cachedInputTokens":null,"outputTokens":null,"reasoningTokens":null,"cost":null,"providerUnits":null},`+
			`"round":1,"pgid":%d,"pid":%d,"pidStartedAt":100}`, deadPgid, deadPid))
	events := filepath.Join(repo, "artifacts", "agents", "null-1", "rounds", "1", "events.jsonl")
	if err := os.MkdirAll(filepath.Dir(events), 0o755); err != nil {
		t.Fatal(err)
	}
	writeText(t, events, "not json at all\n")
	if err := AggregateUsage(repo, mission); err != nil {
		t.Fatalf("aggregate: %v", err)
	}
	entry := readAggregateRounds(t, repo, mission)["null-1"]
	if entry["provenance"] != "unavailable" {
		t.Fatalf("a stream with no usage block must aggregate unavailable: %v", entry)
	}
	if detail, _ := entry["detail"].(string); !strings.Contains(detail, "no usage block") {
		t.Fatalf("the parse failure must land in detail: %v", entry)
	}
	usage, _ := readJSONObjectFile(filepath.Join(missionDir(repo, mission), "usage.json"))
	if unavailable, _ := usage["unavailableJobs"].([]any); len(unavailable) != 1 || unavailable[0] != "null-1" {
		t.Fatalf("the job stays honestly unavailable: %v", unavailable)
	}
}

// Idempotence: a content-equal aggregation skips the write - byte-identical
// file, updatedAt untouched - and updatedAt changes exactly when content
// changes.
func TestAggregateUsageContentEqualWriteSkipped(t *testing.T) {
	repo, mission := fenceEnv(t)
	aggregateFixtureJob(t, repo, "job-a",
		`{"jobId":"job-a","mission":"demo","status":"completed","runtime":"codex","usage":{"outputTokens":100}}`)
	if err := AggregateUsage(repo, mission); err != nil {
		t.Fatal(err)
	}
	usagePath := filepath.Join(missionDir(repo, mission), "usage.json")
	before, err := os.ReadFile(usagePath)
	if err != nil {
		t.Fatal(err)
	}
	// The clock advances; the content does not.
	clock = func() time.Time { return fixedNow.Add(time.Hour) }
	if err := AggregateUsage(repo, mission); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(usagePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatalf("a content-equal aggregation must be byte-identical:\n%s\nvs\n%s", before, after)
	}
	if !strings.Contains(string(after), fixedNow.Format("2006-01-02T15:04:05Z")) {
		t.Fatalf("updatedAt must keep its old value on a skipped write:\n%s", after)
	}
	// New content moves updatedAt.
	aggregateFixtureJob(t, repo, "job-b",
		`{"jobId":"job-b","mission":"demo","status":"completed","runtime":"codex","usage":{"outputTokens":40}}`)
	if err := AggregateUsage(repo, mission); err != nil {
		t.Fatal(err)
	}
	changed, _ := os.ReadFile(usagePath)
	if !strings.Contains(string(changed), fixedNow.Add(time.Hour).Format("2006-01-02T15:04:05Z")) {
		t.Fatalf("updatedAt must advance when content changes:\n%s", changed)
	}
}

// mission-contract-6: the batched ask is the fence's designed recovery
// channel — when its write fails, the refusal must SAY so, not report
// "batched ask written: " with nothing after the colon.
func TestFenceRefusalNamesAFailedAskWrite(t *testing.T) {
	repo, mission := fenceEnv(t)
	if err := ReserveCycle(repo, mission); err != nil {
		t.Fatalf("the first cycle should be allowed: %v", err)
	}
	// Make the asks directory impossible to create: a FILE in its place.
	asksPath := filepath.Join(missionDir(repo, mission), "asks")
	if err := os.WriteFile(asksPath, []byte("not a directory\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := ReserveCycle(repo, mission)
	if err == nil || !strings.Contains(err.Error(), "FAILED to write batched ask") {
		t.Fatalf("the refusal must carry the ask-write failure, got: %v", err)
	}
	if !strings.Contains(err.Error(), "cycles") {
		t.Fatalf("the refusal must still name the tripped fence: %v", err)
	}
}

// The fence-refusal diagnosis (D7): a husked dispatch's reservation must
// not keep counting against fence.jobs or hold a concurrency slot.
func TestReleaseJobFreesAHuskedReservation(t *testing.T) {
	repo, mission := fenceEnv(t)
	if _, err := AuthorizeCap(repo, mission, "husk-1", "codex", "gpt-5-6-sol", "", nil); err != nil {
		t.Fatalf("authorize: %v", err)
	}
	fences, _ := readJSONObjectFile(filepath.Join(missionDir(repo, mission), "fences.json"))
	if _, ok := reservationsMap(fences)["husk-1"]; !ok {
		t.Fatal("reservation missing after authorize")
	}
	if err := ReleaseJob(repo, mission, "husk-1"); err != nil {
		t.Fatalf("release: %v", err)
	}
	fences, _ = readJSONObjectFile(filepath.Join(missionDir(repo, mission), "fences.json"))
	if _, ok := reservationsMap(fences)["husk-1"]; ok {
		t.Fatal("a released reservation still counts")
	}
	// Releasing a job that never reserved is a clean no-op.
	if err := ReleaseJob(repo, mission, "never-reserved"); err != nil {
		t.Fatalf("no-op release errored: %v", err)
	}
}

// The pinned-bytes drift refusals: the fence's cap authority reads the raw
// contract bytes pinned in fences.json, so ANY drift — a real edit or a
// whitespace-only one — must refuse against approvedContractSha256.
func TestAuthorizeCapRefusesPinnedContractDrift(t *testing.T) {
	repo, mission := fenceEnv(t)
	contractPath := filepath.Join(repo, "plans", "mission-demo.contract.md")
	data, err := os.ReadFile(contractPath)
	if err != nil {
		t.Fatal(err)
	}
	writeText(t, contractPath, strings.Replace(string(data), "fence.cycles=", "fence.cycles=9", 1))
	if _, err := AuthorizeCap(repo, mission, "job-drift", "codex", "gpt-5-6-sol", "", nil); err == nil ||
		!strings.Contains(err.Error(), "approvedContractSha256") {
		t.Fatalf("a drifted pinned contract must refuse naming the pin, got %v", err)
	}
}

func TestAuthorizeCapRefusesWhitespaceOnlyDrift(t *testing.T) {
	repo, mission := fenceEnv(t)
	contractPath := filepath.Join(repo, "plans", "mission-demo.contract.md")
	data, err := os.ReadFile(contractPath)
	if err != nil {
		t.Fatal(err)
	}
	// Raw-file hashing deliberately sees what the canonical approval digest
	// ignores: a trailing-whitespace-only edit still drifts the pin.
	writeText(t, contractPath, string(data)+"  \n")
	if _, err := AuthorizeCap(repo, mission, "job-ws", "codex", "gpt-5-6-sol", "", nil); err == nil ||
		!strings.Contains(err.Error(), "approvedContractSha256") {
		t.Fatalf("a whitespace-only drift must refuse naming the pin, got %v", err)
	}
}
