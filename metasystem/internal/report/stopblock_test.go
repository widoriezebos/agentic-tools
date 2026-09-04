package report

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestStopBlock(t *testing.T) {
	b := StopBlock("PLAN says do X")
	if b["decision"] != "block" {
		t.Fatalf("stop-block must be a block decision: %v", b)
	}
	reason, _ := b["reason"].(string)
	if !strings.Contains(reason, "unblocked and nothing is in flight") {
		t.Fatalf("reason missing the standing guidance: %q", reason)
	}
	if !strings.HasSuffix(reason, "PLAN says do X") {
		t.Fatalf("reason must append the caller detail: %q", reason)
	}
}

func TestStopRefusalBlocksOnceThenSurfacesAndRecords(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.json")
	now := time.Date(2026, 9, 4, 10, 0, 0, 0, time.UTC)
	first, err := StopRefusal(path, "session-a", "supervision arming failed", "exact up diagnostic", "unsafe", "health", now)
	if err != nil {
		t.Fatal(err)
	}
	if first["decision"] != "block" {
		t.Fatalf("first external cause must block: %v", first)
	}
	second, err := StopRefusal(path, "session-a", "supervision arming failed", "exact up diagnostic", "unsafe", "health", now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if _, present := second["decision"]; present {
		t.Fatalf("repeated external cause must only surface: %v", second)
	}
	message, _ := second["systemMessage"].(string)
	for _, fragment := range []string{"occurrence 2", "supervision arming failed", "exact up diagnostic", "health"} {
		if !strings.Contains(message, fragment) {
			t.Fatalf("repeat message missing %q: %q", fragment, message)
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var record stopRefusalRecord
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatal(err)
	}
	if record.SchemaVersion != 1 || record.SessionID != "session-a" || len(record.Causes) != 1 {
		t.Fatalf("unexpected refusal record: %+v", record)
	}
	expectedDigest := fmt.Sprintf("%x", sha256.Sum256([]byte("supervision arming failed")))
	if _, ok := record.Causes[expectedDigest]; !ok {
		t.Fatalf("refusal cause was not keyed by its SHA-256 digest: %+v", record.Causes)
	}
	for _, cause := range record.Causes {
		if cause.Count != 2 || cause.FirstAt != now.Format(time.RFC3339) || cause.LastAt != now.Add(time.Minute).Format(time.RFC3339) {
			t.Fatalf("unexpected cause history: %+v", cause)
		}
	}
}

func TestStopRefusalDifferentCauseBlocksAgain(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.json")
	now := time.Date(2026, 9, 4, 10, 0, 0, 0, time.UTC)
	if _, err := StopRefusal(path, "session-a", "cause one", "remedy", "detail", "", now); err != nil {
		t.Fatal(err)
	}
	second, err := StopRefusal(path, "session-a", "cause two", "remedy", "detail", "", now)
	if err != nil {
		t.Fatal(err)
	}
	if second["decision"] != "block" {
		t.Fatalf("a different cause must block once: %v", second)
	}
}

func TestStopRefusalUnreadableRecordReturnsError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.json")
	if err := os.WriteFile(path, []byte("{broken\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := StopRefusal(path, "session-a", "cause", "remedy", "detail", "", time.Now()); err == nil {
		t.Fatal("an unreadable refusal record must be reported to the hook")
	}
}

func TestStopRefusalWaitsBrieflyForOverlappingWriter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.json")
	lockFile, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer lockFile.Close()
	if err := unix.Flock(int(lockFile.Fd()), unix.LOCK_EX); err != nil {
		t.Fatal(err)
	}
	released := make(chan struct{})
	go func() {
		time.Sleep(40 * time.Millisecond)
		_ = unix.Flock(int(lockFile.Fd()), unix.LOCK_UN)
		close(released)
	}()
	response, err := StopRefusal(path, "session-a", "cause", "remedy", "detail", "", time.Now())
	<-released
	if err != nil {
		t.Fatalf("a brief overlapping writer must not force surfacing: %v", err)
	}
	if response["decision"] != "block" {
		t.Fatalf("the first occurrence must still block after waiting for its writer: %v", response)
	}
}

func TestStopBlockEmptyDetail(t *testing.T) {
	b := StopBlock("")
	if !strings.HasSuffix(b["reason"].(string), "\n\n") {
		t.Fatalf("with no detail the reason still ends with the separator: %q", b["reason"])
	}
}
