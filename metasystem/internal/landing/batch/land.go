package batch

// LandingProgress is the durable boundary between local construction, the
// single push, and trailer-based recovery.
type LandingProgress struct {
	Base          string            `json:"base"`
	Commits       map[string]string `json:"commits,omitempty"`
	BuildCommits  map[string]string `json:"buildCommits,omitempty"`
	BranchTip     string            `json:"branchTip,omitempty"` // legacy current-tip projection
	CandidateTip  string            `json:"candidateTip,omitempty"`
	ReceiptTip    string            `json:"receiptTip,omitempty"`
	HeldChecked   bool              `json:"heldChecked,omitempty"`
	PushComplete  bool              `json:"pushComplete,omitempty"`
	PushedTip     string            `json:"pushedTip,omitempty"`
	CleanupDone   bool              `json:"cleanupDone,omitempty"`
	RefusedOrigin string            `json:"refusedOrigin,omitempty"`
	RefusedBase   string            `json:"refusedBase,omitempty"`
	PushRejection *PushRejection    `json:"pushRejection,omitempty"`
}

func (progress LandingProgress) candidateTip() string {
	if progress.CandidateTip != "" {
		return progress.CandidateTip
	}
	return progress.BranchTip
}

func (progress LandingProgress) publishedTip() string {
	if progress.ReceiptTip != "" {
		return progress.ReceiptTip
	}
	return progress.candidateTip()
}

// PushRejection is the durable held state for a remote refusal that was not
// caused by a stale lease. The green proof remains valid while the endpoint is
// unchanged, so later ticks have no work to repeat.
type PushRejection struct {
	Text      string `json:"text"`
	At        string `json:"at"`
	OriginTip string `json:"originTip"`
}
