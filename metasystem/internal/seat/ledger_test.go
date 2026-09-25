package seat

// What ReadJobs keeps off a delegate job record, from records written the way
// each of dispatch's writers writes them.
//
// There are two cap shapes in the tree and this reader must answer for both.
// The ordinary build and its follow-ups record `capMin` alone
// (internal/dispatch/build.go:637, 981; brief.go:536); only the claim-launch
// path records `capRequest.minutes` beside it (claim.go:762-763). A reader
// that knew the second alone would report every build job as having reserved
// nothing.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// writeJobRecord puts one record where a machine's own jobs live.
func writeJobRecord(t *testing.T, root, name string, record map[string]any) {
	t.Helper()
	dir := filepath.Join(root, "artifacts", "agents", "jobs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name+".json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func jobNamed(jobs JobSet, id string) (JobRecord, bool) {
	for _, record := range jobs.Records {
		if record.Job == id {
			return record, true
		}
	}
	return JobRecord{}, false
}

// ledgerFixture is one of each writer's shape: the build's, the claim
// launch's, and a record that carries neither.
func ledgerFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	// The ordinary build (build.go:637): capMin and capDeadline, no
	// capRequest anywhere on the record.
	writeJobRecord(t, root, "job-build", map[string]any{
		"jobId": "job-build", "role": "implementer", "goalId": "goal-a", "round": 2,
		"status": "pending", "createdAt": "2026-09-25T09:00:00Z",
		"startedAt": "2026-09-25T09:00:00Z", "endedAt": nil,
		"capMin": 120, "capDeadline": "2026-09-25T11:00:00Z",
	})
	// The claim launch (claim.go:762-763): both, carrying the same number.
	writeJobRecord(t, root, "job-claim", map[string]any{
		"jobId": "job-claim", "parentJob": "job-build", "role": "code-critic",
		"goalId": "goal-a", "round": 1, "status": "running",
		"createdAt": "2026-09-25T09:30:00Z", "startedAt": "2026-09-25T09:30:00Z",
		"capMin": 60, "capRequest": map[string]any{"minutes": 60},
		"reviewRoundLimit": 2,
	})
	// A record with the second alone, which is what this reader used to
	// require: it must still answer, so the fallback is exercised.
	writeJobRecord(t, root, "job-requested", map[string]any{
		"jobId": "job-requested", "role": "warden", "goalId": "goal-b", "round": 1,
		"status": "completed", "createdAt": "2026-09-25T08:00:00Z",
		"startedAt": "2026-09-25T08:00:00Z", "endedAt": "2026-09-25T08:45:00Z",
		"capRequest": map[string]any{"minutes": 45},
	})
	// And one with no cap at all, which is a job with no cap and not a job
	// with a cap of nought.
	writeJobRecord(t, root, "job-capless", map[string]any{
		"jobId": "job-capless", "role": "implementer", "goalId": "goal-b", "round": 1,
		"status": "pending-setup", "createdAt": "2026-09-25T07:00:00Z",
	})
	return root
}

func TestReadJobsReadsTheCapEveryWriterRecords(t *testing.T) {
	t.Parallel()
	jobs := ReadJobs(ledgerFixture(t))
	if len(jobs.Unreadable) != 0 {
		t.Fatalf("unreadable = %v", jobs.Unreadable)
	}
	for _, one := range []struct {
		job     string
		minutes int
	}{
		// The build's shape: capMin alone, which is the common one.
		{"job-build", 120},
		// The claim launch's: both, agreeing.
		{"job-claim", 60},
		// The second alone, which still answers.
		{"job-requested", 45},
	} {
		record, found := jobNamed(jobs, one.job)
		if !found {
			t.Fatalf("job %s was not read", one.job)
		}
		if record.CapMinutes == nil || *record.CapMinutes != one.minutes {
			t.Fatalf("job %s capMinutes = %v, want %d", one.job, record.CapMinutes, one.minutes)
		}
	}
	capless, found := jobNamed(jobs, "job-capless")
	if !found || capless.CapMinutes != nil {
		t.Fatalf("a job with no cap = %v; null is not nought", capless.CapMinutes)
	}
}

func TestReadJobsKeepsWhatTheRecordsAlreadyCarried(t *testing.T) {
	t.Parallel()
	jobs := ReadJobs(ledgerFixture(t))
	build, _ := jobNamed(jobs, "job-build")
	if build.CapDeadline != "2026-09-25T11:00:00Z" {
		t.Fatalf("capDeadline = %q", build.CapDeadline)
	}
	if build.EndedAt != "" || build.ReviewRoundLimit != nil {
		t.Fatalf("a build carries an endedAt or a round limit: %+v", build)
	}
	claim, _ := jobNamed(jobs, "job-claim")
	if claim.ReviewRoundLimit == nil || *claim.ReviewRoundLimit != 2 {
		t.Fatalf("reviewRoundLimit = %v", claim.ReviewRoundLimit)
	}
	requested, _ := jobNamed(jobs, "job-requested")
	if requested.EndedAt != "2026-09-25T08:45:00Z" {
		t.Fatalf("endedAt = %q", requested.EndedAt)
	}
}

func TestTheCapOnTheBuildsShapeReachesTheComposedRecord(t *testing.T) {
	t.Parallel()
	// The whole point of the fallback: a build's job must not publish a
	// phase sentence with no cap in it.
	working, detail := ComposeWorking(ReadJobs(ledgerFixture(t)), nil)
	if detail != "" || working == nil {
		t.Fatalf("working = %+v detail %q", working, detail)
	}
	if working.Job.ID != "job-claim" || working.Job.CapMinutes == nil || *working.Job.CapMinutes != 60 {
		t.Fatalf("newest job = %+v", working.Job)
	}
	for _, member := range working.Chain {
		if member.Job == "job-build" && (member.CapMinutes == nil || *member.CapMinutes != 120) {
			t.Fatalf("the build member's cap = %v", member.CapMinutes)
		}
	}
}

func TestAnAbsentJobDirectoryIsNoJobsAndNotAFailure(t *testing.T) {
	t.Parallel()
	jobs := ReadJobs(t.TempDir())
	if len(jobs.Records) != 0 || len(jobs.Unreadable) != 0 {
		t.Fatalf("jobs = %+v", jobs)
	}
}
