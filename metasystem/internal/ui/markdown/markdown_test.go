package markdown

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

// fixtures are the sources whose whole tree is written down in testdata, so a
// change to the schema is a change a reviewer reads in a diff.
var fixtures = []string{"blocks", "inlines", "hostile", "slugs", "mermaid", "empty"}

func TestParseMatchesTheGoldenTrees(t *testing.T) {
	t.Parallel()

	for _, name := range fixtures {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			source, err := os.ReadFile(filepath.Join("testdata", name+".md"))
			testutil.Require(t, "read the source", err, nil)
			golden, err := os.ReadFile(filepath.Join("testdata", name+".json"))
			testutil.Require(t, "read the golden tree", err, nil)

			testutil.Expect(t, "the parsed tree", encode(t, Parse(source)), string(golden))
		})
	}
}

// CRLF is accepted, and it changes nothing: the same source with Windows line
// endings parses to the same tree, carriage returns included in no segment.
func TestCarriageReturnsParseLikeTheirAbsence(t *testing.T) {
	t.Parallel()

	for _, name := range fixtures {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			source, err := os.ReadFile(filepath.Join("testdata", name+".md"))
			testutil.Require(t, "read the source", err, nil)
			windows := []byte(strings.ReplaceAll(string(source), "\n", "\r\n"))
			fromWindows := encode(t, Parse(windows))

			testutil.Expect(t, "the parsed tree", fromWindows, encode(t, Parse(source)))
			testutil.Expect(t, "a carriage return survives", strings.Contains(fromWindows, "\\r"), false)
		})
	}
}

// Nothing in the hostile source is interpreted: the tags are html nodes whose
// text is the source, the two scheme links are unresolved, and the remote
// image is an image node rather than anything the browser would fetch.
func TestHostileSourceIsCarriedAsItself(t *testing.T) {
	t.Parallel()

	source, err := os.ReadFile(filepath.Join("testdata", "hostile.md"))
	testutil.Require(t, "read the source", err, nil)

	document := Parse(source)
	blocks := document.Blocks
	htmlBlocks := []string{}
	for _, block := range blocks {
		if block.Type == "html" {
			htmlBlocks = append(htmlBlocks, block.Text)
		}
	}
	testutil.Expect(t, "the script is a block of source text", htmlBlocks, []string{"<script>alert(1)</script>\n"})

	targets := map[string]string{}
	images := []string{}
	htmlInlines := []string{}
	for _, inline := range everyInline(blocks) {
		switch inline.Type {
		case "link":
			targets[inline.Href] = inline.Target
		case "image":
			images = append(images, inline.Src)
		case "html":
			htmlInlines = append(htmlInlines, inline.Text)
		}
	}
	testutil.Expect(t, "the javascript link", targets["javascript:alert(1)"], TargetUnresolved)
	testutil.Expect(t, "the data link", targets["data:text/plain,hello"], TargetUnresolved)
	testutil.Expect(t, "the image", images, []string{"http://example.invalid/pixel.png"})
	testutil.Expect(t, "the inline tags are source text", htmlInlines, []string{
		`<img onerror="alert(1)" src="x">`, `<a href="javascript:alert(1)">`, `</a>`,
	})
}

func TestSlugsAreUniqueWithinOneDocument(t *testing.T) {
	t.Parallel()

	source, err := os.ReadFile(filepath.Join("testdata", "slugs.md"))
	testutil.Require(t, "read the source", err, nil)

	ids := []string{}
	for _, heading := range Parse(source).Headings {
		ids = append(ids, heading.ID)
	}

	testutil.Expect(t, "the heading ids", ids, []string{
		"same-heading", "same-heading-1", "same-heading-2", "a-heading-with-punctuation", "já-přece-věřím",
	})
}

// A document at the reader's limit parses; the limit is the reader's, not this
// package's, and nothing here is quadratic in the source.
func TestAMebibyteOfSourceParses(t *testing.T) {
	t.Parallel()

	paragraph := "A paragraph of ordinary prose with a [link](other/doc.md) in it.\n\n"
	source := strings.Repeat(paragraph, (1<<20)/len(paragraph)+1)

	document := Parse([]byte(source))

	testutil.Expect(t, "the source is at least a mebibyte", len(source) >= 1<<20, true)
	testutil.Expect(t, "every paragraph is a block", len(document.Blocks), strings.Count(source, paragraph))
}

func TestTargetForReadsTheHrefAlone(t *testing.T) {
	t.Parallel()

	for _, row := range []struct {
		href   string
		target string
	}{
		{"https://example.invalid/page", TargetExternal},
		{"http://example.invalid/page", TargetExternal},
		{"HTTPS://example.invalid/page", TargetUnresolved},
		{"#a-heading", TargetFragment},
		{"other/doc.md", TargetUnresolved},
		{"javascript:alert(1)", TargetUnresolved},
		{"data:text/plain,hello", TargetUnresolved},
		{"mailto:someone@example.invalid", TargetUnresolved},
		{"file:///etc/passwd", TargetUnresolved},
		{"", TargetUnresolved},
	} {
		t.Run(row.href, func(t *testing.T) {
			t.Parallel()
			testutil.Expect(t, "the target", TargetFor(row.href), row.target)
		})
	}
}

// A mermaid fence is a code block with its language, and nothing else: the
// browser shows the source, and no diagram is drawn in this build.
func TestAMermaidFenceIsACodeBlock(t *testing.T) {
	t.Parallel()

	source, err := os.ReadFile(filepath.Join("testdata", "mermaid.md"))
	testutil.Require(t, "read the source", err, nil)

	document := Parse(source)

	testutil.Require(t, "one block", len(document.Blocks), 1)
	testutil.Expect(t, "the block type", document.Blocks[0].Type, "code")
	testutil.Expect(t, "the language", document.Blocks[0].Lang, "mermaid")
	testutil.Expect(t, "the text", document.Blocks[0].Text, "graph TD;\n  A-->B;\n")
}

func TestEmptySourceIsAnEmptyDocument(t *testing.T) {
	t.Parallel()

	document := Parse(nil)

	testutil.Expect(t, "the blocks", document.Blocks, []Block{})
	testutil.Expect(t, "the headings", document.Headings, []Heading{})
}

// encode writes a tree the way the golden files are written: indented JSON,
// which a reviewer reads, and which compares as one string.
func encode(t *testing.T, document Document) string {
	t.Helper()
	encoded, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		t.Fatalf("encode the tree: %v", err)
	}
	return string(encoded) + "\n"
}

// everyInline flattens the inlines of a tree, nesting included.
func everyInline(blocks []Block) []Inline {
	out := []Inline{}
	for _, block := range blocks {
		out = append(out, flatten(block.Inlines)...)
		out = append(out, everyInline(block.Blocks)...)
		for _, item := range block.Items {
			out = append(out, everyInline(item.Blocks)...)
		}
		for _, cell := range block.Head {
			out = append(out, flatten(cell)...)
		}
		for _, row := range block.Rows {
			for _, cell := range row {
				out = append(out, flatten(cell)...)
			}
		}
	}
	return out
}

func flatten(inlines []Inline) []Inline {
	out := []Inline{}
	for _, inline := range inlines {
		out = append(out, inline)
		out = append(out, flatten(inline.Inlines)...)
	}
	return out
}
