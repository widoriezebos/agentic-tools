# Genesis authority that cannot be laundered (goal genesis-authority-design)

- Status: DRAFT r1 — awaiting critique r1
- Goal: genesis-authority-design
- Next step: Fold the critique verdict when run gad-critique-r1 concludes; implement only after convergence.
- In flight right now: run gad-critique-r1 (codex xhigh).

## The problem the review proved (plans/genesis-authority-review.md)

Genesis authorization — permitting `goal reconcile` to seed a
virgin target's goal ledger — cannot be made sound by CLASSIFYING
the caller, for three converging reasons:

1. **Source-classification is forgeable.** A delegate runs UNDER
   the real orchestrator main, so a caller-supplied `--genesis-from`
   root can copy the main's announcement (announcements are
   pid+start+command-hash, not secret-backed) while omitting the
   nearer delegate's adapter marker; the ancestry walk then reaches
   MAIN.
2. **The virgin target cannot distinguish HUMAN from machinery.**
   An adapter-supervisor has no runtime signature and no custody
   records in a fresh target, so it falls through to HUMAN, which
   genesis admits.
3. **The genesis-vs-holder decision races the lock.** It is taken
   by a pre-lock `os.Stat`; a baseline appearing before the lock
   sends the caller down a write branch that skips the guard.

The common root: any authority DERIVED by classifying an
unauthenticatable caller against caller-controlled or absent state
is forgeable. The fix must be a CAPABILITY, not a classification.

## The design: a genesis grant minted by the source's real lease

The only authentic authority in the system is the SOURCE
checkout's own lease — a real file the running metasystem owns,
not a caller-supplied path. A delegate is not the source's lease
holder and cannot become one. So genesis rides a capability minted
by proving source-holder authority, consumed once by the target
write, unforgeable and unreplayable.

**Minting (in the source's authenticated context).** `adopt.sh`
already runs from the source checkout. Before the genesis write it
calls a new verb:

    metasystem lease mint-genesis-grant --root <SOURCE> --target <TARGET>

`lease mint-genesis-grant` authenticates the caller as the
SOURCE's holder via the EXISTING `require-holder` path (the source
lease is real, not caller-controlled — a delegate fails here), and
on success writes a GRANT file INTO the target's own control-plane
directory. The grant binds:

- a random nonce (single-use),
- the target's absolute canonical path (so a grant for target A
  cannot seed target B),
- the minting main's identity and a monotonic mint time,
- an HMAC over the above under a per-installation secret the
  source holds (artifacts/agents/genesis.key, 0600, created on
  first mint; NEVER copied by adopt's payload allowlist).

The HMAC is what makes the grant unforgeable: only a process that
can read the source's genesis.key — i.e. the authenticated
source-holder's own machine state — can produce a valid grant. A
delegate cannot read it (it is outside every delegate envelope's
read roots and is not payload).

**Consuming (in the target write).** Genesis reconcile requires
the grant:

    metasystem goal reconcile --root <TARGET> --genesis-grant <PATH>

The reconcile verb, UNDER THE STORE LOCK (closing the race):

1. verifies the grant's HMAC against the source key — reached via
   the grant's own recorded source-key path, itself HMAC-covered,
   so it cannot be redirected; a missing/mismatched key refuses;
2. verifies the grant's target path equals this reconcile's
   canonical target;
3. verifies the grant is unconsumed (a consumed-nonce ledger under
   the target, appended atomically) and within a short freshness
   window;
4. verifies, still under the lock, that the ledger is goal-free
   (the F2 guard, now unconditional — a populated ledger is never
   genesis regardless of grant);
5. writes the baseline and marks the nonce consumed in the same
   locked section.

No classification of the reconcile caller is performed at all —
the grant IS the authority. Holder-only remains the rule for every
non-genesis reconcile and every other goal mutation, unchanged.

## Why this closes all three holes

- **Source forgery (F1):** the grant's HMAC requires the source
  key, which a delegate cannot read; classifying a caller-supplied
  root is gone entirely.
- **Adapter-supervisor fallthrough (F2):** no caller classification
  happens; only a valid grant authorizes, and a machinery caller
  cannot mint one.
- **TOCTOU (F3):** grant verification, freshness, goal-free, nonce
  consumption, and the baseline write are one locked section.

## The human path

A human at a terminal is the source holder (or HUMAN, which
require-holder admits for the true human). `adopt.sh` mints the
grant the same way for a human-run adoption; the human never needs
`--genesis-from` and never hand-manages the grant. A human running
`goal reconcile` directly on an already-initialized project is
holder-only, unchanged.

## Migration and blast radius

The `--genesis-from` flag and the effective-class rule (the
unsound D84 attempt) are REMOVED; the `genesis` authority mode is
removed from the matrix (authority returns to holder-only plus the
capability path). adopt.sh mints and passes the grant. Blast
radius: internal/authority (drop the genesis mode), internal/goal
(the grant-gated genesis branch, the consumed-nonce ledger, the
unconditional goal-free guard), internal/lease or a new
internal/genesis (mint + verify + HMAC + key management),
cmd/metasystem (lease mint-genesis-grant, goal reconcile
--genesis-grant), scripts/adopt.sh (mint-then-pass),
benchmark/validate-kit.sh + scripts/adopt-fixtures.sh (drop the
GENESIS_AUTHORITY_ROOT export; the fixtures now ASSERT a valid
grant is required — a missing/forged grant is refused, closing
review F4/F5), docs.

## Loop discipline

Critique at codex xhigh; two-budget allowance; the critique should
attack: whether the HMAC key is truly unreachable by every
delegate envelope AND never enters adopt's payload; whether the
grant's target-binding survives symlink/relative canonicalization
games; whether the consumed-nonce ledger is atomic against
concurrent reconciles; whether a human-run adoption still works
end to end; and whether ANY residual path still classifies the
reconcile caller.
