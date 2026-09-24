package notifications

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

// journal writes a journal of its own making and answers with its path.
func journal(t *testing.T, lines ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "notifications.jsonl")
	body := ""
	for _, line := range lines {
		body += line + "\n"
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// line is one well-formed journal record. The ids are ordered the way the
// steward's are: as strings.
func line(t *testing.T, id, source, message string, delivered bool) string {
	t.Helper()
	record := map[string]any{
		"id": id, "at": "2026-09-23T10:31:00Z", "message": message,
		"source": source, "delivered": delivered,
	}
	if !delivered {
		record["error"] = "notification not delivered: exit status 1"
	}
	encoded, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

func appendLine(t *testing.T, path, text string) {
	t.Helper()
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString(text); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func ids(notices []Notice) []string {
	found := []string{}
	for _, notice := range notices {
		found = append(found, notice.ID)
	}
	return found
}

func equal(t *testing.T, what string, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: got %v, want %v", what, got, want)
	}
	for index := range got {
		if got[index] != want[index] {
			t.Fatalf("%s: got %v, want %v", what, got, want)
		}
	}
}

func TestPageAnswersNewestFirstAndCarriesEveryField(t *testing.T) {
	t.Parallel()
	path := journal(t,
		line(t, "A1", "steward", "first", true),
		line(t, "A2", "alert", "second", false),
		line(t, "A3", "handoff", "third", true),
	)
	page, err := Page(path, DefaultLimit, "")
	if err != nil {
		t.Fatal(err)
	}
	equal(t, "page", ids(page), []string{"A3", "A2", "A1"})
	undelivered := page[1]
	if undelivered.Delivered || undelivered.Error == "" || undelivered.Source != "alert" {
		t.Fatalf("the undelivered row lost what makes it undelivered: %+v", undelivered)
	}
	if page[0].At != "2026-09-23T10:31:00Z" || page[0].Message != "third" {
		t.Fatalf("a row lost its instant or its message: %+v", page[0])
	}
}

func TestPageHonoursItsLimitAndItsCeiling(t *testing.T) {
	t.Parallel()
	lines := []string{}
	for index := 0; index < 12; index++ {
		lines = append(lines, line(t, "B"+strconv.Itoa(index+10), "steward", "row", true))
	}
	path := journal(t, lines...)
	page, err := Page(path, 5, "")
	if err != nil {
		t.Fatal(err)
	}
	equal(t, "limited page", ids(page), []string{"B21", "B20", "B19", "B18", "B17"})

	whole, err := Page(path, MaxLimit*10, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(whole) != 12 {
		t.Fatalf("a limit above the ceiling did not answer the whole journal: %d", len(whole))
	}
	defaulted, err := Page(path, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(defaulted) != 12 {
		t.Fatalf("a limit of nothing did not fall back to the default: %d", len(defaulted))
	}
}

func TestPageBeforeAnIdIsTheOlderPage(t *testing.T) {
	t.Parallel()
	path := journal(t,
		line(t, "C1", "steward", "oldest", true),
		line(t, "C2", "steward", "older", true),
		line(t, "C3", "steward", "newer", true),
		line(t, "C4", "steward", "newest", true),
	)
	older, err := Page(path, 2, "C3")
	if err != nil {
		t.Fatal(err)
	}
	equal(t, "older page", ids(older), []string{"C2", "C1"})

	unknown, err := Page(path, 2, "NOPE")
	if err != nil {
		t.Fatal(err)
	}
	if len(unknown) != 0 {
		t.Fatalf("a before naming nothing paged something: %v", ids(unknown))
	}
}

func TestATornLastLineIsSkippedAndReadWhenItIsWhole(t *testing.T) {
	t.Parallel()
	path := journal(t, line(t, "D1", "steward", "whole", true))
	appendLine(t, path, `{"id":"D2","at":"2026-09-23T10:32:00Z","mess`)

	page, err := Page(path, DefaultLimit, "")
	if err != nil {
		t.Fatal(err)
	}
	equal(t, "page over a torn tail", ids(page), []string{"D1"})

	follower, behind, err := Open(path, "D1")
	if err != nil {
		t.Fatal(err)
	}
	if len(behind) != 0 {
		t.Fatalf("a torn fragment was served as a notice: %v", ids(behind))
	}
	appendLine(t, path, "age\":\"the rest of the write\",\"source\":\"steward\",\"delivered\":true}\n")
	arrived, err := follower.Next()
	if err != nil {
		t.Fatal(err)
	}
	equal(t, "the completed line", ids(arrived), []string{"D2"})
	if arrived[0].Message != "the rest of the write" {
		t.Fatalf("the completed line was not read whole: %+v", arrived[0])
	}
}

func TestALineThatDoesNotParseIsSkippedAndItsNeighboursAreNot(t *testing.T) {
	t.Parallel()
	path := journal(t,
		line(t, "E1", "steward", "before", true),
		`{"id":"E2",`,
		`{"at":"2026-09-23T10:31:00Z","message":"no id at all","delivered":true}`,
		"",
		line(t, "E3", "steward", "after", true),
	)
	page, err := Page(path, DefaultLimit, "")
	if err != nil {
		t.Fatal(err)
	}
	equal(t, "page around corruption", ids(page), []string{"E3", "E1"})
}

func TestAJournalThatDoesNotExistIsAnEmptyHistory(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "never-written.jsonl")
	page, err := Page(path, DefaultLimit, "")
	if err != nil {
		t.Fatalf("an absent journal is not an error: %v", err)
	}
	if len(page) != 0 {
		t.Fatalf("an absent journal answered with rows: %v", ids(page))
	}
	follower, behind, err := Open(path, "")
	if err != nil {
		t.Fatalf("an absent journal cannot be followed: %v", err)
	}
	if len(behind) != 0 {
		t.Fatalf("an absent journal answered with a backlog: %v", ids(behind))
	}
	appendLine(t, path, line(t, "F1", "steward", "the first delivery ever", true)+"\n")
	arrived, err := follower.Next()
	if err != nil {
		t.Fatal(err)
	}
	equal(t, "the journal's first line", ids(arrived), []string{"F1"})
}

func TestOpenResumesAfterTheIdItIsGivenAndStartsLiveWithoutOne(t *testing.T) {
	t.Parallel()
	path := journal(t,
		line(t, "G1", "steward", "one", true),
		line(t, "G2", "steward", "two", true),
		line(t, "G3", "steward", "three", true),
	)
	_, behind, err := Open(path, "G1")
	if err != nil {
		t.Fatal(err)
	}
	equal(t, "the backlog after G1", ids(behind), []string{"G2", "G3"})

	_, none, err := Open(path, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(none) != 0 {
		t.Fatalf("a fresh stream replayed the history: %v", ids(none))
	}
	_, unknown, err := Open(path, "NOT-IN-THIS-JOURNAL")
	if err != nil {
		t.Fatal(err)
	}
	if len(unknown) != 0 {
		t.Fatalf("an unknown resume point replayed the history: %v", ids(unknown))
	}
}

func TestAReplacedJournalIsReadFromItsBeginning(t *testing.T) {
	t.Parallel()
	path := journal(t,
		line(t, "H1", "steward", "one", true),
		line(t, "H2", "steward", "two", true),
	)
	follower, _, err := Open(path, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(line(t, "H3", "steward", "a shorter journal", true)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	arrived, err := follower.Next()
	if err != nil {
		t.Fatal(err)
	}
	equal(t, "the replaced journal", ids(arrived), []string{"H3"})
}

func TestFollowDeliversWhatArrivesAndEndsWithItsContext(t *testing.T) {
	t.Parallel()
	path := journal(t, line(t, "I1", "steward", "already here", true))
	follower, _, err := Open(path, "")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	batches := make(chan []Notice, 4)
	done := make(chan error, 1)
	go func() {
		done <- Follow(ctx, follower, time.Millisecond, func(notices []Notice) error {
			batches <- notices
			return nil
		})
	}()
	appendLine(t, path, line(t, "I2", "alert", "arrived while watching", true)+"\n")
	equal(t, "the followed batch", ids(<-batches), []string{"I2"})
	cancel()
	if err := <-done; err == nil {
		t.Fatal("a cancelled follow must end with its context's reason")
	}
}
