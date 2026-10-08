package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/census"
	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/readsubject"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot/stateroottest"
)

// connectionBed is a physical Git repository (the work bed's goal ledger
// checkout) with a bare origin. The public commands run with the real unit
// runner Git, branch commit, branch push, branch read and read collection
// owners. Only the goal claim, commit token, model launches, fast gate and
// critic dispatch are per-test fakes, and every write stays in the test's
// temporary directories.
type connectionBed struct {
	*workBed
	t                *testing.T
	origin, worktree string
	mu               sync.Mutex
	edits            map[string]string
	proofWrites      bool
	readFails        bool
	followUps        []string
	delegates        []string
	failPushes       int
	loseCommit       bool
	commits          int
	tokens           int
	closes           [][]string
	failCloses       int
	claimLost        bool
	// isolate, when set, wraps the real adapter-configuration isolation.
	isolate      func(real func(string, string) error, source, destination string) error
	loseReadSave bool
	commitReads  int
	publications int
	reads        [][]string
	// dispatcher, when set, replaces the fake critic dispatch; refusal is
	// what it reports while a cause of refusal is in place, and briefs the
	// briefs of the critics it started.
	dispatcher func(c *connectionBed, install string) func(string, string, string, string, string) (string, error)
	refusal    *delegateOutcome
	briefs     []string
	// skipRead stops a review before the read owner is reached.
	skipRead bool
	// fromPrimary dispatches the critic from the seat's checkout.
	fromPrimary bool
}

func connectionGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", dir}, args...)...)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	output, err := command.Output()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, stderr.String())
	}
	return strings.TrimSpace(string(output))
}

func newConnectionBed(t *testing.T) *connectionBed {
	t.Helper()
	return newConnectionBedWith(t, workApprovedBox)
}

// newConnectionBedWith is the connection bed with its goal record shaped by
// amend.
func newConnectionBedWith(t *testing.T, amend func(*goal.GoalFile)) *connectionBed {
	t.Helper()
	empty := filepath.Join(t.TempDir(), "gitconfig")
	os.WriteFile(empty, nil, 0o600)
	t.Setenv("GIT_CONFIG_GLOBAL", empty)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	c := &connectionBed{workBed: newWorkBedWith(t, amend), t: t, edits: map[string]string{}}
	root := c.root()
	connectionGit(t, root, "init", "-q", "-b", "main")
	connectionGit(t, root, "config", "user.name", "Fixture")
	connectionGit(t, root, "config", "user.email", "fixture@example.invalid")
	os.WriteFile(filepath.Join(root, ".gitignore"), []byte("artifacts/\n.claude/settings.local.json\nmetasystem.conf.local\n"), 0o600)
	conf := filepath.Join(root, "metasystem.conf")
	declarations, err := os.ReadFile(conf)
	if err != nil {
		t.Fatal(err)
	}
	declarations = append(declarations, []byte("\nproof.cheap=true\nproof.audits=true\nproof.deadline=15\n")...)
	if err := os.WriteFile(conf, declarations, 0600); err != nil {
		t.Fatal(err)
	}
	connectionGit(t, root, "add", "-A")
	connectionGit(t, root, "commit", "-q", "-m", "fixture base")
	c.origin = filepath.Join(t.TempDir(), "origin.git")
	connectionGit(t, filepath.Dir(c.origin), "init", "-q", "--bare", "-b", "main", c.origin)
	connectionGit(t, root, "remote", "add", "origin", c.origin)
	connectionGit(t, root, "push", "-q", "origin", "main")
	resolved, _ := filepath.EvalSymlinks(filepath.Dir(root))
	c.worktree = filepath.Join(resolved, filepath.Base(root)+"-"+c.id)
	c.manager.Supervisor = c
	return c
}

// StartSupervisor is the fake model and proof process: a build writes the
// test's edits into the worktree it runs in; a proof may write too.
func (c *connectionBed) StartSupervisor(id, state string) (identity.Ref, error) {
	record, _ := c.manager.Store.Read(id)
	c.mu.Lock()
	switch record.Kind {
	case "build":
		for path, content := range c.edits {
			os.MkdirAll(filepath.Dir(filepath.Join(record.WorkingDirectory, path)), 0o700)
			os.WriteFile(filepath.Join(record.WorkingDirectory, path), []byte(content), 0o644)
		}
	case "proof":
		if c.proofWrites {
			os.WriteFile(filepath.Join(record.WorkingDirectory, "proof-output.txt"), []byte("written by the proof\n"), 0o644)
		}
	}
	c.mu.Unlock()
	c.manager.Store.Update(id, func(current *launch.Record) error {
		code := 0
		current.State, current.ExitCode, current.FinishedAt = launch.Completed, &code, c.manager.Now().UTC().Format(time.RFC3339Nano)
		if record.Kind == "read" && c.readFails {
			failed := 1
			current.State, current.Reason, current.ExitCode = launch.Failed, "fixture-read-failed", &failed
		} else if record.Kind == "read" {
			yes := true
			// Successful fixture reads supply the structured evidence required by collection.
			current.VerdictCounts, current.Measurement.Verdict = &yes, "LAND"
			if err := os.MkdirAll(state, 0700); err != nil {
				return err
			}
			structured, report := filepath.Join(state, "return.json"), filepath.Join(state, "report.md")
			if err := os.WriteFile(structured, []byte(`{"findings":[],"verdictMaterialCount":0}`), 0600); err != nil {
				return err
			}
			if err := os.WriteFile(report, []byte("VERDICT: LAND\n"), 0600); err != nil {
				return err
			}
			current.Outputs = append(current.Outputs, launch.Output{Path: structured}, launch.Output{Path: report})
		}
		return nil
	})
	return workProcessRef(99), nil
}

type connectionTransport struct{ c *connectionBed }

func (x connectionTransport) RemoteTip(repo, remote, ref string) (string, bool, error) {
	return branch.GitPushTransport{}.RemoteTip(repo, remote, ref)
}
func (x connectionTransport) Fetch(repo, remote, ref, destination string) error {
	return branch.GitPushTransport{}.Fetch(repo, remote, ref, destination)
}
func (x connectionTransport) Push(repo, remote, ref, expected, tip string) (branch.CASOutcome, error) {
	x.c.mu.Lock()
	fail := x.c.failPushes > 0
	if fail {
		x.c.failPushes--
	}
	x.c.mu.Unlock()
	if fail {
		return branch.CASUnknown, errors.New("fixture transport: connection reset before the push")
	}
	return branch.GitPushTransport{}.Push(repo, remote, ref, expected, tip)
}

func (c *connectionBed) endpoint() goal.Endpoint {
	return goal.Endpoint{Root: c.root(), Remote: "origin", Branch: "refs/heads/main"}
}

func (c *connectionBed) endpointTip() string {
	tip, err := goalBranchEndpointTipWithGit(c.root(), c.endpoint(), goalBranchGit)
	if err != nil {
		c.t.Fatal(err)
	}
	return tip
}

func (c *connectionBed) connectionOwners() intentOwners {
	owners := c.workOwners()
	owners.work.criticDeath = dispatchcore.CustodyDeathDependencies{
		Reader: &criticCustodyReader{dead: true}, Processes: identity.FixedProcessTable{},
		MatchesTag: func([]string, string) bool { return true },
		TaggedScan: func(string) census.TaggedProcessCensus { return census.TaggedProcessCensus{} },
	}
	root, worktree := c.root(), c.worktree
	// The linked worktree shares the primary's accepted ledger and clock.
	// Read commands may select either installation while their records stay local.
	endpoint, commandNow := owners.dependencies.endpoint, owners.commandNow
	owners.dependencies.endpoint = func(installation string) (goal.Endpoint, error) {
		if sameCanonicalPath(installation, worktree) {
			return endpoint(root)
		}
		return endpoint(installation)
	}
	owners.commandNow = func(stateRoot string) (time.Time, error) {
		if sameCanonicalPath(stateRoot, worktree) {
			return commandNow(root)
		}
		return commandNow(stateRoot)
	}
	owners.resolver = stateroot.NewResolver(func(path string) (string, error) {
		if withinPath(path, worktree) {
			return worktree, nil
		}
		return fakeTop(root)(path)
	}, noExecutable)
	owners.work.git = nil
	owners.work.units = func(stateroot.Layout) *launch.UnitRunner {
		return &launch.UnitRunner{Manager: c.manager, Git: launch.OSGitRunner{}, Root: c.unitRoot}
	}
	owners.connection = intentConnectionOwners{
		endpoint: func(string) (goal.Endpoint, error) { return c.endpoint(), nil },
		claimCheck: func(string, string, goal.Endpoint) func() error {
			return func() error {
				c.mu.Lock()
				defer c.mu.Unlock()
				if c.claimLost {
					return errors.New("goal standing-validation is not claimed by this session")
				}
				return nil
			}
		},
		commitToken: func(_ string, commit func() error) error {
			c.mu.Lock()
			c.tokens++
			c.mu.Unlock()
			return commit()
		},
		transport:  connectionTransport{c},
		rebaseGate: func(string) (string, error) { return "fixture-static-green", nil },
		commit: func(request branch.CommitRequest) (string, error) {
			commit, err := branch.CommitStaged(request)
			c.mu.Lock()
			defer c.mu.Unlock()
			if err == nil {
				c.commits++
			}
			if err == nil && c.loseCommit {
				c.loseCommit = false
				return "", errors.New("fixture: the commit's response was lost")
			}
			return commit, err
		},
	}
	if c.isolate != nil {
		hook, resolver := c.isolate, owners.resolver
		owners.connection.isolate = func(source, destination string) error {
			layout, err := resolver.ResolveLayout(root)
			if err != nil {
				return err
			}
			real := (&intentInvocation{owners: intentOwners{work: owners.work}, layout: layout}).isolateAdapterConfiguration
			return hook(real, source, destination)
		}
	}
	owners.delivery = &intentDeliveryOwners{
		branchState: func(string, string) (intentBranchState, error) {
			return intentBranchState{Status: c.landAdmission(), ReadsWaived: goal.ReadsWaived(c.goalFile(c.id))}, nil
		},
		// The bed's close owner runs as a person's act (its engine wrapper
		// classifies HUMAN); the record-writer authority owner judges that
		// same classification.
		recordWriter: humanRecordWriter,
		process:      c.closeOwner,
		closeOwner: func(root string, args []string) intentProcessResult {
			return c.closeOwner(intentProcess{argv: append([]string{"close-owner"}, args...), dir: root})
		},
		publishRead: func(root, goalID, unit string) (branch.PublishReadResult, error) {
			c.publications++
			return branch.PublishCollectedRead(branch.PublishReadRequest{Repo: root, Remote: "origin", EndpointTip: c.endpointTip(),
				GoalID: goalID, UnitCommit: unit, CheckClaim: func() error { return nil }, Transport: connectionTransport{c}})
		},
		branchRead: func(args []string) (branch.BranchReadResult, int, error) {
			c.reads = append(c.reads, args)
			if c.skipRead {
				return branch.BranchReadResult{}, 1, &branch.ReadNeverLaunchedError{Err: errors.New("fixture: the read owner is not reached")}
			}
			install, goalID := flagValue(args, "--root"), flagValue(args, "--goal")
			delegate := c.criticDispatch(install)
			if c.dispatcher != nil {
				delegate = c.dispatcher(c, install)
			}
			retry, _ := strconv.ParseInt(flagValue(args, "--retry"), 10, 64)
			tip, present, err := branch.GitPushTransport{}.RemoteTip(install, "origin", "refs/heads/goal/"+goalID)
			if err != nil || !present {
				return branch.BranchReadResult{}, 1, fmt.Errorf("origin has no goal/%s: %v", goalID, err)
			}
			result, err := branch.RunBranchRead(branch.BranchReadRequest{Repo: install, Remote: "origin",
				EndpointTip: c.endpointTip(), BranchTip: tip, GoalID: goalID, UnitCommit: flagValue(args, "--unit"),
				Collect: slices.Contains(args, "--collect"), Join: slices.Contains(args, "--join"), BriefPath: flagValue(args, "--brief"), Selected: flagValue(args, "--selected-installation"),
				BuildBriefSHA256: flagValue(args, "--build-brief-sha256"),
				CheckClaim:       func() error { return nil },
				Gate:             func(string) (string, error) { return "gate-run-1", nil },
				Delegate:         delegate,
				Retry:            retry,
				// The follow-up transport is the fixture's: it records the next
				// round of the same chain, as dispatch's follow-up would.
				FollowUp: func(rootJob, brief string) (string, error) {
					c.mu.Lock()
					c.followUps = append(c.followUps, rootJob+" "+brief)
					round := len(c.followUps) + 1
					c.mu.Unlock()
					child := fmt.Sprintf("%s-r%d", rootJob, round)
					c.writeJSON(filepath.Join(install, "artifacts", "agents", "jobs", child+".json"), map[string]any{
						"jobId": child, "role": "code-critic", "status": "running", "round": round, "parentJob": rootJob, "goalId": goalID})
					return child, nil
				},
				Commit: func(request branch.CommitReadRequest) (string, branch.Attestation, error) {
					c.commitReads++
					commit, attestation, err := branch.CommitRead(request)
					if err == nil && c.loseReadSave {
						c.loseReadSave = false
						return "", attestation, errors.New("fixture: interrupted before the read record was saved")
					}
					return commit, attestation, err
				}})
			if err != nil {
				return result, 1, err
			}
			return result, 0, nil
		},
	}
	return owners
}

// criticDispatch is the fake model dispatch of the committed critic: it
// records a running code-critic root for the commit, as dispatch would.
func (c *connectionBed) criticDispatch(install string) func(string, string, string, string, string) (string, error) {
	return func(_, goalID, commit, _, _ string) (string, error) {
		c.mu.Lock()
		job := fmt.Sprintf("crit%d", len(c.delegates)+1)
		c.delegates = append(c.delegates, commit)
		c.mu.Unlock()
		c.writeCritic(install, job, commit, "running", false)
		return job, nil
	}
}

func (c *connectionBed) writeCritic(install, job, commit, status string, closed bool) {
	c.t.Helper()
	record := map[string]any{"jobId": job, "role": "code-critic", "round": 1, "status": status,
		"engineBuild": "fixture-engine", "effectiveModel": "fixture-critic",
		"reviews": "commit:" + commit, "goalId": c.id, "goalRevision": 1, "findingRegister": []any{},
		// A dispatched critic root carries its register round and round limit.
		"findingRegisterRound": 0, "reviewRoundLimit": 3, "criticRoundsConsumed": 0}
	record["operationId"], record["capMin"] = "fixture-critic:"+job, 1
	record["instanceTag"], record["pid"], record["pgid"] = "fixture-critic-"+job, int64(20), int64(20)
	record["pidStartedAt"] = int64(400)
	record["startedAt"], record["endedAt"] = c.manager.Now().UTC().Format(time.RFC3339Nano), c.manager.Now().UTC().Format(time.RFC3339Nano)
	if runtime.GOOS == "darwin" {
		record["pidStartedAtExactMicro"] = int64(400_000_001)
	} else {
		record["pidStartTicks"], record["bootId"] = int64(400), "fixture-boot"
	}
	if closed || status == "completed" {
		subject, present, err := dispatchcore.ComputeReadSubject(dispatchcore.ReadSubjectRequest{RepoRoot: install, Role: "code-critic", Reviews: "commit:" + commit})
		if err != nil || !present {
			c.t.Fatalf("critic subject present=%v err=%v", present, err)
		}
		if closed {
			record["chainClosed"], record["findingRegisterRound"], record["findingRegisterSubjectDigest"] = true, 1, subject.Digest()
			record["closure"] = map[string]any{"criticRoot": job, "round": 1, "subject": subject, "mechanism": "clean"}
		}
		dir := filepath.Join(install, "artifacts", "agents", job, "rounds", "1")
		c.writeJSON(filepath.Join(dir, "subject.json"), subject)
		// Completed examinations supply structured stop evidence and matching prose.
		finding := stopFinding("regression", "connect.txt")
		finding.ID, finding.Material = "F1", false
		c.writeJSON(filepath.Join(dir, "return.json"), map[string]any{"jobId": job, "round": 1, "findings": []any{finding}, "verdict": "1 finding", "verdictMaterialCount": 0, "reviewedTree": subject.Tree})
		if err := os.WriteFile(filepath.Join(dir, "return.md"), []byte("VERDICT: LAND\n"), 0600); err != nil {
			c.t.Fatal(err)
		}
	}
	c.writeJSON(filepath.Join(install, "artifacts", "agents", "jobs", job+".json"), record)
}

func (c *connectionBed) writeJSON(path string, value any) {
	c.t.Helper()
	data, _ := json.MarshalIndent(value, "", "  ")
	os.MkdirAll(filepath.Dir(path), 0o700)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		c.t.Fatal(err)
	}
}

// closeOwner is the whole close owner (the delegate lifecycle's close): it stamps the
// chain's closure after the public close joined the dispositions.
func (c *connectionBed) closeOwner(process intentProcess) intentProcessResult {
	c.closes = append(c.closes, process.argv)
	job := flagValue(process.argv, "--job")
	if c.failCloses > 0 {
		c.failCloses--
		return intentProcessResult{code: 1, stderr: []byte("close check: the evidence mirror is missing\n")}
	}
	if process.argv[0] != "close-owner" || process.argv[1] != "close" || slices.Contains(process.argv, "--runner-closed") {
		c.t.Fatalf("close must invoke the whole owner: %v", process.argv)
	}
	var record map[string]any
	data, _ := os.ReadFile(filepath.Join(process.dir, "artifacts", "agents", "jobs", job+".json"))
	json.Unmarshal(data, &record)
	c.writeCritic(process.dir, job, strings.TrimPrefix(record["reviews"].(string), "commit:"), "completed", true)
	return intentProcessResult{}
}

func (c *connectionBed) do(args ...string) (int, intentResult) {
	c.t.Helper()
	command, rest, ok := resolveIntentArgv(args)
	if !ok {
		c.t.Fatalf("no public command %q", args)
	}
	var stdout, stderr bytes.Buffer
	code := runIntentIn(command, append([]string{"--json"}, rest...), &stdout, &stderr, c.root(), c.connectionOwners())
	var result intentResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		c.t.Fatalf("%v printed no JSON result: %v; stdout=%q stderr=%q", args, err, stdout.String(), stderr.String())
	}
	return code, result
}

func (c *connectionBed) unitCommits(ref string) []string {
	c.t.Helper()
	out := connectionGit(c.t, c.root(), "log", "--format=%H %s", "main.."+ref)
	var commits []string
	for _, line := range strings.Split(out, "\n") {
		if line != "" {
			commits = append(commits, line)
		}
	}
	return commits
}

func (c *connectionBed) runRecord(run string) launch.UnitRunRecord {
	c.t.Helper()
	var record launch.UnitRunRecord
	data, err := os.ReadFile(filepath.Join(c.unitRoot, run, "run.json"))
	if err != nil || json.Unmarshal(data, &record) != nil {
		c.t.Fatalf("run %s unreadable: %v", run, err)
	}
	return record
}

func (c *connectionBed) landAdmission() branch.Status {
	c.t.Helper()
	origin := connectionGit(c.t, c.root(), "ls-remote", c.origin, "refs/heads/goal/"+c.id)
	status, err := branch.InspectStatus(c.root(), c.endpointTip(), strings.Fields(origin)[0], c.id)
	if err != nil {
		c.t.Fatalf("landing admission cannot read the published branch: %v", err)
	}
	return status
}

func (c *connectionBed) dispositions() string {
	path := filepath.Join(c.root(), "dispositions.md")
	os.WriteFile(path, []byte(deliveryDispositionsHeader+"| F1 | noted | style only | none |\n"), 0o600)
	return path
}

// adapterFixture puts a synthetic declared local settings file (the Claude
// runtime declares it in the runtime registry) (plus a synthetic local
// configuration that must never be copied) into the selected checkout.
func (c *connectionBed) adapterFixture() string {
	c.t.Helper()
	settings := `{"permissions":{"allow":["Bash(go test:*)"]},"fixture":"selected-installation"}`
	os.MkdirAll(filepath.Join(c.root(), ".claude"), 0o700)
	os.WriteFile(filepath.Join(c.root(), ".claude", "settings.local.json"), []byte(settings), 0o600)
	os.WriteFile(filepath.Join(c.root(), "metasystem.conf.local"), []byte("fixture.secret = never-copied\n"), 0o600)
	return settings
}

// TestIntentBuiltUnitToLanding (VMI-10, VMI-CONN-01/03/04/06) drives the
// public build, review unit, close and fold commands through the named
// physical-Git adapter journey: its claims are Git-specific (staging exact
// path bytes, commit, amend and replay, push, read collection), so the unit
// runner Git, branch commit, push, read, collection, publication and range
// owners are the real ones on a repository with a bare origin. The claim,
// commit token, model launches, fast gate and critic dispatch are per-test
// fakes. The whole close owner is a FAKE here: it stamps a recorded closure
// the way the lifecycle's close would; the real close/register fixture and the
// real public land are delivery's (TestIntentCloseWholeOwner and its land
// fixtures) and are not claimed by this test. Landing is observed through
// the landing admission reader on the published branch only.
func TestIntentBuiltUnitToLanding(t *testing.T) {
	c := newConnectionBedWith(t, func(file *goal.GoalFile) {
		workApprovedBox(file)
		// This journey builds four units and corrects one of them.
		file.Budget.AttemptLimit = 5
		file.Approved.Digest = goal.ApprovalDigest(file.Intent, file.Tier, *file.Budget, file.Risk)
	})
	settings := c.adapterFixture()
	base := c.endpointTip()
	brief := c.brief("brief.md", "Build the connection.\n")
	c.edits = map[string]string{"connect.txt": "the built result\n", "café.txt": "accented\n", "tab\tname.txt": "tab\n",
		"quote\"d name.txt": "quoted\n", "dir with space/space name.txt": "spaced\n"}
	code, result := c.do(append([]string{"work", "build", c.id, "connect", "--last", "--brief", brief, "--lines", "10"}, designGateCheck...)...)
	if code != 0 || result.Outcome != intentConfirmed {
		t.Fatalf("build: code=%d %+v", code, result)
	}
	run := resultData(t, result)["run"].(string)
	if result.Next == nil || !slices.Equal(result.Next.Argv, []string{"metasystem", "work", "review", c.id, "--work", "connect"}) || run == "" {
		t.Fatalf("a green build's next act is its committed review: %+v", result.Next)
	}
	if head := connectionGit(t, c.worktree, "rev-parse", "HEAD"); head != base ||
		connectionGit(t, c.worktree, "symbolic-ref", "--short", "HEAD") != "goal/"+c.id {
		t.Fatalf("the first build must create goal/%s at the endpoint tip in %s (HEAD %s)", c.id, c.worktree, head)
	}
	// The adapter's declared local settings reach the generated worktree
	// through the session-isolation owner; the local configuration does not.
	if data, err := os.ReadFile(filepath.Join(c.worktree, ".claude", "settings.local.json")); err != nil || string(data) != settings {
		t.Fatalf("declared adapter settings did not reach the worktree: %q %v", data, err)
	}
	if _, err := os.Stat(filepath.Join(c.worktree, "metasystem.conf.local")); !os.IsNotExist(err) {
		t.Fatal("metasystem.conf.local must never be copied into a goal worktree")
	}
	install := c.worktree

	// A later edit is never absorbed: refused before staging or commit.
	late := filepath.Join(c.worktree, "late.txt")
	os.WriteFile(late, []byte("typed after the build\n"), 0o644)
	code, result = c.do("work", "review", "run:"+run)
	if result.Outcome != intentRefused || !strings.Contains(strings.Join(result.Details, " "), "UNIT_RESULT_CHANGED") ||
		connectionGit(t, c.worktree, "diff", "--cached", "--name-only") != "" || c.commits != 0 {
		t.Fatalf("stale result: code=%d %+v", code, result)
	}
	os.Remove(late)
	// The same path with other bytes is refused too.
	os.WriteFile(filepath.Join(c.worktree, "café.txt"), []byte("accentuated\n"), 0o644)
	if _, result = c.do("work", "review", "run:"+run); !strings.Contains(strings.Join(result.Details, " "), "UNIT_RESULT_CHANGED") || c.commits != 0 {
		t.Fatalf("changed bytes at a result path: %+v", result)
	}
	os.WriteFile(filepath.Join(c.worktree, "café.txt"), []byte("accented\n"), 0o644)

	// The commit is made but its response is lost, then publication fails:
	// exactly one unit commit, reconciled and reported partial.
	c.loseCommit, c.failPushes = true, 1
	code, result = c.do("work", "review", "run:"+run)
	if result.Outcome != intentPartial || result.Next == nil ||
		!slices.Equal(slices.DeleteFunc(slices.Clone(result.Next.Argv), func(word string) bool { return word == "--json" }), []string{"metasystem", "work", "review", "run:" + run}) {
		t.Fatalf("lost commit then failed push: code=%d %+v", code, result)
	}
	subjects := c.runRecord(run).Subjects
	if c.commits != 1 || len(subjects) != 1 || subjects[0].Commit == "" || subjects[0].Published != "" ||
		subjects[0].ExpectedParent != base || len(c.unitCommits("goal/"+c.id)) != 1 || len(subjects[0].Paths) != 5 {
		t.Fatalf("one retained, unpublished unit commit of the five exact paths: commits=%d %+v", c.commits, subjects)
	}
	// A read-clean last unit leads to hand-in rather than another build.
	first := subjects[0].Commit
	// The build's last-unit declaration survives a later review and a lost commit response.
	if message := connectionGit(t, c.worktree, "show", "-s", "--format=%B", first); !strings.Contains(message, "Goal-Whole: "+c.id) {
		t.Fatalf("last build's commit lost its goal end: %s", message)
	}
	for _, path := range []string{"café.txt", "tab\tname.txt", "quote\"d name.txt", "dir with space/space name.txt"} {
		if out := connectionGit(t, c.root(), "cat-file", "-p", first+":"+path); out == "" {
			t.Fatalf("%q is not in the unit commit", path)
		}
	}
	code, result = c.do("work", "review", "run:"+run)
	if result.Outcome != intentInProgress || len(c.delegates) != 1 || c.delegates[0] != first || c.commits != 1 {
		t.Fatalf("retry publishes the same commit and requests one critic: code=%d %+v delegates=%v", code, result, c.delegates)
	}
	if last := c.reads[len(c.reads)-1]; flagValue(last, "--selected-installation") == "" || slices.Contains(last, "--runtime") || slices.Contains(last, "--model") {
		t.Fatalf("the worktree read must carry the selected installation, never a critic override: %v", last)
	}
	if remote := connectionGit(t, c.root(), "ls-remote", c.origin, "refs/heads/goal/"+c.id); !strings.HasPrefix(remote, first) {
		t.Fatalf("the unit commit is published: %q", remote)
	}
	_, result = c.do("work", "review", "run:"+run)
	if result.Outcome != intentInProgress || len(c.delegates) != 1 {
		t.Fatalf("a running critic is waited on, never dispatched again: %+v", result)
	}

	// Finished but unclosed: the author's close is named, nothing collected.
	c.writeCritic(install, "crit1", first, "completed", false)
	_, result = c.do("work", "review", "run:"+run)
	if result.Outcome != intentInProgress || result.Next == nil || !strings.Contains(shellCommand(result.Next.Argv), "work review run:"+run+" --dispositions "+resultData(t, result)["template"].(string)) ||
		len(c.unitCommits("goal/"+c.id)) != 1 || c.commitReads != 0 {
		t.Fatalf("unclosed critic: %+v next=%+v", result, result.Next)
	}
	code, result = c.do("work", "finish", "j2:crit1", "--dispositions", c.dispositions(), "--repo", install)
	if code != 0 || result.Outcome != intentConfirmed || len(c.closes) != 1 {
		t.Fatalf("public close (fake whole owner): code=%d %+v", code, result)
	}

	// Collection installs the Goal-Read but its record save is lost; the
	// retry adopts that exact read, publication then fails once, and the
	// next retry publishes: one attestation, one critic, one collection.
	c.loseReadSave = true
	code, result = c.do("work", "review", "run:"+run)
	if result.Outcome == intentConfirmed || c.commitReads != 1 {
		t.Fatalf("interrupted collection: code=%d %+v reads=%d", code, result, c.commitReads)
	}
	c.failPushes = 1
	code, result = c.do("work", "review", "run:"+run)
	if result.Outcome != intentPartial || c.commitReads != 1 {
		t.Fatalf("adopted read, failed publication: code=%d %+v reads=%d", code, result, c.commitReads)
	}
	attestation := resultData(t, result)["attestation"].(string)
	code, result = c.do("work", "review", "run:"+run)
	if code != 0 || (result.Outcome != intentConfirmed && result.Outcome != intentUnchanged) || resultData(t, result)["attestation"] != attestation ||
		result.Next == nil || !slices.Equal(result.Next.Argv, []string{"metasystem", "work", "land", c.id}) || len(c.delegates) != 1 || c.commitReads != 1 {
		t.Fatalf("published read: code=%d %+v", code, result)
	}
	if commits := c.unitCommits("goal/" + c.id); len(commits) != 2 {
		t.Fatalf("one unit and exactly one Goal-Read: %v", commits)
	}
	status := c.landAdmission()
	if status.Prefix != 1 || len(status.Units) != 1 || status.Units[0].Commit != first {
		t.Fatalf("landing admission must see the one read unit: %+v", status)
	}
	if stop := c.runRecord(run).Rounds[0].Stop; stop == nil || stop.Decision != "close" {
		t.Fatalf("the published clean read must close its unit: %+v", stop)
	}

	if finished := c.runRecord(run); finished.Rounds[0].Stop == nil || finished.Rounds[0].Stop.Decision != "close" {
		t.Fatalf("published round must release its tree: round=%+v subjects=%+v", finished.Rounds[0], finished.Subjects)
	}
	// A later unit survives on the branch after the first unit's read.
	c.edits = map[string]string{"later.txt": "a later unit\n"}
	_, result = c.do(append([]string{"work", "build", c.id, "later", "--brief", c.brief("later.md", "A later unit.\n"), "--lines", "5"}, designGateCheck...)...)
	later := resultData(t, result)["run"].(string)
	if code, result = c.do("work", "review", "run:"+later); result.Outcome != intentInProgress || len(c.delegates) != 2 {
		t.Fatalf("later unit: code=%d %+v", code, result)
	}
	c.writeCritic(install, "crit2", c.runRecord(later).Subjects[0].Commit, "cancelled", false)
	if _, err := c.connectionOwners().work.units(stateroot.Layout{}).CancelRun(later); err != nil {
		t.Fatal(err)
	}

	c.writeCritic(install, "crit2", c.delegates[1], "cancelled", false)
	if _, err := (&launch.UnitRunner{Root: c.unitRoot, Manager: c.manager, Git: launch.OSGitRunner{}}).CancelRun(later); err != nil {
		t.Fatal(err)
	}
	// The clean read ends automatic corrections; a person requests this amend.
	// Same-unit follow-up: the finding is folded and the unit's commit is
	// amended with a lost commit response. The replacement unit (not the
	// replayed tip) is the subject, its old read is dropped, the later unit
	// is replayed, and a new critic reads the replacement.
	c.edits = map[string]string{"connect.txt": "the built result, fixed\n"}
	followUp := c.brief("follow-up.md", "Fix F1.\n")
	code, result = c.do("work", "revise", "run:"+run, "--brief", followUp, "--reason", "Amend the already reviewed unit", "--by", "Wido")
	if code != 0 || result.Outcome != intentConfirmed || result.Next == nil || result.Next.Argv[2] != "review" {
		t.Fatalf("fold unit: code=%d %+v", code, result)
	}
	c.loseCommit = true
	code, result = c.do("work", "review", "run:"+run)
	if result.Outcome != intentInProgress || len(c.delegates) != 3 {
		t.Fatalf("amended subject needs its own critic: code=%d %+v delegates=%v", code, result, c.delegates)
	}
	subjects = c.runRecord(run).Subjects
	if len(subjects) != 2 || subjects[0].Commit != first || subjects[1].Amends != first || subjects[1].Commit == first ||
		subjects[1].Commit == subjects[1].Tip || c.delegates[2] != subjects[1].Commit {
		t.Fatalf("round 2's subject is the replacement unit, separate from the replayed tip: %+v delegates=%v", subjects, c.delegates)
	}
	second := subjects[1].Commit
	if commits := c.unitCommits("goal/" + c.id); len(commits) != 2 || !strings.HasPrefix(commits[1], second) || !strings.Contains(commits[0], "later") {
		t.Fatalf("the amended branch is the replacement then the replayed later unit, without the dropped read: %v", commits)
	}
	c.writeCritic(install, "crit3", second, "completed", false)
	if _, pending := c.do("work", "review", "run:"+run); pending.Outcome != intentInProgress {
		t.Fatalf("the work must retain its examination before closure: %+v", pending)
	}
	if code, result = c.do("work", "finish", "j2:crit3", "--dispositions", c.dispositions(), "--repo", install); code != 0 {
		t.Fatalf("close crit3: %+v", result)
	}
	code, result = c.do("work", "review", "run:"+run)
	if code != 0 || result.Outcome != intentConfirmed {
		t.Fatalf("amended unit read: code=%d %+v", code, result)
	}
	status = c.landAdmission()
	if status.Prefix != 1 || status.Units[0].Commit != second {
		t.Fatalf("landing admission must see exactly the current subject first: %+v", status)
	}
	if c.tokens != 4 || c.commits != 3 {
		t.Fatalf("each subject and the amended review carry use the commit token: tokens=%d commits=%d", c.tokens, c.commits)
	}

	// A failed preliminary read with a passed proof may still request the
	// committed review.
	c.edits, c.readFails = map[string]string{"readfail.txt": "read failed, proof passed\n"}, true
	_, result = c.do(append([]string{"work", "build", c.id, "readfail", "--brief", c.brief("readfail.md", "Read each round: yes\nRead-failed unit.\n"), "--lines", "5"}, designGateCheck...)...)
	readFailed := resultData(t, result)["run"].(string)
	if round := c.runRecord(readFailed).Rounds[0]; round.Outcome != "read-failed" || result.Next == nil || result.Next.Argv[2] != "review" {
		t.Fatalf("read-failed build: %s %+v", round.Outcome, result)
	}
	c.readFails = false
	if code, result = c.do("work", "review", "run:"+readFailed); result.Outcome != intentInProgress || len(c.delegates) != 4 {
		t.Fatalf("read-failed round requests committed review: code=%d %+v", code, result)
	}
	c.writeCritic(install, "crit4", c.runRecord(readFailed).Subjects[0].Commit, "cancelled", false)
	if _, err := c.connectionOwners().work.units(stateroot.Layout{}).CancelRun(readFailed); err != nil {
		t.Fatal(err)
	}

	c.writeCritic(install, "crit4", c.delegates[3], "cancelled", false)
	if _, err := (&launch.UnitRunner{Root: c.unitRoot, Manager: c.manager, Git: launch.OSGitRunner{}}).CancelRun(readFailed); err != nil {
		t.Fatal(err)
	}
	// A proof that writes the worktree refuses committed review before
	// staging.
	c.edits, c.proofWrites = map[string]string{"other.txt": "another unit\n"}, true
	code, result = c.do(append([]string{"work", "build", c.id, "wrote", "--brief", c.brief("wrote.md", "Another unit.\n"), "--lines", "5"}, workCheck...)...)
	if code != 1 || result.Outcome != intentRefused || resultData(t, result)["outcome"] != "proof-wrote" || !strings.Contains(result.Summary, "stopped environment") {
		t.Fatalf("proof-writing build did not hold its result: code=%d %+v", code, result)
	}
	wrote := resultData(t, result)["run"].(string)
	if c.runRecord(wrote).State == "running" {
		t.Fatalf("proof-writing build: %+v run=%+v", result, c.runRecord(wrote))
	}
	code, result = c.do("work", "review", "run:"+wrote)
	wroteRound := c.runRecord(wrote).Rounds[0]
	if code != 1 || result.Outcome != intentRefused || wroteRound.Cause != "environment" || wroteRound.Outcome != "proof-wrote" || wroteRound.Stop == nil || wroteRound.Stop.Decision != "stop" || len(wroteRound.Steps) != 2 || len(wroteRound.Steps[1].LaunchIDs) != 2 || len(wroteRound.Reads) != 0 || connectionGit(t, c.worktree, "diff", "--cached", "--name-only") != "" {
		t.Fatalf("proof-wrote: code=%d result=%+v round=%+v", code, result, wroteRound)
	}
	c.proofWrites = false

	// Without the claim, no build or read of any run is launched: not a new
	// run in the existing worktree, not a follow-up.
	c.claimLost = true
	launches := len(c.starts())
	code, result = c.do(append([]string{"work", "build", c.id, "unclaimed", "--brief", c.brief("unclaimed.md", "No claim.\n"), "--lines", "5"}, designGateCheck...)...)
	// The claim is checked before any run is reserved.
	if result.Outcome == intentConfirmed || !strings.Contains(result.Summary, "is not claimed by this session") || len(c.starts()) != launches {
		t.Fatalf("unclaimed build: code=%d %+v", code, result)
	}
	code, result = c.do("work", "revise", "run:"+readFailed, "--brief", c.brief("unclaimed-fold.md", "No claim.\n"))
	if result.Outcome == intentConfirmed || len(c.starts()) != launches {
		t.Fatalf("unclaimed fold launched: code=%d %+v launches=%d", code, result, len(c.starts())-launches)
	}
	if branch := connectionGit(t, c.root(), "symbolic-ref", "--short", "HEAD"); branch != "main" {
		t.Fatalf("the caller's checkout never moves: on %s", branch)
	}
}

func (c *connectionBed) starts() []string {
	records, _ := os.ReadDir(c.manager.Store.Root)
	var names []string
	for _, record := range records {
		names = append(names, record.Name())
	}
	return names
}

func (c *connectionBed) prepare(owners intentOwners) (string, *intentResult) {
	c.t.Helper()
	layout, err := owners.resolver.ResolveLayout(c.root())
	if err != nil {
		c.t.Fatal(err)
	}
	inv := &intentInvocation{owners: owners, layout: layout, cwd: c.root()}
	return inv.prepareGoalWorktree(c.id)
}

// TestIntentGoalWorktreePreparation (VMI-CONN-04): the build's goal
// worktree is created from the endpoint, from the remote goal branch
// adopted by the branch push owner, or from a local goal branch whose
// history agrees with origin; an occupied or foreign path and divergent
// histories refuse without effects; an existing or concurrently created
// worktree is reused; nothing is prepared without the claim.
func TestIntentGoalWorktreePreparation(t *testing.T) {
	ref := func(c *connectionBed) string { return "refs/heads/goal/" + c.id }
	hasBranch := func(c *connectionBed) bool {
		return exec.Command("git", "-C", c.root(), "rev-parse", "--verify", "-q", ref(c)).Run() == nil
	}

	c := newConnectionBed(t)
	owners := c.connectionOwners()
	owners.connection.claimCheck = func(string, string, goal.Endpoint) func() error {
		return func() error { return errors.New("goal is not claimed by this session") }
	}
	if path, result := c.prepare(owners); path != "" || result == nil || !strings.Contains(strings.Join(result.Details, " "), "not claimed") || hasBranch(c) {
		t.Fatalf("no claim, no preparation: %q %+v", path, result)
	}
	os.MkdirAll(c.worktree, 0o700)
	os.WriteFile(filepath.Join(c.worktree, "mine.txt"), []byte("someone's files\n"), 0o600)
	if path, result := c.prepare(c.connectionOwners()); path != "" || result == nil || !strings.Contains(result.Summary, "occupied") || hasBranch(c) {
		t.Fatalf("occupied path: %q %+v", path, result)
	}
	if data, _ := os.ReadFile(filepath.Join(c.worktree, "mine.txt")); string(data) != "someone's files\n" {
		t.Fatal("an occupied path is left as it is")
	}
	os.RemoveAll(c.worktree)
	connectionGit(t, c.root(), "worktree", "add", "-q", "--detach", c.worktree)
	if path, result := c.prepare(c.connectionOwners()); path != "" || result == nil || !strings.Contains(result.Summary, "detached HEAD") {
		t.Fatalf("foreign registered worktree: %q %+v", path, result)
	}
	connectionGit(t, c.root(), "worktree", "remove", c.worktree)

	// Divergent local and remote goal histories refuse.
	head := connectionGit(t, c.root(), "rev-parse", "HEAD")
	local := connectionGit(t, c.root(), "commit-tree", head+"^{tree}", "-p", head, "-m", "local")
	remote := connectionGit(t, c.root(), "commit-tree", head+"^{tree}", "-p", head, "-m", "remote")
	connectionGit(t, c.root(), "branch", "goal/"+c.id, local)
	connectionGit(t, c.root(), "push", "-q", "origin", remote+":"+ref(c))
	if path, result := c.prepare(c.connectionOwners()); path != "" || result == nil || !strings.Contains(result.Summary, "diverged") {
		t.Fatalf("divergent histories: %q %+v", path, result)
	}
	if _, err := os.Stat(c.worktree); !os.IsNotExist(err) {
		t.Fatal("a refused preparation creates no worktree")
	}
	// A local goal branch ahead of origin is checked out as it is.
	connectionGit(t, c.root(), "push", "-q", "-f", "origin", head+":"+ref(c))
	path, result := c.prepare(c.connectionOwners())
	if result != nil || path != c.worktree || connectionGit(t, path, "rev-parse", "HEAD") != local {
		t.Fatalf("local goal branch: %q %+v", path, result)
	}
	// An existing registered worktree is reused without any new act.
	owners = c.connectionOwners()
	owners.connection.claimCheck = func(string, string, goal.Endpoint) func() error {
		t.Fatal("reuse needs no preparation")
		return nil
	}
	if again, result := c.prepare(owners); result != nil || again != path {
		t.Fatalf("existing worktree: %q %+v", again, result)
	}

	// Remote only: the push owner adopts origin's goal branch first.
	r := newConnectionBed(t)
	tip := r.endpointTip()
	connectionGit(t, r.root(), "push", "-q", "origin", tip+":"+ref(r))
	path, result = r.prepare(r.connectionOwners())
	if result != nil || path != r.worktree || connectionGit(t, path, "rev-parse", "HEAD") != tip ||
		connectionGit(t, r.root(), "rev-parse", "refs/metasystem/goals/origin/"+r.id) != tip {
		t.Fatalf("remote-only adoption: %q %+v", path, result)
	}
	if branch := connectionGit(t, r.root(), "symbolic-ref", "--short", "HEAD"); branch != "main" {
		t.Fatalf("adoption must not switch the caller's checkout: on %s", branch)
	}

	// A detached worktree at the path that this goal's preparation did not
	// lock is foreign, even when origin has the goal branch.
	f := newConnectionBed(t)
	fTip := f.endpointTip()
	connectionGit(t, f.root(), "push", "-q", "origin", fTip+":"+ref(f))
	connectionGit(t, f.root(), "worktree", "add", "-q", "--detach", f.worktree, fTip)
	if path, result := f.prepare(f.connectionOwners()); path != "" || result == nil || !strings.Contains(result.Summary, "did not create") || hasBranch(f) {
		t.Fatalf("foreign detached worktree with a remote branch: %q %+v", path, result)
	}
	connectionGit(t, f.root(), "worktree", "remove", f.worktree)
	// This goal's own interrupted adoption before the branch ref moved is
	// resumed by the push owner and unlocked.
	connectionGit(t, f.root(), "worktree", "add", "-q", "--lock", "--reason", goalWorktreeLockReason(f.id), "--detach", f.worktree, fTip)
	path, result = f.prepare(f.connectionOwners())
	if result != nil || path != f.worktree || connectionGit(t, path, "symbolic-ref", "--short", "HEAD") != "goal/"+f.id ||
		strings.Contains(connectionGit(t, f.root(), "worktree", "list", "--porcelain"), "locked") {
		t.Fatalf("owned pre-ref adoption: %q %+v", path, result)
	}
	// After the owner moved the ref but before it switched the worktree,
	// the clean owned worktree is switched to the adopted branch.
	p := newConnectionBed(t)
	pTip := p.endpointTip()
	adopted := connectionGit(t, p.root(), "commit-tree", pTip+"^{tree}", "-p", pTip, "-m", "remote goal work")
	connectionGit(t, p.root(), "push", "-q", "origin", adopted+":"+ref(p))
	connectionGit(t, p.root(), "worktree", "add", "-q", "--lock", "--reason", goalWorktreeLockReason(p.id), "--detach", p.worktree, pTip)
	connectionGit(t, p.root(), "branch", "goal/"+p.id, adopted)
	path, result = p.prepare(p.connectionOwners())
	if result != nil || path != p.worktree || connectionGit(t, path, "rev-parse", "HEAD") != adopted ||
		strings.Contains(connectionGit(t, p.root(), "worktree", "list", "--porcelain"), "locked") {
		t.Fatalf("owned post-ref adoption: %q %+v", path, result)
	}

	// A concurrent creation that wins the race is reused.
	w := newConnectionBed(t)
	owners = w.connectionOwners()
	real := owners.work.git
	if real == nil {
		layout, _ := owners.resolver.ResolveLayout(w.root())
		real = (&intentInvocation{owners: owners, layout: layout}).work().git
	}
	owners.work.git = func(dir string, args ...string) ([]byte, error) {
		if len(args) > 1 && args[0] == "worktree" && args[1] == "add" {
			if output, err := real(dir, args...); err != nil {
				return output, err
			}
			return nil, errors.New("fatal: a concurrent creation already registered this worktree")
		}
		return real(dir, args...)
	}
	if path, result := w.prepare(owners); result != nil || path != w.worktree || connectionGit(t, path, "symbolic-ref", "--short", "HEAD") != "goal/"+w.id {
		t.Fatalf("racing creation: %q %+v", path, result)
	}
}

// A review's close is aimed at the checkout the review ran in, which arrives
// as a plain path: it replaces the selected installation only when it holds
// metasystem.conf.
func TestReviewCloseAdmitsItsCheckoutThroughParseInstallation(t *testing.T) {
	t.Parallel()
	selected, checkout := t.TempDir(), t.TempDir()
	inv := &intentInvocation{layout: stateroot.Layout{InstallationRoot: stateroottest.Installation(t, selected)}}
	if closer, refused := inv.closerAt(nil, checkout); closer != nil || refused == nil || refused.Outcome != intentRefused ||
		!strings.Contains(strings.Join(refused.Details, "\n"), "is not a metasystem installation") {
		t.Fatalf("closerAt = %v, %+v; want a refusal of a checkout without metasystem.conf", closer, refused)
	}
	if err := os.WriteFile(filepath.Join(checkout, "metasystem.conf"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	closer, refused := inv.closerAt(nil, checkout)
	if refused != nil || closer.layout.InstallationRoot.Path() != checkout || inv.layout.InstallationRoot.Path() != selected {
		t.Fatalf("closerAt = %+v, %+v; want a copy aimed at %s that leaves the selected installation %s", closer, refused, checkout, selected)
	}
}

func TestWorkReviewRetainsMaterialFromGoalWorktree(t *testing.T) {
	t.Parallel()
	w := newDesignGateBed(t, 3)
	code, built, _ := w.work(append([]string{"work", "build", w.id, "--work", "cap", "--brief", w.brief("brief.md", "Build it.\n"), "--lines", "5"}, designGateCheck...)...)
	if code != 0 || built.Outcome != intentConfirmed {
		t.Fatalf("build: code=%d %+v", code, built)
	}
	run := resultData(t, built)["run"].(string)
	// Use the review-close bed's files in the work bed's separate checkout.
	b := &deliveryBed{intentBed: w.intentBed, install: w.worktree}
	b.writeFile(filepath.Join(b.install, "metasystem.conf"), "")
	b.writeJob(map[string]any{"jobId": "critic", "role": "code-critic", "status": "completed", "round": 1, "parentJob": nil, "engineBuild": "fixture-engine", "effectiveModel": "fixture-critic"})
	path := filepath.Join(b.install, "artifacts", "agents", "critic", "rounds", "1", "return.json")
	// The worktree owns a complete immutable read, including subject and provenance.
	readSubject := readsubject.ReadSubject{Kind: readsubject.SubjectCommit, Commit: strings.Repeat("c", 40), Tree: strings.Repeat("d", 40), DiffDigest: "fixture-diff"}
	b.writeJSON(filepath.Join(filepath.Dir(path), "subject.json"), readSubject)
	material, note := stopFinding("regression", "cap.go"), stopFinding("scope", "notes.go")
	material.ID, note.ID, note.Material = "F1", "N1", false
	b.writeJSON(path, map[string]any{"jobId": "critic", "round": 1, "verdict": "1 finding", "verdictMaterialCount": 1, "reviewedTree": readSubject.Tree, "findings": []any{material, note}})
	b.writeFile(filepath.Join(filepath.Dir(path), "return.md"), "VERDICT: FIX material=1\n")
	owners := w.workOwners()
	owners.delivery = &intentDeliveryOwners{branchRead: func([]string) (branch.BranchReadResult, int, error) {
		return branch.BranchReadResult{State: "closed", RootJob: "critic"}, 0, nil
	}}
	layout, err := owners.resolver.ResolveLayout(w.root())
	if err != nil {
		t.Fatal(err)
	}
	inv := &intentInvocation{owners: owners, layout: layout, cwd: w.root(), stateRoot: w.root(), input: intentInput{values: map[string][]string{}},
		reviewWork: &reviewWorkContext{goal: w.id, work: "cap", attempt: 1}}
	runner := inv.unitRunner()
	if runner.ExaminationRoot == b.install {
		t.Fatal("the runner's installation must differ from the critic store")
	}
	err = runner.ReviewSubject(run, func(review launch.UnitReview, retain func(launch.UnitSubject) error) error {
		subject := launch.UnitSubject{Round: review.Round.Number, DiffDigest: review.DiffDigest, Commit: strings.Repeat("c", 40)}
		inv.reviewWork.subject, inv.reviewWork.retain = &subject, retain
		closed := inv.commitReviewChecked(nil, b.install, w.id, subject.Commit, nil, func(branch.BranchReadResult) error {
			head, current, err := runner.WorktreeResult(w.worktree)
			if err != nil {
				return err
			}
			if head != review.Head || current != review.Result {
				return errors.New("the retained worktree changed during its examination")
			}
			return nil
		}, review.Wait)
		if closed.Outcome != intentInProgress || resultData(t, closed)["template"] != filepath.Join(filepath.Dir(path), "decisions.md") {
			t.Fatalf("review close must retain the examination before asking for decisions: %+v", closed)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	record, err := runner.Status(run)
	if err != nil || len(record.Subjects) != 1 || record.Subjects[0].Examination != "critic" || record.Subjects[0].ExaminationRound != 1 ||
		record.Subjects[0].ExaminationReturnPath != path || record.Rounds[0].Material != 1 || len(record.Notes) != 0 {
		t.Fatalf("review close must count the goal worktree's return: %+v err=%v", record, err)
	}
}

func promotionFacts() (launch.UnitReview, launch.UnitSubject, launch.Record, launch.Record, []byte, []byte) {
	yes := true
	review := launch.UnitReview{Record: launch.UnitRunRecord{ID: "unit-run-a", Goal: "goal-a"}, Base: "base", BuildBrief: "brief",
		Round: launch.UnitRound{Number: 1, Outcome: "green", Steps: []launch.UnitStep{
			{Name: "build", LaunchID: "build-a", Model: "builder-alias", State: launch.StepPassed},
			{Name: "read", LaunchID: "read-a", Model: "reader-alias", State: launch.StepPassed, Verdict: "VERDICT: land", VerdictCounts: &yes}}}}
	subject := launch.UnitSubject{Commit: "commit", ExpectedParent: "base", StagedTree: "tree", Published: "commit"}
	build := launch.Record{ID: "build-a", Adapter: "codex-exec", AdapterData: map[string]json.RawMessage{"model": json.RawMessage(`"builder-model"`)}}
	read := launch.Record{ID: "read-a", Kind: "read", State: launch.Completed, Adapter: "codex-exec", VerdictCounts: &yes,
		AdapterData: map[string]json.RawMessage{"model": json.RawMessage(`"reader-model"`)}}
	report := []byte("The changes hold.\nVERDICT: land\n")
	readJSON, _ := json.MarshalIndent(read, "", " ")
	return review, subject, build, read, report, readJSON
}

func TestCleanReadIsPromoted(t *testing.T) {
	t.Parallel()
	review, subject, build, read, report, raw := promotionFacts()
	bundle, reason := unitReadPromotion(review, subject, 7, build, read, report, raw)
	if bundle == nil || reason != "" || bundle.ReadModel != "reader-model" || bundle.BuildModel != "builder-model" || bundle.Report != string(report) || bundle.LaunchRecord != string(raw) || bundle.GoalRevision != 7 || bundle.ExaminedTree != "tree" || bundle.ExaminedBase != "base" {
		t.Fatalf("bundle=%+v reason=%s", bundle, reason)
	}
	subject.Amends, subject.AmendsParent, subject.ExpectedParent = "old-commit", "base", "old-tip"
	if bundle, reason := unitReadPromotion(review, subject, 7, build, read, report, raw); bundle == nil || reason != "" {
		t.Fatalf("amend=%+v reason=%s", bundle, reason)
	}
}

func TestReadOnTheBuildModelIsNotPromoted(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"same model", "fix verdict", "uncounted", "partitioned", "base", "missing report", "missing record"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			review, subject, build, read, report, raw := promotionFacts()
			want := "unavailable"
			switch name {
			case "same model":
				read.AdapterData["model"] = build.AdapterData["model"]
				want = "different"
			case "fix verdict":
				review.Round.Steps[1].Verdict = "VERDICT: fix first (1 material findings)"
				want = "VERDICT: land"
			case "uncounted":
				no := false
				review.Round.Steps[1].VerdictCounts = &no
				want = "does not count"
			case "partitioned":
				review.Round.Steps = append(review.Round.Steps, review.Round.Steps[1])
				want = "exactly one"
			case "base":
				subject.ExpectedParent = "other"
				want = "parent"
			case "missing report":
				report = nil
			case "missing record":
				raw = nil
			}
			bundle, reason := unitReadPromotion(review, subject, 7, build, read, report, raw)
			if bundle != nil || !strings.Contains(reason, want) {
				t.Fatalf("bundle=%+v reason=%q want=%s", bundle, reason, want)
			}
		})
	}
}

func newUnitPromotionReview(t *testing.T, clean bool) (*workBed, *intentInvocation, launch.UnitReview) {
	t.Helper()
	bed := newWorkBed(t)
	owners := bed.workOwners()
	owners.work.inspectRead = func(root, goal, commit string) (branch.BranchReadResult, error) {
		if root != bed.worktree || goal != bed.id || commit != "commit" {
			t.Fatalf("inspection root=%s goal=%s commit=%s", root, goal, commit)
		}
		return branch.BranchReadResult{}, nil
	}
	owners.work.resolveModel = func(_, _, model string) (string, error) {
		if clean && model == "reader-alias" {
			return "reader-model", nil
		}
		return "builder-model", nil
	}
	layout, err := owners.resolver.ResolveLayout(bed.root())
	if err != nil {
		t.Fatal(err)
	}
	review, subject, build, read, report, _ := promotionFacts()
	review.Record.Goal, review.Record.Unit, review.Record.Worktree = bed.id, "promote", bed.worktree
	subject.Tip, bed.head = "commit", "commit"
	review.Subject = &subject
	review.Round.Directory = filepath.Join(t.TempDir(), "round-1")
	build.AdapterData["model"], read.AdapterData["model"] = json.RawMessage(`"builder-alias"`), json.RawMessage(`"reader-alias"`)
	reportPath := filepath.Join(t.TempDir(), "report.md")
	if err := os.WriteFile(reportPath, report, 0o600); err != nil {
		t.Fatal(err)
	}
	read.Outputs = []launch.Output{{Path: reportPath}}
	for _, record := range []launch.Record{build, read} {
		if err := bed.manager.Store.Create(record); err != nil {
			t.Fatal(err)
		}
	}
	return bed, &intentInvocation{owners: owners, layout: layout, stateRoot: bed.root()}, review
}

func runUnitPromotionReview(t *testing.T, bed *workBed, inv *intentInvocation, review launch.UnitReview) intentResult {
	t.Helper()
	want := *review.Subject
	want.Examination, want.ExaminationJob, want.ExaminationRound = "critic-a", "critic-a", 1
	want.ExaminationReturnPath = filepath.Join(bed.worktree, "artifacts", "agents", "critic-a", "rounds", "1", "return.json")
	return inv.reviewUnitRound(&launch.UnitRunner{Manager: bed.manager, Root: bed.unitRoot, Git: workGit{bed}}, nil, review, func(subject launch.UnitSubject) error {
		if !reflect.DeepEqual(subject, want) {
			t.Fatalf("publication rewrote its subject: %+v", subject)
		}
		return nil
	})
}

func TestUnitReviewRecordsThePromotedRead(t *testing.T) {
	t.Parallel()
	for _, clean := range []bool{true, false} {
		t.Run(strconv.FormatBool(clean), func(t *testing.T) {
			t.Parallel()
			bed, inv, review := newUnitPromotionReview(t, clean)
			inv.input = intentInput{values: map[string][]string{"model": {"unused-model"}}}
			reads, publications := 0, 0
			inv.owners.delivery = &intentDeliveryOwners{
				branchState: func(string, string) (intentBranchState, error) { return intentBranchState{}, nil },
				branchRead: func(args []string) (branch.BranchReadResult, int, error) {
					reads++
					index := slices.Index(args, "--unit-read")
					if clean {
						if index < 0 || slices.Contains(args, "--brief") || slices.Contains(args, "--join") {
							t.Fatalf("promotion args %v", args)
						}
						body, err := os.ReadFile(args[index+1])
						var bundle branch.UnitReadBundle
						if err != nil || json.Unmarshal(body, &bundle) != nil || bundle.Goal != bed.id || bundle.ReadLaunch != "read-a" || bundle.ReadModel != "reader-model" || bundle.BuildModel != "builder-model" || bundle.GoalRevision != bed.goalFile(bed.id).Revision {
							t.Fatalf("bundle=%+v err=%v", bundle, err)
						}
					} else if index >= 0 || !slices.Contains(args, "--brief") || !slices.Contains(args, "--join") {
						t.Fatalf("critic args %v", args)
					}
					return branch.BranchReadResult{State: "collected", AttestationCommit: "attestation"}, 0, nil
				},
				publishRead: func(string, string, string) (branch.PublishReadResult, error) {
					publications++
					return branch.PublishReadResult{State: "current"}, nil
				},
			}
			result := runUnitPromotionReview(t, bed, inv, review)
			data, _ := result.Data.(map[string]any)
			if result.Outcome != intentConfirmed || reads != 1 || publications != 1 || !slices.Equal(result.next, inv.publicArgv("work", "build", bed.id, "--work", "NAME", "--brief", "FILE", "--check", "COMMAND")) {
				t.Fatalf("result=%+v reads=%d publications=%d", result, reads, publications)
			}
			if clean && (!strings.Contains(result.Summary, "read-a by reader-model is the unit's read") || data["readNotPromoted"] != nil || !strings.Contains(result.Summary, "--model was not used because no critic started")) {
				t.Fatalf("promotion=%+v", result)
			}
			if !clean && !strings.Contains(fmt.Sprint(data["readNotPromoted"]), "different") {
				t.Fatalf("reason=%+v", result)
			}
			if clean {
				inv.owners.delivery.publishRead = func(string, string, string) (branch.PublishReadResult, error) {
					return branch.PublishReadResult{}, errors.New("publication unavailable")
				}
				result = runUnitPromotionReview(t, bed, inv, review)
				if result.Outcome != intentPartial || !strings.Contains(result.Summary, "--model was not used because no critic started") {
					t.Fatalf("unpublished promotion=%+v", result)
				}
			}
		})
	}
}

func TestUnitReviewRecordsThePromotedReadContinuesRecordedCritic(t *testing.T) {
	t.Parallel()
	for _, state := range []string{"open", "already-collected"} {
		t.Run(state, func(t *testing.T) {
			t.Parallel()
			bed, inv, review := newUnitPromotionReview(t, true)
			(&deliveryBed{intentBed: bed.intentBed, install: bed.worktree}).writeJob(map[string]any{"jobId": "critic-a", "role": "code-critic", "round": 1, "status": "completed", "chainClosed": true})
			inspections, reads := 0, 0
			inv.owners.work.inspectRead = func(string, string, string) (branch.BranchReadResult, error) {
				inspections++
				return branch.BranchReadResult{RootJob: "critic-a"}, nil
			}
			inv.owners.delivery = &intentDeliveryOwners{
				branchState: func(string, string) (intentBranchState, error) { return intentBranchState{}, nil },
				branchRead: func(args []string) (branch.BranchReadResult, int, error) {
					reads++
					if slices.Contains(args, "--unit-read") || !slices.Contains(args, "--join") || slices.Index(args, "--brief") < 0 {
						t.Fatalf("recorded critic args %v", args)
					}
					return branch.BranchReadResult{State: state, RootJob: "critic-a", AttestationCommit: "attestation"}, 0, nil
				},
				publishRead: func(string, string, string) (branch.PublishReadResult, error) {
					return branch.PublishReadResult{State: "current"}, nil
				},
			}
			result := runUnitPromotionReview(t, bed, inv, review)
			data, _ := result.Data.(map[string]any)
			want := intentInProgress
			if state == "already-collected" {
				want = intentUnchanged
			}
			if result.Outcome != want || inspections != 3 || reads != 1 || data["rootJob"] != "critic-a" || !strings.Contains(fmt.Sprint(data["readNotPromoted"]), "critic") {
				t.Fatalf("result=%+v inspections=%d reads=%d", result, inspections, reads)
			}
		})
	}
}

func TestUnitReviewRecordsThePromotedReadFallsBackOnRefusal(t *testing.T) {
	t.Parallel()
	for _, refusal := range []error{
		&branch.OpError{Code: branch.ReadInvalidCode, Message: "unit read does not bind this commit"},
		errors.New("unit bundle cannot be recorded\nrun: metasystem work review --commit commit"),
	} {
		t.Run(refusal.Error(), func(t *testing.T) {
			t.Parallel()
			bed, inv, review := newUnitPromotionReview(t, true)
			(&deliveryBed{intentBed: bed.intentBed, install: bed.worktree}).writeJob(map[string]any{"jobId": "critic-a", "role": "code-critic", "round": 1, "status": "completed", "chainClosed": true})
			reads := 0
			inv.owners.delivery = &intentDeliveryOwners{
				branchState: func(string, string) (intentBranchState, error) { return intentBranchState{}, nil },
				branchRead: func(args []string) (branch.BranchReadResult, int, error) {
					reads++
					if reads == 1 {
						if !slices.Contains(args, "--unit-read") {
							t.Fatalf("promotion args %v", args)
						}
						return branch.BranchReadResult{}, 1, refusal
					}
					if reads != 2 || slices.Contains(args, "--unit-read") || !slices.Contains(args, "--join") || slices.Index(args, "--brief") < 0 {
						t.Fatalf("fallback args %v", args)
					}
					return branch.BranchReadResult{State: "collected", RootJob: "critic-a", AttestationCommit: "attestation"}, 0, nil
				},
				publishRead: func(string, string, string) (branch.PublishReadResult, error) {
					return branch.PublishReadResult{State: "current"}, nil
				},
			}
			result := runUnitPromotionReview(t, bed, inv, review)
			data, _ := result.Data.(map[string]any)
			if result.Outcome != intentConfirmed || reads != 2 || data["readNotPromoted"] != strings.Split(refusal.Error(), "\nrun:")[0] || data["readLaunch"] != nil || strings.Contains(result.Summary, "build's clean read") || !slices.Equal(result.next, inv.publicArgv("work", "build", bed.id, "--work", "NAME", "--brief", "FILE", "--check", "COMMAND")) {
				t.Fatalf("result=%+v reads=%d", result, reads)
			}
		})
	}
}

func TestUnitReviewRecordsThePromotedReadReusesBundle(t *testing.T) {
	t.Parallel()
	bed, inv, review := newUnitPromotionReview(t, true)
	var saved []byte
	var savedPath string
	reads := 0
	inv.owners.delivery = &intentDeliveryOwners{
		branchState: func(string, string) (intentBranchState, error) { return intentBranchState{}, nil },
		branchRead: func(args []string) (branch.BranchReadResult, int, error) {
			reads++
			index := slices.Index(args, "--unit-read")
			if index < 0 {
				t.Fatalf("promotion args %v", args)
			}
			path := args[index+1]
			body, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if reads == 1 {
				saved, savedPath = body, path
			} else if savedPath != path || !bytes.Equal(body, saved) {
				t.Fatalf("bundle changed: before=%s after=%s", saved, body)
			}
			// No attestation save is reported: the repeat must reconcile the
			// published read against exactly the same bundle bytes.
			return branch.BranchReadResult{State: "collected", AttestationCommit: "attestation"}, 0, nil
		},
		publishRead: func(string, string, string) (branch.PublishReadResult, error) {
			return branch.PublishReadResult{State: "current"}, nil
		},
	}
	if result := runUnitPromotionReview(t, bed, inv, review); result.Outcome != intentConfirmed {
		t.Fatalf("first result=%+v", result)
	}
	file := bed.goalFile(bed.id)
	file.Revision++
	bed.addGoal(file)
	// A retained bundle remains the read even if today's alias table changed.
	inv.owners.work.resolveModel = func(_, _, _ string) (string, error) {
		t.Fatal("a repeat resolved models again")
		return "", nil
	}
	if result := runUnitPromotionReview(t, bed, inv, review); result.Outcome != intentConfirmed || reads != 2 {
		t.Fatalf("repeat result=%+v reads=%d", result, reads)
	}
}

func TestUnitReviewRecordsThePromotedReadRefusalDoesNotRepeat(t *testing.T) {
	t.Parallel()
	bed, inv, review := newUnitPromotionReview(t, true)
	inv.command = mustIntentCommand(t, "work review")
	inv.raw = []string{bed.id, "--work", review.Record.Unit}
	reads := 0
	inv.owners.delivery = &intentDeliveryOwners{
		branchState: func(string, string) (intentBranchState, error) { return intentBranchState{}, nil },
		branchRead: func(args []string) (branch.BranchReadResult, int, error) {
			reads++
			if reads == 1 {
				if !slices.Contains(args, "--unit-read") {
					t.Fatalf("promotion args %v", args)
				}
				return branch.BranchReadResult{}, 1, errors.New("bundle refused")
			}
			if reads != 2 || slices.Contains(args, "--unit-read") || !slices.Contains(args, "--join") {
				t.Fatalf("critic args %v", args)
			}
			return branch.BranchReadResult{}, 1, errors.New("critic unavailable")
		},
	}
	result := runUnitPromotionReview(t, bed, inv, review)
	data, _ := result.Data.(map[string]any)
	want := inv.publicArgv("work", "review", bed.id, "--work", review.Record.Unit)
	if result.Outcome != intentRefused || reads != 2 || data["readNotPromoted"] != "bundle refused" || !slices.Equal(result.next, want) {
		t.Fatalf("result=%+v reads=%d", result, reads)
	}
}

func TestUnitReviewRecordsThePromotedReadAlreadyInstalled(t *testing.T) {
	t.Parallel()
	bed, inv, review := newUnitPromotionReview(t, true)
	inspections, reads, publications := 0, 0, 0
	inv.owners.work.inspectRead = func(string, string, string) (branch.BranchReadResult, error) {
		inspections++
		if inspections == 1 {
			return branch.BranchReadResult{}, nil
		}
		return branch.BranchReadResult{State: "collected", AttestationCommit: "attestation"}, nil
	}
	inv.owners.delivery = &intentDeliveryOwners{
		branchState: func(string, string) (intentBranchState, error) { return intentBranchState{}, nil },
		branchRead: func([]string) (branch.BranchReadResult, int, error) {
			reads++
			return branch.BranchReadResult{}, 1, errors.New("bundle refused")
		},
		publishRead: func(string, string, string) (branch.PublishReadResult, error) {
			publications++
			return branch.PublishReadResult{State: "current"}, nil
		},
	}
	result := runUnitPromotionReview(t, bed, inv, review)
	if result.Outcome != intentUnchanged || inspections != 4 || reads != 1 || publications != 1 || resultData(t, result)["attestation"] != "attestation" {
		t.Fatalf("result=%+v inspections=%d reads=%d publications=%d", result, inspections, reads, publications)
	}
}

func TestCleanReadIsPromotedWithTrimmedVerdict(t *testing.T) {
	t.Parallel()
	review, subject, build, read, _, raw := promotionFacts()
	report := []byte("The changes hold.\r\nVERDICT: land \r\n")
	bundle, reason := unitReadPromotion(review, subject, 7, build, read, report, raw)
	if bundle == nil || reason != "" || bundle.Report != string(report) {
		t.Fatalf("bundle=%+v reason=%s", bundle, reason)
	}
	report = []byte("VERDICT: fix first\r\nVERDICT: land \r\n")
	if bundle, reason := unitReadPromotion(review, subject, 7, build, read, report, raw); bundle != nil || !strings.Contains(reason, "VERDICT:") {
		t.Fatalf("first verdict bundle=%+v reason=%s", bundle, reason)
	}
}
