package spend

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func mustSpendTest(t *testing.T, err error) {
	if err != nil {
		t.Fatal(err)
	}
}
func TestDiscoveryFlagsSubagentFilesFromPaths(t *testing.T) {
	if len(readerRegistry) != 1 || readerRegistry[0].name != "claude" ||
		readerRegistry[0].capability != readerCapabilityPerCall || !readerRegistry[0].inScope {
		t.Fatalf("Claude reader registration is incomplete: %+v", readerRegistry)
	}
	root := t.TempDir()
	mustSpendTest(t, os.Mkdir(filepath.Join(root, ".git"), 0o755))
	copyFixtureFile(t, filepath.Join("testdata", "bed-20260902", "metasystem.conf"), filepath.Join(root, "metasystem.conf"), nil)
	home := t.TempDir()
	t.Setenv("HOME", home)
	slug := strings.ReplaceAll(filepath.Clean(root), string(filepath.Separator), "-")
	directory := filepath.Join(home, ".claude", "projects", slug)
	writeSeatTranscript(t, filepath.Join(directory, "top.jsonl"), "top", root, "top-request", 1, 2)
	writeSeatTranscript(t, filepath.Join(directory, "parent", "subagents", "child.jsonl"), "child", root, "child-request", 3, 4)
	discovered := readerRegistry[0].discover(root)
	if len(discovered.files) != 2 {
		t.Fatalf("discovered %d files, want 2: %+v", len(discovered.files), discovered.files)
	}
	files := map[string]transcriptFile{}
	for _, file := range discovered.files {
		files[file.session] = file
	}
	if files["top"].delegate || files["top"].parentSession != "" ||
		!files["child"].delegate || files["child"].parentSession != "parent" {
		t.Fatalf("path metadata is wrong: %+v", files)
	}
	original := readerRegistry
	other := claudeReader()
	other.name, other.inScope = "other", false
	readerRegistry = append(readerRegistry, other)
	defer func() { readerRegistry = original }()
	ledger, err := Measure(root, "bed-m1", bedNow)
	if err != nil || ledger.Seat.Files != 2 || ledger.Seat.LifetimeTokens != 10 || readerScopeLabel(readerRegistry) != "claude" {
		t.Fatalf("out-of-scope reader changed measurement: seat=%+v label=%q err=%v", ledger.Seat, readerScopeLabel(readerRegistry), err)
	}
	readerRegistry[1].inScope = true
	ledger, err = Measure(root, "bed-m1", bedNow)
	if err != nil || ledger.Seat.Files != 4 || readerScopeLabel(readerRegistry) != "claude+other" {
		t.Fatalf("in-scope reader was not measured: seat=%+v label=%q err=%v", ledger.Seat, readerScopeLabel(readerRegistry), err)
	}
	readerRegistry = original
	jobs := filepath.Join(root, "artifacts", "agents", "jobs")
	mustSpendTest(t, os.MkdirAll(jobs, 0o755))
	mustSpendTest(t, os.WriteFile(filepath.Join(jobs, "parent.json"), []byte(`{"jobId":"parent","status":"completed","runtime":"claude","canonicalModelKey":"claude-sonnet-4-20250514","startedAt":"2026-09-02T10:00:00Z","sessionId":"parent","usage":{"inputTokens":7}}`), 0o644))
	ledger, err = Measure(root, "bed-m1", bedNow)
	if err != nil || ledger.Seat.Files != 1 || ledger.Seat.LifetimeTokens != 3 {
		t.Fatalf("delegate parent's subagent transcript entered seat spend: seat=%+v err=%v", ledger.Seat, err)
	}
}
func TestSeamKeepsEveryVisibleGap(t *testing.T) {
	bed := newSpendBed(t)
	slug := strings.ReplaceAll(filepath.Clean(bed.root), string(filepath.Separator), "-")
	projects := filepath.Join(os.Getenv("HOME"), ".claude", "projects")
	blocked := filepath.Join(projects, slug+"-blocked")
	mustSpendTest(t, os.MkdirAll(blocked, 0o755))
	mustSpendTest(t, os.Chmod(blocked, 0))
	t.Cleanup(func() { _ = os.Chmod(blocked, 0o755) })
	discovered := discoverClaudeTranscripts(bed.root)
	writeSeatTranscript(t, filepath.Join(projects, slug+"-foreign", "foreign.jsonl"), "foreign", t.TempDir(), "foreign", 3, 4)
	cache := spendCacheDir(bed.root)
	mustSpendTest(t, os.MkdirAll(filepath.Dir(cache), 0o755))
	mustSpendTest(t, os.Mkdir(cache, 0o555))
	t.Cleanup(func() { _ = os.Chmod(cache, 0o755) })
	ledger, err := Measure(bed.root, "bed-m1", bed.now)
	if len(discovered.gaps) != 1 || discovered.gaps[0].Provenance != "seat unreadable" || discovered.counters.UnreadableFiles != 1 || err != nil || ledger.Seat.UnreadableFiles != 2 || ledger.Seat.AgedFiles != 1 || ledger.Seat.SkippedForeignFiles != 1 || ledger.Seat.UnmeasuredRequests != 2 || ledger.Seat.CacheWriteFailures == 0 {
		t.Fatalf("reader seam hid a visible gap: seat=%+v entries=%+v err=%v", ledger.Seat, ledger.Unmeasured, err)
	}
}
