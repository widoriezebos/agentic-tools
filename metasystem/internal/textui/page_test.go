package textui

import (
	"strings"
	"testing"
	"time"
)

func expectPage(t *testing.T, page *Page, want string) {
	t.Helper()
	want = strings.TrimPrefix(want, "\n")
	if got := page.String(); got != want {
		t.Errorf("page:\n%s\nwant:\n%s", got, want)
	}
}

// The status mockup (design 6.1) built through the page: a banner, a
// headline with facts, and two sections of aligned rows.
func TestPageRendersTheStatusShape(t *testing.T) {
	t.Parallel()
	page := New(fixedEnv())
	page.Banner(
		Attention{State: Alert, Text: "wido has the helm since Mon 12:39: coordinating the verb and machinery batches",
			Hint: Hint{Argv: []string{"metasystem", "helm", "return"}, Reason: "gives the seat back to the machinery"}},
		Attention{State: Live, Text: "wido's grant is live: everything, until tomorrow 10:56 (23h58m left)"},
	)
	page.Headline("m1e is running", "5 helpers", "no jobs", "")
	seats := page.Section("Seats on this host", "bridge live").Table(Column{}, Column{Flex: true})
	seats.Row(Marked(Idle, "landing"), Plain("nothing underway"))
	seats.Row(Marked(Unknown, "m1e"), Plain("switch-on-trial unknown: not claimed since 10:58"))
	lane := page.Section("Landing lane", "").Table(Column{}, Column{Flex: true})
	lane.Row(Status(Running), Plain("batch 4gr18nm8t3nyev9sssda9jgtsq · 1 change (533209e6d from m1e)"))
	expectPage(t, page, `
! wido has the helm since Mon 12:39: coordinating the verb and machinery batches
  → metasystem helm return  gives the seat back to the machinery
● wido's grant is live: everything, until tomorrow 10:56 (23h58m left)

m1e is running · 5 helpers · no jobs

Seats on this host  bridge live
  ○ landing   nothing underway
  ? m1e       switch-on-trial unknown: not claimed since 10:58

Landing lane
  ● running   batch 4gr18nm8t3nyev9sssda9jgtsq · 1 change (533209e6d from m1e)
`)
	if page.Verbose() {
		t.Error("a page is verbose only when its Env says so")
	}
}

// The grant list mockup (design 6.6): a card whose id is never cut.
func TestPageRendersACard(t *testing.T) {
	t.Parallel()
	page := New(fixedEnv())
	page.Headline("1 grant, live")
	card := page.Section("", "").Item(Live, "everything · by wido · until tomorrow 10:56 (23h58m left)")
	card.KV("for", Plain("the main session of m1e (~/GitHub/m1e/metasystem)"))
	card.KV("id", Plain("YY6RQ7765HX224V3NT4TN67JFR-m1e-b6a4eb0a"))
	expectPage(t, page, `
1 grant, live

● everything · by wido · until tomorrow 10:56 (23h58m left)
  for   the main session of m1e (~/GitHub/m1e/metasystem)
  id    YY6RQ7765HX224V3NT4TN67JFR-m1e-b6a4eb0a
`)
}

func TestFactsSectionsTextAndTheHintAreSpacedAndAligned(t *testing.T) {
	t.Parallel()
	env := fixedEnv()
	env.Width = 40
	page := New(env)
	page.Headline("No jobs running in m1e")
	page.Facts(KV{Key: "checkout", Value: []Span{Plain("~/GitHub/m1e")}}, KV{Key: "helm", Value: []Span{Plain("taken"), Dim(" by wido")}})
	section := page.Section("Intent", "")
	section.Text("design and build the most intuitive, powerful, forgiving verbs possible")
	section.KV("a", Plain("1"))
	section.KV("longer", Num(2400))
	page.Hint(Hint{Argv: []string{"metasystem", "work", "status", "--all"}, Reason: "also lists ended jobs"})
	expectPage(t, page, `
No jobs running in m1e

  checkout   ~/GitHub/m1e
  helm       taken by wido

Intent
  design and build the most intuitive,
    powerful, forgiving verbs possible
  a        1
  longer   2,400

→ metasystem work status --all
  also lists ended jobs
`)
}

func TestAHintAfterOnlyAHeadlineHasNoBlankLine(t *testing.T) {
	t.Parallel()
	page := New(fixedEnv())
	page.Headline("No jobs running in m1e")
	page.Hint(Hint{Argv: []string{"metasystem", "work", "status", "--all"}, Reason: "also lists ended jobs"})
	page.Hint(Hint{Argv: []string{"metasystem", "status"}})
	expectPage(t, page, `
No jobs running in m1e
→ metasystem status
`)
	empty := New(fixedEnv())
	if empty.String() != "" || len(empty.Lines()) != 0 {
		t.Errorf("an empty page = %q", empty.String())
	}
}

func TestARefusalCarriesItsHintIndented(t *testing.T) {
	t.Parallel()
	page := New(fixedEnv())
	page.Refusal("Only you, at your enrolled terminal, can add a grant", Hint{Argv: []string{"metasystem", "system", "enroll", "--name", "wido"}, Reason: "here first"})
	expectPage(t, page, `
✗ Only you, at your enrolled terminal, can add a grant
  → metasystem system enroll --name wido  here first
`)
	words := New(fixedEnv())
	words.Refusal("status takes one goal at most", Hint{})
	words.Legacy("a second line the verb printed")
	words.Hint(Hint{Reason: "one goal's work is metasystem status G"})
	expectPage(t, words, `
✗ status takes one goal at most
a second line the verb printed
  → one goal's work is metasystem status G
`)
	done := New(fixedEnv())
	done.Done("granted everything until tomorrow 10:56")
	done.Mark(Running, "the batch is proving")
	expectPage(t, done, `
✓ granted everything until tomorrow 10:56
● the batch is proving
`)
}

// P3: the flexible column is cut with an ellipsis, or moves to a dim line of
// its own when fewer than 20 columns would be left; names are never cut.
func TestTablesCutOrMoveTheFlexibleColumnOnly(t *testing.T) {
	t.Parallel()
	env := fixedEnv()
	env.Width = 62
	page := New(env)
	table := page.Section("Claimed", "").Table(Column{}, Column{}, Column{Right: true}, Column{Flex: true})
	table.Row(Plain("1.1"), Plain("machinery-runs-unattended"), Num(3), Plain("Push 17 (58a943387) landed the batch lane; ledger tip"))
	table.Row(Plain("–"), Plain("switch-on-trial"), Num(12), Plain("short"))
	expectPage(t, page, `
Claimed
  1.1   machinery-runs-unattended    3   Push 17 (58a943387)…
  –     switch-on-trial             12   short
`)
	env.Width = 50
	narrow := New(env)
	moved := narrow.Section("Claimed", "").Table(Column{}, Column{}, Column{Right: true}, Column{Flex: true})
	moved.Row(Plain("1.1"), Plain("machinery-runs-unattended"), Num(3), Plain("Push 17 (58a943387) landed the batch lane; ledger tip"))
	moved.Row(Plain("–"), Plain("switch-on-trial"), Num(12), Plain(""))
	expectPage(t, narrow, `
Claimed
  1.1   machinery-runs-unattended    3
    Push 17 (58a943387) landed the batch lane;
    ledger tip
  –     switch-on-trial             12
`)
	for _, line := range narrow.Lines() {
		if n := width(line); n > 50 {
			t.Errorf("line of %d over 50: %q", n, line)
		}
	}
}

func TestTableHeadersAreDimUppercase(t *testing.T) {
	t.Parallel()
	page := New(fixedEnv())
	table := page.Section("", "").Table(Column{Title: "machine"}, Column{Title: "free", Right: true})
	table.Row(Marked(Running, "m1e"), Plain("231.4 GiB"))
	table.Row(Marked(Alert, "wr-m1"), Plain("73.3 GiB"))
	expectPage(t, page, `
MACHINE        FREE
● m1e     231.4 GiB
! wr-m1    73.3 GiB
`)
}

func TestColourOnlyWrapsSpansThatCarryTheirWords(t *testing.T) {
	t.Parallel()
	env := fixedEnv()
	env.Color = true
	page := New(env)
	page.Headline("m1e is running", "5 helpers")
	section := page.Section("Machinery", "since 10:53")
	section.KV("steward", Status(Running), Dim(" pid 39052"))
	section.Table(Column{}, Column{}).Row(Symbol(Failed), Plain("red"))
	got := page.String()
	for _, want := range []string{
		"\x1b[1mm1e is running\x1b[0m\x1b[2m · \x1b[0m5 helpers",
		"\x1b[1mMachinery\x1b[0m  \x1b[2msince 10:53\x1b[0m",
		"\x1b[32m● running\x1b[0m\x1b[2m pid 39052\x1b[0m",
		"\x1b[31m✗\x1b[0m   red",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("coloured page lacks %q:\n%q", want, got)
		}
	}
	plain := New(fixedEnv())
	plain.Headline("m1e is running")
	plain.Section("s", "").KV("k", Status(Failed))
	if strings.Contains(plain.String(), "\x1b") {
		t.Errorf("an uncoloured page carries an escape: %q", plain.String())
	}
}

func TestASCIISymbolsWhenTheLocaleCannotRenderThem(t *testing.T) {
	t.Parallel()
	env := fixedEnv()
	env.ASCII = true
	page := New(env)
	page.Banner(Attention{State: Alert, Text: "the helm is taken", Hint: Hint{Argv: []string{"metasystem", "helm", "return"}}})
	page.Headline("m1e is running", "5 helpers")
	table := page.Section("", "").Table(Column{}, Column{})
	for _, state := range []State{Running, Stopped, Idle, Alert, Failed, Done, Unknown, Live} {
		table.Row(Status(state), Plain("x"))
	}
	expectPage(t, page, `
! the helm is taken
  -> metasystem helm return

m1e is running, 5 helpers

* running     x
- stopped     x
- idle        x
! attention   x
x failed      x
+ done        x
? unknown     x
* live        x
`)
}

func TestBannerDetailsShowOnlyWhenVerbose(t *testing.T) {
	t.Parallel()
	attention := Attention{State: Alert, Text: "wido's terminal is not enrolled", Hint: Hint{Argv: []string{"metasystem", "system", "enroll", "--name", "wido"}, Reason: "run it there"},
		Detail: []string{"session leader login (34285@1790591981); the enrolled one is another"}}
	quiet := New(fixedEnv())
	quiet.Banner(attention)
	quiet.Headline("m1e is running")
	expectPage(t, quiet, `
! wido's terminal is not enrolled
  → metasystem system enroll --name wido  run it there

m1e is running
`)
	env := fixedEnv()
	env.Verbose = true
	loud := New(env)
	loud.Banner(attention)
	loud.Headline("m1e is running")
	if !loud.Verbose() {
		t.Fatal("a verbose Env makes a verbose page")
	}
	expectPage(t, loud, `
! wido's terminal is not enrolled
  → metasystem system enroll --name wido  run it there
    session leader login (34285@1790591981); the enrolled one is another

m1e is running
`)
}

// P9: prose wraps at the width with a hanging indent; a lone long token
// keeps its own line; legacy lines wrap only on a terminal.
func TestWidthIsHeldExceptForALoneToken(t *testing.T) {
	t.Parallel()
	env := fixedEnv()
	env.Width = 30
	long := "landing lane /Users/wido/LocalStorage/GitHub/agentic-tools-landing: owner running (pid 38928)"
	pipe := NewLegacy(env)
	pipe.Headline("status of the checkout")
	pipe.Legacy(long)
	if lines := pipe.Lines(); len(lines) != 2 || lines[1] != long {
		t.Errorf("a legacy line off a terminal was changed: %q", lines)
	}
	env.TTY = true
	tty := NewLegacy(env)
	tty.Headline("a headline that is longer than thirty columns")
	tty.Legacy(long, "  indented legacy line that runs past the width")
	tty.Hint(Hint{Argv: []string{"metasystem", "system", "start", "--repo", "/Users/wido/LocalStorage/GitHub/agentic-tools-m1e"}, Reason: "starts it"})
	expectPage(t, tty, `
a headline that is longer than
  thirty columns
landing lane
  /Users/wido/LocalStorage/GitHub/agentic-tools-landing:
  owner running (pid 38928)
  indented legacy line that
    runs past the width
→ metasystem system start --repo /Users/wido/LocalStorage/GitHub/agentic-tools-m1e
  starts it
`)
}

func TestNoTrailingSpacesAndNoEmptySections(t *testing.T) {
	t.Parallel()
	page := New(fixedEnv())
	page.Headline("headline")
	page.Section("Empty", "")
	table := page.Section("Rows", "").Table(Column{}, Column{})
	table.Row(Plain("a"), Plain(""))
	table.Row(Plain("bbb"), Plain("x"))
	page.Legacy()
	expectPage(t, page, `
headline

Rows
  a
  bbb   x
`)
	for _, line := range page.Lines() {
		if strings.HasSuffix(line, " ") {
			t.Errorf("trailing space: %q", line)
		}
	}
}

func TestSpansCompose(t *testing.T) {
	t.Parallel()
	env := fixedEnv()
	env.Color = true
	page := New(env)
	page.Section("", "").KV("k", Bold("b"), Plain(" p"), Dim(" d"), Marked(Done, "landed"))
	if got := page.String(); got != "k   \x1b[1mb\x1b[0m p\x1b[2m d\x1b[0m\x1b[32m✓\x1b[0m landed\n" {
		t.Errorf("spans = %q", got)
	}
	if Plain("x").Text() != "x" || Status(Done).Text() != "✓ done" || Marked(Idle, "a").Text() != "○ a" {
		t.Error("a span's text is its symbol and words")
	}
	_ = time.Now
}

// A wrapping flexible column keeps every word: its text continues under the
// column instead of being cut.
func TestAWrappingColumnContinuesUnderItself(t *testing.T) {
	t.Parallel()
	env := fixedEnv()
	env.Width = 50
	page := New(env)
	table := page.Section("Landing lane", "").Table(Column{}, Column{Flex: true, Wrap: true})
	table.Row(Marked(Running, "open"), Plain("batch wa01 waits for goal-x on m1b (build, ~8 min); a separate proof costs ~40 min"))
	table.Row(Marked(Running, "proving"), Plain("batch wa02 started"))
	expectPage(t, page, `
Landing lane
  ● open      batch wa01 waits for goal-x on m1b
              (build, ~8 min); a separate proof
              costs ~40 min
  ● proving   batch wa02 started
`)
	env.Width = 30
	narrow := New(env)
	moved := narrow.Section("Landing lane", "").Table(Column{}, Column{Flex: true, Wrap: true})
	moved.Row(Marked(Running, "collecting"), Plain("batch wa01 waits for goal-x on m1b"))
	expectPage(t, narrow, `
Landing lane
  ● collecting
    batch wa01 waits for
    goal-x on m1b
`)
}

// A fixed text is printed whole on one line however wide: a protocol line
// another agent reads verbatim, such as a peer message's preface.
func TestAFixedTextIsNeverWrapped(t *testing.T) {
	t.Parallel()
	env := fixedEnv()
	env.Width = 20
	page := New(env)
	page.Headline("One message")
	section := page.Section("", "")
	section.Fixed("[peer ask from m1b: a fixed preface longer than the width]")
	section.Text("prose that is longer than the width wraps")
	expectPage(t, page, `
One message

[peer ask from m1b: a fixed preface longer than the width]
prose that is longer
  than the width
  wraps
`)
}
