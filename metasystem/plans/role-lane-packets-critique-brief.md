Working Mode: design
Orchestrator Identity: m3 (lineage mac-m3, dispatch delegate under goal role-lane-packets)
Date: 2026-08-31

# Goal

An independent critique of plans/role-lane-packets-design.md (authored
this hour by the Fable design lane, job implementer-61d326ee25fbea6019b05a58).
The design encodes the R-25-m1 role lanes into role-packets.json and the
engine-side expectations, with refusal for wrong-lane launches.

# Workspace

The dispatch-created job worktree, branched from main. Read everything;
write nothing but your return. The design file to critique is
plans/role-lane-packets-design.md, landed on main at commit 13ab8563
and present in your worktree.

# Threat model (findings outside it close as out-of-scope)

1. CORRECTNESS AGAINST THE RULING: does the encoding preserve R-25-m1
   exactly — Sol builds and Claude critiques, Fable designs and Sol
   critiques — without freezing what R-28-m1 delegates (model tier and
   effort within a family, until 2026-09-30)?
2. ENFORCEMENT REALITY: does the refusal actually bind at a boundary
   that runs before budget spends, following the existing
   exact-equality hazard-table pattern? Could a launch reach a worker
   on a wrong lane through any path the design does not close (conf
   keys, mode scoping, canonical-model aliasing, follow-up dispatches)?
3. THE WARDEN CLAIM: the design's own declared weakest point — the
   warden fixed to the claude family by derivation, not by Wido's
   literal word. Judge whether the derivation holds and whether the
   fallback (roster-authority row plus closure-time same-family check
   in slice 1) is sound.
4. CONF-VERSUS-PACKET PRECEDENCE: is the migration story complete and
   is day-one behavior unambiguous for every existing conf key in
   metasystem.conf and metasystem.conf.local?
5. SLICEABILITY: is slice 1 truly at most 4 hours and independently
   deployable (R-17, the delivery law)?

# Constraints

- Round budget: ONE round. Findings ranked by severity; each finding
  carries the design text it refutes and the evidence.
- Do not redesign — a finding names what is wrong and why, not a
  replacement design (the designer revises; R-25b-m1 forbids anyone
  else's pen on the design).
- Wall-clock budget: 25 minutes.

# Expected Return

The design-critic role's version-2 return per its schema: findings with
severity, evidence, and the refuted text; an explicit verdict on each
of the five threat-model lines; your own self-grade per R-24-m1.

# Gap Rule

If the design file or a cited authority is absent from your packet,
stop and report the gap; never critique from memory.
