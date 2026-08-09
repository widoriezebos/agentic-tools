package registry

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The remaining branches: error surfaces and defensive edges the main
// tables do not reach. Each is still a contract statement — an error
// message is part of how the registry explains itself (D-5).

func TestCorruptionErrorNamesBothLines(t *testing.T) {
	err := &CorruptionError{GarbageLine: 3, RecordLine: 7}
	text := err.Error()
	if !strings.Contains(text, "line 3") || !strings.Contains(text, "line 7") || !strings.Contains(text, "fail closed") {
		t.Fatalf("corruption error must name both lines and the fail-closed duty: %q", text)
	}
}

func TestAppendFrameSurfacesOpenFailure(t *testing.T) {
	directory := t.TempDir()
	if err := os.Chmod(directory, 0o555); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(directory, 0o755)
	err := AppendFrame(filepath.Join(directory, "registry.jsonl"), []byte(`{"a":1}`))
	if err == nil || !strings.Contains(err.Error(), "registry open") {
		t.Fatalf("open failure not surfaced: %v", err)
	}
}

func TestReadFramesSurfacesReadFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "registry.jsonl")
	if err := os.WriteFile(path, []byte("{}\n"), 0o000); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadFrames(path); err == nil {
		t.Fatal("unreadable registry must error, not read as empty")
	}
}

func TestReadFramesSkipsBlankAndWhitespaceLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "registry.jsonl")
	if err := os.WriteFile(path, []byte("\n   \n{\"schemaVersion\":1,\"event\":\"torn\",\"checkoutPath\":\"\",\"at\":\"x\"}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	frames, err := ReadFrames(path)
	if err != nil || len(frames) != 1 {
		t.Fatalf("whitespace lines must be skipped: %v %v", frames, err)
	}
}

func TestWriteCompactedFailsIntoMissingDirectory(t *testing.T) {
	err := WriteCompacted(filepath.Join(t.TempDir(), "absent", "registry.jsonl"), nil)
	if err == nil || !strings.Contains(err.Error(), "compaction temp file") {
		t.Fatalf("missing directory not surfaced: %v", err)
	}
}

func TestWriteCompactedEmptyKeepsEmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "registry.jsonl")
	appendOrFail(t, path, `{"a":1}`)
	if err := WriteCompacted(path, nil); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil || len(content) != 0 {
		t.Fatalf("empty compaction must leave an empty registry: %q %v", content, err)
	}
}

func TestCustodyRecordTimeUnparseableMeansZero(t *testing.T) {
	row := map[string]any{"event": EventCustody, "custodyId": "c", "at": "not-a-time"}
	if got := custodyRecordTime(frames(row), "c"); !got.IsZero() {
		t.Fatalf("unparseable at must yield zero time: %v", got)
	}
	// Zero time means the custody looks OLDER than any grace window:
	// an unparseable birth date never extends retention.
	if age := compactNow.Sub(time.Time{}); age < compactGrace {
		t.Fatal("zero time must fall outside the grace window")
	}
}

func TestNumberAcceptsIntegerKinds(t *testing.T) {
	for _, value := range []any{int(4), int64(4), float64(4)} {
		if got, ok := number(value); !ok || got != 4 {
			t.Fatalf("number(%T) = %v %v", value, got, ok)
		}
	}
	if _, ok := number("4"); ok {
		t.Fatal("number must reject strings")
	}
}

func TestSweepableDefensiveDefault(t *testing.T) {
	claim := &Claim{Closed: true, ClosedBy: "neither"}
	if claim.Sweepable() {
		t.Fatal("an unknown closer must not read as sweepable")
	}
}

func TestCurrentGenerationEmpty(t *testing.T) {
	claim := &Claim{Generations: map[int64]*GenerationSet{}}
	if claim.CurrentGeneration() != nil {
		t.Fatal("no generations means no current set")
	}
}

func TestDuplicateArmingAndCustodyAreIgnored(t *testing.T) {
	custody := raw(EventCustody, "", map[string]any{
		"custodianPid": 10.0, "custodianPidStartedAt": 20.0,
	})
	custody["custodyId"] = "c"
	duplicate := raw(EventCustody, "", map[string]any{
		"custodianPid": 99.0, "custodianPidStartedAt": 99.0,
	})
	duplicate["custodyId"] = "c"
	release := raw(EventCustodyReleased, "", nil)
	release["custodyId"] = "never-opened"

	reduction := reduceOrFail(t,
		raw(EventArming, "t", nil),
		raw(EventArming, "t", nil), // duplicate reservation ignored
		custody, duplicate, release,
	)
	if reduction.Custodies["c"].Custodian.Pid != 10 {
		t.Fatal("a duplicate custody overwrote the original")
	}
	if reduction.Custodies["never-opened"] != nil {
		t.Fatal("a release without a custody minted one")
	}
}
