# ui-connects-to-a-running-agent

- State: queued
- Priority: 1
- Sequence: 6
- Risk: severity=2 novelty=2 exposure=2 accumulation=1 basis="Human control over running agents (severity 2); a new per-runtime adapter and a live view (novelty 2); every seat and runtime (exposure 2); used when a person looks in (accumulation 1)"
- Tier: 2
- Intent: A person can connect to any running agent from the UI as if opening its terminal (Wido 2026-10-02), generically: for every agent kind (seats, the landing lane agent, delegates) and every runtime (Claude, Codex, others) through one small adapter per runtime, and every UI action is a verb (UI parity). Step 1: watch a running agent live (its session stream: what it reads, runs and decides) and talk to it (a message into its inbox, answered in the same view). Step 2: take over: pause the agent and open its own session interactively in a browser terminal (the runtime's resume), and hand it back so the steward continues from there. No tmux dependence.
- Origin: main
- Next step: Design step 1 (verbs agent watch/say, the per-runtime session-stream adapter, the UI view on the fleet page), Astra critique, build
- OpenedAt: 2026-10-02T21:11:09Z
- Revision: 5
- Pinned: ui
- BudgetExceptions: 0

History:
- 2026-10-02T21:11:09Z 71EHXME1ZQTHJR4XME33T8GGG5-m1e-9c612d71 open actor=human:Wido targets=ui-connects-to-a-running-agent
- 2026-10-02T21:11:15Z Y6Z9K94NJ5P6TCQ0ER325MPQZ9-m1e-9c612d71 approve actor=human:Wido targets=ui-connects-to-a-running-agent
- 2026-10-02T21:11:21Z 8PJBM619PH904SFJVC3VVY6V9N-m1e-9c612d71 set-pin actor=human:Wido targets=ui-connects-to-a-running-agent
- 2026-10-02T21:11:27Z JWBQX6BFTF75PDPRW56J5R9TQV-m1e-9c612d71 set-priority actor=human:Wido targets=builder-proves-each-rule-by-mutation,busy-seat-shells-are-ended,cross-cutting-change-inventories-its-readers,evidence-and-build-output-have-retention-and-stay-unindexed,fleet-doctor-repairs-what-stops-other-seats,one-steward-per-checkout-on-its-own-root,receipt-writer-follows-the-worktree-it-runs-in,round-proof-feeds-the-next-brief,seat-works-without-a-person,spend-fence-reports-tokens-per-model-and-cause,stop-hook-never-forces-an-empty-turn,switch-on-trial,terminal-enrollment-per-computer,testing-surfaces-declare-their-mirror,ui-connects-to-a-running-agent reason=priority-order subject=ui-connects-to-a-running-agent from=unranked to=1:6 requested-sequence=6
- 2026-10-02T21:11:46Z 7D39P4ACE666M1JQMKTGFHHRS3-m1e-9c612d71 unapprove actor=human:Wido targets=ui-connects-to-a-running-agent reason=Wido 2026-10-02: watch and talk only; generic machinery first
Integrity: sha256=ddd21c2e77215dbe073262ce20eb3a953705153edb185304e329539f9c7182f0
