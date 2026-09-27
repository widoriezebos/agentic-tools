# The Partner proposes what to do: what landed, and what waits

Written 2026-09-27 for Wido, in plain English, after the night's work on
g1-s58, g1-s60 and g1-s61, and brought up to date the same night after
g1-s63, g1-s64 and g1-s62 landed on your word "Finish all now". The
designs and their Built sections are the record; this page explains
them.

## What landed, first round

Three slices, each after Astra's design loop, an Opus build, Sol's code
read and one fix round with every fix held by a test that failed against
the old code.

- **g1-s58, the Partner proposes what to do** (`4acaa5e7c`). The
  Partner can propose only as verb statements, one `propose` call per
  action, over the goal acts the interface has. Each answer's proposals
  become one card in the conversation: a checkbox per action, the goal
  named, every argument written out, the Partner's reason in its own
  voice. You tick and press Apply; the interface performs the acts one
  by one through the same routes the buttons use, under your sign-in. A
  definite refusal shows the engine's own sentence and the run goes on;
  an answer that does not say what happened stops the run, with
  Continue, Try again and Ask the Partner as your presses. Each line's
  state is recorded on the message before and after the act, with a
  version, so two presses on one line, in one tab or two, cannot apply
  an act twice. The page you are looking at re-reads after each act,
  never while a sheet of yours is open. Older cards fold to one line.
- **g1-s60, proposed by the Partner, in the inbox** (`4f6ee3f71`).
  Proposals you have not answered are the first group on the Decisions
  page, with "n new"; a row opens to the whole line and its acts;
  ticking several and pressing Apply opens a sheet that lists every line
  whole, with one Apply. The card in the drawer and the row in the inbox
  read one record.
- **g1-s61, a chip on the goal's row** (`1f6c6e2a1`). "Pause proposed"
  on the board card, the queue row and the goal's header; red when the
  act was refused; pressing it opens the drawer at that card.

## What landed, second round (the same night, after the verbs landed)

- **g1-s63, the deferred lists worked again** (`1b5fce82a`). The six
  small items the first round's code reads had set aside, built as the
  reads asked, no design loop of their own; Sol found nothing to fix.
  In your terms: the drawer forgets where a chip last sent you once you
  close it; a proposal to open a goal that waits for another goal is
  checked for that other goal when the Partner proposes it, not when
  you press; a proposal keeps the time it was proposed, so a line you
  tried and the engine refused no longer reads "new" and drops to the
  end of its group; the two-tabs-cannot-apply-twice rule is now tested
  on every allowed transition; the act layer's "pushed, unknown" and
  "journal unreadable" answers are proven through a real publish in the
  test bed; the inbox no longer shows a line as "in flight" for the
  moment between its own press and the page's re-read.
- **g1-s64, abandon from the browser and from the Partner**
  (`84f8acfbf`). Your "agreed on the Abandon topic; that needs to fit
  the verbs" is recorded as ruling R-128-ui in the register: a
  signed-in browser session may abandon a goal, the engine admits the
  session at `goal abandon` as it does at park and unpark, and the
  history line names the session as the hand. The Partner can now
  propose Abandon with a reason and, where the work moves elsewhere, a
  successor. Because abandoning is not undone by a press, the card
  shows the goal's whole intent above the reason, lists the goals that
  wait for it ("Holds up: 2 goals wait for it: X, Y") and says what
  happens to them: with a successor they wait for the successor
  instead; without one the engine refuses until they are waived or
  abandoned at a terminal, in its own words, which now name the public
  `--successor` flag. If the goal or its dependents changed since the
  Partner proposed it, the line refuses to send and asks you to open
  the goal and ask again. Sol found one thing material, the intent was
  missing from the card, and it was fixed in the one fix round. No
  Abandon button on any page yet; the Partner's proposal is the only
  way from the browser, by design.
- **g1-s62, one vocabulary** (`dd62c5102`). The Partner now speaks the
  public grammar the terminal speaks: `goal pause`, `goal unapprove`,
  `goal prioritize`, and so on, with the same flags `metasystem goal
  ACTION --help` shows, ten actions in all. Anything else it tries
  (authority flags, forms the interface cannot carry, goal actions the
  interface has no act for) is refused by name with the terminal form
  to use instead. A test in the terminal's own package holds the
  Partner's ten actions and every flag against the verb table, from
  both sides, so a rename there breaks the build here rather than the
  Partner. The pages took the same words: the buttons say Pause,
  Resume, Unapprove and Prioritize where they said Not now, Return to
  queue, Withdraw approval and Set priority; the Decided tab says
  Paused; the help terms name the old word once. Sol found nothing to
  fix; one word in the Partner's instructions ("nine" where the tool
  names ten) was corrected at the landing.

All six are on `ui-development` and `main`; the verbs seat has merged
the UI landings into its own work and has been told the ten goal
actions and their flags must not change under U1d. The interface at
http://127.0.0.1:7878 was restarted onto the final build (still "not
proven", as before, so sign in with your code to act).

Evidence: `~/LocalStorage/agentic-tools-evidence/g1-s6N-20260927/`
(screenshots, the builders' reports, Sol's reads); Sol's reads also
beside each design as `<slice>-sol-read.md`.

## The critique round (2026-09-28)

You asked for Astra to critique the code and for any issues to be fixed.
Astra read the whole body of work in three parts and found sixteen
things, ten of them serious. Four fix rounds followed, each one built
from a failing test, each checked and read again by Astra. You then set
the stop rule: stop when what is left has no real impact or will
practically never happen. Astra's last read found everything closed and
nothing left that you would plausibly hit and pay for, and said stop.

What changed for you, in plain terms:

- **A proposal can no longer be applied twice**, not from two tabs, not
  from the drawer and the inbox together, not by a slow answer arriving
  late. Each press now owns the line it is applying; the server refuses
  any other press until it is done, and a press that lost its line keeps
  its own answer for you to see rather than overwriting what a later
  press did.
- **Repeating an action is fine.** Your ruling: pausing a paused goal,
  approving an approved goal, and so on for all ten actions, reads as
  done, with nothing written twice.
- **The page recovers by itself** from a push whose confirmation was
  lost, from a failed refresh, from a failed dismissal, and it keeps
  a form you were typing in.
- **Signing in for the first time keeps the conversation**, including a
  turn that was still being answered.
- **The Partner's lists are bounded and batched.** At most fifty
  proposals per answer, refused at the moment it tries a fifty-first,
  and its instructions say to offer the next batch; every field it
  sends has a size limit, so a huge proposal can no longer break the
  conversation file.
- **A conversation file that is too large to open is repaired**: the
  original is kept beside it as an archive, the rest is trimmed to what
  fits, and you are told once. Summarizing what was cut is for later,
  as you said.

The record, with Astra's reads verbatim and what was left as a residual,
is `partner-proposals-astra-code-critique.md` beside this page. The
residuals are all of the "three tabs racing in one second" or "a file
another program wrote" kind.

## What waits now

Of the first round's seven waiting items, five are done (items 2, 3, 4
and 6 by g1-s63; item 7, the words and abandon, by g1-s62 and g1-s64)
and one you ruled on (item 5: old proposals may vanish with the
conversation's trim; nothing is applied by that). What is left:

1. **The final check happens in the browser, not inside the ledger
   write** (the first round's item 1, unchanged). The page re-reads the
   goal before sending an approve, an edit or now an abandon, and
   refuses a changed goal; the engine's own compare inside the
   transaction is gate 2's engine work, not a browser change.
2. **One word for you to rule on.** The bulk sheet's title reads "Pause
   3 goals" where the design wrote "Pause 3 selected". Sol called it
   non-material; I left the builder's word because it names what is
   acted on. Say the word if you prefer the other.
3. **Small things each code read set aside**, recorded under "Later,
   when it hurts" in each Built section: the session is recorded on the
   abandoned goal's own history line, not on the lines of the goals it
   repoints to a successor; a proposal recorded before g1-s63 may carry
   a restamped time; the screenshots cannot tell admission time from
   write time (the tests can).
4. **Not built, by design:** an Abandon button on the queue row and the
   goal page; waive and also-abandon from the browser; `goal done` and
   `goal reopen` as browser acts; the other objects' writes as verbs
   (`question ask`, `question withdraw`, `design …`) in the Partner's
   catalogue. Each is one small slice when you want it.

## What I recommend next

Nothing tonight. Use it for a day: ask the Partner to find obsolete
goals and to close them, tick the lines you agree with, apply, and see
whether the card, the inbox group and the chip say what you expect. The
first thing that hurts is the next slice.
