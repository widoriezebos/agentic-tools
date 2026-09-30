package textui

import (
	"strings"
	"unicode/utf8"
)

// State is one word of the fixed state vocabulary (§3), with its symbol
// and colour.
type State int

const (
	Running State = iota // ● green: running, live, collecting, reachable
	Stopped              // ○ dim: stopped
	Idle                 // ○ dim: idle, none, nothing underway
	Alert                // ! yellow: attention, stale, needs a person
	Failed               // ✗ red: failed, refused, red
	Done                 // ✓ green: done, landed, confirmed
	Unknown              // ? yellow: the engine could not read it
	Live                 // ● green: a live grant, informational
)

var stateGlyphs = map[State]struct{ unicode, ascii, word, color string }{
	Running: {"●", "*", "running", colorGreen},
	Stopped: {"○", "-", "stopped", colorDim},
	Idle:    {"○", "-", "idle", colorDim},
	Alert:   {"!", "!", "attention", colorYellow},
	Failed:  {"✗", "x", "failed", colorRed},
	Done:    {"✓", "+", "done", colorGreen},
	Unknown: {"?", "?", "unknown", colorYellow},
	Live:    {"●", "*", "live", colorGreen},
}

const (
	colorBold   = "1"
	colorDim    = "2"
	colorRed    = "31"
	colorGreen  = "32"
	colorYellow = "33"
)

type spanKind int

const (
	kindText   spanKind = iota
	kindStatus          // symbol and word, coloured
	kindMarked          // coloured symbol, then the text
	kindSymbol          // the coloured symbol alone
)

// Span is a run of text with one style. Colour is applied only on a
// terminal and only to spans that carry their words (P8).
type Span struct {
	text    string
	style   string // "", colorBold or colorDim for text spans
	kind    spanKind
	state   State
	nobreak bool
}

// Plain is unstyled text.
func Plain(s string) Span { return Span{text: s} }

// Dim is secondary text.
func Dim(s string) Span { return Span{text: s, style: colorDim} }

// Bold is emphasised text.
func Bold(s string) Span { return Span{text: s, style: colorBold} }

// Num is a number with its thousands grouped.
func Num(n int64) Span { return Span{text: Number(n)} }

// Status is a state's symbol and word: ● running.
func Status(s State) Span { return Span{kind: kindStatus, state: s, nobreak: true} }

// Marked is a state's symbol before text that says the same: ○ landing
// nothing underway.
func Marked(s State, text string) Span { return Span{kind: kindMarked, state: s, text: text} }

// Symbol is a state's symbol alone, for a row whose words say the state.
func Symbol(s State) Span { return Span{kind: kindSymbol, state: s, nobreak: true} }

// Text is the span as a monochrome Unicode reading shows it.
func (s Span) Text() string { return s.plain(false) }

func (s Span) glyph(ascii bool) string {
	glyph := stateGlyphs[s.state]
	if ascii {
		return glyph.ascii
	}
	return glyph.unicode
}

func (s Span) plain(ascii bool) string {
	switch s.kind {
	case kindStatus:
		return s.glyph(ascii) + " " + stateGlyphs[s.state].word
	case kindMarked:
		return s.glyph(ascii) + " " + s.text
	case kindSymbol:
		return s.glyph(ascii)
	}
	return s.text
}

func paint(code, text string) string { return "\x1b[" + code + "m" + text + "\x1b[0m" }

func (s Span) render(env Env) string {
	if !env.Color {
		return s.plain(env.ASCII)
	}
	color := stateGlyphs[s.state].color
	switch s.kind {
	case kindStatus:
		return paint(color, s.plain(env.ASCII))
	case kindMarked:
		return paint(color, s.glyph(env.ASCII)) + " " + s.text
	case kindSymbol:
		return paint(color, s.glyph(env.ASCII))
	}
	if s.style == "" || s.text == "" {
		return s.text
	}
	return paint(s.style, s.text)
}

func width(s string) int { return utf8.RuneCountInString(s) }

// Hint is the one command that does what needs doing (P13): the command,
// shell-quoted and never cut, and a short reason. A hint with no command
// is the reason alone.
type Hint struct {
	Argv   []string
	Reason string
}

func (h Hint) empty() bool { return len(h.Argv) == 0 && h.Reason == "" }

func (h Hint) spans(env Env) []Span {
	arrow := "→ "
	if env.ASCII {
		arrow = "-> "
	}
	spans := []Span{Dim(arrow)}
	if len(h.Argv) == 0 {
		return append(spans, Plain(h.Reason))
	}
	spans = append(spans, Span{text: Command(h.Argv), nobreak: true})
	if h.Reason != "" {
		spans = append(spans, Dim("  "+h.Reason))
	}
	return spans
}

// Attention is one standing condition before the headline (P12): Alert for
// one that needs a person, Live or Running for an informational one. Detail
// lines show only with --verbose.
type Attention struct {
	State  State
	Text   string
	Hint   Hint
	Detail []string
}

// KV is one key/value row.
type KV struct {
	Key   string
	Value []Span
}

// line is one logical output line: a prefix that never breaks and a body
// that wraps with a hanging indent of hang columns.
type line struct {
	prefix []Span
	body   []Span
	hang   int
	nowrap bool
}

func spaces(n int) Span { return Plain(strings.Repeat(" ", max(n, 0))) }

type block interface {
	lines(p *Page) []line
	spaced() bool
}

// Page collects blocks and renders them by the design's rules.
type Page struct {
	env     Env
	legacy  bool
	banner  []Attention
	heads   []line
	refused bool
	blocks  []block
	hint    *Hint
}

// New is a page for a converted view: every line fits the width.
func New(env Env) *Page {
	if env.Width <= 0 || env.Width > MaxWidth {
		env.Width = MaxWidth
	}
	return &Page{env: env}
}

// NewLegacy is the page of a verb not yet converted: its words print as
// they are and wrap only on a terminal, so a pipe reads them unchanged.
func NewLegacy(env Env) *Page {
	page := New(env)
	page.legacy = true
	return page
}

// Env is the page's environment.
func (p *Page) Env() Env { return p.env }

// Verbose reports whether callers add their verbose-only blocks.
func (p *Page) Verbose() bool { return p.env.Verbose }

// Banner adds standing conditions above the headline.
func (p *Page) Banner(items ...Attention) { p.banner = append(p.banner, items...) }

// Headline is the first line (P1): text · fact · fact; empty facts are
// left out.
func (p *Page) Headline(text string, facts ...string) {
	body := []Span{Bold(text)}
	separator := " · "
	if p.env.ASCII {
		separator = ", "
	}
	for _, fact := range facts {
		if fact != "" {
			body = append(body, Dim(separator), Plain(fact))
		}
	}
	p.heads = append(p.heads, line{body: body, hang: 2})
}

// Done is an act's headline: ✓ what happened.
func (p *Page) Done(text string) { p.Mark(Done, text) }

// Mark is a headline led by a state's symbol.
func (p *Page) Mark(state State, text string) {
	p.heads = append(p.heads, line{prefix: []Span{Symbol(state), Plain(" ")}, body: []Span{Bold(text)}, hang: 2})
}

// Refusal is a refusal's two lines: ✗ line 1, and its hint indented under
// it. A later Hint is indented the same way.
func (p *Page) Refusal(line1 string, hint Hint) {
	p.refused = true
	p.heads = append(p.heads, line{prefix: []Span{Symbol(Failed), Plain(" ")}, body: []Span{Bold(line1)}, hang: 2})
	if !hint.empty() {
		p.heads = append(p.heads, p.hintLines(hint, 2)...)
	}
}

// Hint sets the one hint, always rendered last; a later call replaces it.
func (p *Page) Hint(h Hint) {
	if h.empty() {
		return
	}
	p.hint = &h
}

// Facts is a headingless key/value block under the headline.
func (p *Page) Facts(rows ...KV) {
	section := &Section{indent: 2}
	for _, row := range rows {
		section.KV(row.Key, row.Value...)
	}
	p.blocks = append(p.blocks, section)
}

// Section adds a section: a heading of one to four words and a dim aside.
// A section without a title prints its rows at column 0.
func (p *Page) Section(title, aside string) *Section {
	section := &Section{title: title, aside: aside}
	if title != "" {
		section.indent = 2
	}
	p.blocks = append(p.blocks, section)
	return section
}

type legacyBlock []string

func (b legacyBlock) spaced() bool { return false }

func (b legacyBlock) lines(p *Page) []line {
	var out []line
	for _, text := range b {
		lead := len(text) - len(strings.TrimLeft(text, " "))
		out = append(out, line{prefix: []Span{spaces(lead)}, body: []Span{Plain(strings.TrimLeft(text, " "))}, hang: lead + 2})
	}
	return out
}

// Legacy adds lines exactly as a verb printed them before it was
// converted; they follow the line above without a blank line.
func (p *Page) Legacy(lines ...string) {
	if len(lines) > 0 {
		p.blocks = append(p.blocks, legacyBlock(lines))
	}
}

// hintLines are a hint's lines: the arrow, the command and its reason on
// one line when they fit; otherwise the reason wraps under the command.
func (p *Page) hintLines(h Hint, indent int) []line {
	spans := h.spans(p.env)
	whole := line{prefix: append([]Span{spaces(indent)}, spans[0]), body: spans[1:], hang: indent + 2}
	total := indent
	for _, span := range spans {
		total += width(span.plain(p.env.ASCII))
	}
	if len(h.Argv) == 0 || h.Reason == "" || total <= p.env.Width || (p.legacy && !p.env.TTY) {
		return []line{whole}
	}
	return []line{{prefix: append([]Span{spaces(indent)}, spans[0], spans[1]), nowrap: true},
		{prefix: []Span{spaces(indent + 2)}, body: []Span{Dim(h.Reason)}, hang: indent + 2}}
}

// Lines renders the page, one string per output line.
func (p *Page) Lines() []string {
	var out []string
	emit := func(l line) { out = append(out, p.render(l)...) }
	for _, item := range p.banner {
		emit(line{prefix: []Span{Symbol(item.State), Plain(" ")}, body: []Span{Plain(item.Text)}, hang: 2})
		if !item.Hint.empty() {
			for _, l := range p.hintLines(item.Hint, 2) {
				emit(l)
			}
		}
		if p.env.Verbose {
			for _, detail := range item.Detail {
				emit(line{prefix: []Span{spaces(4)}, body: []Span{Dim(detail)}, hang: 6})
			}
		}
	}
	if len(out) > 0 && (len(p.heads) > 0 || len(p.blocks) > 0 || p.hint != nil) {
		out = append(out, "")
	}
	for _, head := range p.heads {
		emit(head)
	}
	spacedBlocks := false
	for _, b := range p.blocks {
		lines := b.lines(p)
		if len(lines) == 0 {
			continue
		}
		if b.spaced() {
			spacedBlocks = true
			if len(out) > 0 && out[len(out)-1] != "" {
				out = append(out, "")
			}
		}
		for _, l := range lines {
			emit(l)
		}
	}
	if p.hint != nil {
		indent := 0
		if p.refused {
			indent = 2
		}
		if spacedBlocks && !p.refused {
			out = append(out, "")
		}
		for _, l := range p.hintLines(*p.hint, indent) {
			emit(l)
		}
	}
	return out
}

// String is the page as printed: its lines, each ended by a newline.
func (p *Page) String() string {
	lines := p.Lines()
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n") + "\n"
}

// render lays one logical line out: as it is when it fits, when it must
// not wrap, or on a legacy page off a terminal; otherwise its body wraps at
// word boundaries under a hanging indent, a lone long token on its own line.
func (p *Page) render(l line) []string {
	plainWidth := 0
	for _, span := range append(append([]Span{}, l.prefix...), l.body...) {
		plainWidth += width(span.plain(p.env.ASCII))
	}
	if l.nowrap || plainWidth <= p.env.Width || (p.legacy && !p.env.TTY) {
		return []string{p.join(l.prefix, l.body)}
	}
	var lines []string
	prefix := l.prefix
	used := 0
	for _, span := range prefix {
		used += width(span.plain(p.env.ASCII))
	}
	var current []Span
	for _, word := range words(l.body) {
		size := 0
		for _, span := range word {
			size += width(span.plain(p.env.ASCII))
		}
		if len(current) > 0 && used+1+size > p.env.Width {
			lines = append(lines, p.join(prefix, current))
			prefix, current, used = []Span{spaces(l.hang)}, nil, l.hang
		}
		if len(current) > 0 {
			current = append(current, Plain(" "))
			used++
		}
		current = append(current, word...)
		used += size
	}
	return append(lines, p.join(prefix, current))
}

// words splits spans at spaces into words; a word may join several spans
// that touch, and an unbreakable span is one word.
func words(spans []Span) [][]Span {
	var out [][]Span
	var current []Span
	flush := func() {
		if len(current) > 0 {
			out = append(out, current)
			current = nil
		}
	}
	for _, span := range spans {
		if span.nobreak || span.kind != kindText {
			current = append(current, span)
			continue
		}
		for index, part := range strings.Split(span.text, " ") {
			if index > 0 {
				flush()
			}
			if part != "" {
				piece := span
				piece.text = part
				current = append(current, piece)
			}
		}
	}
	flush()
	return out
}

func (p *Page) join(prefix, body []Span) string {
	var out strings.Builder
	for _, span := range append(append([]Span{}, prefix...), body...) {
		out.WriteString(span.render(p.env))
	}
	return strings.TrimRight(out.String(), " ")
}

// Section is a heading and its rows.
type Section struct {
	title, aside string
	indent       int
	rows         []sectionRow
}

type sectionRow interface {
	lines(p *Page, s *Section, keyWidth int) []line
}

type kvRow KV

func (r kvRow) lines(p *Page, s *Section, keyWidth int) []line {
	return []line{kvLine(s.indent, keyWidth, KV(r))}
}

func kvLine(indent, keyWidth int, kv KV) line {
	prefix := []Span{spaces(indent), Plain(kv.Key), spaces(keyWidth - width(kv.Key) + 3)}
	return line{prefix: prefix, body: kv.Value, hang: indent + keyWidth + 3}
}

type textRow string

func (r textRow) lines(p *Page, s *Section, _ int) []line {
	return []line{{prefix: []Span{spaces(s.indent)}, body: []Span{Plain(string(r))}, hang: s.indent + 2}}
}

func (s *Section) spaced() bool { return true }

func (s *Section) lines(p *Page) []line {
	keyWidth := 0
	for _, row := range s.rows {
		if kv, ok := row.(kvRow); ok {
			keyWidth = max(keyWidth, width(kv.Key))
		}
	}
	var body []line
	for _, row := range s.rows {
		body = append(body, row.lines(p, s, keyWidth)...)
	}
	if len(body) == 0 {
		return nil
	}
	if s.title == "" {
		return body
	}
	heading := line{body: []Span{Bold(s.title)}, hang: 2}
	if s.aside != "" {
		heading.body = append(heading.body, Plain("  "), Dim(s.aside))
	}
	return append([]line{heading}, body...)
}

// KV adds a key/value row; keys align across the section.
func (s *Section) KV(key string, value ...Span) {
	s.rows = append(s.rows, kvRow{Key: key, Value: value})
}

// Text adds wrapped prose with a hanging indent.
func (s *Section) Text(text string) { s.rows = append(s.rows, textRow(text)) }

// Item adds a card: its symbol and title, then key/value rows under it.
func (s *Section) Item(state State, title string) *Item {
	item := &Item{state: state, title: title}
	s.rows = append(s.rows, item)
	return item
}

// Item is one card of a section.
type Item struct {
	state State
	title string
	rows  []KV
}

// KV adds a row under the card; keys align within the card.
func (i *Item) KV(key string, value ...Span) { i.rows = append(i.rows, KV{Key: key, Value: value}) }

func (i *Item) lines(p *Page, s *Section, _ int) []line {
	out := []line{{prefix: []Span{spaces(s.indent), Symbol(i.state), Plain(" ")}, body: []Span{Plain(i.title)}, hang: s.indent + 2}}
	keyWidth := 0
	for _, row := range i.rows {
		keyWidth = max(keyWidth, width(row.Key))
	}
	for _, row := range i.rows {
		out = append(out, kvLine(s.indent+2, keyWidth, row))
	}
	return out
}

// Column describes one table column.
type Column struct {
	Title string // "" for every column: no header row
	Right bool   // numbers
	Flex  bool   // the one column that may be cut or moved (P3)
}

// Table adds a table of aligned columns.
func (s *Section) Table(cols ...Column) *Table {
	table := &Table{cols: cols}
	s.rows = append(s.rows, table)
	return table
}

// Table is rows of aligned cells, one span per cell.
type Table struct {
	cols []Column
	rows [][]Span
}

// Row adds one row; missing cells are empty.
func (t *Table) Row(cells ...Span) { t.rows = append(t.rows, cells) }

const tableGap = 3

func (t *Table) lines(p *Page, s *Section, _ int) []line {
	if len(t.rows) == 0 {
		return nil
	}
	ascii := p.env.ASCII
	cell := func(row []Span, col int) Span {
		if col < len(row) {
			return row[col]
		}
		return Plain("")
	}
	header := false
	for _, col := range t.cols {
		header = header || col.Title != ""
	}
	widths := make([]int, len(t.cols))
	for index, col := range t.cols {
		if header {
			widths[index] = width(col.Title)
		}
		for _, row := range t.rows {
			widths[index] = max(widths[index], width(cell(row, index).plain(ascii)))
		}
	}
	flex, moved := -1, false
	for index, col := range t.cols {
		if col.Flex {
			flex = index
		}
	}
	if flex >= 0 {
		fixed := s.indent
		for index := range t.cols {
			if index != flex {
				fixed += widths[index] + tableGap
			}
		}
		room := p.env.Width - fixed
		if widths[flex] > room {
			if room >= 20 {
				widths[flex] = room
			} else {
				moved = true
			}
		}
	}
	format := func(row []Span) line {
		last := -1
		for index := range t.cols {
			if moved && index == flex {
				continue
			}
			if width(cell(row, index).plain(ascii)) > 0 {
				last = index
			}
		}
		spans := []Span{spaces(s.indent)}
		first := true
		for index, col := range t.cols {
			if index > last {
				break
			}
			if moved && index == flex {
				continue
			}
			if !first {
				spans = append(spans, spaces(tableGap))
			}
			first = false
			span := cell(row, index)
			if index == flex && !moved {
				span = cut(span, widths[index], ascii)
			}
			pad := widths[index] - width(span.plain(ascii))
			switch {
			case col.Right:
				spans = append(spans, spaces(pad), span)
			case index == last:
				spans = append(spans, span)
			default:
				spans = append(spans, span, spaces(pad))
			}
		}
		return line{prefix: spans, nowrap: true}
	}
	var out []line
	if header {
		titles := make([]Span, len(t.cols))
		for index, col := range t.cols {
			titles[index] = Dim(strings.ToUpper(col.Title))
		}
		out = append(out, format(titles))
	}
	for _, row := range t.rows {
		out = append(out, format(row))
		if moved {
			if text := cell(row, flex); width(text.plain(ascii)) > 0 {
				text.style = colorDim
				out = append(out, line{prefix: []Span{spaces(s.indent + 2)}, body: []Span{text}, hang: s.indent + 2})
			}
		}
	}
	return out
}

// cut shortens a text span to room columns, ending it with an ellipsis.
func cut(span Span, room int, ascii bool) Span {
	text := span.plain(ascii)
	if width(text) <= room || span.kind != kindText {
		return span
	}
	ellipsis := "…"
	if ascii {
		ellipsis = "..."
	}
	runes := []rune(text)
	keep := max(room-width(ellipsis), 0)
	span.text = strings.TrimRight(string(runes[:keep]), " ") + ellipsis
	return span
}
