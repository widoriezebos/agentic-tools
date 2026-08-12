# Dispositions: kill-shell plan, round 31

Job: design-critic-20260812t040533z-2dbd (codex gpt-5.6-sol, xhigh).
4 findings, 4 material, all accepted — by ending the family's
generating cause, as with the discriminator in round 19.

## The generating-cause resolution

Rounds 29-31 grew edges on a freshness-attestation system (source
digests, digest-build-digest equality, stamp interfaces, pre-binary
digest ownership) that the plan never needed: the rule exists so a
stale binary cannot carry old policy, and go-gate.sh already solves
that today by REBUILDING UNCONDITIONALLY — Go's incremental cache
makes it seconds, and the compiler itself is the only truthful
authority on its own inputs. The freshness contract is superseded
whole: the template bootstrap ALWAYS rebuilds before executing the
verb; the embedded stamp remains informational provenance, never an
oracle; and the publication protocol (markers, admitted flag, one
lock) still serializes who writes bin/metasystem. Nothing computes
digests, so nothing needs a lawful pre-binary digest owner.

| id | disposition |
| --- | --- |
| KS-R31-001 | accepted — dissolved: no digest-equality claim exists to be untruthful. |
| KS-R31-002 | accepted — dissolved: no allowlist pretends to be the compiler's input set; the compiler reads its own inputs on every rebuild. |
| KS-R31-003 | accepted — dissolved: nothing computes a digest, so no pre-binary owner is needed. |
| KS-R31-004 | accepted — the re-derivation wording is deleted with the family; the under-lock step is now: rebuild happened this invocation, claim lock, rename. |
