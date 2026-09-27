package act

// What this server process is doing to its ledger right now.
//
// The act layer owns every act this process executes, and two things follow
// from that ownership.
//
// The first is the second press. Two tabs read one proposal, or a human
// presses Apply again while the first press is still publishing; nothing in
// the ledger stops the second one, because the engine gives every request its
// own operation identifier and a second identical edit is a second record
// (Astra A-01). So while an act on one goal is executing here, a second
// request for that same goal and that same act is refused before it reads
// anything at all — the first press holds the act, and the page's next read
// says what the ledger did with it.
//
// The second is recovery. A push that has landed and not yet been confirmed
// is, to the engine's recovery rule, an entry to classify; to the request that
// made it, it is the act it is still executing. Recovery that classified it
// first took the entry away, and the request that HAD landed its act then
// answered refused and recorded no authority proof beside it (Astra D-01). So
// a publication with its settlement, and a recovery, take one lock and never
// run beside each other.
//
// Both facts are this process's, held per checkout root because that is what
// they are about: one server, one clone. A second server on the same clone is
// excluded by the checkout lock long before anything here.

import "sync"

// InFlight is what a second press on an act already running is told. It is not
// a refusal of the act — the first press is applying it — which is why it says
// where the answer will come from.
const InFlight = "this act on this goal is being applied by another press; the next read says what happened"

// flight names one act: the goal it is about, and the verb it is.
type flight struct{ goal, act string }

// owner is one checkout's ledger as this process holds it.
type owner struct {
	// publications is the one lock. It is held from the moment a request is
	// assembled through its publication and its settlement, and across a
	// recovery, so recovery never classifies an entry a request of this
	// process is still executing.
	publications sync.Mutex
	// running guards the registry and nothing else, so a second press is
	// answered at once rather than queuing behind the first act's whole
	// publication.
	running sync.Mutex
	acts    map[flight]bool
}

// owners is every checkout this process has acted on. A server acts on one,
// so the map has one entry; it is a map rather than a single value because
// the package's own tests each own a checkout of their own and must not share
// one lock.
var owners = struct {
	sync.Mutex
	by map[string]*owner
}{by: map[string]*owner{}}

func ownerOf(root string) *owner {
	owners.Lock()
	defer owners.Unlock()
	held, known := owners.by[root]
	if !known {
		held = &owner{acts: map[flight]bool{}}
		owners.by[root] = held
	}
	return held
}

// begin registers one act as this process's own and answers the refusal a
// second press meets while the first runs. The release it returns clears the
// registration, whatever the act answered.
func (o *owner) begin(id, action string) (func(), error) {
	key := flight{goal: id, act: action}
	o.running.Lock()
	defer o.running.Unlock()
	if o.acts[key] {
		return nil, refuse(KindEngine, "in-flight", InFlight)
	}
	o.acts[key] = true
	return func() {
		o.running.Lock()
		defer o.running.Unlock()
		delete(o.acts, key)
	}, nil
}
