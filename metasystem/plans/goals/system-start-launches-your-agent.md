# system-start-launches-your-agent

- State: queued
- Priority: 1
- Sequence: 5
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="Changes what system start does for a person at a terminal; an agent running system start must never spawn a nested agent; reverts cleanly with --no-agent or no config key"
- Tier: 2
- Intent: A person starts working in a checkout with one command: metasystem system start arms the machinery and then hands the terminal to the agent of choice (launch.agent in metasystem.conf.local, --agent NAME to override, --no-agent to only arm); the agent replaces the process so the person's shell stays its parent; only a person at a real terminal gets an agent, an agent or script running system start only arms; runtime-generic via the adapters
- Origin: main
- Next step: Write a short design (config key, flags, the person-and-terminal check, exec through the runtime adapter's launch command, how host setup ends with it, relation to the UI seat's unattended seat sessions), one critique round, then build; smallest thing that works Design note (ui seat, 2026-09-30): a person-started agent from system start keeps a person's lineage (not METASYSTEM_OWNER_LINEAGE=steward-seat, launch kind seat, which g1-s77 uses for steward-started opt-in seat mains), so the seat census counts it as a live main and the steward stays out of its way; the census already does this for an announced main.
- OpenedAt: 2026-09-30T16:55:50Z
- Revision: 3
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-30T16:55:50Z EWJPHN6H6ZEZSRZ86G6BZ9C4E5-m1e-b6a4eb0a open actor=human:wido targets=system-start-launches-your-agent
- 2026-09-30T16:56:00Z BNF1HFA4GASJX5EKB56DQM1FPA-m1e-b6a4eb0a set-priority actor=human:wido targets=adoption-filled-delivery-passes-on-trunk,beds-run-under-the-oldest-supported-bash,brief-declares-the-round-boundary,builder-proves-each-rule-by-mutation,busy-seat-shells-are-ended,critique-closes-on-folded-proof,cross-cutting-change-inventories-its-readers,delegate-sandbox-runs-the-beds,design-rounds-read-less-and-repeat-less,error-checks-use-typed-errors,every-round-gets-an-independent-read,evidence-and-build-output-have-retention-and-stay-unindexed,failures-show-observed-against-expected,fixture-runners-and-fakes-bound-their-own-life,fleet-doctor-repairs-what-stops-other-seats,follow-up-brief-cannot-cite-fresh-trunk-files,machinery-runs-unattended-on-codex,one-steward-per-checkout-on-its-own-root,proof-runs-reap-what-they-armed-and-a-census-names-the-rest,receipt-writer-follows-the-worktree-it-runs-in,round-proof-feeds-the-next-brief,seat-successor-continues-a-handoff-without-a-human,seats-spend-tokens-in-bounded-sessions,spend-fence-reports-tokens-per-model-and-cause,stop-hook-never-forces-an-empty-turn,system-start-launches-your-agent,terminal-enrollment-per-computer,testing-surfaces-declare-their-mirror reason=priority-order subject=system-start-launches-your-agent from=unranked to=1:5 requested-sequence=5
- 2026-09-30T18:21:56Z 1WV9W1D32SNSPMAKZ86CJK1VGM-m1e-b6a4eb0a edit actor=human:wido targets=system-start-launches-your-agent
Integrity: sha256=d003a3b147b11b937f13df350dcd97281fdd57e66dabf25424bfbd057072dc92
