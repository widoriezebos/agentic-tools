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
	"strings"
	"testing"
	"time"
)

func cancellationServer(t *testing.T) (string, string, context.CancelFunc, <-chan error, <-chan http.ConnState) {
	t.Helper()
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	states := make(chan http.ConnState, 16)
	go func() {
		done <- serve(ctx, dir, func(_ net.Conn, state http.ConnState) {
			select {
			case states <- state:
			default:
			}
		})
	}()
	deadline := time.Now().Add(5 * time.Second)
	basePath := filepath.Join(dir, "base-url")
	for time.Now().Before(deadline) {
		if data, err := os.ReadFile(basePath); err == nil && strings.TrimSpace(string(data)) != "" {
			t.Cleanup(cancel)
			return dir, strings.TrimSpace(string(data)), cancel, done, states
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	t.Fatal("fake server did not publish its base URL")
	return "", "", nil, nil, nil
}

func waitConnectionState(t *testing.T, states <-chan http.ConnState, want http.ConnState) {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case got := <-states:
			if got == want {
				return
			}
		case <-deadline:
			t.Fatalf("fake server connection never reached state %s", want)
		}
	}
}

func waitServerCancellation(t *testing.T, done <-chan error) {
	t.Helper()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("fake server cancellation failed: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("fake server cancellation waited for a fixture client")
	}
}

func assertConnectionHeld(t *testing.T, states <-chan http.ConnState) {
	t.Helper()
	timer := time.NewTimer(250 * time.Millisecond)
	defer timer.Stop()
	for {
		select {
		case state := <-states:
			if state == http.StateIdle || state == http.StateClosed {
				t.Fatalf("controlled request was not held; connection became %s", state)
			}
		case <-timer.C:
			return
		}
	}
}

func TestCancellationClosesAcceptedPartialRequest(t *testing.T) {
	_, base, cancel, done, states := cancellationServer(t)
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
	dir, base, cancel, done, states := cancellationServer(t)
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
	assertConnectionHeld(t, states)
	cancel()
	waitServerCancellation(t, done)
	waitConnectionState(t, states, http.StateClosed)
	if _, err := os.Stat(release); !os.IsNotExist(err) {
		t.Fatalf("paused handler acquired a release that the test never supplied: %v", err)
	}
}
