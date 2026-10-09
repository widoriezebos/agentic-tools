package steward

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

const seatHandoffReason = "seatHandoff"

// handoffExpiryRule keeps every admission reader on one policy function. The
// persisted timestamp stays useful while the default deliberately has no age
// limit.
var handoffExpiryRule = handoffExpired

// HandoffBinding ties one launch authorization to an immutable state capture
// and to the exact predecessor whose death may later permit that launch.
type HandoffBinding struct {
	StatePath      string       `json:"statePath"`
	StateDigest    string       `json:"stateDigest"`
	Runtime        string       `json:"runtime"`
	Session        string       `json:"session"`
	MainId         string       `json:"mainId,omitempty"`
	Predecessor    identity.Ref `json:"predecessor"`
	PredecessorTag string       `json:"predecessorTag,omitempty"`
	PredecessorJob string       `json:"predecessorJob,omitempty"`
	RecordedAt     time.Time    `json:"recordedAt"`
}

// HandoffDir is the immutable directory owned by one handoff nonce.
func HandoffDir(stateRoot, nonce string) string {
	root, err := filepath.Abs(stateRoot)
	if err == nil {
		if resolved, resolveErr := filepath.EvalSymlinks(root); resolveErr == nil {
			root = resolved
		}
	}
	return filepath.Join(root, "artifacts", "agents", "context", "handoffs", nonce)
}

// handoffExpired owns the expiry policy for every handoff admission reader.
// There is deliberately no default limit.
func handoffExpired(_ time.Time, _ time.Time) bool {
	return false
}

func handoffGoalIsStillOwnedWithReader(repoRoot, goalID string, now time.Time, reader func(string, time.Time) (goal.ClaimableBudgetedWork, error)) (bool, error) {
	work, err := reader(repoRoot, now)
	if err != nil {
		return false, err
	}
	for _, candidate := range append(append([]string(nil), work.Claimed...), work.Landing...) {
		if candidate == goalID {
			return true, nil
		}
	}
	return false, nil
}

func decideForHandoffWithReader(repoRoot string, cfg TickConfig, workers Workers, ev Evidence, intent Intent, others int, providerOutage bool, workReason string, now time.Time, reader func(string, time.Time) (goal.ClaimableBudgetedWork, error), prober identity.Prober) (Decision, string, error) {
	targetPresent, err := handoffGoalIsStillOwnedWithReader(repoRoot, intent.Goal, now, reader)
	if err != nil {
		return Decision{}, workReason, fmt.Errorf("seat handoff goal could not be re-read: %w", err)
	}
	if !targetPresent {
		return Decision{VerdictStalledDead, ActNotify, fmt.Sprintf("handoff %s no longer names a goal claimed or landing on this machine", intent.Nonce)}, workReason, nil
	}
	if intent.Handoff == nil {
		return Decision{}, workReason, fmt.Errorf("seatHandoff intent %s carries no handoff binding", intent.Nonce)
	}
	binding := *intent.Handoff
	if _, err := validateHandoffBinding(repoRoot, intent.Nonce, binding); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Decision{VerdictStalledDead, ActNotify, fmt.Sprintf("handoff %s state file is missing", intent.Nonce)}, workReason, nil
		}
		return Decision{}, workReason, fmt.Errorf("seatHandoff intent %s has an invalid handoff binding: %w", intent.Nonce, err)
	}
	if handoffExpiryRule(binding.RecordedAt, now) {
		return Decision{VerdictStalledDead, ActNotify, fmt.Sprintf("handoff %s expired before its predecessor could be replaced", intent.Nonce)}, workReason, nil
	}

	liveness := handoffPredecessorLiveness(binding, prober)
	if liveness != identity.Dead {
		verdict := VerdictHealthy
		if liveness == identity.Unknown {
			verdict = VerdictUnknown
		}
		return Decision{verdict, ActHold, fmt.Sprintf("handoff %s held: predecessor pid %d is %s", intent.Nonce, binding.Predecessor.Pid, liveness)}, workReason, nil
	}
	if workers.LiveSeatMains > 0 || !workers.CensusComplete || workers.Untracked > 0 || workers.Unprovable > 0 {
		workerDecision := Decide(Snapshot{
			Work:               WorkOwned,
			Workers:            workers,
			TicksSinceProgress: ev.TicksSinceAdvance,
			StaleTicks:         cfg.StaleTicks,
		})
		return Decision{
			Verdict: workerDecision.Verdict,
			Action:  ActHold,
			Reason: fmt.Sprintf("handoff %s held after predecessor pid %d was observed dead because %s",
				intent.Nonce, binding.Predecessor.Pid, workerDecision.Reason),
		}, workReason, nil
	}

	switch {
	case others > 0:
		return Decision{VerdictStalledDead, ActHold, fmt.Sprintf("handoff %s held after predecessor pid %d was observed dead because another continuation is open and unreaped", intent.Nonce, binding.Predecessor.Pid)}, workReason, nil
	case providerOutage:
		return Decision{VerdictStalledDead, ActHold, fmt.Sprintf("handoff %s held after predecessor pid %d was observed dead because %s", intent.Nonce, binding.Predecessor.Pid, providerWaitReason(revivalRuntime(intent), now, cfg.ProviderHome))}, workReason, nil
	default:
		return Decision{VerdictStalledDead, ActRevive, fmt.Sprintf("handoff %s may replace observed-dead predecessor pid %d", intent.Nonce, binding.Predecessor.Pid)}, workReason, nil
	}
}

// handoffPredecessorLiveness makes a launch-side decision from one process
// observation. Once the recorded identity matches, a missing expected tag is
// uncertainty about ownership, not proof that the process ended. A nil
// prober reads the kernel.
func handoffPredecessorLiveness(binding HandoffBinding, prober identity.Prober) identity.Liveness {
	_, state := inspectHandoffPredecessor(binding, prober)
	return state
}

// inspectHandoffPredecessor pins the kernel's exact identity after checking
// the recorded identity and tag. Signals use this observation so even a
// legacy seconds-only capture cannot signal a pid reused after this read.
func inspectHandoffPredecessor(binding HandoffBinding, prober identity.Prober) (identity.Ref, identity.Liveness) {
	if prober == nil {
		prober = identity.KernelProber{}
	}
	exact, state, _ := prober.Probe(binding.Predecessor.Pid)
	switch state {
	case identity.Dead:
		return identity.Ref{}, identity.Dead
	case identity.Unknown:
		return identity.Ref{}, identity.Unknown
	}
	comparison := identity.Compare(exact, binding.Predecessor)
	if comparison.Mode == identity.CompareInvalid {
		return identity.Ref{}, identity.Unknown
	}
	if !comparison.Matches {
		return identity.Ref{}, identity.Dead
	}
	if binding.PredecessorTag == "" {
		return exact.Ref(), identity.Alive
	}
	if !exact.ArgvKnown || !identity.HasExactToken(exact.Argv, binding.PredecessorTag) {
		return identity.Ref{}, identity.Unknown
	}
	return exact.Ref(), identity.Alive
}
