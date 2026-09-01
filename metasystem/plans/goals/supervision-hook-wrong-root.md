# supervision-hook-wrong-root

- State: queued
- Intent: The harness supervision hook resolves the wrong repository on nested checkouts: run from metasystem/ inside the agentic-tools-m3 clone it derives the git toplevel (the outer repo) as its metasystem root, reports a bootstrap world (no ledger, no steward), and its turn evidence never lands where health's hook-freshness role reads - m3 has hook-freshness=dead since enrollment with the hook firing every turn. DONE means the hook resolves the metasystem project root deterministically on nested checkouts, its turn evidence lands, and hook-freshness goes alive, proven by a fixture running the hook from a nested layout
- Origin: main
- Next step: Appetite: 1h, full ladder per the 2026-09-01 law: design (root-derivation rule - likely the resolution scripts/metasystem-config.sh uses, never git toplevel), design critique, build, code critique, tests. EVIDENCE, two machines independently: m3 diagnosed 2026-09-01 - running 'bash scripts/agents/supervision-hook.sh claude stop' from metasystem/ prints health for the OUTER repository root verbatim (bootstrap world, no ledger, no steward) while the real project is the nested checkout; m2 confirmed the same hour that its hook-freshness is dead identically ('no hook turn generation is recorded'). It is the SHAPE, not one machine: every seat's hook has been reporting the wrong repository and its turn evidence never reaches the health role that reads it. Impact: hook-freshness has been dead since enrollment on both seats, so the harness turn signal the health surface depends on has never once been true
- OpenedAt: 2026-09-01T07:25:56Z
- Revision: 2

History:
- 2026-09-01T07:25:56Z HJPEPF3NATCRT1F2FE5080H6S6-m3-a5da21ff open actor=m3+mac-m3 targets=supervision-hook-wrong-root
- 2026-09-01T08:36:18Z M4Y1ZAC9GBG995JWNQZX6MFE6Z-m3-a5da21ff edit actor=m3+mac-m3 targets=supervision-hook-wrong-root
Integrity: sha256=126b96b710b3dfb0ee2b57c9c4dc0468851a41785030a7dd25d18161bf52a90c
