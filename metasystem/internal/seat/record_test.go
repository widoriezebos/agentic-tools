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
	// A three-deep chain: the root, its follow-up, and the newest record,
	// whose own parent is the middle one. The root is the walk's answer, not
	// the immediate parent.
	jobs := JobSet{Records: []JobRecord{
		{Job: "job-old", Role: "implementer", Goal: "goal-a", Round: 1, Status: "completed", CreatedAt: "2026-09-24T09:00:00Z", StartedAt: "2026-09-24T09:00:05Z"},
		{Job: "job-mid", Parent: "job-old", Role: "critic", Goal: "goal-a", Round: 2, Status: "completed", CreatedAt: "2026-09-24T09:15:00Z", StartedAt: "2026-09-24T09:15:05Z"},
		{Job: "job-new", Parent: "job-mid", Role: "critic", Goal: "goal-a", Round: 2, Status: "running", CreatedAt: "2026-09-24T09:30:00Z", StartedAt: "2026-09-24T09:30:05Z"},
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
		t.Fatalf("record chain = %+v; the root is the walk's answer, not the parent", record.Chain)
	}
	if record.Chain.StartedAt == nil || *record.Chain.StartedAt != "2026-09-24T09:30:05Z" {
		t.Fatalf("record chain startedAt = %v", record.Chain.StartedAt)
	}
}

func TestComposeLeavesTheChainNullWhenNothingIsInFlight(t *testing.T) {
	t.Parallel()
	jobs := JobSet{Records: []JobRecord{{Job: "job-done", Role: "implementer", Status: "completed", CreatedAt: "2026-09-24T09:00:00Z"}}}
	record, detail, err := Compose("m1e", fixtureRunner(), jobs, fixtureClock)
	if err != nil || detail != "" || record.Chain != nil {
		t.Fatalf("idle compose = chain %+v detail %q err %v", record.Chain, detail, err)
	}
}

func TestPendingSetupReservationComposesAChainWithNoStartedAt(t *testing.T) {
	t.Parallel()
	jobs := JobSet{Records: []JobRecord{
		{Job: "job-reserved", Role: "implementer", Goal: "goal-b", Status: "pending-setup", CreatedAt: "2026-09-24T09:31:00Z"},
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
		Records:    []JobRecord{{Job: "job-live", Role: "implementer", Status: "running", CreatedAt: "2026-09-24T09:30:00Z"}},
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

func TestAnIncompleteOrUnrootedNewestJobYieldsANullChain(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		jobs   JobSet
		detail string
	}{
		{
			name: "the newest record names no role",
			jobs: JobSet{Records: []JobRecord{
				{Job: "job-done", Role: "implementer", Status: "completed", CreatedAt: "2026-09-24T09:00:00Z"},
				{Job: "job-live", Status: "running", CreatedAt: "2026-09-24T09:30:00Z"},
			}},
			detail: "names no role",
		},
		{
			name: "the newest record names no createdAt",
			jobs: JobSet{Records: []JobRecord{
				{Job: "job-live", Role: "implementer", Status: "running"},
			}},
			detail: "names no createdAt",
		},
		{
			name: "the newest record's parent is not in the scan",
			jobs: JobSet{Records: []JobRecord{
				{Job: "job-live", Parent: "job-gone", Role: "implementer", Status: "running", CreatedAt: "2026-09-24T09:30:00Z"},
			}},
			detail: "does not resolve to a root",
		},
		{
			name: "the newest record's ancestry cycles",
			jobs: JobSet{Records: []JobRecord{
				{Job: "job-a", Parent: "job-b", Role: "implementer", Status: "running", CreatedAt: "2026-09-24T09:30:00Z"},
				{Job: "job-b", Parent: "job-a", Role: "implementer", Status: "running", CreatedAt: "2026-09-24T09:20:00Z"},
			}},
			detail: "does not resolve to a root",
		},
	}
	for _, test := range cases {
		chain, detail := NewestChain(test.jobs)
		if chain != nil {
			t.Errorf("%s composed a chain: %+v", test.name, chain)
		}
		if !strings.Contains(detail, test.detail) {
			t.Errorf("%s detail = %q; want it to name %q", test.name, detail, test.detail)
		}
	}
	// An incomplete record that is NOT the newest in flight does not hide the
	// work in hand behind it.
	healthy := JobSet{Records: []JobRecord{
		{Job: "job-broken", Status: "running", CreatedAt: "2026-09-24T09:00:00Z"},
		{Job: "job-live", Role: "implementer", Goal: "goal-a", Status: "running", CreatedAt: "2026-09-24T09:30:00Z"},
	}}
	chain, detail := NewestChain(healthy)
	if chain == nil || chain.Job != "job-live" || chain.Root != "job-live" || detail != "" {
		t.Fatalf("chain = %+v detail = %q", chain, detail)
	}
}

func TestAComposedChainAlwaysSatisfiesTheReader(t *testing.T) {
	t.Parallel()
	// Whatever the composer emits, the reader on the other machine must
	// accept: the two halves of the record are one contract.
	jobs := JobSet{Records: []JobRecord{
		{Job: "job-live", Role: "implementer", Status: "running", CreatedAt: "2026-09-24T09:30:00Z"},
	}}
	record, detail, err := Compose("m1e", fixtureRunner(), jobs, fixtureClock)
	if err != nil || detail != "" {
		t.Fatalf("compose = detail %q err %v", detail, err)
	}
	encoded, err := record.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseRecord(encoded); err != nil {
		t.Fatalf("the composer emitted a record its own reader refuses: %v\n%s", err, encoded)
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

func TestAChainOfEmptyFieldsIsAMalformedRecord(t *testing.T) {
	t.Parallel()
	whole := map[string]any{
		"root": "job-a", "job": "job-b", "role": "implementer", "round": 2,
		"goal": "goal-a", "startedAt": "2026-09-24T09:30:05Z",
	}
	body := func(chain any) []byte {
		return mustJSON(t, map[string]any{
			"presenceSchema": 1, "machine": "m1c", "repoIdentity": "repo-b", "generation": 3,
			"engine": "8a01d77", "armedLineage": "no-lease", "tickSeconds": 600,
			"chain": chain, "tickAt": "2026-09-24T09:00:00Z",
		})
	}
	if _, err := ParseRecord(body(whole)); err != nil {
		t.Fatalf("a complete chain was refused: %v", err)
	}
	// An object with nothing in it is not a chain, and neither is one with a
	// field missing or emptied: a reader that took either would report a
	// machine as running nothing in particular.
	broken := []struct {
		name  string
		chain map[string]any
	}{{"an empty object", map[string]any{}}}
	for _, key := range []string{"root", "job", "role", "round", "goal", "startedAt"} {
		missing := map[string]any{}
		emptied := map[string]any{}
		for field, value := range whole {
			emptied[field] = value
			if field != key {
				missing[field] = value
			}
		}
		broken = append(broken, struct {
			name  string
			chain map[string]any
		}{"a chain with no " + key, missing})
		if key == "round" {
			emptied[key] = -1
		} else if key == "startedAt" {
			emptied[key] = "not a time"
		} else {
			emptied[key] = ""
		}
		if key == "goal" {
			// A goal-free job is lawful; only its absence is not.
			continue
		}
		broken = append(broken, struct {
			name  string
			chain map[string]any
		}{"a chain whose " + key + " is empty", emptied})
	}
	for _, test := range broken {
		if _, err := ParseRecord(body(test.chain)); err == nil ||
			!strings.Contains(err.Error(), "SEAT_PRESENCE_MALFORMED") {
			t.Errorf("%s parsed: %v", test.name, err)
		}
	}
	// A pending-setup reservation's null startedAt is lawful, and so is an
	// unknown key inside the chain.
	reservation := map[string]any{}
	for field, value := range whole {
		reservation[field] = value
	}
	reservation["startedAt"] = nil
	reservation["somethingNewer"] = 42
	record, err := ParseRecord(body(reservation))
	if err != nil {
		t.Fatalf("a reservation chain was refused: %v", err)
	}
	if record.Chain == nil || record.Chain.StartedAt != nil {
		t.Fatalf("reservation chain = %+v", record.Chain)
	}
}

func TestEncodeAndParseRoundTrip(t *testing.T) {
	t.Parallel()
	record, _, err := Compose("m1e", fixtureRunner(), JobSet{Records: []JobRecord{
		{Job: "job-live", Role: "implementer", Goal: "goal-a", Round: 2, Status: "running", CreatedAt: "2026-09-24T09:30:00Z", StartedAt: "2026-09-24T09:30:05Z"},
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
