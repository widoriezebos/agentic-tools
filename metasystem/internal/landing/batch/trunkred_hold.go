package batch

// TrunkRedHold records a durable trunk-red observation for a batch.
type TrunkRedHold struct {
	Opid        string     `json:"opid"`
	Opids       []string   `json:"opids"`
	Red         TrunkRed   `json:"red"`
	Entries     []EntryRef `json:"entries"`
	RecordedAt  string     `json:"recordedAt,omitempty"`
	CheckedTree string     `json:"checkedTree,omitempty"`
}
