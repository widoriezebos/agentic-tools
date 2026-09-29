package board

import (
	"bufio"
	"encoding/json"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// Event is one nudge from the bridge: its kind and nothing else. A
// subscriber re-reads the board on it and classifies what it reads with
// its own prober and clock (D14B-11).
type Event struct {
	Kind string `json:"event"`
}

// SubscribeOptions are a subscription's kinds and its silence bound: a
// bridge that sends no line for two heartbeats is lost. After is the clock;
// nil is time.After.
type SubscribeOptions struct {
	Kinds     []string
	Heartbeat time.Duration
	After     func(time.Duration) <-chan time.Time
}

// Subscription is one live connection to the bridge: the raw cards of its
// snapshot, and its events. Events closes when the connection ends, falls
// silent for two heartbeats, or is closed; the subscriber then reads the
// board directly and connects again at its next tick. The client classifies
// nothing.
type Subscription struct {
	Snapshot []Card
	Events   <-chan Event
	conn     net.Conn
	closed   chan struct{}
	once     sync.Once
	lost     atomic.Bool
}

// errNoSnapshot is a bridge that accepted and never answered the subscribe.
var errNoSnapshot = errors.New("the bridge sent no snapshot within two heartbeats")

// Subscribe sends the one subscribe on conn and waits for the snapshot. Any
// failure closes conn and is returned: the caller reads the board directly.
func Subscribe(conn net.Conn, options SubscribeOptions) (*Subscription, error) {
	if options.After == nil {
		options.After = time.After
	}
	if options.Heartbeat <= 0 {
		options.Heartbeat = DefaultHeartbeat
	}
	var request struct {
		Subscribe struct {
			Kinds []string `json:"kinds"`
		} `json:"subscribe"`
	}
	request.Subscribe.Kinds = append([]string{}, options.Kinds...)
	encoded, _ := json.Marshal(request)
	lines := make(chan []byte)
	quit := make(chan struct{})
	go func() {
		defer close(lines)
		scanner := bufio.NewScanner(conn)
		scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
		for scanner.Scan() {
			select {
			case lines <- append([]byte(nil), scanner.Bytes()...):
			case <-quit:
				return
			}
		}
	}()
	fail := func(err error) (*Subscription, error) {
		close(quit)
		conn.Close()
		return nil, err
	}
	if _, err := conn.Write(append(encoded, '\n')); err != nil {
		return fail(err)
	}
	silence := 2 * options.Heartbeat
	var first []byte
	select {
	case line, open := <-lines:
		if !open {
			return fail(errors.New("the bridge closed the connection before its snapshot"))
		}
		first = line
	case <-options.After(silence):
		return fail(errNoSnapshot)
	}
	var snapshot struct {
		Snapshot []Card `json:"snapshot"`
	}
	if err := json.Unmarshal(first, &snapshot); err != nil {
		return fail(err)
	}
	events := make(chan Event, 16)
	sub := &Subscription{Snapshot: snapshot.Snapshot, Events: events, conn: conn, closed: make(chan struct{})}
	go func() {
		defer close(events)
		defer close(quit)
		defer conn.Close()
		timer := options.After(silence)
		for {
			select {
			case <-sub.closed:
				return
			case <-timer:
				sub.lost.Store(true)
				return
			case line, open := <-lines:
				if !open {
					sub.lost.Store(true)
					return
				}
				timer = options.After(silence)
				var event Event
				if json.Unmarshal(line, &event) != nil || event.Kind == "" {
					continue
				}
				select {
				case events <- event:
				default:
					// A pending nudge already asks for the one read.
				}
			}
		}
	}()
	return sub, nil
}

// Lost reports whether the bridge went away or fell silent, as opposed to
// the subscriber closing the subscription itself.
func (s *Subscription) Lost() bool { return s.lost.Load() }

// Close ends the subscription.
func (s *Subscription) Close() {
	s.once.Do(func() {
		close(s.closed)
		s.conn.Close()
	})
}
