package missionrunner

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

// Launching one host turn: spawn the runtime adapter in its own session,
// prove the start (the process leads its own group and carries the minted
// instance tag), release the host's start gate, heartbeat while it runs,
// enforce the turn cap, and wind the group down without ever signaling a
// group that is not provably ours.

// hostProcess tracks a started child and its single Wait.
type hostProcess struct {
	cmd  *exec.Cmd
	done chan struct{}
	err  error
}

// startProcess starts a command and begins its one Wait in the background.
func startProcess(cmd *exec.Cmd) (*hostProcess, error) {
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	process := &hostProcess{cmd: cmd, done: make(chan struct{})}
	go func() {
		process.err = cmd.Wait()
		close(process.done)
	}()
	return process, nil
}

// exited reports whether the child has already been reaped.
func (p *hostProcess) exited() bool {
	select {
	case <-p.done:
		return true
	default:
		return false
	}
}

// waitFor waits up to the given duration for the child to exit.
func (p *hostProcess) waitFor(limit time.Duration) bool {
	select {
	case <-p.done:
		return true
	case <-time.After(limit):
		return false
	}
}

// exitCode is the child's exit code once it has been reaped; a child ended by
// a signal reads as -1, which every caller treats as a plain failure.
func (p *hostProcess) exitCode() int {
	if p.cmd.ProcessState == nil {
		return -1
	}
	return p.cmd.ProcessState.ExitCode()
}

// hostStartVerified is the start-gate ownership proof: the host leads its own
// process group and its command line carries the instance tag this runner
// minted. A fixture may force the unverified path to exercise the refusal.
func hostStartVerified(pid, pgid int, command, tag string, forceUnverified bool) bool {
	return !forceUnverified && pgid == pid && strings.Contains(command, tag)
}

// terminateGroup is the best-effort wind-down of a host group this runner
// launched.
//
// Ownership is proven by the tag on a live member. When the proof is gone —
// the tagged host exited and only untagged children linger, or the pgid was
// recycled — the group is NOT ours to signal, and that is a normal way for a
// turn to end, not a mission-fatal error: the thing we launched is no longer
// running. Failing here killed a whole mission at the moment its host
// finished (the runner died with "lost ownership proof" seconds after the
// turn result landed, and the driver then polled a dead mission for hours).
// We never signal without proof; we also never die over a group that already
// stopped being ours. Anything genuinely left behind is UNTRACKED to the
// census, which is the safety net designed to catch it.
func (e *Engine) terminateGroup(pgid int, tag string, allowFake bool) error {
	if !groupAlive(pgid) {
		return nil
	}
	if !groupOwned(pgid, tag, allowFake) {
		fmt.Fprintf(os.Stderr, "host process group %d is no longer provably ours; "+
			"leaving it to the census rather than signaling an unowned group\n", pgid)
		e.emit("wind-down", fmt.Sprintf("group %d unowned; skipped", pgid), map[string]string{
			"missionId": e.Mission, "action": "skipped-unowned", "reason": "ownership-proof-absent",
		})
		return nil
	}
	e.emit("wind-down", fmt.Sprintf("group %d", pgid), map[string]string{
		"missionId": e.Mission, "action": "sigterm",
	})
	_ = unix.Kill(-pgid, syscall.SIGTERM)
	grace, err := ScaledSeconds(5)
	if err != nil {
		return err
	}
	pollInterval, err := Interval("METASYSTEM_HEARTBEAT_INTERVAL_MS", 50)
	if err != nil {
		return err
	}
	deadline := time.Now().Add(time.Duration(grace) * time.Second)
	for groupAlive(pgid) && time.Now().Before(deadline) {
		time.Sleep(pollInterval)
	}
	if groupAlive(pgid) {
		if !groupOwned(pgid, tag, allowFake) {
			fmt.Fprintf(os.Stderr, "ownership proof for host process group %d disappeared "+
				"during wind-down; skipping the kill of an unowned group\n", pgid)
			return nil
		}
		_ = unix.Kill(-pgid, syscall.SIGKILL)
	}
	return nil
}

// patchTurn merges fields into a turn record on disk and returns the result.
func patchTurn(path string, fields map[string]any) (map[string]any, error) {
	turn, err := readDocLabeled(path, "turn record", 3)
	if err != nil {
		return nil, err
	}
	for key, value := range fields {
		turn[key] = value
	}
	if err := atomicWriteJSON(path, turn); err != nil {
		return nil, err
	}
	return turn, nil
}

// writeStartSignal reports the runner's launch verdict to the process that
// spawned it: verified with the first turn's id, or refused with an error.
func writeStartSignal(path string, verified bool, turnID, errMsg any) error {
	return atomicWriteJSON(path, map[string]any{"verified": verified, "turnId": turnID, "error": errMsg})
}

// notifyStarted releases the start-signal handshake exactly once, on the
// first verified host start.
func (e *Engine) notifyStarted(startSignal, turnID string, notified *bool) error {
	if *notified {
		return nil
	}
	if err := writeStartSignal(startSignal, true, turnID, nil); err != nil {
		return err
	}
	*notified = true
	return nil
}

// A host turn in three named steps (Phase 3b): assemble the adapter
// command, spawn and verify the start handshake, then supervise to exit
// under the turn cap and record what came back. hostLaunch is the shared
// context; launchHost is only the sequence.
type hostLaunch struct {
	turnID, turnDir string
	turn            map[string]any
	leasePath       string
	startSignal     string
	notified        *bool

	runtime, tag         string
	resultPath, turnPath string
	hostGate             string
	fakeRuntime          bool
	grace                int
	command              *exec.Cmd
	process              *hostProcess
	pid                  int
}

// launchHost runs one host turn end to end and reports the adapter's exit
// code, the parsed host result (nil when unusable), and a launch detail for
// the turn log. A returned error is a runner defect, not a failed turn.
func (e *Engine) launchHost(turnID, turnDir string, turn map[string]any, leasePath, startSignal string, notified *bool) (int, map[string]any, string, error) {
	l := &hostLaunch{turnID: turnID, turnDir: turnDir, turn: turn,
		leasePath: leasePath, startSignal: startSignal, notified: notified}
	if err := e.assembleHostCommand(l); err != nil {
		return 0, nil, "", err
	}
	if code, detail, done, err := e.spawnAndVerifyHost(l); err != nil || done {
		return code, nil, detail, err
	}
	return e.superviseHostToExit(l)
}

// assembleHostCommand resolves the adapter, builds its argument list and
// environment, and opens the host log; nothing has started yet.
func (e *Engine) assembleHostCommand(l *hostLaunch) error {
	l.runtime = TurnRecordOf(l.turn).Runtime()
	adapter := filepath.Join(e.Root, "scripts", "agents", "hosts", l.runtime+".sh")
	if info, err := os.Stat(adapter); err != nil || !info.Mode().IsRegular() || unix.Access(adapter, unix.X_OK) != nil {
		return failf(3, "host adapter is not installed or executable: %s", adapter)
	}
	prompt := filepath.Join(l.turnDir, "prompt.md")
	l.resultPath = filepath.Join(l.turnDir, "result.json")
	l.turnPath = filepath.Join(l.turnDir, "turn.json")
	l.hostGate = filepath.Join(l.turnDir, "host.start")
	l.tag = "metasystem-host-" + l.turnID
	l.fakeRuntime = l.runtime == "fake"
	args := []string{
		"start-turn",
		"--mission", e.Mission,
		"--turn-id", l.turnID,
		"--prompt", prompt,
		"--result", l.resultPath,
		"--instance-tag", l.tag,
	}
	// The raw assertion, not the lens: the lens reads an empty-string
	// session as absent, but the original contract passes --resume-session
	// verbatim whenever the field is a string AT ALL — and a conversion
	// commit may not narrow that, even where narrowing looks saner
	// (typed-documents rule: the projection is a lens, never a filter).
	if session, ok := l.turn["hostSession"].(string); ok {
		args = append(args, "--resume-session", session)
	}
	gateTimeout, err := ScaledSeconds(10)
	if err != nil {
		return err
	}
	command := exec.Command(adapter, args...)
	command.Dir = e.Root
	command.Env = append(gitAuthorEnvironment(l.turnID),
		"METASYSTEM_MISSION_ID="+e.Mission,
		// Every TURN launches a fresh host process, which arms in the target
		// and becomes the lease holder under its own per-process mainId.
		// Without a shared lineage the next turn's host takes the lease from
		// its own dead predecessor and sweeps whatever delegates that turn
		// left in flight — the loop that cost bm-2 two of three delegates.
		// The host's arming inherits this, so every turn of one mission is
		// the same logical writer and succession renews instead.
		"METASYSTEM_OWNER_LINEAGE="+MissionLineage(e.Mission),
		"METASYSTEM_MISSION_LEASE="+l.leasePath,
		"METASYSTEM_MISSION_TURN="+l.turnID,
		"METASYSTEM_HOST_START_GATE="+l.hostGate,
		"METASYSTEM_HOST_START_GATE_TIMEOUT_SEC="+strconv.Itoa(gateTimeout),
	)
	hostLog, err := os.OpenFile(filepath.Join(l.turnDir, "host.log"), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	command.Stdout = hostLog
	command.Stderr = hostLog
	command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	l.command = command
	return nil
}

// spawnAndVerifyHost starts the adapter and holds it to the start
// handshake: the host must lead its own group and carry the minted tag
// within the grace window, or the start is wound down as unverified. On a
// verified start the turn record goes running, the start gate opens, and
// the start signal fires. done=true means the turn already settled
// (start-unverified) and carries its exit code and detail.
func (e *Engine) spawnAndVerifyHost(l *hostLaunch) (code int, detail string, done bool, err error) {
	defer func() {
		if closer, ok := l.command.Stdout.(*os.File); ok && (done || err != nil) {
			closer.Close()
		}
	}()
	grace, err := ScaledSeconds(5)
	if err != nil {
		return 0, "", false, err
	}
	l.grace = grace
	handshakePoll, err := Interval("METASYSTEM_HANDSHAKE_POLL_INTERVAL_MS", 20)
	if err != nil {
		return 0, "", false, err
	}
	process, err := startProcess(l.command)
	if err != nil {
		return 0, "", false, err
	}
	l.process = process
	l.pid = l.command.Process.Pid
	forceUnverified := l.fakeRuntime && os.Getenv("METASYSTEM_FAKE_HOST_START_UNVERIFIED") == "1"
	deadline := time.Now().Add(time.Duration(grace) * time.Second)
	var started int64
	haveStarted := false
	verified := false
	for !time.Now().After(deadline) {
		if process.exited() {
			break
		}
		if at, err := processStartedAt(l.pid); err == nil {
			started, haveStarted = at, true
			published := true
			if l.fakeRuntime && publishFakeIdentity(l.pid, at, l.pid, l.tag) != nil {
				published = false
			}
			if published {
				pgid, pgErr := unix.Getpgid(l.pid)
				verified = pgErr == nil && hostStartVerified(l.pid, pgid, processCommand(l.pid, l.fakeRuntime), l.tag, forceUnverified)
			}
		}
		if verified {
			break
		}
		if err := e.heartbeat(l.turnID); err != nil {
			return 0, "", false, err
		}
		time.Sleep(handshakePoll)
	}
	if !verified || !haveStarted {
		if !process.exited() {
			if err := e.terminateGroup(l.pid, l.tag, l.fakeRuntime); err != nil {
				return 0, "", false, err
			}
		}
		if !process.waitFor(scaledDuration(grace)) {
			return 0, "", false, failf(3, "host process %d did not exit during start wind-down", l.pid)
		}
		if _, err := patchTurn(l.turnPath, map[string]any{
			"status": "failed", "outcome": "failed",
			"error": "start-unverified", "detail": "start-unverified", "endedAt": nowISO(),
		}); err != nil {
			return 0, "", false, err
		}
		return 3, "start-unverified", true, nil
	}
	if _, err := patchTurn(l.turnPath, map[string]any{
		"pid": l.pid, "pidStartedAt": started, "pgid": l.pid, "instanceTag": l.tag,
		"status": "running", "outcome": "running",
	}); err != nil {
		return 0, "", false, err
	}
	if err := atomicWriteText(l.hostGate, "started\n"); err != nil {
		return 0, "", false, err
	}
	if err := e.notifyStarted(l.startSignal, l.turnID, l.notified); err != nil {
		return 0, "", false, err
	}
	return 0, "", false, nil
}

// superviseHostToExit heartbeats the running host under the turn cap,
// winds the group down when the cap fires, and reads back the result.
func (e *Engine) superviseHostToExit(l *hostLaunch) (int, map[string]any, string, error) {
	defer func() {
		if closer, ok := l.command.Stdout.(*os.File); ok {
			closer.Close()
		}
	}()
	capDuration, err := turnCapFromDoc(l.turn)
	if err != nil {
		return 0, nil, "", err
	}
	heartbeatInterval, err := Interval("METASYSTEM_HEARTBEAT_INTERVAL_MS", 100)
	if err != nil {
		return 0, nil, "", err
	}
	capped := false
	capDeadline := time.Now().Add(capDuration)
	for !l.process.exited() {
		if err := e.heartbeat(l.turnID); err != nil {
			return 0, nil, "", err
		}
		if !time.Now().Before(capDeadline) {
			if err := e.terminateGroup(l.pid, l.tag, l.fakeRuntime); err != nil {
				return 0, nil, "", err
			}
			capped = true
			break
		}
		l.process.waitFor(heartbeatInterval)
	}
	if !l.process.waitFor(scaledDuration(l.grace)) {
		if err := e.terminateGroup(l.pid, l.tag, l.fakeRuntime); err != nil {
			return 0, nil, "", err
		}
		if !l.process.waitFor(scaledDuration(l.grace)) {
			return 0, nil, "", failf(3, "host process %d did not exit during wind-down", l.pid)
		}
	}
	if capped {
		if _, err := patchTurn(l.turnPath, map[string]any{
			"status": "failed", "outcome": "capped",
			"error": "turn-cap", "detail": "host turn reached host.turn-cap-min",
			"endedAt": nowISO(), "hostEndedAt": nowISO(),
		}); err != nil {
			return 0, nil, "", err
		}
		return 3, nil, "capped", nil
	}
	// The host PROCESS boundary is here, and it is the honest end of the
	// host's wall clock (decision D13): the turn's endedAt lands only
	// after adjudication, drain, ledger, and state — bookkeeping and
	// legitimately slow drains that twice tripped the benchmark's cap
	// gate on turns whose hosts finished inside their cap.
	if _, err := patchTurn(l.turnPath, map[string]any{"hostEndedAt": nowISO()}); err != nil {
		return 0, nil, "", err
	}
	var result map[string]any
	if _, err := os.Stat(l.resultPath); err == nil {
		if doc, err := readDocLabeled(l.resultPath, "host result", 3); err == nil {
			result = doc
		}
	}
	detail := "host result received"
	if result == nil {
		detail = "host exited without a usable result"
	}
	return l.process.exitCode(), result, detail, nil
}

// scaledDuration renders a scaled-seconds allowance as a duration.
func scaledDuration(seconds int) time.Duration {
	return time.Duration(seconds) * time.Second
}

// gitAuthorEnvironment is the process environment with the git author pinned
// to a mission identity, so anchors and host commits carry who acted.
func gitAuthorEnvironment(identityName string) []string {
	return append(os.Environ(),
		"GIT_AUTHOR_NAME="+identityName,
		"GIT_AUTHOR_EMAIL="+identityName+"@metasystem.invalid",
	)
}
