# ui-connects-to-a-running-agent

- State: approved
- Priority: 1
- Sequence: 6
- Risk: severity=2 novelty=2 exposure=2 accumulation=1 basis="Human control over running agents (severity 2); a new per-runtime adapter and a live view (novelty 2); every seat and runtime (exposure 2); used when a person looks in (accumulation 1)"
- Tier: 2
- Intent: From the UI a person can watch and talk to any running agent (Wido 2026-10-02: 'watching and talking to it is fine'), generically for every agent kind (seats, the landing lane agent, delegates) and every runtime, and every UI action is a verb (UI parity). Step 1, machinery only: talk through the existing agent inbox (agent ask / agent reply), shown per agent in the UI with a message box and its replies; watch through the machinery's own records of that agent (claims, commits, hand-ins, questions, reviews, landings, verbs run) as a live per-agent activity view. Step 2, optional: the detailed session stream via one small adapter per runtime. No take-over, no tmux dependence.
- Origin: main
- Next step: Design step 1 (verbs agent watch/say, the per-runtime session-stream adapter, the UI view on the fleet page), Astra critique, build
- OpenedAt: 2026-10-02T21:11:09Z
- Revision: 7
- Pinned: ui
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-10-02T21:11:54Z revision=7 opid=K5KQZBVS69G6E3DR97J38HP971-m1e-9c612d71 authority=proven digest=0d7b47751136135e0c08ed6f611e56a274f82a833f94113dc0f2516e291b2b6c episode=7

History:
- 2026-10-02T21:11:09Z 71EHXME1ZQTHJR4XME33T8GGG5-m1e-9c612d71 open actor=human:Wido targets=ui-connects-to-a-running-agent
- 2026-10-02T21:11:15Z Y6Z9K94NJ5P6TCQ0ER325MPQZ9-m1e-9c612d71 approve actor=human:Wido targets=ui-connects-to-a-running-agent
- 2026-10-02T21:11:21Z 8PJBM619PH904SFJVC3VVY6V9N-m1e-9c612d71 set-pin actor=human:Wido targets=ui-connects-to-a-running-agent
- 2026-10-02T21:11:27Z JWBQX6BFTF75PDPRW56J5R9TQV-m1e-9c612d71 set-priority actor=human:Wido targets=builder-proves-each-rule-by-mutation,busy-seat-shells-are-ended,cross-cutting-change-inventories-its-readers,evidence-and-build-output-have-retention-and-stay-unindexed,fleet-doctor-repairs-what-stops-other-seats,one-steward-per-checkout-on-its-own-root,receipt-writer-follows-the-worktree-it-runs-in,round-proof-feeds-the-next-brief,seat-works-without-a-person,spend-fence-reports-tokens-per-model-and-cause,stop-hook-never-forces-an-empty-turn,switch-on-trial,terminal-enrollment-per-computer,testing-surfaces-declare-their-mirror,ui-connects-to-a-running-agent reason=priority-order subject=ui-connects-to-a-running-agent from=unranked to=1:6 requested-sequence=6
- 2026-10-02T21:11:46Z 7D39P4ACE666M1JQMKTGFHHRS3-m1e-9c612d71 unapprove actor=human:Wido targets=ui-connects-to-a-running-agent reason=Wido 2026-10-02: watch and talk only; generic machinery first
- 2026-10-02T21:11:50Z DY1EDNVQ43ZJPN44MXXYZFKFS6-m1e-9c612d71 edit actor=human:Wido targets=ui-connects-to-a-running-agent
- 2026-10-02T21:11:54Z K5KQZBVS69G6E3DR97J38HP971-m1e-9c612d71 approve actor=human:Wido targets=ui-connects-to-a-running-agent
Integrity: sha256=283d800b8e3575428971678b35529e5221ca9113306bec3a99903750ddac19ec
