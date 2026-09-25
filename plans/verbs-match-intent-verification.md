# Verification of the intent-based command redesign

The implementation candidate is `3e59d41077fe4cbfba6ad00f5b09f20e5c9da1a8`,
tree `88069098c0bdcc4e6de6fb703887dee35fe50be6`. Root independently verified
that source in a physical clone with separate Git administration. All product runtime obligations and final correction checks passed. The
commit containing this record integrates that source locally. It does not
install the executable into the running checkout or publish a remote branch.

All detailed artifacts below are retained under
`/Users/wido/LocalStorage/agentic-tools-evidence/verbs-match-intent-20260925`.
The complete accepted design and obligation matrix are in
[the design](designs/verbs-match-intent.md). The inventory contains
[23 related goals](verbs-match-intent-goal-index.md).

## Scope and authority

Wido authorized the complete public command redesign, root-authored design,
independent Fable or Opus design critique, subsequent implementation, and
machinery bypass. The stop hook is ignored as instructed. The goal ledger
remains truthful: no formal approval, claim, testing attempt, certificate or
production human authority has been invented for this verification.

Product implementation was authored by Opus 5.5. Independent Sol reviews and
root's source reads cover the component changes and integration resolutions.
Fable's design critique and the connection amendment are closed; each material
finding has a disposition and proof. Source provenance and exact launches
remain in the handoff and external evidence directory.

## Observed runtime behavior

| Behavior | What root ran and observed | Retained evidence |
| --- | --- | --- |
| Public CLI and compatibility | Final rebuilt engine passed 72 exact commands: all 48 verb help pages, human/agent/internal help, equivalent directory/file/symlink paths, conflicts before mutation, forged authority refusal, literal typo remedy, exact budget and parked resume, nested process readers, legacy and internal readers | `root-release-public-runtime-3e59.json` |
| Goal authority and lifecycle | Physical Git fixtures exercised explicit fake human authority, lawful agent conclusion, parked and stopped resume, unchanged budget and claim epoch, and partial results after publication | `root-runtime-lifecycle-r2.json`, `root-runtime-lifecycle-r2-records.json`, `root-runtime-parked-final.json` |
| Named work and continuation | Changed retained brief refuses build resume and wait; restored brief returns the same run without another launch | `root-runtime-work-final.json` |
| Unit lock release | An inherited file descriptor retains a close-only lock; explicit unlock releases it. The promoted deterministic regression passes | `root-unit-lock-descriptor-probe.json`; `internal/launch/unit_lock_test.go` |
| Generated worktree and builder | Actual OS child observes worktree cwd, brief, model and declared runtime settings; repeated invocation starts no second child; interrupted settings copy finishes on retry | `root-connection-final-runtime.log` |
| Committed critic | Actual branch-read owner, built delegate, dispatch script and Claude adapter launch a real fixture executable. Plain prose gains the default mode; selected critic and existing maximal-model authorization reach the worktree. Earlier candidates fail the same probes | `root-connection-brief-red.log`, `root-connection-brief-green.log`, `root-connection-final-delegate.log` |
| Connected delivery | The c78 command group passed the physical same-goal A/B build, fold A, real register/close, collection/publication and public batch admission journey; repeated admission adds no duplicate member | `verification-final-c78-repeat/native-result.json` and its command-interface-smoke logs |

Model executables, goal authority and expensive proof operations are declared
fixtures where used. Actual process, filesystem, Git, closure and publication
owners run at the boundaries named above. This does not certify production
terminal ancestry, the semantic quality of a live model's review, or a live
production landing. Later batch sealing and push are covered separately by
the existing batch lifecycle fixture, not by the same-subject admission claim.

## Selected release checks

The existing protected testing contract was computed from product base
`4ebc7c15726ef5f8d44f1d72b2231aefae11da05` and the exact candidate, using the
live goal's risk tuple `(3, 2, 3, 2)`. It selects **198 groups: 160 native and
38 existing shell sections**, deep mode, nine workers subject to the inherited
worker ceiling. Selection and all actual results are retained separately.

The external verification driver calls the real `proofrun.RunTestPlan` and
supplies the final built engine through `WithResourceCustodyExecutable`.
The first run omitted that argument and failed custody startup before tests;
`verification-final-70d13aa39/driver-diagnosis.txt` records why that run is
invalid. It contributes no passing or failing product evidence. The corrected
run is `verification-final-70d13aa39-r2`.

The section driver extracts the selected existing bodies without changing
their checks, removes the governed entry admission wrapper under the user's
bypass, retains checkout and collection guards, and records both source hashes.
It creates no governed receipt. Each selected body must exit zero and emit its
own matching passing row; completeness is checked separately.

The evidence is deliberately recorded by revision, rather than presented as
one green result on the final source:

| Execution | Observed result | Evidence directory or file |
| --- | --- | --- |
| Complete 160 native groups at `70d13aa39` | 152 passed; five fixture failures; three groups could not start without an external driver input | `verification-final-70d13aa39-r2/native-result.json` |
| Ten affected or connected groups at `c78a9c7c0` | All passed after seven test-file corrections and the driver correction; includes ordinary and batch-tagged command suites | `verification-final-c78-repeat/native-result.json` |
| All 38 original sections at `c78a9c7c0` | 32 passed and six failed; every selected section completed | `verification-final-sections-c78/section-results.json`, `driver-status.json` |
| Final fast/static prerequisites | Both passed | `verification-final-3e59-fast/native-result.json` |
| Final documentation and protocol sections | Both passed | `verification-final-sections-3e59/section-results.json` |
| Final public CLI walkthrough | All 72 commands passed | `root-release-public-runtime-3e59.json` |
| Final adoption bodies | All ten scenarios passed; driver exited zero | `verification-final-adoption-3e59/result.json` |
| Five final affected native groups | All passed; driver exited zero; complete collection | `verification-final-3e59-native/native-result.json`, `native-status.json` |
| Thirteen previously failed scenarios | All passed: ten landing, two goal completion and one supervision scenario; each driver exited zero | `verification-final-scenario-drivers-3e59.json` |

The source at `c78a9c7c0` changed only seven test files relative to the first
complete native run. Root read the patch and independently reproduced the
baseline child-wait failure before its fixture repair. Assertions and strict
Git and model/session checks remain intact. The earlier stop-cost performance
observation retains its original revision; its measured path and private
helpers did not change. The batch-tagged pass is likewise explicitly c78
coverage, not a final-revision tagged execution.

The final production corrections after c78 are confined to two public-command
wording checks in the existing doctrine audit and registration of the existing
fixture-authority flag for legacy `done`. The other changes are meaningful
owner tests, four explicit fixture callers and audit-facing prose. Root read
them all. The five final native groups cover the common parser, audit,
launch and branch owners and test-environment policy. The read-only source
analysis is `release-evidence-composition-final.md`.

Of the six original section failures, two were interrupted by stale audit
wording and the new ruling's reference. That same audit failure interrupted
ten landing scenarios and one supervision scenario. Two goal-completion
scenarios lacked explicit fixture proof and exposed the missing parser flag.
The adoption parent stopped at its full coverage prerequisite before any
scenario started; the complete race tests passed before that refusal.
Original passing scenarios remain evidence at their actual source revision.
Filtered reruns are not described as new complete section passes.

The adoption diagnostic preserved all ten scenario bodies, capabilities,
checks and cleanup. Its external wrapper omitted only parent proof-launch
and witness startup under Wido's bypass. It asserted no worker authority or
witness and issued no governed receipt. Both the unchanged-body extraction
and actual terminal passes are retained. The original failed adoption section
remains failed evidence.

Both affected owner packages passed the builder's race/coverage execution
above unchanged floors: launch 83.4839% against 82.8%; branch 78.3509% against
77.9%. These packages, including their tests, are byte-identical between the
coverage producer and final candidate. The final completion regression proves
that an exact configured fake root succeeds and ordinary or unconfigured
roots refuse without ledger effects; its mutation check fails without the
one-line parser correction.

Six unrelated coverage-ratchet problems predate this redesign:

- Missing floor registrations for unchanged `internal/rulings`,
  `internal/seat/launch`, `internal/ui/decisions` and `internal/ui/fleet`.
- HTTP UI coverage of 87.7%, below its 88.5% floor.
- UI session coverage of 88.8%, below its 89.5% floor.

Root ran both UI packages at baseline `4ebc7c157` with race and coverage.
All tests passed and the measured percentages exactly matched the candidate.
`release-baseline-ui-coverage-status.json` records the actual command and
successful exit. The six ratchet failures remain unresolved; no floor was
lowered. This release has diagnostic evidence under explicit machinery
bypass, not a green legacy full gate or a governed testing certificate.

No dependencies were added. Necessary process-wide fixtures remain serial;
eligible new tests were made parallel first. The command-package serial count
moved from 536 to 541 under the authorized bypass. No assertion, coverage
floor or other package baseline was weakened.

The final correction checks are complete. The local integration commit
contains a receipt and these completion records. The descriptive account
`final-diagnostic-release-account.json` links the actual terminal results and
their hashes; it is not a substitute for a governed certificate. The final
source manifest is `final-source-manifest-3e59.json`; correction decisions are
in `release-correction-root-adjudication.json`. Every original failed or
invalid run remains available beside its explicitly labeled replacement.
