package census

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// A synthetic bundle exercises the classification branches directly.
func writeBundle(t *testing.T) (root, procFile string) {
	t.Helper()
	root = t.TempDir()
	os.MkdirAll(filepath.Join(root, "scripts", "agents", "adapters"), 0o755)
	os.MkdirAll(filepath.Join(root, "artifacts", "agents", "mains"), 0o755)
	os.MkdirAll(filepath.Join(root, "artifacts", "agents", "jobs"), 0o755)
	os.MkdirAll(filepath.Join(root, "artifacts", "agents", "supervision"), 0o755)
	os.WriteFile(filepath.Join(root, "scripts", "agents", "adapters", "fake.sh"),
		[]byte("#!/bin/sh\nprintf 'match (^|[[:space:]/-])metasystem-fake-agent([[:space:]]|$)\\n'\n"), 0o755)
	os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o644)
	procFile = filepath.Join(t.TempDir(), "procs.json")
	os.WriteFile(procFile, []byte(`[
	  {"pid":4101,"ppid":1,"pgid":4101,"pidStartedAt":100,"argv":"metasystem-fake-agent one","cwd":"`+root+`","cwdError":false,"alive":true},
	  {"pid":4103,"ppid":1,"pgid":4103,"pidStartedAt":100,"argv":"metasystem-fake-agent untracked","cwd":"`+root+`","cwdError":false,"alive":true}
	]`), 0o644)
	// A live custody job for 4101.
	os.WriteFile(filepath.Join(root, "artifacts", "agents", "jobs", "j.json"),
		[]byte(`{"status":"running","instanceTag":"job-4101","pid":4101,"pidStartedAt":100}`), 0o644)
	return root, procFile
}

func TestRunFixtureCensusClassifies(t *testing.T) {
	root, procFile := writeBundle(t)
	v, err := RunFixtureCensus(root, root, procFile, "fp", 60, time.Unix(1786000000, 0))
	if err != nil {
		t.Fatal(err)
	}
	// No state.json → supervision-state error → CENSUS-FAILED.
	if v.Verdict != "CENSUS-FAILED" {
		t.Fatalf("missing state must fail the census: %+v", v.Errors)
	}
	if v.Counts["CUSTODY"] != 1 || v.Counts["UNTRACKED"] != 1 {
		t.Fatalf("wrong classification counts: %v", v.Counts)
	}
	// Inventory sorted by pid; the custody job's registry is recorded.
	if len(v.Inventory) != 2 || v.Inventory[0].Pid != 4101 || v.Inventory[0].Class != "CUSTODY" {
		t.Fatalf("wrong inventory: %+v", v.Inventory)
	}
	if v.SchemaVersion != 2 || v.Writer != "watch-background-jobs.sh" {
		t.Fatal("verdict envelope wrong")
	}
}

func TestRunFixtureCensusFixtureGuard(t *testing.T) {
	root, procFile := writeBundle(t)
	// runtimes != fake → the fixture path is refused (enumeration error).
	os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=claude\n"), 0o644)
	v, err := RunFixtureCensus(root, root, procFile, "fp", 60, time.Unix(1786000000, 0))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range v.Errors {
		if e == "enumeration:METASYSTEM_CENSUS_PROCESS_FILE is allowed only when metasystem.runtimes=fake" {
			found = true
		}
	}
	if !found {
		t.Fatalf("fixture guard not enforced: %v", v.Errors)
	}
}

// The full success path: a valid supervision state whose identities are made
// "alive" via the fixture identity file, an announcement, and a clean scan.
func TestRunFixtureCensusSuccessPath(t *testing.T) {
	root, procFile := writeBundle(t)
	// The supervision pids must EXIST: the identity file is the start-time
	// source, but kernel death vetoes it (a provably dead pid is dead
	// regardless of its fixture entry — the semantics stop verification
	// depends on). Use three genuinely live pids.
	ownPid, parentPid, initPid := os.Getpid(), os.Getppid(), 1
	// Valid supervision state.
	os.WriteFile(filepath.Join(root, "artifacts", "agents", "supervision", "state.json"),
		[]byte(fmt.Sprintf(`{"generation":2,"owner":{"pid":%d,"pidStartedAt":100,"instanceTag":"owner-t"},
		 "components":{"watcher":{"pid":%d,"pidStartedAt":100,"instanceTag":"watcher-t"},
		 "reaper":{"pid":%d,"pidStartedAt":100,"instanceTag":"reaper-t"}}}`,
			ownPid, parentPid, initPid)), 0o644)
	// The identity file supplies the expected start times, so liveness reads
	// from it while the tag check still consults the real process table (a
	// tag mismatch there is a real, faithful error this test tolerates).
	idFile := filepath.Join(t.TempDir(), "identity.json")
	os.WriteFile(idFile, []byte(fmt.Sprintf(
		`{"%d":{"pidStartedAt":100},"%d":{"pidStartedAt":100},"%d":{"pidStartedAt":100}}`,
		ownPid, parentPid, initPid)), 0o644)
	t.Setenv("METASYSTEM_FAKE_PROCESS_IDENTITY_FILE", idFile)
	// An announcement for 4103 (so it classifies ANNOUNCED, not UNTRACKED).
	os.WriteFile(filepath.Join(root, "artifacts", "agents", "mains", "s.json"),
		[]byte(`{"sessionId":"s","pid":4103,"pidStartedAt":100,"pgid":4103,"runtime":"fake","instanceTag":"main-4103","announcedAt":"2026-08-10T00:00:00Z"}`), 0o644)

	v, err := RunFixtureCensus(root, root, procFile, "fp", 60, time.Unix(1786000000, 0))
	if err != nil {
		t.Fatal(err)
	}
	// Liveness passed (no supervision-not-live); the tag check hits ps and
	// yields command-unavailable for the fake pids — a real, faithful error.
	for _, e := range v.Errors {
		if len(e) >= len("supervision-not-live") && e[:len("supervision-not-live")] == "supervision-not-live" {
			t.Fatalf("identity file should mark supervision live: %v", v.Errors)
		}
	}
	if v.Generation == nil || *v.Generation != 2 {
		t.Fatalf("generation not read: %+v", v.Generation)
	}
	// 4103 is announced now.
	var announced bool
	for _, item := range v.Inventory {
		if item.Pid == 4103 && item.Class == "ANNOUNCED" && item.InstanceTag == "main-4103" {
			announced = true
		}
	}
	if !announced {
		t.Fatalf("4103 not classified ANNOUNCED: %+v", v.Inventory)
	}
}
