package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/conflict"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/helm"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/textui"
)

func TestHandInCarriesUnitRounds(t *testing.T) {
	t.Parallel()
	testHandInUnitRounds(t, readBranch(2, "unit-read", "critic-root"), [2]string{"promoted", "critic"})
}

func TestHandInTierOneReadsAreWaived(t *testing.T) {
	t.Parallel()
	state := readBranch(0)
	state.ReadsWaived = true
	testHandInUnitRounds(t, state, [2]string{"waived", "waived"})
}

func TestHandInUnreadUnitHasNoRead(t *testing.T) {
	t.Parallel()
	b, owners, install := plainLaneBedWith(t, true, "critic-root")
	owners.status = readBranch(1, "critic-root")
	code, result := b.do("work", "land", "standing-validation", "--through", strings.Repeat("1", 40))
	expectOutcome(t, "partial goal", code, result, intentRefused)
	if !strings.Contains(result.Summary, "u2 has no clean read") {
		t.Fatalf("unread unit was not named: %+v", result)
	}
	if entries, err := plain.Entries(install); err != nil || len(entries) != 0 {
		t.Fatalf("partial goal was queued: %+v %v", entries, err)
	}
}

func testHandInUnitRounds(t *testing.T, state intentBranchState, reads [2]string, args ...string) {
	t.Helper()
	w := newWorkBed(t)
	for _, name := range []string{"u1", "u2"} {
		code, built, _ := w.work(append([]string{"work", "build", w.id, name, "--brief", w.brief(name+".md", "Build it.\n"), "--lines", "5"}, workCheck...)...)
		expectOutcome(t, "build "+name, code, built, intentConfirmed)
		run := resultData(t, built)["run"].(string)
		record, err := (&launch.UnitRunner{Root: w.unitRoot}).Status(run)
		if err != nil {
			t.Fatal(err)
		}
		if name == "u1" {
			record.Rounds = []launch.UnitRound{{Number: 1}, {Number: 2, Cause: "provider-limit"}, {Number: 3, Cause: "sandbox-denied"}, {Number: 4, Cause: "provider-limit"},
				{Number: 5, Outcome: "green", Steps: []launch.UnitStep{{Name: "proof:static"}, {Name: "proof:unit"}, {Name: "proof:check"}}}}
		} else {
			record.Rounds[0].Steps = nil
		}
		writeQuestionFixture(t, filepath.Join(w.unitRoot, run, "run.json"), record)
	}
	if state.ReadsWaived {
		file := w.goalFile(w.id)
		file.Tier, file.Budget.ReviewRoundLimit = 1, 0
		file.Risk.Severity, file.Risk.Novelty = 1, 1
		file.Approved.Digest = goal.ApprovalDigest(file.Intent, file.Tier, *file.Budget, file.Risk)
		w.addGoal(file)
	}
	l := &laneVerbBed{cwd: w.root(), home: t.TempDir(), landingA: t.TempDir(), pid: 4242, helmed: true}
	install := filepath.Join(l.landingA, "metasystem")
	b := &deliveryBed{intentBed: w.intentBed, install: w.root(), owners: &intentDeliveryOwners{now: func() time.Time { return laneTestNow }}}
	b.writeFile(filepath.Join(install, "metasystem.conf"), "")
	b.writeJSON(lane.RecordPath(l.home), lane.Record{Root: l.landingA, Install: install, CustodyEpoch: 1, RegisteredBy: "Wido", At: laneTestNow.Format(time.RFC3339)})
	landing := &landingOwners{status: state}
	landing.install(b)
	b.owners.laneRegistrant = func(string) string { return "seat" }
	b.owners.laneRoot = func(string, time.Time) (string, bool, error) { return l.landingA, true, nil }
	b.owners.laneInstall = func(string) (string, error) { return install, nil }
	b.owners.landingGate = func(*intentInvocation, string, string) (string, error) { return "the bed's landing", nil }
	owners := w.workOwners()
	owners.delivery = b.owners
	owners.landing = l.owners().landing
	owners.landing.plainProve.Git = func(_ string, args ...string) (string, error) {
		return "main", nil
	}
	owners.landing.plainProve.Incidents = func(string, string, string) ([]goal.TrunkRedEntry, error) { return nil, nil }
	owners.policies = config.PolicyReaders{Registry: func(string) (config.PolicyRegistry, error) {
		return config.PolicyRegistry{Lane: l.landingA}, nil
	}, Helm: func(string) helm.State { return helm.State{} }, ConfPath: func(string) (string, error) { return filepath.Join(install, "metasystem.conf"), nil }}
	// The hand-in now runs through the goal's connection: endpoint, claim
	// check, section and rebase are the bed's facts, never Git.
	tip := state.BranchTip
	bedGit := owners.work.git
	owners.work.git = func(dir string, args ...string) ([]byte, error) {
		if strings.Join(args, " ") == "rev-parse --verify -q refs/heads/goal/"+w.id {
			return []byte(tip + "\n"), nil
		}
		if bedGit != nil {
			return bedGit(dir, args...)
		}
		return nil, nil
	}
	owners.connection = intentConnectionOwners{
		endpoint:    func(string) (goal.Endpoint, error) { return goal.Endpoint{Remote: "origin"}, nil },
		endpointTip: func(string, goal.Endpoint) (string, error) { return state.EndpointTip, nil },
		claimCheck:  func(string, string, goal.Endpoint) func() error { return func() error { return nil } },
		section:     func(_ string, body func(func(func() error) error) error) error { return body(nil) },
		rebase: func(branch.RebaseRequest) (branch.RebaseResult, error) {
			return branch.RebaseResult{State: "held", OldTip: tip, NewTip: tip, MainTip: state.EndpointTip, Carried: []string{}, NeedsReview: []string{}}, nil
		},
		recordRebase: func(*intentInvocation, string, branch.RebaseResult) error { return nil },
	}
	code, result := w.runJSON(owners, append([]string{"work", "land", w.id}, args...)...)
	expectOutcome(t, "hand-in", code, result, intentConfirmed)
	entries, err := plain.Entries(install)
	want := []plain.UnitRounds{
		{Unit: "u1", Counted: 2, Machinery: map[string]int{"provider-limit": 2, "sandbox-denied": 1}, Proof: []string{"static", "unit", "check"}, Read: reads[0]},
		{Unit: "u2", Counted: 1, Proof: []string{"check"}, Read: reads[1]},
	}
	if err != nil || len(entries) != 1 || !reflect.DeepEqual(entries[0].Units, want) {
		t.Fatalf("queued unit rounds: %+v %v; want %+v", entries, err, want)
	}
	statusOwners := l.owners()
	statusOwners.ownEngine = w.owners().ownEngine
	statusOwners.landing.view = func(string) lane.View { return lane.View{Root: &l.landingA, Summary: "The landing lane is idle."} }
	statusOwners.landing.plainProve.Git = func(_ string, args ...string) (string, error) {
		if args[0] == "cat-file" {
			return "", os.ErrNotExist
		}
		return strings.Repeat("e", 40), nil
	}
	code, out, stderr := w.run(statusOwners, "landing", "status")
	words := oneSpaced(out)
	for _, part := range []string{"u1 2 counted rounds; machinery rounds provider-limit=2, sandbox-denied=1; proof static, unit, check; read " + reads[0], "u2 1 counted rounds; proof check; read " + reads[1]} {
		if code != 0 || !strings.Contains(words, part) {
			t.Fatalf("landing status lacks %q: code=%d %s %s", part, code, out, stderr)
		}
	}
}

func TestLandingStatusSaysRecords(t *testing.T) {
	t.Parallel()
	install := t.TempDir()
	var line plain.Line
	if err := json.Unmarshal([]byte(`{"goal":"design","branch":"goal/design","sha":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","seat":"ui","records":true}`), &line); err != nil {
		t.Fatal(err)
	}
	if _, _, err := plain.HandIn(install, line); err != nil {
		t.Fatal(err)
	}
	for _, state := range []string{plain.StateWaiting, plain.StateLanded, plain.StateReturned} {
		if state == plain.StateReturned {
			if _, _, err := plain.ReturnProven(install, "design", "unclassified", "records check failed", true, "fixture", time.Time{}, plain.ProveSeams{Person: &plain.ActProvenance{Kind: "return", Person: "fixture"}}); err != nil {
				t.Fatal(err)
			}
		}
		entries, err := plain.Entries(install)
		if err != nil {
			t.Fatal(err)
		}
		entries, err = plain.Landed(entries, func(string) (bool, error) { return state == plain.StateLanded, nil })
		if err != nil {
			t.Fatal(err)
		}
		entries = append(entries, plain.Entry{Goal: "code", Branch: "goal/code", SHA: line.SHA, Seat: "ui", State: state})
		page := textui.New(textui.Env{Verbose: true})
		withPlainLane(func(*textui.Page) {}, landingStatusData{View: lane.View{Root: &install}, Queue: entries})(page)
		words := oneSpaced(page.String())
		for _, want := range []string{"goal/design records at bbbbbbbbbbbb from ui · " + state, "goal/code at bbbbbbbbbbbb from ui · " + state} {
			if !strings.Contains(words, want) {
				t.Fatalf("status %s lacks %q: %s", state, want, words)
			}
		}
		if state == plain.StateReturned && !strings.Contains(words, "returned: records check failed") {
			t.Fatal(words)
		}
	}
}

// plainLaneBed is a delivery bed whose lane is registered at /landing: its
// installation is a folder of the test, where the hand-in's queue.jsonl
// lands.
func plainLaneBed(t *testing.T, sources ...string) (*deliveryBed, *landingOwners, string) {
	t.Helper()
	return plainLaneBedWith(t, false, sources...)
}

func plainLaneBedWith(t *testing.T, withoutGit bool, sources ...string) (*deliveryBed, *landingOwners, string) {
	t.Helper()
	b := newDeliveryBedWith(t, nil, withoutGit)
	b.owners.landingGate = func(*intentInvocation, string, string) (string, error) { return "the bed's landing", nil }
	b.owners.recordLanded = func(*intentInvocation, string) error { return nil }
	owners := &landingOwners{status: readBranch(2, sources...)}
	owners.install(b)
	b.owners.laneRoot = func(string, time.Time) (string, bool, error) { return "/landing", true, nil }
	b.owners.laneContains = func(sha, main string) (bool, error) { return sha == main, nil }
	b.owners.laneLatest = func(install, id, main string) (plain.Entry, bool, error) {
		entry, known, err := plain.Latest(install, id)
		if known && entry.State != plain.StateReturned && entry.SHA == main {
			entry.State = plain.StateLanded
		}
		return entry, known, err
	}
	// The bed exposes a goal worktree and keeps explicit rebase isolated from Git.
	root := b.root()
	b.work.git = func(_ string, args ...string) ([]byte, error) {
		if strings.Join(args, " ") == "worktree list --porcelain" {
			return []byte("worktree " + root + "\nbranch refs/heads/goal/standing-validation\n"), nil
		}
		return nil, nil
	}
	tip := owners.status.BranchTip
	b.connection.endpoint = func(string) (goal.Endpoint, error) { return goal.Endpoint{Remote: "origin"}, nil }
	b.connection.endpointTip = func(string, goal.Endpoint) (string, error) { return owners.status.EndpointTip, nil }
	b.connection.claimCheck = func(string, string, goal.Endpoint) func() error { return func() error { return nil } }
	b.connection.section = func(_ string, body func(func(func() error) error) error) error { return body(nil) }
	b.connection.rebase = func(branch.RebaseRequest) (branch.RebaseResult, error) {
		return branch.RebaseResult{State: "held", OldTip: tip, NewTip: tip, MainTip: owners.status.EndpointTip, Carried: []string{}, NeedsReview: []string{}}, nil
	}
	b.connection.recordRebase = func(*intentInvocation, string, branch.RebaseResult) error { return nil }
	install := filepath.Join(t.TempDir(), "lane", "metasystem")
	b.owners.laneInstall = func(root string) (string, error) {
		if root != "/landing" {
			t.Errorf("the lane installation was asked for %q", root)
		}
		return install, nil
	}
	provedPerson := enrolledPersonProver(t, b.root(), syncRequestTestNow)
	notAncestor := exec.Command("/usr/bin/false").Run()
	home := t.TempDir()
	b.laneInputs = func(invOwners *intentOwners) {
		root, configured, err := b.owners.laneRoot(b.root(), laneTestNow)
		helmMust(t, err)
		// Only registered lanes supply lane-policy inputs (lane-reads-its-policies.md:45).
		if !configured {
			return
		}
		currentInstall, err := b.owners.laneInstall(root)
		helmMust(t, err)
		b.writeFile(filepath.Join(currentInstall, "metasystem.conf"), "metasystem.template=true\nlanding.trunk-red=auto\n")
		b.writeJSON(lane.RecordPath(home), lane.Record{Root: filepath.Dir(currentInstall), Install: currentInstall, CustodyEpoch: 1, RegisteredBy: "Wido", At: laneTestNow.Format(time.RFC3339)})
		invOwners.commandNow = func(string) (time.Time, error) { return syncRequestTestNow, nil }
		invOwners.landing.home = func() (string, error) { return home, nil }
		invOwners.landing.machine = func(string) (string, error) { return "fixture", nil }
		invOwners.landing.plainProve.Git = func(_ string, args ...string) (string, error) {
			if args[0] == "fetch" {
				return "", nil
			}
			if args[0] == "rev-parse" {
				return "main", nil
			}
			if args[0] == "merge-base" {
				return "", notAncestor
			}
			return "", nil
		}
		invOwners.landing.plainProve.Incidents = func(string, string, string) ([]goal.TrunkRedEntry, error) {
			data, exists := b.repo.commit(b.repo.accepted).files["plans/goals/trunk-red.json"]
			if !exists {
				return nil, nil
			}
			entries, problems := goal.ParseTrunkRed(data)
			if len(problems) > 0 {
				return nil, fmt.Errorf("%v", problems)
			}
			return entries, nil
		}
		invOwners.policies = config.PolicyReaders{Registry: func(string) (config.PolicyRegistry, error) {
			return config.PolicyRegistry{Lane: filepath.Dir(currentInstall)}, nil
		}, Helm: func(string) helm.State { return helm.State{} }, ConfPath: func(string) (string, error) { return filepath.Join(currentInstall, "metasystem.conf"), nil }}
		invOwners.prove = provedPerson
	}
	return b, owners, install
}

func TestWorkLandAgainAfterReturn(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"explicit", "regenerated", "resolved", "unknown", "empty", "untouched", "partial", "changed"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			b := newLandRebaseBed(t)
			b.rebase.State, b.rebase.NewTip = "held", b.rebase.OldTip
			b.rebase.Regenerated = []string{"gen/out"}
			line := plain.Line{Goal: "standing-validation", Branch: "goal/standing-validation", SHA: b.rebase.OldTip, Delivered: "Makes landing reliable"}
			if _, _, err := plain.HandIn(b.lane, line); err != nil {
				t.Fatal(err)
			}
			line.Outcome, line.Reason = plain.StateReturned, "app-standard fails since it joined"
			if name != "explicit" && name != "unknown" {
				line.Conflict = &conflict.Return{Paths: []conflict.Path{{Path: "gen/out", Class: conflict.Generated}}}
				line.Reason = "merge conflict"
			}
			switch name {
			case "empty":
				line.Conflict.Paths = nil
			case "untouched":
				line.Conflict.Paths[0].Path = "source.go"
			case "partial":
				line.Conflict.Paths = append(line.Conflict.Paths, conflict.Path{Path: "source.go", Class: conflict.Builder})
			case "resolved":
				line.Conflict.Paths = append(line.Conflict.Paths, conflict.Path{Path: "source.go", Class: conflict.Builder})
				b.rebase.Resolved = []string{"source.go"}
			case "changed":
				b.rebase.OldTip = strings.Repeat("9", 40)
			}
			queue := filepath.Join(plain.Dir(b.lane), "queue.jsonl")
			before, err := os.ReadFile(queue)
			if err != nil {
				t.Fatal(err)
			}
			returned, err := json.Marshal(line)
			if err != nil {
				t.Fatal(err)
			}
			b.writeFile(queue, string(before)+string(returned)+"\n")
			args := []string{}
			if name == "explicit" {
				args = append(args, "--again")
			}
			code, result := b.runJSON(b.owners, append([]string{"work", "land", line.Goal}, args...)...)
			want := intentRefused
			// Landing requires an explicit retry even after paths were resolved.
			if name == "explicit" {
				want = intentConfirmed
			}
			expectOutcome(t, "return", code, result, want)
			if want == intentRefused {
				wantNext := "metasystem work rebase " + line.Goal
				if line.Conflict == nil {
					wantNext = "metasystem work land " + line.Goal + " --json"
				}
				if !strings.Contains(result.Summary, "returned: "+line.Reason) || result.Next == nil || strings.Join(result.Next.Argv, " ") != wantNext {
					t.Fatalf("return remedy: %+v", result)
				}
				after, err := os.ReadFile(queue)
				if err != nil || string(after) != string(before)+string(returned)+"\n" {
					t.Fatalf("refused return wrote queue: %q err=%v", after, err)
				}
				return
			}
			entries, err := plain.Entries(b.lane)
			if err != nil || len(entries) != 2 || entries[0].State != plain.StateReturned || entries[0].Reason != line.Reason || entries[1].State != plain.StateWaiting || entries[1].Delivered != line.Delivered || !strings.Contains(result.Summary, "re-queued after a return that needed no change") {
				t.Fatalf("re-queue: %+v entries=%+v err=%v", result, entries, err)
			}
			if strings.Contains(result.Summary, "channel will say nothing") {
				t.Fatalf("re-queue lost its delivery sentence: %+v", result)
			}
			before, _ = os.ReadFile(queue)
			code, result = b.runJSON(b.owners, "work", "land", line.Goal, "--again", "--delivered", "Another sentence")
			expectOutcome(t, "waiting", code, result, intentUnchanged)
			after, err := os.ReadFile(queue)
			if err != nil || string(after) != string(before) || !strings.Contains(result.Summary, "waiting") {
				t.Fatalf("repeat wrote queue: %q -> %q; %+v err=%v", before, after, result, err)
			}
		})
	}
}

// Plain lane step 1: with a lane registered, work land G passes the seat's
// gates, appends one line {goal, branch, sha, seat, at} to the lane's
// queue.jsonl and says "handed to the lane"; a repeat at the same sha is
// success and appends nothing; work land G then shows the line's state:
// waiting, returned with its reason, and landed once main contains its sha
// (derived, nothing recorded), naming goal done. A new sha after a return
// hands in again. Nothing is proved or pushed by the seat.
func TestWorkLandHandsInToThePlainLane(t *testing.T) {
	t.Parallel()
	b, owners, install := plainLaneBed(t, "critic-root", "critic-root")
	code, result := b.do("work", "land", "standing-validation")
	expectOutcome(t, "hand-in", code, result, intentConfirmed)
	if !strings.Contains(result.Summary, "handed to the lane") || len(owners.pushes) != 0 || owners.candidates != 0 {
		t.Fatalf("hand-in: %+v pushes=%v candidates=%d", result, owners.pushes, owners.candidates)
	}
	queue := filepath.Join(install, "artifacts", "agents", "landing", "queue.jsonl")
	data, err := os.ReadFile(queue)
	if err != nil || strings.Count(string(data), "\n") != 1 {
		t.Fatalf("one queue line: %q %v", data, err)
	}
	entries, err := plain.Entries(install)
	if err != nil || len(entries) != 1 {
		t.Fatalf("entries: %+v %v", entries, err)
	}
	if got := entries[0]; got.Goal != "standing-validation" || got.Branch != "goal/standing-validation" || got.SHA != strings.Repeat("2", 40) || got.Seat == "" || got.At == "" {
		t.Fatalf("the line: %+v", got)
	}

	code, result = b.do("work", "land", "standing-validation")
	expectOutcome(t, "repeat", code, result, intentUnchanged)
	if !strings.Contains(result.Summary, "waiting") {
		t.Fatalf("a repeat shows the waiting line: %+v", result)
	}
	if data, _ := os.ReadFile(queue); strings.Count(string(data), "\n") != 1 {
		t.Fatalf("a repeat appends nothing: %q", data)
	}

	if _, _, err := plain.ReturnProven(install, "standing-validation", "unclassified", "app-standard fails since it joined", true, "fixture", time.Now(), plain.ProveSeams{Person: &plain.ActProvenance{Kind: "return", Person: "fixture"}}); err != nil {
		t.Fatal(err)
	}
	code, result = b.do("work", "land", "standing-validation")
	expectOutcome(t, "returned", code, result, intentRefused)
	if !strings.Contains(result.Summary, "returned: app-standard fails since it joined") {
		t.Fatalf("the seat sees the return: %+v", result)
	}

	// The seat fixed it: a new sha hands in again. It is a real commit of
	// the seat's repository, and once main holds it the line reads landed.
	git := func(args ...string) string {
		out, err := exec.Command("git", append([]string{"-C", b.root(), "-c", "user.name=seat", "-c", "user.email=seat@example.invalid", "-c", "commit.gpgsign=false"}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("commit", "--quiet", "--allow-empty", "-m", "the fix")
	fixed := git("rev-parse", "HEAD")
	owners.status.BranchTip = fixed
	code, result = b.do("work", "land", "standing-validation")
	expectOutcome(t, "hand-in again", code, result, intentConfirmed)
	code, result = b.do("work", "land", "standing-validation")
	expectOutcome(t, "waiting again", code, result, intentUnchanged)
	if !strings.Contains(result.Summary, "waiting") {
		t.Fatalf("not yet in main: %+v", result)
	}
	owners.status.EndpointTip = fixed
	code, result = b.do("work", "land", "standing-validation")
	expectOutcome(t, "landed", code, result, intentUnchanged)
	if !strings.Contains(result.Summary, "landed on main") || result.Next == nil || !strings.Contains(strings.Join(result.Next.Argv, " "), "goal done standing-validation") {
		t.Fatalf("the seat sees the landing and concludes the goal itself: %+v", result)
	}
	if data, _ := os.ReadFile(queue); strings.Count(string(data), "\n") != 3 {
		t.Fatalf("landing records nothing: %q", data)
	}
}

// The seat's gates stay at hand-in: a unit without a clean read is refused
// there and nothing is queued.
func TestWorkLandKeepsTheReadGateBeforeTheHandIn(t *testing.T) {
	t.Parallel()
	b, _, install := plainLaneBed(t, "critic-root")
	b.owners.branchState = func(string, string) (intentBranchState, error) { return readBranch(1, "critic-root"), nil }
	code, result := b.do("work", "land", "standing-validation")
	expectOutcome(t, "unread unit", code, result, intentRefused)
	if !strings.Contains(result.Summary, "no clean read") {
		t.Fatalf("the read gate refuses: %+v", result)
	}
	if entries, _ := plain.Entries(install); len(entries) != 0 {
		t.Fatalf("a refused hand-in queued: %+v", entries)
	}
}
