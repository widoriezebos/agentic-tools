package testrun

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

type stewardRearmDependencies struct {
	now      func() time.Time
	facts    func() (landedRearmFacts, error)
	boundary func() (bool, error)
	lock     func() (func(), error)
	attempts func() ([]proofrun.Attempt, error)
	forward  func(string) error
	rebuild  func() error
	start    func() error
}

// RearmStewardAtBoundary uses the seat's own fetch and build. The replacement
// up runs detached: the resident runner must exit before up can replace it.
func RearmStewardAtBoundary(projectRoot string, boundary func() (bool, error)) (replaced bool, err error) {
	defer func() {
		if err != nil {
			_ = steward.NoteRearmFailure(projectRoot, err, time.Now())
			replaced, err = false, nil
		}
	}()
	installation := projectRoot
	for _, path := range []string{"go.mod", "cmd/devgate/main.go", "cmd/metasystem/main.go"} {
		info, err := os.Stat(filepath.Join(installation, path))
		if os.IsNotExist(err) || (err == nil && !info.Mode().IsRegular()) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
	}
	ctx := context.Background()
	prefix, err := (gittree.Workspace{Dir: installation}).Prefix()
	if err != nil {
		return false, nil
	}
	installed, err := steward.VerifyIdentity(steward.RepoIdentityPath(projectRoot), projectRoot)
	if err != nil {
		return false, err
	}
	source := installed.EngineBuild
	if installed.LandedCommit != "" {
		source = installed.LandedCommit
	}
	return rearmStewardWithDependencies(projectRoot, stewardRearmDependencies{
		now: time.Now,
		facts: func() (landedRearmFacts, error) {
			return readLandedRearmFacts(ctx, landedRearmClock, steward.RearmResolveSeconds(installation), installation, projectRoot, prefix, source, func(tip string) (bool, error) { return false, nil })
		},
		boundary: boundary,
		lock:     func() (func(), error) { return landedRearmMutationLock(installation) },
		attempts: func() ([]proofrun.Attempt, error) { return proofrun.ReadAttempts(installation) },
		forward:  func(tip string) error { return landedRearmFastForward(ctx, installation, tip) },
		rebuild:  func() error { return landedRearmRebuild(ctx, installation) },
		start: func() error {
			log, err := os.OpenFile(filepath.Join(projectRoot, "artifacts", "agents", "steward", "runner.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
			if err != nil {
				return err
			}
			defer log.Close()
			command := exec.Command(filepath.Join(installation, "bin", "metasystem"), "up", "--repo", projectRoot)
			command.Dir, command.Stdout, command.Stderr = installation, log, log
			command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
			if err := command.Start(); err != nil {
				return err
			}
			return command.Process.Release()
		},
	})
}

func rearmStewardWithDependencies(root string, deps stewardRearmDependencies) (replaced bool, err error) {
	defer func() {
		if err != nil {
			_ = steward.NoteRearmFailure(root, err, deps.now())
			replaced, err = false, nil
		}
	}()
	facts, err := deps.facts()
	if errors.Is(err, steward.ErrNoLandingRef) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if facts.Head == facts.Tip && facts.Source != "" && strings.HasPrefix(facts.Tip, facts.Source) {
		return false, steward.ClearDeferredRearm(root)
	}
	if facts.Head != facts.Tip {
		if err := steward.NoteDeferredRearm(root, facts.Tip, facts.Head, deps.now()); err != nil {
			return false, err
		}
	}
	ready, err := deps.boundary()
	if err != nil || !ready {
		return false, err
	}
	facts.SourceOwnsTip = false
	if decision := decideLandedRearm(facts, root); decision.Refusal != nil {
		return false, decision.Refusal
	}
	unlock, err := deps.lock()
	if err != nil {
		return false, err
	}
	defer unlock()
	ready, err = deps.boundary()
	if err != nil || !ready {
		return false, err
	}
	attempts, err := deps.attempts()
	if err != nil {
		return false, err
	}
	for _, attempt := range attempts {
		if attempt.Terminal == nil {
			return false, nil
		}
	}
	if err := deps.forward(facts.Tip); err != nil {
		return false, err
	}
	if err := deps.rebuild(); err != nil {
		return false, err
	}
	if err := deps.start(); err != nil {
		return false, err
	}
	return true, nil
}
