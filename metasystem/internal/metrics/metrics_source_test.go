package metrics

import (
	"crypto/sha1"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

const (
	sourceBaseline = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	sourceLanding  = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	sourceFixes    = "cccccccccccccccccccccccccccccccccccccccc"
	sourceAccepted = "dddddddddddddddddddddddddddddddddddddddd"
	sourceBlob     = "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
)

type sourceSnapshot struct {
	machine      string
	mainTip      string
	acceptedTip  string
	receiptBlob  string
	moveLog      string
	mainLog      string
	legacyPatch  string
	currentPatch string
	receiptText  string
	goalFiles    map[string][]byte
}

type sourceFixture struct {
	*fixtureRepo
	facts sourceSnapshot
}

func newSourceFixture(t *testing.T) *sourceFixture {
	t.Helper()
	base := t.TempDir()
	repo := &fixtureRepo{t: t, repo: filepath.Join(base, "repository"), evidence: filepath.Join(base, "evidence")}
	repo.root = filepath.Join(repo.repo, "metasystem")
	for _, path := range []string{
		filepath.Join(repo.root, "plans"), filepath.Join(repo.root, "artifacts", "agents"),
		filepath.Join(repo.evidence, "suite-failures"),
	} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	repo.write("metasystem/metasystem.conf", "evidence.root="+repo.evidence+"\n")
	repo.write("metasystem/memory/receipts.log", "")
	return &sourceFixture{fixtureRepo: repo}
}

func (f *sourceFixture) seedFullWorld() {
	f.t.Helper()
	f.originalReceipt = "1770000000|2026-08-19T12:00:00Z|RECEIPT|type=implement|outcome=shipped|skills=none|verify=clean|corrections=1|stop_loss=no|delegate=fake:model:j1|goal=g1|built_by=delegate|critique_waived=none|waiver_stream=none|note=landed"
	oldReceipt := "1770000001|2026-08-19T13:00:00Z|RECEIPT|type=review|outcome=shipped|skills=none|verify=clean|corrections=0|stop_loss=no|delegate=none|critique_waived=none|waiver_stream=none|note=old row"
	digest := fmt.Sprintf("%x", sha1.Sum([]byte(f.originalReceipt)))
	correction1 := "1771000000|2026-08-25T00:00:00Z|CORRECTION|ref_epoch=1770000000|ref_sha1=" + digest + "|field=corrections|was=1|now=2|reason=first"
	correction2 := "1771000001|2026-08-25T00:01:00Z|CORRECTION|ref_epoch=1770000000|ref_sha1=" + digest + "|field=corrections|was=1|now=3|reason=last"
	receiptText := strings.Join([]string{f.originalReceipt, oldReceipt, correction1, correction2}, "\n") + "\n"
	f.write("metasystem/memory/receipts.log", receiptText)
	f.write("metasystem/payload.txt", "one\ntwo\nthree\nfour\n")

	g1 := &goal.GoalFile{
		Id: "g1", State: goal.StateDone, Intent: "Ship fixture work.", Origin: goal.OriginMain,
		NextStep: "Ship it.", Conclude: "Shipped.", OpenedAt: "2026-08-01T00:00:00Z", Revision: 3,
		Budget: &goal.Budget{ElapsedLimit: "1d", AttemptLimit: 3, ReservedJobMinutesLimit: 480, ActiveJobLimit: 2},
		History: []goal.HistoryLine{
			history("2026-08-01T00:00:00Z", "01J5X00000000000000000P000-machine-a-11111111", "open"),
			history("2026-08-05T00:00:00Z", "01J5X00000000000000000P010-machine-a-11111111", "claim"),
			history("2026-08-20T12:00:00Z", "01J5X00000000000000000P020-machine-a-11111111", "done"),
		},
	}
	parked := &goal.GoalFile{
		Id: "parked-old", State: goal.StateParked, Intent: "Parked debt.", Origin: goal.OriginMain,
		NextStep: "Revisit.", OpenedAt: "2026-06-01T00:00:00Z", Revision: 2,
		Parked: &goal.ParkRecord{By: "machine-a+fixture", At: "2026-07-01T00:00:00Z", Because: "waiting"},
		History: []goal.HistoryLine{
			{At: "2026-06-01T00:00:00Z", Opid: "01J5X00000000000000000P030-machine-a-11111111", Verb: "open", Actor: "machine-a+fixture", Targets: []string{"parked-old"}, Keep: -1},
			{At: "2026-07-01T00:00:00Z", Opid: "01J5X00000000000000000P040-machine-a-11111111", Verb: "park", Actor: "machine-a+fixture", Targets: []string{"parked-old"}, Keep: -1},
		},
	}
	queued := &goal.GoalFile{
		Id: "queued-no-budget", State: goal.StateQueued, Intent: "Queued debt.", Origin: goal.OriginMain,
		NextStep: "Investigate later.", OpenedAt: "2026-07-01T00:00:00Z", Revision: 1,
		History: []goal.HistoryLine{{At: "2026-07-01T00:00:00Z", Opid: "01J5X00000000000000000P050-machine-a-11111111", Verb: "open", Actor: "machine-a+fixture", Targets: []string{"queued-no-budget"}, Keep: -1}},
	}
	collision := &goal.GoalFile{
		Id: "collision", State: goal.StateClaimed, Intent: "Collision fixture.", Origin: goal.OriginMain,
		NextStep: "Continue.", OpenedAt: "2026-08-01T00:00:00Z", Revision: 2,
		Claimed: &goal.ClaimRecord{Machine: "machine-a", Lineage: "fixture", At: "2026-08-21T00:00:00Z"},
		History: []goal.HistoryLine{
			{At: "2026-08-01T00:00:00Z", Opid: "01J5X00000000000000000P060-machine-a-11111111", Verb: "open", Actor: "machine-a+fixture", Targets: []string{"collision"}, Keep: -1},
			{At: "2026-08-21T00:00:00Z", Opid: "01J5X00000000000000000P070-machine-a-11111111", Verb: "steal", Actor: "machine-a+fixture", Targets: []string{"collision"}, Displaced: "machine-b+other@2026-08-20T00:00:00Z", Keep: -1},
		},
	}
	goalFiles := map[string][]byte{}
	for _, file := range []*goal.GoalFile{g1, parked, queued, collision} {
		path := filepath.Join("metasystem", "plans", "goals", file.Id+".md")
		if file.Id == "g1" {
			path = filepath.Join("metasystem", "plans", "goals", "done", file.Id+".md")
		}
		content := goal.RenderFile(file)
		f.write(filepath.ToSlash(path), string(content))
		goalFiles[strings.TrimPrefix(filepath.ToSlash(path), "metasystem/")] = append([]byte(nil), content...)
	}

	f.job("j1", map[string]any{
		"jobId": "j1", "role": "implementer", "status": "completed", "goalId": "g1", "round": 1,
		"runtime": "codex", "startedAt": "2026-08-18T10:00:00Z", "endedAt": "2026-08-18T12:00:00Z",
		"usage": map[string]any{"inputTokens": 10, "cost": map[string]any{"amount": 1, "currency": "USD"}, "providerUnits": map[string]any{"name": "credits", "value": 2}},
	})
	f.job("critic", map[string]any{
		"jobId": "critic", "role": "design-critic", "status": "completed", "goalId": "g1", "round": 1,
		"runtime": "claude", "startedAt": "2026-08-18T12:00:00Z", "endedAt": "2026-08-18T13:00:00Z",
		"usage": map[string]any{"outputTokens": 5, "cost": map[string]any{"amount": 2, "currency": "EUR"}},
	})
	f.job("critic-r2", map[string]any{
		"jobId": "critic-r2", "parentJob": "critic", "role": "design-critic", "status": "completed", "goalId": "g1", "round": 2,
		"runtime": "codex", "startedAt": "2026-08-18T13:00:00Z", "endedAt": "2026-08-18T14:00:00Z",
		"usage": map[string]any{"inputTokens": 5, "providerUnits": map[string]any{"name": "credits", "value": 1}},
	})

	chain := filepath.Join(f.root, "artifacts", "agents", "critiques", "fixture-chain")
	if err := os.MkdirAll(chain, 0o755); err != nil {
		f.t.Fatal(err)
	}
	for path, content := range map[string]string{
		"attribution": "goal g1\n", "r1-output.md": "ok\n", "r2-output.md": "ok\n",
	} {
		if err := os.WriteFile(filepath.Join(chain, path), []byte(content), 0o644); err != nil {
			f.t.Fatal(err)
		}
	}
	enumeration := filepath.Join(f.root, "artifacts", "agents", "enumeration-report.txt")
	if err := os.WriteFile(enumeration, []byte("section\tdirect-validation\tretained validator\tpass\t0\n"), 0o644); err != nil {
		f.t.Fatal(err)
	}
	proofAt := time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC)
	if err := os.Chtimes(enumeration, proofAt, proofAt); err != nil {
		f.t.Fatal(err)
	}
	f.writeJSON("metasystem/artifacts/agents/goal-transactions/rejected.json", map[string]any{
		"intent": map[string]any{"verb": "claim", "targets": []string{"g1"}},
		"phase":  "terminal", "outcome": "rejected", "attempts": 1,
		"terminalAt": "2026-08-21T00:00:00Z", "evidence": "claim refused",
	})

	goalNumstat := ""
	for _, path := range []string{
		"metasystem/plans/goals/collision.md",
		"metasystem/plans/goals/done/g1.md",
		"metasystem/plans/goals/parked-old.md",
		"metasystem/plans/goals/queued-no-budget.md",
	} {
		goalNumstat += fmt.Sprintf("%d\t0\t%s\n", strings.Count(string(goalFiles[strings.TrimPrefix(path, "metasystem/")]), "\n"), path)
	}
	mainLog := strings.Join([]string{
		"\x1e" + sourceBaseline + "\x1f\x1ffixture@example.invalid\x1f2026-08-01T00:00:00Z\n\n1\t0\tmetasystem/metasystem.conf\n0\t0\tmetasystem/memory/receipts.log\n",
		"\x1e" + sourceLanding + "\x1f" + sourceBaseline + "\x1ffixture@example.invalid\x1f2026-08-19T12:00:00Z\n\n2\t0\tmetasystem/memory/receipts.log\n4\t0\tmetasystem/payload.txt\n",
		"\x1e" + sourceFixes + "\x1f" + sourceLanding + "\x1ffixture@example.invalid\x1f2026-08-25T00:02:00Z\n\n2\t0\tmetasystem/memory/receipts.log\n",
		"\x1e" + sourceAccepted + "\x1f" + sourceFixes + "\x1fgoals@metasystem.invalid\x1f2026-08-26T00:00:00Z\n\n" + goalNumstat,
	}, "")
	currentPatch := "\x1e" + sourceBaseline + "\n\n" +
		"diff --git a/metasystem/memory/receipts.log b/metasystem/memory/receipts.log\n" +
		"new file mode 100644\nindex 0000000..e69de29\n" +
		"\x1e" + sourceLanding + "\n\n" +
		"diff --git a/metasystem/memory/receipts.log b/metasystem/memory/receipts.log\n" +
		"--- a/metasystem/memory/receipts.log\n+++ b/metasystem/memory/receipts.log\n" +
		"@@ -0,0 +1,2 @@\n+" + f.originalReceipt + "\n+" + oldReceipt + "\n" +
		"\x1e" + sourceFixes + "\n\n" +
		"diff --git a/metasystem/memory/receipts.log b/metasystem/memory/receipts.log\n" +
		"--- a/metasystem/memory/receipts.log\n+++ b/metasystem/memory/receipts.log\n" +
		"@@ -2,0 +3,2 @@\n+" + correction1 + "\n+" + correction2 + "\n"
	f.facts = sourceSnapshot{
		machine: "machine-a", mainTip: sourceAccepted, acceptedTip: sourceAccepted,
		receiptBlob: sourceBlob, moveLog: sourceBaseline + "\n", mainLog: mainLog,
		legacyPatch: "", currentPatch: currentPatch, receiptText: receiptText, goalFiles: goalFiles,
	}
}

type sourceReply struct {
	root   string
	args   []string
	stdout string
	err    error
}

type sourceCalls struct {
	t            *testing.T
	root         string
	snapshot     sourceSnapshot
	replies      []sourceReply
	next         int
	machineCalls int
	goalCalls    int
	wantMachine  bool
	wantGoals    bool
}

func (f *sourceFixture) calls() *sourceCalls {
	return f.callsForSnapshot(f.facts, false)
}

func (f *sourceFixture) callsForSnapshot(s sourceSnapshot, gitFactsOnly bool) *sourceCalls {
	f.t.Helper()
	gitRoot := f.repo
	root := f.root
	current := "metasystem/memory/receipts.log"
	legacy := "metasystem/plans/receipts.log"
	replies := []sourceReply{
		{root: root, args: []string{"rev-parse", "--show-toplevel"}, stdout: gitRoot + "\n"},
		{root: root, args: []string{"rev-parse", "--show-prefix"}, stdout: "metasystem/\n"},
		{root: gitRoot, args: []string{"rev-parse", "--verify", "refs/heads/main"}, stdout: s.mainTip + "\n"},
		{root: gitRoot, args: []string{"log", "--reverse", "--diff-filter=A", "--format=%H", s.mainTip, "--", current}, stdout: s.moveLog},
		{root: gitRoot, args: []string{"rev-parse", s.mainTip + ":" + current}, stdout: s.receiptBlob + "\n"},
		{root: gitRoot, args: []string{"log", "--topo-order", "--reverse", "--format=%x1e%H%x1f%P%x1f%ae%x1f%cI", "--numstat", s.mainTip}, stdout: s.mainLog},
		{root: gitRoot, args: []string{"log", "--reverse", "--format=%x1e%H", "-p", "--unified=0", s.mainTip, "--", legacy}, stdout: s.legacyPatch},
		{root: gitRoot, args: []string{"log", "--reverse", "--format=%x1e%H", "-p", "--unified=0", s.mainTip, "--", current}, stdout: s.currentPatch},
	}
	if !gitFactsOnly {
		replies = append(replies, sourceReply{root: root, args: []string{"cat-file", "-p", s.mainTip + ":" + current}, stdout: s.receiptText})
		accepted := sourceReply{root: root, args: []string{"rev-parse", "--verify", goal.AcceptedRef}, stdout: s.acceptedTip + "\n"}
		if s.acceptedTip == "" {
			accepted.stdout = ""
			accepted.err = errors.New("accepted ref absent")
		}
		replies = append(replies, accepted)
	}
	return &sourceCalls{t: f.t, root: root, snapshot: s, replies: replies, wantMachine: !gitFactsOnly, wantGoals: !gitFactsOnly && s.acceptedTip != ""}
}

func (c *sourceCalls) source() metricsSource {
	return metricsSource{
		resolveMachine: func(root string) (string, error) {
			c.t.Helper()
			c.machineCalls++
			if !c.wantMachine || root != c.root || c.machineCalls != 1 || c.next != 0 {
				c.t.Fatalf("machine resolution out of order: root=%q calls=%d git calls=%d", root, c.machineCalls, c.next)
			}
			return c.snapshot.machine, nil
		},
		gitStdout: func(root string, args ...string) (string, error) {
			c.t.Helper()
			wantMachineCalls := 0
			if c.wantMachine {
				wantMachineCalls = 1
			}
			if c.machineCalls != wantMachineCalls || c.next >= len(c.replies) {
				c.t.Fatalf("unexpected Git call: root=%q args=%q", root, args)
			}
			want := c.replies[c.next]
			if root != want.root || !reflect.DeepEqual(args, want.args) {
				c.t.Fatalf("Git call %d: got root=%q args=%q; want root=%q args=%q", c.next, root, args, want.root, want.args)
			}
			c.next++
			return want.stdout, want.err
		},
		committedGoals: func(root, tip string) (map[string][]byte, error) {
			c.t.Helper()
			c.goalCalls++
			if !c.wantGoals || root != c.root || tip != c.snapshot.acceptedTip || c.goalCalls != 1 || c.next != len(c.replies) {
				c.t.Fatalf("committed goal read out of order: root=%q tip=%q calls=%d git calls=%d", root, tip, c.goalCalls, c.next)
			}
			files := make(map[string][]byte, len(c.snapshot.goalFiles))
			for path, content := range c.snapshot.goalFiles {
				files[path] = append([]byte(nil), content...)
			}
			return files, nil
		},
	}
}

func (c *sourceCalls) assertConsumed() {
	c.t.Helper()
	wantMachine, wantGoals := 0, 0
	if c.wantMachine {
		wantMachine = 1
	}
	if c.wantGoals {
		wantGoals = 1
	}
	if c.machineCalls != wantMachine || c.next != len(c.replies) || c.goalCalls != wantGoals {
		c.t.Fatalf("source calls incomplete: machine=%d Git=%d/%d goals=%d", c.machineCalls, c.next, len(c.replies), c.goalCalls)
	}
}

func (f *sourceFixture) loadGitFacts() (gitFacts, error) {
	f.t.Helper()
	calls := f.callsForSnapshot(f.facts, true)
	facts, err := loadGitFactsWithGit(f.root, calls.source().gitStdout)
	calls.assertConsumed()
	return facts, err
}

func (f *sourceFixture) report(opts Options) (Result, error) {
	f.t.Helper()
	calls := f.calls()
	result, err := reportWithSource(opts, calls.source())
	calls.assertConsumed()
	return result, err
}

func (f *sourceFixture) loadWorld() (world, error) {
	f.t.Helper()
	calls := f.calls()
	w, err := loadWorldWithSource(f.root, calls.source())
	calls.assertConsumed()
	return w, err
}
