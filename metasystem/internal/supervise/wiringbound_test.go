package supervise

import "time"

// wiringBound is the wait on any real-process fact these wiring proofs
// assert (a readable argv, a reap, a group gone): an order of magnitude
// above what the fact needs on a loaded box, so the proofs wait for the
// fact and never for an instant (plans/time-bound-tests-design.md).
const wiringBound = 30 * time.Second
