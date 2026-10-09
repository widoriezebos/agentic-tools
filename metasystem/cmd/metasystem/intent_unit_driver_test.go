package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/readsubject"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

// The subprocess gives this parallel test its own executable lookup. Git is
// unavailable: the executable implements only the fixture's recorded facts.
func TestDriverPublicReviewPublication(t *testing.T) {
	t.Parallel()
	if os.Getenv("METASYSTEM_DRIVER_TEST") == "" {
		python, err := exec.LookPath("python3")
		if err != nil {
			t.Fatal(err)
		}
		bin := t.TempDir()
		if err := testexec.WriteFile(filepath.Join(bin, "git"), []byte("#!"+python+"\n"+driverGitScript), 0700); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(os.Args[0], "-test.run=^TestDriverPublicReviewPublication$", "-test.timeout=30m", "-test.v")
		cmd.Env = append(os.Environ(), "METASYSTEM_DRIVER_TEST=1", "PATH="+bin)
		out, err := cmd.CombinedOutput()
		t.Log(string(out))
		if err != nil {
			t.Fatal(err)
		}
		return
	}
	for _, scenario := range []string{"clean", "committed critic", "judgements", "stop", "person", "helm", "publication changes helm", "check reds", "held proof", "held build"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			d := newDriverReviewFixture(t, scenario)
			var heldRun string
			if strings.HasPrefix(scenario, "held ") {
				kind := strings.TrimPrefix(scenario, "held ")
				file := *d.bed.goalFile(d.bed.id)
				file.Id, file.Priority, file.Sequence = "a-held-goal", 1, 1
				d.bed.addGoal(&file)
				other := &workBed{intentBed: d.bed.intentBed, id: file.Id, worktree: filepath.Join(filepath.Dir(d.bed.worktree), "held-work"), manager: d.bed.manager, starter: d.bed.starter,
					unitRoot: d.bed.unitRoot, branchListed: true, head: d.bed.head, designGate: d.bed.designGate, workOwnersHook: d.bed.workOwnersHook, readDirs: map[string]bool{}}
				if err := os.MkdirAll(other.worktree, 0700); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(other.removeReadDirs)
				other.starter.fail[kind] = true
				other.manager.Supervisor = driverReviewStarter{d, &stopReadStarter{bed: other, reads: [][]readsubject.Finding{nil}}}
				owners := other.workOwners()
				owners.work.config = d.owners.work.config
				otherGit := owners.work.git
				owners.work.git = func(root string, args ...string) ([]byte, error) {
					if data, ok := d.raw.responses[branchRawKey(args...)]; ok {
						return data, nil
					}
					return otherGit(root, args...)
				}
				owners.binding = func(_ string, id string, _ time.Time) (dispatchcore.GoalBinding, error) {
					binding := *d.bed.initialBinding
					binding.GoalID, binding.File = id, &file
					return binding, nil
				}
				brief := other.brief("held.md", "Build the held unit.\n")
				code, held := other.runJSON(owners, "work", "build", other.id, "--work", "a-held", "--brief", brief, "--lines", "10")
				if code != 1 || resultData(t, held)["state"] != "awaiting-judgement" {
					t.Fatalf("unit not held: %d %+v", code, held)
				}
				heldRun = resultData(t, held)["run"].(string)
				other.starter.fail[kind] = false
				d.bed.manager.Supervisor = driverReviewStarter{d, &stopReadStarter{bed: d.bed, reads: [][]readsubject.Finding{nil}}}
				units := d.owners.work.units
				d.owners.work.units = func(layout stateroot.Layout) *launch.UnitRunner {
					runner := units(layout)
					runner.Git = driverHeldTreeGit{runner.Git, other}
					return runner
				}
				git := d.owners.work.git
				d.owners.work.git = func(root string, args ...string) ([]byte, error) {
					data, err := git(root, args...)
					if slices.Equal(args, []string{"worktree", "list", "--porcelain"}) && err == nil {
						data = append(data, []byte("\nworktree "+other.worktree+"\nHEAD "+other.head+"\nbranch refs/heads/goal/"+other.id+"\n")...)
					}
					return data, err
				}
			}
			brief := d.bed.brief("unit.md", "Read each round: yes\nBuild the unit.\n")
			code, built := d.bed.runJSON(d.owners, "work", "build", d.bed.id, "--json", "--work", "u1", "--brief", brief, "--lines", "10", "--read-tool-calls", "12")
			if code != 0 && scenario != "check reds" {
				t.Fatalf("build %d %+v", code, built)
			}
			run := resultData(t, built)["run"].(string)
			d.run = run
			if scenario == "helm" {
				d.takeHelm(t)
			}
			if heldRun != "" {
				held, err := (&launch.UnitRunner{Root: d.bed.unitRoot}).Status(heldRun)
				if err != nil || held.State != "awaiting-judgement" {
					t.Fatalf("held input moved: %+v %v", held, err)
				}
			}
			d.cycle(t)
			if scenario == "committed critic" || scenario == "judgements" || scenario == "stop" {
				d.cycle(t)
			}
			current := d.record(t)
			if current.ReviewAct == nil {
				t.Fatal("steward did not retain the next review act")
			}
			if heldRun != "" {
				held, err := (&launch.UnitRunner{Root: d.bed.unitRoot}).Status(heldRun)
				if err != nil || held.ReviewAct == nil || held.ReviewAct.State == "satisfied" || len(held.Revisions) != 0 || current.ReviewAct.State != "satisfied" || d.commits != 1 || d.readCommits != 1 {
					t.Fatalf("held unit hid later publication: held=%+v clean=%+v commits=%d reads=%d err=%v", held, current.ReviewAct, d.commits, d.readCommits, err)
				}
				return
			}
			if scenario == "check reds" {
				if current.Rounds[0].Outcome != "proof-red" || len(current.Revisions) != 0 || d.commits != 0 {
					t.Fatalf("check red did not hold: %+v", current.ReviewAct)
				}
				if !slices.Contains(current.ReviewAct.Command, "revise") || !slices.Contains(current.ReviewAct.Command, "--by") {
					t.Fatalf("check correction remedy unavailable: %+v", current.ReviewAct)
				}
				d.bed.starter.fail["proof"] = false
				correction := d.bed.brief("proof-correction.md", "Correct the fixture-red check together with the unit behavior.\n")
				args := []string{"work", "revise", d.bed.id, "--work", "u1", "--after", "1", "--brief", correction, "--reason", "Correct the retained failed check", "--by", "Wido"}
				code, revised := d.bed.runJSON(d.owners, args...)
				if code != 0 {
					t.Fatalf("check correction %d %+v", code, revised)
				}
				code, revised = d.bed.runJSON(d.owners, args...)
				current = d.record(t)
				if code != 0 || len(current.Revisions) != 1 || len(current.Rounds) != 2 || current.Rounds[1].Outcome != "green" {
					t.Fatalf("check correction replay %d %+v", code, revised)
				}
				if !strings.Contains(string(mustRead(t, filepath.Join(current.Rounds[1].Directory, "checked-build.md"))), "fixture-red") {
					t.Fatal("correction omitted the check red")
				}
				d.cycle(t)
				current = d.record(t)
				if current.ReviewAct.State != "satisfied" || d.commits != 1 || d.readCommits != 1 {
					t.Fatalf("corrected result not published: %+v", current.ReviewAct)
				}
				return
			} else if scenario == "person" || scenario == "helm" {
				if current.ReviewAct.State != "prepared" || d.commits != 0 || d.reads != 0 {
					t.Fatalf("observe-only %+v", current)
				}
				// A person's ordinary public review remains executable under the policy.
				code, reviewed := d.bed.runJSON(d.owners, "work", "review", d.bed.id, "--work", "u1")
				if code != 0 && reviewed.Outcome != intentInProgress {
					t.Fatalf("prepared command %d %+v", code, reviewed)
				}
				d.cycle(t)
			} else if scenario == "publication changes helm" {
				if current.ReviewAct.State == "satisfied" || d.publications != 0 {
					t.Fatalf("publication crossed helm %+v", current.ReviewAct)
				}
				os.Remove(filepath.Join(d.bed.root(), ".git", "metasystem", "helm.json"))
				d.cycle(t)
			} else if scenario == "judgements" {
				if current.ReviewAct.State != "judgement" || len(current.Revisions) != 0 {
					t.Fatalf("judgement missing: act=%+v revisions=%d", current.ReviewAct, len(current.Revisions))
				}
				template := current.ReviewAct.Template
				body := string(mustRead(t, template))
				if !strings.Contains(body, "DECIDE") {
					t.Fatalf("driver filled decisions: %s", body)
				}
				code, status := d.bed.runJSON(d.owners, "work", "status", d.bed.id, "--work", "u1")
				if code != 0 || status.Next == nil || !slices.Contains(status.Next.Argv, "--dispositions") {
					t.Fatalf("judgement invisible %+v", status)
				}
				code, waited := d.bed.runJSON(d.owners, "work", "wait", d.bed.id, "--work", "u1", "--timeout", "1s")
				if code != 0 && code != 3 || waited.Next == nil || !slices.Contains(waited.Next.Argv, "revise") {
					t.Fatalf("worker wake code=%d next=%+v", code, waited.Next)
				}
				correction := d.bed.brief("correction.md", "Fold the read findings and check reds in one correction.\n\n## Decisions on round 1\n\n| driver-critic:1 | fixed | unit.go:12 |\n")
				args := []string{"work", "revise", d.bed.id, "--work", "u1", "--after", "1", "--brief", correction, "--dispositions", template}
				code, refused := d.bed.runJSON(d.owners, args...)
				if code == 0 || d.record(t).Revisions != nil {
					t.Fatalf("undecided correction accepted %+v", refused)
				}
				body = strings.ReplaceAll(body, "| DECIDE | | |", "| accepted | Fix the missing behavior | |")
				if err := os.WriteFile(template, []byte(body), 0600); err != nil {
					t.Fatal(err)
				}
				// Capture the worker's bound route, then lose its response and repeat it.
				code, revised := d.bed.runJSON(d.owners, args...)
				if code != 0 {
					t.Fatalf("worker revise %d %+v", code, revised)
				}
				launches := len(d.bed.starter.launched())
				code, repeated := d.bed.runJSON(d.owners, args...)
				if code != 0 || len(d.record(t).Revisions) != 1 || len(d.record(t).Rounds) != 2 || len(d.bed.starter.launched()) != launches {
					t.Fatalf("revision replay %+v", repeated)
				}
				d.cycle(t)
				// Current round supersedes the older judgement and is published by the ordinary owners.
				current = d.record(t)
				if current.Rounds[1].Outcome != "green" {
					t.Fatalf("correction did not pass: %s", current.Rounds[1].Outcome)
				}
				if current.ReviewAct.State != "satisfied" {
					d.cycle(t)
					current = d.record(t)
				}
				if act := current.ReviewAct; act.Round != 2 || act.State != "satisfied" || len(current.Revisions) != 1 || d.commits != 2 || d.readCommits != 1 {
					t.Fatalf("corrected publication: %+v commits=%d reads=%d", act, d.commits, d.readCommits)
				}
				d.cycle(t)
				if d.commits != 2 || d.readCommits != 1 {
					t.Fatal("correction replay made another commit or read")
				}
				return
			} else if scenario == "stop" {
				if len(current.Revisions) != 0 || current.ReviewAct.State != "judgement" || current.Rounds[0].Stop.Decision != "stop" {
					t.Fatalf("stop ignored %+v", current)
				}
				questions, damaged := channel.WalkOpenQuestions(d.bed.stateRoot())
				if len(damaged) > 0 || len(questions) != 1 || questions[0].UnitStop == nil || questions[0].UnitStop.Needs == "" {
					t.Fatalf("stop ask missing: %+v %v", questions, damaged)
				}
				d.cycle(t)
				if len(d.record(t).Rounds) != 1 {
					t.Fatal("stopped unit got another correction")
				}
				return
			} else {
				if current.ReviewAct.State == "satisfied" {
					t.Fatal("lost publication response was certified before branch confirmation")
				}
				d.cycle(t)
			}
			wantPublications := 1
			if scenario == "clean" || scenario == "committed critic" {
				wantPublications = 2
			}
			current = d.record(t)
			if current.ReviewAct.State != "satisfied" || current.ReviewAct.Effect != d.raw.read || d.commits != 1 || d.readCommits != 1 || d.publications != wantPublications || d.pushes != 2 {
				t.Fatalf("publication not confirmed %+v commits=%d read commits=%d publications=%d", current.ReviewAct, d.commits, d.readCommits, d.publications)
			}
			before := d.reads
			d.cycle(t)
			if d.reads != before || d.commits != 1 || d.readCommits != 1 {
				t.Fatal("publication replay made another effect")
			}
			if scenario == "committed critic" && d.delegates != 1 || scenario == "clean" && d.delegates != 0 {
				t.Fatalf("wrong read route: delegates=%d", d.delegates)
			}
			code, status := d.bed.runJSON(d.owners, "work", "status", d.bed.id, "--work", "u1")
			if code != 0 || !strings.Contains(resultWords(status), "reviewed") {
				t.Fatalf("status %+v", status)
			}
		})
	}
}

type driverReviewGit struct{ d *driverReviewFixture }

func (g driverReviewGit) Run(dir string, env []string, args ...string) ([]byte, error) {
	d := g.d
	if len(args) > 0 && args[0] == "diff" && slices.Contains(args, "--raw") {
		if d.bed.head == d.raw.base || d.dirty {
			return branchRawEntry("unit.go"), nil
		}
		return nil, nil
	}
	if len(args) > 0 && args[0] == "diff" && slices.Contains(args, "--binary") {
		return []byte("unit patch\n"), nil
	}
	if len(args) > 0 && args[0] == "write-tree" {
		return []byte(d.raw.tree), nil
	}
	return (workGit{d.bed}).Run(dir, env, args...)
}

type driverReviewFixture struct {
	bed                                                          *workBed
	raw                                                          *branchRawFixture
	owners                                                       intentOwners
	run, policy, scenario                                        string
	commits, reads, readCommits, publications, delegates, pushes int
	losePublication                                              bool
	dirty, corrected                                             bool
}

func newDriverReviewFixture(t *testing.T, scenario string) *driverReviewFixture {
	bed := newStopWorkBed(t)
	d := &driverReviewFixture{bed: bed, policy: "auto", scenario: scenario, losePublication: scenario == "clean" || scenario == "committed critic"}
	bed.head = branchRawID("a")
	if scenario == "check reds" {
		bed.starter.fail["proof"] = true
	}
	f := &branchRawFixture{t: t, project: bed.worktree, installation: bed.worktree, base: bed.head, unit: branchRawID("b"), read: branchRawID("d"), tree: branchRawID("c"), unitPath: "unit.go", responses: map[string][]byte{}, used: map[string]int{}}
	d.raw = f
	for _, root := range []string{bed.root(), bed.worktree} {
		os.MkdirAll(filepath.Join(root, ".git", "metasystem"), 0700)
	}
	os.WriteFile(filepath.Join(bed.worktree, "metasystem.conf"), nil, 0600)
	f.add(f.base+"\n", "merge-base", f.base, f.unit)
	f.add(f.unit+" "+f.base+"\n", "rev-list", "--first-parent", "--reverse", "--parents", f.base+".."+f.unit)
	f.add("Goal-Unit: "+bed.id+"/u1\n", "show", "-s", "--format=%(trailers:only,unfold=true)", f.unit)
	f.add(f.unit+" "+f.base+"\n", "rev-list", "--parents", "-n", "1", f.unit)
	f.add("unit.go\n", "diff-tree", "-r", "--no-commit-id", "--name-only", f.unit+"^", f.unit)
	f.responses[branchRawKey("diff-tree", "-r", "-z", "--no-renames", "--full-index", f.unit+"^", f.unit)] = branchRawEntry("unit.go")
	f.add("unit patch\n", "diff", "--no-color", "--no-ext-diff", "--no-textconv", "--no-relative", "--binary", "--full-index", "--no-renames", "--src-prefix=a/", "--dst-prefix=b/", "-U3", f.unit+"^", f.unit)
	f.add(f.base, "rev-parse", f.unit+"^")
	f.add(f.unit, "rev-parse", "--verify", "--end-of-options", f.unit+"^{commit}")
	f.add(f.base, "rev-parse", "--verify", "--end-of-options", f.unit+"^")
	f.add(f.tree, "rev-parse", "--verify", "--end-of-options", f.unit+"^{tree}")
	f.add(f.installation, "rev-parse", "--show-toplevel")
	f.add(f.tree, "rev-parse", f.unit+"^{tree}")
	f.add(f.tree, "rev-parse", f.base+"^{tree}")
	f.add("", "rev-parse", "--show-prefix")
	f.add("proof.cheap=true\nproof.audits=true\nproof.deadline=15\n", "show", f.base+":metasystem.conf")
	f.add("proof.cheap=true\nproof.audits=true\nproof.deadline=15\n", "show", f.unit+":metasystem.conf")
	findings := [][]readsubject.Finding{nil}
	if scenario == "judgements" || scenario == "stop" {
		findings = [][]readsubject.Finding{{stopFinding("regression", "unit.go")}, nil}
	}
	if scenario == "stop" {
		page, data := designGatePage(t, bed, "- Critique: closed at round 1 on 0 material findings (reader)")
		data = append(data, []byte("\n| Unit | Purpose | Estimated changed lines |\n| --- | --- | --- |\n| required-other | Complete required behavior | 5 |\n")...)
		if err := os.WriteFile(page, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	bed.manager.Supervisor = driverReviewStarter{d, &stopReadStarter{bed: bed, reads: findings}}
	if scenario == "person" {
		d.policy = "person"
	}
	owners := bed.workOwners()
	owners.work.config = func(key, _ string) (string, string, int, error) {
		if key == "seat.driver" {
			return d.policy, "fixture", 0, nil
		}
		switch key {
		case "design.gate.mode":
			return "warn", "fixture", 0, nil
		case "testing.contract":
			return "", "fixture", 1, nil
		}
		return "auto", "fixture", 0, nil
	}
	owners.work.resolveModel = func(_, _, model string) (string, error) {
		if scenario == "committed critic" {
			return "same-model", nil
		}
		return model, nil
	}
	owners.work.units = func(stateroot.Layout) *launch.UnitRunner {
		return &launch.UnitRunner{Root: bed.unitRoot, Manager: bed.manager, Git: driverReviewGit{d}, ReviewPolicy: func() (string, error) {
			if scenario == "stop" {
				return "0", nil
			}
			return "auto", nil
		}}
	}
	git := owners.work.git
	owners.work.git = func(dir string, args ...string) ([]byte, error) {
		key := branchRawKey(args...)
		if out, ok := f.responses[key]; ok {
			return out, nil
		}
		if slices.Equal(args, []string{"diff", "--cached", "--name-only", "-z"}) {
			return []byte("unit.go\x00"), nil
		}
		if slices.Contains(args, "add") {
			return nil, nil
		}
		if slices.Equal(args, []string{"diff", "--cached", "--raw", "-z", "--no-abbrev", "HEAD", "--", "."}) {
			return branchRawEntry("unit.go"), nil
		}
		if slices.Equal(args, []string{"write-tree"}) {
			return []byte(f.tree), nil
		}
		return git(dir, args...)
	}
	owners.connection.endpointTip = func(string, goal.Endpoint) (string, error) { return f.base, nil }
	owners.connection.rebaseGate = func(string) (string, error) { return "rebase-gate", nil }
	owners.connection.operationID = func() (string, error) { return "review-operation", nil }
	owners.connection.commitToken = func(_ string, body func() error) error { return body() }
	owners.connection.transport = driverReviewTransport{d}
	owners.connection.commit = func(req branch.CommitRequest) (string, error) { return d.commit(t, req) }
	owners.connection.push = branch.Push
	owners.work.inspectRead = branch.InspectBranchRead
	owners.delivery = &intentDeliveryOwners{
		branchState:  func(string, string) (intentBranchState, error) { return intentBranchState{}, nil },
		recordWriter: humanRecordWriter,
		closeOwner: func(root string, args []string) intentProcessResult {
			return d.closeCritic(t, root, args)
		},
		branchRead: func(args []string) (branch.BranchReadResult, int, error) { return d.read(t, args) },
		publishRead: func(root, id, unit string) (branch.PublishReadResult, error) {
			d.publications++
			hooks := branch.PushHooks{}
			if d.losePublication {
				d.losePublication = false
				hooks.AfterPush = func() error { return errors.New("publication response lost") }
			}
			return branch.PublishCollectedRead(branch.PublishReadRequest{Repo: root, Remote: "origin", EndpointTip: f.base, GoalID: id, UnitCommit: unit, CheckClaim: func() error { return nil }, Transport: driverReviewTransport{d}, Repository: f, Hooks: hooks})
		},
	}
	d.owners = owners
	d.syncGit(t, map[string]string{})
	return d
}

func (d *driverReviewFixture) takeHelm(t *testing.T) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(d.bed.root(), ".git", "metasystem", "helm.json"), []byte(`{"schema":1,"by":"Wido","at":"2026-09-01T10:00:00Z"}`), 0600); err != nil {
		t.Fatal(err)
	}
}
func (d *driverReviewFixture) record(t *testing.T) launch.UnitRunRecord {
	t.Helper()
	record, err := (&launch.UnitRunner{Root: d.bed.unitRoot}).Status(d.run)
	if err != nil {
		t.Fatal(err)
	}
	return record
}
func (d *driverReviewFixture) cycle(t *testing.T) {
	t.Helper()
	var output bytes.Buffer
	code := runStewardRunWithDependencies([]string{"--repo", d.bed.root()}, &output, &output, func(root string, _ steward.WorkerCensus, _ func() error, _ time.Duration, cfg steward.TickConfig) error {
		if cfg.ReviewWork == nil {
			return errors.New("steward review callback missing")
		}
		return cfg.ReviewWork(root)
	}, func(string, string) error { return nil }, nil, func(string) int { return 1 }, nil, d.owners)
	if code != 0 {
		t.Fatalf("public steward %d %s", code, &output)
	}
}

func (d *driverReviewFixture) syncGit(t *testing.T, refs map[string]string) {
	t.Helper()
	path := filepath.Join(d.bed.worktree, ".git", "driver-git.json")
	var state map[string]any
	if data, err := os.ReadFile(path); err == nil {
		json.Unmarshal(data, &state)
	} else {
		state = map[string]any{"refs": map[string]any{}, "blobs": map[string]string{}}
	}
	if refs != nil {
		kept := state["refs"].(map[string]any)
		for key, value := range refs {
			kept[key] = value
		}
		state["refs"] = kept
	}
	replies := map[string]string{}
	for key, value := range d.raw.responses {
		replies[key] = base64.StdEncoding.EncodeToString(value)
	}
	state["replies"] = replies
	state["parents"] = map[string]string{branchRawID("b"): d.raw.base, branchRawID("e"): d.raw.base, d.raw.read: d.raw.unit}
	data, _ := json.Marshal(state)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
}

type driverReviewTransport struct{ d *driverReviewFixture }

func (x driverReviewTransport) RemoteTip(repo, remote, ref string) (string, bool, error) {
	data, err := os.ReadFile(filepath.Join(x.d.bed.worktree, ".git", "driver-git.json"))
	if err != nil {
		return "", false, err
	}
	var state struct{ Refs map[string]string }
	json.Unmarshal(data, &state)
	tip := state.Refs["remote"]
	return tip, tip != "", nil
}
func (x driverReviewTransport) Fetch(string, string, string, string) error { return nil }
func (x driverReviewTransport) Push(repo, remote, ref, expected, tip string) (branch.CASOutcome, error) {
	path := filepath.Join(x.d.bed.worktree, ".git", "driver-git.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return branch.CASUnknown, err
	}
	var state map[string]any
	json.Unmarshal(data, &state)
	refs := state["refs"].(map[string]any)
	if prior, _ := refs["remote"].(string); prior != expected {
		return branch.CASRefused, errors.New("remote moved")
	}
	x.d.pushes++
	refs["remote"] = tip
	data, _ = json.Marshal(state)
	err = os.WriteFile(path, data, 0600)
	return branch.CASLanded, err
}

func (d *driverReviewFixture) commit(t *testing.T, req branch.CommitRequest) (string, error) {
	f := d.raw
	inputs := f.inputs()
	facts, effects := inputs.Facts, inputs.Effects
	effects.ClearFetch = func(string, string) error { return nil }
	head := f.base
	if req.Amend {
		head = f.unit
		facts.Tip = func(_, ref string) (string, bool, error) { return f.unit, true, nil }
		facts.Suffix = func(string, string, string) ([]string, error) { return nil, nil }
		facts.Tree = func(string) (string, error) { return f.tree, nil }
		effects.WithoutPaths = func(_, tree string, paths []string) (string, error) {
			if len(paths) != 0 {
				t.Fatal(paths)
			}
			return tree, nil
		}
		effects.KeepTip = func(_, id, tip string) error {
			d.syncGit(t, map[string]string{"refs/metasystem/goals/before/" + id + "/" + tip: tip})
			return nil
		}
	}
	if !req.Amend {
		facts.Tip = func(string, string) (string, bool, error) { return "", false, nil }
	}
	facts.Head = func(repo string) (string, error) {
		if repo == f.installation {
			return d.bed.head, nil
		}
		return head, nil
	}
	facts.Staged = func(string) ([]string, error) { return []string{"unit.go"}, nil }
	facts.Patch = func(string) ([]byte, error) { return []byte("unit patch\n"), nil }
	effects.Apply = func(string, []byte) error { return nil }
	effects.Commit = func(_, _, trailer string, amend bool) error {
		if trailer != "Goal-Unit: "+d.bed.id+"/u1" || amend != req.Amend {
			t.Fatalf("commit %q amend=%t", trailer, amend)
		}
		if amend {
			next := branchRawID("e")
			for key, value := range f.responses {
				f.responses[strings.ReplaceAll(key, f.unit, next)] = value
			}
			f.add(next+" "+f.base+"\n", "rev-list", "--first-parent", "--reverse", "--parents", f.base+".."+next)
			f.add(next+" "+f.base+"\n", "rev-list", "--parents", "-n", "1", next)
			f.add(next, "rev-parse", "--verify", "--end-of-options", next+"^{commit}")
			f.add("", "rev-list", "--reverse", f.unit+".."+f.unit)
			f.add("", "rev-list", "--reverse", next+".."+next)
			f.add("", "diff-tree", "-r", "-z", "--name-only", f.tree, next+"^{tree}")
			f.unit = next
			d.dirty, d.corrected = false, true
		}
		head = f.unit
		d.commits++
		return nil
	}
	effects.Checkout = func(string, string, string) error { return nil }
	effects.Publish = func(_, _, _, tip, _ string) error {
		d.bed.head = tip
		f.tip = tip
		d.syncGit(t, map[string]string{"refs/heads/goal/" + d.bed.id: tip})
		return nil
	}
	return branch.CommitStagedWithInputs(req, facts, effects)
}

func (d *driverReviewFixture) read(t *testing.T, args []string) (branch.BranchReadResult, int, error) {
	d.reads++
	f := d.raw
	inputs := f.inputs()
	inputs.Effects.ClearFetch = func(string, string) error { return nil }
	inputs.Effects.Publish = func(repo, id, old, next, origin string) error {
		if old != f.unit || next != f.read || origin != f.unit {
			t.Fatalf("attestation publication %s %s %s", old, next, origin)
		}
		f.tip, f.published = next, true
		{
			d.bed.head = next
			d.syncGit(t, map[string]string{"refs/heads/goal/" + id: next, "remote": f.unit, "refs/metasystem/goals/origin/" + id: f.unit})
		}
		return nil
	}
	commit := inputs.Effects.Commit
	inputs.Effects.Commit = func(dir, subject, trailer string, amend bool) error {
		err := commit(dir, subject, trailer, amend)
		if err == nil {
			d.readCommits++
			for path, content := range f.generated {
				f.add(string(content), "show", f.read+":"+path)
			}
			d.syncGit(t, nil)
		}
		return err
	}
	body, _ := os.ReadFile(flagValue(args, "--unit-read"))
	result, err := branch.RunBranchRead(branch.BranchReadRequest{Repo: f.installation, Remote: "origin", EndpointTip: f.base, BranchTip: f.tip, GoalID: d.bed.id, UnitCommit: f.unit,
		Repository: f, BriefPath: flagValue(args, "--brief"), BuildBriefSHA256: flagValue(args, "--build-brief-sha256"), Join: slices.Contains(args, "--join"), Collect: slices.Contains(args, "--collect"), UnitRead: body,
		CheckClaim: func() error { return nil }, Gate: func(string) (string, error) {
			if d.scenario == "publication changes helm" {
				d.takeHelm(t)
			}
			return "gate-1", nil
		}, NewID: func(string) (string, error) { return "gate-op", nil },
		Delegate: func(_, _, _, _, _ string) (string, error) {
			d.delegates++
			d.writeCritic(t)
			return d.criticID(), nil
		},
		Commit: func(req branch.CommitReadRequest) (string, branch.Attestation, error) {
			req.Inputs = inputs
			req.GateRepository = f
			req.Transport = driverReviewTransport{d}
			return branch.CommitRead(req)
		},
	})
	if err != nil {
		t.Logf("branch read refusal: %v", err)
		return result, 1, err
	}
	return result, 0, nil
}
func (d *driverReviewFixture) criticID() string {
	if d.corrected {
		return "driver-critic-fixed"
	}
	return "driver-critic"
}
func (d *driverReviewFixture) writeCritic(t *testing.T) {
	t.Helper()
	b := &deliveryBed{intentBed: d.bed.intentBed, install: d.bed.worktree}
	b.writeJob(map[string]any{"jobId": d.criticID(), "role": "code-critic", "status": "completed", "round": 1, "parentJob": nil, "engineBuild": "fixture-engine", "effectiveModel": "fixture-critic", "reviews": "commit:" + d.raw.unit, "goalId": d.bed.id, "goalRevision": 1, "findingRegister": []any{}, "findingRegisterRound": 0, "reviewRoundLimit": 6, "criticRoundsConsumed": 0, "mirror": map[string]any{"path": filepath.Join(b.install, "artifacts", "mirror")}})
	b.writeJSON(filepath.Join(b.install, "artifacts", "mirror", "manifest.json"), map[string]any{"files": map[string]any{"jobs/" + d.criticID() + ".json": map[string]any{}}})
	subject, err := d.raw.ReadSubject(d.raw.installation, d.raw.unit)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(b.install, "artifacts", "agents", d.criticID(), "rounds", "1")
	b.writeJSON(filepath.Join(dir, "subject.json"), subject)
	findings := []readsubject.Finding{}
	if !d.corrected && (d.scenario == "judgements" || d.scenario == "stop") {
		f := stopFinding("regression", "unit.go")
		f.ID = "F1"
		findings = append(findings, f)
	}
	b.writeJSON(filepath.Join(dir, "return.json"), map[string]any{"jobId": d.criticID(), "round": 1, "verdict": "LAND", "verdictMaterialCount": len(findings), "reviewedTree": d.raw.tree, "findings": findings})
	verdict := "LAND"
	if len(findings) > 0 {
		verdict = fmt.Sprintf("FIX material=%d", len(findings))
	}
	b.writeFile(filepath.Join(dir, "return.md"), "VERDICT: "+verdict+"\n")
}

const driverGitScript = `import sys,os,json,base64,hashlib
args=sys.argv[1:]
repo=args[1];args=args[2:]
while args[:1]==['-c']:args=args[2:]
p=os.path.join(repo,'.git','driver-git.json')
s=json.load(open(p));refs=s['refs'];out=b'';code=0
key='\0'.join(args)
if key in s['replies']:out=base64.b64decode(s['replies'][key])
elif args==['rev-parse','--git-common-dir']:out=os.path.join(repo,'.git').encode()
elif args[0]=='for-each-ref':
 prefix=args[-1]
 fmt=args[1]
 out='\n'.join((v if fmt=='--format=%(objectname)' else k) for k,v in refs.items() if k.startswith(prefix)).encode()
elif args[0]=='rev-parse':
 ref=args[-1].removesuffix('^{commit}')
 out=refs.get(ref,'').encode();code=0 if out else 1
elif args[0]=='symbolic-ref':out=b'refs/heads/goal/standing-validation'
elif args[0]=='hash-object':
 blob=sys.stdin.read();digest=hashlib.sha1(blob.encode()).hexdigest();s['blobs'][digest]=blob;out=digest.encode()
elif args[0]=='cat-file':out=s['blobs'][refs[args[-1]]].encode()
elif args[0]=='update-ref':
 if args[1]=='-d':refs.pop(args[2],None)
 else:refs[args[1]]=args[2]
elif args[:2]==['diff','--quiet'] or args[:3]==['diff','--cached','--quiet']:pass
elif args[:2]==['merge-base','--is-ancestor']:
 tip=args[3];seen=set()
 while tip!=args[2] and tip in s['parents'] and tip not in seen:
  seen.add(tip);tip=s['parents'][tip]
 code=0 if tip==args[2] else 1
else:
 sys.stderr.write('UNSCRIPTED git '+repr(args));code=1
json.dump(s,open(p,'w'))
sys.stdout.buffer.write(out)
sys.exit(code)
`

func (d *driverReviewFixture) closeCritic(t *testing.T, root string, args []string) intentProcessResult {
	t.Helper()
	job := flagValue(args, "--job")
	if err := dispatchcore.CritiqueChainClose(root, job, false); err != nil {
		return intentProcessResult{code: 1, stderr: []byte(err.Error())}
	}
	return intentProcessResult{}
}

type driverReviewStarter struct {
	d       *driverReviewFixture
	starter *stopReadStarter
}

func (s driverReviewStarter) StartSupervisor(id, state string) (identity.Ref, error) {
	ref, err := s.starter.StartSupervisor(id, state)
	if err != nil {
		return ref, err
	}
	record, err := s.d.bed.manager.Store.Read(id)
	if err == nil && record.State.Terminal() {
		ref = workProcessRef(30)
	}
	if err == nil && record.Kind == "build" && record.Round > 1 && s.d.scenario == "judgements" {
		s.d.dirty = true
	}
	if err == nil && record.Kind == "proof" && record.State == launch.Failed {
		_, err = s.d.bed.manager.Store.Update(id, func(r *launch.Record) error { r.Cause = "own"; return nil })
	}
	if err != nil || record.Kind != "read" {
		return ref, err
	}
	var diff string
	json.Unmarshal(record.AdapterData["readDiff"], &diff)
	bytes, err := os.ReadFile(diff)
	if err != nil {
		return ref, err
	}
	data, err := os.ReadFile(filepath.Join(state, "return.json"))
	if err != nil {
		return ref, err
	}
	subject := readsubject.ReadSubject{Kind: readsubject.SubjectLive, ImplementerRoot: record.Goal, ReviewedProjectTree: fmt.Sprintf("%x", sha256.Sum256(bytes)), DiffDigest: fmt.Sprintf("%x", sha256.Sum256(bytes))}
	read, err := readsubject.Collect(id, subject, "fixture-engine", "fixture-read-model", filepath.Join(state, "return.json"), data, record.Measurement.Verdict)
	if err != nil {
		return ref, err
	}
	_, err = s.d.bed.manager.Store.Update(id, func(r *launch.Record) error {
		if read.Material == 0 {
			r.Measurement.Verdict = "VERDICT: land"
			if err := os.WriteFile(filepath.Join(state, "report.md"), []byte("VERDICT: land\n"), 0600); err != nil {
				return err
			}
		}
		r.Read = &read
		r.Outputs = []launch.Output{{Path: filepath.Join(state, "return.json")}, {Path: filepath.Join(state, "report.md")}}
		delete(r.AdapterData, "unitStopInputs")
		return nil
	})
	return ref, err
}

// Each registered checkout reports its own Git top, so custody remains per worktree.
type driverHeldTreeGit struct {
	clean launch.GitRunner
	held  *workBed
}

func (g driverHeldTreeGit) Run(directory string, environment []string, args ...string) ([]byte, error) {
	actual, _ := filepath.EvalSymlinks(directory)
	held, _ := filepath.EvalSymlinks(g.held.worktree)
	if actual == held {
		return (workGit{g.held}).Run(directory, environment, args...)
	}
	return g.clean.Run(directory, environment, args...)
}
