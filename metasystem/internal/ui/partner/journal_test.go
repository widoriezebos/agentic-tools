package partner_test

import (
	"bufio"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/partner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/partner/fakeacp"
)

// The wire journal rotates through its own writer while a connection is writing
// through it, and the connection goes on writing into the fresh file.
//
// This is Astra's F3 on g1-s54. A rename from outside would leave the runtime's
// open descriptor writing the renamed file: every frame after the rotation would
// land in the copy nobody reads, and wire.jsonl would stay empty until the
// process ended. So the rotation is the writer's, and the proof is a live
// connection: the handshake is journalled, the journal rotates, and the turn
// that follows is in the file the journal now names.
func TestTheWireJournalRotatesUnderALiveConnectionAndKeepsWriting(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	journal := partner.NewJournal(filepath.Join(directory, "wire.jsonl"))
	host := hostJournalling(t, journal, fakeacp.Script{Chunks: []string{"after the rotation"}})

	_, err := host.Ready(context.Background())
	testutil.Require(t, "the session opened", err, nil)
	handshake := lines(t, journal.Path())
	testutil.Require(t, "the handshake was journalled", len(handshake) > 0, true)

	// A bound of one byte is past already, so the rotation happens on the first
	// ask — between two of the connection's writes, because a write takes the
	// same lock.
	rotated, err := journal.Rotate(1)
	testutil.Require(t, "rotating", err, nil)
	testutil.Expect(t, "it rotated", rotated, true)

	result, err := host.Prompt(context.Background(), "and now?", func(partner.Update) {})
	testutil.Require(t, "the turn ran", err, nil)
	testutil.Expect(t, "the turn completed over the rotated journal", result.Outcome, partner.OutcomeComplete)

	after := lines(t, journal.Path())
	previous := lines(t, journal.Previous())
	testutil.Expect(t, "the handshake is in the previous", containsFrame(previous, "initialize"), true)
	testutil.Expect(t, "and not in the fresh file", containsFrame(after, "initialize"), false)
	testutil.Expect(t, "the turn after the rotation is in the fresh file",
		containsFrame(after, "session/prompt"), true)
}

// One previous is kept, and the older one goes with it. A second rotation
// replaces the previous rather than adding a third file, so the journal's whole
// cost on disk is at most twice its bound.
func TestTheWireJournalKeepsOnePreviousAndTheOlderGoes(t *testing.T) {
	t.Parallel()
	journal := partner.NewJournal(filepath.Join(t.TempDir(), "wire.jsonl"))
	sink, err := journal.Open()
	testutil.Require(t, "opening", err, nil)
	t.Cleanup(func() { _ = sink.Close() })

	write(t, sink, "first")
	rotated, err := journal.Rotate(1)
	testutil.Require(t, "the first rotation", err, nil)
	testutil.Expect(t, "it rotated", rotated, true)
	write(t, sink, "second")
	rotated, err = journal.Rotate(1)
	testutil.Require(t, "the second rotation", err, nil)
	testutil.Expect(t, "it rotated again", rotated, true)
	write(t, sink, "third")

	testutil.Expect(t, "the current file holds what was written after the last rotation",
		strings.Join(lines(t, journal.Path()), ""), "third")
	testutil.Expect(t, "the previous holds the one before it",
		strings.Join(lines(t, journal.Previous()), ""), "second")
	held, err := os.ReadDir(filepath.Dir(journal.Path()))
	testutil.Require(t, "reading the directory", err, nil)
	testutil.Expect(t, "and there are two files and no third", len(held), 2)
}

// Nothing is lost and no line is torn while the journal rotates under writers,
// because a write and a rotation take the same lock: every line written is in
// one of the two files, whole, exactly once.
func TestNoLineIsTornOrLostWhenTheJournalRotatesUnderWriters(t *testing.T) {
	t.Parallel()
	journal := partner.NewJournal(filepath.Join(t.TempDir(), "wire.jsonl"))
	sink, err := journal.Open()
	testutil.Require(t, "opening", err, nil)
	t.Cleanup(func() { _ = sink.Close() })

	// Enough is written first that the bound is already passed, so the one
	// rotation lands in the middle of the concurrent writes rather than after
	// them.
	for at := 0; at < 50; at++ {
		write(t, sink, "before-"+strconv.Itoa(at))
	}
	const writers, each = 8, 40
	started := make(chan struct{})
	var writing sync.WaitGroup
	for writer := 0; writer < writers; writer++ {
		writing.Add(1)
		go func(writer int) {
			defer writing.Done()
			<-started
			for at := 0; at < each; at++ {
				_, _ = sink.Write([]byte("during-" + strconv.Itoa(writer) + "-" + strconv.Itoa(at) + "\n"))
			}
		}(writer)
	}
	close(started)
	rotated, err := journal.Rotate(1)
	testutil.Require(t, "rotating under the writers", err, nil)
	testutil.Expect(t, "it rotated", rotated, true)
	writing.Wait()

	seen := map[string]int{}
	for _, line := range append(lines(t, journal.Previous()), lines(t, journal.Path())...) {
		seen[line]++
	}
	wrong := []string{}
	for at := 0; at < 50; at++ {
		if line := "before-" + strconv.Itoa(at); seen[line] != 1 {
			wrong = append(wrong, line+" appears "+strconv.Itoa(seen[line])+" times")
		}
	}
	for writer := 0; writer < writers; writer++ {
		for at := 0; at < each; at++ {
			line := "during-" + strconv.Itoa(writer) + "-" + strconv.Itoa(at)
			if seen[line] != 1 {
				wrong = append(wrong, line+" appears "+strconv.Itoa(seen[line])+" times")
			}
		}
	}
	testutil.Expect(t, "every line written is in one of the two files exactly once, whole: "+
		strings.Join(wrong, "; "), len(wrong), 0)
	testutil.Expect(t, "and nothing else is in either file", len(seen), 50+writers*each)
}

// A journal under its bound is left alone, and a bound of zero is a bound that
// is off — the one way this store's bounds are disabled (g1-s54 D4).
func TestAJournalUnderItsBoundOrWithTheBoundOffIsNotRotated(t *testing.T) {
	t.Parallel()
	journal := partner.NewJournal(filepath.Join(t.TempDir(), "wire.jsonl"))
	sink, err := journal.Open()
	testutil.Require(t, "opening", err, nil)
	t.Cleanup(func() { _ = sink.Close() })
	write(t, sink, "a short life")

	rotated, err := journal.Rotate(8 << 20)
	testutil.Require(t, "asking under the bound", err, nil)
	testutil.Expect(t, "nothing rotated", rotated, false)

	rotated, err = journal.Rotate(0)
	testutil.Require(t, "asking with the bound off", err, nil)
	testutil.Expect(t, "nothing rotated then either", rotated, false)
	_, err = os.Stat(journal.Previous())
	testutil.Expect(t, "and no previous was made", os.IsNotExist(err), true)
	testutil.Expect(t, "the journal still holds what was written",
		strings.Join(lines(t, journal.Path()), ""), "a short life")
}

// A journal no process has open is left alone. The next process truncates it,
// so rotating it would keep a dead process's frames as the one previous and
// make the store bigger rather than smaller.
func TestAJournalNoProcessHasOpenIsLeftAlone(t *testing.T) {
	t.Parallel()
	journal := partner.NewJournal(filepath.Join(t.TempDir(), "wire.jsonl"))
	sink, err := journal.Open()
	testutil.Require(t, "opening", err, nil)
	write(t, sink, "one process's frames")
	testutil.Require(t, "closing", sink.Close(), nil)

	rotated, err := journal.Rotate(1)
	testutil.Require(t, "asking a closed journal", err, nil)
	testutil.Expect(t, "nothing rotated", rotated, false)
	_, err = os.Stat(journal.Previous())
	testutil.Expect(t, "and no previous was made", os.IsNotExist(err), true)

	// The next process opens it again and starts from nothing, which is the
	// bound the journal has always had across processes.
	next, err := journal.Open()
	testutil.Require(t, "opening again", err, nil)
	t.Cleanup(func() { _ = next.Close() })
	testutil.Expect(t, "the file was truncated", journal.Size(), int64(0))
	testutil.Expect(t, "and holds nothing", len(lines(t, journal.Path())), 0)
}

// An old runtime cannot close the new runtime's journal, and it cannot write
// into it either.
//
// This is Sol's read of g1-s54 under R-124. One writer serves every runtime a
// host spawns, and Host.Ready may see a connection as dead and open the next
// runtime's journal before the old endpoint's close callback has run. A close
// that was not bound to its own open would then close the descriptor the new
// runtime writes through, and every frame after it would report success and land
// nowhere. So the ordering is forced here: the old open's teardown runs last.
func TestAnOldRuntimesTeardownCannotCloseOrWriteTheNewRuntimesJournal(t *testing.T) {
	t.Parallel()
	journal := partner.NewJournal(filepath.Join(t.TempDir(), "wire.jsonl"))
	old, err := journal.Open()
	testutil.Require(t, "the old runtime's journal opened", err, nil)
	write(t, old, "the old runtime's last frame")

	fresh, err := journal.Open()
	testutil.Require(t, "the new runtime's journal opened", err, nil)
	t.Cleanup(func() { _ = fresh.Close() })
	testutil.Expect(t, "the new open is a new generation", fresh.Generation() != old.Generation(), true)

	testutil.Expect(t, "the old runtime's close closes nothing and says why",
		errors.Is(old.Close(), partner.ErrStaleJournal), true)
	_, err = old.Write([]byte("a replaced runtime's frame\n"))
	testutil.Expect(t, "and its writes are refused rather than reported as written",
		errors.Is(err, partner.ErrStaleJournal), true)

	write(t, fresh, "the new runtime's frame")
	testutil.Expect(t, "so the journal holds the new runtime's frames and only those",
		lines(t, journal.Path()), []string{"the new runtime's frame"})
}

// The same ordering through the host that produces it: the old session is torn
// down with its journal close held, the next session opens the journal, the held
// teardown runs, and the turn that follows is journalled.
func TestTheNewSessionsFramesLandAfterTheOldSessionsTeardownRuns(t *testing.T) {
	t.Parallel()
	journal := partner.NewJournal(filepath.Join(t.TempDir(), "wire.jsonl"))
	host, closes := hostJournallingHoldingCloses(t, journal,
		fakeacp.Script{Chunks: []string{"after the old runtime went"}}, true)

	_, err := host.Ready(context.Background())
	testutil.Require(t, "the first session opened", err, nil)
	host.Close()
	_, err = host.Ready(context.Background())
	testutil.Require(t, "the next session opened", err, nil)

	// The old endpoint's teardown, forced to run after the new open.
	teardown := <-closes
	teardown()

	result, err := host.Prompt(context.Background(), "and now?", func(partner.Update) {})
	testutil.Require(t, "the turn ran", err, nil)
	testutil.Expect(t, "the turn completed", result.Outcome, partner.OutcomeComplete)
	testutil.Expect(t, "and its frames are in the journal the old teardown did not close",
		containsFrame(lines(t, journal.Path()), "session/prompt"), true)
}

// The host that spawns a runtime's own command holds the journal's writer, so
// housekeeping has something to ask; a host a test or the walkthrough opens
// over its own endpoint holds none.
func TestTheHostHoldsTheJournalsWriterOnlyWhereItKeepsAJournal(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "wire.jsonl")
	spawning := partner.NewHost(partner.Runtime{Name: "claude", Argv: []string{"true"}}, t.TempDir(), path)
	testutil.Require(t, "the spawning host keeps one", spawning.Journal() != nil, true)
	testutil.Expect(t, "at the path it was given", spawning.Journal().Path(), path)
	testutil.Expect(t, "and names where a previous goes", spawning.Journal().Previous(),
		filepath.Join(filepath.Dir(path), "wire.1.jsonl"))

	named := partner.NewHost(partner.Runtime{Name: "claude", Argv: []string{"true"}}, t.TempDir(), "")
	testutil.Expect(t, "a host with no journal path keeps no writer", named.Journal() == nil, true)
	over := partner.NewHostOn(partner.Runtime{Name: "claude"}, t.TempDir(), fakeacp.Open(fakeacp.Script{}))
	t.Cleanup(over.Close)
	testutil.Expect(t, "and neither does one opened over an endpoint", over.Journal() == nil, true)
}

// hostJournalling is a host over the fake server with the journal attached as
// the connection's sink, which is what the spawning host does with the file it
// opens.
func hostJournalling(t *testing.T, journal *partner.Journal, script fakeacp.Script) *partner.Host {
	t.Helper()
	host, _ := hostJournallingHoldingCloses(t, journal, script, false)
	return host
}

// hostJournallingHoldingCloses is the same host, except that with hold set each
// endpoint's journal close is handed to the caller rather than run: the test
// decides when an old runtime's teardown happens, which is the ordering the
// generation exists to survive.
func hostJournallingHoldingCloses(t *testing.T, journal *partner.Journal, script fakeacp.Script,
	hold bool) (*partner.Host, chan func()) {
	t.Helper()
	opener := fakeacp.Open(script)
	closes := make(chan func(), 4)
	// Whatever the test left held is run before the descriptors leave with the
	// process, after the host's own teardown has handed over the last of them.
	t.Cleanup(func() {
		for {
			select {
			case teardown := <-closes:
				teardown()
			default:
				return
			}
		}
	})
	host := partner.NewHostOn(partner.Runtime{Name: "claude"}, t.TempDir(),
		func(ctx context.Context) (partner.Endpoint, error) {
			endpoint, err := opener(ctx)
			if err != nil {
				return endpoint, err
			}
			sink, err := journal.Open()
			if err != nil {
				return endpoint, err
			}
			endpoint.Journal = sink
			closing := endpoint.Close
			endpoint.Close = func() {
				closing()
				if hold {
					closes <- func() { _ = sink.Close() }
					return
				}
				_ = sink.Close()
			}
			return endpoint, nil
		})
	t.Cleanup(host.Close)
	return host, closes
}

func write(t *testing.T, sink *partner.Sink, line string) {
	t.Helper()
	_, err := sink.Write([]byte(line + "\n"))
	testutil.Require(t, "writing "+line, err, nil)
}

// lines is one file's whole lines, and nothing that is not one.
func lines(t *testing.T, path string) []string {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("cannot read %s: %v", path, err)
	}
	defer func() { _ = file.Close() }()
	read := []string{}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for scanner.Scan() {
		read = append(read, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("cannot scan %s: %v", path, err)
	}
	return read
}

func containsFrame(read []string, method string) bool {
	for _, line := range read {
		if strings.Contains(line, `"method":"`+method+`"`) {
			return true
		}
	}
	return false
}
