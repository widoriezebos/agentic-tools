package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/loopstop"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/readsubject"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

// stopReadStarter supplies the external critic's immutable output; collection
// and the unit decision remain the production owners.
type stopReadStarter struct {
	bed         *workBed
	reads       [][]readsubject.Finding
	next        int
	beforeBuild func()
	stderr      *bytes.Buffer
	corrupt     string
}

func (s *stopReadStarter) StartSupervisor(id, state string) (identity.Ref, error) {
	record, err := s.bed.manager.Store.Read(id)
	if err != nil {
		return identity.Ref{}, err
	}
	if record.Kind == "build" && s.beforeBuild != nil {
		s.beforeBuild()
	}
	ref, err := s.bed.starter.StartSupervisor(id, state)
	if err != nil || record.Kind != "read" {
		return ref, err
	}
	index := s.next
	if index >= len(s.reads) {
		index = len(s.reads) - 1
	}
	findings := s.reads[index]
	findings = append([]readsubject.Finding(nil), findings...)
	if s.corrupt == "missing-class" && len(findings) > 0 {
		findings[0].Class = ""
	}
	s.next++
	count := 0
	for _, finding := range findings {
		if finding.Material {
			count++
		}
	}
	if findings == nil {
		findings = []readsubject.Finding{}
	}
	data, err := json.Marshal(map[string]any{"findings": findings, "verdictMaterialCount": count})
	if err != nil {
		return ref, err
	}
	if s.corrupt == "missing-class" {
		var raw map[string]any
		if err := json.Unmarshal(data, &raw); err != nil {
			return ref, err
		}
		rows := raw["findings"].([]any)
		if len(rows) > 0 {
			delete(rows[0].(map[string]any), "class")
		}
		data, err = json.Marshal(raw)
		if err != nil {
			return ref, err
		}
	}
	path := filepath.Join(state, "return.json")
	if err := os.MkdirAll(state, 0700); err != nil {
		return ref, err
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return ref, err
	}
	verdict := "LAND"
	if count > 0 {
		verdict = fmt.Sprintf("material=%d", count)
	}
	if s.corrupt == "prose-disagrees" {
		verdict = "LAND"
	}
	if err := os.WriteFile(filepath.Join(state, "report.md"), []byte("VERDICT: "+verdict+"\n"), 0600); err != nil {
		return ref, err
	}
	_, err = s.bed.manager.Store.Update(id, func(r *launch.Record) error {
		r.Outputs = append(r.Outputs, launch.Output{Path: path}, launch.Output{Path: filepath.Join(state, "report.md")})
		r.Measurement.Verdict = verdict
		return nil
	})
	return ref, err
}

func stopFinding(class, where string) readsubject.Finding {
	return readsubject.Finding{Class: class, Where: where, Severity: "high", Material: true, Claim: "The required behavior is missing", Evidence: where + ":12", Change: "Implement the required behavior"}
}

func newStopWorkBed(t *testing.T) *workBed {
	t.Helper()
	b := newWorkBedWith(t, func(file *goal.GoalFile) {
		workApprovedBox(file)
		file.Budget.ReviewRoundLimit = 6
		file.Approved.Digest = goal.ApprovalDigest(file.Intent, file.Tier, *file.Budget, file.Risk)
	})
	b.manager.Settings.UnitCountedRounds = 6
	for index, value := range b.manager.Settings.Values {
		if value.Key == launch.UnitCountedRoundsKey {
			b.manager.Settings.Values[index].Value = "6"
		}
	}
	return b
}

func stopPublic(t *testing.T, b *workBed, policy string, args ...string) (int, intentResult, string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(b.root(), ".git"), 0700); err != nil {
		t.Fatal(err)
	}
	owners := b.workOwners()
	units := owners.work.units
	owners.work.units = func(layout stateroot.Layout) *launch.UnitRunner {
		runner := units(layout)
		runner.ReviewPolicy = nil
		return runner
	}
	owners.policies.Registry = func(string) (config.PolicyRegistry, error) { return config.PolicyRegistry{}, nil }
	owners.lookupEnv = func(key string) (string, bool) { return policy, key == config.EnvName("review.stop") }
	command, rest, ok := resolveIntentArgv(args)
	if !ok {
		t.Fatalf("missing public command: %v", args)
	}
	var out, errors bytes.Buffer
	if starter, ok := b.manager.Supervisor.(*stopReadStarter); ok {
		starter.stderr = &errors
	}
	code := runIntentIn(command, append([]string{"--json"}, rest...), &out, &errors, b.root(), owners)
	var result intentResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("public command %v: %v %q %q", args, err, out.String(), errors.String())
	}
	b.recordReadDirs(result)
	return code, result, errors.String()
}

func stopBuild(t *testing.T, b *workBed, policy string) (string, launch.UnitRunRecord, string) {
	t.Helper()
	brief := b.brief("stop-build.md", "Read each round: yes\nBuild the unit.\n")
	check := workCheck
	code, result, _ := stopPublic(t, b, policy, append([]string{"work", "build", b.id, "stopped", "--brief", brief, "--lines", "5"}, check...)...)
	if code != 0 {
		t.Fatalf("build: %d %+v", code, result)
	}
	run := resultData(t, result)["run"].(string)
	record, err := (&launch.UnitRunner{Root: b.unitRoot}).Status(run)
	if err != nil {
		t.Fatal(err)
	}
	return run, record, brief
}

func TestIntentUnitCleanReadAdmitsOnlyReasonedPersonRevision(t *testing.T) {
	t.Parallel()
	b := newStopWorkBed(t)
	s := &stopReadStarter{bed: b, reads: [][]readsubject.Finding{{}}}
	b.manager.Supervisor = s
	run, before, brief := stopBuild(t, b, "auto")
	if before.Rounds[0].Stop == nil || before.Rounds[0].Stop.Decision != "close" || len(before.Rounds[0].Reads) != 1 {
		t.Fatalf("read did not close: %+v", before)
	}
	launches := len(b.starter.launched())
	code, result, _ := stopPublic(t, b, "auto", "work", "revise", b.id, "--work", "stopped", "--brief", brief)
	if code == 0 || len(b.starter.launched()) != launches {
		t.Fatalf("clean automatic revise launched: %d %+v", code, result)
	}
	s.beforeBuild = func() {
		if s.stderr == nil || !strings.Contains(s.stderr.String(), "Impact:") {
			t.Error("builder started before impact was printed")
		}
		impacts, _ := filepath.Glob(filepath.Join(b.root(), "artifacts", "agents", "channel", "unit-stop-overrides", "*.json"))
		if len(impacts) != 1 {
			t.Errorf("builder started without prior recorded impact: %v", impacts)
		}
	}
	code, result, stderr := stopPublic(t, b, "broken-policy", "work", "revise", b.id, "--work", "stopped", "--brief", brief, "--reason", "Correct despite the advisory read", "--by", "Wido")
	if code != 0 || !strings.Contains(stderr, "Impact:") || len(b.starter.launched()) <= launches {
		t.Fatalf("person revision did not take effect: %d %+v %q", code, result, stderr)
	}
	after, err := (&launch.UnitRunner{Root: b.unitRoot}).Status(run)
	if err != nil || after.CorrectionBudget == nil || *after.CorrectionBudget != *before.CorrectionBudget || len(after.Rounds) != 2 || after.Revisions[0].Person != "Wido" {
		t.Fatalf("person reset automatic history: %+v %v", after, err)
	}
}

func TestIntentUnitStopIsRecordedBeforeAnotherRevision(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name string
		next []readsubject.Finding
	}{
		{"equal", []readsubject.Finding{stopFinding("scope", "d.go"), stopFinding("missing-reader", "e.go"), stopFinding("false-premise", "f.go")}},
		{"repeated", []readsubject.Finding{stopFinding("regression", "new.go"), stopFinding("missing-reader", "reader.go")}},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			b := newStopWorkBed(t)
			b.manager.Supervisor = &stopReadStarter{bed: b, reads: [][]readsubject.Finding{{stopFinding("regression", "a.go"), stopFinding("incomplete-item", "b.go"), stopFinding("weakened-test", "c_test.go")}, row.next}}
			run, record, _ := stopBuild(t, b, "auto")
			if record.Rounds[0].Stop == nil || record.Rounds[0].Stop.Decision != "continue" {
				t.Fatalf("initial read: %+v", record)
			}
			if record.CountedCap != 6 || record.MaxRounds != 6 || record.CorrectionBudget == nil || *record.CorrectionBudget != 2 {
				t.Fatalf("stop test was masked by an admission cap: %+v", record)
			}
			var text strings.Builder
			text.WriteString("Correct the findings.\n\n## Decisions on round 1\n\n| Finding | Decision | Evidence |\n| --- | --- | --- |\n")
			for _, f := range record.Rounds[0].Reads[0].Findings {
				fmt.Fprintf(&text, "| %s | fixed | %s:12 |\n", f.ID, f.Where)
			}
			brief := b.brief("correction.md", text.String())
			code, result, _ := stopPublic(t, b, "auto", "work", "revise", b.id, "--work", "stopped", "--brief", brief)
			if code != 0 {
				t.Fatalf("first correction: %d %+v", code, result)
			}
			statusCode, status, _ := stopPublic(t, b, "auto", "work", "status", b.id, "--work", "stopped")
			if statusCode != 0 || !strings.Contains(fmt.Sprint(status.Data), "stop") {
				t.Fatalf("status hid stop: %d %+v", statusCode, status)
			}
			record, err := (&launch.UnitRunner{Root: b.unitRoot}).Status(run)
			if err != nil || record.Rounds[1].Stop == nil || record.Rounds[1].Stop.Decision != "stop" {
				t.Fatalf("correction has no recorded stop: %+v %v", record, err)
			}
			launches := len(b.starter.launched())
			code, result, _ = stopPublic(t, b, "auto", "work", "revise", b.id, "--work", "stopped", "--brief", b.brief("again.md", "Correct again.\n"))
			if code == 0 || len(b.starter.launched()) != launches {
				t.Fatalf("stopped correction launched: %d %+v", code, result)
			}
		})
	}
}

func TestIntentUnitReviewPolicyStopsAutonomousEffects(t *testing.T) {
	t.Parallel()
	for _, policy := range []string{"0", "person", "malformed"} {
		t.Run(policy, func(t *testing.T) {
			t.Parallel()
			b := newStopWorkBed(t)
			b.manager.Supervisor = &stopReadStarter{bed: b, reads: [][]readsubject.Finding{{stopFinding("regression", "a.go")}}}
			initial := policy
			if policy == "malformed" {
				initial = "auto"
			}
			_, record, _ := stopBuild(t, b, initial)
			if policy != "malformed" && (record.Rounds[0].Stop == nil || record.Rounds[0].Stop.Decision != "stop") {
				t.Fatalf("policy allowed continuation: %+v", record)
			}
			launches := len(b.starter.launched())
			brief := b.brief("policy-correction.md", fmt.Sprintf("Correct the finding.\n\n## Decisions on round 1\n\n| Finding | Decision | Evidence |\n| --- | --- | --- |\n| %s | fixed | a.go:12 |\n", record.Rounds[0].Reads[0].Findings[0].ID))
			code, result, _ := stopPublic(t, b, policy, "work", "revise", b.id, "--work", "stopped", "--brief", brief)
			if code == 0 || len(b.starter.launched()) != launches {
				t.Fatalf("policy %s launched autonomous correction: %d %+v", policy, code, result)
			}
		})
	}
}

func TestIntentUnitUnknownReadRetriesOnceWithoutAnotherAttempt(t *testing.T) {
	t.Parallel()
	for _, corrupt := range []string{"missing-class", "prose-disagrees"} {
		t.Run(corrupt, func(t *testing.T) {
			t.Parallel()
			b := newStopWorkBed(t)
			s := &stopReadStarter{bed: b, reads: [][]readsubject.Finding{{stopFinding("regression", "a.go")}}, corrupt: corrupt}
			b.manager.Supervisor = s
			run, before, brief := stopBuild(t, b, "auto")
			if before.Rounds[0].Stop == nil || !strings.HasPrefix(before.Rounds[0].Stop.Handoff, "stopped ") || len(before.Rounds[0].Reads) != 0 || before.Rounds[0].Material != -1 {
				t.Fatalf("corrupt read became known: %+v", before)
			}
			launches := len(b.starter.launched())
			code, result, _ := stopPublic(t, b, "auto", "work", "review", b.id, "--work", "stopped")
			if code == 0 || s.next != 2 || len(b.starter.launched()) != launches+1 {
				t.Fatalf("fresh examination: %d %+v launches=%v reads=%d", code, result, b.starter.launched(), s.next)
			}
			after, err := (&launch.UnitRunner{Root: b.unitRoot}).Status(run)
			if err != nil || len(after.Rounds) != 1 || len(after.Revisions) != 0 || after.Rounds[0].UnknownRetries != 1 || len(after.Rounds[0].Reads) != 0 || after.Rounds[0].Material != -1 || *after.CorrectionBudget != *before.CorrectionBudget || after.Rounds[0].Stop.Attempt != before.Rounds[0].Stop.Attempt || after.Rounds[0].Stop.Budget != before.Rounds[0].Stop.Budget || !strings.HasPrefix(after.Rounds[0].Stop.Handoff, "stopped ") {
				t.Fatalf("unknown input spent an attempt or lost its handoff: %+v %v", after, err)
			}
			for _, name := range []string{"stop-register.json", "unknown-retry.json"} {
				data, err := os.ReadFile(filepath.Join(after.Rounds[0].Directory, name))
				var retained loopstop.Stop
				if name == "stop-register.json" {
					var wrapper struct{ Stop loopstop.Stop }
					if err == nil {
						err = json.Unmarshal(data, &wrapper)
					}
					retained = wrapper.Stop
				} else if err == nil {
					err = json.Unmarshal(data, &retained)
				}
				if err != nil || retained.Decision != "stop" || !strings.HasPrefix(retained.Handoff, "stopped ") {
					t.Fatalf("%s lacks retained stop: %s %v", name, data, err)
				}
				if name == "stop-register.json" && (retained.Subject != after.Rounds[0].Stop.Subject || retained.Attempt != after.Rounds[0].Stop.Attempt) {
					t.Fatalf("stop register describes another unit: %+v", retained)
				}
			}
			launches = len(b.starter.launched())
			for _, verb := range []string{"review", "revise"} {
				args := []string{"work", verb, b.id, "--work", "stopped"}
				if verb == "revise" {
					args = append(args, "--brief", brief)
				}
				code, result, _ = stopPublic(t, b, "auto", args...)
				if code == 0 || len(b.starter.launched()) != launches || s.next != 2 {
					t.Fatalf("%s admitted a third examination: %d %+v", verb, code, result)
				}
			}
			code, result, stderr := stopPublic(t, b, "auto", "work", "revise", b.id, "--work", "stopped", "--brief", brief, "--reason", "Correct explicitly despite unreadable advisory evidence", "--by", "Wido")
			if code != 0 || !strings.Contains(stderr, "Impact:") || len(b.starter.launched()) <= launches {
				t.Fatalf("unknown advisory evidence refused the person: %d %+v %q", code, result, stderr)
			}
		})
	}
}

func TestIntentRunRevisionRecordsPersonAndClosesAddressedAsk(t *testing.T) {
	t.Parallel()
	b := newStopWorkBed(t)
	s := &stopReadStarter{bed: b, reads: [][]readsubject.Finding{{stopFinding("regression", "a.go")}, {}}}
	b.manager.Supervisor = s
	run, record, brief := stopBuild(t, b, "0")
	stop := record.Rounds[0].Stop
	finding := record.Rounds[0].Reads[0].Findings[0].ID
	q, err := channel.Ask(channel.AskRequest{RepoRoot: b.root(), Goal: b.id, Kind: "stop", Machine: "machine", Facts: []string{"The finding is unresolved"}, Now: b.manager.Now(), UnitStop: &channel.UnitStopQuestion{Loop: stop.Loop, Subject: stop.Subject, Attempt: stop.Attempt, Finding: finding, Review: record.Rounds[0].Reads[0].ID, Needs: "metasystem work revise run:" + run + " --brief " + brief + " --reason 'Correct explicitly' --by Wido", AcceptableActs: []string{"work-revise"}}})
	if err != nil {
		t.Fatal(err)
	}
	s.beforeBuild = func() {
		if s.stderr == nil || !strings.Contains(s.stderr.String(), "Impact:") {
			t.Error("run correction started before its impact was printed")
		}
	}
	code, result, stderr := stopPublic(t, b, "auto", "work", "revise", "run:"+run, "--brief", brief, "--reason", "Correct explicitly", "--by", "Wido")
	if code != 0 || !strings.Contains(stderr, "Impact:") {
		t.Fatalf("run revision: %d %+v %q", code, result, stderr)
	}
	stored, err := channel.ReadQuestion(b.root(), q.ID)
	if err != nil || stored.State != "closed" {
		t.Fatalf("run admission left its addressed ask open: %+v %v", stored, err)
	}
	paths, _ := filepath.Glob(filepath.Join(b.root(), "artifacts", "agents", "channel", "unit-stop-overrides", "*.json"))
	if len(paths) != 1 {
		t.Fatalf("run override records: %v", paths)
	}
	data, err := os.ReadFile(paths[0])
	var impact struct{ Goal, Who, Reason string }
	if err != nil || json.Unmarshal(data, &impact) != nil || impact.Goal != b.id || impact.Who != "Wido" || impact.Reason != "Correct explicitly" {
		t.Fatalf("person identity not retained: %s %v", data, err)
	}
}
