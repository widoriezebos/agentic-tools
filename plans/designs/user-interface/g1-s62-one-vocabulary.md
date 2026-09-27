# g1-s62: one vocabulary, the Partner's public grammar and the pages' words

- Kind: design
- Id: 01M3GZ2C1CA55JSYT796S4RX4V
- Status: draft
- Goals: browser-interface

Wido, 2026-09-27, with the verbs seat's word that the object-and-action
surface is stable on main and the goal actions keep their names: "so
you can now continue the build. Finish all now." This is the unit
g1-s58 kept for last (its revision 5, read by Astra, superseded for the
build by revision 8 until the verbs landed) and g1-s59 (the button
words, read by Astra: zero material, on hold behind the verbs), folded
into one slice as g1-s58's later list said they would be: the card,
the buttons, the terminal and the Partner take one word in one slice.
Author Fable. Every cite read at `2c87659a4`, the merge of main's
`4ca568e6c`.

## What exists and binds

1. **The public surface is one table, keyed by object and action.** Every
   public form is `metasystem OBJECT ACTION …`; the table is
   `intentCommand{object, action, name, audience, summary, usage, flags,
   hidden, …}` (cmd/metasystem/intent.go:48-81), its public rows
   `publicIntentCommands()` (intent.go:977), the goal object's rows in
   cmd/metasystem/intent_planning.go:83-376 (`open`, `edit`, `approve`,
   `unapprove`, `pause`, `resume`, `prioritize`, `block`, `unblock` among
   them, each with `audience` human, agent or both and its flags marked
   advanced or hidden where they are authority or plumbing). The
   Partner's `kit` tool reads a projection of it, name, summary, scope
   and usage (cmd/metasystem/ui_describe.go:87-96). The verbs design
   rules that help, JSON help, the Partner catalogue and typo suggestions
   are generated from that one table (metasystem/plans/designs/verbs-object-action.md:88-96,
   G2; R2 at :490) and names this unit: "The UI lane's g1-s58
   public-grammar unit waits for U1 and consumes its table" (:791-792).
   The nine goal actions' public forms, from the merged executable's
   `goal ACTION --help`: `goal open G --intent TEXT --next TEXT --risk
   severity=N,novelty=N,exposure=N,accumulation=N --basis TEXT` with
   `--label`, `--blocked-by`, `--blocks` repeatable; `goal approve G…
   [--budget BOX]`; `goal unapprove G --reason TEXT`; `goal prioritize G
   1|2|3 [--sequence N]`; `goal block G --on G2`; `goal unblock G --on
   G2`; `goal pause G --reason TEXT`; `goal resume G`; `goal edit G
   [--intent TEXT] [--next TEXT]` with `--label` and `--unlabel`
   repeatable. The words are the ones g1-s58's revision 5 wrote down,
   now under the object.
2. **What g1-s58 built and persists.** The `propose` tool's catalogue is
   the nine route ids with the route bodies' field names; the card's
   verb word is the page's button word through one map; the message
   persists the route id (`open-goal` … `edit-goal`), so the public name
   is the tool's boundary and a rename touches no stored proposal
   (g1-s58 D1, revision 8 and Built). The join test holds the catalogue
   to the interface's act table (`internal/ui/httpd/proposals_test.go`).
   The inbox rows and the goal-row chip take their words from the same
   map (g1-s60 D2, g1-s61 D2).
3. **The pages' words for the same acts differ.** `Withdraw approval…`
   and `Set priority…` in the board's card menu (src/backlog/menu.ts:40,
   43); `Withdraw approval` as the act sheet's title and button
   (src/backlog/ActSheet.tsx:115, 146) and on the Approved tab
   (src/decisions/DecisionsPane.tsx:871); `Not now` on the queue row, the
   selection bar and the bulk sheet's title, button, eyebrow and sheet
   name (src/decisions/BulkSheet.tsx:140-162, InboxRow.tsx:37); `Return
   to queue` (DecisionsPane.tsx:786-808, InboxRow.tsx:292); `Set
   priority` as the rank sheet's title and its submit button
   (src/backlog/RankSheet.tsx:76, 95); the help terms `Not now` and
   `Return to queue` (src/help/terms.ts:213-221); the Decided tab "Not
   now". The question sheet's own `Withdraw` withdraws a question
   (src/project/Sheet.tsx:541) and is not a goal act. The vocabulary the
   Partner is given is generated from the help terms and held by a drift
   test (src/help/terms.test.ts:142; internal/ui/partner/vocabulary.json).
4. **The reads that stand.** Astra read revision 5's grammar (g1-s58's
   record: S58-13, the advanced data flags admitted by name under the
   descriptor's spelling, the join test reading the descriptors
   themselves) and g1-s59's word table (zero material; S59-01 scoped the
   absence check to the four goal acts, S59-02 named the rank sheet's
   button and the bulk sheet's eyebrow and sheet name). Both stand and
   are taken here as written.

## Decisions

- D1. **The Partner speaks `goal ACTION` with the public flags.** The
  tool's `verb` is the action's public name under the goal object,
  `open`, `approve`, `unapprove`, `pause`, `resume`, `prioritize`, `block`,
  `unblock`, `edit`, and its fields are the action's public flags under
  their own names and spellings, `intent`, `next`, `risk`, `basis`,
  `label`, `unlabel`, `blocked-by`, `blocks`, `reason`, `on`, `priority`,
  `sequence`, beside `goal` and `explanation`; `risk` in the command's
  own form, parsed by `goal.ParseRiskRecord`; an edit's labels composed
  at admission by `goal.ApplyLabelDelta` from the labels read. The tool
  maps each to the route id and body it has today; the message persists
  the route id as before. Refused, by name and in words that say where
  the form is done: the authority and plumbing flags (`by`, `id`,
  `temporary-human-word`, `review-by`, `origin`, `under`, `verified`,
  `approved-ref`, the `-file` forms) and the forms the interface's route
  cannot carry (`next-append`, `risk`, `basis`, `tier`, `evidence` on an
  edit; `budget`, `under` on an approve; `under`, `verified` on a resume).
  A goal action the interface has no act for (`done`, `reopen`, `budget`,
  `claim`, `release`, `accept-risk`, `pin`, `split`, `group`, `ungroup`,
  `notes`, `repair`, `check`, and `abandon` until g1-s64) is refused with
  its own usage line from the catalogue, "not an act the interface has;
  at a terminal: metasystem goal done G --reason TEXT". The tool's
  description lists the nine as `goal open` … `goal edit`.
- D2. **The join test reads the table.** In `cmd/metasystem`, which has
  both: every catalogue verb is a row of `publicIntentCommands()` with
  object `goal` and that action, audience human or both and not hidden;
  every catalogue field is one of that row's flags that is not hidden,
  under the descriptor's own spelling, or its positional; no catalogue
  field is an authority flag. The existing join to the interface's act
  table stays, so the catalogue is held from both sides.
- D3. **One word on the card, the rows, the chip and the buttons.** The
  word map from route id to word becomes the action's name, Open,
  Approve, Unapprove, Pause, Resume, Prioritize, Block, Unblock, Edit,
  and the pages' buttons take the same words: `Withdraw approval` and
  `Withdraw` become `Unapprove` (card menu, act sheet title and button,
  Approved tab; the question sheet's `Withdraw` stays); `Set priority`
  becomes `Prioritize` (card menu, rank sheet title and submit button);
  `Not now` and `Not now for n selected` become `Pause` and `Pause n
  selected` (queue row, selection bar, bulk sheet title, button, eyebrow
  and sheet name, inbox row); `Return to queue` becomes `Resume`; the
  Decided tab `Not now` becomes `Paused`. The help terms are rewritten
  under the new words, each saying the old word once ("Pause, called
  Not now until the verbs landed"), their explanations keeping their
  substance; the Partner's vocabulary follows by regeneration. `Open goal`
  on the new-goal sheet's button stays: it is the object and the action.
- D4. **The Partner is told in the grammar's words.** Its instructions say:
  "call `propose` with the goal action's public name and its flags, as
  `metasystem goal ACTION --help` shows them; the interface has nine of
  them today and the tool names them". `deposit`'s refusal sentence for
  the kind `proposal` names `propose`.
- D5. **Nothing else moves.** No other object's actions; no abandon (its
  own slice, g1-s64); no display string outside the four goal acts'
  words; the routes, bodies and acts untouched.

## Not here, later

The checkout writes as verbs (`question ask`, `question withdraw`,
`design …`), each one catalogue row, one admission rule and one client;
the U1d set, none of which is a goal action.

## Verification and box

Go: the tool's catalogue under the public names and flags, each verb's
required fields and bounds, a foreign key refused by name, the
authority flags and the unsupported forms refused by name, a goal
action with no act refused with its usage line, `risk` parsed by
`ParseRiskRecord`, an edit's labels through `ApplyLabelDelta` with a
label in both lists refused; the join test in `cmd/metasystem` as D2,
and the existing join to the act table green; the skill drift test and
the vocabulary drift test green after regeneration. Frontend: the word
map's nine words; every test that asserted an old word asserts the new
one; a grep over `src/` finds none of the four goal acts' old words
outside the help terms' "called … until" clauses and comments, the
question sheet's `Withdraw` standing; the guards stay green; the bundle
rebuilt. Walkthrough: the card with `Pause` and `Unapprove`, the queue
with `Pause` and the selection bar, the bulk sheet, the `Paused` tab, the
board's card menu, the act sheet; screenshots at 1280 and 400, light
and dark. Budgets as always. Box: one build lane (Claude on Opus), one
code read (Codex on Sol) with one fix round under R-124, after Astra's
scoped read; one attempt, 90 to 150 job-minutes; lands after g1-s63,
whose bundle it would otherwise conflict with.

## Self-grade

High: two designs the critic has read, joined as their later lists
said, on a table the verbs seat calls stable. Weakest: "Paused" as a
tab name beside nouns, which g1-s59's read accepted.
