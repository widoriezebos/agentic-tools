# The Partner proposes what to do: what landed, and what waits

Written 2026-09-27 for Wido, in plain English, after the night's work on
g1-s58, g1-s60 and g1-s61. The designs and their Built sections are the
record; this page explains them.

## What landed

Three slices, each after Astra's design loop, an Opus build, Sol's code
read and one fix round with every fix held by a test that failed against
the old code. All three are on `ui-development` and `main`, and the
interface server at http://127.0.0.1:7878 was restarted onto the final
build (still "not proven", as before, so sign in with your code to act).

- **g1-s58, the Partner proposes what to do** (`4acaa5e7c`). The
  Partner can propose only as verb statements, one `propose` call per
  action, over the nine goal acts the interface has (open, approve,
  withdraw, set priority, block, unblock, not now, return to queue, edit).
  Each answer's proposals become one card in the conversation: a
  checkbox per action, the goal named, every argument written out, the
  Partner's reason in its own voice. You tick and press Apply; the
  interface performs the acts one by one through the same routes the
  buttons use, under your sign-in. A definite refusal shows the engine's
  own sentence and the run goes on; an answer that does not say what
  happened stops the run, with Continue, Try again and Ask the Partner as
  your presses. Each line's state is recorded on the message before and
  after the act, with a version, so two presses on one line, in one tab
  or two, cannot apply an act twice. The page you are looking at re-reads
  after each act, never while a sheet of yours is open. Older cards fold
  to one line. Two fixes to code every act shares came out of it: the act
  layer now says "pushed, unknown" or "journal unreadable" where it cannot
  know, instead of "refused", and the page no longer calls the ledger's
  definite non-writes unresolved.
- **g1-s60, proposed by the Partner, in the inbox** (`4f6ee3f71`).
  Proposals you have not answered are the first group on the Decisions
  page, with "n new"; a row opens to the whole line and its four acts;
  ticking several and pressing Apply opens a sheet that lists every line
  whole, an approval's budget included, with one Apply. The card in the
  drawer and the row in the inbox read one record.
- **g1-s61, a chip on the goal's row** (`1f6c6e2a1`). "Not now proposed"
  on the board card, the queue row and the goal's header; red when the
  act was refused; pressing it opens the drawer at that card.

Evidence: `~/LocalStorage/agentic-tools-evidence/g1-s58-20260927/`,
`g1-s60-20260927/`, `g1-s61-20260927/` (screenshots and the builders'
reports); Sol's reads beside each design as `<slice>-sol-read.md`.

## What waits, and why, item by item

The rule that decided these is R-124 (yours, 2026-09-25): after a code
read, a finding is fixed in the one fix round only when the slice does
not work or is not safe without it; the rest is recorded as "later, when
it hurts" so the slice stays small. Each item below passed that test.

### 1. The final check happens in the browser, not inside the ledger write

When you press Apply on an approval, the page first re-reads the goal
from the canonical branch and refuses the line if the goal's intent, next
step, tier, labels or budget changed since the Partner proposed it. What
it cannot do is make that check part of the ledger write itself. So if
someone at a terminal edits the goal in the fraction of a second between
the page's read and the write landing, you would approve the edited goal
without having seen the edit. Every Approve button in the interface has
had this same gap since the beginning; the card did not add it and is
slightly tighter than the sheet. Closing it means teaching the engine's
approve operation to compare what you reviewed against what it is about
to write, which is planned engine work (gate 2), not a browser change.

### 2. A proposed "waits for" on a new goal is not checked until you apply it

When the Partner proposes to open a goal that waits for another goal,
the service checks that the goal being opened is new, but not that the
goal it would wait for exists. If the Partner names a blocker that does
not exist, you do not find out on the card; you find out when you press
Apply and the engine refuses it with its reason on the line. Nothing
wrong is written; you learn one press later than you could.

### 3. A refused or retried line looks "new" again in the inbox

Every time a proposal's state changes, the record stamps the current
time on it. In the Decisions inbox that time is what "how old" and "n
new" read. So a proposal the Partner made yesterday that you tried this
morning and the engine refused now reads "today" and drops to the end of
its group. Cosmetic: nothing is lost or misrepresented about the act,
only the age and the ordering. The fix is keeping the original time
beside the time of the last change.

### 4. Reopening the drawer lands where the chip last sent you

Pressing a chip on a goal opens the drawer at that goal's proposal card.
That target is remembered and never cleared, so if you later open the
drawer with its ordinary toggle it opens at that same card instead of at
the latest message. You scroll once. This came from the last fix round
itself and was outside what that round was allowed to touch.

### 5. Very old proposals can disappear with the conversation's history

The conversation keeps its last two hundred messages and drops older
ones. A proposal lives on the message that made it, so if you ignore a
proposal for two hundred messages' worth of conversation it vanishes
from the card, the inbox and the chip together. Nothing is applied by
that; a choice you never made simply stops being offered. Whether a
waiting proposal should keep its message alive is a question nobody has
answered yet, so it is a decision rather than a fix.

### 6. Some behaviour is proven less directly than the design asked

Four cases where the code does the right thing as far as the walkthrough
shows, but the test that guards it is narrower than specified:

- the two-tabs-cannot-apply-twice rule is tested on one kind of
  transition, not every kind;
- the act layer's new "pushed but unknown" answer is tested by feeding
  it a hand-made result rather than a real failed publish;
- the rule that refreshing a page never destroys a form you are typing
  in is held by a test that reads the source code, because this app has
  no library for mounting a page in a test;
- for about ten milliseconds after an inbox run, a line the run left
  "unresolved" reads "in flight" until the page re-reads.

None of these is a wrong behaviour; they are places where a future
change could break something without a test catching it.

### 7. Two things wait on you or on the other seat

- **The words and the grammar.** The button words (Not now, Withdraw
  approval, Set priority) still differ from the public verbs the terminal
  uses, and the Partner speaks internal route ids rather than public
  verb names, because you said the verb system is being rebuilt and not
  to design against it yet. g1-s59 (the button rename) and g1-s58's
  public grammar (its revision 5, kept in the record) wait for that
  work to land.
- **Abandon.** "Close these goals" can only mean Pause until you rule
  that a browser session may abandon a goal, which the engine currently
  reserves for a terminal (R-125 admits a session for park and unpark
  only). A ruling in the same shape would make abandon one small slice.

## What I recommend next

Items 2, 3, 4 and the tests in 6 are an hour or two together. The
pattern for them exists: g1-s57 worked the deferred lists of eight
earlier slices as one slice, with no design loop of its own because the
reads were the critique. A g1-s62 of that kind would take the drawer
target cleared on close, the blocker check at admission, the original
time kept beside the write time, the wider race test and a fixture
publish for the act layer. Item 1 belongs to gate 2's engine work; item
5 and the abandon ruling need a word from you.
