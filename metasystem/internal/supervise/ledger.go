package supervise

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/registry"
)

// RegistryLedger implements Ledger against the machine-wide
// supervision registry (docs/design/supervision-registry.md): the owner's
// write-ahead relaunched/launched records and its terminal exited
// record. Every append holds the registry lock and frames per REG-1;
// this adapter is the owner's slice of that contract.
//
// The lock and framing live in internal/registry and internal/lock;
// this type composes them so the owner sees only the four Ledger
// methods and never the mechanics.
type RegistryLedger struct {
	// Append writes one framed record under the registry lock. The
	// binary supplies the lock-held append; tests supply a recorder.
	// It returns an error only when the record could not be durably
	// written — write-ahead gating depends on that (SLC-R6-006).
	Append func(record map[string]any) error
	// CheckoutPath is the canonical path stamped on every record.
	CheckoutPath string
	// OwnerTag is the claim these records belong to.
	OwnerTag string
	// now is injectable; nil means time.Now.
	now func() time.Time
}

func (l *RegistryLedger) at() string {
	clock := l.now
	if clock == nil {
		clock = time.Now
	}
	return clock().UTC().Format(time.RFC3339)
}

func (l *RegistryLedger) base(event string) map[string]any {
	return map[string]any{
		"schemaVersion": 1,
		"event":         event,
		"checkoutPath":  l.CheckoutPath,
		"at":            l.at(),
		"ownerTag":      l.OwnerTag,
		"engine":        "go",
		"engineBuild":   BuildStamp,
	}
}

// AppendRelaunched is the GATING write-ahead (SLC-R6-006): a failure
// here means launch_set launches nothing this cycle.
func (l *RegistryLedger) AppendRelaunched(generation int64, watcherTag, reaperTag string, retiredThrough int64) error {
	record := l.base(registry.EventRelaunched)
	record["generation"] = generation
	record["watcherTag"] = watcherTag
	record["reaperTag"] = reaperTag
	record["retiredThrough"] = retiredThrough
	if err := l.Append(record); err != nil {
		return fmt.Errorf("relaunched append (write-ahead): %w", err)
	}
	return nil
}

// AppendLaunched records one component's identity; a failure is
// retried by the owner at every observation (SLC-R6-006).
func (l *RegistryLedger) AppendLaunched(held Held) error {
	record := l.base(registry.EventLaunched)
	record["generation"] = held.Generation
	record["component"] = string(held.Component)
	record["pid"] = held.Identity.Pid
	record["pidStartedAt"] = held.Identity.StartedAtSec
	if err := l.Append(record); err != nil {
		return fmt.Errorf("launched append: %w", err)
	}
	return nil
}

// AppendExited is best-effort (REG-5): an owner that cannot speak must
// still die, so a failure is logged by the caller, never fatal.
func (l *RegistryLedger) AppendExited(reason, diagnosis string, teardownComplete bool) {
	record := l.base(registry.EventExited)
	record["reason"] = reason
	record["diagnosis"] = diagnosis
	record["teardownComplete"] = teardownComplete
	_ = l.Append(record)
}

// EncodeRecord frames one ledger record exactly as the registry
// expects — exported so the binary's lock-held append and the tests
// share one encoder.
func EncodeRecord(record map[string]any) ([]byte, error) {
	return json.Marshal(record)
}
