package delegation

import (
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// statusJob is status_job: the record's status on stdout and the census
// verdict on stderr.
func (s *session) statusJob(args []string) error {
	if len(args) != 2 || args[0] != "--job" || !validID(args[1]) {
		return s.usageExit()
	}
	job := args[1]
	record := s.recordPath(job)
	if !exists(record) {
		s.eprintln("status: no job record for " + job)
		return exitWith(6)
	}
	status := fieldOr(record, "status")
	switch status {
	case "pending", "running", "completed", "failed", "timeout", "cancelled":
		s.println(status)
		s.eprintln(s.censusVerdictLine())
		return nil
	}
	return exitWith(7)
}

// censusVerdictLine is surface_census_verdict.
func (s *session) censusVerdictLine() string {
	verdict := filepath.Join(s.agents, "supervision", "last-census.json")
	if !isFile(verdict) {
		return "CENSUS verdict=ABSENT"
	}
	value, ok := field(verdict, "verdict")
	if !ok {
		value = "UNREADABLE"
	}
	completedText, ok := field(verdict, "completedAtEpoch")
	if !ok {
		completedText = "0"
	}
	completed, _ := strconv.ParseInt(completedText, 10, 64)
	fingerprint, ok := field(verdict, "fingerprint")
	if !ok {
		fingerprint = "unavailable"
	}
	return fmt.Sprintf("CENSUS verdict=%s age=%ds fingerprint=%s", value, s.nowUnix()-completed, fingerprint)
}

// watchJob is watch_job: a job with no record is knowable now (vanished,
// exit 5); otherwise the job watcher blocks to terminal and its exit code
// rides through.
func (s *session) watchJob(args []string) error {
	job := ""
	for index := 0; index < len(args); {
		if args[index] != "--job" || index+1 >= len(args) {
			return s.usageExit()
		}
		job = args[index+1]
		index += 2
	}
	if job == "" {
		return s.usageExit()
	}
	record := s.recordPath(job)
	if !exists(record) {
		s.eprintf("watch: no job record for %s — it never existed here or was reaped\n", job)
		return exitWith(5)
	}
	watched := fieldOr(record, "workspaceRoot")
	if watched == "" || watched == "null" {
		watched = s.root
	}
	return exitWith(s.l.ports.Host.WatchJob(s.ctx, s.root, job, s.inv.CallerPid, watched))
}

// cancelJob is cancel_job: a record that never published a process is
// concluded by the owned cancel; a launched one through its runtime.
func (s *session) cancelJob(args []string) error {
	if err := s.fenceBrain("cancel"); err != nil {
		return err
	}
	if len(args) != 2 || args[0] != "--job" {
		return s.usageExit()
	}
	job := args[1]
	s.dieJob = job
	if !validID(job) || !isFile(s.recordPath(job)) {
		return s.die(1, "unknown job: "+job)
	}
	if err := s.leaseEntryCheck(); err != nil {
		return err
	}
	record := s.recordPath(job)
	status := fieldOr(record, "status")
	pid := fieldOr(record, "pid")
	if status == "pending-setup" || pid == "" || pid == "null" {
		return s.runHeld(s.inv.ClaimEpoch, func() error {
			if err := s.internalAuthority(AuthorityHolderOnly, job); err != nil {
				return err
			}
			return s.internalCancel(job)
		})
	}
	runtime := fieldOr(record, "runtime")
	return s.runHeld(s.inv.ClaimEpoch, func() error {
		if err := s.l.ports.Adapter.Cancel(s.ctx, runtime, job); err != nil {
			var cancel *AdapterCancelError
			if errors.As(err, &cancel) {
				if cancel.Output != "" {
					s.eprintln(cancel.Output)
				}
				return exitWith(cancel.Code)
			}
			s.eprintln(err.Error())
			return exitWith(1)
		}
		return nil
	})
}

// internalCancel is internal_cancel: mark the record cancelling before any
// kill, wind its group down, conclude it cancelled and release its fence
// slot.
func (s *session) internalCancel(job string) error {
	record := s.recordPath(job)
	if !isFile(record) {
		return exitWith(1)
	}
	acquired, err := s.acquireLifecycleLockUntil(job, 5)
	if err != nil {
		return err
	}
	if !acquired {
		return exitWith(1)
	}
	status := fieldOr(record, "status")
	switch status {
	case "pending-setup", "pending", "running":
	default:
		s.releaseLifecycleLock(job)
		return nil
	}
	// The marker lands before the kill, so a reaper concluding the dead
	// group first still reads the cancel. A marker that cannot land stops
	// the cancel.
	patch, err := s.mustTemp(s.recordLocks, "cancelling")
	if err != nil {
		s.releaseLifecycleLock(job)
		return err
	}
	_ = writePatch(patch, `{"phase":"cancelling"}`)
	if _, err := s.recordCAS(job, status, status, patch); err != nil {
		status = fieldOr(record, "status")
		switch status {
		case "pending-setup", "pending", "running":
			// An already-marked record is a prior cancel's footprint; this
			// cancel finishes it. Anything else refuses.
			if fieldOr(record, "phase") != "cancelling" {
				status = fieldOr(record, "status")
				switch status {
				case "pending-setup", "pending", "running":
					s.releaseLifecycleLock(job)
					return s.die(1, fmt.Sprintf("cancel could not mark %s; refusing to kill an unmarked job", job))
				default:
					s.releaseLifecycleLock(job)
					return nil
				}
			}
		default:
			s.releaseLifecycleLock(job)
			return nil
		}
	}
	pidText := fieldOr(record, "pid")
	pgidText := fieldOr(record, "pgid")
	pgid, pgidErr := strconv.ParseInt(pgidText, 10, 64)
	hasGroup := naturalPattern.MatchString(pgidText) && pgidErr == nil && pgid > 1
	pid, _ := strconv.ParseInt(pidText, 10, 64)
	if hasGroup {
		if !s.windDownGroup(record) {
			s.releaseLifecycleLock(job)
			return exitWith(1)
		}
	} else if naturalPattern.MatchString(pidText) && pid > 0 {
		s.releaseLifecycleLock(job)
		return s.die(1, fmt.Sprintf("cancel refused: %s has a recorded process but no primary process group", job))
	}
	patch, err = s.mustTemp(s.recordLocks, "cancel")
	if err != nil {
		s.releaseLifecycleLock(job)
		return err
	}
	// A death stamp needs a real signalable group; a record that never had
	// one must not claim a death that never happened.
	body := `{"error":null,"phase":"cancelled"`
	if hasGroup {
		body += fmt.Sprintf(`,"groupDeathProvenAt":"%s"`, s.nowISO())
		if s.windDownKilled {
			body += `,"cancelEscalatedToKill":true`
		}
	}
	if s.cancelRefusalClass == "stopped" {
		body += `,"refusalClass":"stopped"`
	}
	_ = writePatch(patch, body+"}")
	if _, err := s.recordCAS(job, status, "cancelled", patch); err != nil && s.stopCancelAuthorized != "" {
		s.releaseLifecycleLock(job)
		return exitWith(1)
	}
	// The cancelled reservation's fence slot is not abandoned debt.
	if missionID := fieldOr(record, "mission"); missionID != "" && missionID != "null" {
		s.missionReleaseJob(missionID, job)
	}
	if s.stopCancelAuthorized == "" {
		_ = s.mirrorRecordLocked(job) // cancel holds the lifecycle lock
	}
	s.releaseLifecycleLock(job)
	return nil
}

// breachStopRun is internal_breach_stop_run: reconcile the stop batch until
// complete, cancelling each pending job under the batch's authority and
// each pending proof attempt.
func (s *session) breachStopRun(stopID string) error {
	for {
		now, err := s.goalNow()
		if err != nil {
			return s.die(1, fmt.Sprintf("breach-stop %s is indeterminate: %s", stopID, err.Error()))
		}
		batch, err := dispatch.ReconcileStopBatch(s.root, stopID, now)
		if err == nil && batch.State == "INDETERMINATE" {
			return s.die(1, fmt.Sprintf("breach-stop %s is indeterminate: %s", stopID, encodeJSON(batch)))
		}
		if err != nil {
			message := err.Error()
			var op *dispatch.OpError
			if asOpError(err, &op) {
				message = op.Error()
			}
			return s.die(1, fmt.Sprintf("breach-stop %s is indeterminate: %s", stopID, message))
		}
		if batch.State == goal.StopBatchComplete {
			return nil
		}
		pending, err := goal.ReadStopBatch(s.root, stopID)
		if err != nil {
			return s.verbFailure(err)
		}
		for _, job := range pending.Pending {
			if job == "" {
				continue
			}
			if err := dispatch.AuthorizeStopCancellation(s.root, stopID, job); err != nil {
				_ = s.verbFailure(err)
				return s.die(1, fmt.Sprintf("breach-stop %s lost cancellation authority for %s", stopID, job))
			}
			s.stopCancelAuthorized = stopID
			cancelErr := s.internalCancel(job)
			s.stopCancelAuthorized = ""
			if cancelErr != nil {
				return cancelErr
			}
		}
		for _, attempt := range pending.PendingProofs {
			if attempt == "" {
				continue
			}
			if err := s.cancelStopProof(stopID, attempt); err != nil {
				_ = s.verbFailure(err)
				return s.die(1, fmt.Sprintf("breach-stop %s could not cancel proof attempt %s", stopID, attempt))
			}
		}
	}
}

// cancelStopProof is `job stop-proof-cancel`: the fixture instant when the
// root authorizes one, else the owner's own clock.
func (s *session) cancelStopProof(stopID, attempt string) error {
	clock, fixture, err := s.goalClock()
	if err != nil {
		return err
	}
	if fixture {
		return dispatch.CancelStopProofAt(s.root, stopID, attempt, clock())
	}
	return dispatch.CancelStopProof(s.root, stopID, attempt)
}

// closeChain is close_chain, the chain's finish: mirror every terminal
// member, fold and close a critic register, close-check, then mark the root
// closed and remove the chain's build cache.
func (s *session) closeChain(args []string) error {
	if err := s.fenceBrain("close"); err != nil {
		return err
	}
	if len(args) < 2 || args[0] != "--job" {
		return s.usageExit()
	}
	job := args[1]
	s.dieJob = job
	runnerClosed, reconcileEvidence := false, ""
	for index := 2; index < len(args); {
		switch args[index] {
		case "--runner-closed":
			runnerClosed = true
			index++
		case "--reconcile-evidence":
			if index+1 >= len(args) || reconcileEvidence != "" {
				return s.usageExit()
			}
			reconcileEvidence = args[index+1]
			index += 2
		default:
			return s.usageExit()
		}
	}
	if err := s.leaseEntryCheck(); err != nil {
		return err
	}
	if !validID(job) || !isFile(s.recordPath(job)) {
		return s.die(1, "unknown job: "+job)
	}
	rootID, err := s.rootJobID(job)
	if err != nil {
		s.eprintln(err.Error())
		return s.die(1, "cannot resolve job chain")
	}
	if rootID != job {
		return s.die(1, "close requires the root job id: "+rootID)
	}
	if err := s.acquireChainLock(rootID); err != nil {
		return err
	}
	s.cleanupChain = rootID
	s.trap = func(int) error { return s.releaseChainLock(s.cleanupChain) }
	epoch := s.inv.ClaimEpoch
	if reconcileEvidence != "" {
		if !validID(reconcileEvidence) {
			return s.die(2, "invalid review evidence job id: "+reconcileEvidence)
		}
		if err := s.runHeld(epoch, func() error {
			return s.callbackCritiqueMutation("review-reference-reconcile", []string{"--root-job", rootID, "--evidence-job", reconcileEvidence})
		}); err != nil {
			return err
		}
	}
	// Closing asserts the chain's evidence is durable, so make it durable
	// first for every terminal member; close-check remains the authority.
	members, err := dispatch.ChainMemberStatuses(s.jobs, rootID, true)
	if err != nil {
		_ = s.verbFailure(err)
	}
	for _, line := range members {
		member, _, _ := strings.Cut(line, "|")
		_ = s.mirrorRecord(member)
	}
	role := fieldOr(s.recordPath(rootID), "role")
	critic := role == "design-critic" || role == "code-critic" || role == "warden"
	if critic {
		outcome, err := dispatch.CritiqueRegisterClose(s.root, rootID)
		if err != nil {
			return s.verbFailure(err)
		}
		s.println(outcome)
	}
	if err := s.verbFailure(dispatch.CloseCheck(s.root, rootID)); err != nil {
		return err
	}
	if critic {
		closeArgs := []string{"--root-job", rootID}
		if runnerClosed {
			closeArgs = append(closeArgs, "--runner-closed")
		}
		if err := s.runHeld(epoch, func() error { return s.callback("__critique-close", closeArgs) }); err != nil {
			return err
		}
	} else {
		status, ok := field(s.recordPath(rootID), "status")
		if !ok {
			return exitWith(1)
		}
		patch, err := s.mustTemp(s.recordLocks, "close")
		if err != nil {
			return err
		}
		body := `{"chainClosed":true}`
		if runnerClosed {
			body = `{"chainClosed":true,"runnerClosed":true}`
		}
		_ = writePatch(patch, body)
		casErr := s.runHeld(epoch, func() error { return s.internalRecordCAS(rootID, status, status, patch) })
		removeQuietly(patch)
		if casErr != nil {
			return casErr
		}
	}
	if err := s.releaseChainLock(rootID); err != nil {
		return err
	}
	s.trap = nil
	return nil
}

// admitCritiqueRead is admit_critique_read: the computed critic read is
// admitted under the lease before its job is published; a redundant read
// mirrors the prior critic chain whose event it recorded.
func (s *session) admitCritiqueRead(role, requestingRoot string, round int64, subjectFile, servedGoal string) error {
	result, err := s.mustTemp(s.recordLocks, "read-admission")
	if err != nil {
		return err
	}
	s.cleanupAdmission = result
	args := []string{"--role", role, "--root-job", requestingRoot, "--round", strconv.FormatInt(round, 10)}
	if servedGoal != "" {
		args = append(args, "--goal", servedGoal)
	}
	args = append(args, "--subject-file", subjectFile, "--result", result)
	// The admission's diagnostics are captured, then carried by the die.
	stderr := s.stderr
	var captured strings.Builder
	s.stderr = &captured
	stdoutMark := s.stdout.Len()
	admitErr := s.runHeld(s.inv.ClaimEpoch, func() error {
		return s.callbackCritiqueMutation("critique-read-admission", args)
	})
	output := strings.TrimRight(captured.String()+s.stdout.String()[stdoutMark:], "\n")
	s.stdout.Truncate(stdoutMark)
	s.stderr = stderr
	code := ExitCode(admitErr)
	if code == 0 {
		if fieldOr(result, "decision") != "ADMITTED" {
			return s.die(1, "critique read admission returned success without an ADMITTED result")
		}
		removeQuietly(result)
		s.cleanupAdmission = ""
		return nil
	}
	if code == 11 && fileNonEmpty(result) && fieldOr(result, "decision") == "REDUNDANT_READ" {
		prior := fieldOr(result, "criticRoot")
		if fieldOr(result, "eventRecorded") == "true" && prior != "" {
			_ = s.mirrorRecord(prior)
		}
	}
	if output == "" {
		output = "critique read admission failed without a diagnostic"
	}
	return s.die(code, output)
}
