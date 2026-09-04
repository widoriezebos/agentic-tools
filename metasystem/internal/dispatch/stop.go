package dispatch

// The breach-stop custodian owns the one ranked transition from goal
// admission to job cancellation. It closes the goal fence while holding only
// the goal-revision lock, releases that lock, and then advances a durable
// batch by authorizing one lifecycle cancellation at a time.

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goalrevision"
)

const stopCustodianLineage = "goal-stop-custodian"

const (
	stopLocalPending       = "LOCAL_PENDING"
	stopLocalTerminal      = "LOCAL_TERMINAL"
	stopForeignReportOnly  = "FOREIGN_REPORT_ONLY"
	stopAlreadyTerminal    = "ALREADY_TERMINAL"
	stopCancelled          = "CANCELLED"
	stopTerminalDuringStop = "TERMINAL_DURING_STOP"
)

type GoalBinding struct {
	GoalID     string
	Revision   uint64
	Tier       uint8
	GateWidth  string
	Machine    string
	Lineage    string
	Capability goal.StopCapability
	Fence      *goal.StopFence
	File       *goal.GoalFile
}

func effectiveGoalTier(tree *goal.TreeGoals, file *goal.GoalFile) (uint8, error) {
	if file.Tier >= 1 && file.Tier <= 3 {
		return file.Tier, nil
	}
	if tree.Root != nil && tree.Root.TierLaw == "" {
		return 3, nil
	}
	return 0, fmt.Errorf("goal %s has no tier; classify the goal first: goal edit --tier", file.Id)
}

func ResolveGoalBinding(root, id string, now time.Time) (GoalBinding, error) {
	if id == "" {
		return GoalBinding{}, fmt.Errorf("a goal id is required")
	}
	endpoint, err := goal.ResolveEndpoint(root)
	if err != nil {
		return GoalBinding{}, err
	}
	projection, err := goal.Project(endpoint, false, now)
	if err != nil {
		return GoalBinding{}, err
	}
	file := projection.Tree.Live[id]
	if file == nil || file.State != goal.StateClaimed || file.Claimed == nil {
		return GoalBinding{}, fmt.Errorf("goal %s is not a claimed accepted goal", id)
	}
	if file.StopCapability == nil {
		return GoalBinding{}, fmt.Errorf("goal %s revision %d predates breach-stop authority; resume or re-claim it before dispatch", id, file.Claimed.Revision)
	}
	tier, err := effectiveGoalTier(projection.Tree, file)
	if err != nil {
		return GoalBinding{}, err
	}
	gateWidth := "area"
	if file.Risk != nil {
		gateWidth = file.Risk.GateWidth()
	}
	return GoalBinding{
		GoalID: id, Revision: file.Claimed.Revision, Tier: tier, GateWidth: gateWidth, Machine: file.Claimed.Machine,
		Lineage: file.Claimed.Lineage, Capability: *file.StopCapability, Fence: file.StopFence, File: file,
	}, nil
}

// GoalRevisionLockDir is the single lock owner shared by dispatch, stop, and
// resume. A revision is part of the path so a resumed revision never waits on
// a stale predecessor's lock.
func GoalRevisionLockDir(root, id string, revision uint64) (string, error) {
	return goalrevision.Path(root, id, revision)
}

// GoalLockBusyError keeps the ranked-lock refusal machine-readable while
// retaining the holder evidence an operator needs to retry safely.
func GoalLockBusyError(directory, key string) error {
	return goalrevision.BusyError(directory, key)
}

func liveStopReason(projection BudgetProjection) string {
	if projection.ElapsedState == ElapsedBreach {
		return goal.StopReasonElapsedLimit
	}
	if len(projection.Breaches) > 0 {
		return goal.StopReasonCorruptOverLimit
	}
	return ""
}

func stopIdentity(id string, revision, epoch uint64) (string, string) {
	stopID := fmt.Sprintf("stop-%s-r%d-f%d", id, revision, epoch)
	sum := sha256.Sum256([]byte(stopID))
	const alphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
	ulid := make([]byte, 26)
	for i := range ulid {
		ulid[i] = alphabet[int(sum[i])%len(alphabet)]
	}
	return stopID, string(ulid)
}

// EnsureBreachStop closes or rediscovers the exact fence and creates its
// resumable batch. The caller must cancel only after this function returns.
func EnsureBreachStop(root, id string, revision uint64, now time.Time) (goal.StopBatch, error) {
	binding, err := ResolveGoalBinding(root, id, now)
	if err != nil {
		return goal.StopBatch{}, err
	}
	if revision != 0 && binding.Revision != revision {
		return goal.StopBatch{}, fmt.Errorf("goal %s moved from revision %d to %d", id, revision, binding.Revision)
	}
	held, err := goalrevision.Acquire(root, id, binding.Revision, "breach-stop")
	if err != nil {
		return goal.StopBatch{}, err
	}
	defer held.Release()

	// Re-read after the lock. This is the launch-fence decision point.
	binding, err = ResolveGoalBinding(root, id, now)
	if err != nil {
		return goal.StopBatch{}, err
	}
	if binding.Revision != revision && revision != 0 {
		return goal.StopBatch{}, fmt.Errorf("goal %s moved from revision %d to %d while stop waited", id, revision, binding.Revision)
	}
	var firingEvidence *goal.StopFiringEvidence
	if binding.Fence == nil {
		budget := ProjectBudget(root, binding.File, now)
		if budget.Status != BudgetKnown {
			return goal.StopBatch{}, fmt.Errorf("cannot prove a live-stop boundary: %s", budget.Unknown.Reason)
		}
		reason := liveStopReason(budget)
		if reason == "" {
			return goal.StopBatch{}, fmt.Errorf("goal %s revision %d has no live-stop breach", id, binding.Revision)
		}
		if reason == goal.StopReasonElapsedLimit {
			firingEvidence = &goal.StopFiringEvidence{
				ElapsedUsed: budget.Elapsed.String(), AdmissionLimit: budget.Limits.ElapsedLimit,
				BreachBoundary: budget.ElapsedBreachLimit.String(), GracePercent: budget.ElapsedGracePercent,
			}
		}
		stopID, ulid := stopIdentity(id, binding.Revision, binding.Capability.FenceEpoch+1)
		endpoint, endpointErr := goal.ResolveEndpoint(root)
		if endpointErr != nil {
			return goal.StopBatch{}, endpointErr
		}
		request := goal.CloseStopRequest{
			VerbRequest: goal.VerbRequest{
				Endpoint: endpoint,
				Actor:    goal.Actor{Machine: binding.Machine, Lineage: stopCustodianLineage},
				Ulid:     ulid, Now: now, ClaimEpoch: binding.Capability.ClaimEpoch,
			},
			GoalID: id, StopID: stopID, Reason: reason, Capability: binding.Capability,
		}
		result, closeErr := goal.CloseStop(request)
		if closeErr != nil {
			return goal.StopBatch{}, closeErr
		}
		if result.Outcome != goal.OutcomeConfirmed && result.Outcome != goal.OutcomeAbandoned {
			return goal.StopBatch{}, fmt.Errorf("closing goal %s fence ended %s: %s", id, result.Outcome, result.Detail)
		}
		binding, err = ResolveGoalBinding(root, id, now)
		if err != nil {
			return goal.StopBatch{}, err
		}
	}
	if binding.Fence == nil {
		return goal.StopBatch{}, fmt.Errorf("goal %s revision %d fence closure was not observable", id, binding.Revision)
	}
	fence := binding.Fence
	if existing, readErr := goal.ReadStopBatch(root, fence.StopID); readErr == nil {
		if existing.GoalID != id || existing.GoalRevision != binding.Revision ||
			existing.FenceEpoch != fence.Epoch || existing.CapabilityGeneration != binding.Capability.Generation ||
			existing.Machine != binding.Capability.Machine || existing.ClaimEpoch != binding.Capability.ClaimEpoch ||
			existing.Reason != fence.Reason {
			return goal.StopBatch{}, fmt.Errorf("stop batch %s contradicts the accepted fence", fence.StopID)
		}
		return existing, nil
	} else if !os.IsNotExist(readErr) {
		return goal.StopBatch{}, readErr
	}
	stamp := now.UTC().Format(time.RFC3339)
	batch := goal.StopBatch{
		StopID: fence.StopID, GoalID: id, GoalRevision: binding.Revision,
		FenceEpoch: fence.Epoch, CapabilityGeneration: binding.Capability.Generation,
		Machine: binding.Machine, ClaimEpoch: binding.Capability.ClaimEpoch,
		Reason: fence.Reason, State: goal.StopBatchOpen, OpenedAt: stamp, UpdatedAt: stamp,
		FiringEvidence: firingEvidence,
	}
	if err := goal.WriteStopBatch(root, batch); err != nil {
		return goal.StopBatch{}, err
	}
	return batch, nil
}

// GoalRecoveryPolicy rebuilds a stranded fence mutation only from a fresh
// accepted goal and budget projection while holding that revision's lock.
// Stored stop reasons and capability strings are never authority inputs.
type GoalRecoveryPolicy struct {
	Now time.Time
}

func (policy GoalRecoveryPolicy) BreachStop(endpoint goal.Endpoint, entry goal.Entry) (goal.PublishRequest, func(), error) {
	if len(entry.Intent.Targets) != 1 {
		return goal.PublishRequest{}, nil, fmt.Errorf("breach-stop recovery needs exactly one recorded goal target")
	}
	id := entry.Intent.Targets[0]
	now := policy.Now.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	binding, err := ResolveGoalBinding(endpoint.Root, id, now)
	if err != nil {
		return goal.PublishRequest{}, nil, err
	}
	held, err := goalrevision.Acquire(endpoint.Root, id, binding.Revision, "breach-stop-recovery")
	if err != nil {
		return goal.PublishRequest{}, nil, err
	}
	release := func() { _ = held.Release() }
	binding, err = ResolveGoalBinding(endpoint.Root, id, now)
	if err != nil {
		release()
		return goal.PublishRequest{}, nil, err
	}
	if binding.Fence != nil {
		release()
		return goal.PublishRequest{}, nil, fmt.Errorf("goal %s revision %d already has a fence; recovery will not replace it", id, binding.Revision)
	}
	projection := ProjectBudget(endpoint.Root, binding.File, now)
	if projection.Status != BudgetKnown {
		release()
		return goal.PublishRequest{}, nil, fmt.Errorf("cannot re-establish breach-stop authority: %s", projection.Unknown.Reason)
	}
	reason := liveStopReason(projection)
	if reason == "" {
		release()
		return goal.PublishRequest{}, nil, fmt.Errorf("goal %s revision %d is not over its live budget", id, binding.Revision)
	}
	stopID, ulid := stopIdentity(id, binding.Revision, binding.Capability.FenceEpoch+1)
	actor := goal.Actor{Machine: binding.Machine, Lineage: stopCustodianLineage}
	if entry.Machine != actor.Machine || entry.Lineage != actor.Lineage ||
		entry.Opid != goal.Opid(ulid, actor.Machine, actor.Lineage) {
		release()
		return goal.PublishRequest{}, nil, fmt.Errorf("the breach-stop journal identity does not match the live custodian operation")
	}
	request := goal.CloseStopRequest{
		VerbRequest: goal.VerbRequest{Endpoint: endpoint, Actor: actor, Ulid: ulid, Now: now,
			ClaimEpoch: binding.Capability.ClaimEpoch},
		GoalID: id, StopID: stopID, Reason: reason, Capability: binding.Capability,
	}
	return goal.CloseStopPublishRequest(request), release, nil
}

type StopRouteCondition string

const (
	StopRouteBreach        StopRouteCondition = "BREACH"
	StopRouteIndeterminate StopRouteCondition = "INDETERMINATE"
)

type StopRoute struct {
	GoalID    string
	Revision  uint64
	StopID    string
	Reason    string
	Condition StopRouteCondition
	Failure   string
}

// FindBreachStops supplies the steward's heal-before-notify routes.
func FindBreachStops(root string, now time.Time) ([]StopRoute, error) {
	if !goal.NewWorld(root) {
		return nil, nil
	}
	endpoint, err := goal.ResolveEndpoint(root)
	if err != nil {
		return nil, err
	}
	projection, err := goal.Project(endpoint, false, now)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(projection.Tree.Live))
	for id := range projection.Tree.Live {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	var routes []StopRoute
	for _, id := range ids {
		file := projection.Tree.Live[id]
		if file.State != goal.StateClaimed || file.Claimed == nil {
			continue
		}
		if file.StopFence != nil {
			batch, readErr := goal.ReadStopBatch(root, file.StopFence.StopID)
			if readErr == nil && batch.State == goal.StopBatchComplete {
				continue
			}
			route := StopRoute{
				GoalID: id, Revision: file.Claimed.Revision, StopID: file.StopFence.StopID,
				Reason: file.StopFence.Reason, Condition: StopRouteBreach,
			}
			if readErr != nil && !os.IsNotExist(readErr) {
				route.Condition = StopRouteIndeterminate
				route.Failure = readErr.Error()
			} else if readErr == nil && batch.State == goal.StopBatchIndeterminate {
				route.Condition = StopRouteIndeterminate
				route.Failure = batch.Failure
				if route.Failure == "" {
					route.Failure = "the stop batch is INDETERMINATE"
				}
			}
			routes = append(routes, route)
			continue
		}
		budget := ProjectBudget(root, file, now)
		if budget.Status != BudgetKnown {
			routes = append(routes, StopRoute{
				GoalID: id, Revision: file.Claimed.Revision, Condition: StopRouteIndeterminate,
				Failure: fmt.Sprintf("BUDGET_UNKNOWN record=%s reason=%s", budget.Unknown.Record, budget.Unknown.Reason),
			})
			continue
		}
		if liveStopReason(budget) == "" {
			continue
		}
		route := StopRoute{
			GoalID: id, Revision: file.Claimed.Revision, Reason: liveStopReason(budget), Condition: StopRouteBreach,
		}
		if file.StopCapability == nil {
			route.Condition = StopRouteIndeterminate
			route.Failure = "the claimed revision has no stop capability"
		}
		routes = append(routes, route)
	}
	return routes, nil
}

// ReconcileStopBatch scans after the goal lock has been released. It records
// matching local jobs as pending and reports foreign custody without trying
// to cancel it.
func ReconcileStopBatch(root, stopID string, now time.Time) (goal.StopBatch, error) {
	batch, err := goal.ReadStopBatch(root, stopID)
	if err != nil {
		return goal.StopBatch{}, err
	}
	if batch.State == goal.StopBatchComplete || batch.State == goal.StopBatchIndeterminate {
		return batch, nil
	}
	jobsDir := filepath.Join(root, "artifacts", "agents", "jobs")
	entries, err := os.ReadDir(jobsDir)
	if err != nil && !os.IsNotExist(err) {
		batch.State, batch.Failure = goal.StopBatchIndeterminate, "job-record directory is unreadable: "+err.Error()
		batch.UpdatedAt = now.UTC().Format(time.RFC3339)
		return batch, goal.WriteStopBatch(root, batch)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	var pending, terminal, foreign []string
	observed := append([]goal.StopJob(nil), batch.Observed...)
	observedIndex := make(map[string]int, len(observed))
	for i, item := range observed {
		observedIndex[item.JobID+"\x00"+item.OperationID] = i
	}
	outcomes := append([]goal.StopOutcome(nil), batch.CancelOutcomes...)
	outcomeSeen := make(map[string]bool, len(outcomes))
	for _, outcome := range outcomes {
		outcomeSeen[outcome.JobID+"\x00"+outcome.OperationID+"\x00"+outcome.Outcome] = true
	}
	stamp := now.UTC().Format(time.RFC3339)
	recordOutcome := func(jobID, operationID, outcome string) {
		key := jobID + "\x00" + operationID + "\x00" + outcome
		if outcomeSeen[key] {
			return
		}
		outcomeSeen[key] = true
		outcomes = append(outcomes, goal.StopOutcome{
			JobID: jobID, OperationID: operationID, Outcome: outcome, ObservedAt: stamp,
		})
	}
	failure := ""
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		path := filepath.Join(jobsDir, entry.Name())
		record, readErr := readObject(path)
		if readErr != nil {
			failure = fmt.Sprintf("authoritative job record %s is unreadable: %v", entry.Name(), readErr)
			break
		}
		lens := JobRecordOf(record)
		if lens.GoalID() != batch.GoalID {
			continue
		}
		revision, ok := lens.GoalRevision()
		if !ok {
			failure = fmt.Sprintf("goal-bound job record %s is revisionless", entry.Name())
			break
		}
		if revision != batch.GoalRevision {
			continue
		}
		jobID := lens.JobID()
		if jobID == "" || jobID+".json" != entry.Name() {
			failure = fmt.Sprintf("job record %s contradicts its jobId", entry.Name())
			break
		}
		operationID := lens.OperationID()
		if operationID == "" {
			failure = fmt.Sprintf("goal-bound job record %s has no operation generation", entry.Name())
			break
		}
		epoch, epochOK := lens.ClaimEpoch()
		machine := lens.MachineID()
		if machine == "" || !epochOK {
			failure = fmt.Sprintf("goal-bound job record %s has unproven custody coordinates", entry.Name())
			break
		}
		status := lens.Status()
		disposition := stopLocalPending
		if machine != batch.Machine || epoch != batch.ClaimEpoch {
			disposition = stopForeignReportOnly
		} else if TerminalStatus(status) {
			disposition = stopLocalTerminal
		} else if status != "pending-setup" && status != "pending" && status != "running" {
			failure = fmt.Sprintf("job %s has lifecycle status %q", jobID, status)
			break
		}
		generationKey := jobID + "\x00" + operationID
		priorStatus := ""
		if index, exists := observedIndex[generationKey]; exists {
			priorStatus = observed[index].Status
			observed[index].Machine = machine
			observed[index].ClaimEpoch = epoch
			observed[index].Status = status
			observed[index].Disposition = disposition
			observed[index].LastObservedAt = stamp
		} else {
			observedIndex[generationKey] = len(observed)
			observed = append(observed, goal.StopJob{
				JobID: jobID, OperationID: operationID, Machine: machine, ClaimEpoch: epoch,
				Status: status, Disposition: disposition, LastObservedAt: stamp,
			})
		}
		if disposition == stopForeignReportOnly {
			foreign = append(foreign, jobID)
			recordOutcome(jobID, operationID, stopForeignReportOnly)
			continue
		}
		if TerminalStatus(status) {
			terminal = append(terminal, jobID)
			switch {
			case priorStatus == "":
				recordOutcome(jobID, operationID, stopAlreadyTerminal)
			case !TerminalStatus(priorStatus) && status == "cancelled":
				recordOutcome(jobID, operationID, stopCancelled)
			case !TerminalStatus(priorStatus):
				recordOutcome(jobID, operationID, stopTerminalDuringStop)
			}
			continue
		}
		pending = append(pending, jobID)
	}
	batch.Pass++
	batch.Pending, batch.Terminal, batch.Foreign = pending, terminal, foreign
	batch.Observed, batch.CancelOutcomes = observed, outcomes
	batch.UpdatedAt = stamp
	batch.CompletedAt = ""
	batch.Failure = failure
	if failure != "" {
		batch.State = goal.StopBatchIndeterminate
	} else if len(pending) == 0 {
		batch.State = goal.StopBatchComplete
		batch.CompletedAt = batch.UpdatedAt
	} else {
		batch.State = goal.StopBatchOpen
	}
	if err := goal.WriteStopBatch(root, batch); err != nil {
		return goal.StopBatch{}, err
	}
	return batch, nil
}

// AuthorizeStopCancellation proves the exact job is still in the exact open
// batch before shell cancellation takes the job-lifecycle lock.
func AuthorizeStopCancellation(root, stopID, jobID string) error {
	batch, err := goal.ReadStopBatch(root, stopID)
	if err != nil {
		return err
	}
	if batch.State != goal.StopBatchOpen {
		return fmt.Errorf("stop batch %s is %s, not OPEN", stopID, batch.State)
	}
	listed := false
	for _, pending := range batch.Pending {
		if pending == jobID {
			listed = true
			break
		}
	}
	if !listed {
		return fmt.Errorf("job %s is not pending in stop batch %s", jobID, stopID)
	}
	record, err := readObject(filepath.Join(root, "artifacts", "agents", "jobs", jobID+".json"))
	if err != nil {
		return err
	}
	lens := JobRecordOf(record)
	revision, revisionOK := lens.GoalRevision()
	epoch, epochOK := lens.ClaimEpoch()
	if lens.JobID() != jobID || lens.GoalID() != batch.GoalID || !revisionOK || revision != batch.GoalRevision ||
		lens.MachineID() != batch.Machine || !epochOK || epoch != batch.ClaimEpoch {
		return fmt.Errorf("job %s no longer matches stop batch %s authority", jobID, stopID)
	}
	if TerminalStatus(lens.Status()) {
		return nil
	}
	if lens.Status() != "pending-setup" && lens.Status() != "pending" && lens.Status() != "running" {
		return fmt.Errorf("job %s has uncancellable status %q", jobID, lens.Status())
	}
	return nil
}
