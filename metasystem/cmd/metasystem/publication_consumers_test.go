package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func TestIntentPublicationBaseOverridesOnlyItsExecution(t *testing.T) {
	t.Parallel()
	fixture := filepath.Join(t.TempDir(), "selection.json")
	helmMust(t, (launch.CheckSelection{Version: 1, Base: "publication-parent"}).Write(fixture))
	cheap := fmt.Sprintf("test \"$LANDING_PROOF_BASE\" = publication-parent && /bin/cp %s \"$METASYSTEM_CHECK_SELECTION\"", shellCommand([]string{fixture}))
	b := declaredCheckBed(t, "proof.cheap="+cheap+"\nproof.audits=printf audited\nproof.deadline=15\n")
	code, built, _ := b.work("work", "build", b.id, "parent", "--brief", b.brief("parent.md", "Build.\n"), "--lines", "10")
	if code != 0 {
		t.Fatalf("build: %d %+v", code, built)
	}
	planPath := resultData(t, built)["plan"].(string)
	before := mustRead(t, planPath)
	plan, err := launch.ReadUnitPlan(planPath)
	helmMust(t, err)
	if plan.Check.Base == "publication-parent" {
		t.Fatal("fixture did not distinguish frozen and publication bases")
	}
	prior := filepath.Join(filepath.Dir(planPath), "check-prior", "evidence.json")
	transferWriteJSON(t, prior, map[string]string{"source": "unchanged prior evidence"})
	priorBytes := mustRead(t, prior)
	owners := b.workOwners()
	owners.lookupEnv = func(key string) (string, bool) {
		switch key {
		case launch.KindEnv:
			return "proof", true
		case "METASYSTEM_PUBLICATION_BASE":
			return "publication-parent", true
		}
		return "", false
	}
	git := owners.work.git
	owners.work.git = func(root string, args ...string) ([]byte, error) {
		if slices.Equal(args, []string{"rev-parse", "--show-toplevel"}) {
			return []byte(b.worktree), nil
		}
		return git(root, args...)
	}
	command, rest, _ := resolveIntentArgv([]string{"test", "run", "--unit-run", resultData(t, built)["run"].(string)})
	var out, problem bytes.Buffer
	code = runIntentIn(command, append([]string{"--json"}, rest...), &out, &problem, b.worktree, owners)
	var result intentResult
	helmMust(t, json.Unmarshal(out.Bytes(), &result))
	if code != 0 {
		t.Fatalf("publication check: %d %+v %s", code, result, &problem)
	}
	execution := resultData(t, result)["execution"].(string)
	var evidence launch.CheckEvidence
	helmMust(t, json.Unmarshal(mustRead(t, filepath.Join(execution, "evidence.json")), &evidence))
	if evidence.Selection.Base != "publication-parent" || !evidence.Strength.Complete || evidence.Source != filepath.Base(execution) || !bytes.Equal(before, mustRead(t, planPath)) || !bytes.Equal(priorBytes, mustRead(t, prior)) {
		t.Fatalf("publication did not retain an isolated execution on its parent: %+v", evidence)
	}
}

func TestCarryProducesThenReusesItsEvidence(t *testing.T) {
	t.Parallel()
	for _, audits := range []string{"printf audited", "true"} {
		t.Run(audits, func(t *testing.T) {
			t.Parallel()
			b, owners, _, _ := unitCarryIntentBed(t, true)
			subject := branch.AttestationSubject{Commit: "corrected-commit", Tree: strings.Repeat("d", 40), Parent: "base-commit"}
			owners.connection.readRepository = carryFullRepository{root: b.worktree, common: t.TempDir(), subject: subject}
			owners.prove = enrolledPersonProver(t, b.root(), b.manager.Now())
			installCarrySelectionProducer(t, b.worktree)
			marker := filepath.Join(t.TempDir(), "ran")
			script := filepath.Join(t.TempDir(), "cheap")
			helmMust(t, testexec.WriteFile(script, []byte("#!/bin/sh\n"+carrySelectionProducerCommand(t)+" || exit $?\nprintf ran >> "+shellCommand([]string{marker})+"\n"), 0700))
			b.declaredCheap, b.declaredAudits, b.declaredDeadline = shellCommand([]string{script}), audits, "15"
			original := owners.work.git
			owners.work.git = func(root string, args ...string) ([]byte, error) {
				switch {
				case slices.Equal(args, []string{"show", "-s", "--format=%(trailers:only,unfold=true)", subject.Commit}):
					return []byte("Goal-Unit: " + b.id + "/u1"), nil
				case slices.Equal(args, []string{"rev-parse", "HEAD"}):
					return []byte(subject.Commit), nil
				case len(args) == 2 && args[0] == "rev-parse" && strings.HasSuffix(args[1], "^{tree}"):
					return []byte(subject.Tree), nil
				case slices.Equal(args, []string{"status", "--porcelain", "--untracked-files=all"}):
					return nil, nil
				}
				return original(root, args...)
			}
			carry := func() string {
				code, output, problem := b.run(owners, "work", "review", "--commit", subject.Commit, "--goal", b.id, "--check-only")
				if code != 0 {
					t.Fatalf("carry: %d %s %s", code, output, problem)
				}
				return problem
			}
			carry()
			paths, err := filepath.Glob(filepath.Join(b.root(), "artifacts", "unit-checks", "carry", subject.Commit, "check-*", "evidence.json"))
			helmMust(t, err)
			if len(paths) != 1 {
				t.Fatalf("first carry executions: %v", paths)
			}
			var produced launch.CheckEvidence
			helmMust(t, json.Unmarshal(mustRead(t, paths[0]), &produced))
			if !produced.Strength.Complete || produced.Content.Before != subject.Tree || produced.Content.After != subject.Tree || produced.Source != filepath.Base(filepath.Dir(paths[0])) {
				t.Fatalf("first carry produced incomplete evidence: %+v", produced)
			}
			assertCarrySelectionEvidence(t, filepath.Join(filepath.Dir(paths[0]), "result.json"), true, audits != "true")
			message := carry()
			want := "ran"
			if audits == "true" {
				want = "ranran"
				if !strings.Contains(message, "strength differs") {
					t.Fatalf("cheap carry evidence was not refused by strength: %s", message)
				}
			}
			results, err := filepath.Glob(filepath.Join(b.root(), "artifacts", "unit-checks", "carry", subject.Commit, "check-*", "result.json"))
			helmMust(t, err)
			if len(results) != len(want)/len("ran") {
				t.Fatalf("second carry executions: %v", results)
			}
			if string(mustRead(t, marker)) != want {
				t.Fatalf("second carry ran the wrong commands: %s", mustRead(t, marker))
			}
			if audits != "true" {
				contractPath := filepath.Join(b.worktree, "testing.json")
				var contract testpolicy.Contract
				helmMust(t, json.Unmarshal(mustRead(t, contractPath), &contract))
				contract.Groups[0].TargetMS++
				transferWriteJSON(t, contractPath, contract)
				if message := carry(); !strings.Contains(message, "contract digest differs") || string(mustRead(t, marker)) != "ranran" {
					t.Fatalf("changed contract did not rerun: %s", message)
				}
			}
		})
	}
}
