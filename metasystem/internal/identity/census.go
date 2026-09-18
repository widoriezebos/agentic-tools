package identity

// ProcessCensus is one snapshot of the machine's process ids and parent links.
// Parent links that the kernel did not disclose remain unknown.
type ProcessCensus struct {
	pids    []int64
	parents map[int64]int64
}

// Pids returns the process ids present in the snapshot.
func (census ProcessCensus) Pids() []int64 {
	return append([]int64(nil), census.pids...)
}

// Parent returns the normalized parent recorded for pid in the snapshot.
func (census ProcessCensus) Parent(pid int64) (int64, bool) {
	parent, present := census.parents[pid]
	if !present || parent < 0 {
		return 0, false
	}
	if parent == 0 || parent == pid {
		return 0, true
	}
	return parent, true
}
