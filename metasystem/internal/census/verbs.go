package census

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// The remaining small census verbs, completing the surface that mirrors
// process-census.py: alive (liveness), authentication-identity (start time +
// command from one source), and signature-check (adapter positive/lookalike
// contract).

// Alive ports the `alive` verb: true iff the pid is live at expectedStart.
func Alive(pid, expectedStart int64) bool {
	return identityAlive(pid, expectedStart)
}

// AuthIdentity ports authentication_identity: start time and command from ONE
// source — the fixture identity file when installed (its `started` and
// `command`), else the process table (ps lstart + command). This one-source
// rule is why a main can recognize its own announcement.
func AuthIdentity(pid int64) (map[string]any, error) {
	if fixture := os.Getenv("METASYSTEM_FAKE_PROCESS_IDENTITY_FILE"); fixture != "" {
		if data, err := os.ReadFile(fixture); err == nil {
			var table map[string]struct {
				Started *int64  `json:"started"`
				Command *string `json:"command"`
			}
			if json.Unmarshal(data, &table) == nil {
				if entry, ok := table[fmt.Sprint(pid)]; ok && entry.Started != nil && entry.Command != nil && *entry.Command != "" {
					return map[string]any{"pid": pid, "pidStartedAt": *entry.Started, "command": *entry.Command}, nil
				}
			}
		}
	}
	return psIdentity(pid)
}

func psIdentity(pid int64) (map[string]any, error) {
	// Native: the kernel prober gives the start time (whole seconds) and argv.
	exact, state, err := identity.KernelProber{}.Probe(pid)
	if err != nil || state != identity.Alive {
		return nil, fmt.Errorf("no such process: %d", pid)
	}
	command := strings.Join(exact.Argv, " ")
	if command == "" {
		return nil, fmt.Errorf("unreadable command for pid %d", pid)
	}
	return map[string]any{"pid": pid, "pidStartedAt": exact.StartedAt.Unix(), "command": command}, nil
}

// SignatureCheck ports the `signature-check` verb: the positive argv must
// classify as the adapter's runtime and the lookalike must NOT — the
// adapters' self-test that their signatures are neither too loose nor too
// tight (KI-14). Returns an error when the contract fails.
func SignatureCheck(adapterPath, positive, lookalike string) error {
	text, err := SignatureText(adapterPath)
	if err != nil {
		return err
	}
	matches, excludes := parseSignatureText(text)
	sig, err := CompileSignature("check", matches, excludes)
	if err != nil {
		return err
	}
	positiveOK := sig.matches(positive)
	lookalikeOK := sig.matches(lookalike)
	if !positiveOK || lookalikeOK {
		return fmt.Errorf("signature positive/lookalike contract failed for %s", adapterBase(adapterPath))
	}
	return nil
}

func adapterBase(path string) string {
	if i := strings.LastIndexByte(path, '/'); i >= 0 {
		return path[i+1:]
	}
	return path
}
