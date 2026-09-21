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

type batchOwnerLease struct {
	root, session string
	pid, started  int64
	epoch         int64
	announced     bool
}

type productionBatchOwnerInputs struct {
	ledgerOwner batch.LedgerOwner
	machine     string
	sample      func() proofrun.LoadSample
	lockDir     string
	queueDir    string
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

var batchOwnerAcquire = acquireBatchOwner
var batchOwnerConstruct = newProductionBatchOwner
var batchOwnerAnnounce = lease.AnnounceWithPair
var batchOwnerRetire = lease.Retire
var batchOwnerSetenv = os.Setenv
var batchOwnerResume = func(owner *batch.Owner) { owner.Resume() }
var batchOwnerRequire = func(held batchOwnerLease) error { return held.require() }
var batchOwnerTick = func(owner *batch.Owner, id string) error { return owner.TickOnce(id) }
var cadenceProductionClock = time.Now
var batchOwnerCadenceTick = func(root string, held batchOwnerLease, clock func() time.Time) error {
	_, err := cadenceTick(root, held, clock)
	return err
}
var batchOwnerCadenceStart = func(tick func()) { go tick() }
var batchOwnerCadenceReport = func(err error) {
	line, _ := json.Marshal(map[string]any{"component": "landing-owner", "cadence": "tick", "error": err.Error()})
	fmt.Fprintln(os.Stderr, string(line))
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
	command := batchOwnerLaunchCommand(binary, root)
	command.Env = os.Environ()
	command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	command.Stdin, command.Stdout, command.Stderr = nil, nil, nil
	return command.Start()
}

func batchOwnerLaunchCommand(binary, repositoryRoot string) *exec.Cmd {
	controlRoot := batch.ModuleRoot(repositoryRoot)
	command := exec.Command(binary, "up", "--recover-only", "--if-down", "--repo", repositoryRoot, "--metasystem-root", controlRoot)
	command.Dir = controlRoot
	return command
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
	return acquireBatchOwnerWithRetention(root, false)
}

func acquireBatchOwnerForComponent(root string) (batchOwnerLease, error) {
	return acquireBatchOwnerWithRetention(root, true)
}

func acquireBatchOwnerWithRetention(root string, retainAnnouncement bool) (batchOwnerLease, error) {
	pid := int64(os.Getpid())
	exact, state, err := (identity.KernelProber{}).Probe(pid)
	if err != nil || state != identity.Alive || exact.StartedAt.Unix() < 1 {
		return batchOwnerLease{}, fmt.Errorf("BATCH_OWNER_IDENTITY_UNKNOWN: pid %d state=%s: %v", pid, state, err)
	}
	session := "landing-owner-" + strconv.FormatInt(pid, 10)
	held := batchOwnerLease{root: root, session: session, pid: pid, started: exact.StartedAt.Unix()}
	if _, err := batchOwnerAnnounce(root, session, pid, exact.StartedAt.Unix(), exact.StartTicks, exact.BootID,
		"landing-owner", "metasystem", landingOwnerLineage); err != nil {
		held.announced = held.hasAnnouncement()
		if !retainAnnouncement {
			err = errors.Join(err, held.retire())
		}
		return held, fmt.Errorf("BATCH_OWNER_OWNED_ELSEWHERE: %w", err)
	}
	held.announced = true
	holder, err := lease.RequireHolder(root, pid, nil)
	if err != nil {
		if !retainAnnouncement {
			err = errors.Join(err, held.retire())
		}
		return held, fmt.Errorf("BATCH_OWNER_OWNED_ELSEWHERE: holder proof failed: %w", err)
	}
	if !holder.Holder || holder.ClaimEpoch == nil || holder.MainId == nil {
		proofErr := fmt.Errorf("BATCH_OWNER_OWNED_ELSEWHERE: holder proof returned class=%s holder=%t", holder.Class, holder.Holder)
		if !retainAnnouncement {
			proofErr = errors.Join(proofErr, held.retire())
		}
		return held, proofErr
	}
	held.epoch = *holder.ClaimEpoch
	if err := batchOwnerSetenv("METASYSTEM_OWNER_LINEAGE", landingOwnerLineage); err != nil {
		if !retainAnnouncement {
			err = errors.Join(err, held.retire())
		}
		return held, err
	}
	return held, nil
}

func (held batchOwnerLease) hasAnnouncement() bool {
	for _, announcement := range lease.AnnouncementsFor(held.root, held.pid) {
		matchesSession := announcement.RuntimeSession == held.session ||
			(announcement.RuntimeSession == "" && announcement.SessionId == held.session)
		if announcement.PidStartedAt == held.started && matchesSession {
			return true
		}
	}
	return false
}

func (held batchOwnerLease) require() error {
	holder, err := lease.RequireHolder(held.root, held.pid, &held.epoch)
	if err != nil {
		return fmt.Errorf("BATCH_OWNER_OWNED_ELSEWHERE: holder proof failed: %w", err)
	}
	if !holder.Holder || holder.ClaimEpoch == nil || holder.MainId == nil {
		return fmt.Errorf("BATCH_OWNER_OWNED_ELSEWHERE: holder proof returned class=%s holder=%t", holder.Class, holder.Holder)
	}
	return nil
}

func (held batchOwnerLease) retire() error {
	if !held.announced {
		return nil
	}
	return batchOwnerRetire(held.root, held.session, held.pid, held.started)
}

func resolveBatchOwnerSettings(seatRoot, landingRoot string, maxWait time.Duration, now func() time.Time) (config.BatchLanding, error) {
	if landingRoot != "" {
		return config.ResolveExplicitBatchLanding(landingRoot, seatRoot, maxWait, now)
	}
	return config.ResolveBatchLanding(filepath.Join(seatRoot, "metasystem.conf"), seatRoot, now)
}

func fetchBatchTree(root string) (string, error) {
	controlRoot := batch.ModuleRoot(root)
	endpoint, err := goal.ResolveEndpoint(controlRoot)
	if err != nil {
		return "", err
	}
	fetched, err := goal.FetchAdvance(endpoint)
	if err != nil {
		return "", err
	}
	origin, tree, err := fetchBatchOrigin(root)
	if err != nil {
		return "", err
	}
	if origin != fetched.Tip {
		return "", fmt.Errorf("BATCH_BASE_MOVED: validated goal tip %s differs from origin/main %s", fetched.Tip, origin)
	}
	return tree, nil
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
		prefix, prefixErr := (gittree.Workspace{Dir: root}).Prefix()
		if prefixErr != nil {
			return nil, prefixErr
		}
		data, present, readErr := (gittree.Workspace{Dir: root}).FileAt(tree, prefix+name)
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
	controlRoot := batch.ModuleRoot(root)
	return batch.ReturnSeams{
		Read: func(_ string, tree, goalID string) (batch.ReturnLedgerGoal, error) {
			return batchReturnLedgerGoal(controlRoot, tree, goalID)
		},
		Target: func(unit batch.Unit) batch.ReturnTarget {
			return productionReturnTarget(controlRoot, tree(), identity.KernelProber{}, unit)
		},
		HandBack: func(goalID string, source batch.Claim, epoch uint64) error {
			record, err := batchReturnLedgerGoal(controlRoot, tree(), goalID)
			if err != nil {
				return err
			}
			loaded, err := findBatchUnit(root, record.Batch, goalID)
			if err != nil {
				return err
			}
			return batchChildRunner(controlRoot, landingOwnerLineage, "goal", "handover", "--root", controlRoot, "--id", goalID,
				"--lineage", landingOwnerLineage, "--target-machine", source.Machine,
				"--target-lineage", source.Lineage, "--target-claim-epoch", strconv.FormatUint(epoch, 10),
				"--batch", record.Batch, "--target-root", loaded.SeatRoot)
		},
		Release: func(goalID, next string) error {
			if err := batchChildRunner(controlRoot, landingOwnerLineage, "goal", "edit", "--root", controlRoot, "--id", goalID, "--next", next, "--lineage", landingOwnerLineage); err != nil {
				return err
			}
			return batchChildRunner(controlRoot, landingOwnerLineage, "goal", "release", "--root", controlRoot, "--id", goalID, "--lineage", landingOwnerLineage)
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
	controlRoot := batch.ModuleRoot(root)
	record, err := batch.NewStore(root, nil).Load(batchID)
	if err != nil {
		return err
	}
	for _, unit := range record.Units {
		if unit.State != batch.UnitJoined {
			continue
		}
		ledger, err := read(controlRoot, tree, unit.GoalID)
		if err != nil {
			return fmt.Errorf("read joined goal %s before rebind: %w", unit.GoalID, err)
		}
		if ledger.Claimed && ledger.Machine == machine && ledger.Lineage == landingOwnerLineage && ledger.Batch == batchID && ledger.ClaimEpoch == uint64(epoch) {
			continue
		}
		if ledger.ClaimEpoch > uint64(epoch) {
			return fmt.Errorf("rebind joined goal %s: claim epoch %d is ahead of owner epoch %d", unit.GoalID, ledger.ClaimEpoch, epoch)
		}
		if err := run(controlRoot, "goal", "handover", "--root", controlRoot, "--id", unit.GoalID,
			"--lineage", landingOwnerLineage, "--target-machine", machine,
			"--target-lineage", landingOwnerLineage, "--target-claim-epoch", strconv.FormatInt(epoch, 10),
			"--batch", batchID); err != nil {
			return fmt.Errorf("rebind joined goal %s: %w", unit.GoalID, err)
		}
	}
	return nil
}

func resolveProductionBatchOwnerInputs(root string) (productionBatchOwnerInputs, error) {
	controlRoot := batch.ModuleRoot(root)
	ledgerOwner, err := productionTrunkRedLedgerOwner(controlRoot)
	if err != nil {
		return productionBatchOwnerInputs{}, err
	}
	machine, err := goal.ResolveMachine(controlRoot)
	if err != nil {
		return productionBatchOwnerInputs{}, err
	}
	lockDir, queueDir, err := batchOwnerFixtureProofLockDirectories(root)
	if err != nil {
		return productionBatchOwnerInputs{}, err
	}
	return productionBatchOwnerInputs{ledgerOwner: ledgerOwner, machine: machine, lockDir: lockDir, queueDir: queueDir}, nil
}

func batchOwnerFixtureProofLockDirectories(root string) (string, string, error) {
	directory, selected, err := proofrun.FixtureHostAdmissionDirectory(root)
	if err != nil || !selected {
		return "", "", err
	}
	return filepath.Join(directory, "batch-proof-lock"), filepath.Join(directory, "batch-proof-queue"), nil
}

func newProductionBatchOwner(settings config.BatchLanding, held batchOwnerLease, inputs productionBatchOwnerInputs, now func() time.Time) (*batch.Owner, error) {
	controlRoot := batch.ModuleRoot(settings.Root)
	sample := inputs.sample
	if sample == nil {
		sample = func() proofrun.LoadSample { return proofrun.SampleLoad(controlRoot, "", held.pid, now()) }
	}
	store := batch.NewStore(settings.Root, identity.KernelProber{}).WithLedgerOwner(inputs.ledgerOwner)
	latestTree := ""
	returns := productionReturnSeams(settings.Root, func() string { return latestTree })
	fetch := func() (string, error) {
		tree, err := fetchBatchTree(settings.Root)
		if err == nil {
			latestTree = tree
		}
		return tree, err
	}
	return batch.NewOwner(batch.OwnerOptions{
		Store: store, Settings: settings, Actor: landingOwnerLineage, PID: held.pid, LockDir: inputs.lockDir, QueueDir: inputs.queueDir, Now: now,
		FetchTree: fetch, ReadClaim: func(_ string, tree, batchID, goalID string) (batch.Claim, error) {
			return batch.ReadClaimAt(controlRoot, tree, batchID, goalID)
		}, Returns: returns,
		ResumeJoinAdmission: func(batchID string, unit batch.Unit) (batch.JoinAdmission, error) {
			return productionJoinAdmission(settings.Root, batchID, unit)
		},
		Rebind: func(batchID, tree string) error {
			return rebindBatchClaims(settings.Root, batchID, tree, inputs.machine, held.epoch, batch.ReadReturnLedgerGoal, func(root string, args ...string) error {
				return batchChildRunner(root, landingOwnerLineage, args...)
			})
		},
		Mint: func() (string, error) {
			ulid, err := goal.NewOperationULID()
			if err != nil {
				return "", err
			}
			return goal.Opid(ulid, inputs.machine, landingOwnerLineage), nil
		},
		LogRed: func(id string, outcome batch.TrunkRedRecordOutcome) {
			line, _ := json.Marshal(map[string]any{"component": "landing-owner", "batch": id, "trunkRed": outcome})
			fmt.Fprintln(os.Stderr, string(line))
		},
		BaseCommit:    func(tree string) (string, error) { return commitForTree(settings.Root, "origin/main", tree) },
		RunDiagnostic: clearingDiagnostic(settings.Root),
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
		Sample: sample,
		Admission: func(sample proofrun.LoadSample) proofrun.AdmissionCap {
			cap, err := proofrun.ResolveAdmissionCap(filepath.Join(controlRoot, "metasystem.conf"), sample.Cores)
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

func parseBatchOwner(args []string, verb string, clock func() time.Time) (config.BatchLanding, time.Duration, error) {
	flags := flag.NewFlagSet("landing batch "+verb, flag.ContinueOnError)
	root := pathFlag(flags, "root", ".", "seat checkout root")
	landingRoot := pathFlag(flags, "landing-root", "", "resolved dedicated landing checkout")
	maxWait := flags.Duration("max-wait", config.DefaultBatchMaxWait, "maximum wait for another unit")
	interval := flags.Duration("interval", time.Minute, "owner tick interval")
	if flags.Parse(args) != nil || flags.NArg() != 0 || *interval <= 0 {
		return config.BatchLanding{}, 0, fmt.Errorf("usage: metasystem landing batch %s --root ROOT [--landing-root ROOT --max-wait DURATION]", verb)
	}
	if _, err := goalCommandNow(*root); err != nil {
		return config.BatchLanding{}, 0, err
	}
	settings, err := resolveBatchOwnerSettings(*root, *landingRoot, *maxWait, clock)
	return settings, *interval, err
}

func runBatchOwner(args []string) (code int) {
	clock := cadenceProductionClock
	settings, interval, err := parseBatchOwner(args, "owner", clock)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	inputs, err := resolveProductionBatchOwnerInputs(settings.Root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	held, err := batchOwnerAcquire(settings.Root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer func() {
		if retireErr := held.retire(); retireErr != nil {
			fmt.Fprintln(os.Stderr, retireErr)
			code = 1
		}
	}()
	owner, err := batchOwnerConstruct(settings, held, inputs, clock)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	wake, stop, cleanup := batchOwnerSignals()
	defer cleanup()
	if err := loopBatchOwner(owner, held, settings.Root, clock, interval, wake, stop); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func loopBatchOwner(owner *batch.Owner, held batchOwnerLease, root string, clock func() time.Time, interval time.Duration, wake <-chan struct{}, stop <-chan struct{}) error {
	return loopBatchOwnerWithCadence(owner, held, root, clock, interval, wake, stop, newBatchOwnerCadence())
}

func loopBatchOwnerWithCadence(owner *batch.Owner, held batchOwnerLease, root string, clock func() time.Time, interval time.Duration, wake <-chan struct{}, stop <-chan struct{}, cadence *batchOwnerCadence) error {
	defer cadence.stop()
	for {
		if err := batchOwnerRequire(held); err != nil {
			return err
		}
		runBatchOwnerPass(owner, held, root, clock, cadence)
		timer := time.NewTimer(interval)
		select {
		case <-wake:
			if !timer.Stop() {
				<-timer.C
			}
		case <-timer.C:
		case <-stop:
			if !timer.Stop() {
				<-timer.C
			}
			return nil
		}
	}
}

func runBatchOwnerPass(owner *batch.Owner, held batchOwnerLease, root string, clock func() time.Time, cadence *batchOwnerCadence) {
	batchOwnerResume(owner)
	cadence.start(func() {
		if err := batchOwnerCadenceTick(root, held, clock); err != nil {
			batchOwnerCadenceReport(err)
		}
	})
}

func runBatchTick(args []string) (code int) {
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
	inputs, err := resolveProductionBatchOwnerInputs(settings.Root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	held, err := batchOwnerAcquire(settings.Root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer func() {
		if retireErr := held.retire(); retireErr != nil {
			fmt.Fprintln(os.Stderr, retireErr)
			code = 1
		}
	}()
	owner, err := batchOwnerConstruct(settings, held, inputs, func() time.Time { return now })
	if err == nil {
		err = batchOwnerRequire(held)
	}
	if err == nil {
		err = batchOwnerTick(owner, *id)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}
