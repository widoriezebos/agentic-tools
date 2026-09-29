package board

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"time"
)

// The kinds of event a subscriber may ask for. An event carries nothing:
// every subscriber re-reads the board and classifies it itself (D14B-11).
const (
	KindCard    = "card"
	KindStall   = "stall"
	KindMessage = "message"
)

// DefaultHeartbeat is the bridge's heartbeat, the interface's own stream
// cadence; a client that hears nothing for two of them reads directly.
const DefaultHeartbeat = 25 * time.Second

var knownKinds = map[string]bool{KindCard: true, KindStall: true, KindMessage: true}

// Bridge is the board's reporter (batch-lane design D14-r2, R25): it keeps
// the last read of the armed seats' raw cards, and pushes each change to its
// subscribers over the user-only socket, raw cards in the snapshot and
// events that carry nothing. It classifies nothing, writes no card, takes no
// command, and decides nothing; its one write is the sweep of terminal cards
// older than Keep.
type Bridge struct {
	Home  string
	Seats func() ([]Seat, error)
	Stall time.Duration
	Keep  time.Duration
	Now   func() time.Time

	mu          sync.Mutex
	last        map[string][]byte
	cards       map[string]Card
	stalled     map[string]bool
	subscribers map[*subscriber]bool
}

type subscriber struct {
	conn  net.Conn
	kinds map[string]bool
	out   chan []byte
}

// Run serves listener until stop closes: every signal on changed re-reads
// the board and emits one card event per card changed, added or removed;
// every tick emits a stall event for each card whose last real progress
// crossed the stall bound since the last tick, a heartbeat, and sweeps.
func (b *Bridge) Run(stop <-chan struct{}, listener net.Listener, changed <-chan struct{}, ticks <-chan time.Time) error {
	b.mu.Lock()
	b.last, b.cards, b.stalled, b.subscribers = map[string][]byte{}, map[string]Card{}, map[string]bool{}, map[*subscriber]bool{}
	b.mu.Unlock()
	b.refresh(false)
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go b.serve(conn)
		}
	}()
	for {
		select {
		case <-stop:
			err := listener.Close()
			b.mu.Lock()
			for sub := range b.subscribers {
				b.dropLocked(sub)
			}
			b.mu.Unlock()
			if errors.Is(err, net.ErrClosed) {
				err = nil
			}
			return err
		case _, open := <-changed:
			if !open {
				changed = nil
				continue
			}
			b.refresh(true)
		case now, open := <-ticks:
			if !open {
				ticks = nil
				continue
			}
			if len(b.Sweep()) > 0 {
				b.refresh(true)
			}
			b.checkStalls()
			heartbeat, _ := json.Marshal(map[string]string{"heartbeat": now.UTC().Format(time.RFC3339)})
			b.broadcast("", heartbeat)
		}
	}
}

func cardKey(card Card) string { return card.Seat.Machine + "/" + card.Goal }

// refresh reads the board afresh; with emit, each changed card is one event.
func (b *Bridge) refresh(emit bool) {
	cards, ok := b.read()
	if !ok {
		return
	}
	next := map[string][]byte{}
	nextCards := map[string]Card{}
	for _, card := range cards {
		encoded, _ := json.Marshal(card)
		next[cardKey(card)], nextCards[cardKey(card)] = encoded, card
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	changes := 0
	for key, encoded := range next {
		if previous, seen := b.last[key]; !seen || !bytes.Equal(previous, encoded) {
			changes++
			if b.cards[key].LastProgressAt != nextCards[key].LastProgressAt || b.cards[key].Stage != nextCards[key].Stage {
				delete(b.stalled, key)
			}
		}
	}
	for key := range b.last {
		if _, still := next[key]; !still {
			changes++
			delete(b.stalled, key)
		}
	}
	b.last, b.cards = next, nextCards
	if !emit {
		return
	}
	for range changes {
		b.broadcastLocked(KindCard, []byte(`{"event":"card"}`))
	}
}

// read is the raw cards of the armed seats; false when the seats cannot be
// read, and then the last read stands.
func (b *Bridge) read() ([]Card, bool) {
	seats, err := b.Seats()
	if err != nil {
		return nil, false
	}
	return Cards(b.Home, seats), true
}

// checkStalls emits one stall event for each card whose last real progress
// is now older than the stall bound, once per crossing.
func (b *Bridge) checkStalls() {
	now := b.Now()
	b.mu.Lock()
	defer b.mu.Unlock()
	for key, card := range b.cards {
		if card.Stage.progressing() && now.Sub(card.LastProgressAt) > b.Stall && !b.stalled[key] {
			b.stalled[key] = true
			b.broadcastLocked(KindStall, []byte(`{"event":"stall"}`))
		}
	}
}

func (b *Bridge) broadcast(kind string, message []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.broadcastLocked(kind, message)
}

// broadcastLocked queues message for every subscriber of kind (every
// subscriber for a heartbeat); a subscriber too slow to take it is dropped,
// and reads the board directly.
func (b *Bridge) broadcastLocked(kind string, message []byte) {
	for sub := range b.subscribers {
		if kind != "" && !sub.kinds[kind] {
			continue
		}
		select {
		case sub.out <- append(append([]byte(nil), message...), '\n'):
		default:
			b.dropLocked(sub)
		}
	}
}

func (b *Bridge) dropLocked(sub *subscriber) {
	if b.subscribers[sub] {
		delete(b.subscribers, sub)
		close(sub.out)
	}
}

// serve admits one client: its first line must be a subscribe to known
// kinds, anything else closes the connection; after it, anything the client
// sends closes it too. The socket carries no command.
func (b *Bridge) serve(conn net.Conn) {
	reader := bufio.NewReader(conn)
	line, err := reader.ReadBytes('\n')
	if err != nil {
		conn.Close()
		return
	}
	var request struct {
		Subscribe *struct {
			Kinds []string `json:"kinds"`
		} `json:"subscribe"`
	}
	decoder := json.NewDecoder(bytes.NewReader(line))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&request) != nil || request.Subscribe == nil {
		conn.Close()
		return
	}
	sub := &subscriber{conn: conn, kinds: map[string]bool{}, out: make(chan []byte, 64)}
	for _, kind := range request.Subscribe.Kinds {
		if !knownKinds[kind] {
			conn.Close()
			return
		}
		sub.kinds[kind] = true
	}
	b.mu.Lock()
	if b.subscribers == nil {
		b.mu.Unlock()
		conn.Close()
		return
	}
	snapshot := make([]Card, 0, len(b.cards))
	for _, card := range b.cards {
		snapshot = append(snapshot, card)
	}
	sort.Slice(snapshot, func(i, j int) bool { return cardKey(snapshot[i]) < cardKey(snapshot[j]) })
	encoded, _ := json.Marshal(map[string][]Card{"snapshot": snapshot})
	sub.out <- append(encoded, '\n')
	b.subscribers[sub] = true
	b.mu.Unlock()
	go func() {
		for message := range sub.out {
			if _, err := conn.Write(message); err != nil {
				b.mu.Lock()
				b.dropLocked(sub)
				b.mu.Unlock()
				for range sub.out {
				}
				break
			}
		}
		conn.Close()
	}()
	// Anything more from the client, or its going away, ends the
	// subscription.
	_, _ = reader.ReadByte()
	b.mu.Lock()
	b.dropLocked(sub)
	b.mu.Unlock()
}

// Sweep removes the terminal cards of the armed seats whose terminal stage
// began more than Keep ago, each under its seat's lock and re-read before
// removal, and returns what it removed as seat/goal.json. It removes nothing
// else.
func (b *Bridge) Sweep() []string {
	seats, err := b.Seats()
	if err != nil || b.Keep <= 0 {
		return nil
	}
	now := b.Now()
	var removed []string
	for _, seat := range seats {
		if !SafeName(seat.Machine) {
			continue
		}
		dir := filepath.Join(Dir(b.Home), seat.Machine)
		for _, card := range Cards(b.Home, []Seat{seat}) {
			if !expired(card, now, b.Keep) {
				continue
			}
			if b.removeExpired(dir, card.Goal, now) {
				removed = append(removed, seat.Machine+"/"+card.Goal+".json")
			}
		}
	}
	sort.Strings(removed)
	return removed
}

func expired(card Card, now time.Time, keep time.Duration) bool {
	stamp := card.Since
	if card.Writer.At.After(stamp) {
		stamp = card.Writer.At
	}
	return card.Stage.Terminal() && now.Sub(stamp) > keep
}

func (b *Bridge) removeExpired(dir, goal string, now time.Time) bool {
	unlock, err := lockSeat(dir)
	if err != nil {
		return false
	}
	defer unlock()
	path := filepath.Join(dir, goal+".json")
	card, ok := readCardFile(path)
	if !ok || !expired(card, now, b.Keep) {
		return false
	}
	return os.Remove(path) == nil
}

// Cards reads the raw cards of exactly the given seats' directories and
// classifies nothing: what the bridge carries. A malformed card, or one
// filed under another goal's or seat's name, is left out.
func Cards(home string, seats []Seat) []Card {
	var cards []Card
	for _, seat := range seats {
		if !SafeName(seat.Machine) {
			continue
		}
		read, _ := readSeat(filepath.Join(Dir(home), seat.Machine), seat)
		cards = append(cards, read...)
	}
	return cards
}

// maxSocketPath is the longest socket path the platform binds (sun_path
// less its terminating NUL).
func maxSocketPath() int {
	if runtime.GOOS == "linux" {
		return 107
	}
	return 103
}

// Listen binds the bridge's socket under home. The caller holds the board's
// flock, which proves no live holder: a socket standing at the path was left
// by a holder that ended without cleanup and is removed; anything else there
// is BRIDGE_SOCKET_PATH_OCCUPIED and is left untouched. The socket is 0600
// in the 0700 host directory; closing the listener removes it.
func Listen(home string) (net.Listener, error) {
	path := SocketPath(home)
	if len(path) > maxSocketPath() {
		return nil, fmt.Errorf("BRIDGE_SOCKET_PATH_TOO_LONG: the bridge's socket %s is %d bytes, over this platform's %d; the bridge does not serve and every reader reads the board directly; move the home to a shorter path", path, len(path), maxSocketPath())
	}
	if _, err := boardDir(home); err != nil {
		return nil, err
	}
	info, err := os.Lstat(path)
	switch {
	case err == nil && info.Mode()&fs.ModeSocket != 0:
		if err := os.Remove(path); err != nil {
			return nil, err
		}
	case err == nil:
		return nil, fmt.Errorf("BRIDGE_SOCKET_PATH_OCCUPIED: %s is not a socket (a file or directory stands there); the bridge does not serve and every reader reads the board directly; move it aside so the bridge can bind", path)
	case !errors.Is(err, fs.ErrNotExist):
		return nil, err
	}
	listener, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		listener.Close()
		return nil, err
	}
	return listener, nil
}

// Dial connects to the bridge's socket under home.
func Dial(home string) (net.Conn, error) {
	return net.Dial("unix", SocketPath(home))
}

// LockPath is the board's flock: the steward that holds it runs the bridge.
// The file is opened and never removed, so its inode is stable.
func LockPath(home string) string { return filepath.Join(Dir(home), ".bridge.flock") }

// EnsureBoard creates the private board directory under home, as every
// writer does, and returns it: the directory the bridge's flock lives in.
func EnsureBoard(home string) (string, error) { return boardDir(home) }
