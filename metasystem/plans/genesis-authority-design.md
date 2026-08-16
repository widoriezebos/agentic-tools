# Genesis authority: the sound fix is architectural (goal genesis-authority-design)

- Status: PARKED FOR HUMAN DESIGN DECISION (D86). Three review
  rounds (plans/genesis-authority-review.md) and one design round
  (plans/gad-critique-r1.md) converged on an IMPOSSIBILITY result,
  not a fix. The interim state is the D84 defense-in-depth code,
  which is validated working for legitimate provisioning.
- Goal: genesis-authority-design (parked)

## Why there is no filesystem-only sound fix

Genesis authorizes `goal reconcile` to seed a virgin target's
goal ledger. Every attempt to make it unforgeable failed for one
converging reason, proven across four critique passes:

1. **A notEnforced delegate needs no CLI at all.** Devin declares
   writeRoots/readRoots/network all notEnforced (runtimes.go): a
   rogue devin delegate can write goals-accepted.json directly,
   bypassing every CLI authority check. For it, genesis authority
   is not the weakest link — the missing OS sandbox is.
2. **Even a sandboxed delegate can read the control plane.** Both
   shipped presets grant readRoots ["."] = the repo toplevel, so a
   claude/codex delegate (writeRoots mapped) can still READ the
   main announcements (artifacts/agents/mains/*.json) and any
   secret key, copy the announcement into a worktree-local crafted
   root, and launder its classification to MAIN.
3. **A caller-supplied HMAC key self-verifies.** With no trusted
   key identity pinned independent of caller data, a delegate
   signs its own grant with its own key (gad-critique r1-002).

The common root: authority derived from filesystem state or from
classifying a caller-controlled root is forgeable by any process
that can read the repo and write a crafted root — which every
delegate can.

## The two real architectural directions (human's call)

Neither is a patch; both are substantial and deserve unhurried
design:

A. **OS-level delegate sandboxing.** Make the notEnforced runtimes
   actually enforced (the standing residual-waiver gap): a delegate
   that genuinely cannot read the control plane or write outside
   its worktree cannot forge or bypass. This is the same work the
   ExpectedEnvelopeEnforcement=notEnforced residuals already track;
   genesis soundness is downstream of it.
B. **An asymmetric trust anchor compiled into the binary, plus
   control-plane read-exclusion.** The verification (public) key is
   baked into the trusted payload binary; the signing (private) key
   lives outside every delegate read root (requires tightening
   readRoots to exclude the control plane, a permission-model
   change). Genesis then verifies a signature against the pinned
   public key — not a caller-supplied one — and only the source's
   held private key can mint. Depends on (A)'s read-exclusion to
   keep the private key unreadable.

## The honest interim posture

The D84 code (effective-class rule + goal-free guard under the
lock) is KEPT: it closes the accidental and contract-following
misuse cases and the legitimate provisioning path is validated
(the kit provisioning bridge passes). It is defense-in-depth, not
a boundary against a rogue same-user delegate — which is out of
scope for filesystem authority and belongs to direction A. This
matches the metasystem's existing honesty: enforcement it cannot
guarantee is marked notEnforced, never claimed.

Review history: plans/genesis-authority-review.md,
plans/gad-critique-r1.md.
