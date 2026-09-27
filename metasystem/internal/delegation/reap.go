package delegation

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/returnschema"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/usage"
)

// handshakeBackstopGraceSec is how long the reaper waits past a record's
// handshake budget before calling an unfinished handshake process-lost.
const handshakeBackstopGraceSec = 2

func usageRootJobID(jobs, job string) (string, error) { return usage.RootJobID(jobs, job) }

func terminal(status string) bool {
	switch status {
	case "completed", "failed", "timeout", "cancelled":
		return true
	}
	return false
}

func (s *session) parseReapArgs(args []string) (string, string, error) {
	job, purpose := "", ""
	for index := 0; index < len(args); {
		switch args[index] {
		case "--job":
			if index+1 >= len(args) {
				return "", "", s.usageExit()
			}
			job = args[index+1]
			index += 2
		case "--purpose":
			if index+1 >= len(args) || args[index+1] != "post-wait" {
				return "", "", s.usageExit()
			}
			purpose = args[index+1]
			index += 2
		default:
			return "", "", s.usageExit()
		}
	}
	if job != "" && !validID(job) {
		return "", "", s.usageExit()
	}
	return job, purpose, nil
}

// reapJobs is reap_jobs: the lease-held single-shot reap the waiter and the
// mission drain use. A standing reaper mode must not come back: Go owns the
// standing sweep and shell reapers hold no kill authority.
func (s *session) reapJobs(args []string) error {
	// The brain fence holds on the lease re-entry too: a held reap (the
	// post-wait reap included) on a fenced checkout refuses like the public one.
	if err := s.fenceBrain("reap"); err != nil {
		return err
	}
	job, purpose, err := s.parseReapArgs(args)
	if err != nil {
		return err
	}
	if !s.leaseReentry {
		if err := s.leaseEntryCheck(); err != nil {
			return err
		}
		return s.runHeld(s.inv.ClaimEpoch, func() error { return s.internalReapHeld(job, "") })
	}
	if job != "" {
		reapErr := s.reapOne(job)
		// A waited round still needs its terminal record reconciled, but the
		// next round owns the same chain cache until an explicit reap or
		// close cleans it.
		if purpose != "post-wait" {
			s.reapChainBuildCache(job)
		}
		return reapErr
	}
	if err := os.MkdirAll(s.jobs, 0o755); err != nil {
		return exitWith(1)
	}
	// One failing reap must not starve the jobs after it: visit every
	// record, then report the sweep's verdict.
	records, _ := filepath.Glob(filepath.Join(s.jobs, "*.json"))
	sort.Strings(records)
	failed := false
	for _, record := range records {
		if !isFile(record) {
			continue
		}
		if s.reapOne(strings.TrimSuffix(filepath.Base(record), ".json")) != nil {
			failed = true
		}
	}
	s.reapChainBuildCaches()
	if failed {
		s.eprintln("reap sweep finished with failures (see above)")
		return exitWith(1)
	}
	return nil
}

// reapOne is reap_one: an explicit reap has no next tick, so a busy lock is
// waited out under a bound and a timeout is a real failure.
func (s *session) reapOne(job string) error {
	acquired, err := s.acquireLifecycleLockUntil(job, 5)
	if err != nil {
		return err
	}
	if !acquired {
		return exitWith(1)
	}
	defer s.releaseLifecycleLock(job)
	return s.reapOneLocked(job)
}

// reapOneLocked is reap_one_locked: conclude a record whose process is lost
// or whose budget expired; reconcile a terminal record's usage and evidence.
func (s *session) reapOneLocked(job string) error {
	record := s.recordPath(job)
	if !isFile(record) {
		return nil
	}
	status := fieldOr(record, "status")
	switch status {
	case "completed", "failed", "timeout", "cancelled":
		if rootID, err := s.rootJobID(job); err == nil && rootID != "" {
			s.aggregateChainUsage(rootID)
		}
		_ = s.aggregateMissionUsage(record)
		_ = s.mirrorRecord(job)
		return nil
	case "pending-setup", "pending", "running":
	default:
		return nil
	}
	now, err := s.goalNow()
	if err != nil {
		return s.verbFailure(err)
	}
	facts, err := dispatch.ComputeReapFacts(record, handshakeBackstopGraceSec, now)
	if err != nil {
		return s.verbFailure(err)
	}
	pid := fieldOr(record, "pid")
	phase := fieldOr(record, "phase")
	noPid := pid == "" || pid == "null"
	// A cancellation that won before any process identity was published has
	// no group to kill and no death to prove: conclude the marker directly.
	if noPid && phase == "cancelling" {
		patch, err := s.tempFile(s.recordLocks, "cancelled-before-launch")
		if err == nil {
			_ = writePatch(patch, `{"error":null,"phase":"supervision"}`)
			_, _ = s.recordCASQuiet(job, status, "cancelled", patch)
		}
		_ = s.mirrorRecord(job)
		return nil
	}
	if status == "pending-setup" {
		if facts.ReconciliationDue {
			if err := s.reconcileReservation(job); err != nil {
				return err
			}
		}
		return nil
	}
	// A job is inside its handshake while it has no session. The dispatcher
	// owns the handshake verdict; the reaper is the backstop for a
	// dispatcher that is no longer there, and "provably gone" needs the
	// record to name a supervisor first.
	if facts.HandshakeWaiting {
		if noPid || s.jobSupervisorMatches(record) {
			return nil
		}
	}
	if noPid {
		if facts.ReconciliationDue {
			if err := s.reconcileReservation(job); err != nil {
				return err
			}
		}
		return nil
	}
	// The cap is judged before process liveness: an expired budget is a fact
	// of the record alone; judging liveness first made the verdict a race.
	// The priority applies only to a job that actually ran.
	if status != "running" || !facts.BudgetExpired {
		if !s.jobSupervisorMatches(record) {
			if !s.windDownGroup(record) {
				return exitWith(1)
			}
			if s.recollectLostReturn(job, record, status) {
				return nil
			}
			patch, err := s.mustTemp(s.recordLocks, "lost")
			if err != nil {
				return err
			}
			_ = writePatch(patch, fmt.Sprintf(`{"error":"process-lost","phase":"supervision","groupDeathProvenAt":"%s"}`, s.nowISO()))
			observed, casErr := s.recordCASQuiet(job, status, "failed", patch)
			s.reapVerdictEvents(job, "failed", "process-lost", casErr, observed)
			_ = s.mirrorRecord(job)
			return nil
		}
	}
	if facts.BudgetExpired {
		if !s.windDownGroup(record) {
			return exitWith(1)
		}
		patch, err := s.mustTemp(s.recordLocks, "timeout")
		if err != nil {
			return err
		}
		_ = writePatch(patch, fmt.Sprintf(`{"error":"budget-cap","phase":"supervision","groupDeathProvenAt":"%s"}`, s.nowISO()))
		observed, casErr := s.recordCASQuiet(job, status, "timeout", patch)
		s.reapVerdictEvents(job, "timeout", "budget-cap", casErr, observed)
		missionID := fieldOr(record, "mission")
		if casErr == nil && missionID != "" && missionID != "null" {
			reason := "job-cap-min"
			if fieldOr(record, "capResolution.truncatedBy") == "wall-clock" {
				reason = "wall-clock-hours"
			}
			if err := s.missionRefuse(missionID, reason); err != nil {
				s.eprintf("MISSION-FENCE-ASK-FAILED mission=%s job=%s error=%s\n", missionID, job, strings.ReplaceAll(err.Error(), "\n", " "))
			}
			_ = s.aggregateMissionUsage(record)
		}
		_ = s.mirrorRecord(job)
	}
	return nil
}

// recordCASQuiet is `cas_out=$(record_cas ... 2>/dev/null)`: the verdict
// compare-and-swap whose observation is captured, not printed.
func (s *session) recordCASQuiet(job, expect, target, patch string) (string, error) {
	stdout, stderr := s.stdout.Len(), s.stderr
	s.stderr = discard{}
	observed, err := s.recordCAS(job, expect, target, patch)
	s.stderr = stderr
	s.stdout.Truncate(stdout)
	return observed, err
}

type discard struct{}

func (discard) Write(p []byte) (int, error) { return len(p), nil }

// reapVerdictEvents is reap_verdict_events: the verdict, or the refused
// compare with what it found.
func (s *session) reapVerdictEvents(job, verdict, reason string, casErr error, observed string) {
	missionID := fieldOr(s.recordPath(job), "mission")
	switch code := ExitCode(casErr); code {
	case 0:
		s.emit("job-verdict", map[string]string{"jobId": job, "missionId": missionID, "verdict": verdict, "reason": reason, "summary": reason})
	case 3:
		observed = strings.TrimPrefix(strings.TrimSpace(observed), "observed=")
		if observed == "" {
			observed = "unknown"
		}
		s.emit("verdict-refused", map[string]string{"jobId": job, "missionId": missionID, "attempted": verdict, "observed": observed,
			"summary": fmt.Sprintf("CAS refused: wanted %s, found %s", verdict, observed)})
	}
}

// jobSupervisorMatches is job_supervisor_matches: a live or uninspectable
// supervisor defers every kill-capable caller. The retired script also
// tried a fake runtime's trusted-launcher proof through an identity probe
// the engine never had, so that branch never matched; it is not ported.
func (s *session) jobSupervisorMatches(record string) bool {
	pidText := fieldOr(record, "pid")
	tag := fieldOr(record, "instanceTag")
	pid, _ := strconv.ParseInt(pidText, 10, 64)
	switch s.tagState(pid, pidText, tag) {
	case "live", "unknown":
		return true
	}
	return false
}

// windDownOneGroup is wind_down_one_group: TERM, then KILL, each re-proving
// the group's ownership first.
func (s *session) windDownOneGroup(record string, pgid int64) bool {
	process := s.l.ports.Process
	clock := s.l.ports.Clock
	tag := fieldOr(record, "instanceTag")
	if !process.GroupExists(pgid) {
		return true
	}
	if !process.GroupOwned(record, pgid, tag) {
		s.eprintf("refusing to signal unowned process group %d\n", pgid)
		return false
	}
	_ = process.SignalGroup(pgid, SignalTerm)
	waitGone := func() {
		until := clock.Now().Add(2 * time.Second)
		for process.GroupExists(pgid) && clock.Now().Before(until) {
			clock.Sleep(50 * time.Millisecond)
		}
	}
	waitGone()
	if process.GroupExists(pgid) {
		if !process.GroupOwned(record, pgid, tag) {
			s.eprintf("lost ownership proof for process group %d\n", pgid)
			return false
		}
		_ = process.SignalGroup(pgid, SignalKill)
		s.windDownKilled = true
	}
	waitGone()
	if process.GroupExists(pgid) {
		s.eprintf("process group %d survived KILL\n", pgid)
		return false
	}
	return true
}

// windDownGroup is wind_down_group over every custody group of the record.
func (s *session) windDownGroup(record string) bool {
	s.windDownKilled = false
	object, err := dispatch.ReadRecordObject(record)
	if err != nil {
		s.eprintln(err.Error())
		return false
	}
	groups, err := s.l.ports.Process.CustodyGroups(object)
	if err != nil {
		s.eprintln(err.Error())
		return false
	}
	refused := false
	for _, pgid := range groups {
		if !s.windDownOneGroup(record, pgid) {
			refused = true
		}
	}
	return !refused
}

// recollectLostReturn is recollect_lost_return: a job dying process-lost
// with a complete, schema-valid return in its newest round delivered its
// work, and concludes completed with recollection provenance.
func (s *session) recollectLostReturn(job, record, status string) bool {
	if status != "running" {
		return false
	}
	role := fieldOr(record, "role")
	rounds, _ := os.ReadDir(filepath.Join(s.agents, job, "rounds"))
	best, roundDir := int64(0), ""
	for _, entry := range rounds {
		if !entry.IsDir() || !naturalPattern.MatchString(entry.Name()) {
			continue
		}
		number, _ := strconv.ParseInt(entry.Name(), 10, 64)
		if number > best {
			best, roundDir = number, filepath.Join(s.agents, job, "rounds", entry.Name())
		}
	}
	returnFile := filepath.Join(roundDir, "return.json")
	if roundDir == "" || !fileNonEmpty(returnFile) {
		return false
	}
	if len(returnschema.ReturnCompleteRole(s.root, role, returnFile)) > 0 {
		return false
	}
	usagePath := ""
	if fileNonEmpty(filepath.Join(roundDir, "usage.json")) {
		usagePath = filepath.Join(roundDir, "usage.json")
	}
	patch, err := s.tempFile(s.recordLocks, "recollect")
	if err != nil {
		return false
	}
	defer removeQuietly(patch)
	if err := s.l.ports.Adapter.ResultPatch(patch, "null", "supervision", usagePath); err != nil {
		return false
	}
	if err := setJSONFields(patch, map[string]string{"recollectedAt": s.nowISO(), "recollectedFrom": "process-lost"}); err != nil {
		return false
	}
	observed, casErr := s.recordCASQuiet(job, status, "completed", patch)
	if casErr != nil {
		return false
	}
	s.reapVerdictEvents(job, "completed", "recollected", nil, observed)
	_ = s.mirrorRecord(job)
	return true
}

// aggregateChainUsage is aggregate_chain_usage.
func (s *session) aggregateChainUsage(chain string) {
	record := s.recordPath(chain)
	if !isFile(record) {
		return
	}
	status := fieldOr(record, "status")
	if !terminal(status) {
		return
	}
	patch, err := s.tempFile(s.recordLocks, "usage")
	if err != nil {
		return
	}
	unchanged, err := dispatch.ChainUsage(s.jobs, chain, patch)
	if err != nil {
		_ = s.verbFailure(err)
	}
	if err == nil && unchanged {
		removeQuietly(patch)
		return
	}
	_, _ = s.recordCAS(chain, status, status, patch)
}

// aggregateMissionUsage is aggregate_mission_usage.
func (s *session) aggregateMissionUsage(record string) error {
	missionID := fieldOr(record, "mission")
	if missionID == "" || missionID == "null" {
		return nil
	}
	return s.missionAggregateUsage(missionID)
}

// mirrorFail is mirror_fail: a durable trace beside the jobs it failed for.
func (s *session) mirrorFail(job, reason string) {
	line := fmt.Sprintf("%s %s %s\n", s.l.ports.Clock.Now().UTC().Format("2006-01-02T15:04:05Z"), job, reason)
	_ = appendFile(filepath.Join(filepath.Dir(s.jobs), "mirror-failures.log"), []byte(line))
	s.eprintf("cannot mirror %s: %s\n", job, reason)
}

// mirrorRecord is mirror_record: a terminal job's evidence copied under the
// evidence root, stamped on exactly the job that was mirrored.
func (s *session) mirrorRecord(job string) error {
	record := s.recordPath(job)
	if !isFile(record) {
		return nil
	}
	status := fieldOr(record, "status")
	if !terminal(status) {
		return nil
	}
	evidence, err := s.configGet("evidence.root", "")
	if err != nil {
		return err
	}
	if !strings.HasPrefix(evidence, "/") {
		s.mirrorFail(job, "evidence.root must be absolute")
		return exitWith(1)
	}
	rootID, err := s.rootJobID(job)
	if err != nil {
		s.eprintln(err.Error())
		return exitWith(1)
	}
	result, err := s.tempFile(s.recordLocks, "mirror-result")
	if err != nil {
		return exitWith(1)
	}
	if err := dispatch.Mirror(s.root, s.repoScope, evidence, rootID, job, result); err != nil {
		_ = s.verbFailure(err)
		s.mirrorFail(job, "copy or verification failed (see stderr above)")
		return exitWith(1)
	}
	if fieldOr(result, "unchanged") == "true" && fieldOr(record, "mirror.manifest") == fieldOr(result, "manifest") {
		removeQuietly(result)
		return nil
	}
	content, err := os.ReadFile(result)
	if err != nil {
		return exitWith(1)
	}
	patch, err := s.tempFile(s.recordLocks, "mirror-patch")
	if err != nil {
		return exitWith(1)
	}
	if err := os.WriteFile(patch, []byte(`{"mirror":`+strings.TrimRight(string(content), "\n")+"}\n"), 0o600); err != nil {
		return exitWith(1)
	}
	if _, err := s.recordCAS(job, status, status, patch); err != nil {
		return exitWith(1)
	}
	removeQuietly(result)
	return nil
}

// removeChainBuildCache is remove_chain_build_cache: the chain's warm gate
// cache in the root worktree's private git dir (0.5 to 1 GB once warm).
func (s *session) removeChainBuildCache(rootJob string) {
	workspace := fieldOr(s.recordPath(rootJob), "workspaceRoot")
	if workspace == "" || workspace == "null" || !isDir(workspace) {
		return
	}
	worktrees, err := physicalDir(s.worktrees)
	if err != nil {
		return
	}
	resolved, err := physicalDir(workspace)
	if err != nil || !strings.HasPrefix(resolved+"/", worktrees+"/") {
		return
	}
	out, _, err := s.l.ports.Git.Run(s.ctx, workspace, "rev-parse", "--absolute-git-dir")
	if err != nil {
		return
	}
	gitDir := strings.TrimSpace(string(out))
	cache := filepath.Join(gitDir, "metasystem-build-cache")
	if !strings.Contains(gitDir, "/.git/worktrees/") || !isDir(cache) {
		return
	}
	_ = os.RemoveAll(cache)
}

func (s *session) reapChainBuildCache(job string) {
	if !isFile(s.recordPath(job)) {
		return
	}
	rootJob, err := s.rootJobID(job)
	if err != nil || rootJob == "" || !isFile(s.recordPath(rootJob)) || !isDir(filepath.Join(s.worktrees, rootJob)) {
		return
	}
	s.reapChainBuildCacheOfRoot(rootJob)
}

// reapChainBuildCaches visits chains that have a job worktree, not every
// record: the cache can only live under one.
func (s *session) reapChainBuildCaches() {
	entries, _ := os.ReadDir(s.worktrees)
	for _, entry := range entries {
		if entry.IsDir() && isFile(s.recordPath(entry.Name())) {
			s.reapChainBuildCacheOfRoot(entry.Name())
		}
	}
}

func (s *session) reapChainBuildCacheOfRoot(rootJob string) {
	members, err := dispatch.ChainMemberStatuses(s.jobs, rootJob, false)
	if err != nil {
		return
	}
	terminalMembers, err := dispatch.ChainMemberStatuses(s.jobs, rootJob, true)
	if err != nil {
		return
	}
	if len(members) > 0 && len(members) == len(terminalMembers) {
		s.removeChainBuildCache(rootJob)
	}
}

// setJSONFields is `json set --file PATH --field k=v ...` for string values.
func setJSONFields(path string, fields map[string]string) error {
	object, err := dispatch.ReadRecordObject(path)
	if err != nil {
		return err
	}
	for key, value := range fields {
		object[key] = value
	}
	return os.WriteFile(path, []byte(encodeJSON(object)+"\n"), 0o600)
}
