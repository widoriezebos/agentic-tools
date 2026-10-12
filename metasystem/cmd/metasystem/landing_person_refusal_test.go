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

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/helm"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/realpath"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

type personRefusalBed struct {
	root, install, home string
	now                 time.Time
	owners              intentOwners
}

func newPersonRefusalBed(t *testing.T) *personRefusalBed {
	t.Helper()
	b := &personRefusalBed{root: realpath.Resolve(t.TempDir()), home: t.TempDir(), now: laneTestNow}
	b.install = filepath.Join(b.root, "metasystem")
	for _, dir := range []string{b.install, filepath.Join(b.root, ".git"), lane.HostDir(b.home)} {
		helmMust(t, os.MkdirAll(dir, 0755))
	}
	helmMust(t, os.WriteFile(filepath.Join(b.install, "metasystem.conf"), []byte("metasystem.template=true\ntesting.contract=testing.json\n"), 0600))
	record, err := json.Marshal(lane.Record{Root: b.root, Install: b.install, CustodyEpoch: 1, RegisteredBy: "Wido"})
	helmMust(t, err)
	helmMust(t, os.WriteFile(lane.RecordPath(b.home), record, 0600))
	b.owners = intentOwners{
		resolver:   stateroot.NewResolver(func(string) (string, error) { return b.root, nil }, os.Executable),
		prove:      enrolledPersonProver(t, b.install, b.now),
		commandNow: func(string) (time.Time, error) { return b.now, nil },
		policies: config.PolicyReaders{
			Registry: func(string) (config.PolicyRegistry, error) { return config.PolicyRegistry{Lane: b.root}, nil },
			ConfPath: func(string) (string, error) { return filepath.Join(b.install, "metasystem.conf"), nil },
			Helm:     func(string) helm.State { return helm.State{} },
		},
		landing: laneVerbOwners{
			home: func() (string, error) { return b.home, nil },
			now:  func() time.Time { return b.now },
			person: func(root string) (string, error) {
				if root != b.install && root != b.root {
					return "", errors.New("the terminal is not enrolled at this root")
				}
				return "Wido", nil
			},
			machine: func(string) (string, error) { return "fixture", nil },
			probe:   func(string) (lane.OwnerProbe, error) { return lane.OwnerProbe{}, nil },
			ready:   func(string) error { return nil },
			wake:    func(string) lane.WakeSources { return lane.WakeSources{} },
			view:    func(string) lane.View { root := b.root; return lane.View{Root: &root} },
			plainProve: plain.ProveSeams{
				Now: func() time.Time { return b.now },
				Git: func(_ string, args ...string) (string, error) {
					if args[0] == "symbolic-ref" {
						return "main", nil
					}
					return "", nil
				},
				Incidents: func(string, string, string) ([]goal.TrunkRedEntry, error) { return nil, nil },
			},
		},
	}
	return b
}

func (b *personRefusalBed) run(t *testing.T, cwd, action string, args ...string) (int, string) {
	t.Helper()
	command, ok := findIntentAction("landing", action)
	if !ok {
		t.Fatal(action)
	}
	var out, err bytes.Buffer
	code := runIntentIn(command, args, &out, &err, cwd, b.owners)
	return code, out.String() + err.String()
}

func (b *personRefusalBed) records(t *testing.T) []plain.Intervention {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(plain.Dir(b.install), "interventions.jsonl"))
	helmMust(t, err)
	var entries []plain.Intervention
	for _, line := range bytes.Split(bytes.TrimSpace(data), []byte{'\n'}) {
		var entry plain.Intervention
		helmMust(t, json.Unmarshal(line, &entry))
		entries = append(entries, entry)
	}
	return entries
}

func TestLandingPersonRefusedReturnAndResolve(t *testing.T) {
	t.Parallel()
	for _, action := range []string{"return", "resolve"} {
		t.Run(action, func(t *testing.T) {
			t.Parallel()
			b := newPersonRefusalBed(t)
			cwd, args, want := b.root, []string{"missing", "--cause", "own", "--json"}, "nothing of goal missing waits"
			if action == "resolve" {
				cwd, args, want = t.TempDir(), []string{"--json"}, "runs in the registered landing checkout"
			}
			code, output := b.run(t, cwd, action, args...)
			var result intentResult
			helmMust(t, json.Unmarshal([]byte(output), &result))
			if code != 1 || result.Outcome != intentRefused || !strings.Contains(result.Summary, want) {
				t.Fatalf("exit=%d %s", code, output)
			}
			entries := b.records(t)
			if len(entries) != 1 {
				t.Fatalf("records: %+v", entries)
			}
			e := entries[0]
			if e.Row != "person-refused" || !strings.HasPrefix(e.Subject, "landing "+action) || e.What != result.Summary || e.Why == "" || e.How != "the person's verb was refused; R-142/143: a person is never denied a verb, so this is a defect" || e.At != b.now.UTC().Format(time.RFC3339Nano) {
				t.Fatalf("record: %+v", e)
			}
			if action == "return" && !strings.Contains(e.Subject, "missing") {
				t.Fatal(e.Subject)
			}
		})
	}
}

func TestLandingPersonRefusalHealthWindow(t *testing.T) {
	t.Parallel()
	b := newPersonRefusalBed(t)
	for range 7 {
		b.run(t, b.root, "return", "missing", "--cause", "own")
	}
	for _, args := range [][]string{nil, {"--json"}} {
		before := idemTreeDigest(t, b.install)
		code, output := b.run(t, b.root, "health", args...)
		if code != 0 || !strings.Contains(output, "person-refused") || !strings.Contains(output, "7 refusals") || !strings.Contains(output, "a person was refused 7 times in 24 h: landing return") {
			t.Fatalf("health: %d %s", code, output)
		}
		idemSameTree(t, "health read", before, idemTreeDigest(t, b.install))
	}
	b.now = b.now.Add(24*time.Hour - time.Nanosecond)
	_, output := b.run(t, b.root, "health")
	if !strings.Contains(output, "7 refusals") {
		t.Fatal(output)
	}
	b.now = b.now.Add(time.Nanosecond)
	code, output := b.run(t, b.root, "health")
	if code != 0 || strings.Contains(output, "person-refused:") || strings.Contains(output, "; 7 refusals") {
		t.Fatalf("expired health: %d %s", code, output)
	}
}

func TestLandingPersonRefusalWriteFailurePreservesResult(t *testing.T) {
	t.Parallel()
	b := newPersonRefusalBed(t)
	helmMust(t, os.MkdirAll(filepath.Join(plain.Dir(b.install), "interventions.jsonl"), 0755))
	for _, args := range [][]string{{"missing", "--cause", "own", "--json"}, {"missing", "--cause", "own"}} {
		code, output := b.run(t, b.root, "return", args...)
		if code != 1 || !strings.Contains(output, "nothing of goal missing waits") || !strings.Contains(output, "Problems") && !strings.Contains(output, `"problems"`) || !strings.Contains(output, "could not record the person's refusal") {
			t.Fatalf("write failure: %d %s", code, output)
		}
		if args[len(args)-1] == "--json" {
			var result struct {
				Outcome string
				Details []string
			}
			helmMust(t, json.Unmarshal([]byte(output), &result))
			if result.Outcome != intentRefused || len(result.Details) != 2 || !strings.Contains(result.Details[1], "Problems:") {
				t.Fatalf("result: %+v", result)
			}
		}
	}
}

func TestLandingPersonProtectedResolveDoesNotCount(t *testing.T) {
	t.Parallel()
	b := newPersonRefusalBed(t)
	_, _, err := plain.HandIn(b.install, plain.Line{Goal: "g", SHA: "goal", At: b.now.Format(time.RFC3339)})
	helmMust(t, err)
	b.owners.landing.plainResolve.Git = func(_ string, args ...string) (string, error) {
		switch strings.Join(args, " ") {
		case "show HEAD:metasystem/testing.json":
			return `{"schemaVersion":1,"projectRisk":{"severity":1,"exposure":1,"reversibility":"revert","detection":"immediate","recovery":"bounded"},"surfaces":[{"id":"app","paths":["src/**"],"standard":["check"]}],"groups":[{"id":"check","kind":"unit","adapter":"go","cwd":".","inputs":["src/**"],"platforms":["any"],"targetMs":1000,"packages":["internal/a"],"tests":["TestA"]}],"unknown":["check"]}`, nil
		case "diff --name-only --diff-filter=U -z":
			return "src.go\x00", nil
		case "rev-parse --verify HEAD^{commit}":
			return "main", nil
		case "rev-parse --verify MERGE_HEAD^{commit}":
			return "goal", nil
		case "diff --name-only -z AUTO_MERGE --":
			return "my-edits.go\x00", nil
		case "ls-files --others --exclude-standard -z":
			return "", nil
		default:
			t.Fatalf("unexpected Git call: %v", args)
			return "", errors.New("unexpected Git")
		}
	}
	code, output := b.run(t, b.root, "resolve", "--json")
	var result intentResult
	helmMust(t, json.Unmarshal([]byte(output), &result))
	if code != 1 || result.Outcome != intentFailed || !strings.Contains(result.Summary, "changes outside the recorded merge") {
		t.Fatalf("protected: %d %s", code, output)
	}
	entries := b.records(t)
	if len(entries) != 1 || entries[0].Row != "person-protected" || entries[0].Why != "LANE_RESOLVE_EDITS" {
		t.Fatalf("protected record: %+v", entries)
	}
	_, output = b.run(t, b.root, "health")
	if strings.Contains(output, "person-refused") || strings.Contains(output, "; 1 refusals") {
		t.Fatal(output)
	}
	entry, _, err := plain.Latest(b.install, "g")
	if err != nil || entry.State != plain.StateWaiting {
		t.Fatalf("protection changed queue: %+v %v", entry, err)
	}
}

func TestLandingPersonPushRefusalKeepsAgentRuleVisible(t *testing.T) {
	t.Parallel()
	b := newPersonRefusalBed(t)
	b.owners.landing.push = func(string, string, time.Time, func(string, string) error) (plain.PushOutcome, error) {
		return plain.PushOutcome{}, &plain.Refusal{Code: plain.CodeNotFastForward, Reason: "pushing would rewrite main; nothing was pushed"}
	}
	code, output := b.run(t, b.root, "push", "--json")
	var result intentResult
	helmMust(t, json.Unmarshal([]byte(output), &result))
	if code != 1 || result.Outcome != intentRefused {
		t.Fatalf("push: %d %s", code, output)
	}
	entries := b.records(t)
	if len(entries) != 1 || entries[0].Row != "person-protected" || entries[0].Why != plain.CodeNotFastForward {
		t.Fatalf("protected push: %+v", entries)
	}
	_, output = b.run(t, b.root, "health")
	if strings.Contains(output, "person-refused") || strings.Contains(output, "; 1 refusals") {
		t.Fatal(output)
	}
}

func TestLandingPersonRefusalNamesVerbsAndSkipsAgents(t *testing.T) {
	t.Parallel()
	b := newPersonRefusalBed(t)
	b.run(t, b.root, "return", "missing", "--cause", "own")
	b.run(t, t.TempDir(), "resolve")
	prover := b.owners.prove
	b.owners.prove = nil
	code, output := b.run(t, b.root, "return", "missing", "--cause", "own")
	if code != 1 || !strings.Contains(output, "nothing of goal missing waits") || len(b.records(t)) != 2 {
		t.Fatalf("agent counted: %d %s", code, output)
	}
	b.owners.prove = prover
	_, output = b.run(t, b.root, "health")
	if !strings.Contains(output, "2 refusals") || !strings.Contains(output, "landing return, landing resolve") {
		t.Fatal(output)
	}
}

func TestLandingPersonRefusalAuthorityAndResumePaths(t *testing.T) {
	t.Parallel()
	for _, path := range []string{"authority", "unsetting", "readiness"} {
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			b := newPersonRefusalBed(t)
			wantCode := 1
			switch path {
			case "authority":
				_, err := lane.SetPause(b.home, "Wido", b.now)
				helmMust(t, err)
				b.owners.landing.person = func(string) (string, error) { return "", errors.New("terminal enrollment cannot be read") }
				wantCode = 3
			case "unsetting":
				_, err := lane.Unset(b.home, "Wido", b.now, false, lane.UnsetSeams{Settle: func(lane.Layout) (lane.Settlement, error) { return lane.Settlement{Live: []string{"agent"}}, nil }})
				helmMust(t, err)
			case "readiness":
				b.owners.landing.ready = func(string) error { return lane.UnarmedRefusal(b.root) }
				wantCode = 3
			}
			code, output := b.run(t, b.root, "start", "--json")
			var result intentResult
			helmMust(t, json.Unmarshal([]byte(output), &result))
			if code != wantCode || result.Outcome != intentRefused {
				t.Fatalf("%s: exit=%d %s", path, code, output)
			}
			if path == "authority" {
				if _, err := os.Stat(filepath.Join(plain.Dir(b.install), "interventions.jsonl")); !os.IsNotExist(err) {
					t.Fatalf("a failed person check recorded a person's refusal: %v", err)
				}
				return
			}
			entries := b.records(t)
			if len(entries) != 1 || entries[0].Row != "person-refused" || entries[0].What != result.Summary {
				t.Fatalf("%s: entries=%+v", path, entries)
			}
		})
	}
}

func TestLandingPersonRefusalChecksCallerRoot(t *testing.T) {
	t.Parallel()
	for _, path := range []string{"start from seat", "start outside install", "set from seat"} {
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			b := newPersonRefusalBed(t)
			cwd := realpath.Resolve(t.TempDir())
			proveAt := cwd
			b.owners.resolver = stateroot.NewResolver(func(string) (string, error) { return "", errors.New("no repository") }, noExecutable)
			if path != "start outside install" {
				proveAt = filepath.Join(cwd, "metasystem")
				helmMust(t, os.MkdirAll(proveAt, 0755))
				helmMust(t, os.WriteFile(filepath.Join(proveAt, "metasystem.conf"), []byte("metasystem.template=true\n"), 0600))
				b.owners.resolver = stateroot.NewResolver(func(string) (string, error) { return cwd, nil }, noExecutable)
			}
			var checked []string
			b.owners.landing.person = func(root string) (string, error) {
				checked = append(checked, root)
				if root != proveAt {
					return "", errors.New("only the caller's terminal is enrolled")
				}
				return "Wido", nil
			}
			action, args := "start", []string{"--json"}
			if path == "set from seat" {
				action, args = "set", []string{b.root, "--json"}
				b.owners.landing.validate = func(string, string, time.Time) (string, error) {
					return "", errors.New("the checkout cannot be registered")
				}
			} else {
				_, err := lane.Unset(b.home, "Wido", b.now, false, lane.UnsetSeams{Settle: func(lane.Layout) (lane.Settlement, error) {
					return lane.Settlement{Live: []string{"agent"}}, nil
				}})
				helmMust(t, err)
			}
			code, output := b.run(t, cwd, action, args...)
			if code != 1 || len(checked) != 1 || checked[0] != proveAt {
				t.Fatalf("exit=%d checked=%v want=%s %s", code, checked, proveAt, output)
			}
			entries := b.records(t)
			if len(entries) != 1 || entries[0].Row != "person-refused" || !strings.HasPrefix(entries[0].Subject, "landing "+action) {
				t.Fatalf("caller refusal: %+v", entries)
			}
		})
	}
}

func TestLandingPersonRefusalSkipsUsageErrors(t *testing.T) {
	t.Parallel()
	for _, action := range []string{"set", "return", "prove"} {
		t.Run(action, func(t *testing.T) {
			t.Parallel()
			b := newPersonRefusalBed(t)
			args := []string{"--json"}
			if action == "prove" {
				args = append(args, "--classify", "saved", "--trunk")
			}
			code, output := b.run(t, b.root, action, args...)
			var result intentResult
			helmMust(t, json.Unmarshal([]byte(output), &result))
			if code != 2 || result.Outcome != intentRefused {
				t.Fatalf("usage error: exit=%d %s", code, output)
			}
			if _, err := os.Stat(filepath.Join(plain.Dir(b.install), "interventions.jsonl")); !os.IsNotExist(err) {
				t.Fatalf("usage error recorded a person's refusal: %v", err)
			}
		})
	}
}

func TestLandingPersonRefusalSetUsesExplicitSeat(t *testing.T) {
	t.Parallel()
	b := newPersonRefusalBed(t)
	seat := realpath.Resolve(t.TempDir())
	b.owners.landing.person = func(root string) (string, error) {
		if root != seat {
			t.Fatalf("person checked at %s, want the explicit seat %s", root, seat)
		}
		return "Wido", nil
	}
	command, ok := findIntentAction("landing", "set")
	if !ok {
		t.Fatal("landing set is missing")
	}
	inv := intentInvocation{command: command, cwd: b.root, owners: b.owners, input: intentInput{values: map[string][]string{"seat": {seat}}}}
	result := inv.observeLandingRefusal(intentResult{Outcome: intentRefused, code: 1, Summary: "the checkout cannot be registered"})
	entries := b.records(t)
	if result.code != 1 || len(entries) != 1 || entries[0].What != result.Summary {
		t.Fatalf("result=%+v entries=%+v", result, entries)
	}
}

func TestLandingPersonRefusalDoesNotRecreateMissingInstall(t *testing.T) {
	t.Parallel()
	b := newPersonRefusalBed(t)
	_, err := lane.Unset(b.home, "Wido", b.now, false, lane.UnsetSeams{Settle: func(lane.Layout) (lane.Settlement, error) {
		return lane.Settlement{Live: []string{"agent"}}, nil
	}})
	helmMust(t, err)
	helmMust(t, os.RemoveAll(b.install))
	code, output := b.run(t, b.root, "start", "--json")
	var result intentResult
	helmMust(t, json.Unmarshal([]byte(output), &result))
	if code != 1 || result.Outcome != intentRefused || !strings.Contains(result.Summary, "being unset") || !strings.Contains(output, "Problems:") {
		t.Fatalf("missing install: %d %s", code, output)
	}
	if _, err := os.Stat(b.install); !os.IsNotExist(err) {
		t.Fatalf("recording recreated the missing install: %v", err)
	}
}

func TestLandingPersonRefusalBackgroundAttemptDoesNotReprove(t *testing.T) {
	t.Parallel()
	b := newPersonRefusalBed(t)
	b.owners.prove = func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
		t.Fatal("a background attempt re-proved a person")
		return humanauthority.Proof{}, errors.New("agent")
	}
	code, output := b.run(t, b.root, "prove", "--attempt", "background", "--json")
	var result intentResult
	helmMust(t, json.Unmarshal([]byte(output), &result))
	if code != 1 || result.Outcome != intentRefused || !strings.Contains(output, "LANE_PROOF_ADMISSION") {
		t.Fatalf("background refusal: %d %s", code, output)
	}
	if _, err := os.Stat(filepath.Join(plain.Dir(b.install), "interventions.jsonl")); !os.IsNotExist(err) {
		t.Fatalf("background attempt counted as a person's refusal: %v", err)
	}
}

func TestLandingPersonRefusalUsesTheVerbsAuthorityObservation(t *testing.T) {
	t.Parallel()
	b := newPersonRefusalBed(t)
	prove := b.owners.prove
	calls := 0
	b.owners.prove = func(root string, pid int64, reader humanauthority.Reader, lineage, grant string, now time.Time) (humanauthority.Proof, error) {
		calls++
		if calls > 1 {
			t.Fatal("recording a refusal repeated the verb's authority observation")
		}
		return prove(root, pid, reader, lineage, grant, now)
	}
	code, output := b.run(t, b.root, "return", "missing", "--cause", "own", "--json")
	if code != 1 || calls != 1 || len(b.records(t)) != 1 {
		t.Fatalf("original authority: exit=%d calls=%d %s", code, calls, output)
	}
}

func TestLandingPersonRefusalKeepsTheAdmittedInstall(t *testing.T) {
	t.Parallel()
	b := newPersonRefusalBed(t)
	other := filepath.Join(t.TempDir(), "metasystem")
	helmMust(t, os.MkdirAll(other, 0755))
	replacement, err := json.Marshal(lane.Record{Root: filepath.Dir(other), Install: other, CustodyEpoch: 2, RegisteredBy: "Wido"})
	helmMust(t, err)
	prove := b.owners.prove
	b.owners.prove = func(root string, pid int64, reader humanauthority.Reader, lineage, grant string, now time.Time) (humanauthority.Proof, error) {
		proof, err := prove(root, pid, reader, lineage, grant, now)
		helmMust(t, os.WriteFile(lane.RecordPath(b.home), replacement, 0600))
		return proof, err
	}
	code, output := b.run(t, b.root, "return", "missing", "--cause", "own", "--json")
	if code != 1 || len(b.records(t)) != 1 {
		t.Fatalf("admitted install: %d %s", code, output)
	}
	if _, err := os.Stat(filepath.Join(plain.Dir(other), "interventions.jsonl")); !os.IsNotExist(err) {
		t.Fatalf("the refused act was recorded in another install: %v", err)
	}
}
