package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

func declaredCarryBed(t *testing.T, declaration string, cached bool) (*workBed, intentOwners, string, string) {
	t.Helper()
	b, owners, _ := rebaseIntentBed(t)
	repo := b.worktree
	connectionGit(t, repo, "init", "-q", "-b", "main")
	connectionGit(t, repo, "config", "user.name", "fixture")
	connectionGit(t, repo, "config", "user.email", "fixture@example.invalid")
	writeUnitCarryFile(t, filepath.Join(repo, ".gitignore"), "artifacts/\n")
	if declaration != "absent file" {
		writeUnitCarryFile(t, filepath.Join(repo, "metasystem.conf"), declaration)
		connectionGit(t, repo, "add", "metasystem.conf")
	}
	connectionGit(t, repo, "add", ".gitignore")
	connectionGit(t, repo, "commit", "-qm", "base")
	base := connectionGit(t, repo, "rev-parse", "HEAD")
	remote := filepath.Join(t.TempDir(), "origin.git")
	connectionGit(t, filepath.Dir(remote), "init", "-q", "--bare", remote)
	connectionGit(t, repo, "remote", "add", "origin", remote)
	connectionGit(t, repo, "push", "-q", "origin", "HEAD:main")
	connectionGit(t, repo, "checkout", "-qb", "goal/"+b.id)
	writeUnitCarryFile(t, filepath.Join(repo, "unit.go"), "unit change\n")
	connectionGit(t, repo, "add", "unit.go")
	unit, err := branch.CommitStaged(branch.CommitRequest{Repo: repo, Remote: "origin", EndpointTip: base, GoalID: b.id, Unit: "u1", Kind: branch.Unit, OpID: "unit", CheckClaim: func() error { return nil }})
	if err != nil {
		t.Fatal(err)
	}
	digest, err := branch.UnitDigest(repo, unit)
	if err != nil {
		t.Fatal(err)
	}
	readPath := "metasystem/records/misc/read.md"
	writeUnitCarryFile(t, filepath.Join(repo, readPath), "Read "+unit+" change "+digest)
	_, _, err = branch.CommitRead(branch.CommitReadRequest{Repo: repo, Remote: "origin", EndpointTip: base, GoalID: b.id, Unit: "u1", OpID: "read", ReaderRecord: readPath, CheckClaim: func() error { return nil }, GateRunID: "old-static-gate", GateTree: connectionGit(t, repo, "rev-parse", unit+"^{tree}")})
	if err != nil {
		t.Fatal(err)
	}
	tip := connectionGit(t, repo, "rev-parse", "HEAD")
	connectionGit(t, repo, "push", "-q", "origin", "HEAD:goal/"+b.id)
	connectionGit(t, repo, "checkout", "-q", "main")
	writeUnitCarryFile(t, filepath.Join(repo, "main.go"), "independent main change\n")
	connectionGit(t, repo, "add", "main.go")
	connectionGit(t, repo, "commit", "-qm", "main change")
	main := connectionGit(t, repo, "rev-parse", "HEAD")
	connectionGit(t, repo, "push", "-q", "origin", "HEAD:main")
	connectionGit(t, repo, "checkout", "-q", "goal/"+b.id)
	if cached {
		// Retain the predecessor, then put its equivalent unit onto current main.
		connectionGit(t, repo, "update-ref", "refs/metasystem/goals/before/"+b.id+"/"+tip, tip)
		connectionGit(t, repo, "checkout", "-qB", "goal/"+b.id, main)
		connectionGit(t, repo, "cherry-pick", unit)
		subject := connectionGit(t, repo, "rev-parse", "HEAD")
		tree := connectionGit(t, repo, "rev-parse", "HEAD^{tree}")
		path := filepath.Join(repo, ".git", "metasystem", "goal-reads", b.id, subject+".json")
		writeUnitCarryFile(t, path, fmt.Sprintf(`{"schemaVersion":1,"goal":%q,"unitCommit":%q,"tree":%q,"gateRunId":"old-static-gate"}`, b.id, subject, tree))
		connectionGit(t, repo, "cherry-pick", tip)
		connectionGit(t, repo, "push", "-q", "--force", "origin", "HEAD:goal/"+b.id)
	}
	owners.connection.rebase = branch.Rebase
	owners.connection.endpointTip = func(string, goal.Endpoint) (string, error) { return main, nil }
	owners.connection.rebaseGate = func(string) (string, error) { t.Fatal("carry ran the static gate"); return "", nil }
	return b, owners, main, unit
}

func TestWorkRebaseDeclaredCheckSubjectAndFailures(t *testing.T) {
	t.Parallel()
	for _, state := range []string{"green", "cached gate", "cached missing", "missing", "unreadable", "bad deadline", "cheap red", "audit red", "edited tree", "committed edit", "record write failure"} {
		t.Run(state, func(t *testing.T) {
			t.Parallel()
			script := filepath.Join(t.TempDir(), "check")
			cheap, audit := 0, 0
			if state == "cheap red" {
				cheap = 9
			}
			if state == "audit red" {
				audit = 17
			}
			body := fmt.Sprintf("#!/bin/sh\nprintf '%%s|%%s|%%s\\n' \"$1\" \"$PWD\" \"$PATH\"\ncase $1 in cheap) exit %d;; audits) exit %d;; esac\n", cheap, audit)
			if err := testexec.WriteFile(script, []byte(body), 0700); err != nil {
				t.Fatal(err)
			}
			declaration := "proof.cheap=" + shellCommand([]string{script, "cheap"}) + "\nproof.audits=" + shellCommand([]string{script, "audits"}) + "\nproof.deadline=15\n"
			if state == "missing" || state == "cached missing" {
				declaration = ""
			}
			if state == "bad deadline" {
				declaration = strings.Replace(declaration, "deadline=15", "deadline=0", 1)
			}
			if state == "edited tree" {
				declaration = strings.Replace(declaration, "proof.cheap=", "proof.cheap=printf edited > unit.go; ", 1)
			}
			if state == "committed edit" {
				declaration = strings.Replace(declaration, "proof.cheap=", "proof.cheap=printf edited > unit.go; git commit -qam 'check changed its tree'; ", 1)
			}
			if state == "unreadable" {
				declaration = "absent file"
			}
			b, owners, main, original := declaredCarryBed(t, declaration, state == "cached gate" || state == "cached missing")

			if state == "record write failure" {
				writeUnitCarryFile(t, filepath.Join(b.root(), "artifacts", "unit-checks", "carry"), "blocks execution storage")
			}
			code, result := b.runJSON(owners, "work", "rebase", b.id)
			data, err := json.Marshal(result.Data)
			var rebased branch.RebaseResult
			if err != nil || json.Unmarshal(data, &rebased) != nil {
				t.Fatalf("result: %+v %v", result, err)
			}
			if state == "cheap red" || state == "audit red" || state == "edited tree" || state == "committed edit" || state == "record write failure" {
				if code == 0 {
					t.Fatalf("invalid check accepted: %+v", result)
				}
			} else if code != 0 {
				t.Fatalf("rebase exit=%d: %+v", code, result)
			}
			if state == "missing" || state == "cached missing" || state == "unreadable" || state == "bad deadline" {
				if !slices.Equal(rebased.NeedsReview, []string{"u1"}) || len(rebased.Carried) != 0 || !strings.Contains(rebased.ReviewReasons["u1"], "proof.") {
					t.Fatalf("missing declaration: %+v", rebased)
				}
				journals, err := filepath.Glob(filepath.Join(b.worktree, ".git", "metasystem", "goal-reads", b.id, "*.json"))
				if err != nil || len(journals) != 1 {
					t.Fatalf("missing declaration journal: %v %v", journals, err)
				}
				data, err := os.ReadFile(journals[0])
				var journal struct {
					GateRunID   string
					GateFailure string
				}
				if err != nil || json.Unmarshal(data, &journal) != nil || journal.GateRunID != "" || journal.GateFailure != rebased.ReviewReasons["u1"] {
					t.Fatalf("stale or missing failure: %s %v", data, err)
				}
				return
			}
			if state == "record write failure" {
				return
			}
			executions, err := filepath.Glob(filepath.Join(b.root(), "artifacts", "unit-checks", "carry", "*", "check-*", "result.json"))
			if err != nil || len(executions) != 1 {
				t.Fatalf("executions=%v err=%v result=%+v", executions, err, result)
			}
			var execution struct {
				ExecutionID string
				Check       launch.UnitCheck
				Exits       []launch.CheckExit
			}
			evidence, err := os.ReadFile(executions[0])
			if err != nil || json.Unmarshal(evidence, &execution) != nil || len(execution.Exits) != 2 {
				t.Fatalf("evidence=%s err=%v", evidence, err)
			}
			if execution.Exits[0].Exit != cheap || execution.Exits[1].Exit != audit || !strings.Contains(execution.Exits[0].Output, execution.Check.Directory) || !slices.Equal(execution.Check.Environment, os.Environ()) {
				t.Fatalf("execution=%+v", execution)
			}
			if code != 0 {
				return
			}
			if !slices.Equal(rebased.Carried, []string{"u1"}) || len(rebased.NeedsReview) != 0 {
				t.Fatalf("carry=%+v", rebased)
			}
			commits, err := branch.ValidateRange(b.worktree, main, rebased.NewTip, b.id)
			if err != nil {
				t.Fatal(err)
			}
			subject := commits[0].ID
			if subject == original {
				t.Fatal("rebase did not create a fresh subject")
			}
			att, err := branch.ValidateAttestationAt(b.worktree, rebased.NewTip, main, b.id, "u1", subject)
			if err != nil || att.Gate.Kind != "unit-check" || att.Gate.RunID != execution.ExecutionID || att.Gate.RunID == "old-static-gate" || att.Gate.Tree != connectionGit(t, b.worktree, "rev-parse", subject+"^{tree}") {
				t.Fatalf("attestation=%+v err=%v", att, err)
			}
			// A later checkout edit must not choose the subject's declarations.
			writeUnitCarryFile(t, filepath.Join(b.worktree, "metasystem.conf"), "proof.cheap=false\n")
			if execution.Check.Cheap != strings.SplitN(strings.TrimPrefix(declaration, "proof.cheap="), "\n", 2)[0] {
				t.Fatal("candidate declaration was used")
			}
		})
	}
}

func TestWorkRebaseDeclaredCheckEvidenceValidation(t *testing.T) {
	t.Parallel()
	b, owners, main, _ := declaredCarryBed(t, "proof.cheap=true\nproof.audits=true\nproof.deadline=15\n", false)
	code, result := b.runJSON(owners, "work", "rebase", b.id)
	if code != 0 {
		t.Fatalf("rebase: %d %+v", code, result)
	}
	tip := connectionGit(t, b.worktree, "rev-parse", "HEAD")
	commits, err := branch.ValidateRange(b.worktree, main, tip, b.id)
	if err != nil {
		t.Fatal(err)
	}
	subject := commits[0].ID
	original, err := branch.ValidateAttestation(b.worktree, main, b.id, "u1", subject)
	if err != nil {
		t.Fatal(err)
	}
	var execution struct {
		ExecutionID string             `json:"executionId"`
		Check       launch.UnitCheck   `json:"check"`
		Exits       []launch.CheckExit `json:"exits"`
	}
	if err := json.Unmarshal([]byte(original.Gate.Evidence), &execution); err != nil {
		t.Fatal(err)
	}
	if len(execution.Check.Environment) != 0 {
		t.Fatal("committed proof exposed the execution environment")
	}
	for _, damage := range []string{"cheap red", "audit red", "missing audit", "wrong execution", "wrong tree", "changed command", "changed digest"} {
		att := original
		var changed struct {
			ExecutionID string             `json:"executionId"`
			Check       launch.UnitCheck   `json:"check"`
			Exits       []launch.CheckExit `json:"exits"`
		}
		if err := json.Unmarshal([]byte(att.Gate.Evidence), &changed); err != nil {
			t.Fatal(err)
		}
		switch damage {
		case "cheap red":
			changed.Exits[0].Exit = 9
		case "audit red":
			changed.Exits[1].Exit = 17
		case "missing audit":
			changed.Exits = changed.Exits[:1]
		case "wrong execution":
			changed.ExecutionID = "another-check"
		case "wrong tree":
			changed.Check.SourceTree = strings.Repeat("0", 40)
		case "changed command":
			changed.Check.Audits = "false"
		case "changed digest":
			att.Gate.CommandDigest = strings.Repeat("0", 64)
		}
		evidence, err := json.Marshal(changed)
		if err != nil {
			t.Fatal(err)
		}
		att.Gate.Evidence, att.SHA256 = string(evidence), ""
		sealed, err := json.Marshal(att)
		if err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(sealed)
		att.SHA256 = hex.EncodeToString(digest[:])
		data, err := json.MarshalIndent(att, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		writeUnitCarryFile(t, filepath.Join(b.worktree, "metasystem", "records", "reads", b.id, subject+".json"), string(data)+"\n")
		if _, err := branch.ValidateAttestation(b.worktree, main, b.id, "u1", subject); err == nil || !strings.Contains(goal.RecordText(err), branch.ReadUngatedCode) {
			t.Errorf("%s accepted or refused by another boundary: %v", damage, err)
		}
	}
}
