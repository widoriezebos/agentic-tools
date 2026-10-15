package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/enginebuild"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy/adapter"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/verbresult"
)

// Planning runs the real CLI against the shipped contract and copied ledger
// pages. Git supplies the exact candidate tree and installation prefix.
func TestLandingMainMoveRealContractPlanGitAdapter(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	install := filepath.Join(root, "metasystem")
	for _, path := range []string{"testing.json", "go.mod", "plans/README.md", "plans/goals/backlog.md", "plans/goals/lane-first-run-fixes.md", "plans/goals/lane-rehearsal-before-landing.md"} {
		writeTestingFixtureFile(t, filepath.Join(install, path), mustReadTestingFixtureFile(t, filepath.Join("..", "..", path)), 0600)
	}
	writeTestingFixtureFile(t, filepath.Join(install, "metasystem.conf"), []byte("testing.contract=testing.json\n"), 0600)
	testingFixtureGit(t, root, "init", "-q", "-b", "main")
	testingFixtureGit(t, root, "add", ".")
	testingFixtureGit(t, root, "-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "real contract and ledger")
	contract, err := testpolicy.Load(filepath.Join(install, "testing.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"metasystem/plans/goals/lane-first-run-fixes.md", "metasystem/plans/goals/backlog.md", "metasystem/plans/README.md"} {
		affected, err := testpolicy.Affected(contract, []string{path})
		if err != nil || len(affected.Groups) == 0 {
			t.Fatalf("real input %s is not covered: %+v %v", path, affected, err)
		}
	}
	head := strings.TrimSpace(testingFixtureGit(t, root, "rev-parse", "HEAD"))
	testingFixtureGit(t, root, "update-ref", "refs/remotes/origin/main", head)
	testingFixtureGit(t, root, "config", "metasystem.steward.landing-ref", "refs/remotes/origin/main")
	engine := filepath.Join(t.TempDir(), "metasystem")
	build := testenv.GoContext(t.Context(), "build", "-buildvcs=false", "-ldflags", enginebuild.StampLinkerFlags(head), "-o", engine, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build real planning engine: %v %s", err, output)
	}
	digest, err := fileSHA256(engine)
	if err != nil {
		t.Fatal(err)
	}
	canonicalInstall, err := canonicalProofRoot(install)
	if err != nil {
		t.Fatal(err)
	}
	canonicalEngine, err := canonicalPath(engine)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(steward.RepoIdentityPath(install)), 0700); err != nil {
		t.Fatal(err)
	}
	if err := steward.MintIdentity(steward.RepoIdentityPath(install), steward.InstallIdentity{
		RepoIdentity: canonicalInstall, Generation: 1, InstallPath: canonicalEngine, InstallDigest: "sha256:" + digest,
		MintedAt: "2026-10-11T10:45:00Z", Enrollment: steward.EnrollmentFixture, MintedBy: "machine-rebuild",
		EngineBuild: head[:12], LandedCommit: head, LandingRef: "refs/remotes/origin/main",
	}); err != nil {
		t.Fatal(err)
	}
	command := exec.CommandContext(t.Context(), engine, "test", "plan", "--root", install, "--purpose", "diagnostic", "--mode", "canary", "--groups", "shipped-installation-standard", "--policy-child", "--json")
	command.Env = fixtureCommandEnvironment(t)
	command.Dir = root
	var stderr bytes.Buffer
	command.Stderr = &stderr
	result, err := verbresult.Run(command, "test plan")
	var output testrun.PlanOutput
	if err != nil || result.Outcome != verbresult.Confirmed || result.DecodeData(&output) != nil || !slices.Contains(output.Plan.SelectedGroups, "shipped-installation-standard") || output.CandidateTree == "" || output.ContractDigest == "" {
		t.Fatalf("real plan: %+v error=%v stderr=%s", result, err, &stderr)
	}
	t.Logf("real plan selects %v for tree %s", output.Plan.SelectedGroups, output.CandidateTree)
	for _, path := range []string{"plans/goals/lane-first-run-fixes.md", "plans/README.md"} {
		original := mustReadTestingFixtureFile(t, filepath.Join(install, path))
		writeTestingFixtureFile(t, filepath.Join(install, path), append(original, '\n'), 0600)
		testingFixtureGit(t, root, "add", "metasystem/"+path)
		command := exec.CommandContext(t.Context(), engine, "test", "plan", "--root", install, "--purpose", "diagnostic", "--mode", "auto", "--policy-child", "--json")
		command.Env, command.Dir = fixtureCommandEnvironment(t), root
		var stderr bytes.Buffer
		command.Stderr = &stderr
		result, err := verbresult.Run(command, "test plan")
		var selected testrun.PlanOutput
		if err != nil || result.Outcome != verbresult.Confirmed || result.DecodeData(&selected) != nil || !slices.Contains(selected.Plan.SelectedGroups, "shipped-installation-standard") {
			t.Fatalf("real automatic plan for %s: %+v error=%v stderr=%s", path, result, err, &stderr)
		}
		t.Logf("real automatic plan for %s selects %v", path, selected.Plan.SelectedGroups)
	}

}

// Only real Git with the engine's guard installed can prove that a record
// fold publishes without a porcelain commit and leaves no pending merge.
func TestLandingMainMoveGuardHookGitAdapter(t *testing.T) {
	t.Parallel()
	engine := intentTestEngine(t)
	for _, conflict := range []bool{false, true} {
		t.Run(map[bool]string{false: "lands", true: "conflict"}[conflict], func(t *testing.T) {
			t.Parallel()
			b := newBatchVerbBed(t, "1")
			mainMoveAffectedChecks(t, b, nil)
			sha := b.seat(t, "a")
			path := "metasystem/plans/goals/lane-first-run-fixes.md"
			if conflict {
				seat := filepath.Join(filepath.Dir(b.checkout), "seat-a")
				writeTestingFixtureFile(t, filepath.Join(seat, path), []byte("batch ledger update\n"), 0600)
				b.git(t, seat, "add", path)
				b.git(t, seat, "commit", "-qm", "batch ledger update")
				b.git(t, seat, "push", "--quiet", "origin", "goal/a")
				sha = b.git(t, seat, "rev-parse", "HEAD")
				if _, _, err := plain.HandIn(b.installation, plain.Line{Goal: "a", Branch: "goal/a", SHA: sha, Seat: "seat-a"}); err != nil {
					t.Fatal(err)
				}
			}
			b.success(t, "landing", "run")
			b.assemble(t, sha)
			b.success(t, "landing", "prove", "--gate", "--wait")
			b.success(t, "landing", "prove", "--wait")
			proven := b.git(t, b.checkout, "rev-parse", "HEAD")
			writer := t.TempDir()
			b.git(t, writer, "clone", "--quiet", b.origin, ".")
			ledger := []byte("# lane-first-run-fixes\n\n- State: open\n- Intent: check records\n")
			writeTestingFixtureFile(t, filepath.Join(writer, path), ledger, 0600)
			b.git(t, writer, "add", path)
			b.git(t, writer, "commit", "-qm", "goal slice-start")
			b.git(t, writer, "push", "--quiet", "origin", "main")
			main := b.git(t, writer, "rev-parse", "HEAD")
			hook := "#!/bin/sh\nexec " + shellQuote(engine) + " internal pre-commit --root " + shellQuote(b.installation) + "\n"
			if err := testexec.WriteFile(filepath.Join(b.checkout, ".git", "hooks", "pre-commit"), []byte(hook), 0755); err != nil {
				t.Fatal(err)
			}
			checks := b.executions(t)
			code, output := b.run(t, "landing", "push")
			if !conflict {
				if code == 0 || !strings.Contains(output, plain.CodeUnproven) || b.git(t, writer, "ls-remote", "origin", "refs/heads/main") != main+"\trefs/heads/main" {
					t.Fatalf("unchecked fold published: %d %s", code, output)
				}
				b.success(t, "landing", "prove", "--wait")
				code, output = b.run(t, "landing", "push")
			}
			if conflict {
				if code == 0 || !strings.Contains(output, plain.CodeNotFastForward) || b.git(t, b.checkout, "rev-parse", "HEAD") != proven || b.git(t, writer, "ls-remote", "origin", "refs/heads/main") != main+"\trefs/heads/main" {
					t.Fatalf("conflicted fold: exit=%d output=%s", code, output)
				}
			} else {
				push, found, err := plain.LastPush(b.installation)
				checked, _, _ := plain.LastResult(b.installation)
				if checked.Scope != "scoped" || !slices.Equal(checked.Ran, []string{"records"}) {
					t.Fatalf("fold affected checks: %+v", checked)
				}
				if code != 0 || err != nil || !found || !slices.Equal(push.FoldedCommits, []string{main}) || b.git(t, b.checkout, "show", "-s", "--format=%P", "HEAD") != proven+" "+main || b.git(t, writer, "ls-remote", "origin", "refs/heads/main") != push.Commit+"\trefs/heads/main" {
					t.Fatalf("guarded fold: exit=%d output=%s push=%+v error=%v", code, output, push, err)
				}
			}
			if staged := b.git(t, b.checkout, "diff", "--cached", "--name-only"); staged != "" {
				t.Fatalf("fold left staged paths: %s", staged)
			}
			if _, err := os.Stat(filepath.Join(b.checkout, ".git", "MERGE_HEAD")); !os.IsNotExist(err) {
				t.Fatalf("fold left merge state: %v", err)
			}
			wantChecks := checks
			if !conflict {
				wantChecks++
			}
			if b.executions(t) != wantChecks {
				t.Fatal("fold did not execute exactly its affected checks")
			}
			t.Logf("guarded landing push exit=%d: %s", code, output)
		})
	}
}

// The public status, proof and push handlers drive a real scratch origin;
// only proof execution and agent launch use the existing fixture seams.
func TestLandingMainMovePublicCommandsGitAdapter(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"records-only", "code"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			b := newBatchVerbBed(t, "1")
			mainMoveAffectedChecks(t, b, nil)
			sha := b.seat(t, "a")
			b.success(t, "landing", "run")
			b.assemble(t, sha)
			b.success(t, "landing", "prove", "--gate", "--wait")
			b.success(t, "landing", "prove", "--wait")
			proven := b.git(t, b.checkout, "rev-parse", "HEAD")
			writer := t.TempDir()
			b.git(t, writer, "clone", "--quiet", b.origin, ".")
			path := "plans/goals/lane-first-run-fixes.md"
			if kind == "code" {
				path = "plans/README.md"
			}
			writeTestingFixtureFile(t, filepath.Join(writer, "metasystem", path), []byte("# fixture main change\n"), 0600)
			b.git(t, writer, "add", ".")
			b.git(t, writer, "commit", "-qm", "main moved")
			b.git(t, writer, "push", "--quiet", "origin", "main")
			newMain := b.git(t, writer, "rev-parse", "HEAD")
			b.git(t, b.checkout, "fetch", "--quiet", "origin")
			code, text := b.run(t, "landing", "status")
			var status struct{ Data plain.Status }
			if err := json.Unmarshal([]byte(text), &status); err != nil || code != 0 || status.Data.MainMove.Kind != kind || status.Data.MainMove.Count != 1 {
				t.Fatalf("public status: exit=%d output=%s error=%v", code, text, err)
			}
			checks := b.executions(t)
			code, text = b.run(t, "landing", "push")
			if kind == "records-only" {
				if code == 0 || !strings.Contains(text, plain.CodeUnproven) {
					t.Fatalf("unchecked fold published: %d %s", code, text)
				}
				b.success(t, "landing", "prove", "--wait")
				code, text = b.run(t, "landing", "push")
			}
			if kind == "code" {
				if code == 0 || !strings.Contains(text, plain.CodeNotFastForward) || b.git(t, b.checkout, "rev-parse", "HEAD") != proven {
					t.Fatalf("code move bypassed rebuild: exit=%d output=%s", code, text)
				}
			} else {
				push, ok, err := plain.LastPush(b.installation)
				checked, _, _ := plain.LastResult(b.installation)
				if checked.Scope != "scoped" || !slices.Equal(checked.Ran, []string{"records"}) {
					t.Fatalf("public fold affected checks: %+v", checked)
				}
				if code != 0 || err != nil || !ok || !slices.Equal(push.FoldedCommits, []string{newMain}) || b.executions(t) != checks+1 || b.git(t, b.checkout, "rev-parse", "HEAD^1") != proven {
					t.Fatalf("public folded push: exit=%d output=%s push=%+v error=%v", code, text, push, err)
				}
			}
			t.Logf("landing status reports %s; landing push exit=%d, output=%s", kind, code, text)
		})
	}
}

// The scratch contract declares records separately from executable inputs.
func mainMoveAffectedChecks(t *testing.T, b *batchVerbBed, fail *bool) {
	t.Helper()
	contract := testpolicy.Contract{SchemaVersion: testpolicy.ExecutionContractSchemaVersion,
		ProjectRisk: testpolicy.ProjectRisk{Severity: 1, Exposure: 1, Reversibility: "revert", Detection: "immediate", Recovery: "bounded"},
		Surfaces:    []testpolicy.Surface{{ID: "fixture", Paths: []string{"metasystem/**"}, Standard: []string{"records"}}}, Unknown: []string{"records"}}
	for _, id := range []string{"records", "code"} {
		input := "metasystem/plans/**"
		if id == "code" {
			input = "metasystem/internal/**"
		}
		contract.Groups = append(contract.Groups, testpolicy.Group{ID: id, Kind: "unit", Adapter: "command", CWD: ".", Phase: "acceptance", EnvironmentMode: "inherit", Inputs: []string{input}, Platforms: []string{"any"}, TargetMS: 1, Argv: []string{"fixture"}, Format: "exit-status"})
	}
	data, err := json.Marshal(contract)
	helmMust(t, err)
	writeTestingFixtureFile(t, filepath.Join(b.installation, "testing.json"), data, 0600)
	b.configuration += "testing.contract=testing.json\n"
	b.policy(t, "1")
	b.git(t, b.checkout, "add", "metasystem/testing.json", "metasystem/metasystem.conf")
	b.git(t, b.checkout, "commit", "-qm", "fixture record check declaration")
	b.git(t, b.checkout, "push", "--quiet", "origin", "main")
	b.main = b.git(t, b.checkout, "rev-parse", "HEAD")
	seedCurrentTrunkClock(t, b.installation, b.now)
	b.owners.landing.plainProve.Closure = func(string, string, string) (adapter.Closure, error) { return adapter.Closure{}, nil }
	previous := b.owners.landing.plainProve.Command
	b.owners.landing.plainProve.Command = func(cmd *exec.Cmd) error {
		if len(cmd.Args) > 2 && cmd.Args[1] == "test" && cmd.Args[2] == "impact" {
			return previous(cmd)
		}
		scoped := landingCommandValue(cmd, "LANDING_PROOF_SCOPE") == "scoped"
		if scoped && landingCommandValue(cmd, "LANDING_PROOF_GROUPS") != "records" {
			t.Fatalf("fold executed unrelated checks: %v", cmd.Env)
		}
		err := previous(cmd)
		fmt.Fprintln(cmd.Stdout, "landing environment fixture-toolchain")
		if scoped && fail != nil && *fail {
			fmt.Fprintln(cmd.Stdout, "landing group records failed 1\nLANDING-FAILED\trecords\t\nLANDING-CHECKED\t1")
			return errors.New("affected check failed")
		}
		if scoped {
			fmt.Fprintln(cmd.Stdout, "landing group records passed 1")
		}
		return err
	}
}

func TestLandingMainMoveFailedAffectedCheckKeepsRemoteGitAdapter(t *testing.T) {
	t.Parallel()
	b := newBatchVerbBed(t, "1")
	fail := false
	mainMoveAffectedChecks(t, b, &fail)
	b.owners.landing.plainProve.Judge = func(string, string, []plain.FailedUnit) (map[string]plain.UnitJudgement, error) {
		return map[string]plain.UnitJudgement{"records": {Affected: true}}, nil
	}
	sha := b.seat(t, "a")
	b.success(t, "landing", "run")
	b.assemble(t, sha)
	b.success(t, "landing", "prove", "--gate", "--wait")
	b.success(t, "landing", "prove", "--wait")
	writer := t.TempDir()
	b.git(t, writer, "clone", "--quiet", b.origin, ".")
	writeTestingFixtureFile(t, filepath.Join(writer, "metasystem/plans/designs/fixture.md"), []byte("# fixture design\n"), 0600)
	b.git(t, writer, "add", ".")
	b.git(t, writer, "commit", "-qm", "design input")
	b.git(t, writer, "push", "--quiet", "origin", "main")
	main := b.git(t, writer, "rev-parse", "HEAD")
	if code, text := b.run(t, "landing", "push"); code == 0 || !strings.Contains(text, plain.CodeUnproven) {
		t.Fatalf("unchecked fold: %d %s", code, text)
	}
	fail = true
	if code, text := b.run(t, "landing", "prove", "--wait"); code == 0 {
		t.Fatalf("failed affected check passed: %s", text)
	}
	red, _, err := plain.LastResult(b.installation)
	if err != nil || red.Result != plain.Red || red.Scope != "scoped" || !slices.Contains(red.Ran, "records") || red.Cause != nil && red.Cause.Kind == "environment" {
		t.Fatalf("failure did not execute the affected check: %+v %v", red, err)
	}
	if code, text := b.run(t, "landing", "push"); code == 0 {
		t.Fatalf("red fold published: %s", text)
	}
	if got := b.git(t, writer, "ls-remote", "origin", "refs/heads/main"); got != main+"\trefs/heads/main" {
		t.Fatalf("failed fold moved remote: %s", got)
	}
}

func landingCommandValue(cmd *exec.Cmd, key string) string {
	for _, entry := range cmd.Env {
		if value, ok := strings.CutPrefix(entry, key+"="); ok {
			return value
		}
	}
	return ""
}

func TestLandingMainMoveTrunkReportsAffectedChecksGitAdapter(t *testing.T) {
	t.Parallel()
	b := newBatchVerbBed(t, "1")
	mainMoveAffectedChecks(t, b, nil)
	b.success(t, "landing", "prove", "--trunk", "--fresh", "--wait")
	writer := t.TempDir()
	b.git(t, writer, "clone", "--quiet", b.origin, ".")
	writeTestingFixtureFile(t, filepath.Join(writer, "metasystem/plans/designs/fixture.md"), []byte("# fixture design\n"), 0600)
	b.git(t, writer, "add", ".")
	b.git(t, writer, "commit", "-qm", "design input")
	b.git(t, writer, "push", "--quiet", "origin", "main")
	b.git(t, b.checkout, "fetch", "--quiet", "origin")
	command := b.owners.landing.plainProve.Command
	b.owners.landing.plainProve.Command = func(cmd *exec.Cmd) error { err := command(cmd); b.now = b.now.Add(2 * time.Minute); return err }
	code, output := b.run(t, "landing", "prove", "--trunk", "--wait")
	var result struct {
		Summary string
		Data    plain.Result
	}
	if err := json.Unmarshal([]byte(output), &result); err != nil || code != 0 || result.Data.Scope != "scoped" || !strings.Contains(result.Summary, "main moved by records only; 1 check run (2 min), the rest holds from the ") || strings.Contains(result.Summary, "full check") {
		t.Fatalf("affected main message: exit=%d data=%+v output=%s error=%v", code, result, output, err)
	}
}
