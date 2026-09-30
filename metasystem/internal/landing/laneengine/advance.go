package laneengine

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/candidateengine"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/enginebuild"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

// AdvanceRequest is one `landing engine advance`: the host home, the lane
// checkout (the Git toplevel, where the batch records live), the lane
// installation (the module root, where the enrollment lives) and the
// admission RequireSelf gave the verb.
type AdvanceRequest struct {
	Home, Checkout, Installation string
	Identity                     Identity
}

// AdvanceOutcome is what an advance did. Changed is false when the enrolled
// engine already is landed main's build and its bytes are in place.
type AdvanceOutcome struct {
	Changed                        bool
	Commit                         string
	PreviousGeneration, Generation int
}

// Conditions are the lane states an advance waits for. Each answers the
// blocking thing in words, "" when it does not block; an error blocks too.
type Conditions struct {
	// Paused reads the person's pause under the host flock. An unreadable
	// pause is paused (K2). Unit K-a's lane gate replaces it.
	Paused func() (bool, error)
	// BatchInFlight names a batch that has begun and not finished.
	BatchInFlight func() (string, error)
	// CustodyLive names a live or unknown execution custody (K9). Until unit
	// K-f's custody store exists, the host's proving flock is the custody.
	CustodyLive func() (string, error)
}

// Steps are the effects of an advance.
type Steps struct {
	// FetchMain fetches origin's main into refs/remotes/origin/main and
	// returns its commit: the landed main.
	FetchMain func() (string, error)
	// Head is the lane checkout's HEAD commit.
	Head func() (string, error)
	// Build builds the checkout's engine into staging.
	Build func(staging string) error
	// ReArm re-enrolls the engine now at installPath (the steward's machine
	// rebuild re-arm, which itself refuses a build not landed).
	ReArm func(installPath string) (steward.ReArmOutcome, error)
}

// pausePath is the lane's pause record. It is lane's own file, read here
// fail-closed until unit K-a's gate owns the read.
func pausePath(home string) string {
	return filepath.Join(lane.HostDir(home), "landing-lane-paused.json")
}

// ProductionConditions reads the host's pause, the lane checkout's batch
// records and the proving flock.
func ProductionConditions(home, checkout string) Conditions {
	return Conditions{
		Paused: func() (bool, error) {
			if err := os.MkdirAll(lane.HostDir(home), 0o700); err != nil {
				return true, err
			}
			held, err := lock.File(lane.LockPath(home), 0o600, lock.Exclusive)
			if err != nil {
				return true, err
			}
			defer held.Release()
			_, err = os.ReadFile(pausePath(home))
			if errors.Is(err, fs.ErrNotExist) {
				return false, nil
			}
			return true, err
		},
		BatchInFlight: func() (string, error) {
			records, err := batch.NewStore(checkout, identity.KernelProber{}).Records()
			if err != nil {
				return "", err
			}
			for _, record := range records {
				switch record.State {
				case batch.StateOpen, batch.StateLanded, batch.StateDissolved:
				default:
					return "batch " + record.BatchID + " is " + record.State, nil
				}
			}
			return "", nil
		},
		CustodyLive: func() (string, error) {
			holder, busy, err := lane.ProbeProving(home)
			if err != nil || !busy {
				return "", err
			}
			return "a test run (" + holder + ") holds the host's proving lock", nil
		},
	}
}

const (
	gitDeadline   = 2 * time.Minute
	buildDeadline = 20 * time.Minute
)

func gitOutput(dir string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), gitDeadline)
	defer cancel()
	var stderr bytes.Buffer
	command := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	command.Stderr = &stderr
	out, err := command.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w (%s)", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(string(out)), nil
}

// ProductionSteps fetch and read Git in the lane checkout, build with the
// landed tree's own stamped build (cmd/devgate), and re-arm through the
// steward.
func ProductionSteps(checkout, installation string) Steps {
	return Steps{
		FetchMain: func() (string, error) {
			if _, err := gitOutput(checkout, "fetch", "--quiet", "origin", "+refs/heads/main:refs/remotes/origin/main"); err != nil {
				return "", err
			}
			return gitOutput(checkout, "rev-parse", "--verify", "refs/remotes/origin/main^{commit}")
		},
		Head: func() (string, error) { return gitOutput(checkout, "rev-parse", "--verify", "HEAD^{commit}") },
		Build: func(staging string) error {
			ctx, cancel := context.WithTimeout(context.Background(), buildDeadline)
			defer cancel()
			argv := candidateengine.DefaultBuildArgv(staging)
			command := exec.CommandContext(ctx, argv[0], argv[1:]...)
			command.Dir = installation
			var output bytes.Buffer
			command.Stdout, command.Stderr = &output, &output
			if err := command.Run(); err != nil {
				tail := strings.TrimSpace(output.String())
				if len(tail) > 2000 {
					tail = tail[len(tail)-2000:]
				}
				return fmt.Errorf("build the landed engine: %w (%s)", err, tail)
			}
			return nil
		},
		ReArm: func(installPath string) (steward.ReArmOutcome, error) {
			return steward.ReArmRebuiltEngine(installation, installation, installPath)
		},
	}
}

// gate refuses while the lane is paused, a batch is in flight or custody is
// live or unknown. It runs before the build and again immediately before the
// installed engine changes.
func gate(conditions Conditions, again []string) error {
	paused, err := conditions.Paused()
	if paused || err != nil {
		refusal := &Refusal{Code: CodeAdvancePaused, Message: "the landing lane is stopped, so its engine wasn't changed",
			Argv: []string{"metasystem", "landing", "status"}, Detail: "metasystem landing status shows who stopped it; only a person resumes it"}
		if err != nil {
			refusal.Message = "whether the landing lane is stopped can't be read, so its engine wasn't changed"
			refusal.Detail = err.Error()
		}
		return refusal
	}
	busy, err := conditions.BatchInFlight()
	if busy != "" || err != nil {
		refusal := &Refusal{Code: CodeAdvanceBatchInFlight, Message: "a landing batch is underway, so the engine wasn't changed; it moves only between batches",
			Argv: []string{"metasystem", "landing", "status"}, Detail: busy + "; metasystem landing status shows it"}
		if err != nil {
			refusal.Message = "the landing batches can't be read, so the engine wasn't changed; it moves only between batches"
			refusal.Detail = err.Error()
		}
		return refusal
	}
	live, err := conditions.CustodyLive()
	if live != "" || err != nil {
		refusal := &Refusal{Code: CodeAdvanceCustodyLive, Message: "landing tests are still running, so the engine wasn't changed",
			Argv: again, Detail: live + "; run metasystem landing engine advance again once it ends"}
		if err != nil {
			refusal.Message = "whether landing tests still run is unknown, so the engine wasn't changed"
			refusal.Detail = err.Error()
		}
		return refusal
	}
	return nil
}

// Advance moves the lane to the engine built from landed origin/main: the
// one way the lane's engine changes. It refuses while paused, while a batch
// is in flight and while custody is live; it builds only when the checkout's
// HEAD is the fetched landed main, installs only a build stamped with that
// commit, and puts the enrolled bytes back when the re-arm does not take.
func Advance(request AdvanceRequest, conditions Conditions, steps Steps) (AdvanceOutcome, error) {
	again := []string{"metasystem", "landing", "engine", "advance"}
	enrolled := request.Identity.Enrolled
	if enrolled.InstallPath == "" || enrolled.InstallDigest == "" {
		return AdvanceOutcome{}, errors.New("the advance was not admitted by the lane's enrolled engine")
	}
	if err := gate(conditions, again); err != nil {
		return AdvanceOutcome{}, err
	}
	main, err := steps.FetchMain()
	if err != nil {
		return AdvanceOutcome{}, fmt.Errorf("fetch landed main: %w", err)
	}
	head, err := steps.Head()
	if err != nil {
		return AdvanceOutcome{}, fmt.Errorf("read the lane checkout's HEAD: %w", err)
	}
	if head != main {
		return AdvanceOutcome{}, &Refusal{Code: CodeAdvanceNotLanded,
			Message: "the lane checkout isn't on landed main, so its engine wasn't changed",
			Argv:    []string{"git", "-C", request.Checkout, "checkout", "--detach", main},
			Detail:  fmt.Sprintf("HEAD %s, origin/main %s; check out landed main, then metasystem landing engine advance again", head, main)}
	}
	outcome := AdvanceOutcome{Commit: main, PreviousGeneration: enrolled.Generation, Generation: enrolled.Generation}
	if installed, err := fileDigest(enrolled.InstallPath); err == nil && installed == enrolled.InstallDigest && enrolled.EngineBuild == main {
		return outcome, nil
	}
	dir := filepath.Dir(enrolled.InstallPath)
	suffix := strconv.Itoa(os.Getpid())
	staging := filepath.Join(dir, ".metasystem.advance."+suffix)
	saved := filepath.Join(dir, ".metasystem.enrolled."+suffix)
	defer func() { _ = os.Remove(staging) }()
	if err := steps.Build(staging); err != nil {
		return AdvanceOutcome{}, err
	}
	stamp, err := readStamp(staging)
	if err != nil {
		return AdvanceOutcome{}, fmt.Errorf("read the built engine's stamp: %w", err)
	}
	if stamp != main {
		return AdvanceOutcome{}, &Refusal{Code: CodeAdvanceNotLanded,
			Message: "the engine built in the lane checkout isn't landed main's, so it wasn't installed",
			Argv:    []string{"git", "-C", request.Checkout, "status"},
			Detail:  fmt.Sprintf("built stamp %q, landed main %s: the checkout's engine files differ from main", stamp, main)}
	}
	if err := gate(conditions, again); err != nil {
		return AdvanceOutcome{}, err
	}
	if err := os.Link(enrolled.InstallPath, saved); err != nil {
		return AdvanceOutcome{}, fmt.Errorf("keep the enrolled engine while the new one is armed: %w", err)
	}
	defer func() { _ = os.Remove(saved) }()
	if err := os.Rename(staging, enrolled.InstallPath); err != nil {
		return AdvanceOutcome{}, fmt.Errorf("install the landed engine: %w", err)
	}
	rearmed, err := steps.ReArm(enrolled.InstallPath)
	if err == nil && rearmed.Status != "re-armed" && rearmed.Status != "already-current" {
		err = fmt.Errorf("the steward did not arm the landed engine (status %q)", rearmed.Status)
	}
	if err != nil {
		if restoreErr := os.Rename(saved, enrolled.InstallPath); restoreErr != nil {
			return AdvanceOutcome{}, fmt.Errorf("re-arm the landed engine: %w; restoring the enrolled engine failed too: %v", err, restoreErr)
		}
		return AdvanceOutcome{}, fmt.Errorf("re-arm the landed engine: %w; the enrolled engine is back in place", err)
	}
	outcome.Changed = true
	if rearmed.Generation > 0 {
		outcome.Generation = rearmed.Generation
	}
	return outcome, nil
}

func readStamp(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	return enginebuild.ReadStamp(file)
}
