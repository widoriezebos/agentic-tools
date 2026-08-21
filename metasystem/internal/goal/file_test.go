package goal

import (
	"strings"
	"testing"
)

func claimedGolden() *GoalFile {
	return &GoalFile{
		Id:       "backlog-git-sync",
		State:    StateClaimed,
		Intent:   "Multiple machines work the backlog in parallel with git as the sync",
		Origin:   "main",
		NextStep: "Implement against the obligation matrix, fixtures first",
		OpenedAt: "2026-08-20T00:31:00Z",
		Revision: 3,
		Blocked:  []string{"wall-o15-head-accounting"},
		Arc:      "",
		Claimed:  &ClaimRecord{Machine: "mac-studio", Lineage: "session-a", At: "2026-08-20T00:35:00Z"},
		History: []HistoryLine{
			{At: "2026-08-20T00:31:00Z", Opid: "01J5X0000000000000000000A0-mac-studio-1a2b3c4d", Verb: "open", Actor: "mac-studio+session-a", Targets: []string{"backlog-git-sync"}, Keep: -1},
			{At: "2026-08-20T00:35:00Z", Opid: "01J5X0000000000000000000B1-mac-studio-1a2b3c4d", Verb: "claim", Actor: "mac-studio+session-a", Targets: []string{"backlog-git-sync"}, Keep: -1},
			{At: "2026-08-20T01:00:00Z", Opid: "01J5X0000000000000000000C2-mac-studio-1a2b3c4d", Verb: "edit", Actor: "mac-studio+session-a", Targets: []string{"backlog-git-sync"}, Keep: -1},
		},
	}
}

func TestGoldenClaimedFileRoundTrips(t *testing.T) {
	golden := claimedGolden()
	bytes1 := RenderFile(golden)
	parsed, problems := ParseFile(bytes1)
	if len(problems) != 0 {
		t.Fatalf("golden claimed file must parse clean, got %v", problems)
	}
	bytes2 := RenderFile(parsed)
	if string(bytes1) != string(bytes2) {
		t.Fatalf("render/parse/render is not a fixed point:\n%s\n---\n%s", bytes1, bytes2)
	}
	if parsed.Claimed == nil || parsed.Claimed.Machine != "mac-studio" {
		t.Fatalf("claim record lost: %+v", parsed.Claimed)
	}
	if parsed.Revision != 3 || len(parsed.History) != 3 {
		t.Fatalf("revision/history lost: rev=%d len=%d", parsed.Revision, len(parsed.History))
	}
}

func TestGoldenArchivedFileCarriesExplicitDoneState(t *testing.T) {
	done := &GoalFile{
		Id:       "custody-death-proof",
		State:    StateDone,
		Intent:   "Prove custody death is detected",
		Origin:   "main",
		Conclude: "Landed with the supervision chain; witness in the suite",
		OpenedAt: "2026-08-20T00:40:00Z",
		Revision: 2,
		History: []HistoryLine{
			{At: "2026-08-20T00:40:00Z", Opid: "01J5X0000000000000000000D3-mac-studio-1a2b3c4d", Verb: "open", Actor: "mac-studio+session-a", Keep: -1},
			{At: "2026-08-20T02:00:00Z", Opid: "01J5X0000000000000000000E4-mac-studio-1a2b3c4d", Verb: "done", Actor: "mac-studio+session-a", Keep: -1},
		},
	}
	parsed, problems := ParseFile(RenderFile(done))
	if len(problems) != 0 {
		t.Fatalf("golden archived file must parse clean, got %v", problems)
	}
	if parsed.State != StateDone || parsed.Conclude == "" {
		t.Fatalf("archived file must carry State: done and Concluded, got %+v", parsed)
	}
}

func TestTamperedBytesFailIntegrityByName(t *testing.T) {
	bytes := RenderFile(claimedGolden())
	tampered := strings.Replace(string(bytes), "session-a", "session-b", 1)
	_, problems := ParseFile([]byte(tampered))
	found := false
	for _, p := range problems {
		if strings.Contains(string(p), "Integrity mismatch") {
			found = true
		}
	}
	if !found {
		t.Fatalf("a hand edit without a recomputed digest must fail Integrity by name, got %v", problems)
	}
}

func TestMissingIntegrityLineRefuses(t *testing.T) {
	bytes := RenderFile(claimedGolden())
	body, _, _ := splitIntegrity(bytes)
	_, problems := ParseFile(body)
	found := false
	for _, p := range problems {
		if strings.Contains(string(p), "missing Integrity") {
			found = true
		}
	}
	if !found {
		t.Fatalf("want missing-Integrity problem, got %v", problems)
	}
}

func TestStateRecordAgreementIsValidated(t *testing.T) {
	f := claimedGolden()
	f.State = StateQueued // record says claimed, state says queued
	_, problems := ParseFile(RenderFile(f))
	found := false
	for _, p := range problems {
		if strings.Contains(string(p), "Claimed record on a queued goal") {
			found = true
		}
	}
	if !found {
		t.Fatalf("state/record divergence must refuse, got %v", problems)
	}
}

func TestHistoryGrammarRoundTripsEveryField(t *testing.T) {
	line := HistoryLine{
		At:        "2026-08-20T01:00:00Z",
		Opid:      "01J5X0000000000000000000F5-intel-nuc-9f8e7d6c",
		Verb:      "park",
		Actor:     "human:wido",
		Targets:   []string{"a-goal", "b-goal"},
		Displaced: "mac-studio+session-a@2026-08-20T00:35:00Z",
		Ack:       false,
		Keep:      -1,
		Reason:    "operator assignment: second machine takes the arc, free text with = and spaces",
	}
	rendered := RenderHistoryLine(line)
	parsed, err := ParseHistoryLine(rendered)
	if err != nil {
		t.Fatalf("parse of rendered line: %v", err)
	}
	if RenderHistoryLine(parsed) != rendered {
		t.Fatalf("history line is not a fixed point:\n%s\n%s", rendered, RenderHistoryLine(parsed))
	}
	if parsed.Reason != line.Reason {
		t.Fatalf("reason= must consume the remainder losslessly, got %q", parsed.Reason)
	}
}

func TestPruneKeepFieldIsLawful(t *testing.T) {
	parsed, err := ParseHistoryLine("- 2026-08-20T03:00:00Z 01J5X0000000000000000000A6-mac-studio-1a2b3c4d prune actor=mac-studio+session-a targets=old-one,old-two keep=50")
	if err != nil {
		t.Fatalf("prune keep= line must parse: %v", err)
	}
	if parsed.Keep != 50 {
		t.Fatalf("keep lost: %d", parsed.Keep)
	}
}

func TestUnknownHistoryKeyRefuses(t *testing.T) {
	_, err := ParseHistoryLine("- 2026-08-20T03:00:00Z opid verb actor=a+b sneaky=1")
	if err == nil || !strings.Contains(err.Error(), "unknown History key") {
		t.Fatalf("unknown key must refuse by name, got %v", err)
	}
}

func TestHistoryPrefixIsADiagnosticHelper(t *testing.T) {
	full := claimedGolden().History
	if !HistoryIsPrefix(full[:2], full) {
		t.Fatal("a strict prefix must be detected")
	}
	if HistoryIsPrefix(full, full) {
		t.Fatal("equal histories are not a strict prefix")
	}
	divergent := append([]HistoryLine{}, full[:1]...)
	divergent = append(divergent, HistoryLine{At: "x", Opid: "y", Verb: "z", Actor: "a+b", Keep: -1})
	if HistoryIsPrefix(divergent, full) {
		t.Fatal("a divergent history is not a prefix")
	}
}

func TestOpidAttributesExecution(t *testing.T) {
	a := Opid("01J5X0000000000000000000A0", "mac-studio", "session-a")
	b := Opid("01J5X0000000000000000000A0", "mac-studio", "session-b")
	if a == b {
		t.Fatal("different lineages must hash differently")
	}
	if !strings.HasPrefix(a, "01J5X0000000000000000000A0-mac-studio-") {
		t.Fatalf("opid shape: %s", a)
	}
}

func TestParkedRecordRoundTripsWithDisplacementAndFreeText(t *testing.T) {
	f := claimedGolden()
	f.State = StateParked
	f.Claimed = nil
	f.Parked = &ParkRecord{
		By:        "operator",
		At:        "2026-08-20T04:00:00Z",
		Because:   "yields to the sync build; free text with = signs and, commas",
		Displaced: "mac-studio+session-a@2026-08-20T00:35:00Z",
	}
	parsed, problems := ParseFile(RenderFile(f))
	if len(problems) != 0 {
		t.Fatalf("parked golden must parse clean, got %v", problems)
	}
	if parsed.Parked == nil || parsed.Parked.Because != f.Parked.Because || parsed.Parked.Displaced != f.Parked.Displaced {
		t.Fatalf("park record lost: %+v", parsed.Parked)
	}
	if string(RenderFile(parsed)) != string(RenderFile(f)) {
		t.Fatal("parked file is not a render fixed point")
	}
}
