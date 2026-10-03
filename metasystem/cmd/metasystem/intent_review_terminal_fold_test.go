package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

// terminalFoldBed is a one-round code-critic examination whose register was
// never folded: nothing dispatched a follow-up, so the register stands at
// round 0 while its terminal round 1 is complete. The close owner is the
// real one.
func terminalFoldBed(t *testing.T, job string, findings []any, rigor []any) (*deliveryBed, func(map[string][]string) *intentInvocation) {
	t.Helper()
	b := newDeliveryBed(t)
	realCloseOwner(t, b)
	b.writeFile(filepath.Join(b.install, "artifacts", "agents", "capabilities", "close.json"), `{"ok":true}`)
	if err := os.MkdirAll(filepath.Join(b.install, "artifacts", "agents", "record-locks"), 0o755); err != nil {
		t.Fatal(err)
	}
	conf := filepath.Join(b.install, "metasystem.conf")
	existing, _ := os.ReadFile(conf)
	b.writeFile(conf, string(existing)+"\nevidence.root="+t.TempDir()+"\n")
	b.writeJob(map[string]any{"jobId": job, "role": "code-critic", "status": "completed", "round": 1,
		"destructiveReach": "DESIGN-BEARING", "dispatchMode": "fresh", "sessionId": job + "-session", "endedAt": "2026-10-02T11:00:00Z",
		"capabilitySnapshot": "artifacts/agents/capabilities/close.json", "parentJob": nil,
		"findingRegister": []any{}, "findingRegisterRound": 0})
	verdict := "clean"
	if len(findings) > 0 {
		verdict = "findings"
	}
	b.writeJSON(filepath.Join(b.install, "artifacts", "agents", job, "rounds", "1", "return.json"),
		map[string]any{"jobId": job, "round": 1, "findings": findings, "rigor": rigor, "verdict": verdict})
	owners := b.intentBed.owners()
	owners.delivery = b.owners
	layout, err := stateroot.NewResolver(fakeTop(b.root()), noExecutable).ResolveLayout(b.root())
	if err != nil {
		t.Fatal(err)
	}
	return b, func(values map[string][]string) *intentInvocation {
		return &intentInvocation{owners: owners, layout: layout, cwd: b.root(), stateRoot: b.root(), input: intentInput{values: values},
			reviewWork: &reviewWorkContext{goal: bedGoal, work: "main", attempt: 1}}
	}
}

// TestIntentCommitReviewClosesItsTerminalRound: a one-round commit review
// closes through the real close owner without any follow-up having folded
// its round, both clean and with the author's dispositions.
func TestIntentCommitReviewClosesItsTerminalRound(t *testing.T) {
	subject := strings.Repeat("c", 40)
	t.Run("clean", func(t *testing.T) {
		b, invocation := terminalFoldBed(t, "critclean", []any{}, []any{})
		if pending := invocation(map[string][]string{}).closeWorkReview(nil, b.install, subject, "critclean"); pending != nil {
			t.Fatalf("a clean one-round review did not close: %+v data=%v", pending, pending.Data)
		}
		critic := b.job("critclean")
		if closed, _ := critic["chainClosed"].(bool); !closed {
			t.Fatalf("the clean chain is not closed: %v", critic)
		}
		if round, _ := critic["findingRegisterRound"].(float64); round != 1 {
			t.Fatalf("the terminal round was not folded: %v", critic["findingRegisterRound"])
		}
	})
	t.Run("dispositioned", func(t *testing.T) {
		b, invocation := terminalFoldBed(t, "critdecided", []any{
			map[string]any{"id": "F1", "material": true, "claim": "a material claim", "evidence": "evidence"},
			map[string]any{"id": "F2", "material": false, "claim": "a remark", "evidence": "evidence"},
		}, foldRigor("F1"))
		digest, _, err := reviewReturnDigest(filepath.Join(b.install, "artifacts", "agents", "critdecided", "rounds", "1", "return.json"))
		if err != nil {
			t.Fatal(err)
		}
		decided := filepath.Join(b.root(), "decided.md")
		b.writeFile(decided, reviewBinding{Goal: bedGoal, Work: "main", Attempt: 1, Subject: subject, Examination: "critdecided", Round: 1, Return: digest}.line()+"\n\n"+
			deliveryDispositionsHeader+"| F1 | refuted | the claimed path is unreachable | none |\n| F2 | noted | a remark | none |\n")
		if pending := invocation(map[string][]string{"dispositions": {decided}}).closeWorkReview(nil, b.install, subject, "critdecided"); pending != nil {
			t.Fatalf("a dispositioned one-round review did not close: %+v data=%v", pending, pending.Data)
		}
		critic := b.job("critdecided")
		if closed, _ := critic["chainClosed"].(bool); !closed {
			t.Fatalf("the dispositioned chain is not closed: %v", critic)
		}
		register, _ := critic["findingRegister"].([]any)
		if len(register) != 1 {
			t.Fatalf("the register does not carry the material finding: %v", register)
		}
		if entry, _ := register[0].(map[string]any); entry["findingId"] != "F1" || entry["status"] != "resolved" || entry["resolution"] != "refuted" {
			t.Fatalf("the refuted finding is not resolved in the register: %v", entry)
		}
	})
}

// TestIntentCommitReviewUndecidedTerminalFindingRefuses: folding the
// terminal round never decides a finding. A material finding the author
// accepted (a fix is required) stays open and the close refuses.
func TestIntentCommitReviewUndecidedTerminalFindingRefuses(t *testing.T) {
	b, invocation := terminalFoldBed(t, "critopen", []any{
		map[string]any{"id": "F1", "material": true, "claim": "a material claim", "evidence": "evidence"},
	}, foldRigor("F1"))
	decided := filepath.Join(b.root(), "accepted.md")
	b.writeFile(decided, deliveryDispositionsHeader+"| F1 | accepted | a fix is required | none |\n")
	closed := invocation(map[string][]string{"dispositions": {decided}}).closeChain("critopen")
	if closed.Outcome == intentConfirmed || closed.Outcome == intentUnchanged {
		t.Fatalf("a chain with an undecided finding closed: %+v", closed)
	}
	critic := b.job("critopen")
	if done, _ := critic["chainClosed"].(bool); done {
		t.Fatalf("the chain was marked closed: %v", critic)
	}
	register, _ := critic["findingRegister"].([]any)
	if len(register) != 1 {
		t.Fatalf("the terminal round's finding is not in the register: %v", register)
	}
	if entry, _ := register[0].(map[string]any); entry["status"] != "open" {
		t.Fatalf("the accepted finding is not open: %v", entry)
	}
	if err := dispatchcore.CloseCheck(b.install, "critopen"); err == nil || !strings.Contains(err.Error(), "unresolved finding F1") {
		t.Fatalf("the close check does not refuse the open finding: %v", err)
	}
}

// foldRigor names the finding's artifact, which the register needs to admit
// a material finding.
func foldRigor(id string) []any {
	return []any{map[string]any{"findingId": id, "artifact": "metasystem/internal/x.go", "rigorClass": "bounded", "facts": nil}}
}
