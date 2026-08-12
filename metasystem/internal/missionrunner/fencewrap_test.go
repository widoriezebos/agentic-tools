package missionrunner

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// The engine-level fence wrappers around the (already covered) pure math:
// an absent counters file means no fence is reached; a counters file at its
// limits means reached, witnessed through the engine.
func TestFenceReachedThroughTheEngine(t *testing.T) {
	root := t.TempDir()
	engine := &Engine{Root: root, Mission: "mr-fence"}
	values := map[string]string{
		"fence.wall-clock-hours": "1",
		"fence.cycles":           "2",
		"fence.jobs":             "5",
		"fence.concurrency":      "3",
	}
	// Absent counters: not reached, and no error.
	reached, err := engine.fenceReached(values)
	if err != nil || reached {
		t.Fatalf("absent counters: reached=%v err=%v", reached, err)
	}
	// Counters at the cycle limit: reached.
	dir := filepath.Dir(engine.fencesPath())
	os.MkdirAll(dir, 0o755)
	startedAt := time.Now().UTC().Add(-time.Minute).Format(time.RFC3339)
	counters := `{"startedAt":"` + startedAt + `","cycles":2,"reservations":[]}` + "\n"
	if err := os.WriteFile(engine.fencesPath(), []byte(counters), 0o644); err != nil {
		t.Fatal(err)
	}
	reached, err = engine.fenceReached(values)
	if err != nil {
		t.Fatalf("counters at limit: %v", err)
	}
	if !reached {
		t.Fatal("the cycle fence was reached and not reported")
	}
	// Malformed counters surface as an error, never a silent false.
	os.WriteFile(engine.fencesPath(), []byte("{broken\n"), 0o644)
	if _, err := engine.fenceReached(values); err == nil {
		t.Fatal("malformed counters read as a clean verdict")
	}
}

// The engine's own writer delegation (converted to the durable-write owner).
func TestEngineAtomicWriteBytes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "doc.json")
	if err := atomicWriteBytes(path, []byte("{}\n")); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "{}\n" {
		t.Fatalf("got %q", data)
	}
}
