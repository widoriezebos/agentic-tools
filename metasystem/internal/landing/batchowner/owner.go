package batchowner

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/ownercall"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/helm"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/strictjson"
)

const LandingOwnerLineage = "landing-m1l"

type BatchOwnerLease struct {
	Root, Session string
	Pid, Started  int64
	Epoch         int64
	Announced     bool
}

type ProductionBatchOwnerInputs struct {
	LedgerOwner batch.LedgerOwner
	Machine     string
	Sample      func() proofrun.LoadSample
	LockDir     string
	QueueDir    string
	// Log is where the owner reports each red, start wait and error: the
	// standard error of the component or command that runs it, and the
	// supervised component's own log file.
	Log io.Writer
	// TickErrors keeps the owner's last reported error for landing status;
	// nil keeps none.
	TickErrors *TickErrors
}

// BatchOwnerSource supplies raw command inputs for a single invocation.
type BatchOwnerSource struct {
	CommandNow func(string) (time.Time, error)
	LandingGit func(string, []string, []string) ([]byte, error)
	GoalConfig func(string, string) (string, error)
}

func (source *BatchOwnerSource) Validate() error {
	if source != nil && (source.CommandNow == nil || source.LandingGit == nil || source.GoalConfig == nil) {
		return fmt.Errorf("batch owner raw input source is incomplete")
	}
	return nil
}

type BatchOwnerEnsureSeams struct {
	Inspect func(string) (int64, identity.Liveness, error)
	Wake    func(int64) error
	Launch  func(string) error
}

var BatchOwnerEnsure = BatchOwnerEnsureSeams{
	Inspect: inspectBatchOwner,
	Wake:    func(pid int64) error { return syscall.Kill(int(pid), syscall.SIGUSR1) },
	Launch:  launchBatchOwner,
}

var BatchOwnerAcquire = AcquireBatchOwner
var BatchOwnerConstruct = newProductionBatchOwner
var BatchOwnerAnnounce = lease.AnnounceWithPair
var BatchOwnerRetire = lease.Retire
var BatchOwnerSetenv = os.Setenv
var BatchOwnerResume = func(owner *batch.Owner) { owner.Resume() }
var BatchOwnerRequire = func(held BatchOwnerLease) error { return held.Require() }
var BatchOwnerSweepSources = sweepBatchSourcesWorktrees
var BatchOwnerTick = func(owner *batch.Owner, id string) error { return owner.TickOnce(id) }
var CadenceProductionClock = time.Now
var BatchOwnerCadenceTick = func(root string, held BatchOwnerLease, clock func() time.Time) error {
	_, err := Engine.CadenceTick(root, held, clock)
	return err
}
var BatchOwnerCadenceStart = func(tick func()) { go tick() }
var BatchOwnerCadenceReport = func(log io.Writer, err error) {
	line, _ := json.Marshal(map[string]any{"component": "landing-owner", "cadence": "tick", "error": err.Error()})
	fmt.Fprintln(log, string(line))
}

func inspectBatchOwner(root string) (int64, identity.Liveness, error) {
	return InspectBatchOwnerWith(root, identity.KernelProber{}, lease.CurrentHolder, lease.AnnouncementsFor)
}

func InspectBatchOwnerWith(root string, prober identity.Prober, readHolder func(string) (lease.CurrentHolderView, error), announcements func(string, int64) []lease.Announcement) (int64, identity.Liveness, error) {
	holder, err := readHolder(root)
	if err != nil {
		if errors.Is(err, lease.ErrLeaseAbsent) {
			return 0, identity.Dead, nil
		}
		return 0, identity.Unknown, err
	}
	if holder.OwnerLineage != LandingOwnerLineage {
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
	// No announcement names the holder: a holder pid the kernel reports as
	// gone is a dead owner (its lease is stale), so a person's start can take
	// over; any other reading stays unknown.
	if holder.Pid > 0 {
		if _, state, probeErr := prober.Probe(holder.Pid); probeErr == nil && state == identity.Dead {
			return holder.Pid, identity.Dead, nil
		}
	}
	return holder.Pid, identity.Unknown, fmt.Errorf("landing owner announcement is absent")
}

func launchBatchOwner(root string) error {
	binary, err := os.Executable()
	if err != nil {
		return err
	}
	command := BatchOwnerLaunchCommand(binary, root)
	command.Env = os.Environ()
	command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	command.Stdin, command.Stdout, command.Stderr = nil, nil, nil
	return command.Start()
}

func BatchOwnerLaunchCommand(binary, repositoryRoot string) *exec.Cmd {
	controlRoot := batch.ModuleRoot(repositoryRoot)
	command := exec.Command(binary, "up", "--recover-only", "--if-down", "--repo", repositoryRoot, "--metasystem-root", controlRoot)
	command.Dir = controlRoot
	return command
}

func EnsureBatchOwner(root string) error {
	if err := LandingCheckoutPresent(root); err != nil {
		return err
	}
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
	pid, state, err := BatchOwnerEnsure.Inspect(root)
	if err != nil && state == identity.Unknown {
		return fmt.Errorf("BATCH_OWNER_INDETERMINATE: %w", err)
	}
	switch state {
	case identity.Alive:
		return BatchOwnerEnsure.Wake(pid)
	case identity.Dead:
		return BatchOwnerEnsure.Launch(root)
	default:
		return fmt.Errorf("BATCH_OWNER_INDETERMINATE: owner liveness is unknown")
	}
}

func AcquireBatchOwner(root string) (BatchOwnerLease, error) {
	return acquireBatchOwnerWithRetention(root, false)
}

func AcquireBatchOwnerForComponent(root string) (BatchOwnerLease, error) {
	return acquireBatchOwnerWithRetention(root, true)
}

func acquireBatchOwnerWithRetention(root string, retainAnnouncement bool) (BatchOwnerLease, error) {
	pid := int64(os.Getpid())
	exact, state, err := (identity.KernelProber{}).Probe(pid)
	if err != nil || state != identity.Alive || exact.StartedAt.Unix() < 1 {
		return BatchOwnerLease{}, fmt.Errorf("BATCH_OWNER_IDENTITY_UNKNOWN: pid %d state=%s: %v", pid, state, err)
	}
	session := "landing-owner-" + strconv.FormatInt(pid, 10)
	held := BatchOwnerLease{Root: root, Session: session, Pid: pid, Started: exact.StartedAt.Unix()}
	if _, err := BatchOwnerAnnounce(root, session, pid, exact.StartedAt.Unix(), exact.StartTicks, exact.BootID,
		"landing-owner", "metasystem", LandingOwnerLineage); err != nil {
		held.Announced = held.hasAnnouncement()
		if !retainAnnouncement {
			err = errors.Join(err, held.Retire())
		}
		return held, fmt.Errorf("BATCH_OWNER_OWNED_ELSEWHERE: %w", err)
	}
	held.Announced = true
	holder, err := lease.RequireHolder(root, pid, nil)
	if err != nil {
		if !retainAnnouncement {
			err = errors.Join(err, held.Retire())
		}
		return held, fmt.Errorf("BATCH_OWNER_OWNED_ELSEWHERE: holder proof failed: %w", err)
	}
	if !holder.Holder || holder.ClaimEpoch == nil || holder.MainId == nil {
		proofErr := fmt.Errorf("BATCH_OWNER_OWNED_ELSEWHERE: holder proof returned class=%s holder=%t", holder.Class, holder.Holder)
		if !retainAnnouncement {
			proofErr = errors.Join(proofErr, held.Retire())
		}
		return held, proofErr
	}
	held.Epoch = *holder.ClaimEpoch
	if err := BatchOwnerSetenv("METASYSTEM_OWNER_LINEAGE", LandingOwnerLineage); err != nil {
		if !retainAnnouncement {
			err = errors.Join(err, held.Retire())
		}
		return held, err
	}
	return held, nil
}

func (held BatchOwnerLease) hasAnnouncement() bool {
	for _, announcement := range lease.AnnouncementsFor(held.Root, held.Pid) {
		matchesSession := announcement.RuntimeSession == held.Session ||
			(announcement.RuntimeSession == "" && announcement.SessionId == held.Session)
		if announcement.PidStartedAt == held.Started && matchesSession {
			return true
		}
	}
	return false
}

func (held BatchOwnerLease) Require() error {
	holder, err := lease.RequireHolder(held.Root, held.Pid, &held.Epoch)
	if err != nil {
		return fmt.Errorf("BATCH_OWNER_OWNED_ELSEWHERE: holder proof failed: %w", err)
	}
	if !holder.Holder || holder.ClaimEpoch == nil || holder.MainId == nil {
		return fmt.Errorf("BATCH_OWNER_OWNED_ELSEWHERE: holder proof returned class=%s holder=%t", holder.Class, holder.Holder)
	}
	return nil
}

func (held BatchOwnerLease) Retire() error {
	if !held.Announced {
		return nil
	}
	return BatchOwnerRetire(held.Root, held.Session, held.Pid, held.Started)
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
	origin, tree, err := FetchBatchOrigin(root)
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

// liveClaims maps every claimed goal of the ledger at tree to its holder.
func liveClaims(root, tree string) (map[string]string, error) {
	files, err := goalFilesAt(root, tree)
	if err != nil {
		return nil, err
	}
	claims := map[string]string{}
	for _, file := range files {
		if file.Claimed != nil {
			claims[file.Id] = file.Claimed.Machine
		}
	}
	return claims, nil
}

var BatchReturnTargetSeams = struct {
	Goals         func(string, string) ([]*goal.GoalFile, error)
	Holder        func(string) (lease.CurrentHolderView, error)
	Announcements func(string, string) []lease.Announcement
}{goalFilesAt, lease.CurrentHolder, lease.AnnouncementsForOwnerLineage}

var BatchReturnLedgerGoal = batch.ReadReturnLedgerGoal

func ProductionReturnTarget(landingRoot, tree string, prober identity.Prober, unit batch.Unit) batch.ReturnTarget {
	files, err := BatchReturnTargetSeams.Goals(landingRoot, tree)
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
	holder, holderErr := BatchReturnTargetSeams.Holder(unit.SeatRoot)
	announcements := BatchReturnTargetSeams.Announcements(unit.SeatRoot, unit.Claim.Lineage)
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

// BatchOwnerCallSet is the ledger and landing owners the landing path calls
// in its own process, each under an explicit invocation context (design 6.2):
// the process making the call is the supplied identity, and the lineage is
// named, never inherited. The engine sets the production set at start; tests
// replace it.
type BatchOwnerCallSet struct {
	Handover func(ownercall.Invocation, ownercall.HandoverRequest) error
	EditNext func(invocation ownercall.Invocation, root, goalID, next string) error
	Release  func(invocation ownercall.Invocation, root, goalID string) error
	Held     func(root, base, commit, remote, ref string) error
}

var BatchOwnerCalls BatchOwnerCallSet

// LandingOwnerInvocation is the landing owner's own context: it supplies
// itself, as its children's parent did, and its lineage.
func LandingOwnerInvocation() ownercall.Invocation {
	return ownercall.FromThisProcess(LandingOwnerLineage)
}

// batchEditNext rewrites a goal's next step as the landing owner.
func batchEditNext(root, goalID, next string) error {
	return BatchOwnerCalls.EditNext(LandingOwnerInvocation(), root, goalID, next)
}

func ProductionReturnSeams(root string, tree func() string) batch.ReturnSeams {
	controlRoot := batch.ModuleRoot(root)
	return batch.ReturnSeams{
		Read: func(_ string, tree, goalID string) (batch.ReturnLedgerGoal, error) {
			return BatchReturnLedgerGoal(controlRoot, tree, goalID)
		},
		Target: func(unit batch.Unit) batch.ReturnTarget {
			return ProductionReturnTarget(controlRoot, tree(), identity.KernelProber{}, unit)
		},
		HandBack: func(goalID string, source batch.Claim, epoch uint64) error {
			record, err := BatchReturnLedgerGoal(controlRoot, tree(), goalID)
			if err != nil {
				return err
			}
			loaded, err := findBatchUnit(root, record.Batch, goalID)
			if err != nil {
				return err
			}
			return BatchOwnerCalls.Handover(LandingOwnerInvocation(), ownercall.HandoverRequest{Root: controlRoot, GoalID: goalID,
				TargetMachine: source.Machine, TargetLineage: source.Lineage, TargetEpoch: int64(epoch),
				Batch: record.Batch, TargetRoot: loaded.SeatRoot})
		},
		Release: func(goalID, next string) error {
			if err := batchEditNext(controlRoot, goalID, next); err != nil {
				return err
			}
			return BatchOwnerCalls.Release(LandingOwnerInvocation(), controlRoot, goalID)
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

func RebindBatchClaims(root, batchID, tree, machine string, epoch int64, read func(string, string, string) (batch.ReturnLedgerGoal, error), handover func(ownercall.HandoverRequest) error) error {
	controlRoot := batch.ModuleRoot(root)
	record, err := batch.NewStore(root, nil).Load(batchID)
	if err != nil {
		return err
	}
	for _, unit := range record.Units {
		// A change holds no ledger claim: its authority is its own commit,
		// so there is nothing to rebind and no goal file to read.
		if unit.State != batch.UnitJoined || unit.IsChange() {
			continue
		}
		ledger, err := read(controlRoot, tree, unit.GoalID)
		if err != nil {
			return fmt.Errorf("read joined goal %s before rebind: %w", unit.GoalID, err)
		}
		if ledger.Claimed && ledger.Machine == machine && ledger.Lineage == LandingOwnerLineage && ledger.Batch == batchID && ledger.ClaimEpoch == uint64(epoch) {
			continue
		}
		if ledger.ClaimEpoch > uint64(epoch) {
			return fmt.Errorf("rebind joined goal %s: claim epoch %d is ahead of owner epoch %d", unit.GoalID, ledger.ClaimEpoch, epoch)
		}
		if err := handover(ownercall.HandoverRequest{Root: controlRoot, GoalID: unit.GoalID, TargetMachine: machine,
			TargetLineage: LandingOwnerLineage, TargetEpoch: epoch, Batch: batchID}); err != nil {
			return fmt.Errorf("rebind joined goal %s: %w", unit.GoalID, err)
		}
	}
	return nil
}

func ResolveProductionBatchOwnerInputs(root string) (ProductionBatchOwnerInputs, error) {
	return ResolveProductionBatchOwnerInputsWithSource(root, nil)
}

func ResolveProductionBatchOwnerInputsWithSource(root string, source *BatchOwnerSource) (ProductionBatchOwnerInputs, error) {
	if err := source.Validate(); err != nil {
		return ProductionBatchOwnerInputs{}, err
	}
	controlRoot := batch.ModuleRoot(root)
	resolveOwner := ProductionTrunkRedLedgerOwner
	resolveMachine := goal.ResolveMachine
	if source != nil {
		resolveOwner = func(root string) (batch.LedgerOwner, error) {
			return ProductionBatchLedgerOwnerWithConfig(root, source.GoalConfig)
		}
		resolveMachine = func(root string) (string, error) { return goal.ResolveMachineWithConfig(root, source.GoalConfig) }
	}
	ledgerOwner, err := resolveOwner(controlRoot)
	if err != nil {
		return ProductionBatchOwnerInputs{}, err
	}
	machine, err := resolveMachine(controlRoot)
	if err != nil {
		return ProductionBatchOwnerInputs{}, err
	}
	lockDir, queueDir, err := BatchOwnerFixtureProofLockDirectories(root)
	if err != nil {
		return ProductionBatchOwnerInputs{}, err
	}
	return ProductionBatchOwnerInputs{LedgerOwner: ledgerOwner, Machine: machine, LockDir: lockDir, QueueDir: queueDir}, nil
}

func BatchOwnerFixtureProofLockDirectories(root string) (string, string, error) {
	directory, selected, err := proofrun.FixtureHostAdmissionDirectory(root)
	if err != nil || !selected {
		return "", "", err
	}
	return filepath.Join(directory, "batch-proof-lock"), filepath.Join(directory, "batch-proof-queue"), nil
}

func newProductionBatchOwner(settings config.BatchLanding, held BatchOwnerLease, inputs ProductionBatchOwnerInputs, now func() time.Time) (*batch.Owner, error) {
	controlRoot := batch.ModuleRoot(settings.Root)
	// The pipeline keys resolve from the landing checkout's own file; a file
	// that cannot resolve them keeps the compiled defaults.
	if withPipeline, err := settings.WithPipeline(filepath.Join(controlRoot, "metasystem.conf")); err == nil {
		settings = withPipeline
	}
	sample := inputs.Sample
	if sample == nil {
		sample = func() proofrun.LoadSample { return proofrun.SampleLoad(controlRoot, "", held.Pid, now()) }
	}
	store := batch.NewStore(settings.Root, identity.KernelProber{}).WithLedgerOwner(inputs.LedgerOwner)
	latestTree := ""
	returns := ProductionReturnSeams(settings.Root, func() string { return latestTree })
	fetch := func() (string, error) {
		tree, err := fetchBatchTree(settings.Root)
		if err == nil {
			latestTree = tree
		}
		return tree, err
	}
	return batch.NewOwner(batch.OwnerOptions{
		Store: store, Settings: settings, Actor: LandingOwnerLineage, PID: held.Pid, LockDir: inputs.LockDir, QueueDir: inputs.QueueDir, Now: now,
		FetchTree: fetch, ReadClaim: func(_ string, tree, batchID, goalID string) (batch.Claim, error) {
			return batch.ReadClaimAt(controlRoot, tree, batchID, goalID)
		}, Returns: returns,
		ResumeJoinAdmission: func(batchID string, unit batch.Unit) (batch.JoinAdmission, error) {
			return productionJoinAdmission(settings.Root, batchID, unit)
		},
		Rebind: func(batchID, tree string) error {
			return RebindBatchClaims(settings.Root, batchID, tree, inputs.Machine, held.Epoch, batch.ReadReturnLedgerGoal, func(request ownercall.HandoverRequest) error {
				return BatchOwnerCalls.Handover(LandingOwnerInvocation(), request)
			})
		},
		Mint: func() (string, error) {
			ulid, err := goal.NewOperationULID()
			if err != nil {
				return "", err
			}
			return goal.Opid(ulid, inputs.Machine, LandingOwnerLineage), nil
		},
		LogRed: func(id string, outcome batch.TrunkRedRecordOutcome) {
			line, _ := json.Marshal(map[string]any{"component": "landing-owner", "batch": id, "trunkRed": outcome})
			fmt.Fprintln(inputs.Log, string(line))
		},
		BaseCommit:    func(tree string) (string, error) { return commitForTree(settings.Root, "origin/main", tree) },
		RunDiagnostic: clearingDiagnostic(settings.Root),
		DescendsFrom: func(descendant, ancestor string) (bool, error) {
			_, err := GitOutput(settings.Root, "merge-base", "--is-ancestor", ancestor, descendant)
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
		Launch: func(request batch.Dispatch) error {
			id := request.ID
			record, err := store.Load(id)
			if err != nil {
				return err
			}
			switch record.State {
			case batch.StateDiagnosing:
				return ExecuteBatchDiagnosis(settings.Root, id, LandingOwnerLineage, now())
			case batch.StateLanding:
				return ExecuteBatchLanding(settings.Root, id, LandingOwnerLineage, now())
			default:
				return ExecuteBatchProof(settings.Root, id, LandingOwnerLineage, request.Window, request.Token, request.Sample, now(), ProductionBatchProofDependencies)
			}
		},
		ProbeRun: func(id string, record batch.Record) (batch.RunProbe, error) {
			return ProbeBatchProofRun(controlRoot, id, record, identity.KernelProber{}, proofrun.ReadAttempts)
		},
		After: time.After,
		// The start is decided from the host board, read afresh at every
		// decision and checked against the ledger at the latest fetched
		// tree (D14, R22).
		Pipeline: ProductionPipeline(settings.Pipeline.Stall, func() (map[string]string, error) {
			if latestTree == "" {
				return nil, fmt.Errorf("no tree fetched yet")
			}
			return liveClaims(settings.Root, latestTree)
		}),
		LogWait: func(id, line string) {
			encoded, _ := json.Marshal(map[string]any{"component": "landing-owner", "batch": id, "start": line})
			fmt.Fprintln(inputs.Log, string(encoded))
		},
		HelmActive: func(root string) bool { return helm.Active(root).Active },
		BaseMove:   batchBaseMove(settings.Root),
		Early:      productionEarlySeams(settings.Root),
		// One batch proves at a time on the host (U12).
		Proving: LandingLaneProving(LandingLaneHome),
		Report:  ownerReport(inputs.Log, inputs.TickErrors),
	})
}

// batchProofResultPath is the result file a batch's tip proof writes.
func batchProofResultPath(controlRoot, id string) string {
	return filepath.Join(controlRoot, "artifacts", "agents", "proof-runs", "batch", id+".json")
}

func argvNamesResult(argv []string, path string) bool {
	for index, argument := range argv {
		if argument == "--result="+path || argument == "--result" && index+1 < len(argv) && argv[index+1] == path {
			return true
		}
	}
	return false
}

// ProbeBatchProofRun reads a planned proof's launcher from the proof store
// after an owner restart: a non-terminal attempt of the head goal on the
// planned tree, started after the plan, whose live launcher (when its argv is
// readable) writes this batch's result file, is live; the batch's result file for the planned tree, written by
// a terminal attempt that started after the plan, is terminal; else dead.
func ProbeBatchProofRun(controlRoot, id string, record batch.Record, prober identity.Prober, readAttempts func(string) ([]proofrun.Attempt, error)) (batch.RunProbe, error) {
	joined := slices.DeleteFunc(slices.Clone(record.Units), func(unit batch.Unit) bool { return unit.State != batch.UnitJoined })
	if len(joined) == 0 || record.Proof == nil {
		return batch.RunProbe{State: batch.RunDead, Detail: "no joined head goal"}, nil
	}
	head, err := batchChargeID(controlRoot, batch.ChargeUnit(joined), nil)
	if err != nil {
		return batch.RunProbe{}, err
	}
	attempts, err := readAttempts(controlRoot)
	if err != nil {
		return batch.RunProbe{}, err
	}
	planned := ""
	for _, entry := range record.History {
		if entry.Verb == "prove" && entry.Detail == "planned" {
			planned = entry.At
		}
	}
	plannedAt, _ := time.Parse(time.RFC3339Nano, planned)
	resultPath := batchProofResultPath(controlRoot, id)
	for _, attempt := range attempts {
		started, parseErr := time.Parse(time.RFC3339Nano, attempt.StartedAt)
		if attempt.GoalID != head || attempt.Terminal != nil || attempt.CandidateTree != record.Proof.Tree || parseErr != nil || started.Before(plannedAt) ||
			identity.AliveRef(prober, attempt.Launcher.Ref()) != identity.Alive {
			continue
		}
		// The launcher names the result file it writes; one naming another
		// file is some other proof of the head goal on the same tree.
		if exact, _, probeErr := prober.Probe(attempt.Launcher.Pid); probeErr == nil && exact.ArgvKnown && !argvNamesResult(exact.Argv, resultPath) {
			continue
		}
		return batch.RunProbe{State: batch.RunLive}, nil
	}
	var result proofrun.TestResult
	if strictjson.Read(resultPath, &result) == nil && result.CandidateTree == record.Proof.Tree {
		for _, attempt := range attempts {
			started, parseErr := time.Parse(time.RFC3339Nano, attempt.StartedAt)
			if attempt.AttemptID == result.AttemptID && attempt.Terminal != nil && parseErr == nil && !started.Before(plannedAt) {
				return batch.RunProbe{State: batch.RunTerminal, Result: result}, nil
			}
		}
	}
	return batch.RunProbe{State: batch.RunDead, Detail: "no live launcher for goal " + head}, nil
}

// RunBatchOwnerPass is one supervised owner pass; ticks, when set, keeps the
// last error a resume of the batches reported, cleared by a clean resume.
func RunBatchOwnerPass(out io.Writer, owner *batch.Owner, held BatchOwnerLease, root string, clock func() time.Time, cadence *BatchOwnerCadence, ticks *TickErrors) {
	resume := func(owner *batch.Owner) {
		ticks.Begin()
		BatchOwnerResume(owner)
		ticks.End()
	}
	BatchOwnerPassWith(owner, root, BatchOwnerPassSeams{Helm: helm.Active, Resume: resume, Out: out, Now: clock,
		Cadence: func() {
			cadence.start(func() {
				if err := BatchOwnerCadenceTick(root, held, clock); err != nil {
					BatchOwnerCadenceReport(out, err)
				}
			})
		}})
}

type BatchOwnerPassSeams struct {
	Helm    func(string) helm.State
	Resume  func(*batch.Owner)
	Cadence func()
	Out     io.Writer
	Now     func() time.Time
}

// BatchOwnerPassWith is one owner pass. The landing seat's own helm stands the
// whole pass down: neither the batches nor the cadence run, and every queue
// registration the owner holds is withdrawn on every held pass. Another seat's
// helm holds only the batches carrying its units (Owner.Resume); the cadence
// is main's proof and runs on. Each hold prints one helm: line and appends one
// yield record in the landing seat's common dir, on the transition only.
func BatchOwnerPassWith(owner *batch.Owner, root string, seams BatchOwnerPassSeams) {
	if state := seams.Helm(root); state.Active && owner != nil {
		first, withdrawn, err := owner.Withdraw()
		if err != nil {
			fmt.Fprintf(seams.Out, "helm: landing owner: withdraw queue registrations: %v\n", err)
		}
		if first {
			entries := strings.Join(withdrawn, ",")
			fmt.Fprintf(seams.Out, "helm: the landing seat is at the helm (%s); the landing owner stands down, queue entries withdrawn: %s\n", state.By, cmpOrNone(entries))
			helm.RecordYield(root, helm.Yield{At: seams.Now(), Boundary: "landing-owner", Gate: "owner-pass", Would: "resume every batch and tick the cadence",
				By: state.By, Subject: "landing seat " + root + " at the helm; queue entries withdrawn: " + cmpOrNone(entries)})
		}
		return
	}
	seams.Resume(owner)
	if owner != nil {
		for _, held := range owner.Held() {
			if !held.New {
				continue
			}
			by := seams.Helm(held.Seat).By
			fmt.Fprintf(seams.Out, "helm: batch %s held whole for seat %s at the helm (%s); queue entry withdrawn: %s\n", held.ID, held.Seat, by, cmpOrNone(held.Entry))
			helm.RecordYield(root, helm.Yield{At: seams.Now(), Boundary: "landing-owner", Gate: "batch-tick", Would: "tick batch " + held.ID, By: by,
				Subject: "batch " + held.ID + " held for seat " + held.Seat + "; queue entry " + cmpOrNone(held.Entry) + " withdrawn"})
		}
	}
	seams.Cadence()
}

func cmpOrNone(value string) string {
	if value == "" {
		return "none"
	}
	return value
}

// BridgeNudges keeps the landing owner component subscribed to the host
// board's bridge (batch-lane design D14-r2, R25): an event is a nudge that
// carries nothing the owner trusts, on which the component runs its pass as
// on a tick, and the pass's start re-reads the board and classifies it with
// the owner's own prober and clock. Without the bridge (none holds the
// flock, the dial is refused, or it fell silent for two heartbeats) the
// owner reads the board at its tick, the same decision at most one tick
// later, and connects again at its next tick; it never waits for the
// bridge.
type BridgeNudges struct {
	Dial    func() (net.Conn, error)
	Options board.SubscribeOptions
	Report  func(string)
	sub     *board.Subscription
	state   string
}

// NewBridgeNudges is the production subscription of this host's bridge.
func NewBridgeNudges(log io.Writer) *BridgeNudges {
	return &BridgeNudges{
		Dial: func() (net.Conn, error) {
			home, err := board.Home()
			if err != nil {
				return nil, err
			}
			return board.Dial(home)
		},
		Options: board.SubscribeOptions{Kinds: []string{board.KindCard, board.KindStall}, Heartbeat: board.DefaultHeartbeat},
		Report: func(line string) {
			encoded, _ := json.Marshal(map[string]any{"component": "landing-owner", "bridge": line})
			fmt.Fprintln(log, string(encoded))
		},
	}
}

// Events is the live subscription's nudges; nil, which never delivers,
// while the owner reads directly.
func (n *BridgeNudges) Events() <-chan board.Event {
	if n == nil || n.sub == nil {
		return nil
	}
	return n.sub.Events
}

// Ensure connects when no subscription is live, at once or not at all.
func (n *BridgeNudges) Ensure() {
	if n == nil || n.sub != nil {
		return
	}
	conn, err := n.Dial()
	if err == nil {
		n.sub, err = board.Subscribe(conn, n.Options)
	}
	if err != nil {
		n.say("bridge absent (" + err.Error() + "): the board is read at each tick")
		return
	}
	n.say("bridge live: the batch decides on each board event")
}

// Lost records that the subscription ended; the owner reads directly until
// its next tick connects again.
func (n *BridgeNudges) Lost() {
	if n == nil || n.sub == nil {
		return
	}
	n.sub.Close()
	n.sub = nil
	n.say("bridge absent (the connection ended): the board is read at each tick")
}

// Close ends the subscription with the component.
func (n *BridgeNudges) Close() {
	if n != nil && n.sub != nil {
		n.sub.Close()
		n.sub = nil
	}
}

// say reports a change of the bridge's state once, not on every tick.
func (n *BridgeNudges) say(line string) {
	state := strings.SplitN(line, " ", 3)[1]
	if state == n.state {
		return
	}
	n.state = state
	if n.Report != nil {
		n.Report(line)
	}
}
