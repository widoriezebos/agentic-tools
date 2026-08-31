# The counselor — high-level design

Ratified in conversation (Wido, 2026-08-24): the name, the seat, and
the shape below. Detailed design per slice runs its own critique chain
at claim time; this note is the standing direction.

## The problem the seat exists for

The generic metasystem is learn-once because it custodies itself. The
app-specific net cannot be learn-once, because its subject moves: a
guardrail is a claim about the app as it was when the guardrail was
written, and every concluded goal makes the app slightly different from
the app the net was built to bound. Nothing measures that gap today —
the human is supposed to notice it, with no instrument for noticing.
The counselor is the instrument, wrapped in a person-shaped
conversation.

## The seat, by contrast with its siblings

- The WARDEN is adversarial and transactional: it judges changes to the
  net when the net is touched, and has no opinion between transactions.
- The STEWARD is mechanical and ambient: it watches liveness
  continuously, speaks in noticings, never judges content.
- The COUNSELOR is advisory and ambient, ABOUT CONTENT: it holds a
  standing model of the gap between what the app is becoming and what
  the net actually bounds, and it speaks to the human — in briefs and
  sittings, never refusals.

Like the warden, the counselor has NO PEN on the net: it proposes, the
human rules, and every change it argues for still flows through the
existing lanes — the warden's lane for net changes, the human tier for
identity and battery. The counselor adds judgment without adding
authority; the custody architecture is unchanged by its existence.

## The two-part split

- GENERIC (metasystem-owned, freely upgraded): the counselor ENGINE —
  the drift analyses, the brief format, the sitting's interview
  pattern. When the metasystem upgrades, the counselor gets smarter.
- APP-SPECIFIC (app-owned, never overwritten, at worst migrated): the
  counselor's MEMORY — the living evidence table
  (`docs/covenant-evidence.md`, born at inception), the accepted-risk
  register, the human's past rulings, the margin history. What the
  counselor knows about THIS app survives every upgrade untouched.

Custody of the memory files (RULED, slice-one design chain, codex
r2-F1, Wido's rulings recorded on the goal 2026-08-24): the evidence
table is GATE-DEFINING INPUT — the traceability gate takes proof
identity and declared dependencies from it — so it stays
guardrail-classed with the covenant, exactly as the inception closure
law lands it; a doctored table could otherwise make the gate greener
than the tree allows, and the warden's lane, not file-presence
checking, is the laundering defense. The counselor still has no pen:
its bookkeeping is proposals the human lands through the existing
lanes. The accepted-risk register's custody stays UNDECIDED until it
has a format and a consumer (slice 3).

## The six drift signals

1. INTENT THAT ESCAPED THE COVENANT — the diff between the goal ledger
   (what was built) and the requirements table (what is guaranteed). A
   concluded goal with no covenant row is a feature that entered the
   app without entering the contract. Both sides are structured data
   already custodied; this is the highest-value, cheapest signal.
2. PROOFS DRIFTING OFF THEIR SUBJECTS — a requirement's proof still
   runs and passes, but the code implementing the requirement has moved
   and the proof no longer touches it. "Observed" in the evidence table
   must mean observed RECENTLY against the CURRENT tree.
3. GOLDEN STALENESS AND TOOTHLESSNESS — a golden unchanged while the
   feature it pins changed N times is probably stale; a golden that has
   never failed may guard nothing. The second needs the mutation-probe
   idea (break the thing deliberately, confirm the golden screams).
4. THRESHOLD DRIFT — a battery passing with ever-widening margin is a
   ratchet that should have moved; one scraping the line every run is
   honest tension or noise. Trend data over mission history.
5. IDENTITY COVERAGE — sourcePaths says where the app lives; recent
   missions' accepted trees say where code actually landed. Code
   growing outside the declared identity is a blind spot forming.
6. ACCEPTED-RISK AGING — the things the human deliberately chose not to
   guard, recorded with the why and the assumptions. The counselor
   re-surfaces a risk when its recorded basis moves, instead of every
   review re-litigating all of them.

## The two rhythms

- AMBIENT: one-line noticings on the steward's existing durable notify
  channel, sparse — the nudge, not the lecture.
- THE SITTING: a periodic net review on the human's seat, the same
  conversational pattern as the inception interview, triggered by
  cadence (every N concluded goals, or on demand). The counselor
  arrives with a prepared brief — the ledger-vs-covenant diff since
  last sitting, margin trends, staleness, floating proofs — and the
  human rules line by line, display-then-confirm. Inception is the
  first sitting; the counselor owns every sitting after.

## Relations to the standing program

- INCEPTION births the counselor's memory: the evidence table is the
  seed, authored with the human at the moment the game becomes
  playable.
- GUARDRAIL-ADEQUACY is the counselor's mechanical gates: its
  traceability gate folds into slice one below; its mutation-probe
  remainder stays its own later work under the counselor's roof.
- GUARDRAILS-FIRST-EVOLUTION is the counselor's doctrine: the net grows
  BEFORE the feature — at intake, the counselor's question is "what
  does the net need to grow to bound this goal?".
- APP-DOCTRINE's evolution loop is the design-side sibling; doctrine
  staleness may become a seventh signal when that loop is built.

## The carving (each slice independently deployable)

1. THE LIVING EVIDENCE TABLE + TRACEABILITY GATE — re-derive the
   inception-born table against the current tree on demand: every
   covenant requirement backed by a matching table row (one identity
   namespace), every DECLARED dependency present and symlink-safe,
   and dishonest statuses refused (a covenant-backed planned-floating
   row is intent guaranteed by nothing). Recorded statuses are
   carried as claims, labeled unverified. Folds guardrail-adequacy
   slice one. (Amended 2026-08-24 with Wido's confirmation: the
   original "statuses recomputed" — proving "observed" happened
   recently against the current tree — needs observation metadata
   (tree digest, timestamp, run evidence) that nothing records yet;
   it moves to a later named slice under this roof, alongside signal
   2's machinery.)
2. THE DRIFT BRIEF — signals 1, 4, and 5 composed into one
   plain-English document on demand: the ledger-vs-covenant diff,
   margin trends, identity coverage.
3. THE SITTING — the net-review interview skill (inception's pattern,
   reused) plus the accepted-risk register (app-owned, signal 6).
4. AMBIENT NOTICINGS — the counselor's nudges on the steward channel,
   drawing from the brief machinery.

Signals 2 and 3 (proof-subject binding, golden probes) are later
slices under the same roof — they need the mutation-probe machinery
and are deliberately not in the first four.

## The responsibility matrix (reviewed with the counselor added; Wido 2026-08-24)

Every seat and role, what it owns, and the boundary the counselor's
arrival makes explicit:

- THE HUMAN — the top custody tier. Owns: rulings, conclusions of
  human-origin goals, taint resolutions, identity and battery changes,
  the seal. Nothing below may absorb any of these.
- THE SEAT (dispatch delegate + custodial mechanics) — the human-facing chair. Owns: backlog order,
  appetites, intake, the interview CHAIRS (inception and the
  counselor's sittings run ON this seat), landing discipline. The
  counselor's absorbed doctrine adds ONE question to the dispatch delegate's
  intake: "what does the net need to grow to bound this goal?" — the
  counselor owns the question's content, the dispatch delegate asks it.
- THE COUNSELOR — advisory, ambient, about content. Owns: the drift
  model, the brief, the evidence table's rederivation, the
  accepted-risk register, the sitting's substance. Boundary: never
  judges liveness (the steward's), never adjudicates changes (the
  warden's), never rules (the human's), never holds the chair (the
  human's). Brief PREPARATION may run as a dispatched job; the
  sitting itself is the human's seat channeling the counselor's
  brief to the human.
- THE WARDEN — adversarial, transactional, about changes to the net.
  Owns: the review lane for guardrail-classed changes. Boundary with
  the counselor: the counselor proposes a net change; once the human
  accepts and work begins, the RESULTING change is the warden's to
  judge like any other — proposal provenance earns no leniency.
- THE STEWARD — mechanical, ambient, about liveness. Owns: the
  patience clocks, revival, the durable notify channel. The counselor's
  noticings RIDE the steward's channel but the steward never composes
  them — it carries content-blind envelopes either way.
- THE NARRATOR — the running plain-English account of now. Speaks for
  the machine's day; the counselor speaks for the net's health. Two
  voices, one channel discipline, no overlap of subject.
- MISSION-DISPATCH ROLES (implementer, code-critic, design-critic,
  verifier, investigator, behavior-judge, steward-continuation) —
  unchanged: they live inside missions; the counselor lives outside
  them. The counselor's findings become goals, and goals become
  missions those roles serve.
