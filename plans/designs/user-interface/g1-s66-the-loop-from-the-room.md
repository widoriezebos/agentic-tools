# g1-s66: the loop from the room — critique, fold and hand over without leaving the interface

- Kind: design
- Id: 01M3KCV1EDQPB9X3PFZBE0R77R
- Status: draft
- Goals: browser-interface, partner-runs-the-design-loop

Wido, 2026-09-28, after asking whether the design discussion we were
having in a terminal could be held in the interface with the Partner,
and being told it could, but that the production half could not: "What
would close the gap is three things, each a slice: the design verbs in
the Partner's grammar, so a critique round is a proposal you apply;
suggest at the level of a whole section, so a fold is one press; and a
sitting that can hand its record to this seat with one line, 'continue
from the record'" — "I want this. make a great UX design for this and
then a goal with prio 1 in my name so we can pick this up later."
Author Fable. Every cite re-read at `83f60f56a`.

## 1. What exists and binds

1. **The sitting leaves a record and stops there.** A design sitting
   deposits facts, proposals, decisions and open questions into the
   design record by the human's press and ends with an Outcome section
   (g1-s53, g1-s55); Ask it sends an open question to the register. What
   happens to the design afterwards happens at a terminal: the author
   rewrites it, the critique is dispatched, the folds are written, the
   goal is opened. The paper's reset rules already say "the design page
   is the brief" (docs/paper/18-outlook.md, "Rules from the first week
   of self-application"), so a seat that reads the page has its brief.
2. **The critique loop is an engine verb with a human adjudicator.**
   `metasystem design review FILE [--goal G] [--dispositions FILE
   [--after N]] [--retry N]` retains and starts the configured critique
   lane (Codex on Astra, `development/project-rules-local.md`), a
   changed design with `--dispositions` continues the same bounded
   chain, and "the author decides findings; critique does not accept the
   design on the human's behalf" (`cmd/metasystem/intent_review_help.go`,
   `designReviewHelpForms`). The skill fixes the loop's shape: the
   materiality tests (R-121, R-124), the designer's disposition on every
   finding, accept or refute, the failsafe round declared at start, no
   third round, `--findings RETURN --dispositions FILE` checking that
   every finding of a round is decided
   (`skills/design-critique/SKILL.md`). Every design in this directory
   carries the result of that loop as a **Dispositions** table at its
   foot, one row per finding: id, finding, fold.
3. **The Partner's grammar holds the ten goal acts and nothing else.**
   It proposes verb statements in the terminal's public grammar, one
   `propose` per act, held against the verb table by a test; the human
   ticks and applies under their own sign-in through the act layer
   (g1-s58, g1-s62). `goal open` is one of the ten, so a goal opened from
   the browser exists today (`httpd/acts.go:262-283`). A browser act that
   launches a process has a precedent: `POST /api/fleet/launch` starts a
   machine on this host under the human's session (g1-s43). The design
   verbs are on the "not built, by design" list of the proposals arc
   (`partner-proposals-what-landed-and-what-waits.md`).
4. **The Partner writes with you, one field at a time.** `suggest`
   prepares words for one field of an editor the human handed over; the
   human presses Use; nothing is saved by the tool (g1-s51, g1-s52;
   `internal/ui/uitools/suggest.go`). The document editor is CodeMirror
   over a record's whole source, saved through one whole-source edit
   under a revision check (`httpd/write.go`, `project/edit.go`); the
   sitting already replaces exactly one section of that source under
   the same rule (`partner/sitting.ts`, `outcomeWritten`).
5. **A design and its goal.** A design names its goals one way on its
   `Goals:` line; "New goal for this design" opens a goal with its intent
   prefilled and appends the id to that line, so nobody types an id
   (g1-s26). The New goal sheet opens with a prefilled intent
   (`backlog/OpenSheet.tsx:105`); approval stays the human's act and
   shows in the Decisions inbox.

## 2. What you want after a sitting

- **H1. Send it to critique with one press**, and see the round come
  back as cards on the design, not as a file in a terminal.
- **H2. Answer each finding as the designer**, accept or refute with
  your reason, and have that be the Dispositions table the design
  already carries.
- **H3. Fold with one press.** The Partner drafts the section anew, you
  see old and new side by side, Use writes it.
- **H4. Close the loop by the rules without knowing them.** The
  failsafe round, no third, the exit line on the page.
- **H5. Hand it over with one line.** A goal opened from the sitting's
  outcome whose next step says "continue from the record", approved by
  your press, picked up by a seat that reads the page as its brief.

## 3. The room, continued

You end a design sitting; the Outcome is recorded. On the design page,
beside the status, stands **Send to critique**: one press, and the page
says "Critique · round 1 of 2 · Astra reading" with the chain's own
state. You go do something else. The bell says the round is back. On
the design page, under a new heading **Round 1**, the findings stand as
cards: id, severity, material or not, the claim in one sentence, the
evidence as anchor chips into the code and the design, the change it
asks for. Each card has three presses. **Fold** asks the Partner for the
section's new text; a section card appears with the old text and the
new side by side, the changed lines marked, and **Use** writes exactly
that section into the design under the reading rule, then writes the
row "folded" into the Dispositions table with the fold's one-line
summary. **Refute** asks for your reason and writes the row "refuted:
reason". **Defer** writes "deferred" with the field it will build on.
When every card of the round has its row, **Send round 2** is one press;
the page says the failsafe round is the last. Round 2 comes back the
same way; when its findings pass the works-and-safe test the page
prints the skill's exit line, "closed at round 2 on N fixture
obligations", and the critique heading folds. Nothing here needed a
terminal, and nothing here was written by anyone but you: the Partner
drafted, Astra attacked, you decided.

Then **Open a goal from this design**: the New goal sheet opens with
the intent taken from the Outcome's first paragraph and the next step
"Continue from the record plans/designs/user-interface/g1-s66-…md: build
step 1 as its §5 says", the design's `Goals:` line gains the id when the
goal opens, and the goal waits in your inbox for your approve. A seat
claims it and its builder reads the page. That is the sitting's own
test made into a press: a fresh worker and the record alone.

The Partner can propose every one of these presses in a conversation:
"Send g1-s66 to critique", "Open the goal from its outcome", and you
tick and apply.

## 4. Decisions

- D1 (step 1). **The design verbs as browser acts.** `design review
  FILE --goal G` and `design review FILE --dispositions FILE --after N`
  run through the act layer under the human's sign-in, the way the
  launch act does, starting the configured critique lane and returning
  the chain's reference; the design page reads the chain's rounds and
  their findings from the run store. The Partner's grammar gains the two
  statements, held against the verb table by the existing test. A
  session no-op holds: sending a design already in a round rejoins it
  (R-129-ui).
- D2 (step 1). **Findings as cards.** A round's findings, in the shape
  the brief template already demands (stable id, severity, material,
  claim, evidence, change, the two tests), are read from the chain's
  findings register and shown on the design page under "Round N" and in
  the drawer as cards. Each card offers Fold, Refute and Defer; each
  press writes one row into the design's Dispositions table through the
  whole-source edit under the sitting's reading rule, marked with the
  finding's id so a reload cannot write it twice; Refute requires a
  reason.
- D3 (step 1). **Fold by section.** `suggest` gains a target of a
  document and a heading: the Partner drafts that section anew, and the
  interface shows a section card with old and new side by side and the
  changed lines marked; Use replaces exactly that section under the
  revision check, as the Outcome is replaced today; Not this dismisses.
  Fold on a finding card asks the Partner for the section the finding
  names and opens that card.
- D4 (step 1). **The next round, and the close.** When every finding of
  the current round has a row, Send round 2 composes the dispositions
  file from the table and runs the verb with `--dispositions --after N`;
  the chain's own accounting refuses a third round; the page shows the
  skill's exit line from the chain's state and never computes one
  itself.
- D5 (step 1). **From the outcome to a goal.** The End sheet and the
  design page gain Open a goal from this design: the New goal sheet
  prefilled with the intent from the Outcome's first paragraph and the
  next step "Continue from the record <path>: build step 1 as its §
  says"; on open the design's `Goals:` line gains the id (g1-s26). The
  approve stays the human's act in the inbox.
- D6 (later). `design write G --brief FILE` from the browser, a new
  design authored by the design lane; a whole-record suggestion with a
  page-level diff; the code-critique loop (`work review`) from the
  browser in the same shape; several designs' rounds side by side.

## 5. Step 1, the smallest thing that works

D1 to D5, in two build lanes: the engine's act and findings read (D1,
the register read behind D2, D4's composition), and the interface's
cards, the section suggestion and the goal prefill (D2, D3, D5). What
exists after it: a design goes to critique with one press, comes back
as cards, is folded and answered from the page, closes by the rules,
and becomes a goal in the inbox with one press. Not in step 1: D6.

## 6. Payload and routes

`POST /api/design/<path>/review` `{goal, after?}` runs the verb under
the signed-in session and answers the chain reference; `GET
/api/design/<path>/review` answers the rounds with their findings and
the chain's state and exit line. `suggest` gains `{document, section}`
and the section card; the Dispositions row write is a whole-source edit
composing one row under the table's heading, marked `[f:<id>]` as
deposits are marked. The New goal sheet's prefill takes a next step
beside the intent. The proposal grammar gains `design review` in its
two forms. The cut guard's call sites gain the two reads.

## 7. Not here, later

D6; a critic other than the configured lane chosen from the page; the
findings of a code critique on a build; a page that shows two rounds'
diffs of the design itself; running the loop for several designs at
once.

## 8. Verification and box

Go: the act runs the verb under the session and refuses without one;
the rejoin no-op; the findings read from the register in the brief's
shape and nothing else; the dispositions composition round-trips the
table; the exit line comes from the chain's state. Frontend: Send to
critique and its state line; the round's cards with three presses; Fold
opening the section card with old and new and Use writing exactly that
section; Refute refusing without a reason; a reload not writing a row
twice; Send round 2 appearing only when every card has a row; the exit
line; Open a goal from this design prefilling intent and next step and
the `Goals:` line gaining the id; the Partner proposing the two
statements and the verb-table test holding them; the guards stay green.
Walkthrough: a canned round of two findings, one folded, one refuted,
and the goal opened; screenshots at 1280 and 400. Box: two build lanes
(Claude on Opus), one code read each by the code-review lane the roster
names, with one fix round under R-124, after Astra's read of this page;
two attempts, 300 to 450 job-minutes.

## 9. Self-grade

High on D2, D4 and D5: cards, a table row and a prefilled sheet are
shapes the interface has. High on D3: one section replaced under the
reading rule is the Outcome's own mechanism with a heading argument.
Medium on D1: the first browser act that starts a critic, on the launch
act's precedent; the spend it starts is the roster's and the budget's,
not the page's. Weakest: the findings register's shape is the engine's
and this page reads it as the brief template writes it; if a critic
returns prose instead, the page shows the prose and offers no presses,
which is honest and enough.
