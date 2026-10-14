package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/project"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/readsubject"
)

func testDesignReviewPersonActs(t *testing.T) {
	t.Parallel()
	for _, form := range []string{"scope", "ruling"} {
		t.Run(form, func(t *testing.T) {
			t.Parallel()
			b, dir, _ := designEvidenceBed(t, evidenceInventory, acceptanceUnits)
			if err := os.MkdirAll(filepath.Join(b.root(), ".git"), 0700); err != nil {
				t.Fatal(err)
			}
			prior := mustRead(t, b.design)
			file := b.goalFile(bedGoal)
			file.ReviewObligations = append(file.ReviewObligations, goal.ReviewObligation{Finding: "required-item", Chain: "rev1", Artifact: "reader.go", Test: "TestUnknownEvidence", Fixture: "TestUnknownEvidence", State: "open"})
			b.addGoal(file)
			at, _ := b.commandNow(b.root())
			var questions []string
			for _, subject := range []string{"rev1", "another-root"} {
				q, err := channel.Ask(channel.AskRequest{RepoRoot: b.install, Goal: bedGoal, Kind: "stop", Facts: []string{"unknown evidence"}, Now: at, UnitStop: &channel.UnitStopQuestion{Loop: "design-round", Subject: subject, Attempt: 1, Finding: "unknown", Needs: "metasystem design review FILE --ruling TEXT --reason TEXT --by NAME", AcceptableActs: []string{"design-ruling"}}})
				if err != nil {
					t.Fatal(err)
				}
				questions = append(questions, q.ID)
			}
			b.writeFile(filepath.Join(dir, "return.json"), "unreadable advisory return")
			b.writeFile(filepath.Join(dir, "return.md"), "unreadable advisory prose")
			replacement := filepath.Join(b.root(), "scope.md")
			b.writeFile(replacement, strings.Replace(string(prior), "First version.", "Specified replacement scope.", 1))
			value := "accept despite unknown evidence (ruling advisory note)"
			if form == "scope" {
				value = replacement
			}
			args := []string{"design", "review", b.design, "--" + form, value, "--reason", "bounded first use", "--by", "Wido", "--json"}
			owners := b.intentBed.owners()
			owners.delivery = b.owners
			owners.delivery.designRecord = func(string) (intentDesignRecord, []byte, error) {
				t.Fatal("design page read before person proof")
				return intentDesignRecord{}, nil, nil
			}
			owners.delivery.designChains = func(string, string, string) []dispatchcore.DesignCritiqueChain {
				t.Fatal("critique records read before person proof")
				return nil
			}
			owners.delivery.examinationRead = func(string, string) (readsubject.Read, error) {
				t.Fatal("advisory return read before person proof")
				return readsubject.Read{}, nil
			}
			for _, proof := range []goalAuthorityProver{fixedFixtureGoalAuthority, func(root string, _ int64, _ humanauthority.Reader, _, _ string, at time.Time) (humanauthority.Proof, error) {
				return humanauthority.HelmProof(root, humanauthority.HelmGrant{By: "Wido", Grant: "grant"}, at)
			}} {
				owners.prove = proof
				code, result := b.runJSON(owners, args...)
				if code == 0 || result.Outcome != intentRefused || !strings.Contains(result.Summary, "enrolled terminal") || result.Next == nil || !strings.Contains(strings.Join(result.Next.Argv, " "), "design review") {
					t.Fatalf("agent acquired person's %s power: %d %+v", form, code, result)
				}
			}
			owners.delivery.designRecord = readIntentDesignRecord
			owners.delivery.designChains = dispatchcore.DesignCritiqueChains
			owners.delivery.examinationRead = dispatchcore.CollectExamination
			now, _ := owners.commandNow(b.root())
			owners.prove = enrolledPersonProver(t, b.root(), now)
			forged := append([]string(nil), args...)
			for i := range forged {
				if forged[i] == "Wido" {
					forged[i] = "Someone else"
				}
			}
			if code, _ := b.runJSON(owners, forged...); code == 0 {
				t.Fatal("forged --by authorized the act")
			}
			observer := &designAcceptanceRepository{Repository: b.repo}
			endpoint := owners.dependencies.endpoint
			owners.dependencies.endpoint = func(root string) (goal.Endpoint, error) {
				e, err := endpoint(root)
				e.Repository = observer
				return e, err
			}
			var out bytes.Buffer
			impactSeen := false
			stderr := &unitImpactWriter{observe: func(text string) {
				if !strings.Contains(text, "Impact:") {
					return
				}
				impactSeen = true
				if !bytes.Equal(prior, mustRead(t, b.design)) || len(b.goalFile(bedGoal).DesignExits) != 0 {
					t.Fatal("impact followed an effect")
				}
				paths, _ := filepath.Glob(filepath.Join(b.install, "artifacts", "design-person", "*.json"))
				if len(paths) != 1 {
					t.Fatalf("prior intent not retained: %v", paths)
				}
				if !bytes.Equal(prior, mustRead(t, paths[0]+".prior.md")) {
					t.Fatal("undo page was not retained before effect")
				}
				var retained goal.DesignExit
				if err := json.Unmarshal(mustRead(t, paths[0]), &retained); err != nil || retained.Expected != string(prior) || retained.Who != "Wido" || retained.Impact == "" {
					t.Fatalf("retained impact: %+v %v", retained, err)
				}
			}}
			observer.before = func() error {
				if !impactSeen {
					t.Fatal("goal publication preceded printed impact")
				}
				return errors.New("storage unavailable")
			}
			command, rest, ok := resolveIntentArgv(args)
			if !ok {
				t.Fatal("person public form missing")
			}
			if code := runIntentIn(command, rest, &out, stderr, b.root(), owners); code == 0 {
				t.Fatal("failed publication reported success")
			}
			if !impactSeen || !bytes.Equal(prior, mustRead(t, b.design)) || b.job("rev1")["chainClosed"] == true {
				t.Fatal("failed effect changed page or closed critique")
			}
			observer.before = nil
			code, result := b.runJSON(owners, args...)
			if code != 0 || result.Outcome != intentConfirmed {
				t.Fatalf("person act on unknown advisory: %d %+v", code, result)
			}
			file = b.goalFile(bedGoal)
			page := mustRead(t, b.design)
			if len(file.DesignExits) != 1 || file.DesignExits[0].Who != "Wido" || file.DesignExits[0].Reason != "bounded first use" || file.DesignExits[0].Page != string(page) || !strings.Contains(string(page), "Critique: ruled by Wido") || strings.Contains(string(page), "0 material") || b.job("rev1")["chainClosed"] != true {
				t.Fatalf("ruled publication: %+v page=%s", file.DesignExits, page)
			}
			if form == "scope" && !strings.Contains(string(page), "Specified replacement scope.") {
				t.Fatal("scope did not take effect")
			}
			if len(file.ReviewObligations) != 1 || file.ReviewObligations[0].State != "open" {
				t.Fatal("ruling discharged implementation proof")
			}
			for i, id := range questions {
				q, err := channel.ReadQuestion(b.install, id)
				want := "closed"
				if i == 1 {
					want = "open"
				}
				if err != nil || q.State != want {
					t.Fatalf("held question %s: %+v %v", id, q, err)
				}
			}
			acceptanceBuild(t, b, true)
			if !strings.Contains(string(mustRead(t, filepath.Join(dir, "return.json"))), "unreadable advisory") || b.fresh != 1 || len(b.followUps) != 0 {
				t.Fatal("person act invented a clean examination")
			}
			if code, result := b.runJSON(owners, args...); code != 0 || len(b.goalFile(bedGoal).DesignExits) != 1 {
				t.Fatalf("person replay: %d %+v exits=%+v", code, result, b.goalFile(bedGoal).DesignExits)
			}
			paths, _ := filepath.Glob(filepath.Join(b.install, "artifacts", "design-person", "*.prior.md"))
			if len(paths) != 1 {
				t.Fatalf("undo scope: %v", paths)
			}
			if code, result := b.runJSON(owners, "design", "review", b.design, "--scope", paths[0], "--reason", "undo the prior act", "--by", "Wido"); code != 0 {
				t.Fatalf("printed undo route failed: %d %+v", code, result)
			}
			priorBody, _ := project.DesignBodyDigest(b.design, prior)
			restoredBody, _ := project.DesignBodyDigest(b.design, mustRead(t, b.design))
			if priorBody != restoredBody || len(b.goalFile(bedGoal).DesignExits) != 2 || b.fresh != 1 {
				t.Fatal("undo lost prior scope, history, or spent examinations")
			}
			if code, result := b.runJSON(owners, args...); code != 0 || result.Outcome != intentConfirmed || len(b.goalFile(bedGoal).DesignExits) != 3 {
				t.Fatalf("same act after undo: %d %+v exits=%+v", code, result, b.goalFile(bedGoal).DesignExits)
			}
			acceptanceBuild(t, b, true)
		})
	}
}

func testDesignReviewConcreteFoldPublishesExit(t *testing.T) {
	t.Parallel()
	b, dir, returned := designEvidenceBed(t, evidenceInventory, acceptanceUnits)
	if err := os.MkdirAll(filepath.Join(b.root(), ".git"), 0700); err != nil {
		t.Fatal(err)
	}
	b.lineage = b.goalFile(bedGoal).Claimed.Lineage
	returned["findings"], returned["rigor"], returned["verdictMaterialCount"] = []any{evidenceFinding("F1", "## Collection:1", "")}, []any{evidenceRigor("F1")}, 1
	b.writeJSON(filepath.Join(dir, "return.json"), returned)
	b.writeFile(filepath.Join(dir, "return.md"), "VERDICT: REVISE material=1\n")
	template := b.decide(b.review(), map[string]string{"F1": "accepted | concrete requirement | ## Collection:1"})
	// Decision 4 requires the supplied Decision passage and its unit test/checklist mapping.
	b.writeFile(b.design, suppliedFoldPage(t, string(mustRead(t, b.design)), concreteFoldMapping()))
	expected := mustRead(t, b.design)
	closed := b.review("--dispositions", template)
	file := b.goalFile(bedGoal)
	if closed.Outcome != intentConfirmed || b.job("rev1")["chainClosed"] != true || len(file.DesignExits) != 1 {
		t.Fatalf("concrete fold did not publish its exit: %+v exits=%+v", closed, file.DesignExits)
	}
	exit := file.DesignExits[0]
	if exit.Expected != string(expected) || exit.Page != string(mustRead(t, b.design)) || !strings.Contains(exit.Page, "on 1 material findings folded as 1 unit acceptance items") {
		t.Fatalf("fold exit lost exact candidate/count: %+v", exit)
	}
	acceptanceBuild(t, b, true)
	if again := b.review("--dispositions", template); again.Outcome != intentUnchanged || len(b.goalFile(bedGoal).DesignExits) != 1 {
		t.Fatalf("fold replay: %+v", again)
	}
}

func TestDesignReviewPersonInvalidUnits(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, units, reason string
	}{
		{"missing table", "", "no sized units table"},
		{"bad estimate", strings.Replace(acceptanceUnits, "40", "unknown", 1), "no number in its total size column"},
		{"empty table", "\n## Units\n\n| Unit | Lines | Production lines |\n| --- | --- | --- |\n", "no sized units table"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			b, _, _ := designEvidenceBed(t, evidenceInventory, tc.units)
			if err := os.MkdirAll(filepath.Join(b.root(), ".git"), 0700); err != nil {
				t.Fatal(err)
			}
			owners := b.intentBed.owners()
			owners.delivery = b.owners
			now, _ := owners.commandNow(b.root())
			owners.prove = enrolledPersonProver(t, b.root(), now)
			prior := mustRead(t, b.design)
			code, result := b.runJSON(owners, "design", "review", b.design, "--ruling", "accept this scope", "--reason", "bounded first use", "--by", "Wido")
			if code == 0 || !strings.Contains(result.Summary, tc.reason) || !bytes.Equal(prior, mustRead(t, b.design)) || len(b.goalFile(bedGoal).DesignExits) != 0 || b.job("rev1")["chainClosed"] == true {
				t.Fatalf("invalid units published or lost their refusal reason: %d %+v", code, result)
			}
		})
	}
}
