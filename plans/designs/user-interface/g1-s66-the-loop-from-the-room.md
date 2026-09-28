# g1-s66: the loop from the room — critique, fold and hand over without leaving the interface

- Kind: design
- Id: 01M3KCV1EDQPB9X3PFZBE0R77R
- Status: draft
- Goals: browser-interface partner-runs-the-design-loop

Wido, 2026-09-28, after asking whether the design discussion we were
having in a terminal could be held in the interface with the Partner,
and being told it could, but that the production half could not: "What
would close the gap is three things, each a slice: the design verbs in
the Partner's grammar, so a critique round is a proposal you apply;
suggest at the level of a whole section, so a fold is one press; and a
sitting that can hand its record to this seat with one line, 'continue
from the record'" — "I want this. make a great UX design for this and
then a goal with prio 1 in my name so we can pick this up later."
Author Fable. Every cite re-read at `83f60f56a`; revision 2
(2026-09-28) folds Astra's round 1, re-read at `3e1262633`: the
decisions live in the engine's own decisions file until the close, the
cards read the round's return, the act carries the reader budget, and
a section is replaced only when its heading is one.

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
row "accepted" into the round's decisions with the fold's one-line
amendment. **Refute** asks for your reason and writes "refuted:
reason". **Defer** writes "noted" on a small finding, and on a material
one asks you for the evidence that it is outside the brief and writes
"out-of-scope". When every card of the round has its row, **Answer the
round** is one press; the page says what the engine did: "closed", when
nothing you folded changed the design, or "round 2 requested, the
failsafe round", when it did. Round 2 comes back the same way; you
answer it; the page prints the engine's own closing words and the
design gains its Dispositions table at the foot. Nothing here needed a
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
  FILE --goal G --tool-calls N` and `design review FILE --dispositions
  FILE --after N --tool-calls N` run through the act layer under the
  human's sign-in, the way the launch act does, starting the configured
  critique lane and returning the chain's reference; the design page
  reads the chain's rounds and state from the run store. The engine
  refuses a review without a reader budget and never invents one
  (`cmd/metasystem/intent_delivery.go:771`), so Send to critique is a
  small sheet, not a bare press: the funding goal, preselected when the
  design's `Goals:` line names one approved goal and chosen when it
  names several, and the reader budget prefilled from the setting
  `review.design.tool-calls` (default 30, the number every brief in this
  directory used), both editable; the engine's own refusals (no
  approved goal, no budget, a claim held elsewhere) are shown verbatim.
  The Partner's grammar gains the two statements, held against the verb
  table by the existing test. A session no-op holds: sending a design
  already in a round rejoins it (R-129-ui).
- D2 (step 1). **Findings as cards, from the round's return; decisions
  in the engine's file.** A round's findings are read from that round's
  retained return, never from the findings register: the register keeps
  only material findings and merges a recurring id across rounds
  (`internal/dispatch/finding_register.go:1329`, `:1119`), and the
  structured return carries id, severity, material, claim and evidence
  (`scripts/agents/schemas/design-critic.schema.json:38`). A card shows
  those five, and the change asked for and the two tests only where the
  return's prose carries them under the finding's id; the register is
  read for the chain's state alone. A card's identity is chain, round
  and id, so round 2's card for a recurring id is a new card. Each card
  offers Fold, Refute and Defer; each press writes one row into the
  engine's own decisions file for that examination, the template the
  engine writes beside the return (`cmd/metasystem/intent_design_review
  .go:146`, `decisions.md`, with its `Review binding` header naming the
  goal, the record, the examination, the round and the subject and
  return digests, `intent_review_binding.go:37`), in the engine's four
  columns (Finding id, Disposition, Reasoning and evidence, Amendment;
  `internal/validate/critiqueclosed.go:13`) and its four values: Fold
  writes `accepted` with the amendment; Refute writes `refuted` with
  the reason, required; Defer writes `noted` for a non-material finding
  and, for a material one, `out-of-scope` with the evidence that it is
  outside the brief's threat model, required (`critiqueclosed.go:246`,
  where `noted` cannot answer a material finding). Nothing is written
  into the design by a press, so the design's digest stays the reviewed
  one until a fold changes it (D4). A row is written once per card; a
  reload finds the row in the file.
- D3 (step 1). **Fold by section.** `suggest` gains a target of a
  document and a heading: the Partner drafts that section anew, and the
  interface shows a section card with old and new side by side and the
  changed lines marked; Use replaces exactly that section under the
  revision check, as the Outcome is replaced today; Not this dismisses.
  A heading identifies a section only when it occurs exactly once: the
  Outcome writer takes the first match and appends when there is none
  (`partner/sitting.ts:399`, `:434`), and the writer checks the
  document's revision, not the section (`project/edit.go:89`), so the
  section card refuses, in words and keeping the draft, a heading that
  is absent or occurs twice, and after a revision conflict the card
  re-reads the section and shows the comparison against the words as
  they are now before offering Use again. Fold on a finding card asks
  the Partner for the section the finding names and opens that card;
  Use then writes the design and the `accepted` row (D2) in that order,
  and a Use that succeeded without its row is offered the row again.
- D4 (step 1). **The next round, and the close, both the engine's.**
  When every finding of the current round has a row, one press, Answer
  the round, runs the verb with `--dispositions FILE --after N` on the
  engine's decisions file (D2); the engine then does what its own rule
  says: a design whose digest is still the reviewed one closes the
  chain (`intent_design_review.go:178`, `closeDesignCritique`), and a
  design a fold changed gets its follow-up examination in the same
  chain, which the chain's accounting refuses past the round limit.
  The page shows the engine's outcome and its wording, "closed", "closed
  on N fixture obligations", "round 2 requested", or the refusal that
  leaves the chain open for a human, and never computes an exit line.
  Three obligations on the engine lane, all the verb's and reached from
  the terminal and the page alike. First, the author's validated
  decisions reach the register: today the join persists only
  `out-of-scope` (`internal/validate/critiqueclosed.go:55`), and the
  close then reads the canonical register (`delegation/phases.go:369`),
  where a still-open bounded finding refuses closure
  (`dispatch/finding_register.go:662`), so a refuted finding on an
  unchanged design would refuse where it should close; the verb
  applies a validated `refuted` (and `accepted` with its amendment) to
  the register's finding before the close, keeping the difference
  between refuting a finding and accepting its risk, held by a fixture
  through the real register close. Second, the final round exits by
  the engine's own classification and nothing else
  (`finding_register.go:651-690`, the skill at `design-critique/SKILL
  .md:67`): clean closure, closure on fixture obligations with the
  obligations published and their wording, or the human-required
  refusal with the chain left unclosed; "all decided" is never a
  closure condition of its own. Third, the close appends the design's
  Dispositions section (one row per finding: id, finding, fold, as
  every design here carries) composed from every answered round of the
  chain, each round's own decisions file (`intent_design_review.go:
  369`, `intent_review_binding.go:78`), keeping the round, the
  reasoning and the amendment, so a recurring id shows both
  adjudications and a quiet final round does not erase the first
  round's trail; it is the close's last act, when the digest no longer
  matters.
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

`POST /api/design/<path>/review` `{goal, toolCalls, after?}` runs the
verb under the signed-in session and answers the chain reference and
the engine's outcome words; `GET /api/design/<path>/review` answers the
rounds, each with its findings read from that round's retained return
(`artifacts/agents/<root>/rounds/<N>/return.json`,
`intent_delivery.go:512`; `outputs.md` names the design, not the
return) and its decisions file as it stands, and the chain's state. `PUT
/api/design/<path>/review/<round>/decisions` `{finding, disposition,
reasoning, amendment}` writes one row into the engine's decisions file
under its `Review binding` header, refusing a finding the round does
not carry, a disposition outside the four, and a material finding
`noted`. `suggest` gains `{document, section}` and the section card,
refusing an absent or duplicate heading. The New goal sheet's prefill
takes a next step beside the intent. The proposal grammar gains
`design review` in its two forms. The setting `review.design.tool-
calls` is read with `ConfValue`. The cut guard's call sites gain the
three reads.

## 7. Not here, later

D6; a critic other than the configured lane chosen from the page; the
findings of a code critique on a build; a page that shows two rounds'
diffs of the design itself; running the loop for several designs at
once.

## 8. Verification and box

Go: the act runs the verb under the session and refuses without one,
passes the reader budget and the funding goal, and shows the engine's
refusal for a missing budget verbatim; the rejoin no-op; the findings
read from the round's return, a non-material finding and a recurring
id both shown as their own cards; the decisions row write under the
binding header, refusing an unknown finding, a fifth value and a
material `noted`; Answer the round closing an unchanged design whose
one material finding was refuted, through the real register close, and
requesting the follow-up for a changed one; the final round exiting
each of the three ways by the engine's classification (clean, on
fixture obligations with them published, human-required with the chain
left open); the close appending the Dispositions section from every
answered round with a recurring id shown twice; the exit words come
from the engine. Frontend: the Send
to critique sheet with the goal and the budget; the round's cards with
three presses; Fold opening the section card with old and new and Use
writing exactly that section, refusing an absent or duplicate heading
and re-reading after a conflict; Refute refusing without a reason,
Defer on a material finding requiring evidence; a reload finding the
rows in the file; Answer the round appearing only when every card has a
row; the engine's words shown; Open a goal from this design prefilling intent and next step and
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
not the page's. Weakest: the close's two obligations on the engine lane
(the appended section, the human-residue close) are the one place this
page asks the verb to grow; if a critic returns prose instead of the
structured return, the page shows the prose and offers no presses,
which is honest and enough.

## Dispositions (Astra round 1, 2026-09-28, under R-121 and R-124)

Read of revision 1 at `3e1262633`, verbatim in
`g1-s66-astra-critique.md`. Five material findings, all folded; every
cited line re-read at whole-function depth before folding.

| id | finding | fold |
|---|---|---|
| S66-01 | the act as written cannot start a review: the engine requires `--tool-calls N` and refuses without it (`intent_delivery.go:771`, its test at `:223`), and nothing names the funding goal | D1: Send to critique is a sheet with the funding goal (preselected from `Goals:` when one) and the reader budget from the setting `review.design.tool-calls`, both editable, the engine's refusals shown verbatim |
| S66-02 | the findings register keeps only material findings and merges a recurring id across rounds (`finding_register.go:1329`, `:1119`); the return schema carries five fields (`design-critic.schema.json:38`); a document-wide `[f:<id>]` mark makes round 2 look answered by round 1 | D2: cards read from the round's retained return, five fields plus prose where present; the register for state only; identity is chain, round and id |
| S66-03 | a dispositions file composed from a "folded/refuted/deferred" table lacks the `Review binding` header and the engine's four columns and four values, and `noted` cannot answer a material finding (`intent_review_binding.go:37`, `critiqueclosed.go:13`, `:246`); the design template has no Dispositions section (`write.go:120`) | D2: rows are written into the engine's own decisions template beside the return, in its columns and values; Fold → accepted, Refute → refuted, Defer → noted or, for a material finding, out-of-scope with evidence |
| S66-04 | every row written into the design changes its digest, so the engine requests a follow-up instead of closing (`intent_delivery.go:894`, `intent_design_review.go:178`); no explicit close after the final round | D2 and D4: nothing is written into the design by a press; Answer the round runs the verb and the engine closes an unchanged design or requests the follow-up; two obligations on the engine lane: the close appends the Dispositions section from the file, and a decided file on a design changed after the final round closes as human residue |
| S66-05 | a heading names the first match and appends when absent (`sitting.ts:399`, `:434`); the writer checks the revision, not the section (`edit.go:89`) | D3: the section card refuses an absent or duplicate heading and keeps the draft; after a revision conflict it re-reads and compares again before Use |

Astra also verified, and the design leans on, that the launch act
requires a live session proof (`launch.go:72`), that review
continuation retains its operation identity and rejoins its child
(`intent_design_review.go:231`), that the recorder keeps words after a
revision conflict (`recording.ts:159`), and that goal ids are appended
space-separated (`write.go:402`). Two build lanes were judged not a
violation of the smallest thing.

**Round 2, the declared failsafe (2026-09-28, at `0324a0cc4`):**
S66-01, S66-02, S66-03 and S66-05 confirmed answered; S66-04 held
open through three new material findings on the close, all folded
into D4. S66-06: the join persists only `out-of-scope`
(`critiqueclosed.go:55`) and the close reads the canonical register
(`phases.go:369`, `finding_register.go:662`), so a refuted finding on
an unchanged design would refuse where it should close; fold: the verb
applies validated refutations and acceptances to the register before
the close, with a fixture through the real close. S66-07: my "human
residue" rule replaced the engine's exit classification
(`finding_register.go:651-690`, the skill at `:67`); fold: the final
round exits only by that classification, clean, on fixture
obligations, or human-required with the chain left open. S66-08: the
appendix from the closing round's file alone omitted earlier
adjudications, since each round has its own decisions file
(`intent_design_review.go:369`, `intent_review_binding.go:78`); fold:
the appendix composes from every answered round. Non-material: §6's
`outputs.md` corrected to `return.json`. Closed at the failsafe round
on three folds, with one scoped confirmation read on S66-06 to S66-08
alone (recorded below when it returns).
