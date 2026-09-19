package branch_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
)

func TestBindLandedUnitRefusesOmittedFold(t *testing.T) {
	t.Parallel()
	f := newBranchFixture(t)
	write(t, f.root, "metasystem/code.go", "base code")
	write(t, f.root, "metasystem/plans/goal-a.md", "base plan")
	git(t, f.root, "add", ".")
	git(t, f.root, "commit", "-qm", "fold base")
	git(t, f.root, "push", "-q", "origin", "HEAD:main")
	f.base = git(t, f.root, "rev-parse", "HEAD")
	stage(t, f, "metasystem/plans/goal-a.md", "folded plan")
	if _, err := branch.CommitStaged(branch.CommitRequest{Repo: f.root, Remote: "origin", EndpointTip: f.base,
		GoalID: "goal-a", OpID: "fold-plan", Kind: branch.Plan, CheckClaim: claimAllowed}); err != nil {
		t.Fatal(err)
	}
	unit := commitUnit(t, f, "u1", "metasystem/code.go", "unit code")
	writeReadJob(t, f.root, "critic-fold", unit, "completed", false)
	readCommit, _, err := branch.CommitRead(branch.CommitReadRequest{Repo: f.root, Remote: "origin", EndpointTip: f.base,
		GoalID: "goal-a", Unit: "u1", OpID: "fold-read", RootJob: "critic-fold", CheckClaim: claimAllowed,
		GateRunID: "fold-fast", GateTree: unitTree(t, f, unit)})
	if err != nil {
		t.Fatal(err)
	}
	git(t, f.root, "switch", "--quiet", "--detach", unit)
	write(t, f.root, "metasystem/plans/goal-a.md", "base plan")
	omitted, err := (gittree.Workspace{Dir: f.root}).Snapshot(unit)
	if err != nil {
		t.Fatal(err)
	}
	baseTree := git(t, f.root, "rev-parse", f.base+"^{tree}")
	if _, err := branch.BindLandedUnit(f.root, readCommit, f.base, "goal-a", unit, baseTree, omitted); err == nil ||
		!strings.Contains(err.Error(), "metasystem/plans/goal-a.md") {
		t.Fatalf("omitted fold = %v", err)
	}
	applied := unitTree(t, f, unit)
	if _, err := branch.BindLandedUnit(f.root, readCommit, f.base, "goal-a", unit, baseTree, applied); err != nil {
		t.Fatalf("applied fold: %v", err)
	}
}

func rewriteLegacyAttestation(t *testing.T, root, path string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
	if err != nil {
		t.Fatal(err)
	}
	var att branch.Attestation
	if err := json.Unmarshal(data, &att); err != nil {
		t.Fatal(err)
	}
	att.Source.ClosureSHA256 = ""
	att.SHA256 = ""
	digestInput, err := json.Marshal(att)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(digestInput)
	att.SHA256 = hex.EncodeToString(sum[:])
	data, err = json.MarshalIndent(att, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	write(t, root, path, string(append(data, '\n')))
}

func TestCriticAttestationSurvivesFreshCloneWithoutJobStore(t *testing.T) {
	t.Parallel()
	f := newBranchFixture(t)
	unit := commitUnit(t, f, "u1", "metasystem/code.go", "one")
	writeReadJob(t, f.root, "critic-portable", unit, "completed", false)
	readCommit, att, err := branch.CommitRead(branch.CommitReadRequest{Repo: f.root, Remote: "origin", EndpointTip: f.base,
		GoalID: "goal-a", Unit: "u1", OpID: "portable-read", RootJob: "critic-portable", CheckClaim: claimAllowed,
		GateRunID: "portable-fast", GateTree: unitTree(t, f, unit)})
	if err != nil || att.Source.ClosureSHA256 == "" {
		t.Fatalf("portable read=%s source=%+v err=%v", readCommit, att.Source, err)
	}
	if _, err := branch.Push(pushRequest(f, "portable-push")); err != nil {
		t.Fatal(err)
	}
	clone := filepath.Join(t.TempDir(), "fresh")
	git(t, filepath.Dir(clone), "clone", "-q", "--branch", "goal/goal-a", f.origin, clone)
	if _, err := os.Stat(filepath.Join(clone, "artifacts", "agents")); !os.IsNotExist(err) {
		t.Fatalf("fresh clone unexpectedly has job store: %v", err)
	}
	baseTree := git(t, clone, "rev-parse", f.base+"^{tree}")
	unitTree := git(t, clone, "rev-parse", unit+"^{tree}")
	if _, err := branch.BindLandedUnit(clone, readCommit, f.base, "goal-a", unit, baseTree, unitTree); err != nil {
		t.Fatalf("portable closure in fresh clone: %v", err)
	}

	bundlePath := "metasystem/records/reads/goal-a/" + unit + ".closure.json"
	bundle, err := os.ReadFile(filepath.Join(clone, filepath.FromSlash(bundlePath)))
	if err != nil {
		t.Fatal(err)
	}
	write(t, clone, bundlePath, string(append(bundle, ' ')))
	git(t, clone, "add", bundlePath)
	git(t, clone, "commit", "-qm", "tampered closure snapshot")
	tampered := git(t, clone, "rev-parse", "HEAD")
	if _, err := branch.ValidateAttestationAt(clone, tampered, f.base, "goal-a", "u1", unit); err == nil || !strings.Contains(err.Error(), "fails its digest") {
		t.Fatalf("tampered closure = %v", err)
	}
	for _, unsafe := range []string{"../escape.json", "/absolute.json", "C:/absolute.json"} {
		git(t, clone, "switch", "--quiet", "--detach", readCommit)
		var bundleDocument struct {
			SchemaVersion int               `json:"schemaVersion"`
			RootJob       string            `json:"rootJob"`
			Files         map[string]string `json:"files"`
		}
		if err := json.Unmarshal(bundle, &bundleDocument); err != nil {
			t.Fatal(err)
		}
		bundleDocument.Files[unsafe] = "{}\n"
		maliciousBundle, err := json.MarshalIndent(bundleDocument, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		maliciousBundle = append(maliciousBundle, '\n')
		bundleSum := sha256.Sum256(maliciousBundle)
		attestationData, err := os.ReadFile(filepath.Join(clone, filepath.FromSlash("metasystem/records/reads/goal-a/"+unit+".json")))
		if err != nil {
			t.Fatal(err)
		}
		var maliciousAttestation branch.Attestation
		if err := json.Unmarshal(attestationData, &maliciousAttestation); err != nil {
			t.Fatal(err)
		}
		maliciousAttestation.Source.ClosureSHA256 = hex.EncodeToString(bundleSum[:])
		maliciousAttestation.SHA256 = ""
		attestationDigestInput, _ := json.Marshal(maliciousAttestation)
		attestationSum := sha256.Sum256(attestationDigestInput)
		maliciousAttestation.SHA256 = hex.EncodeToString(attestationSum[:])
		maliciousData, _ := json.MarshalIndent(maliciousAttestation, "", "  ")
		write(t, clone, bundlePath, string(maliciousBundle))
		write(t, clone, "metasystem/records/reads/goal-a/"+unit+".json", string(append(maliciousData, '\n')))
		git(t, clone, "add", "metasystem/records/reads/goal-a")
		git(t, clone, "commit", "-qm", "unsafe closure snapshot")
		unsafeSnapshot := git(t, clone, "rev-parse", "HEAD")
		if _, err := branch.ValidateAttestationAt(clone, unsafeSnapshot, f.base, "goal-a", "u1", unit); err == nil || !strings.Contains(err.Error(), "unsafe path") {
			t.Fatalf("unsafe closure path %q = %v", unsafe, err)
		}
	}

	attestationPath := "metasystem/records/reads/goal-a/" + unit + ".json"
	rewriteLegacyAttestation(t, f.root, attestationPath)
	git(t, f.root, "add", attestationPath)
	git(t, f.root, "rm", "-q", bundlePath)
	git(t, f.root, "commit", "-qm", "legacy attestation snapshot")
	legacy := git(t, f.root, "rev-parse", "HEAD")
	if _, err := branch.ValidateAttestationAt(f.root, legacy, f.base, "goal-a", "u1", unit); err != nil {
		t.Fatalf("legacy attestation with local job store: %v", err)
	}
	git(t, f.root, "push", "-q", "origin", legacy+":refs/heads/legacy-attestation")
	git(t, clone, "fetch", "-q", "origin", "legacy-attestation")
	if _, err := branch.ValidateAttestationAt(clone, legacy, f.base, "goal-a", "u1", unit); err == nil || !strings.Contains(err.Error(), "code-critic root") {
		t.Fatalf("legacy attestation without local job store = %v", err)
	}
}

func writeReadJob(t *testing.T, root, job, commit, status string, openFinding bool) {
	t.Helper()
	subject, present, err := dispatch.ComputeReadSubject(dispatch.ReadSubjectRequest{RepoRoot: root, Role: "code-critic", Reviews: "commit:" + commit})
	if err != nil || !present {
		t.Fatalf("subject present=%v err=%v", present, err)
	}
	register := []any{}
	if openFinding {
		register = append(register, map[string]any{"findingId": "defect-a", "critic": job, "rigorClass": "bounded",
			"factsDigest": strings.Repeat("0", 64), "status": "open", "evidenceDigest": strings.Repeat("1", 64), "multiplicity": 1})
	}
	record := map[string]any{"jobId": job, "role": "code-critic", "round": 1, "status": status,
		"reviews": "commit:" + commit, "goalId": "goal-a", "goalRevision": 1, "findingRegister": register,
		"findingRegisterRound": 1, "findingRegisterSubjectDigest": subject.Digest()}
	if status == "completed" {
		record["chainClosed"] = true
		record["closure"] = map[string]any{"criticRoot": job, "round": 1, "subject": subject, "mechanism": "clean"}
	}
	writeJSONFixture(t, root, "artifacts/agents/jobs/"+job+".json", record)
	if status == "completed" {
		writeJSONFixture(t, root, "artifacts/agents/"+job+"/rounds/1/subject.json", subject)
		writeJSONFixture(t, root, "artifacts/agents/"+job+"/rounds/1/return.json", map[string]any{"jobId": job, "round": 1, "reviewedTree": subject.Tree})
	}
}

func TestGoalBranchReadRunsGateDispatchesAndCollectsClosedCritic(t *testing.T) {
	t.Parallel()
	f := newBranchFixture(t)
	unit := commitUnit(t, f, "u1", "metasystem/code.go", "one")
	gateCalls, delegateCalls := 0, 0
	var detached string
	request := branch.BranchReadRequest{Repo: f.root, Remote: "origin", EndpointTip: f.base, BranchTip: unit,
		GoalID: "goal-a", UnitCommit: unit, CheckClaim: claimAllowed,
		Gate: func(worktree string) (string, error) {
			gateCalls++
			detached = worktree
			if got := git(t, worktree, "rev-parse", "HEAD^{tree}"); got != unitTree(t, f, unit) {
				t.Fatalf("gate tree=%s", got)
			}
			return "go gate: fast mode passed", nil
		},
		Delegate: func(brief, goalID, commit string) (string, error) {
			delegateCalls++
			body, err := os.ReadFile(brief)
			if err != nil || goalID != "goal-a" || commit != unit || !strings.Contains(string(body), "git diff "+unit+"^ "+unit) ||
				!strings.Contains(string(body), "goals-live-on-branches-design.md") {
				t.Fatalf("brief=%q goal=%s commit=%s err=%v", body, goalID, commit, err)
			}
			writeReadJob(t, f.root, "critic-read", unit, "running", false)
			return "critic-read", nil
		},
		NewID: func(string) (string, error) { return "fast-unit-tree", nil },
	}
	result, err := branch.RunBranchRead(request)
	if err != nil || result.State != "dispatched" || result.RootJob != "critic-read" || gateCalls != 1 || delegateCalls != 1 {
		t.Fatalf("dispatch result=%+v gate=%d delegate=%d err=%v", result, gateCalls, delegateCalls, err)
	}
	if _, err := os.Stat(detached); !os.IsNotExist(err) {
		t.Fatalf("temporary worktree still exists: %v", err)
	}
	request.Collect = true
	result, err = branch.RunBranchRead(request)
	if err != nil || result.State != "open" || gateCalls != 1 || delegateCalls != 1 {
		t.Fatalf("open result=%+v gate=%d delegate=%d err=%v", result, gateCalls, delegateCalls, err)
	}
	writeReadJob(t, f.root, "critic-read", unit, "completed", false)
	result, err = branch.RunBranchRead(request)
	if err != nil || result.State != "collected" || result.AttestationCommit == "" || gateCalls != 1 || delegateCalls != 1 {
		t.Fatalf("collect result=%+v gate=%d delegate=%d err=%v", result, gateCalls, delegateCalls, err)
	}
	attestation, err := branch.ValidateAttestation(f.root, f.base, "goal-a", "u1", unit)
	if err != nil || attestation.Source.RootJob != "critic-read" || attestation.Gate.RunID != "fast-unit-tree" {
		t.Fatalf("attestation=%+v err=%v", attestation, err)
	}
	bound, err := branch.BindLandedUnit(f.root, result.AttestationCommit, f.base, "goal-a", unit,
		git(t, f.root, "rev-parse", f.base+"^{tree}"), unitTree(t, f, unit))
	if err != nil || bound.CriticRoot != "critic-read" || bound.GoalRevision != 1 || bound.Digest != attestation.Subject.UnitDigest {
		t.Fatalf("bound=%+v err=%v", bound, err)
	}
	write(t, f.root, "extra.txt", "extra")
	extraTree, err := (gittree.Workspace{Dir: f.root}).Snapshot(result.AttestationCommit)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := branch.BindLandedUnit(f.root, result.AttestationCommit, f.base, "goal-a", unit,
		git(t, f.root, "rev-parse", f.base+"^{tree}"), extraTree); err == nil || !strings.Contains(err.Error(), "does not match attested digest") {
		t.Fatalf("extra candidate path err=%v", err)
	}
	nested := filepath.Join(f.root, "metasystem")
	writeReadJob(t, nested, "critic-read", unit, "completed", false)
	nestedTree := git(t, f.root, "rev-parse", unit+":metasystem")
	bound, err = branch.BindLandedUnit(nested, result.AttestationCommit, f.base, "goal-a", unit,
		"4b825dc642cb6eb9a060e54bf8d69288fbee4904", nestedTree)
	if err != nil || bound.Digest != attestation.Subject.UnitDigest {
		t.Fatalf("nested bound=%+v err=%v", bound, err)
	}
}

func TestGoalBranchReadRedGateAndUncleanClosureDispatchNothingFurther(t *testing.T) {
	t.Parallel()
	t.Run("red gate", func(t *testing.T) {
		t.Parallel()
		f := newBranchFixture(t)
		unit := commitUnit(t, f, "u1", "metasystem/code.go", "one")
		delegates := 0
		_, err := branch.RunBranchRead(branch.BranchReadRequest{Repo: f.root, Remote: "origin", EndpointTip: f.base,
			BranchTip: unit, GoalID: "goal-a", UnitCommit: unit, CheckClaim: claimAllowed,
			Gate:     func(string) (string, error) { return "go gate: staticcheck red", errors.New("exit 1") },
			Delegate: func(string, string, string) (string, error) { delegates++; return "critic", nil }})
		if err == nil || !strings.Contains(err.Error(), branch.ReadUngatedCode) || !strings.Contains(err.Error(), "staticcheck red") || delegates != 0 {
			t.Fatalf("red gate err=%v delegates=%d", err, delegates)
		}
	})

	t.Run("open finding", func(t *testing.T) {
		t.Parallel()
		f := newBranchFixture(t)
		unit := commitUnit(t, f, "u1", "metasystem/code.go", "one")
		request := branch.BranchReadRequest{Repo: f.root, Remote: "origin", EndpointTip: f.base, BranchTip: unit,
			GoalID: "goal-a", UnitCommit: unit, CheckClaim: claimAllowed,
			Gate: func(string) (string, error) { return "green", nil }, NewID: func(string) (string, error) { return "fast-open", nil },
			Delegate: func(string, string, string) (string, error) {
				writeReadJob(t, f.root, "critic-open", unit, "completed", true)
				return "critic-open", nil
			}}
		if _, err := branch.RunBranchRead(request); err != nil {
			t.Fatal(err)
		}
		request.Collect = true
		if _, err := branch.RunBranchRead(request); err == nil || !strings.Contains(err.Error(), "clean") {
			t.Fatalf("open finding collect=%v", err)
		}
		path := filepath.Join(f.root, "metasystem", "records", "reads", "goal-a", unit+".json")
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("unclean closure wrote an attestation: %v", err)
		}
	})
}
