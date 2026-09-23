package steward

// Operator notification, delivery-gated: a message is DELIVERED only
// when the configured command exits zero — that is the acknowledgment
// the launch gate requires. The command comes from the repository's
// local git configuration (metasystem.steward.notify-command); darwin
// falls back to the platform notifier; anywhere else, no configured
// command means delivery cannot be claimed and installation refuses
// up front (the glue enforces that; this layer just reports).

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// notifyTimeout bounds one delivery attempt; a hung notifier is a
// failed attempt, retried next tick, never a wedged tick.
const notifyTimeout = 15 * time.Second

var notifyPlatformOS = runtime.GOOS
var notifyCommandContext = exec.CommandContext

var deliverNotification = DeliverNotice
var handoffDeliveryAfterSnapshot = func() {}

type notifyKind int

const (
	notifyUnavailable notifyKind = iota
	notifyConfigured
	notifyPlatform
	notifyFixtureLog
)

func resolveNotify(repoRoot string) (string, notifyKind) {
	out, err := exec.Command("git", "-C", repoRoot, "config", "--get", "metasystem.steward.notify-command").Output()
	if err == nil {
		if cmd := strings.TrimSpace(string(out)); cmd != "" {
			return cmd, notifyConfigured
		}
	}
	top := canonicalPath(repoRoot)
	if installed, err := VerifyIdentity(RepoIdentityPath(top), top); err == nil && installed.Enrollment == EnrollmentFixture {
		return "", notifyFixtureLog
	}
	if notifyPlatformOS == "darwin" {
		return "", notifyPlatform
	}
	return "", notifyUnavailable
}

// NotifyCommand resolves the configured delivery command.
func NotifyCommand(repoRoot string) (string, bool) {
	command, kind := resolveNotify(repoRoot)
	return command, kind != notifyUnavailable
}

// Notice is one message on its way to the operator, with what the interface
// needs in order to say where it came from: which part of the steward raised
// it, and the episode or nonce it belongs to. The message alone is what the
// notifier receives; the rest is for the record.
type Notice struct {
	Message string
	// Source is one of the four below. An empty source journals as steward,
	// because a message with no stated origin is the steward speaking.
	Source string
	// Ref is the episode id or the pending nonce, where one is known.
	Ref string
}

// The four sources a notification can carry.
const (
	NoticeAlert   = "alert"
	NoticeHandoff = "handoff"
	NoticeVerdict = "verdict"
	NoticeSteward = "steward"
)

// Deliver attempts one delivery. Returning nil MEANS delivered — the
// caller may gate a launch on it.
func Deliver(repoRoot, message string) error {
	return DeliverNotice(repoRoot, Notice{Message: message, Source: NoticeSteward})
}

// DeliverNotice is Deliver with the origin written down. Every attempt, on
// every path, leaves one line in the journal before this returns: the
// interface reads that file, and a message the operator was shown but the
// journal never heard of would be a toast with no history.
//
// The journal never gates delivery. A line that cannot be written costs the
// interface a row; refusing the delivery over it would cost the operator the
// alarm, which is the thing that matters. A delivery that failed is journaled
// all the same, with delivered false and the reason, because "we tried and the
// channel was down" is exactly what a human needs to see.
func DeliverNotice(repoRoot string, notice Notice) error {
	err := deliver(repoRoot, notice.Message)
	journalNotice(repoRoot, notice, err)
	return err
}

// deliver is the delivery itself, unchanged: the resolved channel, the
// platform notifier, or the fixture log, and nil only when one of them
// accepted the message.
func deliver(repoRoot, message string) error {
	command, kind := resolveNotify(repoRoot)
	if kind == notifyUnavailable {
		return fmt.Errorf("no notification channel is configured and this platform has no default; the operator cannot be reached")
	}
	if kind == notifyFixtureLog {
		return appendFixtureNotification(repoRoot, message)
	}
	ctx, cancel := context.WithTimeout(context.Background(), notifyTimeout)
	defer cancel()
	var cmd *exec.Cmd
	if kind == notifyPlatform {
		title := "metasystem steward - " + canonicalPath(repoRoot)
		script := fmt.Sprintf("display notification %q with title %q", message, title)
		cmd = notifyCommandContext(ctx, "osascript", "-e", script)
	} else {
		cmd = notifyCommandContext(ctx, "/bin/sh", "-c", command)
		cmd.Env = append(cmd.Environ(), "STEWARD_MESSAGE="+message)
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("notification not delivered: %v (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func appendFixtureNotification(repoRoot, message string) error {
	directory := runnerDir(canonicalPath(repoRoot))
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return fmt.Errorf("fixture notification not delivered: %w", err)
	}
	path := filepath.Join(directory, "notifications.log")
	log, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("fixture notification not delivered: %w", err)
	}
	line := time.Now().UTC().Format(time.RFC3339) + " " + message + "\n"
	if _, err := log.WriteString(line); err != nil {
		_ = log.Close()
		return fmt.Errorf("fixture notification not delivered: %w", err)
	}
	if err := log.Close(); err != nil {
		return fmt.Errorf("fixture notification not delivered: %w", err)
	}
	return nil
}

// DeliverPending retries the queue: each delivered message leaves it;
// the first failure stops the pass (the channel is down — one named
// failure beats a burst of them). Returns how many were delivered.
func DeliverPending(repoRoot string) (int, error) {
	pending, err := PendingNotifications(repoRoot)
	if err != nil {
		return 0, err
	}
	delivered := 0
	for _, n := range pending {
		if n.DeliveryOwner == legacyHealthDeliveryOwner {
			continue
		}
		if n.Nonce == "verdict-"+string(VerdictStalledDead) {
			// Older runners queued proven-death alerts before attempting
			// revival. Retire that obsolete intent instead of delivering it
			// after an upgrade or after the condition has already healed.
			if err := MarkDelivered(repoRoot, n.Nonce); err != nil {
				return delivered, err
			}
			continue
		}
		if strings.HasPrefix(n.Nonce, "handoff-") {
			handoffDeliveryAfterSnapshot()
			wasDelivered, err := deliverPendingHandoff(repoRoot, n)
			if err != nil {
				return delivered, err
			}
			if wasDelivered {
				delivered++
			}
			continue
		}
		if err := deliverPendingNotification(repoRoot, n); err != nil {
			return delivered, err
		}
		delivered++
	}
	return delivered, nil
}

func deliverPendingHandoff(repoRoot string, snapshot PendingNotification) (bool, error) {
	authorized, err := handoffNoticeIsAuthorized(repoRoot, snapshot.Nonce)
	if err != nil {
		return false, err
	}
	if !authorized {
		if err := clearPendingNotification(repoRoot, snapshot.Nonce); err != nil {
			return false, fmt.Errorf("stale handoff notice %s could not be retired: %w", snapshot.Nonce, err)
		}
		return false, nil
	}
	if err := deliverPendingNotification(repoRoot, snapshot); err != nil {
		return false, err
	}
	return true, nil
}

func handoffNoticeIsAuthorized(repoRoot, noticeNonce string) (bool, error) {
	nonce := strings.TrimPrefix(noticeNonce, "handoff-")
	live, err := LiveIntents(repoRoot)
	if err != nil {
		return false, err
	}
	for _, intent := range live {
		if intent.Nonce != nonce {
			continue
		}
		if intent.Reason != seatHandoffReason || intent.Handoff == nil {
			return false, fmt.Errorf("pending handoff notice %s does not name a live bound seatHandoff intent", noticeNonce)
		}
		return true, nil
	}
	return false, nil
}

func deliverPendingNotification(repoRoot string, n PendingNotification) error {
	if err := deliverNotification(repoRoot, Notice{
		Message: n.Message, Source: noticeSourceOf(n.Nonce), Ref: n.Nonce,
	}); err != nil {
		return err
	}
	// The intent acknowledges FIRST: a crash between these two
	// writes then repeats a delivery (benign) instead of
	// stranding an undelivered-looking intent with no pending
	// message (a permanent suppression).
	if err := markIntentNotified(repoRoot, n.Nonce); err != nil {
		return err
	}
	if err := MarkDelivered(repoRoot, n.Nonce); err != nil {
		return err
	}
	return nil
}

// markIntentNotified flips the live intent matching a delivered
// message; a nonce with no live intent is ordinary (verdict and reap
// messages have none).
func markIntentNotified(repoRoot, nonce string) error {
	live, err := LiveIntents(repoRoot)
	if err != nil {
		return err
	}
	for _, it := range live {
		if it.Nonce == nonce && !it.Notified {
			it.Notified = true
			return UpdateIntent(repoRoot, it)
		}
	}
	return nil
}

// The notification journal.
//
// The macOS toast is a message that exists for four seconds and then does not
// exist at all. The journal is the same message written down: one JSON line
// per attempt, appended under the repository root, in the order the attempts
// were made. The interface reads it for its history and follows it for what
// arrives next, and neither of those needs the steward to know it is watched.
//
// One line per write, O_APPEND, so two processes delivering at once interleave
// whole lines rather than halves of them. The fixture installation keeps its
// own human-readable log beside this one: that log is part of what a fixture
// delivery IS, and the journal is the record of the attempt either way.

// NotificationJournalName is the file the journal is kept in, under the
// steward's own directory in the repository.
const NotificationJournalName = "notifications.jsonl"

// NotificationJournalPath is where the journal for one repository lives. The
// interface server is given this path and reads nothing else of the steward's.
func NotificationJournalPath(repoRoot string) string {
	return filepath.Join(runnerDir(canonicalPath(repoRoot)), NotificationJournalName)
}

// journalRecord is the line, and the shape the interface parses. The two
// optional fields are omitted rather than written empty, so a line says only
// what is known about the attempt it records.
type journalRecord struct {
	ID        string `json:"id"`
	At        string `json:"at"`
	Message   string `json:"message"`
	Source    string `json:"source"`
	Ref       string `json:"ref,omitempty"`
	Delivered bool   `json:"delivered"`
	Error     string `json:"error,omitempty"`
}

// noticeSourceOf reads the origin out of a pending nonce. The queue's nonces
// are prefixed by what raised them; anything else is the steward's own voice.
func noticeSourceOf(nonce string) string {
	switch {
	case strings.HasPrefix(nonce, "handoff-"):
		return NoticeHandoff
	case strings.HasPrefix(nonce, "verdict-"):
		return NoticeVerdict
	default:
		return NoticeSteward
	}
}

// journalNotice appends one line for one attempt. Its errors are dropped on
// purpose: see DeliverNotice.
func journalNotice(repoRoot string, notice Notice, delivery error) {
	source := notice.Source
	if strings.TrimSpace(source) == "" {
		source = NoticeSteward
	}
	now := time.Now()
	record := journalRecord{
		ID:        nextNotificationID(now),
		At:        now.UTC().Format(time.RFC3339),
		Message:   notice.Message,
		Source:    source,
		Ref:       notice.Ref,
		Delivered: delivery == nil,
	}
	if delivery != nil {
		record.Error = delivery.Error()
	}
	line, err := json.Marshal(record)
	if err != nil {
		return
	}
	directory := runnerDir(canonicalPath(repoRoot))
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return
	}
	file, err := os.OpenFile(filepath.Join(directory, NotificationJournalName),
		os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	// One write per line: with O_APPEND a single write lands whole, and two
	// writes would let another appender land between them.
	_, _ = file.Write(append(line, '\n'))
	_ = file.Close()
}

// The journal's identities.
//
// A ULID is a millisecond timestamp followed by eighty bits of randomness,
// written as twenty-six Crockford base32 characters, so ids sort in the order
// they were minted and a browser can compare two of them as strings. The
// interface uses exactly that: what is unread is what sorts above the last id
// the viewer saw, and a reconnecting stream resumes at the id it last had.
//
// Within one process the ids strictly increase even inside a single
// millisecond, because a clock that has not moved must not make two attempts
// look simultaneous.
const notificationIDAlphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

var (
	notificationIDMutex sync.Mutex
	lastNotificationID  string
)

func nextNotificationID(now time.Time) string {
	notificationIDMutex.Lock()
	defer notificationIDMutex.Unlock()
	id := mintNotificationID(now)
	if id <= lastNotificationID {
		id = afterNotificationID(lastNotificationID)
	}
	lastNotificationID = id
	return id
}

// mintNotificationID is the timestamp and the randomness, encoded. Randomness
// that cannot be read leaves the low bits as they were, which is still a
// well-formed id in the right millisecond; the monotonic step above is what
// keeps it distinct from the one before it.
func mintNotificationID(now time.Time) string {
	var raw [16]byte
	milli := uint64(now.UTC().UnixMilli())
	for index := 5; index >= 0; index-- {
		raw[index] = byte(milli & 0xff)
		milli >>= 8
	}
	_, _ = notificationEntropy(raw[6:])
	return encodeNotificationID(raw)
}

// notificationEntropy is crypto/rand, named so a test can read the encoding
// without the randomness.
var notificationEntropy = rand.Read

// encodeNotificationID reads the sixteen bytes as one 128-bit number and
// writes it five bits at a time, least significant last. Twenty-six
// characters carry 130 bits, so the first one carries only the top three.
func encodeNotificationID(raw [16]byte) string {
	high := binary.BigEndian.Uint64(raw[:8])
	low := binary.BigEndian.Uint64(raw[8:])
	id := make([]byte, 26)
	for index := len(id) - 1; index >= 0; index-- {
		id[index] = notificationIDAlphabet[low&0x1f]
		low = low>>5 | high<<59
		high >>= 5
	}
	return string(id)
}

// afterNotificationID is the next id in sort order: the last character steps
// through the alphabet and carries into the one before it. An id that is all
// Z has nowhere to carry to, which cannot happen inside one millisecond of
// real deliveries, and answers with itself rather than with a shorter id.
func afterNotificationID(id string) string {
	if id == "" {
		return strings.Repeat(notificationIDAlphabet[:1], 26)
	}
	stepped := []byte(id)
	for index := len(stepped) - 1; index >= 0; index-- {
		at := strings.IndexByte(notificationIDAlphabet, stepped[index])
		if at < 0 {
			return id
		}
		if at+1 < len(notificationIDAlphabet) {
			stepped[index] = notificationIDAlphabet[at+1]
			return string(stepped)
		}
		stepped[index] = notificationIDAlphabet[0]
	}
	return id
}
