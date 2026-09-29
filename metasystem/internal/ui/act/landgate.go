package act

import (
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// The landing gate's two human acts from the browser (g1-s70 D2, D4, §6).

// Sitting records that this hand's review sitting of a goal stands, or that it
// has ended: the hold every seat reads, written before the room reports the
// sitting open and released by every way it ends. The record is the review
// record the sitting is on, in its home.
func (a Authority) Sitting(id, record string, open bool) error {
	if strings.TrimSpace(id) == "" {
		return refuse(KindRequest, "no-goal", "a sitting holds one goal waiting to land")
	}
	path, err := goal.ReviewRecordPath(a.root, record)
	if err != nil {
		return refuse(KindRequest, "record", err.Error())
	}
	action := "goal review --release"
	if open {
		action = "goal review --hold"
	}
	request, done, err := a.request(id, action)
	if err != nil {
		return err
	}
	defer done()
	result, publishErr := goal.Sitting(request, id, path, open, &a.proof)
	return a.settle(request, result, publishErr, action)
}

// LandWithoutSitting records this hand's decision that a goal at or above the
// landing gate's tier lands without a sitting, with the reason, bound to the
// goal branch's tip the caller resolved. The reason is required.
func (a Authority) LandWithoutSitting(id, tip, reason string) error {
	if strings.TrimSpace(id) == "" {
		return refuse(KindRequest, "no-goal", "a decision to land names one goal")
	}
	if _, err := goal.WithoutSittingLine(tip, a.human, reason); err != nil {
		return refuse(KindRequest, "reason", err.Error())
	}
	request, done, err := a.request(id, "goal land-without-sitting")
	if err != nil {
		return err
	}
	defer done()
	result, publishErr := goal.LandWithoutSitting(request, id, tip, reason, &a.proof)
	return a.settle(request, result, publishErr, "goal land-without-sitting")
}
