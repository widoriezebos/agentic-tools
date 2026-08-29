package adapter

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// The fake runtime is the deterministic stand-in the fixture suite dispatches
// instead of a real CLI. Everything here backs its adapter: the canned role
// return and typed usage every fake turn reports, the capability snapshot its
// probe publishes, the guarded write/network mechanism that snapshot attests,
// the effective-permissions edits its behavior markers ask for, and the pass
// record its selftest leaves behind.

// WriteFakeUsage writes the fixed typed usage the fake runtime reports for
// every turn. The values are arbitrary but stable, so fixtures can assert
// usage aggregation against known numbers.
func WriteFakeUsage(outputPath string) error {
	return atomicWriteJSON(outputPath, map[string]any{
		"availability":      "native",
		"inputTokens":       11,
		"cachedInputTokens": 2,
		"outputTokens":      7,
		"reasoningTokens":   nil,
		"cost":              nil,
		"providerUnits":     map[string]any{"name": "fake-unit", "value": 1},
	})
}

// WriteFakeReturn writes the canned, schema-valid return for a fake turn:
// the job identity read from its record, the working mode read from the
// assembled prompt, and the fixed role-specific fields a compliant agent of
// that role must produce. A role outside the fake's repertoire is refused.
func WriteFakeReturn(recordPath, promptPath, outputPath string) error {
	record, err := readObject(recordPath)
	if err != nil {
		return err
	}
	if record == nil {
		return fmt.Errorf("job record %s is not a JSON object", recordPath)
	}
	mode, err := workingMode(promptPath)
	if err != nil {
		return err
	}

	value := map[string]any{
		"schemaVersion": 2,
		"jobId":         record["jobId"],
		"round":         record["round"],
		"runtime":       "fake",
		"sessionId":     stringOrUnobserved(record["sessionId"]),
		"model": map[string]any{
			"requested": record["requestedModel"],
			"effective": stringOrUnobserved(record["effectiveModel"]),
		},
		"evidence": []any{map[string]any{
			"command":  "fake protocol simulator",
			"observed": "canned role return",
			"level":    "ran",
		}},
		"gaps": []any{},
		"mode": mode,
		// The simulator stands in for a compliant agent, and a compliant agent
		// emits `claimed` with both members: null means it claims nothing,
		// which is what a simulator that observes its own identity honestly
		// reports.
		"claimed": map[string]any{"sessionId": nil, "model": nil},
	}

	role, _ := record["role"].(string)
	switch role {
	case "design-critic", "code-critic", "warden":
		value["schemaVersion"] = 3
		value["findings"] = []any{}
		value["rigor"] = []any{}
		value["verdictMaterialCount"] = 0
		if role == "design-critic" {
			workspace, _ := record["workspaceRoot"].(string)
			commit, err := gitHead(workspace)
			if err != nil {
				return err
			}
			value["reviewedCommit"] = commit
		} else {
			value["reviewedTree"] = strings.Repeat("0", 40)
		}
	case "implementer":
		value["riskiestPart"] = "fake boundary"
		value["diffBoundary"] = []any{}
		value["whatWasDone"] = "simulated implementation"
	case "steward-continuation":
		value["goal"] = "fake-goal"
		value["outcome"] = "continued"
		value["landed"] = []any{}
		value["remaining"] = "simulated continuation"
		value["receipts"] = 0
	case "verifier":
		value["riskiestPart"] = "fake boundary"
		value["whatWasDone"] = "simulated verification"
	case "investigator":
		value["frozenFrame"] = "simulated frozen frame"
		value["theories"] = []any{map[string]any{
			"statement":       "fixture theory",
			"evidenceFor":     "marker",
			"evidenceAgainst": "none",
		}}
		value["classifications"] = []any{"falsified-continue"}
		value["stopLoss"] = map[string]any{"triggered": false, "trigger": nil}
	default:
		return fmt.Errorf("unsupported fake role: %s", role)
	}
	return atomicWriteJSON(outputPath, value)
}

// workingMode reads the prompt's "Working Mode:" header, defaulting to
// implement when the prompt does not carry one.
func workingMode(promptPath string) (string, error) {
	data, err := os.ReadFile(promptPath)
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(data), "\n") {
		if rest, found := strings.CutPrefix(line, "Working Mode:"); found {
			return strings.TrimSpace(rest), nil
		}
	}
	return "implement", nil
}

// gitHead is the commit the design-critic role reports as
// reviewedCommit: HEAD of the job's workspace.
func gitHead(workspace string) (string, error) {
	output, err := exec.Command("git", "-C", workspace, "rev-parse", "HEAD").Output()
	if err != nil {
		return "", fmt.Errorf("cannot read the reviewed commit in %s: %w", workspace, err)
	}
	return strings.TrimSpace(string(output)), nil
}

// SetEffectiveNetwork rewrites the network field of an effective-permissions
// file. The fake's effective-wider and effective-narrower behavior markers
// use it to simulate a runtime whose real grant differs from the request, so
// the widening check has something true to refuse.
func SetEffectiveNetwork(effectivePath, network string) error {
	effective, err := readObject(effectivePath)
	if err != nil {
		return err
	}
	if effective == nil {
		return fmt.Errorf("effective permissions file %s is not a JSON object", effectivePath)
	}
	effective["network"] = network
	return atomicWriteJSON(effectivePath, effective)
}

// FakeGuardedWrite writes a probe line to target only when a writeRoots
// member of the permissions envelope contains it, reporting whether the write
// was allowed. This is the fake runtime's write-enforcement mechanism: its
// probe drives it with a deny-all envelope to prove a denied write really is
// refused before publishing a snapshot that declares writeRoots mapped.
func FakeGuardedWrite(permissionsPath, targetPath string) (bool, error) {
	permissions, err := readObject(permissionsPath)
	if err != nil {
		return false, err
	}
	target := resolveLenient(targetPath)
	for _, root := range stringList(permissions["writeRoots"]) {
		if !pathContains(resolveLenient(root), target) {
			continue
		}
		if err := os.WriteFile(targetPath, []byte("fake envelope write probe\n"), 0o644); err != nil {
			return false, err
		}
		return true, nil
	}
	return false, nil
}

// FakeGuardedNetwork sends one probe request to host:port only when the
// permissions envelope allows network access, reporting whether the call was
// allowed. Paired with FakeGuardedWrite as the fake's enforcement mechanism.
func FakeGuardedNetwork(permissionsPath, host, port string) (bool, error) {
	permissions, err := readObject(permissionsPath)
	if err != nil {
		return false, err
	}
	if network, _ := permissions["network"].(string); network != "allow" {
		return false, nil
	}
	connection, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), time.Second)
	if err != nil {
		return false, err
	}
	defer connection.Close()
	if _, err := connection.Write([]byte("GET /fake-envelope-probe HTTP/1.0\r\n\r\n")); err != nil {
		return false, err
	}
	return true, nil
}

// WriteFakeCapabilitySnapshot writes the capability snapshot the fake probe
// publishes and returns its path. The profile selects what the snapshot
// declares: "current" enables every capability, "old" is the same shape aged
// by ageDays so staleness rules have something dated to refuse, and
// "unverified-network" declares network enforcement unverified. handshakeSec
// is the session-established ceiling the probe measured for this machine.
//
// The dated name carries a per-day sequence and is created exclusively, so
// two probes racing the same sequence cannot clobber one another's capture.
func WriteFakeCapabilitySnapshot(dir, profile string, ageDays, handshakeSec int) (string, error) {
	switch profile {
	case "current", "old", "unverified-network":
	default:
		return "", fmt.Errorf("unknown fake probe profile: %s", profile)
	}
	if ageDays < 0 {
		return "", fmt.Errorf("snapshot age must not be negative")
	}
	if handshakeSec < 1 {
		return "", fmt.Errorf("handshake ceiling must be a positive number of seconds")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}

	captured := now().UTC().Add(-time.Duration(ageDays) * 24 * time.Hour)
	prefix := fmt.Sprintf("fake-fake-1-fake-config-v1-%s-", captured.Format("20060102"))
	sequence := nextSequence(dir, prefix)
	path := filepath.Join(dir, fmt.Sprintf("%s%03d.json", prefix, sequence))

	enabled := profile == "current"
	networkEnforcement := "mapped"
	unverified := []any{}
	if profile == "unverified-network" {
		networkEnforcement = "notEnforced"
		unverified = []any{"network"}
	}
	value := map[string]any{
		"runtime":         "fake",
		"cliVersion":      "fake-1",
		"configHash":      "fake-config-v1",
		"configKeyHashes": map[string]any{},
		"capturedAt":      timestampUTC(captured),
		"sequence":        sequence,
		"transports":      []any{"stdin", "file"},
		"capabilities": map[string]any{
			"resume":                       enabled,
			"sessionEstablishedSignal":     enabled,
			"nativeStructuredOutput":       enabled,
			"nativeEvents":                 enabled,
			"nativeUsage":                  enabled,
			"gracefulCancel":               enabled,
			"hooks":                        enabled,
			"protocolServer":               enabled,
			"nativeBudget":                 enabled,
			"sessionEstablishedTimeoutSec": handshakeSec,
		},
		"permissions": map[string]any{"unverified": unverified},
		"envelopeEnforcement": map[string]any{
			"writeRoots": "mapped",
			"readRoots":  "notEnforced",
			"network":    networkEnforcement,
		},
		"profile": profile,
	}
	if err := exclusiveWriteJSON(path, value); err != nil {
		return "", err
	}
	return path, nil
}

// WriteFakeSelftestRecord writes the pass record the fake adapter's selftest
// leaves behind: which behaviors the run proved by driving them, and which
// the adapter merely constructs without behavioral proof.
func WriteFakeSelftestRecord(outputPath, jobID string) error {
	if jobID == "" {
		return fmt.Errorf("selftest job id is required")
	}
	return atomicWriteJSON(outputPath, map[string]any{
		"runtime":  "fake",
		"job":      jobID,
		"passedAt": timestampUTC(now()),
		"provenBehaviorally": []any{
			"dispatch", "return-validation", "resume-identity", "cancel",
			"usage-extraction", "denied-write", "denied-network",
		},
		"constructedOnly": []any{"readRoots", "approvals", "tools"},
	})
}

// stringOrUnobserved keeps a non-empty string and turns anything else into
// the "unobserved" placeholder the return schema expects when a runtime never
// revealed the value.
func stringOrUnobserved(value any) string {
	if s, ok := value.(string); ok && s != "" {
		return s
	}
	return "unobserved"
}

// resolveLenient resolves a path to its absolute, symlink-free form. A path
// that does not exist yet resolves through its parent, so a boundary check on
// a file about to be created still compares canonical forms.
func resolveLenient(path string) string {
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	if real, err := filepath.EvalSymlinks(path); err == nil {
		return real
	}
	if real, err := filepath.EvalSymlinks(filepath.Dir(path)); err == nil {
		return filepath.Join(real, filepath.Base(path))
	}
	return path
}

// pathContains reports whether target lies at or under root, comparing purely
// lexically on already-resolved paths.
func pathContains(root, target string) bool {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
