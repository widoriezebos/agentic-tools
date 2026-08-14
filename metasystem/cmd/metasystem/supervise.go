package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/census"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/supervise"
)

// runSuperviseDeriveCeiling relays `supervise derive-ceiling`: the
// watcher-ceiling derivation lives in supervise.DeriveCeiling
// (script-orchestration-04). --max-cap spelling is validated here (exit 2,
// the arming script's historical usage refusal); source refusals exit 1.
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

// runCensusFingerprint prints the supervision fingerprint for --repo, using
// --root as the metasystem root (defaults to the binary's checkout). It lives
// here because it registers under the supervise family (cli-10).
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
			// <root>/bin/metasystem is two deep: Dir^2, not Dir^3 (cli-4;
			// the third Dir pointed the default at the checkout's parent).
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
