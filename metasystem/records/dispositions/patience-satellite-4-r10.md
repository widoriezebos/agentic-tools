# Dispositions: patience-satellite-4, round 10

Job: design-critic-20260811t200502z-ea5d (codex gpt-5.6-sol, xhigh).
6 findings, 6 material, all accepted — two directly, four through a
deliberate simplification this round adopts after naming the
generating cause of the loop's churn.

## The generating-cause decision

Convergence stalled (15 → 13 → 4 → 4 → 4 → 6 → 4 → 5 → 7 → 6)
because almost every finding since round 3 attacks one surface: the
rules for counting DAMAGED records deterministically (damage
taxonomies, fallback rows, ordering edge cases, started-work proofs
over broken fields). That surface exists because of r3/P4-031's
fail-toward-vocal rule — my disposition, not a human ruling — which
made patience double-cover damage that the janitor, watchdog, and
usage jurisdictions already voice. This round supersedes it:

**Patience evaluates CLEAN evidence only.** A record participates
only if it is readable, mission-owned, identity-sound (valid jobId
equal to the filename stem), status in a KNOWN vocabulary, and — for
counting — provably started. Damaged records (unreadable, unknown or
missing status, broken fields) are OUT of patience entirely, with
the other jurisdictions' existing vocal channels owning them.
Superseded thereby, with reasons on this record: r3/P4-031 and
r6/P4-043's damaged-status counting, r7/P4-050's damaged quantifier,
the damaged branches of r9/P4-058, and selection rows 5-6. What
survives: orphan chains (clean records with broken lineage) remain
floor-independent damage reports — their records are sound, only
their ancestry is not.

| id | disposition |
| --- | --- |
| P4-063 | accepted — the schema fact was wrong in r9: certification evidence is a STRING. The predicate becomes a non-empty TRIMMED string; no return-contract change. |
| P4-064 | accepted, dissolved by the simplification — a damaged-status job no longer counts at all, so the ordering cannot hide it behind a witness; the drought it represented is the janitor's report, not a patience count. |
| P4-065 | accepted — floor selection quantifies over the STREAK set (the counted jobs newer than the newest witness), consistent with current-drought semantics; pre-witness history influences nothing. |
| P4-066 | accepted — started-work proof is uniform for every terminal status including cancelled: a recorded post-setup transition (handshake success or running phase), never status alone. A pending-cancelled husk never counts. |
| P4-067 | accepted, dissolved — rows 5-6 (the cross-record damage fallback) are deleted with the damage surface; the table is rows 1-4 over qualifying one-record evidence, else infinite. |
| P4-068 | accepted — the overflow projection is exempt from the chain-closed filter by design: it names no chains, only a count pointing at the ledger, and its one-booking staleness joins the documented crash-contract lags. The detail lines remain filterable because their identities are in the durable bytes. |
