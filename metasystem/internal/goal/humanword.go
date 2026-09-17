package goal

import (
	"regexp"
	"strings"
)

var pendingHumanWordPattern = regexp.MustCompile(`(?i)\b(RULING NEEDED|WAITING ON THE HUMAN|WAITING ON [^[:space:]]+ (CALL|RULING|DECISION|ANSWER|WORD)|QUESTION TO THE HUMAN|PARK REQUEST(ED)?)\b`)

// NextStepNamesAPendingHumanWord reports whether the newest entry records a pending human decision.
func NextStepNamesAPendingHumanWord(nextStep string) bool {
	newest := nextStep
	if earlier := strings.Index(nextStep, " EARLIER:"); earlier >= 0 {
		newest = nextStep[:earlier]
	}
	return pendingHumanWordPattern.MatchString(newest)
}
