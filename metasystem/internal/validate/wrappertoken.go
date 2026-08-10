package validate

import (
	"encoding/json"
	"math"
	"os"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// ProcessTree reads the two process facts the wrapper-token proof
// needs: a pid's parent and its kernel start time in whole seconds.
// The kernel-backed implementation is KernelProcessTree; tests inject
// a fixed tree.
type ProcessTree interface {
	// ParentPid returns a pid's parent, or ok=false when the pid is
	// gone, unreadable, or has no distinct ancestor.
	ParentPid(pid int64) (int64, bool)
	// StartedAtSec returns a live pid's start time in epoch seconds,
	// or ok=false when the pid cannot be read as alive.
	StartedAtSec(pid int64) (int64, bool)
}

// KernelProcessTree answers from the kernel: parent pids via the
// proc_info query and start times via the exact-identity prober.
type KernelProcessTree struct{}

func (KernelProcessTree) ParentPid(pid int64) (int64, bool) {
	return identity.ParentPid(pid)
}

func (KernelProcessTree) StartedAtSec(pid int64) (int64, bool) {
	exact, state, err := identity.KernelProber{}.Probe(pid)
	if err != nil || state != identity.Alive {
		return 0, false
	}
	return exact.StartedAt.Unix(), true
}

// WrapperToken proves that the caller runs under the live commit
// wrapper the token names: the token must carry an integer wrapperPid,
// an integer wrapperPidStartedAt, and a 32-character nonce; the wrapper
// pid must appear in the caller's process ancestry; and the process at
// that pid must have started at exactly the recorded second, so a
// recycled pid never passes. Anything unreadable or mismatched fails
// the proof — absence of evidence never authorizes a commit.
func WrapperToken(tokenPath string, callerPid int64, tree ProcessTree) bool {
	data, err := os.ReadFile(tokenPath)
	if err != nil {
		return false
	}
	var token map[string]any
	if err := json.Unmarshal(data, &token); err != nil {
		return false
	}
	wrapperPid, pidOK := integerField(token["wrapperPid"])
	startedAt, startOK := integerField(token["wrapperPidStartedAt"])
	nonce, nonceOK := token["nonce"].(string)
	if !pidOK || !startOK || !nonceOK || len(nonce) != 32 {
		return false
	}

	seen := map[int64]bool{}
	current := callerPid
	for current > 0 && !seen[current] {
		seen[current] = true
		if current == wrapperPid {
			actual, ok := tree.StartedAtSec(wrapperPid)
			return ok && actual == startedAt
		}
		parent, ok := tree.ParentPid(current)
		if !ok {
			break
		}
		current = parent
	}
	return false
}

// integerField accepts only a JSON number with an integral value; a
// boolean, string, or fractional number is not a pid or a start time.
func integerField(value any) (int64, bool) {
	number, ok := value.(float64)
	if !ok || number != math.Trunc(number) ||
		number < math.MinInt64 || number >= math.MaxInt64 {
		return 0, false
	}
	return int64(number), true
}
