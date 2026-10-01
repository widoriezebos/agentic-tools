package laneengine

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/candidateengine"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/enginebuild"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/custody"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/custody/laneprobe"
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
	// Force is a person's word past custody whose state can't be read; it
	// never goes past custody that is live. By names the person: the
	// custody records they went past are recorded settled in their name.
	Force bool
	By    string
}

// AdvanceOutcome is what an advance did. Changed is false when the enrolled
// engine already is landed main's build and its bytes are in place.
type AdvanceOutcome struct {
	Changed                        bool
	Commit                         string
	PreviousGeneration, Generation int
}

// Conditions are the lane states an advance waits for, read under the locks
// Hold takes. Each answers the blocking thing in words, "" when it does not
// block; an error blocks too.
type Conditions struct {
	// Hold takes, without waiting, the host's proving lock (landing prove's
	// proofs hold it), then the host
	// flock every gated operation reads the pause under (K2),
	// and keeps both until release. The second check, the install and the
	// re-arm all run inside one Hold, so no proof or begin starts on either
	// engine while the engine changes.
	Hold func() (release func(), err error)
	// Admit is the lane gate's reading for an advance (lane.AdmitHeld): the
	// lane record, the unset fence and the person's pause, each failing
	// closed; it runs with the host flock already held. A *lane.Refusal
	// names what holds the lane.
	Admit func() error
	// BatchInFlight names a batch that has begun and not finished.
	BatchInFlight func() (string, error)
	// Custody reads the lane's one custody barrier (K9): every execution
	// the kernel launched and the installation's proof leases.
	Custody func() (custody.Settlement, error)
	// Override records a person's word past the custody records that read
	// unknown.
	Override func(by string) ([]string, error)
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

// ProductionConditions holds the host's proving lock and flock, and reads the
// host's pause and the lane checkout's batch records.
func ProductionConditions(home, checkout string) Conditions {
	// The lease filter needs the lane's installation; a layout that can't be
	// read leaves it unknown, which reads the leases as unknown custody.
	installation := ""
	if layout, err := lane.NewLayout(checkout); err == nil {
		installation = string(layout.Install)
	}
	return Conditions{
		// The advance holds the proving lock itself, so custody reads the
		// store and the leases only.
		Custody: func() (custody.Settlement, error) {
			return custody.Settle(home, laneprobe.Production(home, installation, false))
		},
		Override: func(by string) ([]string, error) {
			return custody.Override(home, by, laneprobe.Production(home, installation, false))
		},
		Hold: func() (func(), error) {
			if err := os.MkdirAll(lane.HostDir(home), 0o700); err != nil {
				return nil, err
			}
			proving, err := lock.File(lane.ProvingPath(home), 0o600, lock.TryExclusive)
			if err != nil {
				if lock.Busy(err) {
					return nil, &Refusal{Code: CodeAdvanceCustodyLive, Message: "landing tests are still running, so the engine wasn't changed",
						Argv:   []string{"metasystem", "landing", "engine", "advance"},
						Detail: "a test run (" + lane.ProvingHolder(home) + ") holds the host's proving lock; run metasystem landing engine advance again once it ends"}
				}
				return nil, err
			}
			// The holder's pid, as lane.HoldProving writes it, so status
			// names the advance as the holder.
			if file := proving.File(); file.Truncate(0) == nil {
				_, _ = file.WriteAt([]byte(strconv.Itoa(os.Getpid())+"\n"), 0)
			}
			host, err := lock.File(lane.LockPath(home), 0o600, lock.Exclusive)
			if err != nil {
				_ = proving.Release()
				return nil, err
			}
			return func() { _ = host.Release(); _ = proving.Release() }, nil
		},
		Admit: func() error {
			_, err := lane.AdmitHeld(home, lane.OpAdvance, lane.AuthorityAgent)
			return err
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
			command.Dir, command.Env = installation, buildEnvironment(os.Environ())
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

// buildEnvironment is the build's environment: the stamp is always the
// commit the build classifies, never an inherited override.
func buildEnvironment(environ []string) []string {
	kept := make([]string, 0, len(environ))
	for _, entry := range environ {
		if !strings.HasPrefix(entry, "METASYSTEM_BUILD_STAMP=") {
			kept = append(kept, entry)
		}
	}
	return kept
}

// gate takes the Hold and refuses while the lane is paused, a batch is in
// flight or custody is live or unknown. On success the caller owns release.
func gate(conditions Conditions, again []string, request AdvanceRequest) (func(), error) {
	release, err := conditions.Hold()
	if err != nil {
		if _, ok := err.(*Refusal); ok {
			return nil, err
		}
		return nil, &Refusal{Code: CodeAdvanceCustodyLive, Message: "whether landing tests still run is unknown, so the engine wasn't changed",
			Argv: again, Detail: err.Error()}
	}
	if err := check(conditions, again, request); err != nil {
		release()
		return nil, err
	}
	return release, nil
}

// admitRefusal is the lane gate's refusal as the advance's: a pause (a
// readable one or not) is CodeAdvancePaused, any other lane refusal keeps
// the lane's code and fix, and a lane that can't be read refuses too.
func admitRefusal(err error) *Refusal {
	status := []string{"metasystem", "landing", "status"}
	var held *lane.Refusal
	switch {
	case errors.As(err, &held) && held.Code == lane.CodePaused:
		return &Refusal{Code: CodeAdvancePaused, Message: "the landing lane is stopped, so its engine wasn't changed",
			Argv: status, Detail: "metasystem landing status shows who stopped it; only a person resumes it"}
	case errors.As(err, &held):
		argv := held.Argv
		if len(argv) == 0 {
			argv = status
		}
		return &Refusal{Code: held.Code, Message: held.Message, Argv: argv, Detail: held.Fix}
	}
	return &Refusal{Code: CodeAdvancePaused, Message: "whether the landing lane is stopped can't be read, so its engine wasn't changed",
		Argv: status, Detail: err.Error()}
}

func check(conditions Conditions, again []string, request AdvanceRequest) error {
	if err := conditions.Admit(); err != nil {
		return admitRefusal(err)
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
	if conditions.Custody == nil {
		return &Refusal{Code: CodeAdvanceCustodyUnknown, Message: "whether landing work still runs can't be read, so the engine wasn't changed",
			Argv: []string{"metasystem", "landing", "status"}, Detail: "no custody read is wired"}
	}
	settlement, err := conditions.Custody()
	if err != nil {
		settlement.Unknown = append(settlement.Unknown, err.Error())
	}
	if len(settlement.Live) > 0 {
		return &Refusal{Code: CodeAdvanceCustodyLive, Message: "landing work is still running, so the engine wasn't changed",
			Argv: again, Detail: strings.Join(settlement.Live, "; ") + "; run metasystem landing engine advance again once it ends"}
	}
	if len(settlement.Unknown) > 0 && !request.Force {
		return &Refusal{Code: CodeAdvanceCustodyUnknown, Message: "whether landing work still runs can't be read, so the engine wasn't changed",
			Argv: append(slices.Clone(again), "--force"), Detail: strings.Join(settlement.Unknown, "; ") + "; a person who has checked it goes past it with --force"}
	}
	if len(settlement.Unknown) > 0 && conditions.Override != nil {
		if _, err := conditions.Override(request.By); err != nil {
			return &Refusal{Code: CodeAdvanceCustodyUnknown, Message: "the person's word past unknown landing work couldn't be recorded, so the engine wasn't changed",
				Argv: append(slices.Clone(again), "--force"), Detail: err.Error()}
		}
	}
	return nil
}

// leftovers removes what an earlier advance that died left beside the
// enrolled engine: a staged build or a kept enrolled copy whose process no
// longer runs. It runs under the Hold.
func leftovers(dir string) {
	for _, prefix := range []string{".metasystem.advance.", ".metasystem.enrolled."} {
		paths, _ := filepath.Glob(filepath.Join(dir, prefix+"*"))
		for _, path := range paths {
			pid, err := strconv.Atoi(strings.TrimPrefix(filepath.Base(path), prefix))
			if err != nil || pid <= 0 || pid == os.Getpid() {
				continue
			}
			if errors.Is(syscall.Kill(pid, 0), syscall.ESRCH) {
				_ = os.Remove(path)
			}
		}
	}
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
	release, err := gate(conditions, again, request)
	if err != nil {
		return AdvanceOutcome{}, err
	}
	release()
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
	release, err = gate(conditions, again, request)
	if err != nil {
		return AdvanceOutcome{}, err
	}
	defer release()
	leftovers(dir)
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
		return AdvanceOutcome{}, afterFailedReArm(request, enrolled, saved, rearmed, err)
	}
	outcome.Changed = rearmed.Status == "re-armed"
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

// afterFailedReArm decides what a failed re-arm leaves. The old bytes go
// back only while the enrollment still names them (the steward failed before
// it wrote the new enrollment); once the enrollment names the new engine, the
// new bytes stay, because restoring the old ones would leave an enrollment
// that names bytes no longer installed.
func afterFailedReArm(request AdvanceRequest, previous steward.InstallIdentity, saved string, rearmed steward.ReArmOutcome, cause error) error {
	current, readErr := Enrollment(request.Installation)
	if rearmed.Stage < steward.StageMinted && readErr == nil && current.InstallDigest == previous.InstallDigest {
		if restoreErr := os.Rename(saved, previous.InstallPath); restoreErr != nil {
			return fmt.Errorf("re-arm the landed engine: %w; restoring the enrolled engine failed too: %v", cause, restoreErr)
		}
		if rearmed.Stage < steward.StageStopAttempted {
			return fmt.Errorf("re-arm the landed engine: %w; the enrolled engine is back in place", cause)
		}
		// The steward stopped (or tried to stop) the old runner before it
		// failed: the enrolled bytes are back, but nothing guards the lane.
		return &Refusal{Code: CodeAdvanceRunnerDown,
			Message: "the landing lane's engine is back as it was, but its steward isn't running",
			Argv:    []string{"metasystem", "system", "start", "--repo", request.Checkout},
			Detail:  fmt.Sprintf("the re-arm stopped the old runner, then failed: %v; a person at a terminal no agent started runs metasystem system start", cause)}
	}
	detail := fmt.Sprintf("the steward re-arm failed after writing the new enrollment: %v", cause)
	if readErr == nil {
		detail = fmt.Sprintf("engine number %d from %s is enrolled and stays installed; its re-arm failed: %v",
			current.Generation, current.EngineBuild, cause)
	} else {
		detail += "; the enrollment can't be read: " + readErr.Error()
	}
	return &Refusal{Code: CodeAdvanceRunnerDown,
		Message: "the landing lane's new engine is enrolled but its steward didn't start",
		Argv:    []string{"metasystem", "system", "start", "--repo", request.Checkout},
		Detail:  detail + "; a person at a terminal no agent started runs metasystem system start"}
}
