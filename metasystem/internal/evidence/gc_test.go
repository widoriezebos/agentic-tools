package evidence

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The pass runs against a frozen clock so grace windows and archive ages are
// exact.
var testNow = time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)

func freezeClock(t *testing.T) {
	t.Helper()
	restore := now
	now = func() time.Time { return testNow }
	t.Cleanup(func() { now = restore })
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func digestOf(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

// checkout builds a minimal checkout and evidence root pair.
func checkout(t *testing.T) (root, evidenceRoot, agents, jobs string) {
	t.Helper()
	base := t.TempDir()
	root = filepath.Join(base, "checkout")
	evidenceRoot = filepath.Join(base, "durable")
	agents = filepath.Join(root, "artifacts", "agents")
	jobs = filepath.Join(agents, "jobs")
	if err := os.MkdirAll(jobs, 0o755); err != nil {
		t.Fatal(err)
	}
	return root, evidenceRoot, agents, jobs
}

func runGC(t *testing.T, root, evidenceRoot string) string {
	t.Helper()
	var out strings.Builder
	if err := GC(root, evidenceRoot, 5400, &out); err != nil {
		t.Fatalf("GC: %v", err)
	}
	return out.String()
}

func manifestJSON(updatedAt string, files map[string]string) string {
	var entries []string
	for name, sha := range files {
		entries = append(entries, fmt.Sprintf("%q: {\"sha256\": %q}", name, sha))
	}
	return fmt.Sprintf("{\"updatedAt\": %q, \"files\": {%s}}", updatedAt, strings.Join(entries, ", "))
}

func TestGCRefusesWithoutAbsoluteEvidenceRoot(t *testing.T) {
	var out strings.Builder
	if err := GC(t.TempDir(), "relative/evidence", 5400, &out); err == nil {
		t.Fatal("expected a refusal for a non-absolute evidence root")
	}
}

func TestCollectsClosedFullyMirroredChain(t *testing.T) {
	freezeClock(t)
	root, evidenceRoot, agents, jobs := checkout(t)
	payload := "the round result\n"
	logContent := "adapter log\n"
	writeFile(t, filepath.Join(agents, "chain-a", "rounds", "1", "return.json"), payload)
	writeFile(t, filepath.Join(jobs, "chain-a.json"), `{"jobId": "chain-a", "status": "completed", "chainClosed": true}`)
	writeFile(t, filepath.Join(jobs, "chain-a-r2.json"), `{"jobId": "chain-a-r2", "status": "failed"}`)
	writeFile(t, filepath.Join(jobs, "chain-a.log"), logContent)
	// A sibling chain that merely shares the prefix must not gate collection.
	writeFile(t, filepath.Join(jobs, "chain-a-extra.json"), `{"jobId": "chain-a-extra", "status": "running"}`)
	writeFile(t, filepath.Join(evidenceRoot, "agents", "seg1", "chain-a", "manifest.json"),
		manifestJSON("2026-08-10T10:00:00Z", map[string]string{
			"rounds/1/return.json": digestOf(payload),
			"jobs/chain-a.log":     digestOf(logContent),
		}))

	out := runGC(t, root, evidenceRoot)

	if !strings.Contains(out, "collected chain-a\n") {
		t.Fatalf("chain was not collected: %s", out)
	}
	if _, err := os.Stat(filepath.Join(agents, "chain-a")); !os.IsNotExist(err) {
		t.Fatal("chain payload should be gone")
	}
	if _, err := os.Stat(filepath.Join(jobs, "chain-a.log")); !os.IsNotExist(err) {
		t.Fatal("the mirrored log should be gone")
	}
	// Job records always stay at collection time: they are the registry.
	if _, err := os.Stat(filepath.Join(jobs, "chain-a.json")); err != nil {
		t.Fatal("the job record must stay")
	}
}

func TestKeepsChainsTheMirrorCannotVouchFor(t *testing.T) {
	freezeClock(t)
	root, evidenceRoot, agents, jobs := checkout(t)

	// A round still live.
	writeFile(t, filepath.Join(agents, "live", "rounds", "1", "x"), "x")
	writeFile(t, filepath.Join(jobs, "live.json"), `{"jobId": "live", "status": "running"}`)
	// Terminal but not closed: working state between rounds.
	writeFile(t, filepath.Join(agents, "open", "rounds", "1", "x"), "x")
	writeFile(t, filepath.Join(jobs, "open.json"), `{"jobId": "open", "status": "completed"}`)
	// Closed but never mirrored.
	writeFile(t, filepath.Join(agents, "unmirrored", "rounds", "1", "x"), "x")
	writeFile(t, filepath.Join(jobs, "unmirrored.json"), `{"jobId": "unmirrored", "status": "completed", "chainClosed": true}`)
	// Closed and mirrored, but a payload file diverged from the manifest.
	writeFile(t, filepath.Join(agents, "diverged", "rounds", "1", "x"), "local edit")
	writeFile(t, filepath.Join(jobs, "diverged.json"), `{"jobId": "diverged", "status": "completed", "chainClosed": true}`)
	writeFile(t, filepath.Join(evidenceRoot, "agents", "diverged", "manifest.json"),
		manifestJSON("2026-08-10T10:00:00Z", map[string]string{"rounds/1/x": digestOf("mirrored bytes")}))
	// A directory with no job records at all.
	writeFile(t, filepath.Join(agents, "stray", "note.txt"), "not a chain")

	out := runGC(t, root, evidenceRoot)

	for chain, reason := range map[string]string{
		"live":       "a round is still live",
		"open":       "chain not closed; working state",
		"unmirrored": "no mirror manifest",
		"diverged":   "mirror does not account for: rounds/1/x",
		"stray":      "no job records; not this tool's to judge",
	} {
		if !strings.Contains(out, "kept      "+chain+": "+reason) {
			t.Fatalf("expected %s kept with %q, got: %s", chain, reason, out)
		}
		if _, err := os.Stat(filepath.Join(agents, chain)); err != nil {
			t.Fatalf("%s payload must survive: %v", chain, err)
		}
	}
	if !strings.Contains(out, "evidence-gc: 0 collected, 5 kept\n") {
		t.Fatalf("unexpected summary: %s", out)
	}
}

func TestPrunesMirroredRecordsPastTheGraceWindow(t *testing.T) {
	freezeClock(t)
	root, evidenceRoot, _, jobs := checkout(t)

	// Collected long ago: mirrored well past the grace window.
	writeFile(t, filepath.Join(jobs, "old.json"), `{"jobId": "old", "status": "completed"}`)
	writeFile(t, filepath.Join(jobs, "old-r2.json"), `{"jobId": "old-r2", "status": "completed"}`)
	writeFile(t, filepath.Join(evidenceRoot, "agents", "old", "manifest.json"),
		manifestJSON("2026-08-10T08:00:00Z", map[string]string{
			"jobs/old.json":    "irrelevant",
			"jobs/old-r2.json": "irrelevant",
		}))
	// Mirrored only recently: records stay for the staleness and census reads.
	writeFile(t, filepath.Join(jobs, "recent.json"), `{"jobId": "recent", "status": "completed"}`)
	writeFile(t, filepath.Join(evidenceRoot, "agents", "recent", "manifest.json"),
		manifestJSON("2026-08-10T11:30:00Z", map[string]string{"jobs/recent.json": "irrelevant"}))
	// Terminal, mirrored, old — but the manifest never captured this record.
	writeFile(t, filepath.Join(jobs, "old-r3.json"), `{"jobId": "old-r3", "status": "completed"}`)
	// Not terminal: never pruned.
	writeFile(t, filepath.Join(jobs, "busy.json"), `{"jobId": "busy", "status": "running"}`)

	runGC(t, root, evidenceRoot)

	for record, wantGone := range map[string]bool{
		"old.json":    true,
		"old-r2.json": true,
		"old-r3.json": false,
		"recent.json": false,
		"busy.json":   false,
	} {
		_, err := os.Stat(filepath.Join(jobs, record))
		if gone := os.IsNotExist(err); gone != wantGone {
			t.Fatalf("%s: gone=%v, want %v", record, gone, wantGone)
		}
	}
}

func TestSweepsResidueOfTerminalJobs(t *testing.T) {
	freezeClock(t)
	root, evidenceRoot, agents, jobs := checkout(t)
	writeFile(t, filepath.Join(jobs, "done.json"), `{"jobId": "done", "status": "completed"}`)
	writeFile(t, filepath.Join(jobs, "busy.json"), `{"jobId": "busy", "status": "running"}`)

	// Heartbeats: terminal job's go, live job's stay.
	writeFile(t, filepath.Join(agents, "hb", "done"), "hb")
	writeFile(t, filepath.Join(agents, "hb", "done.start"), "hb")
	writeFile(t, filepath.Join(agents, "hb", "busy"), "hb")
	// Locks and lifecycle dirs follow the job's status.
	writeFile(t, filepath.Join(agents, "record-locks", "done.lock"), "")
	writeFile(t, filepath.Join(agents, "record-locks", "done.lifecycle.d", "step"), "")
	writeFile(t, filepath.Join(agents, "record-locks", "busy.lock"), "")
	// mktemp leftovers go only once they are stale.
	stale := filepath.Join(agents, "record-locks", "tmp.stale")
	writeFile(t, stale, "")
	os.Chtimes(stale, testNow.Add(-2*time.Hour), testNow.Add(-2*time.Hour))
	fresh := filepath.Join(agents, "record-locks", "tmp.fresh")
	writeFile(t, fresh, "")
	os.Chtimes(fresh, testNow, testNow)

	out := runGC(t, root, evidenceRoot)

	for path, wantGone := range map[string]bool{
		filepath.Join(agents, "hb", "done"):                       true,
		filepath.Join(agents, "hb", "done.start"):                 true,
		filepath.Join(agents, "hb", "busy"):                       false,
		filepath.Join(agents, "record-locks", "done.lock"):        true,
		filepath.Join(agents, "record-locks", "done.lifecycle.d"): true,
		filepath.Join(agents, "record-locks", "busy.lock"):        false,
		stale: true,
		fresh: false,
	} {
		_, err := os.Stat(path)
		if gone := os.IsNotExist(err); gone != wantGone {
			t.Fatalf("%s: gone=%v, want %v", path, gone, wantGone)
		}
	}
	// The hb directory empties and is itself pruned as a non-spine empty...
	if !strings.Contains(out, "residue removed:") {
		t.Fatalf("missing residue line: %s", out)
	}
}

func TestKeepsNewestSnapshotPerConfigurationIdentity(t *testing.T) {
	freezeClock(t)
	root, evidenceRoot, agents, _ := checkout(t)
	caps := filepath.Join(agents, "capabilities")
	// Same identity: only the newest capture survives. The config hash sorts
	// against the date on purpose: newest is by capture stamp, not file name.
	writeFile(t, filepath.Join(caps, "rt-1.0-zzz-20260801-002.json"), "{}")
	writeFile(t, filepath.Join(caps, "rt-1.0-zzz-20260802-001.json"), "{}")
	// A different config hash is a different identity and must survive, even
	// though its capture is older than the other identity's newest.
	writeFile(t, filepath.Join(caps, "rt-1.0-aaa-20260730-001.json"), "{}")

	runGC(t, root, evidenceRoot)

	for name, wantGone := range map[string]bool{
		"rt-1.0-zzz-20260801-002.json": true,
		"rt-1.0-zzz-20260802-001.json": false,
		"rt-1.0-aaa-20260730-001.json": false,
	} {
		_, err := os.Stat(filepath.Join(caps, name))
		if gone := os.IsNotExist(err); gone != wantGone {
			t.Fatalf("%s: gone=%v, want %v", name, gone, wantGone)
		}
	}
}

func TestPrunesEmptyDirsButNeverTheSpine(t *testing.T) {
	freezeClock(t)
	root, evidenceRoot, agents, _ := checkout(t)
	for _, dir := range []string{"capabilities", "mains", "record-locks", "supervision"} {
		if err := os.MkdirAll(filepath.Join(agents, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(agents, "supervision", "armed"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(agents, "worktrees", "gone", "deeper"), 0o755); err != nil {
		t.Fatal(err)
	}

	runGC(t, root, evidenceRoot)

	for _, dir := range []string{"jobs", "capabilities", "mains", "record-locks", "supervision"} {
		if _, err := os.Stat(filepath.Join(agents, dir)); err != nil {
			t.Fatalf("spine dir %s must survive empty: %v", dir, err)
		}
	}
	if _, err := os.Stat(filepath.Join(agents, "supervision", "armed")); err != nil {
		t.Fatal("the supervision tree must never be pruned")
	}
	if _, err := os.Stat(filepath.Join(agents, "worktrees")); !os.IsNotExist(err) {
		t.Fatal("nested empty non-spine dirs should collapse in one pass")
	}
}

func TestAgesEventArchivesOnlyAfterAVerifiedDurableCopy(t *testing.T) {
	freezeClock(t)
	root, evidenceRoot, agents, _ := checkout(t)
	archiveDir := filepath.Join(agents, "events-archive")
	durableDir := filepath.Join(evidenceRoot, "events", filepath.Base(root))

	oldName := "events-20260701T000000Z-abc.jsonl" // 40 days old by stamp
	youngName := "events-20260809T000000Z-def.jsonl"
	writeFile(t, filepath.Join(archiveDir, oldName), "old events\n")
	writeFile(t, filepath.Join(archiveDir, youngName), "young events\n")
	// A foreign same-name durable file must be replaced, never trusted.
	writeFile(t, filepath.Join(durableDir, oldName), "corrupt copy")
	writeFile(t, filepath.Join(archiveDir, "notes.txt"), "ignored")

	out := runGC(t, root, evidenceRoot)

	if data, err := os.ReadFile(filepath.Join(durableDir, oldName)); err != nil || string(data) != "old events\n" {
		t.Fatalf("durable copy not verified byte-identical: %q, %v", data, err)
	}
	if _, err := os.Stat(filepath.Join(archiveDir, oldName)); !os.IsNotExist(err) {
		t.Fatal("an old, verified-durable archive should be aged out locally")
	}
	if !strings.Contains(out, "collected events archive "+oldName+"\n") {
		t.Fatalf("missing archive collection line: %s", out)
	}
	// The young archive is copied for durability but kept locally.
	if data, err := os.ReadFile(filepath.Join(durableDir, youngName)); err != nil || string(data) != "young events\n" {
		t.Fatalf("young archive not copied: %q, %v", data, err)
	}
	if _, err := os.Stat(filepath.Join(archiveDir, youngName)); err != nil {
		t.Fatal("a young archive stays local even once durable")
	}
}
