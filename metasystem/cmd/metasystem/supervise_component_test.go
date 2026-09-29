package main

import (
	"bufio"
	"go/parser"
	"go/token"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
)

// fakeBridgeServer answers one subscribe on the server end of a pipe with
// an empty snapshot and then whatever the test sends on lines.
func fakeBridgeServer(t *testing.T, server net.Conn, lines <-chan string) {
	t.Helper()
	go func() {
		defer server.Close()
		reader := bufio.NewReader(server)
		if _, err := reader.ReadString('\n'); err != nil {
			return
		}
		if _, err := io.WriteString(server, `{"snapshot":[]}`+"\n"); err != nil {
			return
		}
		for line := range lines {
			if _, err := io.WriteString(server, line+"\n"); err != nil {
				return
			}
		}
	}()
}

// TestSuperviseComponentRunsTheOwnerOnABridgeEvent (R25, U10c-2; the owner
// half of TestBridgeCarriesCardsAndEveryReaderClassifies): the landing owner
// component, subscribed to the bridge, runs its work (the owner's pass,
// whose start reads the board afresh) on a bridge event without waiting for
// its ticker; when the bridge goes away it says so, keeps working at its
// tick by direct reads, and connects again at its next tick; a refused dial
// never waits.
func TestSuperviseComponentRunsTheOwnerOnABridgeEvent(t *testing.T) {
	t.Parallel()
	lines := make(chan string)
	dials := 0
	nudges := &bridgeNudges{
		dial: func() (net.Conn, error) {
			dials++
			if dials > 1 {
				return nil, syscall.ECONNREFUSED
			}
			server, client := net.Pipe()
			fakeBridgeServer(t, server, lines)
			return client, nil
		},
		options: board.SubscribeOptions{Kinds: []string{board.KindCard, board.KindStall}, Heartbeat: time.Hour,
			After: func(time.Duration) <-chan time.Time { return nil }},
	}
	reports := make(chan string, 8)
	nudges.report = func(line string) { reports <- line }
	nudges.Ensure()
	if line := <-reports; !strings.HasPrefix(line, "bridge live") {
		t.Fatalf("first report %q", line)
	}

	stop := make(chan os.Signal)
	ticks := make(chan time.Time)
	wake := make(chan os.Signal)
	ran := make(chan string, 4)
	done := make(chan struct{})
	go func() {
		defer close(done)
		superviseLoop(stop, ticks, wake, nudges,
			func() bool { ran <- "tick"; return true },
			func() { ran <- "event" })
	}()
	lines <- `{"event":"card"}`
	if got := <-ran; got != "event" {
		t.Fatalf("the bridge event ran %q; want the work without the ticker", got)
	}
	close(lines)
	if line := <-reports; !strings.HasPrefix(line, "bridge absent") {
		t.Fatalf("report after the bridge went away %q", line)
	}
	// The bridge went away: the next tick works by direct reads and tries
	// the bridge again, which refuses at once.
	ticks <- time.Time{}
	if got := <-ran; got != "tick" {
		t.Fatalf("after the bridge went away the tick ran %q", got)
	}
	stop <- syscall.SIGTERM
	<-done
	if dials != 2 {
		t.Fatalf("dials %d; want the first and one retry at the next tick", dials)
	}
	select {
	case line := <-reports:
		t.Fatalf("a refused retry of an absent bridge reported again: %q", line)
	default:
	}
}

// TestStatusAndWorkStatusHaveNoPathToTheSocket (R25, U10c-2): the one-shot
// views read the board directly; none of their files dials, subscribes or
// names the bridge's socket.
func TestStatusAndWorkStatusHaveNoPathToTheSocket(t *testing.T) {
	t.Parallel()
	for _, file := range []string{"intent_table.go", "intent_selection.go", "intent_process.go"} {
		parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		source, _ := os.ReadFile(file)
		for _, forbidden := range []string{"board.Dial", "board.Subscribe", "bridgeNudges", "bridge.sock", "net.Dial"} {
			if strings.Contains(string(source), forbidden) {
				t.Errorf("%s names %s: a one-shot view never uses the bridge", file, forbidden)
			}
		}
		for _, spec := range parsed.Imports {
			if path, _ := strconv.Unquote(spec.Path.Value); path == "net" {
				t.Errorf("%s imports net", file)
			}
		}
	}
}
