# Evidence examination: Fable critique dispositions

Design: [Examine existing evidence before repeating tests](designs/examine-evidence-before-retesting.md).
Author and adjudicator: Codex coordinator. Independent critic: Fable 5.1.
User request: September 26, 2026, tracked design and goal, explicitly Fable critique.

## Round 1

Launch `20260926t073146-b3c6b556c9`, Claude session
`89620448-ca6f-48b9-93b0-e18bdcabb9e8`, returned normally with actual exit zero.
The [complete critique](examine-evidence-before-retesting-review-r1.md) found
three material issues. Its input is preserved in commit
`ed653ba6962eb8b845f7af4c896dbe339e0627b0`, SHA256
`71da354b48988662ecbaa1f2554c2c009b6f5136c61a1f15d93393754f1fc6c5`.
Root read the full findings and checked the cited closure, coverage and result
owners. The three findings are all dispositioned below.

| Finding | Disposition | Design correction and rationale |
| --- | --- | --- |
| ER-C1: manual closure blocks automatic test flow | Accept the defect; refine the remedy. | Evidence-kind examination completes at the authenticated examiner's terminal return, collected and bound by the review owner. It does not use author dispositions or fabricate an empty ordinary code/design finding register. Existing ordinary closure remains unchanged. Typed `execute` decisions are residual proof, not dismissible findings. This removes the first-use deadlock and preserves independent judgment. |
| ER-C2: failed-parent versus component contradictions | Accept. | Explicit component precedence: its own failed parent does not contradict a passing child; newer component failures/incompleteness, opaque failed parents, relevant live producers and ambiguous ordering block credit. Detailed parents constrain their actual components and parent checks. Scope comparisons to the relevant execution context; unrelated work is not a global veto. Partial runs have aggregate `partial`, never `pass`; structured child completion follows reaping/cleanup. |
| ER-C3: full root digest prevents audit-only reuse | Accept the defect; reject a projection as sufficient proof of every runtime input. | Add a deterministic `CoverageCodeIdentity` through the existing manifest/discovery owners, requiring equality of Go source/tests/testdata/embedded assets, dependencies/toolchain, instrumentation, inventories, ratchet and platform. Retain the full old/current delta: outside-projection changes still require examiner reasoning about actual consumers and invocation, otherwise fresh coverage. Go tests can read arbitrary external files, so a path projection alone cannot establish complete measurement-input equality. Old numerical observations retain their revision; old whole-attempt reuse stays strict. |

The ER-C3 choice is deliberate: automatic exact coverage carry based only on a
smaller file list would reintroduce the brittle dependency assumption Wido
asked us to avoid. Code identity is a necessary guard; independent applicability
examination handles the remaining changed inputs. This is also why a changed
script is not automatically dismissed as irrelevant.

## Additional suggestions from round 1

All seven were considered. They require no new role or generic framework:

- Native success: included as a required owner-derived terminal observation;
  pending coverage or a caller-supplied success flag cannot complete it.
- Examiner budget: existing code-review configuration and approved admission
  supply the bound for both verbs; attachment spends no new admission.
- Subject purity: preserve the existing freshness episode/binding and expiry;
  never add invocation time or a random episode to request identity.
- Residual scenarios: use the parent named-selection entry with its reaper
  and cleanup, not a direct child path.
- Citation scope: references resolve only within the frozen observation set.
- Examiner role: explicitly use `code-critic` with evidence-kind subject/return.
- Schema rollout: require compatible worker/verifier installation before
  enabling the new path; old engines refuse the new representation.

Fable's smallest-slice observation supports retaining the complete first path;
deeper deterministic per-scenario dependency selection remains deferred to its
existing goal. No product implementation or runtime proof is claimed here.

## Review boundary

The second and final round continues the same Fable session and checks the
revised design, these dispositions and any remaining step-1 failure. This is a
recorded direct critique under the user's requested model and existing machinery
bypass, not a fabricated registered critic-chain closure or implementation
acceptance. The design remains `draft` until human acceptance.

## Round 2 and final disposition

Launch `20260926t074515-7e5926f0be` continued the same Fable session and returned
normally with exit zero. The [complete final critique](examine-evidence-before-retesting-review-r2.md)
reports **zero material findings**, resolving ER-C1, ER-C2 and ER-C3 on their
substantive terms. Its reviewed design SHA256 is
`a39d39ffa3b58895b5a99292d74e4b8582634e2dd8509f96b1d2b3f7ada466cb`.
Root agrees with those dispositions after reading the full report.

ER-C4 is accepted as a required mechanical correction: the named existing
reader bound did not exist. The final design explicitly adds
`role.code-critic.evidence.tool-calls`, shipping 24, through the existing
configuration owner. Both verbs use it to fill the brief; absent, malformed or
non-positive values refuse before dispatch. The normal approved review budget
continues to control admission. This resolves the missing implementation choice.

Root also made the final round's concrete obligations explicit: verifier-derived
completion, fresh/independent session, complete decision map, immutable return
digest and enumeration of every outside-projection changed path. These refine
ER-1, ER-2 and ER-3 fixtures without adding an owner or protocol layer.

Of the two workflow alternatives, step 1 uses a dedicated evidence-kind subject
and performs its examination before heavy proof-attempt admission. Combining
commit and evidence subjects is deferred. Both public entry points still attach
to the same evidence examination. The intermediate-identity failure boundary
remains a recorded examiner judgment, as described in the design and final review.

The final design SHA256 is `049243fd0cdde457d68193df5918a6d101c308d801c207c2cec80f1812669fe6`.
Its post-review changes are the mechanical ER-C4 correction and the explicit
fixture/workflow choices listed above. No third review was run: the declared
failsafe was round 2, the critic reported no material remainder, and root found
no new shape-level defect. This distinguishes the exact reviewed bytes from the
final clarified design instead of claiming Fable reread it.

All four finding IDs have dispositions. The design is ready for implementation
planning and human acceptance; it remains `draft`, and product implementation
has not started. The complete design, both critiques, dispositions and handoff
are tracked inside MetaSystem. No external evidence directory is required to
understand what to build or why.
