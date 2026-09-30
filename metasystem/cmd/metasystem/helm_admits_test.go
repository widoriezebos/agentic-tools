package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/helm"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/landpath"
)

// helmAdmitters are the roots whose person proofs this test binary sends to
// a faked helm admission; every other root's helm declines, so no other test
// changes. main wires the production admitter the same way, once.
var helmAdmitters sync.Map

func init() {
	humanauthority.AtHelm = func(root string, pid int64) (humanauthority.HelmGrant, bool) {
		if admitter, ok := helmAdmitters.Load(resolvedHelmRoot(root)); ok {
			return admitter.(*helmAdmitter).admit(root, pid)
		}
		return humanauthority.HelmGrant{}, false
	}
}

// resolvedHelmRoot is the key helmAdmitters holds a root under.
func resolvedHelmRoot(root string) string {
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		return resolved
	}
	return root
}

// useHelmAdmitter sends the person proofs of root to admitter.
func useHelmAdmitter(root string, admitter *helmAdmitter) {
	helmAdmitters.Store(resolvedHelmRoot(root), admitter)
}

// fakeHelmAdmitter admits from cwd with git answering entry for the work
// tree's own .git (the primary checkout when it is cwd/.git) and the caller
// classified class; yields and the stderr line are collected.
type fakeHelm struct {
	admitter  *helmAdmitter
	stderr    bytes.Buffer
	yields    []helm.Yield
	machinery bool
	machErr   error
}

func newFakeHelm(cwd, entry, class string) *fakeHelm {
	f := &fakeHelm{}
	common := filepath.Join(cwd, ".git")
	f.admitter = &helmAdmitter{owners: helmAdmitOwners{
		cwd: func() (string, error) { return cwd, nil },
		git: func(dir string, args ...string) landpath.GitResult {
			switch {
			case args[len(args)-1] == "--show-toplevel":
				return landpath.GitResult{Stdout: []byte(cwd + "\n")}
			case args[len(args)-2] == "--resolve-git-dir":
				return landpath.GitResult{Stdout: []byte(entry + "\n")}
			}
			return landpath.GitResult{Stdout: []byte(common + "\n")}
		},
		classify:  func(string, int64) (string, error) { return class, nil },
		machinery: func(string, int64) (bool, error) { return f.machinery, f.machErr },
		yield:     func(_ string, y helm.Yield) { f.yields = append(f.yields, y) },
		verb:      func() string { return "goal open" },
		stderr:    &f.stderr,
	}}
	return f
}

func helmSeatDir(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	helmMust(t, os.MkdirAll(filepath.Join(root, ".git", "worktrees", "w"), 0o755))
	return root
}

func takeHelmAt(t *testing.T, root string) {
	t.Helper()
	_, err := helm.Write(root, helm.Record{By: "wido", At: helmNow.Format(time.RFC3339), Reason: "by hand", Checkout: root})
	helmMust(t, err)
}

func TestHelmAdmitsOnlyInThePrimaryCheckoutOfTheHeldSeat(t *testing.T) {
	t.Parallel()
	root := helmSeatDir(t)
	common := filepath.Join(root, ".git")

	inactive := newFakeHelm(root, common, "DELEGATE")
	if _, ok := inactive.admitter.admit(root, 80); ok || len(inactive.yields) != 0 {
		t.Fatal("the helm admitted with no signature")
	}

	takeHelmAt(t, root)
	primary := newFakeHelm(root, common, "DELEGATE")
	grant, ok := primary.admitter.admit(root, 80)
	if !ok || grant != (humanauthority.HelmGrant{By: "wido", Since: helmNow.Format(time.RFC3339), Class: "DELEGATE", Checkout: common}) {
		t.Fatalf("the primary checkout at the helm: %+v %t", grant, ok)
	}
	if len(primary.yields) != 1 || primary.yields[0].Boundary != "person-proof" || primary.yields[0].Gate != "human-proof" ||
		primary.yields[0].Would != "refuse" || primary.yields[0].Subject != "verb=goal open class=DELEGATE cwd="+root {
		t.Fatalf("yields %+v", primary.yields)
	}
	line := "HUMAN AT THE HELM (wido): goal open runs as wido's act\n"
	if primary.stderr.String() != line {
		t.Fatalf("stderr %q", primary.stderr.String())
	}
	// The same act proves again: one yield, one line.
	if again, ok := primary.admitter.admit(root, 80); !ok || again != grant || len(primary.yields) != 1 || strings.Count(primary.stderr.String(), "\n") != 1 {
		t.Fatalf("a second proof of the same act yielded again: %+v", primary.yields)
	}

	linked := newFakeHelm(root, filepath.Join(common, "worktrees", "w"), "DELEGATE")
	if _, ok := linked.admitter.admit(root, 80); ok || len(linked.yields) != 0 || linked.stderr.Len() != 0 {
		t.Fatal("a linked worktree was admitted")
	}

	machinery := newFakeHelm(root, common, "DELEGATE")
	machinery.machinery = true
	if _, ok := machinery.admitter.admit(root, 80); ok || len(machinery.yields) != 0 {
		t.Fatal("a caller under the machinery was admitted (HB-01)")
	}
	unreadable := newFakeHelm(root, common, "DELEGATE")
	unreadable.machErr = errors.New("custody unreadable")
	if _, ok := unreadable.admitter.admit(root, 80); ok || len(unreadable.yields) != 0 {
		t.Fatal("a caller whose ancestry could not be read was admitted")
	}

	// The act's root names another seat (--repo): not admitted (HB-02).
	other := helmSeatDir(t)
	elsewhere := newFakeHelm(root, common, "DELEGATE")
	if _, ok := elsewhere.admitter.admit(other, 80); ok || len(elsewhere.yields) != 0 {
		t.Fatal("an act on another seat was admitted by this seat's helm")
	}
}

func TestHelmAdmitsUnderAMalformedSignature(t *testing.T) {
	t.Parallel()
	root := helmSeatDir(t)
	common := filepath.Join(root, ".git")
	_, err := helm.Write(root, helm.Record{By: "wido"})
	helmMust(t, err)
	fake := newFakeHelm(root, common, "")
	grant, ok := fake.admitter.admit(root, 80)
	if !ok || grant.By != "unknown" || grant.Class != "unavailable" || !strings.HasPrefix(fake.stderr.String(), "HUMAN AT THE HELM (signature unreadable): ") {
		t.Fatalf("malformed signature: %+v %t %q", grant, ok, fake.stderr.String())
	}
}

func TestPublicVerbIsTheWordsBeforeTheFirstFlag(t *testing.T) {
	t.Parallel()
	for args, want := range map[string]string{"goal done G --reason x": "goal done G", "status --json": "status", "goal list --all": "goal list", "": ""} {
		if got := publicVerb(strings.Fields(args)); got != want {
			t.Fatalf("publicVerb(%q) = %q, want %q", args, got, want)
		}
	}
}

// helmGoalBed is the goal CLI bed at the helm: Wido enrolled at tty-1 (pid
// 20), and every person proof walks from bash 80 under codex 70, the agent
// beside him; no agent lineage is named.
func helmGoalBed(t *testing.T) (*goalCLIBed, func() *fakeHelm) {
	t.Helper()
	bed := newGoalCLIBed(t, goalCLISeed{noEnrollment: true})
	bed.lineage = ""
	helmMust(t, os.MkdirAll(filepath.Join(bed.root, ".git"), 0o755))
	_, err := humanauthority.Enroll(bed.root, 20, person(), "Wido", helmNow)
	helmMust(t, err)
	bed.prove = func(root string, _ int64, _ humanauthority.Reader, _, _ string, now time.Time) (humanauthority.Proof, error) {
		return humanauthority.Prove(root, 80, person(), now)
	}
	// Each act is one process: a fresh admitter per act.
	act := func() *fakeHelm {
		fake := newFakeHelm(bed.root, filepath.Join(bed.root, ".git"), "DELEGATE")
		fake.admitter.owners.yield = func(root string, y helm.Yield) { fake.yields = append(fake.yields, y); helm.RecordYield(root, y) }
		useHelmAdmitter(bed.root, fake.admitter)
		return fake
	}
	return bed, act
}

func TestHelmPersonProofYieldsForTheGoalVerbs(t *testing.T) {
	t.Parallel()
	bed, act := helmGoalBed(t)

	fake := act()
	code, out, errOut := bed.public("goal", "done", "ship-widget", "--reason", "x")
	if code == 0 || !strings.Contains(out+errOut, "this terminal isn't enrolled") || len(fake.yields) != 0 {
		t.Fatalf("done before the take: %d %q %q", code, out, errOut)
	}

	takeHelmAt(t, bed.root)
	fake = act()
	gcliLedgerMust(t, bed, "goal", "done", "ship-widget", "--reason", "landed at the helm")
	if record := bed.goalRecord("ship-widget"); !strings.Contains(record, " done actor=human:wido ") {
		t.Fatalf("done at the helm is not the holder's act:\n%s", record)
	}
	if len(fake.yields) != 1 || !strings.Contains(fake.yields[0].Subject, "class=DELEGATE") {
		t.Fatalf("done at the helm yielded %+v", fake.yields)
	}

	fake = act()
	gcliLedgerMust(t, bed, "goal", "open", "helm-opened", "--intent", "Opened at the helm.", "--next", "Continue.",
		"--risk", "severity=1,novelty=1,exposure=1,accumulation=1", "--basis", "fixture risk")
	if record := bed.goalRecord("helm-opened"); !strings.Contains(record, "human:wido") || len(fake.yields) != 1 {
		t.Fatalf("open at the helm: yields %+v\n%s", fake.yields, record)
	}

	fake = act()
	gcliLedgerMust(t, bed, "goal", "approve", "fix-docs", "--budget", "1d/10/720m/1/3")
	if record := bed.goalRecord("fix-docs"); !strings.Contains(record, "human:wido") || len(fake.yields) != 1 {
		t.Fatalf("approve at the helm: yields %+v\n%s", fake.yields, record)
	}
	proofs, _ := filepath.Glob(filepath.Join(bed.root, "artifacts", "agents", "authority", "proofs", "*.json"))
	helmBlocks := 0
	for _, path := range proofs {
		var record struct {
			Action string
			Proof  humanauthority.Proof
		}
		data, err := os.ReadFile(path)
		helmMust(t, err, json.Unmarshal(data, &record))
		if record.Action == "goal approve" && record.Proof.Helm != nil && record.Proof.Helm.By == "wido" && record.Proof.Helm.Class == "DELEGATE" {
			helmBlocks++
		}
	}
	if helmBlocks != 1 {
		t.Fatalf("approve's proof record has no helm block among %v", proofs)
	}
	if lines := yieldLines(t, filepath.Join(bed.root, ".git", "metasystem", "helm-yields.log")); len(lines) != 3 {
		t.Fatalf("helm-yields.log holds %d lines, want 3", len(lines))
	}
}

// TestHelmTakeIgnoresAHelmProofForTheEnrollmentGrade: a take at another
// terminal while at the helm enrolls that terminal from the real walk; the
// helm proof never makes it "proven" (the take is the one proof).
func TestHelmTakeIgnoresAHelmProofForTheEnrollmentGrade(t *testing.T) {
	t.Parallel()
	b := newHelmBed(t, 20, true)
	b.wantTake(nil, 0, "proven", "Wido")
	grant := newFakeHelm(b.root, filepath.Join(b.root, ".git"), "HUMAN")
	useHelmAdmitter(b.inst, grant.admitter)
	if record := b.wantTake(nil, 0, "proven", "Wido"); record.Enrollment != "proven" {
		t.Fatalf("the enrolled terminal's take: %+v", record)
	}
	b.owners.helm.pid = func() int64 { return 60 }
	_, out := b.run("helm", "take", "--reason", "by hand")
	enrollment, err := humanauthority.ReadEnrollment(b.inst)
	if err != nil || enrollment.Generation != 2 || !strings.Contains(out, "enrolled now, as Wido") {
		t.Fatalf("a take at another terminal at the helm did not enroll it from the real walk: %+v %v\n%s", enrollment, err, out)
	}
	// The take never asks the helm to admit: ProveTerminal is its proof.
	b.owners.helm.pid = func() int64 { return 80 }
	before := len(grant.yields)
	if code, out := b.run("helm", "take", "--reason", "by hand"); code != 3 || len(grant.yields) != before {
		t.Fatalf("an agent's take at the helm: %d %d yields\n%s", code, len(grant.yields), out)
	}
}
