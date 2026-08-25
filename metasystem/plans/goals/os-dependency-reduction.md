# os-dependency-reduction

- State: claimed
- Intent: Remove or reduce OS/tool dependencies to the declared minimum: no perl, no python3, and a pinned inventory that refuses new external binaries undeclared (Wido 2026-08-24: remove or at least reduce as much as possible)
- Origin: human
- Next step: Appetite: 4h across three slices. SLICE ONE RESCOPED 2026-08-25 (the two-one-liner estimate was wrong): perl appears at ~25 call sites across FIVE fixture scripts (adopt-fixtures, dispatch-fixtures, supervision-fixtures, delegate-caps-fixtures, mission-fixtures) plus the preflight-commands.sh dependency comment — all line-oriented conf substitutions/deletions and one insert-after-match, converted to a shared awk tmp+mv helper (portable: no sed -i divergence between mac and the VM); ~1.5h, codex-built, verified by the touched fixture suites. Slice two (3h): port the benchmark kit's python3 (extractor.py and schema helpers) into Go engine verbs. Slice three (45min): the dependency ratchet — the external-binary inventory as declared data, an undeclared command -v/exec refusing at the gate. Out of scope by ruling: git and the runtime CLIs stay.
- OpenedAt: 2026-08-24T15:41:03Z
- Revision: 3
- Claimed: machine=m2 lineage=mac-coordinator at=2026-08-25T20:38:12Z

History:
- 2026-08-24T15:41:03Z BEZ13078T604YC32GYRHD6SKQ2-m2-bc1be9cb open actor=human:wido targets=os-dependency-reduction
- 2026-08-25T20:38:12Z ZZQ1FGVHFEF3MWB6ZQGA0158XN-m2-bc1be9cb claim actor=m2+mac-coordinator targets=os-dependency-reduction
- 2026-08-25T20:39:27Z EDTZA5NFXD0X227F04YT7H5Z8K-m2-bc1be9cb edit actor=m2+mac-coordinator targets=os-dependency-reduction
Integrity: sha256=ac286c675ff3400d847a918b3112c692a4616c6f68fe79f687591540540a98c8
