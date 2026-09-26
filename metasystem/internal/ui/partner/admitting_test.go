package partner_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/partner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/partner/fakeacp"
)

// Whether a sitting could be opened at all, asked before anything is created for
// it.
//
// Sol's third finding: the route created the draft record a sitting was started
// on before anything had judged the purpose or asked the runtime whether it could
// take a turn, so a refusal left a record in the project nobody asked for. Two
// things are proved here — that the two refusals are reachable without writing
// anything, and that asking does not cost the turn that follows its instructions.

// admitting is a Partner over a fake runtime, with the prompts it was sent.
func admitting(t *testing.T, script fakeacp.Script) (*partner.Service, *fakeacp.Servers) {
	t.Helper()
	root := t.TempDir()
	script.Models = []string{"fake-1"}
	opener, handed := fakeacp.OpenWatched(script)
	runtime := partner.Runtime{
		Name: "fake", Model: "fake-1", ReadOnly: "a fake server reads nothing",
		Install: "install the fake with nothing at all",
	}
	host := partner.NewHostOn(runtime, root, opener)
	t.Cleanup(host.Close)
	service := partner.NewService(runtime, host,
		func(human string) (*partner.Conversation, error) { return partner.OpenConversation(root, human) },
		partner.Facts{}, func() time.Time { return time.Date(2026, 9, 26, 9, 0, 0, 0, time.UTC) })
	return service, handed
}

// A purpose this build has no moves for is refused before the runtime is even
// asked, so nothing was started and nothing could have been created.
func TestAPurposeThisBuildDoesNotOfferIsRefusedWithoutStartingAnything(t *testing.T) {
	t.Parallel()
	service, handed := admitting(t, fakeacp.Script{Chunks: []string{"never asked"}})

	err := service.Admits(context.Background(), "review")

	if err == nil {
		t.Fatalf("a review sitting was admitted by a build that has no moves for one")
	}
	testutil.Expect(t, "that it says which purposes this build has",
		strings.Contains(err.Error(), "review and learning sittings are not in this build"), true)
	_, sent := handed.First()
	testutil.Expect(t, "that the runtime was never prompted", sent, false)
	testutil.Expect(t, "and the two this build does offer are admitted",
		service.Admits(context.Background(), partner.PurposeShapeDesign), nil)
	testutil.Expect(t, "both of them",
		service.Admits(context.Background(), partner.PurposeShapeIntent), nil)
}

// A runtime that cannot start refuses the admission with its own words and the
// line that installs it — which is the refusal the route has to have BEFORE it
// creates a draft record.
func TestARuntimeThatCannotStartRefusesTheAdmissionWithItsOwnWords(t *testing.T) {
	t.Parallel()
	service, _ := admitting(t, fakeacp.Script{InitError: "not signed in"})

	err := service.Admits(context.Background(), partner.PurposeShapeDesign)

	if err == nil {
		t.Fatalf("a runtime that refuses initialize admitted a sitting")
	}
	testutil.Expect(t, "that it carries the runtime's words",
		strings.Contains(err.Error(), "not signed in"), true)
}

// And asking costs the opening turn nothing.
//
// Admitting the runtime starts its process, so the session the opening turn is
// sent into was opened by the admission rather than by the turn. It is still that
// session's first prompt, and it is still given how to answer here — a build that
// read freshness from the start alone would have sent the first turn of every
// sitting without its instructions.
func TestTheOpeningTurnIsStillTheSessionsFirstPromptAfterAnAdmission(t *testing.T) {
	t.Parallel()
	service, handed := admitting(t, fakeacp.Script{Chunks: []string{"Two rulings touch this."}})
	events, stop := service.Subscribe()
	defer stop()

	testutil.Require(t, "the admission", service.Admits(context.Background(), partner.PurposeShapeDesign), nil)
	_, err := service.Sit(context.Background(), "Wido", subjectOf(), partner.PurposeShapeDesign, onTheRecord())
	testutil.Require(t, "the sitting opened", err, nil)
	drain(t, events)

	prompt, sent := handed.First()
	testutil.Require(t, "the opening turn was sent", sent, true)
	testutil.Expect(t, "with the standing rule in front of it",
		strings.HasPrefix(prompt, "You are the Project Partner for this MetaSystem workspace."), true)
	testutil.Expect(t, "and how to answer here",
		strings.Contains(prompt, "How to answer here, from this kit's own "+partner.SkillPath), true)
	testutil.Expect(t, "and the opening request the interface asks in the human's name",
		strings.Contains(prompt, "Bring what the records already hold about it"), true)
}
