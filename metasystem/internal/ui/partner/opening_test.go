package partner

import (
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

// One opener per transcript (Astra F-02).
//
// Opening a conversation used to be an unsynchronised read: two callers that
// both found no cached object each ran a loader, and whichever finished second
// was discarded. That was harmless while a load only read the file. It is not
// harmless now that a load REPAIRS it (R-131-ui): the loser has already
// archived, and already rewritten the transcript from the reading it took
// before the winner appended anything — so a message accepted after the
// recovery is in neither the rewritten file nor either archive.
//
// So the first open of one key is serialized through the service's own
// ownership: one opener loads, and every other caller waits for it and takes
// the object it installed.

func TestConcurrentFirstOpensOfOneTranscriptRecoverItOnce(t *testing.T) {
	t.Parallel()
	directory, path, _ := archiveUnreadableBed(t)

	// The gate is in the loader itself: the first one announces that it is
	// inside and waits, so the second caller arrives while the first is
	// halfway through the repair rather than after it. A second loader that
	// entered here announces itself too, which is what the count below reads.
	var loaders int64
	inside := make(chan struct{}, 4)
	proceed := make(chan struct{})
	service := NewService(Runtime{}, nil, func(human string) (*Conversation, error) {
		first := atomic.AddInt64(&loaders, 1) == 1
		inside <- struct{}{}
		if first {
			<-proceed
		}
		return OpenConversation(directory, human)
	}, Facts{}, nil)

	type opened struct {
		conversation *Conversation
		err          error
	}
	firstOpen := make(chan opened, 1)
	go func() {
		conversation, err := service.conversation("Wido")
		firstOpen <- opened{conversation, err}
	}()
	<-inside

	secondOpen := make(chan opened, 1)
	running := make(chan struct{})
	go func() {
		close(running)
		conversation, err := service.conversation("Wido")
		secondOpen <- opened{conversation, err}
	}()
	<-running
	close(proceed)

	one, two := <-firstOpen, <-secondOpen
	testutil.Require(t, "the first caller has a conversation", one.err, nil)
	testutil.Require(t, "and so has the second", two.err, nil)
	testutil.Expect(t, "one loader read the transcript", atomic.LoadInt64(&loaders), int64(1))
	testutil.Expect(t, "and both callers hold that one conversation",
		two.conversation == one.conversation, true)
	testutil.Expect(t, "the original was archived once", len(archiveArchives(t, directory)), 1)
	testutil.Expect(t, "and the human is told once", archiveNotices(one.conversation.Messages(0)), 1)

	// And the message accepted after the recovery is in the file the second
	// caller reads. A discarded loader that rewrote the transcript from its own
	// older reading would have taken it out again.
	testutil.Require(t, "a message is accepted after the recovery",
		one.conversation.Append(Message{ID: "m5", Turn: "t3", Role: RoleHuman,
			Text: "said after the recovery"}), nil)
	body, err := os.ReadFile(path)
	testutil.Require(t, "the transcript reads", err, nil)
	testutil.Expect(t, "it carries what was accepted",
		strings.Contains(string(body), "said after the recovery"), true)
	held := two.conversation.Messages(0)
	testutil.Require(t, "the second caller reads a conversation", len(held) > 0, true)
	testutil.Expect(t, "whose last message is the one just accepted", held[len(held)-1].ID, "m5")
	testutil.Expect(t, "with the messages the repair kept before it",
		archiveIDs(held[:len(held)-2]), "m1,m3,m4")
}
