package httpd

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/notifications"
)

// The steward's notifications at the boundary: what the history answers, what
// the stream sends and when, and that both take the policy every other read
// takes. What the journal itself means is proved in internal/ui/notifications.

// journalWith writes a journal and answers with its path.
func journalWith(t *testing.T, lines ...string) string {
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

func journalLine(t *testing.T, id, source, message string, delivered bool) string {
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

func appendJournal(t *testing.T, path, text string) {
	t.Helper()
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString(text + "\n"); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func historyAnswer(t *testing.T, label string, response *httptest.ResponseRecorder) notificationsPayload {
	t.Helper()
	var payload notificationsPayload
	testutil.Require(t, label, json.Unmarshal(response.Body.Bytes(), &payload), nil)
	return payload
}

func TestNotificationHistoryAnswersNewestFirstWithItsUnreadMarkUnclaimed(t *testing.T) {
	t.Parallel()
	path := journalWith(t,
		journalLine(t, "N1", "steward", "the runner armed", true),
		journalLine(t, "N2", "alert", "HEALTH unhealthy — runner stale", false),
	)
	served := New(Info{NotificationJournal: path}, loopback(), testBundle())

	response := request(t, served, http.MethodGet, notificationsPath, "127.0.0.1:7878", nil)

	testutil.Require(t, "status", response.Code, http.StatusOK)
	testutil.Expect(t, "content type", response.Header().Get("Content-Type"), "application/json")
	testutil.Expect(t, "nothing is cached", response.Header().Get("Cache-Control"), "no-store")
	payload := historyAnswer(t, "decode the history", response)
	testutil.Require(t, "rows", len(payload.Notifications), 2)
	testutil.Expect(t, "newest first", payload.Notifications[0].ID, "N2")
	testutil.Expect(t, "its source", payload.Notifications[0].Source, "alert")
	testutil.Expect(t, "the delivery gate, visible", payload.Notifications[0].Delivered, false)
	if payload.Notifications[0].Error == "" {
		t.Fatal("an undelivered row carried no reason")
	}
	if payload.UnreadFrom != nil {
		t.Fatalf("the server claimed to know what a viewer has seen: %v", *payload.UnreadFrom)
	}
	// The raw body carries the field, rather than omitting it.
	if !strings.Contains(response.Body.String(), `"unreadFrom":null`) {
		t.Fatalf("the unread mark was omitted instead of answered: %s", response.Body.String())
	}
}

func TestNotificationHistoryPagesOlderByBefore(t *testing.T) {
	t.Parallel()
	path := journalWith(t,
		journalLine(t, "P1", "steward", "oldest", true),
		journalLine(t, "P2", "steward", "older", true),
		journalLine(t, "P3", "steward", "newest", true),
	)
	served := New(Info{NotificationJournal: path}, loopback(), testBundle())

	first := historyAnswer(t, "decode the first page", request(t, served, http.MethodGet, notificationsPath+"?limit=1", "127.0.0.1:7878", nil))
	testutil.Require(t, "one row", len(first.Notifications), 1)
	testutil.Expect(t, "the newest", first.Notifications[0].ID, "P3")

	older := historyAnswer(t, "decode the older page", request(t, served, http.MethodGet,
		notificationsPath+"?limit=200&before="+first.Notifications[0].ID, "127.0.0.1:7878", nil))
	testutil.Require(t, "the older page", len(older.Notifications), 2)
	testutil.Expect(t, "still newest first", older.Notifications[0].ID, "P2")
	testutil.Expect(t, "down to the oldest", older.Notifications[1].ID, "P1")
}

func TestAnEngineWithNoJournalSaysSoOnBothRoutes(t *testing.T) {
	t.Parallel()
	served := New(Info{}, loopback(), testBundle())
	for _, path := range []string{notificationsPath, notificationsStreamPath} {
		response := request(t, served, http.MethodGet, path, "127.0.0.1:7878", nil)
		testutil.Expect(t, "status for "+path, response.Code, http.StatusInternalServerError)
		if !strings.Contains(response.Body.String(), "without a notification journal") {
			t.Fatalf("%s did not say why it cannot answer: %s", path, response.Body.String())
		}
	}
}

func TestNotificationRoutesTakeTheSamePolicyAsEveryOtherRead(t *testing.T) {
	t.Parallel()
	path := journalWith(t, journalLine(t, "Q1", "steward", "one", true))
	served := New(Info{NotificationJournal: path}, loopback(), testBundle())

	for _, resource := range []string{notificationsPath, notificationsStreamPath} {
		foreign := request(t, served, http.MethodGet, resource, "attacker.invalid", nil)
		testutil.Expect(t, "a foreign host is refused on "+resource, foreign.Code, http.StatusForbidden)

		crossSite := request(t, served, http.MethodGet, resource, "127.0.0.1:7878",
			map[string]string{"Sec-Fetch-Site": "cross-site"})
		testutil.Expect(t, "a cross-site fetch is refused on "+resource, crossSite.Code, http.StatusForbidden)

		written := request(t, served, http.MethodPost, resource, "127.0.0.1:7878", nil)
		testutil.Expect(t, "a write is refused on "+resource, written.Code, http.StatusMethodNotAllowed)
		testutil.Expect(t, "what "+resource+" takes instead", written.Header().Get("Allow"), "GET, HEAD")
	}

	// Nothing lives beneath either resource.
	beneath := request(t, served, http.MethodGet, notificationsPath+"/2", "127.0.0.1:7878", nil)
	testutil.Expect(t, "an unserved path under /api", beneath.Code, http.StatusNotFound)
}

// opened is one live stream, and the one goroutine reading it.
//
// One reader, because two would race for the same lines: a second reader
// started for the second event would find the first had already taken it off
// the connection and buffered it where nobody looks.
type opened struct {
	lines    chan string
	problems chan error
	stop     func()
}

// streamed runs the stream against a real server, so the response is written
// as it is produced rather than buffered into a recorder.
func streamed(t *testing.T, path string, lastEventID string) *opened {
	t.Helper()
	served := httptest.NewServer(New(Info{NotificationJournal: path}, loopback(), testBundle()))
	ctx, cancel := context.WithCancel(context.Background())
	asked, err := http.NewRequestWithContext(ctx, http.MethodGet, served.URL+notificationsStreamPath, nil)
	if err != nil {
		cancel()
		served.Close()
		t.Fatal(err)
	}
	// The test server binds a port of its own; the handler's allowed hosts are
	// the ones it was constructed with, so the request names one of those.
	asked.Host = "127.0.0.1:7878"
	if lastEventID != "" {
		asked.Header.Set("Last-Event-ID", lastEventID)
	}
	response, err := served.Client().Do(asked)
	if err != nil {
		cancel()
		served.Close()
		t.Fatal(err)
	}
	testutil.Expect(t, "status", response.StatusCode, http.StatusOK)
	testutil.Expect(t, "content type", response.Header.Get("Content-Type"), "text/event-stream")
	testutil.Expect(t, "nothing is cached", response.Header.Get("Cache-Control"), "no-store")
	stream := &opened{lines: make(chan string, 256), problems: make(chan error, 1)}
	reader := bufio.NewReader(response.Body)
	go func() {
		for {
			line, err := reader.ReadString('\n')
			if line != "" {
				stream.lines <- strings.TrimRight(line, "\n")
			}
			if err != nil {
				stream.problems <- err
				return
			}
		}
	}()
	stream.stop = func() {
		cancel()
		_ = response.Body.Close()
		served.Close()
	}
	t.Cleanup(stream.stop)
	return stream
}

// line is the next line the stream sent. A stream that says nothing is the
// bug, so this fails rather than waiting forever.
func (o *opened) line(t *testing.T) string {
	t.Helper()
	select {
	case line := <-o.lines:
		return line
	case err := <-o.problems:
		t.Fatalf("the stream ended: %v", err)
	case <-time.After(10 * time.Second):
		t.Fatal("the stream said nothing")
	}
	return ""
}

// event is the next complete event, with the comments between them skipped.
func (o *opened) event(t *testing.T) (id string, name string, data string) {
	t.Helper()
	for {
		line := o.line(t)
		switch {
		case strings.HasPrefix(line, "id: "):
			id = strings.TrimPrefix(line, "id: ")
		case strings.HasPrefix(line, "event: "):
			name = strings.TrimPrefix(line, "event: ")
		case strings.HasPrefix(line, "data: "):
			data = strings.TrimPrefix(line, "data: ")
		case line == "" && name != "":
			return id, name, data
		}
	}
}

// The stream opens quietly. A page that has just loaded its history must not
// be told about every message it already shows, which would raise a toast for
// each of them.
func TestTheStreamOpensWithACommentAndSendsOnlyWhatArrives(t *testing.T) {
	t.Parallel()
	previous := notificationTick
	notificationTick = 5 * time.Millisecond
	t.Cleanup(func() { notificationTick = previous })

	path := journalWith(t,
		journalLine(t, "S1", "steward", "already in the history", true),
		journalLine(t, "S2", "steward", "also already there", true),
	)
	stream := streamed(t, path, "")

	if opening := stream.line(t); !strings.HasPrefix(opening, ": ") {
		t.Fatalf("the stream did not open with a comment: %q", opening)
	}
	if retry := stream.line(t); !strings.HasPrefix(retry, "retry: ") {
		t.Fatalf("the stream did not offer a retry hint: %q", retry)
	}

	appendJournal(t, path, journalLine(t, "S3", "alert", "HEALTH unhealthy — runner stale", false))
	id, name, data := stream.event(t)
	testutil.Expect(t, "the event's id", id, "S3")
	testutil.Expect(t, "the event's name", name, "notification")
	var notice notifications.Notice
	testutil.Require(t, "decode the event", json.Unmarshal([]byte(data), &notice), nil)
	testutil.Expect(t, "the message", notice.Message, "HEALTH unhealthy — runner stale")
	testutil.Expect(t, "the source", notice.Source, "alert")
	testutil.Expect(t, "the delivery gate, visible", notice.Delivered, false)
}

// A reconnecting browser sends back the last id it received, and the stream
// gives it exactly what it missed.
func TestTheStreamResumesFromTheLastEventTheBrowserSaw(t *testing.T) {
	t.Parallel()
	previous := notificationTick
	notificationTick = 5 * time.Millisecond
	t.Cleanup(func() { notificationTick = previous })

	path := journalWith(t,
		journalLine(t, "R1", "steward", "before the drop", true),
		journalLine(t, "R2", "handoff", "missed while disconnected", true),
		journalLine(t, "R3", "steward", "missed too", true),
	)
	stream := streamed(t, path, "R1")

	firstID, _, _ := stream.event(t)
	testutil.Expect(t, "the first thing it missed", firstID, "R2")
	secondID, _, _ := stream.event(t)
	testutil.Expect(t, "the second thing it missed", secondID, "R3")

	appendJournal(t, path, journalLine(t, "R4", "steward", "and then live", true))
	liveID, _, _ := stream.event(t)
	testutil.Expect(t, "and then what arrives", liveID, "R4")
}

// The heartbeat is a comment. It keeps a silent connection warm and reaches no
// listener on the page.
func TestTheStreamBeatsWithAComment(t *testing.T) {
	t.Parallel()
	previousTick, previousBeat := notificationTick, notificationHeartbeat
	notificationTick, notificationHeartbeat = 5*time.Millisecond, 10*time.Millisecond
	t.Cleanup(func() { notificationTick, notificationHeartbeat = previousTick, previousBeat })

	path := journalWith(t, journalLine(t, "B1", "steward", "quiet", true))
	stream := streamed(t, path, "")

	// The opening comment and the retry hint, then the blank line that ends
	// the opening; the next ": " line is a beat.
	stream.line(t)
	stream.line(t)
	for {
		if strings.HasPrefix(stream.line(t), ": ") {
			return
		}
	}
}
