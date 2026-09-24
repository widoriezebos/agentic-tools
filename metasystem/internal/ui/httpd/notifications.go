package httpd

// The steward's notifications, as the interface reads them.
//
// The steward already reaches the operator: on a Mac it raises a notification
// centre toast, and everywhere else it runs the configured command. That
// channel does not change here and is not routed through anything below. What
// changes is that every attempt is also written down, and these two routes are
// how a page that is open reads what was written.
//
// There are two readings and they answer different questions. The history is
// "what has the steward said?", and it is a page of the journal, newest first,
// with an older page behind a `before`. The stream is "what is it saying
// now?", and it is Server-Sent Events: the page holds one for its life, the
// browser reconnects it when the connection drops, and the Last-Event-ID the
// browser sends back is how a reconnect resumes where the page left off
// instead of replaying everything or losing what it missed.
//
// A stream is a long-lived GET and nothing more. It takes the same policy as
// every other read — loopback, this origin, GET — and authenticates nothing
// beyond that, because under the ui.session server there is nothing else to
// authenticate: reaching this port already means being on this machine.

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/notifications"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/partner"
)

// The two resources, matched exactly: what lies beneath them belongs to no
// resource, so it is a 404 like any other unserved path under /api.
const (
	notificationsPath       = "/api/notifications"
	notificationsStreamPath = "/api/notifications/stream"
)

// How the stream behaves in time. The journal is polled rather than watched:
// one stat a second per open stream, which costs nothing and needs no platform
// facility. The heartbeat is a comment, so it reaches no listener on the page
// and exists only to keep a silent connection from being reaped. The retry
// hint is what the browser waits before reconnecting after a drop.
//
// The tick and the heartbeat are the defaults every handler starts with; a
// test that wants the stream in milliseconds sets them on its own handler, so
// parallel streams never share a clock.
const (
	notificationTick      = time.Second
	notificationHeartbeat = 25 * time.Second
	notificationRetry     = 3 * time.Second
)

// notificationsPayload is the history resource.
//
// unreadFrom is always null in this build and is written all the same: what a
// viewer has seen is the viewer's, kept in their own browser, and the server
// has no opinion about it. The field is here so that a build which learns to
// remember it per human does not change the shape of this answer.
type notificationsPayload struct {
	Notifications []notifications.Notice `json:"notifications"`
	UnreadFrom    *string                `json:"unreadFrom"`
}

// notifications answers a page of the journal. A journal that has never been
// written is an empty history and a 200: an installation that has never
// notified anybody has said nothing, which is an answer.
func (h *handler) notifications(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if h.info.NotificationJournal == "" {
		writeFailure(w, "this engine was built without a notification journal")
		return
	}
	query := r.URL.Query()
	page, err := notifications.Page(h.info.NotificationJournal,
		notificationLimit(query.Get("limit")), query.Get("before"))
	if err != nil {
		writeFailure(w, err.Error())
		return
	}
	_ = json.NewEncoder(w).Encode(notificationsPayload{Notifications: page, UnreadFrom: nil})
}

// notificationLimit reads the page size. Anything that is not a number is no
// request for a size at all, which is the default; the package clamps the
// rest.
func notificationLimit(asked string) int {
	if asked == "" {
		return notifications.DefaultLimit
	}
	limit, err := strconv.Atoi(asked)
	if err != nil {
		return notifications.DefaultLimit
	}
	return limit
}

// notificationStream is the live reading.
//
// It writes the opening comment and the retry hint and nothing else, so a page
// that connects fresh receives no history down the stream — the history route
// is what the page loads history from, and a stream that replayed it would
// raise a toast for every message the steward ever sent. A reconnect is the
// exception and the reason Last-Event-ID exists: the page already has
// everything up to that id, and what it is missing is exactly what follows it.
func (h *handler) notificationStream(w http.ResponseWriter, r *http.Request) {
	if h.info.NotificationJournal == "" {
		w.Header().Set("Content-Type", "application/json")
		writeFailure(w, "this engine was built without a notification journal")
		return
	}
	flusher, streamable := w.(http.Flusher)
	if !streamable {
		w.Header().Set("Content-Type", "application/json")
		writeFailure(w, "this server cannot hold a stream open")
		return
	}
	follower, behind, err := notifications.Open(h.info.NotificationJournal, r.Header.Get("Last-Event-ID"))
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		writeFailure(w, err.Error())
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	// A HEAD asks what this resource is, not for its contents. Holding one
	// open would hold a connection for a client that is never going to read
	// from it.
	if r.Method == http.MethodHead {
		return
	}
	_, _ = io.WriteString(w, ": the steward's notifications\nretry: "+
		strconv.FormatInt(notificationRetry.Milliseconds(), 10)+"\n\n")
	flusher.Flush()
	if !writeNotices(w, flusher, behind) {
		return
	}

	// One goroutine per stream watches the journal; this one writes. They are
	// separate so that the heartbeat is sent on its own clock rather than
	// between two polls, and both end with the request's context: the client
	// going away cancels it, and the follow returns.
	ctx := r.Context()
	arrivals := make(chan []notifications.Notice)
	go func() {
		defer close(arrivals)
		_ = notifications.Follow(ctx, follower, h.streamTick, func(notices []notifications.Notice) error {
			select {
			case arrivals <- notices:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})
	}()

	// The Partner's beats ride this same stream, under their own event type
	// and with no id of their own. There is one stream per page by rule, and
	// the alternative to sharing it is a second one — which the cut guard
	// refuses and which would be a second reconnection policy besides.
	var partnerEvents <-chan partner.Event
	if h.info.Partner != nil {
		events, stop := h.info.Partner.Subscribe()
		defer stop()
		partnerEvents = events
	}

	beat := time.NewTicker(h.streamHeartbeat)
	defer beat.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case notices, open := <-arrivals:
			if !open {
				return
			}
			if !writeNotices(w, flusher, notices) {
				return
			}
		case event, open := <-partnerEvents:
			if !open {
				partnerEvents = nil
				continue
			}
			if !writePartnerEvent(w, flusher, event) {
				return
			}
		case <-beat.C:
			// A comment keeps the connection warm and reaches no listener.
			if _, err := io.WriteString(w, ": still here\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

// writeNotices writes one event per notice and reports whether the client is
// still there. The id is the notice's own, which is what the browser sends
// back as Last-Event-ID; the event name is what the page listens for.
func writeNotices(w http.ResponseWriter, flusher http.Flusher, notices []notifications.Notice) bool {
	for _, notice := range notices {
		body, err := json.Marshal(notice)
		if err != nil {
			continue
		}
		if _, err := io.WriteString(w, "id: "+notice.ID+"\nevent: notification\ndata: "+string(body)+"\n\n"); err != nil {
			return false
		}
		flusher.Flush()
	}
	return true
}
