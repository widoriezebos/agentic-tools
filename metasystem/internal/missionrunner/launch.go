package missionrunner

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/fixtureauth"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/boundedexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/contract"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/mission"
)

// The launch side of the runner: what happens in the caller's process before
// the detached run loop exists — stale-lease cleanup, supervision arming, the
// contract preflight and pin, and the start-signal handshake with the child.

// pathExists reports whether a path resolves to anything.
func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// stateBorn reports whether the mission's birth certificate exists: a
// regular file at state.json. Nothing else counts as a birth — a
// directory or other object squatting on the path cannot carry the state
// hash chain, so treating it as a living mission would both block the
// corrected retry and shield stillborn artifacts from cleanup.
func stateBorn(statePath string) bool {
	info, err := os.Lstat(statePath)
	return err == nil && info.Mode().IsRegular()
}

// launchLock serializes the start side of one mission id: the parent's
// evidence checks and pin writes and the child's birth each hold it
// exclusively, so a launcher's cached decision can never mutate a
// mission that was born after the check was made.
type launchLock struct{ f *os.File }

func (e *Engine) acquireLaunchLock() (*launchLock, error) {
	path := filepath.Join(e.missionDir(), "launch.lock")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX); err != nil {
		f.Close()
		return nil, failf(3, "cannot acquire the mission launch lock: %v", err)
	}
	return &launchLock{f: f}, nil
}

func (l *launchLock) release() {
	_ = unix.Flock(int(l.f.Fd()), unix.LOCK_UN)
	_ = l.f.Close()
}

// birthRecordPath is the mission's durable birth record: written the
// moment state is first published and never removed by any cleanup. A
// missing state file alone is ambiguous — stillborn remnants and a
// LIVING mission whose state was lost look identical without it — and
// only honest never-born absence may authorize the stillborn machinery.
func (e *Engine) birthRecordPath() string {
	return filepath.Join(e.missionDir(), "born.json")
}

// bornEvidence reports whether the mission provably lived: the durable
// birth record, or a ledger that booked at least one cycle (a stillborn
// ledger never gets past its header). Absence must be PROVEN — a probe
// that cannot read is an error, never a green light, because the
// callers authorize destruction on emptiness.
func (e *Engine) bornEvidence(ledger string) (string, error) {
	if pathExists(e.birthRecordPath()) {
		return "its birth record exists", nil
	}
	data, err := os.ReadFile(ledger)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", failf(3, "cannot prove the mission never lived: its ledger is unreadable: %v", err)
	}
	if strings.Contains(string(data), "\n### Cycle ") {
		return "its ledger booked cycles", nil
	}
	return "", nil
}

// missionAnchorsExist reports whether ANY anchor survives in the
// mission's ref namespace. With no state file and no birth evidence,
// surviving anchors still mean a mission may have lived here — or a
// birth crashed mid-staging — and both are a human's call, never the
// stillborn machinery's. The stillborn sweep itself keys on the
// narrower birth evidence: a same-pass failure drops its own staging
// anchors, so only honest emptiness reaches a retry.
func (e *Engine) missionAnchorsExist() (bool, error) {
	stdout, stderr, code := gitCaptured(e.Root, "for-each-ref",
		"--format=%(refname)", "refs/metasystem/missions/"+e.Mission+"/")
	if code != 0 {
		// Only a SUCCESSFUL empty enumeration proves absence; a failed
		// probe must never authorize what runs on emptiness.
		return false, failf(3, "cannot prove the mission's anchor namespace is empty: %s", firstDetail(stderr, stdout))
	}
	return strings.TrimSpace(stdout) != "", nil
}

// startAmbiguityRefusal is every start entry's freeze decision when no
// state file exists: durable birth evidence or surviving anchors refuse
// the start with the remedy named; only honest emptiness returns nil.
func (e *Engine) startAmbiguityRefusal(ledger string) error {
	evidence, err := e.bornEvidence(ledger)
	if err != nil {
		return err
	}
	if evidence != "" {
		// The remedy must be PERFORMABLE for the evidence at hand:
		// telling an operator to remove a birth record that booked
		// cycles never wrote — or that is already gone — unwedges
		// nothing.
		remedy := "restore state.json or remove born.json by hand"
		if evidence == "its ledger booked cycles" {
			remedy = "restore state.json, or archive the mission directory (its ledger included) by hand"
		}
		return failf(3, "mission start refused: this mission has birth evidence (%s) but no state file; a birth may have been interrupted or the state lost — %s; nothing was touched", evidence, remedy)
	}
	anchored, err := e.missionAnchorsExist()
	if err != nil {
		return err
	}
	if anchored {
		return failf(3, "mission start refused: this mission's anchor namespace is not empty; a mission may have lived here or a birth crashed mid-staging — inspect refs/metasystem/missions/%s/ and remove the refs by hand; nothing was touched", e.Mission)
	}
	return nil
}

// stateShapeRefusal freezes a mission id whose state path is occupied by
// anything that exists but is not a regular file. Such an object is
// neither a birth certificate nor a clean absence: treating it as absent
// would let a start sweep artifacts that may belong to a living mission
// behind a symlink, and treating it as born would read bytes the state
// chain never covered. Nothing is touched until a human removes it.
func stateShapeRefusal(statePath string) error {
	info, err := os.Lstat(statePath)
	if err != nil || info.Mode().IsRegular() {
		return nil
	}
	return failf(3, "mission state path is occupied by a non-regular object (%s); remove %s by hand before any start or resume", info.Mode(), statePath)
}

// gitCaptured runs one git command on the runner's own surface: the
// repository-steering environment stripped and object replacement
// disabled, so an inherited GIT_DIR or a planted replace ref can never
// steer what the runner reads. The same posture applies to every runner
// git surface.
func gitCaptured(dir string, args ...string) (stdout, stderr string, code int) {
	full := append([]string{"-c", "core.useReplaceRefs=false", "-c", "gc.auto=0", "-c", "maintenance.auto=false"}, args...)
	return runCaptured(dir, gittree.ScrubbedEnviron(), "git", full...)
}

// runCaptured runs a command from a working directory, capturing both
// streams. A command that could not start reports exit -1 with the launch
// error as its stderr.
func runCaptured(dir string, env []string, name string, args ...string) (stdout, stderr string, code int) {
	command := exec.Command(name, args...)
	command.Dir = dir
	if env != nil {
		command.Env = env
	} else {
		// Runner-owned scripts inherit the SCRUBBED environment: none of
		// them lawfully needs a repository-steering variable, and any git
		// they spawn must judge the checkout it runs in.
		command.Env = gittree.ScrubbedEnviron()
	}
	var outBuf, errBuf bytes.Buffer
	command.Stdout = &outBuf
	command.Stderr = &errBuf
	// Every command this runner launches is bounded (B4): a hung child
	// would otherwise stall the mission turn that is waiting for it.
	limit := boundedexec.Timeout(filepath.Join(dir, "metasystem.conf"), boundedexec.Local)
	err := boundedexec.Run(command, limit, "mission runner command "+name)
	switch typed := err.(type) {
	case nil:
		code = 0
	case *exec.ExitError:
		code = typed.ExitCode()
	default:
		code = -1
		fmt.Fprintln(&errBuf, err)
	}
	return outBuf.String(), errBuf.String(), code
}

// firstDetail words a wrapped tool's refusal: stderr when it said anything
// there, its stdout otherwise, trimmed.
func firstDetail(stderr, stdout string) string {
	if detail := strings.TrimSpace(stderr); detail != "" {
		return detail
	}
	return strings.TrimSpace(stdout)
}

// staleTurnOpen reports whether a turn record still claims to be pending or
// running — the turns a stale-lease cleanup must wind down and mark lost.
func staleTurnOpen(turn map[string]any) bool {
	status, _ := turn["status"].(string)
	if status == "pending" || status == "running" {
		return true
	}
	return turn["outcome"] == "running"
}

// markerHoldsOnlyOwner reports whether a lease marker directory holds nothing
// but owner.json record files — the only content a clean acquisition leaves.
// Anything else refuses the automatic cleanup: an unexpected file means some
// other process treats this directory as its own.
func markerHoldsOnlyOwner(marker string, entries []os.DirEntry) bool {
	for _, entry := range entries {
		if entry.Name() != "owner.json" {
			return false
		}
		info, err := os.Stat(filepath.Join(marker, entry.Name()))
		if err != nil || !info.Mode().IsRegular() {
			return false
		}
	}
	return true
}

// cleanupStaleLease clears a dead predecessor's lease so a new runner can
// take the mission: refuse when the recorded runner is provably still live,
// wind down any host group an open turn left behind and mark those turns
// lost, then remove the marker directory and lease record.
func (e *Engine) cleanupStaleLease() error {
	dir := e.missionDir()
	marker := filepath.Join(dir, "lease.d")
	leasePath := filepath.Join(dir, "lease.json")
	// The same flock the acquirer holds: classification must never
	// judge a half-published claim, or two runners can be minted for
	// one mission.
	release, lockErr := lease.LockBounded(filepath.Join(dir, "lease.lock"), "mission lease")
	if lockErr != nil {
		return lockErr
	}
	defer release()
	markerExists := pathExists(marker)
	if !markerExists && !pathExists(leasePath) {
		return nil
	}
	leaseDoc := map[string]any{}
	if pathExists(leasePath) {
		var err error
		if leaseDoc, err = readDocLabeled(leasePath, "mission lease", 3); err != nil {
			return err
		}
	}
	pid, pidOK := jsonInt(leaseDoc["pid"])
	tag, tagOK := leaseDoc["instanceTag"].(string)
	if pidOK && tagOK && pidExists(int(pid)) && strings.Contains(processCommand(int(pid), fixtureauth.CommandProbe{}), tag) {
		return failf(3, "mission runner is already live for %s", e.Mission)
	}
	turnPaths, _ := filepath.Glob(filepath.Join(dir, "turns", "*", "turn.json"))
	sort.Strings(turnPaths)
	for _, path := range turnPaths {
		turn, err := readJSONDoc(path)
		if err != nil || !staleTurnOpen(turn) {
			continue
		}
		pgid, pgidOK := jsonInt(turn["pgid"])
		turnTag, turnTagOK := turn["instanceTag"].(string)
		if !pgidOK || !turnTagOK || !groupAlive(int(pgid)) {
			continue
		}
		if err := e.terminateGroup(int(pgid), turnTag, turn["runtime"] == "fake"); err != nil {
			return err
		}
		for key, value := range map[string]any{
			"status": "failed", "outcome": "failed",
			"error": "turn-lost", "detail": "turn-lost", "endedAt": nowISO(),
		} {
			turn[key] = value
		}
		if err := atomicWriteJSON(path, turn); err != nil {
			return err
		}
	}
	if markerExists {
		entries, err := os.ReadDir(marker)
		if err != nil {
			return err
		}
		if !markerHoldsOnlyOwner(marker, entries) {
			return failf(3, "stale mission lease marker contains unexpected files: %s", marker)
		}
		for _, entry := range entries {
			if err := os.Remove(filepath.Join(marker, entry.Name())); err != nil {
				return err
			}
		}
		if err := os.Remove(marker); err != nil {
			return err
		}
	}
	if err := os.Remove(leasePath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// armingIdentity is who this runner arms supervision as.
type armingIdentity struct {
	session string
	pid     int
	started int64
	tag     string
	lineage string // empty when arming beneath a live holder
}

// resolveArmingIdentity decides who arms: session, pid, start, tag, lineage.
//
// The lineage is the mission's own ONLY when this runner is the main. Beneath
// a live holder the runner is part of that main's work, so it announces
// nothing new and must not rewrite that holder's lineage; only the
// unattended branch carries a fresh lineage.
//
// A runner started BY the main that holds this checkout is part of that
// main's work, not a second writer competing for the same checkout. Arming
// under a fresh identity there announces a second main and is correctly
// refused as OWNED-ELSEWHERE. Unattended — a benchmark target, a scratch
// checkout, anything with no live holder — the runner IS the main, and
// announces itself.
func (e *Engine) resolveArmingIdentity() (armingIdentity, error) {
	pid := os.Getpid()
	if view, err := lease.ClassifyVerb(e.Root, int64(pid)); err == nil {
		if view.Holder && view.Announcement != nil {
			return armingIdentity{
				session: view.Announcement.SessionId,
				pid:     int(view.Announcement.Pid),
				started: view.Announcement.PidStartedAt,
				tag:     view.Announcement.InstanceTag,
			}, nil
		}
	}
	started, err := processStartedAt(pid)
	if err != nil {
		return armingIdentity{}, err
	}
	return armingIdentity{
		session: fmt.Sprintf("mission-runner-%s-%d", e.Mission, pid),
		pid:     pid,
		started: started,
		tag:     "mission-runner.sh",
		lineage: MissionLineage(e.Mission),
	}, nil
}

// pinVerifiedContract records the preflighted contract snapshot as the bytes
// this mission runs against, under the mission's fence lock. The pin is the
// raw-file SHA-256 of one preflight invocation's verified bytes, including
// the Approval line and trailing whitespace.
func (e *Engine) pinVerifiedContract(mode string, snapshot []byte, approvedSHA string) error {
	dir := e.missionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	lock, err := os.OpenFile(filepath.Join(dir, "mission-fence.lock"), os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := unix.Flock(int(lock.Fd()), unix.LOCK_EX); err != nil {
		return err
	}
	fencesPath := e.fencesPath()
	var fences map[string]any
	if pathExists(fencesPath) {
		if fences, err = readDocLabeled(fencesPath, "mission fence counters", 3); err != nil {
			return err
		}
	} else {
		if mode != "start" {
			return failf(3, "mission resume refused: fence state is absent")
		}
		fences = map[string]any{
			"schemaVersion": 1,
			"missionId":     e.Mission,
			"startedAt":     nowISO(),
			"cycles":        0,
			"reservations":  map[string]any{},
		}
	}
	if schema, ok := jsonInt(fences["schemaVersion"]); !ok || schema != 1 || fences["missionId"] != e.Mission {
		return failf(3, "mission fence counters have an invalid identity")
	}
	if mode == "start" {
		// Birth evidence outranks EVERYTHING a start could write here:
		// with the state file gone but the mission provably lived (or a
		// birth interrupted), no pin, fence, or clock may change.
		if err := stateShapeRefusal(filepath.Join(e.missionDir(), "state.json")); err != nil {
			return err
		}
		born := stateBorn(filepath.Join(e.missionDir(), "state.json"))
		if !born {
			if err := e.startAmbiguityRefusal(filepath.Join(e.missionDir(), "ledger.md")); err != nil {
				return err
			}
		}
		if fences["approvedContractSha256"] != nil {
			// A pin WITHOUT a born mission is a stillborn remnant: the
			// state hash chain is the birth certificate, and until it
			// exists a corrected start may simply re-pin — no partial
			// cleanup can wedge the mission id.
			if born {
				return failf(3, "mission start refused: approved contract is already pinned; use resume")
			}
			// The never-born mission spent none of its sealed budget:
			// the remnant's clock resets so an interrupted cleanup
			// cannot eat the first cycle's wall time.
			fences["startedAt"] = nowISO()
		}
	}
	sum := sha256.Sum256(snapshot)
	if hex.EncodeToString(sum[:]) != approvedSHA {
		return failf(3, "mission preflight snapshot does not match its verified raw-file sha256")
	}
	if err := atomicWriteBytes(e.approvedContractPath(), snapshot); err != nil {
		return err
	}
	fences["approvedContractSha256"] = approvedSHA
	return atomicWriteJSON(fencesPath, fences)
}

// armAndPreflight arms supervision as the resolved identity, preflights the
// authored contract, and pins the verified snapshot for this mission.
func (e *Engine) armAndPreflight(mode string) error {
	// Checks and writes happen under ONE exclusive hold: without it, a
	// second launcher could pass the birth checks, pause, and overwrite
	// the pin and clock of a mission the first launcher birthed in the
	// gap.
	lock, err := e.acquireLaunchLock()
	if err != nil {
		return err
	}
	defer lock.release()
	identity, err := e.resolveArmingIdentity()
	if err != nil {
		return err
	}

	args := []string{
		"--repo", e.Root,
		"--session", identity.session,
		"--pid", strconv.Itoa(identity.pid),
		"--start-time", strconv.FormatInt(identity.started, 10),
		"--tag", identity.tag,
	}
	if identity.lineage != "" {
		// Every process of this mission derives the same lineage, so a
		// successor renews the lease instead of taking it over and sweeping
		// the predecessor's in-flight delegates.
		args = append(args, "--owner-lineage", identity.lineage)
	}
	stdout, stderr, code := runCaptured(e.Root, nil,
		filepath.Join(e.Root, "scripts", "agents", "arm-supervision.sh"), args...)
	if code != 0 || !strings.Contains(stdout, "up outcome=armed") {
		return failf(3, "mission start refused: supervision did not arm: %s", firstDetail(stderr, stdout))
	}
	verified, err := os.CreateTemp("", "mission-"+e.Mission+"-verified.*.contract.md")
	if err != nil {
		return err
	}
	verifiedPath := verified.Name()
	verified.Close()
	defer os.Remove(verifiedPath)
	// The PUBLIC ladder names every non-regular contract shape before
	// anything dereferences or READS it: a symlinked contract would
	// otherwise refuse with only the generic origin error, and a FIFO
	// would hang the blocking read outright. Honest absence falls
	// through — that is contract preflight's refusal to name.
	if err := contractShapeRefusal(e.contractPath()); err != nil {
		return err
	}
	_, rawSHA, err := contract.Preflight(e.contractPath(), verifiedPath)
	if err != nil {
		return failf(3, "mission start refused by preflight: %v", err)
	}
	snapshot, err := os.ReadFile(verifiedPath)
	if err != nil {
		return failf(3, "mission start refused: verified contract snapshot is unreadable: %v", err)
	}
	// The wall's repository preconditions gate every launch mode: a
	// repository that can hide mode drift, or a start over
	// unsealed dirt, refuses before the first turn. The values come from
	// the VERIFIED SNAPSHOT bytes — never a reread of the mutable
	// authored file, which could change between verification and use.
	_, values, _, err := e.parseContractAt(verifiedPath, false)
	if err != nil {
		return err
	}
	if err := e.wallPreflight(mode, values, snapshot); err != nil {
		return err
	}
	return e.pinVerifiedContract(mode, snapshot, rawSHA)
}

// Launch starts or resumes the mission: validate the request, clear a stale
// lease, arm and pin, spawn the detached run loop, and hold the caller until
// the first host turn verifiably starts (or the refusal is known). Prints the
// outcome and returns the process exit code.
func (e *Engine) Launch(mode string, foreground bool) int {
	if err := e.launch(mode, foreground); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return exitFor(err)
	}
	return 0
}

func (e *Engine) launch(mode string, foreground bool) error {
	statePath := filepath.Join(e.missionDir(), "state.json")
	if err := stateShapeRefusal(statePath); err != nil {
		return err
	}
	if mode == "start" && stateBorn(statePath) {
		return failf(3, "mission state already exists; use resume")
	}
	// Birth evidence is consulted before ANY mutation on the launch
	// path — the stale-lease cleanup below rewrites lease and turn
	// records, and a mission that lived must stay untouched.
	if mode == "start" && !stateBorn(statePath) {
		if err := e.startAmbiguityRefusal(filepath.Join(e.missionDir(), "ledger.md")); err != nil {
			return err
		}
	}
	if mode == "resume" {
		if !pathExists(statePath) {
			return failf(7, "mission state does not exist")
		}
		state, err := e.verifyState(statePath, false)
		if err != nil {
			return err
		}
		if state["status"] == "parked" && state["parkReason"] == drainStalledReason {
			// The drain-stalled park writes state then ask; a crash between
			// the two leaves a park nobody can answer. The public resume
			// re-raises the missing ask idempotently before anything else,
			// then refuses as usual — the human answers the ask and resumes
			// again.
			if err := e.ensureDrainStallAsk(state); err != nil {
				return err
			}
		}
		// A crash between the verification write and the turn record's
		// terminal patch leaves a completed mission with a non-terminal
		// record; the projection is idempotently derivable from the
		// durable state, so it repairs here before any refusal.
		if err := e.repairTerminalTurnRecords(state); err != nil {
			return err
		}
		if state["status"] != "running" {
			// A consumed-but-unconcluded acceptance is admitted through to
			// the resume path, which completes its verification first —
			// except under a wall-violation park, whose exit is the
			// human's resolution. Without this lane, a park written at the
			// acceptance would strand its turn unconcluded forever.
			pendingVerification := false
			if openTurn, _ := state["openTurn"].(map[string]any); openTurn != nil && state["parkReason"] != "wall-violation" {
				if turnID, _ := openTurn["turnId"].(string); turnID != "" && mission.UnverifiedAcceptance(state) == turnID {
					pendingVerification = true
				}
			}
			if !pendingVerification {
				// A completed mission may still owe its terminal delivery
				// or anchor (crash after the verification write): heal
				// idempotently BEFORE the refusal, so the terminal-lag
				// case is never stranded behind it.
				if state["status"] == "completed" {
					// The LIVE-runner refusal comes first: a
					// runner mid-conclusion still owes its delivery and
					// closing anchor, and reconciling under it would
					// anchor the completed hash to pre-delivery bytes —
					// the one shape no later heal admits.
					if err := e.cleanupStaleLease(); err != nil {
						return err
					}
					if err := e.healTerminalPublication(statePath, state); err != nil {
						return err
					}
				}
				return failf(3, "mission is %s; answer its park reason before resume", valueString(state["status"]))
			}
		}
	}
	if err := e.cleanupStaleLease(); err != nil {
		return err
	}
	if err := e.armAndPreflight(mode); err != nil {
		return err
	}
	tag := fmt.Sprintf("metasystem-mission-runner-%s-%s", e.Mission, randomHex(3))
	signalPath := filepath.Join(e.missionDir(), "runner-start-"+randomHex(4)+".json")
	_, _, logPath := e.runnerPaths()
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return err
	}
	logFile, err := os.OpenFile(logPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer logFile.Close()
	self, err := os.Executable()
	if err != nil {
		return err
	}
	command := exec.Command(self, "mission", "run-loop",
		"--root", e.Root,
		"--mission", e.Mission,
		"--mode", mode,
		"--instance-tag", tag,
		"--start-signal", signalPath,
	)
	command.Dir = e.Root
	if foreground {
		command.Stdout = os.Stdout
		command.Stderr = os.Stderr
	} else {
		command.Stdout = logFile
		command.Stderr = logFile
		command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	}
	process, err := startProcess(command)
	if err != nil {
		return err
	}
	// The verification window bounds REAL work — the child runner's boot,
	// preflight, and first host spawn are git-heavy and do not compress —
	// and the poll loop exits the moment the signal lands, so generosity
	// is free. Floor at the full base: compression must never shrink it
	// (the nested-birth wedge: a 5s floor under a ~8s real nested birth).
	verifyWindow, err := ScaledWaitAtLeast(15, 15*time.Second)
	if err != nil {
		return err
	}
	graceWindow, err := ScaledWait(5)
	if err != nil {
		return err
	}
	poll, err := Interval("METASYSTEM_HANDSHAKE_POLL_INTERVAL_MS", 50)
	if err != nil {
		return err
	}
	deadline := time.Now().Add(verifyWindow)
	for !time.Now().After(deadline) {
		if pathExists(signalPath) {
			signal, err := readDocLabeled(signalPath, "runner start signal", 3)
			if err != nil {
				return err
			}
			_ = os.Remove(signalPath)
			if signal["verified"] == true {
				if foreground {
					<-process.done
				}
				fmt.Printf("mission=%s started=yes turn=%s\n", e.Mission, valueString(signal["turnId"]))
				return nil
			}
			process.waitFor(graceWindow)
			return failf(3, "mission start refused: %s", valueString(signal["error"]))
		}
		if process.exited() {
			message := "runner exited before verified host start"
			recordPath, _, _ := e.runnerPaths()
			if pathExists(recordPath) {
				if record, err := readDocLabeled(recordPath, "mission runner record", 3); err == nil {
					if detail := valueString(record["error"]); detail != "" {
						message = detail
					}
				}
			}
			return failf(3, "%s", message)
		}
		time.Sleep(poll)
	}
	if !process.exited() {
		pid := command.Process.Pid
		if strings.Contains(processCommand(pid, fixtureauth.CommandProbe{}), tag) {
			if pgid, pgErr := unix.Getpgid(pid); pgErr == nil {
				if err := e.terminateGroup(pgid, tag, false); err != nil {
					return err
				}
			}
		}
	}
	return failf(3, "mission runner start verification timed out")
}
