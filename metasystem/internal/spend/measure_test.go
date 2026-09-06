package spend

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/mission"
)

var bedNow = time.Date(2026, 9, 2, 20, 0, 0, 0, time.UTC)

type spendBed struct {
	root string
	now  time.Time
}

func newSpendBed(t *testing.T) spendBed {
	t.Helper()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	copyFixtureFile(t, filepath.Join("testdata", "bed-20260902", "metasystem.conf"), filepath.Join(root, "metasystem.conf"), nil)
	copyFixtureTree(t, filepath.Join("testdata", "bed-20260902", "jobs"), filepath.Join(root, "artifacts", "agents", "jobs"), nil)

	home := t.TempDir()
	t.Setenv("HOME", home)
	slug := strings.ReplaceAll(filepath.Clean(root), string(filepath.Separator), "-")
	transcripts := filepath.Join(home, ".claude", "projects", slug)
	rewrite := func(data []byte) []byte { return []byte(strings.ReplaceAll(string(data), "@REPO@", root)) }
	copyFixtureTree(t, filepath.Join("testdata", "bed-20260902", "transcripts"), transcripts, rewrite)
	aged := bedNow.Add(-72 * time.Hour)
	if err := os.Chtimes(filepath.Join(transcripts, "aged-session.jsonl"), aged, aged); err != nil {
		t.Fatal(err)
	}
	return spendBed{root: root, now: bedNow}
}

func writeSeatTranscript(t *testing.T, path, session, cwd, request string, input, output int) {
	t.Helper()
	line := seatTranscriptLine(t, session, cwd, request, input, output)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, line, 0o644); err != nil {
		t.Fatal(err)
	}
}

func seatTranscriptLine(t *testing.T, session, cwd, request string, input, output int) []byte {
	t.Helper()
	line, err := json.Marshal(map[string]any{
		"type": "assistant", "sessionId": session, "requestId": request,
		"cwd": cwd, "timestamp": bedNow.Format(time.RFC3339),
		"message": map[string]any{
			"model": "claude-sonnet-4-20250514",
			"usage": map[string]any{"input_tokens": input, "output_tokens": output},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return append(line, '\n')
}

func copyFixtureTree(t *testing.T, source, destination string, rewrite func([]byte) []byte) {
	t.Helper()
	entries, err := os.ReadDir(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(destination, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			copyFixtureTree(t, filepath.Join(source, entry.Name()), filepath.Join(destination, entry.Name()), rewrite)
			continue
		}
		copyFixtureFile(t, filepath.Join(source, entry.Name()), filepath.Join(destination, entry.Name()), rewrite)
	}
}

func copyFixtureFile(t *testing.T, source, destination string, rewrite func([]byte) []byte) {
	t.Helper()
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if rewrite != nil {
		data = rewrite(data)
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func measureBed(t *testing.T) (spendBed, Ledger) {
	t.Helper()
	bed := newSpendBed(t)
	ledger, err := Measure(bed.root, "bed-m1", bed.now)
	if err != nil {
		t.Fatal(err)
	}
	return bed, ledger
}

func closeEnough(left, right float64) bool { return math.Abs(left-right) < 0.000001 }

func TestMeasureReplays20260902Bed(t *testing.T) {
	_, ledger := measureBed(t)
	if !closeEnough(ledger.DayScope.Tokens, 173523756) || !closeEnough(ledger.DayScope.Money, 67.911555) {
		t.Fatalf("machine-day totals do not replay the observed bed: %+v", ledger.DayScope)
	}
	dispatch := ledger.GoalScopes["dispatch-cap-necessity"]
	if !closeEnough(dispatch.Tokens, 33922917) || !closeEnough(dispatch.Money, 38.34) || dispatch.Unpriced != 4 || dispatch.Unmeasured != 1 {
		t.Fatalf("dispatch goal totals changed: %+v", dispatch)
	}
	twoBars := ledger.GoalScopes["two-bars-for-changes"]
	if !closeEnough(twoBars.Tokens, 21174914) || !closeEnough(twoBars.Money, 29.571555) || twoBars.Unpriced != 3 || twoBars.Unmeasured != 1 {
		t.Fatalf("two-bars goal totals changed: %+v", twoBars)
	}
	if ledger.DayScope.Unpriced != 8 || len(ledger.Inflight) != 1 || ledger.Inflight[0] != "running" {
		t.Fatalf("uncertainty and inflight counts changed: day=%+v inflight=%v", ledger.DayScope, ledger.Inflight)
	}
	for _, id := range []string{"unavailable-one", "unavailable-two"} {
		found := false
		for _, entry := range ledger.Unmeasured {
			found = found || entry.ID == id
		}
		if !found {
			t.Fatalf("unavailable job %s disappeared from the ledger: %+v", id, ledger.Unmeasured)
		}
	}
}

func TestUnreadableJobRecordCannotDisappear(t *testing.T) {
	bed, ledger := measureBed(t)
	found := false
	for _, entry := range ledger.Unmeasured {
		if strings.HasSuffix(entry.File, "jobs/invalid.json") && entry.Provenance == "unreadable" && strings.Contains(entry.Detail, "invalid.json") {
			found = true
		}
	}
	if !found || ledger.DayScope.Unreadable != 1 {
		t.Fatalf("the malformed record was not disclosed: scope=%+v entries=%+v", ledger.DayScope, ledger.Unmeasured)
	}

	root := t.TempDir()
	copyFixtureFile(t, filepath.Join("testdata", "bed-20260902", "metasystem.conf"), filepath.Join(root, "metasystem.conf"), nil)
	if err := os.MkdirAll(filepath.Join(root, "artifacts", "agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	jobs := filepath.Join(root, "artifacts", "agents", "jobs")
	if err := os.WriteFile(jobs, []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", t.TempDir())
	if _, err := Measure(root, "bed-m1", bed.now); err == nil || !strings.Contains(err.Error(), jobs) {
		t.Fatalf("an unlistable jobs path must make measurement unknown and name the path: %v", err)
	}
}

func TestSeatTranscriptExcludesSharedCheckoutDelegateSession(t *testing.T) {
	_, ledger := measureBed(t)
	if !closeEnough(ledger.Seat.DayTokens, 118425925) || ledger.Seat.Files != 2 {
		t.Fatalf("the shared-checkout delegate transcript was counted as seat spend: %+v", ledger.Seat)
	}
	if !closeEnough(ledger.GoalScopes["dispatch-cap-necessity"].Tokens, 33922917) {
		t.Fatalf("the delegate job record itself was lost: %+v", ledger.GoalScopes["dispatch-cap-necessity"])
	}
}

func TestSeatTranscriptShapeFailureIsUnmeasured(t *testing.T) {
	_, ledger := measureBed(t)
	if ledger.Seat.UnmeasuredRequests != 2 {
		t.Fatalf("seat shape failures were not counted: %+v", ledger.Seat)
	}
	var missingUsage, invalidJSON bool
	for _, entry := range ledger.Unmeasured {
		if entry.Goal != "seat" {
			continue
		}
		missingUsage = missingUsage || strings.Contains(entry.Detail, "message.usage is not an object")
		invalidJSON = invalidJSON || strings.Contains(entry.Detail, "line is not JSON")
	}
	if !missingUsage || !invalidJSON {
		t.Fatalf("shape-failure reasons are incomplete: %+v", ledger.Unmeasured)
	}
}

func TestSeatGoalDoesNotSilentlyLoseAgedTranscriptSpend(t *testing.T) {
	_, ledger := measureBed(t)
	seatGoal := ledger.GoalScopes["seat"]
	if ledger.Seat.AgedFiles != 1 || !closeEnough(ledger.Seat.DayTokens, 118425925) || !closeEnough(ledger.Seat.LifetimeTokens, 118426225) || !closeEnough(seatGoal.Tokens, 118426225) {
		t.Fatalf("the aged transcript did not stay in the lifetime goal scope only: seat=%+v goal=%+v", ledger.Seat, seatGoal)
	}
}

func TestSeatSlugIsGitToplevelNotRepoRoot(t *testing.T) {
	toplevel := t.TempDir()
	if err := os.Mkdir(filepath.Join(toplevel, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	repoRoot := filepath.Join(toplevel, "metasystem")
	copyFixtureFile(t, filepath.Join("testdata", "bed-20260902", "metasystem.conf"), filepath.Join(repoRoot, "metasystem.conf"), nil)

	home := t.TempDir()
	t.Setenv("HOME", home)
	projects := filepath.Join(home, ".claude", "projects")
	toplevelSlug := strings.ReplaceAll(filepath.Clean(toplevel), string(filepath.Separator), "-")
	writeSeatTranscript(t, filepath.Join(projects, toplevelSlug, "toplevel.jsonl"), "seat-top", toplevel, "top-request", 10, 20)
	writeSeatTranscript(t, filepath.Join(projects, toplevelSlug+"-"+filepath.Base(repoRoot), "nested.jsonl"), "seat-nested", toplevel, "nested-request", 30, 40)
	writeSeatTranscript(t, filepath.Join(projects, "-other-project", "foreign.jsonl"), "seat-foreign", toplevel, "foreign-request", 1000, 2000)

	ledger, err := Measure(repoRoot, "bed-m1", bedNow)
	if err != nil {
		t.Fatal(err)
	}
	seatGoal := ledger.GoalScopes["seat"]
	if ledger.Seat.Files != 2 || ledger.Seat.UnreadableFiles != 0 ||
		!closeEnough(ledger.Seat.DayTokens, 100) ||
		!closeEnough(ledger.Seat.LifetimeTokens, 100) ||
		!closeEnough(seatGoal.Tokens, 100) {
		t.Fatalf("the Git-toplevel slug prefix did not include both matching directories or admitted a foreign slug: seat=%+v goal=%+v", ledger.Seat, seatGoal)
	}
}

func TestUnresolvableGitToplevelIsSeatUnreadable(t *testing.T) {
	repoRoot := t.TempDir()
	copyFixtureFile(t, filepath.Join("testdata", "bed-20260902", "metasystem.conf"), filepath.Join(repoRoot, "metasystem.conf"), nil)
	t.Setenv("HOME", t.TempDir())

	ledger, err := Measure(repoRoot, "bed-m1", bedNow)
	if err != nil {
		t.Fatalf("an unresolvable Git toplevel made measurement fail: %v", err)
	}
	found := false
	for _, entry := range ledger.Unmeasured {
		if entry.Goal == "seat" && entry.Provenance == "seat unreadable" &&
			strings.Contains(entry.Detail, "cannot resolve Git toplevel") &&
			strings.Contains(entry.Detail, "no .git file or directory found") {
			found = true
		}
	}
	if !found || ledger.Seat.UnreadableFiles != 1 {
		t.Fatalf("the unresolvable Git toplevel was not counted as one seat-unreadable entry: seat=%+v entries=%+v", ledger.Seat, ledger.Unmeasured)
	}
}

func TestSeatUnreadableTranscriptIsCountedNotSkipped(t *testing.T) {
	bed := newSpendBed(t)
	ledger, err := Measure(bed.root, "bed-m1", bed.now)
	if err != nil {
		t.Fatalf("an unreadable transcript path made measurement fail: %v", err)
	}
	found := false
	for _, entry := range ledger.Unmeasured {
		if strings.HasSuffix(entry.File, "unreadable-session.jsonl") &&
			entry.Provenance == "seat unreadable" && strings.Contains(entry.Detail, "unreadable-session.jsonl") &&
			strings.Contains(entry.Detail, "is a directory") {
			found = true
		}
	}
	seatGoal := ledger.GoalScopes["seat"]
	if !found || ledger.Seat.UnreadableFiles != 1 || ledger.Seat.Files != 2 ||
		!closeEnough(ledger.Seat.DayTokens, 118425925) ||
		!closeEnough(ledger.Seat.LifetimeTokens, 118426225) ||
		!closeEnough(seatGoal.Tokens, 118426225) {
		t.Fatalf("the unreadable transcript was skipped or changed readable spend: seat=%+v goal=%+v entries=%+v", ledger.Seat, seatGoal, ledger.Unmeasured)
	}
}

func TestDayIsUTCDateOfStartedAt(t *testing.T) {
	_, ledger := measureBed(t)
	found := false
	for _, row := range ledger.Rows {
		if row.Goal == "dispatch-cap-necessity" && row.Runtime == "claude" && row.Day == "2026-09-02" {
			found = true
		}
	}
	if !found {
		t.Fatalf("the record begun on September 1 in UTC-07 was not assigned to September 2 UTC: %+v", ledger.Rows)
	}
}

func TestNativeCostWinsOverPriceTable(t *testing.T) {
	settings := config.SpendSettings{Currency: "USD", Prices: map[config.SpendPriceKey]float64{
		{Runtime: "claude", Model: "model", Class: "input"}: 999,
	}}
	money, priced, unpriced, foreign := price("claude", "model", map[string]float64{"inputTokens": 1000000}, &mission.UsageCost{Currency: "USD", Amount: 4.25}, false, settings)
	if money != 4.25 || priced != 1 || unpriced != 0 || foreign != 0 {
		t.Fatalf("native target-currency cost did not win: money=%v priced=%d unpriced=%d foreign=%d", money, priced, unpriced, foreign)
	}
}

func TestUnpricedModelIsNeverZero(t *testing.T) {
	settings := config.SpendSettings{Currency: "USD", Prices: map[config.SpendPriceKey]float64{}}
	money, priced, unpriced, _ := price("codex", "unpriced-model", map[string]float64{"inputTokens": 0}, nil, false, settings)
	if money != 0 || priced != 0 || unpriced != 1 {
		t.Fatalf("a present zero-valued token class was mislabeled as zero cost: money=%v priced=%d unpriced=%d", money, priced, unpriced)
	}
}

func TestForeignCurrencyIsCountedBeside(t *testing.T) {
	settings := config.SpendSettings{Currency: "USD", Prices: map[config.SpendPriceKey]float64{
		{Runtime: "claude", Model: "model", Class: "input"}: 2,
	}}
	money, priced, unpriced, foreign := price("claude", "model", map[string]float64{"inputTokens": 1000000}, &mission.UsageCost{Currency: "EUR", Amount: 5}, false, settings)
	if money != 2 || priced != 1 || unpriced != 0 || foreign != 1 {
		t.Fatalf("foreign native cost was converted or hidden instead of counted beside derived cost: money=%v priced=%d unpriced=%d foreign=%d", money, priced, unpriced, foreign)
	}
}

func TestSeatTranscriptDedupesByRequestId(t *testing.T) {
	_, ledger := measureBed(t)
	if !closeEnough(ledger.Seat.DayTokens, 118425925) {
		t.Fatalf("both streamed snapshots were counted instead of last-wins request deduplication: %+v", ledger.Seat)
	}
}

func TestSeatTranscriptFiltersByCwd(t *testing.T) {
	_, ledger := measureBed(t)
	if !closeEnough(ledger.Seat.DayTokens, 118425925) {
		t.Fatalf("a worktree or foreign working directory entered seat spend: %+v", ledger.Seat)
	}
}

func TestSeatTranscriptSkipsForeignCheckoutAfterFirstCWDLine(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	copyFixtureFile(t, filepath.Join("testdata", "bed-20260902", "metasystem.conf"), filepath.Join(root, "metasystem.conf"), nil)
	home := t.TempDir()
	t.Setenv("HOME", home)
	slug := strings.ReplaceAll(filepath.Clean(root), string(filepath.Separator), "-")
	foreignRoot := filepath.Join(filepath.Dir(root), "another-checkout")
	foreignPath := filepath.Join(home, ".claude", "projects", slug+"-other-checkout", "foreign.jsonl")
	first := seatTranscriptLine(t, "foreign-session", foreignRoot, "foreign-request", 1000, 2000)
	content := append(append([]byte(nil), first...), bytes.Repeat([]byte{' '}, 5*1024*1024-len(first))...)
	if err := os.MkdirAll(filepath.Dir(foreignPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(foreignPath, content, 0o644); err != nil {
		t.Fatal(err)
	}
	prefixCollision := filepath.Join(home, ".claude", "projects", slug+"suffix", "collision.jsonl")
	writeSeatTranscript(t, prefixCollision, "collision", root, "collision-request", 5000, 5000)

	readBytes := 0
	priorObserver := transcriptBytesRead
	transcriptBytesRead = func(count int) { readBytes += count }
	cacheWrites := 0
	priorCacheWriter := spendCacheWriter
	spendCacheWriter = func(path, text string) error {
		cacheWrites++
		return priorCacheWriter(path, text)
	}
	t.Cleanup(func() {
		transcriptBytesRead = priorObserver
		spendCacheWriter = priorCacheWriter
	})
	ledger, err := Measure(root, "bed-m1", bedNow)
	if err != nil {
		t.Fatal(err)
	}
	if ledger.Seat.SkippedForeignFiles != 1 || ledger.Seat.Files != 0 || ledger.Seat.UnmeasuredRequests != 0 || ledger.Seat.LifetimeTokens != 0 {
		t.Fatalf("the foreign checkout transcript was not skipped as one file: %+v", ledger.Seat)
	}
	if readBytes == 0 || readBytes >= len(content) {
		t.Fatalf("foreign selection consumed %d of %d transcript bytes instead of stopping after the first cwd line", readBytes, len(content))
	}
	readBytes = 0
	cacheWrites = 0
	if _, err := Measure(root, "bed-m1", bedNow.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if readBytes != 0 || cacheWrites != 0 {
		t.Fatalf("the unchanged foreign transcript consumed %d bytes and made %d cache writes", readBytes, cacheWrites)
	}
	file, err := os.OpenFile(foreignPath, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write([]byte{' '}); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := Measure(root, "bed-m1", bedNow.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if readBytes != 0 || cacheWrites != 0 {
		t.Fatalf("the grown foreign transcript consumed %d bytes and made %d cache writes", readBytes, cacheWrites)
	}
}

func TestTranscriptCursorMatchesFullParseAcrossChanges(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	copyFixtureFile(t, filepath.Join("testdata", "bed-20260902", "metasystem.conf"), filepath.Join(root, "metasystem.conf"), nil)
	home := t.TempDir()
	t.Setenv("HOME", home)
	slug := strings.ReplaceAll(filepath.Clean(root), string(filepath.Separator), "-")
	path := filepath.Join(home, ".claude", "projects", slug, "seat.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	initial := append([]byte("not-json\n"), seatTranscriptLine(t, "seat", root, "request-a", 10, 1)...)
	if err := os.WriteFile(path, initial, 0o644); err != nil {
		t.Fatal(err)
	}

	call := 0
	assertMatchesFullParse := func(label string) Ledger {
		t.Helper()
		call++
		now := bedNow.Add(time.Duration(call) * time.Minute)
		ledger, err := Measure(root, "bed-m1", now)
		if err != nil {
			t.Fatalf("%s incremental measurement failed: %v", label, err)
		}
		incremental, err := os.ReadFile(Path(root, now))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(transcriptCursorPath(root, path)); err != nil {
			t.Fatal(err)
		}
		if _, err := Measure(root, "bed-m1", now); err != nil {
			t.Fatalf("%s full measurement failed: %v", label, err)
		}
		full, err := os.ReadFile(Path(root, now))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(incremental, full) {
			t.Fatalf("%s cursor ledger differs from a full parse\nincremental=%s\nfull=%s", label, incremental, full)
		}
		return ledger
	}

	ledger := assertMatchesFullParse("initial file")
	if ledger.Seat.LifetimeTokens != 11 || ledger.Seat.UnmeasuredRequests != 1 {
		t.Fatalf("the initial full semantics are wrong: %+v", ledger.Seat)
	}
	appendTranscript := func(line []byte) {
		t.Helper()
		file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.Write(line); err != nil {
			file.Close()
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
	}
	appendTranscript(seatTranscriptLine(t, "seat", root, "request-b", 20, 2))
	ledger = assertMatchesFullParse("appended request")
	if ledger.Seat.LifetimeTokens != 33 {
		t.Fatalf("the appended request was not merged: %+v", ledger.Seat)
	}
	appendTranscript(seatTranscriptLine(t, "seat", root, "request-a", 100, 10))
	ledger = assertMatchesFullParse("request identifier rewritten after cursor")
	if ledger.Seat.LifetimeTokens != 132 {
		t.Fatalf("the later request identifier did not replace the earlier request: %+v", ledger.Seat)
	}
	partial := bytes.TrimSuffix(seatTranscriptLine(t, "seat", root, "request-c", 30, 3), []byte{'\n'})
	split := len(partial) / 2
	appendTranscript(partial[:split])
	ledger = assertMatchesFullParse("partial trailing line")
	if ledger.Seat.LifetimeTokens != 132 || ledger.Seat.UnmeasuredRequests != 2 {
		t.Fatalf("the partial trailing line did not match full-parse invalid-line semantics: %+v", ledger.Seat)
	}
	appendTranscript(append(append([]byte(nil), partial[split:]...), '\n'))
	ledger = assertMatchesFullParse("partial trailing line completed after the cursor")
	if ledger.Seat.LifetimeTokens != 165 || ledger.Seat.UnmeasuredRequests != 1 {
		t.Fatalf("the completed trailing line did not replace its partial snapshot: %+v", ledger.Seat)
	}
	truncated := seatTranscriptLine(t, "seat", root, "request-b", 20, 2)
	if err := os.WriteFile(path, truncated, 0o644); err != nil {
		t.Fatal(err)
	}
	ledger = assertMatchesFullParse("truncated file")
	if ledger.Seat.LifetimeTokens != 22 || ledger.Seat.UnmeasuredRequests != 0 {
		t.Fatalf("the truncated file retained stale cursor entries: %+v", ledger.Seat)
	}
	if err := os.WriteFile(transcriptCursorPath(root, path), []byte("{corrupt"), 0o644); err != nil {
		t.Fatal(err)
	}
	ledger = assertMatchesFullParse("corrupt cursor cache")
	if ledger.Seat.LifetimeTokens != 22 {
		t.Fatalf("the corrupt cursor was trusted: %+v", ledger.Seat)
	}
	cacheBytes, err := os.ReadFile(transcriptCursorPath(root, path))
	if err != nil {
		t.Fatal(err)
	}
	var cache transcriptCursorCache
	if json.Unmarshal(cacheBytes, &cache) != nil || !validTranscriptCursor(cache, path) {
		t.Fatalf("the corrupt cursor cache was not replaced with valid state: %s", cacheBytes)
	}
}

func TestCacheWriteFailureKeepsTheFullMeasurement(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	copyFixtureFile(t, filepath.Join("testdata", "bed-20260902", "metasystem.conf"), filepath.Join(root, "metasystem.conf"), nil)
	jobs := filepath.Join(root, "artifacts", "agents", "jobs")
	if err := os.MkdirAll(jobs, 0o755); err != nil {
		t.Fatal(err)
	}
	record := `{"jobId":"terminal","goalId":"cache-failure","status":"completed","runtime":"fake","canonicalModelKey":"fixture-model","startedAt":"2026-09-02T10:00:00Z","usage":{"inputTokens":7}}`
	if err := os.WriteFile(filepath.Join(jobs, "terminal.json"), []byte(record), 0o644); err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	slug := strings.ReplaceAll(filepath.Clean(root), string(filepath.Separator), "-")
	writeSeatTranscript(t, filepath.Join(home, ".claude", "projects", slug, "seat.jsonl"), "seat", root, "request", 10, 2)

	cacheDirectory := spendCacheDir(root)
	if err := os.MkdirAll(filepath.Dir(cacheDirectory), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cacheDirectory, []byte("blocks cache directory creation"), 0o644); err != nil {
		t.Fatal(err)
	}
	blocked, err := Measure(root, "bed-m1", bedNow)
	if err != nil {
		t.Fatalf("cache publication failure stopped measurement: %v", err)
	}
	if blocked.Seat.CacheWriteFailures != 2 || blocked.Seat.LifetimeTokens != 12 {
		t.Fatalf("cache failures were not disclosed beside the complete spend: %+v", blocked.Seat)
	}
	if err := os.Remove(cacheDirectory); err != nil {
		t.Fatal(err)
	}
	writable, err := Measure(root, "bed-m1", bedNow)
	if err != nil {
		t.Fatal(err)
	}
	blocked.Seat.CacheWriteFailures = 0
	blockedBytes, err := json.Marshal(blocked)
	if err != nil {
		t.Fatal(err)
	}
	writableBytes, err := json.Marshal(writable)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(blockedBytes, writableBytes) {
		t.Fatalf("cache publication failure changed the full measurement\nblocked=%s\nwritable=%s", blockedBytes, writableBytes)
	}
}

func TestPendingTerminalMeasurementIsReadAgainWhenItSettles(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	copyFixtureFile(t, filepath.Join("testdata", "bed-20260902", "metasystem.conf"), filepath.Join(root, "metasystem.conf"), nil)
	jobs := filepath.Join(root, "artifacts", "agents", "jobs")
	if err := os.MkdirAll(jobs, 0o755); err != nil {
		t.Fatal(err)
	}
	recordPath := filepath.Join(jobs, "settling.json")
	if err := os.WriteFile(recordPath, []byte(`{"jobId":"settling","status":"completed"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", t.TempDir())

	reads := 0
	priorJobUsageAt := jobUsageAt
	jobUsageAt = func(string, string) mission.JobMeasurement {
		reads++
		record := map[string]any{
			"jobId": "settling", "goalId": "settling-goal", "status": "completed",
			"runtime": "fake", "canonicalModelKey": "fixture-model", "startedAt": "2026-09-02T10:00:00Z",
		}
		if reads == 1 {
			return mission.JobMeasurement{Record: record, Tokens: map[string]float64{}, Provenance: "pending", Detail: "process group is still alive"}
		}
		return mission.JobMeasurement{Record: record, Tokens: map[string]float64{"inputTokens": 7}, Provenance: "derived"}
	}
	t.Cleanup(func() { jobUsageAt = priorJobUsageAt })
	first, err := Measure(root, "bed-m1", bedNow)
	if err != nil {
		t.Fatal(err)
	}
	if reads != 1 || first.GoalScopes["settling-goal"].Tokens != 0 || first.GoalScopes["settling-goal"].Unmeasured != 1 {
		t.Fatalf("the pending first outcome was not measured as pending: reads=%d scope=%+v", reads, first.GoalScopes["settling-goal"])
	}
	second, err := Measure(root, "bed-m1", bedNow.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if reads != 2 || second.GoalScopes["settling-goal"].Tokens != 7 || second.GoalScopes["settling-goal"].Unmeasured != 0 {
		t.Fatalf("the settled outcome was not re-read and counted: reads=%d scope=%+v", reads, second.GoalScopes["settling-goal"])
	}
	if _, err := Measure(root, "bed-m1", bedNow.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if reads != 2 {
		t.Fatalf("the settled terminal outcome was not cached: reads=%d", reads)
	}
}

func TestDeletedTranscriptCursorIsPruned(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	copyFixtureFile(t, filepath.Join("testdata", "bed-20260902", "metasystem.conf"), filepath.Join(root, "metasystem.conf"), nil)
	home := t.TempDir()
	t.Setenv("HOME", home)
	slug := strings.ReplaceAll(filepath.Clean(root), string(filepath.Separator), "-")
	transcript := filepath.Join(home, ".claude", "projects", slug, "deleted.jsonl")
	writeSeatTranscript(t, transcript, "seat", root, "request", 10, 2)
	if _, err := Measure(root, "bed-m1", bedNow); err != nil {
		t.Fatal(err)
	}
	cursor := transcriptCursorPath(root, transcript)
	if _, err := os.Stat(cursor); err != nil {
		t.Fatalf("the initial measurement did not create its cursor: %v", err)
	}
	if err := os.Remove(transcript); err != nil {
		t.Fatal(err)
	}
	if _, err := Measure(root, "bed-m1", bedNow.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(cursor); !os.IsNotExist(err) {
		t.Fatalf("the cursor for a deleted transcript was not pruned: %v", err)
	}
}

func TestTranscriptCursorPreservesDelegateFilteringBeforeRequestReplacement(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	copyFixtureFile(t, filepath.Join("testdata", "bed-20260902", "metasystem.conf"), filepath.Join(root, "metasystem.conf"), nil)
	home := t.TempDir()
	t.Setenv("HOME", home)
	slug := strings.ReplaceAll(filepath.Clean(root), string(filepath.Separator), "-")
	path := filepath.Join(home, ".claude", "projects", slug, "seat.jsonl")
	content := append(seatTranscriptLine(t, "seat-session", root, "same-request", 10, 1),
		seatTranscriptLine(t, "delegate-session", root, "same-request", 100, 100)...)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
	first, err := Measure(root, "bed-m1", bedNow)
	if err != nil {
		t.Fatal(err)
	}
	if first.Seat.LifetimeTokens != 200 {
		t.Fatalf("the later request did not initially replace the earlier request: %+v", first.Seat)
	}
	jobs := filepath.Join(root, "artifacts", "agents", "jobs")
	if err := os.MkdirAll(jobs, 0o755); err != nil {
		t.Fatal(err)
	}
	record := `{"jobId":"delegate","status":"completed","runtime":"fake","canonicalModelKey":"fixture-model","startedAt":"2026-09-02T10:00:00Z","sessionId":"delegate-session","usage":{"inputTokens":1}}`
	if err := os.WriteFile(filepath.Join(jobs, "delegate.json"), []byte(record), 0o644); err != nil {
		t.Fatal(err)
	}
	second, err := Measure(root, "bed-m1", bedNow.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if second.Seat.LifetimeTokens != 11 {
		t.Fatalf("delegate filtering happened after replacement instead of before it: %+v", second.Seat)
	}
}

func TestWarmMeasureReadsNoTranscriptBytesOrTerminalJobRecord(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	copyFixtureFile(t, filepath.Join("testdata", "bed-20260902", "metasystem.conf"), filepath.Join(root, "metasystem.conf"), nil)
	jobs := filepath.Join(root, "artifacts", "agents", "jobs")
	if err := os.MkdirAll(jobs, 0o755); err != nil {
		t.Fatal(err)
	}
	terminalPath := filepath.Join(jobs, "terminal.json")
	runningPath := filepath.Join(jobs, "running.json")
	job := func(id, status string) []byte {
		return []byte(fmt.Sprintf(`{"jobId":%q,"status":%q,"runtime":"fake","canonicalModelKey":"fixture-model","startedAt":"2026-09-02T10:00:00Z","usage":{"inputTokens":1}}`, id, status))
	}
	if err := os.WriteFile(terminalPath, job("terminal", "completed"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(runningPath, job("running", "running"), 0o644); err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	slug := strings.ReplaceAll(filepath.Clean(root), string(filepath.Separator), "-")
	transcriptPath := filepath.Join(home, ".claude", "projects", slug, "seat.jsonl")
	writeSeatTranscript(t, transcriptPath, "seat", root, "request", 10, 2)

	jobReads := map[string]int{}
	priorJobUsageAt := jobUsageAt
	jobUsageAt = func(repoRoot, recordPath string) mission.JobMeasurement {
		jobReads[recordPath]++
		return priorJobUsageAt(repoRoot, recordPath)
	}
	readBytes := 0
	priorObserver := transcriptBytesRead
	transcriptBytesRead = func(count int) { readBytes += count }
	t.Cleanup(func() {
		jobUsageAt = priorJobUsageAt
		transcriptBytesRead = priorObserver
	})
	if _, err := Measure(root, "bed-m1", bedNow); err != nil {
		t.Fatal(err)
	}
	if jobReads[terminalPath] != 1 || jobReads[runningPath] != 1 || readBytes == 0 {
		t.Fatalf("the cold measurement did not exercise both sources: jobs=%v transcriptBytes=%d", jobReads, readBytes)
	}
	jobReads = map[string]int{}
	readBytes = 0
	if _, err := Measure(root, "bed-m1", bedNow.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if jobReads[terminalPath] != 0 || jobReads[runningPath] != 1 || readBytes != 0 {
		t.Fatalf("the warm measurement reread cached input: jobs=%v transcriptBytes=%d", jobReads, readBytes)
	}
	if err := os.WriteFile(terminalJobCachePath(root), []byte("not-json"), 0o644); err != nil {
		t.Fatal(err)
	}
	jobReads = map[string]int{}
	if _, err := Measure(root, "bed-m1", bedNow.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if jobReads[terminalPath] != 1 || jobReads[runningPath] != 1 {
		t.Fatalf("a corrupt terminal-job cache was trusted: jobs=%v", jobReads)
	}
	cacheBytes, err := os.ReadFile(terminalJobCachePath(root))
	if err != nil {
		t.Fatal(err)
	}
	var cache terminalJobMeasurementCache
	if json.Unmarshal(cacheBytes, &cache) != nil || !validTerminalJobCache(cache) {
		t.Fatalf("the corrupt terminal-job cache was not rewritten: %s", cacheBytes)
	}
}

func TestSeatCodexRuntimeIsUnmeasured(t *testing.T) {
	_, ledger := measureBed(t)
	if !ledger.Seat.CodexUnmeasured {
		t.Fatalf("the absent Codex seat meter was presented as measured: %+v", ledger.Seat)
	}
}

func TestLedgerSkipsContentEqualRewrite(t *testing.T) {
	bed, first := measureBed(t)
	path := Path(bed.root, bed.now)
	old := time.Date(2001, 2, 3, 4, 5, 6, 0, time.UTC)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	second, err := Measure(bed.root, "bed-m1", bed.now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !info.ModTime().Equal(old) || !second.ObservedAt.Equal(first.ObservedAt) {
		t.Fatalf("content-equal measurement rewrote the ledger: mtime=%s first=%s second=%s", info.ModTime(), first.ObservedAt, second.ObservedAt)
	}
}

func TestAdmissionNeverConsultsSpend(t *testing.T) {
	moduleRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command("go", "list", "-deps", "./internal/dispatch", "./internal/goal", "./internal/goalbudget")
	command.Dir = moduleRoot
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("dependency proof did not run: %v\n%s", err, output)
	}
	for _, dependency := range strings.Fields(string(output)) {
		if strings.HasSuffix(dependency, "/internal/spend") {
			t.Fatalf("an admission package imports spend: %s", dependency)
		}
	}
}
