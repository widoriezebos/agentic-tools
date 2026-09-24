package goal

// The canonical projection: every read — list, next, the
// open-work verdict — comes from the ACCEPTED ref's tree, never the
// checkout. Offline reads work from the last accepted tree with a
// staleness banner past the threshold; the sync mode is a durable
// identity (the root record's word against the clone's config,
// mismatch refused by name); single-machine mode banners itself and
// names backlog-local-promotion as the only exit.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/boundedexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// Projection is one read of the accepted world.
type Projection struct {
	Root                 string
	Tip                  string
	Tree                 *TreeGoals
	Banners              []string
	Horizon              ApprovalHorizon
	claimAdmissionLoader tierBoxLoader
}

// StaleThreshold is how old an accepted tree may grow before the
// projection banners its staleness.
const StaleThreshold = 30 * time.Minute

// The Stop hook budget is sixty seconds, shipped in the registration templates
// under metasystem/scripts/enforcement and owned by the hook's deadline parent.
// A fresh projection keeps its existing tighter bound within that budget.
const (
	defaultFreshFetchProcessTimeout = 3 * time.Second
	defaultFreshProjectionTimeout   = 4 * time.Second
)

type projectionDependencies struct {
	fetch          func(Endpoint) (AdvanceResult, error)
	timeout        time.Duration
	processTimeout time.Duration
	deadline       <-chan time.Time
	source         *projectionSource
}

// projectionSource binds one accepted repository and machine to a read. The
// endpoint is reused for classification and projection so they see one owner.
type projectionSource struct {
	endpoint Endpoint
	machine  string
}

func (source *projectionSource) matchesRoot(root string) error {
	if source.endpoint.Root != root {
		return fmt.Errorf("projection source root %q does not match resolved state root %q", source.endpoint.Root, root)
	}
	if source.endpoint.Repository == nil {
		return fmt.Errorf("projection source for %q has no repository", root)
	}
	if err := ValidateMachineNickname(source.machine); err != nil {
		return fmt.Errorf("projection source for %q has no usable machine: %w", root, err)
	}
	return nil
}

func acceptedGoalWorldFor(endpoint Endpoint) (bool, error) {
	tip, present, err := endpoint.repository().Accepted()
	if err != nil {
		return false, fmt.Errorf("accepted reference %s is unreadable: %w", AcceptedRef, err)
	}
	if !present {
		if _, statErr := os.Stat(filepath.Join(endpoint.Root, filepath.FromSlash(goalsPrefix+"backlog.md"))); statErr == nil {
			return false, fmt.Errorf("the canonical goal root exists but accepted reference %s is missing or unreadable", AcceptedRef)
		} else if !os.IsNotExist(statErr) {
			return false, fmt.Errorf("the canonical goal root cannot be inspected: %w", statErr)
		}
		return false, nil
	}
	tip = strings.TrimSpace(tip)
	if tip == "" {
		return false, fmt.Errorf("accepted reference %s resolved without a commit", AcceptedRef)
	}
	files, err := endpoint.repository().Files(tip, goalsPrefix+"backlog.md")
	if err != nil {
		return false, fmt.Errorf("accepted reference %s does not carry a readable canonical goal root: %w", AcceptedRef, err)
	}
	if len(files[goalsPrefix+"backlog.md"]) == 0 {
		return false, fmt.Errorf("accepted reference %s does not carry a readable canonical goal root", AcceptedRef)
	}
	return true, nil
}

func (dependencies projectionDependencies) withDefaults() projectionDependencies {
	if dependencies.fetch == nil {
		dependencies.fetch = func(endpoint Endpoint) (AdvanceResult, error) {
			return boundedFetchAdvance(endpoint, dependencies.processTimeout)
		}
	}
	if dependencies.timeout <= 0 {
		dependencies.timeout = defaultFreshProjectionTimeout
	}
	if dependencies.processTimeout <= 0 {
		dependencies.processTimeout = defaultFreshFetchProcessTimeout
	}
	return dependencies
}

// Project reads the accepted tree. With fetchFirst, the read-side
// validator runs before the read (the --fetch flag); otherwise the
// read is offline-capable and banners staleness.
func Project(e Endpoint, fetchFirst bool, now time.Time) (Projection, error) {
	return project(e, fetchFirst, now, projectionDependencies{})
}

func project(e Endpoint, fetchFirst bool, now time.Time, dependencies projectionDependencies) (Projection, error) {
	dependencies = dependencies.withDefaults()
	if fetchFirst {
		if err := fetchProjectionWithinDeadline(e, dependencies); err != nil {
			return Projection{}, err
		}
	}
	tip, present, err := e.repository().Accepted()
	if err != nil {
		return Projection{}, err
	}
	if !present {
		return Projection{}, fmt.Errorf("no accepted tree; the first fetch or the migration bootstraps it")
	}
	tree, err := loadTreeFor(e, tip)
	if err != nil {
		return Projection{}, err
	}
	p := Projection{Root: e.Root, Tip: tip, Tree: tree, Horizon: approvalHorizon(tree, now)}

	// The durable sync-mode identity: the root record's word against
	// the clone's config. A local-mode ledger with a remote config is
	// the forbidden promotion; a remote-mode ledger pointed at local
	// is a split-brain risk. Both refuse by name.
	if tree.Root != nil {
		recordMode := tree.Root.SyncMode
		configLocal := e.LocalMode()
		if recordMode == SyncLocal && !configLocal {
			return Projection{}, fmt.Errorf("sync-mode mismatch refused: the ledger is committed local, the config says remote %q — promotion is the backlog-local-promotion goal, not a config flip", e.Remote)
		}
		if recordMode == SyncRemote && configLocal {
			return Projection{}, fmt.Errorf("sync-mode mismatch refused: the ledger is committed remote, the config says local — a split brain is not a mode")
		}
		if recordMode == SyncLocal {
			p.Banners = append(p.Banners, "single-machine mode: multi-machine guarantees are void here; joining a fleet is the backlog-local-promotion goal")
		}
	}

	// Staleness: the accepted COMMIT's age is the tree's age.
	if committed, err := e.repository().CommitTime(tip); err == nil {
		age := now.Sub(committed)
		if age > StaleThreshold {
			p.Banners = append(p.Banners, fmt.Sprintf("the accepted tree is %s old; goal list --fetch validates and advances it", age.Round(time.Minute)))
		}
	}
	return p, nil
}

func fetchProjectionWithinDeadline(e Endpoint, dependencies projectionDependencies) error {
	done := make(chan error, 1)
	fetch := dependencies.fetch
	go func(fetch func(Endpoint) (AdvanceResult, error)) {
		_, err := fetch(e)
		done <- err
	}(fetch)
	deadline := dependencies.deadline
	var timer *time.Timer
	if deadline == nil {
		timer = time.NewTimer(dependencies.timeout)
		deadline = timer.C
		defer timer.Stop()
	}
	select {
	case err := <-done:
		return err
	case <-deadline:
		return fmt.Errorf("fresh canonical ledger fetch timed out after %s", dependencies.timeout)
	}
}

// boundedFetchAdvance is FetchAdvance's read-side acceptance sequence with a
// process-group bound around the one network operation. Keeping the outer
// projection deadline as well keeps both the child and its caller within the
// sixty-second Stop budget shipped under metasystem/scripts/enforcement and
// owned by the hook's deadline parent.
func boundedFetchAdvance(e Endpoint, processTimeout time.Duration) (AdvanceResult, error) {
	if e.Repository != nil {
		return FetchAdvance(e)
	}
	if e.LocalMode() {
		return FetchAdvance(e)
	}
	nonce, err := readNonce()
	if err != nil {
		return AdvanceResult{}, err
	}
	// Cleanup is armed BEFORE the capture, not after it. git creates the
	// per-operation ref during the fetch, so a capture that fails once the
	// deadline has killed the transport has already written a ref that nothing
	// would then remove. CleanupRefs deletes both of this opid's refs and
	// ignores whether they existed, so arming it early costs nothing on the
	// paths that never create one.
	defer CleanupRefs(e, nonce)
	fetched, err := captureRemoteTipWithinDeadline(e, nonce, processTimeout)
	if err != nil {
		return AdvanceResult{}, err
	}

	if err := SyncModeGate(e, fetched); err != nil {
		return AdvanceResult{}, err
	}
	accepted, present, acceptedErr := e.repository().Accepted()
	if acceptedErr != nil {
		return AdvanceResult{}, acceptedErr
	}
	if present && accepted == fetched {
		return AdvanceResult{Tip: accepted, Detail: "already at the canonical tip"}, nil
	}
	if present {
		if err := acceptanceGatesFor(e, accepted, fetched); err != nil {
			return AdvanceResult{}, err
		}
	}
	if err := validateCommitFor(e, fetched); err != nil {
		return AdvanceResult{}, err
	}
	detail := "accepted " + short(fetched)
	if present {
		if diagnosed, diagErr := prefixDiagnosisFor(e, accepted, fetched); diagErr == nil && len(diagnosed) > 0 {
			detail += "; " + strings.Join(diagnosed, "; ")
		}
	}
	if err := advanceAcceptedFor(e, fetched); err != nil {
		return AdvanceResult{}, err
	}
	return AdvanceResult{Tip: fetched, Advanced: true, Detail: detail}, nil
}

func captureRemoteTipWithinDeadline(e Endpoint, nonce string, processTimeout time.Duration) (string, error) {
	ref := fetchRefFor(nonce)
	args := []string{
		"-C", e.Root, "-c", "core.logAllRefUpdates=false",
		"fetch", "--no-tags", "--refmap=", e.Remote, "+" + e.Branch + ":" + ref,
	}
	cmd := commandWithEnvironment(e.commandEnv, "git", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := boundedexec.Run(cmd, boundedexec.FixedBound(processTimeout, "Stop-hook fresh-ledger fetch"), "fresh canonical ledger fetch"); err != nil {
		return "", fmt.Errorf("git fetch: %w (%s)", err, strings.TrimSpace(stderr.String()))
	}
	out, err := goalGit(e.Root, nil, "rev-parse", "--verify", ref)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// ClaimableBudgetedWork is the shared backlog-and-activity predicate consumed
// by both TurnVerdict and the steward. Claimable is goal.Next's ready frontier
// in the converted world, and every queued legacy goal before migration.
// InFlight contains only claims and jobs joined to a process that is alive at
// its recorded birth identity. NonTerminalJobs contains every job id whose
// record has not reached a terminal status, independent of whether its process
// is live.
type ClaimableBudgetedWork struct {
	Claimed         []string
	Landing         []string
	Claimable       []string
	Refused         []AdmissionRefusal
	InFlight        []string
	NonTerminalJobs []string
	Queued          int
	GoalFree        bool
	GoalFacts       map[string]GoalFacts
	fencedClaims    []*GoalFile
	landingClaims   []*GoalFile
	ownedClaims     map[string]*GoalFile
}

// LandingClaims returns this machine's claims waiting to land (goal
// land-ready): live work for the landing, never the seat's working claim.
func (w ClaimableBudgetedWork) LandingClaims() []*GoalFile { return w.landingClaims }

// OwnedClaim returns the exact accepted goal record used to compute this
// claimable-work snapshot. Callers that need claim provenance must not join a
// later projection to these claim ids because the accepted revision can move
// between the two reads.
func (w ClaimableBudgetedWork) OwnedClaim(id string) (*GoalFile, bool) {
	file, ok := w.ownedClaims[id]
	return file, ok
}

func (w ClaimableBudgetedWork) HasInFlight() bool { return len(w.InFlight) > 0 }

// HasDelegateJobInFlight is the Claude turn-exit exemption. A live seat may
// keep a ledger claim joined to its announcement after its turn has ended;
// only a non-terminal delegate job proves that separate work is still being
// carried while claimable backlog waits.
func (w ClaimableBudgetedWork) HasDelegateJobInFlight() bool {
	for _, activity := range w.InFlight {
		if strings.HasPrefix(activity, "job:") {
			return true
		}
	}
	return false
}

// ResolveStateRoot maps a containing template checkout to the metasystem
// installation where its repository-local state lives. Adopted repositories
// and callers already rooted at the installation remain unchanged.
func ResolveStateRoot(root string) (string, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve goal state root: %w", err)
	}
	absRoot = filepath.Clean(absRoot)
	if filepath.Base(absRoot) == "metasystem" {
		return absRoot, nil
	}
	developmentMarker := filepath.Join(absRoot, "development", "metasystem-design.md")
	info, err := os.Stat(developmentMarker)
	if err != nil {
		if os.IsNotExist(err) {
			return absRoot, nil
		}
		return "", fmt.Errorf("resolve goal state root: template marker unreadable: %w", err)
	}
	if info.IsDir() {
		return absRoot, nil
	}
	installation := filepath.Join(absRoot, "metasystem")
	conf, err := os.Stat(filepath.Join(installation, "metasystem.conf"))
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("resolve goal state root: template installation is missing metasystem.conf")
		}
		return "", fmt.Errorf("resolve goal state root: template installation unreadable: %w", err)
	}
	if conf.IsDir() {
		return "", fmt.Errorf("resolve goal state root: template installation has a directory where metasystem.conf must be a file")
	}
	return installation, nil
}

// acceptedGoalWorld distinguishes a legacy checkout from uncertainty about a
// canonical ledger. Once a canonical root record is materialized, losing its
// accepted reference is corruption, never permission to fall back to legacy.
func acceptedGoalWorld(root string) (bool, error) {
	out, err := gitIn(root, "rev-parse", "--verify", "--quiet", AcceptedRef)
	if err != nil {
		if _, statErr := os.Stat(filepath.Join(root, filepath.FromSlash(goalsPrefix+"backlog.md"))); statErr == nil {
			return false, fmt.Errorf("the canonical goal root exists but accepted reference %s is missing or unreadable", AcceptedRef)
		} else if !os.IsNotExist(statErr) {
			return false, fmt.Errorf("the canonical goal root cannot be inspected: %w", statErr)
		}
		return false, nil
	}
	tip := strings.TrimSpace(out)
	if tip == "" {
		return false, fmt.Errorf("accepted reference %s resolved without a commit", AcceptedRef)
	}
	if _, err := gitIn(root, "cat-file", "-e", tip+":./"+goalsPrefix+"backlog.md"); err != nil {
		return false, fmt.Errorf("accepted reference %s does not carry a readable canonical goal root: %w", AcceptedRef, err)
	}
	return true, nil
}

// ReadClaimableBudgetedWork performs the fresh canonical read and the one
// process-liveness join used by every runtime-independent owner.
func ReadClaimableBudgetedWork(root string, now time.Time) (ClaimableBudgetedWork, error) {
	return readClaimableBudgetedWork(root, now, identity.KernelProber{}, projectionDependencies{})
}

func readClaimableBudgetedWork(root string, now time.Time, prober identity.Prober, dependencies projectionDependencies) (ClaimableBudgetedWork, error) {
	resolved, err := ResolveStateRoot(root)
	if err != nil {
		return ClaimableBudgetedWork{}, err
	}
	root = resolved
	var converted bool
	if dependencies.source == nil {
		converted, err = acceptedGoalWorld(root)
	} else if err = dependencies.source.matchesRoot(root); err == nil {
		converted, err = acceptedGoalWorldFor(dependencies.source.endpoint)
	}
	if err != nil {
		return ClaimableBudgetedWork{}, err
	}
	if !converted {
		return readLegacyClaimableWork(root, prober)
	}
	var machine string
	var endpoint Endpoint
	if dependencies.source == nil {
		machine, err = ResolveMachine(root)
		if err != nil {
			return ClaimableBudgetedWork{}, err
		}
		endpoint, err = ResolveEndpoint(root)
		if err != nil {
			return ClaimableBudgetedWork{}, err
		}
	} else {
		machine, endpoint = dependencies.source.machine, dependencies.source.endpoint
	}
	projection, err := project(endpoint, true, now, dependencies)
	if err != nil {
		return ClaimableBudgetedWork{}, err
	}
	return ClaimableWorkFromProjection(projection, machine, prober)
}

// ClaimableWorkFromProjection joins an accepted goal projection to live
// activity for one machine. The projection root also owns local admission
// rules and process records used by this judgment.
func ClaimableWorkFromProjection(projection Projection, machine string, prober identity.Prober) (ClaimableBudgetedWork, error) {
	if projection.Tree == nil {
		return ClaimableBudgetedWork{}, fmt.Errorf("the accepted goal tree is unreadable")
	}
	frontier, err := Next(projection, machine)
	if err != nil {
		return ClaimableBudgetedWork{}, err
	}
	work := ClaimableBudgetedWork{
		Claimed:   append([]string(nil), frontier.Claimed...),
		Landing:   append([]string(nil), frontier.Landing...),
		Refused:   append([]AdmissionRefusal(nil), frontier.Refused...),
		GoalFree:  projection.Tree.Root != nil && projection.Tree.Root.Free != nil,
		GoalFacts: map[string]GoalFacts{}, ownedClaims: map[string]*GoalFile{},
	}
	for id, file := range projection.Tree.Live {
		if file == nil {
			continue
		}
		work.GoalFacts[id] = GoalFacts{Id: id, Intent: file.Intent, NextStep: file.NextStep, Revision: fmt.Sprint(file.Revision)}
	}
	for _, id := range frontier.Fenced {
		if file := projection.Tree.Live[id]; file != nil {
			work.fencedClaims = append(work.fencedClaims, file)
		}
	}
	for _, id := range frontier.Landing {
		if file := projection.Tree.Live[id]; file != nil {
			work.landingClaims = append(work.landingClaims, file)
		}
	}
	// A landing claim is joined to liveness like a working claim: a process
	// landing it is live backlog activity.
	claimLineages := make(map[string]string, len(frontier.Claimed)+len(frontier.Landing))
	for _, id := range append(append([]string(nil), frontier.Claimed...), frontier.Landing...) {
		if file := projection.Tree.Live[id]; file != nil && file.Claimed != nil {
			claimLineages[id] = file.Claimed.Lineage
			work.ownedClaims[id] = file
		}
	}
	work.Queued = len(frontier.Awaiting)
	work.Claimable = append(work.Claimable, frontier.Ready...)
	work.InFlight, work.NonTerminalJobs, err = readLiveBacklogActivity(projection.Root, claimLineages, false, prober)
	if err != nil {
		return ClaimableBudgetedWork{}, err
	}
	return work, nil
}

func readLegacyClaimableWork(root string, prober identity.Prober) (ClaimableBudgetedWork, error) {
	store := &Store{Root: root}
	ledger, problems, err := store.ReadLedger()
	if err != nil {
		return ClaimableBudgetedWork{}, err
	}
	if ledger == nil {
		return ClaimableBudgetedWork{}, nil
	}
	if len(problems) > 0 {
		return ClaimableBudgetedWork{}, fmt.Errorf("legacy goal ledger has %d parse problems", len(problems))
	}
	work := ClaimableBudgetedWork{GoalFree: ledger.Free != nil, Queued: len(ledger.Queued), GoalFacts: map[string]GoalFacts{}}
	for _, queued := range ledger.Queued {
		work.Claimable = append(work.Claimable, queued.Id)
		work.GoalFacts[queued.Id] = GoalFacts{Id: queued.Id, Intent: queued.Intent, NextStep: queued.NextStep}
	}
	legacyClaim := ledger.Current != nil
	if legacyClaim {
		work.Claimed = append(work.Claimed, ledger.Current.Id)
		work.GoalFacts[ledger.Current.Id] = GoalFacts{Id: ledger.Current.Id, Intent: ledger.Current.Intent, NextStep: ledger.Current.NextStep, Revision: ledger.Revision()}
	}
	work.InFlight, work.NonTerminalJobs, err = readLiveBacklogActivity(root, nil, legacyClaim, prober)
	if err != nil {
		return ClaimableBudgetedWork{}, err
	}
	return work, nil
}

type activityProcessRef struct {
	Pid           int64  `json:"pid"`
	PidStartedAt  int64  `json:"pidStartedAt"`
	PidExactMicro int64  `json:"pidStartedAtExactMicro,omitempty"`
	PidStartTicks int64  `json:"pidStartTicks,omitempty"`
	BootID        string `json:"bootId,omitempty"`
}

func (r activityProcessRef) identityRef() identity.Ref {
	return identity.Ref{
		Pid: r.Pid, StartedAtSec: r.PidStartedAt, StartedAtUnixMicro: r.PidExactMicro,
		StartTicks: r.PidStartTicks, BootID: r.BootID,
	}
}

func recordedProcessAlive(prober identity.Prober, ref activityProcessRef) bool {
	return ref.Pid > 0 && ref.PidStartedAt > 0 && identity.AliveRef(prober, ref.identityRef()) == identity.Alive
}

type backlogJobRecord struct {
	JobId  string `json:"jobId"`
	GoalID string `json:"goalId"`
	Status string `json:"status"`
	activityProcessRef
	Creator          *activityProcessRef  `json:"creatorLiveness"`
	CustodyProcesses []activityProcessRef `json:"custodyProcesses"`
}

func nonTerminalGoalJobs(root string, goals map[string]bool) ([]string, error) {
	paths, err := filepath.Glob(filepath.Join(root, "artifacts", "agents", "jobs", "*.json"))
	if err != nil {
		return nil, err
	}
	var jobs []string
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", filepath.Base(path), err)
		}
		var record backlogJobRecord
		if err := json.Unmarshal(data, &record); err != nil {
			return nil, fmt.Errorf("%s is malformed: %w", filepath.Base(path), err)
		}
		if !goals[record.GoalID] {
			continue
		}
		switch record.Status {
		case "completed", "failed", "cancelled", "timeout":
			continue
		}
		id := record.JobId
		if id == "" {
			id = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		}
		jobs = append(jobs, id)
	}
	sort.Strings(jobs)
	return jobs, nil
}

func liveJobRecord(prober identity.Prober, record backlogJobRecord) bool {
	switch record.Status {
	case "pending-setup":
		return record.Creator != nil && recordedProcessAlive(prober, *record.Creator)
	case "pending":
		if recordedProcessAlive(prober, record.activityProcessRef) {
			return true
		}
		if record.Creator != nil && recordedProcessAlive(prober, *record.Creator) {
			return true
		}
	case "running":
		if recordedProcessAlive(prober, record.activityProcessRef) {
			return true
		}
	}
	for _, custody := range record.CustodyProcesses {
		if recordedProcessAlive(prober, custody) {
			return true
		}
	}
	return false
}

func readLiveBacklogActivity(root string, claimLineages map[string]string, legacyClaim bool, prober identity.Prober) ([]string, []string, error) {
	var activity []string
	nonTerminalJobs := map[string]bool{}
	paths, err := filepath.Glob(filepath.Join(root, "artifacts", "agents", "jobs", "*.json"))
	if err != nil {
		return nil, nil, err
	}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", filepath.Base(path), err)
		}
		var record backlogJobRecord
		if err := json.Unmarshal(data, &record); err != nil {
			return nil, nil, fmt.Errorf("%s is malformed: %w", filepath.Base(path), err)
		}
		id := record.JobId
		if id == "" {
			id = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		}
		switch record.Status {
		case "completed", "failed", "cancelled", "timeout":
		default:
			nonTerminalJobs[id] = true
		}
		switch record.Status {
		case "pending-setup", "pending", "running":
			if liveJobRecord(prober, record) {
				activity = append(activity, "job:"+id)
			}
		}
	}
	nonTerminal := make([]string, 0, len(nonTerminalJobs))
	for id := range nonTerminalJobs {
		nonTerminal = append(nonTerminal, id)
	}
	sort.Strings(nonTerminal)

	if len(claimLineages) == 0 && !legacyClaim {
		sort.Strings(activity)
		return activity, nonTerminal, nil
	}
	announcements, err := filepath.Glob(filepath.Join(root, "artifacts", "agents", "mains", "*.json"))
	if err != nil {
		return nil, nil, err
	}
	liveLineages := map[string]bool{}
	for _, path := range announcements {
		var record struct {
			MainId       string `json:"mainId"`
			OwnerLineage string `json:"ownerLineage"`
			activityProcessRef
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", filepath.Base(path), err)
		}
		if err := json.Unmarshal(data, &record); err != nil {
			return nil, nil, fmt.Errorf("%s is malformed: %w", filepath.Base(path), err)
		}
		if record.MainId == "" || !recordedProcessAlive(prober, record.activityProcessRef) {
			continue
		}
		lineage := record.OwnerLineage
		if lineage == "" {
			lineage = record.MainId
		}
		liveLineages[lineage] = true
	}
	if legacyClaim && len(liveLineages) > 0 {
		activity = append(activity, "claim:legacy-current")
	}
	for id, lineage := range claimLineages {
		if liveLineages[lineage] {
			activity = append(activity, "claim:"+id)
		}
	}
	sort.Strings(activity)
	return activity, nonTerminal, nil
}

// AdmissionRefusal records a goal the claim gate would refuse, with the
// gate's own cause. It is a fact about the goal, never about the machine.
type AdmissionRefusal struct {
	GoalID string `json:"goalId"`
	Cause  string `json:"cause"`
}

// NextVerdict is the complete ordered frontier read by backlog health,
// dispatch, and steward decisions.
type NextVerdict struct {
	Claimed  []string           // this machine's claimed goals in backlog order
	Fenced   []string           // this machine's breach-stopped claims in backlog order
	Landing  []string           // this machine's claims waiting to land (goal land-ready) in backlog order
	Ready    []string           // approved and unexpired with every blocker done
	Blocked  []string           // approved and unexpired behind an open blocker
	Awaiting []string           // queued or carrying an expired relayed approval
	Refused  []AdmissionRefusal // approved and otherwise eligible, but the claim gate refuses; the cause is the gate's text

	TrunkRedOwned     []TrunkRedEntry
	TrunkRedElsewhere []TrunkRedEntry
}

type NextSelectionKind string

const (
	NextSelectionContinue NextSelectionKind = "continue"
	NextSelectionReady    NextSelectionKind = "ready"
	NextSelectionNone     NextSelectionKind = "none"
)

// NextSelection is the one orientation outcome for a machine. A held claim
// always wins over waiting work so a read never directs a machine into a
// second claim.
type NextSelection struct {
	Kind   NextSelectionKind
	GoalID string
}

// SelectNext reduces the complete frontier to one machine action while
// leaving NextVerdict.Ready intact for backlog-health consumers.
func SelectNext(frontier NextVerdict) NextSelection {
	if len(frontier.Claimed) > 0 {
		return NextSelection{Kind: NextSelectionContinue, GoalID: frontier.Claimed[0]}
	}
	if len(frontier.Ready) > 0 {
		return NextSelection{Kind: NextSelectionReady, GoalID: frontier.Ready[0]}
	}
	return NextSelection{Kind: NextSelectionNone}
}

// Next computes the frontier for one machine from a projection. A judgement
// about one goal leaves that goal behaving as before; uncertainty about the
// repository's admission law makes the whole frontier indeterminate.
func Next(p Projection, machine string, requiredLabels ...string) (NextVerdict, error) {
	v := NextVerdict{}
	t := p.Tree
	for _, entry := range t.TrunkRed {
		if entry.Closed != nil {
			continue
		}
		if entry.Owner.Machine == machine {
			v.TrunkRedOwned = append(v.TrunkRedOwned, entry)
		} else {
			v.TrunkRedElsewhere = append(v.TrunkRedElsewhere, entry)
		}
	}
	admission := newClaimAdmissionContext(p.Root, p.claimAdmissionLoader)
	for _, id := range OrderedOpenGoalIDs(t.Live) {
		f := t.Live[id]
		switch f.State {
		case StateClaimed:
			if f.Claimed != nil && f.Claimed.Machine == machine {
				// A breach-stopped goal is waiting on a human and must not
				// keep the machine from taking the next item.
				switch {
				case f.IsFencedClaim():
					v.Fenced = append(v.Fenced, id)
				case f.IsLandingClaim():
					// Built work waiting to land keeps its claim for the
					// landing and never blocks the next claim.
					v.Landing = append(v.Landing, id)
				default:
					v.Claimed = append(v.Claimed, id)
				}
			}
		case StateQueued:
			if MatchesLabels(f.Labels, requiredLabels) {
				v.Awaiting = append(v.Awaiting, id)
			}
		case StateApproved:
			if !MatchesLabels(f.Labels, requiredLabels) {
				continue
			}
			if expired, _ := f.ApprovalExpired(p.Horizon); expired {
				v.Awaiting = append(v.Awaiting, id)
				continue
			}
			// Pinning belongs to this member only. Arc siblings remain
			// independently claimable; dependency edges own ordering.
			if f.Pinned != "" && f.Pinned != machine {
				continue
			}
			ready := true
			for _, dep := range f.Blocked {
				if depState(t, dep) != StateDone {
					ready = false
					break
				}
			}
			if ready {
				// A ranked head that claim would refuse must not mask later
				// work, so claim admission owns this final readiness check.
				if _, err := requireApprovedForClaimWithContext(admission, t, f, p.Horizon.Now, "claim"); err == nil {
					v.Ready = append(v.Ready, id)
				} else if isGoalAdmissionRefusal(err) {
					v.Refused = append(v.Refused, AdmissionRefusal{GoalID: id, Cause: err.Error()})
				} else {
					return NextVerdict{}, fmt.Errorf("cannot answer claimable backlog: %w", err)
				}
			} else {
				v.Blocked = append(v.Blocked, id)
			}
		}
	}
	return v, nil
}
