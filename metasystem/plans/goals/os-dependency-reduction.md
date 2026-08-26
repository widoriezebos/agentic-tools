# os-dependency-reduction

- State: claimed
- Intent: Remove or reduce OS/tool dependencies to the declared minimum: no perl, no python3, and a pinned inventory that refuses new external binaries undeclared (Wido 2026-08-24: remove or at least reduce as much as possible)
- Origin: human
- Next step: SLICE ONE LANDED e5300bf: twenty-four perl sites across five fixture suites became conf_edit (bash-3.2 awk, byte-oracle-proven incl. the two certified edges); perl is out of the dependency list. SLICE TWO next (3h): port the benchmark kit's python3 — extractor.py and its schema helpers — into Go engine verbs (the engine already validates JSON schemas for returns); the kit shell stays orchestration-thin and python3 leaves the dependency list. SLICE THREE after (45min): the dependency ratchet — the external-binary inventory as declared data, an undeclared command -v/exec refusing at the gate. Out of scope by ruling: git and the runtime CLIs stay.
- OpenedAt: 2026-08-24T15:41:03Z
- Revision: 4
- Claimed: machine=m2 lineage=mac-coordinator at=2026-08-25T20:38:12Z

History:
- 2026-08-24T15:41:03Z BEZ13078T604YC32GYRHD6SKQ2-m2-bc1be9cb open actor=human:wido targets=os-dependency-reduction
- 2026-08-25T20:38:12Z ZZQ1FGVHFEF3MWB6ZQGA0158XN-m2-bc1be9cb claim actor=m2+mac-coordinator targets=os-dependency-reduction
- 2026-08-25T20:39:27Z EDTZA5NFXD0X227F04YT7H5Z8K-m2-bc1be9cb edit actor=m2+mac-coordinator targets=os-dependency-reduction
- 2026-08-26T03:17:58Z 55P2DANY0VRRK0XNZWS240SHXH-m2-bc1be9cb edit actor=m2+mac-coordinator targets=os-dependency-reduction
Integrity: sha256=70b1e17311481da10f0c301f4ecac814a68e6c4e921294c8220b3d7b6fa04f77
