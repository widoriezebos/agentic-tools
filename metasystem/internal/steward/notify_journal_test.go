package steward

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

// journalOf reads every line the journal holds for one repository. A torn or
// absent file is a failure here: these tests write the journal themselves.
func journalOf(t *testing.T, root string) []journalRecord {
	t.Helper()
	data, err := os.ReadFile(NotificationJournalPath(root))
	if err != nil {
		t.Fatalf("the journal was not written: %v", err)
	}
	records := []journalRecord{}
	for _, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
		var record journalRecord
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("journal line is not JSON: %q %v", line, err)
		}
		records = append(records, record)
	}
	return records
}

func TestEveryDeliveryPathLeavesOneJournalLine(t *testing.T) {
	t.Parallel()
	root, sink := notifyRepo(t, "")
	if err := Deliver(root, "the steward speaks"); err != nil {
		t.Fatal(err)
	}
	if err := DeliverNotice(root, Notice{
		Message: "HEALTH unhealthy — runner stale", Source: NoticeAlert, Ref: "alert-abc-1",
	}); err != nil {
		t.Fatal(err)
	}
	for _, nonce := range []string{"handoff-5000", "verdict-stalled", "reap-7"} {
		if err := QueueNotification(root, PendingNotification{Nonce: nonce, Message: "msg-" + nonce}); err != nil {
			t.Fatal(err)
		}
	}
	// The handoff nonce names no live bound intent, so it retires without a
	// delivery; the other two go through the pending path.
	if _, err := DeliverPending(root); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(sink); err != nil || !strings.Contains(string(data), "the steward speaks") {
		t.Fatalf("the notifier did not receive the message: %q %v", data, err)
	}

	records := journalOf(t, root)
	if len(records) != 4 {
		t.Fatalf("every attempt journals exactly one line: %+v", records)
	}
	for _, record := range records {
		if !record.Delivered || record.Error != "" {
			t.Fatalf("a delivered attempt journaled as undelivered: %+v", record)
		}
		if record.ID == "" || record.At == "" || record.Message == "" {
			t.Fatalf("a journal line is missing its id, instant or message: %+v", record)
		}
		if _, err := time.Parse(time.RFC3339, record.At); err != nil {
			t.Fatalf("a journal instant is not RFC3339: %q %v", record.At, err)
		}
	}
	// The queue drains in the order the pending directory lists it, which is
	// not the order it was filled in, so the pairs are compared as a set.
	pairs := []string{}
	for _, record := range records {
		pairs = append(pairs, record.Source+"/"+record.Ref)
	}
	sort.Strings(pairs)
	want := []string{"alert/alert-abc-1", "steward/", "steward/reap-7", "verdict/verdict-stalled"}
	if strings.Join(pairs, " ") != strings.Join(want, " ") {
		t.Fatalf("the sources and references did not follow the path each message took: %v", pairs)
	}
}

func TestAFailedDeliveryIsJournaledWithItsReason(t *testing.T) {
	t.Parallel()
	root, _ := notifyRepo(t, "exit 1")
	if err := DeliverNotice(root, Notice{Message: "nobody heard this", Source: NoticeSteward}); err == nil {
		t.Fatal("a failing notifier must not claim delivery")
	}
	records := journalOf(t, root)
	if len(records) != 1 {
		t.Fatalf("a failed attempt journals one line: %+v", records)
	}
	if records[0].Delivered {
		t.Fatalf("a failed attempt journaled as delivered: %+v", records[0])
	}
	if !strings.Contains(records[0].Error, "notification not delivered") {
		t.Fatalf("the journal did not carry the reason: %+v", records[0])
	}
	if records[0].Message != "nobody heard this" {
		t.Fatalf("the journal did not carry the message: %+v", records[0])
	}
}

func TestTheFixtureLogKeepsItsOwnLineBesideTheJournal(t *testing.T) {
	t.Parallel()
	root := notifyRepoWithoutCommand(t)
	notifyIdentity(t, root, EnrollmentFixture)
	if err := DeliverNotice(root, Notice{Message: "HEALTH unhealthy", Source: NoticeSteward}); err != nil {
		t.Fatal(err)
	}
	log, err := os.ReadFile(filepath.Join(root, "artifacts", "agents", "steward", "notifications.log"))
	if err != nil || !strings.Contains(string(log), "HEALTH unhealthy") {
		t.Fatalf("the fixture log lost its own line: %q %v", log, err)
	}
	records := journalOf(t, root)
	if len(records) != 1 || !records[0].Delivered {
		t.Fatalf("the fixture delivery did not journal: %+v", records)
	}
}

func TestAJournalThatCannotBeWrittenDoesNotFailTheDelivery(t *testing.T) {
	t.Parallel()
	root, sink := notifyRepo(t, "")
	// A file where the steward's directory belongs: the journal cannot be
	// created, and the operator must still be reached.
	if err := os.MkdirAll(filepath.Join(root, "artifacts", "agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "artifacts", "agents", "steward"), []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Deliver(root, "the channel still works"); err != nil {
		t.Fatalf("a journal that cannot be written must not gate delivery: %v", err)
	}
	if data, err := os.ReadFile(sink); err != nil || !strings.Contains(string(data), "the channel still works") {
		t.Fatalf("the message did not reach the notifier: %q %v", data, err)
	}
}

func TestJournalIdentitiesIncrease(t *testing.T) {
	t.Parallel()
	root, _ := notifyRepo(t, "")
	for index := 0; index < 8; index++ {
		if err := Deliver(root, "line"); err != nil {
			t.Fatal(err)
		}
	}
	records := journalOf(t, root)
	if len(records) != 8 {
		t.Fatalf("eight deliveries journal eight lines: %+v", records)
	}
	for index := 1; index < len(records); index++ {
		if records[index].ID <= records[index-1].ID {
			t.Fatalf("ids did not increase: %q then %q", records[index-1].ID, records[index].ID)
		}
	}
	for _, record := range records {
		if len(record.ID) != 26 {
			t.Fatalf("an id is not twenty-six characters: %q", record.ID)
		}
		for _, character := range record.ID {
			if !strings.ContainsRune(notificationIDAlphabet, character) {
				t.Fatalf("an id is not Crockford base32: %q", record.ID)
			}
		}
	}
}

func TestIdentitiesIncreaseWithinOneMillisecondAndAcrossThem(t *testing.T) {
	t.Parallel()
	// The minting seam is package state, so this subtest owns the clock it
	// reads rather than racing another test for it. It is the one place the
	// encoding is checked without the randomness.
	frozen := time.Date(2026, 9, 23, 10, 31, 0, 0, time.UTC)
	first := mintNotificationID(frozen)
	second := mintNotificationID(frozen)
	later := mintNotificationID(frozen.Add(time.Millisecond))
	if first[:10] != second[:10] {
		t.Fatalf("two ids in one millisecond disagree about the millisecond: %q %q", first, second)
	}
	if later[:10] <= first[:10] {
		t.Fatalf("a later millisecond did not sort above an earlier one: %q %q", first, later)
	}
	stepped := afterNotificationID(first)
	if stepped <= first {
		t.Fatalf("the monotonic step did not increase the id: %q then %q", first, stepped)
	}
	if len(stepped) != len(first) {
		t.Fatalf("the monotonic step changed the id's length: %q then %q", first, stepped)
	}
	carried := afterNotificationID("0000000000000000000000000Z")
	if carried != "00000000000000000000000010" {
		t.Fatalf("the step did not carry into the character before it: %q", carried)
	}
}
