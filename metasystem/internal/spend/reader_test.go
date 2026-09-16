package spend

import (
	"encoding/json"
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

func TestCallIdentityReproducesThePrototype(t *testing.T) {
	root := t.TempDir()
	mustSpendTest(t, os.Mkdir(filepath.Join(root, ".git"), 0o755))
	copyFixtureFile(t, filepath.Join("testdata", "bed-20260902", "metasystem.conf"), filepath.Join(root, "metasystem.conf"), nil)
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".claude", "projects", strings.ReplaceAll(filepath.Clean(root), string(filepath.Separator), "-"))
	mustSpendTest(t, os.MkdirAll(dir, 0o755))
	usage := func(in, create, read, out, reason int) map[string]any {
		return map[string]any{"input_tokens": in, "cache_creation_input_tokens": create, "cache_read_input_tokens": read, "output_tokens": out, "thinking_tokens": reason}
	}
	line := func(id, request, model, stamp string, tokens map[string]any) []byte {
		message := map[string]any{"model": model, "usage": tokens}
		if id != "" {
			message["id"] = id
		}
		raw, _ := json.Marshal(map[string]any{"type": "assistant", "sessionId": "seat", "requestId": request, "cwd": root, "timestamp": stamp, "message": message})
		return append(raw, '\n')
	}
	a := line("same-file", "ignored-a", "first-model", "2026-09-02T10:00:00Z", usage(10, 2, 3, 4, 5))
	a = append(a, line("same-file", "ignored-b", "later-model", "2026-09-02T11:00:00Z", usage(100, 0, 0, 8, 0))...)
	a = append(a, line("cross-file", "cross-a", "cross-late", "2026-09-02T12:00:00Z", usage(20, 0, 0, 2, 0))...)
	a = append(a, line("", "request-key", "request-model", "2026-09-02T10:00:00Z", usage(1, 0, 0, 2, 0))...)
	a = append(a, line("", "", "line-model", "2026-09-02T10:00:00Z", usage(2, 0, 0, 3, 0))...)
	a = append(a, line("synthetic", "synthetic", "<synthetic>", "2026-09-02T10:00:00Z", usage(9, 0, 0, 9, 0))...)
	a = append(a, line("measured-after-gap", "gap", "unknown", "2026-09-02T10:00:00Z", nil)...)
	a = append(a, line("measured-after-gap", "measured", "measured-model", "2026-09-02T10:01:00Z", usage(6, 7, 8, 9, 10))...)
	a = append(a, line("measured-after-bad-stamp", "bad-stamp", "bad-stamp-model", "invalid", usage(60, 0, 0, 6, 0))...)
	a = append(a, line("measured-after-bad-stamp", "measured", "measured-stamp-model", "2026-09-02T10:02:00Z", usage(1, 2, 3, 4, 5))...)
	mustSpendTest(t, os.WriteFile(filepath.Join(dir, "a.jsonl"), a, 0o644))
	mustSpendTest(t, os.WriteFile(filepath.Join(dir, "b.jsonl"), line("cross-file", "cross-b", "cross-early", "2026-09-02T09:00:00Z", usage(7, 1, 2, 3, 4)), 0o644))
	ledger, err := Measure(root, "bed-m1", bedNow)
	if err != nil {
		t.Fatal(err)
	}
	models := map[string]float64{}
	for _, row := range ledger.Rows {
		if row.Goal == "seat" {
			models[row.Model] += row.Tokens.Total()
		}
	}
	if ledger.Seat.LifetimeTokens != 108 || ledger.Seat.UnmeasuredRequests != 1 || models["measured-model"] != 40 || models["measured-stamp-model"] != 15 || models["first-model"] != 28 || models["cross-early"] != 17 || models["later-model"] != 0 || models["cross-late"] != 0 {
		t.Fatalf("identity or raw classes diverged: seat=%+v models=%v gaps=%+v", ledger.Seat, models, ledger.Unmeasured)
	}
}

func TestJobDigestCoversOnlyOwnershipFields(t *testing.T) {
	base := map[string]any{"jobId": "job", "sessionId": "session", "resumedSessionId": "parent", "role": "implementer", "runtime": "claude", "startedAt": "2026-09-02T10:00:00Z", "status": "completed"}
	digest := func(path string, record map[string]any) (string, readerJobs) {
		jobs := newReaderJobs()
		jobs.add(path, record)
		return jobDigest(jobs), jobs
	}
	want, index := digest("a.json", base)
	if len(index.bySession["session"]) != 1 || !index.referencedSessions["parent"] {
		t.Fatalf("job index is incomplete: %+v", index)
	}
	for _, change := range []struct{ path, field, value string }{{"b.json", "", ""}, {"a.json", "jobId", "other"}, {"a.json", "sessionId", "other"}, {"a.json", "resumedSessionId", "other"}, {"a.json", "role", "other"}, {"a.json", "runtime", "other"}, {"a.json", "startedAt", "2026-09-02T11:00:00Z"}} {
		mutated := map[string]any{}
		for key, value := range base {
			mutated[key] = value
		}
		if change.field != "" {
			mutated[change.field] = change.value
		}
		if got, _ := digest(change.path, mutated); got == want {
			t.Fatalf("%s did not invalidate the job digest", change.field)
		}
	}
	base["status"] = "failed"
	if got, _ := digest("a.json", base); got != want {
		t.Fatal("status invalidated the job digest")
	}
}
