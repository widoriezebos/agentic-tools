package registry

import (
	"fmt"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// REG-3 compaction: rewrite the file as the fold's output. Retention
// is decided over the REDUCTION, but performed as a FILTER over the
// original frames — a kept record keeps its exact bytes, so
// compaction can never corrupt what it retains.

// CompactFrames returns the frames the contract requires to survive:
//
//   - live claims: `arming`/`armed` plus every UNRETIRED generation's
//     `relaunched`/`launched` records (SLC-R8-002: a generation may be
//     dropped only when a later relaunched covers it via the
//     contiguous retiredThrough watermark, SLC-R9-003);
//   - sweepable closed claims until swept: the same, plus the terminal;
//   - bound unreleased custody TOGETHER WITH its bound claim's full
//     reduced skeleton — opening records, terminal, and `swept` if
//     landed — even when that claim closed clean (SLC-R9-006,
//     SLC-R10-001: dropping the terminal reopened a phantom claim);
//   - unbound custody still inside its grace window (SLC-R6-002).
//
// Everything else is dropped: clean closed claims, released custody,
// torn markers, tolerated fragments.
func CompactFrames(frames []Frame, now time.Time, grace time.Duration) ([]Frame, error) {
	reduction, err := Reduce(frames)
	if err != nil {
		return nil, fmt.Errorf("compaction refused, registry corrupt: %w", err)
	}

	keepClaim := map[string]bool{}       // opening records + unretired generations
	keepTerminal := map[string]bool{}    // exited/reaped (+ swept)
	keepGenerations := map[string]bool{} // generation-bearing records survive

	for tag, claim := range reduction.Claims {
		switch {
		case claim.Open():
			keepClaim[tag] = true
			keepGenerations[tag] = true
		case claim.Sweepable():
			keepClaim[tag] = true
			keepGenerations[tag] = true
			keepTerminal[tag] = true
		}
	}
	keepCustody := map[string]bool{}
	for id, custody := range reduction.Custodies {
		if custody.Released {
			continue
		}
		if custody.BoundOwnerTag != "" {
			keepCustody[id] = true
			tag := custody.BoundOwnerTag
			keepClaim[tag] = true
			keepTerminal[tag] = true
			// The skeleton carries binding and closure; a claim that
			// is also live or sweepable already keeps its generations.
			continue
		}
		// Unbound: fresh custody survives its grace window so a
		// compaction firing between `custody` and `arming` cannot
		// orphan a provisioner mid-flight (SLC-R6-002).
		if custodyAge := now.Sub(custodyRecordTime(frames, id)); custodyAge < grace {
			keepCustody[id] = true
		}
	}

	var kept []Frame
	for _, frame := range frames {
		if frame.Record == nil {
			continue // tolerated fragments do not survive a rewrite
		}
		record, err := ParseRecord(frame.Record)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", frame.Line, err)
		}
		keep := false
		switch record.Event {
		case TornEvent:
			keep = false
		case EventArming, EventArmed:
			keep = keepClaim[record.OwnerTag]
		case EventRelaunched, EventLaunched:
			keep = keepGenerations[record.OwnerTag] &&
				record.Generation > reduction.Claims[record.OwnerTag].RetiredThrough
		case EventExited, EventReaped, EventSwept:
			keep = keepTerminal[record.OwnerTag]
		case EventCustody:
			keep = keepCustody[record.CustodyID]
		case EventCustodyReleased:
			keep = false // released custody is absorbing and droppable
		}
		if keep {
			kept = append(kept, frame)
		}
	}
	return kept, nil
}

func custodyRecordTime(frames []Frame, custodyID string) time.Time {
	for _, frame := range frames {
		if frame.Record == nil {
			continue
		}
		if frame.Record["event"] == EventCustody && frame.Record["custodyId"] == custodyID {
			if at, ok := frame.Record["at"].(string); ok {
				if parsed, err := time.Parse(time.RFC3339, at); err == nil {
					return parsed
				}
			}
		}
	}
	return time.Time{}
}

// WriteCompacted replaces the registry file with the kept frames via
// an atomic rename. The caller must hold the registry lock (REG-4:
// compaction holds it across read-reduce-replace, so a concurrent
// append can never be discarded by the rewrite).
func WriteCompacted(path string, kept []Frame) error {
	var content strings.Builder
	for _, frame := range kept {
		content.Write(frame.Raw)
		content.WriteByte('\n')
	}
	// The registry's directory must already exist: compaction rewrites a
	// ledger in place and must never invent its location — the owner's
	// MkdirAll would mask a wrong path (contract pinned by an existing
	// test, preserved across the B5 conversion).
	if _, err := os.Stat(filepath.Dir(path)); err != nil {
		return fmt.Errorf("compaction temp file: %w", err)
	}
	// Through the durable-write owner (go-production-grade B5). The registry
	// is durable machine-wide state; its anchor conversion follows with the
	// caller migration.
	_, err := atomicfile.WriteText(path, content.String(), "")
	if err != nil {
		return fmt.Errorf("compaction write: %w", err)
	}
	return nil
}
