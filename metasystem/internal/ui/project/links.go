package project

import (
	"net/url"
	"path"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/markdown"
)

// classify raises the relative Markdown links of a parsed tree to document
// references. Every other href keeps the classification the parser gave it, so
// a scheme this interface does not open stays unresolved and is rendered as
// text rather than as an anchor.
func classify(blocks []markdown.Block, from string) {
	for index := range blocks {
		block := &blocks[index]
		classifyInlines(block.Inlines, from)
		classify(block.Blocks, from)
		for item := range block.Items {
			classify(block.Items[item].Blocks, from)
		}
		for cell := range block.Head {
			classifyInlines(block.Head[cell], from)
		}
		for row := range block.Rows {
			for cell := range block.Rows[row] {
				classifyInlines(block.Rows[row][cell], from)
			}
		}
	}
}

func classifyInlines(inlines []markdown.Inline, from string) {
	for index := range inlines {
		inline := &inlines[index]
		classifyInlines(inline.Inlines, from)
		if inline.Type != "link" || inline.Target != markdown.TargetUnresolved {
			continue
		}
		if id, ok := DocumentID(from, inline.Href); ok {
			inline.Target = markdown.TargetDocument
			inline.ID = id
		}
	}
}

// DocumentID resolves one href against the directory of the document it was
// written in, and reports the id of the document it names.
//
// It answers false for anything this interface does not serve as a document:
// an absolute path, a scheme, a file that is not Markdown, a name that climbs
// out of the checkout, and a name beneath a directory the route refuses. The
// admissibility test is the route's own, so a link is offered only when the
// route would answer it, and a link the route would refuse is text.
func DocumentID(from, href string) (string, bool) {
	target, _, _ := strings.Cut(href, "#")
	target, _, _ = strings.Cut(target, "?")
	if target == "" || strings.HasPrefix(target, "/") || hasScheme(target) {
		return "", false
	}
	if decoded, err := url.PathUnescape(target); err == nil {
		target = decoded
	}
	resolved := path.Join(path.Dir(from), target)
	if !admissibleID(resolved) {
		return "", false
	}
	return resolved, true
}

// hasScheme reports whether a href begins with a URL scheme, which is the one
// shape a relative path can never have: a colon before the first slash.
func hasScheme(href string) bool {
	for index, character := range href {
		switch {
		case character == ':':
			return index > 0
		case character == '/' || character == '.' || character == '#' || character == '?':
			return false
		case isSchemeCharacter(character, index):
			continue
		default:
			return false
		}
	}
	return false
}

func isSchemeCharacter(character rune, index int) bool {
	letter := (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z')
	if index == 0 {
		return letter
	}
	digit := character >= '0' && character <= '9'
	return letter || digit || character == '+' || character == '-'
}
