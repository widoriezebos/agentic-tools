package phase_test

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel/fake"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel/phase"
)

// serveFake runs the fake channel server in dir until the test ends.
func serveFake(t *testing.T, dir string) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	ready := make(chan string, 1)
	go func() { done <- fake.ServeReady(ctx, dir, ready) }()
	select {
	case <-ready:
	case err := <-done:
		cancel()
		t.Fatalf("fake did not start: %v", err)
	}
	t.Cleanup(func() {
		cancel()
		if err := <-done; err != nil {
			t.Error(err)
		}
	})
}

// posts are the texts the fake server was asked to post.
func posts(t *testing.T, dir string) []string {
	t.Helper()
	f, err := os.Open(filepath.Join(dir, "journal.jsonl"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var out []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var row struct {
			Method string              `json:"method"`
			Form   map[string][]string `json:"form"`
		}
		if json.Unmarshal(scanner.Bytes(), &row) == nil && row.Method == "chat.postMessage" {
			text := ""
			if len(row.Form["text"]) > 0 {
				text = row.Form["text"][0]
			}
			out = append(out, text)
		}
	}
	return out
}

// quietRoot is a named machine's checkout whose channel is the fake
// server, with a status interval that has long passed.
func quietRoot(t *testing.T) (root, dir string) {
	t.Helper()
	root = t.TempDir()
	for _, args := range [][]string{{"init", "--quiet", "-b", "main"}, {"config", "metasystem.goal.machine", "m"}} {
		if out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	dir = filepath.Join(root, "fake")
	serveFake(t, dir)
	writeConfig(t, root, "channel.destination.fleet.adapter=fake\nchannel.destination.fleet.fake.dir="+dir+"\nchannel.destination.fleet.fake.face=slack\nchannel.human.slack.user-id=slack-human\nchannel.status.interval-minutes=1\n", "channel.human.totp-secret=JBSWY3DPEHPK3PXP\n")
	return root, dir
}

// Decision 7: the tick posts no periodic status report, even when the
// report's content changed and its interval has passed; the channel only
// carries landings and what needs a response.
func TestTickPostsNoPeriodicStatusReport(t *testing.T) {
	t.Parallel()
	root, dir := quietRoot(t)
	if err := channel.SaveStatusState(root, channel.StatusState{ContentDigest: channel.Digest("an older report")}); err != nil {
		t.Fatal(err)
	}
	if _, err := phase.Run(context.Background(), root); err != nil {
		t.Fatal(err)
	}
	if got := posts(t, dir); len(got) != 0 {
		t.Fatalf("the tick posted %q; want no message", got)
	}
}

// unreachable is a channel every post to fails.
type unreachable struct{}

func (unreachable) Post(context.Context, channel.DestinationConfig, string, *channel.MessageRef) (channel.MessageRef, error) {
	return channel.MessageRef{}, errors.New("send failed: unreachable")
}
func (unreachable) Receive(context.Context, channel.DestinationConfig, []channel.MessageRef, channel.Cursor) ([]channel.Inbound, channel.Cursor, error) {
	return nil, "", nil
}
func (unreachable) Confirm(context.Context, channel.DestinationConfig, channel.Cursor) error {
	return nil
}
func (unreachable) Credential(context.Context, channel.DestinationConfig) (channel.CredentialIdentity, error) {
	return channel.CredentialIdentity{}, nil
}

// A landing line whose post failed is retried by the next tick, once.
func TestTickRetriesAFailedLandedLineOnce(t *testing.T) {
	t.Parallel()
	root, dir := quietRoot(t)
	sha := "0123456789abcdef0123456789abcdef01234567"
	if err := channel.PostLanded(context.Background(), root, unreachable{}, channel.DestinationConfig{}, "Seats now ask you when they are stuck.", sha, time.Now()); err == nil {
		t.Fatal("a post to an unreachable channel reported no error")
	}
	for range 2 {
		if _, err := phase.Run(context.Background(), root); err != nil {
			t.Fatal(err)
		}
	}
	if got := posts(t, dir); len(got) != 1 || got[0] != "Seats now ask you when they are stuck." {
		t.Fatalf("posts = %q; want the landing line once", got)
	}
}

// NotifyLanded with no channel configured does nothing and records nothing.
func TestNotifyLandedWithoutChannelDoesNothing(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := phase.NotifyLanded(context.Background(), root, "Seats now ask you when they are stuck.", "0123456789abcdef", time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "artifacts")); !os.IsNotExist(err) {
		t.Fatalf("a root with no channel got channel records: %v", err)
	}
}
