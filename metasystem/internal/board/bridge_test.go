package board

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// pipeListener hands the bridge the server ends of net.Pipe connections.
type pipeListener struct {
	conns  chan net.Conn
	closed chan struct{}
	once   sync.Once
}

func newPipeListener() *pipeListener {
	return &pipeListener{conns: make(chan net.Conn), closed: make(chan struct{})}
}

func (l *pipeListener) Accept() (net.Conn, error) {
	select {
	case conn := <-l.conns:
		return conn, nil
	case <-l.closed:
		return nil, net.ErrClosed
	}
}

func (l *pipeListener) Close() error   { l.once.Do(func() { close(l.closed) }); return nil }
func (l *pipeListener) Addr() net.Addr { return pipeAddr{} }

type pipeAddr struct{}

func (pipeAddr) Network() string { return "pipe" }
func (pipeAddr) String() string  { return "pipe" }

// dial connects one client end to the bridge.
func (l *pipeListener) dial() net.Conn {
	server, client := net.Pipe()
	l.conns <- server
	return client
}

// mutableProber answers from a table it lets a test change: pid -> start
// seconds; absent is dead.
type mutableProber struct {
	mu    sync.Mutex
	alive map[int64]int64
}

func (p *mutableProber) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	started, ok := p.alive[pid]
	if !ok {
		return identity.Exact{}, identity.Dead, nil
	}
	return identity.Exact{Pid: pid, StartedAt: time.Unix(started, 0)}, identity.Alive, nil
}

func (p *mutableProber) kill(pid int64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.alive, pid)
}

// artificialClock is a clock a test moves by hand.
type artificialClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *artificialClock) Now() time.Time { c.mu.Lock(); defer c.mu.Unlock(); return c.now }
func (c *artificialClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// rawLine sends one line on conn and reads the bridge's next line.
func readLine(t *testing.T, reader *bufio.Reader) string {
	t.Helper()
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read from the bridge: %v", err)
	}
	return strings.TrimSuffix(line, "\n")
}

// TestBridgeCarriesCardsAndEveryReaderClassifies (R25, U10c-1's half): the
// snapshot carries raw cards and no classification; a card written under a
// fake watch is one card event carrying no content; a card whose last real
// progress crosses the stall bound under the artificial clock is one stall
// event, once; a card whose owner dies with no file event classifies the
// same through the socket's snapshot as through a direct read; a client that
// sends anything but a subscribe, or anything after it, is disconnected; a
// subscriber's kinds filter the events it receives.
func TestBridgeCarriesCardsAndEveryReaderClassifies(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	clock := &artificialClock{now: t0}
	prober := &mutableProber{alive: map[int64]int64{41: 1000}}
	seat := seatOf("m1b")
	seats := []Seat{seat}
	put(t, home, Card{Seat: seat, Goal: "goal-a", Stage: StageBuild, Owner: &Owner{Pid: 41, PidStartedAt: 1000}, Since: t0, LastProgressAt: t0, Writer: Writer{At: t0}})
	listener := newPipeListener()
	changed := make(chan struct{})
	ticks := make(chan time.Time)
	stop := make(chan struct{})
	bridge := &Bridge{Home: home, Seats: func() ([]Seat, error) { return seats, nil }, Stall: 20 * time.Minute, Keep: 24 * time.Hour, Now: clock.Now}
	done := make(chan error, 1)
	go func() { done <- bridge.Run(stop, listener, changed, ticks) }()

	// The wire, read raw.
	raw := listener.dial()
	defer raw.Close()
	rawReader := bufio.NewReader(raw)
	if _, err := io.WriteString(raw, `{"subscribe":{"kinds":["card","stall"]}}`+"\n"); err != nil {
		t.Fatal(err)
	}
	var snapshot map[string]json.RawMessage
	if err := json.Unmarshal([]byte(readLine(t, rawReader)), &snapshot); err != nil {
		t.Fatal(err)
	}
	if len(snapshot) != 1 || snapshot["snapshot"] == nil {
		t.Fatalf("the snapshot carries more than the cards: %v", snapshot)
	}
	var cards []Card
	if err := json.Unmarshal(snapshot["snapshot"], &cards); err != nil || len(cards) != 1 || cards[0].Goal != "goal-a" {
		t.Fatalf("snapshot cards %+v: %v", cards, err)
	}
	for _, word := range []string{"unknown", "reason", "stalled", "underway"} {
		if strings.Contains(strings.ToLower(string(snapshot["snapshot"])), `"`+word) {
			t.Errorf("the snapshot carries a classification field %q: %s", word, snapshot["snapshot"])
		}
	}

	// A subscriber through the client, for card events only.
	sub, err := Subscribe(listener.dial(), SubscribeOptions{Kinds: []string{KindCard}, Heartbeat: time.Hour, After: func(time.Duration) <-chan time.Time { return nil }})
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Close()
	if len(sub.Snapshot) != 1 {
		t.Fatalf("client snapshot %+v", sub.Snapshot)
	}

	// A card written under the fake watch: one event, no content.
	put(t, home, Card{Seat: seat, Goal: "goal-b", Stage: StageLandReady, Since: t0.Add(10 * time.Minute), LastProgressAt: t0.Add(10 * time.Minute), Writer: Writer{At: t0}})
	changed <- struct{}{}
	if line := readLine(t, rawReader); line != `{"event":"card"}` {
		t.Fatalf("card event %q", line)
	}
	if event := <-sub.Events; event.Kind != KindCard {
		t.Fatalf("client event %+v", event)
	}
	// A signal with nothing changed is no event: the next line is the tick's.
	changed <- struct{}{}

	// The stall crossing, under the artificial clock: once.
	clock.advance(21 * time.Minute)
	ticks <- clock.Now()
	if line := readLine(t, rawReader); line != `{"event":"stall"}` {
		t.Fatalf("stall event %q", line)
	}
	if line := readLine(t, rawReader); !strings.HasPrefix(line, `{"heartbeat":`) {
		t.Fatalf("heartbeat %q", line)
	}
	ticks <- clock.Now()
	if line := readLine(t, rawReader); !strings.HasPrefix(line, `{"heartbeat":`) {
		t.Fatalf("a second tick over the same stall is a heartbeat alone, got %q", line)
	}

	// The owner dies with no file event: the snapshot's cards and a direct
	// read classify identically.
	prober.kill(41)
	direct, _ := Read(home, seats, prober, clock.Now(), 20*time.Minute)
	throughSocket := Classify(seats, sub.Snapshot, prober, clock.Now(), 20*time.Minute)
	if reasonOf(direct, "goal-a") != "writer dead (pid 41)" || reasonOf(throughSocket, "goal-a") != reasonOf(direct, "goal-a") {
		t.Fatalf("direct %q, through the socket %q", reasonOf(direct, "goal-a"), reasonOf(throughSocket, "goal-a"))
	}

	// A client that sends a command, and a subscriber that speaks again.
	for _, first := range []string{`{"command":"write"}`, `{"subscribe":{"kinds":["card"]}}`} {
		conn := listener.dial()
		reader := bufio.NewReader(conn)
		if _, err := io.WriteString(conn, first+"\n"); err != nil {
			t.Fatal(err)
		}
		if strings.HasPrefix(first, `{"subscribe"`) {
			readLine(t, reader)
			if _, err := io.WriteString(conn, `{"command":"write"}`+"\n"); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := reader.ReadString('\n'); !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrClosedPipe) {
			t.Fatalf("after %s the bridge kept the connection: %v", first, err)
		}
		conn.Close()
	}

	close(stop)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if _, open := <-sub.Events; open {
		t.Fatal("the subscription outlived the bridge")
	}
}

func reasonOf(picture Picture, goal string) string {
	for _, unknown := range picture.Unknown {
		if unknown.Goal == goal {
			return unknown.Reason
		}
	}
	return ""
}

// TestBridgeSweepsOnlyTerminalCardsPastKeep (R25, U10c-1): the sweep removes
// a terminal card older than board.keep-hours and nothing else: a fresh
// terminal card, an old live card and every other file stay.
func TestBridgeSweepsOnlyTerminalCardsPastKeep(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	now := t0.Add(48 * time.Hour)
	seat := seatOf("m1b")
	put(t, home, Card{Seat: seat, Goal: "old-landed", Stage: StageLanded, Since: t0, LastProgressAt: t0, Writer: Writer{At: t0}})
	put(t, home, Card{Seat: seat, Goal: "fresh-landed", Stage: StageReturned, Since: now.Add(-time.Hour), LastProgressAt: now.Add(-time.Hour), Writer: Writer{At: now.Add(-time.Hour)}})
	put(t, home, Card{Seat: seat, Goal: "old-live", Stage: StageLandReady, Since: t0, LastProgressAt: t0, Writer: Writer{At: t0}})
	before := listing(t, Dir(home))
	bridge := &Bridge{Home: home, Seats: func() ([]Seat, error) { return []Seat{seat}, nil }, Stall: time.Minute, Keep: 24 * time.Hour, Now: func() time.Time { return now }}
	removed := bridge.Sweep()
	if !slices.Equal(removed, []string{"m1b/old-landed.json"}) {
		t.Fatalf("swept %v", removed)
	}
	after := listing(t, Dir(home))
	want := slices.DeleteFunc(before, func(name string) bool { return name == "m1b/old-landed.json" })
	if !reflect.DeepEqual(after, want) {
		t.Fatalf("the sweep changed more than the old terminal card:\nbefore %v\nafter  %v", before, after)
	}
}

func listing(t *testing.T, dir string) []string {
	t.Helper()
	var names []string
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			relative, _ := filepath.Rel(dir, path)
			names = append(names, filepath.ToSlash(relative))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return names
}

// shortHome is a board home under a short path: a unix socket's path is
// bounded (104 bytes on macOS) and the macOS temporary root is long.
func shortHome(t *testing.T) string {
	t.Helper()
	home, err := os.MkdirTemp("/tmp", "brd")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(home) })
	return home
}

// TestBridgeListensOnlyWhereTheStaleRuleAllows (R25, U10c-1, D14B-09): the
// holder of the flock binds the constant socket path 0600; a socket left
// behind by a holder that ended without cleanup is removed and bound over;
// a regular file at the path is BRIDGE_SOCKET_PATH_OCCUPIED and stays
// untouched; a path longer than the platform's bound is
// BRIDGE_SOCKET_PATH_TOO_LONG; a clean close removes the socket it made.
func TestBridgeListensOnlyWhereTheStaleRuleAllows(t *testing.T) {
	t.Parallel()
	home := shortHome(t)
	first, err := Listen(home)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(SocketPath(home))
	if err != nil || info.Mode()&os.ModeSocket == 0 || info.Mode().Perm() != 0o600 {
		t.Fatalf("socket %v: %v", info, err)
	}
	// Ended without cleanup: the pathname outlives its listener.
	first.(*net.UnixListener).SetUnlinkOnClose(false)
	first.Close()
	if BridgeState(home) != BridgeLive {
		t.Fatal("the stale pathname is still a socket at the path")
	}
	second, err := Listen(home)
	if err != nil {
		t.Fatalf("the next holder could not bind over the stale socket: %v", err)
	}
	conn, err := Dial(home)
	if err != nil {
		t.Fatalf("dial the new holder: %v", err)
	}
	conn.Close()
	second.Close()
	if _, err := os.Lstat(SocketPath(home)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a clean close left the socket: %v", err)
	}
	if err := os.WriteFile(SocketPath(home), []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Listen(home); err == nil || !strings.Contains(err.(interface{ RefusalDetail() string }).RefusalDetail(), "BRIDGE_SOCKET_PATH_OCCUPIED") {
		t.Fatalf("a regular file at the path: %v", err)
	}
	if data, _ := os.ReadFile(SocketPath(home)); string(data) != "keep" {
		t.Fatalf("the regular file was touched: %q", data)
	}
	long := filepath.Join(shortHome(t), strings.Repeat("x", 110))
	if _, err := Listen(long); err == nil || !strings.Contains(err.(interface{ RefusalDetail() string }).RefusalDetail(), "BRIDGE_SOCKET_PATH_TOO_LONG") {
		t.Fatalf("a long path: %v", err)
	}
	if _, err := os.Lstat(long); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("a refused listen created the board")
	}
}

// TestBridgeAnnouncesMailboxWrites (R25, R26; U10e-2): a message published
// on the board, and a marker written for it, are each one message event
// carrying nothing, to a subscriber of messages; a tick finds a write the
// watch missed; the text never crosses the socket.
func TestBridgeAnnouncesMailboxWrites(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	clock := &artificialClock{now: t0}
	listener := newPipeListener()
	changed, ticks, stop := make(chan struct{}), make(chan time.Time), make(chan struct{})
	bridge := &Bridge{Home: home, Seats: func() ([]Seat, error) { return []Seat{seatOf("m1b")}, nil }, Stall: time.Hour, Keep: time.Hour, MailboxKeep: 7 * 24 * time.Hour, Now: clock.Now}
	done := make(chan error, 1)
	go func() { done <- bridge.Run(stop, listener, changed, ticks) }()
	raw := listener.dial()
	defer raw.Close()
	reader := bufio.NewReader(raw)
	if _, err := io.WriteString(raw, `{"subscribe":{"kinds":["message"]}}`+"\n"); err != nil {
		t.Fatal(err)
	}
	readLine(t, reader)
	published, err := Publish(home, Request{Kind: KindAsk, From: Sender{Machine: "m1a"}, To: Address{Machine: "m1b"}, Text: "SECRET TEXT"}, t0)
	if err != nil {
		t.Fatal(err)
	}
	changed <- struct{}{}
	if line := readLine(t, reader); line != `{"event":"message"}` {
		t.Fatalf("message event %q", line)
	}
	if err := Mark(published.Message, "m1b", "L", "tool", t0); err != nil {
		t.Fatal(err)
	}
	ticks <- clock.Now()
	if line := readLine(t, reader); line != `{"event":"message"}` {
		t.Fatalf("a marker found at the tick: %q", line)
	}
	if line := readLine(t, reader); !strings.HasPrefix(line, `{"heartbeat":`) {
		t.Fatalf("heartbeat %q", line)
	}
	close(stop)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

// TestBridgeSweepNeverRemovesAnUnofferedMessage (R26; U10e-2's half of
// TestUnofferedMessagesAreNeverSwept, artificial clock): a goal ask queued
// with nobody holding the goal survives its deadline and eight days with the
// sweep run at every step; the first holder is offered the original id with
// the deadline line; once replied and the reply offered, keep-days after the
// close the sweep removes both messages, their markers and the empty goal
// mailbox, and nothing else; a closed thread whose reply was never offered
// is kept.
func TestBridgeSweepNeverRemovesAnUnofferedMessage(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	clock := &artificialClock{now: t0}
	bridge := &Bridge{Home: home, Seats: func() ([]Seat, error) { return nil, nil }, Keep: time.Hour, MailboxKeep: 7 * 24 * time.Hour, Now: clock.Now}
	ask, err := Publish(home, Request{Kind: KindAsk, From: Sender{Machine: "m1a"}, To: Address{Goal: "goal-q"}, Text: "queued", IfSilent: "land alone", Deadline: 30 * time.Minute}, t0)
	if err != nil {
		t.Fatal(err)
	}
	kept, err := Publish(home, Request{Kind: KindAsk, From: Sender{Machine: "m1a"}, To: Address{Machine: "m1c"}, Text: "answered, reply unread"}, t0)
	if err != nil {
		t.Fatal(err)
	}
	for _, step := range []time.Duration{31 * time.Minute, 24 * time.Hour, 8 * 24 * time.Hour} {
		clock.now = t0.Add(step)
		if removed := bridge.SweepMessages(); len(removed) != 0 {
			t.Fatalf("at +%v the sweep removed %v", step, removed)
		}
	}
	inbox, err := Pending(home, "m1b", func() (Ownership, error) { return Ownership{Live: map[string]string{"goal-q": "m1b"}}, nil }, clock.Now())
	if err != nil || len(inbox.Messages) != 1 || inbox.Messages[0].ID != ask.Message.ID ||
		!strings.Contains(Render(inbox.Messages[0], clock.Now(), time.UTC), "[deadline 2026-09-29 10:30 passed; the asker said it would: land alone]") {
		t.Fatalf("the first holder's offer = %+v, %v", inbox, err)
	}
	if err := Mark(inbox.Messages[0], "m1b", "L", "start", clock.Now()); err != nil {
		t.Fatal(err)
	}
	inbox.Release()
	reply, err := Publish(home, Request{Kind: KindReply, From: Sender{Machine: "m1b"}, To: Address{Machine: "m1a"}, Thread: ask.Message.ID, Text: "here"}, clock.Now())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Publish(home, Request{Kind: KindReply, From: Sender{Machine: "m1c"}, To: Address{Machine: "m1a"}, Thread: kept.Message.ID, Text: "unread reply"}, clock.Now()); err != nil {
		t.Fatal(err)
	}
	for _, seat := range []string{"m1a", "m1c"} {
		inbox, _ := Pending(home, seat, func() (Ownership, error) { return Ownership{}, nil }, clock.Now())
		for _, message := range inbox.Messages {
			if message.ID == reply.Message.ID || message.ID == kept.Message.ID {
				if err := Mark(message, seat, "L", "inbox", clock.Now()); err != nil {
					t.Fatal(err)
				}
			}
		}
		inbox.Release()
	}
	closed := clock.Now()
	clock.now = closed.Add(6 * 24 * time.Hour)
	if removed := bridge.SweepMessages(); len(removed) != 0 {
		t.Fatalf("before keep-days the sweep removed %v", removed)
	}
	clock.now = closed.Add(8 * 24 * time.Hour)
	removed := bridge.SweepMessages()
	want := []string{"goal/goal-q/mailbox/messages/" + ask.Message.ID + ".json", "m1a/mailbox/messages/" + reply.Message.ID + ".json"}
	if !slices.Equal(removed, want) {
		t.Fatalf("swept %v, want %v", removed, want)
	}
	if _, err := os.Stat(filepath.Join(Dir(home), GoalNamespace, "goal-q")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the empty goal mailbox stayed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(Dir(home), "m1a", "mailbox", "delivered", reply.Message.ID)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the swept reply's markers stayed: %v", err)
	}
	for _, id := range []string{kept.Message.ID} {
		if _, err := os.Stat(filepath.Join(Dir(home), "m1c", "mailbox", "messages", id+".json")); err != nil {
			t.Fatalf("the thread whose reply was never offered was swept: %v", err)
		}
	}
}
