package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/census"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/ownercall"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	seatlaunch "github.com/widoriezebos/agentic-tools/metasystem/internal/seat/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stopfence"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/supervise"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/up"
)

// The command, session lifecycle, adoption and claim transaction are real.
// Only the immutable Git transport and external process effects are supplied.
type claimSessionBed struct {
	*goalCLIBed
	owners                 intentOwners
	preparations, attempts int
	prepared               bool
	capture                func(context.Context, string) (string, error)
	afterStart             func()
}

func newClaimSessionBed(t *testing.T) *claimSessionBed {
	t.Helper()
	return newClaimSessionBedAt(t, "")
}

func newClaimSessionBedAt(t *testing.T, checkout string) *claimSessionBed {
	t.Helper()
	b := newGoalCLIBed(t, goalCLISeed{checkout: checkout, allowTerminalProof: true, amend: func(files map[string]*goal.GoalFile) {
		f := files["ship-widget"]
		f.State, f.Claimed, f.StopCapability = goal.StateQueued, nil, nil
		third := *files["fix-docs"]
		third.Id = "z-next"
		third.History = append([]goal.HistoryLine(nil), third.History...)
		for i := range third.History {
			third.History[i].Targets = []string{third.Id}
			third.History[i].Opid = goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FAY", "fixture-machine", "fixture-lineage")
		}
		files[third.Id] = &third
		for _, id := range []string{"ship-widget", "fix-docs", "z-next"} {
			files[id].Arc = "claim-arc"
			files[id].Tier = 1
			files[id].Risk = &goal.RiskRecord{Severity: 1, Novelty: 1, Exposure: 1, Accumulation: 1, Basis: "fixture risk"}
		}
	}})
	b.lineage = launch.SeatOwnerLineage
	gcliForgivingMust(t, b, "goal", "approve", "ship-widget", "--budget", "norm", "--by", "Wido", gcliForgivingFixture)
	gcliForgivingMust(t, b, "goal", "approve", "fix-docs", "--budget", "norm", "--by", "Wido", gcliForgivingFixture)
	gcliForgivingMust(t, b, "goal", "approve", "z-next", "--budget", "norm", "--by", "Wido", gcliForgivingFixture)
	inputs := freshUpFixture(t, b, b.dependencies(&bytes.Buffer{}, &bytes.Buffer{}))
	caller := ownercall.CurrentProcess()
	if err := lease.Retire(b.root, "goal-cli-fixture", caller.Pid, caller.StartedAt); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(b.root, "artifacts", "agents", "mains", "worktree-lease.json")); err != nil {
		t.Fatal(err)
	}
	bed := &claimSessionBed{goalCLIBed: b}
	bed.owners = b.owners(&bytes.Buffer{}, &bytes.Buffer{})
	endpoint := bed.owners.dependencies.endpoint
	bed.owners.dependencies.endpoint = func(root string) (goal.Endpoint, error) {
		e, err := endpoint(root)
		e.Repository = freshCommandRepository{Repository: e.Repository, attempts: &bed.attempts, capture: func(ctx context.Context, opid string) (string, error) {
			if bed.capture != nil {
				return bed.capture(ctx, opid)
			}
			return b.repo.Capture(opid)
		}}
		return e, err
	}
	bed.owners.processes = defaultProcessIntentOwners()
	bed.owners.processes.process.repositoryTop = fakeTop(b.root)
	bed.owners.processes.executable = inputs.executable
	exact, state, err := (identity.KernelProber{}).Probe(caller.Pid)
	if err != nil || state != identity.Alive {
		t.Fatalf("original caller: %s %v", state, err)
	}
	bed.owners.processes.up = func(options up.Options) up.Result {
		bed.preparations++
		if options.CallerPid != caller.Pid || options.OwnerLineage != b.lineage {
			t.Fatalf("preparation replaced original caller/lineage: %+v", options)
		}
		options.FindSessionAncestor = func(_ string, pid int64, _ string) (census.AgentAncestor, error) {
			if pid != caller.Pid {
				t.Fatalf("session inference replaced caller %d with %d", caller.Pid, pid)
			}
			return census.AgentAncestor{Pid: exact.Pid, PidStartedAt: exact.StartedAt.Unix(), PidStartTicks: exact.StartTicks, BootID: exact.BootID, Runtime: "fake"}, nil
		}
		options.EnsureArmed = func(supervise.EnsureOptions) (supervise.EnsureResult, error) {
			return supervise.EnsureResult{Action: "joined", Generation: 1}, nil
		}
		options.EnsureStewardRunner = func(string, *steward.EnrolledBinary) (steward.EnsureRunnerResult, error) {
			return steward.EnsureRunnerResult{Action: "already-running", Generation: 1}, nil
		}
		result := up.Run(options)
		bed.prepared = result.Outcome == "armed"
		if bed.afterStart != nil {
			bed.afterStart()
		}
		return result
	}
	return bed
}

func (b *claimSessionBed) run(args ...string) (int, intentResult) {
	b.t.Helper()
	var out, diagnostic bytes.Buffer
	command, rest, ok := resolveIntentArgv(append(args, "--json"))
	if !ok {
		b.t.Fatalf("unknown command %v", args)
	}
	code := runIntentIn(command, rest, &out, &diagnostic, b.root, b.owners)
	var result intentResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		b.t.Fatalf("command result: %v out=%s err=%s", err, &out, &diagnostic)
	}
	return code, result
}

func TestClaimSessionPreparesOriginalCallerThenReadsFresh(t *testing.T) {
	t.Parallel()
	b := newClaimSessionBed(t)
	var decisionTip string
	b.capture = func(ctx context.Context, opid string) (string, error) {
		if ctx != nil && b.prepared {
			c, err := lease.ClassifyVerb(b.root, b.caller.Pid)
			if err != nil || !c.Holder || c.ClaimEpoch == nil || *c.ClaimEpoch <= 0 {
				t.Fatalf("fresh decision precedes authenticated session: %+v %v", c, err)
			}
			decisionTip = b.tip()
		}
		return b.repo.Capture(opid)
	}
	code, result := b.run("goal", "claim", "ship-widget")
	f := gcliForgivingParse(t, b.goalRecord("ship-widget"))
	c, err := lease.ClassifyVerb(b.root, b.caller.Pid)
	data := resultData(t, result)
	observation := data["observation"].(map[string]any)
	if code != 0 || result.Outcome != intentConfirmed || b.preparations != 1 || b.attempts != 2 || decisionTip == "" || observation["tip"] != decisionTip || observation["outcome"] != "fresh" || err != nil || !c.Holder || c.ClaimEpoch == nil || f.StopCapability == nil || f.StopCapability.ClaimEpoch != *c.ClaimEpoch || f.Claimed.Lineage != b.lineage {
		t.Fatalf("claim did not prepare then freshly publish: exit=%d starts=%d reads=%d decision=%s result=%+v holder=%+v file=%+v err=%v", code, b.preparations, b.attempts, decisionTip, result, c, f, err)
	}
	before := b.goalRecord("ship-widget")
	b.owners.prove = func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
		t.Fatal("an authenticated agent with its session lineage was treated as a person")
		return humanauthority.Proof{}, nil
	}
	code, result = b.run("goal", "claim", "ship-widget")
	if code != 0 || b.preparations != 1 || b.attempts != 3 || b.goalRecord("ship-widget") != before {
		t.Fatalf("owning session was prepared again: exit=%d starts=%d reads=%d result=%+v", code, b.preparations, b.attempts, result)
	}
}

func TestClaimSessionWithoutLineageRequiresPersonProof(t *testing.T) {
	t.Parallel()
	for _, holder := range []bool{false, true} {
		for _, named := range []bool{false, true} {
			t.Run(fmt.Sprintf("holder=%t named=%t", holder, named), func(t *testing.T) {
				t.Parallel()
				b := newClaimSessionBed(t)
				if holder {
					b.announceHolder()
				}
				b.lineage = ""
				proved := false
				b.owners.prove = func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
					proved = true
					return humanauthority.Proof{Outcome: humanauthority.OutcomeTerminalMissing}, errors.New("no terminal a person typed at")
				}
				before := b.tip()
				args := []string{"goal", "claim"}
				if named {
					args = append(args, "ship-widget")
				}
				code, result := b.run(args...)
				if code == 0 || !proved || result.Outcome != intentRefused || b.preparations != 0 || b.attempts != 0 || b.tip() != before {
					t.Fatalf("a caller without lineage bypassed person proof: exit=%d proved=%t starts=%d reads=%d result=%+v", code, proved, b.preparations, b.attempts, result)
				}
			})
		}
	}
}

func TestClaimSessionDelegateRefusedBeforePreparation(t *testing.T) {
	t.Parallel()
	for _, engine := range []bool{false, true} {
		t.Run(fmt.Sprintf("engine=%t", engine), func(t *testing.T) {
			t.Parallel()
			b := newClaimSessionBed(t)
			if code, result := b.run("session", "start"); code != 0 {
				t.Fatalf("holding session preparation: %d %+v", code, result)
			}
			b.attempts = 0
			holder, err := lease.CurrentHolder(b.root)
			if err != nil {
				t.Fatal(err)
			}
			exact, state, err := (identity.KernelProber{}).Probe(agentChildPid(t))
			if err != nil || state != identity.Alive {
				t.Fatalf("delegate process: %s %v", state, err)
			}
			b.owners.dependencies.authorityFacts.caller = ownercall.Process{Pid: exact.Pid, StartedAt: exact.StartedAt.Unix()}
			classification, err := lease.ClassifyVerb(b.root, exact.Pid)
			if err != nil || classification.Class != lease.ClassDelegate || classification.Holder {
				t.Fatalf("delegate classification: %+v %v", classification, err)
			}
			if !engine {
				if err := os.Remove(filepath.Join(b.root, "bin", "metasystem")); err != nil {
					t.Fatal(err)
				}
			}
			b.owners.processes.executable = func() (string, error) {
				t.Fatal("delegate claim read the engine path before refusing authority")
				return "", nil
			}
			before := b.tip()
			code, result := b.run("goal", "claim", "ship-widget")
			if code == 0 || result.Outcome != intentRefused || !strings.Contains(result.Summary, "authority") || b.preparations != 1 || b.attempts != 0 || b.tip() != before {
				t.Fatalf("delegate reached session preparation: exit=%d starts=%d reads=%d result=%+v", code, b.preparations, b.attempts, result)
			}
			if result.Next != nil || !strings.Contains(result.Decision, "the session that holds this checkout") || !strings.Contains(result.Decision, holder.MainId) || strings.Contains(resultWords(result), "metasystem session start") {
				t.Fatalf("delegate remedy invites preparation or omits the holding session: %+v", result)
			}
			// The remedy moves the act to the existing holder, retaining the
			// delegate's refusal and reusing the holder's authenticated session.
			b.owners.dependencies.authorityFacts.caller = b.caller
			if code, result = b.run("goal", "claim", "ship-widget"); code != 0 || b.preparations != 1 || b.attempts != 1 || gcliForgivingParse(t, b.goalRecord("ship-widget")).State != goal.StateClaimed {
				t.Fatalf("claim from the holding session loops or fails: %d %+v", code, result)
			}
		})
	}
}

func TestClaimSessionSeesOwnershipMovedDuringPreparation(t *testing.T) {
	t.Parallel()
	for _, named := range []bool{true, false} {
		t.Run(map[bool]string{true: "named seat", false: "automatic"}[named], func(t *testing.T) {
			t.Parallel()
			b := newClaimSessionBed(t)
			var foreign string
			b.afterStart = func() {
				_, foreign = freshRemoteEdit(t, b.goalCLIBed, func(f *goal.GoalFile) {
					f.State = goal.StateClaimed
					f.Claimed = &goal.ClaimRecord{Machine: "other-machine", Lineage: "other-lineage", At: b.clock().Format("2006-01-02T15:04:05Z07:00"), Revision: f.Revision}
				})
			}
			args := []string{"goal", "claim"}
			if named {
				args = append(args, "ship-widget")
			}
			code, result := b.run(args...)
			a := gcliForgivingParse(t, b.goalRecord("ship-widget"))
			next := gcliForgivingParse(t, b.goalRecord("fix-docs"))
			if code != 0 || b.preparations != 1 || b.attempts != 2 || foreign == "" || a.Claimed.Machine != "other-machine" || next.State != goal.StateClaimed || next.Claimed.Machine != b.machine {
				t.Fatalf("post-start decision lost foreign owner or lawful alternative: exit=%d result=%+v first=%+v next=%+v", code, result, a, next)
			}
		})
	}
}

func TestClaimSessionReadFailureKeepsAdoptionsAndRetryReusesHolder(t *testing.T) {
	t.Parallel()
	for _, format := range []string{"JSON", "text"} {
		t.Run(format, func(t *testing.T) {
			t.Parallel()
			b := newClaimSessionBed(t)
			// Two ordinary personal reservations: one matches, the other requires a
			// fresh session under its different explicit lineage.
			b.lineage = "matching-lineage"
			for _, id := range []string{"ship-widget", "fix-docs"} {
				lineage := b.lineage
				if id == "fix-docs" {
					lineage = "other-lineage"
				}
				b.owners.dependencies.claimHolder.Holder = func(string) (lease.CurrentHolderView, error) {
					return lease.CurrentHolderView{MainId: "dead-main", OwnerLineage: lineage}, nil
				}
				b.owners.dependencies.claimHolder.Caller = func(string, int64) (lease.ClassifyResult, error) {
					return lease.ClassifyResult{Class: lease.ClassUntrusted}, nil
				}
				if code, r := b.run("goal", "claim", id, "--by", "Wido", gcliForgivingFixture); code != 0 {
					t.Fatalf("reserve: %d %+v", code, r)
				}
			}
			b.owners.dependencies.claimHolder = claimHolderReaders{}
			reserved := gcliForgivingParse(t, b.goalRecord("ship-widget"))
			b.attempts = 0
			fail := true
			b.capture = func(ctx context.Context, opid string) (string, error) {
				if ctx != nil && b.prepared && fail {
					return "", errors.New("transport unavailable")
				}
				return b.repo.Capture(opid)
			}
			if format == "text" {
				var out, diagnostic bytes.Buffer
				command, rest, ok := resolveIntentArgv([]string{"goal", "claim", "ship-widget"})
				if !ok {
					t.Fatal("claim public parser missing")
				}
				code := runIntentIn(command, rest, &out, &diagnostic, b.root, b.owners)
				text := out.String() + diagnostic.String()
				if code == 0 || b.preparations != 1 || b.attempts != 2 || !strings.Contains(text, "ship-widget: adoption restamped") || !strings.Contains(text, "fix-docs: adoption pending") || !strings.Contains(text, "transport unavailable") || !strings.Contains(text, "session preparation") {
					t.Fatalf("ordinary text lost preparation facts: exit=%d text=%s", code, text)
				}
				return
			}
			code, result := b.run("goal", "claim", "ship-widget")
			adopted := gcliForgivingParse(t, b.goalRecord("ship-widget"))
			data := resultData(t, result)
			adoption, ok := data["adoption"].(map[string]any)
			if !ok {
				t.Fatalf("adoption missing: exit=%d result=%+v", code, result)
			}
			goals, ok := adoption["goals"].([]any)
			if !ok {
				t.Fatalf("adoption omitted goals: exit=%d starts=%d reads=%d result=%+v data=%+v", code, b.preparations, b.attempts, result, data)
			}
			if code == 0 || b.preparations != 1 || b.attempts != 2 || !strings.Contains(result.Summary, "transport unavailable") || adoption["pending"] != true || len(goals) != 2 || !reflect.DeepEqual(reserved.Claimed, adopted.Claimed) || !reflect.DeepEqual(reserved.Approved, adopted.Approved) || !reflect.DeepEqual(reserved.Budget, adopted.Budget) || adopted.StopCapability.ClaimEpoch <= 0 {
				t.Fatalf("failed claim erased preparation/adoptions: exit=%d result=%+v before=%+v after=%+v", code, result, reserved, adopted)
			}
			seen := map[string]string{}
			for _, raw := range goals {
				g := raw.(map[string]any)
				seen[g["goal"].(string)] = g["outcome"].(string)
			}
			if seen["ship-widget"] != "restamped" || seen["fix-docs"] != "pending" {
				t.Fatalf("confirmed/pending adoption lost: %v", seen)
			}
			fail = false
			code, result = b.run("goal", "claim", "ship-widget")
			if code != 0 || b.preparations != 1 || b.attempts != 3 || b.goalRecord("ship-widget") != string(goal.RenderFile(adopted)) {
				t.Fatalf("retry restarted or changed adopted binding: %d %+v", code, result)
			}
		})
	}
}

func TestClaimSessionPreparationFailureAndPersonBypass(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"engine path", "engine changed", "stopped", "identity", "preparation", "ledger", "person"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			b := newClaimSessionBed(t)
			original := b.tip()
			args := []string{"goal", "claim", "ship-widget"}
			switch mode {
			case "engine path":
				b.owners.processes.executable = func() (string, error) { return "", errors.New("engine path unreadable") }
			case "engine changed":
				binary, _ := b.owners.processes.executable()
				// Keeping enrollment's digest proves the real owner rejects new bytes.
				if err := testexec.WriteFile(binary, []byte("#!/bin/sh\nexit 98\n"), 0755); err != nil {
					t.Fatal(err)
				}
			case "stopped":
				if err := stopfence.Write(b.root, stopfence.Record{SchemaVersion: 1, State: stopfence.StateClosed, Phase: stopfence.PhaseStopped, Generation: 1, ChangedAt: b.clock().Format("2006-01-02T15:04:05Z07:00"), Checkout: b.root}); err != nil {
					t.Fatal(err)
				}
			case "preparation":
				b.owners.processes.up = func(options up.Options) up.Result {
					b.preparations++
					options.FindSessionAncestor = func(string, int64, string) (census.AgentAncestor, error) {
						return census.AgentAncestor{}, errors.New("runtime ancestry unreadable")
					}
					options.EnsureArmed = func(supervise.EnsureOptions) (supervise.EnsureResult, error) {
						t.Fatal("failed identity started supervision")
						return supervise.EnsureResult{}, nil
					}
					return up.Run(options)
				}
			case "ledger":
				b.capture = func(context.Context, string) (string, error) { return "", errors.New("ledger transport unavailable") }
			case "identity":
				b.owners.dependencies.authorityFacts.caller = ownercall.Process{Pid: b.caller.Pid, StartedAt: b.caller.StartedAt + 1}
			case "person":
				args = append(args, "--by", "Wido", gcliForgivingFixture)
			}
			code, result := b.run(args...)
			if mode == "person" {
				f := gcliForgivingParse(t, b.goalRecord("ship-widget"))
				if code != 0 || b.preparations != 0 || f.StopCapability.ClaimEpoch != 0 || f.Claimed.By != "human:Wido" {
					t.Fatalf("person prepared an agent: %d %+v file=%+v", code, result, f)
				}
				return
			}
			wantReads := 0
			if mode == "ledger" {
				wantReads = 2
			}
			if code == 0 || b.tip() != original || b.attempts != wantReads || result.Outcome != intentRefused && result.Outcome != intentFailed {
				t.Fatalf("failed preparation claimed: exit=%d starts=%d reads=%d result=%+v", code, b.preparations, b.attempts, result)
			}
			if mode == "stopped" && !strings.Contains(result.Decision, "metasystem system start") {
				t.Fatalf("stop's clearing act lost: %+v", result)
			}
			if mode == "preparation" && !strings.Contains(strings.Join(result.Details, "\n"), "runtime ancestry unreadable") {
				t.Fatalf("underlying preparation cause lost: %+v", result)
			}
			if mode == "engine changed" || mode == "stopped" {
				if !strings.Contains(result.Decision, "metasystem system start") {
					t.Fatalf("preparation's clearing command missing: %+v", result)
				}
				process := b.owners.processes.process
				// The terminal and process effects are supplied; system start
				// still opens the real fence and enrolls the real engine bytes.
				b.owners.processes.process = supBOwners(supBProber{10: identity.Alive}, nil)
				if mode == "engine changed" {
					b.owners.processes.process.armSteps = func(scope processScope, _ int, _ processArmAuthority) (processArmResult, error) {
						content, err := os.ReadFile(scope.Binary)
						if err != nil {
							return processArmResult{}, err
						}
						digest := sha256.Sum256(content)
						err = steward.MintIdentity(steward.RepoIdentityPath(b.root), steward.InstallIdentity{
							RepoIdentity: b.root, Generation: 2, InstallPath: scope.Binary, InstallDigest: fmt.Sprintf("sha256:%x", digest), MintedAt: b.clock().Format(time.RFC3339),
						})
						return processArmResult{}, err
					}
				}
				if code, result = b.run("system", "start"); code != 0 {
					t.Fatalf("printed repair command failed: %d %+v", code, result)
				}
				b.owners.processes.process = process
				if code, result = b.run(args...); code != 0 || b.preparations != 2 {
					t.Fatalf("claim after repair failed: %d %+v starts=%d", code, result, b.preparations)
				}
			}
		})
	}
}

func TestClaimSessionReclassifiesAfterPreparation(t *testing.T) {
	t.Parallel()
	b := newClaimSessionBed(t)
	before := b.tip()
	b.afterStart = func() {
		for _, a := range lease.AnnouncementsFor(b.root, b.caller.Pid) {
			if err := lease.Retire(b.root, a.SessionId, a.Pid, a.PidStartedAt); err != nil {
				t.Fatal(err)
			}
		}
	}
	code, result := b.run("goal", "claim", "ship-widget")
	if code == 0 || b.preparations != 1 || b.attempts != 1 || b.tip() != before || !strings.Contains(result.Summary, "original caller") || resultData(t, result)["preparation"] == nil {
		t.Fatalf("preparation result lent writing authority to a vanished session: %d %+v", code, result)
	}
	b.afterStart = nil
	if result.Next == nil || !reflect.DeepEqual(result.Next.Argv, []string{"metasystem", "session", "start"}) {
		t.Fatalf("authority remedy missing: %+v", result)
	}
	if code, result = b.run(result.Next.Argv[1:]...); code != 0 {
		t.Fatalf("session remedy: %d %+v", code, result)
	}
	if code, result = b.run("goal", "claim", "ship-widget"); code != 0 || b.preparations != 2 {
		t.Fatalf("retry did not reuse repaired session: %d %+v starts=%d", code, result, b.preparations)
	}
}

func TestClaimSessionForeignWriterKeepsOwnerAndNamesIsolation(t *testing.T) {
	t.Parallel()
	b := newClaimSessionBed(t)
	held := testutil.StartHeldProcess(t, exec.Command("/bin/sh", "-c", "printf x >&3; IFS= read -r _ || :"))
	exact, state, err := (identity.KernelProber{}).Probe(int64(held.Command.Process.Pid))
	if err != nil || state != identity.Alive {
		t.Fatalf("foreign session identity: %s %v", state, err)
	}
	if _, err := lease.AnnounceWithPair(b.root, "foreign-session", exact.Pid, exact.StartedAt.Unix(), exact.StartTicks, exact.BootID, "foreign-session", "fake", "foreign-lineage"); err != nil {
		t.Fatal(err)
	}
	beforeHolder, err := lease.CurrentHolder(b.root)
	if err != nil {
		t.Fatal(err)
	}
	before := b.tip()
	code, result := b.run("goal", "claim", "ship-widget")
	afterHolder, err := lease.CurrentHolder(b.root)
	if code == 0 || err != nil || !reflect.DeepEqual(beforeHolder, afterHolder) || b.tip() != before || b.preparations != 1 || b.attempts != 0 || result.Next == nil || !reflect.DeepEqual(result.Next.Argv, []string{"metasystem", "session", "isolate"}) {
		t.Fatalf("advisor claimed or displaced foreign holder: %d %+v before=%+v after=%+v reads=%d err=%v", code, result, beforeHolder, afterHolder, b.attempts, err)
	}
	var isolated *claimSessionBed
	var out, diagnostic bytes.Buffer
	command, rest, ok := resolveIntentArgv(append(result.Next.Argv[1:], "--root", b.root))
	if !ok || command.passthrough == nil {
		t.Fatalf("isolation remedy is not a public command: %+v", result.Next)
	}
	code = runPassthrough(command, func(args []string, stdout, stderr io.Writer) int {
		return runSessionIsolateWith(args, stdout, stderr, func(options *seatlaunch.SecondSessionOptions) {
			options.Now = b.clock
			options.Token = func() (string, error) { return "abcd", nil }
			options.Git = func(args ...string) (string, error) {
				if reflect.DeepEqual(args, []string{"-C", b.root, "rev-parse", "--show-toplevel"}) {
					return b.root + "\n", nil
				}
				if len(args) == 9 && args[2] == "worktree" && args[3] == "add" {
					isolated = newClaimSessionBedAt(t, args[7])
					isolated.repo = b.repo
					return "", nil
				}
				return "", fmt.Errorf("unexpected isolation Git request: %v", args)
			}
			options.ArmSupervision = func(root string, args []string) error {
				if isolated == nil || root != isolated.root {
					return fmt.Errorf("isolation did not create its destination: %s", root)
				}
				if code, result := isolated.run("session", "start"); code != 0 {
					return fmt.Errorf("isolated session preparation: %d %+v", code, result)
				}
				return nil
			}
		})
	}, rest, &out, &diagnostic)
	if code != 0 || isolated == nil || !strings.Contains(out.String(), isolated.root) {
		t.Fatalf("printed isolation remedy failed: %d out=%s err=%s", code, &out, &diagnostic)
	}
	if code, result = isolated.run("goal", "claim", "ship-widget"); code != 0 || isolated.preparations != 1 {
		t.Fatalf("claim in isolated writer checkout: %d %+v", code, result)
	}
	if current, err := lease.CurrentHolder(b.root); err != nil || !reflect.DeepEqual(current, beforeHolder) {
		t.Fatalf("isolation changed original writer: %+v %v", current, err)
	}
}

func TestClaimSessionPublicationFailureKeepsAdoptionOutcomes(t *testing.T) {
	t.Parallel()
	b := newClaimSessionBed(t)
	for _, id := range []string{"ship-widget", "fix-docs"} {
		lineage := b.lineage
		if id == "fix-docs" {
			lineage = "different-lineage"
		}
		b.owners.dependencies.claimHolder.Holder = func(string) (lease.CurrentHolderView, error) {
			return lease.CurrentHolderView{MainId: "dead-main", OwnerLineage: lineage}, nil
		}
		b.owners.dependencies.claimHolder.Caller = func(string, int64) (lease.ClassifyResult, error) {
			return lease.ClassifyResult{Class: lease.ClassUntrusted}, nil
		}
		if code, result := b.run("goal", "claim", id, "--by", "Wido", gcliForgivingFixture); code != 0 {
			t.Fatalf("reservation: %d %+v", code, result)
		}
	}
	b.owners.dependencies.claimHolder = claimHolderReaders{}
	reserved := gcliForgivingParse(t, b.goalRecord("ship-widget"))
	endpoint := b.owners.dependencies.endpoint
	attempted := false
	b.owners.dependencies.endpoint = func(root string) (goal.Endpoint, error) {
		e, err := endpoint(root)
		repository := e.Repository.(freshCommandRepository)
		repository.Repository = personClaimTransport{Repository: repository.Repository, publish: func(parent, commit string) (goal.CASOutcome, error) {
			if b.prepared {
				attempted = true
				return goal.CASUnknown, errors.New("claim publication transport unavailable")
			}
			return b.repo.Publish(parent, commit)
		}}
		e.Repository = repository
		return e, err
	}
	code, result := b.run("goal", "claim", "z-next")
	adopted := gcliForgivingParse(t, b.goalRecord("ship-widget"))
	unclaimed := gcliForgivingParse(t, b.goalRecord("z-next"))
	encoded, _ := json.Marshal(result)
	adoption, ok := resultData(t, result)["adoption"].(map[string]any)
	if code == 0 || !attempted || !ok || adoption["pending"] != true || len(adoption["goals"].([]any)) != 2 || !strings.Contains(string(encoded), "claim publication transport unavailable") || adopted.StopCapability.ClaimEpoch <= 0 || !reflect.DeepEqual(adopted.Claimed, reserved.Claimed) || !reflect.DeepEqual(adopted.Approved, reserved.Approved) || !reflect.DeepEqual(adopted.Budget, reserved.Budget) || unclaimed.State != goal.StateApproved || unclaimed.Claimed != nil {
		t.Fatalf("publication failure erased earlier outcomes: %d %+v attempted=%v adopted=%+v", code, result, attempted, adopted)
	}
}

func TestClaimSessionBuildProceedsWithPendingAdoption(t *testing.T) {
	t.Parallel()
	b := newClaimSessionBed(t)
	// A personal reservation belongs to another explicit lineage; starting
	// this agent's session cannot adopt it or change its ownership.
	b.owners.dependencies.claimHolder.Holder = func(string) (lease.CurrentHolderView, error) {
		return lease.CurrentHolderView{MainId: "dead-main", OwnerLineage: "other-lineage"}, nil
	}
	b.owners.dependencies.claimHolder.Caller = func(string, int64) (lease.ClassifyResult, error) {
		return lease.ClassifyResult{Class: lease.ClassUntrusted}, nil
	}
	if code, result := b.run("goal", "claim", "fix-docs", "--by", "Wido", gcliForgivingFixture); code != 0 {
		t.Fatalf("personal reservation: %d %+v", code, result)
	}
	b.owners.dependencies.claimHolder = claimHolderReaders{}
	reserved := b.goalRecord("fix-docs")
	// This approved goal uses a tier with a review budget.
	freshRemoteEdit(t, b.goalCLIBed, func(file *goal.GoalFile) {
		file.Tier = 2
		file.Risk.Severity, file.Risk.Novelty = 2, 2
		file.Approved.Digest = goal.ApprovalDigest(file.Intent, file.Tier, *file.Budget, file.Risk)
	})
	// The real approval owner supplies the review budget needed by build.
	if code, result := b.run("goal", "approve", "ship-widget", "--elapsed-limit", "4h", "--attempt-limit", "4", "--reserved-job-minutes-limit", "240", "--active-job-limit", "1", "--review-round-limit", "2", "--by", "Wido", gcliForgivingFixture); code != 0 {
		t.Fatalf("build approval: %d %+v", code, result)
	}
	work := newWorkBed(t)
	work.id = "ship-widget"
	b.owners.work = work.workOwners().work
	b.owners.connection = intentConnectionOwners{
		endpoint: b.owners.dependencies.endpoint,
		claimCheck: func(root, id string, endpoint goal.Endpoint) func() error {
			return goalBranchClaimCheckWith(root, id, endpoint, func(string, string) (string, error) { return b.machine, nil }, func(string) string { return b.root })
		},
	}
	brief := filepath.Join(b.root, "build-brief.md")
	if err := os.WriteFile(brief, []byte("Build the unit.\n"), 0644); err != nil {
		t.Fatal(err)
	}
	// --check ends option parsing, so --json precedes it.
	var out, diagnostic bytes.Buffer
	command, args, ok := resolveIntentArgv(append([]string{"work", "build", "ship-widget", "--work", "main", "--brief", brief, "--lines", "5", "--json"}, workCheck...))
	if !ok {
		t.Fatal("public build parser missing")
	}
	code := runIntentIn(command, args, &out, &diagnostic, b.root, b.owners)
	var result intentResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("build result: %v out=%s err=%s", err, &out, &diagnostic)
	}
	claimed := gcliForgivingParse(t, b.goalRecord("ship-widget"))
	if claimed.State != goal.StateClaimed || claimed.Claimed.Lineage != b.lineage || claimed.StopCapability.ClaimEpoch <= 0 || b.goalRecord("fix-docs") != reserved || b.preparations != 1 || len(work.starter.launched()) == 0 || resultData(t, result)["state"] != "awaiting-judgement" || strings.Contains(result.Summary, "claim was not granted") {
		t.Fatalf("a granted claim with pending adoption did not build: exit=%d result=%+v launches=%v err=%s", code, result, work.starter.launched(), &diagnostic)
	}
	adoption, ok := resultData(t, result)["adoption"].(map[string]any)
	if code != 1 || result.Outcome != intentPartial || !ok || adoption["pending"] != true || !strings.Contains(resultWords(result), "goal=fix-docs adoption=pending") || !strings.Contains(result.Summary, "adoption remains pending") {
		t.Fatalf("build hid pending adoption: exit=%d result=%+v", code, result)
	}
}
