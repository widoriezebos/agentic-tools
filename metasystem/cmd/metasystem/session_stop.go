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
	classifySessionStopView   = classifyVerbCaller
	sessionStopNow            = time.Now
	proveSessionStopHuman     = func(root string, pid int64, now time.Time) (humanauthority.Proof, error) {
		proof, err := humanauthority.ProveTerminal(root, pid, nil, now)
		if err != nil {
			return proof, err
		}
		if !proof.TerminalValidFor(root) {
			return humanauthority.Proof{}, fmt.Errorf("terminal human authority was not proven")
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
	root := pathFlag(flags, "root", ".", "checkout root")
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
	marker, refusal, code := authorizeSessionStop(stateRoot, *by)
	if code != 0 {
		fmt.Fprintln(os.Stderr, refusal)
		return code
	}
	fmt.Printf("session stop authorized once for %s at holder %s epoch %d by %s\n",
		marker.SessionId, marker.HolderMainId, marker.ClaimEpoch, marker.By)
	return 0
}

// authorizeSessionStop mints one quiet-stop authorization for the current
// announced main session at a proven human terminal. A refusal returns its
// sentence and exit code and writes nothing.
func authorizeSessionStop(stateRoot, by string) (goal.SessionStop, string, int) {
	callerPID := int64(os.Getpid())
	classification, err := classifySessionStopCaller(stateRoot, callerPID)
	if err != nil {
		return goal.SessionStop{}, fmt.Sprintf("session stop refused: caller classification failed: %v", err), 3
	}
	if classification.Class != lease.ClassHuman {
		return goal.SessionStop{}, fmt.Sprintf("session stop refused: this is human-reserved; caller classifies %s", classification.Class), 3
	}

	now := sessionStopNow().UTC()
	humanProof, err := proveSessionStopHuman(stateRoot, int64(os.Getppid()), now)
	if err != nil {
		return goal.SessionStop{}, fmt.Sprintf("session stop refused: attended human authority was not proven: %v", err), 3
	}
	holder, err := currentSessionStopHolder(stateRoot)
	if err != nil {
		return goal.SessionStop{}, fmt.Sprintf("session stop refused: the current checkout holder cannot be read: %v", err), 1
	}
	view, err := classifySessionStopView(stateRoot, callerPID)
	if err != nil {
		return goal.SessionStop{}, fmt.Sprintf("session stop refused: the current lease epoch cannot be read: %v", err), 1
	}
	if holder.MainId == "" || holder.SessionId == "" || view.ClaimEpoch == nil || *view.ClaimEpoch < 1 {
		return goal.SessionStop{}, "session stop refused: the current checkout holder lacks a main identity, announced session, or lease epoch", 1
	}

	marker, err := (&goal.Store{Root: stateRoot}).WriteSessionStop(goal.SessionStop{
		SchemaVersion: 3,
		SessionId:     holder.SessionId,
		HolderMainId:  holder.MainId,
		ClaimEpoch:    *view.ClaimEpoch,
		By:            by,
		WrittenAt:     now.Format(time.RFC3339),
		ExpiresAt:     now.Add(sessionStopLifetime).Format(time.RFC3339),
		Human: goal.SessionStopProcessRef{
			Pid: humanProof.InvokerRef.PID, PidStartedAt: humanProof.InvokerRef.PIDStartedAt,
			PidStartTicks: humanProof.InvokerRef.StartTicks, BootID: humanProof.InvokerRef.BootID,
		},
	}, humanProof)
	if err != nil {
		return goal.SessionStop{}, fmt.Sprintf("session stop: %v", err), 1
	}
	return marker, "", 0
}

func runSessionEnd(args []string) int {
	flags := flag.NewFlagSet("session end", flag.ContinueOnError)
	root := pathFlag(flags, "root", ".", "checkout root")
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
