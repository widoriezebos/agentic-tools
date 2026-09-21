package project

import (
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/markdown"
)

// Every link target kind, decided from the href and the document it was
// written in. A link the route would refuse is not offered as one.
func TestDocumentIDResolvesAgainstTheDocumentsDirectory(t *testing.T) {
	t.Parallel()

	for _, row := range []struct {
		name string
		from string
		href string
		id   string
	}{
		{name: "a sibling", from: "docs/a.md", href: "b.md", id: "docs/b.md"},
		{name: "a child", from: "docs/a.md", href: "design/c.md", id: "docs/design/c.md"},
		{name: "a parent", from: "docs/design/c.md", href: "../a.md", id: "docs/a.md"},
		{name: "an explicit here", from: "docs/a.md", href: "./b.md", id: "docs/b.md"},
		{name: "at the checkout root", from: "a.md", href: "b.md", id: "b.md"},
		{name: "with a fragment", from: "docs/a.md", href: "b.md#a-heading", id: "docs/b.md"},
		{name: "percent encoded", from: "docs/a.md", href: "a%20name.md", id: "docs/a name.md"},
		{name: "a missing document", from: "docs/a.md", href: "gone.md", id: "docs/gone.md"},

		{name: "an absolute path", from: "docs/a.md", href: "/etc/passwd.md"},
		{name: "out of the checkout", from: "docs/a.md", href: "../../elsewhere.md"},
		{name: "not markdown", from: "docs/a.md", href: "picture.png"},
		{name: "a refused directory", from: "a.md", href: ".git/config.md"},
		{name: "a refused directory in any case", from: "a.md", href: ".GIT/config.md"},
		{name: "a bare fragment", from: "docs/a.md", href: "#a-heading"},
		{name: "an external page", from: "docs/a.md", href: "https://example.invalid/page"},
		{name: "a script scheme", from: "docs/a.md", href: "javascript:alert(1)"},
		{name: "a data scheme", from: "docs/a.md", href: "data:text/plain,hello"},
		{name: "a mail scheme", from: "docs/a.md", href: "mailto:someone@example.invalid"},
		{name: "a file scheme", from: "docs/a.md", href: "file:///etc/passwd.md"},
		{name: "nothing at all", from: "docs/a.md", href: ""},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()

			id, resolved := DocumentID(row.from, row.href)

			testutil.Expect(t, "resolved", resolved, row.id != "")
			testutil.Expect(t, "the id", id, row.id)
		})
	}
}

// classify raises only what it can open, leaves every other target as the
// parser left it, and reaches every place an inline can be.
func TestClassifyRaisesRelativeMarkdownLinksOnly(t *testing.T) {
	t.Parallel()

	source := []byte(`# Title

A [sibling](b.md), an [external](https://example.invalid), a [fragment](#title), and a [script](javascript:alert(1)).

> A quote with a [link](c.md).

- An item with a [link](d.md).

| Head |
| --- |
| A [cell link](e.md) |
`)
	document := markdown.Parse(source)

	classify(document.Blocks, "docs/a.md")

	targets := map[string]string{}
	ids := map[string]string{}
	for _, inline := range everyLink(document.Blocks) {
		targets[inline.Href] = inline.Target
		ids[inline.Href] = inline.ID
	}
	testutil.Expect(t, "the sibling", targets["b.md"], markdown.TargetDocument)
	testutil.Expect(t, "the sibling's id", ids["b.md"], "docs/b.md")
	testutil.Expect(t, "the external page", targets["https://example.invalid"], markdown.TargetExternal)
	testutil.Expect(t, "the fragment", targets["#title"], markdown.TargetFragment)
	testutil.Expect(t, "the script scheme", targets["javascript:alert(1)"], markdown.TargetUnresolved)
	testutil.Expect(t, "the script scheme has no id", ids["javascript:alert(1)"], "")
	testutil.Expect(t, "a link inside a quote", ids["c.md"], "docs/c.md")
	testutil.Expect(t, "a link inside a list item", ids["d.md"], "docs/d.md")
	testutil.Expect(t, "a link inside a table cell", ids["e.md"], "docs/e.md")
}

func everyLink(blocks []markdown.Block) []markdown.Inline {
	links := []markdown.Inline{}
	var walkInlines func(inlines []markdown.Inline)
	walkInlines = func(inlines []markdown.Inline) {
		for _, inline := range inlines {
			if inline.Type == "link" {
				links = append(links, inline)
			}
			walkInlines(inline.Inlines)
		}
	}
	for _, block := range blocks {
		walkInlines(block.Inlines)
		links = append(links, everyLink(block.Blocks)...)
		for _, item := range block.Items {
			links = append(links, everyLink(item.Blocks)...)
		}
		for _, cell := range block.Head {
			walkInlines(cell)
		}
		for _, row := range block.Rows {
			for _, cell := range row {
				walkInlines(cell)
			}
		}
	}
	return links
}
