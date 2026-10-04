package dispatch

import (
	"os"
	"path/filepath"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
)

// WidenForCodexSandbox is a Codex job's requested envelope on a host that runs
// Codex unsandboxed: the network allowed, the host's whole file system
// readable and writable, approvals and tools as requested, and widenedBy
// naming the setting. The record then says the job is uncontained, which it
// is, instead of claiming a boundary nothing enforces; the effective envelope
// materialized from it agrees with it, so the widening check still refuses
// every other widening.
func WidenForCodexSandbox(requested map[string]any) map[string]any {
	widened := make(map[string]any, len(requested)+1)
	for key, value := range requested {
		widened[key] = value
	}
	widened["network"] = "allow"
	widened["readRoots"] = []any{"/"}
	widened["writeRoots"] = []any{"/"}
	widened["widenedBy"] = config.CodexSandboxWidening
	return widened
}

// admittedRequest is the requested envelope a fresh job is admitted with: a
// Codex job on a host whose launch.codex.sandbox is danger-full-access gets
// the widened envelope, and every other job its request unchanged. A
// checkout without a metasystem.conf reads the compiled default.
func admittedRequest(lookupEnv func(string) (string, bool), root, runtime string, requested map[string]any) (map[string]any, error) {
	if runtime != "codex" {
		return requested, nil
	}
	confPath := ""
	if root != "" {
		if info, err := os.Stat(filepath.Join(root, "metasystem.conf")); err == nil && info.Mode().IsRegular() {
			confPath = filepath.Join(root, "metasystem.conf")
		}
	}
	mode, err := config.CodexSandbox(confPath, lookupEnv)
	if err != nil {
		return nil, err
	}
	if mode != config.CodexSandboxFullAccess {
		return requested, nil
	}
	return WidenForCodexSandbox(requested), nil
}
