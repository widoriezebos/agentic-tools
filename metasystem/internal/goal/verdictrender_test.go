package goal

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTurnVerdictBoundsRunHistoryAndWritesTheFullDisplay(t *testing.T) {
	store := testStore(t)
	runs := make([]RunFact, 0, 200)
	for i := 0; i < 200; i++ {
		id := fmt.Sprintf("stale-run-%03d", i)
		switch {
		case i < 70:
			runs = append(runs, RunFact{Id: id, Hung: true, ExpectHung: "inspect its log"})
		case i < 135:
			runs = append(runs, RunFact{Id: id, Status: "ended-unknown", ExpectUnknown: "inspect its terminal record"})
		default:
			runs = append(runs, RunFact{Id: id, Status: "green", TerminalSeq: int64(i + 1)})
		}
	}
	scan := ScanResult{
		Busy: []Item{{Kind: "job", Id: "unwatched-job", Detail: "unwatched-job [running, codex]"}},
		Jobs: []JobFact{{Id: "unwatched-job", MainId: "main-1", StartedAt: "2026-08-01T10:00:00Z", Status: "running"}},
		Runs: runs,
	}
	verdict, err := store.TurnVerdict(scan, "bounded-session", "", "main-1")
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(verdict.Display, "\n")
	if !strings.Contains(lines[0], "work you launched is unwatched") {
		t.Fatalf("the verdict is not first: %q", lines[0])
	}
	watchedAt := strings.Index(verdict.Display, "work you launched is unwatched")
	hungAt := strings.Index(verdict.Display, "70 runs look hung, oldest stale-run-000")
	if watchedAt < 0 || hungAt < 0 || watchedAt > hungAt {
		t.Fatalf("the actionable line does not precede the run summary: %s", verdict.Display)
	}
	if strings.Count(verdict.Display, "runs look hung") != 1 || strings.Count(verdict.Display, "runs ended ended-unknown") != 1 {
		t.Fatalf("run classes were not summarized exactly once: %s", verdict.Display)
	}
	if len([]rune(verdict.Display)) > TurnVerdictDisplayRuneLimit {
		t.Fatalf("display has %d runes, limit is %d", len([]rune(verdict.Display)), TurnVerdictDisplayRuneLimit)
	}
	artifactPath := turnVerdictArtifactPath(store.Root, "bounded-session")
	full, err := os.ReadFile(artifactPath)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 200; i++ {
		id := fmt.Sprintf("stale-run-%03d", i)
		if !strings.Contains(string(full), id) {
			t.Fatalf("full display is missing %s", id)
		}
	}
	if !strings.HasPrefix(string(full), "run stale-run-000 looks hung;") || strings.Contains(string(full), "Full turn verdict:") {
		t.Fatalf("artifact is not the unbounded legacy display: %s", full)
	}
	if !strings.Contains(verdict.Display, artifactPath) {
		t.Fatalf("bounded display does not name %s: %s", artifactPath, verdict.Display)
	}
}

func TestTurnVerdictPrintsTheFirstThreeOfFourGreenContinuations(t *testing.T) {
	store := testStore(t)
	runs := []RunFact{
		{Id: "green-a", Status: "green", TerminalSeq: 1, ExpectGreen: "continue alpha"},
		{Id: "green-b", Status: "green", TerminalSeq: 2, ExpectGreen: "continue beta"},
		{Id: "green-c", Status: "green", TerminalSeq: 3, ExpectGreen: "continue gamma"},
		{Id: "green-d", Status: "green", TerminalSeq: 4, ExpectGreen: "continue delta"},
	}
	verdict, err := store.TurnVerdict(ScanResult{Runs: runs}, "green-session", "", "main-1")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"0 runs finished green without a recorded continuation; 4 with one; 1 more in the full turn verdict file", "continue alpha", "continue beta", "continue gamma"} {
		if !strings.Contains(verdict.Display, want) {
			t.Fatalf("display is missing %q: %s", want, verdict.Display)
		}
	}
	if strings.Contains(verdict.Display, "continue delta") {
		t.Fatalf("the fourth continuation should remain in the full verdict file: %s", verdict.Display)
	}
}

func TestTurnVerdictPrintsOneHungRunInFull(t *testing.T) {
	store := testStore(t)
	verdict, err := store.TurnVerdict(ScanResult{Runs: []RunFact{{Id: "hung-one", Hung: true, ExpectHung: "inspect it"}}}, "hung-session", "", "main-1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(verdict.Display, "run hung-one looks hung; the run record says: inspect it") {
		t.Fatalf("single hung run was not printed in full: %s", verdict.Display)
	}
	if strings.Contains(verdict.Display, "1 runs look hung") {
		t.Fatalf("single hung run was summarized: %s", verdict.Display)
	}
}

func TestTurnVerdictTrimsActionableItemsFromTheBottom(t *testing.T) {
	var actionable []string
	for i := 0; i < 40; i++ {
		actionable = append(actionable, fmt.Sprintf("OPEN-WORK item-%02d: %s", i, strings.Repeat("work", 40)))
	}
	fileLine := "Full turn verdict: /checkout/artifacts/agents/supervision/stop-verdicts/session.txt"
	display := boundedVerdictLines("OPEN WORK (40)", actionable, []string{"40 runs went red, oldest old-run"}, fileLine)
	if len([]rune(display)) > TurnVerdictDisplayRuneLimit || !strings.HasPrefix(display, "OPEN WORK (40)\n") || !strings.HasSuffix(display, fileLine) {
		t.Fatalf("mandatory verdict or file line was lost: %s", display)
	}
	if !strings.Contains(display, "actionable items trimmed") || strings.Contains(display, "OPEN-WORK item-39:") {
		t.Fatalf("actions were not trimmed from the bottom with a notice: %s", display)
	}
}

func TestRunWarningSummariesCoverEveryClass(t *testing.T) {
	classes := []runWarningClass{runLooksHung, runEndedUnknown, runWentRed, runLivenessUnknown, runUnsupervised}
	var warnings []runWarning
	for _, class := range classes {
		warnings = append(warnings,
			runWarning{class: class, id: fmt.Sprintf("old-%d", class), line: "old detail"},
			runWarning{class: class, id: fmt.Sprintf("new-%d", class), line: "new detail"})
	}
	lines := summarizeRunWarnings(runDisplayLines{warnings: warnings, unreadable: []string{"first unreadable", "second unreadable"}})
	for _, want := range []string{"runs look hung", "runs ended ended-unknown", "runs went red", "runs of unknown liveness", "runs unsupervised", "2 run records unreadable"} {
		if strings.Count(strings.Join(lines, "\n"), want) != 1 {
			t.Fatalf("summary is missing one %q class: %v", want, lines)
		}
	}
}

func TestTurnVerdictReportsArtifactWriteFailureWithoutChangingTheDecision(t *testing.T) {
	store := testStore(t)
	blockedDirectory := filepath.Join(store.Root, "artifacts", "agents", "supervision", "stop-verdicts")
	if err := os.MkdirAll(filepath.Dir(blockedDirectory), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(blockedDirectory, []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	verdict, err := store.TurnVerdict(ScanResult{Busy: []Item{{Kind: "job", Id: "live", Detail: "live job"}}}, "write-failure", "", "main-1")
	if err != nil {
		t.Fatal(err)
	}
	if verdict.ShouldBlock || !strings.Contains(verdict.Display, "Full turn verdict could not be written to") || len(verdict.Diagnostics) == 0 || !strings.Contains(verdict.Diagnostics[len(verdict.Diagnostics)-1], "could not be written") {
		t.Fatalf("artifact failure changed the verdict or stayed hidden: %+v", verdict)
	}
}
