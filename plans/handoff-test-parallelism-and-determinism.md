# Test parallelism and Git-free tests: current handoff

Updated 24 September 2026, 12:51 CEST. Continue `finish-test-repairs-and-integrate`; do not claim a new goal or expand scope. Root Astra coordinates Sol6 xhigh builders/readers. The user authorizes commits, pushes, tmux and bypassing broken machinery. Work remains active.

## Authority and safety

- Never read, echo, copy or diff actual `metasystem/metasystem.conf.local` (secrets).
- Preserve inherited changes, four zero launch caps, and append-only receipt/narrator logs.
- Ordinary tests must stub Git, including setup. Only narrow named native adapter tests may use it, with a concrete reason. Keep original behavior assertions and actual policy; stubs supply raw repository facts, not final verdicts.
- Mac suites: nine workers. Linux VM: six workers, six CPUs, 16 GiB. Keep inherited worker allowance intact. Full Linux validation is required for this goal's runner/process changes; subsequent work uses selective Linux checks.
- No new audit system, inventory, emulator or dependency. User wants a small, reliable test capability. Use existing seams.
- All builds/read clones are private `git clone --shared` checkouts. Frozen rollout trees are read-only. Never copy build caches. Never clear a shared/default Go cache; clear only an explicit inactive private cache. Preserve the pinned analyzer in `C/steward-deadcode-build/go-cache`.
- Do not repeat valid behavior tests for report packaging, path labels, or deletion of verified unreachable wrappers. Compile/static checks suffice for those edits. Rerun affected behavior for actual reachable changes.

## Paths

R = `/Users/wido/LocalStorage/GitHub/agentic-tools-m1e`
E = `/Users/wido/LocalStorage/agentic-tools-evidence/test-parallelism-20260921`
C = E + `/coordination/git-free-tests-20260923`
T = `/Users/wido/LocalStorage/transport/test-parallelism-20260921`
UI clone = `/Users/wido/LocalStorage/GitHub/agentic-tools-ui`

## Accepted and saved work

- Main/origin main: `761586c9734c78d9d8cbef1e7f8d30ff4e31db46`. Final integration has NOT happened.
- Accepted/staged: **229 batches, 1,451 original parent tests converted**. `C/git-conversion-count-live-229.json`. Provisional 212 old typed candidates remain, but there is no certified global remaining denominator.
- `C/accepted/<name>.json` and `.patch` preserve reviewed hashes, files, actual read/proof paths, counts and limitations. Root has read each accepted computed patch and independent report; focused proof is retained.
- Latest frozen snapshot: `E/rollout/parallel-final-candidate-20260923bw/source`, commit `759333a5a0abb41095936f3b3c8a682c36843b1b`, tree `2700a822cfedf7a3a650612cb3399578a4ee4788`. Next label `bx`.
- Remote backup verified 12:21 CEST: branch `backup/test-parallelism-20260924`, commit `1ca0518777afa45de3a934b021c01e63a958533f`, tree `d5fca050a90050f95622b421f293fed522ef444e`, through 222 batches / 1,440 parents. `C/remote-backup-20260924-latest.json` owns latest backup identity. Batches223–229 and this concise handoff are staged/pending backup.
- Backup uses an alternate index from the frozen commit plus explicit handoff/receipt/narrator paths, then normal fast-forward push and ls-remote verification. Main index stays intact. No secret path in tree.

Recent accepted batches: 211 validate-paththree; 212 batch-protected-two; 213 cmd-read-items-three; 214 cmd-brain-boot-six; 215 missionrunner-host-first-six; 216 batch-package-selection-four; 217 landing-receipt-reader-three; 218 missionrunner-host-four; 219 cmd-session-stop-two; 220 composed-bu215-dead-three (zero parents); 221 batch-dependent-five; 222 cmd-candidate-engine-four; 223 composed-bv222-compile-repair (zero parents); 224 cmd-batch-owner-seven (+7); 225 hcl03-bw223-site-refresh (zero); 226 cmd-candidate-engine-one-B (+1); 227 missionrunner-faulted-three (+3); 228 engine-binding-two-bw223 (zero); 229 parallel-ratchet-twelve-bw223 (zero).

## VM cleanup is complete

Measured actual host allocation for `~/.lima/metasystem-debian/disk`: **60.32 to 28.77 GiB, 31.55 GiB reclaimed**. Guest uses 29 GiB with 66 GiB free; Mac had 116 GiB free at 12:15. Removed disposable Go/Staticcheck caches and six confirmed orphan compiler directories, then sync/fstrim. Source, worktrees, evidence, tools and other apps preserved. VM remains running. Exact evidence: `C/vm-cleanup-20260924/{results.json,summary.md}`. Do not repeat blind cleanup.

## Current delegates and next decisions

1. **validate_and_anchor**: last full-run prerequisite. Exact twelve safe top-level `t.Parallel()` additions, five test files; SHA84cdf82d167f28d1dc8d39aa5fb08b17579eb0e64c92a785ff3c4750645df3ba. Root read full patch. Verifier ran all twelve plus parallel-ratchet: 13 pass, five package exits0, race/atomic, no skips. Builder cache denial before execution retained; verifier used clone-owned writable cache. Independent whole-patch/helper-safety read passed, zero material findings; accepted229. Unit `E/units/parallel-ratchet-bw223-prepared/source/metasystem/artifacts/parallel-ratchet-bw223/{builder,verifier}`. Snapshot bx and immediately launch full Mac9.
2. **proof_worker_conversion**: preparing next full Mac9 run via existing enrolled human terminal, exact candidate frontend and current goal. No source edits, no launch until root supplies bx identity. Memo `C/mac9-next-launch-az-timing-facts.md`. Do not reuse historical goal/lineage or pass internal-only --control-root. Ordinary `test run --root MAIN/metasystem --goal finish-test-repairs-and-integrate --tree TREE --mode auto --purpose delivery --cap-min 120 --all-groups --result OUT/result.json`. Main index must match frozen candidate. Use exact current source frontend; leave installed binary untouched. Need current wall time and actual concurrency, not more old numbers.
3. **dispatch_budget_resume**: inventory repair accepted228; now small read-only diagnosis why AZ configured9 but at most4 groups, source/request distinction between group slots and native test workers. No source changes or secrets.
4. **landing_composition**: metrics-three isolated builder starting at bv222 after second bounded fresh critique found no material issue. First critic failed to return usable report; not counted green. Root design `C/cmd-metrics-three-root-design.md`, three ordinary parents, five paths/450-line ceiling. Local branch failure occurs at endpoint fetch BEFORE Sweep. Preserve real Done/Authorize/policy with raw endpoint replies. Must not delay current whole-suite launch.
5. **counselor_conversion**: B accepted226; **steward_final_conversion**: faulted3 accepted227; both idle. No extra broad tests requested.
6. **carrier_correction**: branch-family scout complete; CheckPrintsKinds/SweepLists are small potential next pair. Root design not dispatched; StatusAbsentOrigin depends on endpoint seam. CheckFetchesCurrentOriginTip is retained native adapter. No edits yet.
7. **gaterun_conversion**: dependent-five accepted221, idle; last followup had thread-limit refusal, do not assume running.

Accepted226 root additionally checked captured filtered environment: `testingEnvironment` removes all Git variables, while actual commit/projection owner reads changed ambient Git environment; original foreign-environment case remains meaningful. Accepted227 retains actual Seal/Measure/wall/conclusion with independent anchored/current ledger bytes and missing instruments removed before reopening turn. Exact proof and independent reports are named in acceptance records.

## Validation and full-suite plan

- Full pinned staticcheck on bv222 initially failed on integration-unused os/exec in gate_test.go; that compile error also produced false U1000 claims about helpers with test callers. Accepted223 removes only unused import, redundant local assignment and equivalent fixture struct literal. Full staticcheck and batch compile-only then passed. Evidence `E/units/composed-bv222-compile-repair`.
- Stop audit through208+ passed (three added, none removed). Original top-level t.Parallel-loss scan throughbr210 found none; not a global parallel-runtime guarantee.
- Last complete Mac run was AZ161: **failed in55m33.6s**, 167 selected groups,142 pass/5 reuse/17 fail/3 blocked. Nine configured workers, maximum four concurrent groups observed. Do not claim measured speedup or optimal CPU use. Facts `C/full-az-mac-failure-facts.md`.
- HCL03 116 site replacements accepted225: exact source sites, no policy/code change, focused and full package passed. Fresh old-AZ focused checks on bw223 gave seven passes and two failures; inventory now fixed228, parallel-ratchet twelve-marker proof and independent read green, accepted229. No full suite active yet.
- After these focused prerequisites, start a selected full suite on a frozen candidate while independent conversions continue if safe. Use native input-identity reuse for unchanged groups; do not defer all whole-system observation merely until every remaining conversion finishes. Reprove changed execution identities, never blanket reuse.
- Then full Mac9/Linux6 validation with actual worker overlap and wall time, masked gate-fence/fail-open checks, matching retained proof, Main commit/push, installed/generic-app witness. Do not claim delivery before these pass.
- Last communicated18–22 CEST delivery range was LOW confidence. No reliable final ETA or current whole-suite runtime yet. User corrected root twice for quoting old55m measurement when asked about new implementation. Root acknowledged new runtime is unmeasured and prioritized immediate full run once two prerequisites pass.

## Review/provenance lessons for next compaction

- Candidate A review found lost cold-parent t.Parallel, mutable facts under fixed IDs, and incomplete commit request identity. All fixed; final key uses actual normalized argv/environment/stdin, excluding project-tree identity absent from actual commit input. Same actual request must give same synthetic OID. Native object serialization has separate small witnesses.
- Batch dependent-five initial builder edited shared frozen br210. Exact patch/proof preserved, root reversed only those two files and verified clean HEAD. Correction ran privately; native witness now uses the SAME declared A/B/C patch bytes as the policy fixture.
- Compile errors can make staticcheck's unused-function results incomplete. Repair compilation first, verify callers before deleting.
- Stale local wait85814ee3240c2c7c38b752f4e627211d (contract-nine already completed) was ended through `metasystem wait end`. `goal next` confirms current claimed goal. Never manually edit goal ledgers.
- The runtime exposes no manual compaction tool here. The automatic checkpoint was restored around12:40. Keep this note current and do not change launch caps.

Historical continuation notes through12:20 are preserved at `C/handoff-history-through-20260924-1229.md` and in remote backup commit1ca051877. This current note replaces repeated old status blocks; read history only for a specific missing fact.
