package spend

import (
	"encoding/json"
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
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(line, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
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
