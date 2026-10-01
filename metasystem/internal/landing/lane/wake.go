package lane

// Why the landing agent would run now (simple lane §1): work is queued, a
// batch is unfinished, or a proof ended whose result the agent has not been
// woken for. While the lane's proof runs it is held instead: the agent that
// started the proof ended its turn and is woken when it ends. landing status --json carries these as "wake",
// and the keeper wakes the agent on the very same read, in process: no
// process reads another's text. An idle lane runs no model.

import (
	"errors"
	"fmt"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
)

// The wake reasons, in the order they are listed.
const (
	WakeQueued          = "queued"
	WakeUnfinishedBatch = "unfinished-batch"
	// WakeProofFinished is a proof that ended after the agent was last
	// started, while members still wait: the agent reads its result.
	WakeProofFinished = "proof-finished"
)

// ProofFact is the lane's proof as the wake reads it: the one running, or
// the last that ended.
type ProofFact struct {
	// Running is a proof whose process runs: it holds every agent start.
	Running bool `json:"running"`
	// Died is a proof recorded running whose process is gone without a
	// result: it holds nothing, and the next prove runs it again.
	Died    bool   `json:"died,omitempty"`
	Attempt string `json:"attempt,omitempty"`
	Tree    string `json:"tree,omitempty"`
	// Since is when the running (or died) proof started.
	Since string `json:"since,omitempty"`
	// EndedAt is when the last finished proof ended.
	EndedAt string `json:"endedAt,omitempty"`
}

// Wake is why the landing agent would run now. Reasons empty is an idle
// lane. Unread names each source that could not be read and why: an unread
// source is never a reason, and it is shown, never hidden.
type Wake struct {
	Reasons []string `json:"reasons"`
	Unread  []string `json:"unread"`
	// Proving is the lane's proof that runs: no agent starts until it
	// ends, whatever the reasons.
	Proving *ProofFact `json:"proving,omitempty"`
}

// WakeSources are the reads a wake is built from. Records nil reads the
// lane's batch records from its checkout.
type WakeSources struct {
	Records func(root string) ([]batch.Record, error)
	// Proof reads the lane's proof at the lane checkout; nil reads none.
	Proof func(root string) (ProofFact, error)
}

// ReadWake reads why the landing agent of the registered lane would run
// now; agent is the keeper's record of the agent it last started.
func ReadWake(record Record, now time.Time, sources WakeSources, agent AgentState) Wake {
	records, err := readRecords(ViewSources{Records: sources.Records}, record.Root)
	proof, proofErr := readProof(sources.Proof, record.Root)
	return wakeOf(records, err, proof, proofErr, agent)
}

func readProof(read func(string) (ProofFact, error), root string) (ProofFact, error) {
	if read == nil {
		return ProofFact{}, nil
	}
	return read(root)
}

func wakeOf(records []batch.Record, recordsErr error, proof ProofFact, proofErr error, agent AgentState) Wake {
	wake := Wake{Reasons: []string{}, Unread: []string{}}
	if recordsErr != nil {
		wake.Unread = append(wake.Unread, "batches: "+recordsErr.Error())
	}
	if proofErr != nil {
		wake.Unread = append(wake.Unread, "proof: "+proofErr.Error())
	}
	queued, unfinished := batchReasons(records)
	if queued {
		wake.Reasons = append(wake.Reasons, WakeQueued)
	}
	if unfinished {
		wake.Reasons = append(wake.Reasons, WakeUnfinishedBatch)
	}
	switch {
	case proof.Running:
		wake.Proving = &proof
	case queued && endedAfter(proof.EndedAt, agent.StartedAt):
		wake.Reasons = append(wake.Reasons, WakeProofFinished)
	}
	return wake
}

// endedAfter says whether a proof that ended at ended ended after the
// agent's last start (none: any ended proof is newer).
func endedAfter(ended, started string) bool {
	end, err := time.Parse(time.RFC3339Nano, ended)
	if err != nil {
		return false
	}
	start, err := time.Parse(time.RFC3339Nano, started)
	return err != nil || end.After(start)
}

// batchReasons reads the batch records: queued is a batch collecting with a
// member joined; unfinished is a batch past collecting that has neither
// landed nor dissolved (proving, landing, diagnosing, or held) and still
// has a member: a memberless one is the keeper's to settle, not the
// agent's.
func batchReasons(records []batch.Record) (queued, unfinished bool) {
	for _, record := range records {
		switch record.State {
		case batch.StateLanded, batch.StateDissolved:
		case batch.StateOpen, batch.StateSealed:
			if len(members(record)) > 0 {
				queued = true
			}
		default:
			if !batch.Memberless(record) {
				unfinished = true
			}
		}
	}
	return queued, unfinished
}

// keeperActor is the actor a keeper's settlement records in a batch's
// history.
const keeperActor = "landing-keeper"

// SettleMemberless settles as dissolved each batch of the lane at root that
// is past collecting and has no member left (batch.Memberless): nothing in
// it can be proven, pushed or returned, so no agent is woken for it. It
// lists the batches it settled; a batch it could not settle is in the error,
// and the rest are still settled.
func SettleMemberless(root string, now time.Time) ([]string, error) {
	records, err := readRecords(ViewSources{}, root)
	store := batch.NewStore(root, nil)
	settled := []string{}
	var failed []error
	for _, record := range records {
		if !batch.Memberless(record) {
			continue
		}
		done, settleErr := batch.DissolveMemberless(store, record.BatchID, keeperActor, now)
		if settleErr != nil {
			failed = append(failed, fmt.Errorf("batch %s: %w", record.BatchID, settleErr))
			continue
		}
		if done {
			settled = append(settled, record.BatchID)
		}
	}
	return settled, errors.Join(append([]error{err}, failed...)...)
}
