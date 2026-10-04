package delegation

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/fixtureauth"
)

// composeRequest is one compose-role-packet call.
type composeRequest struct {
	role, brief, job, runtime, model, toolPolicy string
	round                                        int64
	mission, destructiveReach                    string
	goalTier                                     uint8
	roundDir                                     string
	capMin                                       string
	capTruncated                                 bool
	sources                                      []string
	continuations                                []dispatch.CompositionContinuation
	// prefix names the temporaries ("" for a dispatch, "follow-" for a
	// follow-up), as the retired shell's mktemp templates did.
	prefix string
}

// composedPacket is the composition's three temporaries.
type composedPacket struct {
	prompt, composition, stage string
}

// compositionRefusal relays a typed composition refusal (exit 9) or an
// owner failure (exit 1), as compose-role-packet did.
func (s *session) compositionRefusal(err error) error {
	var refusal *dispatch.CompositionRefusal
	if errors.As(err, &refusal) {
		output := encodeJSON(map[string]any{"outcome": refusal.Code, "headline": "refused", "source": refusal.Source, "detail": refusal.Detail})
		s.recordOutcomeRaw(output)
		s.println(output)
		return exitWith(9)
	}
	s.eprintln(err.Error())
	return exitWith(1)
}

// composePacket composes the role's closed packet into fresh temporaries,
// registered for cleanup until they are published.
func (s *session) composePacket(request composeRequest) (composedPacket, error) {
	capMin, err := strconv.ParseInt(request.capMin, 10, 64)
	if err != nil {
		s.eprintln(fmt.Sprintf("invalid value %q for flag -cap-min: parse error", request.capMin))
		return composedPacket{}, exitWith(2)
	}
	var packet composedPacket
	if packet.prompt, err = s.mustTemp(s.recordLocks, request.prefix+"composed-packet"); err != nil {
		return packet, err
	}
	if packet.composition, err = s.mustTemp(s.recordLocks, request.prefix+"composition"); err != nil {
		return packet, err
	}
	stage, stageErr := os.MkdirTemp(s.recordLocks, request.prefix+"composed-staged.*")
	if stageErr != nil {
		s.eprintln("mktemp: " + stageErr.Error())
		return packet, exitWith(1)
	}
	packet.stage = stage
	s.cleanupPrompt, s.cleanupComposition, s.cleanupStage = packet.prompt, packet.composition, packet.stage
	margin := int64(-1)
	if capMin > 0 {
		resolved, marginErr := dispatch.ReturnMarginMinutes(s.settingsPath())
		if marginErr != nil {
			s.eprintln("job compose-role-packet: " + marginErr.Error())
			return packet, exitWith(1)
		}
		margin = resolved
	}
	_, err = dispatch.ComposeRolePacket(dispatch.ComposeRolePacketParams{
		LookupEnv: s.configLookup(), Root: s.root, SettingsFile: s.settingsPath(), Role: request.role, Brief: request.brief, JobID: request.job, Runtime: request.runtime,
		Model: request.model, ToolPolicy: request.toolPolicy, Round: request.round, Mission: request.mission,
		DestructiveReach: dispatch.HazardClass(request.destructiveReach), GoalTier: request.goalTier,
		Output: packet.prompt, CompositionOutput: packet.composition, StageDir: packet.stage,
		ReferenceDir: filepath.Join(request.roundDir, "staged"), ExtraSources: request.sources,
		Continuations: request.continuations, CapMinutes: capMin, ReturnMarginMinutes: margin,
		CapTruncated: request.capTruncated,
	})
	if err != nil {
		return packet, s.compositionRefusal(err)
	}
	if entries, readErr := os.ReadDir(packet.stage); readErr == nil && len(entries) == 0 {
		_ = os.Remove(packet.stage)
		packet.stage = ""
		s.cleanupStage = ""
	}
	return packet, nil
}

// publishComposition moves the composed packet into its round directory.
func (s *session) publishComposition(packet composedPacket, roundDir string) error {
	if packet.stage != "" {
		if err := os.Rename(packet.stage, filepath.Join(roundDir, "staged")); err != nil {
			return s.die(1, err.Error())
		}
		s.cleanupStage = ""
	}
	if err := os.Rename(packet.prompt, filepath.Join(roundDir, "prompt.md")); err != nil {
		return s.die(1, err.Error())
	}
	s.cleanupPrompt = ""
	if err := os.Rename(packet.composition, filepath.Join(roundDir, "composition.json")); err != nil {
		return s.die(1, err.Error())
	}
	s.cleanupComposition = ""
	return nil
}

// claimPreflight compares the retry identity without occupancy or
// reservation; true means the operation is a replay of a matched claim.
func (s *session) claimPreflight(request claimRequest) (bool, error) {
	output, code := s.claimLaunch(request, true)
	if code != 0 {
		if output != "" {
			s.recordOutcomeRaw(output)
			s.println(output)
		}
		return false, exitWith(code)
	}
	return valueFieldOr(output, "outcome") == "PREFLIGHT-MATCHED", nil
}

// claimReservation takes the job's lifecycle lock and reserves the launch
// through the typed claim; it returns the adapter launch capability and the
// process-creation claim of a WON claim.
func (s *session) claimReservation(lifecycleJob, sessionJob string, request claimRequest) (string, string, error) {
	acquired, err := s.acquireLifecycleLockUntil(lifecycleJob, 5)
	if err != nil {
		return "", "", err
	}
	if !acquired {
		s.recordOutcome("LOCK_BUSY", "refused", fmt.Sprintf("rank=job-lifecycle key=%s retry=retry-after-the-named-holder-releases", lifecycleJob), lifecycleJob)
		return "", "", s.die(1, lockBusyMessage("job "+lifecycleJob, "another dispatch", -1))
	}
	s.cleanupLifecycle = lifecycleJob
	preparation, err := s.mustTemp(s.recordLocks, request.prefixed("claim-occupancy"))
	if err != nil {
		return "", "", err
	}
	if err := dispatch.WriteClaimOccupancyPreparation(s.root, request.session, preparation); err != nil {
		removeQuietly(preparation)
		return "", "", s.verbFailure(err)
	}
	request.creatorPID = s.inv.CallerPid
	request.occupancy = preparation
	output, code := s.claimLaunch(request, false)
	removeQuietly(preparation)
	outcome := valueFieldOr(output, "outcome")
	if outcome == "" {
		s.eprintln(output)
		return "", "", exitWith(1)
	}
	if outcome != string(dispatch.ClaimWON) {
		s.recordOutcomeRaw(output)
		if exists(s.recordPath(lifecycleJob)) {
			s.cleanupJob = ""
		}
		s.println(output)
		// The command ends here whatever the claim's exit code: a bound or
		// replayed operation is an answer, not a launch.
		return "", "", &Exit{Code: code}
	}
	if code != 0 {
		return "", "", exitWith(code)
	}
	capability := valueFieldOr(output, "evidence.launchCapability")
	if capability == "" {
		return "", "", s.die(1, "claim-launch won without an adapter launch capability")
	}
	creation := valueFieldOr(output, "evidence.creationClaimPath")
	if creation == "" {
		return "", "", s.die(1, "claim-launch won without a process-creation claim")
	}
	s.cleanupCreationClaim = creation
	s.cleanupAuthorization = ""
	if err := s.releaseCapAuthorityLock(); err != nil {
		return "", "", err
	}
	return capability, creation, nil
}

// prefixed is the mktemp template of a follow-up claim's temporaries.
func (request claimRequest) prefixed(name string) string {
	if request.dispatchMode == "follow-up" {
		return "follow-" + name
	}
	return name
}

// finalizeAndLaunch is finalize_and_launch, the authorize-and-launch tail
// shared by dispatch and follow-up: record setup, the adapter launch under
// the ranked locks, the process-creation fence handshake, the session
// handshake wait, and the optional wait to terminal.
func (s *session) finalizeAndLaunch(job, chain, recordJSON, runtime, adapterVerb, budget string, wait bool, launchCapability, creationClaim string) error {
	epoch := s.inv.ClaimEpoch
	if err := s.runHeld(epoch, func() error { return s.internalRecordSetup(job, recordJSON) }); err != nil {
		return err
	}
	if err := s.releaseCapAuthorityLock(); err != nil {
		return err
	}
	// Launch returns only after the adapter has published the exact process
	// identity. The chain and goal locks cover reservation, spawn, and
	// identity publication as one ranked interval; the creation claim stays
	// visible until the post-publication fence read and any required
	// cancellation finish.
	tag, ok := field(s.recordPath(job), "instanceTag")
	if !ok || tag == "" || tag == "null" {
		return s.die(1, fmt.Sprintf("job %s record carries no reservation instance tag", job))
	}
	if pause := s.env.FixturePauseBeforeLaunch; pause != "" {
		if err := s.fixturePauseBeforeLaunch(pause); err != nil {
			return err
		}
	}
	launchErr := s.runHeld(epoch, func() error {
		return s.internalLaunch(runtime, adapterVerb, job, tag, launchCapability)
	})
	s.releaseExitLifecycle()
	if err := s.releaseGoalRevisionLock(); err != nil {
		return err
	}
	if err := s.releaseChainLock(chain); err != nil {
		return err
	}
	s.cleanupChain = ""
	s.trap = func(int) error { return s.guardRelease() }
	fence, fenceErr := dispatch.FenceAfterLaunch(s.root, job)
	if fenceErr != nil {
		s.eprintln(fenceErr.Error())
	}
	if fenceErr == nil && fence.Outcome == dispatch.LaunchFenceRefusedStopped {
		s.cancelRefusalClass = "stopped"
		_ = s.internalCancel(job)
		s.cancelRefusalClass = ""
		_ = s.closeCreationClaim(creationClaim)
		s.recordOutcomeRaw(encodeJSON(fence))
		return exitWith(1)
	}
	if fenceErr != nil || fence.Outcome != dispatch.LaunchFenceOpen {
		_ = s.closeCreationClaim(creationClaim)
		return s.die(1, fmt.Sprintf("job %s could not complete its process-creation fence handshake", job))
	}
	if err := s.closeCreationClaim(creationClaim); err != nil {
		return s.die(1, fmt.Sprintf("job %s could not close its process-creation claim", job))
	}
	if launchErr != nil {
		s.recordOutcome("LAUNCH-FAILED", "refused", "the adapter launch failed after reservation", job)
		patch, err := s.mustTemp(s.recordLocks, "launch-failed")
		if err != nil {
			return err
		}
		if err := writePatch(patch, `{"error":"launch_failed"}`); err == nil {
			_ = s.runHeld(epoch, func() error { return s.internalRecordCAS(job, "pending", "failed", patch) })
		}
		removeQuietly(patch)
		return exitWith(3)
	}
	if !s.awaitHandshake(job, budget, epoch) {
		s.recordOutcome("HANDSHAKE-FAILED", "refused", "the runtime did not establish its session within the recorded deadline", job)
		return exitWith(3)
	}
	if wait {
		return exitWith(s.waitForJob(job))
	}
	s.println(job)
	// The exact waiter command: the agent never invents a polling loop.
	s.eprintf("wait for it with: metasystem work wait j2:%s\n", job)
	return nil
}

// fixturePauseBeforeLaunch is `job fixture-pause-before-launch`.
func (s *session) fixturePauseBeforeLaunch(raw string) error {
	if !fixtureauth.FixtureModeRoot(s.root) {
		s.eprintln("job fixture-pause-before-launch is available only in a fixture-mode root")
		return exitWith(1)
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil || value <= 0 {
		s.eprintln("job fixture-pause-before-launch: --seconds must be a positive decimal")
		return exitWith(2)
	}
	s.l.ports.Clock.Sleep(time.Duration(value * float64(time.Second)))
	return nil
}

// launchAdapter is launch_adapter: start the adapter in its own session,
// publish its proven identity through the ownership compare-and-swap, then
// open its start gate.
func (s *session) launchAdapter(runtime, verb, job, tag, capability string) error {
	poll, err := s.pollInterval(20)
	if err != nil {
		return err
	}
	gate := filepath.Join(s.heartbeats, job+".start")
	if err := os.MkdirAll(s.heartbeats, 0o755); err != nil {
		return exitWith(1)
	}
	// No spawn for a record that already left pending (a cancel can land
	// between setup and launch); the authoritative close is the ownership
	// compare-and-swap below, but not starting at all is cheaper.
	if status, _ := field(s.recordPath(job), "status"); status != "pending" {
		return exitWith(1)
	}
	// The launched adapter does not join the checkout execution guard. The
	// retired script meant to wrap it as a guard member, but it launched
	// from its lease-held __launch child, where the guard was never held,
	// so no delegate ever held the guard past its dispatcher; a running
	// delegate must not queue the next dispatch or validation behind it
	// (the concurrent-read fixtures depend on it). The port's guard member
	// wrapper stays available for a caller that wants membership.
	request := AdapterLaunch{Runtime: runtime, Verb: verb, Job: job, StartGate: gate, InstanceTag: tag, LaunchCapability: capability}
	pid, err := s.l.ports.Adapter.Launch(s.ctx, request)
	if err != nil {
		s.eprintln(err.Error())
		return exitWith(1)
	}
	capSeconds, err := s.fixtureWaitCap(5)
	if err != nil {
		return err
	}
	clock := s.l.ports.Clock
	started := clock.Now()
	deadline := started.Add(time.Duration(capSeconds) * time.Second)
	patch, err := s.tempFile(s.recordLocks, "launch")
	if err != nil {
		return exitWith(1)
	}
	// The handshake deadline is stamped at launch, because that is when the
	// dispatcher starts waiting.
	budget := fieldOr(s.recordPath(job), "sessionEstablishedTimeoutSec")
	provenAt := s.nowISO()
	var handshakeDeadline int64
	if positiveInteger(budget) {
		seconds, _ := strconv.ParseInt(budget, 10, 64)
		handshakeDeadline = s.nowUnix() + seconds
	}
	processes, err := s.l.ports.Process.ClaimProcesses()
	if err != nil {
		removeQuietly(patch)
		return exitWith(1)
	}
	for dispatch.BuildOwnershipPatch(patch, pid, pid, tag, provenAt, handshakeDeadline, processes.Reader) != nil {
		if !clock.Now().Before(deadline) {
			elapsed := int64(clock.Now().Sub(started) / time.Second)
			s.eprintf("adapter start identity ceiling reached for %s (elapsed: %ds; scaled cap: %ds)\n", job, elapsed, capSeconds)
			removeQuietly(patch)
			return exitWith(1)
		}
		clock.Sleep(poll)
	}
	// A lost ownership compare means the record moved on under this launch
	// (a cancel concluded it mid-launch). The retired shell meant to
	// re-prove the launch-captured identity and wind the group down here,
	// but its identity probe never existed in the engine and always failed,
	// so the started adapter was left to its own start-gate timeout; that
	// observed behavior is kept.
	if _, err := s.l.ports.Records.CAS(job, "pending", "pending", patch); err != nil {
		removeQuietly(patch)
		return exitWith(verbCode(err))
	}
	removeQuietly(patch)
	if err := touch(gate); err != nil {
		return exitWith(1)
	}
	return nil
}

// awaitHandshake is await_handshake: wait on the deadline stamped at launch
// for the session to be recorded, else write the handshake-timeout verdict.
func (s *session) awaitHandshake(job, timeout string, epoch *int64) bool {
	if !positiveInteger(timeout) {
		return false
	}
	budget, _ := strconv.ParseInt(timeout, 10, 64)
	if budget > 60 {
		return false
	}
	poll, err := s.pollInterval(50)
	if err != nil {
		return false
	}
	record := s.recordPath(job)
	var deadline int64
	if stamped := fieldOr(record, "handshakeDeadline"); positiveInteger(stamped) {
		deadline, _ = strconv.ParseInt(stamped, 10, 64)
	} else {
		deadline = s.nowUnix() + budget
	}
	for s.nowUnix() <= deadline {
		if isFile(record) {
			status := fieldOr(record, "status")
			session := fieldOr(record, "sessionId")
			switch status {
			case "running", "completed":
				if isFile(filepath.Join(s.jobs, job+".log")) && session != "null" && session != "" {
					return true
				}
			case "failed", "cancelled", "timeout":
				return false
			}
		}
		s.l.ports.Clock.Sleep(poll)
	}
	_ = s.runHeld(epoch, func() error { return s.internalHandshakeTimeout(job) })
	return false
}

// waitForJob is wait_for_job: block to terminal through the job waiter, then
// reconcile the record (post-wait reap) and map the waiter's verdict to the
// dispatcher's exit codes.
func (s *session) waitForJob(job string) int {
	_ = touch(filepath.Join(s.heartbeats, job+".waiting"))
	outcome := s.l.ports.Host.WaitJob(s.ctx, s.root, job, s.inv.CallerPid)
	if outcome.Code == 4 && outcome.Output != "" {
		s.eprintln(strings.TrimRight(outcome.Output, "\n"))
	}
	switch outcome.Code {
	case 0, 1, 2, 3:
		if err := s.runHeld(s.inv.ClaimEpoch, func() error { return s.internalReapHeld(job, "post-wait") }); err != nil {
			return 3
		}
		return []int{0, 3, 4, 8}[outcome.Code]
	case 4:
		return 5
	default:
		if outcome.Output != "" {
			s.eprintln(strings.TrimRight(outcome.Output, "\n"))
		}
		return outcome.Code
	}
}

// failSetupHusk is fail_setup_husk: a refused dispatch releases the mission
// slot it reserved and fails the pending-setup reservation it left.
func (s *session) failSetupHusk(job string) {
	if job == "" {
		return
	}
	record := s.recordPath(job)
	if !isFile(record) {
		if s.mission != "" {
			s.missionReleaseJob(s.mission, job)
		}
		return
	}
	patch, err := s.tempFile("", "metasystem-husk-fail")
	if err != nil {
		return
	}
	// Classify the refusal for the flight recorder: a refused dispatch
	// that emitted nothing left mktemp sizes as the only diagnostic.
	refusalClass := "setup"
	message := s.lastDie
	switch {
	case strings.Contains(message, "mission fence"):
		refusalClass = "fence"
	case strings.Contains(message, "worktree"):
		refusalClass = "worktree"
	case strings.Contains(message, "permission"):
		refusalClass = "envelope"
	case strings.Contains(message, "snapshot") || strings.Contains(message, "capabilit"):
		refusalClass = "capability"
	}
	if message == "" {
		message = "unknown"
	}
	s.emit("job-refused", map[string]string{
		"jobId": job, "missionId": s.mission, "reasonClass": refusalClass,
		"summary": fmt.Sprintf("dispatch refused (%s): %s", refusalClass, message),
	})
	_ = writePatch(patch, fmt.Sprintf(`{"error":"dispatch-refused","phase":"setup","refusalClass":"%s"}`, refusalClass))
	_, _ = s.l.ports.Records.CAS(job, "pending-setup", "failed", patch)
	removeQuietly(patch)
	// A dispatch that died in setup never started a process: its fence
	// reservation must not keep counting against the mission's budget.
	if s.mission != "" {
		s.missionReleaseJob(s.mission, job)
	}
}

// releaseUnpublishedAuthorization releases a mission fence authorization
// the command never published.
func (s *session) releaseUnpublishedAuthorization(job string) {
	if job == "" || s.mission == "" {
		return
	}
	s.missionReleaseJob(s.mission, job)
}
