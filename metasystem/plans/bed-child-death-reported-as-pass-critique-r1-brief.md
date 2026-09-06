Working Mode: implement
Orchestrator Identity: m1c (lineage main-1788680061-17829-64951c, dispatch delegate under goal bed-child-death-reported-as-pass, tier 3, hazard DESIGN-BEARING, code critique of chain bcd-build1)
Date: 2026-09-06

# Review brief: bed-child-death-reported-as-pass, round one

Round budget: three focused rounds for the goal's tier-3 box; this is
round one. The orchestrator adjudicates every finding; you edit nothing.

Threat model: one seat and its fixture beds on one macOS host whose only
bash is 3.2.57, no adversaries. In scope: a bed child that dies and is
still reported passed; a finished child reported failed; a signal
status lost or replaced; a scenario assertion weakened; a self-test
that could pass vacuously itself; a same-statement `local` declaration
that bash 3.2 expands before assigning. Out of scope: hostile inputs,
the engine, and the re-arm scenarios' own assertions.

Scope: the computed diff of chain bcd-build1 after round two (job
bcd-build1-r2) against its base. Round one changed nothing (it
gap-stopped on the trap's zero status); round two implemented
metasystem/plans/bed-child-death-reported-as-pass-fold-r2-brief.md
(landed 51a018c1), which binds. The computed diff is
metasystem/artifacts/agents/bcd-build1/rounds/2/diff.patch and its
reviewed tree is 24d426277ab18bbe01920433dbab62bc8f83054f; carry that
hash into your return exactly.

# Goal

Say whether the change ships a defect, violates its brief, or damages
what certifies it. Apply the materiality criterion verbatim:

> Would the change ship a defect, violate its brief, or damage what certifies it?

# What to attack

1. The sentinel: `fixture_child_completed=1` is the script's last
   statement and nothing after it can fail; every path a child can take
   to a normal end reaches it; no early `exit 0` in a scenario bypasses
   it in a way that now reads as a death (an explicit `exit 0` before
   the end would be reported as a death; say whether any scenario does
   that).
2. cleanup: the status rule (signal status first; unset flag with
   status 0 becomes 70 with the message; otherwise the captured status)
   runs before the body, and the body's evidence-keeping branch keeps a
   death's temp directory; the function ends with `exit "$status"` on
   every path but the second-entry return.
3. on_signal: records 128 plus the signal and calls cleanup; the exit
   status the parent sees for INT and TERM is 130 and 143; no path
   double-runs cleanup.
4. The two split declarations in install_rearm_engine and make_repo:
   derived names assigned on their own line after what they read;
   behavior otherwise identical.
5. The self-test: first in the list; its child does nothing but die on
   an unset variable under the trap; the parent passes it only on a
   nonzero status plus the "unbound variable" line and fails the suite
   on zero; ordinary scenarios keep the zero-passes rule; the failed
   list and its tail output still work for it.
6. Bash 3.2 cleanliness of every new line, and conformance: only
   metasystem/scripts/agents/supervision-fixtures.sh changed; nothing
   under plans.

# Evidence you may run

If your runtime gives you a shell, from the reviewed worktree root (the
metasystem directory), without arming anything:

- `bash -n ./scripts/agents/supervision-fixtures.sh`
- the direct child probe from the fold brief (bed-death-self-test child; expected rc=70)
- `bash --version`

If it does not, say so in gaps and review by reading; the orchestrator
ran the probe and the whole bed seat-side under the stock bash.

# Expected Return

The code-critic role's version-3 JSON with every required property,
the reviewed tree hash carried exactly, findings only, each with a
stable id, severity, a boolean material value, a plain-English claim
and evidence marked ran, read or inferred; one rigor row per material
finding. A round with zero material findings is the closure candidate;
say so plainly and list what you checked.

# Gap Rule

stop and report a gap; never fill it silently.
