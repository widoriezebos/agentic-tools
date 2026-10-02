package main

// Decision 7 of the blocked-agent-asks-the-human design: the channel says
// one line per landing on main, "landed: G at SHA", once per sha, from
// where the landing becomes fact: the lane's landing push and a seat's own
// hand landing. A refused push says nothing; a failed post fails nothing.

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
)

// fakeChannelPosts are the texts the fake channel server was asked to post.
func fakeChannelPosts(t *testing.T, dir string) []string {
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
		if json.Unmarshal(scanner.Bytes(), &row) == nil && row.Method == "chat.postMessage" && len(row.Form["text"]) > 0 {
			out = append(out, row.Form["text"][0])
		}
	}
	return out
}

// appendChannelConf points an installation's local settings at the fake
// channel server in dir.
func appendChannelConf(t *testing.T, install, dir string) {
	t.Helper()
	f, err := os.OpenFile(filepath.Join(install, "metasystem.conf.local"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("channel.destination.fleet.adapter=fake\nchannel.destination.fleet.fake.dir=" + dir + "\nchannel.destination.fleet.fake.face=slack\n"); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

// landing push posts one landing line naming the goals it landed and the
// pushed sha; the repeat push, which pushes nothing, posts nothing.
func TestLandingPushPostsOneLandedLine(t *testing.T) {
	t.Parallel()
	bed := newPlainVerbBed(t)
	bed.setCommand(t, bed.script(t, "prove-green.sh", 0))
	dir, _ := commandFakeBed(t)
	appendChannelConf(t, bed.installation, dir)
	head := bed.merge(t, bed.seat(t, "goal-a"), bed.seat(t, "goal-b"))
	if code, text := bed.run(t, "landing", "prove", "--wait"); code != 0 {
		t.Fatalf("a green prove = %d\n%s", code, text)
	}
	if code, text := bed.run(t, "landing", "push"); code != 0 {
		t.Fatalf("push = %d\n%s", code, text)
	}
	want := "landed: goal-a, goal-b at " + shortLandingID(head)
	if got := fakeChannelPosts(t, dir); len(got) != 1 || got[0] != want {
		t.Fatalf("posts = %q; want [%q]", got, want)
	}
	if code, text := bed.run(t, "landing", "push"); code != 0 {
		t.Fatalf("repeat push = %d\n%s", code, text)
	}
	if got := fakeChannelPosts(t, dir); len(got) != 1 {
		t.Fatalf("a repeat push posted again: %q", got)
	}
}

// A refused push put nothing on main, so it posts nothing.
func TestRefusedLandingPushPostsNothing(t *testing.T) {
	t.Parallel()
	bed := newPlainVerbBed(t)
	dir, _ := commandFakeBed(t)
	appendChannelConf(t, bed.installation, dir)
	bed.merge(t, bed.seat(t, "goal-a"))
	if code, text := bed.run(t, "landing", "push"); code != 1 || !strings.Contains(text, "never proven") {
		t.Fatalf("an unproven push = %d\n%s", code, text)
	}
	if got := fakeChannelPosts(t, dir); len(got) != 0 {
		t.Fatalf("a refused push posted %q", got)
	}
}

// A channel that can't be reached fails nothing: the push stands and the
// landing is kept for one retry.
func TestLandingPushStandsWhenTheLandedPostFails(t *testing.T) {
	t.Parallel()
	bed := newPlainVerbBed(t)
	bed.setCommand(t, bed.script(t, "prove-green.sh", 0))
	dir := filepath.Join(filepath.Dir(bed.checkout), "unreachable-channel")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "base-url"), []byte("http://127.0.0.1:1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	appendChannelConf(t, bed.installation, dir)
	head := bed.merge(t, bed.seat(t, "goal-a"))
	if code, text := bed.run(t, "landing", "prove", "--wait"); code != 0 {
		t.Fatalf("a green prove = %d\n%s", code, text)
	}
	if code, text := bed.run(t, "landing", "push"); code != 0 || !strings.Contains(text, "pushed "+shortLandingID(head)) {
		t.Fatalf("push = %d\n%s", code, text)
	}
	if pending := channel.LoadLandedState(bed.installation).Pending; len(pending) != 1 || pending[0].SHA != head {
		t.Fatalf("pending = %+v; want the landing kept for a retry", pending)
	}
}

// A seat's own hand landing posts one landing line; a refused attempt
// before it and the repeat after it post nothing.
func TestHandLandingPostsOneLandedLine(t *testing.T) {
	t.Parallel()
	b := newDeliveryBed(t)
	dir, _ := commandFakeBed(t)
	appendChannelConf(t, b.install, dir)
	owners := &landingOwners{status: readBranch(1, "reader-record")}
	owners.install(b)
	code, result := b.do("work", "land", "standing-validation")
	expectOutcome(t, "unread unit", code, result, intentRefused)
	if got := fakeChannelPosts(t, dir); len(got) != 0 {
		t.Fatalf("a refused landing posted %q", got)
	}
	owners.status = readBranch(2, "reader-record", "reader-record")
	code, result = b.do("work", "land", "standing-validation")
	expectOutcome(t, "landed", code, result, intentConfirmed)
	if got := fakeChannelPosts(t, dir); len(got) != 1 || got[0] != "landed: standing-validation at land1" {
		t.Fatalf("posts = %q; want one landing line", got)
	}
	code, result = b.do("work", "land", "standing-validation")
	expectOutcome(t, "repeat", code, result, intentUnchanged)
	if got := fakeChannelPosts(t, dir); len(got) != 1 {
		t.Fatalf("a repeat landing posted again: %q", got)
	}
}
