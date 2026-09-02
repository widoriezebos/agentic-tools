# Audit: what stops code without design, and design without critique, from being pushed (2026-09-02, m1b)

Wido's question (relayed through m1's setup words, 2026-09-02): "coding
without design critique, and worse, coding without design. The
metasystem should have a hard guard against this — it must require
evidence of this having happened by the various roles before anything
can be pushed. What is in place, what is not in place, and what must be
on the backlog (well described) to get this in place?"

Author: m1b (dispatch-delegate seat, Claude Fable 5.1), reading the
tree at origin/main 25782041 with the engine built from it. Every claim
below says how it was established: RAN (executed on this machine),
READ (the cited lines), or INFERRED (reasoned from the two).

## The verdict in one paragraph

The ladder Wido ordered on 2026-09-01 (backlog item → design → design
critique → build → code critique → tests, ruling R-38-m2) is enforced by
machinery at exactly ONE rung and only for work that travels the
delegated-chain path: a DESIGN-BEARING or DESTRUCTIVE-REACH chain cannot
CLOSE without an independent critique of its final implementation
round. Nothing mechanical proves that a design existed before the build
was dispatched, nothing proves the design was critiqued, and the
landing boundary (commit → push) consumes chain closure only in
observe mode except for two declaration-shape codes. Since the landing
gate went live at 02:40 today, 24 seat landings were stamped; 23 of
them carry a would-refuse verdict and landed anyway. One further gap is
not on any goal: the commit gate's agent/human distinction rests on
process classification, and an unannounced agent session — this
session, right now — is classified as a worker and takes the human
branch, where no landing refusal applies.

## A. What is ENFORCED (machinery refuses)

A1. Chain closure demands an independent critique for the two stricter
hazard classes. READ internal/dispatch/hazard.go:165-214
(validateHazardCompletion) and :261-302
(validateIndependentCritiqueReference). A chain whose members carry
destructiveReach DESIGN-BEARING or DESTRUCTIVE-REACH cannot close
unless the root record's independentCritiqueJobRef names a completed
job (:266-269) with role code-critic, design-critic, or warden
(:273-276), that reviews exactly the final work round (:277-279), was a
fresh-context dispatch in a distinct session (:280-286), ran at the
class's maximal critic effort on a runtime that proves it (:287-296),
and ended at or after the final round (:297-300). DESTRUCTIVE-REACH
additionally needs a verifier's live-proof job over the final round
(:304-327). The MECHANICAL class needs neither (:37-41, :196-198).

A2. The hazard table cannot be weakened one-sidedly. READ hazard.go:
57-76: the role-packet table must equal the engine's compiled minimums
row for row or admission refuses.

A3. The landing gate refuses two declaration shapes for agent commits.
READ scripts/agents/commit.sh:297-334 (the landing observe call and
the refusal branch) and scripts/agents/landing-promotion.json (refuse
codes: missing-declaration, conflicting-declarations). An agent commit
that declares neither --chain nor --direct-fix, or declares
contradictory things, refuses (internal/landing/observe.go:74-85 for
the verdicts; promotion.go:25-40 promotes only the listed codes to
Mode=refuse; commit.sh:319 applies Mode=refuse only when
agent_commit=1). Landed 4a351338 by m0 under Wido's verbatim
narrow-promotion order R-40-m0.

A4. A closed-chain landing must carry the chain's certified bytes
unchanged. READ observe.go:106-153 (observeChain) and :336-383
(bindCertifiedChange): with --chain, the chain root must be an
implementer chain (:121-123) of a stricter class (:124-127), closed
(:128-130), and the landing tree's changes over the certified paths
must equal the critic-reviewed patch applied to the current base
(:350-367); any extra path outside the certified set needs the
register-carriage declaration (:140-142). THIS IS OBSERVE-ONLY TODAY
(see B1) — the machinery exists, the refusal bit is not promoted.

A5. Direct fixes have a floor and only two classes. READ observe.go:
683-704 (register-carriage, exact-revert; anything else is
unknown-direct-fix-class) and :795-816 (neverDirectFix: AGENTS.md,
CLAUDE.md, wow.md, metasystem.conf, go.mod/go.sum, bin/metasystem, and
every path under .claude/ .github/ cmd/ docs/ internal/ memory/ plans/
records/ scripts/ skills/ templates/ …). The register-carriage allowlist
(scripts/agents/register-carriage-paths.txt) is four entries: the
narrator digest, rulings, receipts, and plans/handoff-*.md. OBSERVE-ONLY
TODAY (see B1).

A6. Commit provenance is stamped on every wrapper commit. READ
commit.sh:363-365: trailers Machine, Landing-Provenance,
Landing-Provenance-Verdict travel with each commit made through the
wrapper. RAN: `git log 4a351338..HEAD` shows them.

## B. What is OBSERVED, NOT ENFORCED (machinery records a verdict, then lets it through)

B1. Every landing verdict except the two shapes in A3. READ
promotion.go:90-117 lists 21 verdict codes; the promotion record
promotes 2. So today an agent may land with --chain naming an OPEN
chain (chain-open), a MECHANICAL chain (chain-not-design-bearing), a
chain whose certified bytes differ from the landing
(chain-output-mismatch), or --direct-fix register-carriage over code
and docs (direct-fix-floor-refused): each stamps would-refuse and the
commit proceeds.

B2. The observation window, measured. RAN on origin/main at 25782041:

| since the gate landed (4a351338, 02:40 today) | commits |
|---|---|
| total on main | 52 |
| goal-verb commits (engine-made, no wrapper, no trailer) | 28 |
| wrapper commits, stamped | 24 |
| of which pass (bar b, register carriage) | 1 |
| would-refuse code=direct-fix-floor-refused | 15 |
| would-refuse code=chain-open | 4 |
| would-refuse code=unknown-direct-fix-class | 2 |
| would-refuse code=missing-declaration (a PROMOTED code) | 2 |

Read plainly: if the full bar were live today, 23 of the 24 seat
landings since 02:40 would have refused. The 15 floor refusals are
records, briefs, and designs landed under a register-carriage
declaration that the allowlist does not cover (plans/*-brief.md,
plans/*-design.md, records/*). The 4 chain-open landings declared a
chain that had not closed. INFERRED: the seats are declaring in good
faith against a bar they cannot yet satisfy, which is exactly the
observation the promotion window exists to collect; it also says the
allowlist and the "what is a lawful record landing" question must be
settled before promotion, or promotion halts every seat.

B3. Two commits carry a PROMOTED refuse code and still landed:
d081ef07 (m1, metasystem.conf change, "The Fable lane moves to five
point one", 09:15 today) and 8a9d3296 (m0b, records, 02:43 today).
Both were made after the gate landed. The refusal applies only on the
agent branch of commit.sh (agent_commit=1) and only when the base tree
at commit time already carries the promotion record (promotion.go:
42-54 reads it from HEAD). INFERRED: each landing either took the human
branch (C1) or was committed on a checkout whose HEAD predated the gate
and was then rebased over it by land.sh — the commit alone cannot say
which; both paths are open today and neither is on a goal.

## C. What is CONDUCT-ONLY or ABSENT (nothing mechanical)

C1. THE HUMAN BRANCH IS TAKEN BY UNANNOUNCED AGENT SESSIONS. Not on any
goal. READ commit.sh:8-17: the wrapper asks `lease require-holder`; if
the answer carries no claimEpoch it re-executes itself as
`__lease-held human`, and commit.sh:23-30 then sets agent_commit=0 —
the landing refusal at :319 never fires. READ internal/lease/verbs.go
RequireHolderAt (:359-369): a caller classified DELEGATE returns
Holder:false with NO error and no epoch (:369); gateHolder (:480-482) passes
ClassDelegate outright (`case ClassHuman, ClassDelegate,
ClassAdapterSupervisor, ClassSupervision: return nil`). READ
internal/lease/classify.go:341-346: a process whose ancestor is a
runtime adapter (a `claude` or `codex` process by signature) with no
authenticated main announcement is DELEGATE. RAN from this session:
`bin/metasystem lease require-holder --root . --caller-pid $$` →
`{"claimEpoch":null,"class":"DELEGATE","holder":false}` exit 0. This
session is unannounced because `metasystem up` aborts at the
accepted-engine check (internal/up/up.go:394-396, ENROLLMENT_DRIFT: no
steward identity on this machine) before it reaches
session-announcement (:437-446). Consequence: any agent session on a
machine whose steward is not armed, and any delegate worker with a
shell, commits through the wrapper on the HUMAN branch: every landing
verdict is stamped observe-only, including the two promoted codes.
The same DELEGATE class also passes the dispatch door's lease check
(dispatch.sh:237-244 reads the class and never the holder bit;
verbs.go:365-369 returns success for DELEGATE); what actually stops a
dispatch on an unarmed machine is the census check
(dispatch.sh:137-143: no supervision verdict → refuse), and `up` arms
supervision only after the steward check passes.
INFERRED: this is the most likely mechanism behind B3; not proven for
those two commits (their machines' announcement state at the time is
not recorded).

C2. A design must exist before a build is dispatched. ABSENT. The
dispatch door admits an implementer chain on the dispatcher's declared
hazard class and a goal reference; nothing checks for a design
artifact, a closed design chain, or a critique of that design. Goal
design-gate-at-dispatch (queued, budget 1d/6/240m/1) is the backlog
item; goal manifest-floor-at-dispatch (queued, 4h/6/240m/1) covers the
sibling hole that the class itself is self-declared (hazard.go admits
on `member.record["destructiveReach"]`, :179, with no cross-check
against what the work touches). READ the dispatch door: `metasystem delegate` requires the FLAGS
--role, --brief, --goal, --destructive-reach
(cmd/metasystem/delegate.go:361-362), but `--goal none-explicit` is
accepted and stripped before dispatch.sh ever sees a goal
(delegate.go:302-304), and dispatch.sh treats the goal as optional
(scripts/agents/dispatch.sh:1202, :1327). The hazard class is the
dispatcher's own flag, checked only for enum membership
(delegate.go:344-345; dispatch.sh:1201) — and a fixture environment
variable may supply it (dispatch.sh:1200,
METASYSTEM_DISPATCH_FIXTURE_HAZARD). Nothing in internal/dispatch,
delegate.go, or dispatch.sh mentions a design reference, a certified
artifact, or a waiver (RAN: grep). The brief-authority preflight
(dispatch.sh:1348 → internal/dispatch/brief.go:62) proves only that
repository paths a brief names EXIST in the delegate's base tree
(refusal text brief.go:33-35); it proves nothing about their content.

C3. A design was critiqued before the build. ABSENT, and today a design
chain cannot even close lawfully: goal design-critique-chain-binding
(queued, 4h/6/240m/1) records that the two binding doors (--reviews at
dispatch, --reconcile-evidence at close) refuse the design-critic role
while the closure gate accepts it (hazard.go:274). READ the three surfaces: `--reviews` refuses every role but
code-critic, warden, verifier at both doors (delegate.go:364-366;
dispatch.sh:1237-1238); at close, `--reconcile-evidence` sends
design-critic to the default arm and refuses the binding
(internal/dispatch/review_reference.go:83-96); yet the closure gate
requires the critic's `reviews` field to equal the final work round
(hazard.go:277-279). So the design-critic acceptance at hazard.go:274
is dead in practice: no design-critic job can ever carry the binding
the gate demands. There is no
role named designer in scripts/agents/role-packets.json (roles:
behavior-judge, code-critic, design-critic, implementer, investigator,
steward-continuation, verifier, warden); designs are authored by an
implementer-role chain whose brief says "write one design file" (RAN:
plans/fable-5-1-rollover-design-brief.md, Working Mode: design). So a
"certified design" has no machine identity today: no schema, no
return field, no record that a build brief could cite and a gate could
verify. What DOES exist as raw material: the design-critic return schema
requires a machine-readable verdictMaterialCount and a boolean
`material` per finding (scripts/agents/schemas/design-critic.schema.json),
the per-chain finding register folds critic rounds
(internal/dispatch/finding_register.go:18-34), and the chain root
carries independentCritiqueJobRef (review_reference.go:9-10). A "design
chain closed at zero material findings" is therefore OBSERVABLE but is
never minted as an identity, never named on a build record, never
checked. The ladder itself lives only in prose
(docs/orchestration.md:27-37; skills/design-critique/SKILL.md).

C4. A commit is bound to a backlog item. ABSENT at the commit boundary.
Goal commit-goal-binding (queued, 1d/6/240m/1) specifies the Goal-Item
trailer verified against the ledger, the enumerated exemptions, and the
R-39-m2 single-use waiver for unbound changes. Today the ladder's first
rung is conduct (R-38-m2, R-39-m2).

C5. MECHANICAL work needs no critique at all. By design of the hazard
table (hazard.go:37-41); Wido's 2026-09-01 order retires the exemption
via goal critique-always (queued, 4h/6/240m/1).

C6. Landing consumes the design's provenance. ABSENT. Bar (a) proves the
landing carries a CLOSED IMPLEMENTATION chain's certified bytes
(observe.go:106-153); it does not and cannot prove the implementation
chain was itself admitted against a critiqued design, because C2 is
absent and the admission record carries no such field.

C7. After-the-fact audit of what reached main. ABSENT. READ internal/steward/health.go:43-58: no health role reads git
history (RAN: grep for `git log`/`rev-list` over internal/steward and
internal/audit → none); the closest, claimed-goal-delivery, reads
memory/receipts.log (delivery.go:85). RAN: grep for the trailer names
over all Go and shell → exactly three files: commit.sh (the writer),
static-reproof-fixtures.sh and observe_test.go (tests). No reader
exists; a would-refuse verdict is minted and never looked at again. No .github
directory exists in this tree (RAN: ls); no CI, pre-push, or
server-side check examines trailers on origin. A plain `git push`
outside land.sh lands anything. The one git hook the engine can install
is a PRE-COMMIT guard against unread plan files, enrolled on the first
goal-ledger mutation (cmd/metasystem/goalsync_verbs.go:36-50 around
scripts/agents/pre-commit-guard.sh); it is not enrolled in this clone
(RAN: .git/hooks holds only samples) and it is never pre-push or
pre-receive. The goal-verb commits (28 of 52 today)
bypass the wrapper by design (they are ledger records) (READ internal/goal/txn.go:282-284: the ledger commits with
`commit-tree` and publishes by ref update, :353-367; its only trailer
is Goal-Transaction) and
carry no provenance trailer, which is lawful but means "every commit
has a stamped verdict" is not a property of main today.

C8. Human commits are sovereign by design (commit.sh:288-291 comment,
observe.go:59-61). That is correct — Wido's own commits should not be
gated by the agents' bar — but it makes C1 the whole game: the bar is
only as strong as the proof that the committer is not an agent.

C9. The seat governance record's two open items remain conduct-only:
the dispatch hand also presses the landing button (remedy: the
two-bars bar going live), and narrator plus acceptor sit in one actor
(remedy awaits Wido at the 2026-11-30 review). READ
plans/seat-governance-record.md, activated R-30-m1.

## D. What must be on the backlog (extend existing goals; nothing new duplicates them)

The backlog already carries the ladder's teeth as five queued goals:
design-gate-at-dispatch (dispatch refuses a stricter-class build without
a certified design reference; retires --goal none-explicit),
manifest-floor-at-dispatch (the class cannot be understated),
critique-always (MECHANICAL loses its critique exemption),
design-critique-chain-binding (a design chain can close), and
commit-goal-binding (every commit names its backlog item; the R-39
waiver). two-bars-for-changes holds the landing bar's promotion. What
the audit adds are FOUR extensions and ONE sequencing fact, written
below in the form the goals use (hazard, mechanism, refusal shape,
what a builder starts from), to be applied with `goal edit`:

D1. two-bars-for-changes — new slice "the refuse bit binds on the
caller's class, not on the lease's epoch". HAZARD: C1 — an unannounced
agent session or a worker with a shell commits on the human branch and
no landing refusal applies; two promoted-code landings today are
consistent with it. MECHANISM: commit.sh's branch choice consults the
caller CLASS from `lease require-holder` (HUMAN → human branch; MAIN
with epoch → agent branch; DELEGATE, SUPERVISION, ADAPTER-SUPERVISOR,
STEWARD, UNTRUSTED → refuse to commit at all, naming the class and the
lawful path: announce the main via `metasystem up`, or commit from a
person's terminal). Engine side: RequireHolder keeps reporting the
class; the wrapper stops collapsing "no epoch" into "human".
REFUSAL SHAPE: "commit refused: caller is DELEGATE, not a person or an
announced main; run metasystem up from the session (steward armed) or
commit from a human terminal". TEST: fixture callers of each class —
HUMAN commits without gate; MAIN commits under the gate; DELEGATE
refuses; the existing epoch-changed refusal still holds. BUILDER STARTS
FROM: commit.sh:8-30, internal/lease/verbs.go RequireHolderAt and
gateHolder (:480-482), classify.go:341-346, the static-reproof fixtures that
already stage callers. Appetite: one 4h slice.

D2. two-bars-for-changes — promotion evidence and the precondition for
the full bar. FACT for Wido's review: 23 of 24 stamped landings since
02:40 would refuse under the full bar (table in B2); 15 are record and
brief landings the register-carriage allowlist does not cover. BEFORE
promoting any further code, the lawful landing path for records, briefs,
designs, and critique records must be named: either the allowlist grows
to the record trees (records/**, plans/*-brief.md, plans/*-design.md,
memory/known-issues.md, memory/backlog-notes.md — append-or-new-file
rules, never edits to instruction files) or those landings ride the
chain that produced them. Whichever, the verdict stream must reach
mostly-pass on real landings for a window before chain-open and
direct-fix-floor-refused are promoted, or promotion stops the fleet on
day one. REFUSAL SHAPE once promoted: unchanged from observe.go's codes.
BUILDER STARTS FROM: landing-promotion.json (the one-line promotion),
register-carriage-paths.txt, observe.go:449-503 (registerCarriage),
:795-816 (neverDirectFix).

D3. design-gate-at-dispatch — define the certified-design reference and
sequence behind design-critique-chain-binding. HAZARD: the goal says
"the brief references a certified design artifact (a design chain
closed at zero material …)" but no such object exists (C3): design
chains cannot close, no role authors designs, no field carries a
design's identity. MECHANISM: (1) design-critique-chain-binding lands
first so a design chain (implementer role, Working Mode: design) closes
with a design-critic independentCritiqueJobRef; (2) the dispatch
request gains a typed field designChain=<root job id>; admission
verifies the named chain is closed, its critique job's role is
design-critic, and the chain's certified output paths exist in the
build's base tree at the reviewed blob ids (the same
bindCertifiedChange comparison the landing bar uses, observe.go:336-
383, run at the dispatch base); (3) the admission record persists
designChain and the critic job id so the landing bar can consume it
(D4). Human waiver: a recorded human word naming the dispatch, in the
strict-form family the register already uses. REFUSAL SHAPE:
"delegate refused: DESIGN-BEARING build names no closed design chain
(designChain missing)"; "…names chain X whose critique is not a
design-critic"; "…whose design file plans/<x>.md is not in the base
tree at the reviewed digest". TEST: the four refusals and the pass;
MECHANICAL exempt until critique-always lands, then the exemption
follows that goal's rule. BUILDER STARTS FROM: the delegate verb and
internal/dispatch admission (the goal already names role-packets.json
and hazard.go), observe.go's binding functions to reuse, the
design-critique-chain-binding change.

D4. two-bars-for-changes — bar (a) consumes the design provenance once
D3 mints it. HAZARD: C6 — a closed implementation chain proves its own
critique, not that a critiqued design preceded it. MECHANISM: after
design-gate-at-dispatch lands, observeChain additionally reads the root
record's designChain and its design-critic job id and refuses a
DESIGN-BEARING landing whose chain carries none
(chain-without-design), verifying the design chain's closure the same
way the dispatch gate did. REFUSAL SHAPE: "would-refuse
code=chain-without-design" (observe first, promote with the rest).
TEST: chain with design passes; chain admitted under a human waiver
passes with the waiver named in provenance; chain without either
refuses. Sequenced behind D3. BUILDER STARTS FROM: observe.go:106-153,
promotion.go's known-code list.

D5. commit-goal-binding slice 2 (the steward's after-the-fact sweep) —
extend the sweep to the landing trailers. HAZARD: C7 — nothing looks at
what actually reached main; a push outside the wrapper, a stale-clone
commit rebased over a newer gate, or a human-branch agent landing leaves
no alarm. MECHANISM: the same tick role that will sweep Goal-Item
trailers also sweeps each new canonical-branch commit for a
Landing-Provenance-Verdict trailer: absent (and not a goal-verb commit
by its shape) or would-refuse → an escalation episode naming commit,
machine, and code, delivered through the alert channel. TEST: synthetic
commits of each shape raise or stay silent as specified. BUILDER STARTS
FROM: internal/steward tick roles (the ruling sweep's shape,
ruling_sweep.go), the alert-escalation-channel delivery.

Everything else Wido asked for is already on the backlog in the shape
he wants (critique-always, manifest-floor-at-dispatch,
gate-governance-records for the discipline around the new gates). The
dependency order, so the seats do not build on air:
design-critique-chain-binding → design-gate-at-dispatch (+ D3) →
two-bars D4; critique-always and manifest-floor-at-dispatch are
independent of those; D1 and D5 are independent of everything and D1 is
the cheapest real teeth available today.

## E. What I need from Wido

E1. The steward arm word for machine m1b. Without it `metasystem up`
stops at ENROLLMENT_DRIFT before arming supervision or announcing this
session, so: no census verdict → every dispatch refuses
(dispatch.sh:137-143), and this session stays class DELEGATE, which —
per C1 — commits on the human branch where the gate does not apply.
Executing backlog work from this seat waits on this word. The command I will run with his verbatim words:
`bin/metasystem steward arm --repo . --temporary-human-word "<his
words>" --review-by 2026-09-06` (the R-29-m2 mechanized path every
other seat used; re-ratified at his terminal session).

E2. Budget tuples for the extensions above are his word (m1's setup
instruction); the parent goals already carry tuples set under the
standing R-44-m0b/R-45-m0b law (small 4h/10/240m/1), which the seats
have been applying. If he prefers those to stand, no new word is
needed; if he wants D1 pulled forward as its own claim, a tuple for it.

E3. His read on D2 before any further promotion: grow the allowlist to
the record trees, or make record landings ride their chains. The
15-of-24 number says promotion without that answer stops every seat.
