package missionrunner

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestActiveJobs(t *testing.T) {
	root := t.TempDir()
	mission := "demo"
	jobs := jobsDirPath(root)
	writeJSONFile(t, filepath.Join(jobs, "done.json"), map[string]any{"jobId": "done", "mission": mission, "status": "completed"})
	writeJSONFile(t, filepath.Join(jobs, "busy.json"), map[string]any{"jobId": "busy", "mission": mission, "status": "running"})
	writeJSONFile(t, filepath.Join(jobs, "limbo.json"), map[string]any{"jobId": "limbo", "mission": mission})
	writeJSONFile(t, filepath.Join(jobs, "foreign.json"), map[string]any{"jobId": "foreign", "mission": "other", "status": "running"})
	writeJSONFile(t, filepath.Join(jobs, "anon.json"), map[string]any{"mission": mission, "status": "pending"})
	if err := os.WriteFile(filepath.Join(jobs, "corrupt.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := ActiveJobs(root, mission)
	// A record without a status is active; one without a jobId reports under
	// its file stem; other missions' jobs and unreadable records are ignored.
	want := []string{"anon", "busy", "limbo"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("active jobs: got %v, want %v", got, want)
	}
}

func TestCloseableChains(t *testing.T) {
	root := t.TempDir()
	mission := "demo"
	jobs := jobsDirPath(root)
	// r1's chain is fully terminal and unclosed: closeable.
	writeJSONFile(t, filepath.Join(jobs, "r1.json"), map[string]any{"jobId": "r1", "mission": mission, "status": "completed"})
	writeJSONFile(t, filepath.Join(jobs, "c1.json"), map[string]any{"jobId": "c1", "mission": mission, "status": "failed", "parentJob": "r1"})
	// r2's chain still has a running member: not closeable.
	writeJSONFile(t, filepath.Join(jobs, "r2.json"), map[string]any{"jobId": "r2", "mission": mission, "status": "completed"})
	writeJSONFile(t, filepath.Join(jobs, "c2.json"), map[string]any{"jobId": "c2", "mission": mission, "status": "running", "parentJob": "r2"})
	// r3 is terminal but already closed.
	writeJSONFile(t, filepath.Join(jobs, "r3.json"), map[string]any{"jobId": "r3", "mission": mission, "status": "completed", "chainClosed": true})
	// x and y point at each other: a cycle belongs to no chain and blocks none.
	writeJSONFile(t, filepath.Join(jobs, "x.json"), map[string]any{"jobId": "x", "mission": mission, "status": "running", "parentJob": "y"})
	writeJSONFile(t, filepath.Join(jobs, "y.json"), map[string]any{"jobId": "y", "mission": mission, "status": "running", "parentJob": "x"})
	// orphan's parent is not among this mission's records: dropped, not blocking.
	writeJSONFile(t, filepath.Join(jobs, "orphan.json"), map[string]any{"jobId": "orphan", "mission": mission, "status": "running", "parentJob": "elsewhere"})

	got := CloseableChains(root, mission)
	if !reflect.DeepEqual(got, []string{"r1"}) {
		t.Fatalf("closeable chains: got %v, want [r1]", got)
	}
}

func TestCloseableChainsRootAlone(t *testing.T) {
	root := t.TempDir()
	mission := "demo"
	writeJSONFile(t, filepath.Join(jobsDirPath(root), "solo.json"), map[string]any{"jobId": "solo", "mission": mission, "status": "cancelled"})
	if got := CloseableChains(root, mission); !reflect.DeepEqual(got, []string{"solo"}) {
		t.Fatalf("a lone terminal root closes its own chain: got %v", got)
	}
}

func TestActiveJobsSeesUnstampedReservationHusks(t *testing.T) {
	// A dispatch that crashes during setup leaves a pending-setup record with
	// no mission stamp while the mission's fence reservation already names
	// the job. The drain must see it, or the concurrency slot leaks forever.
	root := t.TempDir()
	mission := "demo"
	writeJSONFile(t, filepath.Join(jobsDirPath(root), "husk-1.json"),
		map[string]any{"jobId": "husk-1", "status": "pending-setup"})
	writeJSONFile(t, filepath.Join(root, "artifacts", "agents", "missions", mission, "fences.json"),
		map[string]any{"reservations": map[string]any{"husk-1": map[string]any{"capMin": 15}}})
	got := ActiveJobs(root, mission)
	want := []string{"husk-1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("active jobs: got %v, want %v", got, want)
	}
}
