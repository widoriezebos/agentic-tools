package main

// The mission contract-measure verb is the per-cycle reading the mission runner
// records: it runs the contract's gate and guards against the current candidate,
// classifies the gate metrics against the prior cycle's reading, and prints the
// measurement as JSON.
