package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

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
