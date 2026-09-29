package launch

// What this verb refuses, by name.
//
// Every code below is this package's own. The steps themselves refuse in
// their owner's words — git's, the build fence's, the validator's, the arm's
// — and those are recorded verbatim rather than translated into a code here:
// a launch that failed because the gate fence refused the new clone's build
// must say so in the fence's sentence, because that is what a human acts on.

import (
	"fmt"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
)

// The preflight's refusals, judged before any step and before the lock, and
// the two the steps themselves raise.
const (
	// CodeNicknameInvalid is a nickname the presence publisher would refuse,
	// or this checkout's own.
	CodeNicknameInvalid = "SEAT_LAUNCH_NICKNAME_INVALID"
	// CodeNicknameTaken is a nickname a sibling clone, a presence ref or a
	// claim at the accepted tip already names.
	CodeNicknameTaken = "SEAT_LAUNCH_NICKNAME_TAKEN"
	// CodeDestinationExists is a destination that is already there and was
	// not created by the launch a resume names.
	CodeDestinationExists = "SEAT_LAUNCH_DESTINATION_EXISTS"
	// CodeDestinationInsideACheckout is a destination beneath another git
	// checkout, which would make the machine a directory of that one.
	CodeDestinationInsideACheckout = "SEAT_LAUNCH_DESTINATION_INSIDE_A_CHECKOUT"
	// CodeRunning is the host lock: one launch at a time on this host.
	CodeRunning = "SEAT_LAUNCH_RUNNING"
	// CodeDiskShort is free disk at the destination's parent against twice
	// the size of this checkout.
	CodeDiskShort = "SEAT_LAUNCH_DISK_SHORT"
	// CodeEvidenceRootUnsafe is a seat with no evidence root set, or a
	// sibling evidence root that is this seat's own, or that resolves through
	// a symlink to somewhere else.
	CodeEvidenceRootUnsafe = "SEAT_LAUNCH_EVIDENCE_ROOT_UNSAFE"
	// CodeWordRequired is a resume that reaches enrollment with neither a
	// signed-in session's verdict on its record nor the temporary pair: the
	// record never held a word, and a record created without an enrollment
	// is never stamped with one later (g1-s72 S72-02).
	CodeWordRequired = "SEAT_LAUNCH_WORD_REQUIRED"
	// CodeWordInvalid is the temporary pair beside a record a signed-in
	// session already enrolled (g1-s72 D4): the record says whose machine
	// this is, and a second authority beside it is refused rather than
	// chosen between.
	CodeWordInvalid = "SEAT_LAUNCH_WORD_INVALID"
	// CodeRecordConflict is a machine or a destination asked for beside a
	// record a signed-in session enrolled, which names its own: one verdict
	// arms one clone, so a stale record path never enrolls another machine
	// under the session that launched the first (g1-s72 S72-01).
	CodeRecordConflict = "SEAT_LAUNCH_RECORD_CONFLICT"
	// CodeIDInvalid is a launch id that is not the one path segment a record
	// is named by.
	CodeIDInvalid = "SEAT_LAUNCH_ID_INVALID"
	// CodeFleetUnreadable is a presence copy this seat could not bring in or
	// could not read. A launch judges a nickname against the fleet, and a
	// fleet it cannot read is not a fleet it may guess about.
	CodeFleetUnreadable = "SEAT_LAUNCH_FLEET_UNREADABLE"
	// CodeSupervisionDown is `up --recover-only --if-down` returning with no
	// live process holding the new machine's supervision owner lock.
	CodeSupervisionDown = "SEAT_LAUNCH_SUPERVISION_DOWN"
	// CodeIdentityUnreadable is a clone whose enrolled identity cannot be
	// read after arming, so no presence record can be recognised as its own.
	CodeIdentityUnreadable = "SEAT_LAUNCH_IDENTITY_UNREADABLE"
	// CodeUnknown is a discard naming a launch this host has no record of.
	CodeUnknown = "SEAT_LAUNCH_UNKNOWN"
	// CodeDiscardRunning is a discard of a launch still starting or running,
	// whose verb would write the mark away at its next step.
	CodeDiscardRunning = "SEAT_LAUNCH_DISCARD_RUNNING"
)

// Refusal is one of the codes above with the sentence a human acts on.
type Refusal struct {
	Code    string
	Message string
}

func (r *Refusal) Error() string {
	if r.Code == CodeEvidenceRootUnsafe {
		// Rule H1: the one damage refusal of the launch guides. The new
		// machine's evidence would land in this seat's root or outside
		// the evidence tree; the person fixes the root and resumes.
		return r.Code + ": " + r.Message + "; the new machine's evidence would mix with another root's, so set " + config.EvidenceRootKey + " in this seat's metasystem.conf.local to a directory of its own under the fleet's evidence tree, then retry the launch (Retry on the fleet page, or metasystem machine start NAME --resume ID)"
	}
	return r.Code + ": " + r.Message
}

// refuse is the one constructor, so every refusal reads the same way.
func refuse(code, format string, args ...any) *Refusal {
	return &Refusal{Code: code, Message: fmt.Sprintf(format, args...)}
}
