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
	"strings"
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
// command, and decides nothing; its writes are the sweeps: terminal cards
// older than Keep, and peer-message threads closed, every message offered,
// and MailboxKeep past their close (zero sweeps no message). A mailbox write
// is one message event that carries nothing, never a message's text.
type Bridge struct {
	Home        string
	Seats       func() ([]Seat, error)
	Stall       time.Duration
	Keep        time.Duration
	MailboxKeep time.Duration
	Now         func() time.Time

	mu          sync.Mutex
	mail        string
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
	b.refreshMail(false)
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
			b.refreshMail(true)
		case now, open := <-ticks:
			if !open {
				ticks = nil
				continue
			}
			if len(b.Sweep()) > 0 {
				b.refresh(true)
			}
			b.SweepMessages()
			// A marker written under a message's own delivered directory is
			// not under the watch; the tick finds it.
			b.refreshMail(true)
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

// refreshMail reads the mailboxes' listing afresh; with emit, a change is
// one message event, which carries nothing.
func (b *Bridge) refreshMail(emit bool) {
	print := mailPrint(b.Home)
	b.mu.Lock()
	defer b.mu.Unlock()
	changed := print != b.mail
	b.mail = print
	if emit && changed {
		b.broadcastLocked(KindMessage, []byte(`{"event":"message"}`))
	}
}

// mailPrint is every message and marker name on the board, in order: what a
// message event reports changed, and nothing of any message's content.
func mailPrint(home string) string {
	var names []string
	mailboxes := seatMailboxes(home)
	goals, _ := goalMailboxes(home)
	for _, goal := range goals {
		mailboxes = append(mailboxes, mailboxDir(home, Address{Goal: goal}))
	}
	for _, mailbox := range mailboxes {
		messages, _ := os.ReadDir(filepath.Join(mailbox, "messages"))
		for _, entry := range messages {
			names = append(names, mailbox+"/m/"+entry.Name())
		}
		delivered, _ := os.ReadDir(filepath.Join(mailbox, "delivered"))
		for _, entry := range delivered {
			markers, _ := os.ReadDir(filepath.Join(mailbox, "delivered", entry.Name()))
			for _, marker := range markers {
				names = append(names, mailbox+"/d/"+entry.Name()+"/"+marker.Name())
			}
		}
	}
	return strings.Join(names, "\n")
}

// SweepMessages removes the threads that may leave the board (Sweepable:
// closed, every message offered to its addressee, MailboxKeep past the
// close): each message's file and markers, then a goal mailbox left empty.
// An unoffered message is never removed. It returns what it removed, board
// relative, sorted.
func (b *Bridge) SweepMessages() []string {
	if b.MailboxKeep <= 0 {
		return nil
	}
	threads, err := Threads(b.Home)
	if err != nil {
		return nil
	}
	now := b.Now()
	board := Dir(b.Home)
	var removed []string
	for _, thread := range threads {
		if !thread.Sweepable(now, b.MailboxKeep) {
			continue
		}
		for _, message := range thread.Messages {
			if removeMessage(message) {
				relative, _ := filepath.Rel(board, filepath.Join(message.mailbox, "messages", message.ID+".json"))
				removed = append(removed, filepath.ToSlash(relative))
			}
		}
	}
	goals, _ := goalMailboxes(b.Home)
	for _, goal := range goals {
		removeEmptyGoalMailbox(b.Home, goal)
	}
	sort.Strings(removed)
	return removed
}

// removeMessage removes one message's markers and then its file.
func removeMessage(message Message) bool {
	if message.mailbox == "" || !ValidID(message.ID) {
		return false
	}
	markers := filepath.Join(message.mailbox, "delivered", message.ID)
	for _, machine := range message.offeredTo {
		_ = os.Remove(filepath.Join(markers, machine+".json"))
	}
	_ = os.Remove(markers)
	_ = os.Remove(filepath.Join(message.mailbox, "concluded", message.ID+".json"))
	return os.Remove(filepath.Join(message.mailbox, "messages", message.ID+".json")) == nil
}

// removeEmptyGoalMailbox removes a goal's mailbox directories when no
// message is left in it; a directory that is not empty stays.
func removeEmptyGoalMailbox(home, goal string) {
	mailbox := mailboxDir(home, Address{Goal: goal})
	if entries, err := os.ReadDir(filepath.Join(mailbox, "messages")); err != nil || len(entries) > 0 {
		return
	}
	_ = os.Remove(filepath.Join(mailbox, "concluded"))
	for _, dir := range []string{filepath.Join(mailbox, "messages"), filepath.Join(mailbox, "delivered"), mailbox, filepath.Dir(mailbox)} {
		if os.Remove(dir) != nil {
			return
		}
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
		return nil, coded("BRIDGE_SOCKET_PATH_TOO_LONG", fmt.Sprintf("path=%s bytes=%d limit=%d", path, len(path), maxSocketPath()),
			fmt.Errorf("the board bridge is off: its socket path is %d bytes, over the limit of %d; use a shorter home", len(path), maxSocketPath()))
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
		return nil, coded("BRIDGE_SOCKET_PATH_OCCUPIED", "path="+path, fmt.Errorf("the board bridge is off: a file or directory stands at its socket %s; move it aside", path))
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
