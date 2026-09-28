package delegation

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/authority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/capability"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/contract"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/mission"
)

// subject is the launch a dispatch or follow-up is admitting: the fields the
// shared admission helpers read from dispatch.sh's dynamically scoped locals.
type subject struct {
	role             string
	reviews          string
	goal             string
	goalRevision     uint64
	goalTier         uint8
	goalWidth        string
	destructiveReach string
}

// leaseEntryCheck is lease_entry_check: the caller must be the lease holder
// (or a caller the lease admits ungated); its epoch and main id are kept.
func (s *session) leaseEntryCheck() error {
	view, err := s.l.ports.Lease.RequireHolder(s.inv, nil)
	if err != nil {
		s.eprintln(err.Error())
		return exitWith(1)
	}
	s.inv.ClaimEpoch = view.ClaimEpoch
	s.inv.MainID = ""
	if view.MainId != nil {
		s.inv.MainID = *view.MainId
	}
	s.inv.CallerClass = view.Class
	return nil
}

func (s *session) currentEpoch() string {
	if s.inv.ClaimEpoch == nil {
		return ""
	}
	return strconv.FormatInt(*s.inv.ClaimEpoch, 10)
}

// runHeld is lease_run_held: fn runs under the lease at the epoch (the lease
// owner runs a steward caller ungated; its authority is checked per write).
func (s *session) runHeld(epoch *int64, fn func() error) error {
	var ran bool
	err := s.l.ports.Lease.Held(s.inv, epoch, func() error {
		ran = true
		return fn()
	})
	if !ran && err != nil {
		s.eprintln(err.Error())
		return exitWith(1)
	}
	return err
}

// internalAuthority is internal_authority: classify the supplied caller and
// judge it against the matrix for mode.
func (s *session) internalAuthority(mode AuthorityMode, job string) error {
	if !authority.ValidMode(string(mode)) {
		return s.die(1, fmt.Sprintf("unknown control-plane mode %q", mode))
	}
	if err := s.l.ports.Lease.Authorize(s.inv, mode, job); err != nil {
		if strings.HasPrefix(err.Error(), "control-plane write refused: caller classification failed") {
			return s.die(1, "control-plane write refused: caller classification failed")
		}
		s.eprintln(err.Error())
		return exitWith(1)
	}
	return nil
}

// requireGoalAdmission is require_goal_admission.
func (s *session) requireGoalAdmission() error {
	now, err := s.goalNow()
	if err != nil {
		s.eprintln(err.Error())
		s.recordOutcome("BUDGET_UNKNOWN", "refused", err.Error(), s.outcomeJob())
		return s.die(1, "dispatch refused because the governing goal admission could not be evaluated")
	}
	verdict, err := dispatch.EvaluateGoalAdmission(s.root, s.env.OwnerLineage, now)
	if err != nil {
		s.eprintln(err.Error())
		s.recordOutcome("BUDGET_UNKNOWN", "refused", err.Error(), s.outcomeJob())
		return s.die(1, "dispatch refused because the governing goal admission could not be evaluated")
	}
	output := strings.Join(dispatch.FormatGoalAdmission(verdict), "\n")
	if output != "" {
		s.eprintln(output)
	}
	if !verdict.Refused() {
		return nil
	}
	for _, refusal := range verdict.Refusals {
		if refusal.LiveStopReason != "" {
			s.recordOutcome("REFUSED-BUDGET", "refused", output, s.outcomeJob())
			// Breach-stop owns the next locks. Drop every admission lock
			// before it enters cancellation so the stop path cannot wait on
			// this dispatcher.
			if err := s.releaseCapAuthorityLock(); err != nil {
				return err
			}
			if err := s.releaseGoalRevisionLock(); err != nil {
				return err
			}
			if s.cleanupChain != "" {
				if err := s.releaseChainLock(s.cleanupChain); err != nil {
					return err
				}
				s.cleanupChain = ""
			}
			if err := s.runBreachStopRoutes(); err != nil {
				return err
			}
			return s.die(1, "dispatch refused: breach-stop closed admission and wound down the breached revision")
		}
	}
	s.recordOutcome("REFUSED-BUDGET", "refused", output, "")
	return s.die(1, "dispatch refused by the goal admission verdict above; supply or revise the governing structured budget before another round")
}

// runBreachStopRoutes is run_breach_stop_routes.
func (s *session) runBreachStopRoutes() error {
	now, err := s.goalNow()
	if err != nil {
		s.eprintln(err.Error())
		return s.die(1, "breach-stop routes could not be resolved")
	}
	routes, err := dispatch.FindBreachStops(s.root, now)
	if err != nil {
		_ = s.verbFailure(err)
		return s.die(1, "breach-stop routes could not be resolved")
	}
	if len(routes) == 0 {
		return s.die(1, "goal admission required breach-stop but supplied no stoppable route")
	}
	for _, route := range routes {
		if route.GoalID == "" {
			continue
		}
		if route.Failure != "" {
			return s.die(1, fmt.Sprintf("breach-stop for %s revision %d is indeterminate: %s", route.GoalID, route.Revision, route.Failure))
		}
		batch, err := s.breachStop(route.GoalID, route.Revision)
		if err != nil {
			return s.die(1, fmt.Sprintf("breach-stop could not close %s revision %d", route.GoalID, route.Revision))
		}
		if err := s.breachStopRun(batch.StopID); err != nil {
			return err
		}
	}
	return nil
}

// breachStop is `job breach-stop`: the stop-custodian gate for the supplied
// caller, then the idempotent stop batch for one exact revision.
func (s *session) breachStop(goalID string, revision uint64) (goal.StopBatch, error) {
	caller, err := s.l.ports.Lease.Classify(s.inv)
	if err != nil {
		s.eprintln(fmt.Sprintf("job breach-stop: caller authority is unreadable: %v", err))
		return goal.StopBatch{}, exitWith(1)
	}
	if err := authority.Authorize("stop-custodian", map[string]any{"class": caller.Class, "holder": caller.Holder}, ""); err != nil {
		s.eprintln(err.Error())
		return goal.StopBatch{}, exitWith(1)
	}
	now, err := s.goalNow()
	if err != nil {
		return goal.StopBatch{}, s.verbFailure(err)
	}
	// Rule H1: a person may order the stop the custodian would take, and
	// the fence closure names that person as its actor.
	orderedBy := ""
	if caller.Class == lease.ClassHuman {
		orderedBy, err = s.l.ports.Host.BreachStopOrderingHuman(context.Background(), s.root, s.inv.CallerPid, now)
		if err != nil {
			s.eprintln("job breach-stop: " + err.Error())
			return goal.StopBatch{}, exitWith(1)
		}
	}
	batch, err := s.l.ports.Goal.BreachStop(goalID, revision, now, orderedBy)
	if err != nil {
		return goal.StopBatch{}, s.verbFailure(err)
	}
	return batch, nil
}

// requireGoalRevisionAdmission is require_goal_revision_admission.
func (s *session) requireGoalRevisionAdmission(subj *subject, proposed int64, dispatchMode string) error {
	if subj.goal == "" {
		return nil
	}
	evaluate := func() (dispatch.GoalRevisionAdmission, error) {
		now, err := s.goalNow()
		if err != nil {
			return dispatch.GoalRevisionAdmission{}, err
		}
		return dispatch.EvaluateGoalRevisionAdmissionForDispatchWithReads(s.root, subj.goal, subj.goalRevision, uint64(proposed), now,
			subj.role, dispatchMode, dispatch.ConcreteProofAdmissionReads(), dispatch.HazardClass(subj.destructiveReach))
	}
	verdict, err := evaluate()
	if err == nil && verdict.LiveStopReason == "" && verdict.Refused() && verdict.Extension != nil {
		extension := verdict.Extension
		if extension.EvidenceKind == "" || extension.EvidenceID == "" || extension.EvidenceAt == "" {
			return s.die(1, "dispatch refused because the budget extension offer is incomplete")
		}
		// The dispatcher's owner lock and the extension owner use the same
		// path; the owner acquires it, replays the exact seam and publishes
		// atomically, then this dispatch reacquires before judging again.
		if err := s.releaseGoalRevisionLock(); err != nil {
			return err
		}
		output, _ := s.l.ports.Host.ExtendBudget(s.ctx, ExtendBudgetRequest{
			Root: s.root, GoalID: subj.goal, Revision: subj.goalRevision, ProposedCap: uint64(proposed),
			Role: subj.role, DispatchMode: dispatchMode, DestructiveReach: subj.destructiveReach,
			CallerPid: s.inv.CallerPid, OwnerLineage: s.env.OwnerLineage,
		})
		if output = strings.TrimRight(output, "\n"); output != "" {
			s.eprintln(output)
		}
		// A refused extension (another dispatch spent the marker first, the
		// box moved, the owner lost the lock race) is judged again below and
		// refuses through the ordinary budget path, never as an internal fault.
		if err := s.acquireGoalRevisionLock(subj.goal, subj.goalRevision); err != nil {
			return err
		}
		verdict, err = evaluate()
	}
	if err != nil {
		message := err.Error()
		var op *dispatch.OpError
		if asOpError(err, &op) {
			message = op.Error()
		}
		s.eprintln(message)
		s.recordOutcome("BUDGET_UNKNOWN", "refused", message, s.outcomeJob())
		return s.die(1, "dispatch refused because exact goal revision admission could not be evaluated")
	}
	if !verdict.Refused() && verdict.LiveStopReason == "" {
		return nil
	}
	// The text rendering the retired shell relayed on a refusal: the policy
	// notice, then the policy refusal or the verdict's lines.
	var rendered []string
	if verdict.PolicyNotice != "" {
		rendered = append(rendered, verdict.PolicyNotice)
	}
	liveStop := false
	switch {
	case !verdict.Refused():
	case verdict.PolicyRefusal != "":
		rendered = append(rendered, verdict.PolicyRefusal)
	default:
		rendered = append(rendered, dispatch.FormatGoalRevisionAdmission(verdict)...)
		liveStop = verdict.LiveStopReason != ""
	}
	output := strings.Join(rendered, "\n")
	if output != "" {
		s.eprintln(output)
	}
	if !verdict.Refused() {
		return nil
	}
	if liveStop {
		s.recordOutcome("REFUSED-BUDGET", "refused", output, s.outcomeJob())
		// Stop takes the goal lock without the lower-ranked cap lock.
		if err := s.releaseCapAuthorityLock(); err != nil {
			return err
		}
		if err := s.releaseGoalRevisionLock(); err != nil {
			return err
		}
		batch, err := s.breachStop(subj.goal, subj.goalRevision)
		if err != nil {
			return s.die(1, fmt.Sprintf("breach-stop could not close %s revision %d", subj.goal, subj.goalRevision))
		}
		if err := s.releaseChainLock(s.cleanupChain); err != nil {
			return err
		}
		s.cleanupChain = ""
		if err := s.breachStopRun(batch.StopID); err != nil {
			return err
		}
		return s.die(1, fmt.Sprintf("dispatch refused: breach-stop %s closed the launch fence and completed its cancellation pass", batch.StopID))
	}
	s.recordOutcome("REFUSED-BUDGET", "refused", output, s.outcomeJob())
	return s.die(1, "dispatch refused by the exact goal revision admission verdict above")
}

// requireGoalTierLadder is require_goal_tier_ladder.
func (s *session) requireGoalTierLadder(subj *subject) error {
	if subj.goal == "" {
		return nil
	}
	switch subj.goalTier {
	case 1:
		if isReviewRole(subj.role) || subj.reviews != "" {
			return s.die(1, fmt.Sprintf("tier 1 goal %s refuses critic roles and --reviews; raise it first with goal edit --tier 2", subj.goal))
		}
	case 2:
		if subj.role == "design-critic" {
			return s.die(1, fmt.Sprintf("tier 2 goal %s refuses the design-critic role; raise it first with goal edit --tier 3", subj.goal))
		}
	case 3:
	default:
		return s.die(1, fmt.Sprintf("goal %s has no usable claimed-revision tier", subj.goal))
	}
	return nil
}

// requireSliceAdmission is require_slice_admission.
func (s *session) requireSliceAdmission(proposed int64, approval, approvalGoal string, approvalRevision uint64) error {
	verdict, err := dispatch.EvaluateSliceAdmission(s.root, uint64(proposed), approval, approvalGoal, approvalRevision)
	if err != nil {
		s.eprintln(err.Error())
		s.recordOutcome("SLICE_ADMISSION_UNKNOWN", "refused", err.Error(), s.outcomeJob())
		return s.die(1, "dispatch refused because slice-cap admission could not be evaluated")
	}
	if !verdict.Refused() {
		return nil
	}
	output := encodeJSON(map[string]any{"outcome": verdict.Reason, "headline": "refused", "detail": verdict.Refusal})
	s.eprintln(output)
	s.recordOutcomeRaw(output)
	return s.die(1, "dispatch refused by the slice-cap admission verdict above")
}

// resolveNonmissionCap is resolve_nonmission_cap (job resolve-cap --output).
func (s *session) resolveNonmissionCap(role, runtime, model, aliasSource, requested, output string) error {
	capMin, rule, origin, err := dispatch.ResolveCap(filepath.Join(s.root, "metasystem.conf"), role, runtime, model, aliasSource, requested)
	if err != nil {
		s.eprintln(err.Error())
		return exitWith(1)
	}
	return s.verbFailure(dispatch.WriteCapResolution(output, capMin, rule, origin))
}

// authorizeJobCap is authorize_job_cap.
func (s *session) authorizeJobCap(job, role, runtime, modelKey, aliasSource, missionID, override, noun, output string) error {
	if missionID != "" {
		if err := dispatch.RefuseUnsignedMissionCap(filepath.Join(s.root, "metasystem.conf"), role, runtime, modelKey, aliasSource); err != nil {
			s.eprintln(err.Error())
			return exitWith(1)
		}
		result, err := s.missionAuthorizeCap(missionID, job, runtime, modelKey, aliasSource, override)
		if err != nil {
			return s.die(1, fmt.Sprintf("mission %s refused by the mission fence: %s", noun, err.Error()))
		}
		if err := os.WriteFile(output, []byte(result+"\n"), 0o644); err != nil {
			return s.die(1, err.Error())
		}
	} else if err := s.resolveNonmissionCap(role, runtime, modelKey, aliasSource, override, output); err != nil {
		return err
	}
	capValue := fieldOr(output, "capMin")
	if !positiveInteger(capValue) {
		return s.die(1, "dispatch cap authority returned an invalid capMin")
	}
	capMin, _ := strconv.ParseInt(capValue, 10, 64)
	watchCap, err := dispatch.WatcherCeiling(filepath.Join(s.agents, "supervision", "state.json"), s.l.ports.Clock.Now())
	if err != nil {
		return s.verbFailure(err)
	}
	if capMin >= watchCap {
		return s.die(1, fmt.Sprintf("dispatch cap %dm must stay below the live watcher's attested %dm ceiling; re-arm supervision with --rearm --max-cap %d", capMin, watchCap, capMin))
	}
	return nil
}

// missionAuthorizeCap is `mission fence-authorize-cap`; the error text is
// what the verb printed.
func (s *session) missionAuthorizeCap(missionID, job, runtime, model, aliasSource, requested string) (string, error) {
	var requestedPtr *int
	if requested != "" {
		value, err := strconv.Atoi(requested)
		if err != nil {
			return "", fmt.Errorf("invalid value %q for flag -requested: parse error", requested)
		}
		requestedPtr = &value
	}
	if !validID(missionID) {
		return "", fmt.Errorf("invalid mission id")
	}
	if !validID(job) || !validID(runtime) || model == "" || model != config.CanonicalModel(model) ||
		(aliasSource != "" && aliasSource != config.CanonicalModel(aliasSource)) || (requestedPtr != nil && *requestedPtr < 1) {
		return "", fmt.Errorf("invalid mission cap authorization request")
	}
	clock, _, err := s.goalClock()
	if err != nil {
		return "", err
	}
	result, err := mission.AuthorizeCapWithClock(s.root, missionID, job, runtime, model, aliasSource, requestedPtr, clock)
	if err != nil {
		return "", err
	}
	encoded, _ := json.Marshal(result)
	return string(encoded), nil
}

// missionReleaseJob is `mission fence-release-job ... || true`.
func (s *session) missionReleaseJob(missionID, job string) {
	if !validID(missionID) || job == "" {
		return
	}
	clock, _, err := s.goalClock()
	if err != nil {
		return
	}
	_ = mission.ReleaseJobWithClock(s.root, missionID, job, clock)
}

// missionAggregateUsage is `mission fence-aggregate-usage`.
func (s *session) missionAggregateUsage(missionID string) error {
	if !validID(missionID) {
		s.eprintln("invalid mission id")
		return exitWith(2)
	}
	if err := mission.AggregateUsage(s.root, missionID); err != nil {
		s.eprintln(err.Error())
		return exitWith(1)
	}
	return nil
}

// missionRefuse is `mission fence-refuse`; the error is the verb's stderr.
func (s *session) missionRefuse(missionID, reason string) error {
	if !validID(missionID) {
		return fmt.Errorf("invalid mission id")
	}
	_, err := mission.Refuse(s.root, missionID, reason)
	return err
}

// signedEnvelopeAllows is signed_dispatch_envelope_allows.
func (s *session) signedEnvelopeAllows(missionID, pair string) bool {
	return contract.DispatchEnvelopeAllows(s.root, missionID, pair) == nil
}

// selectSnapshot is select_snapshot: select the capability snapshot for the
// runtime's configuration identity, self-healing a genuine miss with one
// probe.
func (s *session) selectSnapshot(runtime, role, envelope, output string) error {
	adapter := s.l.ports.Adapter
	if !adapter.Installed(runtime) {
		return s.die(1, "runtime adapter is not installed: "+runtime)
	}
	identity, err := adapter.ConfigIdentity(s.ctx, runtime)
	if err != nil {
		return s.die(1, fmt.Sprintf("could not read %s adapter configuration identity", runtime))
	}
	maxAgeText, err := s.configGet("capability.snapshot-max-age-days", "30")
	if err != nil {
		return err
	}
	if !naturalPattern.MatchString(maxAgeText) {
		return s.die(1, "capability.snapshot-max-age-days must be a non-negative integer")
	}
	maxAge, _ := strconv.Atoi(maxAgeText)
	err = capability.Select(s.root, runtime, role, identity, maxAge, envelope, output)
	if err == nil {
		return nil
	}
	s.eprintln(err.Error())
	// Self-heal ONLY a genuine snapshot miss, absent or stale. A select that
	// found a snapshot and refused on policy must stand: a fresh probe would
	// launder the unverified state away.
	if !strings.Contains(err.Error(), "no capability snapshot matches") && !strings.Contains(err.Error(), "capability snapshot is stale") {
		return exitWith(1)
	}
	if err := adapter.Probe(s.ctx, runtime); err != nil {
		return s.die(1, fmt.Sprintf("capability snapshot missed and the %s adapter probe failed", runtime))
	}
	identity, err = adapter.ConfigIdentity(s.ctx, runtime)
	if err != nil {
		return s.die(1, fmt.Sprintf("could not read %s adapter configuration identity", runtime))
	}
	if err := capability.Select(s.root, runtime, role, identity, maxAge, envelope, output); err != nil {
		s.eprintln(err.Error())
		return exitWith(1)
	}
	return nil
}

// snapshotFields is read_snapshot_fields.
type snapshotFields struct {
	path            string
	fallbacks       string
	signal          string
	handshakeBudget string
	resume          string
}

func readSnapshotFields(path string) (snapshotFields, error) {
	var fields snapshotFields
	var ok bool
	if fields.path, ok = field(path, "path"); !ok {
		return fields, fmt.Errorf("snapshot selection carries no path")
	}
	if fields.fallbacks, ok = field(path, "fallbacks"); !ok {
		return fields, fmt.Errorf("snapshot selection carries no fallbacks")
	}
	if fields.signal, ok = field(path, "sessionEstablishedSignal"); !ok {
		return fields, fmt.Errorf("snapshot selection carries no session signal")
	}
	if fields.handshakeBudget, ok = field(path, "sessionEstablishedTimeoutSec"); !ok {
		return fields, fmt.Errorf("snapshot selection carries no handshake budget")
	}
	fields.resume = fieldOr(path, "resume")
	return fields, nil
}

// enforceInlineInputLimit is enforce_inline_input_limit.
func (s *session) enforceInlineInputLimit(content, hint string) (int64, error) {
	maxKB, err := s.configGet("dispatch.max-inline-input-kb", "64")
	if err != nil {
		return 0, err
	}
	if !positiveInteger(maxKB) {
		return 0, s.die(1, "dispatch.max-inline-input-kb must be a positive integer")
	}
	limit, _ := strconv.ParseInt(maxKB, 10, 64)
	info, err := os.Stat(content)
	if err != nil {
		return 0, s.die(1, err.Error())
	}
	if info.Size() > limit*1024 {
		return 0, s.die(1, "inline input exceeds dispatch.max-inline-input-kb; pass a file reference in the "+hint)
	}
	return info.Size(), nil
}

// expandPermissions is expand_permissions.
func (s *session) expandPermissions(requested, workspace string, isWorktree bool, output string) error {
	presets := filepath.Join(s.root, "scripts", "agents", "permissions")
	var source, preset string
	switch {
	case strings.HasPrefix(requested, presets+"/") && strings.HasSuffix(requested, ".json"):
		source = requested
		preset = strings.TrimSuffix(filepath.Base(requested), ".json")
	case isFile(requested):
		source, preset = requested, "custom"
	default:
		source, preset = filepath.Join(presets, requested+".json"), requested
	}
	if !isFile(source) {
		return s.die(1, "unknown permissions preset or envelope file: "+requested)
	}
	networkFloor, err := s.configGet("dispatch.permissions.network", "")
	if err != nil {
		return err
	}
	switch networkFloor {
	case "", "deny", "allow":
	default:
		return s.die(1, "dispatch.permissions.network must be deny or allow")
	}
	return s.verbFailure(dispatch.ExpandPermissions(source, s.repoScope, workspace, isWorktree, preset, networkFloor, output))
}

// permissionEnvelopeRequestsWrites is permission_envelope_requests_writes.
func (s *session) permissionEnvelopeRequestsWrites(requested string) bool {
	source := requested
	if !isFile(requested) {
		source = filepath.Join(s.root, "scripts", "agents", "permissions", requested+".json")
	}
	if !isFile(source) {
		return false
	}
	roots, ok := field(source, "writeRoots")
	if !ok || !strings.HasPrefix(roots, "[") {
		return false
	}
	return roots != "[]"
}

func isReviewRole(role string) bool {
	return role == "code-critic" || role == "design-critic" || role == "warden"
}

// briefAuthority is brief_authority.
func (s *session) briefAuthority(brief, baseTree string) error {
	_, err := dispatch.ReadBriefAdmissionAtRoot(brief, s.root, baseTree, s.repoScope, false)
	return s.verbFailure(err)
}

// appendReturnPathForm is append_return_path_form.
func appendReturnPathForm(source, destination string) error {
	const sentence = "Every path in your return (diffBoundary, files) is relative to the repository root, so it starts with `metasystem/`."
	content, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	out := append([]byte(nil), content...)
	present := false
	for _, line := range strings.Split(string(content), "\n") {
		if line == sentence {
			present = true
			break
		}
	}
	if !present {
		out = append(out, []byte("\n# Return path form\n\n"+sentence+"\n")...)
	}
	return os.WriteFile(destination, out, 0o600)
}

// rootJobID is `adapter root-job`.
func (s *session) rootJobID(job string) (string, error) {
	return usageRootJobID(s.jobs, job)
}
