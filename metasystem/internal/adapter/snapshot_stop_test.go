package adapter

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func stopDeliveryJSON(level string) string {
	value := map[string]any{
		"schemaVersion": 1, "envelope": "shared-reason-v1",
		"blockField": "decision", "blockValue": "block", "blockTextField": "reason", "allowTextField": "systemMessage",
		"humanVisibleFields": []string{"reason", "systemMessage"}, "duplicateBehavior": "unknown", "reportReadRoute": "unknown",
		"instructionHash": "", "trustProbe": "", "observationArtifact": "", "launchBinary": "", "seatCommandBinary": "",
		"continuationLimit": nil, "level": level,
	}
	data, _ := json.Marshal(map[string]any{"stopDelivery": value})
	return string(data)
}

func TestCapabilitySnapshotStopDeliverySeparatesExpectationFromObservation(t *testing.T) {
	originalNow := now
	now = func() time.Time { return time.Date(2026, 9, 13, 12, 34, 56, 0, time.UTC) }
	t.Cleanup(func() { now = originalNow })
	dir := t.TempDir()
	path, err := WriteCapabilitySnapshot(dir, "codex", "1.2.3", strings.Repeat("c", 64), `{}`, stopDeliveryJSON("unobserved"), `{}`,
		`{"writeRoots":"mapped","readRoots":"notEnforced","network":"mapped"}`, `{}`)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var snapshot map[string]any
	if err := json.Unmarshal(data, &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot["runtime"] != "codex" || snapshot["cliVersion"] != "1.2.3" || snapshot["capturedAt"] != "2026-09-13T12:34:56Z" {
		t.Fatalf("outer snapshot identity changed: %v", snapshot)
	}
	delivery := snapshot["capabilities"].(map[string]any)["stopDelivery"].(map[string]any)
	if delivery["level"] != "unobserved" || delivery["instructionHash"] != "" || delivery["launchBinary"] != "" {
		t.Fatalf("static expectation became installation evidence: %v", delivery)
	}
	if filepath.Base(path) != "codex-1.2.3-"+strings.Repeat("c", 64)+"-20260913-001.json" {
		t.Fatalf("dated snapshot path changed: %s", path)
	}
}

func TestCapabilitySnapshotRejectsIncompleteStopDeliveryEvidence(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(map[string]any)
	}{
		{name: "visible-fields", mutate: func(value map[string]any) { value["humanVisibleFields"] = []string{"reason"} }},
		{name: "unobserved-evidence", mutate: func(value map[string]any) { value["instructionHash"] = strings.Repeat("a", 64) }},
		{name: "emitted-without-binding", mutate: func(value map[string]any) { value["level"] = "emitted" }},
		{name: "observed-without-trust", mutate: func(value map[string]any) { value["level"] = "observed" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			var capabilities map[string]any
			if err := json.Unmarshal([]byte(stopDeliveryJSON("unobserved")), &capabilities); err != nil {
				t.Fatal(err)
			}
			test.mutate(capabilities["stopDelivery"].(map[string]any))
			data, _ := json.Marshal(capabilities)
			if _, err := WriteCapabilitySnapshot(t.TempDir(), "codex", "1", strings.Repeat("d", 64), `{}`, string(data), `{}`,
				`{"writeRoots":"mapped","readRoots":"notEnforced","network":"mapped"}`, `{}`); err == nil {
				t.Fatal("incomplete Stop delivery evidence was accepted")
			}
		})
	}
}
