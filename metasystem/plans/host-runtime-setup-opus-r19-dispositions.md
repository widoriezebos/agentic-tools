# Host runtime setup: final code review dispositions

Claude Opus 5 at xhigh reviewed the complete final implementation from
`host-setup-recover1-r19`, tree
`dd7e23a14cbe3cc6bd6a1640a8ab6eb860758a7b`, and the separate outer registration
and instruction patch with SHA256
`74087d7fbcf86f6ac2a238214da04a8c59dfbdde4dc25713d3c9f3a9ce368de7`.
The canonical return is
`artifacts/agents/host-setup-opus-r19-review/rounds/1/return.json`.
Main read the complete return, findings, evidence and gaps. It reports zero
material findings and six nonmaterial observations. No source amendment is
required. This is the first round of this exact-tree review, and review stops
here under the zero-material stopping rule.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| HOST-C7-N01 | noted | The accurate dependency inventory records unchanged published Python fixture debt. Main's final actual audit passes, the generated Stop and receipt assertions now live in Go, and no new interpreter dependency is introduced. Existing unchanged fixture debt is not represented as eliminated. See recovery-r19-final-focused-results.json and the critic's independently reproduced base failure. | none |
| HOST-C7-N02 | noted | The refusal package contains declarations and has no executable production statements. Both ratchets retain every previously published floor. A hypothetical future addition of executable logic is outside this patch; current complete package coverage and the inventory checks remain required. | none |
| HOST-C7-N03 | noted | Host-dependent process fixtures are a real verification concern, already exercised by main under its actual Codex host. The complete Claude hook bed passed in 212.47 seconds; mixed-runtime, actual Devin repair, cold-host, forged-hint and detached-custody cases also passed. Exact production-source parity carries those results to R19. The final full battery remains pending and is required before landing. See recovery-r14-main-results.json, recovery-r8-runtime-native.log and recovery-r19-proof-parity.json. | none |
| HOST-C7-N04 | noted | A plain boolean could be more concise, but the current call and implementation have the specified behavior. This readability preference does not justify another source change or review round. | none |
| HOST-C7-N05 | noted | The supported verified platforms are macOS and Debian arm64, where the path separator is already a slash. Windows support is outside the declared platform contract. | none |
| HOST-C7-N06 | noted | Main already applied the exact user-instruction correction, blob 18c885e33bc9e6e547ea7863a995371eac96ac0e. Main will generate actual registrations through the reviewed setup owner and compare their bytes, modes and links with the separately reviewed artifact. The descriptor lists 45 generated entries in total, including the two root instruction files; they are not additional entries. No duplicate application of the instruction patch is planned. | none |

The critic independently checked all 2671 source blob hashes, the 56-path
conformance boundary, Go vet, unchanged coverage floors, canonical profile
copies and relative links, and the outer artifact digests. Native runtime and
socket proof remains main's responsibility and is identified separately in
`host-runtime-setup-verification.md`. A zero material count is not itself public
chain closure or a full-suite pass; those actual operations and their results
are retained under `artifacts/host-runtime-setup`.
