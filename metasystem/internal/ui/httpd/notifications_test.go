package httpd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/decisions"
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

// journalAt is one line recorded at a stated instant, for a reader that is
// about when a line was written rather than about what it says.
func journalAt(t *testing.T, id, source, message string, at time.Time) string {
	t.Helper()
	encoded, err := json.Marshal(map[string]any{
		"id": id, "at": at.UTC().Format(time.RFC3339), "message": message,
		"source": source, "delivered": true,
	})
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
		journalLine(t, "N2", "alert", "the steward could not reach the operator", false),
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
	seen     []string
	lines    chan string
	problems chan error
	stop     func()
}

// streamed runs the stream against a real server, so the response is written
// as it is produced rather than buffered into a recorder. Its handler polls the
// journal every few milliseconds, and that clock is this test's alone.
func streamed(t *testing.T, path string, lastEventID string) *opened {
	t.Helper()
	return streamedOver(t, fastStream(Info{NotificationJournal: path}), lastEventID)
}

// fastStream is a handler whose stream polls in milliseconds rather than
// seconds. It is set before the handler serves anything, on this handler only.
func fastStream(info Info) *handler {
	served := newHandler(info, loopback(), testBundle(), randomNonce)
	served.streamTick = 5 * time.Millisecond
	return served
}

// streamedFrom is streamed over a server this test composed itself, which is
// how the Partner's half of the one stream is opened.
func streamedFrom(t *testing.T, info Info, lastEventID string) *opened {
	t.Helper()
	return streamedOver(t, newHandler(info, loopback(), testBundle(), randomNonce), lastEventID)
}

func streamedOver(t *testing.T, h *handler, lastEventID string) *opened {
	t.Helper()
	served := httptest.NewServer(h)
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
				// A test that has stopped reading stops this reader too.
				select {
				case stream.lines <- strings.TrimRight(line, "\n"):
				case <-ctx.Done():
					return
				}
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

// line waits for the next line within the test binary’s deadline and names
// the lines already received if the stream stops making progress.
func (o *opened) line(t testenv.AwaitTB) string {
	t.Helper()
	var next string
	testenv.AwaitOr(t, "the next stream line", func() bool {
		select {
		case next = <-o.lines:
			o.seen = append(o.seen, next)
			return true
		case err := <-o.problems:
			t.Fatalf("the stream ended: %v; lines seen: %q", err, o.seen)
		default:
		}
		return false
	}, func() string { return fmt.Sprintf("lines seen: %q", o.seen) })
	return next
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

	appendJournal(t, path, journalLine(t, "S3", "alert", "the steward could not reach the operator", false))
	id, name, data := stream.event(t)
	testutil.Expect(t, "the event's id", id, "S3")
	testutil.Expect(t, "the event's name", name, "notification")
	var notice notifications.Notice
	testutil.Require(t, "decode the event", json.Unmarshal([]byte(data), &notice), nil)
	testutil.Expect(t, "the message", notice.Message, "the steward could not reach the operator")
	testutil.Expect(t, "the source", notice.Source, "alert")
	testutil.Expect(t, "the delivery gate, visible", notice.Delivered, false)
}

// A reconnecting browser sends back the last id it received, and the stream
// gives it exactly what it missed.
func TestTheStreamResumesFromTheLastEventTheBrowserSaw(t *testing.T) {
	t.Parallel()
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
	path := journalWith(t, journalLine(t, "B1", "steward", "quiet", true))
	served := fastStream(Info{NotificationJournal: path})
	served.streamHeartbeat = 10 * time.Millisecond
	stream := streamedOver(t, served, "")

	// The opening comment and the retry hint, then the blank line that ends
	// the opening; the next ": " line is a beat.
	stream.line(t)
	stream.line(t)
	for {
		select {
		case line := <-stream.lines:
			if strings.HasPrefix(line, ": ") {
				return
			}
		case err := <-stream.problems:
			t.Fatalf("the stream ended: %v", err)
		}
	}
}

// The steward's raw health verdict is an operator's reading, read on purpose
// through system status and the logs. It is never a person's notification: not
// in the history, not down the stream, and not in what the inbox counts
// (owner's rule, 2026-10-03). A notice beside it that a person can act on
// still arrives everywhere.
func TestARawHealthVerdictIsNoPersonsNotification(t *testing.T) {
	t.Parallel()
	const raw = "HEALTH unhealthy — steward-runner=alive (pid 4242); trunk-red=dead (remedy: run it)"
	const actionable = "seat m1e has been idle with approved work for 40 minutes"
	at := overviewNow.Add(-time.Hour)
	path := journalWith(t,
		journalAt(t, "H0", "steward", "the runner armed", at),
		journalAt(t, "H1", "alert", raw, at),
		journalAt(t, "H2", "alert", actionable, at),
	)

	history := historyAnswer(t, "decode the history", request(t,
		New(Info{NotificationJournal: path}, loopback(), testBundle()),
		http.MethodGet, notificationsPath, "127.0.0.1:7878", nil))
	got := []string{}
	for _, notice := range history.Notifications {
		got = append(got, notice.ID)
	}
	testutil.Expect(t, "the history", strings.Join(got, ","), "H2,H0")

	// A resuming stream is given what it missed, and the verdict is not in it;
	// a live stream is given what arrives, and the verdict is not in that.
	stream := streamed(t, path, "H0")
	id, _, _ := stream.event(t)
	testutil.Expect(t, "what the resumed stream missed", id, "H2")
	appendJournal(t, path, journalAt(t, "H3", "alert", "HEALTH STOPPED healthy — steward-runner=alive", at))
	appendJournal(t, path, journalAt(t, "H4", "alert", actionable, at))
	id, _, data := stream.event(t)
	testutil.Expect(t, "what the live stream sends next", id, "H4")
	testutil.Expect(t, "and no verdict rides inside it", strings.Contains(data, "HEALTH"), false)

	info := decisionsInfo()
	info.NotificationJournal = journalWith(t,
		journalAt(t, "D1", "alert", raw, at),
		journalAt(t, "D2", "alert", actionable, at),
	)
	page := decisionsPage(t, New(info, loopback(), testBundle()), "the read")
	alerts := []string{}
	for _, need := range page.NeedsYou {
		if need.Kind == decisions.KindAlert {
			alerts = append(alerts, need.ID)
		}
	}
	testutil.Expect(t, "the alerts the inbox carries", strings.Join(alerts, ","), "D2")
	// The count is the count of the same inbox with the verdict never written.
	without := decisionsInfo()
	without.NotificationJournal = journalWith(t, journalAt(t, "D2", "alert", actionable, at))
	baseline := decisionsPage(t, New(without, loopback(), testBundle()), "the read without the verdict")
	testutil.Expect(t, "and what it counts", page.Counts.NeedsYou, baseline.Counts.NeedsYou)
}

// The deadline belongs to the test binary; an expired fixture deadline
// proves the read’s failure path without waiting for elapsed time.
type expiredStreamDeadline struct{ failure string }

func (*expiredStreamDeadline) Helper()                     {}
func (*expiredStreamDeadline) Deadline() (time.Time, bool) { return time.Unix(1, 0), true }
func (d *expiredStreamDeadline) Fatalf(format string, args ...any) {
	d.failure = fmt.Sprintf(format, args...)
	panic(d)
}

func TestStreamLineDeadlineReportsTheLinesAlreadySeen(t *testing.T) {
	t.Parallel()
	stream := &opened{lines: make(chan string, 1), problems: make(chan error, 1)}
	stream.lines <- "event: partner"
	testutil.Expect(t, "the available line is read", stream.line(t), "event: partner")
	deadline := &expiredStreamDeadline{}
	func() {
		defer func() {
			if got := recover(); got != deadline {
				t.Fatalf("deadline failure = %v, want the fixture failure", got)
			}
		}()
		stream.line(deadline)
	}()
	if !strings.Contains(deadline.failure, "still awaiting the next stream line") ||
		!strings.Contains(deadline.failure, `lines seen: ["event: partner"]`) {
		t.Fatalf("stream deadline lost its awaited event or received lines: %s", deadline.failure)
	}
}
