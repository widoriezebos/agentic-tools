package watch

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestJobGoalFieldWireShapesRemainDistinct(t *testing.T) {
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name      string
		job       string
		wantField GoalFieldState
		wantTotal AggregateVerdict
	}{
		{name: "explicit no-goal", job: `{"jobId":"job-one","status":"failed","goalId":null}`, wantField: GoalFieldNull, wantTotal: AggregateAttention},
		{name: "absent legacy shape", job: `{"jobId":"job-one","status":"failed"}`, wantField: GoalFieldAbsent, wantTotal: AggregateUnknown},
		{name: "empty contradiction", job: `{"jobId":"job-one","status":"failed","goalId":""}`, wantField: GoalFieldEmpty, wantTotal: AggregateUnknown},
		{name: "bound goal", job: `{"jobId":"job-one","status":"failed","goalId":"goal-one"}`, wantField: GoalFieldBound, wantTotal: AggregateHealthy},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			writeWatchHealth(t, root, now.Add(-time.Minute), "healthy", "alive")
			dir := filepath.Join(root, "artifacts", "agents", "jobs")
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "job-one.json"), []byte(test.job), 0o644); err != nil {
				t.Fatal(err)
			}
			snapshot := readAt(root, now)
			if snapshot.Aggregate != test.wantTotal {
				t.Fatalf("aggregate=%s want=%s: %+v", snapshot.Aggregate, test.wantTotal, snapshot)
			}
			jobs := snapshot.Sections[0]
			if len(jobs.Items) != 1 || jobs.Items[0].GoalField != test.wantField {
				t.Fatalf("goal field shape collapsed: %+v", jobs)
			}
			wantStore := SectionReadable
			if test.wantField == GoalFieldAbsent || test.wantField == GoalFieldEmpty {
				wantStore = SectionDegraded
			}
			if jobs.Verdict != wantStore {
				t.Fatalf("store verdict=%s want=%s", jobs.Verdict, wantStore)
			}
		})
	}
}

func TestHealthUnknownDoesNotBecomeHealthy(t *testing.T) {
	now := time.Date(2026, 9, 1, 10, 1, 0, 0, time.UTC)
	root := t.TempDir()
	path := filepath.Join(root, "artifacts", "agents", "steward", "health.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	data := `{"verdict":{"schema":1,"observedAt":"2026-09-01T10:00:00Z","observation":1,"aggregate":"unknown","roles":[{"role":"claimed-goal-delivery","status":"unknown","reason":"unreadable source"}]}}`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := readAt(root, now).Aggregate; got != AggregateUnknown {
		t.Fatalf("unknown producer verdict became %s", got)
	}
}

func TestStaleHealthMakesPersistedGoalBoundFailureDead(t *testing.T) {
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	root := t.TempDir()
	writeWatchHealth(t, root, now.Add(-time.Hour), "healthy", "alive")
	jobs := filepath.Join(root, "artifacts", "agents", "jobs")
	if err := os.MkdirAll(jobs, 0o755); err != nil {
		t.Fatal(err)
	}
	job := fmt.Sprintf(`{"jobId":"goal-failure","status":"failed","goalId":"goal-one","endedAt":%q}`, now.Add(-30*time.Minute).Format(time.RFC3339))
	if err := os.WriteFile(filepath.Join(jobs, "goal-failure.json"), []byte(job), 0o644); err != nil {
		t.Fatal(err)
	}

	snapshot := readAt(root, now)
	if snapshot.Aggregate != AggregateAttention || snapshot.ExitCode() == 0 {
		t.Fatalf("stale health over a persisted goal failure must be dead and nonzero: %+v", snapshot)
	}
	var health Section
	for _, section := range snapshot.Sections {
		if section.Class == ClassHealth {
			health = section
			break
		}
	}
	if len(health.Items) == 0 || health.Items[0].Kind != "health-freshness" || health.Items[0].Verdict != "dead" ||
		health.Items[0].Evidence != "artifacts/agents/steward/health.json" ||
		!containsAll(health.Items[0].Problem, "health record artifacts/agents/steward/health.json", "stale", "age=") {
		t.Fatalf("stale record was not named with its age: %+v", health)
	}
}

func TestUnreadableHealthRecordIsDead(t *testing.T) {
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	root := t.TempDir()
	path := filepath.Join(root, "artifacts", "agents", "steward", "health.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("not-json"), 0o644); err != nil {
		t.Fatal(err)
	}
	snapshot := readAt(root, now)
	health := snapshot.Sections[3]
	if snapshot.Aggregate != AggregateAttention || snapshot.ExitCode() != 1 || len(health.Items) != 1 ||
		health.Items[0].Verdict != "dead" || !containsAll(health.Items[0].Problem, "health record artifacts/agents/steward/health.json", "unreadable", "age=unknown") {
		t.Fatalf("unreadable health record did not fail dead: %+v", snapshot)
	}
}

func TestCompletedRoundNewerThanLandingReceiptHasUnknownConsumption(t *testing.T) {
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	root := t.TempDir()
	writeWatchHealth(t, root, now.Add(-time.Minute), "healthy", "alive")
	jobs := filepath.Join(root, "artifacts", "agents", "jobs")
	if err := os.MkdirAll(jobs, 0o755); err != nil {
		t.Fatal(err)
	}
	job := fmt.Sprintf(`{"jobId":"late-return","role":"implementer","round":2,"status":"completed","goalId":"goal-one","endedAt":%q}`, now.Add(-time.Minute).Format(time.RFC3339))
	if err := os.WriteFile(filepath.Join(jobs, "late-return.json"), []byte(job), 0o644); err != nil {
		t.Fatal(err)
	}
	receipts := filepath.Join(root, "memory", "receipts.log")
	if err := os.MkdirAll(filepath.Dir(receipts), 0o755); err != nil {
		t.Fatal(err)
	}
	receiptAt := now.Add(-time.Hour)
	line := fmt.Sprintf("%d|%s|RECEIPT|type=implement|outcome=shipped|goal=goal-one\n", receiptAt.Unix(), receiptAt.Format(time.RFC3339))
	if err := os.WriteFile(receipts, []byte(line), 0o644); err != nil {
		t.Fatal(err)
	}

	snapshot := readAt(root, now)
	completed := snapshot.Sections[1]
	if completed.Class != ClassCompletedRounds || completed.Verdict != SectionReadable || len(completed.Items) != 1 ||
		completed.Items[0].Verdict != VerdictUnknownConsumption || completed.Items[0].GoalID != "goal-one" ||
		snapshot.Aggregate != AggregateUnknown {
		t.Fatalf("completed round newer than its receipt was silent or guessed consumed: %+v", snapshot)
	}
}

func TestCompletedRoundWithNoLandingReceiptHasUnknownConsumption(t *testing.T) {
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	root := t.TempDir()
	writeWatchHealth(t, root, now.Add(-time.Minute), "healthy", "alive")
	jobs := filepath.Join(root, "artifacts", "agents", "jobs")
	if err := os.MkdirAll(jobs, 0o755); err != nil {
		t.Fatal(err)
	}
	job := fmt.Sprintf(`{"jobId":"first-return","role":"implementer","round":1,"status":"completed","goalId":"goal-one","endedAt":%q}`, now.Add(-time.Minute).Format(time.RFC3339))
	if err := os.WriteFile(filepath.Join(jobs, "first-return.json"), []byte(job), 0o644); err != nil {
		t.Fatal(err)
	}

	snapshot := readAt(root, now)
	completed := snapshot.Sections[1]
	if completed.Verdict != SectionReadable || len(completed.Items) != 1 ||
		completed.Items[0].Verdict != VerdictUnknownConsumption ||
		!strings.Contains(completed.Items[0].Problem, "goal has no landing receipt") ||
		snapshot.Aggregate != AggregateUnknown {
		t.Fatalf("a first completed round without a receipt was silent: %+v", snapshot)
	}
}

func TestFoldedCriticRoundIsNotUnknownConsumption(t *testing.T) {
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	root := t.TempDir()
	writeWatchHealth(t, root, now.Add(-time.Minute), "healthy", "alive")
	jobs := filepath.Join(root, "artifacts", "agents", "jobs")
	if err := os.MkdirAll(jobs, 0o755); err != nil {
		t.Fatal(err)
	}
	job := fmt.Sprintf(`{"jobId":"critic-one","role":"code-critic","round":1,"status":"completed","goalId":"goal-one","parentJob":null,"findingRegister":[],"findingRegisterRound":2,"endedAt":%q}`, now.Add(-2*time.Minute).Format(time.RFC3339))
	if err := os.WriteFile(filepath.Join(jobs, "critic-one.json"), []byte(job), 0o644); err != nil {
		t.Fatal(err)
	}
	job = fmt.Sprintf(`{"jobId":"critic-two","role":"code-critic","round":2,"status":"completed","goalId":"goal-one","parentJob":"critic-one","endedAt":%q}`, now.Add(-time.Minute).Format(time.RFC3339))
	if err := os.WriteFile(filepath.Join(jobs, "critic-two.json"), []byte(job), 0o644); err != nil {
		t.Fatal(err)
	}

	snapshot := readAt(root, now)
	if completed := snapshot.Sections[1]; completed.Verdict != SectionEmpty || len(completed.Items) != 0 {
		t.Fatalf("a critic return proven consumed by its finding-register marker remained noisy: %+v", completed)
	}
}

func TestGoalBoundFailureNewerThanHealthObservationNeedsAttention(t *testing.T) {
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	root := t.TempDir()
	writeWatchHealth(t, root, now.Add(-time.Minute), "healthy", "alive")
	jobs := filepath.Join(root, "artifacts", "agents", "jobs")
	if err := os.MkdirAll(jobs, 0o755); err != nil {
		t.Fatal(err)
	}
	job := fmt.Sprintf(`{"jobId":"later-failure","status":"failed","goalId":"goal-one","endedAt":%q}`, now.Add(-30*time.Second).Format(time.RFC3339))
	if err := os.WriteFile(filepath.Join(jobs, "later-failure.json"), []byte(job), 0o644); err != nil {
		t.Fatal(err)
	}

	snapshot := readAt(root, now)
	item := snapshot.Sections[0].Items[0]
	if snapshot.Aggregate != AggregateAttention || snapshot.ExitCode() != 1 || item.Stage != "STALE" ||
		!strings.Contains(item.Problem, "newer than the owning health observation") {
		t.Fatalf("a fresh summary hid its later goal-bound failure: %+v", snapshot)
	}
}

func TestUnreadableJobAppearsOnlyOnce(t *testing.T) {
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	root := t.TempDir()
	writeWatchHealth(t, root, now.Add(-time.Minute), "healthy", "alive")
	jobs := filepath.Join(root, "artifacts", "agents", "jobs")
	if err := os.MkdirAll(jobs, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(jobs, "broken.json"), []byte("not-json"), 0o644); err != nil {
		t.Fatal(err)
	}

	snapshot := readAt(root, now)
	if len(snapshot.Sections[0].Items) != 1 || snapshot.Sections[0].Items[0].Verdict != VerdictUnreadable {
		t.Fatalf("the jobs class lost its typed per-record error: %+v", snapshot.Sections[0])
	}
	if len(snapshot.Sections[1].Items) != 0 || snapshot.Sections[1].Verdict != SectionEmpty {
		t.Fatalf("the completed-rounds class duplicated the job parse error: %+v", snapshot.Sections[1])
	}
}

func TestMissingCheckoutDoesNotBecomeHealthy(t *testing.T) {
	root := filepath.Join(t.TempDir(), "absent")
	snapshot := Read(root)
	if snapshot.Aggregate != AggregateUnknown || snapshot.Empty || len(snapshot.Sections) != len(trackedClassOrder) {
		t.Fatalf("missing checkout must be a typed unknown snapshot: %+v", snapshot)
	}
	if snapshot.Sections[0].Verdict != SectionDegraded || snapshot.Sections[0].Items[0].Verdict != VerdictUnreadable {
		t.Fatalf("missing checkout evidence must be explicit: %+v", snapshot.Sections[0])
	}
}

func writeWatchHealth(t *testing.T, root string, observed time.Time, aggregate, delivery string) {
	t.Helper()
	path := filepath.Join(root, "artifacts", "agents", "steward", "health.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	data := fmt.Sprintf(`{"verdict":{"schema":1,"observedAt":%q,"observation":1,"aggregate":%q,"roles":[{"role":"claimed-goal-delivery","status":%q}]}}`,
		observed.UTC().Format(time.RFC3339), aggregate, delivery)
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

func containsAll(value string, fragments ...string) bool {
	for _, fragment := range fragments {
		if !strings.Contains(value, fragment) {
			return false
		}
	}
	return true
}
