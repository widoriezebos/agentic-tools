package review

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

// The evidence owner (g1-s71 D4, §6): one owner bounded to the path the review
// record's Evidence line names, which is usually outside the checkout, by an
// anchored root open. It answers two reads — a listing and one file — and the
// listing, present and the browser share one evidence-relative path.

func evidenceFixture(t *testing.T) (Owner, string) {
	t.Helper()
	home := t.TempDir()
	checkout := t.TempDir()
	evidence := filepath.Join(home, "evidence", "ledger-sync")
	writeFile(t, filepath.Join(evidence, "room-1280-light.png"), "\x89PNG\r\n\x1a\nnot really a picture")
	writeFile(t, filepath.Join(evidence, "report.md"), "# Report\n\n## What ran\n\nThe room opened.\n")
	writeFile(t, filepath.Join(evidence, "notes", "walk.txt"), "no headings here\njust lines\n")
	writeFile(t, filepath.Join(evidence, "trace.bin"), "\x00\x01\x02")
	writeFile(t, filepath.Join(home, "secret.txt"), "outside the evidence\n")
	return Owner{Checkout: checkout, Home: home}, "~/evidence/ledger-sync/"
}

func TestTheEvidenceLineIsReadFromTheRecordsHead(t *testing.T) {
	t.Parallel()
	source := "# Review of g1-s9\n\n- Kind: review\n- Goals: g1-s9\n- Evidence: ~/evidence/ledger-sync/\n\n## Facts\n\n- Evidence: not the head\n"
	testutil.Expect(t, "the head's line", EvidenceIn(source), "~/evidence/ledger-sync/")
	testutil.Expect(t, "no line, no path", EvidenceIn("# Review\n\n- Kind: review\n\n## Facts\n\n- Evidence: words\n"), "")
}

func TestTheListingNamesImagesAndTextUnderTheEvidencePathOnly(t *testing.T) {
	t.Parallel()
	owner, named := evidenceFixture(t)

	listing, err := owner.EvidenceList(named)

	testutil.Require(t, "the listing", err, nil)
	testutil.Expect(t, "images and text, by evidence-relative path, in order", listing.Entries, []EvidenceEntry{
		{Path: "notes/walk.txt", Kind: EvidenceText, Size: 28},
		{Path: "report.md", Kind: EvidenceText, Size: 40},
		{Path: "room-1280-light.png", Kind: EvidenceImage, Size: 28},
	})
	testutil.Expect(t, "the counts", []int{listing.Supplied, listing.Total}, []int{3, 3})
	testutil.Expect(t, "a tree under the bounds is the whole", listing.Cut, false)
}

func TestTheListingIsBoundedAtFiveHundredEntries(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	dir := filepath.Join(home, "many")
	for at := range MaxEvidenceEntries + 3 {
		writeFile(t, filepath.Join(dir, "shot-"+strings.Repeat("0", 4-len(itoa(at)))+itoa(at)+".png"), "x")
	}
	listing, err := Owner{Home: home}.EvidenceList("~/many")
	testutil.Require(t, "the listing", err, nil)
	testutil.Expect(t, "bounded, and says the whole", []int{len(listing.Entries), listing.Supplied, listing.Total},
		[]int{MaxEvidenceEntries, MaxEvidenceEntries, MaxEvidenceEntries + 3})
	testutil.Expect(t, "the whole walked, so no cut", listing.Cut, false)
}

// The walk's work is bounded, not only its answer: a tree wider than the
// visited bound and deeper than the depth bound is answered within both, with
// the cut named, and nothing past the depth bound is listed.
func TestTheListingsWalkStopsAtItsBoundsAndSaysItIsAPart(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	dir := filepath.Join(home, "wide")
	for at := range MaxEvidenceVisited + 50 {
		writeFile(t, filepath.Join(dir, "d"+itoa(at%10), "shot-"+itoa(at)+".png"), "x")
	}
	deep := filepath.Join(home, "deep")
	for depth := range MaxEvidenceDepth + 3 {
		deep = filepath.Join(deep, "level-"+itoa(depth))
	}
	writeFile(t, filepath.Join(deep, "bottom.txt"), "the bottom\n")
	writeFile(t, filepath.Join(home, "deep", "top.txt"), "the top\n")

	listing, err := Owner{Home: home}.EvidenceList("~/wide")
	testutil.Require(t, "the wide listing", err, nil)
	testutil.Expect(t, "the walk stopped within the visited bound", listing.Total <= MaxEvidenceVisited, true)
	testutil.Expect(t, "the answer bounded", []int{len(listing.Entries), listing.Supplied}, []int{MaxEvidenceEntries, MaxEvidenceEntries})
	testutil.Expect(t, "the wide cut named", listing.Cut, true)

	listing, err = Owner{Home: home}.EvidenceList("~/deep")
	testutil.Require(t, "the deep listing", err, nil)
	testutil.Expect(t, "the top listed, the bottom past the depth bound not", listing.Entries,
		[]EvidenceEntry{{Path: "top.txt", Kind: EvidenceText, Size: 8}})
	testutil.Expect(t, "the deep cut named", listing.Cut, true)
}

// A walk that reaches its bound before it finds any image or text answers an
// empty listing that is cut, never one that claims the path holds nothing: a
// screenshot below the depth bound, or after the visited bound's other entries.
func TestAListingCutBeforeAnyFileIsFoundIsEmptyAndSaysItIsCut(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	deep := filepath.Join(home, "deep-only")
	for depth := range MaxEvidenceDepth + 1 {
		deep = filepath.Join(deep, "level-"+itoa(depth))
	}
	writeFile(t, filepath.Join(deep, "room-1280-light.png"), "\x89PNG\r\n\x1a\npicture")
	for at := range MaxEvidenceVisited {
		writeFile(t, filepath.Join(home, "wide-only", "trace-"+itoa(at)+".bin"), "x")
	}
	writeFile(t, filepath.Join(home, "wide-only", "shots", "room-1280-light.png"), "\x89PNG\r\n\x1a\npicture")

	for _, named := range []string{"~/deep-only", "~/wide-only"} {
		listing, err := Owner{Home: home}.EvidenceList(named)
		testutil.Require(t, named+" listed", err, nil)
		testutil.Expect(t, named+" found nothing within the bounds", []int{len(listing.Entries), listing.Supplied, listing.Total}, []int{0, 0, 0})
		testutil.Expect(t, named+" says it is cut", listing.Cut, true)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := ""
	for ; n > 0; n /= 10 {
		digits = string(rune('0'+n%10)) + digits
	}
	return digits
}

func TestOneEvidenceFileIsAnImageOrTextFromOutsideTheCheckout(t *testing.T) {
	t.Parallel()
	owner, named := evidenceFixture(t)

	image, err := owner.EvidenceFile(named, "room-1280-light.png")
	testutil.Require(t, "the image", err, nil)
	testutil.Expect(t, "an image, its type and bytes", []string{image.Kind, image.Type, string(image.Body)},
		[]string{EvidenceImage, "image/png", "\x89PNG\r\n\x1a\nnot really a picture"})

	report, err := owner.EvidenceFile(named, "report.md")
	testutil.Require(t, "the report", err, nil)
	testutil.Expect(t, "text, whole", []any{report.Kind, report.Text, report.Supplied, report.Total},
		[]any{EvidenceText, "# Report\n\n## What ran\n\nThe room opened.\n", 5, 5})

	plain, err := owner.EvidenceFile(named, "notes/walk.txt")
	testutil.Require(t, "a headingless text", err, nil)
	testutil.Expect(t, "a report without headings is a report", plain.Text, "no headings here\njust lines\n")
	// The section renderer's own blocks, over the bytes this read returned: a
	// report's headings, and a headingless text's paragraphs.
	kinds := func(read EvidenceFile) []string {
		said := []string{}
		for _, block := range read.Blocks {
			said = append(said, block.Type)
		}
		return said
	}
	testutil.Expect(t, "the report's blocks", kinds(report), []string{"heading", "heading", "paragraph"})
	testutil.Expect(t, "the headingless text's", kinds(plain), []string{"paragraph"})

	// A relative Evidence line is the checkout's: the path is joined to it.
	writeFile(t, filepath.Join(owner.Checkout, "artifacts", "evidence", "shot.png"), "png")
	inside, err := owner.EvidenceFile("artifacts/evidence", "shot.png")
	testutil.Require(t, "a relative evidence path", err, nil)
	testutil.Expect(t, "read under the checkout", string(inside.Body), "png")
}

func TestEvidenceOutsideThePathIsRefusedInWords(t *testing.T) {
	t.Parallel()
	owner, named := evidenceFixture(t)
	outside := filepath.Join(owner.Home, "secret.txt")
	if err := os.Symlink(outside, filepath.Join(owner.Home, "evidence", "ledger-sync", "link.txt")); err != nil {
		t.Fatal(err)
	}
	for _, asked := range []string{"../../secret.txt", "/etc/hosts", "notes/../../../secret.txt", "", "link.txt", "notes", "missing.png"} {
		_, err := owner.EvidenceFile(named, asked)
		var refusal *Refusal
		if !errors.As(err, &refusal) {
			t.Fatalf("%q: want a refusal in words, got %v", asked, err)
		}
	}
	_, err := owner.EvidenceFile(named, "trace.bin")
	var refusal *Refusal
	testutil.Expect(t, "a kind that is neither image nor text is refused", errors.As(err, &refusal), true)
	testutil.Expect(t, "and says so", strings.Contains(err.Error(), "images and text"), true)

	listing, err := owner.EvidenceList(named)
	testutil.Require(t, "the listing", err, nil)
	for _, entry := range listing.Entries {
		if entry.Path == "link.txt" {
			t.Fatalf("the listing names a link out of the evidence path")
		}
	}

	_, err = owner.EvidenceList("")
	testutil.Expect(t, "no Evidence line is refused", errors.As(err, &refusal), true)
	_, err = owner.EvidenceList("~/nowhere")
	testutil.Expect(t, "a path that is not there is refused", errors.As(err, &refusal), true)
}

func TestEvidenceTextIsBoundedAndMustBeText(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	dir := filepath.Join(home, "run")
	writeFile(t, filepath.Join(dir, "long.log"), strings.Repeat("a line\n", MaxEvidenceLines+10))
	writeFile(t, filepath.Join(dir, "latin.txt"), "caf\xe9\n")
	writeFile(t, filepath.Join(dir, "huge.txt"), strings.Repeat("x", MaxEvidenceBytes+1))
	owner := Owner{Home: home}

	long, err := owner.EvidenceFile("~/run", "long.log")
	testutil.Require(t, "a long log", err, nil)
	testutil.Expect(t, "the line bound, saying the whole", []int{long.Supplied, long.Total, strings.Count(long.Text, "\n")},
		[]int{MaxEvidenceLines, MaxEvidenceLines + 10, MaxEvidenceLines})

	_, err = owner.EvidenceFile("~/run", "latin.txt")
	testutil.Expect(t, "bytes that are not UTF-8 are refused", err != nil && strings.Contains(err.Error(), "UTF-8"), true)
	_, err = owner.EvidenceFile("~/run", "huge.txt")
	testutil.Expect(t, "past four megabytes is refused", err != nil && strings.Contains(err.Error(), "larger"), true)
}

func TestAShapingReadNamesTheCheckoutsHead(t *testing.T) {
	t.Parallel()
	checkout := t.TempDir()
	writeFile(t, filepath.Join(checkout, "owner.go"), "package owner\n")
	owner := Owner{Checkout: checkout, Git: fakeGit{refs: map[string]string{"HEAD": commit("a")}}}

	read, err := owner.AsItStands("owner.go", 0, 0)

	testutil.Require(t, "the read", err, nil)
	testutil.Expect(t, "the head at the moment of the read, as provenance", read.Head, commit("a"))
	testutil.Expect(t, "and still no commit pinned", read.Commit, "")
}
