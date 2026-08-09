package registry

import (
	"strings"
	"testing"
)

// REG-2's validator is per-event-type. Every rejection row here is a
// shape a writer bug could produce; the registry marks them corrupt
// rather than guessing (REG-5).

func TestParseRecordRejections(t *testing.T) {
	cases := []struct {
		name string
		row  map[string]any
		want string // fragment of the error
	}{
		{"missing schemaVersion", map[string]any{"event": EventArming}, "schemaVersion"},
		{"wrong schemaVersion", raw2(map[string]any{"schemaVersion": 2.0}), "schemaVersion"},
		{"fractional number", raw2(map[string]any{"schemaVersion": 1.5}), "schemaVersion"},
		{"unregistered event", raw2(map[string]any{"event": "not-a-thing"}), "unregistered"},
		{"unparseable at", raw2(map[string]any{"at": "yesterday"}), "unparseable at"},
		{"missing checkoutPath", raw2(map[string]any{"checkoutPath": ""}), "checkoutPath"},
		{"claim without ownerTag", raw2(map[string]any{"ownerTag": ""}), "ownerTag"},
		{"custody without custodyId", raw2(map[string]any{"event": EventCustody, "ownerTag": nil}), "custodyId"},
		{"armed without pid", raw2(map[string]any{"event": EventArmed}), "ownerPid"},
		{"armed with zero start", raw2(map[string]any{
			"event": EventArmed, "ownerPid": 4.0, "ownerPidStartedAt": 0.0, "generation": 1.0,
		}), "ownerPidStartedAt"},
		{"armed without generation", raw2(map[string]any{
			"event": EventArmed, "ownerPid": 4.0, "ownerPidStartedAt": 5.0, "generation": -1.0,
		}), "generation"},
		{"relaunched without tags", raw2(map[string]any{
			"event": EventRelaunched, "generation": 1.0, "retiredThrough": 0.0,
		}), "component tags"},
		{"relaunched generation zero", raw2(map[string]any{
			"event": EventRelaunched, "generation": 0.0, "watcherTag": "w", "reaperTag": "r", "retiredThrough": 0.0,
		}), "generation"},
		{"relaunched negative watermark", raw2(map[string]any{
			"event": EventRelaunched, "generation": 1.0, "watcherTag": "w", "reaperTag": "r", "retiredThrough": -1.0,
		}), "retiredThrough"},
		{"launched bad component", raw2(map[string]any{
			"event": EventLaunched, "generation": 1.0, "component": "janitor", "pid": 4.0, "pidStartedAt": 5.0,
		}), "component"},
		{"launched bad pid", raw2(map[string]any{
			"event": EventLaunched, "generation": 1.0, "component": "watcher", "pid": "four", "pidStartedAt": 5.0,
		}), "pid"},
		{"launched bad start", raw2(map[string]any{
			"event": EventLaunched, "generation": 1.0, "component": "watcher", "pid": 4.0, "pidStartedAt": -2.0,
		}), "pidStartedAt"},
		{"exited bad reason", raw2(map[string]any{
			"event": EventExited, "reason": "tired", "teardownComplete": true,
		}), "reason"},
		{"exited missing teardownComplete", raw2(map[string]any{
			"event": EventExited, "reason": "shutdown",
		}), "teardownComplete"},
		{"reaped bad reason", raw2(map[string]any{
			"event": EventReaped, "reason": "vibes", "sweepPending": false,
		}), "reason"},
		{"reaped missing sweepPending", raw2(map[string]any{
			"event": EventReaped, "reason": "owner-dead",
		}), "sweepPending"},
		{"custody bad custodian", func() map[string]any {
			row := raw2(map[string]any{"event": EventCustody, "custodianPid": 0.0, "custodianPidStartedAt": 5.0})
			row["custodyId"] = "c"
			return row
		}(), "custodianPid"},
		{"custody bad custodian start", func() map[string]any {
			row := raw2(map[string]any{"event": EventCustody, "custodianPid": 5.0})
			row["custodyId"] = "c"
			return row
		}(), "custodianPidStartedAt"},
	}
	for _, row := range cases {
		t.Run(row.name, func(t *testing.T) {
			_, err := ParseRecord(row.row)
			if err == nil {
				t.Fatalf("accepted invalid record: %v", row.row)
			}
			if !strings.Contains(err.Error(), row.want) {
				t.Fatalf("error %q does not name the defect %q", err, row.want)
			}
		})
	}
}

// raw2 builds a base valid arming record and overlays the mutation.
func raw2(overlay map[string]any) map[string]any {
	row := map[string]any{
		"schemaVersion": 1.0,
		"event":         EventArming,
		"checkoutPath":  "/repo",
		"at":            "2026-08-09T20:00:00Z",
		"ownerTag":      "tag-x",
	}
	for key, value := range overlay {
		if value == nil {
			delete(row, key)
			continue
		}
		row[key] = value
	}
	return row
}

func TestParseRecordAcceptsKilledListAndDiagnosis(t *testing.T) {
	row := raw2(map[string]any{
		"event": EventReaped, "reason": "shutdown-escalated", "sweepPending": true,
		"killed": []any{"pid 4 (watcher)", 7.0, "pid 9 (reaper)"},
	})
	record, err := ParseRecord(row)
	if err != nil {
		t.Fatal(err)
	}
	if len(record.Killed) != 2 {
		t.Fatalf("killed list mishandled: %v", record.Killed)
	}
	row = raw2(map[string]any{
		"event": EventExited, "reason": "giving-up", "teardownComplete": false,
		"diagnosis": "watcher died 5 consecutive observations",
	})
	record, err = ParseRecord(row)
	if err != nil {
		t.Fatal(err)
	}
	if record.Diagnosis == "" || record.TeardownComplete {
		t.Fatalf("exited fields lost: %+v", record)
	}
}

func TestTornMarkerParsesBare(t *testing.T) {
	record, err := ParseRecord(map[string]any{
		"schemaVersion": 1.0, "event": TornEvent, "checkoutPath": "", "at": "not-a-time",
	})
	if err != nil || record.Event != TornEvent {
		t.Fatalf("torn must parse with no claim state: %v %v", record, err)
	}
}
