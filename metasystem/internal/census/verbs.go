package census

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// The remaining small census verbs, completing the surface that mirrors
// process-census.py: alive (liveness), authentication-identity (start time +
// command from one source), and signature-check (adapter positive/lookalike
// contract).

// Alive ports the `alive` verb: true iff the pid is live at expectedStart.
func Alive(pid, expectedStart int64) bool {
	return identityAlive(pid, expectedStart)
}

var psIdentityLine = regexp.MustCompile(
	`(?s)\s*([A-Z][a-z]{2}\s+[A-Z][a-z]{2}\s+[0-9]{1,2}\s+[0-9]{2}:[0-9]{2}:[0-9]{2}\s+[0-9]{4})\s+(.+?)\s*$`)

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
	cmd := exec.Command("ps", "-p", fmt.Sprint(pid), "-o", "lstart=,command=")
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	out, err := cmd.Output()
	if err != nil || strings.TrimSpace(string(out)) == "" {
		return nil, fmt.Errorf("no such process: %d", pid)
	}
	m := psIdentityLine.FindStringSubmatch(string(out))
	if m == nil {
		return nil, fmt.Errorf("unreadable process identity for pid %d", pid)
	}
	parsed, err := time.ParseInLocation("Mon Jan 2 15:04:05 2006", normalizeLstart(m[1]), time.Now().Location())
	if err != nil {
		return nil, fmt.Errorf("unreadable start time for pid %d: %w", pid, err)
	}
	command := strings.TrimRight(m[2], "\n")
	if command == "" {
		return nil, fmt.Errorf("unreadable command for pid %d", pid)
	}
	return map[string]any{"pid": pid, "pidStartedAt": parsed.Unix(), "command": command}, nil
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
