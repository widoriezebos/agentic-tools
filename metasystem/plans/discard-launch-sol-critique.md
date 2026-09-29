**SOL-DL-01 — High.** Discard can be lost or can overwrite a retry’s newer record. **Evidence (read):** [record.go:Discard](/Users/wido/LocalStorage/GitHub/agentic-tools-discard/metasystem/internal/seat/launch/record.go:395) loads and rewrites the whole record; [ui_launch.go:launchRecordFor](/Users/wido/LocalStorage/GitHub/agentic-tools-discard/metasystem/cmd/metasystem/ui_launch.go:130) independently loads that record before Retry saves it. If both load the failed record, whichever saves last can erase the discard mark or replace the retry’s `starting` state. **Smallest fix:** serialize read, check, and write for Discard and Retry on the same launch record, including subsequent launch writes that could replace the mark.

**SOL-DL-02 — Medium.** A joined machine can show an enabled Dismiss button that the server refuses. **Evidence (read):** [FleetPane.tsx:TheFleet](/Users/wido/LocalStorage/GitHub/agentic-tools-discard/metasystem/internal/ui/web/_app/src/fleet/FleetPane.tsx:338) treats a machine row as joined even while its launch record says `running`; [LaunchCard.tsx:LaunchCard](/Users/wido/LocalStorage/GitHub/agentic-tools-discard/metasystem/internal/ui/web/_app/src/fleet/LaunchCard.tsx:58) then shows Folded’s Dismiss, while [record.go:Discard](/Users/wido/LocalStorage/GitHub/agentic-tools-discard/metasystem/internal/seat/launch/record.go:410) refuses it. Presence confirmation can still be in progress at that point. **Smallest fix:** keep the running card in its full state; show Folded’s Dismiss only once the record has stopped changing.

**Deferred:** The checkout hand retains the shared origin and write checks. Showing the next older card after a discard follows the brief’s card selection rule. Tests were not run during this read-only critique.

VERDICT: 2 material

Codex session ID: 01a0ebc4-e071-7650-a358-194135e1631a
Resume in Codex: codex resume 01a0ebc4-e071-7650-a358-194135e1631a
