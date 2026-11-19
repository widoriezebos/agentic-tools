package launch

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"
)

var ErrCheckUnsettled = errors.New("the check process cleanup is unconfirmed")

// checkProcess owns timing only. The command owner settles its custody before
// returning, including descendants that outlive the direct process.
func checkProcess(ctx context.Context, command *exec.Cmd, progress <-chan struct{}, idle, budget time.Duration, clock CheckContext) (string, error) {
	if clock.RunCommand == nil {
		return "environment", fmt.Errorf("the check command owner is unavailable")
	}
	running, cancel := context.WithCancel(ctx)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- clock.RunCommand(running, command) }()
	deadline := clock.After(idle)
	ceiling := clock.After(checkCeilingFactor * idle)
	var limit <-chan time.Time
	if budget > 0 {
		limit = clock.After(budget)
	}
	completed := func(err error) (string, error) {
		if errors.Is(err, ErrCheckUnsettled) {
			return "environment", err
		}
		return "completed", err
	}
	reason := ""
	for reason == "" {
		select {
		case err := <-done:
			return completed(err)
		case <-ctx.Done():
			reason = "cancelled"
		case <-progress:
			deadline = clock.After(idle)
		case <-deadline:
			reason = "deadline"
		case <-ceiling:
			reason = "deadline"
		case <-limit:
			select {
			case err := <-done:
				return completed(err)
			default:
				select {
				case <-deadline:
					reason = "deadline"
				case <-ceiling:
					reason = "deadline"
				default:
					reason = "budget"
				}
			}
		}
	}
	cancel()
	err := <-done
	if errors.Is(err, ErrCheckUnsettled) {
		return "environment", err
	}
	if reason == "deadline" {
		err = fmt.Errorf("the check stopped making progress")
	}
	return reason, err
}
