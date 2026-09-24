package steward

import (
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// runnerTick uses the accepted goal projection and the runner's real evidence
// store while keeping repository mark reads scoped to the fixture.
func (b *decisionTickRepository) runnerTick() func(string, TickConfig, WorkerCensus) (TickResult, error) {
	expectedRoot := canonicalPath(b.root)
	return func(root string, cfg TickConfig, census WorkerCensus) (TickResult, error) {
		if root != expectedRoot {
			return TickResult{}, fmt.Errorf("tick root %q, want %q", root, expectedRoot)
		}
		path := EvidencePath(root)
		prev, err := LoadEvidence(path)
		if err != nil {
			return TickResult{}, err
		}
		reads := 0
		want := [][]string{
			{"rev-parse", "HEAD"},
			{"rev-parse", "--verify", "--quiet", "refs/metasystem/goals/accepted"},
		}
		values := []string{b.head, b.acceptedTip()}
		marks, err := currentMarksWithReader(root, func(readRoot string, args ...string) ([]byte, error) {
			if readRoot != root || reads >= len(want) || !reflect.DeepEqual(args, want[reads]) {
				return nil, fmt.Errorf("unexpected mark read: root=%q args=%v index=%d", readRoot, args, reads)
			}
			value := values[reads]
			reads++
			return []byte(value + "\n"), nil
		})
		if err != nil {
			return TickResult{}, err
		}
		if reads != len(want) {
			return TickResult{}, fmt.Errorf("mark reads: consumed %d of %d", reads, len(want))
		}
		routed, workReads := false, 0
		var routeErr error
		work := openWorkDependencies{
			NewWorld: func(readRoot string) bool {
				if readRoot != root || routed || workReads != 0 {
					routeErr = fmt.Errorf("unexpected goal world route: root=%q routed=%t reads=%d", readRoot, routed, workReads)
					return false
				}
				routed = true
				return true
			},
			ReadClaimableBudgetedWork: func(readRoot string, now time.Time) (goal.ClaimableBudgetedWork, error) {
				if readRoot != root || !routed || workReads != 0 || now.IsZero() {
					return goal.ClaimableBudgetedWork{}, fmt.Errorf("unexpected accepted goal read: root=%q routed=%t reads=%d time=%s", readRoot, routed, workReads, now)
				}
				workReads++
				tree, problems := goal.ParseTreeFiles(b.files)
				if len(problems) != 0 {
					return goal.ClaimableBudgetedWork{}, fmt.Errorf("accepted goal files: %v", problems)
				}
				return goal.ClaimableWorkFromProjection(goal.Projection{
					Root: root, Tree: tree, Horizon: goal.ApprovalHorizon{Now: now},
				}, "bed-m1", identity.KernelProber{})
			},
		}
		result, err := decideTickWithDependencies(root, cfg, census, prev, marks, work)
		if routeErr != nil {
			return TickResult{}, routeErr
		}
		if err != nil {
			return TickResult{}, err
		}
		if !routed || workReads != 1 {
			return TickResult{}, fmt.Errorf("accepted goal reads: routed=%t consumed=%d of 1", routed, workReads)
		}
		if err := SaveEvidence(root, path, result.Evidence); err != nil {
			return TickResult{}, err
		}
		return result, nil
	}
}

func runnerPendingDelivery(expectedRoot, command string) func(string) (int, error) {
	expectedRoot = canonicalPath(expectedRoot)
	return func(root string) (int, error) {
		if root != expectedRoot {
			return 0, fmt.Errorf("delivery root %q, want %q", root, expectedRoot)
		}
		deps := notificationDependencies{
			configuredCommand: func(readRoot string) ([]byte, error) {
				if readRoot != expectedRoot {
					return nil, fmt.Errorf("notification config root %q, want %q", readRoot, expectedRoot)
				}
				return []byte(command + "\n"), nil
			},
			platform: "linux", commandContext: exec.CommandContext,
		}
		return deliverPendingWith(root, func(deliveryRoot, message string) error {
			return deliverWithDependencies(deliveryRoot, message, deps)
		})
	}
}

func stopRunnerLoop(root string) error {
	return os.WriteFile(runnerStopPath(root), []byte("stop\n"), 0o644)
}
