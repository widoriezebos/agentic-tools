# Dispositions: verification chain, round 2 (MV-2)

All nine accepted (IL-21 level: read). Folded into the consolidated design; per the human's ruling of 2026-08-07, implementation follows with the code-critic as the verifier at code resolution.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| MV-2-1 | accepted | Side effects preceded any sweepable record. | The job record is created first, in pending-setup, epoch-tagged; the claim sweep cleans stale setups. |
| MV-2-2 | accepted | Would-block proves contention, not ownership. | The wrapper writes a pid-tuple token; the guard verifies its own ancestry against it. |
| MV-2-3 | accepted | Root and round records cannot share one CAS. | protocolError lives on the round record, atomic with its terminal transition; the chain view is computed. |
| MV-2-4 | accepted | One counter, two jobs. | revision (every write) versus claimEpoch (claims only); ownership logic compares claimEpoch. |
| MV-2-5 | accepted | Adapter supervisors fell through to HUMAN. | The walk tests supervision and custody records; the matrix gains the ADAPTER SUPERVISORS row. |
| MV-2-6 | accepted | Only dispatch and commit had operation-spanning fences. | D-5a: one rule — every holder verb's terminal mutation is lease-locked with an epoch recheck. |
| MV-2-7 | accepted | Two literals reversed a settled adjudication. | One literal, unobserved; the shipped unreported migrates. |
| MV-2-8 | accepted | No grammar, no canonicalization, no per-key naming. | JSON filter grammar over a canonical flattened view; per-key value hashes name changes; malformed hashes all. |
| MV-2-9 | accepted | "Copies local configuration" was an unresolved interface. | Adapters declare local-config-paths; the helper copies exactly that, audited for links back. |
