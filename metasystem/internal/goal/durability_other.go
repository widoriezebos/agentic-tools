//go:build !linux

package goal

func (Endpoint) syncPublished(commit, recoveredOp string) string { return "" }
