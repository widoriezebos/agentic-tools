package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	metarun "github.com/widoriezebos/agentic-tools/metasystem/internal/run"
)

var (
	waitRegisterProber    identity.Prober = identity.KernelProber{}
	waitRegisterNow                       = func() time.Time { return time.Now().UTC() }
	waitRegisterBootClock                 = identity.BootClock
)

type resolvedWaitCaller struct {
	view           lease.ClassifyResult
	owner          metarun.Caller
	runtimeSession string
}

func resolveWaitCaller(root string, callerPID int64) (resolvedWaitCaller, int, error) {
	view, err := classifyVerbCaller(root, callerPID)
	if err != nil {
		return resolvedWaitCaller{}, metarun.ExitWaiterUnknown, fmt.Errorf("wait caller identity is uncertain: %w", err)
	}
	if view.Class != lease.ClassMain || !view.Holder || view.Announcement == nil {
		return resolvedWaitCaller{}, metarun.ExitWaiterBusy, fmt.Errorf("wait registration is eligible only for the live checkout holder's main session")
	}
	lineage := view.Announcement.OwnerLineage
	if lineage == "" {
		lineage = view.MainId
	}
	runtimeSession := view.Announcement.EffectiveRuntimeSession()
	if runtimeSession == "" {
		return resolvedWaitCaller{}, metarun.ExitWaiterBusy, fmt.Errorf("wait registration requires the main's authenticated runtime session")
	}
	owner := metarun.Caller{Class: view.Class, MainId: view.MainId, OwnerLineage: lineage, ClaimEpoch: view.ClaimEpoch, SessionId: runtimeSession}
	return resolvedWaitCaller{view: view, owner: owner, runtimeSession: runtimeSession}, 0, nil
}

func waitFlagWasSet(flags *flag.FlagSet, name string) bool {
	set := false
	flags.Visit(func(item *flag.Flag) {
		if item.Name == name {
			set = true
		}
	})
	return set
}

func printRegisteredWait(row metarun.Waiter, jsonOutput bool) {
	if jsonOutput {
		encoded, _ := json.Marshal(row)
		fmt.Println(string(encoded))
		return
	}
	fmt.Printf("registered %s wait %s until %s\n", row.Kind, row.WaitID, row.Deadline)
}

func runWaitRegister(args []string) int {
	flags := flag.NewFlagSet("wait register", flag.ContinueOnError)
	root := pathFlag(flags, "root", ".", "checkout or installation state root")
	pid := flags.Int64("pid", 0, "tracked process identifier")
	label := flags.String("label", "", "description of the tracked work")
	jobID := flags.String("job", "", "delegate job covered by the tracked process")
	human := flags.Bool("human", false, "wait for a human answer")
	question := flags.String("question", "", "question awaiting a human answer")
	timeout := flags.Duration("timeout", metarun.DefaultLocalWaitTimeout, "positive wait deadline, at most 24h")
	jsonOutput := flags.Bool("json", false, "print the registered row as JSON")
	if flags.Parse(args) != nil || flags.NArg() != 0 {
		return metarun.ExitInvalidWait
	}
	timeoutExplicit := waitFlagWasSet(flags, "timeout")
	if *timeout <= 0 || *timeout > metarun.MaxRegisteredWaitTimeout {
		fmt.Fprintln(os.Stderr, "wait timeout must be positive and no longer than 24 hours")
		return metarun.ExitInvalidWait
	}
	kind := "local"
	if *human {
		kind = "human"
		if !timeoutExplicit {
			fmt.Fprintln(os.Stderr, "human wait requires --timeout")
			return metarun.ExitInvalidWait
		}
		if strings.TrimSpace(*question) == "" || *pid != 0 || *label != "" || *jobID != "" {
			fmt.Fprintln(os.Stderr, "human wait requires --question and accepts no --pid, --label, or --job")
			return metarun.ExitInvalidWait
		}
	} else {
		if *pid <= 0 || strings.TrimSpace(*label) == "" || *question != "" {
			fmt.Fprintln(os.Stderr, "local wait requires --pid and --label and accepts no --question")
			return metarun.ExitInvalidWait
		}
		if *jobID != "" {
			if err := metarun.ValidateWaitSelector(metarun.WaitSelector{Kind: "job", TargetID: *jobID}); err != nil {
				fmt.Fprintln(os.Stderr, err)
				return metarun.ExitInvalidWait
			}
		}
	}
	stateRoot, err := goal.ResolveStateRoot(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return metarun.ExitWaiterIO
	}
	resolved, code, err := resolveWaitCaller(stateRoot, waitCallerPID())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return code
	}
	scanCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	openWorkSignature, err := waitOpenWorkSignature(scanCtx, stateRoot)
	cancel()
	if err != nil {
		fmt.Fprintln(os.Stderr, "the open-work signature could not be read:", err)
		return metarun.ExitWaiterIO
	}
	row, err := (&metarun.Store{Root: stateRoot, Prober: waitRegisterProber}).RegisterDetachedWait(metarun.RegisterWaitRequest{
		Kind: kind, Pid: *pid, Label: *label, Question: *question, JobID: *jobID,
		Owner: resolved.owner, RuntimeSession: resolved.runtimeSession, Runtime: resolved.view.Announcement.Runtime,
		Timeout: *timeout, OpenWorkSignature: openWorkSignature,
	}, metarun.WaitOptions{Now: waitRegisterNow, BootClock: waitRegisterBootClock})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return metarun.WaiterExitCode(err)
	}
	printRegisteredWait(row, *jsonOutput)
	return 0
}

func runWaitEnd(args []string) int {
	flags := flag.NewFlagSet("wait end", flag.ContinueOnError)
	root := pathFlag(flags, "root", ".", "checkout or installation state root")
	waitID := flags.String("wait-id", "", "registered wait identifier")
	jsonOutput := flags.Bool("json", false, "print the ended row as JSON")
	if flags.Parse(args) != nil || flags.NArg() != 0 || !metarun.ValidWaitID(*waitID) {
		if *waitID == "" || !metarun.ValidWaitID(*waitID) {
			fmt.Fprintln(os.Stderr, "wait end requires a valid --wait-id")
		}
		return metarun.ExitInvalidWait
	}
	stateRoot, err := goal.ResolveStateRoot(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return metarun.ExitWaiterIO
	}
	resolved, code, err := resolveWaitCaller(stateRoot, waitCallerPID())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return code
	}
	row, err := (&metarun.Store{Root: stateRoot}).EndDetachedWait(*waitID, resolved.owner, resolved.runtimeSession,
		metarun.WaitOptions{Now: waitRegisterNow, BootClock: waitRegisterBootClock})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return metarun.WaiterExitCode(err)
	}
	if *jsonOutput {
		encoded, _ := json.Marshal(row)
		fmt.Println(string(encoded))
	} else {
		fmt.Printf("ended %s wait %s by %s\n", row.Kind, row.WaitID, row.InterruptedBy)
	}
	return 0
}
