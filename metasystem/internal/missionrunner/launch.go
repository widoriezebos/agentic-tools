package missionrunner

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
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
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
)

// The launch side of the runner: what happens in the caller's process before
// the detached run loop exists — stale-lease cleanup, supervision arming, the
// contract preflight and pin, and the start-signal handshake with the child.

// pathExists reports whether a path resolves to anything.
func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// runCaptured runs a command from a working directory, capturing both
// streams. A command that could not start reports exit -1 with the launch
// error as its stderr.
func runCaptured(dir string, env []string, name string, args ...string) (stdout, stderr string, code int) {
	command := exec.Command(name, args...)
	command.Dir = dir
	if env != nil {
		command.Env = env
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
	if pidOK && tagOK && pidExists(int(pid)) && strings.Contains(processCommand(int(pid), false), tag) {
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
// nothing new and must not rewrite that holder's lineage — see D-3a in
// plans/lease-succession.md, which scopes this fix to the unattended branch.
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
		holder, _ := view["holder"].(bool)
		if announcement, ok := view["announcement"].(*lease.Announcement); holder && ok && announcement != nil {
			return armingIdentity{
				session: announcement.SessionId,
				pid:     int(announcement.Pid),
				started: announcement.PidStartedAt,
				tag:     announcement.InstanceTag,
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
	if mode == "start" && fences["approvedContractSha256"] != nil {
		return failf(3, "mission start refused: approved contract is already pinned; use resume")
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
	if code != 0 || !strings.Contains(stdout, "ARMED") {
		return failf(3, "mission start refused: supervision did not arm: %s", firstDetail(stderr, stdout))
	}
	verified, err := os.CreateTemp("", "mission-"+e.Mission+"-verified.*.contract.md")
	if err != nil {
		return err
	}
	verifiedPath := verified.Name()
	verified.Close()
	defer os.Remove(verifiedPath)
	_, rawSHA, err := contract.Preflight(e.contractPath(), verifiedPath)
	if err != nil {
		return failf(3, "mission start refused by preflight: %v", err)
	}
	snapshot, err := os.ReadFile(verifiedPath)
	if err != nil {
		return failf(3, "mission start refused: verified contract snapshot is unreadable: %v", err)
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
	if mode == "start" && pathExists(statePath) {
		return failf(3, "mission state already exists; use resume")
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
		if state["status"] != "running" {
			return failf(3, "mission is %s; answer its park reason before resume", valueString(state["status"]))
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
	verifySeconds, err := ScaledSeconds(15)
	if err != nil {
		return err
	}
	graceSeconds, err := ScaledSeconds(5)
	if err != nil {
		return err
	}
	poll, err := Interval("METASYSTEM_HANDSHAKE_POLL_INTERVAL_MS", 50)
	if err != nil {
		return err
	}
	deadline := time.Now().Add(scaledDuration(verifySeconds))
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
			process.waitFor(scaledDuration(graceSeconds))
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
		if strings.Contains(processCommand(pid, false), tag) {
			if pgid, pgErr := unix.Getpgid(pid); pgErr == nil {
				if err := e.terminateGroup(pgid, tag, false); err != nil {
					return err
				}
			}
		}
	}
	return failf(3, "mission runner start verification timed out")
}
