package delegation

import (
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/protocol"
)

var commitReviewPattern = regexp.MustCompile(`^commit:[0-9a-f]{40}$`)

// dispatchArgs are dispatch_job's flags.
type dispatchArgs struct {
	role, brief, modeOverride, runtimeOverride, modelOverride  string
	job, reviews, outputs, design, workspace                   string
	permissionsOverride, missionOverride, capOverride          string
	stream, goal, stewardIntent, destructiveReach, approvedRef string
	sources                                                    []string
	useWorktree, workspaceSelected, wait, approveEscalation    bool
	servingGoal                                                bool
}

func (s *session) parseDispatchArgs(args []string) (dispatchArgs, error) {
	var a dispatchArgs
	for index := 0; index < len(args); index++ {
		flag := args[index]
		value := func(target *string) error {
			if index+1 >= len(args) {
				return s.usageExit()
			}
			*target = args[index+1]
			index++
			return nil
		}
		var err error
		switch flag {
		case "--role":
			err = value(&a.role)
		case "--brief":
			err = value(&a.brief)
		case "--mode":
			err = value(&a.modeOverride)
		case "--runtime":
			err = value(&a.runtimeOverride)
		case "--model":
			err = value(&a.modelOverride)
		case "--job-id":
			err = value(&a.job)
		case "--reviews":
			err = value(&a.reviews)
		case "--outputs":
			err = value(&a.outputs)
		case "--design":
			err = value(&a.design)
		case "--workspace":
			err = value(&a.workspace)
			a.workspaceSelected = true
		case "--worktree":
			a.useWorktree = true
		case "--permissions":
			err = value(&a.permissionsOverride)
		case "--mission":
			err = value(&a.missionOverride)
		case "--stream":
			err = value(&a.stream)
		case "--goal":
			err = value(&a.goal)
		case "--destructive-reach":
			err = value(&a.destructiveReach)
		case "--cap-min":
			err = value(&a.capOverride)
		case "--approved-ref":
			err = value(&a.approvedRef)
		case "--source":
			var source string
			err = value(&source)
			a.sources = append(a.sources, source)
		case "--approve-escalation":
			a.approveEscalation = true
		case "--serving-goal":
			a.servingGoal = true
		case "--steward-intent":
			err = value(&a.stewardIntent)
		case "--wait":
			a.wait = true
		default:
			return a, s.usageExit()
		}
		if err != nil {
			return a, err
		}
	}
	return a, nil
}

// dispatchJob is dispatch_job: every admission a fresh delegate launch
// passes, then its reservation, record and launch (finalizeAndLaunch).
func (s *session) dispatchJob(args []string) error {
	if err := s.fenceBrain("dispatch"); err != nil {
		return err
	}
	if err := s.engineSkewPreflight(""); err != nil {
		return err
	}
	a, err := s.parseDispatchArgs(args)
	if err != nil {
		return err
	}
	stewardMode := false
	if a.stewardIntent != "" {
		// The unattended continuation: every launch input comes from the
		// consumed authorization; nothing here is caller-selectable.
		if a.role+a.brief+a.runtimeOverride+a.modelOverride+a.job+a.permissionsOverride+a.missionOverride+a.workspace+a.reviews+a.modeOverride+a.stream+a.goal+a.capOverride+a.approvedRef != "" ||
			a.useWorktree || a.servingGoal || a.wait || a.approveEscalation {
			return s.die(2, "--steward-intent admits no other selection flags; the authorization decides")
		}
		// An inherited mission scope would refuse or rescope the detached
		// continuation; the authorization is the whole context.
		s.env.MissionID, s.env.MissionLease, s.env.MissionTurn = "", "", ""
		authorization, err := s.l.ports.Steward.AuthorizeDispatch(s.inv, a.stewardIntent)
		if err != nil {
			s.eprintln("steward authorize-dispatch: " + err.Error())
			return s.die(1, "steward continuation refused: the authorization did not verify")
		}
		a.role, a.brief, a.job = authorization.Role, authorization.Brief, authorization.JobId
		a.runtimeOverride, a.modelOverride, a.permissionsOverride = authorization.Runtime, authorization.Model, authorization.Permissions
		a.useWorktree = true
		stewardMode = true
		a.destructiveReach = "DESTRUCTIVE-REACH"
	}
	if a.destructiveReach == "" {
		a.destructiveReach = s.env.FixtureHazard
	}
	s.dieJob = a.job
	switch {
	case a.role == "" || !isFile(a.brief):
		return s.usageExit()
	case a.destructiveReach != "MECHANICAL" && a.destructiveReach != "DESIGN-BEARING" && a.destructiveReach != "DESTRUCTIVE-REACH":
		return s.usageExit()
	}
	if a.goal != "" && !validID(a.goal) {
		return s.die(2, "invalid goal id: "+a.goal)
	}
	if !protocol.Dispatchable(a.role) {
		return s.die(1, "unknown dispatch role: "+a.role)
	}
	if err := s.requireOpenDispatchFence(); err != nil {
		return err
	}
	subj := &subject{role: a.role, reviews: a.reviews, goal: a.goal, destructiveReach: a.destructiveReach}
	// Source admission is a pure preflight. A forbidden assertion refuses
	// before a runtime probe, worktree, branch, quarantine, lock, or job
	// exists. The tier is not resolved yet; the validate-only path reads none.
	if len(a.sources) > 0 {
		if _, _, err := dispatch.ValidateRolePacketSources(s.root, a.role, a.sources); err != nil {
			return s.compositionRefusal(err)
		}
	}
	jobLabel := a.job
	if jobLabel == "" {
		jobLabel = a.role
	}
	if err := s.guardAcquire("dispatch " + jobLabel); err != nil {
		return s.die(1, "dispatch refused: checkout execution guard acquisition failed")
	}
	s.trap = func(int) error { return s.guardRelease() }
	if s.env.GuardFixture != "" {
		if err := s.guardFixtureWait(); err != nil {
			return err
		}
		release := s.guardRelease()
		s.trap = nil
		return release
	}
	if a.role == "code-critic" || a.role == "warden" {
		if a.reviews == "" {
			return s.die(2, a.role+" dispatch requires --reviews <implementer-job-id>")
		}
		if a.role == "code-critic" && commitReviewPattern.MatchString(a.reviews) {
			commit := strings.TrimPrefix(a.reviews, "commit:")
			out, _, err := s.l.ports.Git.Run(s.ctx, s.root, "cat-file", "-t", commit)
			if err != nil || strings.TrimSpace(string(out)) != "commit" {
				return s.die(1, "code-critic dispatch --reviews commit subject is not a readable commit: "+a.reviews)
			}
		} else if err := s.requireImplementerReview(a.role, a.reviews); err != nil {
			return err
		}
	} else if a.role == "verifier" && a.reviews != "" {
		if err := s.requireImplementerReview("verifier", a.reviews); err != nil {
			return err
		}
	} else if a.reviews != "" {
		return s.die(2, "--reviews is only valid for the code-critic, warden, and verifier roles")
	}
	if a.role == "design-critic" {
		if a.outputs == "" || a.design == "" {
			return s.die(2, "design-critic dispatch requires --outputs <file> and --design <file>")
		}
	} else if a.outputs+a.design != "" {
		return s.die(2, "--outputs and --design are only valid for the design-critic role")
	}
	if a.useWorktree && a.workspace != "" {
		return s.die(2, "--workspace and --worktree are mutually exclusive")
	}
	if a.approveEscalation && (!s.request.StdinTTY || !s.request.StderrTTY) {
		return s.die(1, "--approve-escalation needs an interactive terminal; nothing was dispatched\nremove the flag, or run the same dispatch from a terminal")
	}
	mode, modeErr := dispatch.BriefModeOnly(a.brief)
	if modeErr != nil {
		_ = s.verbFailure(modeErr)
		return s.die(1, "brief headers are invalid; Working Mode must be filled and Boundary and Ceiling must appear together")
	}
	if a.modeOverride != "" && a.modeOverride != mode {
		return s.die(1, "--mode contradicts the brief's Working Mode header")
	}
	// --serving-goal resolves before any job state exists: a missing usable
	// Current goal refuses the whole dispatch, leaving nothing behind.
	goalSection := ""
	if a.servingGoal {
		section, err := dispatch.ServingGoalSection(s.root)
		if err != nil {
			s.eprintln(err.Error())
			return s.die(3, "no current goal to project (--serving-goal)")
		}
		goalSection = section
	}
	if stewardMode {
		// No lease exists to hold (the worker is provably dead); the
		// internal entries enforce the steward's one-job authority.
		s.inv.CallerClass, s.inv.ClaimEpoch, s.inv.MainID = "STEWARD", nil, ""
	} else if err := s.leaseEntryCheck(); err != nil {
		return err
	}

	// The roster, tier, and escalation decisions live in the engine; this
	// keeps only the approval ladder below.
	roster, rosterErr := dispatch.ResolveRoster(dispatch.RosterParams{
		ConfPath: filepath.Join(s.root, "metasystem.conf"), Role: a.role, Mode: mode,
		RuntimeOverride: a.runtimeOverride, ModelOverride: a.modelOverride, LookupEnv: s.configLookup(),
	})
	if rosterErr != nil {
		s.eprintln(rosterErr.Error())
		s.recordOutcome("REFUSED-ROSTER", "refused", "role roster resolution refused before job claim or launch", "")
		return exitWith(1)
	}
	runtime, model := roster.Runtime, roster.Model
	aliasedFrom := roster.AliasedFrom
	rosterAliasedFrom := roster.RosterAliasedFrom

	missionID, missionLease, missionTurn, err := s.resolveMission(a.missionOverride)
	if err != nil {
		return err
	}
	s.mission = missionID
	_ = missionLease
	// Mission provenance is complete or refused: a mission-scoped dispatch
	// binds the turn it runs in and the stream it serves.
	if a.stream != "" {
		if missionID == "" {
			return s.die(2, "--stream requires a mission context")
		}
		if !validID(a.stream) {
			return s.die(2, "invalid mission stream id: "+a.stream)
		}
	}
	if missionID != "" {
		if missionTurn == "" {
			return s.die(2, "mission dispatch needs a mission runner turn, and none is set here\ndispatch from inside the mission's host turn")
		}
		if a.stream == "" {
			return s.die(2, "mission dispatch requires --stream <mission-stream-id> naming the stream this job serves")
		}
	}
	approvalName, approvedAt := "", ""
	if roster.EscalationRequired {
		switch {
		case a.approveEscalation:
			name, err := s.confirmEscalation(roster.RosterPair, roster.RequestedPair, roster.CostDirection)
			if err != nil {
				return err
			}
			approvalName, approvedAt = name, s.nowISO()
		case missionID != "" && s.signedEnvelopeAllows(missionID, roster.RequestedPair):
		case !roster.TiersPresent:
			return s.die(1, fmt.Sprintf("dispatch escalation refused: %s is requested, the roster gives %s, and no model tiers rank them\nconfigure model.tier.* to rank both, add %s to a signed envelope.dispatch-allow mission contract, or run again from a terminal with --approve-escalation", roster.RequestedPair, roster.RosterPair, roster.RequestedPair))
		default:
			return s.die(1, fmt.Sprintf("dispatch escalation refused: %s is requested, the roster gives %s (cost %s)\nremove the override to use %s, add %s to a signed envelope.dispatch-allow mission contract, or run again from a terminal with --approve-escalation", roster.RequestedPair, roster.RosterPair, roster.CostDirection, roster.RosterPair, roster.RequestedPair))
		}
	} else if a.approveEscalation {
		return s.die(1, "--approve-escalation is not needed: the requested pair needs no escalation approval\nremove the flag")
	}

	permissionName := "none"
	if a.role == "code-critic" || a.role == "design-critic" {
		permissionName = "critic"
	}
	if a.permissionsOverride != "" {
		permissionName = a.permissionsOverride
	} else {
		configured, err := s.configGet("dispatch.permissions."+a.role, permissionName)
		if err != nil {
			return err
		}
		permissionName = configured
	}
	if a.role == "warden" {
		// The warden holds the no-pen seat: its write authority is bound by
		// the role, never by a caller flag, a config key, or a file that
		// shadows a preset name.
		if a.permissionsOverride != "" {
			return s.die(2, "the warden role dispatches with the zero-write preset; --permissions cannot change it")
		}
		permissionName = wardenPermissions
	}
	if !a.useWorktree && !a.workspaceSelected && s.permissionEnvelopeRequestsWrites(permissionName) {
		a.useWorktree = true
	}

	operationBriefHash, err := sha256File(a.brief)
	if err != nil {
		return s.die(1, err.Error())
	}
	goalMachine, goalClaimEpoch := "", ""
	if a.goal != "" {
		binding, err := s.l.ports.Goal.Binding(a.goal)
		if err != nil {
			_ = s.verbFailure(err)
			return s.die(1, fmt.Sprintf("cannot bind delegate operation to accepted goal %s stop authority", a.goal))
		}
		subj.goalRevision, subj.goalTier, subj.goalWidth = binding.Revision, binding.Tier, binding.GateWidth
		goalMachine = binding.Machine
		goalClaimEpoch = strconv.FormatInt(binding.Capability.ClaimEpoch, 10)
		if current := s.currentEpoch(); current != "" && current != goalClaimEpoch {
			return s.die(1, staleClaimMessage(a.goal, subj.goalRevision, goalClaimEpoch, current))
		}
		if err := s.requireGoalTierLadder(subj); err != nil {
			return err
		}
	}
	job := a.job
	if job == "" {
		derived, err := dispatch.DefaultOperationID(a.goal, subj.goalRevision, dispatch.DispatchModeFresh, a.role, operationBriefHash, "")
		if err != nil {
			s.eprintln(err.Error())
			return s.die(1, "could not derive the delegate operation identity")
		}
		job = derived
		// A start refused at setup left a husk holding this name: a repeat
		// of the request is its next attempt under a fresh name, and the
		// husk stays as the record of the refused start.
		var refused []string
		for attempt := 2; attempt <= maxOperationAttempts && s.huskAt(job); attempt++ {
			refused = append(refused, job)
			job = dispatch.OperationAttempt(derived, attempt)
		}
		if len(refused) > 0 {
			s.eprintln(fmt.Sprintf("attempt %d of this request; refused at setup before it: %s", len(refused)+1, strings.Join(refused, ", ")))
		}
	}
	s.dieJob = job
	if !validID(job) {
		return s.die(2, "invalid job id: "+job)
	}
	authorityBase := s.repoScope
	if a.useWorktree && isDir(filepath.Join(s.worktrees, job)) {
		authorityBase = filepath.Join(s.worktrees, job)
	} else if a.workspaceSelected {
		resolved, err := physicalDir(a.workspace)
		if err != nil {
			return s.die(1, "workspace does not exist: "+a.workspace)
		}
		authorityBase = resolved
	}
	if err := s.briefAuthority(a.brief, authorityBase, a.role, a.reviews); err != nil {
		return s.die(1, "brief authority admission refused: "+err.Error())
	}
	// A new goal-free operation has no exact revision seam, so its global
	// budget gate runs before process setup; goal-bound work waits until its
	// revision lock is held.
	recordPath := s.recordPath(job)
	if !exists(recordPath) && a.goal == "" {
		if err := s.requireGoalAdmission(); err != nil {
			return err
		}
	}
	// Preconditions before the id is reserved keep a refused launch from
	// leaving a pending-setup husk that consumes the caller's chosen name.
	if err := s.requireFreshCensus(); err != nil {
		return err
	}
	s.reportPlanDrift()
	reservationClaimEpoch := goalClaimEpoch
	if reservationClaimEpoch == "" {
		reservationClaimEpoch = s.currentEpoch()
	}
	for _, dir := range []string{s.jobs, s.recordLocks, s.capabilities, s.worktrees} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return s.die(1, err.Error())
		}
	}
	if err := s.acquireLaunchChainLock(job); err != nil {
		return err
	}
	s.cleanupJob, s.cleanupChain, s.cleanupAuthorization = job, job, ""
	s.trap = s.launchTrap(false)
	// A payload without a reservation belongs to another operation; beside
	// a reservation, claim-launch resolves it as the same operation.
	payload := filepath.Join(s.agents, job)
	if !exists(recordPath) && exists(payload) {
		return s.die(1, "job payload collision: "+job)
	}
	if a.goal != "" {
		if err := s.acquireGoalRevisionLock(a.goal, subj.goalRevision); err != nil {
			return err
		}
	}

	var launchMode, workspace string
	if a.useWorktree {
		launchMode = "worktree"
		workspace = filepath.Join(s.worktrees, job)
		if !exists(recordPath) {
			if err := s.createJobWorktree(job, workspace); err != nil {
				return err
			}
		}
	} else {
		launchMode = "shared-checkout"
		workspace = a.workspace
		if workspace == "" {
			workspace = s.repoScope
		}
		resolved, err := physicalDir(workspace)
		if err != nil {
			return s.die(1, "workspace does not exist: "+workspace)
		}
		workspace = resolved
	}
	if a.role == "design-critic" {
		if !isFile(a.outputs) {
			return s.die(2, "design-critic --outputs file does not exist: "+a.outputs)
		}
		designCheck := a.design
		if !filepath.IsAbs(designCheck) {
			designCheck = filepath.Join(workspace, designCheck)
		}
		if !isFile(designCheck) {
			return s.die(2, "design-critic --design file does not exist in the reviewed workspace: "+a.design)
		}
	}
	subjectTemp := ""
	if isReviewRole(a.role) {
		temp, err := s.mustTemp(s.recordLocks, "subject")
		if err != nil {
			return err
		}
		subjectTemp = temp
		s.cleanupSubject = subjectTemp
		present, code, message := s.readSubject(dispatch.ReadSubjectRequest{
			RepoRoot: s.root, Role: a.role, Reviews: a.reviews, Workspace: workspace,
			Design: a.design, DeclaredOutputs: a.outputs,
		}, subjectTemp)
		if code != 0 {
			return s.die(code, message)
		}
		if present && fileNonEmpty(subjectTemp) {
			if err := s.admitCritiqueRead(a.role, job, 1, subjectTemp, a.goal); err != nil {
				return err
			}
		}
	}
	brief := a.brief
	if a.role == "implementer" && a.goal != "" {
		composed, err := s.appendTestingRequirement(brief, a.goal, subj.goalWidth, "brief-gate")
		if err != nil {
			if _, ok := err.(*Exit); ok {
				return err
			}
			return s.die(1, "could not compose the implementer's shared testing requirement")
		}
		brief = composed
	}
	if isReviewRole(a.role) && !a.useWorktree && workspace == s.repoScope && s.permissionEnvelopeRequestsWrites(permissionName) {
		return s.die(2, a.role+" refused: a review role could write in the coordinator's checkout\npass --worktree to keep its writes in a worktree of its own")
	}
	permissionJSON, err := s.mustTemp(s.recordLocks, "permissions")
	if err != nil {
		return err
	}
	if err := s.expandPermissions(permissionName, workspace, a.useWorktree, permissionJSON); err != nil {
		return err
	}
	permissionDigest, err := sha256File(permissionJSON)
	if err != nil {
		return s.die(1, err.Error())
	}
	toolPolicy, ok := field(permissionJSON, "tools")
	if !ok {
		return exitWith(1)
	}
	snapshotJSON, err := s.mustTemp(s.recordLocks, "snapshot")
	if err != nil {
		return err
	}
	if err := s.selectSnapshot(runtime, a.role, permissionJSON, snapshotJSON); err != nil {
		return err
	}
	snapshot, err := readSnapshotFields(snapshotJSON)
	if err != nil {
		return exitWith(1)
	}

	// The resolved goal section joins the brief before the hash: it is part
	// of the recorded bytes and survives every fallback rebuild.
	if goalSection != "" {
		composed, err := s.mustTemp(s.recordLocks, "brief")
		if err != nil {
			return err
		}
		content, readErr := os.ReadFile(brief)
		if readErr != nil {
			return s.die(1, readErr.Error())
		}
		if err := os.WriteFile(composed, append(content, []byte("\n"+goalSection)...), 0o600); err != nil {
			return s.die(1, err.Error())
		}
		brief = composed
	}
	if a.role == "design-critic" {
		composed, err := s.appendDeclaredOutputs(brief, a.outputs)
		if err != nil {
			return err
		}
		brief = composed
	}
	withPathForm, err := s.mustTemp(s.recordLocks, "brief-return-path-form")
	if err != nil {
		return err
	}
	if err := appendReturnPathForm(brief, withPathForm); err != nil {
		return s.die(1, err.Error())
	}
	brief = withPathForm

	roundDir := filepath.Join(payload, "rounds", "1")
	// The cap is authorized before the packet is composed so the packet can
	// carry its return-by line.
	productRoots := []string{workspace}
	if err := s.acquireCapAuthorityLock(); err != nil {
		return err
	}
	capResolution, err := s.mustTemp(s.recordLocks, "cap-resolution")
	if err != nil {
		return err
	}
	modelKey := config.CanonicalModel(model)
	if modelKey == "" {
		return s.die(1, "requested model has no canonical cap-key form")
	}
	capTruncated := ""
	if exists(recordPath) && missionID != "" {
		capValue := a.capOverride
		if capValue == "" {
			capValue = fieldOr(recordPath, "capMin")
		}
		if err := s.writeRepeatedCapResolution(capValue, capResolution); err != nil {
			return err
		}
		// The standing reservation's truncation, so the repeat composes the
		// same bytes as the first wrapper.
		capTruncated = fieldOr(recordPath, "capResolution.truncatedBy")
	} else {
		s.cleanupAuthorization = job
		if err := s.authorizeJobCap(job, a.role, runtime, modelKey, aliasedFrom, missionID, a.capOverride, "dispatch", capResolution); err != nil {
			return err
		}
		capTruncated = fieldOr(capResolution, "source.truncatedBy")
	}
	capMin, ok := field(capResolution, "capMin")
	if !ok {
		return exitWith(1)
	}
	composition, err := s.composePacket(composeRequest{
		role: a.role, brief: brief, job: job, runtime: runtime, model: model, toolPolicy: toolPolicy,
		round: 1, mission: missionID, destructiveReach: a.destructiveReach, goalTier: subj.goalTier,
		roundDir: roundDir, capMin: capMin, capTruncated: capTruncated != "" && capTruncated != "null",
		sources: a.sources, prefix: "",
	})
	if err != nil {
		return err
	}
	inputBytes, err := s.enforceInlineInputLimit(composition.prompt, "brief")
	if err != nil {
		return err
	}
	inputHash, err := sha256File(composition.prompt)
	if err != nil {
		return s.die(1, err.Error())
	}
	outputStream, err := s.l.ports.Adapter.OutputStream(s.ctx, runtime, roundDir)
	if err != nil {
		return s.die(1, runtime+" adapter could not resolve its child output stream")
	}
	claim := claimRequest{
		opID: job, operationID: job, session: runtime + ":" + job, dispatchMode: "fresh", resumedSession: "",
		runtime: runtime, model: model, role: a.role, aliasSource: aliasedFrom, reviews: a.reviews,
		launchMode: launchMode, permissionDigest: permissionDigest, productRoots: productRoots,
		capMin: capMin, inputHash: inputHash, goalID: a.goal, goalRevision: subj.goalRevision,
		goalTier: subj.goalTier, gateWidth: subj.goalWidth, destructiveReach: a.destructiveReach, adapterVerb: "dispatch",
	}
	replay, err := s.claimPreflight(claim)
	if err != nil {
		return err
	}
	capNumber, _ := strconv.ParseInt(capMin, 10, 64)
	if !replay {
		if err := s.requireGoalRevisionAdmission(subj, capNumber, "fresh"); err != nil {
			return err
		}
		if err := s.requireGoalAdmission(); err != nil {
			return err
		}
		if err := s.requireSliceAdmission(capNumber, a.approvedRef, a.goal, subj.goalRevision); err != nil {
			return err
		}
	}
	claim.mainID, claim.claimEpoch, claim.machineID, claim.approvedRef = s.inv.MainID, reservationClaimEpoch, goalMachine, a.approvedRef
	launchCapability, creationClaim, err := s.claimReservation(job, job, claim)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(roundDir, 0o755); err != nil {
		return s.die(1, err.Error())
	}
	s.publishSubject(subjectTemp, roundDir)
	if a.role == "code-critic" && commitReviewPattern.MatchString(a.reviews) {
		if err := s.writeCommitSubject(strings.TrimPrefix(a.reviews, "commit:"), a.reviews, roundDir); err != nil {
			return err
		}
	}
	if err := copyFile(brief, filepath.Join(payload, "brief.md")); err != nil {
		return s.die(1, err.Error())
	}
	if err := s.publishComposition(composition, roundDir); err != nil {
		return err
	}

	recordJSON, err := s.mustTemp(s.recordLocks, "record")
	if err != nil {
		return err
	}
	reasoningEffort, ok := field(filepath.Join(roundDir, "composition.json"), "configurationObligations.builderReasoningEffort")
	if !ok {
		return exitWith(1)
	}
	params := dispatch.BuildRecordParams{
		LookupEnv: s.configLookup(), Output: recordJSON, Job: job, Role: a.role, Mission: missionID, MissionTurn: missionTurn, Stream: a.stream,
		Root: s.root, Runtime: runtime, Workspace: workspace, CapResolution: capResolution, Model: model,
		AliasedFrom: aliasedFrom, RosterAliasedFrom: rosterAliasedFrom, Overridden: roster.Overridden,
		Snapshot: snapshot.path, InputBytes: inputBytes, InputHash: inputHash, Permissions: permissionJSON,
		Fallbacks: snapshot.fallbacks, ApprovalName: approvalName, ApprovedAt: approvedAt,
		RosterPair: roster.RosterPair, RequestedPair: roster.RequestedPair, CostDirection: roster.CostDirection,
		Reviews: a.reviews, DeclaredOutputs: a.outputs, Design: a.design, GoalID: a.goal,
		GoalRevision: subj.goalRevision, GoalTier: subj.goalTier, GateWidth: subj.goalWidth, MachineID: goalMachine,
		ApprovedRef: a.approvedRef, DestructiveReach: dispatch.HazardClass(a.destructiveReach),
		ReasoningEffort: reasoningEffort, MainID: s.inv.MainID, ClaimEpoch: reservationClaimEpoch,
		Composition: filepath.Join(roundDir, "composition.json"), LaunchMode: dispatch.LaunchMode(launchMode),
		ProductRoots: productRoots, OutputStream: outputStream,
	}
	if err := setSnapshotBooleans(&params.Signal, &params.HandshakeBudget, snapshot); err != nil {
		s.eprintln(err.Error())
		return exitWith(2)
	}
	if err := s.verbFailure(dispatch.BuildRecord(params)); err != nil {
		return err
	}
	removeQuietly(capResolution)
	return s.finalizeAndLaunch(job, job, recordJSON, runtime, "dispatch", snapshot.handshakeBudget, a.wait, launchCapability, creationClaim)
}

// requireImplementerReview checks --reviews names a known implementer job.
func (s *session) requireImplementerReview(role, reviews string) error {
	if !validID(reviews) {
		return s.die(2, "invalid implementer job id for --reviews: "+reviews)
	}
	record := s.recordPath(reviews)
	if !isFile(record) {
		return s.die(1, fmt.Sprintf("%s dispatch cannot review unknown implementer job: %s", role, reviews))
	}
	if fieldOr(record, "role") != "implementer" {
		return s.die(1, fmt.Sprintf("%s dispatch --reviews must name an implementer job: %s", role, reviews))
	}
	return nil
}

// setSnapshotBooleans parses the snapshot's session signal and handshake
// budget the way build-record's strict flags did.
func setSnapshotBooleans(signal *bool, budget *int64, snapshot snapshotFields) error {
	switch snapshot.signal {
	case "true":
		*signal = true
	case "false":
		*signal = false
	default:
		return fmt.Errorf("invalid value %q for flag -signal: must be true or false", snapshot.signal)
	}
	value, err := strconv.ParseInt(snapshot.handshakeBudget, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid value %q for flag -handshake-budget: parse error", snapshot.handshakeBudget)
	}
	*budget = value
	return nil
}

// physicalDir is `cd DIR && pwd -P`.
func physicalDir(dir string) (string, error) {
	absolute, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", err
	}
	if !isDir(resolved) {
		return "", fmt.Errorf("%s is not a directory", dir)
	}
	return resolved, nil
}

// createJobWorktree adds the job's worktree on a fresh agent/JOB branch and
// links its quarantine object store (issue #5): the delegate's Git writes
// its loose objects into a private directory inside the worktree's own git
// dir and reads the shared store through alternates, so the shared objects/
// stays read-only to the delegate. The quarantine sits outside the
// shippable projection, so the conformance snapshot never sweeps it.
func (s *session) createJobWorktree(job, workspace string) error {
	if exists(workspace) {
		return s.die(1, "job worktree already exists: "+workspace)
	}
	// The worktree is the chain's delegate workspace, registered before it
	// holds a byte and accepted with its gitdir and .git inode once git has
	// made it (engine-owns-disk-lifetimes 3.1, U5f).
	stores := diskstore.CheckoutRegistry(s.root)
	registered, err := stores.Register(diskstore.Registration{Path: workspace, Git: true, Class: diskstore.DelegateClass,
		Owner: diskstore.Owner{Kind: diskstore.OwnerDelegate, Ref: job}, Checkout: s.root, Lifetime: diskstore.LifetimeOwner,
		CapBytes: 8 << 30, CapKind: diskstore.CapTarget, Layout: diskstore.LayoutCopy}, s.l.ports.Clock.Now(), rand.Reader)
	if err != nil {
		return s.die(1, "could not register the job worktree as a store: "+err.Error())
	}
	if _, stderr, err := s.l.ports.Git.Run(s.ctx, s.repoScope, "worktree", "add", "-q", "-b", "agent/"+job, workspace, "HEAD"); err != nil {
		s.stderr.Write(stderr)
		return s.die(1, "could not create job worktree")
	}
	identity, err := diskstore.ReadGitIdentity(workspace)
	if err == nil {
		_, err = stores.Transition(registered.ID, []diskstore.State{diskstore.StateReserved}, diskstore.StateAccepted,
			func(record *diskstore.Record) { record.Identity = identity })
	}
	if err != nil {
		return s.die(1, "could not accept the job worktree's store record: "+err.Error())
	}
	out, _, err := s.l.ports.Git.Run(s.ctx, workspace, "rev-parse", "--absolute-git-dir")
	if err != nil {
		return s.die(1, "could not resolve the worktree git dir")
	}
	quarantine := filepath.Join(strings.TrimSpace(string(out)), "objects-quarantine")
	if err := os.MkdirAll(quarantine, 0o755); err != nil {
		return s.die(1, "could not create the quarantine object store")
	}
	commonOut, _, err := s.l.ports.Git.Run(s.ctx, s.repoScope, "rev-parse", "--git-common-dir")
	if err != nil {
		return exitWith(1)
	}
	common := filepath.Join(strings.TrimSpace(string(commonOut)), "objects")
	if !filepath.IsAbs(common) {
		common = filepath.Join(s.repoScope, common)
	}
	if err := os.MkdirAll(filepath.Join(common, "info"), 0o755); err != nil {
		return exitWith(1)
	}
	// The line is added under the alternates lock by an atomic rewrite,
	// the same one the quarantine absorb removes it by (3.7).
	if err := diskstore.AddAlternate(common, quarantine); err != nil {
		return s.die(1, "could not link the quarantine into the shared object store")
	}
	return nil
}

// confirmEscalation is confirm_escalation: the interactive APPROVE <name>.
func (s *session) confirmEscalation(rosterPair, requestedPair, costDirection string) (string, error) {
	s.eprintf("Roster resolution: %s\n", rosterPair)
	s.eprintf("Requested pair: %s\n", requestedPair)
	s.eprintf("Cost direction: %s\n", costDirection)
	s.eprintf("Type APPROVE <name> to confirm: ")
	confirmation := readLine(s.request.Stdin)
	if !strings.HasPrefix(confirmation, "APPROVE ") {
		return "", s.die(1, "escalation approval declined; nothing was dispatched\nrun again without the override, or from a terminal with --approve-escalation and type APPROVE and your name")
	}
	name := strings.TrimPrefix(confirmation, "APPROVE ")
	if name == "" || strings.TrimLeft(name, " \t\n\v\f\r") != name || strings.TrimRight(name, " \t\n\v\f\r") != name || strings.ContainsFunc(name, isControl) {
		return "", s.die(1, "escalation approval declined: the name after APPROVE is empty or badly formed\ntype APPROVE followed by a name without leading, trailing or control characters")
	}
	return name, nil
}

// resolveMission is resolve_mission: an explicit id, or the inherited
// mission context, validated against its live lease.
func (s *session) resolveMission(explicit string) (string, string, string, error) {
	envID, envLease, envTurn := s.env.MissionID, s.env.MissionLease, s.env.MissionTurn
	if (envID != "" || envLease != "") && (envID == "" || envLease == "") {
		return "", "", "", s.die(1, "the inherited mission context is incomplete: it names a mission or a lease, not both\nrun it from the mission's own turn, or pass --mission")
	}
	if envTurn != "" && envID == "" && explicit == "" {
		return "", "", "", s.die(1, "the inherited mission context names a runner turn but no mission\npass --mission, or run it from the mission's own turn")
	}
	if envTurn != "" && !validID(envTurn) {
		return "", "", "", s.die(1, "invalid inherited mission turn id")
	}
	if explicit != "" && envID != "" && explicit != envID {
		return "", "", "", s.die(1, "--mission names another mission than the one this process runs in\ndrop --mission, or run it outside that mission")
	}
	missionID := explicit
	if missionID == "" {
		missionID = envID
	}
	if missionID == "" {
		return "", "", "", nil
	}
	leasePath := envLease
	if leasePath == "" {
		leasePath = filepath.Join(s.agents, "missions", missionID, "lease.json")
	}
	if err := dispatch.ValidateMission(s.root, missionID, leasePath); err != nil {
		_ = s.verbFailure(err)
		return "", "", "", s.die(1, fmt.Sprintf("mission %s does not have a live, matching lease", missionID))
	}
	return missionID, leasePath, envTurn, nil
}

// readSubject is `job read-subject`: the critic round's read subject, or the
// verb's exit code and output.
func (s *session) readSubject(request dispatch.ReadSubjectRequest, output string) (bool, int, string) {
	subject, present, err := dispatch.ComputeReadSubject(request)
	if err != nil {
		s.noteRefusal(err)
		code := verbCode(err)
		message := err.Error()
		var op *dispatch.OpError
		if asOpError(err, &op) {
			message = op.Error()
		}
		return false, code, message
	}
	if !present {
		return false, 0, ""
	}
	if output != "" {
		if err := dispatch.WriteReadSubject(output, subject); err != nil {
			message := err.Error()
			var op *dispatch.OpError
			if asOpError(err, &op) {
				message = op.Error()
			}
			return false, verbCode(err), message
		}
	}
	return true, 0, ""
}

// appendTestingRequirement composes the implementer's shared testing
// requirement onto a brief (a fresh temporary).
func (s *session) appendTestingRequirement(brief, goalID, gateWidth, prefix string) (string, error) {
	composed, err := s.mustTemp(s.recordLocks, prefix)
	if err != nil {
		return "", err
	}
	content, err := os.ReadFile(brief)
	if err != nil {
		return "", err
	}
	requirement, err := dispatch.TestingRequirement(goalID, gateWidth)
	if err != nil {
		_ = s.verbFailure(err)
		return "", err
	}
	requirement += dispatch.PeerMessagesPointer(dispatch.PeerMessagesWaiting(s.root))
	if err := os.WriteFile(composed, append(content, []byte(requirement)...), 0o600); err != nil {
		return "", err
	}
	return composed, nil
}

// appendDeclaredOutputs adds the design critic's declared-outputs manifest
// and its digest to the brief. The digest is of the manifest, never of the
// design page: a critic hashing the page against it stops on a false
// mismatch.
func (s *session) appendDeclaredOutputs(brief, outputs string) (string, error) {
	digest, err := sha256File(outputs)
	if err != nil {
		return "", s.die(1, err.Error())
	}
	composed, err := s.mustTemp(s.recordLocks, "brief-outputs")
	if err != nil {
		return "", err
	}
	content, err := os.ReadFile(brief)
	if err != nil {
		return "", s.die(1, err.Error())
	}
	manifest, err := os.ReadFile(outputs)
	if err != nil {
		return "", s.die(1, err.Error())
	}
	var builder strings.Builder
	builder.Write(content)
	builder.WriteString("\n## Declared Outputs\n\nThe design declares these output paths (the manifest; its SHA-256 digest follows). The design page itself is read at the path the brief names and carries no declared digest.\n\n")
	for _, line := range splitLinesKeepLast(string(manifest)) {
		builder.WriteString("- " + line + "\n")
	}
	fmt.Fprintf(&builder, "\n- SHA-256 digest of this manifest: %s\n", digest)
	if err := os.WriteFile(composed, []byte(builder.String()), 0o600); err != nil {
		return "", s.die(1, err.Error())
	}
	return composed, nil
}

// splitLinesKeepLast splits text into sed's input lines: a final line
// without a newline is still a line.
func splitLinesKeepLast(text string) []string {
	if text == "" {
		return nil
	}
	parts := strings.Split(text, "\n")
	if parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	return parts
}

// writeRepeatedCapResolution is `job cap-resolution --rule
// repeated-operation --origin existing-reservation`.
func (s *session) writeRepeatedCapResolution(capValue, output string) error {
	capMin, err := strconv.ParseInt(capValue, 10, 64)
	if err != nil || capMin < 1 {
		s.eprintln("job cap-resolution: --cap (>=1), --rule, --origin, and --output are required")
		return exitWith(2)
	}
	return s.verbFailure(dispatch.WriteCapResolution(output, capMin, "repeated-operation", "existing-reservation"))
}

// writeCommitSubject records a code-critic commit review's diff and tree.
func (s *session) writeCommitSubject(commit, reviews, roundDir string) error {
	diff, _, err := s.l.ports.Git.Run(s.ctx, s.root, "diff", "--binary", "--full-index", commit+"^", commit)
	if err != nil || os.WriteFile(filepath.Join(roundDir, "diff.patch"), diff, 0o644) != nil {
		return s.die(1, "code-critic commit subject has no readable parent diff: "+reviews)
	}
	tree, _, err := s.l.ports.Git.Run(s.ctx, s.root, "rev-parse", commit+"^{tree}")
	if err != nil || os.WriteFile(filepath.Join(roundDir, "reviewedTree"), tree, 0o644) != nil {
		return s.die(1, "code-critic commit subject has no readable tree: "+reviews)
	}
	return nil
}

// publishSubject moves a computed read subject into the round directory.
func (s *session) publishSubject(subjectTemp, roundDir string) {
	if subjectTemp != "" && fileNonEmpty(subjectTemp) {
		_ = os.Rename(subjectTemp, filepath.Join(roundDir, "subject.json"))
	}
	removeQuietly(subjectTemp)
	s.cleanupSubject = ""
}

// launchTrap is the dispatch and follow-up EXIT trap: a failed command
// fails its setup husk and releases an unpublished fence authorization;
// every exit closes the creation claim, removes temporaries and releases
// the ranked locks and the execution guard.
func (s *session) launchTrap(followUp bool) func(int) error {
	return func(code int) error {
		if code != 0 {
			s.failSetupHusk(s.cleanupJob)
			s.releaseUnpublishedAuthorization(s.cleanupAuthorization)
		}
		s.cleanupCreationClaimQuietly()
		s.cleanupCompositionTemporaries()
		s.cleanupSubjectTemp()
		if followUp {
			s.cleanupFollowUpMessage()
		}
		var first error
		keep := func(err error) {
			if err != nil && first == nil {
				first = err
			}
		}
		keep(s.releaseCapAuthorityLock())
		s.releaseExitLifecycle()
		keep(s.releaseGoalRevisionLock())
		keep(s.releaseChainLock(s.cleanupChain))
		if !followUp {
			_ = s.guardRelease()
		}
		return first
	}
}

func readLine(reader interface{ Read([]byte) (int, error) }) string {
	if reader == nil {
		return ""
	}
	var line []byte
	buffer := make([]byte, 1)
	for {
		count, err := reader.Read(buffer)
		if count > 0 {
			if buffer[0] == '\n' {
				break
			}
			line = append(line, buffer[0])
		}
		if err != nil {
			break
		}
	}
	return string(line)
}

func isControl(r rune) bool { return r < 0x20 || r == 0x7f }

// maxOperationAttempts bounds the fresh names one request may take after
// starts refused at setup.
const maxOperationAttempts = 20

// huskAt says whether job's record is a start refused at setup.
func (s *session) huskAt(job string) bool {
	record, err := dispatch.ReadRecordObject(s.recordPath(job))
	return err == nil && dispatch.NeverLaunched(record)
}
