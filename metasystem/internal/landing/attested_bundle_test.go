package landing_test

import (
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	goalbranch "github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing"
)

type attestedBundleFixture struct {
	t                  *testing.T
	root, origin, base string
	unit, readCommit   string
}

func bundleGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	command.Env = gittree.ScrubbedEnviron()
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
	return strings.TrimSpace(string(output))
}

func bundleWrite(t *testing.T, root, path string, data []byte) {
	t.Helper()
	path = filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func bundleJSON(t *testing.T, root, path string, value any) {
	t.Helper()
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	bundleWrite(t, root, path, append(data, '\n'))
}

func newAttestedBundleFixture(t *testing.T) attestedBundleFixture {
	t.Helper()
	f := attestedBundleFixture{t: t, root: t.TempDir(), origin: filepath.Join(t.TempDir(), "origin.git")}
	bundleGit(t, f.root, "init", "-q", "-b", "main")
	bundleGit(t, f.root, "config", "user.name", "fixture")
	bundleGit(t, f.root, "config", "user.email", "fixture@example.invalid")
	bundleGit(t, filepath.Dir(f.origin), "init", "-q", "--bare", f.origin)
	bundleGit(t, f.root, "remote", "add", "origin", f.origin)
	bundleWrite(t, f.root, ".gitignore", []byte("artifacts/\n"))
	bundleWrite(t, f.root, "metasystem/product.go", []byte("package product\n"))
	for _, name := range []string{"path-classes.txt", "landing-classes.json", "landing-promotion.json"} {
		data, err := os.ReadFile(filepath.Join("..", "..", "scripts", "agents", name))
		if err != nil {
			t.Fatal(err)
		}
		bundleWrite(t, f.root, "scripts/agents/"+name, data)
	}
	bundleWrite(t, f.root, "memory/receipts.log", []byte("receipt=existing\n"))
	bundleWrite(t, f.root, "memory/rulings.md", []byte("| R-1 | fixture |\n| R-35-m0 | fixture |\n| R-54-m1 | fixture |\n"))
	bundleWrite(t, f.root, "records/narrator-digest.log", []byte("digest=existing\n"))
	bundleWrite(t, f.root, "plans/goals/goal-a.md", goal.RenderFile(&goal.GoalFile{
		Id: "goal-a", State: goal.StateClaimed, Intent: "Fixture ownership.", Origin: goal.OriginMain,
		NextStep: "Land the unit.", OpenedAt: "2026-09-03T08:00:00Z", Revision: 1,
		Claimed: &goal.ClaimRecord{Machine: "m1", Lineage: "lineage", At: "2026-09-03T08:01:00Z", Revision: 1},
		History: []goal.HistoryLine{{At: "2026-09-03T08:01:00Z", Opid: "01ARZ3NDEKTSV4RRFFQ69G5FAW-m1-00000001",
			Verb: "claim", Actor: "m1+lineage", Targets: []string{"goal-a"}, Keep: -1}},
	}))
	bundleGit(t, f.root, "add", ".")
	bundleGit(t, f.root, "commit", "-qm", "base")
	f.base = bundleGit(t, f.root, "rev-parse", "HEAD")
	bundleGit(t, f.root, "push", "-q", "origin", "HEAD:main")
	bundleWrite(t, f.root, "metasystem/product.go", []byte("package product\n// changed\n"))
	bundleGit(t, f.root, "add", "metasystem/product.go")
	var err error
	f.unit, err = goalbranch.CommitStaged(goalbranch.CommitRequest{Repo: f.root, Remote: "origin", EndpointTip: f.base,
		GoalID: "goal-a", Unit: "u1", OpID: "landing-bundle-unit", Kind: goalbranch.Unit, CheckClaim: func() error { return nil }})
	if err != nil {
		t.Fatal(err)
	}
	subject, present, err := dispatch.ComputeReadSubject(dispatch.ReadSubjectRequest{RepoRoot: f.root, Role: "code-critic", Reviews: "commit:" + f.unit})
	if err != nil || !present {
		t.Fatalf("subject present=%v err=%v", present, err)
	}
	rootRecord := map[string]any{"jobId": "critic-boundary", "role": "code-critic", "round": 1, "status": "completed",
		"reviews": "commit:" + f.unit, "goalId": "goal-a", "goalRevision": 1, "chainClosed": true,
		"findingRegister": []any{}, "findingRegisterRound": 1, "findingRegisterSubjectDigest": subject.Digest(),
		"closure": map[string]any{"criticRoot": "critic-boundary", "round": 1, "subject": subject, "mechanism": "clean"}}
	bundleJSON(t, f.root, "artifacts/agents/jobs/critic-boundary.json", rootRecord)
	bundleJSON(t, f.root, "artifacts/agents/critic-boundary/rounds/1/subject.json", subject)
	bundleJSON(t, f.root, "artifacts/agents/critic-boundary/rounds/1/return.json",
		map[string]any{"jobId": "critic-boundary", "round": 1, "reviewedTree": subject.Tree})
	f.readCommit, _, err = goalbranch.CommitRead(goalbranch.CommitReadRequest{Repo: f.root, Remote: "origin", EndpointTip: f.base,
		GoalID: "goal-a", Unit: "u1", OpID: "landing-bundle-read", RootJob: "critic-boundary", CheckClaim: func() error { return nil },
		GateRunID: "boundary-fast", GateTree: subject.Tree})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := goalbranch.Push(goalbranch.PushRequest{Repo: f.root, Remote: "origin", EndpointTip: f.base,
		GoalID: "goal-a", OpID: "landing-bundle-push", CheckClaim: func() error { return nil }}); err != nil {
		t.Fatal(err)
	}
	return f
}

func TestAttestedBoundaryValidatesTheClosureBundle(t *testing.T) {
	f := newAttestedBundleFixture(t)
	clone := filepath.Join(t.TempDir(), "fresh")
	bundleGit(t, filepath.Dir(clone), "clone", "-q", f.origin, clone)
	bundleGit(t, clone, "config", "user.name", "fixture")
	bundleGit(t, clone, "config", "user.email", "fixture@example.invalid")
	bundleGit(t, clone, "fetch", "-q", "origin", "goal/goal-a")
	if _, err := os.Stat(filepath.Join(clone, "artifacts", "agents")); !os.IsNotExist(err) {
		t.Fatalf("fresh clone unexpectedly has job store: %v", err)
	}
	bundleWrite(t, clone, "metasystem/product.go", []byte("package product\n// changed\n"))
	bundleGit(t, clone, "add", "metasystem/product.go")
	candidate, err := (gittree.Workspace{Dir: clone}).StagedTree()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := landing.CreateTestReceipt(clone, candidate, "true", io.Discard, io.Discard); err != nil {
		t.Fatal(err)
	}
	bind := func(commit, snapshot, base, goalID, before, after string) (landing.AttestedUnit, error) {
		bound, err := goalbranch.BindLandedUnit(clone, snapshot, base, goalID, commit, before, after)
		return landing.AttestedUnit(bound), err
	}
	params := landing.ObserveParams{RepoRoot: clone, CandidateTree: candidate, Goal: "goal-a", Actor: "m1+lineage",
		Attested: f.unit, AttestedSnapshot: f.readCommit, AttestedBase: f.base,
		TestReceipt: landing.TestReceiptPath(clone, candidate), BindAttested: bind}
	if got := landing.Observe(params); got.Verdict != "pass" || got.Bar != landing.BarAttested {
		t.Fatalf("portable boundary = %+v", got)
	}

	bundlePath := "metasystem/records/reads/goal-a/" + f.unit + ".closure.json"
	bundleGit(t, clone, "switch", "--quiet", "--detach", f.readCommit)
	data, err := os.ReadFile(filepath.Join(clone, filepath.FromSlash(bundlePath)))
	if err != nil {
		t.Fatal(err)
	}
	bundleWrite(t, clone, bundlePath, append(data, ' '))
	bundleGit(t, clone, "add", bundlePath)
	bundleGit(t, clone, "commit", "-qm", "tampered bundle")
	params.AttestedSnapshot = bundleGit(t, clone, "rev-parse", "HEAD")
	bundleGit(t, clone, "switch", "--quiet", "main")
	bundleWrite(t, clone, "metasystem/product.go", []byte("package product\n// changed\n"))
	if got := landing.Observe(params); got.Verdict == "pass" || !strings.Contains(got.Detail, "fails its digest") {
		t.Fatalf("tampered boundary = %+v", got)
	}
}
