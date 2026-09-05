package registry

import (
	"fmt"
	"log"
	"sort"
)

// REG-3: reduction is a fold over all records. Per ownerTag a claim
// OPENS at arming, gains identity at armed, and CLOSES at exited or
// reaped — terminal events are absorbing, and `swept` is the one
// permitted post-terminal update. Per custodyId a custody opens at
// `custody`, binds through the custodyId on a claim's arming/armed,
// and closes (absorbing) at `custody-released`.

// GenerationSet is one relaunch generation's recorded set: the tags
// minted write-ahead and the identities captured per component
// (SLC-R3-013, SLC-R4-011).
type GenerationSet struct {
	WatcherTag string
	ReaperTag  string
	// Identities maps component name to its captured identity; a
	// component missing here is inside its unrecorded launch window.
	Identities map[string]ProcessRef
}

// ProcessRef is a recorded process identity. Kill proof additionally
// demands claim-consistent argv (REG-6); a ProcessRef alone is never
// authority to signal.
type ProcessRef struct {
	Pid          int64
	PidStartedAt int64
}

// Claim is the reduced state of one ownerTag.
type Claim struct {
	OwnerTag     string
	CheckoutPath string
	CustodyID    string // binding, if the arming/armed carried one (D-3)

	Reserved bool // arming seen
	Armed    bool // armed seen while the reservation was open
	Owner    ProcessRef
	// Generations pairs relaunched/launched records BY GENERATION
	// (SLC-R7-007) — never append order, which a delayed retry can
	// scramble.
	Generations map[int64]*GenerationSet
	// RetiredThrough is the highest contiguous watermark any
	// relaunched record proved (SLC-R8-002, SLC-R9-003).
	RetiredThrough int64

	Closed           bool
	ClosedBy         string // EventExited or EventReaped
	Reason           string
	TeardownComplete bool
	SweepPending     bool
	Swept            bool
}

// Sweepable reports whether the claim still holds a possibly-surviving
// set (SLC-R4-012, SLC-R5-015): closed by exited with
// teardownComplete false or by reaped with sweepPending true, until a
// swept record lands.
func (c *Claim) Sweepable() bool {
	if !c.Closed || c.Swept {
		return false
	}
	switch c.ClosedBy {
	case EventExited:
		return !c.TeardownComplete
	case EventReaped:
		return c.SweepPending
	}
	return false
}

// Open reports whether the claim consumes attention as non-closed:
// a reservation or an armed claim without a terminal.
func (c *Claim) Open() bool { return c.Reserved && !c.Closed }

// CurrentGeneration returns the highest recorded generation's set,
// or nil when none was ever relaunched.
func (c *Claim) CurrentGeneration() *GenerationSet {
	var highest int64 = -1
	for number := range c.Generations {
		if number > highest {
			highest = number
		}
	}
	if highest < 0 {
		return nil
	}
	return c.Generations[highest]
}

// Custody is the reduced state of one custodyId (D-3: same-lifetime
// provisioners only).
type Custody struct {
	CustodyID    string
	CheckoutPath string
	Custodian    ProcessRef
	// BoundOwnerTag names the claim whose arming/armed carried this
	// custodyId; empty while unbound.
	BoundOwnerTag string
	Released      bool
}

// Reduction is the fold's output: the machine-wide custody view.
type Reduction struct {
	// Claims by ownerTag; Order preserves first-appearance order for
	// deterministic reporting and compaction output.
	Claims map[string]*Claim
	Order  []string

	Custodies    map[string]*Custody
	CustodyOrder []string

	// SeenTags holds every ownerTag that appears AT ALL, open or
	// closed — REG-2's uniqueness rule refuses a reservation whose
	// tag was ever seen (SLC-R5-013, SLC-R7-008).
	SeenTags map[string]bool

	// ownerCheckouts records the first checkout path carried by each owner
	// identity, including a sequence-illegal row that cannot open a claim.
	// Later rows for the same owner may not move this selection.
	ownerCheckouts map[string]string

	// Fragments counts tolerated non-JSON lines, reported not acted on.
	Fragments int
}

// TagSeen answers the arming gate's uniqueness check.
func (r *Reduction) TagSeen(tag string) bool { return r.SeenTags[tag] }

// OwnerCheckoutPath returns the checkout selected by the first valid record
// for an owner identity. A later row for another checkout is sequence-illegal
// and cannot change this selection.
func (r *Reduction) OwnerCheckoutPath(ownerTag string) (string, bool) {
	path, found := r.ownerCheckouts[ownerTag]
	return path, found
}

// SortedTags returns claim tags in first-appearance order.
func (r *Reduction) SortedTags() []string {
	return append([]string(nil), r.Order...)
}

// Reduce folds parsed frames into the machine-wide view. Records that
// are individually valid but ILLEGAL in sequence (an armed with no
// open reservation, a second terminal, a swept on a non-sweepable
// claim) are DROPPED with the reduction continuing: guarded appends
// (REG-3) prevent writers from creating those shapes, so at read time
// they are the residue of pre-guard history, and refusing the whole
// registry for them would fail closed on shapes the contract already
// makes harmless. Structurally invalid records are a different
// matter: ParseRecord errors mark the registry corrupt (REG-5), which
// the caller signals by failing closed.
func Reduce(frames []Frame) (*Reduction, error) {
	reduction := &Reduction{
		Claims:         map[string]*Claim{},
		Custodies:      map[string]*Custody{},
		SeenTags:       map[string]bool{},
		ownerCheckouts: map[string]string{},
	}
	custodyPaths := map[string]string{}
	for _, frame := range frames {
		if frame.Record == nil {
			reduction.Fragments++
			continue
		}
		record, err := ParseRecord(frame.Record)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", frame.Line, err)
		}
		if record.Event == TornEvent {
			continue
		}
		switch record.Event {
		case EventArming, EventArmed, EventRelaunched, EventLaunched, EventExited, EventReaped, EventSwept:
			if path, seen := reduction.ownerCheckouts[record.OwnerTag]; seen && path != record.CheckoutPath {
				logCheckoutConflict(frame.Line, "owner tag", record.OwnerTag, path, record.Event, record.CheckoutPath)
				continue
			}
			if record.CustodyID != "" {
				if path, seen := custodyPaths[record.CustodyID]; seen && path != record.CheckoutPath {
					logCheckoutConflict(frame.Line, "custody", record.CustodyID, path, record.Event, record.CheckoutPath)
					continue
				}
			}
			reduction.ownerCheckouts[record.OwnerTag] = record.CheckoutPath
			if record.CustodyID != "" {
				custodyPaths[record.CustodyID] = record.CheckoutPath
			}
			reduction.foldClaim(record)
		case EventCustody, EventCustodyReleased:
			if path, seen := custodyPaths[record.CustodyID]; seen && path != record.CheckoutPath {
				logCheckoutConflict(frame.Line, "custody", record.CustodyID, path, record.Event, record.CheckoutPath)
				continue
			}
			custodyPaths[record.CustodyID] = record.CheckoutPath
			reduction.foldCustody(record)
		}
	}
	reduction.bindCustodies()
	return reduction, nil
}

func logCheckoutConflict(line int, identityKind, identity, recordedPath, event, conflictingPath string) {
	log.Printf("supervision registry: dropped sequence-illegal line %d: %s %q was recorded for checkout %q before a %s record named checkout %q", line, identityKind, identity, recordedPath, event, conflictingPath)
}

func (r *Reduction) foldClaim(record *Record) {
	r.SeenTags[record.OwnerTag] = true
	claim := r.Claims[record.OwnerTag]
	if claim == nil {
		if record.Event != EventArming {
			// A claim event with no reservation on record: pre-guard
			// residue or a compaction survivor of a shape the current
			// rules no longer write. For terminals we still track the
			// claim so sweep bookkeeping is not lost; everything else
			// is dropped.
			if record.Event != EventExited && record.Event != EventReaped && record.Event != EventSwept {
				return
			}
		}
		claim = &Claim{
			OwnerTag:     record.OwnerTag,
			CheckoutPath: record.CheckoutPath,
			Generations:  map[int64]*GenerationSet{},
		}
		r.Claims[record.OwnerTag] = claim
		r.Order = append(r.Order, record.OwnerTag)
	}
	switch record.Event {
	case EventArming:
		if claim.Reserved || claim.Closed {
			return // reopening is refused at the door (SLC-R5-004)
		}
		claim.Reserved = true
		claim.CustodyID = record.CustodyID
	case EventArmed:
		// GUARDED (SLC-R5-003): legal only while the reservation
		// reduces open. A late armed after reap/compaction is refused
		// at append time; one already in the file is dropped here.
		if !claim.Reserved || claim.Closed || claim.Armed {
			return
		}
		claim.Armed = true
		claim.Owner = ProcessRef{Pid: record.OwnerPid, PidStartedAt: record.OwnerPidStartedAt}
		if record.CustodyID != "" {
			claim.CustodyID = record.CustodyID
		}
	case EventRelaunched:
		if claim.Closed {
			return
		}
		set := claim.generation(record.Generation)
		set.WatcherTag = record.WatcherTag
		set.ReaperTag = record.ReaperTag
		// The watermark is contiguous (SLC-R9-003): it only ever
		// advances, and compaction drops exactly the generations a
		// watermark covers.
		if record.RetiredThrough > claim.RetiredThrough {
			claim.RetiredThrough = record.RetiredThrough
		}
	case EventLaunched:
		if claim.Closed {
			return
		}
		// Paired BY GENERATION (SLC-R7-007): a stale gen-1 retry
		// landing after gen-2's records updates gen 1's identities,
		// never gen 2's.
		set := claim.generation(record.Generation)
		set.Identities[record.Component] = ProcessRef{Pid: record.Pid, PidStartedAt: record.PidStartedAt}
	case EventExited:
		if claim.Closed {
			return // terminals are absorbing
		}
		claim.Closed = true
		claim.ClosedBy = EventExited
		claim.Reason = record.Reason
		claim.TeardownComplete = record.TeardownComplete
	case EventReaped:
		if claim.Closed {
			return
		}
		claim.Closed = true
		claim.ClosedBy = EventReaped
		claim.Reason = record.Reason
		claim.SweepPending = record.SweepPending
	case EventSwept:
		// The one post-terminal update (SLC-R5-015): clears sweepable,
		// reopens nothing.
		if claim.Closed {
			claim.Swept = true
		}
	}
}

func (c *Claim) generation(number int64) *GenerationSet {
	set := c.Generations[number]
	if set == nil {
		set = &GenerationSet{Identities: map[string]ProcessRef{}}
		c.Generations[number] = set
	}
	return set
}

func (r *Reduction) foldCustody(record *Record) {
	custody := r.Custodies[record.CustodyID]
	switch record.Event {
	case EventCustody:
		if custody != nil {
			return
		}
		r.Custodies[record.CustodyID] = &Custody{
			CustodyID:    record.CustodyID,
			CheckoutPath: record.CheckoutPath,
			Custodian:    ProcessRef{Pid: record.CustodianPid, PidStartedAt: record.CustodianPidStartedAt},
		}
		r.CustodyOrder = append(r.CustodyOrder, record.CustodyID)
	case EventCustodyReleased:
		if custody == nil {
			return
		}
		custody.Released = true // absorbing; names its custodyId so a
		// late release of one custody can never hide another (SLC-R4-007)
	}
}

// bindCustodies derives each custody's binding from the claims'
// arming/armed custodyId (D-3: custody binds at arming; there is no
// separate binding event, SLC-R5-005).
func (r *Reduction) bindCustodies() {
	tags := append([]string(nil), r.Order...)
	sort.Strings(tags) // deterministic when two claims name one custody
	for _, tag := range tags {
		claim := r.Claims[tag]
		if claim.CustodyID == "" {
			continue
		}
		custody := r.Custodies[claim.CustodyID]
		if custody != nil && custody.BoundOwnerTag == "" {
			custody.BoundOwnerTag = claim.OwnerTag
		}
	}
}
