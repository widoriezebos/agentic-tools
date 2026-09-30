package proofrun

import "fmt"

// The refusals in this file are machine protocol as well as text: a test
// run's admission refusal leaves the run's process on its output, and the
// landing owner reads the leading code there (internal/landing/batchowner
// batchAdmissionRefusalCode). Their code stays first and byte-identical, so
// this file is the one place in the package where a code leads a line a
// person may read.

// retryPriorOutsideTree refuses a retry whose earlier run tested another
// candidate tree.
func retryPriorOutsideTree(tree string) error {
	return fmt.Errorf("RETRY_PRIOR_OUTSIDE_TREE: the run this retry names tested another tree than %s", tree)
}
