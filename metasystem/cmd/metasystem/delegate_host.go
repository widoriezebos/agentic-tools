package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/delegation"
	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/ownercall"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	metarun "github.com/widoriezebos/agentic-tools/metasystem/internal/run"
)

// engineHost is the delegation lifecycle's host port: the owners that still
// live in this command layer (the durable job waiter and watcher, and the
// consumption-earned budget extension), called in-process with the
// lifecycle's supplied caller (design 6.2).
type engineHost struct{}

// WaitJob is `internal wait --root R --job J` for the supplied caller; the
// WAIT line is captured with the verb's diagnostics.
func (engineHost) WaitJob(_ context.Context, root, job string, callerPid int64) delegation.WaitOutcome {
	var output strings.Builder
	code := compatibilityWaitCommand([]string{"--root", root, "--job", job}, nil, callerPid, func(result metarun.WaitResult, jsonOutput bool) {
		writeWaitResult(&output, result, jsonOutput)
	}, io.Discard, os.Stderr)
	return delegation.WaitOutcome{Code: code, Output: output.String()}
}

// WatchJob is `job watch`: block to terminal while the watched workspace's
// suite journal is tailed onto stderr; the waiter's exit code rides through.
func (engineHost) WatchJob(_ context.Context, root, job string, callerPid int64, progressRoot string) int {
	stopProgress := startSuiteProgressPrinter(progressRoot, 2*time.Second, os.Stderr)
	defer stopProgress()
	return compatibilityWaitCommand([]string{"--root", root, "--job", job}, nil, callerPid, func(metarun.WaitResult, bool) {}, io.Discard, os.Stderr)
}

// BreachStopOrderingHuman is job breach-stop's person (rule H1) for the
// lifecycle's supplied caller: the enrolled terminal's name, proven from
// that caller exactly as the verb proves it from its own parent.
func (engineHost) BreachStopOrderingHuman(_ context.Context, root string, callerPid int64, now time.Time) (string, error) {
	return breachStopOrderingHumanWith(root, lease.ClassifyResult{Class: lease.ClassHuman}, "", now, func(root string, now time.Time) (string, error) {
		proof, err := humanauthority.Prove(root, callerPid, nil, now)
		if err != nil {
			return "", err
		}
		flags := &syncFlags{root: root}
		if err := resolveGoalHuman(flags, proof); err != nil {
			return "", err
		}
		return flags.by, nil
	})
}

// ExtendBudget is `goal extend-budget` for the supplied caller: its lineage
// is the request's, never this process's environment.
func (engineHost) ExtendBudget(_ context.Context, request delegation.ExtendBudgetRequest) (string, int) {
	dependencies := defaultSyncRequestDependencies()
	dependencies.authorityFacts.caller = ownercall.Process{Pid: request.CallerPid}
	lineage := request.OwnerLineage
	dependencies.ownerLineage = func() string { return lineage }
	var output strings.Builder
	code := goalExtendBudgetTo([]string{
		"--root", request.Root, "--id", request.GoalID, "--revision", fmt.Sprint(request.Revision),
		"--proposed-cap", fmt.Sprint(request.ProposedCap), "--role", request.Role,
		"--dispatch-mode", request.DispatchMode, "--destructive-reach", request.DestructiveReach,
	}, goalCommandNow, dependencies, dispatchcore.ConcreteProofAdmissionReads(), &output, &output)
	return output.String(), code
}
