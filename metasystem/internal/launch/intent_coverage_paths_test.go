package launch

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// The design document's retained outcomes: which attempt an outcome may be
// recorded for, that a recorded outcome is replayed rather than re-judged,
// that a judge's failure records nothing, and that a damaged or foreign entry
// or a held document lock refuses instead of answering. These reuse the
// design fixture of design_request_test.go; no author process is started.

// twoDesignAttempts makes a document with two retained attempts.
func twoDesignAttempts(t *testing.T) (*Manager, DesignRequest) {
	t.Helper()
	m, _, request := designFixture(t)
	if _, err := m.RequestDesign(request); err != nil {
		t.Fatalf("the first attempt: %v", err)
	}
	changed := request
	changed.Brief = []byte("Design the reader and the writer.\n")
	if second, err := m.RequestDesign(changed); err != nil || second.Attempt.Attempt != 2 {
		t.Fatalf("the second attempt: %+v %v", second, err)
	}
	return m, request
}

// An older attempt is recorded as superseded without being judged, the newest
// attempt is recorded as its judge decided, and a recorded outcome is replayed
// as it was without judging again. The retained entry answers the same.
func TestDesignOutcomeIsJudgedOnlyForTheNewestAttemptAndOnlyOnce(t *testing.T) {
	t.Parallel()

	m, request := twoDesignAttempts(t)
	judged := 0
	judge := func(attempt DesignAttempt) (string, string, string, error) {
		judged++
		return "published", "the draft was valid", "abc123", nil
	}

	older, err := m.RecordDesignOutcome(request.Destination, 1, judge)
	if err != nil || older.Outcome != "superseded" || older.Detail != "attempt 2 became current" || judged != 0 {
		t.Fatalf("the older attempt's outcome = %+v %v judged %d; want superseded unjudged", older, err, judged)
	}
	newest, err := m.RecordDesignOutcome(request.Destination, 2, judge)
	if err != nil || newest.Outcome != "published" || newest.Published != "abc123" || judged != 1 {
		t.Fatalf("the newest attempt's outcome = %+v %v judged %d; want published once", newest, err, judged)
	}
	replayed, err := m.RecordDesignOutcome(request.Destination, 2, judge)
	if err != nil || replayed != newest || judged != 1 {
		t.Fatalf("a replayed outcome = %+v %v judged %d; want the recorded outcome unjudged", replayed, err, judged)
	}

	recordID, attempts, err := m.DesignDocument(request.Destination)
	if err != nil || recordID != request.RecordID || len(attempts) != 2 ||
		attempts[0].Outcome != "superseded" || attempts[1].Outcome != "published" {
		t.Fatalf("the retained document = %q %+v %v", recordID, attempts, err)
	}
	if listed, err := m.DesignAttempts(request.Destination); err != nil || len(listed) != 2 || listed[1] != newest {
		t.Fatalf("the retained attempts = %+v %v", listed, err)
	}
}

// A judge that fails records nothing, so the same attempt can be judged again
// and its later outcome is the one retained.
func TestDesignOutcomeJudgeFailureRecordsNothing(t *testing.T) {
	t.Parallel()

	m, request := twoDesignAttempts(t)
	unreadable := errors.New("the draft could not be read")
	if _, err := m.RecordDesignOutcome(request.Destination, 2, func(DesignAttempt) (string, string, string, error) {
		return "", "", "", unreadable
	}); !errors.Is(err, unreadable) {
		t.Fatalf("a failing judge answered %v; want its error", err)
	}
	if attempts, _ := m.DesignAttempts(request.Destination); attempts[1].Outcome != "" {
		t.Fatalf("a failing judge recorded %+v", attempts[1])
	}
	later, err := m.RecordDesignOutcome(request.Destination, 2, func(DesignAttempt) (string, string, string, error) {
		return "conflict", "the document moved", "", nil
	})
	if err != nil || later.Outcome != "conflict" {
		t.Fatalf("the later judgment = %+v %v; want conflict", later, err)
	}
}

// An outcome for an attempt the document does not have, or for a document
// with no retained entry, is refused without judging.
func TestDesignOutcomeForAnUnknownAttemptIsRefused(t *testing.T) {
	t.Parallel()

	m, request := twoDesignAttempts(t)
	judge := func(DesignAttempt) (string, string, string, error) {
		t.Fatalf("an unknown attempt was judged")
		return "", "", "", nil
	}
	for _, attempt := range []int{0, 3} {
		if _, err := m.RecordDesignOutcome(request.Destination, attempt, judge); err == nil || !strings.Contains(err.Error(), "DESIGN_ATTEMPT_UNKNOWN") {
			t.Errorf("attempt %d answered %v; want DESIGN_ATTEMPT_UNKNOWN", attempt, err)
		}
	}
	elsewhere := filepath.Join(filepath.Dir(request.Destination), "other.md")
	if _, err := m.RecordDesignOutcome(elsewhere, 1, judge); err == nil || !strings.Contains(err.Error(), "DESIGN_ATTEMPT_UNKNOWN") {
		t.Errorf("a document with no entry answered %v; want DESIGN_ATTEMPT_UNKNOWN", err)
	}
	if attempts, err := m.DesignAttempts(elsewhere); err != nil || len(attempts) != 0 {
		t.Errorf("a document with no entry lists %+v %v; want nothing", attempts, err)
	}
}

// A retained entry that is not JSON, or that names another document, is
// refused as corrupt by every reader rather than read as some attempt list.
func TestADamagedOrForeignDesignEntryIsRefusedAsCorrupt(t *testing.T) {
	t.Parallel()

	for name, content := range map[string]string{
		"a damaged entry": "{not json",
		"a foreign entry": `{"goal":"g","recordId":"R","destination":"/somewhere/else.md","attempts":[{"attempt":1}]}`,
	} {
		m, _, request := designFixture(t)
		dir := m.designDir(request.Destination)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "request.json"), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := m.DesignAttempts(request.Destination); err == nil || !strings.Contains(err.Error(), "DESIGN_ENTRY_CORRUPT") {
			t.Errorf("%s listed with %v; want DESIGN_ENTRY_CORRUPT", name, err)
		}
		if _, _, err := m.DesignDocument(request.Destination); err == nil || !strings.Contains(err.Error(), "DESIGN_ENTRY_CORRUPT") {
			t.Errorf("%s read with %v; want DESIGN_ENTRY_CORRUPT", name, err)
		}
		if _, err := m.RecordDesignOutcome(request.Destination, 1, func(DesignAttempt) (string, string, string, error) {
			return "published", "", "", nil
		}); err == nil {
			t.Errorf("%s accepted an outcome", name)
		}
	}
}

// While another caller holds the document lock, recording an outcome is
// refused as busy and judges nothing. The lock is held through the same
// owner every design act takes, so nothing waits on time.
func TestDesignOutcomeIsBusyWhileTheDocumentIsLocked(t *testing.T) {
	t.Parallel()

	m, request := twoDesignAttempts(t)
	held, err := m.designLock(request.Destination)
	if err != nil {
		t.Fatalf("holding the document lock: %v", err)
	}
	defer releaseUnitLock(held)

	_, err = m.RecordDesignOutcome(request.Destination, 2, func(DesignAttempt) (string, string, string, error) {
		t.Fatalf("a locked document was judged")
		return "", "", "", nil
	})
	if err == nil || !strings.Contains(err.Error(), "DESIGN_BUSY") {
		t.Fatalf("recording during a held lock answered %v; want DESIGN_BUSY", err)
	}
}

// A cancellation recorded after the supervisor's claim but before its child
// starts finishes the launch as cancelled before any child exists, and a read
// launch so finished carries no verdict counts rather than zero verdicts.
func TestACancellationBeforeTheChildFinishesWithoutStartingIt(t *testing.T) {
	t.Parallel()

	for _, kind := range []string{"build", "read"} {
		m, processes, _, _ := manager(t)
		id := "cancel-before-child-" + kind
		seed(t, m, id, Starting)
		m.supervisorClaimed = func(claimed Record) {
			// The durable half of Cancel: the request lands on the claimed record.
			if _, err := m.Store.Update(claimed.ID, func(record *Record) error {
				record.Kind, record.Reason = kind, "cancel-requested"
				return nil
			}); err != nil {
				t.Errorf("recording the cancellation: %v", err)
			}
		}

		record, err := m.Supervise(id)

		if !errors.Is(err, errLaunchCancelledBeforeChild) {
			t.Fatalf("%s: supervising a cancelled launch answered %v; want cancelled-before-child", kind, err)
		}
		if record.State != Cancelled || record.Reason != "cancelled-before-child" || record.Child != nil ||
			record.FinishedAt == "" || record.OutputOwnerUnproven || len(processes.signals) != 0 {
			t.Fatalf("%s: the cancelled launch = %+v signals %v; want cancelled with no child", kind, record, processes.signals)
		}
		if counts := record.VerdictCounts; (kind == "read") != (counts != nil && !*counts) {
			t.Fatalf("%s: verdict counts = %v; want false only for a read", kind, counts)
		}
	}
}

// Listing launches answers every record as Status would: a running launch
// whose supervisor and child are both proven dead is failed as lost, and a
// launch still starting is answered unchanged. A store that has never been
// written lists nothing, and a store whose root is not absolute refuses.
func TestListingLaunchesAnswersEachAsItsStatus(t *testing.T) {
	t.Parallel()

	m, _, probe, _ := manager(t)
	seed(t, m, "list-running", Running)
	seed(t, m, "list-starting", Starting)
	probe.states[10], probe.states[20] = identity.Dead, identity.Dead

	records, err := m.List()
	if err != nil || len(records) != 2 {
		t.Fatalf("listing = %+v %v; want two records", records, err)
	}
	byID := map[string]Record{}
	for _, record := range records {
		byID[record.ID] = record
	}
	if lost := byID["list-running"]; lost.State != Failed || lost.Reason != "supervisor-lost" {
		t.Fatalf("the lost running launch listed as %+v; want failed supervisor-lost", lost)
	}
	if starting := byID["list-starting"]; starting.State != Starting {
		t.Fatalf("the starting launch listed as %+v; want it unchanged", starting)
	}

	empty := &Manager{Store: Store{Root: filepath.Join(t.TempDir(), "never-written")}}
	if records, err := empty.List(); err != nil || len(records) != 0 {
		t.Fatalf("an unwritten store listed %+v %v; want nothing", records, err)
	}
	relative := &Manager{Store: Store{Root: "relative/launches"}}
	if _, err := relative.List(); err == nil {
		t.Fatalf("a relative store root was listed")
	}
}
