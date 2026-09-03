# adoption-inventory-from-install-set

- State: queued
- Intent: In the root layout (the installation is the repository root) path ownership follows a hand-written shipped inventory in internal/stateroot/owner.go that names fewer files than scripts/adopt.sh installs (twenty-eight docs files and the runtime registration directories answer application-owned); replace the hand list with the set adoption actually installs, recorded at adoption time and read by the ownership oracle, so the path class verb, the critique waiver and the landing evaluator agree for every installed file in every layout; also answer directory queries for installed trees as installation-owned.
- Origin: main
- Next step: TIER 2 per R-54-m1 (mechanical logic inside an existing owner): build plus one code review, no design round. Origin: PCM-CC6-001 and PCM-CC6-003 of the path-class manifest's closing review (records under artifacts/agents/path-class-cc6); the manifest's design names this goal in its resolution paragraph. Shape: adopt.sh writes the installed set (one path per line) into the adopted repository; shippedInventoryPath reads it when present and falls back to today's list; the adoption tracer in scripts/agents/adopt-fixtures.sh proves the written set equals the tracer's expected set; owner tests cover a docs file adoption copies, an application docs file, and a bare directory query. Waits for human approval for execution.
- OpenedAt: 2026-09-03T11:02:27Z
- Revision: 1

History:
- 2026-09-03T11:02:27Z FWPDGT2Z546RC77WY6WR27APWR-m1-7bb1546e open actor=m1+main-1788333680-2840-7f79f4 targets=adoption-inventory-from-install-set
Integrity: sha256=bdf5068ecaa8f3cbe784e50262071cad5c728f8488e967ad0c6e52664652e642
