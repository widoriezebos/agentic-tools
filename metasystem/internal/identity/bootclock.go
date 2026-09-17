package identity

import (
	"fmt"
	"strings"
	"time"
)

// DarwinBootReadings holds the kernel readings a Darwin boot identity is
// built from.
type DarwinBootReadings struct {
	SessionUUID  string
	SessionErr   error
	BootTimeSec  int64
	BootTimeUsec int64
	BootTimeErr  error
	Now          time.Time
	Elapsed      time.Duration
}

// DarwinBootIdentity chooses a session UUID, then a kernel boot time, then a
// minute estimate for the current Darwin boot. It returns an error when the
// elapsed time is negative or the estimated boot minute is not after the Unix
// epoch.
func DarwinBootIdentity(r DarwinBootReadings) (string, error) {
	if r.Elapsed < 0 {
		return "", fmt.Errorf("identity: monotonic boot clock is implausible")
	}
	if session := strings.Trim(r.SessionUUID, " \x00"); r.SessionErr == nil && session != "" {
		return session, nil
	}
	if r.BootTimeErr == nil && r.BootTimeSec > 0 && r.BootTimeUsec >= 0 && r.BootTimeUsec <= 999999 {
		return fmt.Sprintf("%d.%06d", r.BootTimeSec, r.BootTimeUsec), nil
	}
	estimatedBoot := r.Now.UTC().Add(-r.Elapsed).Truncate(time.Minute)
	if estimatedBoot.Unix() <= 0 {
		return "", fmt.Errorf("identity: boot time estimate is implausible")
	}
	return "estimated-" + estimatedBoot.Format("20060102T1504Z"), nil
}
