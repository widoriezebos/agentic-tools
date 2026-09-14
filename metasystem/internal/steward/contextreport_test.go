package steward

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/usage"
)

func TestContextReportComputesTheWeek(t *testing.T) {
	t.Run("baseline and committed boundary", func(t *testing.T) {
		fixture := seedContextReportBaseline(t)
		callsPath, reportPath, report, err := WriteContextReport(fixture.root, fixture.weekStart, fixture.now)
		if err != nil {
			t.Fatal(err)
		}
		assertContextReportBaseline(t, report)
		callsBefore := reportTestRead(t, callsPath)
		reportBefore := reportTestRead(t, reportPath)
		if reportTestLines(t, callsPath) != 25 || !bytes.Contains(reportBefore, []byte(contextReportCoverage)) {
			t.Fatalf("published report does not carry the 25-call cohort or coverage statement")
		}

		samplesPath := usage.SamplesPath(fixture.root, "claude", "claude-main")
		committedSize := reportTestSize(t, samplesPath)
		completeSuffix := fmt.Sprintf(`{"kind":"sample","runtime":"claude","session":"claude-main","invocationId":"uncommitted","promptTokens":200001,"inputTokens":200001,"cacheCreation":0,"cacheRead":0,"at":%q,"ordinal":999,"source":"claude-transcript"}`+"\n"+
			`{"kind":"marker","runtime":"claude","session":"claude-main","at":%q,"ordinal":1000,"detail":"uncommitted"}`+"\n",
			fixture.weekStart.Add(2*time.Hour).Format(time.RFC3339Nano), fixture.weekStart.Add(2*time.Hour).Format(time.RFC3339Nano))
		reportTestAppend(t, samplesPath, completeSuffix)
		_, _, complete, err := WriteContextReport(fixture.root, fixture.weekStart, fixture.now)
		if err != nil {
			t.Fatal(err)
		}
		assertContextReportBaseline(t, complete)
		if reportTestSize(t, samplesPath) != committedSize {
			t.Fatal("complete uncommitted suffix survived Calls recovery")
		}
		reportTestAppend(t, samplesPath, `{"kind":"sample","runtime":"claude"`)
		_, _, partial, err := WriteContextReport(fixture.root, fixture.weekStart, fixture.now)
		if err != nil {
			t.Fatal(err)
		}
		assertContextReportBaseline(t, partial)
		if !bytes.Equal(reportTestRead(t, callsPath), callsBefore) || !bytes.Equal(reportTestRead(t, reportPath), reportBefore) {
			t.Fatal("an uncommitted suffix changed the published report pair")
		}
	})

	for _, scenario := range []string{"reset", "compaction", "reference mismatch", "over ceiling"} {
		t.Run(scenario, func(t *testing.T) {
			fixture := seedContextReportBaseline(t)
			switch scenario {
			case "reset":
				seedContextReportSession(t, fixture.root, "claude", "claude-reset", reportClaudeCall("reset", 110000, fixture.weekStart.Add(4*time.Hour)))
				if err := usage.RegisterSession(fixture.root, "claude", "claude-reset", 101, 1001); err != nil {
					t.Fatal(err)
				}
			case "compaction":
				reportTestAppend(t, fixture.transcripts["claude/claude-main"], reportClaudeMarker(fixture.weekStart.Add(5*time.Hour)))
				seedContextReportRead(t, fixture.root, "claude", "claude-main", fixture.transcripts["claude/claude-main"])
			case "reference mismatch":
				writeContextReportJob(t, fixture.root, "reference-bad", "failed", "reference_mismatch: digest", "", fixture.weekStart.Add(6*time.Hour))
			case "over ceiling":
				reportTestAppend(t, fixture.transcripts["claude/claude-main"], reportClaudeCall("too-large", 200001, fixture.weekStart.Add(6*time.Hour)))
				seedContextReportRead(t, fixture.root, "claude", "claude-main", fixture.transcripts["claude/claude-main"])
			}
			_, _, report, err := WriteContextReport(fixture.root, fixture.weekStart, fixture.now)
			if err != nil {
				t.Fatal(err)
			}
			if report.Pass {
				t.Fatalf("%s evidence produced PASS: %+v", scenario, report)
			}
			switch scenario {
			case "reset":
				if report.Resets != 1 {
					t.Fatalf("reset count = %d, want 1", report.Resets)
				}
			case "compaction":
				if report.Compactions["claude"] != 1 {
					t.Fatalf("compactions = %+v", report.Compactions)
				}
			case "reference mismatch":
				if !reflect.DeepEqual(report.ReferenceMismatches, []string{"reference-bad"}) {
					t.Fatalf("reference mismatches = %v", report.ReferenceMismatches)
				}
			case "over ceiling":
				if report.Max != 200001 {
					t.Fatalf("maximum = %d", report.Max)
				}
			}
		})
	}
}

func TestContextReportUsesOneEvidenceSnapshot(t *testing.T) {
	weekStart, now := contextReportTestWeek()
	stateRoot := t.TempDir()
	evidence := usage.CallEvidence{
		Sessions: []usage.CallSession{{Runtime: "claude", Session: "snapshot"}},
		Registrations: []usage.CallRegistration{{
			Runtime: "claude", Session: "snapshot", PID: 101, PIDStartedAt: 1001,
			FirstSeen: weekStart.Add(time.Minute),
		}},
		Samples: []usage.CallSample{{
			Runtime: "claude", Session: "snapshot", InvocationID: "only",
			PromptTokens: 100000, InputTokens: 100000, At: weekStart.Add(time.Hour),
			Ordinal: 1, Source: "claude-transcript",
		}},
	}

	previous := readContextCallEvidence
	reads := 0
	readContextCallEvidence = func(root string) (usage.CallEvidence, error) {
		reads++
		if root != stateRoot {
			t.Fatalf("snapshot root = %s, want %s", root, stateRoot)
		}
		return evidence, nil
	}
	t.Cleanup(func() { readContextCallEvidence = previous })

	_, _, report, err := WriteContextReport(stateRoot, weekStart, now)
	if err != nil {
		t.Fatal(err)
	}
	if reads != 1 || !report.Pass || report.Samples != 1 || report.Max != 100000 {
		t.Fatalf("snapshot reads=%d report=%+v", reads, report)
	}
}

func TestContextPrunePreservesRetainedWeek(t *testing.T) {
	for _, scenario := range []string{"passing", "failing", "empty"} {
		t.Run(scenario, func(t *testing.T) {
			weekStart, now := contextReportRetirableWeek()
			root := t.TempDir()
			if scenario != "empty" {
				fixture := seedContextReportBaselineAt(t, weekStart, now)
				root, weekStart, now = fixture.root, fixture.weekStart, fixture.now
				if scenario == "failing" {
					reportTestAppend(t, fixture.transcripts["claude/claude-main"], reportClaudeMarker(weekStart.Add(time.Hour)))
					seedContextReportRead(t, root, "claude", "claude-main", fixture.transcripts["claude/claude-main"])
				}
			}
			oldAt := weekStart.Add(-14 * 24 * time.Hour)
			seedContextReportSession(t, root, "claude", "retired-old", reportClaudeCall("retired-old", 90000, oldAt))
			appendContextReportRegistration(t, root, usage.CallRegistration{
				Runtime: "claude", Session: "retired-old", PID: 909, PIDStartedAt: 9009, FirstSeen: oldAt,
			})
			appendContextReportRegistration(t, root, usage.CallRegistration{
				Runtime: "claude", Session: "registry-only", PID: 910, PIDStartedAt: 9010, FirstSeen: oldAt,
			})
			setContextReportPairTimes(t, root, "claude", "retired-old", oldAt)

			callsPath, reportPath, before, err := WriteContextReport(root, weekStart, now)
			if err != nil {
				t.Fatal(err)
			}
			callsBytes, reportBytes := reportTestRead(t, callsPath), reportTestRead(t, reportPath)
			removed, err := usage.PruneCallSessions(root, weekStart)
			if err != nil || removed != 1 {
				t.Fatalf("prune removed=%d err=%v", removed, err)
			}
			_, _, after, err := WriteContextReport(root, weekStart, now)
			if err != nil || !reflect.DeepEqual(after, before) {
				t.Fatalf("retained report before=%+v after=%+v err=%v", before, after, err)
			}
			if !bytes.Equal(reportTestRead(t, callsPath), callsBytes) || !bytes.Equal(reportTestRead(t, reportPath), reportBytes) {
				t.Fatal("retained report bytes changed after pruning out-of-window evidence")
			}
		})
	}
}

func TestContextPrunePreservesResetPrefix(t *testing.T) {
	weekStart, now := contextReportRetirableWeek()
	root := t.TempDir()
	oldAt := weekStart.Add(-14 * 24 * time.Hour)
	seedContextReportSession(t, root, "claude", "predecessor", reportClaudeCall("predecessor", 90000, oldAt))
	seedContextReportSession(t, root, "claude", "current", reportClaudeCall("current", 100000, weekStart.Add(time.Hour)))
	appendContextReportRegistration(t, root, usage.CallRegistration{Runtime: "claude", Session: "predecessor", PID: 77, PIDStartedAt: 777, FirstSeen: oldAt})
	appendContextReportRegistration(t, root, usage.CallRegistration{Runtime: "claude", Session: "current", PID: 77, PIDStartedAt: 777, FirstSeen: weekStart.Add(time.Minute)})
	setContextReportPairTimes(t, root, "claude", "predecessor", oldAt)
	_, _, before, err := WriteContextReport(root, weekStart, now)
	if err != nil || before.Resets != 1 {
		t.Fatalf("report before prune=%+v err=%v", before, err)
	}
	if removed, err := usage.PruneCallSessions(root, weekStart); err != nil || removed != 1 {
		t.Fatalf("prune removed=%d err=%v", removed, err)
	}
	_, _, after, err := WriteContextReport(root, weekStart, now)
	if err != nil || after.Resets != 1 {
		t.Fatalf("report after prune=%+v err=%v", after, err)
	}
}

func TestContextReportSerializesWithPrune(t *testing.T) {
	fixture := seedHistoricalContextReportBaseline(t)
	for _, session := range []struct{ runtime, name string }{{"claude", "claude-main"}, {"codex", "codex-main"}} {
		setContextReportPairTimes(t, fixture.root, session.runtime, session.name, fixture.weekStart.Add(-time.Hour))
	}
	snapshotReady := make(chan struct{})
	releaseSnapshot := make(chan struct{})
	originalRead := readContextCallEvidence
	readContextCallEvidence = func(root string) (usage.CallEvidence, error) {
		evidence, err := originalRead(root)
		close(snapshotReady)
		<-releaseSnapshot
		return evidence, err
	}
	t.Cleanup(func() { readContextCallEvidence = originalRead })
	type reportResult struct {
		report ContextReport
		err    error
	}
	reportDone := make(chan reportResult, 1)
	go func() {
		_, _, report, err := WriteContextReport(fixture.root, fixture.weekStart, fixture.now)
		reportDone <- reportResult{report: report, err: err}
	}()
	select {
	case <-snapshotReady:
	case <-time.After(5 * time.Second):
		close(releaseSnapshot)
		t.Fatal("report did not finish its evidence snapshot")
	}
	if removed, err := usage.PruneCallSessions(fixture.root, fixture.weekStart.AddDate(0, 0, 7)); err != nil || removed != 2 {
		close(releaseSnapshot)
		t.Fatalf("concurrent prune removed=%d err=%v", removed, err)
	}
	close(releaseSnapshot)
	result := <-reportDone
	if result.err != nil || result.report.Samples != 25 || !result.report.Pass {
		t.Fatalf("snapshotted report=%+v err=%v", result.report, result.err)
	}
}

func TestContextReportRefusesRetiredEvidenceWithoutPublishing(t *testing.T) {
	fixture := seedHistoricalContextReportBaseline(t)
	callsPath, reportPath, _, err := WriteContextReport(fixture.root, fixture.weekStart, fixture.now)
	if err != nil {
		t.Fatal(err)
	}
	callsBefore, reportBefore := reportTestRead(t, callsPath), reportTestRead(t, reportPath)
	for _, session := range []struct{ runtime, name string }{{"claude", "claude-main"}, {"codex", "codex-main"}} {
		setContextReportPairTimes(t, fixture.root, session.runtime, session.name, fixture.weekStart.Add(-time.Hour))
	}
	retainedSince := fixture.weekStart.AddDate(0, 0, 7)
	if removed, err := usage.PruneCallSessions(fixture.root, retainedSince); err != nil || removed != 2 {
		t.Fatalf("prune removed=%d err=%v", removed, err)
	}
	_, _, _, err = WriteContextReport(fixture.root, fixture.weekStart, fixture.now)
	var retired *ContextEvidenceRetiredError
	if !errors.As(err, &retired) || !retired.WeekStart.Equal(fixture.weekStart) || !retired.RetainedSince.Equal(retainedSince) {
		t.Fatalf("retired evidence error=%v", err)
	}
	if !bytes.Equal(reportTestRead(t, callsPath), callsBefore) || !bytes.Equal(reportTestRead(t, reportPath), reportBefore) {
		t.Fatal("retired report request changed previously published evidence")
	}
}

func TestContextPrunePreservesUnassignableEvidence(t *testing.T) {
	weekStart, _ := contextReportRetirableWeek()
	root := t.TempDir()
	oldAt := weekStart.Add(-14 * 24 * time.Hour)
	seedContextReportSession(t, root, "claude", "undated", reportClaudeCallWithTimestamp("undated", 10, ""))
	seedContextReportSession(t, root, "claude", "unknown-source", reportClaudeCall("unknown-source", 10, oldAt))
	samplesPath := usage.SamplesPath(root, "claude", "unknown-source")
	data := bytes.ReplaceAll(reportTestRead(t, samplesPath), []byte("claude-transcript"), []byte("bogus--transcript"))
	if err := os.WriteFile(samplesPath, data, 0o644); err != nil {
		t.Fatal(err)
	}
	seedContextReportSession(t, root, "claude", "unregistered", reportClaudeCall("unregistered", 10, oldAt))
	seedContextReportSession(t, root, "claude", "conflict", reportClaudeCall("same-call", 10, oldAt))
	secondConflict := filepath.Join(t.TempDir(), "conflict-copy.jsonl")
	reportTestWrite(t, secondConflict, reportClaudeCall("same-call", 11, oldAt))
	seedContextReportRead(t, root, "claude", "conflict", secondConflict)
	for _, session := range []string{"undated", "unknown-source", "conflict"} {
		appendContextReportRegistration(t, root, usage.CallRegistration{Runtime: "claude", Session: session, PID: 50, PIDStartedAt: 500, FirstSeen: oldAt})
	}
	for _, session := range []string{"undated", "unknown-source", "unregistered", "conflict"} {
		setContextReportPairTimes(t, root, "claude", session, oldAt)
	}
	if removed, err := usage.PruneCallSessions(root, weekStart); err != nil || removed != 0 {
		t.Fatalf("unassignable prune removed=%d err=%v", removed, err)
	}
	for _, session := range []string{"undated", "unknown-source", "unregistered", "conflict"} {
		if _, err := os.Stat(usage.CursorPath(root, "claude", session)); err != nil {
			t.Fatalf("unassignable cursor %s was removed: %v", session, err)
		}
	}
}

func TestContextReportPropagatesRecoveryError(t *testing.T) {
	for _, scenario := range []string{"missing cursor", "malformed cursor", "missing samples", "short samples"} {
		t.Run(scenario, func(t *testing.T) {
			fixture := seedContextReportBaseline(t)
			callsPath, reportPath, _, err := WriteContextReport(fixture.root, fixture.weekStart, fixture.now)
			if err != nil {
				t.Fatal(err)
			}
			callsBefore := reportTestRead(t, callsPath)
			reportBefore := reportTestRead(t, reportPath)
			cursorPath := usage.CursorPath(fixture.root, "claude", "claude-main")
			samplesPath := usage.SamplesPath(fixture.root, "claude", "claude-main")
			var evidencePath string
			var evidence []byte
			switch scenario {
			case "missing cursor":
				if err := os.Remove(cursorPath); err != nil {
					t.Fatal(err)
				}
				evidencePath, evidence = samplesPath, reportTestRead(t, samplesPath)
			case "malformed cursor":
				evidencePath, evidence = cursorPath, []byte("{malformed\n")
				if err := os.WriteFile(cursorPath, evidence, 0o644); err != nil {
					t.Fatal(err)
				}
			case "missing samples":
				if err := os.Remove(samplesPath); err != nil {
					t.Fatal(err)
				}
			case "short samples":
				original := reportTestRead(t, samplesPath)
				evidencePath, evidence = samplesPath, append([]byte(nil), original[:len(original)-1]...)
				if err := os.WriteFile(samplesPath, evidence, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			_, _, _, err = WriteContextReport(fixture.root, fixture.weekStart, fixture.now)
			if err == nil || !strings.Contains(err.Error(), cursorPath) || !strings.Contains(err.Error(), samplesPath) {
				t.Fatalf("recovery error = %v", err)
			}
			if !bytes.Equal(reportTestRead(t, callsPath), callsBefore) || !bytes.Equal(reportTestRead(t, reportPath), reportBefore) {
				t.Fatal("a recovery error changed the previous report pair")
			}
			if evidencePath != "" && !bytes.Equal(reportTestRead(t, evidencePath), evidence) {
				t.Fatalf("recovery error changed damaged evidence at %s", evidencePath)
			}
			if scenario == "missing samples" {
				if _, statErr := os.Stat(samplesPath); !os.IsNotExist(statErr) {
					t.Fatalf("missing samples were recreated: %v", statErr)
				}
			}
		})
	}
}

func TestContextReportDeduplicatesTranscriptReplays(t *testing.T) {
	weekStart, now := contextReportTestWeek()
	root := t.TempDir()
	at := weekStart.Add(2 * time.Hour)
	firstPath := filepath.Join(t.TempDir(), "P.jsonl")
	copyPath := filepath.Join(t.TempDir(), "Q.jsonl")
	content := reportClaudeCall("shared", 110000, at)
	reportTestWrite(t, firstPath, content)
	reportTestWrite(t, copyPath, content)
	seedContextReportRead(t, root, "claude", "replayed", firstPath)
	if err := usage.RegisterSession(root, "claude", "replayed", 201, 2001); err != nil {
		t.Fatal(err)
	}
	seedContextReportSession(t, root, "claude", "other-session", reportClaudeCall("shared", 110000, at))
	if err := usage.RegisterSession(root, "claude", "other-session", 202, 2002); err != nil {
		t.Fatal(err)
	}
	seedContextReportSession(t, root, "codex", "other-runtime", reportCodexCall("shared", 110000, at, 1))
	if err := usage.RegisterSession(root, "codex", "other-runtime", 203, 2003); err != nil {
		t.Fatal(err)
	}
	_, _, baseline, err := WriteContextReport(root, weekStart, now)
	if err != nil || !baseline.Pass || baseline.Samples != 3 {
		t.Fatalf("baseline = %+v, err=%v", baseline, err)
	}

	seedContextReportRead(t, root, "claude", "replayed", copyPath)
	seedContextReportRead(t, root, "claude", "replayed", firstPath)
	raw, _, err := usage.Calls(root, "claude", "replayed", time.Time{})
	if err != nil || len(raw) != 3 {
		t.Fatalf("raw replay history = %d, err=%v", len(raw), err)
	}
	_, _, report, err := WriteContextReport(root, weekStart, now)
	if err != nil {
		t.Fatal(err)
	}
	if !report.Pass || report.Samples != baseline.Samples || report.P95 != baseline.P95 || report.Max != baseline.Max || report.DuplicateSamples != 2 {
		t.Fatalf("normalized replay report = %+v, baseline=%+v", report, baseline)
	}
}

func TestContextReportHandlesFallbackAndConflictingIdentities(t *testing.T) {
	weekStart, _ := contextReportTestWeek()
	weekEnd := weekStart.AddDate(0, 0, 7)
	at := weekStart.Add(time.Hour)
	provider := usage.CallSample{Runtime: "claude", Session: "one", InvocationID: "provider", PromptTokens: 100, InputTokens: 80, CacheCreation: 10, CacheRead: 10, At: at, Ordinal: 1, Source: "claude-transcript"}
	providerCopy := provider
	providerCopy.Ordinal = 99
	providerCopy.Source = "claude-transcript; restarted: path changed"
	fallback := usage.CallSample{Runtime: "claude", Session: "one", InvocationID: "line:7", PromptTokens: 200, InputTokens: 200, At: at, Ordinal: 7, Source: "claude-transcript"}
	fallbackCopy := fallback
	fallbackCopy.Source = "different annotation"
	fallbackReused := fallback
	fallbackReused.At = at.Add(time.Hour)
	marker := usage.Marker{Runtime: "claude", Session: "one", Kind: "compaction", At: at, Ordinal: 8, Detail: "trigger=auto"}
	samples, markers, duplicateSamples, duplicateMarkers, err := normalizeContextCalls(
		[]usage.CallSample{provider, providerCopy, fallback, fallbackCopy, fallbackReused},
		[]usage.Marker{marker, marker}, weekStart, weekEnd)
	if err != nil || len(samples) != 3 || len(markers) != 1 || duplicateSamples != 2 || duplicateMarkers != 1 {
		t.Fatalf("normalization = samples=%#v markers=%#v duplicate=%d/%d err=%v", samples, markers, duplicateSamples, duplicateMarkers, err)
	}

	providerConflict := providerCopy
	providerConflict.PromptTokens = 999999
	if _, _, _, _, err := normalizeContextCalls([]usage.CallSample{provider, providerConflict}, nil, weekStart, weekEnd); err == nil || !strings.Contains(err.Error(), "provider") {
		t.Fatalf("provider conflict = %v", err)
	}
	fallbackConflict := fallbackCopy
	fallbackConflict.PromptTokens = 999999
	if _, _, _, _, err := normalizeContextCalls([]usage.CallSample{fallback, fallbackConflict}, nil, weekStart, weekEnd); err == nil || !strings.Contains(err.Error(), "fallback") {
		t.Fatalf("fallback conflict = %v", err)
	}
	conflictRoot := t.TempDir()
	firstPath := seedContextReportSession(t, conflictRoot, "claude", "conflict", reportClaudeCall("same", 100000, at))
	if err := usage.RegisterSession(conflictRoot, "claude", "conflict", 302, 3002); err != nil {
		t.Fatal(err)
	}
	callsPath, reportPath, _, err := WriteContextReport(conflictRoot, weekStart, weekStart.Add(3*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	callsBefore, reportBefore := reportTestRead(t, callsPath), reportTestRead(t, reportPath)
	conflictingPath := filepath.Join(t.TempDir(), "conflicting.jsonl")
	reportTestWrite(t, conflictingPath, reportClaudeCall("same", 999999, at))
	seedContextReportRead(t, conflictRoot, "claude", "conflict", conflictingPath)
	if _, _, _, err := WriteContextReport(conflictRoot, weekStart, weekStart.Add(3*time.Hour)); err == nil || !strings.Contains(err.Error(), "conflicting provider") {
		t.Fatalf("published conflict = %v (first transcript %s)", err, firstPath)
	}
	if !bytes.Equal(reportTestRead(t, callsPath), callsBefore) || !bytes.Equal(reportTestRead(t, reportPath), reportBefore) {
		t.Fatal("a normalization conflict changed the previous report pair")
	}

	t.Run("unexpected source survives replay normalization", func(t *testing.T) {
		root := t.TempDir()
		content := reportClaudeCall("replayed-source", 100000, at)
		first := seedContextReportSession(t, root, "claude", "source-check", content)
		copyPath := filepath.Join(t.TempDir(), "source-copy.jsonl")
		reportTestWrite(t, copyPath, content)
		seedContextReportRead(t, root, "claude", "source-check", copyPath)
		if err := usage.RegisterSession(root, "claude", "source-check", 303, 3003); err != nil {
			t.Fatal(err)
		}
		samplesPath := usage.SamplesPath(root, "claude", "source-check")
		committed := reportTestRead(t, samplesPath)
		index := bytes.LastIndex(committed, []byte("claude-transcript"))
		if index < 0 {
			t.Fatalf("replayed source not found in %s (first transcript %s)", samplesPath, first)
		}
		copy(committed[index:], []byte("unexpected-source"))
		if err := os.WriteFile(samplesPath, committed, 0o644); err != nil {
			t.Fatal(err)
		}
		_, _, report, err := WriteContextReport(root, weekStart, weekStart.Add(3*time.Hour))
		if err != nil || report.Pass || report.Samples != 1 || report.DuplicateSamples != 1 ||
			!contextReportHasFailure(report, "unexpected sample source") {
			t.Fatalf("unexpected-source replay report = %+v, err=%v", report, err)
		}
	})

	root := t.TempDir()
	seedContextReportSession(t, root, "claude", "undated", reportClaudeCallWithTimestamp("undated", 100000, "not-a-time")+reportClaudeMarkerWithTimestamp("not-a-time"))
	if err := usage.RegisterSession(root, "claude", "undated", 301, 3001); err != nil {
		t.Fatal(err)
	}
	_, _, report, err := WriteContextReport(root, weekStart, weekStart.Add(3*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if report.Pass || report.UnassignableSamples != 1 || report.UnassignableMarkers != 1 || report.Samples != 0 {
		t.Fatalf("undated report = %+v", report)
	}
}

func TestContextReportWindowAndCoverage(t *testing.T) {
	weekStart, now := contextReportTestWeek()
	weekEnd := weekStart.AddDate(0, 0, 7)
	t.Run("half-open boundaries and declared exclusions", func(t *testing.T) {
		root := t.TempDir()
		content := reportClaudeCall("before", 100, weekStart.Add(-time.Nanosecond)) +
			reportClaudeCall("start", 101, weekStart) +
			reportClaudeCall("end-minus", 102, weekEnd.Add(-time.Nanosecond)) +
			reportClaudeCall("end", 103, weekEnd)
		seedContextReportSession(t, root, "claude", "boundaries", content)
		for index, row := range []struct{ runtime, session string }{{"claude", "boundaries"}, {"devin", "excluded"}, {"fake", "no-stream"}} {
			if err := usage.RegisterSession(root, row.runtime, row.session, int64(400+index), int64(4000+index)); err != nil {
				t.Fatal(err)
			}
		}
		callsPath, reportPath, report, err := WriteContextReport(root, weekStart, now)
		if err != nil {
			t.Fatal(err)
		}
		if !report.Pass || report.Samples != 2 || report.P95 != 102 || report.Max != 102 {
			t.Fatalf("boundary report = %+v", report)
		}
		calls := reportTestRead(t, callsPath)
		if bytes.Contains(calls, []byte(`"before"`)) || bytes.Contains(calls, []byte(`"end"`)) || !bytes.Contains(calls, []byte(`"start"`)) || !bytes.Contains(calls, []byte(`"end-minus"`)) {
			t.Fatalf("half-open export = %s", calls)
		}
		digest := sha256.Sum256(calls)
		if !bytes.Contains(reportTestRead(t, reportPath), []byte(hex.EncodeToString(digest[:]))) {
			t.Fatal("report does not bind the calls export digest")
		}
	})

	t.Run("registrations outside the window do not create gaps", func(t *testing.T) {
		root := t.TempDir()
		seedContextReportSession(t, root, "claude", "current", reportClaudeCall("current", 100, weekStart.Add(time.Hour)))
		if err := usage.RegisterSession(root, "claude", "current", 480, 4800); err != nil {
			t.Fatal(err)
		}
		registry := filepath.Join(root, "artifacts", "agents", "context", "sessions.jsonl")
		for _, row := range []usage.CallRegistration{
			{Runtime: "claude", Session: "retired-before", PID: 481, PIDStartedAt: 4801, FirstSeen: weekStart.Add(-time.Nanosecond)},
			{Runtime: "claude", Session: "started-after", PID: 482, PIDStartedAt: 4802, FirstSeen: weekEnd},
		} {
			encoded, err := json.Marshal(row)
			if err != nil {
				t.Fatal(err)
			}
			reportTestAppend(t, registry, string(encoded)+"\n")
		}
		_, _, report, err := WriteContextReport(root, weekStart, now)
		if err != nil || !report.Pass || report.Samples != 1 || contextReportHasFailure(report, "has no dated sample in the week") {
			t.Fatalf("out-of-window registrations changed the report = %+v, err=%v", report, err)
		}
	})

	t.Run("reset predicate", func(t *testing.T) {
		rows := []usage.CallRegistration{
			{Runtime: "claude", Session: "predecessor", PID: 1, PIDStartedAt: 10, FirstSeen: weekStart.Add(-time.Hour)},
			{Runtime: "claude", Session: "successor", PID: 1, PIDStartedAt: 10, FirstSeen: weekStart.Add(time.Hour)},
			{Runtime: "claude", Session: "successor", PID: 1, PIDStartedAt: 10, FirstSeen: weekStart.Add(time.Hour)},
			{Runtime: "claude", Session: "pid-reused", PID: 1, PIDStartedAt: 11, FirstSeen: weekStart.Add(2 * time.Hour)},
		}
		if got := contextReportResets(distinctContextRegistrations(rows), weekStart, weekEnd); got != 1 {
			t.Fatalf("resets = %d, want one successor and no PID-reuse reset", got)
		}
	})

	t.Run("empty and registered gaps", func(t *testing.T) {
		root := t.TempDir()
		_, _, empty, err := WriteContextReport(root, weekStart, now)
		if err != nil || empty.Pass || !contextReportHasFailure(empty, "cohort is empty") {
			t.Fatalf("empty report = %+v, err=%v", empty, err)
		}
		if err := usage.RegisterSession(root, "claude", "missing-samples", 501, 5001); err != nil {
			t.Fatal(err)
		}
		_, _, missing, err := WriteContextReport(root, weekStart, now)
		if err != nil || missing.Pass || !contextReportHasFailure(missing, "registered per-call session claude/missing-samples") {
			t.Fatalf("registered gap = %+v, err=%v", missing, err)
		}
	})

	t.Run("p95 must be strictly below the bound", func(t *testing.T) {
		root := t.TempDir()
		seedContextReportSession(t, root, "claude", "at-bound", reportClaudeCall("at-bound", ContextBoundTokens, weekStart.Add(time.Hour)))
		if err := usage.RegisterSession(root, "claude", "at-bound", 503, 5003); err != nil {
			t.Fatal(err)
		}
		_, _, report, err := WriteContextReport(root, weekStart, now)
		if err != nil || report.Pass || report.P95 != ContextBoundTokens || !contextReportHasFailure(report, "not below") {
			t.Fatalf("bound report = %+v, err=%v", report, err)
		}
	})

	t.Run("markdown escapes lawful session identities", func(t *testing.T) {
		root := t.TempDir()
		session := "unsafe|session\n## injected"
		seedContextReportSession(t, root, "claude", session, reportClaudeCall("safe-render", 100, weekStart.Add(time.Hour)))
		if err := usage.RegisterSession(root, "claude", session, 504, 5004); err != nil {
			t.Fatal(err)
		}
		_, reportPath, report, err := WriteContextReport(root, weekStart, now)
		if err != nil || !report.Pass {
			t.Fatalf("unsafe-identity report = %+v, err=%v", report, err)
		}
		markdown := reportTestRead(t, reportPath)
		if bytes.Contains(markdown, []byte("claude/unsafe|session\n## injected")) ||
			!bytes.Contains(markdown, []byte(`claude/unsafe\|session ## injected`)) {
			t.Fatalf("session identity was not rendered as inline Markdown:\n%s", markdown)
		}
	})

	t.Run("unregistered explicit-session evidence", func(t *testing.T) {
		root := t.TempDir()
		seedContextReportSession(t, root, "claude", "unregistered", reportClaudeCall("one", 100, weekStart.Add(time.Hour)))
		_, _, report, err := WriteContextReport(root, weekStart, now)
		if err != nil || report.Pass || report.Samples != 1 ||
			!contextReportHasFailure(report, "reset coverage is unavailable for claude/unregistered: no registered process evidence") {
			t.Fatalf("unregistered explicit-session evidence = %+v, err=%v", report, err)
		}
	})

	t.Run("malformed registry", func(t *testing.T) {
		root := t.TempDir()
		registry := filepath.Join(root, "artifacts", "agents", "context", "sessions.jsonl")
		reportTestWrite(t, registry, `{"runtime":"claude"}`)
		if _, _, _, err := WriteContextReport(root, weekStart, now); err == nil || !strings.Contains(err.Error(), registry) {
			t.Fatalf("malformed registry error = %v", err)
		}
	})

	t.Run("handoff proxy terminal coverage and ordering", func(t *testing.T) {
		fixture := seedContextReportBaseline(t)
		if err := MintIntent(fixture.root, Intent{Nonce: "handoff", Reason: "seatHandoff"}); err != nil {
			t.Fatal(err)
		}
		if _, err := ConsumeIntent(fixture.root, "handoff"); err != nil {
			t.Fatal(err)
		}
		if err := StampLaunch(fixture.root, "handoff"); err != nil {
			t.Fatal(err)
		}
		consumed := filepath.Join(fixture.root, "artifacts", "agents", "steward", "consumed", "handoff.json")
		stamp := fixture.weekStart.Add(3 * time.Hour)
		if err := os.Chtimes(consumed, stamp, stamp); err != nil {
			t.Fatal(err)
		}
		writeContextReportJob(t, fixture.root, "z-job", "failed", "reference_mismatch: z", "", fixture.weekStart.Add(2*time.Hour))
		writeContextReportJob(t, fixture.root, "a-job", "failed", "", "reference_mismatch: a", fixture.weekStart.Add(2*time.Hour))
		_, _, report, err := WriteContextReport(fixture.root, fixture.weekStart, fixture.now)
		if err != nil || report.Handoffs != 1 || !reflect.DeepEqual(report.ReferenceMismatches, []string{"a-job", "z-job"}) || report.Pass {
			t.Fatalf("operational signals = %+v, err=%v", report, err)
		}
		writeContextReportJob(t, fixture.root, "no-time", "failed", "reference_mismatch: missing time", "", time.Time{})
		_, _, incomplete, err := WriteContextReport(fixture.root, fixture.weekStart, fixture.now)
		if err != nil || incomplete.Pass || !contextReportHasFailure(incomplete, "no-time has no valid terminal time") {
			t.Fatalf("missing terminal time = %+v, err=%v", incomplete, err)
		}
	})
}

func TestContextReportExcludesTranscriptDiagnostics(t *testing.T) {
	weekStart, now := contextReportTestWeek()
	root := t.TempDir()
	seedContextReportSession(t, root, "claude", "live", reportClaudeCall("live-call", 100000, weekStart.Add(time.Hour)))
	if err := usage.RegisterSession(root, "claude", "live", 601, 6001); err != nil {
		t.Fatal(err)
	}
	callsPath, _, baseline, err := WriteContextReport(root, weekStart, now)
	if err != nil || !baseline.Pass {
		t.Fatalf("baseline = %+v, err=%v", baseline, err)
	}
	callsBefore := reportTestRead(t, callsPath)
	writeLiveContextHolder(t, root, "claude", "live")
	diagnostic := filepath.Join(t.TempDir(), "diagnostic.jsonl")
	reportTestWrite(t, diagnostic, reportClaudeCall("diagnostic-high", 210001, weekStart.Add(2*time.Hour))+reportClaudeMarker(weekStart.Add(3*time.Hour)))
	role, reading, err := ContextBudgetLine(root, root, now, ContextOptions{Transcript: diagnostic})
	if err != nil || reading.Latest == nil || reading.Latest.PromptTokens != 210001 || role.Status != HealthDead {
		t.Fatalf("inferred diagnostic = role=%+v reading=%+v err=%v", role, reading, err)
	}
	role, _, err = ContextBudgetLine(root, root, now, ContextOptions{Runtime: "claude", Session: "absent", Transcript: diagnostic})
	if err != nil || role.Status != HealthDead {
		t.Fatalf("absent-session diagnostic = role=%+v err=%v", role, err)
	}
	_, _, report, err := WriteContextReport(root, weekStart, now)
	if err != nil {
		t.Fatal(err)
	}
	if report.Samples != baseline.Samples || report.Resets != baseline.Resets || !reflect.DeepEqual(report.Compactions, baseline.Compactions) || report.Pass != baseline.Pass || !bytes.Equal(reportTestRead(t, callsPath), callsBefore) {
		t.Fatalf("diagnostics entered the week: baseline=%+v after=%+v", baseline, report)
	}
}

type contextReportFixture struct {
	root        string
	weekStart   time.Time
	now         time.Time
	transcripts map[string]string
}

func seedContextReportBaseline(t *testing.T) contextReportFixture {
	t.Helper()
	weekStart, now := contextReportTestWeek()
	return seedContextReportBaselineAt(t, weekStart, now)
}

func seedHistoricalContextReportBaseline(t *testing.T) contextReportFixture {
	t.Helper()
	weekStart, _ := contextReportRetirableWeek()
	return seedContextReportBaselineAt(t, weekStart, weekStart.Add(12*time.Hour))
}

func seedContextReportBaselineAt(t *testing.T, weekStart, now time.Time) contextReportFixture {
	t.Helper()
	root := t.TempDir()
	var claude strings.Builder
	for index := 0; index < 20; index++ {
		claude.WriteString(reportClaudeCall(fmt.Sprintf("claude-%02d", index), int64(100000+index*1000), weekStart.Add(time.Duration(index+1)*time.Minute)))
	}
	claude.WriteString(reportClaudeMarker(weekStart.Add(-time.Hour)))
	claudePath := seedContextReportSession(t, root, "claude", "claude-main", claude.String())
	var codex strings.Builder
	for index := 0; index < 5; index++ {
		codex.WriteString(reportCodexCall(fmt.Sprintf("codex-%02d", index), 120000, weekStart.Add(time.Duration(index+30)*time.Minute), int64(index+1)))
	}
	codexPath := seedContextReportSession(t, root, "codex", "codex-main", codex.String())
	appendContextReportRegistration(t, root, usage.CallRegistration{
		Runtime: "claude", Session: "claude-main", PID: 101, PIDStartedAt: 1001, FirstSeen: weekStart.Add(time.Minute),
	})
	appendContextReportRegistration(t, root, usage.CallRegistration{
		Runtime: "codex", Session: "codex-main", PID: 102, PIDStartedAt: 1002, FirstSeen: weekStart.Add(time.Minute),
	})
	return contextReportFixture{
		root: root, weekStart: weekStart, now: now,
		transcripts: map[string]string{"claude/claude-main": claudePath, "codex/codex-main": codexPath},
	}
}

func assertContextReportBaseline(t *testing.T, report ContextReport) {
	t.Helper()
	if !report.Pass || report.Samples != 25 || report.P95 != 120000 || report.Max != 120000 || report.Resets != 0 ||
		report.Compactions["claude"] != 0 || report.Compactions["codex"] != 0 || len(report.ReferenceMismatches) != 0 {
		t.Fatalf("baseline report = %+v", report)
	}
}

func contextReportTestWeek() (time.Time, time.Time) {
	current := time.Now().UTC()
	start := time.Date(current.Year(), current.Month(), current.Day(), 0, 0, 0, 0, time.UTC)
	return start, start.Add(12 * time.Hour)
}

func contextReportRetirableWeek() (time.Time, time.Time) {
	start, _ := contextReportTestWeek()
	start = start.AddDate(0, 0, -22)
	return start, start.Add(12 * time.Hour)
}

func seedContextReportSession(t *testing.T, root, runtimeName, session, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), runtimeName+"-"+strings.ReplaceAll(session, "/", "-")+".jsonl")
	reportTestWrite(t, path, content)
	seedContextReportRead(t, root, runtimeName, session, path)
	return path
}

func seedContextReportRead(t *testing.T, root, runtimeName, session, transcript string) {
	t.Helper()
	reading, err := usage.LatestCall(root, runtimeName, session, usage.ReadOptions{
		Capability: usage.PerCall, Transcript: transcript, Now: time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if reading.Capability != usage.PerCall {
		t.Fatalf("unexpected seed reading: %+v", reading)
	}
}

func reportClaudeCall(id string, tokens int64, at time.Time) string {
	return reportClaudeCallWithTimestamp(id, tokens, at.Format(time.RFC3339Nano))
}

func reportClaudeCallWithTimestamp(id string, tokens int64, at string) string {
	return fmt.Sprintf(`{"type":"assistant","requestId":%q,"timestamp":%q,"message":{"usage":{"input_tokens":%d,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}}}`+"\n", id, at, tokens)
}

func reportClaudeMarker(at time.Time) string {
	return reportClaudeMarkerWithTimestamp(at.Format(time.RFC3339Nano))
}

func reportClaudeMarkerWithTimestamp(at string) string {
	return fmt.Sprintf(`{"type":"system","subtype":"compact_boundary","timestamp":%q,"compactMetadata":{"trigger":"auto","preTokens":149000}}`+"\n", at)
}

func reportCodexCall(id string, tokens int64, at time.Time, ordinal int64) string {
	return fmt.Sprintf(`{"type":"token_usage_record","timestamp":%q,"ordinal":%d,"payload":{"response_id":%q,"usage":{"input_tokens":%d,"cached_input_tokens":0,"cache_write_input_tokens":0}}}`+"\n", at.Format(time.RFC3339Nano), ordinal, id, tokens)
}

func writeContextReportJob(t *testing.T, root, id, status, errorText, refusal string, endedAt time.Time) {
	t.Helper()
	record := map[string]any{"jobId": id, "status": status, "error": errorText, "refusalClass": refusal}
	if !endedAt.IsZero() {
		record["endedAt"] = endedAt.Format(time.RFC3339Nano)
	}
	data, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "artifacts", "agents", "jobs", id+".json")
	reportTestWrite(t, path, string(data)+"\n")
}

func appendContextReportRegistration(t *testing.T, root string, row usage.CallRegistration) {
	t.Helper()
	data, err := json.Marshal(row)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "artifacts", "agents", "context", "sessions.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	_, writeErr := file.Write(append(data, '\n'))
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		t.Fatalf("registration write=%v close=%v", writeErr, closeErr)
	}
}

func setContextReportPairTimes(t *testing.T, root, runtimeName, session string, at time.Time) {
	t.Helper()
	for _, path := range []string{usage.CursorPath(root, runtimeName, session), usage.SamplesPath(root, runtimeName, session)} {
		if err := os.Chtimes(path, at, at); err != nil {
			t.Fatal(err)
		}
	}
}

func contextReportHasFailure(report ContextReport, part string) bool {
	for _, failure := range report.Failures {
		if strings.Contains(failure, part) {
			return true
		}
	}
	return false
}

func reportTestWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func reportTestAppend(t *testing.T, path, content string) {
	t.Helper()
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString(content); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func reportTestRead(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func reportTestSize(t *testing.T, path string) int64 {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info.Size()
}

func reportTestLines(t *testing.T, path string) int {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	count := 0
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		count++
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	return count
}
