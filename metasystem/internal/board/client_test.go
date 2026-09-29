package board

import (
	"bufio"
	"io"
	"net"
	"sync"
	"testing"
	"time"
)

// timers is an artificial After: every call is recorded and fired by hand.
type timers struct {
	mu      sync.Mutex
	pending []chan time.Time
	asked   []time.Duration
}

func (tm *timers) After(d time.Duration) <-chan time.Time {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	fired := make(chan time.Time, 1)
	tm.pending = append(tm.pending, fired)
	tm.asked = append(tm.asked, d)
	return fired
}

func (tm *timers) count() int { tm.mu.Lock(); defer tm.mu.Unlock(); return len(tm.pending) }

func (tm *timers) fireLatest() {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.pending[len(tm.pending)-1] <- t0
}

// TestClientFallsBackOnRefusalAndOnSilence (R25, U10c-1): a dial with no
// socket at the path fails at once, so the caller reads the board
// directly; a subscription whose bridge falls silent for two heartbeats is
// lost (its events channel closes); a heartbeat re-arms the silence bound;
// the client sends exactly one subscribe and classifies nothing.
func TestClientFallsBackOnRefusalAndOnSilence(t *testing.T) {
	t.Parallel()
	if _, err := Dial(shortHome(t)); err == nil {
		t.Fatal("a dial with no socket connected")
	}
	server, client := net.Pipe()
	defer server.Close()
	clock := &timers{}
	subscribed := make(chan string, 1)
	go func() {
		line, _ := bufio.NewReader(server).ReadString('\n')
		subscribed <- line
		io.WriteString(server, `{"snapshot":[]}`+"\n")
		io.WriteString(server, `{"heartbeat":"2026-09-29T10:00:25Z"}`+"\n")
		io.WriteString(server, `{"event":"card"}`+"\n")
	}()
	sub, err := Subscribe(client, SubscribeOptions{Kinds: []string{KindCard, KindStall}, Heartbeat: 25 * time.Second, After: clock.After})
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Close()
	if line := <-subscribed; line != `{"subscribe":{"kinds":["card","stall"]}}`+"\n" {
		t.Fatalf("subscribe line %q", line)
	}
	if event := <-sub.Events; event.Kind != KindCard {
		t.Fatalf("event %+v", event)
	}
	// The snapshot, the heartbeat and the event each re-armed the bound.
	if clock.count() < 3 || clock.asked[0] != 50*time.Second {
		t.Fatalf("silence bounds asked %v", clock.asked)
	}
	clock.fireLatest()
	if _, open := <-sub.Events; open {
		t.Fatal("two silent heartbeats did not drop the subscription")
	}
	if !sub.Lost() {
		t.Fatal("the subscription does not say it was lost")
	}
}
