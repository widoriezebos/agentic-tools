package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/census"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/supervise"
)

// runSuperviseDeriveCeiling relays `supervise derive-ceiling`: the
// watcher-ceiling derivation lives in supervise.DeriveCeiling.
// --max-cap spelling is validated here (exit 2, the usage refusal the
// arming script relies on); source refusals exit 1.
func runSuperviseDeriveCeiling(args []string) int {
	flags := flag.NewFlagSet("supervise derive-ceiling", flag.ContinueOnError)
	conf := flags.String("conf", "", "path to metasystem.conf")
	maxCap := flags.String("max-cap", "", "declared maximum cap in minutes (optional)")
	if flags.Parse(args) != nil {
		return 2
	}
	if *conf == "" {
		fmt.Fprintln(os.Stderr, "supervise derive-ceiling: --conf is required")
		return 2
	}
	declared := int64(0)
	if *maxCap != "" {
		n, err := strconv.ParseInt(*maxCap, 10, 64)
		if err != nil || n < 1 {
			fmt.Fprintln(os.Stderr, "--max-cap must be a positive integer")
			return 2
		}
		declared = n
	}
	ceiling, err := supervise.DeriveCeiling(*conf, declared, os.Environ())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Println(ceiling)
	return 0
}

// runSuperviseVerifyArmed relays `supervise verify-armed`: one arming
// attempt's verdict from supervise.ArmedNow.
// Exit 0 armed, 1 not yet; the arming script owns the retry loop and the
// timeout message.
func runSuperviseVerifyArmed(args []string) int {
	flags := flag.NewFlagSet("supervise verify-armed", flag.ContinueOnError)
	agents := flags.String("agents", "", "artifacts/agents directory")
	ownerPid := flags.Int64("owner-pid", 0, "owner pid")
	ownerStart := flags.Int64("owner-start", 0, "owner start epoch seconds")
	ownerTag := flags.String("owner-tag", "", "owner instance tag")
	interval := flags.Int64("interval", 60, "census interval seconds")
	if flags.Parse(args) != nil {
		return 2
	}
	if *agents == "" || *ownerPid < 1 || *ownerStart < 1 {
		fmt.Fprintln(os.Stderr, "supervise verify-armed: --agents, --owner-pid, and --owner-start are required")
		return 2
	}
	if supervise.ArmedNow(*agents, *ownerPid, *ownerStart, *ownerTag, *interval, time.Now()) {
		return 0
	}
	return 1
}

// runCensusFingerprint prints the supervision fingerprint for --repo, using
// --root as the metasystem root (defaults to the binary's checkout). It lives
// here because it registers under the supervise family.
func runCensusFingerprint(args []string) int {
	flags := flag.NewFlagSet("supervise fingerprint", flag.ContinueOnError)
	repo := flags.String("repo", "", "checkout root to fingerprint")
	root := flags.String("root", "", "metasystem root (defaults to this checkout)")
	if flags.Parse(args) != nil {
		return 2
	}
	if *repo == "" {
		fmt.Fprintln(os.Stderr, "supervise fingerprint: --repo is required")
		return 2
	}
	metasystemRoot := *root
	if metasystemRoot == "" {
		if exe, err := os.Executable(); err == nil {
			// <root>/bin/metasystem is two deep: Dir^2, not Dir^3 — a
			// third Dir points the default at the checkout's parent.
			metasystemRoot = filepath.Dir(filepath.Dir(exe))
		}
	}
	fp, err := census.Fingerprint(metasystemRoot, *repo)
	if err != nil {
		fmt.Fprintln(os.Stderr, "supervise fingerprint:", err)
		return 1
	}
	fmt.Println(fp)
	return 0
}

// runSuperviseStatus reads a checkout's supervision surface — lock,
// state, heartbeats — and prints one JSON object with three-way
// verdicts. Diagnostic surface for fixtures and humans; the REAL
// owner loop lands with the process adapters (Phase 0 continues).
func runSuperviseStatus(args []string) int {
	flags := flag.NewFlagSet("supervise status", flag.ContinueOnError)
	repo := flags.String("repo", "", "checkout root")
	if flags.Parse(args) != nil {
		return 2
	}
	if *repo == "" {
		fmt.Fprintln(os.Stderr, "supervise status: --repo is required")
		return 2
	}
	supervision := filepath.Join(*repo, "artifacts", "agents", "supervision")
	result := map[string]any{"repo": *repo}

	owner := map[string]any{}
	ownerPath := filepath.Join(supervision, "lock.d", "owner.json")
	if content, err := os.ReadFile(ownerPath); err == nil {
		var record struct {
			Pid          int64  `json:"pid"`
			PidStartedAt int64  `json:"pidStartedAt"`
			InstanceTag  string `json:"instanceTag"`
		}
		if json.Unmarshal(content, &record) == nil {
			owner["pid"] = record.Pid
			owner["instanceTag"] = record.InstanceTag
			owner["liveness"] = identity.AliveRef(identity.KernelProber{},
				identity.Ref{Pid: record.Pid, StartedAtSec: record.PidStartedAt}).String()
		} else {
			owner["error"] = "owner file unparseable (uninspectable is alive)"
		}
	} else if os.IsNotExist(err) {
		owner["state"] = "absent"
	} else {
		owner["error"] = err.Error()
	}
	result["owner"] = owner

	if _, err := os.Stat(filepath.Join(supervision, "state.json")); err == nil {
		result["stateFile"] = "present"
	} else if os.IsNotExist(err) {
		result["stateFile"] = "absent"
	} else {
		result["stateFile"] = "indeterminate"
	}

	encoded, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "supervise status:", err)
		return 1
	}
	fmt.Println(string(encoded))
	return 0
}
