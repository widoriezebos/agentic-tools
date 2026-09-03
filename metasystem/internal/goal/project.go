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
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/boundedexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// Projection is one read of the accepted world.
type Projection struct {
	Tip     string
	Tree    *TreeGoals
	Banners []string
	Horizon ApprovalHorizon
}

// StaleThreshold is how old an accepted tree may grow before the
// projection banners its staleness.
const StaleThreshold = 30 * time.Minute

// The provider Stop hook allows five seconds. A fresh projection therefore
// gives transport at most four seconds, leaving the caller time to emit the
// fail-closed verdict that the hook can honor.
var (
	freshFetchProcessTimeout = 3 * time.Second
	freshProjectionTimeout   = 4 * time.Second
	fetchForProjection       = boundedFetchAdvance
)

// Project reads the accepted tree. With fetchFirst, the read-side
// validator runs before the read (the --fetch flag); otherwise the
// read is offline-capable and banners staleness.
func Project(e Endpoint, fetchFirst bool, now time.Time) (Projection, error) {
	if fetchFirst {
		if err := fetchProjectionWithinDeadline(e); err != nil {
			return Projection{}, err
		}
	}
	tipOut, err := goalGit(e.Root, nil, "rev-parse", "--verify", "--quiet", AcceptedRef)
	if err != nil {
		return Projection{}, fmt.Errorf("no accepted tree; the first fetch or the migration bootstraps it")
	}
	tip := strings.TrimSpace(tipOut)
	tree, err := loadTree(e.Root, tip)
	if err != nil {
		return Projection{}, err
	}
	p := Projection{Tip: tip, Tree: tree, Horizon: approvalHorizon(tree, now)}

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
	if ageOut, err := goalGit(e.Root, nil, "log", "-1", "--format=%ct", tip); err == nil {
		if seconds := strings.TrimSpace(ageOut); seconds != "" {
			var epoch int64
			if _, scanErr := fmt.Sscanf(seconds, "%d", &epoch); scanErr == nil {
				age := now.Sub(time.Unix(epoch, 0))
				if age > StaleThreshold {
					p.Banners = append(p.Banners, fmt.Sprintf("the accepted tree is %s old; goal list --fetch validates and advances it", age.Round(time.Minute)))
				}
			}
		}
	}
	return p, nil
}

func fetchProjectionWithinDeadline(e Endpoint) error {
	done := make(chan error, 1)
	go func() {
		_, err := fetchForProjection(e)
		done <- err
	}()
	timer := time.NewTimer(freshProjectionTimeout)
	defer timer.Stop()
	select {
	case err := <-done:
		return err
	case <-timer.C:
		return fmt.Errorf("fresh canonical ledger fetch timed out after %s", freshProjectionTimeout)
	}
}

// boundedFetchAdvance is FetchAdvance's read-side acceptance sequence with a
// process-group bound around the one network operation. Keeping the outer
// projection deadline as well ensures both the child and its caller finish
// before the five-second Stop-hook ceiling.
func boundedFetchAdvance(e Endpoint) (AdvanceResult, error) {
	if e.LocalMode() {
		return FetchAdvance(e)
	}
	nonce, err := readNonce()
	if err != nil {
		return AdvanceResult{}, err
	}
	fetched, err := captureRemoteTipWithinDeadline(e, nonce)
	if err != nil {
		return AdvanceResult{}, err
	}
	defer CleanupRefs(e, nonce)

	if err := SyncModeGate(e, fetched); err != nil {
		return AdvanceResult{}, err
	}
	acceptedOut, acceptedErr := goalGit(e.Root, nil, "rev-parse", "--verify", "--quiet", AcceptedRef)
	accepted := strings.TrimSpace(acceptedOut)
	if acceptedErr == nil && accepted == fetched {
		return AdvanceResult{Tip: accepted, Detail: "already at the canonical tip"}, nil
	}
	if acceptedErr == nil {
		if err := AcceptanceGates(e.Root, accepted, fetched); err != nil {
			return AdvanceResult{}, err
		}
	}
	if err := ValidateCommit(e.Root, fetched); err != nil {
		return AdvanceResult{}, err
	}
	detail := "accepted " + short(fetched)
	if acceptedErr == nil {
		if diagnosed, diagErr := PrefixDiagnosis(e.Root, accepted, fetched); diagErr == nil && len(diagnosed) > 0 {
			detail += "; " + strings.Join(diagnosed, "; ")
		}
	}
	if err := AdvanceAccepted(e.Root, fetched); err != nil {
		return AdvanceResult{}, err
	}
	return AdvanceResult{Tip: fetched, Advanced: true, Detail: detail}, nil
}

func captureRemoteTipWithinDeadline(e Endpoint, nonce string) (string, error) {
	ref := fetchRefFor(nonce)
	args := []string{
		"-C", e.Root, "-c", "core.logAllRefUpdates=false",
		"fetch", "--no-tags", "--refmap=", e.Remote, "+" + e.Branch + ":" + ref,
	}
	cmd := exec.Command("git", args...)
	cmd.Env = environWithoutGitSteering()
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := boundedexec.Run(cmd, boundedexec.FixedBound(freshFetchProcessTimeout, "Stop-hook fresh-ledger fetch"), "fresh canonical ledger fetch"); err != nil {
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
// with a valid structured budget in the converted world, and every queued
// legacy goal before migration. InFlight contains only claims and jobs joined
// to a process that is alive at its recorded birth identity.
type ClaimableBudgetedWork struct {
	Claimed   []string
	Claimable []string
	InFlight  []string
	Queued    int
	GoalFree  bool
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
	return readClaimableBudgetedWork(root, now, identity.KernelProber{})
}

func readClaimableBudgetedWork(root string, now time.Time, prober identity.Prober) (ClaimableBudgetedWork, error) {
	resolved, err := ResolveStateRoot(root)
	if err != nil {
		return ClaimableBudgetedWork{}, err
	}
	root = resolved
	converted, err := acceptedGoalWorld(root)
	if err != nil {
		return ClaimableBudgetedWork{}, err
	}
	if !converted {
		return readLegacyClaimableWork(root, prober)
	}
	machine, err := ResolveMachine(root)
	if err != nil {
		return ClaimableBudgetedWork{}, err
	}
	endpoint, err := ResolveEndpoint(root)
	if err != nil {
		return ClaimableBudgetedWork{}, err
	}
	projection, err := Project(endpoint, true, now)
	if err != nil {
		return ClaimableBudgetedWork{}, err
	}
	if projection.Tree == nil {
		return ClaimableBudgetedWork{}, fmt.Errorf("the accepted goal tree is unreadable")
	}
	frontier := Next(projection, machine)
	work := ClaimableBudgetedWork{
		Claimed:  append([]string(nil), frontier.Claimed...),
		GoalFree: projection.Tree.Root != nil && projection.Tree.Root.Free != nil,
	}
	claimLineages := make(map[string]string, len(frontier.Claimed))
	for _, id := range frontier.Claimed {
		if file := projection.Tree.Live[id]; file != nil && file.Claimed != nil {
			claimLineages[id] = file.Claimed.Lineage
		}
	}
	work.Queued = len(frontier.Awaiting)
	for _, id := range frontier.Ready {
		file := projection.Tree.Live[id]
		if file != nil && file.Budget != nil && file.Budget.Validate() == nil {
			work.Claimable = append(work.Claimable, id)
		}
	}
	work.InFlight, err = readLiveBacklogActivity(root, claimLineages, false, prober)
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
	work := ClaimableBudgetedWork{GoalFree: ledger.Free != nil, Queued: len(ledger.Queued)}
	for _, queued := range ledger.Queued {
		work.Claimable = append(work.Claimable, queued.Id)
	}
	legacyClaim := ledger.Current != nil
	if legacyClaim {
		work.Claimed = append(work.Claimed, ledger.Current.Id)
	}
	work.InFlight, err = readLiveBacklogActivity(root, nil, legacyClaim, prober)
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
	Status string `json:"status"`
	activityProcessRef
	Creator          *activityProcessRef  `json:"creatorLiveness"`
	CustodyProcesses []activityProcessRef `json:"custodyProcesses"`
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

func readLiveBacklogActivity(root string, claimLineages map[string]string, legacyClaim bool, prober identity.Prober) ([]string, error) {
	var activity []string
	paths, err := filepath.Glob(filepath.Join(root, "artifacts", "agents", "jobs", "*.json"))
	if err != nil {
		return nil, err
	}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", filepath.Base(path), err)
		}
		var record backlogJobRecord
		if err := json.Unmarshal(data, &record); err != nil {
			return nil, fmt.Errorf("%s is malformed: %w", filepath.Base(path), err)
		}
		switch record.Status {
		case "pending-setup", "pending", "running":
			if liveJobRecord(prober, record) {
				id := record.JobId
				if id == "" {
					id = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
				}
				activity = append(activity, "job:"+id)
			}
		}
	}

	if len(claimLineages) == 0 && !legacyClaim {
		sort.Strings(activity)
		return activity, nil
	}
	announcements, err := filepath.Glob(filepath.Join(root, "artifacts", "agents", "mains", "*.json"))
	if err != nil {
		return nil, err
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
			return nil, fmt.Errorf("%s: %w", filepath.Base(path), err)
		}
		if err := json.Unmarshal(data, &record); err != nil {
			return nil, fmt.Errorf("%s is malformed: %w", filepath.Base(path), err)
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
	return activity, nil
}

// NextVerdict is the frontier read the dispatcher and the steward
// consume: claimed-first, then the queued frontier, then blocked.
type NextVerdict struct {
	Claimed  []string // this machine's claimed goals, sorted
	Ready    []string // approved and unexpired with every blocker done
	Blocked  []string // approved and unexpired behind an open blocker
	Awaiting []string // queued or carrying an expired relayed approval
}

// Next computes the frontier for one machine from a projection.
func Next(p Projection, machine string, requiredLabels ...string) NextVerdict {
	v := NextVerdict{}
	t := p.Tree
	for _, id := range sortedGoalIds(t.Live) {
		f := t.Live[id]
		switch f.State {
		case StateClaimed:
			if f.Claimed != nil && f.Claimed.Machine == machine {
				v.Claimed = append(v.Claimed, id)
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
				v.Ready = append(v.Ready, id)
			} else {
				v.Blocked = append(v.Blocked, id)
			}
		}
	}
	return v
}
