# job-record-birth-token

- State: queued
- Intent: Every job record incarnation carries a mandatory, immutable, machine-minted birth token: the create path mints it under the record lock (timestamp plus nonce generation - a second-precision timestamp alone collides on same-second identifier reuse), ignores any caller-supplied value, and the field joins immutableFields. Proven necessary by executable spike (records/misc/alert-channel-spike-verdicts.md, 2026-09-01): no shipped field qualifies - createdAt is neither mandatory nor immutable through the real writers, startedAt and claimEpoch are optional and caller-supplied, inode identity changes on every atomic rewrite - and the alert design's retention pin (its critical identifier-reuse closure) depends on exactly this contract. Consumer: alert-escalation-channel revision 11; Ruling R applies - whoever builds this runs every reader of the record identity
- Origin: main
- Next step: Small mechanical item, 4h box (R-44-m0b): design note from the spike's implied rule, Sol critique, build in internal/dispatch record writers, every incarnation-comparison caller run
- OpenedAt: 2026-09-01T21:26:07Z
- Revision: 1

History:
- 2026-09-01T21:26:07Z YGRS58CS27XPHHVC7FAVCK8B9R-m0b-6638932d open actor=m0b+main-1788250419-3170380-8a1fb3 targets=job-record-birth-token
Integrity: sha256=f186416aaf12b4ab21488ab0e0a147471975da3eef64d8a5d46faa47bc84a6bf
