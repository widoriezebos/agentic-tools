package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/loopstop"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/readsubject"
)

func TestDesignReviewKeepsCanonicalHistory(t *testing.T) {
	t.Parallel()
	for _, state := range []string{"running", "closed", "lost-entry", "broken-entry", "broken-index", "lost-subject", "unrelated-legacy", "broken-subject", "broken-subject-lost-entry", "broken-subject-uncollected"} {
		t.Run(state, func(t *testing.T) {
			t.Parallel()
			b, dir, _ := designEvidenceBed(t, evidenceInventory)
			if result := b.review(); result.Outcome != intentConfirmed {
				t.Fatalf("collection: %+v", result)
			}
			root := b.job("rev1")
			switch state {
			case "unrelated-legacy":
				b.writeJob(map[string]any{"jobId": "other-design", "role": "design-critic", "goalId": bedGoal, "design": "missing-other-design.md", "round": 1})
			case "running":
				root["status"] = "running"
			case "closed":
				root["chainClosed"] = true
			case "lost-entry":
				if err := os.Remove(filepath.Join(b.install, "artifacts", "agents", "intent-review", "design-01designreader", "chain.json")); err != nil {
					t.Fatal(err)
				}
			case "broken-entry":
				b.writeFile(filepath.Join(b.install, "artifacts", "agents", "intent-review", "design-01designreader", "chain.json"), "{")
			case "broken-index":
				b.writeFile(filepath.Join(b.install, "artifacts", "agents", "jobs", "unknown.json"), "{")
			case "broken-subject", "broken-subject-lost-entry", "broken-subject-uncollected":
				if state == "broken-subject-uncollected" {
					delete(root, "read")
				}
				b.writeFile(filepath.Join(dir, "subject.json"), "{")
				if state == "broken-subject-lost-entry" {
					if err := os.Remove(filepath.Join(b.install, "artifacts", "agents", "intent-review", "design-01designreader", "chain.json")); err != nil {
						t.Fatal(err)
					}
				}
			case "lost-subject":
				if err := os.Remove(filepath.Join(dir, "subject.json")); err != nil {
					t.Fatal(err)
				}
			}
			b.writeJob(root)
			moved := filepath.Join(filepath.Dir(b.design), "renamed.md")
			if err := os.Rename(b.design, moved); err != nil {
				t.Fatal(err)
			}
			b.design = moved
			// A second root must be observable even though this fixture's
			// default dispatcher rejoins its one admitted operation.
			b.handler = func(p intentProcess) intentProcessResult {
				t.Fatalf("renaming or unreadable history reached fresh dispatch: %v", p.argv)
				return intentProcessResult{}
			}
			result := b.review()
			if state == "running" && (result.Outcome != intentInProgress || !strings.Contains(result.Summary, "running")) {
				t.Fatalf("renamed page lost its live root: %+v", result)
			}
			if (state == "lost-entry" || state == "unrelated-legacy") && result.Outcome != intentConfirmed {
				t.Fatalf("retained canonical subject did not recover the entry: %+v", result)
			}
			if strings.HasPrefix(state, "broken") || state == "lost-subject" {
				if result.Outcome != intentFailed || !strings.Contains(result.Summary, "unknown") {
					t.Fatalf("unknown history became a new chain: %+v", result)
				}
			}
			if b.fresh != 1 || len(b.followUps) != 0 || b.closes != 0 {
				t.Fatal("canonical lookup changed execution or closure")
			}
		})
	}
}

func TestDesignReviewRetriesUnknownEvidenceOnce(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"recover", "reservation-recover", "retry-recover", "double-fill", "source-first", "retry-exhausted", "failed", "moved-candidate", "deadline"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			b, dir, returned := designEvidenceBed(t, evidenceInventory, acceptanceUnits)
			original := mustRead(t, b.design)
			prose := mustRead(t, filepath.Join(dir, "return.md"))
			b.writeFile(filepath.Join(dir, "return.md"), "unreadable advisory evidence\n")
			stopped := b.review()
			if stopped.Outcome != intentFailed || !strings.Contains(stopped.Summary, "unknown") {
				t.Fatalf("unknown collection: %+v", stopped)
			}
			before := b.job("rev1")["criticRoundsConsumed"]
			if scenario == "recover" {
				b.writeFile(filepath.Join(dir, "return.md"), string(prose))
				if result := b.review(); result.Outcome != intentConfirmed || len(b.followUps) != 0 || b.job("rev1")["designStop"] != nil || b.job("rev1")["findingRegisterStop"] != nil {
					t.Fatalf("retained collection did not recover: %+v", result)
				}
				return
			}
			if scenario == "reservation-recover" {
				b.handler = func(intentProcess) intentProcessResult {
					return intentProcessResult{stdout: []byte(`{"outcome":"RECONCILING"}`)}
				}
				if pending := b.review("--retry", "1"); pending.Outcome != intentInProgress || b.job("rev1")["unknownExaminationRetryFrom"] != "rev1" {
					t.Fatalf("retry intent was not retained before dispatch: %+v", pending)
				}
				b.writeFile(filepath.Join(dir, "return.md"), string(prose))
				recovered := b.review("--retry", "1")
				if recovered.Outcome != intentConfirmed || recovered.Next == nil || strings.Contains(strings.Join(recovered.Next.Argv, " "), "--retry") || b.job("rev1")["findingRegisterStop"] != nil || b.job("rev1")["criticRoundsConsumed"] != float64(1) {
					t.Fatalf("reserved source recovery did not restore its read and executable decisions remedy: %+v", recovered)
				}
				if _, err := os.Stat(filepath.Join(dir, "read.json")); err != nil {
					t.Fatalf("recovered immutable read was not retained: %v", err)
				}
				return
			}
			if scenario == "failed" {
				b.finish("rev1", 1, "failed")
				if err := os.Remove(filepath.Join(dir, "return.json")); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "moved-candidate" {
				b.writeFile(b.design, strings.Replace(string(original), "First version.", "Another candidate.", 1))
			}
			if scenario == "deadline" {
				root := b.job("rev1")
				root["status"], root["error"], root["groupDeathProvenAt"] = "timeout", "budget-cap", "2026-09-25T12:00:00Z"
				b.writeJob(root)
			}
			dispatch := b.handler
			b.handler = func(p intentProcess) intentProcessResult {
				if flagValue(p.argv, "--follow-up") == "rev1" {
					if err := dispatchcore.ReservedUnknownExaminationRetry(b.install, "rev1"); err != nil {
						t.Fatalf("production retry admission: %v", err)
					}
					if _, err := dispatchcore.CritiqueRegisterAdvance(b.install, "rev1", "rev1"); err != nil {
						t.Fatalf("production retry register replay: %v", err)
					}
				}
				result := dispatch(p)
				if flagValue(p.argv, "--follow-up") == "rev1" {
					child := b.job("rev1-r2")
					child["examinationRetryOf"], child["engineBuild"], child["effectiveModel"] = "rev1", "fixture-engine", "fixture-critic"
					b.writeJob(child)
					b.writeFile(filepath.Join(b.install, "artifacts", "agents", "rev1", "rounds", "2", "subject.json"), string(mustRead(t, filepath.Join(dir, "subject.json"))))
				}
				return result
			}
			retry := b.review("--retry", "1")
			if scenario == "moved-candidate" || scenario == "deadline" {
				if retry.Outcome != intentFailed || len(b.followUps) != 0 {
					t.Fatalf("different candidate or deadline retried: %+v", retry)
				}
				return
			}
			if retry.Outcome != intentInProgress || len(b.followUps) != 1 || b.job("rev1")["unknownExaminationRetryFrom"] != "rev1" {
				t.Fatalf("single retry was not reserved: %+v", retry)
			}
			if replay := b.review("--retry", "1"); replay.Outcome != intentInProgress || len(b.followUps) != 1 {
				t.Fatalf("retry replay launched twice: %+v", replay)
			}
			if scenario == "source-first" {
				b.writeFile(filepath.Join(dir, "return.md"), string(prose))
				if _, err := dispatchcore.CritiqueRegisterAdvance(b.install, "rev1", "rev1"); err != nil {
					t.Fatal(err)
				}
				before = b.job("rev1")["criticRoundsConsumed"]
			}
			b.finish("rev1-r2", 2, "completed")
			next := filepath.Join(b.install, "artifacts", "agents", "rev1", "rounds", "2")
			returned["jobId"], returned["round"] = "rev1-r2", 2
			b.writeJSON(filepath.Join(next, "return.json"), returned)
			b.writeFile(filepath.Join(next, "return.md"), "unreadable advisory evidence\n")
			if scenario == "retry-recover" || scenario == "double-fill" || scenario == "source-first" {
				b.writeFile(filepath.Join(next, "return.md"), string(prose))
				if result := b.review(); result.Outcome != intentConfirmed || b.job("rev1")["criticRoundsConsumed"] != before || b.job("rev1")["designStop"] != nil {
					t.Fatalf("recovered retry charged a completed examination or lost original section history: %+v", result)
				}
				if scenario == "source-first" {
					root := b.job("rev1")
					if len(root["designExaminations"].([]any)) != 1 || root["read"].(map[string]any)["id"] != "rev1" || root["findingRegisterRound"] != float64(2) {
						t.Fatalf("retry filled the recovered source twice: %v", root)
					}
					return
				}
				var read readsubject.Read
				if err := json.Unmarshal(mustRead(t, filepath.Join(next, "read.json")), &read); err != nil || read.Design == nil || read.Design.Root != "rev1" || read.Subject.DesignPage != string(original) {
					t.Fatalf("retry's immutable evidence: %+v %v", read, err)
				}
				if scenario == "double-fill" {
					root := b.job("rev1")
					before := mustRead(t, filepath.Join(b.install, "artifacts", "agents", "jobs", "rev1.json"))
					b.writeFile(filepath.Join(dir, "return.md"), string(prose))
					if err := dispatchcore.RecordDesignEvidenceStop(b.install, "rev1", loopstop.Stop{Loop: "design-round", Subject: "rev1", Decision: "stop", Class: "unknown design evidence", Handoff: "stopped unreadable-design-evidence"}); err != nil {
						t.Fatal(err)
					}
					if _, err := dispatchcore.CritiqueRegisterAdvance(b.install, "rev1", "rev1"); err != nil {
						t.Fatal(err)
					}
					after := b.job("rev1")
					if after["findingRegisterRound"] != root["findingRegisterRound"] || len(after["designExaminations"].([]any)) != 1 || after["read"].(map[string]any)["id"] != "rev1-r2" {
						t.Fatalf("recovered source filled the retry's examination twice: before=%s after=%v", before, after)
					}
				}
				return
			}
			stopped = b.review()
			if stopped.Outcome != intentFailed || !strings.Contains(stopped.Summary, "stopped") || b.job("rev1")["criticRoundsConsumed"] != before {
				t.Fatalf("exhausted unknown retry: %+v", stopped)
			}
			for _, field := range []string{"designStop", "findingRegisterStop"} {
				encoded, _ := json.Marshal(b.job("rev1")[field])
				var stop loopstop.Stop
				if err := json.Unmarshal(encoded, &stop); err != nil || stop.Decision != "stop" || !strings.HasPrefix(stop.Handoff, "stopped ") || stop.Subject != "rev1" || stop.Tree != digestText(original) || !strings.Contains(stop.Command(), "--ruling TEXT --reason TEXT --by NAME") || !strings.Contains(stop.Command(), "--scope FILE --reason TEXT --by NAME") {
					t.Fatalf("%s lacks the bound stop/person remedies: %+v %v", field, stop, err)
				}
			}
			if _, err := dispatchcore.CritiqueRegisterClose(b.install, "rev1"); err == nil {
				t.Fatal("unknown register closed")
			}
			if result := b.review("--retry", "2"); result.Outcome != intentFailed || len(b.followUps) != 1 || b.closes != 0 || b.job("rev1")["chainClosed"] == true {
				t.Fatalf("exhaustion admitted a third execution or closure: %+v", result)
			}
			owners := b.intentBed.owners()
			owners.delivery = b.owners
			if err := os.MkdirAll(filepath.Join(b.root(), ".git"), 0700); err != nil {
				t.Fatal(err)
			}
			var display, displayErr bytes.Buffer
			show, rest, ok := resolveIntentArgv([]string{"design", "review", b.design, "--tool-calls", "30"})
			code := runIntentIn(show, rest, &display, &displayErr, b.root(), owners)
			words := display.String() + displayErr.String()
			if !ok || code != 1 || !strings.Contains(words, "--ruling TEXT --reason TEXT --by NAME") || !strings.Contains(words, "--scope FILE --reason TEXT --by NAME") {
				t.Fatalf("display lacks executable remedies: %s %s", display.String(), displayErr.String())
			}
			now, _ := owners.commandNow(b.root())
			owners.prove = enrolledPersonProver(t, b.root(), now)
			if stopped.Next == nil {
				t.Fatal("exhausted stop omitted its person remedy")
			}
			args := append([]string{}, stopped.Next.Argv[1:]...)
			for i := range args {
				if args[i] == "TEXT" {
					args[i] = "bounded first use"
				} else if args[i] == "NAME" {
					args[i] = "Wido"
				}
			}
			args = append(args, "--json")
			var out bytes.Buffer
			impact := false
			stderr := &unitImpactWriter{observe: func(text string) {
				if strings.Contains(text, "Impact:") {
					impact = true
					if !bytes.Equal(original, mustRead(t, b.design)) || len(b.goalFile(bedGoal).DesignExits) != 0 {
						t.Fatal("ruling effect preceded impact")
					}
				}
			}}
			command, rest, ok := resolveIntentArgv(args)
			if !ok {
				t.Fatalf("printed ruling is not executable: %v", args)
			}
			if code := runIntentIn(command, rest, &out, stderr, b.root(), owners); code != 0 || !impact || b.job("rev1")["chainClosed"] != true || !strings.Contains(string(mustRead(t, b.design)), "Critique: ruled by Wido") || len(b.followUps) != 1 || b.job("rev1")["criticRoundsConsumed"] != before {
				t.Fatalf("printed ruling failed on unreadable evidence: exit=%d output=%s", code, out.String())
			}
		})
	}
}
