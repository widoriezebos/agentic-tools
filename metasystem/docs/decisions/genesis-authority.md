# Genesis authority: refused alternatives

- Kind: decision
- Id: 01M3MKDZM3BST9EXX9WXPD3317
- Status: accepted

The rule (`goal.AdoptionShaped` in `internal/goal/genesis.go`, and the genesis
arm of `internal/authority`) and its posture are documented in the code and in
ruling R-141-m1e (genesis is cooperative, not unforgeable). These are the
alternatives refused, with reasons, so they are not re-proposed. Distilled
2026-09-28 from `records/genesis/` (design §1 and §7, dispositions GA-R1-01
and GA-R3-04, review holes 1-2) and decision D120 of the 2026-08-13 decisions
log, all removed that day (tag `records-archive-2026-09-28`).

- Admit machinery unconditionally because a goal-free baseline is
  "worthless": refused. A Goal-free declaration carries a plans-stream digest
  the turn verdict reads as an all-clear, so a non-holder could launder
  arbitrary declaration bytes.
- Classify genesis against a second, caller-named source root
  (`--genesis-from`): removed. A crafted root with a copied announcement
  raised a delegate to MAIN, and discarding non-MAIN source results let an
  adapter-supervisor fall through to HUMAN on a virgin target.
- A per-user authority registry, or the ancestors' cwd, as the authority
  root: refused. Same-user writable, stale, and still refuses the lapsed
  session and the sandboxed kit gate.
- A capability minted by the source's holder: refused. The grant must name
  its source to be verified, the harness again chooses which root mints, and
  it certifies the source's holder, which the target ledger never checks.
- Requiring the declaration's digest to be current: refused. Anyone can
  compute it from filenames, and it refused adoption into targets that
  already carry plans/*.md; a stale digest only blocks the target's verdict,
  the safe direction.
- Refusing genesis BY CALLER CLASS: refused. It protected no invariant and
  broke lawful provisioning twice (D96; the kit on the Mac 2026-08-19).
- Open follow-up recommendation: adoption's skeleton ledger should be
  verb-written, not shell-written, with its digest over the target's actual
  plans set.
