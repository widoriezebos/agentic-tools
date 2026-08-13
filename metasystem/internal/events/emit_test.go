package events

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func sandbox(t *testing.T, registry string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "scripts", "agents")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if registry != "" {
		if err := os.WriteFile(filepath.Join(dir, "event-registry.json"), []byte(registry), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func readLines(t *testing.T, root string) []map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, "artifacts", "agents", "events.jsonl"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	var out []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		var obj map[string]any
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			t.Fatalf("unparseable event line %q: %v", line, err)
		}
		out = append(out, obj)
	}
	return out
}

const leaseRegistry = `{"events":{"lease-claimed":{"emitters":["lease"]},"other":{"emitters":["dispatch"]}}}`

func TestEmitWritesRegisteredEvent(t *testing.T) {
	root := sandbox(t, leaseRegistry)
	e := &Emitter{Component: "lease", Pid: 42, PidStartedAt: 100}
	e.Emit(root, "lease-claimed", "fresh claim", map[string]string{"epoch": "1"})

	lines := readLines(t, root)
	if len(lines) != 1 {
		t.Fatalf("want 1 event, got %d", len(lines))
	}
	ev := lines[0]
	if ev["event"] != "lease-claimed" || ev["component"] != "lease" || ev["summary"] != "fresh claim" {
		t.Fatalf("unexpected event: %v", ev)
	}
	if ev["epoch"] != "1" {
		t.Fatalf("payload field lost: %v", ev)
	}
	if ev["seq"].(float64) != 1 {
		t.Fatalf("seq should start at 1, got %v", ev["seq"])
	}
}

func TestEmitDropsUnregisteredEventAndWrongEmitter(t *testing.T) {
	root := sandbox(t, leaseRegistry)
	e := &Emitter{Component: "lease"}
	e.Emit(root, "not-in-registry", "x", nil) // unregistered
	e.Emit(root, "other", "x", nil)           // registered, but lease may not emit it
	if lines := readLines(t, root); len(lines) != 0 {
		t.Fatalf("want no events written, got %d", len(lines))
	}
}

func TestEmitWritesWhenRegistryAbsent(t *testing.T) {
	root := sandbox(t, "") // no registry file
	e := &Emitter{Component: "lease"}
	e.Emit(root, "anything", "witness must not be silenced", nil)
	if lines := readLines(t, root); len(lines) != 1 {
		t.Fatalf("absent registry should not silence the witness; got %d lines", len(lines))
	}
}

func TestEmitHonorsHardCap(t *testing.T) {
	root := sandbox(t, leaseRegistry)
	e := &Emitter{Component: "lease"}
	huge := strings.Repeat("A", 10000)
	e.Emit(root, "lease-claimed", huge, map[string]string{"blob": huge})

	data, err := os.ReadFile(filepath.Join(root, "artifacts", "agents", "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	// The line is "\n"+json; the whole append must be within the cap.
	if len(data) > capBytes {
		t.Fatalf("event line %d bytes exceeds hard cap %d", len(data), capBytes)
	}
	// The required envelope must survive even after shrinking.
	lines := readLines(t, root)
	if len(lines) != 1 || lines[0]["event"] != "lease-claimed" || lines[0]["pid"] == nil {
		t.Fatalf("required envelope lost under cap: %v", lines)
	}
}

func TestEmitShrinksOptionalFieldsUnderCap(t *testing.T) {
	root := sandbox(t, leaseRegistry)
	e := &Emitter{Component: "lease"}
	// Many payload fields (each already clipped to the payload cap) whose sum
	// exceeds the hard cap even with an empty summary, forcing shrink() to drop
	// optional fields largest-first.
	fields := map[string]string{}
	for i := 0; i < 40; i++ {
		fields[fmt.Sprintf("f%02d", i)] = strings.Repeat("Z", 256)
	}
	e.Emit(root, "lease-claimed", "", fields)

	data, err := os.ReadFile(filepath.Join(root, "artifacts", "agents", "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if len(data) > capBytes {
		t.Fatalf("shrink failed to bring the line under the cap: %d bytes", len(data))
	}
	lines := readLines(t, root)
	if len(lines) != 1 || lines[0]["event"] != "lease-claimed" || lines[0]["seq"] == nil {
		t.Fatalf("required envelope must survive shrink: %v", lines)
	}
}

func TestEmitSequenceIncrements(t *testing.T) {
	root := sandbox(t, leaseRegistry)
	e := &Emitter{Component: "lease"}
	e.Emit(root, "lease-claimed", "a", nil)
	e.Emit(root, "lease-claimed", "b", nil)
	lines := readLines(t, root)
	if len(lines) != 2 || lines[0]["seq"].(float64) != 1 || lines[1]["seq"].(float64) != 2 {
		t.Fatalf("seq should be 1 then 2: %v", lines)
	}
}

// lease-census-6: a payload field named like an envelope field must not
// clobber the emitter's kernel-fact identity.
func TestEmitProtectsTheEnvelopeFromPayloadClobber(t *testing.T) {
	dir := t.TempDir()
	e := &Emitter{Component: "test", Pid: 42, PidStartedAt: 7}
	e.Emit(dir, "clobber-probe", "probing", map[string]string{
		"pid": "evil", "seq": "evil", "ts": "evil", "schemaVersion": "evil",
		"pidStartedAt": "evil", "component": "evil", "event": "evil", "honest": "kept",
	})
	lines := readLines(t, dir)
	if len(lines) == 0 {
		t.Fatal("no event emitted")
	}
	record := lines[len(lines)-1]
	if pid, _ := record["pid"].(float64); int64(pid) != 42 {
		t.Fatalf("pid was clobbered: %v", record["pid"])
	}
	if start, _ := record["pidStartedAt"].(float64); int64(start) != 7 {
		t.Fatalf("pidStartedAt was clobbered: %v", record["pidStartedAt"])
	}
	if record["component"] != "test" || record["ts"] == "evil" || record["schemaVersion"] == "evil" {
		t.Fatalf("envelope fields were clobbered: %v", record)
	}
	if record["payload.pid"] != "evil" || record["honest"] != "kept" {
		t.Fatalf("payload fields must survive under their prefixed names: %v", record)
	}
}
