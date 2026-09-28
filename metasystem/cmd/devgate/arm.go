package main

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/shellquote"
	"golang.org/x/sys/unix"
)

// armOptions is the witness-producing controller the sourced
// witness-gate.sh once was. The caller names itself as the controller
// (its $$), and receives the state to adopt in a file it sources.
type armOptions struct {
	requested  bool
	fallback   string
	controller int64
	stateOut   string
	delivery   bool
}

// armState is what the controller hands back: the witness state directory
// the caller removes at its exit, whether an inherited ENGINE witness was
// reused, and the environment the caller exports or scrubs for its nested
// validations.
type armState struct {
	witnessState  string
	engineReused  bool
	exports       map[string]string
	unsets        []string
	stateOut      string
	written       bool
	writeFailures io.Writer
}

// runArm is the witness-producing gate: every battery stage shares ONE
// proof of the tree instead of re-running the race suite per nested
// validation. The fallback is an explicit choice: "plain" runs the ordinary
// full gate when the witness is ineligible before a gate starts (an executed
// gate is never retried); "none" arms nothing and runs nothing, for callers
// whose nested validations carry their own gates. A witness that existed at
// entry is consumed or refused, never replaced in place.
func runArm(ctx context.Context, root string, d deps, options armOptions) int {
	if options.fallback != "plain" && options.fallback != "none" {
		got := options.fallback
		if got == "" {
			got = "unset"
		}
		fmt.Fprintf(d.stderr, "witness-gate refused: --arm must be plain or none (got '%s')\n", got)
		return 1
	}
	if options.controller < 1 || options.stateOut == "" {
		fmt.Fprintln(d.stderr, "witness-gate refused: --arm needs --controller-pid PID and --state-out FILE")
		return 2
	}
	env := newEnvironment(d.environ())
	state := &armState{exports: map[string]string{}, stateOut: options.stateOut, writeFailures: d.stderr}
	status := armWitness(ctx, root, env, d, options, state)
	if !state.written && !state.write() {
		return 1
	}
	return status
}

func armWitness(ctx context.Context, root string, env *environment, d deps, options armOptions, state *armState) int {
	plain := func() int {
		if options.fallback != "plain" {
			return 0
		}
		// The plain gate may relaunch under its proof owner; the caller's
		// state is settled before it starts.
		if !state.write() {
			return 1
		}
		return runGate(ctx, root, env.clone(), d, gateOptions{})
	}

	if env.get("METASYSTEM_GATE_WITNESS") != "" {
		reused, status, canRun := false, 0, true
		if options.fallback == "none" {
			// A no-fallback caller probes first: a refusal must run nothing.
			// Plain fallback skips the probe so one consumer freeze can
			// either reuse the witness or perform the complete gate from
			// that same export.
			probe := d
			probe.stdout, probe.stderr = io.Discard, io.Discard
			if runGate(ctx, root, env.with("METASYSTEM_GATE_WITNESS_CONSUMER_SCOPE=ENGINE"), probe, gateOptions{witnessCheckOnly: true}) != 0 {
				canRun = false
			}
		}
		if canRun {
			consumer := env.with("METASYSTEM_GATE_WITNESS_CONSUMER_SCOPE=ENGINE")
			marker, err := os.CreateTemp(tempDir(env), "tmp.")
			if err == nil {
				_ = marker.Close()
				consumer.set("METASYSTEM_GATE_WITNESS_REUSE_OUT", marker.Name())
			}
			status = runGate(ctx, root, consumer, d, gateOptions{})
			if err == nil {
				data, _ := os.ReadFile(marker.Name())
				reused = status == 0 && len(data) > 0
				_ = os.Remove(marker.Name())
			}
		}
		state.engineReused = reused
		if !reused {
			// A refused inherited witness cannot become acceptable later in
			// this root and silently change its run class.
			state.unsets = append(state.unsets, "METASYSTEM_GATE_WITNESS", "METASYSTEM_GATE_WITNESS_ROOT",
				"METASYSTEM_GATE_WITNESS_RUN", "METASYSTEM_GATE_WITNESS_EXPORT")
		}
		return status
	}

	eligible := !options.delivery && env.get("METASYSTEM_COVERAGE_RATCHET_SEED") != "1" && env.get("METASYSTEM_GATE_FORCE") != "1"
	if !eligible {
		return plain()
	}
	if frozenToolchainRefusal(env.get("GOFLAGS")) != "" {
		fmt.Fprintln(d.stderr, "witness gate arming voided: GOFLAGS may not contain -modfile or -overlay")
		return 1
	}

	prepared := true
	frozen, err := d.owners.freeze(root)
	if err != nil {
		fmt.Fprintln(d.stderr, "gate witness-freeze:", err)
		prepared = false
	} else if !hex64.MatchString(frozen.Digest) || !isDir(frozen.Root) || !isDir(frozen.SnapshotRoot) {
		if frozen.SnapshotRoot != "" {
			_ = d.owners.cleanupFreeze(frozen.SnapshotRoot)
		}
		prepared = false
	}
	if prepared {
		if dir, err := os.MkdirTemp(tempDir(env), "tmp."); err != nil || os.Chmod(dir, 0o700) != nil {
			prepared = false
		} else {
			state.witnessState = dir
		}
	}
	run := "run-" + strconv.FormatInt(d.selfPid, 10) + "-" + strconv.Itoa(randomRun())
	var controller identity.Ref
	if prepared {
		controller, err = d.owners.processPair(options.controller)
		if err != nil || !controllerOwnsThisRun(d, options.controller, controller) {
			fmt.Fprintf(d.stderr, "witness gate controller %d is not this gate's live ancestor in its session\n", options.controller)
			prepared = false
		}
	}
	if !prepared {
		fmt.Fprintln(d.stderr, "witness gate preparation did not complete")
		if state.witnessState != "" {
			_ = os.RemoveAll(state.witnessState)
			state.witnessState = ""
		}
		if frozen.SnapshotRoot != "" && isDir(frozen.SnapshotRoot) {
			_ = d.owners.cleanupFreeze(frozen.SnapshotRoot)
		}
		return plain()
	}

	executionRoot := env.get("METASYSTEM_PROOF_EXECUTION_ROOT")
	if executionRoot == "" {
		executionRoot = root
	}
	witnessPath := filepath.Join(state.witnessState, "witness.json")
	snapshot := env.with("GOFLAGS="+ownedGoFlags, "METASYSTEM_GATE_FROZEN_TOOLCHAIN=1",
		"METASYSTEM_PROOF_EXECUTION_ROOT="+executionRoot,
		"METASYSTEM_GATE_WITNESS_WRITE="+witnessPath,
		"METASYSTEM_GATE_WITNESS_RUN="+run,
		"METASYSTEM_GATE_WITNESS_MANIFEST_DIGEST="+frozen.Digest,
		"METASYSTEM_GATE_WITNESS_CONTROLLER_PID="+strconv.FormatInt(controller.Pid, 10),
		"METASYSTEM_GATE_WITNESS_CONTROLLER_STARTED_AT="+strconv.FormatInt(controller.StartedAtSec, 10),
		"METASYSTEM_GATE_WITNESS_CONTROLLER_START_TICKS="+strconv.FormatInt(controller.StartTicks, 10),
		"METASYSTEM_GATE_WITNESS_CONTROLLER_BOOT_ID="+controller.BootID)
	exportRoot, err := filepath.EvalSymlinks(frozen.Root)
	if err != nil {
		exportRoot = frozen.Root
	}
	status := runGate(ctx, exportRoot, snapshot, d, gateOptions{})
	release := func() {
		if isDir(frozen.SnapshotRoot) {
			_ = d.owners.cleanupFreeze(frozen.SnapshotRoot)
		}
	}
	dropState := func() {
		_ = os.RemoveAll(state.witnessState)
		state.witnessState = ""
	}
	if status != 0 {
		dropState()
		release()
		if status == 3 {
			fmt.Fprintln(d.stderr, "witness gate refused in its frozen project (reason above); falling back to the plain gate")
			if options.fallback == "plain" {
				return plain()
			}
			return status
		}
		fmt.Fprintf(d.stderr, "witness gate failed in its frozen project (exit %d): the red above is the answer, no fallback\n", status)
		return status
	}
	if info, err := os.Stat(witnessPath); err != nil || !info.Mode().IsRegular() {
		fmt.Fprintln(d.stderr, "witness gate completed without publishing witness evidence")
		dropState()
		release()
		return 1
	}
	if err := installBinary(filepath.Join(exportRoot, "bin", "metasystem"), root, d.selfPid); err != nil {
		fmt.Fprintln(d.stderr, "witness gate completed but the proven binary could not be published")
		dropState()
		release()
		return 1
	}
	if err := d.owners.cleanupFreeze(frozen.SnapshotRoot); err != nil {
		fmt.Fprintln(d.stderr, "gate witness-freeze:", err)
		fmt.Fprintln(d.stderr, "witness gate completed but its frozen project could not be released")
		dropState()
		return 1
	}
	state.exports["METASYSTEM_GATE_WITNESS"] = witnessPath
	state.exports["METASYSTEM_GATE_WITNESS_ROOT"] = state.witnessState
	state.exports["METASYSTEM_GATE_WITNESS_RUN"] = run
	state.unsets = append(state.unsets, "METASYSTEM_GATE_WITNESS_EXPORT")
	fmt.Fprintln(d.stdout, "gate witness armed from complete frozen project")
	fmt.Fprintln(d.stdout, "gate witness armed for this run's nested validations")
	return 0
}

// controllerOwnsThisRun: the named controller is this process's live
// ancestor in its own session. The sourced script recorded its own shell;
// a caller naming itself cannot reach outside its session, so no unrelated
// long-lived process (a login manager, init) can be made the ancestry every
// consumer on the machine descends from.
func controllerOwnsThisRun(d deps, pid int64, controller identity.Ref) bool {
	if pid <= 1 || d.owners.descendant(d.selfPid, controller) != nil {
		return false
	}
	own, err := unix.Getsid(int(d.selfPid))
	if err != nil {
		return false
	}
	theirs, err := unix.Getsid(int(pid))
	return err == nil && own == theirs
}

func randomRun() int {
	var buffer [2]byte
	if _, err := rand.Read(buffer[:]); err != nil {
		return 0
	}
	return int(binary.BigEndian.Uint16(buffer[:]) & 0x7fff)
}

// write publishes the state as a file the caller sources: shell
// assignments with single-quoted values, and the exports and unsets the
// sourced wrapper once performed in its caller.
func (s *armState) write() bool {
	var out strings.Builder
	fmt.Fprintf(&out, "witness_state=%s\n", shellquote.Quote(s.witnessState))
	reused := "0"
	if s.engineReused {
		reused = "1"
	}
	fmt.Fprintf(&out, "witness_engine_reused=%s\n", reused)
	names := make([]string, 0, len(s.exports))
	for name := range s.exports {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		fmt.Fprintf(&out, "export %s=%s\n", name, shellquote.Quote(s.exports[name]))
	}
	if len(s.unsets) > 0 {
		fmt.Fprintf(&out, "unset %s\n", strings.Join(s.unsets, " "))
	}
	if err := os.WriteFile(s.stateOut, []byte(out.String()), 0o600); err != nil {
		fmt.Fprintf(s.writeFailures, "witness gate: cannot write its state for the caller: %v\n", err)
		return false
	}
	s.written = true
	return true
}
