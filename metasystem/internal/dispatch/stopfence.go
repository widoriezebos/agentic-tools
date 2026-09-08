package dispatch

import (
	"fmt"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/stopfence"
)

// LaunchFenceOutcome is the creator's post-publication fence verdict.
type LaunchFenceOutcome string

const (
	LaunchFenceOpen           LaunchFenceOutcome = "OPEN"
	LaunchFenceRefusedStopped LaunchFenceOutcome = "REFUSED-STOPPED"
)

// LaunchFenceResult carries the remembered and observed fence generations so
// the shell seam can decide whether to run its existing owned-job cancel path.
type LaunchFenceResult struct {
	Outcome            LaunchFenceOutcome `json:"outcome"`
	FenceGeneration    int64              `json:"fenceGeneration"`
	ObservedGeneration int64              `json:"observedGeneration"`
	Detail             string             `json:"detail,omitempty"`
}

// FenceBeforeLaunch gives the shell dispatch boundary a typed fence verdict
// before it consults supervision state that a completed stop makes stale.
func FenceBeforeLaunch(root string) (LaunchFenceResult, error) {
	fence, err := stopfence.Read(root)
	if err != nil {
		return LaunchFenceResult{}, fmt.Errorf("fence-before-launch cannot read the process-creation fence: %w", err)
	}
	result := LaunchFenceResult{
		Outcome:            LaunchFenceOpen,
		ObservedGeneration: fence.Generation,
	}
	if fence.State == stopfence.StateClosed {
		result.Outcome = LaunchFenceRefusedStopped
		result.Detail, err = stoppedCheckoutDetail(fence, root)
		if err != nil {
			return LaunchFenceResult{}, err
		}
	}
	return result, nil
}

// FenceAfterLaunch performs the second fence read for the shell-owned launch
// seam. It does not signal: dispatch.sh owns the existing identity-proven
// cancellation path and invokes it when this result is REFUSED-STOPPED.
func FenceAfterLaunch(root, job string) (LaunchFenceResult, error) {
	if !validJobID.MatchString(job) {
		return LaunchFenceResult{}, fmt.Errorf("fence-after-launch job must be a valid job id")
	}
	_, recordPath, _ := paths(root, job)
	record, err := readObject(recordPath)
	if err != nil {
		return LaunchFenceResult{}, fmt.Errorf("fence-after-launch cannot read job %s: %w", job, err)
	}
	generation, ok := numInt(record["fenceGeneration"])
	if !ok || generation < 0 {
		return LaunchFenceResult{}, fmt.Errorf("fence-after-launch job %s has no valid fenceGeneration", job)
	}
	fence, err := stopfence.Read(root)
	if err != nil {
		return LaunchFenceResult{}, fmt.Errorf("fence-after-launch cannot read the process-creation fence: %w", err)
	}
	result := LaunchFenceResult{
		Outcome:            LaunchFenceOpen,
		FenceGeneration:    generation,
		ObservedGeneration: fence.Generation,
	}
	if fence.State == stopfence.StateClosed {
		result.Outcome = LaunchFenceRefusedStopped
		result.Detail, err = stoppedFenceDetail(fence, root, "job "+job)
		if err != nil {
			return LaunchFenceResult{}, err
		}
	} else if fence.Generation != generation {
		result.Outcome = LaunchFenceRefusedStopped
		result.Detail = rearmedFenceDetail(fence, root, "job "+job)
	}
	return result, nil
}

func stoppedClaimResult(fingerprint LaunchFingerprint, fence stopfence.Record, root, thing string) (ClaimResult, error) {
	result := claimResult(ClaimRefusedStopped, fingerprint, map[string]any{
		"resolution":        "checkout-process-creation-fence-closed",
		"refusalClass":      "stopped",
		"fenceGeneration":   fence.Generation,
		"fenceChangedAt":    fence.ChangedAt,
		"creationClaimPath": "",
	})
	var err error
	if thing == "" {
		result.Detail, err = stoppedCheckoutDetail(fence, root)
		return result, err
	}
	result.Detail, err = stoppedFenceDetail(fence, root, thing)
	return result, err
}

func stoppedCheckoutDetail(fence stopfence.Record, root string) (string, error) {
	checkout := fence.Checkout
	if checkout == "" {
		checkout = root
	}
	description, err := stopfence.ClosedDescription(fence, checkout)
	if err != nil {
		return "", err
	}
	command, err := stopfence.ClosedCommand(fence, checkout)
	if err != nil {
		return "", err
	}
	return description + "\nat an agent-free terminal, run: " + command, nil
}

func stoppedFenceDetail(fence stopfence.Record, root, thing string) (string, error) {
	checkout := fence.Checkout
	if checkout == "" {
		checkout = root
	}
	description, err := stopfence.ClosedDescription(fence, checkout)
	if err != nil {
		return "", err
	}
	command, err := stopfence.ClosedCommand(fence, checkout)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s; while %s started, it has been ended\nat an agent-free terminal, run: %s", description, thing, command), nil
}

func rearmedFenceDetail(fence stopfence.Record, root, thing string) string {
	checkout := fence.Checkout
	if checkout == "" {
		checkout = root
	}
	return fmt.Sprintf("the checkout %s was stopped and armed again while %s started; %s has been ended; the caller may retry", checkout, thing, thing)
}
