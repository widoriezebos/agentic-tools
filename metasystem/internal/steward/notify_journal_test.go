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
	f := configuredNotifyFixture(t, 4, "default")
	root, sink := f.root, f.sink
	if err := deliverNoticeWith(root, Notice{Message: "the steward speaks", Source: NoticeSteward}, f.deliver); err != nil {
		t.Fatal(err)
	}
	if err := deliverNoticeWith(root, Notice{
		Message: "HEALTH unhealthy — runner stale", Source: NoticeAlert, Ref: "alert-abc-1",
	}, f.deliver); err != nil {
		t.Fatal(err)
	}
	for _, nonce := range []string{"handoff-5000", "verdict-stalled", "reap-7"} {
		if err := QueueNotification(root, PendingNotification{Nonce: nonce, Message: "msg-" + nonce}); err != nil {
			t.Fatal(err)
		}
	}
	// The handoff nonce names no live bound intent, so it retires without a
	// delivery; the other two go through the pending path.
	if _, err := f.pending(); err != nil {
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
	f := configuredNotifyFixture(t, 1, "exit 1")
	root := f.root
	if err := deliverNoticeWith(root, Notice{Message: "nobody heard this", Source: NoticeSteward}, f.deliver); err == nil {
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

func TestAlertRetriesAndSpendSubmissionJournalTheirEpisodeIDs(t *testing.T) {
	t.Parallel()
	f := newNotifyFixture(t,
		notifyRead{command: "exit 1"},
		notifyRead{command: "default"},
		notifyRead{command: "default"},
	)
	now := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)
	health := HealthVerdict{Aggregate: "unhealthy", FindingDigest: evidenceDigest("runner stale"), ShouldAlert: true}
	failed, err := f.alert(health, "HEALTH unhealthy", now)
	if err != nil || failed.TransportResult != TransportFailed {
		t.Fatalf("first alert attempt did not fail: %+v %v", failed, err)
	}
	retried, err := f.alert(health, "HEALTH unhealthy", now.Add(time.Minute))
	if err != nil || retried.TransportResult != TransportSubmitted || retried.EpisodeID != failed.EpisodeID {
		t.Fatalf("alert retry did not submit the same episode: %+v %v", retried, err)
	}
	observation := SpendObservation{Valid: true, Crossings: []SpendCrossing{
		crossing("day-2026-09-23", "day", "tokens", 1, 125, 100),
	}}
	if err := updateSpendEpisodesWith(f.root, observation, now.Add(2*time.Minute), f.deliver); err != nil {
		t.Fatal(err)
	}
	episodes, err := AlertEpisodes(f.root)
	if err != nil {
		t.Fatal(err)
	}
	var spendID string
	for _, episode := range episodes {
		if episode.Owner == string(RoleSpendFence) {
			spendID = episode.EpisodeID
		}
	}
	if spendID == "" {
		t.Fatal("spend submission did not retain an episode")
	}
	records := journalOf(t, f.root)
	if len(records) != 3 {
		t.Fatalf("three transport attempts must leave three journal lines: %+v", records)
	}
	for index, want := range []struct {
		ref       string
		delivered bool
	}{{failed.EpisodeID, false}, {failed.EpisodeID, true}, {spendID, true}} {
		if records[index].Source != NoticeAlert || records[index].Ref != want.ref || records[index].Delivered != want.delivered {
			t.Fatalf("journal attempt %d lost its episode or result: %+v", index, records[index])
		}
	}
	if records[0].Error == "" || records[1].Error != "" || records[2].Error != "" {
		t.Fatalf("journal reasons did not follow transport results: %+v", records)
	}
}

func TestFailedPendingDeliveryKeepsItsNonceAndJournalReason(t *testing.T) {
	t.Parallel()
	f := configuredNotifyFixture(t, 1, "exit 1")
	nonce := "verdict-stalled"
	if err := QueueNotification(f.root, PendingNotification{Nonce: nonce, Message: "steward: stalled"}); err != nil {
		t.Fatal(err)
	}
	if delivered, err := f.pending(); err == nil || delivered != 0 {
		t.Fatalf("failed pending delivery did not stop the pass: delivered=%d err=%v", delivered, err)
	}
	records := journalOf(t, f.root)
	if len(records) != 1 || records[0].Source != NoticeVerdict || records[0].Ref != nonce || records[0].Delivered || records[0].Error == "" {
		t.Fatalf("failed pending attempt lost its source, nonce, or reason: %+v", records)
	}
	if pending, err := PendingNotifications(f.root); err != nil || len(pending) != 1 || pending[0].Nonce != nonce {
		t.Fatalf("failed pending attempt did not retain the queue entry: %+v %v", pending, err)
	}
}

func TestTheFixtureLogKeepsItsOwnLineBesideTheJournal(t *testing.T) {
	t.Parallel()
	f := absentNotifyFixture(t, 1)
	root := f.root
	notifyIdentity(t, root, EnrollmentFixture)
	if err := deliverNoticeWith(root, Notice{Message: "HEALTH unhealthy", Source: NoticeSteward}, f.deliver); err != nil {
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
	f := configuredNotifyFixture(t, 1, "default")
	root, sink := f.root, f.sink
	// A file where the steward's directory belongs: the journal cannot be
	// created, and the operator must still be reached.
	if err := os.MkdirAll(filepath.Join(root, "artifacts", "agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "artifacts", "agents", "steward"), []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := deliverNoticeWith(root, Notice{Message: "the channel still works"}, f.deliver); err != nil {
		t.Fatalf("a journal that cannot be written must not gate delivery: %v", err)
	}
	if data, err := os.ReadFile(sink); err != nil || !strings.Contains(string(data), "the channel still works") {
		t.Fatalf("the message did not reach the notifier: %q %v", data, err)
	}
}

func TestJournalIdentitiesIncrease(t *testing.T) {
	t.Parallel()
	f := configuredNotifyFixture(t, 8, "default")
	root := f.root
	for index := 0; index < 8; index++ {
		if err := deliverNoticeWith(root, Notice{Message: "line"}, f.deliver); err != nil {
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
