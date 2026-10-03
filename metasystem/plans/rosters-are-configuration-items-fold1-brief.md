# Design brief: rosters-are-configuration-items, revision 5 fold 1 (the moved-effects inventory)

## Revision

Revision: revision 5 of `plans/designs/rosters-are-configuration-items.md`, second attempt: the
page as committed on main (f13778929) plus one new section answering Astra's round 1.

Reason: Astra's design critique, round 1 (Codex on `gpt-6-astra`, job
design-critic-3ee39606de100c9994e09957), returned one material finding, accepted by the seat. The
page moves responsibilities from today's owners to the roster store and reader but carries no
`Moved effects` section, so an implementer would leave some effects without an owner.

## Context pack

Read this pack first. Open another file only to check one of the cited lines
below; do not widen the read beyond that check.

Batch independent reads: when several files or ranges are needed and none depends on another's content, request them all in one turn, never one per turn.

Diagnosis or prior revision:

The prior revision is the document named under "Your output" as its current version; read it
whole. Change only what this brief asks: add the section below, and correct any sentence of the
page that the inventory shows to be wrong. Keep everything else as it is, word for word, so the
critique's next round reads a small change.

The rule the critic applied (`internal/protocol/roles/design-critic.md`, line 22): a design that
moves any responsibility from one owner to another (a write, a log line, a cursor or count, a
cleanup, a notice, a state change) must carry a section headed `Moved effects` whose table
(`| Effect | From | To | Code |`) lists every moved effect, its old owner, its new owner, and the
code that performs it today. The engine checks the table with
`bin/metasystem design review metasystem/plans/designs/rosters-are-configuration-items.md --check-only`
run from the `metasystem/` directory (today it reports `inventory=absent rows=0`); run it on your
draft's content before returning, by checking each Code cell names a file and line range that
exists.

Critique findings being answered:

1. MOVED-EFFECTS-RO-01 (severity medium, material), verbatim: "The first slice moves
   responsibilities without the required Moved effects inventory. In particular, it leaves
   ownership of launch-refusal recording unspecified when roster selection moves. The implementer
   needs an explicit assignment for configuration writes, selection of the recorded
   runtime/model/effort, and refusal recording. This changes step 1's ownership contract; safe
   transfer remains unproven without it." Its evidence: "the page has no section headed Moved
   effects with columns Effect, From, To and Code. Today
   metasystem/cmd/metasystem/intent_work.go:2109–2125 writes through the configuration owner;
   metasystem/internal/launch/settings.go:157–229 resolves model choices;
   metasystem/internal/launch/launch.go:102–125 selects launch values after admission and records
   them at 184–185. Configuration refusals are recorded by metasystem/internal/launch/admit.go:38–49
   only for LAUNCH_ errors. Moving lookup into Start can therefore return the new refusal without
   preserving its existing record unless that responsibility is assigned." The critic's reopening
   trigger: "A revised inventory omits an existing writer, selection effect or refusal record, or
   its cited code shows another responsibility without an assigned owner."

Cited code excerpts:

1. `cmd/metasystem/intent_work.go:2109-2125`, today's configuration write.
2. `internal/launch/settings.go:157-229`, today's launch selection.
3. `internal/launch/launch.go:102-125` and `internal/launch/launch.go:184-185`, selection after
   admission and its record.
4. `internal/launch/admit.go:38-49`, the refusal record, kept only for codes starting `LAUNCH_`.

(Read them there; no excerpt is quoted here, so nothing can drift.)

Example page:

`plans/designs/critique-findings-need-proof.md` stays the example for structure.

### What the new section must hold

- One row per moved effect, across all fifteen units, grouped by the unit that moves it. At
  least: writing a runtime, model or effort setting (`settings set` to the roster store and
  `roster set`); each reader's selection of runtime, model and effort (the launch settings, the
  dispatch resolver, the unit runner and `unitReadModel`, the standalone read, the critic's export
  into another installation, the steward's staged pair and continuation, the Partner, the runtime
  self-test, adoption tailoring); the effort a dispatched job records (the hazard table to the
  row); recording a launch refused for configuration (say which code the roster refusals carry so
  `admit.go` keeps recording them, or which new owner records them); recording which runtime,
  model and effort a launch ran with; `settings show` and `settings check` reporting these keys;
  the proof digest's inputs; and anything else the code shows moving.
- For each row the Code cell names today's code as `path:start-end` from the `metasystem/` module.
- Where an effect is deliberately dropped rather than moved, say so in the To cell ("dropped",
  with the reason) instead of leaving it out.
- If building the table shows a unit or a sentence of the page to be wrong, fix that sentence and
  name the change in the page's revision line.

## Recurring findings

From Astra's rounds on this goal's designs: a path that chooses a model before the roster reader
runs; a value shown that execution never reads; a file read from the wrong checkout; one displayed
value standing for two. The table is where each of these shows up as a row with no owner.

## Tool-call budget

Maximum delegate tool calls: 35

Stop when this number is reached. List anything the budget did not allow you
to check.

## Page-size ceiling

Maximum page size: 4600 words

Cut a draft that exceeds this ceiling. If cutting would make the page
incomplete, stop and propose a split instead.

## Fresh session

This revision runs in a new delegate session. Use the prior page and critique
only through the context pack above. Never resume a delegate from an earlier
revision.

## Page artifact and return shape

Write the page to: the one file named under "Your output" above this request, starting from the
current version given there. It is published as
`/Users/wido/LocalStorage/GitHub/agentic-tools-m1i/metasystem/plans/designs/rosters-are-configuration-items.md`.

Return only these two lines:

```text
/Users/wido/LocalStorage/GitHub/agentic-tools-m1i/metasystem/plans/designs/rosters-are-configuration-items.md
DESIGN: ready (N words, with N the page's word count)
```

or:

```text
/Users/wido/LocalStorage/GitHub/agentic-tools-m1i/metasystem/plans/designs/rosters-are-configuration-items.md
DESIGN: blocked (the reason in one line)
```

When blocked, stop and return `BLOCKED:` with the refusal's first line, what you tried and the decision needed; your seat asks.
