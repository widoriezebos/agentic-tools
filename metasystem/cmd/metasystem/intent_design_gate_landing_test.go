package main

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/designgate"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/landpath"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/narratordigest"
)

func landingGateInvocation(t *testing.T, bed *workBed, stderr *bytes.Buffer) *intentInvocation {
	t.Helper()
	owners := bed.workOwners()
	layout, err := owners.resolver.ResolveLayout(bed.root())
	if err != nil {
		t.Fatal(err)
	}
	return &intentInvocation{owners: owners, layout: layout, stateRoot: bed.stateRoot(), stderr: stderr}
}

func TestLandingDesignCheckRecords(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"unchanged", "head", "adjacent body", "body", "no critique", "open chain", "superseded", "no record", "unestablished", "reserved", "identity failure", "corrupt record", "facts failure"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			bed := newDesignGateBed(t, 2)
			path, data := designGatePage(t, bed, "- Critique: closed at round 2 on 0 material findings (WHO)\n")
			designGateBuild(t, bed, "u")
			identity, _ := bed.designGate.identity(bed.stateRoot())
			_, record := designGateRead(t, bed, identity, "u")
			wantBody := fmt.Sprintf("%x", sha256.Sum256([]byte("Build the gate.\n")))
			if len(record.Designs) != 1 || record.Designs[0].SHA256 != fmt.Sprintf("%x", sha256.Sum256(data)) || record.Designs[0].BodySHA256 != wantBody {
				t.Fatalf("dispatch digests: %+v", record.Designs)
			}
			verdict, compared := "ok", true
			switch scenario {
			case "head":
				data = bytes.ReplaceAll(data, []byte("round 2"), []byte("round 4"))
			case "adjacent body":
				data = bytes.ReplaceAll(data, []byte("\n\nBuild"), []byte("\nBuild"))
			case "body":
				data = bytes.ReplaceAll(data, []byte("Build the gate."), []byte("Build another approach."))
				verdict = "design-changed"
			case "no critique":
				data = bytes.ReplaceAll(data, []byte("- Critique: closed at round 2 on 0 material findings (WHO)\n"), nil)
				verdict, compared = "critique-not-recorded", false
			case "open chain":
				bed.designGate.chains = func(string, string, string) ([]designgate.Chain, error) { return []designgate.Chain{{Round: 3}}, nil }
				verdict, compared = "critique-open", false
			case "superseded":
				data = bytes.ReplaceAll(data, []byte("Status: accepted"), []byte("Status: superseded"))
				replacement := bytes.ReplaceAll(bytes.ReplaceAll(data, []byte("gate-design"), []byte("replacement")), []byte("Status: superseded"), []byte("Status: accepted"))
				if err := os.WriteFile(filepath.Join(filepath.Dir(path), "replacement.md"), replacement, 0o600); err != nil {
					t.Fatal(err)
				}
				verdict = "design-missing"
			case "no record":
				if err := os.Remove(filepath.Join(bed.unitRoot, ".design-gate", identity, bed.id, "u.json")); err != nil {
					t.Fatal(err)
				}
				compared = false
			case "unestablished":
				if err := os.RemoveAll(filepath.Join(bed.unitRoot, ".named")); err != nil {
					t.Fatal(err)
				}
				compared = false
			case "identity failure":
				bed.designGate.identity = func(string) (string, error) { return "", errors.New("unavailable") }
				compared = false
			case "reserved":
				var output bytes.Buffer
				inv := landingGateInvocation(t, bed, &output)
				work, err := inv.work().units(inv.layout).NamedWork(bed.worktree, bed.id)
				if err != nil || len(work) != 1 {
					t.Fatalf("named work: %v %v", work, err)
				}
				if err := os.Remove(filepath.Join(bed.unitRoot, work[0].Run, "run.json")); err != nil {
					t.Fatal(err)
				}
				compared = false
			case "corrupt record":
				if err := os.WriteFile(filepath.Join(bed.unitRoot, ".design-gate", identity, bed.id, "u.json"), []byte("broken"), 0o600); err != nil {
					t.Fatal(err)
				}
				verdict, compared = "unchecked", false
			case "facts failure":
				bed.designGate.chains = func(string, string, string) ([]designgate.Chain, error) { return nil, errors.New("facts unavailable") }
				verdict, compared = "unchecked", false
			}
			if err := os.WriteFile(path, data, 0o600); err != nil {
				t.Fatal(err)
			}
			var stderr bytes.Buffer
			inv := landingGateInvocation(t, bed, &stderr)
			r := landing.ObserveDesign(inv.landingDesignFacts(bed.stateRoot(), bed.id), false)
			if r.Verdict != verdict || r.Compared != compared {
				t.Fatalf("landing record check: %+v want %s compared=%v", r, verdict, compared)
			}
		})
	}
}

func TestLandingDesignCheckDigest(t *testing.T) {
	t.Parallel()
	for _, scenario := range []struct{ failure, person bool }{{false, false}, {true, false}, {false, true}, {true, true}} {
		t.Run(fmt.Sprint(scenario), func(t *testing.T) {
			t.Parallel()
			bed := newDesignGateBed(t, 2)
			writes := 0
			ending := "; the landing goes on"
			if scenario.person {
				ending = "; it goes on at your word"
			}
			bed.designGate.digest = func(root string, entry narratordigest.Entry, now time.Time) error {
				writes++
				if root != bed.stateRoot() || entry.Kind != "lowlight" || entry.Text != "warning: goal "+bed.id+" has no accepted design"+ending || entry.SourceType != "design-gate-landing" || entry.SourceID != bed.id+"@TIP" || now.IsZero() {
					t.Fatalf("digest: %+v root=%s now=%s", entry, root, now)
				}
				if scenario.failure {
					return errors.New("digest unavailable")
				}
				return nil
			}
			var stderr bytes.Buffer
			inv := landingGateInvocation(t, bed, &stderr)
			inv.owners.work.git = func(string, ...string) ([]byte, error) { return []byte("TIP\n"), nil }
			r := landing.ObserveDesign(inv.landingDesignFacts(bed.stateRoot(), bed.id), scenario.person)
			inv.recordLandingDesign(bed.id, &r, time.Now())
			inv.recordLandingDesign(bed.id, &landing.DesignObservation{}, time.Now())
			inv.recordLandingDesign("", &r, time.Now())
			if writes != 1 || (stderr.Len() != 0) != scenario.failure || scenario.failure && !strings.Contains(stderr.String(), "digest unavailable); the landing goes on\nnothing to do:") {
				t.Fatalf("digest writes=%d stderr=%q", writes, stderr.String())
			}
		})
	}
}

func TestLandingDesignCheckMode(t *testing.T) {
	t.Parallel()
	bed := newDesignGateBed(t, 2)
	baseConfig := (&intentInvocation{}).work().config
	bed.config = func(key, path string) (string, string, int, error) {
		if key == config.DesignGateModeKey {
			return "refuse", "fixture", 0, nil
		}
		return baseConfig(key, path)
	}
	var stderr bytes.Buffer
	inv := landingGateInvocation(t, bed, &stderr)
	f := inv.landingDesignFacts(bed.stateRoot(), bed.id)
	if r := landing.ObserveDesign(f, false); !r.RefusesAgent {
		t.Fatalf("agent: %+v", r)
	}
	if r := landing.ObserveDesign(f, true); r.RefusesAgent || !r.Person || !strings.HasSuffix(r.Pair[0], "; it goes on at your word") {
		t.Fatalf("person: %+v", r)
	}
}

func TestLandingDesignCheckGoalLess(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	observed, status := landingPathObserve(landpath.ObserveRequest{Root: root, Tree: "invalid", Actor: "m1+human"})
	if status != 0 || observed.Design != nil {
		t.Fatalf("goal-less live observation: status=%d observation=%+v", status, observed)
	}
	var stdout, stderr bytes.Buffer
	if code := runLandingObserve([]string{"--root", root, "--tree", "invalid", "--actor", "m1+human"}, &stdout, &stderr); code != 0 || strings.Contains(stdout.String(), `"design"`) || stderr.Len() != 0 {
		t.Fatalf("goal-less command: code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	bed := newDesignGateBed(t, 2)
	designGatePage(t, bed, "")
	inv := landingGateInvocation(t, bed, &stderr)
	if result := landing.ObserveDesign(inv.landingDesignFacts(bed.stateRoot(), ""), false); result.Verdict != "not-design-bearing" || result.Pair != [2]string{} {
		t.Fatalf("goal-less facts: %+v", result)
	}
}

func TestLandingDesignCheckCarryWord(t *testing.T) {
	t.Parallel()
	bed := newGoalCLIBed(t, goalCLISeed{rootRecord: func(record *goal.RootRecord) { record.FormatVersion = "2" }})
	var stdout, stderr bytes.Buffer
	dependencies := bed.dependencies(&stdout, &stderr)
	flags := &syncFlags{root: bed.root, id: "ship-widget", by: "Wido", fixtureHumanAuthority: true}
	classification, err := classifyGoalAuthorityFirstWithFacts("carry", flags, dependencies.authorityFacts)
	if err != nil {
		t.Fatal(err)
	}
	proof, err := proveGoalHumanAuthorityAt("carry", flags, bed.prove, bed.commandNow)
	if err != nil {
		t.Fatal(err)
	}
	request, err := syncReqClassifiedWithTerminalGradeAtWithDependencies(bed.root, "Wido", "", &proof, classification, false, bed.commandNow, dependencies)
	if err != nil {
		t.Fatal(err)
	}
	result, err := goal.Carry(request, goal.CarryArgs{Goal: "ship-widget", Workspace: strings.Repeat("a", 40),
		Past: "LANDING_DESIGN_NOT_STANDING", Why: "Land at the person's word", Expires: request.Now.Add(time.Hour)}, &proof)
	if err != nil || result.Outcome != goal.OutcomeConfirmed || !strings.Contains(bed.goalRecord("ship-widget"), "past=LANDING_DESIGN_NOT_STANDING") {
		t.Fatalf("design exception word: result=%+v err=%v", result, err)
	}
}
