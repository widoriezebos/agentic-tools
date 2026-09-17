package batch

import (
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
)

func batchStartRule(lockFree bool, joined int, oldestJoinedAt time.Time, settings config.BatchLanding, sample proofrun.LoadSample, admission proofrun.AdmissionCap) (start bool, quietWindow string, engineCapAllows bool) {
	engineCapAllows = sample.OverlapKnown && !admission.Refuses(sample, false, true)
	if !lockFree || joined == 0 || !engineCapAllows {
		return
	}
	if settings.MaxWaitElapsed(oldestJoinedAt) {
		return true, "expired", engineCapAllows
	} else if joined >= 2 && !sample.Loaded() && sample.OverlappingHost == 0 {
		return true, "quiet", engineCapAllows
	}
	return
}
