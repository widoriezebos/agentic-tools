package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
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
	empty := filepath.Join(t.TempDir(), "gitconfig")
	os.WriteFile(empty, nil, 0o600)
	t.Setenv("GIT_CONFIG_GLOBAL", empty)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	c := &connectionBed{workBed: newWorkBed(t), t: t, edits: map[string]string{}}
	root := c.root()
	connectionGit(t, root, "init", "-q", "-b", "main")
	connectionGit(t, root, "config", "user.name", "Fixture")
	connectionGit(t, root, "config", "user.email", "fixture@example.invalid")
	os.WriteFile(filepath.Join(root, ".gitignore"), []byte("artifacts/\n.claude/settings.local.json\nmetasystem.conf.local\n"), 0o600)
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
func (c *connectionBed) StartSupervisor(id, _ string) (identity.Ref, error) {
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
		current.State, current.ExitCode = launch.Completed, &code
		if record.Kind == "read" && c.readFails {
			failed := 1
			current.State, current.Reason, current.ExitCode = launch.Failed, "fixture-read-failed", &failed
		} else if record.Kind == "read" {
			yes := true
			current.VerdictCounts, current.Measurement.Verdict = &yes, "pass"
		}
		return nil
	})
	return workRef(10), nil
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
	root, worktree := c.root(), c.worktree
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
		transport: connectionTransport{c},
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
		// The bed's close owner runs as a person's act (its engine wrapper
		// classifies HUMAN); the record-writer authority owner judges that
		// same classification.
		recordWriter: humanRecordWriter,
		process:      c.closeOwner,
		publishRead: func(root, goalID, unit string) (branch.PublishReadResult, error) {
			c.publications++
			return branch.PublishCollectedRead(branch.PublishReadRequest{Repo: root, Remote: "origin", EndpointTip: c.endpointTip(),
				GoalID: goalID, UnitCommit: unit, CheckClaim: func() error { return nil }, Transport: connectionTransport{c}})
		},
		branchRead: func(args []string) (branch.BranchReadResult, int, error) {
			c.reads = append(c.reads, args)
			install, goalID := flagValue(args, "--root"), flagValue(args, "--goal")
			retry, _ := strconv.ParseInt(flagValue(args, "--retry"), 10, 64)
			tip, present, err := branch.GitPushTransport{}.RemoteTip(install, "origin", "refs/heads/goal/"+goalID)
			if err != nil || !present {
				return branch.BranchReadResult{}, 1, fmt.Errorf("origin has no goal/%s: %v", goalID, err)
			}
			result, err := branch.RunBranchRead(branch.BranchReadRequest{Repo: install, Remote: "origin",
				EndpointTip: c.endpointTip(), BranchTip: tip, GoalID: goalID, UnitCommit: flagValue(args, "--unit"),
				Collect: slices.Contains(args, "--collect"), BriefPath: flagValue(args, "--brief"),
				CheckClaim: func() error { return nil },
				Gate:       func(string) (string, error) { return "gate-run-1", nil },
				Delegate:   c.criticDispatch(install),
				Retry:      retry,
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
		"reviews": "commit:" + commit, "goalId": c.id, "goalRevision": 1, "findingRegister": []any{}}
	if closed {
		subject, present, err := dispatchcore.ComputeReadSubject(dispatchcore.ReadSubjectRequest{RepoRoot: install, Role: "code-critic", Reviews: "commit:" + commit})
		if err != nil || !present {
			c.t.Fatalf("critic subject present=%v err=%v", present, err)
		}
		record["chainClosed"], record["findingRegisterRound"], record["findingRegisterSubjectDigest"] = true, 1, subject.Digest()
		record["closure"] = map[string]any{"criticRoot": job, "round": 1, "subject": subject, "mechanism": "clean"}
		c.writeJSON(filepath.Join(install, "artifacts", "agents", job, "rounds", "1", "subject.json"), subject)
		c.writeJSON(filepath.Join(install, "artifacts", "agents", job, "rounds", "1", "return.json"),
			map[string]any{"jobId": job, "round": 1, "findings": []any{map[string]any{"id": "F1", "material": false}}, "verdict": "1 finding", "reviewedTree": subject.Tree})
	} else if status == "completed" {
		c.writeJSON(filepath.Join(install, "artifacts", "agents", job, "rounds", "1", "return.json"),
			map[string]any{"jobId": job, "round": 1, "findings": []any{map[string]any{"id": "F1", "material": false}}, "verdict": "1 finding"})
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

// closeOwner is the whole close owner (dispatch.sh close): it stamps the
// chain's closure after the public close joined the dispositions.
func (c *connectionBed) closeOwner(process intentProcess) intentProcessResult {
	c.closes = append(c.closes, process.argv)
	job := flagValue(process.argv, "--job")
	if c.failCloses > 0 {
		c.failCloses--
		return intentProcessResult{code: 1, stderr: []byte("close check: the evidence mirror is missing\n")}
	}
	if filepath.Base(process.argv[0]) != "dispatch.sh" || process.argv[1] != "close" || slices.Contains(process.argv, "--runner-closed") {
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
	command, ok := findIntentCommand(args[0])
	if !ok {
		c.t.Fatalf("no public command %q", args[0])
	}
	var stdout, stderr bytes.Buffer
	code := runIntentIn(command, append([]string{"--json"}, args[1:]...), &stdout, &stderr, c.root(), c.connectionOwners())
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

// adapterFixture puts the repository's real Claude adapter entrypoint and a
// synthetic declared local settings file (plus a synthetic local
// configuration that must never be copied) into the selected checkout.
func (c *connectionBed) adapterFixture() string {
	c.t.Helper()
	adapters := filepath.Join(c.root(), "scripts", "agents", "adapters")
	if err := os.MkdirAll(adapters, 0o700); err != nil {
		c.t.Fatal(err)
	}
	for _, name := range []string{"claude.sh", "runtime-common.sh"} {
		data, err := os.ReadFile(filepath.Join("..", "..", "scripts", "agents", "adapters", name))
		if err != nil {
			c.t.Fatal(err)
		}
		if err := testexec.WriteFile(filepath.Join(adapters, name), data, 0o755); err != nil {
			c.t.Fatal(err)
		}
	}
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
// the way dispatch.sh close would; the real close/register fixture and the
// real public land are delivery's (TestIntentCloseWholeOwner and its land
// fixtures) and are not claimed by this test. Landing is observed through
// the landing admission reader on the published branch only.
func TestIntentBuiltUnitToLanding(t *testing.T) {
	c := newConnectionBed(t)
	settings := c.adapterFixture()
	base := c.endpointTip()
	brief := c.brief("brief.md", "Build the connection.\n")
	c.edits = map[string]string{"connect.txt": "the built result\n", "café.txt": "accented\n", "tab\tname.txt": "tab\n",
		"quote\"d name.txt": "quoted\n", "dir with space/space name.txt": "spaced\n"}
	code, result := c.do(append([]string{"build", c.id, "connect", "--brief", brief, "--lines", "10"}, workCheck...)...)
	if code != 0 || result.Outcome != intentConfirmed {
		t.Fatalf("build: code=%d %+v", code, result)
	}
	run := resultData(t, result)["run"].(string)
	if result.Next == nil || !slices.Equal(result.Next.Argv, []string{"metasystem", "review", c.id, "--work", "connect"}) || run == "" {
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
	code, result = c.do("review", "unit", run)
	if result.Outcome != intentRefused || !strings.Contains(result.Summary, "UNIT_RESULT_CHANGED") ||
		connectionGit(t, c.worktree, "diff", "--cached", "--name-only") != "" || c.commits != 0 {
		t.Fatalf("stale result: code=%d %+v", code, result)
	}
	os.Remove(late)
	// The same path with other bytes is refused too.
	os.WriteFile(filepath.Join(c.worktree, "café.txt"), []byte("accentuated\n"), 0o644)
	if _, result = c.do("review", "unit", run); !strings.Contains(result.Summary, "UNIT_RESULT_CHANGED") || c.commits != 0 {
		t.Fatalf("changed bytes at a result path: %+v", result)
	}
	os.WriteFile(filepath.Join(c.worktree, "café.txt"), []byte("accented\n"), 0o644)

	// The commit is made but its response is lost, then publication fails:
	// exactly one unit commit, reconciled and reported partial.
	c.loseCommit, c.failPushes = true, 1
	code, result = c.do("review", "unit", run)
	if result.Outcome != intentPartial || result.Next == nil ||
		!slices.Equal(slices.DeleteFunc(slices.Clone(result.Next.Argv), func(word string) bool { return word == "--json" }), []string{"metasystem", "review", "unit", run}) {
		t.Fatalf("lost commit then failed push: code=%d %+v", code, result)
	}
	subjects := c.runRecord(run).Subjects
	if c.commits != 1 || len(subjects) != 1 || subjects[0].Commit == "" || subjects[0].Published != "" ||
		subjects[0].ExpectedParent != base || len(c.unitCommits("goal/"+c.id)) != 1 || len(subjects[0].Paths) != 5 {
		t.Fatalf("one retained, unpublished unit commit of the five exact paths: commits=%d %+v", c.commits, subjects)
	}
	first := subjects[0].Commit
	for _, path := range []string{"café.txt", "tab\tname.txt", "quote\"d name.txt", "dir with space/space name.txt"} {
		if out := connectionGit(t, c.root(), "cat-file", "-p", first+":"+path); out == "" {
			t.Fatalf("%q is not in the unit commit", path)
		}
	}
	code, result = c.do("review", "unit", run)
	if result.Outcome != intentInProgress || len(c.delegates) != 1 || c.delegates[0] != first || c.commits != 1 {
		t.Fatalf("retry publishes the same commit and requests one critic: code=%d %+v delegates=%v", code, result, c.delegates)
	}
	if last := c.reads[len(c.reads)-1]; flagValue(last, "--selected-installation") == "" || slices.Contains(last, "--runtime") || slices.Contains(last, "--model") {
		t.Fatalf("the worktree read must carry the selected installation, never a critic override: %v", last)
	}
	if remote := connectionGit(t, c.root(), "ls-remote", c.origin, "refs/heads/goal/"+c.id); !strings.HasPrefix(remote, first) {
		t.Fatalf("the unit commit is published: %q", remote)
	}
	_, result = c.do("review", "unit", run)
	if result.Outcome != intentInProgress || len(c.delegates) != 1 {
		t.Fatalf("a running critic is waited on, never dispatched again: %+v", result)
	}

	// Finished but unclosed: the author's close is named, nothing collected.
	c.writeCritic(install, "crit1", first, "completed", false)
	_, result = c.do("review", "unit", run)
	if result.Outcome != intentInProgress || !strings.Contains(result.Decision, "review unit "+run+" --dispositions FILE") ||
		len(c.unitCommits("goal/"+c.id)) != 1 || c.commitReads != 0 {
		t.Fatalf("unclosed critic: %+v", result)
	}
	code, result = c.do("close", "crit1", "--dispositions", c.dispositions(), "--repo", install)
	if code != 0 || result.Outcome != intentConfirmed || len(c.closes) != 1 {
		t.Fatalf("public close (fake whole owner): code=%d %+v", code, result)
	}

	// Collection installs the Goal-Read but its record save is lost; the
	// retry adopts that exact read, publication then fails once, and the
	// next retry publishes: one attestation, one critic, one collection.
	c.loseReadSave = true
	code, result = c.do("review", "unit", run)
	if result.Outcome == intentConfirmed || c.commitReads != 1 {
		t.Fatalf("interrupted collection: code=%d %+v reads=%d", code, result, c.commitReads)
	}
	c.failPushes = 1
	code, result = c.do("review", "unit", run)
	if result.Outcome != intentPartial || c.commitReads != 1 {
		t.Fatalf("adopted read, failed publication: code=%d %+v reads=%d", code, result, c.commitReads)
	}
	attestation := resultData(t, result)["attestation"].(string)
	code, result = c.do("review", "unit", run)
	if code != 0 || (result.Outcome != intentConfirmed && result.Outcome != intentUnchanged) || resultData(t, result)["attestation"] != attestation ||
		result.Next == nil || !slices.Equal(result.Next.Argv, []string{"metasystem", "land", c.id}) || len(c.delegates) != 1 || c.commitReads != 1 {
		t.Fatalf("published read: code=%d %+v", code, result)
	}
	if commits := c.unitCommits("goal/" + c.id); len(commits) != 2 {
		t.Fatalf("one unit and exactly one Goal-Read: %v", commits)
	}
	status := c.landAdmission()
	if status.Prefix != 1 || len(status.Units) != 1 || status.Units[0].Commit != first {
		t.Fatalf("landing admission must see the one read unit: %+v", status)
	}

	// A later unit survives on the branch after the first unit's read.
	c.edits = map[string]string{"later.txt": "a later unit\n"}
	_, result = c.do(append([]string{"build", c.id, "later", "--brief", c.brief("later.md", "A later unit.\n"), "--lines", "5"}, workCheck...)...)
	later := resultData(t, result)["run"].(string)
	if code, result = c.do("review", "unit", later); result.Outcome != intentInProgress || len(c.delegates) != 2 {
		t.Fatalf("later unit: code=%d %+v", code, result)
	}

	// Same-unit follow-up: the finding is folded and the unit's commit is
	// amended with a lost commit response. The replacement unit (not the
	// replayed tip) is the subject, its old read is dropped, the later unit
	// is replayed, and a new critic reads the replacement.
	c.edits = map[string]string{"connect.txt": "the built result, fixed\n"}
	followUp := c.brief("follow-up.md", "Fix F1.\n")
	code, result = c.do("fold", "unit", run, "--brief", followUp)
	if code != 0 || result.Outcome != intentConfirmed || result.Next == nil || result.Next.Argv[1] != "review" {
		t.Fatalf("fold unit: code=%d %+v", code, result)
	}
	c.loseCommit = true
	code, result = c.do("review", "unit", run)
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
	if code, result = c.do("close", "crit3", "--dispositions", c.dispositions(), "--repo", install); code != 0 {
		t.Fatalf("close crit3: %+v", result)
	}
	code, result = c.do("review", "unit", run)
	if code != 0 || result.Outcome != intentConfirmed {
		t.Fatalf("amended unit read: code=%d %+v", code, result)
	}
	status = c.landAdmission()
	if status.Prefix != 1 || status.Units[0].Commit != second {
		t.Fatalf("landing admission must see exactly the current subject first: %+v", status)
	}
	if c.tokens != 3 || c.commits != 3 {
		t.Fatalf("each subject is committed once under the commit token: tokens=%d commits=%d", c.tokens, c.commits)
	}

	// A failed preliminary read with a passed proof may still request the
	// committed review.
	c.edits, c.readFails = map[string]string{"readfail.txt": "read failed, proof passed\n"}, true
	_, result = c.do(append([]string{"build", c.id, "readfail", "--brief", c.brief("readfail.md", "Read-failed unit.\n"), "--lines", "5"}, workCheck...)...)
	readFailed := resultData(t, result)["run"].(string)
	if round := c.runRecord(readFailed).Rounds[0]; round.Outcome != "read-failed" || result.Next == nil || result.Next.Argv[1] != "review" {
		t.Fatalf("read-failed build: %s %+v", round.Outcome, result)
	}
	c.readFails = false
	if code, result = c.do("review", "unit", readFailed); result.Outcome != intentInProgress || len(c.delegates) != 4 {
		t.Fatalf("read-failed round requests committed review: code=%d %+v", code, result)
	}

	// A proof that writes the worktree refuses committed review before
	// staging.
	c.edits, c.proofWrites = map[string]string{"other.txt": "another unit\n"}, true
	_, result = c.do(append([]string{"build", c.id, "wrote", "--brief", c.brief("wrote.md", "Another unit.\n"), "--lines", "5"}, workCheck...)...)
	wrote := resultData(t, result)["run"].(string)
	code, result = c.do("review", "unit", wrote)
	if result.Outcome != intentRefused || !strings.Contains(result.Summary, "proof-wrote") ||
		connectionGit(t, c.worktree, "diff", "--cached", "--name-only") != "" {
		t.Fatalf("proof-wrote: code=%d %+v", code, result)
	}
	c.proofWrites = false

	// Without the claim, no build or read of any run is launched: not a new
	// run in the existing worktree, not a follow-up.
	c.claimLost = true
	launches := len(c.starts())
	code, result = c.do(append([]string{"build", c.id, "unclaimed", "--brief", c.brief("unclaimed.md", "No claim.\n"), "--lines", "5"}, workCheck...)...)
	// The claim is checked before any run is reserved.
	if result.Outcome == intentConfirmed || !strings.Contains(result.Summary, "GOAL_BRANCH_NOT_HOLDER") || len(c.starts()) != launches {
		t.Fatalf("unclaimed build: code=%d %+v", code, result)
	}
	code, result = c.do("fold", "unit", readFailed, "--brief", c.brief("unclaimed-fold.md", "No claim.\n"))
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
	if path, result := c.prepare(owners); path != "" || result == nil || !strings.Contains(result.Summary, "not claimed") || hasBranch(c) {
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
