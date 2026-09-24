package seat

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// fixtureClock is the one time every behaviour test in this package reads.
// Nothing here touches the wall.
var fixtureClock = time.Date(2026, 9, 24, 9, 40, 0, 0, time.UTC)

func fixtureRunner() RunnerContext {
	return RunnerContext{RepoIdentity: "repo-a", Generation: 4, Engine: "3f9c1e2abc", ArmedLineage: "lineage-7", TickSeconds: 600}
}

func TestComposeBuildsTheRecordFromIdentityAndJobs(t *testing.T) {
	t.Parallel()
	jobs := JobSet{Records: []JobRecord{
		{Job: "job-old", Root: "job-old", Role: "implementer", Goal: "goal-a", Round: 1, Status: "completed", CreatedAt: "2026-09-24T09:00:00Z", StartedAt: "2026-09-24T09:00:05Z"},
		{Job: "job-new", Root: "job-old", Role: "critic", Goal: "goal-a", Round: 2, Status: "running", CreatedAt: "2026-09-24T09:30:00Z", StartedAt: "2026-09-24T09:30:05Z"},
	}}
	record, detail, err := Compose("m1e", fixtureRunner(), jobs, fixtureClock)
	if err != nil || detail != "" {
		t.Fatalf("compose = detail %q err %v", detail, err)
	}
	if record.PresenceSchema != RecordSchema || record.Machine != "m1e" || record.RepoIdentity != "repo-a" {
		t.Fatalf("record identity = %+v", record)
	}
	if record.Generation != 4 || record.Engine != "3f9c1e2abc" || record.ArmedLineage != "lineage-7" || record.TickSeconds != 600 {
		t.Fatalf("record runner context = %+v", record)
	}
	if record.TickAt != "2026-09-24T09:40:00Z" {
		t.Fatalf("record tickAt = %q", record.TickAt)
	}
	if record.Chain == nil || record.Chain.Job != "job-new" || record.Chain.Round != 2 || record.Chain.Root != "job-old" {
		t.Fatalf("record chain = %+v", record.Chain)
	}
	if record.Chain.StartedAt == nil || *record.Chain.StartedAt != "2026-09-24T09:30:05Z" {
		t.Fatalf("record chain startedAt = %v", record.Chain.StartedAt)
	}
}

func TestComposeLeavesTheChainNullWhenNothingIsInFlight(t *testing.T) {
	t.Parallel()
	jobs := JobSet{Records: []JobRecord{{Job: "job-done", Status: "completed", CreatedAt: "2026-09-24T09:00:00Z"}}}
	record, detail, err := Compose("m1e", fixtureRunner(), jobs, fixtureClock)
	if err != nil || detail != "" || record.Chain != nil {
		t.Fatalf("idle compose = chain %+v detail %q err %v", record.Chain, detail, err)
	}
}

func TestPendingSetupReservationComposesAChainWithNoStartedAt(t *testing.T) {
	t.Parallel()
	jobs := JobSet{Records: []JobRecord{
		{Job: "job-reserved", Root: "job-reserved", Role: "implementer", Goal: "goal-b", Status: "pending-setup", CreatedAt: "2026-09-24T09:31:00Z"},
	}}
	record, _, err := Compose("m1e", fixtureRunner(), jobs, fixtureClock)
	if err != nil {
		t.Fatal(err)
	}
	if record.Chain == nil || record.Chain.StartedAt != nil || record.Chain.Round != 0 {
		t.Fatalf("reservation chain = %+v", record.Chain)
	}
	encoded, err := record.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"startedAt": null`) {
		t.Fatalf("encoded reservation = %s", encoded)
	}
}

func TestACorruptJobRecordYieldsANullChainWithANamedDetail(t *testing.T) {
	t.Parallel()
	jobs := JobSet{
		Records:    []JobRecord{{Job: "job-live", Status: "running", CreatedAt: "2026-09-24T09:30:00Z"}},
		Unreadable: []string{"artifacts/agents/jobs/job-torn.json: the job record is not readable JSON"},
	}
	record, detail, err := Compose("m1e", fixtureRunner(), jobs, fixtureClock)
	if err != nil {
		t.Fatal(err)
	}
	if record.Chain != nil {
		t.Fatalf("chain survived a corrupt record: %+v", record.Chain)
	}
	if !strings.Contains(detail, "job-torn.json") {
		t.Fatalf("detail = %q; it must name the record it could not read", detail)
	}
}

func TestComposeRefusesAnUnpublishableNicknameAndAnUnarmedGeneration(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"m1/e", "m1 e", "..", "a..b", "", "m1é"} {
		if _, _, err := Compose(name, fixtureRunner(), JobSet{}, fixtureClock); err == nil ||
			!strings.Contains(err.Error(), "SEAT_MACHINE_NICKNAME_INVALID") {
			t.Errorf("compose(%q) = %v; want the nickname refusal", name, err)
		}
	}
	for _, name := range []string{"m1e", "m-1.e_2", "M1E"} {
		if err := ValidateMachineName(name); err != nil {
			t.Errorf("ValidateMachineName(%q) = %v; want it accepted", name, err)
		}
	}
	unarmed := fixtureRunner()
	unarmed.Generation = 0
	if _, _, err := Compose("m1e", unarmed, JobSet{}, fixtureClock); err == nil {
		t.Fatal("an unarmed generation composed a record")
	}
}

func TestParseAcceptsAHigherSchemaAndIgnoresUnknownKeys(t *testing.T) {
	t.Parallel()
	raw := `{"presenceSchema":7,"machine":"m1c","repoIdentity":"repo-b","generation":3,"engine":"8a01d77",
		"armedLineage":"no-lease","tickSeconds":2400,"chain":null,"tickAt":"2026-09-24T09:00:00Z",
		"asks":[{"to":"m1e"}],"somethingNewer":42}`
	record, err := ParseRecord([]byte(raw))
	if err != nil {
		t.Fatalf("a later schema was refused: %v", err)
	}
	if record.Machine != "m1c" || record.TickSeconds != 2400 || record.Chain != nil {
		t.Fatalf("record = %+v", record)
	}
}

func TestParseRefusesEachMalformedShape(t *testing.T) {
	t.Parallel()
	complete := map[string]any{
		"presenceSchema": 1, "machine": "m1c", "repoIdentity": "repo-b", "generation": 3,
		"engine": "8a01d77", "armedLineage": "no-lease", "tickSeconds": 600, "chain": nil,
		"tickAt": "2026-09-24T09:00:00Z",
	}
	for _, key := range []string{"presenceSchema", "machine", "repoIdentity", "generation", "engine", "armedLineage", "tickSeconds", "chain", "tickAt"} {
		body := map[string]any{}
		for name, value := range complete {
			if name != key {
				body[name] = value
			}
		}
		if _, err := ParseRecord(mustJSON(t, body)); err == nil || !strings.Contains(err.Error(), "SEAT_PRESENCE_MALFORMED") {
			t.Errorf("a record with no %s parsed: %v", key, err)
		}
	}
	broken := []struct {
		name string
		key  string
		bad  any
	}{
		{"a lower schema", "presenceSchema", 0},
		{"a string generation", "generation", "four"},
		{"a zero generation", "generation", 0},
		{"a string cadence", "tickSeconds", "600"},
		{"a zero cadence", "tickSeconds", 0},
		{"a local time", "tickAt", "2026-09-24T09:00:00+02:00"},
		{"a fractional time", "tickAt", "2026-09-24T09:00:00.5Z"},
		{"a word for a time", "tickAt", "soon"},
		{"an unpublishable nickname", "machine", "m1/c"},
		{"a chain that is neither object nor null", "chain", "running"},
		{"an empty engine", "engine", ""},
	}
	for _, test := range broken {
		body := map[string]any{}
		for name, value := range complete {
			body[name] = value
		}
		body[test.key] = test.bad
		if _, err := ParseRecord(mustJSON(t, body)); err == nil || !strings.Contains(err.Error(), "SEAT_PRESENCE_MALFORMED") {
			t.Errorf("%s parsed: %v", test.name, err)
		}
	}
	if _, err := ParseRecord([]byte("not json")); err == nil {
		t.Error("a record that is not JSON parsed")
	}
}

func TestEncodeAndParseRoundTrip(t *testing.T) {
	t.Parallel()
	record, _, err := Compose("m1e", fixtureRunner(), JobSet{Records: []JobRecord{
		{Job: "job-live", Root: "job-live", Role: "implementer", Goal: "goal-a", Round: 2, Status: "running", CreatedAt: "2026-09-24T09:30:00Z", StartedAt: "2026-09-24T09:30:05Z"},
	}}, fixtureClock)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := record.Encode()
	if err != nil {
		t.Fatal(err)
	}
	back, err := ParseRecord(encoded)
	if err != nil {
		t.Fatalf("a composed record did not parse: %v", err)
	}
	if back.Chain == nil || back.Chain.Job != record.Chain.Job || back.Chain.Round != record.Chain.Round ||
		back.Chain.StartedAt == nil || *back.Chain.StartedAt != *record.Chain.StartedAt || back.TickAt != record.TickAt {
		t.Fatalf("round trip = %+v, want %+v", back, record)
	}
	if CommitMessage(record) != "seat presence m1e 2026-09-24T09:40:00Z" {
		t.Fatalf("commit message = %q", CommitMessage(record))
	}
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
