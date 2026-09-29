package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/helm"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

const (
	helmSHA1  = "1111111aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	helmSHA2  = "2222222bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	helmTree1 = "3333333ccccccccccccccccccccccccccccccccc"
	helmTree2 = "4444444ddddddddddddddddddddddddddddddddd"
)

// returnBed is a helm bed at the helm with two commits admitted on branch
// and one act the person proof admitted; every catch-up seam is faked and
// records its calls.
type returnBed struct {
	*helmBed
	asked   []string
	answers []string
	gitArgs [][]string
	done    []struct{ id, by, reason string }
	proofs  []humanauthority.Proof
	forces  []bool
	// refuse, when set, is the plain conclusion's refusal; a forced one confirms.
	refuse string
	reads  [][2]string
	gitErr error
	fail   bool
}

func newReturnBed(t *testing.T, branch string) *returnBed {
	t.Helper()
	b := &returnBed{helmBed: newHelmBed(t, 20, true)}
	b.wantTake(nil, 0, "proven", "Wido")
	engine := filepath.Join(b.inst, "bin", "metasystem")
	helmMust(t, os.MkdirAll(filepath.Dir(engine), 0o755), testexec.WriteFile(engine, []byte("#!/bin/sh\nexit 0\n"), 0o755))
	for _, tree := range []string{helmTree1, helmTree2} {
		helm.RecordYield(b.root, helm.Yield{At: helmNow.Add(time.Minute), Boundary: "pre-commit", Gate: "wrapper-fence", Would: "refuse",
			Subject: "branch=" + branch + " tree=" + tree + " class=DELEGATE"})
	}
	helm.RecordYield(b.root, helm.Yield{At: helmNow.Add(time.Minute), Boundary: "person-proof", Gate: "human-proof", Would: "refuse",
		Subject: "verb=goal open g1 class=DELEGATE cwd=" + b.root})
	h := &b.owners.helm
	h.git = func(dir string, args ...string) (string, error) {
		b.gitArgs = append(b.gitArgs, args)
		if b.gitErr != nil {
			return "", b.gitErr
		}
		switch args[0] {
		case "log":
			return helmSHA2 + "\x1f" + helmTree2 + "\x1fsecond subject\n" + helmSHA1 + "\x1f" + helmTree1 + "\x1ffirst subject\n", nil
		case "merge-base":
			if args[2] == helmSHA1 {
				return "", nil
			}
			return "", errors.New("exit status 1")
		case "diff":
			return "diff --git a/x b/x\n", nil
		case "rev-parse":
			if args[1] == "--verify" {
				return helmSHA1 + "\n", nil
			}
		}
		return "", errors.New("no upstream")
	}
	h.stdinTerminal = func() bool { return true }
	h.ask = func(prompt string) (string, bool) {
		b.asked = append(b.asked, prompt)
		if len(b.answers) == 0 {
			return "", false
		}
		answer := b.answers[0]
		b.answers = b.answers[1:]
		return answer, true
	}
	h.holder = func(string) (lease.CurrentHolderView, error) {
		return lease.CurrentHolderView{}, errors.New("no lease")
	}
	h.done = func(_ *intentInvocation, id, by, reason string, proof humanauthority.Proof, force bool) intentResult {
		b.done = append(b.done, struct{ id, by, reason string }{id, by, reason})
		b.proofs = append(b.proofs, proof)
		b.forces = append(b.forces, force)
		if b.refuse != "" && !force {
			return intentResult{Outcome: intentRefused, Summary: b.refuse}
		}
		if b.fail {
			return intentResult{Outcome: intentRefused, Summary: "g1 has an open read item"}
		}
		return intentResult{Outcome: intentConfirmed, Summary: "confirmed"}
	}
	h.read = func(_ *intentInvocation, patch, brief string) intentResult {
		b.reads = append(b.reads, [2]string{patch, brief})
		if b.fail {
			return intentResult{Outcome: intentFailed, Summary: "the readers cannot start"}
		}
		return intentResult{Outcome: intentConfirmed, Summary: "read:r1 requested; metasystem work status shows it"}
	}
	h.recover = func(processScope) string {
		if b.fail {
			return "supervision re-arms at the next turn end (the Stop hook arms it)"
		}
		return "supervision: recovered"
	}
	h.fence = func(string) error {
		if b.fail {
			return errors.New("no executable engine")
		}
		return nil
	}
	return b
}

func TestHelmReturnReadsBackAndPrintsTheCommandsWithoutATerminal(t *testing.T) {
	t.Parallel()
	b := newReturnBed(t, "refs/heads/main")
	b.owners.helm.stdinTerminal = func() bool { return false }
	out := b.wantReturn(0, "returned the helm Wido held: by hand")
	for _, want := range []string{
		"commits at the helm on main: 1111111 first subject (on origin/main), 2222222 second subject (not pushed)\n",
		"acts at the helm: goal open g1 (DELEGATE)\n",
		"every goal stays open; to conclude one: metasystem goal done G --reason 'landed at the helm by Wido: 2 commits 1111111, 2222222' --by Wido\n",
		"to ask independent readers for feedback: metasystem work review --patch ",
		"the ledger hook is enrolled\n", "supervision: recovered\n",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("return lacks %q:\n%s", want, out)
		}
	}
	if !strings.HasSuffix(out, "the machinery is at the helm again\n") || !strings.Contains(out, " --brief ") {
		t.Fatalf("return does not end with the machinery or lacks the brief:\n%s", out)
	}
	if len(b.asked) != 0 || len(b.done) != 0 || len(b.reads) != 0 || helm.Active(b.root).Active {
		t.Fatalf("without a terminal: asked %q, done %v, reads %v", b.asked, b.done, b.reads)
	}
}

func TestHelmReturnConcludesTheGoalAnAnswerNames(t *testing.T) {
	t.Parallel()
	b := newReturnBed(t, "refs/heads/goal/g1")
	b.answers = []string{"y", "", "n"}
	out := b.wantReturn(0, "goal done g1: confirmed")
	if len(b.done) != 1 || b.done[0] != (struct{ id, by, reason string }{"g1", "Wido", "landed at the helm by Wido: 2 commits 1111111, 2222222"}) {
		t.Fatalf("done calls %+v", b.done)
	}
	inst, err := filepath.EvalSymlinks(b.inst)
	helmMust(t, err)
	if proof := b.proofs[0]; proof.Helm == nil || proof.Helm.By != "Wido" || !proof.ValidFor(inst) || proof.ObservedTerminalID() != "tty-1" {
		t.Fatalf("the conclusion's proof %+v", proof)
	}
	if len(b.reads) != 0 || len(b.asked) != 3 || b.asked[0] != "Conclude goal g1 with these commits? [y/N] " ||
		!strings.HasPrefix(b.asked[1], "Conclusion [Enter: landed at the helm by Wido: 2 commits") ||
		b.asked[2] != "Ask independent readers for feedback on these commits? [y/N] " {
		t.Fatalf("asked %q, reads %v", b.asked, b.reads)
	}
	// The readback came before the first question.
	if !strings.HasPrefix(out, "returned the helm Wido held: by hand\n") || !strings.HasSuffix(out, "the machinery is at the helm again\n") {
		t.Fatalf("return output order:\n%s", out)
	}
}

func TestHelmReturnRequestsTheReadAnAnswerNames(t *testing.T) {
	t.Parallel()
	b := newReturnBed(t, "refs/heads/goal/g1")
	b.answers = []string{"n", "y"}
	b.wantReturn(0, "read: read:r1 requested; metasystem work status shows it")
	if len(b.done) != 0 || len(b.reads) != 1 {
		t.Fatalf("done %v, reads %v", b.done, b.reads)
	}
	patch, brief := b.reads[0][0], b.reads[0][1]
	patchText, patchErr := os.ReadFile(patch)
	briefText, briefErr := os.ReadFile(brief)
	if patchErr != nil || briefErr != nil || !strings.Contains(string(patchText), "diff --git") || strings.Count(string(briefText), "\n") != 3 ||
		!strings.Contains(string(briefText), "1111111 first subject") || !strings.Contains(string(briefText), "2222222 second subject") ||
		!strings.Contains(string(briefText), "feedback only; no goal is bound") {
		t.Fatalf("patch %v %q, brief %v %q", patchErr, patchText, briefErr, briefText)
	}
}

func TestHelmReturnSucceedsWhateverEachStepSays(t *testing.T) {
	t.Parallel()
	b := newReturnBed(t, "refs/heads/goal/g1")
	b.fail = true
	b.answers = []string{"y", "", "y"}
	out := b.wantReturn(0, "goal done g1: refused: g1 has an open read item")
	for _, want := range []string{"read: failed: the readers cannot start\n", "the ledger hook: no executable engine\n",
		"supervision re-arms at the next turn end (the Stop hook arms it)\n", "the machinery is at the helm again\n"} {
		if !strings.Contains(out, want) {
			t.Fatalf("return lacks %q:\n%s", want, out)
		}
	}
	if helm.Active(b.root).Active {
		t.Fatal("a failing catch-up left the helm taken")
	}

	// Git itself failing: one line, no questions about commits nobody can see.
	g := newReturnBed(t, "refs/heads/goal/g1")
	g.gitErr = errors.New("fatal: not a git repository")
	g.answers = []string{"n"}
	out = g.wantReturn(0, "commits at the helm on goal/g1: unavailable: fatal: not a git repository")
	if len(g.reads) != 0 || len(g.asked) != 1 {
		t.Fatalf("with git failing: asked %q reads %v\n%s", g.asked, g.reads, out)
	}
	g.wantReturn(0, "the machinery is at the helm; nothing to return")
}
