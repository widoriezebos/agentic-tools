package host

import (
	"fmt"
	"sort"
)

// RecollectParams is the runtime-neutral input to a delivery
// recollection: re-collect a host turn's delivery past the candidates
// the runner already rejected (agnosticism audit class 5 — the
// session-fault gate, the one-resume limit, validation, and the
// original-error fallback all stay in missionrunner; only the
// runtime-specific recollection lives behind this table).
type RecollectParams struct {
	Root           string
	TurnRecordPath string
	TurnDir        string
	Workspace      string
	RejectDigests  []string
}

// RecollectResult is the neutral output: the qualified reply path, or
// Delivered=false when nothing further qualified.
type RecollectResult struct {
	Delivered bool
	ReplyPath string
}

// RecollectFn is one runtime's registered recollection.
type RecollectFn func(RecollectParams) (RecollectResult, error)

// deliveryRecollectors is the host seam's typed capability table,
// registered from per-runtime seam files at init.
var deliveryRecollectors = map[string]RecollectFn{}

// RegisterDeliveryRecollector registers a runtime's recollection; a
// duplicate runtime key is a declaration bug and panics at init.
func RegisterDeliveryRecollector(runtime string, fn RecollectFn) {
	if fn == nil {
		panic(fmt.Sprintf("nil delivery recollector for %s", runtime))
	}
	if _, dup := deliveryRecollectors[runtime]; dup {
		panic(fmt.Sprintf("delivery recollector for %s registered twice", runtime))
	}
	deliveryRecollectors[runtime] = fn
}

// DeliveryRecollector returns the runtime's registered recollection,
// or nil when the runtime declares none — the caller's signal that no
// resume path exists.
func DeliveryRecollector(runtime string) RecollectFn {
	return deliveryRecollectors[runtime]
}

// DeliveryRecollectorList is the read-only conformance view.
func DeliveryRecollectorList() []string {
	var names []string
	for runtime := range deliveryRecollectors {
		names = append(names, runtime)
	}
	sort.Strings(names)
	return names
}
