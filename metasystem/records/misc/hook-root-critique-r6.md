# Hook-root design critique — round 6 (revision 6)

Chain: revision 6 (landed 723231e66, sha256 77f3f59329a21749c5637f033d86157ce9baf09c4bb81238cabaedf6843bdac1) -> critic hrd-crit6-20260906 (Sol, design-critic, read-only runtime; the coordinator carried its return here verbatim). verdictMaterialCount=1.

## SHR-R6-DEADLINE-RESOLVER-FAIL-OPEN-01 — high, material=True

CLAIM: The round-five deadline finding remains open at its timeout-safety clause. Revision 6 correctly chooses deadline_engine as the parent's single engine, but makes a timely path state-root answer a prerequisite for every timeout refusal. If that call is slow, deadline_record stays empty and the parent emits the shipped record-failure allow line—the opposite of the parent contract for a worker overrun. This is not bounded by saying the worker was also slow: worker slowness is precisely the condition the deadline refusal exists to contain. It is also not durably visible because deadline_log_stop_outcome writes only when deadline_repo is non-empty. The resolver protocol adds another supported trigger: an absent session_id is defaulted by both live parent and worker, yet the design retains the resolver's `|| exit 1` and otherwise-shipped two-nonempty-lines harvest, so it can discard a good root and allow on timeout. Cases 11 and 13 always provide a session and deliberately keep the override's state-root query fast, so they prove the engine split but not this fail-open. If this finding stands, the build must change the parent resolver or timeout outcome so delayed root resolution and a supported missing session cannot turn a worker overrun into an unrecorded allow, and add fixtures for both paths.

EVIDENCE: Design section: Decision 1, “One engine in the parent,” at metasystem/plans/supervision-hook-root-design.md:625-719; the parent failure map at lines 945-949; cases 11 and 13 at lines 1220-1255; and residual risks at lines 1699-1716. Code evidence: metasystem/scripts/agents/supervision-hook.sh:28-31 promises a provider-level refusal; lines 76-94 default the session and resolve the current record; lines 95-109 suppress trail writes without deadline_repo; lines 110-120 require both harvested coordinates; lines 219-234 allow when the record is absent; and lines 307-308 establish that session_id is optional. The installed JSON command returned exit status 3 for an absent session_id. Build change: alter the deadline coordinate and outcome contract and extend metasystem/scripts/agents/supervision-fixtures.sh with slow-state-root and missing-session timeout proofs.

## Gaps the critic named

- The future path state-root verb and revision-six fixtures do not exist at the reviewed commit, so their before-and-after results were established from control flow rather than executed.
- No live fleet machine was available. Layout claims were checked against the repository fixtures, the landed member-A change, and the compiled state-root authority only.
- The declared new record metasystem/records/misc/hook-root-critique-r6.md could not be created because the runtime is read-only. This return is the complete record.
- Revision 6 does not reach the zero-material stopping point: one finding changes the deadline parent's allow-versus-block outcome. The other two round-five findings, the consumer sweep, hook-freshness fixture, operator post-Stop assertion, plain-health side effect, and re-arm landing require no further design change based on the evidence read.

## Coordinator disposition (m1, 2026-09-06)

Sixth revision, sixth critique, one material finding left, and the
critic states that everything else - the other two round-5 folds, the
consumer sweep, the hook-freshness fixture, the operator post-Stop
assertion, the plain-health side effect, the re-arm landing - needs no
further design change. Per the implementation-first ruling the finding
is built behind a fixture, not folded into a revision 7. The build brief
carries it as an obligation:

- SHR-R6-DEADLINE-RESOLVER-FAIL-OPEN-01: the deadline parent's timeout
  refusal never depends on the state-root answer. On a worker overrun the
  parent emits the block regardless of whether the resolver subshell
  answered. The refusal record is written to the resolved root when the
  answer arrived in time and otherwise to the installation's own
  supervision directory (the parent knows deadline_harness_root from its
  own path without any engine), and the record says the root was
  unresolved. The evidence line is written in both cases, never only when
  deadline_repo is non-empty. The session_id default stays as both live
  paths already have it; the resolver protocol does not add a trigger.
  Fixtures: cases 11 and 13 gain the variant where the state-root call is
  slower than the worker's wait: the Stop is blocked, the record lands
  under the installation, the trail line names the unresolved root.
