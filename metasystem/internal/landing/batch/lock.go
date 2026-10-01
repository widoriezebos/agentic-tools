package batch

// DefaultProofLockDir is the root of the per-batch proof locks: each batch's
// mutex is its own directory <root>/batch-<id>, so a landing batch never
// queues behind a proving one. How many prove at once is the host's proving
// flock's to say (U12: one batch proves at a time on the host, the
// OwnerOptions.Proving seam), with the admission cap inside it.
const DefaultProofLockDir = "/tmp/metasystem-batch-locks"
