package adapter

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// The self-test helpers back the full-contract adapter self-test: asserting a
// completed job's typed usage, reading the newest capability snapshot's
// envelope declaration, answering exactly one network request as the denied-
// fetch tripwire, and writing the pass record that states what was actually
// proven.

// SelftestUsageCheck asserts a job record's typed usage matches the
// expectation. "native" requires token counts with native availability;
// "unavailable" requires that availability recorded. "metered" is for a
// runtime whose usage shape depends on the ACCOUNT rather than the runtime:
// an enterprise Devin reports no tokens at all, only ACU, which is a
// different unit and never a token count. Accepting either shape blindly
// would assert nothing, so this asserts the thing that must hold in both
// worlds — a turn is measured by SOMETHING the fence can meter, never by
// nothing.
func SelftestUsageCheck(recordPath, expected string) error {
	record, err := readObject(recordPath)
	if err != nil {
		return fmt.Errorf("cannot read the job record: %w", err)
	}
	usage, ok := record["usage"].(map[string]any)
	if !ok {
		return errors.New("job has no typed usage")
	}
	availability, _ := usage["availability"].(string)
	tokens := isJSONNumber(usage["inputTokens"]) && isJSONNumber(usage["outputTokens"])
	if expected == "metered" {
		units, unitsOK := usage["providerUnits"].(map[string]any)
		metered := unitsOK && isJSONNumber(units["value"]) && isString(units["name"])
		if !tokens && !metered {
			return fmt.Errorf("usage reports neither token counts nor named provider units: %s", compactJSON(usage))
		}
		if tokens && availability != "native" {
			return fmt.Errorf("token-counted usage must declare native availability: %s", compactJSON(usage))
		}
		return nil
	}
	if availability != expected {
		return fmt.Errorf("usage availability is %q, expected %q: %s", availability, expected, compactJSON(usage))
	}
	if expected == "native" && !tokens {
		return fmt.Errorf("native usage must carry numeric inputTokens and outputTokens: %s", compactJSON(usage))
	}
	return nil
}

// SelftestEnvelopeDeclaration reads a runtime's newest parseable capability
// snapshot and returns its envelope-enforcement declaration for one field.
// Absent a snapshot or a declaration, the answer is "mapped": the stricter
// reading, under which the self-test demands behavioural proof of denial.
func SelftestEnvelopeDeclaration(capabilitiesDir, runtime, field string) string {
	matches, _ := filepath.Glob(filepath.Join(capabilitiesDir, runtime+"-*.json"))
	// Newest by (capturedAt, filename) — capability's own rule (review
	// adapter-host-registry-3): snapshot names sort by CLI version and
	// config hash BEFORE date, so after an upgrade like 1.9 -> 1.10 the
	// lexical order this replaced read a STALE declaration — a stale
	// 'mapped' demanded a denial the current runtime cannot produce, a
	// stale 'notEnforced' skipped proof it could have given. The runtime
	// segment is also matched exactly so a runtime whose name extends
	// this one's prefix cannot shadow it.
	var newest map[string]any
	var newestAt string
	var newestPath string
	for _, path := range matches {
		base := filepath.Base(path)
		snapshot, err := readObject(path)
		if err != nil || snapshot == nil {
			continue
		}
		if recorded, ok := snapshot["runtime"].(string); ok && recorded != runtime {
			continue
		}
		capturedAt, _ := snapshot["capturedAt"].(string)
		if newest == nil || capturedAt > newestAt ||
			(capturedAt == newestAt && base > newestPath) {
			newest, newestAt, newestPath = snapshot, capturedAt, base
		}
	}
	enforcement, _ := newest["envelopeEnforcement"].(map[string]any)
	if declared, ok := enforcement[field].(string); ok {
		return declared
	}
	return "mapped"
}

// WriteSelftestRecord writes the pass record for a full-contract self-test.
// The record states what was actually PROVEN, per the snapshot's own
// declaration: a mapped field's attempt was denied and that is behavioural
// proof; a notEnforced field's attempt was not asserted either way (a shell
// can escape it), so claiming it proven would be the exact overclaim a reader
// retains as acceptance evidence. Usage likewise reflects the mode observed.
func WriteSelftestRecord(outputPath, runtime, job, usage string, probeLabels []string, writeEnforcement, networkEnforcement string) error {
	proven := []string{"dispatch", "return-validation", "resume-identity", "cancel", "permitted-read"}
	// Only a mapped (enforced) field yields behavioural proof of denial.
	if writeEnforcement == "mapped" {
		proven = append(proven, "forbidden-write-denied")
	}
	if networkEnforcement == "mapped" {
		proven = append(proven, "denied-network")
	}
	switch usage {
	case "native":
		proven = append(proven, "usage-extraction")
	case "metered":
		proven = append(proven, "usage-metered")
	default:
		proven = append(proven, "usage-unavailable-recording")
	}
	proven = append(proven, probeLabels...)
	value := map[string]any{
		"runtime":            runtime,
		"job":                job,
		"passedAt":           timestampUTC(now()),
		"provenBehaviorally": proven,
		"permissionEnvelopeEvidence": map[string]any{
			"declaredEnforcement": map[string]any{
				"writeRoots": writeEnforcement,
				"network":    networkEnforcement,
			},
			"behaviorallyProven": map[string]any{
				"readRoots":  "permitted-read",
				"writeRoots": enforcementEvidence(writeEnforcement, "forbidden-write-denied"),
				"network":    enforcementEvidence(networkEnforcement, "denied-fetch"),
			},
			"constructedOnly": []string{"approvals", "tools", "readRoots-completeness"},
		},
		"usageAvailability": usage,
	}
	switch usage {
	case "native":
	case "metered":
		value["usageNote"] = "metered: provider units are recorded and fence-metered even when token counts are absent"
	default:
		value["usageNote"] = "no per-turn usage was reported"
	}
	return atomicWriteJSON(outputPath, value)
}

// enforcementEvidence names what a field's attempt proved. A notEnforced
// field cannot be proven denied from one turn (which tool the model reaches
// for decides whether it escapes), so it is stated as such.
func enforcementEvidence(enforcement, deniedTag string) string {
	if enforcement == "mapped" {
		return deniedTag
	}
	return "not-enforced (containment is the operator's, not asserted here)"
}

// SelftestListener is the one-shot tripwire behind the denied-fetch probe: it
// binds an ephemeral loopback port, publishes the port, and answers exactly
// one request, recording its bytes. The request log's very existence is the
// evidence that a supposedly denied fetch got through, so it is written only
// when a connection actually arrives; an idle timeout is a quiet success.
func SelftestListener(portFilePath, requestLogPath string, timeout time.Duration) error {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	defer listener.Close()
	port := listener.Addr().(*net.TCPAddr).Port
	if err := os.WriteFile(portFilePath, []byte(strconv.Itoa(port)), 0o644); err != nil {
		return err
	}
	if tcp, ok := listener.(*net.TCPListener); ok {
		tcp.SetDeadline(time.Now().Add(timeout))
	}
	connection, err := listener.Accept()
	if err != nil {
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			return nil
		}
		return err
	}
	defer connection.Close()
	connection.SetDeadline(time.Now().Add(timeout))
	request := make([]byte, 4096)
	n, readErr := connection.Read(request)
	if n == 0 && readErr != nil && readErr != io.EOF {
		return readErr
	}
	if err := os.WriteFile(requestLogPath, request[:n], 0o644); err != nil {
		return err
	}
	_, err = connection.Write([]byte("HTTP/1.1 200 OK\r\nContent-Length: 2\r\n\r\nOK"))
	return err
}

func isJSONNumber(value any) bool {
	switch value.(type) {
	case json.Number, float64:
		return true
	}
	return false
}

func isString(value any) bool {
	_, ok := value.(string)
	return ok
}

func compactJSON(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprintf("%v", value)
	}
	return string(data)
}
