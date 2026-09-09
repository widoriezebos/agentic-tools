Working Mode: design
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, coordinator under goal landing-receipt-survives-records-drift)
Date: 2026-09-09

# Review brief: second read of the receipt-survives-drift design (revision 2)

FINDING IDS: chain-unique, LRE-01, LRE-02, ... never F-n.

## What you are reading, and why this read exists

`metasystem/plans/landing-receipt-survives-records-drift-design.md`,
revision 2, landed at commit f4abc699, sha256
b9795095aa99ab5272e5573737f19e5418382c0b761654b020bf736f96ef26c7. It folds
the first read, whose register with the coordinator's disposition is
`metasystem/records/misc/landing-receipt-survives-records-drift-critique-r1.md`.
Fold-read cycle 1: the first read found a critical hole (the shared stash),
the fold answered it, and this read decides whether the answer holds. If you
find something material, say so plainly; the coordinator will then say aloud
that this loop has no natural exit and take the land-or-fold call to Wido
before any third cycle.

Write your register as this new file: `metasystem/records/misc/landing-receipt-survives-records-drift-critique-r2.md`

Read-only design critique; implement nothing, run no bed.

## Settled, do not re-derive

Decisions 1 and 2 as landed in revision 1 and unchanged. The two corrections
to the orchestrator's brief. The withdrawal of the autostash.

## Mandate, in order of consequence

1. **Should the rare case exist at all?** The page grew from 548 to 1026
   lines, almost all of it Decision 3e's rare-case protocol (steps F and G:
   move, link a complete file, refresh, reset, restore, re-read), which runs
   only when origin changed a register during this landing's window AND the
   checkout holds a local unstaged append to that same register. The common
   case (step E) is one `git reset --keep` and opens no register. The
   alternative the page does not weigh: refuse the rare case loudly
   (`advance-register-contended` is already a refusal the page defines)
   and let the seat retry after the register-carriage landing that already
   exists for those bytes. Judge it against the code: what are the bytes at
   risk (background digest lines and receipt lines), who consumes them,
   and does any consumer need them preserved across a transport rebase
   rather than re-emitted? If refusing is sound, that removes the capture
   files, the link dance, the digest lock export, the two LOST windows and
   roughly half the page. If it is not sound, say what breaks.
2. **The common path E.** `git reset --keep N` with a clean index never
   opens a path N did not change: confirm from git's documented semantics
   and the page's own probe list. Confirm precondition A (index equals
   HEAD's tree) is checked at the toplevel and that the refuse codes cover
   a stage that lands between the post-commit check and the advance.
3. **If the rare case stays: the protocol F/G.** The link-a-complete-file
   step (F.2) was reasoned from a probe defect and not itself probed. Walk
   it against both writers: the digest under `narratordigest`'s flock
   (`metasystem/internal/narratordigest/digest.go` lines 96-115, an
   atomic replace) and the receipts log's lockless `O_APPEND`
   (`metasystem/internal/receipt/receipt.go` around 446). Are the two
   LOST windows the only ones, and are they bounded as claimed? Is the
   manifest resume after a crash sound, and can two concurrent landings on
   the same L collide in `artifacts/agents/landing/advance/<L>/`?
4. **Decision 4, the compatibility read.** "Raw equality implies filtered
   equality" and the claim that the version field makes a raw-versus-
   filtered misreading impossible: test both against
   `metasystem/internal/landing/receipt.go` as it stands, and the two
   mixed-form pins. Then the crossover procedure: `METASYSTEM_BIN` at a
   proof build of the candidate. What happens on each path if the seat
   forgets it, and is "the live binary fails on the unknown verb before the
   battery" true on the chain path given `land.sh`'s step order?
5. **The rule tables (3a).** Two ordered lists, one per mode. Walk the
   porcelain v1 states once more, including `MM`/`AM` on a register
   without the flag (tolerated only with the append shape against the
   index blob) and `TM`.
6. **The corrections not asked for.** The page now says transport does not
   need a clean worktree because `sync-transport.sh` pushes refs only,
   and that the post-commit check protects something else. Confirm against
   `metasystem/scripts/agents/sync-transport.sh` and say what the
   post-commit check does protect.
7. **Fixtures.** The sentinel-stash canary and the concurrent-writer
   canary: can each fail for the named reason on the untouched tree, and
   does the refusal canary now reach its refusal?
8. **The build boundary.** Name what the implementation map now spans (new
   verb, new gittree constructor, a narratordigest export, land.sh, the
   policy file, fixtures) and whether any of it widens beyond the goal's
   intent.

## Constraints

Wall-clock budget: 45 minutes. Return per the design-critic schema with the
sha256 above as the reviewed identity. Gap rule: stop and report a gap;
never fill it silently.
