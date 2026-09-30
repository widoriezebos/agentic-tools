package lane

// What landing publish reads of a batch (design r10 K3, K4, K6): the
// canonical series landing begin recorded and the proof attempt landing
// prove recorded. Both records are written by unit K-b; this is the minimal
// read interface publish consumes, so the two units meet at one seam.

import "errors"

// BatchBegin is a batch's durable begin record, as publish reads it: the
// series Base..Head (linear) and its tree, and the lane's own commits in it
// (Lane-Resolved replays and the Lane-Integration commit) that held
// accepts.
type BatchBegin struct {
	Batch       string   `json:"batch"`
	OpID        string   `json:"opId"`
	Members     []string `json:"members"`
	Base        string   `json:"base"`
	Head        string   `json:"head"`
	Tree        string   `json:"tree"`
	LaneCommits []string `json:"laneCommits,omitempty"`
}

// The proof subjects and outcomes publish reads.
const (
	SubjectBatch = "batch"
	OutcomeGreen = "green"
)

// ProofAttempt is one recorded proof attempt, as publish reads it: what it
// proved (subject, base, commit and tree), how it ended, and the engines
// that judged it and that the proven tree builds.
type ProofAttempt struct {
	Batch                 string `json:"batch"`
	Attempt               string `json:"attempt"`
	Subject               string `json:"subject"`
	Outcome               string `json:"outcome"`
	Base                  string `json:"base"`
	Commit                string `json:"commit"`
	Tree                  string `json:"tree"`
	PolicyEngineDigest    string `json:"policyEngineDigest"`
	CandidateEngineDigest string `json:"candidateEngineDigest"`
}

// PublishEvidence reads a batch's records for publish.
type PublishEvidence interface {
	// Begin is the batch's begin record.
	Begin(batch string) (BatchBegin, error)
	// Proof is the batch's latest attempt with subject batch.
	Proof(batch string) (ProofAttempt, error)
	// Verify re-verifies the attempt's retained evidence against the tree
	// it names, immediately before publication (K3).
	Verify(ProofAttempt) error
}

// ErrEvidenceUnavailable is evidence this engine cannot read: nothing is
// published without it.
var ErrEvidenceUnavailable = errors.New("this engine can't read the batch's begin and test records")

// NoEvidence is the reader of an engine without the begin and proof
// records: every read is unavailable, so publish publishes nothing.
type NoEvidence struct{}

func (NoEvidence) Begin(string) (BatchBegin, error)   { return BatchBegin{}, ErrEvidenceUnavailable }
func (NoEvidence) Proof(string) (ProofAttempt, error) { return ProofAttempt{}, ErrEvidenceUnavailable }
func (NoEvidence) Verify(ProofAttempt) error          { return ErrEvidenceUnavailable }
