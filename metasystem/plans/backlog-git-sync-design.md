# The multi-machine backlog: git-synced, claimed, dependency-aware

STATUS: CONVERGED at round 10 (2026-08-19, D119) — the declared
failsafe round returned "no demonstrated D115 failure and no
non-arbitrable architecture defect"; its eight fixture-expressible
gaps are folded into the obligation matrix below, and implementation
proceeds against that matrix with fixtures as the arbiter. The
critique record is scratchpad bgs-design-r1..r10; the scope
reduction is D118. Goal backlog-git-sync (D115 + addenda, D118).

## Design inputs (observed live, 2026-08-19)

1. The single-file ledger cannot merge; its digest baseline refuses the
   recovering merge — yet enabled byte-exact recovery once. Keep the
   properties, change the granularity.
2. Cross-machine gaps bite within hours.
3. Ledger ergonomics force workarounds; workarounds cause incidents.
4. Operator requirements: a cross-machine pause lever; a browsable
   split between living backlog and finished work.

## The model

### Files

`plans/goals/<id>.md` — one live goal per file: today's fields plus
`State: queued | claimed | parked`, `OpenedAt`, `Revision`,
`BlockedBy`, `Claimed: machine= lineage= at=`, `Parked: by= at=
because= [displaced=...]`, `LegacyNotes:` (verbatim passthrough of
non-field prose the legacy parser accepted — semantic-lossless
migration with recorded residue), `History:` (append-only; EVERY verb
appends one line carrying its operation id — the opid line is each
mutation's durable, operation-unique evidence), and a trailing
`Integrity: sha256=...`.

Goal ids are filenames: lowercase kebab, max 100 characters — the
named invariant is portable-filesystem safety (255-byte component
bound with headroom for suffixes and copies), not taste.

PINNED SCHEMA SEMANTICS (R4-10): `Revision` is an unsigned integer,
1 at creation, incremented by exactly 1 by every verb write to that
file (a move without content change still increments — the History
append IS a content change). `Integrity` = sha256 over the file's
bytes above the Integrity line, LF-normalized — that is the entire
canonical byte domain, no other normalization. A History line's
grammar: `- <iso8601> <opid> <verb> actor=<machine>+<lineage>|human:<name>
[targets=<ids>] [displaced=<machine>+<lineage>@<claimedAt>] [ack]
[keep=<n>] [reason=<rest of line>]` — one line, fields
space-separated, exactly these keys, `keep=` used only by prune's
root-record line (R9-12), `reason=` (used by park) always LAST and
consuming the remainder of the line (lossless free text without
quoting rules);
the validator and recovery parse THIS grammar and nothing looser.
OPIDs (R4-04) are `<ulid>-<machine>-<lineage-hash8>`: globally unique
by construction and source-attributed. The opid attributes EXECUTION
(the clone's machine and current lineage, always — R7-14); the
History actor field attributes AUTHORITY (`human:<name>` when a
human directed the verb through that clone). Every transaction
commit carries ONE provenance trailer, `Goal-Transaction: <opid>` —
it serves the writer's crash-recovery postcondition and debugging,
and enforces nothing (D118: state is guarded by validation and
integrity digests, history is not authenticated). One opid appears
in exactly one commit; the same opid across the several files one
transaction touches is that one commit's lawful footprint. Golden records (one claimed goal file, one root
record, one archived goal) ship as fixtures with the implementation
and are normative.

`plans/goals/done/<id>.md` — the archive. `goal done` moves the file
here in its transaction. An archived file carries `State: done`
explicitly; placement and State must agree, and the validator
refuses divergence (path never silently implies state — R7-14). Done is terminal BY DEFAULT; `goal reopen` is
the explicit exception that moves a file back (each direction is a
once-per-transition rename; the no-textual-merge argument below covers
both). The living folder is to-do, in-work, or paused; the archive is
the record; `goal list --done` renders it. The operator weighed a
list-verb-only alternative and delegated; the move stays because the
canonical branch never textually merges (every mutation rebuilds on
the fetched tip and publishes by compare-and-swap; a
concluded-vs-edited race resolves as a transition-table refusal on the
rebuilt tip), hand edits are the mediated reconcile path, and the live
directory stays bounded over years.

`plans/goals/backlog.md` — the ROOT RECORD: adoption identity, format
version, migration epoch + manifest digest + migration mode, the
SYNC MODE (`remote` or `local`, written once at migration/adoption —
R6-08), and the Goal-free declaration. Every operation compares the
clone's `goal.sync-remote` config against the committed sync mode
and refuses on mismatch by name: flipping a local-mode clone's config
to a remote is the forbidden promotion (the refusal names
backlog-local-promotion); pointing a remote-mode clone at `local`
is a split-brain risk refused the same way. The adoption identity
is a ULID minted ONCE at migration/adoption and never rewritten
(R8-08 — a stored literal, not a derivation, so benign remote or
path renames cannot change it); it is also the LEDGER IDENTITY
(R7-12): every fetch
compares the fetched tree's root-record identity against the
accepted tree's and refuses a mismatch as a foreign ledger by name —
so re-pointing config at a different remote or branch cannot
silently select another ledger, whatever the config strings say
(the invariant is "same ledger", bound semantically, not "same
config", which would break on benign remote renames). Config cannot
silently change what the ledger is.

Order derives from (OpenedAt, id); OpenedAt written once at open.
goals.md and goals-accepted.json are deleted at migration; no index
file; list/next render from the canonical projection.

### The synchronization endpoint

`goal.sync-remote=<remote-name>` (default origin) and
`goal.sync-branch=<fully-qualified-ref>` (default refs/heads/main).
Fetch: `git fetch <remote> +<branch>:refs/metasystem/goals/fetch/<opid>`
— a PER-OPERATION ref (R4-03): concurrent fetches in one clone can
never rewind a shared ref out from under each other; the operation
captures the fetched object id once and uses it everywhere; the ref is
deleted at the operation's terminal phase. Reads without an operation
use an ephemeral ref the same way.
Push: `git push <remote> --force-with-lease=<branch>:<fetched-tip-oid>
<txn-commit>:<branch>` — force-with-lease with the EXPLICIT expected
object id IS the compare-and-swap (R3-02); no plain push exists in the
protocol. SINGLE-MACHINE MODE (R3-12, R4-06): `goal.sync-remote=local` is a
declared mode for remote-less repositories: transactions CAS (update-ref
with old-value assertion) against a DEDICATED local ledger branch,
`refs/heads/metasystem/goals` — never the user's checked-out branch, so
HEAD provably never moves in either mode (in remote mode only the
REMOTE branch advances; the local branch waits for an ordinary pull).
Multi-machine guarantees are documented void in local mode; `goal list`
banners it. PROMOTION to a remote is DESCOPED from this build
(R5-06): a sound promotion protocol needs its own case table (absent,
equal, remote-ancestor, local-ancestor, divergent over captured
oids), crash ordering across push, root record, accepted ref, and
config, and its own fixtures — a sketch of it is worse than its
absence. Local mode is therefore terminal in v1: joining a fleet from
a local-mode repository is the follow-up goal the migration manifest
opens (backlog-local-promotion), and `goal list`'s local-mode banner
names that goal. Any promotion attempt refuses toward it.

### The ledger transaction

Every mutation — open, claim, release, done, park, unpark, reopen,
edit, prune, declare-free (and its idempotent renewal),
reconcile-accept, migrate — publishes through one transaction:

1. Durable journal entry first (`artifacts/agents/goal-transactions/
   <opid>.json`) — the machine-local crash record, three phases only
   (D118 collapsed the nine-phase machine; R7-10's dead-end
   transition dies with it):

   | Phase | Meaning | Durable fields | Blocks own-clone mutations? |
   |---|---|---|---|
   | created | intent recorded, nothing pushed | opid, machine+lineage, owner process identity (ticks+bootId), and the COMPLETE NORMALIZED COMMAND INTENT — verb, targets, and every argument (reason, conclusion, edit deltas, arc changes, keep count) — enough to rebuild without the original process (R8-03); later the fetched oid and txn commit oid as steps complete | no |
   | pushed | a push left this process; outcome not yet known | expected old tip oid, attempt count, deadline stamp | YES until terminal |
   | terminal | confirmed / confirmed-late / lost / abandoned / rejected / expired, with the evidence (canonical tip, winner's opid, refusal by name, or deadline) | outcome + evidence | no |

   These three phase names are the ONLY journal vocabulary anywhere
   in this design (R8-04). Recovery is ONE rule for every
   non-terminal entry: refetch and evaluate the opid postcondition —
   present = confirmed (confirmed-late when the entry had already
   been terminalized); a competitor's win = lost. When the
   postcondition is absent, the OWNER'S LIVENESS decides (R10-M04,
   consistent with dead-owner-completes below): a DEAD owner's entry
   is COMPLETED from its stored intent, created and pushed alike;
   only the LIVE owner itself abandons its own never-pushed work or
   expires its own retry loop at the deadline. LIVENESS RULES
   (R8-05): journal transitions
   are serialized under a per-clone lock and are monotonic (created →
   pushed → terminal, never backward); a process other than the
   owner touches an entry only when the owner is provably dead by
   the shipped process-identity check (ticks+bootId). RECOVERY
   COMPLETES, IT DOES NOT KILL (R9-07): a dead owner's non-terminal
   entry holds the complete stored intent, so the recovering process
   takes ownership under the lock and FINISHES the operation —
   fresh fetch, rebuild, push — rather than expiring it; the opid
   postcondition resolves identically whether the original delayed
   push or the rebuilt one lands (same opid, same intent), so
   "remote still at the expected tip" is never a wedge — recovery
   advances it. Terminals that remain: a rebuild whose verb refuses
   on the new tip = REJECTED by name; expiry applies only to a LIVE
   owner's own deadline (benign-advancement retries stop at
   goal.publish-deadline and report). TERMINALS ARE BELIEFS, THE
   OPID IS THE TRUTH (R9-08): the sanctioned force-push repair path
   means the branch can lawfully revisit an old tip, so a
   pre-rewind push can land AFTER an entry was terminalized; any
   later interaction that sees the entry's opid in canonical
   history corrects the entry to confirmed-late and reports it —
   safety never rested on the journal (tree validation gates every
   acceptance); the journal is the clone's best account, kept
   honest by evidence. The repair guidance tells the human
   performing branch surgery exactly this: pending fleet operations
   may land late, and validation, not the journal, is the gate.
   Phase writes are durable before the action they describe;
   terminal entries clean the fetch and txn refs.
2. Fetch the canonical ref (or read the local branch in single-machine
   mode).
3. Build in an isolated index seeded from the fetched tree;
   `git commit-tree -p <fetched-tip>` (R3-02: the parent is explicit;
   the commit is a child of exactly the tip the lease will assert);
   temporary ref refs/metasystem/goals/txn/<opid>. The engine owns the
   commit via plumbing; the user's HEAD, index, and worktree never
   participate and never move.
4. Revalidate the FULL read-set on the transaction tree: target state
   and Revision, blockers, quota, graph acyclicity and referential
   integrity, Goal-free rules, the transition table.
5. Journal phase=pushed (records the txn commit oid and the
   expected old tip); push with the lease.
6. Classify by refetch, POSTCONDITION = "my opid's History line is
   present" (R3-04): every verb appends its opid to each touched
   file's History, so the predicate is operation-unique — two same-
   Revision edits cannot confuse it, and reopen/declare-free/
   reconcile/prune all carry it (declare-free and prune write theirs
   in the root record's History). TOTALITY: if the touched file was
   itself later pruned or moved, recovery walks the canonical branch
   log for the opid trailer (every transaction commit carries
   `Goal-Transaction: <opid>`); the predicate always resolves.
   Outcomes: already-true = idempotent success; same-target competitor
   = loss, report the winner; unrelated advancement = rebuild and
   retry UNTIL `goal.publish-deadline` (default 60s — a deadline, not
   a fixed count: four benign advancements in a row are lawful work,
   not failure; R3-13); definite rejection = journal
   terminal=rejected, failure by name (R9-07);
   transport-unknown = the entry STAYS pushed and this process
   stops; a later process classifies it by the one recovery rule.
7. On confirmation: CAS-advance refs/metasystem/goals/accepted
   (update-ref with old value), journal terminal=confirmed, delete
   the txn ref.

### Authority enforcement: two executable points (R3-03)

`commit-tree` runs no hooks and main carries ordinary code commits, so
the trailer alone enforces nothing. The two real enforcement points:

- USER-SIDE: the shipped pre-commit guard (which inspects the staged
  index — exactly its capability) gains one rule: staged changes under
  plans/goals/ refuse in ordinary commits — "goal files change only
  through goal verbs; hand edits go through goal reconcile". This is
  the accidental-edit fence.
- READ-SIDE (the validator every machine runs — D118): the accepted
  ref advances only onto a fetched tip whose TREE validates: every
  goal file parses against the pinned schema, its Integrity digest
  matches its bytes, placement agrees with State, the dependency
  graph is acyclic and referentially closed, claims respect quota
  and arc consistency (all live arc members one ownership state, one
  claimant), CLAIMED IMPLIES EVERY BLOCKER DONE — D115's dependency
  invariant is a full-tree predicate, not just a claim-verb guard
  (R8-02) — and the root record's identity matches the accepted
  one. Validation judges the state, not how it arose: no commit
  walk, no replay, no merge rule — an ordinary code merge is
  invisible to the ledger reader so long as the resulting goal tree
  validates. The accepted ref NEVER moves backward: a fetched tip
  that is not a descendant of the accepted tip refuses by name
  (rewind), and a tip whose tree fails validation refuses naming the
  file and rule; in both cases the projection stays at the accepted
  tree and this clone's mutations refuse with the same diagnosis
  until the remote is fixed.
  HONEST TRUST SCOPE (D118): integrity digests, the schema, and the
  guard catch write interruption and every accidental or tooling
  edit; a deliberate edit that recomputes the digests is the
  same-user adversarial tier recorded as out of scope — the ledger
  is exactly as trustworthy as the repository that carries it, and
  no stronger claim is made. History is not authenticated (round-7
  finding 3 proved a git-internal history guarantee impossible; the
  machinery that pretended otherwise is gone).
  BOOTSTRAP: genesis is ONE commit — migration or adoption output,
  root record included (R7-04). A fresh clone fetches, validates the
  TREE, and sets accepted; there is no anchor chain to walk.
  REMOTE-TREE CORRUPTION, the human path (replaces the repair
  protocol): the refusal names the file and rule; a human fixes the
  remote with ordinary git — revert, edit, force-push, whatever the
  repository's own rules allow — and every clone accepts the fixed
  tree on its next fetch because validation looks at the tree, not
  the path that produced it. The one clone-local verb: `goal repair
  --accept-remote --by <human>` re-points THIS clone's accepted ref
  at a fetched tip that validates but fails the descendant rule
  (someone lawfully rewound the shared branch) — human-reserved,
  journaled, and purely local. Its journal records the old and
  target accepted oids, and its recovery postcondition is LOCAL:
  the accepted ref equals the target oid (no History line, no
  trailer — nothing was pushed; the opid predicate does not apply —
  R8-04). Other clones make their own decision the same way.
  This validator is also the READ CONVERGENCE PATH (R3-01): `goal
  fetch` — implied by reads with --fetch and by every mutation — is
  how machine A observes machine B: validate, then CAS-advance
  accepted, no local mutation required.

### The per-verb effect table (the writer's specification)

This is the WRITER's contract (D118): the transaction builder
produces exactly these effects, the tree validator checks the
resulting state, and reconcile uses the same table to map hand-edit
deltas onto verbs — nothing replays it against history. Common
effects, stated once: every touched goal file's Revision increments
by exactly one and its History gains exactly the opid's line with
the verb and actor; root-record-touching verbs do the same to the
root record; a verb touches NO path outside its listed set. "Arc
cascade" = the asked goal and the arc's other live members, all in
the one commit.

| Verb | Paths touched | Before → after, per file |
|---|---|---|
| open | adds live `<id>.md`; root record iff Goal-free was declared (clears it) | id absent in live+archive → queued, Revision 1; edges exist, no cycle |
| open --claim | as open, state claimed | absent → claimed by actor pair; claim guards hold |
| claim | arc cascade, else the one file | queued → claimed, one claimant across all touched files; every touched goal's blockers done |
| claim --steal | arc cascade, else the one file | claimed → claimed by the new pair; --by recorded in each History line (R7-08) |
| release | arc cascade | claimed by actor → queued, all touched files |
| done | live `<id>.md` deleted + `done/<id>.md` added; the one member only | queued/claimed/parked-per-authority → done with conclusion; sibling arc members untouched |
| park | arc cascade | queued/claimed → parked with reason; displaced recorded when parking another's claim |
| unpark | arc cascade over parked members; root record iff Goal-free clears | parked → queued, whole arc |
| reopen | `done/<id>.md` deleted + live `<id>.md` added; root record iff Goal-free clears | done → queued; claimed under a standing arc claim; parked into a parked arc (arc-conditioned rows) |
| edit | the edited file; on an arc-membership edit, every file whose Arc or state changes | field deltas per edit rules; composed graph acyclic |
| prune | deletes `done/*.md` outside the retention closure; root record (History carries the opid, since the target files die) | files outside closure(live ∪ keep-count done) → absent; nothing inside the closure changes; keep count recorded in the root record's History line |
| declare-free | root record only | no queued/claimed goals on the tree; renewal idempotent |
| reconcile | every path the MAPPED verbs' write sets touch — arc cascades and root-record effects included, not just the hand-edited files (R8-06) | the base→edited delta DECOMPOSES into lawful row deltas, all actor H, each applied with its verb's complete effects |
| migrate | deletes goals.md + goals-accepted.json; adds the full synthesized live+done tree and the root record | legacy ledger → synthesized tree, ONE commit, one opid (R7-04); manifest digest, mode, and sync mode land in the root record; adoption of a fresh repository is the same shape with an empty goal set |

### Reads: the canonical projection

list/next/verdict/serving read the accepted ref's tree, never the
checkout. Staleness reported past a threshold; `--fetch` runs the
read-side validator first. Offline reads work; offline mutations
refuse (except single-machine mode); a published claim may be worked
offline.

### Integrity layers

Per-file Integrity line (write interruption, accidental edits);
Revision + opid History as a DIAGNOSTIC, never an acceptance gate
(R8-11, D118): a file whose History is a strict prefix of the
accepted copy's is NAMED in the staleness/refusal reporting as an
apparent rewind of that goal, but a validating descendant tree is
accepted regardless — an ordinary `git revert` restoring an older,
valid goal state is exactly the sanctioned repair path, and read
acceptance judges the tree alone; the accepted ref (bootstrap =
first fetched tip whose TREE validates; CAS advance; rewind refusal
with the human-reserved clone-local `repair --accept-remote`;
journal crash repair; tamper posture per D118: the accepted ref is
a local cache of the last tree this clone validated). Reconcile =
DELTA MAPPING with a PERSISTED BASE (R8-06, R9-01): the engine
records the MATERIALIZED-EDIT-BASE — the goal-tree object id it
last wrote into this checkout (after every reconcile refresh and
every checkout update of goal paths) — and reconcile diffs the
edited snapshot against THAT recorded oid, never against HEAD and
never against "the accepted tree": consecutive reconciles without a
pull each start from the previously published tree, HEAD moving
mid-session changes nothing, and another machine's concurrent edit
is in neither snapshot so it can never appear as a reverse delta or
be overwritten (falling back to HEAD's goal tree only when no
record exists yet). STABLE CAPTURE (R9-02): the candidate surface
is the working tree's plans/goals/ live + done/ + backlog.md bytes,
staged or not (the index is neither read nor written); capture
reads every candidate file once into one in-memory snapshot, and
mapping plus publication use ONLY snapshot bytes — an editor save
during the session cannot tear the published transaction. The
post-publish refresh compares each file's CURRENT bytes to the
captured snapshot first: files edited since capture are left
untouched and named (re-run reconcile for them); the
materialized-base record is written before the refresh, and the
refresh is idempotently re-runnable (`goal reconcile
--refresh-only`) if publication succeeded but the refresh died.
THE HAND-EDIT GRAMMAR IS EXECUTABLE (R9-03): generated fields
(Revision, History, Integrity, Claimed, OpenedAt) are IGNORED as
input — a hand-created open file needs only the human fields, the
engine synthesizes the rest at publication, and a hand-supplied
generated value that differs from synthesis refuses by file and
field (never silently rewritten). A multi-field delta on one goal
maps to the SMALLEST verb set in pinned precedence — the state
verb first (open/park/unpark/done/reopen), then ONE edit for every
remaining field change — each mapped verb its own History line and
Revision increment, all inside the one transaction commit.
Cross-file cascade recognition: identical park deltas across ALL of
an arc's live members map to one cascade park; a partial-arc park
refuses (cascades are all-or-none). Mapped verbs apply their
COMPLETE write sets (arc cascades, root-record effects). The
editable surface stays CLOSED: edit's fields (intent, next,
blockedBy, arc) plus the shapes of open, park, unpark, done, and
reopen; anything else is unmappable and refuses by file and field.
A mapped row whose before-predicate fails on the fetched tip is a
CONFLICT, refused naming goal and field.

### Goal-free (R3-07 correction)

Lives in the root record. Exclusivity matches TODAY'S semantics
exactly: declare-free refuses while any QUEUED or CLAIMED goal exists;
PARKED goals coexist (the shipped `park --and-none` flow is lawful and
migrates losslessly). open/reopen/unpark clear it in their own
transactions. FRESHNESS DISPOSITION (R7-11), explicit because the
shipped contract hashes the plans/*.md stream: the freshness domain
is that same plans/*.md stream (new plans still expire a declared
absence — behavior unchanged at cutover) PLUS the live plans/goals/
files, EXCLUDING backlog.md itself (R3-11) and the done/ archive —
archive maintenance never expires a declaration. The cutover fixture
pins both directions. declare-free renewal is the same verb,
idempotent, opid-journaled.

### Claims, ownership, serving

Claimant = machine + lineage; machine is the quota key, and the
quota is EXACTLY ONE claim per machine in v1 — an arc counts once,
there is no override knob, and every clone validates the same rule
tree-wide (R8-09; a configurable quota is a follow-up goal if ever
wanted, not a config key now). The pair is the ownership key;
serving follows the asking lineage; a second lineage on the machine
is refused service by name. CLAIM IS
AGENT-ONLY (R3-06): humans direct agents; no human lineage exists, so
no H claim row. `--steal` requires `--by <human>`.

SERVING AND PROJECTION UNDER ARCS (R7-09): claiming an arc claims
every live member, so one claimant may hold several goals at once.
The serving projection and the turn prompt carry ONE LINE PER
CLAIMED GOAL — the prompt grammar extends from exactly-one to
one-or-more serving-goal lines, serving protection covers every
member, and a mission declares the member ids it serves exactly as
it names a single goal today; `goal next` orders the members by
(OpenedAt, id) but imposes no working order.

ARCS (R4-08, R5-05): goals that must be worked together share
`Arc: <arc-id>`. The arc binds OWNERSHIP over its LIVE members only:
claiming any member claims every live member in one transaction, one
claimant, one quota slot. Lifecycle rules, pinned: `done` on a
member archives THAT member (leaving the arc's live set) and keeps
the claim on the remaining live members; `release` and `park` cascade
to ALL live members in the one transaction (park records displacement
per member); `unpark` of any member restores every parked member of
the arc to queued (the arc pauses and resumes whole). A done-and-
reopened member rejoins the live set and, if the arc is claimed,
rejoins under the standing claimant in the same transaction (so
reopen into a claimed arc is C or H — an outside agent cannot inject
work into someone's claim). ARC EDITS are ownership-sensitive and
NEVER split ownership (R6-06): after any membership edit, all live
members share one ownership state, and the edit's own transaction
performs whatever transition that requires. Adding a QUEUED goal to
a CLAIMED arc (actor: claimant or human) claims it in the same
transaction — quota is unchanged, the arc already counts once;
adding a goal that is parked, or claimed elsewhere, refuses. A
PARKED arc's membership edits are human-only; a queued or parked
incoming member parks with the arc's park record, a claimed one
refuses (release first); reopening a done member whose arc is parked
is likewise human-only and lands parked (R7-08). MEMBERSHIP MATRIX
(R8-07) — the complete source-state × destination rules, all as one
`edit --arc` transaction:

| Member leaves / joins | Authority | Result |
|---|---|---|
| leaves a QUEUED arc (detach or move) | A,H | member keeps its state; plain edge edit |
| leaves a CLAIMED arc | C,H | member is RELEASED to queued as it detaches — a detach never silently splits the claim into a second quota slot |
| leaves a PARKED arc | H | member stays parked under its own record |
| joins a QUEUED arc | A,H | member keeps queued; a parked or claimed member refuses (unpark/release first) |
| joins a CLAIMED arc | C,H | queued member with blockers done auto-claims (R8-02 guard); anything else refuses |
| joins a PARKED arc | H | queued or parked member parks with the arc's record; claimed refuses |
| a move between arcs | the stricter of the two rules | detach result feeds the join rule, one transaction — moving the only claimed member into a queued arc therefore lands QUEUED (release, then join; auto-claim fires only when the DESTINATION is claimed) — R9-10 |

There is NO separate merge operation (R9-10): "merging arcs" is
per-member `edit --arc` moves, each composing detach-then-join under
the stricter authority, applied member-by-member in target order —
the outcomes above are exhaustive. A different claimant's member can
never join a claimed arc without its own release first (the join
rule requires the destination arc's claimant), and two claimed arcs
under ONE machine cannot exist to be merged at all: v1's
exactly-one-claim quota already fails that tree in validation
(R9-09) — the former same-claimant-merge row was unreachable and is
gone. Every arc edit validates like any edge edit, and the composed
graph is cycle-checked.

### The pause lever

Park predicate: unclaimed non-human-origin goal — any main or human;
unclaimed HUMAN-ORIGIN goal — human only (R3-06: an agent cannot
silently remove a standing human reservation; this matches the
existing human-origin authority); own claim — the claimant; another's
claim — human only (--by), displaced claimant retained in the record
and History, notified at its next interaction, acknowledgment = its
next History-appending interaction. MISSION-SERVING goals (R3-06,
R8-01): serving enforcement is CLONE-SCOPED, by design rather than
omission (D118 — no fleet mission registry): a mission serves goals
of its own machine's claim, and THAT machine refuses every mutating
verb on a served goal — park, release, done, edit, steal alike —
naming the mission; stop the mission first. The claimant machine
likewise refuses arc-membership edits while any member is served.
No OTHER machine's agent can reach the claim at all (foreign-claim
mutations are human-only), so the cross-machine case is the human
override — and EVERY foreign-human mutation of a claimed goal is
DISPLACEMENT-BEARING (R9-05): park --by and steal --by are the
canonical forms, but a human done, edit, arc detach, or membership
injection on another machine's claim records the same displaced=
marker uniformly, so no lawful override can change a mission's
scope without leaving the signal. THE CHECKPOINT THAT SEES IT
(R9-04): in remote mode the mission runner's TURN BOUNDARY is a
named fetch-validate-advance checkpoint — each turn starts with
goal fetch; displacement observed on a served goal stops the
serving mission by the stop-loss path, naming the displacement.
Offline or transport failure at the checkpoint reports staleness
and CONTINUES (the cooperative pause degrades to
eventual-at-reconnect; a network blip never wedges the runner) —
this is the honest cross-machine contract: protection where the
mission lives, override visible at the next turn boundary, never
silently fought. ACKNOWLEDGMENT HAS A WRITE PATH (R9-06): the
displaced pair's next published transaction piggybacks one
automatic ROOT-RECORD History line — `ack` with targets=<the
displaced goal> and the displaced=<pair>@<at> it answers — in the
same commit; the root record needs no goal authority, which is
exactly why the ack lives there and not on the (now foreign or
parked) goal file. Unpark restores queued, history preserved.

### The transition-authority table

Actors: A=agent main, C=claimant pair, H=human. All rows also require
the fresh canonical revalidation and refuse on mission-serving goals.

| From | Verb | Actor | To | Guards |
|---|---|---|---|---|
| (none) | open | A,H | queued | id free live+archive; edges exist; no cycle; clears Goal-free |
| (none) | open --claim | A | claimed | open guards + claim guards in one transaction (the solo one-command flow that replaces first-open-becomes-Current) |
| queued | claim | A | claimed | blockers done; quota free; not parked |
| queued | edit | A,H | queued | edge rules; post-integration cycle check |
| queued | done | A,H | done | blockers done; conclusion; human-origin: H |
| queued | park | A,H | parked | human-origin: H; reason |
| claimed | release | C | queued | — |
| claimed | done | C,H | done | conclusion; human-origin: H |
| claimed | edit | C,H | claimed | a new blocker must be DONE for every actor — a claimed goal is never blocked (D115's invariant; a human who wants the edge parks or releases first) |
| claimed | park | C; H(--by) | parked | displaced recorded when not C |
| claimed | claim --steal | H(--by) | claimed(new) | steal recorded |
| parked | unpark | A,H | queued | human-origin park: H; clears Goal-free |
| parked | done | H | done | conclusion |
| done | reopen | A,H | queued | refuses under claimed transitive dependents; clears Goal-free; moves from archive |
| done | reopen (member of a claimed arc) | C,H | claimed | rejoins under the standing claimant in the same transaction; the member's blockers must be done (every automatic-claim path carries the claim guards — R8-02) |
| queued | edit --arc (into a claimed arc) | C,H | claimed | claims in the same transaction — so the incoming goal's blockers must be done; quota unchanged |
| queued/parked | edit --arc (into a parked arc) | H | parked | human-only; parks with the arc's record; a claimed incoming member refuses (release first) |
| done | reopen (member of a parked arc) | H | parked | human-only; rejoins parked with the arc's record |
| done | prune | A,H | deleted | retention closure below |
| root | declare-free | A,H | — | no queued/claimed goals; renewal idempotent |
| any | reconcile | H edits | per delta mapping | each delta maps to a row |

SHIPPED-SURFACE DISPOSITIONS (R3-05), decided here: `set-next` →
retained as an alias for `edit --next` (the most-used verb keeps its
name). `promote` → retired; its meaning (make Current) has no global
referent; the error names `claim`. `park --then` / `--and-none` →
retired; the slot-handoff is meaningless without a global Current; the
errors name `claim` and `declare-free`. `reconcile --genesis-from` →
carried unchanged. `next` (R6-09) gets an explicit cutover contract:
it reports the asking lineage's claimed goals first (the work in
progress); otherwise the claimable frontier — queued, blockers done,
not parked — ordered by (OpenedAt, id) and suggesting `goal claim
<id>`; otherwise a blocked/parked summary; a declared Goal-free is
acknowledged as such. The shipped "run goal promote" hint and the
test pinning it die at cutover with the verb they name. First-open UX: `open --claim` (above) preserves the
one-command actionable flow; bare open leaves queued.

Prune retention (R3-10, R4-09): selection first, closure second —
retained = closure(live goals ∪ the keep-count newest done goals),
following BlockedBy transitively THROUGH done goals, so a keep-count
survivor's own older done blocker is retained with it and no edge can
dangle by construction. Prune deletes outside that closed set, one
transaction, `goal.prune-keep` default 50, "oldest" = smallest
OpenedAt. Edges are never rewritten. Fixture: a 51-chain where the
newest depends on the oldest retains both.

### Cutover matrix

Stop verdict / --serving-goal / dispatch → the asking lineage's
claims via the projection (plural under an arc — the prompt grammar
extends to one line per claimed goal, BGS-16). Open-work scanner → no behavior change; goal
policy's reader retargets. Adoption → repo ensured; remote-less
targets configured single-machine mode explicitly; root record
written; legacy pair never written; GUARD ENROLLMENT (R8-10): hooks
are clone-local, so every goal verb preflights the pre-commit guard
in THIS clone — missing means install it, an existing foreign hook
means COMPOSE with it (chain, never overwrite — the shipped
installer's refusal to compose is fixed as part of this build), and
mutations refuse until the guard is in place; docs/project-adaptation.md
(R3-12) rewritten with the new ledger and digest story in the same
change. Command-boundary genesis sentinel (R3-12) → re-keys from
goals-accepted.json's absence to the root record's absence. Authority
prose (AGENTS.md, wow.md, plans/README.md) → rewritten at migration.
Acceptance fixtures naming goals.md → retargeted, enumerated in the
implementation plan. Wall protected paths → plans/goals/ live+archive
added (HIGH). Anything unlisted found later returns to design.

## Migration: a generic verb plus this repository's manifest

`goal migrate` is the generic SEMANTIC-LOSSLESS (R3-07) conversion:
the source domain is exactly "ledgers the shipped parser accepts AND
matching their accepted baseline"; every parsed fact maps to a field,
non-field prose the parser tolerated is preserved verbatim in
LegacyNotes, Parked-plus-Goal-free maps losslessly (see Goal-free),
and the bijection proof is: re-rendering the new files through the
legacy renderer reproduces the parsed semantic model exactly (field
set equality, not byte equality — byte-lossless is explicitly not
claimed and LegacyNotes records the residue).

This repository's changes ride `--manifest
plans/goals-migration-manifest.md`, WHICH EXISTS and is reviewed with
this design (R4-07). Its schema separates two operation kinds with a
hard boundary: `add-goal` (a brand-new goal: id, intent, origin,
next, blockedBy, arc, full text — synthesized OpenedAt per the
post-legacy rule) and `amend-goal` (a REVIEWED SEMANTIC CHANGE to a
legacy goal, listed per field: state overrides, next-step refreshes,
arc assignments — OpenedAt always preserved from synthesis; a
manifest id colliding with a legacy goal in add-goal mode refuses,
and amend-goal of a missing id refuses). Bare `goal migrate` is pure
lossless conversion and may not alter any parsed fact; every semantic
delta lives in an amend entry where the review can see it. Root-level
ledger prose the legacy parser ignored is preserved in the ROOT
record's LegacyNotes (per-goal residue in the goal's own LegacyNotes).
The MIGRATION_EPOCH is a literal in the manifest. The manifest's
sha256 and the migration mode are bound into the journal entry AND
the root record (R3-08); `--manifest` after a completed bare
migration refuses; post-hoc changes go through ordinary verbs.
Preconditions: baseline match; THE REVIEWED-SOURCE BINDING (R5-08):
the manifest carries the sha256 of the exact legacy ledger this
design's expected map was reviewed against, and migrate refuses if
the live ledger's digest differs — a lawful goal change after review
must re-run the review, not migrate silently past it; all-legacy;
clean EXACTLY at plans/goals.md, plans/goals-accepted.json,
plans/goals/, and the manifest path; nothing wider. Synthesis, deterministic in its SEMANTIC PAYLOAD
(R7-13, honestly narrowed by R8-08): positions are ONE-BASED
everywhere — legacy position = the goal's one-based index in the
order the shipped parser yields the ledger, manifest position = the
add-goal entry's one-based index in textual order; OpenedAt = EPOCH
+ position × 1min (legacy) and EPOCH + (1000+position) × 1min
(manifest); every synthesized timestamp the schema requires and the
legacy source lacks is the EPOCH literal — Current → claimed by the
migrating machine+lineage at=EPOCH; legacy parked →
by=legacy-migration at=EPOCH; legacy done → archive. What two runs
CANNOT share are the run-scoped provenance values: the migration
opid (a ULID), its History timestamp, the minted ledger identity,
and the Integrity digests derived from them, PLUS the executing
machine+lineage recorded in the Current goal's synthesized claim
(R9-11) — so the determinism claim is: identical semantic payload
given the same source, manifest, and epoch, MODULO that recorded
executing pair; byte-identical trees exactly when the fixture
injects all four (opid, timestamp, identity, pair); and a completed
migration NEVER re-synthesizes (idempotent exit 0), so production
sees one tree, ever. Crash/rerun: the transaction protocol owns it; completed
migration = idempotent exit 0 keyed on the root record + mode.

### The reviewed expected map (corrected against the record)

Corrections from round 3 (†): runtime-install-execution is QUEUED —
D81 explicitly unblocked it (its stale next-step text is updated by
the manifest, not its state); executable-covenant carries NO blocking
edge — D114 requires co-design with critique-stop-rule, expressed as
one arc note in both files, not a dependency; wall-o11 is the
INDEPENDENT legacy-state-refusal row, unblocked; the genesis pair
carries no invented edge and is PARKED by operator assignment (they
are hand-assigned to the second machine until it can claim them —
parking is the mechanical anti-duplication guard, and its reason says
exactly this; the machine unparks-and-claims when it gains the verbs).
Round-5 corrections: the genesis pair's "same arc" is a REAL field —
both carry Arc: genesis-authority; and the promotion descope adds one
queued goal, backlog-local-promotion (the local-mode ledger's path
into a fleet, designed later; local mode's banner names it).

Manifest goals, all queued, all claimable unless noted:
wall-o13-acceptance-write; wall-o14-sealed-dirty-composition;
wall-o15-head-accounting; wall-o16-host-repo-fence (BlockedBy
wall-o15: one snapshot-scope design owns both); wall-o19-recovery-
ladder; wall-o8-verbatim-interim; wall-o9-extractor-floor;
wall-o10-evidence-durability; wall-o11-legacy-state-refusal †;
backlog-local-promotion (the promotion descope's follow-up).

| Goal | State | BlockedBy | Claimable? |
|---|---|---|---|
| backlog-git-sync | claimed (migrating machine) | — | held |
| host-implementer-wall | parked (umbrella; human unpark+conclude when rows finish) | the nine wall-o* goals | no |
| genesis-authority-design | parked † (operator assignment: Intel machine, in flight), Arc: genesis-authority | — | no |
| provision-genesis-authority | parked † (same assignment), Arc: genesis-authority | — | no |
| critique-stop-rule | queued, Arc: covenant-patience | — | YES (claims the arc) |
| executable-covenant | queued, Arc: covenant-patience † | — | YES (claims the arc) |
| runtime-install-execution | queued † (D81 unblocked; manifest refreshes its next-step text) | — | YES |
| lease-acquire-atomicity | queued (wall landed; its wait is satisfied) | — | YES |
| flake-registry, landing-tooling-fixes, invariant-consolidation, agent-ease-assessment, source-comment-standard, kill-python-fixtures, custody-death-proof, qualified-name-sweep, ki-23-acknowledged-process, mission-completion-protocol, narrator, small-change-lane, fixtures-as-arbiter, two-bars-for-changes | queued | — | YES |
| goal-ledger-ergonomics | parked (superseded-as-park: BGS-14 delivers its core; residue folds to agent-ease-assessment) | — | no |
| disk-hygiene, process-steward, acp-transport | parked (unchanged) | — | no |
| five done goals | archive | — | — |

## What this deliberately does not do

No daemon, lock server, or network lease; no automatic theft or
expiry; no offline multi-machine mutations; no index file; no
per-state folders beyond done/; KI-38, go-gate order, wider
ergonomics stay their own items.

## Obligation matrix

| Obligation id | Severity | Design source | Required behavior | Owner | Code proof | Test proof | Runtime proof | Status | Next action |
|---|---|---|---|---|---|---|---|---|---|
| BGS-1 | CRITICAL | D115 | Different-goal concurrent mutations from two clones both publish; each machine observes the other via goal fetch (read-side advance, no own mutation needed); projections converge | transaction + read validator | `internal/goal/txn.go` + `fetchadvance.go` | two-clone fixture: concurrent different-goal mutations, then fetch-only convergence on both | two machines | MISSING | implement |
| BGS-2 | CRITICAL | D115 | Same-goal claims: force-with-lease CAS with explicit expected oid; exactly one wins; loser classifies by opid postcondition, reverts nothing of the user's, names the winner; parented commits only | claim transaction | `internal/goal/txn.go` | race fixture incl. dirty-worktree + diverged-HEAD legs; a parentless or wrong-parent commit is refused by the lease leg | two machines | MISSING | implement |
| BGS-3 | CRITICAL | R1/R2/R3 | Full read-set revalidation on the transaction tree: Revision, blockers, quota, composed-cycle, referential integrity, Goal-free rules, table rows incl. blocked-done and mission-serving refusals on EVERY verb | validation | `internal/goal/validate.go` | fixtures: composed cycle; blocker reopened mid-race; quota (exactly one, tree-wide); claimed-implies-blockers-done as a full-tree invariant incl. the reopen-into-claimed-arc leg; blocked done; clone-scoped mission-serving release/done/edit/steal refusals incl. arc-membership-edit-while-served; displacement-as-stop-signal leg (human park --by stops the foreign serving mission at its next interaction) | migrated queue | MISSING | implement |
| BGS-4 | CRITICAL | R2/R3 | Migration: semantic-lossless over the shipped parser domain (LegacyNotes residue, Parked+free lawful), baseline precondition, manifest schema (closed keys, refusal rules) + manifest digest + mode bound in journal and root record, reviewed-source ledger digest literal checked BEFORE any mutation, exact clean-path set, deterministic synthesis, idempotent rerun keyed on root record + mode, --manifest-after-bare refusal | goal migrate | `internal/goal/migrate.go` | fixture on a copy of the LIVE ledger asserting the expected map verbatim; park--and-none source leg; LegacyNotes round-trip leg; rerun, divergent-baseline, mode-confusion, unrelated-plans-edit legs; source-digest-mismatch refusal leg; determinism legs (semantic payload identical modulo the recorded executing pair; byte-identical under injected opid/timestamp/identity/pair; completed migration never re-synthesizes); manifest edge legs (explicit blockedBy-clearing syntax `blockedBy: -` distinct from omission; a parked amendment without parked-because refuses, R10-M08) | the migration commit | MISSING | implement |
| BGS-5 | CRITICAL | R2/R3, D118 | Integrity lifecycle: opid-History rollback discrimination; accepted-ref bootstrap/CAS/rewind-refusal incl. a CAS-loss fixture; single-commit genesis; foreign-ledger identity refusal; the human-reserved clone-local `repair --accept-remote --by`; reconcile delta mapping with unmappable-field refusal by name | integrity layers | `internal/goal/accepted.go` | fixtures: a descendant revert restoring an older valid state is ACCEPTED with the prefix diagnosis reported (R8-11); accepted-ref CAS loss recovers; rewind refuses, then repair --accept-remote recovers under a human and its LOCAL postcondition (ref equals target) resolves recovery; fresh-clone bootstrap validates the tree; foreign-ledger fetch refuses by name; corrupt remote tree refuses naming file and rule, then an ordinary git fix is accepted on refetch by a SECOND clone; reconcile persisted-base legs (consecutive reconciles without a pull; HEAD moves mid-session); stable-capture legs (editor save during capture never tears the publication; a file edited after capture survives the refresh and is named; --refresh-only completes a died refresh from the DURABLY captured snapshot — publish, die before refresh, user edit/create/delete, refresh-only preserves and names, R10-M01; ordinary reconcile refuses while that refresh is pending); persisted-base maintenance legs (base advances on ordinary pull/checkout, not just verb writes; the recorded tree stays reachable through rewind repair and git gc, R10-M02); index-independence, unchanged-generated-fields-succeed, altered-generated-fields-refuse, full-arc hand-park, and park-plus-edit compound legs (R10-M07); concurrent foreign edit survives a hand-edit session; hand-created open file publishes with synthesized generated fields; partial-arc hand-park refuses; reconcile conflict leg (before-predicate fails on the fetched tip, named); unmappable reconcile delta names the field | operator workflow | MISSING | implement |
| BGS-6 | CRITICAL | R2/R3, D118 | Journal: three durable phases (created/pushed/terminal); ONE recovery rule — refetch + opid postcondition — total for EVERY verb (walk the canonical log via the provenance trailer when the file moved or died); pushed blocks own-clone mutations process-independently; deadline-based retry under benign advancement; deadline expiry from pushed | journal | `internal/goal/journal.go` | kill-between-push-and-confirm recovery; four-benign-advancements-still-succeeds leg; rebuild-from-stored-intent legs (unrelated and arc-affecting advancement, incl. reason/conclusion/delta arguments); live-owner exclusion leg (a running owner's entry is never terminalized); recovery-completes legs (dead owner at created AND at pushed, past the deadline included — both complete, R10-M04; remote still at expected tip — the rebuilt push finishes the operation); expiry-is-live-owner-only leg; rejected-terminal leg (rebuild refuses on the new tip, named); confirmed-late correction leg (a terminalized entry's opid later appears in canonical history and the journal corrects itself during an ordinary later interaction); per-verb-class postcondition legs incl. prune and declare-free | both machines | MISSING | implement |
| BGS-7 | CRITICAL | R3-03, D118 | Both enforcement points: the pre-commit guard refuses staged plans/goals/ changes in ordinary commits; the read-side validator refuses to advance onto a tree failing schema/integrity/graph/claims/arc/identity validation or onto a non-descendant tip, naming the file and rule, and the projection stays at the accepted tree | guard + validator | guard hook + `internal/goal/fetchadvance.go` | fixtures: user commit touching a goal file refuses (guard); fresh-clone enrollment installs the guard before the first mutation; an existing foreign pre-commit hook is composed, not overwritten; a hand-edited goal file without a recomputed digest pins the projection and is named; a rewound remote refuses until repair --accept-remote; an ordinary code merge leaving the goal tree valid advances | both machines | MISSING | implement |
| BGS-8 | HIGH | operator, R3-06 | Park predicate incl. human-origin protection and agent-only claims; displaced retention + notification + acknowledgment line; unpark history; done-archive move; list --done | park + archive | `internal/goal/verbs.go` | two-clone operator-park fixture incl. the automatic root-record ack line on the displaced pair's next transaction; displacement-bearing foreign done/edit/detach legs (every foreign-human mutation of a claimed goal records displaced=); agent parking human-origin refuses; unpark preserves records; archive move + list --done | operator workflow | MISSING | implement |
| BGS-9 | HIGH | R1/R2/R3 | Transition closure: reopen-under-claimed-dependents refusal; prune closure follows done-to-done edges (no dangling by construction), keep-count and oldest-by-OpenedAt honored; shipped-surface dispositions (set-next alias works; promote/--then/--and-none name their successors) | table + verbs | `internal/goal/validate.go` | per-row fixtures incl. done-to-done prune chains, each retired verb's named error, the literal prune `keep=<n>` History field (R10-M07), and next's cutover contract (claimed-first/frontier/blocked outputs; the promote hint and its pinning test are gone) | migrated queue | MISSING | implement |
| BGS-10 | HIGH | R1/R2, R8-09 | Ownership machine+lineage; quota exactly one per machine, no override, validated tree-wide; second-lineage service refusal by name; steal --by | identity | `internal/goal/identity.go` | foreign release; second-lineage serving; steal without --by | two machines | MISSING | implement |
| BGS-11 | HIGH | R2/R3 | Goal-free: queued/claimed exclusivity with parked coexistence; clears on open/reopen/unpark; digest excludes the root record; renewal idempotent | root record | `internal/goal/root.go` | park --and-none flow lawful; declare-free with queued refuses; digest stability under root-record edits | operator workflow | MISSING | implement |
| BGS-12 | HIGH | wall | plans/goals/ live+archive in the wall's protected table | wall table | wall.go | protected-path fixtures over both dirs | wall fixtures | MISSING | implement |
| BGS-13 | MEDIUM | R2/R3 | Projection reads the accepted tree only; staleness banner; durable sync-mode identity (root record vs config mismatch refuses by name); single-machine-mode banner naming backlog-local-promotion (local mode is terminal until that goal lands); scanner byte-compatible | projection | `internal/goal/project.go` | mid-edit checkout invisibility; staleness; local-mode banner; config-flip promotion refusal; scanner fixture unchanged | both machines | MISSING | implement |
| BGS-14 | MEDIUM | D113/D114, R3-13 | Ergonomics: edit verb; direct queued done; prose caps REMOVED (witnessed by a multi-kilobyte intent, not a 500-byte one); id cap kept at the named filesystem invariant and witnessed | verbs | `internal/goal/verbs.go` | edit queued; one-verb done; 4KB intent accepted; 101-char id refused naming the invariant | operator workflow | MISSING | implement |
| BGS-16 | HIGH | R7-09 | Arc serving projection: the turn prompt and serving projection carry one line per claimed goal (grammar one-or-more); serving protection covers every member; missions name served member ids; stop verdict and scanner read the set | projection + prompt | `internal/dispatch/servinggoal.go` + `internal/validate/turnprompt.go` | multi-line prompt accepted; single-goal prompt unchanged; serving protection refuses mutation of EVERY claimed member; stop verdict over the set; turn-boundary checkpoint legs over the FULL runner lifecycle (first turn, ordinary turns, resume-after-crash-healing before cycle reservation, parked and completed missions — the runner is a code target of this row, R10-M03); displacement stops WITHOUT launching a host or spending a cycle; cached displacement stops while offline (only transport ABSENCE degrades to staleness; a validation failure does not); foreign steal and membership-injection stop legs incl. injecting a member outside the mission's served set (R10-M06); stop → automatic root-record ack → explicit resume, and the answered displacement does not retrigger | wall fixtures | MISSING | implement |
| BGS-15 | HIGH | R4-08/R5-05 | Arcs: live members claim/release/park/unpark as one atomic unit under one claimant counting once against quota; done archives the member and leaves the live set (arc survives); arc edits on a claimed arc need the claimant or a human; merging two claimed arcs refuses; a reopened member rejoins under the standing claim | arcs | `internal/goal/arcs.go` | covenant-patience two-clone race (one winner takes both); member done leaves sibling claimed; reopen into the claimed arc rejoins under the claim; park cascades and unpark restores whole; queued-into-claimed-arc auto-claims under C (blockers done), refuses under a stranger or with an unfinished blocker; parked-arc membership edit is human-only; detach from a claimed arc releases the member (no quota split); move of the only claimed member into a queued arc lands queued; a human move from a claimed source into a claimed destination refuses (per-member composition, R10-M05); cascade park pins ONE acknowledgment for the displaced pair (R10-M06) | two machines | MISSING | implement |
