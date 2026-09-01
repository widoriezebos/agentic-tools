# Alert channel design-evidence spike — executable verdicts

Job implementer-142fd88a8c93640bc0f9969e (codex gpt-5.6-sol,
throwaway package internal/alertspike, tests run, nothing landed),
2026-09-01. Targets: the four open round-4 findings
(records/misc/alert-channel-critique-r10.md). Revision 11 folds these
verdicts as evidence; the critic's full return with test transcripts
is durable in the round evidence.

VERDICTS (from the return, condensed — the return is authoritative):

A design-evidence spike (package metasystem/internal/alertspike, tests only, nothing lands) prototyped the four disputed mechanisms of alert design revision 10. Verdicts: (1) AC9-JOB-ID-ABA-001, the birth token: REFUTED as designed - executed through the real shipped writers, createdAt is neither mandatory nor immutable, and the identifier-reuse ABA reproduces with identical digests; no shipped field qualifies (startedAt and claimEpoch are immutable but optional and caller-supplied; inode and file-birth change on every atomic record rewrite). Rule implied: every create path mints the birth token itself under the record lock, ignoring the caller's value, and the token joins immutableFields; a second-precision timestamp mint still collides on same-second reuse (keeping the disclosed clock-plus-grace-window lean), while a minted generation (timestamp plus nonce) passes the reuse test with no clock assumption. (2) AC9-SCAN-BOUNDEDNESS-001, the read-set bound: the producer scan SURVIVES with measured numbers (8.4ms filename index over 10,020 names with zero opens; 13.3ms for 1,000 whole-record reads), but the retained health path NEEDS RULE - it opened and decoded all 10,020 episodes in 110ms under what is in production the exclusive alert lock, and that set grows forever because producer episodes never auto-clear; the smallest mechanism is no new index at all: the existing filename grammar already separates classes, and restricting the health load to health-named files dropped it to 20 opens in 10.6ms. (3) AC10-STOP-CLEAR-READSET-001, the stop clear: the regression is real, REPRODUCED - a submitted stop episode stays uncleared forever after goal resume because the one-way filename cannot yield the old revision and a submitted attempt is never due for the pre-send recheck; rule implied and executed: a journal-time reversible marker alerts/stop-open/<goal>-r<revision> containing the digest, written before the episode, listed by the clear phase (bounded by open stop episodes, drains on clear), restores the clear with zero episode opens on the no-resume path. (4) AC9-ANSWER-FOLLOWUP-ACTION-001, remedy preconditions: rows 3 and 4 (and row 2 for the same roles) advertise a command that refuses categorically at journal time whenever the failed record's role is code-critic or warden, because the advertised line omits the mandatory --reviews target - rule implied: journal the failed record's own immutable reviews field and render it; row 1's follow-up is valid at journal time but refused after the unpinned completed chain root is collected (the pin covers only failed and timeout records while the command addresses the root - rule implied: pin every chain record the advertised command needs) or after a newer round lands in a non-accepted state; and the design's suffix-strip chain-root derivation is REFUTED against the shipped parentJob walk on a lawful task-r1 fixture - rule implied: adopt the parentJob-walk derivation at journal time.

## Evidence commands

- [ran] cd metasystem && GOCACHE=$TMPDIR/gocache GOTMPDIR=$TMPDIR go test ./internal/alertspike/ -run TestF1 -v
  -> All six birth-token probes passed against the real shipped record writers: RecordCreate persisted a record with no createdAt (the candidate is not mandatory); RecordCAS rewrote createdAt to 1999-01-01
- [ran] cd metasystem && GOCACHE=$TMPDIR/gocache GOTMPDIR=$TMPDIR go test ./internal/alertspike/ -run TestF2 -v
  -> Against 10,000 producer episodes, 20 health episodes, and 1,000 padded job records: the filename-only digest index took 8.4 milliseconds with zero file opens; the per-tick whole-record scan took 13.3
- [ran] cd metasystem && GOCACHE=$TMPDIR/gocache GOTMPDIR=$TMPDIR go test ./internal/alertspike/ -run TestF3 -v
  -> The regression reproduced: after a stop episode for goal revision 8 was journaled and its transport attempt submitted, a goal resume to revision 9 left the episode uncleared across three subsequent sc
- [ran] cd metasystem && GOCACHE=$TMPDIR/gocache GOTMPDIR=$TMPDIR go test ./internal/alertspike/ -run TestF4 -v
  -> Row 1 (follow-up) was accepted on the intact two-record chain at journal time, refused with 'no record file implx.json' after the unpinned completed chain root was collected (the design's pin covers o
- [read] sed -n '60,75p;222,272p;475,544p' metasystem/internal/dispatch/record.go
  -> Confirmed the critic's shipped-code citations: createdAt is absent from the immutableFields map (lines 60-75); RecordCreate validates only job identity and pending-setup status before persisting the c
- [read] sed -n '1687,1757p;1222,1231p' metasystem/scripts/agents/dispatch.sh && sed -n '43,69p' metasystem/internal/usage/usage.
  -> The follow-up gate requires the addressed root record file to exist (line 1698), refuses a closed chain (1724), gates on the newest chain record's status being completed or failed-with-protocol_error
- [ran] cd metasystem && GOCACHE=$TMPDIR/gocache GOTMPDIR=$TMPDIR go test ./internal/alertspike/ -count=1
  -> The whole spike suite passes in one uncached run: ok github.com/widoriezebos/agentic-tools/metasystem/internal/alertspike 1.796s. git status shows the only product writes are the five files under meta

## Spike-declared gaps

- The dispatcher script itself could not be executed even dry: scripts/agents/dispatch.sh runs its lease-entry check and fresh-census requirement before any of the preconditions under test, and the delegate sandbox has no live supervision to satisfy them. Finding 4's follow-up and fresh-dispatch gates are therefore a Go transcription of the cited precondition lines (1698, 1724-1757, 1226-1231) executed against fixtures, plus one real shipped function (usage.RootJobID). If revision 11 needs the refusals proven through the script's own process, the orchestrator must run that outside the sandbox, per the same KI-15 rule that applies to the validation suite.
- The latest-chain-record step in the finding-4 replay approximates the 'metasystem job latest-chain-record' verb as highest-existing <root>-rN record; I did not trace that verb's Go implementation, so an exotic chain shape it handles differently would not be covered by the replay.
