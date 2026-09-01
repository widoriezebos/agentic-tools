# supervision-hook-wrong-root

- State: queued
- Intent: The harness supervision hook resolves the wrong repository on nested checkouts: run from metasystem/ inside the agentic-tools-m3 clone it derives the git toplevel (the outer repo) as its metasystem root, reports a bootstrap world (no ledger, no steward), and its turn evidence never lands where health's hook-freshness role reads - m3 has hook-freshness=dead since enrollment with the hook firing every turn. DONE means the hook resolves the metasystem project root deterministically on nested checkouts, its turn evidence lands, and hook-freshness goes alive, proven by a fixture running the hook from a nested layout
- Origin: main
- Next step: Appetite: 1h, full ladder per the 2026-09-01 law: design (root-derivation rule - likely the same resolution scripts/metasystem-config.sh uses, never git toplevel), design critique, build, code critique, tests. Diagnosed 2026-09-01 morning on m3: manual hook run from metasystem/ printed health for the OUTER root verbatim
- OpenedAt: 2026-09-01T07:25:56Z
- Revision: 1

History:
- 2026-09-01T07:25:56Z HJPEPF3NATCRT1F2FE5080H6S6-m3-a5da21ff open actor=m3+mac-m3 targets=supervision-hook-wrong-root
Integrity: sha256=213c67e18c9c479e16e95a96c5fff82637b9ff65d624e0894942b3759ce65f69
