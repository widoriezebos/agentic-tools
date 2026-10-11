package plain

import (
	"fmt"
	"time"
)

// Wait reads the detached attempt's result. A lost process is recovered by
// the same transition that releases its lane slot, so it returns red.
func Wait(install, checkout string, running Running, seams ProveSeams) (Result, error) {
	seams.Gate = running.Gate
	belongs := func(result Result) bool {
		if result.Attempt == running.Attempt {
			return true
		}
		for _, execution := range result.Executions {
			if execution.Attempt == running.Attempt {
				return true
			}
		}
		return false
	}
	for {
		results, err := readLines[Result](seams.resultsPath(install))
		if err != nil {
			return Result{}, err
		}
		for i := len(results) - 1; i >= 0; i-- {
			if belongs(results[i]) && !seams.alive(running) {
				return results[i], nil
			}
		}
		if seams.WaitPoll != nil {
			seams.WaitPoll()
		}
		err = withLock(install, func() error {
			current, recorded, _, err := ReadRunning(install, seams)
			if err != nil {
				return err
			}
			if recorded && current.Attempt == running.Attempt {
				_, _, _, err = checkState(install, checkout, "", seams)
				return err
			}
			results, err := readLines[Result](seams.resultsPath(install))
			if err != nil {
				return err
			}
			for _, result := range results {
				if belongs(result) {
					return nil
				}
			}
			return fmt.Errorf("the waited check ended without a saved result")
		})
		if err != nil {
			return Result{}, err
		}
		if seams.WaitPoll == nil {
			time.Sleep(250 * time.Millisecond)
		}
	}
}
