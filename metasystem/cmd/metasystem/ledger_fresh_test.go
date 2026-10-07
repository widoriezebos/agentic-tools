package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/census"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/hooks"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/seat"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/supervise"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/up"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/verbresult"
)

// This repository keeps the real in-memory transaction owner and controls only
// transport and immutable read timing. Publication is never replaced.
type freshCommandRepository struct {
	goal.Repository
	ctx          context.Context
	attempts     *int
	publications *int
	capture      func(context.Context, string) (string, error)
	loaded       func(string)
	cleanupErr   error
	cleaned      func(context.Context)
}

func (r freshCommandRepository) WithContext(ctx context.Context) goal.Repository {
	r.ctx = ctx
	return r
}
func (r freshCommandRepository) Capture(opid string) (string, error) {
	if r.ctx != nil {
		*r.attempts++
		if err := r.ctx.Err(); err != nil {
			return "", err
		}
	} else if r.publications != nil {
		*r.publications++
	}
	if r.capture != nil {
		return r.capture(r.ctx, opid)
	}
	return r.Repository.Capture(opid)
}
func (r freshCommandRepository) Accepted() (string, bool, error) {
	if r.ctx != nil && r.ctx.Err() != nil {
		return "", false, r.ctx.Err()
	}
	return r.Repository.Accepted()
}
func (r freshCommandRepository) IsAncestor(old, next string) (bool, error) {
	if r.ctx != nil && r.ctx.Err() != nil {
		return false, r.ctx.Err()
	}
	return r.Repository.IsAncestor(old, next)
}
func (r freshCommandRepository) CommitTime(tip string) (time.Time, error) {
	if r.ctx != nil && r.ctx.Err() != nil {
		return time.Time{}, r.ctx.Err()
	}
	return r.Repository.CommitTime(tip)
}
func (r freshCommandRepository) Files(tip string, prefixes ...string) (map[string][]byte, error) {
	if r.ctx != nil && r.ctx.Err() != nil {
		return nil, r.ctx.Err()
	}
	files, err := r.Repository.Files(tip, prefixes...)
	if r.loaded != nil {
		r.loaded(tip)
	}
	return files, err
}
func (r freshCommandRepository) AcceptedCAS(old, next string) error {
	if r.ctx != nil && r.ctx.Err() != nil {
		return r.ctx.Err()
	}
	return r.Repository.AcceptedCAS(old, next)
}
func (r freshCommandRepository) Release(opid string) error {
	if r.ctx != nil && r.ctx.Err() != nil {
		return r.ctx.Err()
	}
	if r.cleaned != nil {
		r.cleaned(r.ctx)
	}
	if r.cleanupErr != nil {
		return r.cleanupErr
	}
	err := r.Repository.Release(opid)
	if err != nil && strings.Contains(err.Error(), "undeclared goal operation release") {
		return nil
	}
	return err
}

func freshDependencies(b *goalCLIBed, r goal.Repository) syncRequestDependencies {
	var out, diagnostic bytes.Buffer
	d := b.dependencies(&out, &diagnostic)
	d.endpoint = func(root string) (goal.Endpoint, error) {
		e, err := b.endpoint(stateroot.Installation(root).Path())
		e.Repository = r
		return e, err
	}
	return d
}

func freshPublic(b *goalCLIBed, d syncRequestDependencies, args ...string) (int, string) {
	b.t.Helper()
	command, rest, ok := resolveIntentArgv(args)
	if !ok {
		b.t.Fatalf("no public route for %v", args)
	}
	var out, diagnostic bytes.Buffer
	owners := b.owners(&out, &diagnostic)
	owners.dependencies = d
	code := runIntentIn(command, rest, &out, &diagnostic, b.root, owners)
	return code, out.String() + diagnostic.String()
}

func freshRemoteEdit(t *testing.T, b *goalCLIBed, edit func(*goal.GoalFile)) (string, string) {
	t.Helper()
	old := b.tip()
	files, err := b.repo.Files(old, "plans/goals/")
	if err != nil {
		t.Fatal(err)
	}
	file, problems := goal.ParseFile(files["plans/goals/ship-widget.md"])
	if len(problems) != 0 {
		t.Fatal(problems)
	}
	edit(file)
	op := "fresh-fixture"
	parent, err := b.repo.Capture(op)
	if err != nil {
		t.Fatal(err)
	}
	next, err := b.repo.Build(op, parent, []goal.Change{{Path: "plans/goals/ship-widget.md", Content: goal.RenderFile(file)}}, "remote change")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.repo.Publish(parent, next); err != nil {
		t.Fatal(err)
	}
	return old, next
}

func TestLedgerFreshClaimAndNextObserveRemote(t *testing.T) {
	t.Parallel()
	for _, verb := range []string{"claim", "next"} {
		t.Run(verb, func(t *testing.T) {
			t.Parallel()
			b := newGoalCLIBed(t, goalCLISeed{allowTerminalProof: true})
			b.announceHolder()
			_, remote := freshRemoteEdit(t, b, func(file *goal.GoalFile) {
				file.Claimed.Machine = "other-machine"
				file.Claimed.Lineage = "other-lineage"
			})
			attempts := 0
			r := freshCommandRepository{Repository: b.repo, attempts: &attempts}
			d := freshDependencies(b, r)
			if verb == "claim" {
				code, output := freshPublic(b, d, "goal", "claim", "ship-widget")
				if code == 0 || (!strings.Contains(output, "winner:") && !strings.Contains(output, "other-machine")) {
					t.Fatalf("stale named claim: exit=%d %s", code, output)
				}
			} else {
				var out, diagnostic bytes.Buffer
				code := runGoalNextWithInputs([]string{"--root", b.root}, d, b.commandNow, &out, &diagnostic)
				if code != 0 || strings.Contains(out.String(), "continue ship-widget") || !strings.Contains(diagnostic.String(), "other-machine") {
					t.Fatalf("stale next: exit=%d %s %s", code, &out, &diagnostic)
				}
			}
			if attempts != 1 || b.tip() != remote {
				t.Fatalf("fresh decision attempts=%d accepted=%s wanted=%s", attempts, b.tip(), remote)
			}
		})
	}
}

func TestLedgerFreshProjectionPinsTipAndObservationClock(t *testing.T) {
	t.Parallel()
	b := newGoalCLIBed(t, goalCLISeed{allowTerminalProof: true})
	old, remote := freshRemoteEdit(t, b, func(file *goal.GoalFile) { file.NextStep = "Remote step."; file.Claimed.Machine = "pin-remote-machine" })
	attempts := 0
	observed := b.clock().Add(48 * time.Hour)
	r := freshCommandRepository{Repository: b.repo, attempts: &attempts}
	r.capture = func(_ context.Context, opid string) (string, error) { b.setNow(observed); return b.repo.Capture(opid) }
	r.cleaned = func(context.Context) {
		accepted, _, _ := b.repo.Accepted()
		if accepted == remote {
			if err := b.repo.AcceptedCAS(remote, old); err != nil {
				t.Fatal(err)
			}
		}
	}
	e, _ := freshDependencies(b, r).endpoint(b.root)
	p, observation, err := goal.FreshProjection(context.Background(), e, func() (time.Time, error) { return b.commandNow(b.root) })
	if err != nil || p.Tip != remote || p.Tree.Live["ship-widget"].NextStep != "Remote step." || observation.ObservedAt != observed || p.Horizon.Now != observed || attempts != 1 {
		t.Fatalf("unpinned or old-clock projection: %+v observation=%+v attempts=%d err=%v", p, observation, attempts, err)
	}
	var out, diagnostic bytes.Buffer
	code := runGoalNextWithInputs([]string{"--root", b.root}, freshDependencies(b, r), b.commandNow, &out, &diagnostic)
	if code != 0 || strings.Contains(out.String(), "continue your claimed goal: ship-widget") || !strings.Contains(diagnostic.String(), "pin-remote-machine") {
		t.Fatalf("public next reloaded the mutable ref: exit=%d %s %s", code, &out, &diagnostic)
	}
	r.cleaned = nil
	if err := b.repo.AcceptedCAS(old, remote); err != nil {
		t.Fatal(err)
	}
	e.Repository = r
	p, observation, err = goal.FreshProjection(context.Background(), e, func() (time.Time, error) { return b.commandNow(b.root) })
	if err != nil || observation.Outcome != "fresh" || observation.Tip != remote || len(p.Banners) == 0 {
		t.Fatalf("old unchanged commit is not freshly observed: %+v %+v %v", p, observation, err)
	}
}

func TestLedgerFreshSessionAndUpObserveRemote(t *testing.T) {
	t.Parallel()
	for _, verb := range []string{"up", "session"} {
		t.Run(verb, func(t *testing.T) {
			t.Parallel()
			b := newGoalCLIBed(t, goalCLISeed{amend: func(files map[string]*goal.GoalFile) {
				f := files["ship-widget"]
				f.Claimed.Machine, f.Claimed.Lineage = "other-machine", "other-lineage"
				f.StopCapability = &goal.StopCapability{Generation: 2, Revision: 2, Machine: "other-machine", ClaimEpoch: 1}
			}})
			_, remote := freshRemoteEdit(t, b, func(f *goal.GoalFile) {
				f.Claimed.Machine, f.Claimed.Lineage, f.StopCapability.Machine = b.machine, b.lineage, b.machine
			})
			attempts := 0
			inputs := freshUpFixture(t, b, freshDependencies(b, freshCommandRepository{Repository: b.repo, attempts: &attempts}))
			var actual up.Result
			run := inputs.run
			inputs.run = func(options up.Options) up.Result { actual = run(options); return actual }
			var out, diagnostic bytes.Buffer
			var code int
			if verb == "up" {
				code = runUpWithInputs([]string{"--metasystem-root", b.root, "--repo", b.root}, fakeTop(b.root), &out, &diagnostic, inputs)
			} else {
				owners := b.owners(&out, &diagnostic)
				owners.dependencies = inputs.dependencies
				owners.processes = defaultProcessIntentOwners()
				owners.processes.up, owners.processes.executable, owners.processes.process.repositoryTop = inputs.run, inputs.executable, fakeTop(b.root)
				command, args, _ := resolveIntentArgv([]string{"session", "start"})
				code = runIntentIn(command, args, &out, &diagnostic, b.root, owners)
			}
			if code != 0 || attempts != 1 || actual.Adoption == nil || actual.Adoption.Status != "complete" || actual.Adoption.Observation == nil || actual.Adoption.Observation.Tip != remote || !strings.Contains(strings.Join(actual.Lines(), "\n"), "goal=ship-widget epoch=1") {
				t.Fatalf("explicit %s adopted stale claims: exit=%d attempts=%d result=%+v out=%s err=%s", verb, code, attempts, actual, &out, &diagnostic)
			}
		})
	}
}

func TestLedgerFreshNextExpiresApprovalDuringTransport(t *testing.T) {
	t.Parallel()
	b := newGoalCLIBed(t, goalCLISeed{allowTerminalProof: true, amend: func(files map[string]*goal.GoalFile) {
		f := files["ship-widget"]
		f.State, f.Claimed = goal.StateApproved, nil
		f.History[1].Verb, f.History[1].Actor = "approve", "human:Wido"
		f.History[1].AuthorityOutcome, f.History[1].AuthorityReviewBy = goal.AuthorityOutcomeTemporaryHumanWord, "2026-08-20"
		f.Budget = &goal.Budget{ElapsedLimit: "4h", AttemptLimit: 4, ReservedJobMinutesLimit: 240, ActiveJobLimit: 2}
		f.Approved = &goal.ApprovalRecord{By: "human:Wido", At: f.OpenedAt, Revision: 2, Opid: f.History[1].Opid, Authority: goal.ApprovalAuthorityRelayed, ReviewBy: "2026-08-20", Digest: fmt.Sprintf("%x", sha256.Sum256([]byte("intent="+f.Intent+"\nbudget=elapsedLimit=4h attemptLimit=4 reservedJobMinutesLimit=240 activeJobLimit=2\n")))}
	}})
	b.announceHolder()
	attempts := 0
	r := freshCommandRepository{Repository: b.repo, attempts: &attempts, capture: func(_ context.Context, opid string) (string, error) {
		b.setNow(goalCLISeedNow.Add(25 * time.Hour))
		return b.repo.Capture(opid)
	}}
	var out, diagnostic bytes.Buffer
	code := runGoalNextWithInputs([]string{"--root", b.root}, freshDependencies(b, r), b.commandNow, &out, &diagnostic)
	if code != 0 || attempts != 1 || strings.Contains(out.String(), "next ready goal: ship-widget") || !strings.Contains(out.String(), "approval") {
		t.Fatalf("expired approval admitted using pre-fetch time: exit=%d attempts=%d out=%s err=%s", code, attempts, &out, &diagnostic)
	}
}

func TestLedgerFreshNameDoesNotProvePersonAuthority(t *testing.T) {
	t.Parallel()
	b := newGoalCLIBed(t, goalCLISeed{})
	b.announceHolder()
	attempts := 0
	d := freshDependencies(b, freshCommandRepository{Repository: b.repo, attempts: &attempts})
	var out, diagnostic bytes.Buffer
	owners := b.owners(&out, &diagnostic)
	owners.dependencies = d
	owners.prove = func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
		return humanauthority.Proof{}, errors.New("authority unreadable")
	}
	command, args, _ := resolveIntentArgv([]string{"goal", "claim", "ship-widget", "--by", "Wido"})
	code := runIntentIn(command, args, &out, &diagnostic, b.root, owners)
	if code == 0 || attempts != 0 {
		t.Fatalf("a name acquired person authority: exit=%d attempts=%d %s %s", code, attempts, &out, &diagnostic)
	}
}

func TestLedgerFreshSessionDoesNotManufactureAnAncestor(t *testing.T) {
	t.Parallel()
	b := newGoalCLIBed(t, goalCLISeed{})
	attempts := 0
	inputs := freshUpFixture(t, b, freshDependencies(b, freshCommandRepository{Repository: b.repo, attempts: &attempts}))
	inputs.run = func(options up.Options) up.Result {
		options.FindSessionAncestor = func(string, int64, string) (census.AgentAncestor, error) { return census.AgentAncestor{}, nil }
		options.EnsureArmed = func(supervise.EnsureOptions) (supervise.EnsureResult, error) {
			t.Fatal("supervision used a synthetic session")
			return supervise.EnsureResult{}, nil
		}
		return up.Run(options)
	}
	var out, diagnostic bytes.Buffer
	owners := b.owners(&out, &diagnostic)
	owners.dependencies = inputs.dependencies
	owners.processes = defaultProcessIntentOwners()
	owners.processes.up, owners.processes.executable = inputs.run, inputs.executable
	owners.processes.process.repositoryTop = fakeTop(b.root)
	command, args, _ := resolveIntentArgv([]string{"session", "start", "--json"})
	code := runIntentIn(command, args, &out, &diagnostic, b.root, owners)
	if code == 0 || attempts != 0 || !strings.Contains(out.String()+diagnostic.String(), "session-identity") {
		t.Fatalf("absence manufactured a session: exit=%d attempts=%d out=%s err=%s", code, attempts, &out, &diagnostic)
	}
}

func freshUpFixture(t *testing.T, b *goalCLIBed, d syncRequestDependencies) upCommandInputs {
	t.Helper()
	b.announceHolder()
	binary := filepath.Join(b.root, "bin", "metasystem")
	if err := os.MkdirAll(filepath.Dir(binary), 0755); err != nil {
		t.Fatal(err)
	}
	content := []byte("#!/bin/sh\nexit 99\n")
	if err := testexec.WriteFile(binary, content, 0755); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(content)
	if err := steward.MintIdentity(steward.RepoIdentityPath(b.root), steward.InstallIdentity{RepoIdentity: b.root, Generation: 1, InstallPath: binary, InstallDigest: fmt.Sprintf("sha256:%x", digest), MintedAt: b.clock().Format(time.RFC3339)}); err != nil {
		t.Fatal(err)
	}
	exact, state, err := (identity.KernelProber{}).Probe(int64(os.Getpid()))
	if err != nil || state != identity.Alive {
		t.Fatalf("fixture identity: %s %v", state, err)
	}
	return upCommandInputs{dependencies: d, clock: b.commandNow, executable: func() (string, error) { return binary, nil }, run: func(options up.Options) up.Result {
		options.OwnerLineage = b.lineage
		options.FindSessionAncestor = func(string, int64, string) (census.AgentAncestor, error) {
			return census.AgentAncestor{Pid: exact.Pid, PidStartedAt: exact.StartedAt.Unix(), PidStartTicks: exact.StartTicks, BootID: exact.BootID, Runtime: "fake"}, nil
		}
		options.EnsureArmed = func(supervise.EnsureOptions) (supervise.EnsureResult, error) {
			return supervise.EnsureResult{Action: "joined", Generation: 1}, nil
		}
		options.EnsureStewardRunner = func(string, *steward.EnrolledBinary, int) (steward.EnsureRunnerResult, error) {
			return steward.EnsureRunnerResult{Action: "already-running", Generation: 1}, nil
		}
		return up.Run(options)
	}}
}

func TestLedgerFreshSessionAndUpPendingRepairAndOfflineHook(t *testing.T) {
	t.Parallel()
	for _, verb := range []string{"up", "session"} {
		t.Run(verb, func(t *testing.T) {
			t.Parallel()
			b := newGoalCLIBed(t, goalCLISeed{amend: func(files map[string]*goal.GoalFile) {
				files["ship-widget"].StopCapability = &goal.StopCapability{Generation: 2, Revision: 2, Machine: "fixture-machine", ClaimEpoch: 1}
			}})
			attempts := 0
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			cancelled := false
			r := freshCommandRepository{Repository: b.repo, attempts: &attempts, capture: func(c context.Context, opid string) (string, error) {
				b.repo.Capture(opid)
				cancel()
				<-c.Done()
				cancelled = true
				return "", c.Err()
			}}
			r.cleaned = func(c context.Context) {
				if c == nil {
					return
				}
				if c.Err() != nil {
					t.Fatalf("cleanup inherited read cancellation: %v", c.Err())
				}
			}
			d := freshDependencies(b, r)
			d.ctx = ctx
			inputs := freshUpFixture(t, b, d)
			var actual up.Result
			run := inputs.run
			inputs.run = func(options up.Options) up.Result { actual = run(options); return actual }
			invoke := func() (int, string) {
				var out, diagnostic bytes.Buffer
				if verb == "up" {
					code := runUpWithInputs([]string{"--metasystem-root", b.root, "--repo", b.root, "--json"}, fakeTop(b.root), &out, &diagnostic, inputs)
					return code, out.String() + diagnostic.String()
				}
				owners := b.owners(&out, &diagnostic)
				owners.dependencies = inputs.dependencies
				owners.processes = defaultProcessIntentOwners()
				owners.processes.up = inputs.run
				owners.processes.executable = inputs.executable
				owners.processes.process.repositoryTop = fakeTop(b.root)
				command, args, _ := resolveIntentArgv([]string{"session", "start", "--json"})
				code := runIntentIn(command, args, &out, &diagnostic, b.root, owners)
				return code, out.String() + diagnostic.String()
			}
			code, output := invoke()
			remedy := "metasystem up"
			if verb == "session" {
				remedy = "metasystem session start"
			}
			expectedExit := 0
			if verb == "session" {
				expectedExit = 1
			}
			if code != expectedExit || actual.Outcome != "armed" || actual.Adoption == nil || actual.Adoption.Status != "pending" || actual.Adoption.Remedy != remedy || attempts != 1 || !cancelled || !strings.Contains(output, "adoption pending") || !strings.Contains(output, "canceled") || !strings.Contains(output, remedy) {
				t.Fatalf("pending hidden: exit=%d attempts=%d cancelled=%v result=%+v output=%s", code, attempts, cancelled, actual, output)
			}
			envelope := upEnvelope(actual)
			if envelope.Outcome != verbresult.Confirmed {
				t.Fatalf("pending broke up envelope: %+v", envelope)
			}
			if verb == "session" && !strings.Contains(strings.ReplaceAll(output, " ", ""), `"outcome":"partial"`) {
				t.Fatalf("pending session is not partial: %s", output)
			}
			before := b.tip()
			// Hook arming uses the offline reader even while transport is down.
			hook := hookOwners{diagnostics: &bytes.Buffer{}, upInputs: inputs, repositoryTop: fakeTop(b.root)}
			var hookOutput bytes.Buffer
			if code := hook.Up(hooksUpRequest(b.root), &hookOutput, &hookOutput); code != 0 || attempts != 1 {
				t.Fatalf("hook fetched: code=%d attempts=%d output=%s", code, attempts, &hookOutput)
			}
			inputs.dependencies.ctx = context.Background()
			r.capture, r.cleaned = nil, nil
			inputs.dependencies = freshDependencies(b, r)
			for replay := 0; replay < 2; replay++ {
				code, output = invoke()
				if code != 0 || actual.Adoption == nil || actual.Adoption.Status != "complete" || b.tip() != before {
					t.Fatalf("repair/replay changed ownership or accounting: exit=%d result=%+v output=%s", code, actual, output)
				}
			}
		})
	}
}

func TestLedgerFreshPersonNamedPublicationAndAgentRefusal(t *testing.T) {
	t.Parallel()
	b := newGoalCLIBed(t, goalCLISeed{allowTerminalProof: true})
	b.announceHolder()
	attempts, publications := 0, 0
	r := freshCommandRepository{Repository: b.repo, attempts: &attempts, publications: &publications, capture: func(_ context.Context, opid string) (string, error) {
		b.repo.Capture(opid)
		return "", errors.New("transport unavailable")
	}}
	d := freshDependencies(b, r)
	code, output := freshPublic(b, d, "goal", "claim", "ship-widget")
	if code == 0 || attempts != 1 || publications != 0 || !strings.Contains(output, "transport unavailable") || strings.Contains(output, "continue") {
		t.Fatalf("agent selected stale work: exit=%d attempts=%d %s", code, attempts, output)
	}
	agent := d
	agent.proveTerminal = func(string, int64, humanauthority.Reader, time.Time) (humanauthority.Proof, error) {
		return humanauthority.Proof{}, errors.New("agent terminal")
	}
	var agentOut, agentDiagnostic bytes.Buffer
	if code := runGoalNextWithInputs([]string{"--root", b.root}, agent, b.commandNow, &agentOut, &agentDiagnostic); code == 0 || !strings.Contains(agentDiagnostic.String(), "transport unavailable") {
		t.Fatalf("agent next guessed an empty frontier: %d %s %s", code, &agentOut, &agentDiagnostic)
	}
	attempts = 0
	code, output = freshPublic(b, d, "goal", "claim", "ship-widget", "--by", "Wido", "--json")
	var failed verbresult.Result
	if err := json.Unmarshal([]byte(output), &failed); err != nil {
		t.Fatalf("publication failure is not a command result: %v %s", err, output)
	}
	var data struct {
		Claim       json.RawMessage  `json:"claim"`
		Observation goal.Observation `json:"observation"`
	}
	if err := failed.DecodeData(&data); err != nil {
		t.Fatal(err)
	}
	if code == 0 || attempts != 1 || publications != 1 || failed.Outcome != verbresult.Failed || data.Observation.Outcome != "unavailable" || failed.Next == nil || !strings.Contains(strings.Join(failed.Next.Argv, " "), "goal claim ship-widget") || !strings.Contains(output, "during publication") || !strings.Contains(output, "transport unavailable") {
		t.Fatalf("person did not reach safe publication: exit=%d attempts=%d %s", code, attempts, output)
	}
	attempts = 0
	code, output = freshPublic(b, d, "goal", "claim", "--by", "Wido")
	if code == 0 || attempts != 1 || !strings.Contains(output, "name the goal") {
		t.Fatalf("unnamed stale person claim invented work: %d %d %s", code, attempts, output)
	}
	var out, diagnostic bytes.Buffer
	code = runGoalNextWithInputs([]string{"--root", b.root}, d, b.commandNow, &out, &diagnostic)
	if code != 0 || !strings.Contains(out.String(), "stale and non-authoritative") {
		t.Fatalf("person stale next: %d %s %s", code, &out, &diagnostic)
	}
	r.capture = nil
	d = freshDependencies(b, r)
	code, output = freshPublic(b, d, failed.Next.Argv[1:]...)
	if code != 0 {
		t.Fatalf("repaired named command failed: %d %s", code, output)
	}
}

func TestLedgerFreshLocalAndInvalidLedger(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"local", "foreign", "corrupt", "cleanup", "unchanged channel"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			seed := goalCLISeed{}
			if kind == "local" {
				seed.remote = "local"
			}
			b := newGoalCLIBed(t, seed)
			attempts := 0
			r := freshCommandRepository{Repository: b.repo, attempts: &attempts}
			if kind == "cleanup" {
				r.cleanupErr = errors.New("cleanup denied")
			}
			if kind == "foreign" || kind == "corrupt" {
				parent := b.tip()
				files, _ := b.repo.Files(parent, "plans/goals/backlog.md")
				data := files["plans/goals/backlog.md"]
				if kind == "foreign" {
					data = bytes.ReplaceAll(data, []byte("01ARZ3NDEKTSV4RRFFQ69G5FAV"), []byte("01ARZ3NDEKTSV4RRFFQ69G5FAX"))
				} else {
					data = []byte("corrupt ledger")
				}
				b.repo.Capture("invalid-fixture")
				next, err := b.repo.Build("invalid-fixture", parent, []goal.Change{{Path: "plans/goals/backlog.md", Content: data}}, "invalid fixture")
				if err != nil {
					t.Fatal(err)
				}
				b.repo.Publish(parent, next)
			}
			if kind == "unchanged channel" {
				// Whole-tree validity also covers the committed channel when
				// transport observes the same already-accepted ledger tip.
				parent := b.tip()
				if _, err := b.repo.Capture("invalid-channel"); err != nil {
					t.Fatal(err)
				}
				next, err := b.repo.Build("invalid-channel", parent, []goal.Change{{Path: "plans/channel/questions/01J5X0000000000000000000Q1.json", Content: []byte("{broken")}}, "invalid channel")
				if err != nil {
					t.Fatal(err)
				}
				if outcome, err := b.repo.Publish(parent, next); err != nil || outcome != goal.CASLanded {
					t.Fatalf("publish invalid channel fixture: %s %v", outcome, err)
				}
				if err := b.repo.AcceptedCAS(parent, next); err != nil {
					t.Fatal(err)
				}
			}
			e, _ := freshDependencies(b, r).endpoint(b.root)
			p, observation, err := goal.FreshProjection(context.Background(), e, func() (time.Time, error) { return b.commandNow(b.root) })
			if attempts != 1 {
				t.Fatalf("attempts=%d", attempts)
			}
			if kind == "local" {
				if err != nil || observation.Outcome != "fresh" || len(p.Banners) == 0 {
					t.Fatalf("local: %+v %+v %v", p, observation, err)
				}
			} else if err == nil || observation.Outcome != "unavailable" || observation.Cause == "" || p.Tree != nil {
				t.Fatalf("invalid read returned fresh: %+v %+v %v", p, observation, err)
			}
			if kind == "unchanged channel" && !strings.Contains(observation.Cause, "channel-json") {
				t.Fatalf("unchanged tip lost whole-tree validation: %+v", observation)
			}
			if kind == "cleanup" && !strings.Contains(err.Error(), "refs/metasystem/goals/fetch/") {
				t.Fatalf("cleanup failure lost temporary ref: %v", err)
			}
			b.announceHolder()
			d := freshDependencies(b, r)
			d.proveTerminal = func(string, int64, humanauthority.Reader, time.Time) (humanauthority.Proof, error) {
				return humanauthority.Proof{}, errors.New("agent terminal")
			}
			var out, diagnostic bytes.Buffer
			code := runGoalNextWithInputs([]string{"--root", b.root}, d, b.commandNow, &out, &diagnostic)
			if attempts != 2 {
				t.Fatalf("public command preflights=%d", attempts-1)
			}
			cause := ""
			if err != nil {
				cause = err.Error()
			}
			if kind == "cleanup" {
				cause = "cleanup denied"
			}
			if kind == "local" {
				if code != 0 || !strings.Contains(out.String(), "local") {
					t.Fatalf("public local command: %d %s %s", code, &out, &diagnostic)
				}
			} else if code == 0 || !strings.Contains(diagnostic.String(), cause) || !strings.Contains(diagnostic.String(), "metasystem internal goal next") {
				t.Fatalf("public invalid command lost cause/remedy: %d %s %s", code, &out, &diagnostic)
			}
		})
	}
}

// Keep imports and request construction shared with the hook's real owner.
func hooksUpRequest(root string) hooks.UpRequest {
	return hooks.UpRequest{MetasystemRoot: stateroot.Installation(root), Repo: root, Session: "fresh-hook"}
}

func TestLedgerFreshPersonKeepsUnreadableAdvisoryInputs(t *testing.T) {
	t.Parallel()
	b := newGoalCLIBed(t, goalCLISeed{allowTerminalProof: true, amend: func(files map[string]*goal.GoalFile) { files["ship-widget"].Claimed.Machine = "foreign-machine" }})
	attempts, presenceReads := 0, 0
	r := freshCommandRepository{Repository: b.repo, attempts: &attempts, capture: func(context.Context, string) (string, error) { return "", errors.New("transport unavailable") }}
	d := freshDependencies(b, r)
	d.presence = func(string, goal.Endpoint) (seat.Copy, error) {
		presenceReads++
		return seat.Copy{}, errors.New("presence unreadable")
	}
	var out, diagnostic bytes.Buffer
	if code := runGoalNextWithInputs([]string{"--root", b.root}, d, b.commandNow, &out, &diagnostic); code != 0 || presenceReads != 1 || !strings.Contains(out.String(), "stale and non-authoritative") {
		t.Fatalf("advice refused a person: %d presence=%d %s %s", code, presenceReads, &out, &diagnostic)
	}
	// Queue and design reads are advisory to the normal publication transaction.
	b2 := newGoalCLIBed(t, goalCLISeed{})
	b2.announceHolder()
	attempts, publications := 0, 0
	r2 := freshCommandRepository{Repository: b2.repo, attempts: &attempts, publications: &publications, capture: func(context.Context, string) (string, error) { return "", errors.New("transport unavailable") }}
	d2 := freshDependencies(b2, r2)
	out.Reset()
	diagnostic.Reset()
	owners := b2.owners(&out, &diagnostic)
	owners.dependencies = d2
	owners.delivery = defaultIntentDeliveryOwners()
	owners.delivery.laneRoot = func(string, time.Time) (string, bool, error) { return "", false, errors.New("queue unreadable") }
	command, args, _ := resolveIntentArgv([]string{"goal", "claim", "ship-widget", "--by", "Wido"})
	if code := runIntentIn(command, args, &out, &diagnostic, b2.root, owners); code == 0 || publications != 1 || !strings.Contains(out.String()+diagnostic.String(), "during publication") {
		t.Fatalf("advice vetoed publication: %d captures=%d %s %s", code, publications, &out, &diagnostic)
	}
}
