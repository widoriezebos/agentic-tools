package hostcapacity

import (
	"github.com/widoriezebos/agentic-tools/metasystem/internal/hostload"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"testing"
	"time"
)

type countingUsage struct{ calls int }

func (*countingUsage) CapacityBuilds() ([]Build, error) { return nil, nil }
func (reader *countingUsage) CapacityUsage(time.Time) (Usage, error) {
	reader.calls++
	return Usage{}, nil
}

func TestAdmissionReadDoesNotReadUsage(t *testing.T) {
	t.Parallel()
	reader := &countingUsage{}
	Read(t.TempDir(), reader, time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC), Sources{
		Load:         func(time.Time) hostload.Sample { return hostload.Sample{Available: true} },
		Registration: func(string) (lane.Record, bool, error) { return lane.Record{}, false, nil },
	})
	if reader.calls != 0 {
		t.Fatalf("admission read invoked usage %d times", reader.calls)
	}
}
