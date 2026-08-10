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

// launchHost runs one host turn end to end and reports the adapter's exit
// code, the parsed host result (nil when unusable), and a launch detail for
// the turn log. A returned error is a runner defect, not a failed turn.
func (e *Engine) launchHost(turnID, turnDir string, turn map[string]any, leasePath, startSignal string, notified *bool) (int, map[string]any, string, error) {
	runtime, _ := turn["runtime"].(string)
	adapter := filepath.Join(e.Root, "scripts", "agents", "hosts", runtime+".sh")
	if info, err := os.Stat(adapter); err != nil || !info.Mode().IsRegular() || unix.Access(adapter, unix.X_OK) != nil {
		return 0, nil, "", failf(3, "host adapter is not installed or executable: %s", adapter)
	}
	prompt := filepath.Join(turnDir, "prompt.md")
	resultPath := filepath.Join(turnDir, "result.json")
	hostGate := filepath.Join(turnDir, "host.start")
	tag := "metasystem-host-" + turnID
	args := []string{
		"start-turn",
		"--mission", e.Mission,
		"--turn-id", turnID,
		"--prompt", prompt,
		"--result", resultPath,
		"--instance-tag", tag,
	}
	if session, ok := turn["hostSession"].(string); ok {
		args = append(args, "--resume-session", session)
	}
	gateTimeout, err := ScaledSeconds(10)
	if err != nil {
		return 0, nil, "", err
	}
	command := exec.Command(adapter, args...)
	command.Dir = e.Root
	command.Env = append(gitAuthorEnvironment(turnID),
		"METASYSTEM_MISSION_ID="+e.Mission,
		// Every TURN launches a fresh host process, which arms in the target
		// and becomes the lease holder under its own per-process mainId.
		// Without a shared lineage the next turn's host takes the lease from
		// its own dead predecessor and sweeps whatever delegates that turn
		// left in flight — the loop that cost bm-2 two of three delegates.
		// The host's arming inherits this, so every turn of one mission is
		// the same logical writer and succession renews instead.
		"METASYSTEM_OWNER_LINEAGE="+MissionLineage(e.Mission),
		"METASYSTEM_MISSION_LEASE="+leasePath,
		"METASYSTEM_MISSION_TURN="+turnID,
		"METASYSTEM_HOST_START_GATE="+hostGate,
		"METASYSTEM_HOST_START_GATE_TIMEOUT_SEC="+strconv.Itoa(gateTimeout),
	)
	hostLog, err := os.OpenFile(filepath.Join(turnDir, "host.log"), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return 0, nil, "", err
	}
	defer hostLog.Close()
	command.Stdout = hostLog
	command.Stderr = hostLog
	command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	process, err := startProcess(command)
	if err != nil {
		return 0, nil, "", err
	}
	pid := command.Process.Pid

	grace, err := ScaledSeconds(5)
	if err != nil {
		return 0, nil, "", err
	}
	handshakePoll, err := Interval("METASYSTEM_HANDSHAKE_POLL_INTERVAL_MS", 20)
	if err != nil {
		return 0, nil, "", err
	}
	fakeRuntime := runtime == "fake"
	forceUnverified := fakeRuntime && os.Getenv("METASYSTEM_FAKE_HOST_START_UNVERIFIED") == "1"
	deadline := time.Now().Add(time.Duration(grace) * time.Second)
	var started int64
	haveStarted := false
	verified := false
	for !time.Now().After(deadline) {
		if process.exited() {
			break
		}
		if at, err := processStartedAt(pid); err == nil {
			started, haveStarted = at, true
			published := true
			if fakeRuntime && publishFakeIdentity(pid, at, pid, tag) != nil {
				published = false
			}
			if published {
				pgid, pgErr := unix.Getpgid(pid)
				verified = pgErr == nil && hostStartVerified(pid, pgid, processCommand(pid, fakeRuntime), tag, forceUnverified)
			}
		}
		if verified {
			break
		}
		if err := e.heartbeat(turnID); err != nil {
			return 0, nil, "", err
		}
		time.Sleep(handshakePoll)
	}
	turnPath := filepath.Join(turnDir, "turn.json")
	if !verified || !haveStarted {
		if !process.exited() {
			if err := e.terminateGroup(pid, tag, fakeRuntime); err != nil {
				return 0, nil, "", err
			}
		}
		if !process.waitFor(scaledDuration(grace)) {
			return 0, nil, "", failf(3, "host process %d did not exit during start wind-down", pid)
		}
		if _, err := patchTurn(turnPath, map[string]any{
			"status": "failed", "outcome": "failed",
			"error": "start-unverified", "detail": "start-unverified", "endedAt": nowISO(),
		}); err != nil {
			return 0, nil, "", err
		}
		return 3, nil, "start-unverified", nil
	}
	if _, err := patchTurn(turnPath, map[string]any{
		"pid": pid, "pidStartedAt": started, "pgid": pid, "instanceTag": tag,
		"status": "running", "outcome": "running",
	}); err != nil {
		return 0, nil, "", err
	}
	if err := atomicWriteText(hostGate, "started\n"); err != nil {
		return 0, nil, "", err
	}
	if err := e.notifyStarted(startSignal, turnID, notified); err != nil {
		return 0, nil, "", err
	}

	capDuration, err := turnCapFromDoc(turn)
	if err != nil {
		return 0, nil, "", err
	}
	heartbeatInterval, err := Interval("METASYSTEM_HEARTBEAT_INTERVAL_MS", 100)
	if err != nil {
		return 0, nil, "", err
	}
	capped := false
	capDeadline := time.Now().Add(capDuration)
	for !process.exited() {
		if err := e.heartbeat(turnID); err != nil {
			return 0, nil, "", err
		}
		if !time.Now().Before(capDeadline) {
			if err := e.terminateGroup(pid, tag, fakeRuntime); err != nil {
				return 0, nil, "", err
			}
			capped = true
			break
		}
		process.waitFor(heartbeatInterval)
	}
	if !process.waitFor(scaledDuration(grace)) {
		if err := e.terminateGroup(pid, tag, fakeRuntime); err != nil {
			return 0, nil, "", err
		}
		if !process.waitFor(scaledDuration(grace)) {
			return 0, nil, "", failf(3, "host process %d did not exit during wind-down", pid)
		}
	}
	if capped {
		if _, err := patchTurn(turnPath, map[string]any{
			"status": "failed", "outcome": "capped",
			"error": "turn-cap", "detail": "host turn reached host.turn-cap-min", "endedAt": nowISO(),
		}); err != nil {
			return 0, nil, "", err
		}
		return 3, nil, "capped", nil
	}
	var result map[string]any
	if _, err := os.Stat(resultPath); err == nil {
		if doc, err := readDocLabeled(resultPath, "host result", 3); err == nil {
			result = doc
		}
	}
	detail := "host result received"
	if result == nil {
		detail = "host exited without a usable result"
	}
	return process.exitCode(), result, detail, nil
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
