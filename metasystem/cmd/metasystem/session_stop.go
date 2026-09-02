package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
)

const sessionStopLifetime = 8 * time.Hour

var (
	classifySessionStopCaller = lease.Classify
	currentSessionStopHolder  = lease.CurrentHolder
	classifySessionStopView   = lease.ClassifyVerb
	sessionStopNow            = time.Now
	proveSessionStopHuman     = func(root string, pid int64, now time.Time) (humanauthority.Proof, error) {
		proof, err := humanauthority.Prove(root, pid, nil, now)
		if err != nil {
			return humanauthority.Proof{}, err
		}
		if !proof.ValidFor(root) {
			return humanauthority.Proof{}, fmt.Errorf("enrolled-terminal human authority was not proven")
		}
		return proof, nil
	}
)

// runSessionStop is the normal path for minting a quiet-stop authorization.
// It refuses every agent-classified caller before the persistence layer runs.
// Same-user raw-byte forgery remains outside this boundary, as it does for
// every repository-stored human-authority verb; ledger authentication owns
// that separate trust problem.
func runSessionStop(args []string) int {
	flags := flag.NewFlagSet("session stop", flag.ContinueOnError)
	root := flags.String("root", ".", "checkout root")
	by := flags.String("by", "", "name of the attending human")
	if flags.Parse(args) != nil {
		return 2
	}
	if flags.NArg() != 0 || strings.TrimSpace(*by) == "" {
		fmt.Fprintln(os.Stderr, "session stop: --by <human> is required and positional arguments are not accepted")
		return 2
	}

	stateRoot, err := goal.ResolveStateRoot(*root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "session stop refused: the state root cannot be resolved: %v\n", err)
		return 1
	}
	callerPID := int64(os.Getpid())
	classification, err := classifySessionStopCaller(stateRoot, callerPID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "session stop refused: caller classification failed: %v\n", err)
		return 3
	}
	if classification.Class != lease.ClassHuman {
		fmt.Fprintf(os.Stderr, "session stop refused: this is human-reserved; caller classifies %s\n", classification.Class)
		return 3
	}

	now := sessionStopNow().UTC()
	humanProof, err := proveSessionStopHuman(stateRoot, int64(os.Getppid()), now)
	if err != nil {
		fmt.Fprintf(os.Stderr, "session stop refused: attended human authority was not proven: %v\n", err)
		return 3
	}
	holder, err := currentSessionStopHolder(stateRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "session stop refused: the current checkout holder cannot be read: %v\n", err)
		return 1
	}
	view, err := classifySessionStopView(stateRoot, callerPID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "session stop refused: the current lease epoch cannot be read: %v\n", err)
		return 1
	}
	if holder.MainId == "" || holder.SessionId == "" || view.ClaimEpoch == nil || *view.ClaimEpoch < 1 {
		fmt.Fprintln(os.Stderr, "session stop refused: the current checkout holder lacks a main identity, announced session, or lease epoch")
		return 1
	}

	marker, err := (&goal.Store{Root: stateRoot}).WriteSessionStop(goal.SessionStop{
		SchemaVersion: 3,
		SessionId:     holder.SessionId,
		HolderMainId:  holder.MainId,
		ClaimEpoch:    *view.ClaimEpoch,
		By:            *by,
		WrittenAt:     now.Format(time.RFC3339),
		ExpiresAt:     now.Add(sessionStopLifetime).Format(time.RFC3339),
		Human: goal.SessionStopProcessRef{
			Pid: humanProof.InvokerRef.PID, PidStartedAt: humanProof.InvokerRef.PIDStartedAt,
			PidStartTicks: humanProof.InvokerRef.StartTicks, BootID: humanProof.InvokerRef.BootID,
		},
	}, humanProof)
	if err != nil {
		fmt.Fprintf(os.Stderr, "session stop: %v\n", err)
		return 1
	}
	fmt.Printf("session stop authorized once for %s at holder %s epoch %d by %s\n",
		marker.SessionId, marker.HolderMainId, marker.ClaimEpoch, marker.By)
	return 0
}

func runSessionEnd(args []string) int {
	flags := flag.NewFlagSet("session end", flag.ContinueOnError)
	root := flags.String("root", ".", "checkout root")
	session := flags.String("session", "", "announced session id")
	if flags.Parse(args) != nil {
		return 2
	}
	if flags.NArg() != 0 || strings.TrimSpace(*session) == "" {
		fmt.Fprintln(os.Stderr, "session end: --session is required and positional arguments are not accepted")
		return 2
	}
	if err := (&goal.Store{Root: *root}).EndSessionStop(*session); err != nil {
		fmt.Fprintf(os.Stderr, "session end: unused stop authorization could not be retired: %v\n", err)
		return 1
	}
	return 0
}
