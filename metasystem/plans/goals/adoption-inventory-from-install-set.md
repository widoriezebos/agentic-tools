# adoption-inventory-from-install-set

- State: queued
- Priority: 2
- Sequence: 4
- Intent: In the root layout (the installation is the repository root) path ownership follows a hand-written shipped inventory in internal/stateroot/owner.go that names fewer files than scripts/adopt.sh installs (twenty-eight docs files and the runtime registration directories answer application-owned); replace the hand list with the set adoption actually installs, recorded at adoption time and read by the ownership oracle, so the path class verb, the critique waiver and the landing evaluator agree for every installed file in every layout; also answer directory queries for installed trees as installation-owned.
- Origin: main
- Next step: TIER 2 per R-54-m1 (mechanical logic inside an existing owner): build plus one code review, no design round. Origin: PCM-CC6-001 and PCM-CC6-003 of the path-class manifest's closing review (records under artifacts/agents/path-class-cc6); the manifest's design names this goal in its resolution paragraph. Shape: adopt.sh writes the installed set (one path per line) into the adopted repository; shippedInventoryPath reads it when present and falls back to today's list; the adoption tracer in scripts/agents/adopt-fixtures.sh proves the written set equals the tracer's expected set; owner tests cover a docs file adoption copies, an application docs file, and a bare directory query. Waits for human approval for execution.
- OpenedAt: 2026-09-03T11:02:27Z
- Revision: 2
- BudgetExceptions: 0

History:
- 2026-09-03T11:02:27Z FWPDGT2Z546RC77WY6WR27APWR-m1-7bb1546e open actor=m1+main-1788333680-2840-7f79f4 targets=adoption-inventory-from-install-set
- 2026-09-08T15:57:22Z ASKTWG7H5922246DAKSC07PHZJ-m1-7cd0bd60 set-priority actor=human:Wido targets=adoption-inventory-from-install-set reason=priority-order subject=adoption-inventory-from-install-set from=unranked to=2:4 requested-sequence=4
Integrity: sha256=536c855d11f2aa9771ffb441a3a9d33ba2493d469cfb1ee60919755f32ba3961
