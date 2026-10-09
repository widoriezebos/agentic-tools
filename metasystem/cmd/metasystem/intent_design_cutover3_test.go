package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/delegation"
	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/readsubject"
)

func cutoverPositiveRead(t *testing.T) (*designLoopBed, string, map[string]any) {
	t.Helper()
	b, dir, raw := designEvidenceBed(t, evidenceInventory, acceptanceUnits)
	b.lineage = b.goalFile(bedGoal).Claimed.Lineage
	root := b.job("rev1")
	root["reviewRoundLimit"] = 2
	b.writeJob(root)
	raw["findings"] = []any{evidenceFinding("A", "## Publication", ""), evidenceFinding("B", "## Collection:1", "")}
	raw["rigor"] = []any{evidenceRigor("A"), evidenceRigor("B")}
	raw["verdictMaterialCount"] = 2
	b.writeJSON(filepath.Join(dir, "return.json"), raw)
	b.writeFile(filepath.Join(dir, "return.md"), "VERDICT: REVISE material=2\n")
	answer := b.decide(b.review(), map[string]string{"A": "accepted | specified the requirement | ## Publication", "B": "refuted | searched reader.md for page and found line 3 | "})
	b.writeFile(b.design, strings.Replace(string(mustRead(t, b.design)), "First version.", "Second version.", 1))
	return b, answer, raw
}

func TestDesignReviewContinuationHoldHasReleaseAndAdmitsPerson(t *testing.T) {
	t.Parallel()
	for _, person := range []bool{false, true} {
		t.Run(map[bool]string{false: "agent", true: "person"}[person], func(t *testing.T) {
			t.Parallel()
			b, answer, _ := cutoverPositiveRead(t)
			conf := filepath.Join(b.install, "metasystem.conf")
			b.writeFile(conf, string(mustRead(t, conf))+"\nreview.stop=person\n")
			owners := b.intentBed.owners()
			owners.delivery = b.owners
			if person {
				now, _ := owners.commandNow(b.root())
				owners.prove = enrolledPersonProver(t, b.root(), now)
			}
			_, result := b.runJSON(owners, "design", "review", b.design, "--tool-calls", "30", "--dispositions", answer)
			if person {
				if result.Outcome != intentInProgress || len(b.followUps) != 1 {
					t.Fatalf("person held: %+v", result)
				}
			} else {
				if result.Outcome != intentInProgress || len(b.followUps) != 0 || result.Next == nil || !strings.Contains(result.Next.Reason, "release that hold to resume") || strings.Contains(strings.Join(result.Details, " "), "<nil>") {
					t.Fatalf("hold has no release: %+v", result)
				}
			}
		})
	}
}

func TestDesignReviewPolicyErrorNamesSettingRepair(t *testing.T) {
	t.Parallel()
	b, answer, _ := cutoverPositiveRead(t)
	conf := filepath.Join(b.install, "metasystem.conf")
	b.writeFile(conf, string(mustRead(t, conf))+"\nreview.stop=broken\n")
	result := b.review("--dispositions", answer)
	if result.Outcome != intentInProgress || len(b.followUps) != 0 || result.Next == nil || !strings.Contains(result.Next.Reason, "repair the review.stop setting") || strings.Contains(result.Next.Reason, "release") || len(result.Details) != 1 || !strings.Contains(result.Details[0], "review.stop") {
		t.Fatalf("setting error offered an ineffective remedy: %+v", result)
	}
	b.writeFile(conf, strings.Replace(string(mustRead(t, conf)), "review.stop=broken", "review.stop=auto", 1))
	if repaired := b.review("--dispositions", answer); repaired.Outcome != intentInProgress || len(b.followUps) != 1 {
		t.Fatalf("printed repair did not resume the prepared continuation: %+v", repaired)
	}
}

func TestDesignReviewRetainsAuthorDecisionsThroughCleanRead(t *testing.T) {
	t.Parallel()
	b, answer, raw := cutoverPositiveRead(t)
	if result := b.review("--dispositions", answer); result.Outcome != intentInProgress {
		t.Fatalf("continue: %+v", result)
	}
	workspace, err := filepath.EvalSymlinks(b.root())
	if err != nil {
		t.Fatal(err)
	}
	subject, present, err := dispatchcore.ComputeReadSubjectWithFacts(dispatchcore.ReadSubjectRequest{RepoRoot: b.install, Role: "design-critic", Workspace: workspace, RootJob: "rev1"}, admissionFacts{})
	if err != nil || !present {
		t.Fatalf("subject: %v", err)
	}
	dir := filepath.Join(b.install, "artifacts", "agents", "rev1", "rounds", "2")
	b.writeJSON(filepath.Join(dir, "subject.json"), subject)
	child := b.job("rev1-r2")
	child["engineBuild"], child["effectiveModel"] = "fixture-engine", "fixture-critic"
	b.writeJob(child)
	b.finish("rev1-r2", 2, "completed")
	raw["jobId"], raw["round"], raw["wholePageDigest"] = "rev1-r2", 2, subject.ContentDigest
	raw["findings"], raw["rigor"], raw["verdictMaterialCount"] = []any{}, []any{}, 0
	b.writeJSON(filepath.Join(dir, "return.json"), raw)
	b.writeFile(filepath.Join(dir, "return.md"), "VERDICT: REVISE material=0\n")
	result := b.collectHeld()
	for _, entry := range b.job("rev1")["findingRegister"].([]any) {
		f := entry.(map[string]any)
		want := map[string]string{"A": "accepted", "B": "refuted"}[f["findingId"].(string)]
		if want == "" || f["status"] != "resolved" || f["resolution"] != want {
			t.Fatalf("author decision rewritten: %v", f)
		}
	}
	template := b.decide(result, nil)
	// Drive the real register, durable mirror and closure readers.
	b.writeFile(filepath.Join(b.install, "metasystem.conf"), "evidence.root="+t.TempDir()+"\n")
	b.writeFile(filepath.Join(b.install, "artifacts", "agents", "capabilities", "close.json"), `{"ok":true}`)
	for _, job := range []string{"rev1", "rev1-r2"} {
		record := b.job(job)
		record["capabilitySnapshot"] = "artifacts/agents/capabilities/close.json"
		b.writeJob(record)
	}
	realCloseOwner(t, b.deliveryBed, func(ports *delegation.Ports) { ports.Git = designReviewGit{b.root()} })
	close := b.owners.closeOwner
	b.owners.closeOwner = func(root string, args []string) intentProcessResult { b.closes++; return close(root, args) }
	closed := b.review("--dispositions", template)
	if closed.Outcome != intentConfirmed || b.closes != 1 || len(b.goalFile(bedGoal).DesignExits) != 1 {
		t.Fatalf("clean follow-up did not publish: %+v", closed)
	}
	if _, present, err := readsubject.ReadClosedClosure(filepath.Join(b.install, "artifacts", "agents"), b.job("rev1"), []map[string]any{b.job("rev1"), b.job("rev1-r2")}); err != nil || !present {
		t.Fatalf("preserved decisions lost the earned closure: %v", err)
	}
	data, _ := json.Marshal(b.job("rev1")["findingRegister"])
	if strings.Contains(string(data), "withdrawn") {
		t.Fatalf("close rewrote author decisions: %s", data)
	}
}

func TestDesignJobClosePublishesCanonicalExit(t *testing.T) {
	t.Parallel()
	for _, route := range []string{"review", "finish", "renamed", "broken-subject", "positive-stop", "undecided", "multiple-roots", "child"} {
		t.Run(route, func(t *testing.T) {
			t.Parallel()
			b, dir, raw := designEvidenceBed(t, evidenceInventory, acceptanceUnits)
			b.lineage = b.goalFile(bedGoal).Claimed.Lineage
			rows := map[string]string{}
			if route == "positive-stop" || route == "undecided" {
				raw["findings"], raw["rigor"], raw["verdictMaterialCount"] = []any{evidenceFinding("M1", "## Publication", "")}, []any{evidenceRigor("M1")}, 1
				b.writeJSON(filepath.Join(dir, "return.json"), raw)
				b.writeFile(filepath.Join(dir, "return.md"), "VERDICT: REVISE material=1\n")
				if route == "positive-stop" {
					rows["M1"] = "accepted | specified the requirement | ## Publication"
				}
			}
			answer := b.decide(b.collectHeld(), rows)
			if route == "child" {
				child := b.job("rev1")
				child["jobId"], child["parentJob"], child["round"] = "rev1-r2", "rev1", 2
				b.writeJob(child)
			}
			if route == "multiple-roots" {
				other := b.job("rev1")
				other["jobId"] = "second-root"
				b.writeJob(other)
				b.writeJSON(filepath.Join(b.install, "artifacts", "agents", "second-root", "rounds", "1", "subject.json"), map[string]any{"kind": "design", "designPath": raw["designPath"], "designPage": string(mustRead(t, b.design))})
			}
			if route == "positive-stop" {
				mapping := concreteFoldMapping()
				mapping.Finding, mapping.Decision = "M1", "## Publication"
				b.writeFile(b.design, suppliedFoldPage(t, string(mustRead(t, b.design)), mapping))
			}
			if route == "renamed" || route == "broken-subject" {
				moved := filepath.Join(filepath.Dir(b.design), "moved.md")
				if err := os.Rename(b.design, moved); err != nil {
					t.Fatal(err)
				}
				b.design = moved
			}
			if route == "broken-subject" {
				b.writeFile(filepath.Join(dir, "subject.json"), "{")
			}
			action := "review"
			if route == "finish" {
				action = "finish"
			}
			args := []string{"work", action, "j2:rev1", "--dispositions", answer}
			if route == "child" {
				args[2] = "j2:rev1-r2"
			}
			if action == "review" {
				args = append(args, "--tool-calls", "30")
			}
			_, closed := b.do(args...)
			if route == "child" {
				if closed.Outcome != intentRefused || closed.Next == nil || !strings.Contains(strings.Join(closed.Next.Argv, " "), "work finish j2:rev1") || b.closes != 0 || len(b.goalFile(bedGoal).DesignExits) != 0 {
					t.Fatalf("child did not name its root: %+v", closed)
				}
				return
			}
			if route == "broken-subject" || route == "undecided" {
				if closed.Outcome != intentFailed && closed.Outcome != intentRefused || b.closes != 0 || len(b.goalFile(bedGoal).DesignExits) != 0 {
					t.Fatalf("invalid alias closed: %+v", closed)
				}
				return
			}
			if closed.Outcome != intentConfirmed || b.closes != 1 || len(b.goalFile(bedGoal).DesignExits) != 1 {
				t.Fatalf("alias skipped publication: %+v", closed)
			}
			exit := b.goalFile(bedGoal).DesignExits[0]
			if exit.Root != "rev1" || exit.State != "committed" || exit.Page != string(mustRead(t, b.design)) || !strings.Contains(exit.Page, "Status: accepted") {
				t.Fatalf("alias exit: %+v", exit)
			}
			if route == "positive-stop" && len(exit.Items) != 1 {
				t.Fatal("generic close lost the mandatory item")
			}
			_, replay := b.do(args...)
			if replay.Outcome != intentUnchanged || len(b.goalFile(bedGoal).DesignExits) != 1 || b.closes != 1 {
				t.Fatalf("alias replay: %+v", replay)
			}
		})
	}
}

func TestDesignReviewHeldAcceptanceAdmitsPerson(t *testing.T) {
	t.Parallel()
	for _, person := range []bool{false, true} {
		t.Run(map[bool]string{false: "agent", true: "person"}[person], func(t *testing.T) {
			t.Parallel()
			b, _, _ := designEvidenceBed(t, evidenceInventory, acceptanceUnits)
			b.lineage = b.goalFile(bedGoal).Claimed.Lineage
			answer := b.decide(b.collectHeld(), nil)
			conf := filepath.Join(b.install, "metasystem.conf")
			b.writeFile(conf, string(mustRead(t, conf))+"\nreview.stop=person\n")
			owners := b.intentBed.owners()
			owners.delivery = b.owners
			if person {
				now, _ := owners.commandNow(b.root())
				owners.prove = enrolledPersonProver(t, b.root(), now)
			}
			code, result := b.runJSON(owners, "design", "review", b.design, "--tool-calls", "30", "--dispositions", answer)
			if person {
				if code != 0 || result.Outcome != intentConfirmed || b.closes != 1 || len(b.goalFile(bedGoal).DesignExits) != 1 {
					t.Fatalf("person's acceptance held: %+v", result)
				}
			} else if result.Outcome != intentInProgress || result.Next == nil || !strings.Contains(result.Next.Reason, "release that hold to resume") || b.closes != 0 || len(b.goalFile(bedGoal).DesignExits) != 0 {
				t.Fatalf("agent crossed acceptance hold: %+v", result)
			}
		})
	}
}

func TestDesignLegacyResidueOffersExecutableRuling(t *testing.T) {
	t.Parallel()
	b := newDesignLoopBed(t)
	b.lineage = b.goalFile(bedGoal).Claimed.Lineage
	b.writeFile(b.design, string(mustRead(t, b.design))+acceptanceUnits)
	b.review()
	b.finish("rev1", 1, "completed", finding("F1", true, "an unresolved requirement"))
	b.register(1, 1, []int64{1}, map[string]any{"findingId": "F1", "rigorClass": "severe"})
	answer := b.decide(b.review(), map[string]string{"F1": "accepted | specify the requirement | section 2"})
	realCloseOwner(t, b.deliveryBed, func(ports *delegation.Ports) { ports.Git = designReviewGit{b.root()} })
	held := b.review("--dispositions", answer)
	message := fmt.Sprint(held.Data, held.Summary, held.Details)
	command := "metasystem design review '" + b.design + "' --ruling TEXT --reason TEXT --by NAME"
	if held.Outcome != intentRefused || !strings.Contains(message, command) || strings.Contains(message, "goal accept-risk") || b.job("rev1")["chainClosed"] == true || len(b.followUps) != 0 {
		t.Fatalf("legacy residue gave no executable person exit: %+v", held)
	}
	owners := b.intentBed.owners()
	owners.delivery = b.owners
	now, _ := owners.commandNow(b.root())
	owners.prove = enrolledPersonProver(t, b.root(), now)
	code, ruled := b.runJSON(owners, "design", "review", b.design, "--ruling", "accept the retained candidate", "--reason", "legacy advisory evidence", "--by", "Wido")
	if code != 0 || ruled.Outcome != intentConfirmed || len(b.goalFile(bedGoal).DesignExits) != 1 || len(b.followUps) != 0 || b.fresh != 1 {
		t.Fatalf("printed ruling did not take effect: %+v", ruled)
	}
	if page := string(mustRead(t, b.design)); !strings.Contains(page, "Critique: ruled by") || strings.Contains(page, "on 0 material") {
		t.Fatalf("ruling fabricated a clean examination: %s", page)
	}
}
