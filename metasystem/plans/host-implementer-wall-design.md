# The host-implementer wall (goal host-implementer-wall, D99)

- Status: DRAFT r2, RULINGS RESOLVED (D100) — r1 folded
  (plans/hiw-critique-r1.md: 9 findings, 7 structural). Wido ruled:
  NO self-work exception (every product byte comes from a certified
  delegate; the "change this little thing" case gets its own
  backlog goal, small-change-lane, instead of a host exemption),
  and the wall ships at the DEFECT DETECTOR tier now. The design
  below is updated to both rulings — the exception machinery is
  deleted, which removes roughly a third of the build.
- Goal: host-implementer-wall (Current)
- In flight right now: the r2 design critique (codex xhigh); not a
  dispatch job record, so the open-work scanner cannot see it.
- Waiting on the human: nothing — both rulings landed (D100).
- Next step: none.

## What happened (unchanged evidence)

bm-2d rep 1: the Devin host built the whole solution solo — eleven
product files in one turn, zero dispatch attempts in 66 commands —
and the runner accepted a return whose `dispatched` list was empty
beside that diff. Only the kit's grading-time delegation floor
noticed. Wido ruled it a total failure of the metasystem (D99): the
design→critique→implement→critique loop is the product, and it was
bypassed while every mechanical stage worked.

## The invariant (r2, sharpened)

A MISSION HOST TURN NEVER SHIPS IMPLEMENTER WORK, and the runner
PROVES it at turn acceptance with a byte-level equation — never a
path heuristic:

    post-tree = pre-tree
              + the certified delegates' EXACT patches, in order
              + exact machine-owned metadata changes

There is NO self-work term: Wido ruled the exception out entirely
(D100). Every product byte comes from a certified delegate; the
legitimate tiny-fix case is served by the small-change-lane goal,
not by perforating this invariant.

Anything the equation cannot account for is a protocol violation:
the return is refused, the mission parks immediately (outranking
any completion-gate success — the "product" is the disputed
evidence), and the workspace is durably TAINTED until Wido resolves
it. Path-subset authorization is dead (r1 HIW-R1-02: a delegate
touching one line of a file must never license the host to rewrite
the rest of it).

## The pieces, each at its owning boundary (HIW-R1-07)

1. **Conformance validation issues the INTEGRATION AUTHORIZATION**
   (HIW-R1-01). Today "certified" is a host assertion the runner
   copies unchecked into the turn log — the same disease as the
   wall's absence, one layer down. After all implementer-return,
   critic-chain, and merge checks pass, conformance validation
   writes a durable, content-addressed authorization binding: job,
   mission, stream, role, patch digest, reviewed tree, changed
   paths, base. No record, no authorization.
2. **Dispatch records immutable provenance** — mission, turn,
   stream, role — and adjudication compares the host's claims
   against the JOB RECORD, never trusting the return's own role or
   stream fields (HIW-R1-07).
3. **The runner enforces the equation at turn acceptance**
   (HIW-R1-02/03). Snapshots use the conformance validator's
   proven primitive — an isolated index, read-tree, add -A,
   write-tree — capturing the full shippable tree including
   untracked bytes, never a status/ls-files digest (which misses
   modified-again files, unstaged bytes, modes, symlinks,
   deletions). The pre-tree identity lives in runner memory AND a
   hash-chained state channel the host cannot forge — never only
   in the turn directory the host can write. Recovery inspects any
   unfinished turn BEFORE taking a new snapshot, so a crashed
   violation can never become the next baseline. Overlapping
   patches, failed application, or anything needing a rebase means
   fresh conformance or park — conflict-resolution bytes get no
   implicit host exemption (mission doctrine already parks
   unattended conflicts; the orchestrator does not invent
   resolution bytes).
4. **Allowed changes are EXACT machine-owned paths, default-deny**
   (HIW-R1-04): the runner's own turn state, adapter transcripts
   and usage records, dispatch/conformance records, supervision
   and lease state — enumerated precisely, principally under
   artifacts/agents/**, plus mission-declared host output files.
   NO blanket path exemptions: plans/** holds the signed mission
   contract and instruction ledgers (protected even inside
   otherwise host-owned directories), and artifacts/plans can be
   shipped paths in some repositories. Git administrative metadata
   stays outside the shippable-tree comparison by construction.
   Ignored files are outside "ships" by the same construction —
   stated, not accidental.
5. **Violation → durable taint + immediate mission park**
   (HIW-R1-05). A dedicated park path that takes precedence over
   completion-gate success (today ConcludeFaultedTurn completes a
   faulted mission when the gate passes — exactly wrong here).
   Stream-level parking is insufficient (unattributed writes bind
   to no stream), so the MISSION parks. The workspace is marked
   tainted in the hash-chained channel; no later host turn may
   baseline on it until either restoration to a known-safe tree or
   Wido's TYPED resolution that records which attribution claims
   he is waiving — a generic acknowledgement never clears taint.
   Nothing is reverted: the diff is evidence.
6. **No self-work machinery** (D100): the exception is ruled out,
   so nothing here measures residuals or prices edge cases — any
   residual after certified patches and machine-owned metadata is
   simply a violation. The typed `selfWork` return field is NOT
   added; the prompt's "too small to delegate" prose allowance is
   REMOVED in the same change (it was the license D99's host
   stretched), replaced by a pointer to the small-change lane once
   that goal ships.
7. **The delegation floor stays and hardens** (HIW-R1-09):
   grading-time defense in depth, consuming the runner-adjudicated
   certification facts instead of raw host claims. The benchmark's
   mirrored evidence schema updates in the same change so the
   extractor accepts the new records. The completion-gate wording
   in all four bm-2 manifests gains the discipline sentence —
   guidance, with the wall as the enforcement.

## The boundary (HIW-R1-08, settled)

The wall binds exactly at the missionrunner's transition after a
runner-created mission host exits and before the turn is accepted —
objective via mission ID, turn ID, the runner-owned pre-snapshot,
and the return. It does NOT live in git hooks, the commit wrapper,
shared adapters, or interactive helpers: interactive development
never traverses missionrunner acceptance and keeps KI-27's
direct-implementation model. A boundary test proves an interactive
direct commit is unaffected.

## What the wall is NOT (the honest tier line)

r1's laundering table is preserved in the critique file: even with
every revision, a host that can MUTATE DELEGATE WORKTREES can
manufacture apparent delegate authorship. Making the wall an
adversarial security boundary requires isolating delegate worktrees
and conformance evidence from the host process — OS-level work in
the same family as the parked genesis directions. Under the
cooperative posture Wido has ruled twice already (C' for genesis,
the accidental model for two-bars), the wall is a DEFECT DETECTOR
with teeth: it catches every accidental and naive shape, including
the exact D99 failure, and the residual shapes are recorded, not
denied. Ruling 2 below confirms or overrides this tier.

## Wido's rulings (RESOLVED, D100)

**Ruling 1:** NO self-work exception — "every product byte comes
from a certified delegate," with the "change this little thing"
case explicitly supported by its own backlog goal
(small-change-lane) rather than a host exemption.

**Ruling 2:** defect detector NOW. The residual deliberately-
deceptive shapes in the critique's laundering table stay recorded;
OS-level isolation of delegate worktrees stays unbuilt, consistent
with the C' and accidental-model rulings.

## Loop discipline

Codex xhigh. The r2 critique runs over this rulings-resolved text:
attack the tree equation's completeness (modes, symlinks,
submodules, deletions under the isolated-index primitive), the
authorization record's binding set, the taint channel's recovery
semantics, the exact machine-owned path enumeration, and whether
removing the prose allowance from the prompt leaves any legitimate
host activity unaccounted. No code before convergence or the
ratified mechanical-grain exit.
