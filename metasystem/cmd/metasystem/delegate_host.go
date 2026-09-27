package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/delegation"
	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
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
	})
	return delegation.WaitOutcome{Code: code, Output: output.String()}
}

// WatchJob is `job watch`: block to terminal while the watched workspace's
// suite journal is tailed onto stderr; the waiter's exit code rides through.
func (engineHost) WatchJob(_ context.Context, root, job string, callerPid int64, progressRoot string) int {
	stopProgress := startSuiteProgressPrinter(progressRoot, 2*time.Second, os.Stderr)
	defer stopProgress()
	return compatibilityWaitCommand([]string{"--root", root, "--job", job}, nil, callerPid, func(metarun.WaitResult, bool) {})
}

// ExtendBudget is `goal extend-budget` for the supplied caller: its lineage
// is the request's, never this process's environment.
func (engineHost) ExtendBudget(_ context.Context, request delegation.ExtendBudgetRequest) (string, int) {
	dependencies := defaultSyncRequestDependencies()
	dependencies.authorityFacts.caller = processIdentity{pid: request.CallerPid}
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
