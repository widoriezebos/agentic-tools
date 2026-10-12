package plain

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
)

var laneRemedyVerbs = regexp.MustCompile(`metasystem landing ([a-z-]+)([^;\n]*)`)
var laneMechanismWords = []string{"barren", "recorded selection", "fingerprint", "sealed window", "admission", "exception code", "lane-return", "attempt ids", "selection recorded", "recorded selection changed", "goal_land_trunk_red", "--exception", "old-launch-1", "old-launch-2"}

func laneRemedyProblem(text string) error {
	for _, word := range laneMechanismWords {
		if strings.Contains(strings.ToLower(text), word) {
			return fmt.Errorf("mechanism %q in %q", word, text)
		}
	}
	// A proof/classification act retains the attempt it addresses.
	inspected := regexp.MustCompile(`metasystem landing prove(?: [^;\n]*)?`).ReplaceAllString(text, "metasystem landing prove")
	if regexp.MustCompile(`\d{8}T\d{6}`).MatchString(inspected) {
		return fmt.Errorf("attempt id in %q", text)
	}
	matches := laneRemedyVerbs.FindAllStringSubmatch(text, -1)
	for _, match := range matches {
		if match[1] == "land-anyway" {
			words := strings.Fields(match[2])
			if len(words) == 0 || !strings.Contains(text, "lets "+words[0]+" land while main's incident ") || !strings.Contains(text, "undo: metasystem landing give-back "+words[0]+" --reason TEXT") {
				return fmt.Errorf("permission without impact and undo in %q", text)
			}
		}
	}
	for _, match := range matches {
		switch match[1] {
		case "resume", "give-back", "land-anyway", "deploy", "stop", "start":
		case "prove", "set", "unset", "drain", "status":
		default:
			return fmt.Errorf("mechanism verb %q in %q", match[1], text)
		}
	}

	return nil
}

type laneRemedyProducer struct {
	name    string
	produce func(*testing.T) []string
}

// All lane presentation owners share this fixture table; execution commands
// are read from the records separately from the remedies a person sees.
func laneRemedyProducers() []laneRemedyProducer {
	rows := []laneRemedyProducer{
		{"steward main proof", func(t *testing.T) []string { return []string{proofCommand(false, true)} }},
		{"keeper hold", func(t *testing.T) []string {
			b := newStatusBed(t)
			b.record.CustodyEpoch = 1
			if err := os.MkdirAll(lane.HostDir(b.home), 0755); err != nil {
				t.Fatal(err)
			}
			for name, value := range map[string]any{"landing-lane.json": b.record, "landing-agent-keeper.json": lane.AgentState{Barren: 2, BarrenFingerprint: "unchanged", BarrenLaunches: []string{"old-launch-1", "old-launch-2"}}} {
				data, err := json.Marshal(value)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(lane.HostDir(b.home), name), data, 0600); err != nil {
					t.Fatal(err)
				}
			}
			k := lane.AgentKeeper{Home: b.home, Self: b.install, Now: func() time.Time { return bedNow }, Running: func() (string, bool, error) { return "", false, nil }, Fingerprint: func(string) (string, error) { return "unchanged", nil }}
			out := k.Run()
			if out.Outcome != lane.AgentHeld {
				t.Fatalf("hold fixture: %+v", out)
			}
			return []string{out.Line}
		}},
		{"open question", func(t *testing.T) []string {
			headline := LaneQuestionHeadline(channel.Question{ID: "q", OpenedAt: bedNow, Facts: []string{"May I restore the checkout? More detail."}})
			if !strings.Contains(headline, "question q since") || !strings.Contains(headline, "question show channel:q") {
				t.Fatal("question id lost: " + headline)
			}
			return []string{headline}
		}},
		{"proving status", func(t *testing.T) []string {
			b := newStatusBed(t)
			b.lines("running.json", Running{Commit: "main", Tree: "tree", Attempt: "attempt", Pid: 42, Since: bedNow.Format(time.RFC3339Nano)})
			status := b.read(ProveSeams{Now: func() time.Time { return bedNow.Add(7 * time.Minute) }, Alive: func(Running) bool { return true }}, b.git(nil, nil))
			return []string{status.Summary, status.ProofHeadline}
		}},
		{"red main status", func(t *testing.T) []string {
			b := newStatusBed(t)
			b.lines("results.jsonl", Result{Trunk: true, Commit: "main", Tree: "tree", Result: Red, Failed: []FailedUnit{{Unit: "unit", Tests: []string{"TestBroken"}}}, At: bedNow.Format(time.RFC3339Nano)})
			b.lines("queue.jsonl", Line{Goal: "g", SHA: "sha", At: bedNow.Format(time.RFC3339Nano)})
			status := readStatus(b.home, b.record, b.view, ProveSeams{Now: func() time.Time { return bedNow }, Git: func(string, ...string) (string, error) { return "main", nil }, Incidents: func(string, string, string) ([]goal.TrunkRedEntry, error) {
				return []goal.TrunkRedEntry{{ID: "incident-x", Identity: "incident-x"}}, nil
			}}, b.git(nil, nil))
			for _, want := range []string{"incident-x", "TestBroken", "metasystem landing prove --trunk", "lets g land while main's incident incident-x is open", "undo: metasystem landing give-back g --reason TEXT"} {
				if !strings.Contains(status.ProofHeadline, want) {
					t.Fatalf("lost %q: %s", want, status.ProofHeadline)
				}
			}
			return []string{status.Summary, status.ProofHeadline}
		}},
		{"person policy question", func(t *testing.T) []string {
			b := newStatusBed(t)
			b.record.CustodyEpoch = 1
			b.lines("batch.json", Batch{ID: "batch", Base: "main", Lane: b.record, State: BatchPrepared, Selector: PolicyValue{Value: "person"}, Members: []GoalSHA{{Goal: "g", SHA: "sha"}}})
			if err := SyncPolicyQuestion(b.install, func(string) (string, error) { return "fixture", nil }, bedNow); err != nil {
				t.Fatal(err)
			}
			qs, unread := channel.WalkOpenQuestions(b.install)
			if len(qs) != 1 || len(unread) != 0 {
				t.Fatalf("question fixture: %+v %v", qs, unread)
			}
			q := qs[0]
			if channel.LaneStopCommand(q) != "metasystem landing run --goals g" {
				t.Fatal("policy question lost its recorded command")
			}
			headline := LaneQuestionHeadline(q)
			if !strings.Contains(headline, "question "+q.ID+" since") {
				t.Fatal("question id lost: " + headline)
			}
			return []string{q.Facts[0], headline, channel.ReplyInstructions(q)}
		}},
	}
	for _, branch := range []string{"stop question", "observation", "prepare", "hold before wake", "waiting", "lane changed", "hold after wake", "selection changed", "start permission", "stopped after start", "helm", "running unreadable", "starting unreadable", "checkout gone", "reap"} {
		rows = append(rows, laneRemedyProducer{"keeper " + branch, func(t *testing.T) []string {
			b := newStatusBed(t)
			b.record.CustodyEpoch = 1
			if err := os.MkdirAll(b.install, 0755); err != nil {
				t.Fatal(err)
			}
			write := func(name string, value any) {
				t.Helper()
				raw, err := json.Marshal(value)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.MkdirAll(lane.HostDir(b.home), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(lane.HostDir(b.home), name), raw, 0600); err != nil {
					t.Fatal(err)
				}
			}
			write("landing-lane.json", b.record)
			k := lane.AgentKeeper{Home: b.home, Self: b.install, Now: func() time.Time { return bedNow }, Running: func() (string, bool, error) { return "", false, nil }, Start: func(string, lane.Wake) (string, error) { return "agent", nil }, Sources: lane.WakeSources{Reasons: func(string) ([]string, error) { return []string{"queued"}, nil }}}
			fail := errors.New("fixture read failed")
			switch branch {
			case "stop question":
				k.Observe = func(lane.Record) error { return &lane.ObservationError{StopQuestion: fail} }
			case "observation":
				k.Observe = func(lane.Record) error { return fail }
			case "prepare":
				k.Prepare = func(lane.Record) error { return fail }
			case "hold before wake":
				k.Holds = []func(string) (string, error){func(string) (string, error) { return "question q holds the lane", nil }}
			case "waiting":
				k.Waiting = func(string, string) (int, error) { return 0, fail }
			case "lane changed":
				k.Sources.Reasons = func(string) ([]string, error) {
					changed := b.record
					changed.CustodyEpoch++
					write("landing-lane.json", changed)
					return []string{"queued"}, nil
				}
			case "hold after wake":
				calls := 0
				k.Holds = []func(string) (string, error){func(string) (string, error) {
					calls++
					if calls >= 2 {
						return "question q holds the lane", nil
					}
					return "", nil
				}}
			case "selection changed":
				calls := 0
				k.Continuation = func(lane.Record) string {
					calls++
					if calls >= 2 {
						return "other"
					}
					return "batch"
				}
			case "start permission":
				k.AdmitWake = func(string, lane.Wake) (lane.Wake, error) { return lane.Wake{}, fail }
			case "stopped after start":
				k.Start = func(string, lane.Wake) (string, error) {
					changed := b.record
					changed.CustodyEpoch++
					write("landing-lane.json", changed)
					return "agent", nil
				}
				k.Cancel = func(string) error { return nil }
			case "helm":
				k.Helmed = func(string) bool { return true }
			case "running unreadable":
				k.Running = func() (string, bool, error) { return "", false, fail }
			case "starting unreadable":
				if err := os.WriteFile(filepath.Join(lane.HostDir(b.home), "landing-agent-keeper.json"), []byte("{"), 0600); err != nil {
					t.Fatal(err)
				}
			case "checkout gone":
				if err := os.RemoveAll(b.checkout); err != nil {
					t.Fatal(err)
				}
			case "reap":
				write("landing-agent-keeper.json", lane.AgentState{Launch: "agent"})
				k.Reap = []func(string) error{func(string) error { return fail }}
			}
			out := k.Run()
			if out.Outcome != lane.AgentHeld {
				t.Fatalf("%s: %+v", branch, out)
			}
			return []string{out.Line}
		}})
	}

	for _, state := range []string{"resolved", "resolving", "running", "reviewing"} {
		rows = append(rows, laneRemedyProducer{"fix " + state, func(t *testing.T) []string {
			return []string{fixHeadline(&Fix{Goal: "g", State: state, Units: []string{"unit"}, Reason: "recorded selection needs admission", CheckResult: Red, CheckMinutes: 2})}
		}})
	}
	for _, state := range []string{lane.OwnerStopped, lane.OwnerHeld, lane.OwnerIdle, lane.OwnerRunning, lane.OwnerUnready} {
		rows = append(rows, laneRemedyProducer{"owner " + state, func(t *testing.T) []string {
			by := "person"
			return []string{(lane.View{Owner: lane.OwnerView{State: state, StoppedBy: &by, Barren: 2}}).AgentSummary(false)}
		}})
	}
	for _, stop := range []Stop{
		{Loop: "lane-return", Subject: "lane", Required: []string{"metasystem", "landing", "run", "--goals", "g"}},
		{Loop: "lane-return", Subject: "g", Cause: &Cause{Kind: "other", Goal: "g"}},
		{Loop: "lane-proof", Subject: "g", Cause: &Cause{Kind: "environment"}, Required: []string{"metasystem", "landing", "prove"}},
		{Loop: "lane-proof", Subject: "g", Cause: &Cause{Kind: "main", Name: "incident-x", Tests: []string{"TestBroken"}}, Required: []string{"metasystem", "work", "land", "g", "--exception", "GOAL_LAND_TRUNK_RED"}},
		{Loop: "lane-proof", Subject: "main", Trunk: true, Required: []string{"metasystem", "landing", "prove", "--trunk"}},
		{Loop: "lane-classify", Subject: "20261012T123456", Cause: &Cause{Kind: "unclassified", Tests: []string{"TestBroken"}}, Required: []string{"metasystem", "landing", "prove", "--classify", "20261012T123456"}},
	} {
		rows = append(rows, laneRemedyProducer{"stop " + stop.Loop + " " + stop.Subject, func(t *testing.T) []string {
			install := t.TempDir()
			stop.Decision = "stop"
			stop.Handoff = "ask lane"
			stop.At = bedNow.Format(time.RFC3339Nano)
			if err := withLock(install, func() error { return appendLine(stopsPath(install), stop) }); err != nil {
				t.Fatal(err)
			}
			if err := SyncPolicyQuestion(install, func(string) (string, error) { return "fixture", nil }, bedNow); err != nil {
				t.Fatal(err)
			}
			qs, unread := channel.WalkOpenQuestions(install)
			if len(qs) != 1 || len(unread) != 0 {
				t.Fatalf("question fixture: %+v %v", qs, unread)
			}
			headline := StopHeadline(stop)
			if stop.Cause != nil {
				for _, test := range stop.Cause.Tests {
					if !strings.Contains(headline, test) {
						t.Fatal("failing test lost: " + headline)
					}
				}
			}
			return []string{headline, qs[0].Facts[0], LaneQuestionHeadline(qs[0]), channel.ReplyInstructions(qs[0])}
		}})
	}
	for _, command := range []string{"metasystem landing run --goals g", "metasystem landing restart", "metasystem landing prove --gate", "metasystem landing return g --cause other --reason TEXT", "metasystem landing drain", "metasystem landing set PATH", "metasystem system enroll --name Wido", "metasystem helm return --repo fixture"} {
		rows = append(rows, laneRemedyProducer{"refusal " + command, func(t *testing.T) []string {
			got := lane.Remedy(command)
			want := command
			if strings.Contains(command, "run --goals") || strings.Contains(command, "landing restart") {
				want = "metasystem landing resume"
			}
			if strings.Contains(command, "landing return g") {
				want = "metasystem landing give-back g --reason TEXT"
			}
			if got != want {
				t.Errorf("remedy %q, want %q", got, want)
			}
			return []string{got}
		}})
	}
	return rows
}

func TestLaneRemediesNameIntentVerbs(t *testing.T) {
	t.Parallel()
	for _, row := range laneRemedyProducers() {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			for _, text := range row.produce(t) {
				if err := laneRemedyProblem(text); err != nil {
					t.Error(err)
				}
			}
		})
	}
}

func TestLaneRemedyAuditRejectsMutations(t *testing.T) {
	t.Parallel()
	for _, text := range []string{"recorded selection; metasystem landing resume", "run: metasystem landing run --goals g", "selection recorded", "recorded selection changed", "admission", "GOAL_LAND_TRUNK_RED", "--exception", "20261012T123456", "metasystem landing land-anyway g --reason TEXT"} {
		if err := laneRemedyProblem(text); err == nil {
			t.Fatalf("audit accepted mutation %q", text)
		}
	}
	for _, text := range []string{"metasystem landing prove --trunk", "metasystem landing prove --gate", "metasystem landing prove --classify 20261012T123456"} {
		if err := laneRemedyProblem(text); err != nil {
			t.Fatal(err)
		}
	}
}

func TestLaneRemedyKeepsDecisionFactsAndImpact(t *testing.T) {
	t.Parallel()
	stop := Stop{Subject: "g", Cause: &Cause{Kind: "main", Name: "incident-x", Tests: []string{"TestBroken"}}}
	for _, text := range []string{StopHeadline(stop), (PolicyRequest{Required: []string{"metasystem", "landing", "land-anyway", "g", "--reason", "TEXT"}, Subject: PolicySubject{Incidents: []IncidentIdentity{{Identity: "incident-x"}}}}).Remedy()} {
		for _, want := range []string{"g", "incident-x", "lets g land", "undo: metasystem landing give-back g --reason TEXT"} {
			if !strings.Contains(text, want) {
				t.Errorf("lost %q: %s", want, text)
			}
		}
	}
	if text := StopHeadline(stop); !strings.Contains(text, "TestBroken") || !strings.Contains(text, "land a fix on main, then metasystem landing prove --trunk") {
		t.Fatal(text)
	}
	stop.Subject = "g, h"
	for _, subject := range []string{"g", "h"} {
		if text := StopHeadline(stop); !strings.Contains(text, "landing land-anyway "+subject+" --reason TEXT") || !strings.Contains(text, "undo: metasystem landing give-back "+subject+" --reason TEXT") {
			t.Fatal("batch member lost its act: " + text)
		}
	}
	for _, state := range []string{"resolving", "resolved", "running", "reviewing"} {
		headline := fixHeadline(&Fix{Goal: "g", State: state, CheckResult: Red})
		if !strings.HasSuffix(headline, "; nothing to do") || LaneStatusRemedy(lane.View{Owner: lane.OwnerView{State: lane.OwnerHeld}}, false, headline) != "" {
			t.Fatal(headline)
		}
	}
}

func TestLaneRemedyFlakeHeadlinesKeepTests(t *testing.T) {
	t.Parallel()
	for _, published := range []bool{false, true} {
		b := newStatusBed(t)
		b.lines("results.jsonl", Result{Trunk: true, Commit: "main", Result: Green, FlakePublished: published, Reason: "flake on main: TestFlaky", Flakes: []FlakeRecord{{FailedUnit: FailedUnit{Unit: "unit", Tests: []string{"TestFlaky"}}}}})
		status := b.read(ProveSeams{}, b.git(nil, nil))
		if !strings.Contains(status.ProofHeadline, "TestFlaky") {
			t.Fatal(status.ProofHeadline)
		}
	}
}
