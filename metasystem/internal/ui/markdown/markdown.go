// Package markdown parses Markdown into a typed tree the browser renders as
// elements. No HTML string is produced anywhere: goldmark's parser and its AST
// are used, its renderer is never called, and raw HTML in the source becomes a
// node whose text is the source, shown rather than run.
//
// The schema's keys are fixed, because the brain's tools read the same tree
// later. A construct this schema does not model is flattened to what it is made
// of rather than dropped.
package markdown

import (
	"strconv"
	"strings"
	"unicode"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
)

// The link targets this package can decide from the href alone. A relative
// Markdown path is left `TargetUnresolved` here and raised to a document
// reference by the reader that knows the checkout and the document's own id.
const (
	TargetExternal   = "external"
	TargetFragment   = "fragment"
	TargetDocument   = "document"
	TargetUnresolved = "unresolved"
)

// Document is one parsed file: the outline, and the blocks in source order.
type Document struct {
	Headings []Heading `json:"headings"`
	Blocks   []Block   `json:"blocks"`
}

// Heading is one outline row. The id is the slug the browser scrolls to, so a
// cross-document `#anchor` resolves against the same names GitHub uses.
type Heading struct {
	Level int    `json:"level"`
	ID    string `json:"id"`
	Text  string `json:"text"`
}

// Block is one block-level node. The type names which of the remaining fields
// carry anything; a reader switches on it and reads nothing else.
type Block struct {
	Type    string   `json:"type"`
	Level   int      `json:"level,omitempty"`
	ID      string   `json:"id,omitempty"`
	Lang    string   `json:"lang,omitempty"`
	Text    string   `json:"text,omitempty"`
	Ordered bool     `json:"ordered,omitempty"`
	Start   int      `json:"start,omitempty"`
	Align   []string `json:"align,omitempty"`
	Head    []Cell   `json:"head,omitempty"`
	Rows    [][]Cell `json:"rows,omitempty"`
	Inlines []Inline `json:"inlines,omitempty"`
	Blocks  []Block  `json:"blocks,omitempty"`
	Items   []Item   `json:"items,omitempty"`
}

// Cell is one table cell: inlines, with no block structure of its own.
type Cell []Inline

// Item is one list item. Checked is null unless the item is a task item, so a
// plain list is not rendered as a list of unticked boxes.
type Item struct {
	Checked *bool   `json:"checked"`
	Blocks  []Block `json:"blocks"`
}

// Inline is one inline node. Href is carried verbatim, never rewritten, and
// Target says what the renderer is allowed to do with it.
type Inline struct {
	Type    string   `json:"type"`
	Text    string   `json:"text,omitempty"`
	Href    string   `json:"href,omitempty"`
	Target  string   `json:"target,omitempty"`
	ID      string   `json:"id,omitempty"`
	Src     string   `json:"src,omitempty"`
	Alt     string   `json:"alt,omitempty"`
	Inlines []Inline `json:"inlines,omitempty"`
}

// parser is built once: goldmark's parser with the three extensions this
// schema models. The renderer the constructor also builds is never called, and
// WithUnsafe is never set, so no API here can carry HTML.
var parser = goldmark.New(goldmark.WithExtensions(
	extension.Table,
	extension.Strikethrough,
	extension.TaskList,
)).Parser()

// Parse reads Markdown source into the tree. CRLF is accepted: the source is
// normalised first, so every segment this package takes out of it is free of
// the carriage returns a Windows-edited document carries.
func Parse(source []byte) Document {
	normalised := normalise(source)
	root := parser.Parse(text.NewReader(normalised))
	state := &walk{source: normalised, slugs: map[string]int{}, headings: []Heading{}}
	document := Document{Headings: []Heading{}, Blocks: []Block{}}
	if blocks := state.blocks(root); blocks != nil {
		document.Blocks = blocks
	}
	document.Headings = state.headings
	return document
}

// normalise turns CRLF into LF and leaves everything else alone, so byte
// offsets in the parsed tree index the bytes this package reads back.
func normalise(source []byte) []byte {
	if !strings.Contains(string(source), "\r\n") {
		return source
	}
	return []byte(strings.ReplaceAll(string(source), "\r\n", "\n"))
}

// walk carries what the traversal accumulates: the outline, and the slug
// counter that makes a repeated heading's id unique.
type walk struct {
	source   []byte
	slugs    map[string]int
	headings []Heading
}

func (w *walk) blocks(parent ast.Node) []Block {
	var out []Block
	for node := parent.FirstChild(); node != nil; node = node.NextSibling() {
		out = append(out, w.block(node)...)
	}
	return out
}

// block answers the blocks one node becomes: usually one, none for a node this
// schema does not model and that holds nothing, and its children flattened for
// a container this schema has no name for.
func (w *walk) block(node ast.Node) []Block {
	switch typed := node.(type) {
	case *ast.Heading:
		heading := Block{Type: "heading", Level: typed.Level, Inlines: w.inlines(typed)}
		plain := plainText(heading.Inlines)
		heading.ID = w.slug(plain)
		w.headings = append(w.headings, Heading{Level: typed.Level, ID: heading.ID, Text: plain})
		return []Block{heading}
	case *ast.Paragraph:
		return []Block{{Type: "paragraph", Inlines: w.inlines(typed)}}
	case *ast.TextBlock:
		return []Block{{Type: "paragraph", Inlines: w.inlines(typed)}}
	case *ast.FencedCodeBlock:
		return []Block{{Type: "code", Lang: string(typed.Language(w.source)), Text: w.lines(typed)}}
	case *ast.CodeBlock:
		return []Block{{Type: "code", Text: w.lines(typed)}}
	case *ast.Blockquote:
		return []Block{{Type: "quote", Blocks: w.blocks(typed)}}
	case *ast.List:
		return []Block{w.list(typed)}
	case *ast.ThematicBreak:
		return []Block{{Type: "rule"}}
	case *ast.HTMLBlock:
		return []Block{{Type: "html", Text: w.htmlBlock(typed)}}
	case *east.Table:
		return []Block{w.table(typed)}
	default:
		if node.Type() == ast.TypeInline {
			return nil
		}
		return w.blocks(node)
	}
}

func (w *walk) list(node *ast.List) Block {
	block := Block{Type: "list", Ordered: node.IsOrdered(), Items: []Item{}}
	if node.IsOrdered() {
		block.Start = node.Start
	}
	for item := node.FirstChild(); item != nil; item = item.NextSibling() {
		block.Items = append(block.Items, Item{Checked: checkedState(item), Blocks: w.blocks(item)})
	}
	return block
}

// checkedState reports a task item's box, or nil for an ordinary item. The box
// is goldmark's first inline inside the item's first block, and it is read here
// rather than rendered as an inline, because it belongs to the item.
func checkedState(item ast.Node) *bool {
	first := item.FirstChild()
	if first == nil {
		return nil
	}
	for inline := first.FirstChild(); inline != nil; inline = inline.NextSibling() {
		if box, ok := inline.(*east.TaskCheckBox); ok {
			checked := box.IsChecked
			return &checked
		}
	}
	return nil
}

func (w *walk) table(node *east.Table) Block {
	block := Block{Type: "table", Align: []string{}, Rows: [][]Cell{}}
	for _, alignment := range node.Alignments {
		block.Align = append(block.Align, alignmentName(alignment))
	}
	for row := node.FirstChild(); row != nil; row = row.NextSibling() {
		cells := w.cells(row)
		if _, header := row.(*east.TableHeader); header {
			block.Head = cells
			continue
		}
		block.Rows = append(block.Rows, cells)
	}
	return block
}

func (w *walk) cells(row ast.Node) []Cell {
	cells := []Cell{}
	for cell := row.FirstChild(); cell != nil; cell = cell.NextSibling() {
		inlines := w.inlines(cell)
		if inlines == nil {
			inlines = []Inline{}
		}
		cells = append(cells, Cell(inlines))
	}
	return cells
}

func alignmentName(alignment east.Alignment) string {
	switch alignment {
	case east.AlignLeft:
		return "left"
	case east.AlignRight:
		return "right"
	case east.AlignCenter:
		return "center"
	default:
		return "none"
	}
}

func (w *walk) inlines(parent ast.Node) []Inline {
	var out []Inline
	for node := parent.FirstChild(); node != nil; node = node.NextSibling() {
		out = append(out, w.inline(node)...)
	}
	return out
}

func (w *walk) inline(node ast.Node) []Inline {
	switch typed := node.(type) {
	case *ast.Text:
		return w.text(typed)
	case *ast.String:
		return []Inline{{Type: "text", Text: string(typed.Value)}}
	case *ast.CodeSpan:
		return []Inline{{Type: "code", Text: rawText(typed, w.source)}}
	case *ast.Emphasis:
		kind := "emph"
		if typed.Level >= 2 {
			kind = "strong"
		}
		return []Inline{{Type: kind, Inlines: w.inlines(typed)}}
	case *east.Strikethrough:
		return []Inline{{Type: "strike", Inlines: w.inlines(typed)}}
	case *east.TaskCheckBox:
		// The box is the item's, read by checkedState; as an inline it would be
		// a second checkbox in the middle of the item's first line.
		return nil
	case *ast.Link:
		href := string(typed.Destination)
		return []Inline{{Type: "link", Href: href, Target: TargetFor(href), Inlines: w.inlines(typed)}}
	case *ast.AutoLink:
		href := string(typed.URL(w.source))
		label := string(typed.Label(w.source))
		return []Inline{{
			Type: "link", Href: href, Target: TargetFor(href),
			Inlines: []Inline{{Type: "text", Text: label}},
		}}
	case *ast.Image:
		return []Inline{{Type: "image", Src: string(typed.Destination), Alt: rawText(typed, w.source)}}
	case *ast.RawHTML:
		return []Inline{{Type: "html", Text: segmentsText(typed.Segments, w.source)}}
	default:
		return w.inlines(node)
	}
}

// text answers one text node: its characters, and a break node where the
// source had a hard line break. A soft break is a newline in the text, which
// the browser collapses exactly as the source intended.
func (w *walk) text(node *ast.Text) []Inline {
	value := string(node.Segment.Value(w.source))
	out := []Inline{}
	if node.HardLineBreak() {
		if value != "" {
			out = append(out, Inline{Type: "text", Text: value})
		}
		return append(out, Inline{Type: "break"})
	}
	if node.SoftLineBreak() {
		value += "\n"
	}
	if value == "" {
		return nil
	}
	return append(out, Inline{Type: "text", Text: value})
}

func (w *walk) lines(node ast.Node) string {
	return segmentsText(node.Lines(), w.source)
}

// htmlBlock answers the block's source, closure line included, so that what
// the browser shows in the code block is what the file says.
func (w *walk) htmlBlock(node *ast.HTMLBlock) string {
	out := w.lines(node)
	if node.HasClosure() {
		out += string(node.ClosureLine.Value(w.source))
	}
	return out
}

func segmentsText(segments *text.Segments, source []byte) string {
	if segments == nil {
		return ""
	}
	var builder strings.Builder
	for index := range segments.Len() {
		segment := segments.At(index)
		builder.Write(segment.Value(source))
	}
	return builder.String()
}

// rawText concatenates the source text beneath a node, which is what a code
// span and an image's alternative text are made of.
func rawText(node ast.Node, source []byte) string {
	var builder strings.Builder
	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		switch typed := child.(type) {
		case *ast.Text:
			builder.Write(typed.Segment.Value(source))
		case *ast.String:
			builder.Write(typed.Value)
		default:
			builder.WriteString(rawText(child, source))
		}
	}
	return builder.String()
}

// TargetFor classifies a link from its href alone. Only the two spellings a
// browser may open in a new tab are external; everything else that is not a
// fragment is unresolved until a reader that knows the checkout says otherwise,
// so `javascript:`, `data:`, `mailto:` and `file:` never become anchors.
func TargetFor(href string) string {
	switch {
	case strings.HasPrefix(href, "#"):
		return TargetFragment
	case strings.HasPrefix(href, "http://"), strings.HasPrefix(href, "https://"):
		return TargetExternal
	default:
		return TargetUnresolved
	}
}

// plainText is what a heading reads as with every mark removed: the outline's
// text, and the string the slug is made of.
func plainText(inlines []Inline) string {
	var builder strings.Builder
	for _, inline := range inlines {
		switch inline.Type {
		case "text", "code", "html":
			builder.WriteString(inline.Text)
		case "image":
			builder.WriteString(inline.Alt)
		case "break":
			builder.WriteString(" ")
		default:
			builder.WriteString(plainText(inline.Inlines))
		}
	}
	return builder.String()
}

// slug is the GitHub spelling of a heading id: lower case, punctuation
// removed, spaces to hyphens, and a counter appended when a document repeats a
// heading, so every id in one document is distinct.
func (w *walk) slug(heading string) string {
	base := slugOf(heading)
	seen := w.slugs[base]
	w.slugs[base] = seen + 1
	if seen == 0 {
		return base
	}
	return base + "-" + strconv.Itoa(seen)
}

func slugOf(heading string) string {
	var builder strings.Builder
	for _, character := range strings.ToLower(strings.TrimSpace(heading)) {
		switch {
		case character == ' ' || character == '\t' || character == '\n':
			builder.WriteRune('-')
		case character == '-' || character == '_':
			builder.WriteRune(character)
		case unicode.IsLetter(character) || unicode.IsDigit(character):
			builder.WriteRune(character)
		}
	}
	return builder.String()
}
