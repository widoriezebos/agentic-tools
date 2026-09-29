package main

// The production owners of the landing path (internal/landing/landpath): the
// commit boundary and the landing driver call every owner in this process
// (plans/designs/verbs-object-action.md 6.2). The current process is the
// supplied identity lease classification and the ledger owners start from,
// because it is the parent the former wrapper children observed; the owner
// lineage is named on the request, never inherited.

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/behaviorsurface"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/brain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/cachedomain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	goalbranch "github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/helm"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/landpath"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/output"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	metarun "github.com/widoriezebos/agentic-tools/metasystem/internal/run"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/validate"
)

// landingPathGit runs git with this process's environment.
func landingPathGit(call landpath.GitCall) landpath.GitResult {
	// git runs in the directory rather than with -C, as the shell landing
	// ran it, so a git on PATH sees the landing's own argument vector.
	command := exec.Command("git", call.Args...)
	command.Dir = call.Dir
	if call.Env != nil {
		command.Env = call.Env
	}
	if call.Stdin != nil {
		command.Stdin = bytes.NewReader(call.Stdin)
	}
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	err := command.Run()
	code := 0
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			code = exit.ExitCode()
		} else {
			code = -1
			fmt.Fprintln(&stderr, err)
		}
	}
	return landpath.GitResult{Stdout: stdout.Bytes(), Stderr: stderr.Bytes(), Code: code}
}

func landingPathStartedAt(pid int64) (int64, error) {
	exact, state, err := (identity.KernelProber{}).Probe(pid)
	if err != nil {
		return 0, err
	}
	if state != identity.Alive {
		return 0, fmt.Errorf("pid %d is %s", pid, state)
	}
	return exact.StartedAt.Unix(), nil
}

func landingPathNonce() (string, error) {
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(nonce[:]), nil
}

func landingPathFileReadable(path string) bool {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return false
	}
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	file.Close()
	return true
}

func landingPathConfValue(root, key string) string {
	value, found, err := config.CommittedLookup(filepath.Join(root, "metasystem.conf"), key)
	if err != nil || !found {
		return ""
	}
	return value
}

func landingPathSelect(paths []string, prefix string) ([]string, error) {
	if len(paths) == 0 {
		return nil, nil
	}
	policy, err := behaviorsurface.Load()
	if err != nil {
		return nil, err
	}
	projection, err := behaviorsurface.ParseProjection("LANDING")
	if err != nil {
		return nil, err
	}
	var selected []string
	for _, path := range paths {
		included, err := policy.Includes(projection, path, prefix)
		if err != nil {
			return nil, err
		}
		if included {
			selected = append(selected, path)
		}
	}
	return selected, nil
}

// landingPathObserve is the live evaluator: the observation this engine
// decides for one prospective tree.
func landingPathObserve(request landpath.ObserveRequest) (landing.Observation, int) {
	if request.Carried != "" && (request.Judge != "live" && request.Judge != "base" || request.Judge == "base" && request.LiveFailure == "") {
		return landing.Observation{}, 2
	}
	now, err := goalCommandNow(request.Root)
	if err != nil {
		return landing.Observation{}, recordExit(err)
	}
	root := request.Root
	params := landing.ObserveParams{
		RepoRoot: root, CandidateTree: request.Tree, Chain: request.Chain,
		Attested: request.Attested, AttestedSnapshot: request.AttestedSnapshot, AttestedBase: request.AttestedBase,
		DirectFix: request.DirectFix, RevertOf: request.RevertOf, Goal: request.Goal, Actor: request.Actor,
		RootJob: request.RootJob, TestReceipt: request.TestReceipt, Recertification: request.Recertification,
		Carried: request.Carried, ProjectTree: request.ProjectTree, LedgerTip: request.LedgerTip, Judge: request.Judge,
		LiveFailure: request.LiveFailure, CarriedBy: request.CarriedBy, Now: now,
	}
	params.BindAttested = func(commit, snapshot, base, goal, beforeTree, afterTree string) (landing.AttestedUnit, error) {
		bound, err := goalbranch.BindLandedUnit(root, snapshot, base, goal, commit, beforeTree, afterTree)
		return landing.AttestedUnit{Goal: bound.Goal, Unit: bound.Unit, Digest: bound.Digest, CriticRoot: bound.CriticRoot, GateRunID: bound.GateRunID,
			Round: bound.Round, GoalRevision: bound.GoalRevision, FoldPaths: bound.FoldPaths, ChangedPaths: bound.ChangedPaths,
			HasPlan: bound.HasPlan, Destructive: bound.Destructive}, err
	}
	if (request.TestReceipt != "" || request.Carried != "") && request.Recertification == "" {
		params.VerifyTesting = func() (proofrun.TestResult, error) {
			return verifyRetainedTesting(testingSelectionRequest{Root: root, GoalID: request.Goal,
				Mode: testpolicy.ModeAuto, Purpose: testpolicy.PurposeDelivery, Carried: request.Carried != ""})
		}
	}
	return landing.Observe(params), 0
}

func landingPathVerifyRequest(root, tree, goalID string, carried bool) testingSelectionRequest {
	return testingSelectionRequest{Preparation: &testingPreparationState{}, Root: root, Tree: tree, GoalID: goalID,
		Mode: testpolicy.ModeAuto, Purpose: testpolicy.PurposeDelivery, Carried: carried}
}

func landingPathLiveJudge() landpath.Judge {
	return landpath.Judge{
		Observe:   landingPathObserve,
		Workspace: landing.ProjectWorkspaceTree,
		VerifyCarried: func(root, tree, goalID string) ([]byte, int) {
			var stdout bytes.Buffer
			status := testVerifyTo(&stdout, io.Discard, landingPathVerifyRequest(root, tree, goalID, true), true)
			return stdout.Bytes(), status
		},
	}
}

// landingPathBaseJudge builds the engine at HEAD in a detached scratch
// worktree, for a carried landing whose live engine could not decide. The
// base engine is a different binary: it is asked through its own argv.
func landingPathBaseJudge(toplevel, prefix string, stderr io.Writer) (landpath.Judge, func(), error) {
	scratch, err := os.MkdirTemp("", "metasystem-carry-judge.")
	if err != nil {
		return landpath.Judge{}, nil, err
	}
	cleanup := func() { os.RemoveAll(scratch) }
	worktree := filepath.Join(scratch, "base")
	add := exec.Command("git", "-C", toplevel, "worktree", "add", "--detach", worktree, "HEAD")
	add.Stdout, add.Stderr = stderr, stderr
	if err := add.Run(); err != nil {
		cleanup()
		return landpath.Judge{}, nil, err
	}
	engine := filepath.Join(scratch, "judge")
	// Trimmed and in the machine engine cache (disk-lifetimes A3/A5): the
	// detached worktree's path never keys the build.
	build := exec.Command("go", "build", "-trimpath", "-o", engine, "./cmd/metasystem")
	build.Dir = filepath.Join(worktree, strings.TrimSuffix(prefix, "/"))
	environment, carryErr := cachedomain.Carry(os.Environ(), "")
	if carryErr != nil {
		cleanup()
		return landpath.Judge{}, nil, carryErr
	}
	build.Env = environment
	build.Stdout, build.Stderr = stderr, stderr
	buildErr := build.Run()
	remove := exec.Command("git", "-C", toplevel, "worktree", "remove", "--force", worktree)
	remove.Stdout, remove.Stderr = stderr, stderr
	removeErr := remove.Run()
	if buildErr != nil || removeErr != nil {
		cleanup()
		return landpath.Judge{}, nil, errors.Join(buildErr, removeErr)
	}
	data, err := os.ReadFile(engine)
	if err != nil {
		cleanup()
		return landpath.Judge{}, nil, err
	}
	run := func(args ...string) ([]byte, int) {
		command := exec.Command(engine, append([]string{"internal"}, args...)...)
		var stdout bytes.Buffer
		command.Stdout = &stdout
		if err := command.Run(); err != nil {
			var exit *exec.ExitError
			if errors.As(err, &exit) {
				return stdout.Bytes(), exit.ExitCode()
			}
			return stdout.Bytes(), 1
		}
		return stdout.Bytes(), 0
	}
	judge := landpath.Judge{Digest: bytesSHA256(data)}
	judge.Observe = func(request landpath.ObserveRequest) (landing.Observation, int) {
		args := []string{"landing", "observe", "--root", request.Root, "--tree", request.Tree}
		for _, flag := range []struct{ name, value string }{
			{"chain", request.Chain}, {"attested", request.Attested}, {"attested-snapshot", request.AttestedSnapshot},
			{"attested-base", request.AttestedBase}, {"direct-fix", request.DirectFix}, {"revert-of", request.RevertOf},
			{"goal", request.Goal}, {"root-job", request.RootJob}, {"test-receipt", request.TestReceipt},
			{"recertification", request.Recertification}, {"carried", request.Carried}, {"project-tree", request.ProjectTree},
			{"ledger-tip", request.LedgerTip}, {"carried-by", request.CarriedBy}, {"actor", request.Actor},
			{"judge", request.Judge}, {"live-failure", request.LiveFailure},
		} {
			if flag.value != "" {
				args = append(args, "--"+flag.name, flag.value)
			}
		}
		encoded, status := run(args...)
		var observed struct {
			landing.Observation
			RefusesAgent *bool `json:"refusesAgent"`
		}
		if json.Unmarshal(encoded, &observed) != nil {
			return landing.Observation{}, status
		}
		result := observed.Observation
		// An engine without the explicit enforcement field is read
		// conservatively: its would-refuse verdict refuses an agent.
		if observed.RefusesAgent != nil {
			result.RefusesAgent = *observed.RefusesAgent
		} else {
			result.RefusesAgent = strings.HasPrefix(result.VerdictTrailer, "would-refuse ")
		}
		return result, status
	}
	judge.Workspace = func(root, tree string) (string, error) {
		encoded, status := run("landing", "workspace", "--root", root, "--tree", tree)
		if status != 0 {
			return "", fmt.Errorf("base judge landing workspace exited %d", status)
		}
		return strings.TrimSpace(string(encoded)), nil
	}
	judge.VerifyCarried = func(root, tree, goalID string) ([]byte, int) {
		args := []string{"test", "verify", "--root", root, "--tree", tree, "--mode", "auto", "--purpose", "delivery", "--carried", "--json"}
		if goalID != "" {
			args = append(args, "--goal", goalID)
		}
		return run(args...)
	}
	return judge, cleanup, nil
}

func landingPathDrift(root string, requireEmptyIndex bool, stdout, stderr io.Writer) int {
	drift, tolerated, err := landing.WorktreeDrift(root, requireEmptyIndex)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	for _, path := range tolerated {
		fmt.Fprintf(stderr, "tolerated register append: %s\n", path)
	}
	for _, entry := range drift {
		fmt.Fprintf(stdout, "%s\t%c%c\t%s\n", entry.Kind, entry.Index, entry.Worktree, entry.Path)
	}
	if len(drift) != 0 {
		return 1
	}
	return 0
}

func landingPathAdvance(root, upstream string, stdout, stderr io.Writer) int {
	err := landing.Advance(root, upstream, stdout, stderr)
	if err == nil {
		return 0
	}
	fmt.Fprintln(stderr, err)
	var refusal interface{ IsAdvanceRefusal() }
	if errors.As(err, &refusal) {
		return 1
	}
	return 2
}

func landingPathReceiptLine(root, tree, goalID, directFix string) (landpath.ReceiptDecision, error) {
	return landingPathReceiptLineFrom(nil, root, tree, goalID, directFix)
}

// landingPathReceiptLineFrom decides the receipt line reading Git through raw
// (nil is the real Git).
func landingPathReceiptLineFrom(raw func(gittree.RawRequest) gittree.RawResult, root, tree, goalID, directFix string) (landpath.ReceiptDecision, error) {
	decision, err := landing.ObserveReceiptLine(landing.ReceiptLineParams{RepoRoot: root, CandidateTree: tree, Goal: goalID, DirectFix: directFix, RawSource: raw})
	if err != nil {
		return landpath.ReceiptDecision{}, err
	}
	encoded, err := json.Marshal(decision)
	if err != nil {
		return landpath.ReceiptDecision{}, err
	}
	return landpath.ReceiptDecision{Refused: decision.Outcome == landing.ReceiptLineOutcomeRefused, Detail: decision.Detail, Encoded: string(encoded)}, nil
}

func landingPathTestReceipt(root, tree, command string, stdout, stderr io.Writer) int {
	return runLandingTestReceipt([]string{"--root", root, "--tree", tree, "--command", command}, stdout, stderr)
}

func landingPathGateWidth(root, chain string) string {
	data, err := os.ReadFile(filepath.Join(root, "artifacts", "agents", "jobs", chain+".json"))
	if err != nil {
		return ""
	}
	var job struct {
		GateWidth *string `json:"gateWidth"`
	}
	if json.Unmarshal(data, &job) != nil {
		return ""
	}
	if job.GateWidth == nil {
		return "area"
	}
	return *job.GateWidth
}

func landingPathPark(request landpath.ParkRequest) (string, error) {
	result, err := landing.Park(landing.ParkParams{Root: request.Root, Chain: request.Chain, TargetCommit: request.Target,
		Reason: request.Reason, Detail: request.Detail, Recertification: request.Recertification,
		CandidateCommit: request.CandidateCommit, RecoveryRefs: request.RecoveryRefs, CallerPID: request.CallerPID})
	if err != nil {
		var failure *landing.ParkFailure
		if errors.As(err, &failure) {
			return fmt.Sprintf("reason=chain-recertification-park-failed cause=%s error=%v", failure.Cause, failure.Err), err
		}
		return err.Error(), err
	}
	return fmt.Sprintf("state=%s\nreason=%s\nparkRecord=%s", result.State, result.Reason, result.ParkRecord), nil
}

func landingPathSpill(root, verb, ext string, data []byte) (string, error) {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	reference, err := output.Spill(absolute, verb, ext, data, time.Now().UTC())
	if err != nil {
		return "", err
	}
	return reference.Line(), nil
}

func landingPathBootClock() (landpath.BootSample, error) {
	id, elapsed, err := identity.BootClock()
	if err != nil {
		return landpath.BootSample{}, err
	}
	return landpath.BootSample{ID: id, Nanos: elapsed.Nanoseconds()}, nil
}

func landingPathNotifyGoal(root, goalID, publication string, began *landpath.BootSample) {
	stateRoot, err := goal.ResolveStateRoot(root)
	if err != nil {
		return
	}
	hint := metarun.WaitHint{Kind: "goal", TargetID: goalID, PublicationID: publication}
	if began != nil {
		hint.BeganBootNanos, hint.BeganBootID = began.Nanos, began.ID
	}
	metarun.NotifyWaiters(stateRoot, hint)
}

func landingPathGoalFetch(root string) (string, int) {
	var combined bytes.Buffer
	status := goalFetchTo(&combined, &combined, cleanOwnerRoot(root), goal.ResolveEndpoint)
	return combined.String(), status
}

func landingPathCarryStatus(root, carried, goalID, ledgerTip string) (landing.CarryStatus, string, int) {
	now, err := goalCommandNow(root)
	if err == nil {
		var status landing.CarryStatus
		status, err = landing.ReadCarryStatus(root, carried, goalID, ledgerTip, now)
		if err == nil {
			return status, "", 0
		}
	}
	return landing.CarryStatus{}, err.Error(), 1
}

// landingPathCarrying and landingPathCarried call the ledger owners with this
// process as the supplied identity and the landing's lineage named.
func landingPathCarrying(request landpath.CarryingRequest) (string, int) {
	invocation := ownerCallFromThisProcess(request.Lineage)
	req, err := invocation.syncRequest("carrying", request.Root, false)
	var combined bytes.Buffer
	status := goalCarryingTo(&combined, &combined, request.Root, req, err, goalCarryingRequest{
		Goal: request.Goal, Ref: request.Ref, Carrying: request.Carrying, Commit: request.Commit, Tree: request.Tree,
		Workspace: request.Workspace, Past: request.Past, Battery: request.Battery, Missing: request.Missing, Failing: request.Failing,
		Judge: request.Judge, JudgeTree: request.JudgeTree, JudgeDigest: request.JudgeDigest, LiveFailure: request.LiveFailure,
		Ledger: request.Ledger, By: request.By, Abandon: request.Abandon, Why: request.Why, OwnerPID: request.OwnerPID})
	return combined.String(), status
}

func landingPathCarried(request landpath.CarriedRequest) (string, int) {
	invocation := ownerCallFromThisProcess(request.Lineage)
	req, err := invocation.syncRequest("carried", request.Root, false)
	var combined bytes.Buffer
	status := goalCarriedTo(&combined, &combined, request.Root, req, err, goalCarriedRequest{
		Entry: request.Entry, Rebuild: request.RebuildFromCommit, Ref: request.Ref, Goal: request.Goal, Repair: request.RepairCounselor})
	return combined.String(), status
}

func landingPathChannelAskCarry(root, goalID, wants, fact string) {
	askChannelQuestion(root, channelAskInput{Goal: goalID, Kind: "carry", Wants: wants, Facts: []string{fact}})
}

// landingPathOwners are the production owners of the landing path.
func landingPathOwners() landpath.Owners {
	return landpath.Owners{
		Git:        landingPathGit,
		CallerPID:  int64(os.Getpid()),
		Getpid:     func() int64 { return int64(os.Getpid()) },
		LiveEngine: os.Executable,
		Environ:    os.Environ,
		Now:        time.Now,
		RequireHolder: func(root string, caller int64, epoch *int64) (*int64, error) {
			view, err := lease.RequireHolder(root, caller, epoch)
			return view.ClaimEpoch, err
		},
		WithHeld: lease.WithHeld,
		BrainFence: func(root, act string) (string, error) {
			return brain.Fence(root, act, goal.ExistingLedgerIdentity(root)), nil
		},
		ConfValue:  landingPathConfValue,
		StartedAt:  landingPathStartedAt,
		TokenNonce: landingPathNonce,
		WriteToken: func(path string, token landpath.WrapperToken) error {
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return err
			}
			return writeIdentityJSON(path, token)
		},
		RemoveFile:   os.Remove,
		ReadFile:     os.ReadFile,
		FileReadable: landingPathFileReadable,
		FileExists: func(path string) bool {
			_, err := os.Stat(path)
			return err == nil
		},
		Verify: func(request landpath.VerifyRequest, stdout, stderr io.Writer) int {
			return testVerifyTo(stdout, stderr, landingPathVerifyRequest(request.Root, request.Tree, request.Goal, false), false)
		},
		SelectLanding:  landingPathSelect,
		Live:           landingPathLiveJudge,
		BuildBaseJudge: landingPathBaseJudge,
		Held: func(root, base, commit, remote, ref string, stdout, stderr io.Writer) int {
			return landingHeldTo(stdout, stderr, cleanOwnerRoot(root), base, commit, remote, ref)
		},
		WeightAdd: func(root, commit, prefix, goalID string, numstat []byte, stdout, stderr io.Writer) int {
			return gateWeightAddTo(stdout, stderr, root, commit, prefix, goalID, numstat)
		},
		SyncTransport: func(root, branch string, stdout, stderr io.Writer) int {
			last, err := landing.SyncTransport(root, branch, nil)
			if last != "" {
				fmt.Fprintln(stdout, last)
			}
			if err != nil {
				fmt.Fprintln(stderr, err)
				var refusal *landing.TransportError
				if errors.As(err, &refusal) {
					return refusal.Code
				}
				return 1
			}
			return 0
		},
		Drift:           landingPathDrift,
		Advance:         landingPathAdvance,
		ReceiptLine:     landingPathReceiptLine,
		TestReceipt:     landingPathTestReceipt,
		JobGateWidth:    landingPathGateWidth,
		Park:            landingPathPark,
		OutputSpill:     landingPathSpill,
		BootClock:       landingPathBootClock,
		NotifyGoal:      landingPathNotifyGoal,
		GoalFetch:       landingPathGoalFetch,
		CarryStatus:     landingPathCarryStatus,
		GoalCarrying:    landingPathCarrying,
		GoalCarried:     landingPathCarried,
		ChannelAskCarry: landingPathChannelAskCarry,
	}
}

// landingPathCommit runs the commit boundary in this process: the landing
// owner, which supplies itself, commits each unit.
func landingPathCommit(request landpath.CommitRequest) (string, int) {
	var combined bytes.Buffer
	status := landpath.Commit(landingPathOwners(), request, &combined, &combined)
	return combined.String(), status
}

// landingGuardOwners are the production owners of the pre-commit guard.
func landingGuardOwners() landpath.GuardOwners {
	return landpath.GuardOwners{
		Git:          landingPathGit,
		Probe:        os.Getenv("METASYSTEM_GUARD_PROBE"),
		AllowNewPlan: os.Getenv("METASYSTEM_ALLOW_NEW_PLAN") == "1",
		CallerPID:    int64(os.Getpid()),
		Classify: func(root string, caller int64) (string, error) {
			result, err := lease.ClassifyVerbAt(root, root, caller)
			if err != nil {
				return "", err
			}
			return result.Class, nil
		},
		WrapperToken: func(token string, caller int64) bool {
			return validate.WrapperToken(token, caller, validate.KernelProcessTree{})
		},
		AppendObservation: func(root, line string) error {
			log := filepath.Join(root, "artifacts", "agents", "landing-observe.log")
			if err := os.MkdirAll(filepath.Dir(log), 0o755); err != nil {
				return errors.New("its observation directory could not be created")
			}
			file, err := os.OpenFile(log, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
			if err != nil {
				return errors.New("its observation could not be written")
			}
			_, writeErr := fmt.Fprintln(file, line)
			closeErr := file.Close()
			if writeErr != nil || closeErr != nil {
				return errors.New("its observation could not be written")
			}
			return nil
		},
		Helm:      helm.Active,
		HelmYield: helm.RecordYield,
	}
}
