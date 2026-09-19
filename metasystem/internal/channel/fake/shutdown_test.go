package fake

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

func cancellationServer(t *testing.T) (string, string, context.CancelFunc, <-chan error, <-chan http.ConnState, <-chan WaitPoint) {
	t.Helper()
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	states := make(chan http.ConnState, 16)
	parked := make(chan WaitPoint, 1)
	ready := make(chan string, 1)
	go func() {
		done <- ServeWithHooks(ctx, dir, ServeHooks{
			ConnState: func(_ net.Conn, state http.ConnState) {
				select {
				case states <- state:
				default:
				}
			},
			Ready:  ready,
			Parked: parked,
		})
	}()
	select {
	case base := <-ready:
		t.Cleanup(cancel)
		return dir, base, cancel, done, states, parked
	case err := <-done:
		cancel()
		t.Fatalf("fake server did not publish its base URL: %v", err)
		return "", "", nil, nil, nil, nil
	}
}

func waitConnectionState(t *testing.T, states <-chan http.ConnState, want http.ConnState) {
	t.Helper()
	for got := range states {
		if got == want {
			return
		}
	}
	t.Fatalf("fake server connection states closed before state %s", want)
}

func waitServerCancellation(t *testing.T, done <-chan error) {
	t.Helper()
	if err := <-done; err != nil {
		t.Fatalf("fake server cancellation failed: %v", err)
	}
}

func assertConnectionHeld(t *testing.T, states <-chan http.ConnState, parked <-chan WaitPoint) {
	t.Helper()
	if got := <-parked; got != PauseWait {
		t.Fatalf("controlled request parked at %q, want %q", got, PauseWait)
	}
	for {
		select {
		case state := <-states:
			if state == http.StateIdle || state == http.StateClosed {
				t.Fatalf("controlled request was not held; connection became %s", state)
			}
		default:
			return
		}
	}
}

func TestCancellationClosesAcceptedPartialRequest(t *testing.T) {
	_, base, cancel, done, states, _ := cancellationServer(t)
	parsed, err := url.Parse(base)
	if err != nil {
		t.Fatal(err)
	}
	client, err := net.Dial("tcp", parsed.Host)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	waitConnectionState(t, states, http.StateNew)
	if _, err := fmt.Fprintf(client, "GET / HTTP/1.1\r\nHost: %s\r\n", parsed.Host); err != nil {
		t.Fatal(err)
	}
	cancel()
	waitServerCancellation(t, done)
	waitConnectionState(t, states, http.StateClosed)
}

func TestCancellationCancelsPausedHandlerBeforeRelease(t *testing.T) {
	dir, base, cancel, done, states, parked := cancellationServer(t)
	parsed, err := url.Parse(base)
	if err != nil {
		t.Fatal(err)
	}
	release := filepath.Join(t.TempDir(), "release")
	control := controls{PauseBefore: []pauseControl{{Listener: "paused", Method: "sendMessage", Until: release}}}
	data, err := json.Marshal(control)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "control.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	client, err := net.Dial("tcp", parsed.Host)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if _, err := fmt.Fprintf(client, "POST /botfake-telegram-token-paused/sendMessage HTTP/1.1\r\nHost: %s\r\nContent-Type: application/json\r\nContent-Length: 2\r\n\r\n{}", parsed.Host); err != nil {
		t.Fatal(err)
	}
	waitConnectionState(t, states, http.StateActive)
	assertConnectionHeld(t, states, parked)
	cancel()
	waitServerCancellation(t, done)
	waitConnectionState(t, states, http.StateClosed)
	if _, err := os.Stat(release); !os.IsNotExist(err) {
		t.Fatalf("paused handler acquired a release that the test never supplied: %v", err)
	}
}
