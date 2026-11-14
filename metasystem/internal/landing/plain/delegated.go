package plain

import (
	"errors"
	"fmt"
	"os/exec"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
)

// DelegatedRecoveryHeld checks the sole hold a delegate may recover.
// The caller must hold the queue lock followed by the lane lock.
func DelegatedRecoveryHeld(install, home, checkout string, fingerprint func(string) (string, error), s ProveSeams) error {
	held, err := lane.BarrenHeld(home, fingerprint, checkout)
	if err != nil {
		return fmt.Errorf("the barren hold cannot be read: %w", err)
	}
	if !held {
		return errors.New("the lane is not on an unchanged barren hold")
	}
	if _, paused := lane.ReadPause(home); paused {
		return errors.New("a pause prevents delegated recovery")
	}
	stops, err := OpenStops(install)
	if err != nil {
		return fmt.Errorf("the lane stops cannot be read: %w", err)
	}
	for i, stop := range stops {
		if i != 0 || stop.Loop != "lane-return" || stop.Subject != "lane" || stop.Class != "lane unchanged" || stop.Measure.Name != "lane fingerprint" {
			return errors.New("a stop other than the engine's barren stop prevents delegated recovery")
		}
	}
	main, err := checkoutGit(checkout, s).main()
	if err != nil {
		return err
	}
	incidents, err := s.incidents(install, checkout, main)
	if err != nil {
		return fmt.Errorf("the incidents cannot be read: %w", err)
	}
	if len(openIncidents(incidents)) > 0 {
		return errors.New("an open incident prevents delegated recovery")
	}
	_, recorded, live, err := ReadRunning(install, s)
	if err != nil {
		return fmt.Errorf("the check record cannot be read: %w", err)
	}
	if recorded && !live {
		return errors.New("a check whose process is gone prevents delegated recovery")
	}
	if _, err := s.git(checkout, "rev-parse", "--verify", "MERGE_HEAD^{commit}"); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && exit.ExitCode() == 128 {
			return nil
		}
		return fmt.Errorf("the merge state cannot be read: %w", err)
	}
	return errors.New("a merge in progress prevents delegated recovery")
}
