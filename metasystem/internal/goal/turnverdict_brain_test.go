package goal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/brain"
)

func TestBrainTurnVerdictLeadsDisplayAndNeverEnforcesIdleBacklog(t *testing.T) {
	approved := budgetedQueuedGoal("approved", "2026-09-07T00:00:00Z")
	held := &GoalFile{Id: "held", State: StateClaimed, Intent: "Held work", Origin: OriginMain,
		NextStep: "Release it.", OpenedAt: "2026-09-07T00:01:00Z", Revision: 2,
		Claimed: &ClaimRecord{Machine: "bed-m1", Lineage: "old-brain", At: "2026-09-07T00:02:00Z"}}
	remote := &GoalFile{Id: "remote", State: StateClaimed, Intent: "Remote work", Origin: OriginMain,
		NextStep: "Node carries it.", OpenedAt: "2026-09-07T00:03:00Z", Revision: 2,
		Claimed: &ClaimRecord{Machine: "bed-m2", Lineage: "node", At: "2026-09-07T00:04:00Z"}}
	root := servingBed(t, "bed-m1", map[string]*GoalFile{"approved": approved, "held": held, "remote": remote})
	identity := ExistingLedgerIdentity(root)
	stamp := "2026-09-07T00:00:00Z"
	record := brain.Record{Schema: 1, Ledger: identity, Machine: "bed-m1", DeclaredBy: "Wido", DeclaredAt: stamp}
	data, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(brain.Path(root)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(brain.Path(root), append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, 9, 7, 1, 0, 0, 0, time.UTC)
	store := &Store{Root: root, Now: func() time.Time { return now }}
	store.PrepareIdleContinuation = func(IdleEscalationEvent) (string, error) {
		t.Fatal("brain verdict prepared an idle continuation")
		return "", nil
	}
	scan := ScanResult{
		Questions: []Item{{Kind: "question", Id: "ask-one"}},
		Drafts:    []Item{{Kind: "draft", Id: "draft-one"}},
		Open:      []Item{{Kind: "plan", Id: "open-plan", Detail: "OPEN-WORK open-plan: finish it"}},
	}
	first, err := store.TurnVerdict(scan, "brain-session", "", "main-brain")
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(first.Display, "\n")
	if len(lines) < 3 || !strings.HasPrefix(lines[0], "BRAIN SEAT: nodes hold 2 claims (held, remote); 1 approved goals await a node; 1 asks await Wido (ask-one); 1 drafts await approval (draft-one)") {
		t.Fatalf("brain summary was not the first display line: %q", first.Display)
	}
	if lines[1] != "HELD HERE: held; release them to a node" {
		t.Fatalf("held claim was not the next brain line: %q", first.Display)
	}
	if !first.ShouldBlock || first.BlockSource == nil || *first.BlockSource != "open-work" || first.IdleRefusal {
		t.Fatalf("the first ordinary open-work block changed: %+v", first)
	}
	if !first.BrainStatusDue {
		t.Fatal("first brain verdict did not request the unposted status line")
	}
	if _, err := brain.ReadStatus(root); err != nil {
		t.Fatalf("brain verdict did not write status: %v", err)
	}

	if err := brain.MarkStatusPosted(root, now); err != nil {
		t.Fatal(err)
	}
	second, err := store.TurnVerdict(scan, "brain-session", "", "main-brain")
	if err != nil {
		t.Fatal(err)
	}
	if second.ShouldBlock || second.BlockSource != nil || second.IdleRefusal || second.BrainStatusDue {
		t.Fatalf("brain repeated an open-work or idle block, or ignored the fresh status: %+v", second)
	}
	if strings.Contains(second.Display, "the goal file names the next step") || strings.Contains(second.Display, "IDLE WITH BACKLOG") {
		t.Fatalf("brain ran a goal or idle clause: %q", second.Display)
	}
}

func TestCorruptBrainTurnVerdictKeepsItsRemedySecond(t *testing.T) {
	root := servingBed(t, "bed-m1", map[string]*GoalFile{})
	if err := os.MkdirAll(filepath.Dir(brain.Path(root)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(brain.Path(root), []byte("{broken\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	verdict, err := (&Store{Root: root}).TurnVerdict(ScanResult{}, "corrupt-brain", "", "")
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(verdict.Display, "\n")
	if len(lines) < 2 || !strings.HasPrefix(lines[0], "BRAIN SEAT:") || !strings.HasPrefix(lines[1], "this checkout's brain declaration is unreadable") {
		t.Fatalf("corrupt brain prefix order wrong: %q", verdict.Display)
	}
	if verdict.ShouldBlock || verdict.IdleRefusal {
		t.Fatalf("corrupt brain was prodded by the idle ladder: %+v", verdict)
	}
}

func TestUndeclaredCheckoutDropsOnlyBrainDraftScannerFailure(t *testing.T) {
	scan := ScanResult{
		Questions:  []Item{{Kind: "question", Id: "ask-one"}},
		Drafts:     []Item{{Kind: "draft", Id: "draft-one"}},
		Busy:       []Item{{Kind: "job", Id: "pending-one"}, {Kind: "job", Id: "running-one"}},
		Jobs:       []JobFact{{Id: "pending-one", Status: "pending-setup"}, {Id: "running-one", Status: "running"}},
		Unreadable: []string{"draft scan: fixture projection failed", "question scan: malformed fixture question", "plan scan failed"},
	}
	got := withoutBrainOnlyScanEffects(scan)
	if len(got.Questions) != 1 || got.Questions[0].Id != "ask-one" || len(got.Drafts) != 1 || got.Drafts[0].Id != "draft-one" {
		t.Fatalf("undeclared checkout lost the two populated brain fields: %+v", got)
	}
	if len(got.Jobs) != 2 || len(got.Busy) != 2 {
		t.Fatalf("undeclared turn verdict changed the scanner's trunk job classification: %+v", got)
	}
	if len(got.Unreadable) != 1 || got.Unreadable[0] != "plan scan failed" {
		t.Fatalf("brain-only scan failure changed an undeclared checkout's all-clear: %+v", got.Unreadable)
	}
}
