//go:build !unix

package steward

import "time"

// processCPUTime has no portable reading here; a zero cost keeps the bound
// vacuous rather than wrong.
func processCPUTime() time.Duration { return 0 }
