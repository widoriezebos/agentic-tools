package delegation

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatchproc"
)

var followUpRoundPattern = regexp.MustCompile(`^([2-9]|[1-9][0-9]+)$`)

// followUp is follow_up: the chain's newest record decides between a fresh
// round, a repeated wrapper, a continuation after a cap and an examination
// retry; a worktree chain may be rebased onto trunk; critic chains fold
// their register and carry open finding ids; then the shared authorize,
// compose, claim, record and launch tail.
func (s *session) followUp(args []string) error {
	if err := s.fenceBrain("follow-up"); err != nil {
		return err
	}
	if err := s.engineSkewPreflight(""); err != nil {
		return err
	}
	var job, message, operationOverride, approvedRef string
	wait := false
	for index := 0; index < len(args); index++ {
		value := func(target *string) error {
			if index+1 >= len(args) {
				return s.usageExit()
			}
			*target = args[index+1]
			index++
			return nil
		}
		var err error
		switch args[index] {
		case "--job":
			err = value(&job)
		case "--message":
			err = value(&message)
		case "--operation-id":
			err = value(&operationOverride)
		case "--approved-ref":
			err = value(&approvedRef)
		case "--wait":
			wait = true
		default:
			return s.usageExit()
		}
		if err != nil {
			return err
		}
	}
	if !validID(job) || !isFile(message) || !isFile(s.recordPath(job)) {
		return s.usageExit()
	}
	s.dieJob = job
	authorityMessage := message
	operationBriefHash, err := sha256File(message)
	if err != nil {
		return s.die(1, err.Error())
	}
	if err := s.leaseEntryCheck(); err != nil {
		return err
	}
	if err := s.requireOpenDispatchFence(); err != nil {
		return err
	}
	if err := s.requireFreshCensus(); err != nil {
		return err
	}
	s.reportPlanDrift()
	rootID, err := s.rootJobID(job)
	if err != nil {
		s.eprintln(err.Error())
		return s.die(1, "cannot resolve the job chain")
	}
	if err := s.acquireLaunchChainLock(rootID); err != nil {
		return err
	}
	s.cleanupChain = rootID
	s.trap = func(int) error {
		s.cleanupCompositionTemporaries()
		s.cleanupSubjectTemp()
		s.cleanupFollowUpMessage()
		first := s.releaseGoalRevisionLock()
		if err := s.releaseChainLock(s.cleanupChain); err != nil && first == nil {
			first = err
		}
		return first
	}
	rootRecord := s.recordPath(rootID)
	if fieldOr(rootRecord, "chainClosed") == "true" {
		return s.die(1, "job chain is closed")
	}
	latest, err := dispatch.LatestChainRecord(s.jobs, rootID)
	if err != nil {
		_ = s.verbFailure(err)
		return s.die(1, "cannot find the newest chain record")
	}
	status, ok := field(latest, "status")
	if !ok {
		return exitWith(1)
	}
	latestError := fieldOr(latest, "error")
	var round int64
	var child string
	repeatedFollowUp := false
	standingChildRecord := ""
	continuation := ""
	nextRound := func() error {
		value, ok := field(latest, "round")
		number, err := strconv.ParseInt(value, 10, 64)
		if !ok || err != nil {
			return s.die(1, "the chain's newest record carries no round")
		}
		round = number + 1
		child = fmt.Sprintf("%s-r%d", rootID, round)
		if exists(s.recordPath(child)) {
			return s.die(1, "follow-up job id collision: "+child)
		}
		return nil
	}
	switch {
	case status == "completed" || (status == "failed" && latestError == "protocol_error"):
		if err := nextRound(); err != nil {
			return err
		}
	case (status == "pending-setup" || status == "pending" || status == "running") && fieldOr(latest, "dispatchMode") == "follow-up":
		repeatedFollowUp = true
		child = fieldOr(latest, "jobId")
		standingChildRecord = s.recordPath(child)
		roundText := fieldOr(latest, "round")
		if !followUpRoundPattern.MatchString(roundText) {
			roundText = strings.TrimPrefix(child, rootID+"-r")
		}
		if !followUpRoundPattern.MatchString(roundText) || child != rootID+"-r"+roundText {
			return s.die(1, "the active follow-up has no valid round identity")
		}
		round, _ = strconv.ParseInt(roundText, 10, 64)
		parentJob := fieldOr(latest, "parentJob")
		if parentJob == "" || parentJob == "null" {
			if round == 2 {
				parentJob = rootID
			} else {
				parentJob = fmt.Sprintf("%s-r%d", rootID, round-1)
			}
		}
		if !validID(parentJob) || !isFile(s.recordPath(parentJob)) {
			return s.die(1, "the active follow-up has no readable parent record")
		}
		latest = s.recordPath(parentJob)
		// A standing continuation after a cap is repeated as one: the same
		// fresh-context packet with the paragraph the first wrapper kept.
		if fieldOr(standingChildRecord, "continuation") == "after-cap" {
			continuation = "after-cap"
		}
	case status == "timeout" && latestError == "budget-cap" && !strings.HasSuffix(fieldOr(latest, "role"), "critic"):
		// A round cut off at its cap left its work in the chain worktree and
		// wrote no return; an implementer chain continues it in a
		// fresh-context round told so.
		continuationRole := fieldOr(latest, "role")
		continuationWorkspace := fieldOr(latest, "workspaceRoot")
		continuationLaunch := fieldOr(latest, "launchMode")
		if continuationRole != "implementer" {
			return s.die(1, fmt.Sprintf("follow-up refused: the newest round is a %s round cut off at its cap\nonly implementer worktree chains continue; start a fresh %s round (a critique re-runs under its critique cap)", continuationRole, continuationRole))
		}
		if continuationLaunch != "worktree" || continuationWorkspace == "" || continuationWorkspace == "null" {
			return s.die(1, "follow-up refused: the capped round did not run in a job worktree, so nothing is left to continue\nuse a fresh dispatch")
		}
		if !isDir(continuationWorkspace) {
			return s.die(1, fmt.Sprintf("follow-up refused: the chain's worktree %s is gone, so nothing is left to continue\nuse a fresh dispatch", continuationWorkspace))
		}
		continuation = "after-cap"
		if err := nextRound(); err != nil {
			return err
		}
	default:
		refusal := s.examinationRetry(latest)
		if refusal != nil {
			reason := strings.TrimSpace(refusal.Error())
			if reason == "" {
				reason = "not admitted"
			}
			return s.die(1, fmt.Sprintf("follow-up refused: a follow-up does not continue the newest round (%s)\nuse a fresh dispatch; a follow-up continues a completed round, a protocol_error failure, an implementer worktree round cut off at its cap, or a critic round the examination retry admits", reason))
		}
		// A critic round that ended without a return is examined once more
		// in the same chain, under the chain's own round cap.
		if err := nextRound(); err != nil {
			return err
		}
	}
	s.dieChild = child

	rebase := &rebaseState{}
	worktreePath := fieldOr(rootRecord, "workspaceRoot")
	rootLaunchMode := fieldOr(rootRecord, "launchMode")
	if rootLaunchMode != "worktree" && rootLaunchMode != "shared-checkout" {
		rootLaunchMode = s.inferredLaunchMode(worktreePath)
	}
	if !repeatedFollowUp && rootLaunchMode == "worktree" && worktreePath != "" && worktreePath != "null" && isDir(worktreePath) {
		out, _, err := s.l.ports.Git.Run(s.ctx, s.root, "rev-parse", "HEAD")
		if err != nil {
			return s.die(1, "follow-up rebase cannot resolve the trunk commit")
		}
		trunk := strings.TrimSpace(string(out))
		plan, err := dispatch.PlanFollowUpRebase(s.root, rootID, worktreePath, trunk, authorityMessage)
		if err != nil {
			_ = s.verbFailure(err)
			return s.die(1, "follow-up rebase planning refused because the chain boundary or Git history is unreadable")
		}
		if plan.Rebase {
			if err := s.rebaseFollowUpWorktree(worktreePath, trunk, fmt.Sprintf("%s-%d-rebase", rootID, round), rebase); err != nil {
				return s.die(1, "follow-up rebase refused: "+err.Error())
			}
			if message, err = s.prependRebaseParagraph(message, rebase); err != nil {
				return err
			}
		} else {
			rebase.conflictedPaths = append([]string(nil), plan.UnmergedPaths...)
			if len(rebase.conflictedPaths) > 0 {
				rebase.from = ""
				rebase.to = plan.RebasedFrom
				if message, err = s.prependRebaseParagraph(message, rebase); err != nil {
					return err
				}
			}
		}
		if !plan.Rebase && plan.BehindCount > 0 {
			if plan.BehindCount == 1 {
				s.eprintln("WORKTREE-BEHIND: the chain worktree is behind 1 commit, none on this chain's files")
			} else {
				s.eprintf("WORKTREE-BEHIND: the chain worktree is behind %d commits, none on this chain's files\n", plan.BehindCount)
			}
		}
	} else if repeatedFollowUp {
		if err := readRebaseRecordFields(standingChildRecord, rebase); err != nil {
			return s.die(1, "the active follow-up carries unreadable rebase fields")
		}
		if rebase.to != "" {
			if message, err = s.prependRebaseParagraph(message, rebase); err != nil {
				return err
			}
		}
	}
	operationParent := strings.TrimSuffix(filepath.Base(latest), ".json")
	session := fieldOr(latest, "sessionId")
	if session == "" || session == "null" {
		return s.die(1, "follow-up has no resumable session id; use the fresh-context embed fallback")
	}
	// The recorded parent session is part of every follow-up's identity.
	resumedForClaim := session
	role, _ := field(latest, "role")
	runtime, _ := field(latest, "runtime")
	requestedModel, _ := field(latest, "requestedModel")
	model, aliased, err := config.ResolveModelAlias(filepath.Join(s.root, "metasystem.conf"), runtime, requestedModel)
	if err != nil {
		s.eprintln(err.Error())
		return exitWith(1)
	}
	aliasedFrom := ""
	if aliased {
		aliasedFrom = requestedModel
	}
	subj := &subject{role: role, reviews: nullToEmpty(fieldOr(latest, "reviews")), destructiveReach: fieldOr(latest, "destructiveReach")}
	switch subj.destructiveReach {
	case "MECHANICAL", "DESIGN-BEARING", "DESTRUCTIVE-REACH":
	default:
		return s.die(1, "follow-up chain has no admitted destructiveReach class; start a fresh typed delegate chain")
	}
	if approvedRef == "" {
		approvedRef = nullToEmpty(fieldOr(latest, "approvedRef"))
	}
	subj.goal = nullToEmpty(fieldOr(latest, "goalId"))
	workspace, _ := field(latest, "workspaceRoot")
	{
		value, _ := field(latest, "round")
		number, _ := strconv.ParseInt(value, 10, 64)
		round = number + 1
		child = fmt.Sprintf("%s-r%d", rootID, round)
	}
	s.dieChild = child
	goalMachine, goalClaimEpoch := "", ""
	if subj.goal != "" {
		binding, err := s.l.ports.Goal.Binding(subj.goal)
		if err != nil {
			_ = s.verbFailure(err)
			return s.die(1, fmt.Sprintf("cannot bind follow-up %s to accepted goal %s stop authority", child, subj.goal))
		}
		subj.goalRevision = binding.Revision
		tier, _ := strconv.ParseUint(fieldOr(latest, "goalTier"), 10, 8)
		subj.goalTier = uint8(tier)
		subj.goalWidth = fieldOr(latest, "gateWidth")
		if subj.goalWidth == "" || subj.goalWidth == "null" {
			subj.goalWidth = "area"
		}
		goalMachine = binding.Machine
		goalClaimEpoch = strconv.FormatInt(binding.Capability.ClaimEpoch, 10)
		if current := s.currentEpoch(); current != "" && current != goalClaimEpoch {
			return s.die(1, staleClaimMessage(subj.goal, subj.goalRevision, goalClaimEpoch, current))
		}
		if err := s.requireGoalTierLadder(subj); err != nil {
			return err
		}
		if err := s.acquireGoalRevisionLock(subj.goal, subj.goalRevision); err != nil {
			return err
		}
	}
	operationID := operationOverride
	if operationID == "" {
		derived, err := dispatch.DefaultOperationID(subj.goal, subj.goalRevision, dispatch.DispatchModeFollowUp, role, operationBriefHash, operationParent)
		if err != nil {
			s.eprintln(err.Error())
			return s.die(1, "could not derive the follow-up operation identity")
		}
		operationID = derived
	}
	if !validID(operationID) {
		return s.die(2, "invalid follow-up operation id: "+operationID)
	}
	if repeatedFollowUp && fieldOr(standingChildRecord, "operationId") != operationID {
		s.recordOutcome("REFUSED-OPID-MISMATCH", "refused", "the active follow-up is bound to another v2 operation identity", child)
		return exitWith(1)
	}
	reservationClaimEpoch := goalClaimEpoch
	if reservationClaimEpoch == "" {
		reservationClaimEpoch = s.currentEpoch()
	}
	launchMode := fieldOr(latest, "launchMode")
	if launchMode != "worktree" && launchMode != "shared-checkout" {
		launchMode = s.inferredLaunchMode(workspace)
	}
	// Mission provenance refuses before the child reservation exists.
	missionID := nullToEmpty(fieldOr(latest, "mission"))
	missionTurn := ""
	if missionID != "" {
		resolved, _, turn, err := s.resolveMission(missionID)
		if err != nil {
			return err
		}
		missionID, missionTurn = resolved, turn
		parent, err := dispatch.ReadRecordObject(latest)
		if err == nil {
			err = dispatch.VerifyChainIncarnation(s.root, missionID, parent)
		}
		if err != nil {
			message := err.Error()
			var op *dispatch.OpError
			if asOpError(err, &op) {
				message = op.Error()
			}
			if message == "" {
				message = "mission chain incarnation check failed"
			}
			return s.die(1, message)
		}
		if missionTurn == "" {
			return s.die(2, "mission follow-up needs a mission runner turn, and none is set here\nfollow up from inside the mission's host turn")
		}
	}
	s.mission = missionID
	if role == "design-critic" {
		if err := s.syncDesignCriticWorkspace(workspace); err != nil {
			return err
		}
	}
	if err := s.briefAuthority(authorityMessage, workspace, role, subj.reviews); err != nil {
		return s.die(1, "follow-up brief authority admission refused: "+err.Error())
	}
	// The completed critic attempt is folded and the cap is checked while no
	// successor record exists. A repeated wrapper claims a round the winner
	// already folded; the carry appendix is a pure register read, so it is
	// still appended, reconstructing the winner's exact delivered message.
	if isReviewRole(role) {
		if !repeatedFollowUp {
			// The fold's outcome line is captured, not printed: the
			// follow-up's standard output is the child job id alone.
			mark := s.stdout.Len()
			err := s.runHeld(s.inv.ClaimEpoch, func() error {
				return s.callbackCritiqueMutation("critique-register-advance", []string{"--root-job", rootID, "--round-job", operationParent})
			})
			s.stdout.Truncate(mark)
			if err != nil {
				return s.die(1, "could not fold the latest critic attempt into its canonical register")
			}
		}
		previous := s.cleanupMessage
		carried, err := s.mustTemp("", "metasystem-critique-follow")
		if err != nil {
			return err
		}
		s.cleanupMessage = carried
		if err := s.appendCritiqueOpenIDs(message, carried, rootID); err != nil {
			return err
		}
		message = carried
		removeQuietly(previous)
	}
	subjectTemp := ""
	if isReviewRole(role) && !repeatedFollowUp {
		temp, err := s.mustTemp(s.recordLocks, "subject")
		if err != nil {
			return err
		}
		subjectTemp = temp
		s.cleanupSubject = subjectTemp
		present, code, output := s.readSubject(dispatch.ReadSubjectRequest{
			RepoRoot: s.root, Role: role, RootJob: rootID, Workspace: workspace,
		}, subjectTemp)
		if code != 0 {
			return s.die(code, output)
		}
		if present && fileNonEmpty(subjectTemp) {
			if err := s.admitCritiqueRead(role, rootID, round, subjectTemp, ""); err != nil {
				return err
			}
		}
	}
	if !repeatedFollowUp && (role == "implementer" || isReviewRole(role)) {
		stderr := s.stderr
		var captured strings.Builder
		s.stderr = &captured
		mark := s.stdout.Len()
		exhaustErr := s.runHeld(s.inv.ClaimEpoch, func() error {
			return s.callbackCritiqueMutation("critique-exhaustion-advance", []string{"--root-job", rootID, "--role", role, "--message", message, "--successor", child})
		})
		output := strings.TrimRight(captured.String()+s.stdout.String()[mark:], "\n")
		s.stdout.Truncate(mark)
		s.stderr = stderr
		if code := ExitCode(exhaustErr); code != 0 {
			return s.die(code, output)
		}
	}
	if err := os.MkdirAll(s.recordLocks, 0o755); err != nil {
		return exitWith(1)
	}
	s.cleanupJob, s.cleanupChain, s.cleanupAuthorization = child, rootID, ""
	s.trap = s.launchTrap(true)
	permissionJSON, err := s.mustTemp(s.recordLocks, "follow-permissions")
	if err != nil {
		return err
	}
	requested, ok := field(latest, "permissions.requested")
	if !ok {
		return exitWith(1)
	}
	if err := os.WriteFile(permissionJSON, []byte(requested+"\n"), 0o600); err != nil {
		return exitWith(1)
	}
	permissionDigest, err := sha256File(permissionJSON)
	if err != nil {
		return exitWith(1)
	}
	toolPolicy, ok := field(permissionJSON, "tools")
	if !ok {
		return exitWith(1)
	}
	snapshotJSON, err := s.mustTemp(s.recordLocks, "follow-snapshot")
	if err != nil {
		return err
	}
	if err := s.selectSnapshot(runtime, role, permissionJSON, snapshotJSON); err != nil {
		return err
	}
	snapshot, err := readSnapshotFields(snapshotJSON)
	if err != nil {
		return exitWith(1)
	}
	payload := filepath.Join(s.agents, rootID)
	roundDir := filepath.Join(payload, "rounds", strconv.FormatInt(round, 10))
	delivery := message
	if role == "implementer" && subj.goal != "" {
		composed, err := s.appendTestingRequirement(delivery, subj.goal, subj.goalWidth, "follow-gate")
		if err != nil {
			if _, isExit := err.(*Exit); isExit {
				return err
			}
			return s.die(1, "could not compose the follow-up implementer's shared testing requirement")
		}
		delivery = composed
	}
	withPathForm, err := s.mustTemp(s.recordLocks, "follow-return-path-form")
	if err != nil {
		return err
	}
	if err := appendReturnPathForm(delivery, withPathForm); err != nil {
		return s.die(1, err.Error())
	}
	delivery = withPathForm
	// A continuation after a cap never resumes the killed session: it
	// composes fresh context with the prior brief and the prior-worktree
	// paragraph; the prior return rides only when the parent wrote one.
	resumeMode, adapterVerb := "resumed", "follow-up"
	var continuations []dispatch.CompositionContinuation
	freshContextTemp := ""
	continuationTemp := ""
	if snapshot.resume != "true" || continuation != "" {
		resumeMode, adapterVerb = "fresh-context", "dispatch"
		parentRound, _ := field(latest, "round")
		continuations = append(continuations, dispatch.CompositionContinuation{Slot: "prior-brief", Path: filepath.Join(payload, "brief.md")})
		if continuation == "" && fileNonEmpty(filepath.Join(payload, "rounds", parentRound, "return.json")) {
			continuations = append(continuations, dispatch.CompositionContinuation{Slot: "prior-return", Path: filepath.Join(payload, "rounds", parentRound, "return.json")})
		}
		if continuation != "" {
			standing := filepath.Join(roundDir, "prior-worktree.md")
			if fileNonEmpty(standing) {
				// The standing wrapper's paragraph: a repeat composes the
				// same bytes.
				continuations = append(continuations, dispatch.CompositionContinuation{Slot: "prior-worktree", Path: standing})
			} else {
				temp, err := s.mustTemp(s.recordLocks, "follow-prior-worktree")
				if err != nil {
					return err
				}
				continuationTemp = temp
				s.cleanupContinuation = temp
				text, err := dispatch.CapContinuationText(latest, workspace)
				if err == nil {
					err = os.WriteFile(temp, []byte(text), 0o644)
				}
				if err != nil {
					s.eprintln("job cap-continuation: " + err.Error())
					return s.die(1, "could not compose the continuation's prior-worktree paragraph")
				}
				continuations = append(continuations, dispatch.CompositionContinuation{Slot: "prior-worktree", Path: temp})
			}
		}
	}
	// The cap is authorized before the packet is composed; a repeated
	// operation reads the reservation's cap and composes the same bytes.
	productRoots := []string{workspace}
	if err := s.acquireCapAuthorityLock(); err != nil {
		return err
	}
	capResolution, err := s.mustTemp(s.recordLocks, "follow-cap-resolution")
	if err != nil {
		return err
	}
	modelKey := config.CanonicalModel(model)
	if modelKey == "" {
		return s.die(1, "requested model has no canonical cap-key form")
	}
	capTruncated := ""
	if repeatedFollowUp && missionID != "" {
		if err := s.writeRepeatedCapResolution(fieldOr(s.recordPath(child), "capMin"), capResolution); err != nil {
			return err
		}
		capTruncated = fieldOr(s.recordPath(child), "capResolution.truncatedBy")
	} else {
		s.cleanupAuthorization = child
		if err := s.authorizeJobCap(child, role, runtime, modelKey, aliasedFrom, missionID, "", "follow-up", capResolution); err != nil {
			return err
		}
		capTruncated = fieldOr(capResolution, "source.truncatedBy")
	}
	capMin, ok := field(capResolution, "capMin")
	if !ok {
		return exitWith(1)
	}
	packet, err := s.composePacket(composeRequest{
		role: role, brief: delivery, job: child, runtime: runtime, model: model, toolPolicy: toolPolicy,
		round: round, mission: missionID, destructiveReach: subj.destructiveReach, goalTier: subj.goalTier,
		roundDir: roundDir, capMin: capMin, capTruncated: capTruncated != "" && capTruncated != "null",
		continuations: continuations, prefix: "follow-",
	})
	if err != nil {
		return err
	}
	inputBytes, err := s.enforceInlineInputLimit(packet.prompt, "message")
	if err != nil {
		return err
	}
	inputHash, err := sha256File(packet.prompt)
	if err != nil {
		return exitWith(1)
	}
	outputStream, err := s.l.ports.Adapter.OutputStream(s.ctx, runtime, roundDir)
	if err != nil {
		return s.die(1, runtime+" adapter could not resolve its child output stream")
	}
	claim := claimRequest{
		opID: child, operationID: operationID, session: runtime + ":" + session, dispatchMode: "follow-up",
		resumedSession: resumedForClaim, runtime: runtime, model: model, role: role, aliasSource: aliasedFrom,
		reviews: subj.reviews, launchMode: launchMode, permissionDigest: permissionDigest, productRoots: productRoots,
		capMin: capMin, inputHash: inputHash, goalID: subj.goal, goalRevision: subj.goalRevision,
		goalTier: subj.goalTier, gateWidth: subj.goalWidth, destructiveReach: subj.destructiveReach, adapterVerb: adapterVerb,
	}
	replay, err := s.claimPreflight(claim)
	if err != nil {
		return err
	}
	capNumber, _ := strconv.ParseInt(capMin, 10, 64)
	if !replay {
		if err := s.requireGoalRevisionAdmission(subj, capNumber, "follow-up"); err != nil {
			return err
		}
		if err := s.requireGoalAdmission(); err != nil {
			return err
		}
		if err := s.requireSliceAdmission(capNumber, approvedRef, subj.goal, subj.goalRevision); err != nil {
			return err
		}
	}
	claim.mainID, claim.claimEpoch, claim.machineID, claim.approvedRef = s.inv.MainID, reservationClaimEpoch, goalMachine, approvedRef
	launchCapability, creationClaim, err := s.claimReservation(child, child, claim)
	if err != nil {
		removeQuietly(freshContextTemp)
		return err
	}

	if err := os.MkdirAll(roundDir, 0o755); err != nil {
		return s.die(1, err.Error())
	}
	s.publishSubject(subjectTemp, roundDir)
	if continuationTemp != "" {
		_ = os.Rename(continuationTemp, filepath.Join(roundDir, "prior-worktree.md"))
		s.cleanupContinuation = ""
	}
	if err := s.publishComposition(packet, roundDir); err != nil {
		return err
	}
	recordJSON, err := s.mustTemp(s.recordLocks, "follow-record")
	if err != nil {
		return err
	}
	params := dispatch.BuildFollowRecordParams{
		LookupEnv: s.configLookup(), Output: recordJSON, Parent: latest, Job: child, OperationID: operationID, Round: round,
		ParentJob: operationParent, Model: model, AliasedFrom: aliasedFrom, Snapshot: snapshot.path,
		Fallbacks: snapshot.fallbacks, ResumeMode: resumeMode, Continuation: continuation,
		InputBytes: inputBytes, InputHash: inputHash, MissionTurn: missionTurn, MainID: s.inv.MainID,
		ClaimEpoch: reservationClaimEpoch, CapResolution: capResolution, Root: s.root,
		GoalRevision: subj.goalRevision, GoalTier: subj.goalTier, GateWidth: subj.goalWidth, ApprovedRef: approvedRef,
		DestructiveReach: dispatch.HazardClass(subj.destructiveReach), Composition: filepath.Join(roundDir, "composition.json"),
		LaunchMode: dispatch.LaunchMode(launchMode), OutputStream: outputStream,
	}
	if rebase.to != "" {
		params.RebasedTo, params.RebasedFrom = rebase.to, rebase.from
		if len(rebase.conflictedPaths) > 0 {
			params.ConflictedPaths = rebase.conflictedPaths
		}
	}
	if err := setSnapshotBooleans(&params.Signal, &params.HandshakeBudget, snapshot); err != nil {
		s.eprintln(err.Error())
		return exitWith(2)
	}
	if err := s.verbFailure(dispatch.BuildFollowRecord(params)); err != nil {
		return err
	}
	removeQuietly(capResolution)
	s.cleanupFollowUpMessage()
	return s.finalizeAndLaunch(child, rootID, recordJSON, runtime, adapterVerb, snapshot.handshakeBudget, wait, launchCapability, creationClaim)
}

// examinationRetry is `job examination-retry`: nil admits one fresh
// examination round of a critic round that ended without a return.
func (s *session) examinationRetry(latest string) error {
	record, err := dispatch.ReadRecordObject(latest)
	if err != nil {
		return err
	}
	return dispatch.ExaminationRetryAdmissibleWith(s.root, record, dispatch.CustodyDeathDependencies{MatchesTag: dispatchproc.PositionedJobTagAt(s.root)})
}

// inferredLaunchMode is the launch mode of a record that predates the
// field: a workspace under the job worktrees is a worktree launch.
func (s *session) inferredLaunchMode(workspace string) string {
	if strings.HasPrefix(workspace+"/", s.worktrees+"/") {
		return "worktree"
	}
	return "shared-checkout"
}

// syncDesignCriticWorkspace synchronizes a design critic's worktree of this
// repository to its HEAD (the design under review moved); a workspace that
// is its own repository reviews its own head and is never merged into.
func (s *session) syncDesignCriticWorkspace(workspace string) error {
	git := s.l.ports.Git
	commonDir := func(dir string) string {
		out, _, err := git.Run(s.ctx, dir, "rev-parse", "--git-common-dir")
		if err != nil {
			return ""
		}
		value := strings.TrimSpace(string(out))
		if !filepath.IsAbs(value) {
			value = filepath.Join(dir, value)
		}
		resolved, err := physicalDir(value)
		if err != nil {
			return ""
		}
		return resolved
	}
	workspaceGit, harnessGit := commonDir(workspace), commonDir(s.repoScope)
	if workspaceGit != "" && workspaceGit == harnessGit {
		out, _, err := git.Run(s.ctx, s.repoScope, "rev-parse", "HEAD")
		if err != nil {
			return s.die(1, "design-critic follow-up cannot resolve the current commit")
		}
		reviewed := strings.TrimSpace(string(out))
		if resolved, _ := physicalDir(workspace); resolved != s.repoScope {
			status, _, err := git.Run(s.ctx, workspace, "status", "--porcelain")
			if err != nil || strings.TrimSpace(string(status)) != "" {
				return s.die(1, "design-critic follow-up cannot synchronize a dirty critic worktree")
			}
			if _, _, err := git.Run(s.ctx, workspace, "merge", "--ff-only", "-q", reviewed); err != nil {
				return s.die(1, "design-critic follow-up cannot fast-forward its worktree to current commit "+reviewed)
			}
		}
		return nil
	}
	if _, _, err := git.Run(s.ctx, workspace, "rev-parse", "HEAD"); err != nil {
		return s.die(1, "design-critic follow-up cannot resolve the workspace commit")
	}
	return nil
}

// appendCritiqueOpenIDs is append_critique_open_ids: the chain's canonical
// open finding identifiers carried into the next critic round.
func (s *session) appendCritiqueOpenIDs(source, output, criticRoot string) error {
	ids, err := dispatch.CritiqueOpenFindingIDs(s.root, criticRoot)
	if err != nil {
		_ = s.verbFailure(err)
		return s.die(1, "could not read the critic chain's canonical open finding identifiers")
	}
	content, err := os.ReadFile(source)
	if err != nil {
		return s.die(1, err.Error())
	}
	var builder strings.Builder
	builder.Write(content)
	builder.WriteString("\n\n# Canonical critique register carry\n\nOpen finding identifiers:\n")
	written := false
	for _, id := range ids {
		if id != "" {
			builder.WriteString("- " + id + "\n")
			written = true
		}
	}
	if !written {
		builder.WriteString("- none\n")
	}
	return os.WriteFile(output, []byte(builder.String()), 0o600)
}

// rebaseState is the follow-up rebase's provenance.
type rebaseState struct {
	from, to        string
	conflictedPaths []string
}

// readRebaseRecordFields is read_follow_up_rebase_record_fields.
func readRebaseRecordFields(record string, rebase *rebaseState) error {
	object, err := dispatch.ReadRecordObject(record)
	if err != nil {
		return err
	}
	text := func(key string) (string, error) {
		value, present := object[key]
		if !present {
			return "", fmt.Errorf("record has no %s", key)
		}
		if value == nil {
			return "", nil
		}
		typed, ok := value.(string)
		if !ok {
			return "", fmt.Errorf("record %s is not a string", key)
		}
		return typed, nil
	}
	if rebase.from, err = text("rebasedFrom"); err != nil {
		return err
	}
	if rebase.to, err = text("rebasedTo"); err != nil {
		return err
	}
	raw, present := object["conflictedPaths"]
	if !present {
		return errors.New("record has no conflictedPaths")
	}
	encoded, err := json.Marshal(raw)
	if err != nil {
		return err
	}
	var paths []string
	if err := json.Unmarshal(encoded, &paths); err != nil {
		return err
	}
	rebase.conflictedPaths = paths
	return nil
}

// prependRebaseParagraph is prepend_follow_up_rebase_paragraph into a fresh
// temporary that replaces the delivered message.
func (s *session) prependRebaseParagraph(message string, rebase *rebaseState) (string, error) {
	output, err := s.mustTemp("", "metasystem-rebase-follow")
	if err != nil {
		return "", err
	}
	var builder strings.Builder
	if rebase.from != "" {
		fmt.Fprintf(&builder, "The dispatcher fast-forwarded this chain worktree from commit %s to trunk commit %s.", rebase.from, rebase.to)
	} else {
		fmt.Fprintf(&builder, "An earlier dispatcher fast-forwarded this chain worktree to trunk commit %s.", rebase.to)
	}
	if len(rebase.conflictedPaths) > 0 {
		builder.WriteString(" Conflicted paths: ")
		for index, path := range rebase.conflictedPaths {
			if index > 0 {
				builder.WriteString(", ")
			}
			builder.WriteString("`" + path + "`")
		}
		builder.WriteString("; resolve first, keeping both sides' behaviour, then stage each resolved path with `git add`.\n\n")
	} else {
		builder.WriteString(" No conflicted paths remain.\n\n")
	}
	content, err := os.ReadFile(message)
	if err != nil {
		return "", s.die(1, err.Error())
	}
	builder.Write(content)
	if err := os.WriteFile(output, []byte(builder.String()), 0o600); err != nil {
		return "", s.die(1, err.Error())
	}
	s.cleanupMessage = output
	return output, nil
}

// rebaseFollowUpWorktree is rebase_follow_up_worktree: stash the round's
// changes under a reserved tag, fast-forward the worktree to the pinned
// trunk commit, reapply the stash (conflicts stay for the delegate), and
// restore the original worktree on any failure.
func (s *session) rebaseFollowUpWorktree(worktree, trunk, tag string, rebase *rebaseState) error {
	git := s.l.ports.Git
	run := func(args ...string) (string, error) {
		out, stderr, err := git.Run(s.ctx, worktree, args...)
		return strings.TrimSpace(string(out) + string(stderr)), err
	}
	head, _, err := git.Run(s.ctx, worktree, "rev-parse", "HEAD")
	if err != nil {
		return errors.New("cannot resolve the worktree's original commit")
	}
	rebase.from = strings.TrimSpace(string(head))
	rebase.to = trunk
	rebase.conflictedPaths = nil
	if _, _, found := s.stashEntry(worktree, tag, ""); found {
		return fmt.Errorf("the shared stash already contains the reserved tag %s", tag)
	}
	dirty, _, err := git.Run(s.ctx, worktree, "status", "--porcelain", "--untracked-files=all")
	if err != nil {
		return errors.New("cannot inspect the worktree before stashing its changes")
	}
	stashOutput, stashErr := run("stash", "push", "-u", "-m", tag)
	stashHash := ""
	stashFound := false
	if len(dirty) > 0 {
		_, stashHash, stashFound = s.stashEntry(worktree, tag, "")
	}
	restore := func() string {
		if failure := s.restoreRebaseWorktree(worktree, rebase.from, stashHash, tag); failure != "" {
			return "; " + failure
		}
		return ""
	}
	if stashErr != nil || (len(dirty) > 0 && !stashFound) {
		detail := ""
		if stashHash != "" {
			detail = restore()
		}
		stashOutput = strings.ReplaceAll(stashOutput, "\n", " ")
		if stashOutput != "" {
			stashOutput = ": " + stashOutput
		}
		return fmt.Errorf("stashing the round's changes failed%s%s", stashOutput, detail)
	}
	if mergeOutput, err := run("merge", "--ff-only", "-q", trunk); err != nil {
		detail := restore()
		mergeOutput = strings.ReplaceAll(mergeOutput, "\n", " ")
		if mergeOutput != "" {
			mergeOutput = ": " + mergeOutput
		}
		return fmt.Errorf("the worktree cannot fast-forward to trunk commit %s%s%s", trunk, mergeOutput, detail)
	}
	if stashHash != "" {
		applyOutput, applyErr := run("stash", "apply", "-q", stashHash)
		if conflicted, _, err := git.Run(s.ctx, worktree, "diff", "--name-only", "-z", "--diff-filter=U"); err == nil {
			for _, path := range strings.Split(string(conflicted), "\x00") {
				if path != "" {
					rebase.conflictedPaths = append(rebase.conflictedPaths, path)
				}
			}
		}
		untracked, colliding, inspectErr := s.inspectRebaseUntracked(worktree, stashHash)
		if inspectErr != nil {
			return fmt.Errorf("the stash's untracked tree could not be verified after reapplication%s", restore())
		}
		restoreFailed := strings.Contains(applyOutput, "could not restore untracked files")
		if restoreFailed && len(colliding) == 0 {
			colliding = untracked
		}
		if len(colliding) > 0 || restoreFailed {
			names := strings.Join(colliding, ", ")
			if names != "" {
				names = ": " + names
			}
			return fmt.Errorf("reapplying the round's changes collided with untracked paths%s%s", names, restore())
		}
		if applyErr != nil && len(rebase.conflictedPaths) == 0 {
			detail := restore()
			applyOutput = strings.ReplaceAll(applyOutput, "\n", " ")
			if applyOutput != "" {
				applyOutput = ": " + applyOutput
			}
			return fmt.Errorf("reapplying the round's changes at trunk commit %s failed without conflict markers%s%s", trunk, applyOutput, detail)
		}
	}
	if !s.dropRebaseStash(worktree, tag, stashHash) {
		return fmt.Errorf("the rebased changes were restored, but the tagged stash %s could not be removed", tag)
	}
	return nil
}

// stashEntry finds the stash whose subject ends with the tag (and, when
// given, whose hash is expected): its selector and hash.
func (s *session) stashEntry(worktree, tag, expected string) (string, string, bool) {
	out, _, err := s.l.ports.Git.Run(s.ctx, worktree, "stash", "list", "--format=%gd%x09%H%x09%gs")
	if err != nil {
		return "", "", false
	}
	for _, line := range strings.Split(string(out), "\n") {
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) != 3 || !strings.HasSuffix(parts[2], ": "+tag) {
			continue
		}
		if expected != "" && parts[1] != expected {
			continue
		}
		return parts[0], parts[1], true
	}
	return "", "", false
}

// dropRebaseStash is drop_follow_up_rebase_stash: true once no stash with
// the tag and hash remains.
func (s *session) dropRebaseStash(worktree, tag, expected string) bool {
	if expected == "" {
		return true
	}
	selector, _, found := s.stashEntry(worktree, tag, expected)
	if !found {
		return true
	}
	_, _, _ = s.l.ports.Git.Run(s.ctx, worktree, "stash", "drop", "-q", selector)
	_, _, found = s.stashEntry(worktree, tag, expected)
	return !found
}

// restoreRebaseWorktree is restore_follow_up_rebase_worktree: the failure
// text, empty when the original worktree came back and the stash is gone.
func (s *session) restoreRebaseWorktree(worktree, original, stashHash, tag string) string {
	git := s.l.ports.Git
	identity := "tag " + tag
	if stashHash != "" {
		identity = fmt.Sprintf("stash hash %s with tag %s", stashHash, tag)
	}
	var failed []string
	if _, _, err := git.Run(s.ctx, worktree, "reset", "--hard", "-q", original); err != nil {
		failed = append(failed, "git reset --hard to "+original)
	}
	if _, _, err := git.Run(s.ctx, worktree, "clean", "-fdq"); err != nil {
		failed = append(failed, "git clean -fd")
	}
	if stashHash != "" {
		if _, _, err := git.Run(s.ctx, worktree, "stash", "apply", "-q", stashHash); err != nil {
			failed = append(failed, "git stash apply "+stashHash)
		}
	}
	if len(failed) > 0 {
		return "restoring the original worktree failed during " + strings.Join(failed, ", ") + "; " + identity + " remains for manual recovery"
	}
	if !s.dropRebaseStash(worktree, tag, stashHash) {
		return "restoring the original worktree succeeded, but removing " + identity + " failed; it remains for manual recovery"
	}
	return ""
}

// inspectRebaseUntracked lists the stash's untracked paths and those whose
// reapplied bytes do not match the stashed blob.
func (s *session) inspectRebaseUntracked(worktree, stashHash string) ([]string, []string, error) {
	git := s.l.ports.Git
	treeOut, _, err := git.Run(s.ctx, worktree, "rev-parse", stashHash+"^3")
	if err != nil {
		return nil, nil, nil
	}
	listing, _, err := git.Run(s.ctx, worktree, "ls-tree", "-rz", "--full-tree", strings.TrimSpace(string(treeOut)))
	if err != nil {
		return nil, nil, err
	}
	var untracked, colliding []string
	for _, entry := range strings.Split(string(listing), "\x00") {
		if entry == "" {
			continue
		}
		metadata, path, found := strings.Cut(entry, "\t")
		if !found {
			continue
		}
		fields := strings.Fields(metadata)
		blob := fields[len(fields)-1]
		untracked = append(untracked, path)
		actual := ""
		if exists(filepath.Join(worktree, path)) {
			if out, _, err := git.Run(s.ctx, worktree, "hash-object", "--no-filters", "--", path); err == nil {
				actual = strings.TrimSpace(string(out))
			}
		}
		if actual == "" || actual != blob {
			colliding = append(colliding, path)
		}
	}
	return untracked, colliding, nil
}
