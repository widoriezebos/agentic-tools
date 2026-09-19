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

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
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
	if reason != "PLAN says do X\n\n"+stopBlockReason {
		t.Fatalf("reason must put the caller detail before the standing guidance: %q", reason)
	}
}

func TestBoundedIdleStopBlockDoesNotClaimItIsNonRepeating(t *testing.T) {
	detail := "IDLE WITH BACKLOG: refusal 2 of 3; at 3 the steward claims and continues the next goal"
	block := BoundedIdleStopBlock(detail)
	if block["decision"] != "block" || block["reason"] != detail {
		t.Fatalf("bounded idle detail changed: %+v", block)
	}
	if strings.Contains(block["reason"].(string), "does not repeat") {
		t.Fatalf("bounded idle block carried the open-work promise: %+v", block)
	}
}

func TestStopRefusalBlocksOnceThenSurfacesAndRecords(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.json")
	now := time.Date(2026, 9, 4, 10, 0, 0, 0, time.UTC)
	first, err := StopRefusal(path, "session-a", "supervision arming failed", "exact up diagnostic", "unsafe", "health", StopClassSeatActionable, now)
	if err != nil {
		t.Fatal(err)
	}
	if first["decision"] != "block" {
		t.Fatalf("first external cause must block: %v", first)
	}
	second, err := StopRefusal(path, "session-a", "supervision arming failed", "exact up diagnostic", "unsafe", "health", StopClassSeatActionable, now.Add(time.Minute))
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
	if _, err := StopRefusal(path, "session-a", "cause one", "remedy", "detail", "", StopClassSeatActionable, now); err != nil {
		t.Fatal(err)
	}
	second, err := StopRefusal(path, "session-a", "cause two", "remedy", "detail", "", StopClassSeatActionable, now)
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
	if _, err := StopRefusal(path, "session-a", "cause", "remedy", "detail", "", StopClassSeatActionable, time.Now()); err == nil {
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
	previous := stopRefusalLockSleep
	t.Cleanup(func() { stopRefusalLockSleep = previous })
	sleeps := 0
	stopRefusalLockSleep = func(duration time.Duration) {
		sleeps++
		if sleeps != 1 || duration != 10*time.Millisecond {
			t.Fatalf("lock retry sleeps=%d duration=%s", sleeps, duration)
		}
		if err := unix.Flock(int(lockFile.Fd()), unix.LOCK_UN); err != nil {
			t.Fatalf("release overlapping writer: %v", err)
		}
	}
	response, err := StopRefusal(path, "session-a", "cause", "remedy", "detail", "", StopClassSeatActionable, time.Now())
	if err != nil {
		t.Fatalf("a brief overlapping writer must not force surfacing: %v", err)
	}
	if sleeps != 1 {
		t.Fatalf("lock retries=%d, want 1", sleeps)
	}
	if response["decision"] != "block" {
		t.Fatalf("the first occurrence must still block after waiting for its writer: %v", response)
	}
}

func TestStopRefusalReportsAWedgedWriter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.json")
	lockFile, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer lockFile.Close()
	if err := unix.Flock(int(lockFile.Fd()), unix.LOCK_EX); err != nil {
		t.Fatal(err)
	}
	// A writer that never releases is wedged by construction: no wait
	// makes it brief, and the refusal reports the record busy at once.
	previous := stopRefusalLockWait
	stopRefusalLockWait = 0
	t.Cleanup(func() { stopRefusalLockWait = previous })
	if _, err := StopRefusal(path, "session-a", "cause", "remedy", "detail", "", StopClassSeatActionable, time.Now()); err == nil || !strings.Contains(err.Error(), "busy after") {
		t.Fatalf("a wedged writer must be reported to the hook: %v", err)
	}
}

func TestStopBlockClassesInfrastructureAllows(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.json")
	now := time.Date(2026, 9, 12, 10, 0, 0, 0, time.UTC)
	for occurrence := 1; occurrence <= 5; occurrence++ {
		response, err := StopRefusal(path, "session-a", "narrator failed", "the steward owns repair", "detail", "health", StopClassInfrastructure, now.Add(time.Duration(occurrence)*time.Minute))
		if err != nil {
			t.Fatal(err)
		}
		if _, blocked := response["decision"]; blocked {
			t.Fatalf("infrastructure occurrence %d blocked: %+v", occurrence, response)
		}
		if message, _ := response["systemMessage"].(string); !strings.Contains(message, fmt.Sprintf("occurrence %d", occurrence)) {
			t.Fatalf("occurrence missing: %q", message)
		}
	}
	seat, err := StopRefusal(filepath.Join(t.TempDir(), "seat.json"), "session-a", "owned work", "run it", "work remains", "", StopClassSeatActionable, now)
	if err != nil || seat["decision"] != "block" {
		t.Fatalf("seat-actionable behavior changed: %+v %v", seat, err)
	}
}

func TestInfrastructureStopNoticePreservesCheckInTail(t *testing.T) {
	tail := "HEALTH good — hook-freshness=current\nNARRATOR DIGEST since last check-in: landed abc123"
	response, err := StopRefusal(filepath.Join(t.TempDir(), "session.json"), "session-a", "arming failed", "repair it",
		strings.Repeat("large verdict detail ", 400), tail, StopClassInfrastructure, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	message, _ := response["systemMessage"].(string)
	if !strings.HasSuffix(message, tail) {
		t.Fatalf("infrastructure notice trimmed the check-in tail: %q", message)
	}
	if len([]rune(message)) > goal.TurnVerdictDisplayRuneLimit {
		t.Fatalf("infrastructure notice has %d runes", len([]rune(message)))
	}
}

func TestStopBlockEmptyDetail(t *testing.T) {
	b := StopBlock("")
	if b["reason"] != stopBlockReason {
		t.Fatalf("with no detail the reason is the guidance without a leading separator: %q", b["reason"])
	}
}

func TestBoundSystemMessageKeepsFirstLineAndAddsTrimNotice(t *testing.T) {
	first := "SUPERVISION NEEDS ATTENTION"
	message := first + "\n" + strings.Repeat("detail\n", 1000)
	bounded := BoundSystemMessage(message)
	if !strings.HasPrefix(bounded, first+"\n") {
		t.Fatalf("first line changed: %q", bounded)
	}
	if !strings.Contains(bounded, systemMessageTrimNotice) {
		t.Fatalf("trim notice is missing: %q", bounded)
	}
	if len([]rune(bounded)) > goal.TurnVerdictDisplayRuneLimit {
		t.Fatalf("system message has %d runes, limit is %d", len([]rune(bounded)), goal.TurnVerdictDisplayRuneLimit)
	}
}

func TestBoundSystemMessageTrimsAnOversizedFirstLineWithNotice(t *testing.T) {
	bounded := BoundSystemMessage(strings.Repeat("x", goal.TurnVerdictDisplayRuneLimit+100))
	if len([]rune(bounded)) != goal.TurnVerdictDisplayRuneLimit || !strings.HasSuffix(bounded, systemMessageTrimNotice) {
		t.Fatalf("oversized first line was not bounded with a notice: runes=%d", len([]rune(bounded)))
	}
}

func TestBoundSystemMessageClampsNearLimitFirstLines(t *testing.T) {
	for length := 3949; length <= 3953; length++ {
		t.Run(fmt.Sprintf("%d runes", length), func(t *testing.T) {
			message := strings.Repeat("x", length) + "\n" + strings.Repeat("detail", 20)
			bounded := BoundSystemMessage(message)
			if len([]rune(bounded)) > goal.TurnVerdictDisplayRuneLimit {
				t.Fatalf("bounded message has %d runes", len([]rune(bounded)))
			}
			if strings.ContainsRune(bounded, '\x00') {
				t.Fatalf("bounded message contains a NUL rune: %q", bounded)
			}
		})
	}
}
