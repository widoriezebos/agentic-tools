package fake_test

import "time"

// wiringBound is the wait on any real fact these wiring proofs assert (a
// long poll returning after its update, a paused request released): an
// order of magnitude above what the fact needs on a loaded box, so the
// proofs wait for the fact and never for an instant
// (plans/time-bound-tests-design.md).
const wiringBound = 30 * time.Second
