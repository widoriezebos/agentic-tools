package main

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
)

func TestLandingProveImpactOwnProvenanceAndPush(t *testing.T) {
	t.Parallel()
	b := newMergeGateBed(t)
	b.prepareBatch(t)
	const plan = "plan: base main (base)\nselection: internal/a\n"
	plans, proofs := 0, 0
	b.owners.landing.plainProve.Command = func(cmd *exec.Cmd) error {
		if len(cmd.Args) == 6 && reflect.DeepEqual(cmd.Args[1:], []string{"test", "impact", "--plan", "--base", "main"}) {
			plans++
			fmt.Fprint(cmd.Stdout, plan)
			return nil
		}
		proofs++
		if cmd.Args[2] != "cheap-fixture" || commandEnv(cmd, "LANDING_PROOF_SCOPE") != "impact" || commandEnv(cmd, "LANDING_PROOF_BASE") != "main" {
			t.Fatalf("impact command: %v; scope=%s base=%s", cmd.Args, commandEnv(cmd, "LANDING_PROOF_SCOPE"), commandEnv(cmd, "LANDING_PROOF_BASE"))
		}
		fmt.Fprint(cmd.Stdout, "landing environment fixture toolchain\nlanding group fast-static-build green 1\nlanding group unit/internal/a green 2\nLANDING-CHECKED\t0\n")
		return nil
	}
	code, out := b.run(t, b.root, "prove", "--impact", "--wait", "--json")
	var report struct{ Data plain.Result }
	if err := json.Unmarshal([]byte(out), &report); err != nil || code != 0 {
		t.Fatalf("impact exit=%d: %s (%v)", code, out, err)
	}
	r := report.Data
	if r.Result != plain.Green || r.Scope != "impact" || r.Base != "main-tree" || r.BaseCommit != "main" || r.PlanHash != fmt.Sprintf("%x", sha256.Sum256([]byte(plan))) || r.Environment != "fixture toolchain" || r.FullAt != "" || r.FullTree != "" || r.CountedFull || plans != 1 || proofs != 1 {
		t.Fatalf("impact provenance: %+v; plans=%d proofs=%d", r, plans, proofs)
	}
	history, err := plain.Results(b.install)
	if err != nil || len(history) != 1 {
		t.Fatalf("first proof inherited history: %+v %v", history, err)
	}
	code, status := b.run(t, b.root, "status")
	for _, want := range []string{"scope", "impact", "main", "main-tree", r.PlanHash[:12], "environment fingerprint", "present"} {
		if code != 0 || !strings.Contains(status, want) {
			t.Fatalf("status lacks %q: exit=%d %s", want, code, status)
		}
	}
	t.Logf("status text:\n%s", status)
	git := b.owners.landing.plainProve.Git
	pushed := false
	b.owners.landing.plainProve.Git = func(dir string, args ...string) (string, error) {
		if strings.Join(args, " ") == "rev-parse --verify refs/remotes/origin/main^{commit}" {
			return "main", nil
		}
		if len(args) > 0 && args[0] == "push" {
			pushed = true
			return "", nil
		}
		return git(dir, args...)
	}
	code, out = b.run(t, b.root, "push", "--json")
	if code != 0 || !pushed {
		t.Fatalf("first impact push: exit=%d pushed=%v %s", code, pushed, out)
	}
}

func TestLandingProveImpactMissingFingerprintIsErrorWithoutFull(t *testing.T) {
	t.Parallel()
	b := newMergeGateBed(t)
	b.prepareBatch(t)
	proofs := 0
	b.owners.landing.plainProve.Command = func(cmd *exec.Cmd) error {
		if len(cmd.Args) > 1 && cmd.Args[1] == "test" {
			fmt.Fprint(cmd.Stdout, "plan: base main\n")
			return nil
		}
		proofs++
		if commandEnv(cmd, "LANDING_PROOF_SCOPE") != "impact" {
			t.Fatalf("reran at scope %q", commandEnv(cmd, "LANDING_PROOF_SCOPE"))
		}
		fmt.Fprint(cmd.Stdout, "LANDING-CHECKED\t0\n")
		return nil
	}
	code, out := b.run(t, b.root, "prove", "--impact", "--wait", "--json")
	var report struct{ Data plain.Result }
	if err := json.Unmarshal([]byte(out), &report); err != nil || code != 1 || report.Data.Result != plain.Red || report.Data.Scope != "impact" || !strings.Contains(report.Data.ScopeReason, "environment fingerprint is empty") || report.Data.FullAt != "" || report.Data.FullTree != "" || proofs != 1 {
		t.Fatalf("missing fingerprint exit=%d proofs=%d: %s (%v)", code, proofs, out, err)
	}
}

func TestLandingProveImpactStaticRedNamesGroup(t *testing.T) {
	t.Parallel()
	b := newMergeGateBed(t)
	b.prepareBatch(t)
	b.owners.landing.plainProve.Command = func(cmd *exec.Cmd) error {
		if len(cmd.Args) > 1 && cmd.Args[1] == "test" {
			fmt.Fprint(cmd.Stdout, "plan: base main\n")
			return nil
		}
		fmt.Fprint(cmd.Stdout, "landing environment fixture toolchain\nlanding group fast-static-build red 1\nlanding group unit/internal/a green 2\nLANDING-FAILED\tfast-static-build\t\nLANDING-CHECKED\t1\n")
		return exec.Command("/usr/bin/false").Run()
	}
	code, out := b.run(t, b.root, "prove", "--impact", "--wait", "--json")
	var report struct{ Data plain.Result }
	if err := json.Unmarshal([]byte(out), &report); err != nil || code != 1 || report.Data.Result != plain.Red || report.Data.Scope != "impact" || len(report.Data.Failed) != 1 || report.Data.Failed[0].Unit != "fast-static-build" || !strings.Contains(report.Data.Reason, "fast-static-build") || !reflect.DeepEqual(report.Data.Ran, []string{"fast-static-build", "unit/internal/a"}) {
		t.Fatalf("static red exit=%d: %s (%v)", code, out, err)
	}
}

func TestLandingProveImpactEnvironmentErrorAllowsOneRetry(t *testing.T) {
	t.Parallel()
	for _, failure := range []string{"plan", "fingerprint"} {
		for _, retry := range []string{"impact", "full", "second error"} {
			t.Run(failure+"/"+retry, func(t *testing.T) {
				t.Parallel()
				b := newMergeGateBed(t)
				b.prepareBatch(t)
				plans, proofs := 0, 0
				failing := true
				b.owners.landing.plainProve.Command = func(cmd *exec.Cmd) error {
					if len(cmd.Args) > 1 && cmd.Args[1] == "test" {
						plans++
						if failing && failure == "plan" {
							return errors.New("impact plan temporarily unavailable")
						}
						fmt.Fprint(cmd.Stdout, "plan: base main\n")
						return nil
					}
					proofs++
					if !failing {
						fmt.Fprint(cmd.Stdout, "landing environment fixture toolchain\nlanding group fast-static-build green 1\n")
					}
					fmt.Fprint(cmd.Stdout, "LANDING-CHECKED\t0\n")
					return nil
				}
				code, out := b.run(t, b.root, "prove", "--impact", "--wait", "--json")
				var first struct{ Data plain.Result }
				if err := json.Unmarshal([]byte(out), &first); err != nil || code != 1 || first.Data.Result != plain.Red || first.Data.Cause == nil || first.Data.Cause.Kind != "environment" {
					t.Fatalf("first environment error exit=%d: %s (%v)", code, out, err)
				}
				wantReason := "environment fingerprint is empty"
				wantProofs := 1
				if failure == "plan" {
					wantReason, wantProofs = "impact plan could not be read", 0
				}
				if !strings.Contains(first.Data.ScopeReason, wantReason) || plans != 1 || proofs != wantProofs {
					t.Fatalf("first error provenance: %+v; plans=%d proofs=%d", first.Data, plans, proofs)
				}
				failing = retry == "second error"
				args := []string{"prove", "--wait", "--json"}
				wantPlans := 1
				if retry != "full" {
					args = append(args, "--impact")
					wantPlans++
				}
				code, out = b.run(t, b.root, args...)
				var second struct{ Data plain.Result }
				wantCode, wantResult := 0, plain.Green
				if failing {
					wantCode, wantResult = 1, plain.Red
				} else {
					wantProofs++
				}
				if failing && failure == "fingerprint" {
					wantProofs++
				}
				if err := json.Unmarshal([]byte(out), &second); err != nil || code != wantCode || second.Data.Result != wantResult || plans != wantPlans || proofs != wantProofs {
					t.Fatalf("retry after repeat=%q exit=%d plans=%d proofs=%d: %s (%v)", first.Data.Repeat, code, plans, proofs, out, err)
				}
				if first.Data.Repeat != "allowed" {
					t.Fatalf("first environment error lacks its repeat allowance: %+v", first.Data)
				}
				t.Logf("first: red, environment cause, repeat %s; second: %s; plan calls=%d, proof calls=%d", first.Data.Repeat, second.Data.Result, plans, proofs)
				if !failing {
					return
				}
				if second.Data.Cause == nil || second.Data.Cause.Kind != "environment" || second.Data.Repeat == "allowed" {
					t.Fatalf("second environment error renewed its allowance: %+v", second.Data)
				}
				for _, impact := range []bool{false, true} {
					args := []string{"prove", "--wait"}
					if impact {
						args = append(args, "--impact")
					}
					code, out = b.run(t, b.root, args...)
					if code != 1 || !strings.Contains(out, "this code failed its check and gets no other") || plans != wantPlans || proofs != wantProofs {
						t.Fatalf("third prove impact=%v exit=%d plans=%d proofs=%d: %s", impact, code, plans, proofs, out)
					}
				}
			})
		}
	}
}

func TestLandingProveImpactBackgroundKeepsScope(t *testing.T) {
	t.Parallel()
	b := newMergeGateBed(t)
	b.prepareBatch(t)
	b.owners.landing.plainProve.Executable = func() (string, error) { return "/fixture/engine", nil }
	b.owners.landing.plainProve.Launch = func(argv []string, _, _ string) (int64, error) {
		if !strings.Contains(strings.Join(argv, " "), "--impact") {
			t.Fatalf("background loses scope: %v", argv)
		}
		return int64(os.Getpid()), nil
	}
	b.owners.landing.plainProve.Alive = func(plain.Running) bool { return true }
	code, out := b.run(t, b.root, "prove", "--impact", "--json")
	if code != 0 {
		t.Fatalf("start impact exit=%d: %s", code, out)
	}
	running, recorded, _, err := plain.ReadRunning(b.install, b.owners.landing.plainProve)
	if err != nil || !recorded || !running.Impact || running.Admission == nil || running.Admission.Scope != "impact" || running.Admission.Base != "main-tree" || running.Admission.BaseSHA != "main" {
		t.Fatalf("impact admission: %+v %v", running, err)
	}
}
