package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
)

const landingOwnerLineage = "landing-m1l"

func init() { compiledBatchCapabilities[ownerVerbAndTick] = struct{}{} }

type batchOwnerLease struct {
	root, session string
	pid, started  int64
	epoch         int64
}

type batchOwnerEnsureSeams struct {
	inspect func(string) (int64, identity.Liveness, error)
	wake    func(int64) error
	launch  func(string) error
}

var batchOwnerEnsure = batchOwnerEnsureSeams{
	inspect: inspectBatchOwner,
	wake:    func(pid int64) error { return syscall.Kill(int(pid), syscall.SIGUSR1) },
	launch:  launchBatchOwner,
}

func inspectBatchOwner(root string) (int64, identity.Liveness, error) {
	return inspectBatchOwnerWith(root, identity.KernelProber{}, lease.CurrentHolder, lease.AnnouncementsFor)
}

func inspectBatchOwnerWith(root string, prober identity.Prober, readHolder func(string) (lease.CurrentHolderView, error), announcements func(string, int64) []lease.Announcement) (int64, identity.Liveness, error) {
	holder, err := readHolder(root)
	if err != nil {
		if errors.Is(err, lease.ErrLeaseAbsent) {
			return 0, identity.Dead, nil
		}
		return 0, identity.Unknown, err
	}
	if holder.OwnerLineage != landingOwnerLineage {
		return holder.Pid, identity.Unknown, fmt.Errorf("landing checkout is held by lineage %s", holder.OwnerLineage)
	}
	for _, announcement := range announcements(root, holder.Pid) {
		if announcement.MainId != holder.MainId {
			continue
		}
		ref := identity.Ref{Pid: announcement.Pid, StartedAtSec: announcement.PidStartedAt,
			StartTicks: announcement.PidStartTicks, BootID: announcement.BootID}
		return holder.Pid, identity.AliveRef(prober, ref), nil
	}
	return holder.Pid, identity.Unknown, fmt.Errorf("landing owner announcement is absent")
}

func launchBatchOwner(root string) error {
	binary, err := os.Executable()
	if err != nil {
		return err
	}
	command := exec.Command(binary, "up", "--recover-only", "--if-down", "--repo", root, "--metasystem-root", root)
	command.Dir = root
	command.Env = os.Environ()
	command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	command.Stdin, command.Stdout, command.Stderr = nil, nil, nil
	return command.Start()
}

func ensureBatchOwner(root string) error {
	lockPath := filepath.Join(root, "artifacts", "agents", "locks", "landing-owner.ensure.lock")
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		return err
	}
	lock, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := unix.Flock(int(lock.Fd()), unix.LOCK_EX); err != nil {
		return err
	}
	defer unix.Flock(int(lock.Fd()), unix.LOCK_UN)
	pid, state, err := batchOwnerEnsure.inspect(root)
	if err != nil && state == identity.Unknown {
		return fmt.Errorf("BATCH_OWNER_INDETERMINATE: %w", err)
	}
	switch state {
	case identity.Alive:
		return batchOwnerEnsure.wake(pid)
	case identity.Dead:
		return batchOwnerEnsure.launch(root)
	default:
		return fmt.Errorf("BATCH_OWNER_INDETERMINATE: owner liveness is unknown")
	}
}

func acquireBatchOwner(root string) (batchOwnerLease, error) {
	pid := int64(os.Getpid())
	exact, state, err := (identity.KernelProber{}).Probe(pid)
	if err != nil || state != identity.Alive || exact.StartedAt.Unix() < 1 {
		return batchOwnerLease{}, fmt.Errorf("BATCH_OWNER_IDENTITY_UNKNOWN: pid %d state=%s: %v", pid, state, err)
	}
	session := "landing-owner-" + strconv.FormatInt(pid, 10)
	if _, err := lease.AnnounceWithPair(root, session, pid, exact.StartedAt.Unix(), exact.StartTicks, exact.BootID,
		"landing-owner", "metasystem", landingOwnerLineage); err != nil {
		return batchOwnerLease{}, fmt.Errorf("BATCH_OWNER_OWNED_ELSEWHERE: %w", err)
	}
	holder, err := lease.RequireHolder(root, pid, nil)
	if err != nil {
		_ = lease.Retire(root, session, pid, exact.StartedAt.Unix())
		return batchOwnerLease{}, fmt.Errorf("BATCH_OWNER_OWNED_ELSEWHERE: holder proof failed: %w", err)
	}
	if !holder.Holder || holder.ClaimEpoch == nil || holder.MainId == nil {
		_ = lease.Retire(root, session, pid, exact.StartedAt.Unix())
		return batchOwnerLease{}, fmt.Errorf("BATCH_OWNER_OWNED_ELSEWHERE: holder proof returned class=%s holder=%t", holder.Class, holder.Holder)
	}
	if err := os.Setenv("METASYSTEM_OWNER_LINEAGE", landingOwnerLineage); err != nil {
		_ = lease.Retire(root, session, pid, exact.StartedAt.Unix())
		return batchOwnerLease{}, err
	}
	return batchOwnerLease{root: root, session: session, pid: pid, started: exact.StartedAt.Unix(), epoch: *holder.ClaimEpoch}, nil
}

func (held batchOwnerLease) retire() {
	_ = lease.Retire(held.root, held.session, held.pid, held.started)
}

func resolveBatchOwnerSettings(seatRoot, landingRoot string, maxWait time.Duration, now func() time.Time) (config.BatchLanding, error) {
	if landingRoot != "" {
		return config.ResolveExplicitBatchLanding(landingRoot, seatRoot, maxWait, now)
	}
	return config.ResolveBatchLanding(filepath.Join(seatRoot, "metasystem.conf"), seatRoot, now)
}

func fetchBatchTree(root string) (string, error) {
	endpoint, err := goal.ResolveEndpoint(root)
	if err != nil {
		return "", err
	}
	fetched, err := goal.FetchAdvance(endpoint)
	if err != nil {
		return "", err
	}
	return (gittree.Workspace{Dir: root}).TreeOf(fetched.Tip)
}

func goalFilesAt(root, tree string) ([]*goal.GoalFile, error) {
	command := exec.Command("git", "-C", root, "ls-tree", "-r", "--name-only", tree, "--", "plans/goals")
	command.Env = []string{"PATH=" + os.Getenv("PATH"), "LC_ALL=C"}
	out, err := command.Output()
	if err != nil {
		return nil, err
	}
	var files []*goal.GoalFile
	for _, name := range strings.Fields(string(out)) {
		if !strings.HasSuffix(name, ".md") || strings.HasSuffix(name, "/backlog.md") {
			continue
		}
		data, present, readErr := (gittree.Workspace{Dir: root}).FileAt(tree, name)
		if readErr != nil || !present {
			return nil, errors.Join(readErr, fmt.Errorf("goal file %s disappeared from %s", name, tree))
		}
		file, problems := goal.ParseFile(data)
		if len(problems) != 0 {
			return nil, fmt.Errorf("parse %s at %s: %v", name, tree, problems)
		}
		files = append(files, file)
	}
	return files, nil
}

var batchReturnTargetSeams = struct {
	goals         func(string, string) ([]*goal.GoalFile, error)
	holder        func(string) (lease.CurrentHolderView, error)
	announcements func(string, string) []lease.Announcement
}{goalFilesAt, lease.CurrentHolder, lease.AnnouncementsForOwnerLineage}

var batchReturnLedgerGoal = batch.ReadReturnLedgerGoal

func productionReturnTarget(landingRoot, tree string, prober identity.Prober, unit batch.Unit) batch.ReturnTarget {
	files, err := batchReturnTargetSeams.goals(landingRoot, tree)
	if err != nil {
		return batch.ReturnTarget{State: batch.ReturnTargetUnknown, Reason: err.Error()}
	}
	for _, file := range files {
		if file.Id != unit.GoalID && file.Claimed != nil && file.Claimed.Machine == unit.Claim.Machine {
			return batch.ReturnTarget{State: batch.ReturnTargetOccupied, Reason: "source machine holds " + file.Id}
		}
	}
	if unit.SeatRoot == "" {
		return batch.ReturnTarget{State: batch.ReturnTargetUnknown, Reason: "source checkout root is absent"}
	}
	holder, holderErr := batchReturnTargetSeams.holder(unit.SeatRoot)
	announcements := batchReturnTargetSeams.announcements(unit.SeatRoot, unit.Claim.Lineage)
	seenDead := false
	for _, announcement := range announcements {
		ref := identity.Ref{Pid: announcement.Pid, StartedAtSec: announcement.PidStartedAt,
			StartTicks: announcement.PidStartTicks, BootID: announcement.BootID}
		switch identity.AliveRef(prober, ref) {
		case identity.Alive:
			if holderErr == nil && holder.MainId == announcement.MainId && holder.OwnerLineage == unit.Claim.Lineage && holder.ClaimEpoch > 0 {
				return batch.ReturnTarget{State: batch.ReturnTargetLive, Epoch: uint64(holder.ClaimEpoch)}
			}
			return batch.ReturnTarget{State: batch.ReturnTargetRestarted}
		case identity.Dead:
			seenDead = true
		}
	}
	if holderErr == nil && holder.OwnerLineage != "" && holder.OwnerLineage != unit.Claim.Lineage {
		return batch.ReturnTarget{State: batch.ReturnTargetRestarted}
	}
	if seenDead {
		return batch.ReturnTarget{State: batch.ReturnTargetDead}
	}
	return batch.ReturnTarget{State: batch.ReturnTargetUnknown, Reason: "source identity is not provable"}
}

var batchChildRunner = runBatchChildAs

func runBatchChildAs(root, lineage string, args ...string) error {
	binary, err := os.Executable()
	if err != nil {
		return err
	}
	command := exec.Command(binary, args...)
	command.Dir = root
	command.Env = append(os.Environ(), "METASYSTEM_OWNER_LINEAGE="+lineage)
	if output, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("%s: %w", strings.TrimSpace(string(output)), err)
	}
	return nil
}

func productionReturnSeams(root string, tree func() string) batch.ReturnSeams {
	return batch.ReturnSeams{
		Read: batchReturnLedgerGoal,
		Target: func(unit batch.Unit) batch.ReturnTarget {
			return productionReturnTarget(root, tree(), identity.KernelProber{}, unit)
		},
		HandBack: func(goalID string, source batch.Claim, epoch uint64) error {
			record, err := batchReturnLedgerGoal(root, tree(), goalID)
			if err != nil {
				return err
			}
			loaded, err := findBatchUnit(root, record.Batch, goalID)
			if err != nil {
				return err
			}
			return batchChildRunner(root, landingOwnerLineage, "goal", "handover", "--root", root, "--id", goalID,
				"--lineage", landingOwnerLineage, "--target-machine", source.Machine,
				"--target-lineage", source.Lineage, "--target-claim-epoch", strconv.FormatUint(epoch, 10),
				"--batch", record.Batch, "--target-root", loaded.SeatRoot)
		},
		Release: func(goalID, next string) error {
			if err := batchChildRunner(root, landingOwnerLineage, "goal", "edit", "--root", root, "--id", goalID, "--next", next, "--lineage", landingOwnerLineage); err != nil {
				return err
			}
			return batchChildRunner(root, landingOwnerLineage, "goal", "release", "--root", root, "--id", goalID, "--lineage", landingOwnerLineage)
		},
	}
}

func findBatchUnit(root, batchID, goalID string) (batch.Unit, error) {
	record, err := batch.NewStore(root, nil).Load(batchID)
	if err != nil {
		return batch.Unit{}, err
	}
	for _, unit := range record.Units {
		if unit.GoalID == goalID && unit.State == batch.UnitReturnPending {
			return unit, nil
		}
	}
	return batch.Unit{}, fmt.Errorf("batch unit %s is absent", goalID)
}

func rebindBatchClaims(root, batchID, tree, machine string, epoch int64, read func(string, string, string) (batch.ReturnLedgerGoal, error), run func(string, ...string) error) error {
	record, err := batch.NewStore(root, nil).Load(batchID)
	if err != nil {
		return err
	}
	for _, unit := range record.Units {
		if unit.State != batch.UnitJoined {
			continue
		}
		ledger, err := read(root, tree, unit.GoalID)
		if err != nil {
			return fmt.Errorf("read joined goal %s before rebind: %w", unit.GoalID, err)
		}
		if ledger.Claimed && ledger.Machine == machine && ledger.Lineage == landingOwnerLineage && ledger.Batch == batchID && ledger.ClaimEpoch == uint64(epoch) {
			continue
		}
		if ledger.ClaimEpoch > uint64(epoch) {
			return fmt.Errorf("rebind joined goal %s: claim epoch %d is ahead of owner epoch %d", unit.GoalID, ledger.ClaimEpoch, epoch)
		}
		if err := run(root, "goal", "handover", "--root", root, "--id", unit.GoalID,
			"--lineage", landingOwnerLineage, "--target-machine", machine,
			"--target-lineage", landingOwnerLineage, "--target-claim-epoch", strconv.FormatInt(epoch, 10),
			"--batch", batchID); err != nil {
			return fmt.Errorf("rebind joined goal %s: %w", unit.GoalID, err)
		}
	}
	return nil
}

func newProductionBatchOwner(settings config.BatchLanding, held batchOwnerLease, now func() time.Time) (*batch.Owner, error) {
	store := batch.NewStore(settings.Root, identity.KernelProber{}).WithLedgerOwner(productionTrunkRedLedgerOwner(settings.Root))
	latestTree := ""
	returns := productionReturnSeams(settings.Root, func() string { return latestTree })
	fetch := func() (string, error) {
		tree, err := fetchBatchTree(settings.Root)
		if err == nil {
			latestTree = tree
		}
		return tree, err
	}
	machine, err := goal.ResolveMachine(settings.Root)
	if err != nil {
		return nil, err
	}
	return batch.NewOwner(batch.OwnerOptions{
		Store: store, Settings: settings, Actor: landingOwnerLineage, PID: held.pid, Now: now,
		FetchTree: fetch, ReadClaim: batch.ReadClaimAt, Returns: returns,
		Rebind: func(batchID, tree string) error {
			return rebindBatchClaims(settings.Root, batchID, tree, machine, held.epoch, batch.ReadReturnLedgerGoal, func(root string, args ...string) error {
				return batchChildRunner(root, landingOwnerLineage, args...)
			})
		},
		Mint: func() (string, error) {
			ulid, err := goal.NewOperationULID()
			if err != nil {
				return "", err
			}
			return goal.Opid(ulid, machine, landingOwnerLineage), nil
		},
		LogRed: func(id string, outcome batch.TrunkRedRecordOutcome) {
			line, _ := json.Marshal(map[string]any{"component": "landing-owner", "batch": id, "trunkRed": outcome})
			fmt.Fprintln(os.Stderr, string(line))
		},
		BaseCommit: func(tree string) (string, error) { return commitForTree(settings.Root, "origin/main", tree) },
		RunDiagnostic: func(id string, request batch.DiagnosticRequest, claim batch.Claim) (batch.DiagnosticResult, error) {
			return launchBatchDiagnostic(settings.Root, id, request, claim)
		},
		DescendsFrom: func(descendant, ancestor string) (bool, error) {
			_, err := gitOutput(settings.Root, "merge-base", "--is-ancestor", ancestor, descendant)
			if err == nil {
				return true, nil
			}
			var exit *exec.ExitError
			if errors.As(err, &exit) && exit.ExitCode() == 1 {
				return false, nil
			}
			return false, err
		},
		Sample: func() proofrun.LoadSample { return proofrun.SampleLoad(settings.Root, "", held.pid, now()) },
		Admission: func(sample proofrun.LoadSample) proofrun.AdmissionCap {
			cap, err := proofrun.ResolveAdmissionCap(filepath.Join(settings.Root, "metasystem.conf"), sample.Cores)
			if err != nil {
				return proofrun.AdmissionCap{}
			}
			return cap
		},
		Launch: func(id string, sample proofrun.LoadSample, window string) error {
			record, err := store.Load(id)
			if err != nil {
				return err
			}
			switch record.State {
			case batch.StateDiagnosing:
				return executeBatchDiagnosis(settings.Root, id, landingOwnerLineage, now())
			case batch.StateLanding:
				return executeBatchLanding(settings.Root, id, landingOwnerLineage, now())
			default:
				return executeBatchProof(settings.Root, id, landingOwnerLineage, window, sample, now(), productionBatchProofDependencies)
			}
		},
		After: time.After,
		Report: func(id string, err error) {
			line, _ := json.Marshal(map[string]any{"component": "landing-owner", "batch": id, "error": err.Error()})
			fmt.Fprintln(os.Stderr, string(line))
		},
	})
}

func batchOwnerSignals() (<-chan struct{}, <-chan struct{}, func()) {
	wakeSignal, stopSignal := make(chan os.Signal, 1), make(chan os.Signal, 1)
	wake, stop := make(chan struct{}, 1), make(chan struct{})
	signal.Notify(wakeSignal, syscall.SIGUSR1)
	signal.Notify(stopSignal, syscall.SIGTERM, os.Interrupt)
	go func() {
		for range wakeSignal {
			select {
			case wake <- struct{}{}:
			default:
			}
		}
	}()
	go func() { <-stopSignal; close(stop) }()
	return wake, stop, func() {
		signal.Stop(wakeSignal)
		signal.Stop(stopSignal)
		close(wakeSignal)
	}
}

func parseBatchOwner(args []string, verb string) (config.BatchLanding, time.Duration, error) {
	flags := flag.NewFlagSet("landing batch "+verb, flag.ContinueOnError)
	root := pathFlag(flags, "root", ".", "seat checkout root")
	landingRoot := pathFlag(flags, "landing-root", "", "resolved dedicated landing checkout")
	maxWait := flags.Duration("max-wait", config.DefaultBatchMaxWait, "maximum wait for another unit")
	interval := flags.Duration("interval", time.Minute, "owner tick interval")
	if flags.Parse(args) != nil || flags.NArg() != 0 || *interval <= 0 {
		return config.BatchLanding{}, 0, fmt.Errorf("usage: metasystem landing batch %s --root ROOT [--landing-root ROOT --max-wait DURATION]", verb)
	}
	now, err := goalCommandNow(*root)
	if err != nil {
		return config.BatchLanding{}, 0, err
	}
	settings, err := resolveBatchOwnerSettings(*root, *landingRoot, *maxWait, func() time.Time { return now })
	return settings, *interval, err
}

func runBatchOwner(args []string) int {
	if !batchCapabilitiesAvailable() {
		return runBatchVerbSkeleton(nil)
	}
	settings, interval, err := parseBatchOwner(args, "owner")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	held, err := acquireBatchOwner(settings.Root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer held.retire()
	owner, err := newProductionBatchOwner(settings, held, time.Now)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	wake, stop, cleanup := batchOwnerSignals()
	defer cleanup()
	owner.Loop(interval, wake, stop)
	return 0
}

func runBatchTick(args []string) int {
	if !batchCapabilitiesAvailable() {
		return runBatchVerbSkeleton(nil)
	}
	flags := flag.NewFlagSet("landing batch tick", flag.ContinueOnError)
	root := pathFlag(flags, "root", ".", "seat checkout root")
	landingRoot := pathFlag(flags, "landing-root", "", "resolved dedicated landing checkout")
	maxWait := flags.Duration("max-wait", config.DefaultBatchMaxWait, "maximum wait for another unit")
	id := flags.String("batch", "", "batch id")
	if flags.Parse(args) != nil || flags.NArg() != 0 || *id == "" {
		fmt.Fprintln(os.Stderr, "usage: metasystem landing batch tick --root ROOT --batch ULID")
		return 2
	}
	now, err := goalCommandNow(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	settings, err := resolveBatchOwnerSettings(*root, *landingRoot, *maxWait, func() time.Time { return now })
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	held, err := acquireBatchOwner(settings.Root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer held.retire()
	owner, err := newProductionBatchOwner(settings, held, func() time.Time { return now })
	if err == nil {
		err = owner.Tick(*id)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}
