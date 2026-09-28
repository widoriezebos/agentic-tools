package partner_test

import (
	"context"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/partner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/partner/fakeacp"
)

// D3: a review keeps its five walks and a shaping sitting has four of its own,
// each a fixed request with the interface's provenance; a part the sitting's
// purpose does not have is refused in words.
func TestTheWalksAreKeyedByPurpose(t *testing.T) {
	t.Parallel()
	held := reviewService(t, fakeacp.Script{Chunks: []string{"Today: ..."}})
	events, stop := held.service.Subscribe()
	defer stop()
	ctx := context.Background()

	testutil.Expect(t, "a review's five", partner.Walks[partner.PurposeReview],
		[]string{"asked", "built", "examined", "proven", "behaves"})
	testutil.Expect(t, "a design's four", partner.Walks[partner.PurposeShapeDesign],
		[]string{"records", "today", "cases", "open"})
	testutil.Expect(t, "an intent's four", partner.Walks[partner.PurposeShapeIntent],
		[]string{"records", "today", "cases", "open"})

	sitting, err := held.service.Sit(ctx, "Wido", designOf(design), partner.PurposeShapeDesign, inTheRoom(design))
	testutil.Require(t, "the design sitting opened", err, nil)
	drain(t, events)
	_, err = held.service.Walk(ctx, "Wido", design, "today", inTheRoom(design))
	testutil.Require(t, "the Today walk", err, nil)
	drain(t, events)
	read, err := held.service.SnapshotIn("Wido", design, 100)
	testutil.Require(t, "read back", err, nil)
	asked := read.Messages[len(read.Messages)-2]
	testutil.Expect(t, "the interface's own question", asked.Interface, true)
	testutil.Expect(t, "its fixed words", asked.Text, partner.WalkRequest("today", sitting))
	testutil.Expect(t, "which names the walk and the sitting", strings.HasPrefix(asked.Text,
		"Walk me through Today for the sitting on "+design+":"), true)
	testutil.Expect(t, "read from the checkout as it stands", strings.Contains(asked.Text,
		"read the checkout as it stands with your own reads"), true)
	testutil.Expect(t, "answered with the lines the desk reads", strings.Contains(asked.Text, "path:lines"), true)
	testutil.Expect(t, "and never weighed", strings.Contains(asked.Text, "Weigh nothing"), true)
	said := map[string]bool{}
	for _, part := range partner.Walks[partner.PurposeShapeDesign] {
		said[partner.WalkRequest(part, sitting)] = true
	}
	testutil.Expect(t, "four requests, each its own", len(said), 4)

	_, err = held.service.Walk(ctx, "Wido", design, "built", inTheRoom(design))
	testutil.Expect(t, "a review's walk in a shaping room", err.Error(),
		"a walk of this sitting is one of records, today, cases, open; built is none of them")
	_, err = held.service.Walk(ctx, "Wido", reviewB, "built", inTheRoom(reviewB))
	testutil.Expect(t, "a walk where no sitting stands", err.Error(),
		"no sitting is open on "+reviewB+", so there is nothing to walk through")
}
