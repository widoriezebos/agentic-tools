package httpd

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/partner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/project"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/review"
)

// The evidence read at the boundary (g1-s71 D4, §6): the listing and one file
// under the review record's Evidence path, which lies outside the checkout,
// through the one owner; and the Behaves walk handed the same listing.

const evidenced = "metasystem/plans/reviews/review-of-sync.md"

func serveEvidence(t *testing.T) (*handler, string) {
	t.Helper()
	outside := t.TempDir()
	for name, body := range map[string]string{
		"room-1280-light.png": "\x89PNG\r\n\x1a\npicture",
		"report.md":           "# Report\n\nIt ran.\n",
		"walk.txt":            "no headings\n",
	} {
		if err := os.WriteFile(filepath.Join(outside, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	document := func(id string) (project.Document, error) {
		switch id {
		case evidenced:
			return project.Document{Kind: "document", ID: id, Source: "# Review of sync\n\n- Kind: review\n- Goals: sync\n" +
				"- Reviewed: " + reviewTip + " (the tip of goal/sync)\n- Evidence: " + outside + "\n\n## Facts\n",
				Record: &project.Head{Kind: "review", Goals: []string{"sync"}}}, nil
		case reviewed:
			return project.Document{Kind: "document", ID: id, Source: reviewSource(),
				Record: &project.Head{Kind: "review", Goals: []string{"landing"}}}, nil
		case "plans/designs/sessions.md":
			return project.Document{Kind: "document", ID: id, Source: "# Sessions\n\n- Evidence: " + outside + "\n",
				Record: &project.Head{Kind: "design"}}, nil
		}
		return project.Document{}, project.ErrNotFound
	}
	info := Info{Observe: withALanding, Authority: proven(), Document: document,
		Review: &review.Owner{Git: deskGit{}, Checkout: t.TempDir()}}
	return newHandler(info, loopback(), testBundle(), func() string { return "n" }), outside
}

func TestTheEvidenceIsListedAndReadFromOutsideTheCheckout(t *testing.T) {
	t.Parallel()
	h, _ := serveEvidence(t)
	base := reviewPrefix + evidenced + "/evidence"

	listed := get(t, h, base, nil)
	testutil.Require(t, "the listing", listed.Code, http.StatusOK)
	var listing review.EvidenceListing
	testutil.Require(t, "it decodes", json.Unmarshal(listed.Body.Bytes(), &listing), nil)
	paths := []string{}
	for _, entry := range listing.Entries {
		paths = append(paths, entry.Path+" "+entry.Kind)
	}
	testutil.Expect(t, "every image and text, evidence-relative", paths,
		[]string{"report.md text", "room-1280-light.png image", "walk.txt text"})

	image := get(t, h, base+"?path=room-1280-light.png", nil)
	testutil.Require(t, "the image", image.Code, http.StatusOK)
	testutil.Expect(t, "served as what it is", image.Header().Get("Content-Type"), "image/png")
	testutil.Expect(t, "and never sniffed as anything else", image.Header().Get("X-Content-Type-Options"), "nosniff")
	testutil.Expect(t, "its bytes", image.Body.String(), "\x89PNG\r\n\x1a\npicture")

	text := get(t, h, base+"?path=walk.txt", nil)
	testutil.Require(t, "a headingless text", text.Code, http.StatusOK)
	var read review.EvidenceFile
	testutil.Require(t, "the text decodes", json.Unmarshal(text.Body.Bytes(), &read), nil)
	testutil.Expect(t, "its words", []string{read.Kind, read.Text}, []string{"text", "no headings\n"})

	escaped := get(t, h, base+"?path=../secret.txt", nil)
	testutil.Expect(t, "a path out of the evidence is refused", escaped.Code, http.StatusBadRequest)
	testutil.Expect(t, "in words", strings.Contains(escaped.Body.String(), "not a path inside the evidence"), true)

	none := get(t, h, reviewPrefix+reviewed+"/evidence", nil)
	testutil.Expect(t, "a review naming no Evidence path is refused", none.Code, http.StatusBadRequest)
	testutil.Expect(t, "saying so", strings.Contains(none.Body.String(), "names no Evidence path"), true)

	design := get(t, h, reviewPrefix+"plans/designs/sessions.md/evidence", nil)
	testutil.Expect(t, "a shaping sitting's record has no evidence read", design.Code, http.StatusBadRequest)
}

func TestTheBehavesWalkIsHandedTheEvidenceListing(t *testing.T) {
	t.Parallel()
	h, outside := serveEvidence(t)

	candidate := h.candidateFor(evidenced, "behaves")

	testutil.Require(t, "a candidate note", candidate != nil, true)
	testutil.Require(t, "with the evidence", candidate.Evidence != nil, true)
	testutil.Expect(t, "the path the record names", candidate.Evidence.Path, outside)
	testutil.Expect(t, "each entry as the Partner reads it", candidate.Evidence.Entries,
		[]string{"report.md (text, 18 B)", "room-1280-light.png (image, 15 B)", "walk.txt (text, 12 B)"})
	testutil.Expect(t, "no other walk is handed it", h.candidateFor(evidenced, "built") == nil, true)

	bare := h.candidateFor(reviewed, "behaves")
	testutil.Require(t, "a review without evidence", bare != nil && bare.Evidence != nil, true)
	testutil.Expect(t, "is told why there is none", strings.Contains(bare.Evidence.Refusal, "names no Evidence path"), true)
}

// A listing whose walk reached its bound before it found an image or text —
// a screenshot below the depth bound, or after the visited bound's other
// entries — reaches the Behaves walk as a cut, and its note says nothing was
// found within the bounds rather than that the path holds nothing.
func TestTheBehavesWalkIsToldAnEmptyCutListingMayHoldMore(t *testing.T) {
	t.Parallel()
	h, _ := serveEvidence(t)
	deep := filepath.Join(t.TempDir(), "deep-only")
	at := deep
	for depth := range review.MaxEvidenceDepth + 1 {
		at = filepath.Join(at, "level-"+strconv.Itoa(depth))
	}
	writeEvidence(t, filepath.Join(at, "room-1280-light.png"))
	wide := filepath.Join(t.TempDir(), "wide-only")
	for n := range review.MaxEvidenceVisited {
		writeEvidence(t, filepath.Join(wide, "trace-"+strconv.Itoa(n)+".bin"))
	}
	writeEvidence(t, filepath.Join(wide, "shots", "room-1280-light.png"))

	for _, named := range []string{deep, wide} {
		evidence := h.evidenceFor("# Review of sync\n\n- Kind: review\n- Evidence: " + named + "\n\n## Facts\n")
		testutil.Expect(t, named+" is cut and found nothing", []any{evidence.Cut, len(evidence.Entries), evidence.Refusal},
			[]any{true, 0, ""})
		note := partner.EvidenceNote(*evidence)
		testutil.Expect(t, named+" says the cut first", strings.HasPrefix(note, "The listing of the evidence path "+named+
			" stopped at its bound before it found an image or text: nothing was found within the listing's bounds, "+
			"and the path may hold more."), true)
		testutil.Expect(t, named+" never claims the path holds nothing", strings.Contains(note, "holds no image or text"), false)
	}
}

func writeEvidence(t *testing.T, name string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, []byte("\x89PNG\r\n\x1a\npicture"), 0o644); err != nil {
		t.Fatal(err)
	}
}
