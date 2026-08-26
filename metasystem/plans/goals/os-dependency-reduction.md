# os-dependency-reduction

- State: claimed
- Intent: Remove or reduce OS/tool dependencies to the declared minimum: no perl, no python3, and a pinned inventory that refuses new external binaries undeclared (Wido 2026-08-24: remove or at least reduce as much as possible)
- Origin: human
- Next step: SLICE ONE LANDED e5300bf (perl out, byte-oracle-proven). SLICE TWO PARKED 6f7db4a+: the single critique round (a81a, nine material, critical EPC-01) proved an extractor-only port cannot remove python3 — it pervades the kit (pairs.py, compare.py, system-fingerprint.py, provision/staging blocks) and the honest cost is a multi-slice arc (Go/Python JSON canonicalization, a general schema validator, executable-ownership law). DECISION DRAFTED FOR WIDO: (a) fund the full kit port as its own arc, or (b) declare python3 a KIT-SCOPED dependency (the metasystem itself is already python-free after the kill-python landings) and close this item at the ratchet. SLICE THREE PROCEEDS NOW (45min): the dependency ratchet — a declared external-binary inventory and a gate check that refuses undeclared usual-suspect interpreters (perl, ruby, node...) in metasystem scripts, with python3 pinned to whichever scope Wido rules.
- OpenedAt: 2026-08-24T15:41:03Z
- Revision: 5
- Claimed: machine=m2 lineage=mac-coordinator at=2026-08-25T20:38:12Z

History:
- 2026-08-24T15:41:03Z BEZ13078T604YC32GYRHD6SKQ2-m2-bc1be9cb open actor=human:wido targets=os-dependency-reduction
- 2026-08-25T20:38:12Z ZZQ1FGVHFEF3MWB6ZQGA0158XN-m2-bc1be9cb claim actor=m2+mac-coordinator targets=os-dependency-reduction
- 2026-08-25T20:39:27Z EDTZA5NFXD0X227F04YT7H5Z8K-m2-bc1be9cb edit actor=m2+mac-coordinator targets=os-dependency-reduction
- 2026-08-26T03:17:58Z 55P2DANY0VRRK0XNZWS240SHXH-m2-bc1be9cb edit actor=m2+mac-coordinator targets=os-dependency-reduction
- 2026-08-26T04:15:07Z Q3Z2CQMT8685B9E1PVEJ965VMC-m2-bc1be9cb edit actor=m2+mac-coordinator targets=os-dependency-reduction
Integrity: sha256=950df0e75408609886c9e7905ef79f3160ed7ad6e9006f284d31c05924dabd20
