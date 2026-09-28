package partner

import "time"

// SettleWithin sets how long a stopped turn is waited for, for a test that must
// see what happens when it does not settle without waiting on a clock to see it.
func SettleWithin(s *Service, wait time.Duration) {
	s.mu.Lock()
	s.settle = wait
	s.mu.Unlock()
}
