package main

// Decision 7 of the blocked-agent-asks-the-human design, with Wido's word
// on the line (2026-10-02): when work reaches main the channel posts the
// plain sentence of what it delivered, written by the agent that did the
// work (work land G --delivered), once per landing and with nothing added:
// no goal id, no hash. A landing with no sentence posts nothing. A refused
// push says nothing; a failed post fails nothing.

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
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

// seatSaying is bed.seat with the hand-in's sentence of what it delivers.
func (bed *plainVerbBed) seatSaying(t *testing.T, goal, sentence string) string {
	t.Helper()
	dir := filepath.Join(filepath.Dir(bed.checkout), "seat-"+goal)
	bed.git(t, filepath.Dir(bed.checkout), "clone", "--quiet", bed.origin, dir)
	bed.git(t, dir, "checkout", "--quiet", "-b", "goal/"+goal)
	if err := os.WriteFile(filepath.Join(dir, goal+".txt"), []byte(goal+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	bed.git(t, dir, "add", "-A")
	bed.git(t, dir, "commit", "--quiet", "-m", goal)
	bed.git(t, dir, "push", "--quiet", "origin", "goal/"+goal)
	sha := bed.git(t, dir, "rev-parse", "HEAD")
	if _, _, err := plain.HandIn(bed.installation, plain.Line{Goal: goal, Branch: "goal/" + goal, SHA: sha, Seat: "seat-" + goal, At: "2026-10-02T20:00:00Z", Delivered: sentence}); err != nil {
		t.Fatal(err)
	}
	return sha
}

// proveAndPush proves the lane checkout's HEAD green and pushes it.
func (bed *plainVerbBed) proveAndPush(t *testing.T) string {
	t.Helper()
	if code, text := bed.run(t, "landing", "prove", "--wait"); code != 0 {
		t.Fatalf("a green prove = %d\n%s", code, text)
	}
	code, text := bed.run(t, "landing", "push")
	if code != 0 {
		t.Fatalf("push = %d\n%s", code, text)
	}
	return text
}

const (
	sentenceA = "Seats now ask you on Telegram when they are stuck."
	sentenceB = "The channel stays quiet unless something landed."
)

// A lane batch of two posts both sentences, verbatim, in one message; the
// repeat push, which pushes nothing, posts nothing.
func TestLandingPushPostsTheDeliveredSentencesInOneMessage(t *testing.T) {
	t.Parallel()
	bed := newPlainVerbBed(t)
	dir, _ := commandFakeBed(t)
	appendChannelConf(t, bed.installation, dir)
	bed.merge(t, bed.seatSaying(t, "goal-a", sentenceA), bed.seatSaying(t, "goal-b", sentenceB))
	bed.setCommand(t, bed.script(t, "prove-green.sh", 0))
	bed.proveAndPush(t)
	want := sentenceA + "\n" + sentenceB
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

// A landing whose hand-ins carry no sentence posts nothing at all: there
// is no fallback line.
func TestLandingPushWithoutSentencePostsNothing(t *testing.T) {
	t.Parallel()
	bed := newPlainVerbBed(t)
	dir, _ := commandFakeBed(t)
	appendChannelConf(t, bed.installation, dir)
	bed.merge(t, bed.seat(t, "goal-a"))
	bed.setCommand(t, bed.script(t, "prove-green.sh", 0))
	bed.proveAndPush(t)
	if got := fakeChannelPosts(t, dir); len(got) != 0 {
		t.Fatalf("a landing with no sentence posted %q", got)
	}
}

// A refused push put nothing on main, so it posts nothing.
func TestRefusedLandingPushPostsNothing(t *testing.T) {
	t.Parallel()
	bed := newPlainVerbBed(t)
	dir, _ := commandFakeBed(t)
	appendChannelConf(t, bed.installation, dir)
	bed.merge(t, bed.seatSaying(t, "goal-a", sentenceA))
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
	dir := filepath.Join(filepath.Dir(bed.checkout), "unreachable-channel")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "base-url"), []byte("http://127.0.0.1:1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	appendChannelConf(t, bed.installation, dir)
	bed.merge(t, bed.seatSaying(t, "goal-a", sentenceA))
	bed.setCommand(t, bed.script(t, "prove-green.sh", 0))
	head := bed.git(t, bed.checkout, "rev-parse", "HEAD")
	if text := bed.proveAndPush(t); !strings.Contains(text, "pushed "+shortLandingID(head)) {
		t.Fatalf("push:\n%s", text)
	}
	if pending := channel.LoadLandedState(bed.installation).Pending; len(pending) != 1 || pending[0].SHA != head || pending[0].Text != sentenceA {
		t.Fatalf("pending = %+v; want the landing kept for a retry", pending)
	}
}

// work land G --delivered stores the sentence with the lane hand-in, and a
// repeat at the same commit with a sentence adds it to the waiting line.
func TestWorkLandDeliveredIsStoredWithTheHandIn(t *testing.T) {
	t.Parallel()
	b, _, install := plainLaneBed(t, "critic-root", "critic-root")
	code, result := b.do("work", "land", "standing-validation")
	expectOutcome(t, "hand-in", code, result, intentConfirmed)
	code, result = b.do("work", "land", "standing-validation", "--delivered", sentenceA)
	if code != 0 {
		t.Fatalf("hand-in with a sentence = %d %+v", code, result)
	}
	entries, err := plain.Entries(install)
	if err != nil || len(entries) != 1 || entries[0].Delivered != sentenceA {
		t.Fatalf("entries = %+v %v; want one waiting line carrying the sentence", entries, err)
	}
}

// work land without --delivered still hands in, and its two lines say the
// channel will stay silent and give the command with --delivered.
func TestWorkLandWithoutDeliveredHintsAndStillHandsIn(t *testing.T) {
	t.Parallel()
	b, _, install := plainLaneBed(t, "critic-root", "critic-root")
	code, result := b.do("work", "land", "standing-validation")
	expectOutcome(t, "hand-in", code, result, intentConfirmed)
	if entries, _ := plain.Entries(install); len(entries) != 1 {
		t.Fatalf("nothing was handed in: %+v", entries)
	}
	if !strings.Contains(result.Summary, "no sentence") || result.Next == nil || !strings.Contains(strings.Join(result.Next.Argv, " "), "work land standing-validation --delivered") {
		t.Fatalf("the hint: %+v", result)
	}
}

// The guard refuses a sentence that carries a commit hash or a path, in
// plain words, and hands nothing in.
func TestWorkLandDeliveredGuardRefusesHashesAndPaths(t *testing.T) {
	t.Parallel()
	b, _, install := plainLaneBed(t, "critic-root", "critic-root")
	for _, text := range []string{"Fixed the stop hook at 3ed2e2d2b.", "Rewrote internal/channel/landed.go.", "Moved it to internal/channel/inbox.go", "Ships ./bin/metasystem now.", "Fixed 3fa9c12."} {
		code, result := b.do("work", "land", "standing-validation", "--delivered", text)
		expectOutcome(t, text, code, result, intentRefused)
		if !strings.Contains(result.Summary, "plain") {
			t.Fatalf("%q: the refusal is not plain: %+v", text, result)
		}
	}
	if entries, _ := plain.Entries(install); len(entries) != 0 {
		t.Fatalf("a refused sentence handed in: %+v", entries)
	}
	// Ordinary English with a slash is not a path.
	for _, text := range []string{"Seats can ask and/or wait for you.", "The lane now runs 24/7."} {
		code, result := b.do("work", "land", "standing-validation", "--delivered", text)
		if code != 0 || result.Outcome == intentRefused {
			t.Fatalf("%q was refused: %+v", text, result)
		}
	}
}

// A goal returned and handed in again at a new commit without --delivered
// keeps its earlier sentence, and its landing posts it.
func TestReturnedGoalHandedInAgainKeepsItsSentence(t *testing.T) {
	t.Parallel()
	bed := newPlainVerbBed(t)
	dir, _ := commandFakeBed(t)
	appendChannelConf(t, bed.installation, dir)
	bed.seatSaying(t, "goal-a", sentenceA)
	if _, _, err := plain.Return(bed.installation, "goal-a", "red", time.Now()); err != nil {
		t.Fatal(err)
	}
	seat := filepath.Join(filepath.Dir(bed.checkout), "seat-goal-a")
	bed.git(t, seat, "commit", "--quiet", "--allow-empty", "-m", "the fix")
	bed.git(t, seat, "push", "--quiet", "origin", "goal/goal-a")
	fixed := bed.git(t, seat, "rev-parse", "HEAD")
	if _, _, err := plain.HandIn(bed.installation, plain.Line{Goal: "goal-a", Branch: "goal/goal-a", SHA: fixed, Seat: "seat-goal-a", At: "2026-10-02T21:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	bed.merge(t, fixed)
	bed.setCommand(t, bed.script(t, "prove-green.sh", 0))
	bed.proveAndPush(t)
	if got := fakeChannelPosts(t, dir); len(got) != 1 || got[0] != sentenceA {
		t.Fatalf("posts = %q; want [%q]", got, sentenceA)
	}
}

// A seat's own hand landing posts its sentence verbatim, once; a refused
// attempt before it and the repeat after it post nothing.
func TestHandLandingPostsTheDeliveredSentenceOnce(t *testing.T) {
	t.Parallel()
	b := newDeliveryBed(t)
	dir, _ := commandFakeBed(t)
	appendChannelConf(t, b.install, dir)
	owners := &landingOwners{status: readBranch(1, "reader-record")}
	owners.install(b)
	code, result := b.do("work", "land", "standing-validation", "--delivered", sentenceA)
	expectOutcome(t, "unread unit", code, result, intentRefused)
	if got := fakeChannelPosts(t, dir); len(got) != 0 {
		t.Fatalf("a refused landing posted %q", got)
	}
	owners.status = readBranch(2, "reader-record", "reader-record")
	code, result = b.do("work", "land", "standing-validation", "--delivered", sentenceA)
	expectOutcome(t, "landed", code, result, intentConfirmed)
	if got := fakeChannelPosts(t, dir); len(got) != 1 || got[0] != sentenceA {
		t.Fatalf("posts = %q; want [%q]", got, sentenceA)
	}
	code, result = b.do("work", "land", "standing-validation", "--delivered", sentenceA)
	expectOutcome(t, "repeat", code, result, intentUnchanged)
	if got := fakeChannelPosts(t, dir); len(got) != 1 {
		t.Fatalf("a repeat landing posted again: %q", got)
	}
}

// A hand landing with no sentence posts nothing.
func TestHandLandingWithoutSentencePostsNothing(t *testing.T) {
	t.Parallel()
	b := newDeliveryBed(t)
	dir, _ := commandFakeBed(t)
	appendChannelConf(t, b.install, dir)
	owners := &landingOwners{status: readBranch(2, "reader-record", "reader-record")}
	owners.install(b)
	code, result := b.do("work", "land", "standing-validation")
	expectOutcome(t, "landed", code, result, intentConfirmed)
	if got := fakeChannelPosts(t, dir); len(got) != 0 {
		t.Fatalf("a landing with no sentence posted %q", got)
	}
}

// The exception route moves main through the landing path; the landing
// that pushes posts its sentence verbatim.
func TestExceptionLandingPostsTheDeliveredSentence(t *testing.T) {
	b := newCarriedDeliveryBed(t)
	dir, _ := commandFakeBed(t)
	// The main checkout must stay clean for the carried landing, so the
	// channel is configured through the environment, not a .local file.
	for key, value := range map[string]string{"channel.destination.fleet.adapter": "fake", "channel.destination.fleet.fake.dir": dir, "channel.destination.fleet.fake.face": "slack"} {
		t.Setenv(config.EnvName(key), value)
	}
	_, result := b.land("standing-validation", "--exception", "missing-declaration", "--reason", "flaky host", "--by", "Wido", "--upgrade-goals")
	opid, _ := carriedResultData(result)["exception"].(string)
	if result.Outcome != intentPartial || opid == "" {
		t.Fatalf("first request: %+v", result)
	}
	if got := fakeChannelPosts(t, dir); len(got) != 0 {
		t.Fatalf("an exception that landed nothing yet posted %q", got)
	}
	proved := b.provePublicly(result)
	code, landed := b.land(append(append([]string(nil), proved.Next.Argv[3:]...), "--delivered", sentenceA)...)
	if code != 0 || landed.Outcome != intentConfirmed {
		t.Fatalf("the exception did not land: %d %+v", code, landed)
	}
	if got := fakeChannelPosts(t, dir); len(got) != 1 || got[0] != sentenceA {
		t.Fatalf("posts = %q; want [%q]", got, sentenceA)
	}
}

// The staged (--message) and recertified routes run the landing path with
// the production owners; their landed owner posts the sentence verbatim
// once per commit.
func TestLandingPathOwnersPostTheLandedLine(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.template=true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dir, _ := commandFakeBed(t)
	appendChannelConf(t, root, dir)
	owners := landingPathOwners()
	if owners.Landed == nil {
		t.Fatal("the landing path's production owners tell the channel nothing of a landing")
	}
	sha := strings.Repeat("ab", 20)
	for range 2 {
		if err := owners.Landed(root, sentenceA, sha); err != nil {
			t.Fatal(err)
		}
	}
	if got := fakeChannelPosts(t, dir); len(got) != 1 || got[0] != sentenceA {
		t.Fatalf("posts = %q; want [%q]", got, sentenceA)
	}
}

// A goal that landed one slice and hands in its next without --delivered
// does not repost the landed slice's sentence: only a returned hand-in's
// sentence carries over.
func TestNextSliceWithoutSentencePostsNothing(t *testing.T) {
	t.Parallel()
	bed := newPlainVerbBed(t)
	dir, _ := commandFakeBed(t)
	appendChannelConf(t, bed.installation, dir)
	bed.merge(t, bed.seatSaying(t, "goal-a", sentenceA))
	bed.setCommand(t, bed.script(t, "prove-green.sh", 0))
	bed.proveAndPush(t)
	seat := filepath.Join(filepath.Dir(bed.checkout), "seat-goal-a")
	bed.git(t, seat, "fetch", "--quiet", "origin")
	bed.git(t, seat, "merge", "--quiet", "--no-edit", "origin/main")
	bed.git(t, seat, "commit", "--quiet", "--allow-empty", "-m", "the next slice")
	bed.git(t, seat, "push", "--quiet", "origin", "goal/goal-a")
	next := bed.git(t, seat, "rev-parse", "HEAD")
	if _, _, err := plain.HandIn(bed.installation, plain.Line{Goal: "goal-a", Branch: "goal/goal-a", SHA: next, Seat: "seat-goal-a", At: "2026-10-02T22:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	bed.merge(t, next)
	bed.proveAndPush(t)
	if got := fakeChannelPosts(t, dir); len(got) != 1 || got[0] != sentenceA {
		t.Fatalf("posts = %q; want only the first slice's [%q]", got, sentenceA)
	}
}
