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
	"time"
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
	// CodeEvidenceRootUnsafe is a sibling evidence root that is this seat's
	// own, or that resolves through a symlink to somewhere else.
	CodeEvidenceRootUnsafe = "SEAT_LAUNCH_EVIDENCE_ROOT_UNSAFE"
	// CodeWordRequired is a resume that reaches enrollment without the word
	// and the date, which the record never held.
	CodeWordRequired = "SEAT_LAUNCH_WORD_REQUIRED"
)

// Refusal is one of the codes above with the sentence a human acts on.
type Refusal struct {
	Code    string
	Message string
}

func (r *Refusal) Error() string { return r.Code + ": " + r.Message }

// refuse is the one constructor, so every refusal reads the same way.
func refuse(code, format string, args ...any) *Refusal {
	return &Refusal{Code: code, Message: fmt.Sprintf(format, args...)}
}

// reviewByLayout is the date form steward arm's own validator takes.
const reviewByLayout = "2006-01-02"

// PastReviewDate reports whether a review-by date is already behind us.
//
// It is this side's rule and not the engine's: ValidateTemporaryWordPair
// accepts a past date, and the identity reader never compares the date to a
// clock. A date in the past is therefore a review that is due the moment the
// machine joins, which is not what a human choosing one means, so the sheet
// and the route refuse it here. The verb does not: a caller at a terminal who
// means it is the engine's business and not this package's.
func PastReviewDate(reviewBy string, now time.Time) bool {
	date, err := time.Parse(reviewByLayout, reviewBy)
	if err != nil {
		return false
	}
	return date.Before(now.UTC().Truncate(24 * time.Hour))
}
