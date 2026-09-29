package seat

import (
	"strings"
	"testing"
	"time"
)

// machine list shows each machine's free space from its presence
// (engine-owns-disk-lifetimes U6a); a record without the field, as older
// engines publish, reads as before.
func TestPresenceCarriesFreeSpaceToMachineList(t *testing.T) {
	t.Parallel()
	mine := presence("m1e", fixtureClock.Add(-2*time.Minute), 600)
	free := int64(198) << 30
	mine.DiskFreeBytes = &free
	encoded, err := mine.Encode()
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseRecord(encoded)
	if err != nil || parsed.DiskFreeBytes == nil || *parsed.DiskFreeBytes != free {
		t.Fatalf("round trip = %+v, %v", parsed, err)
	}
	old := presence("m1c", fixtureClock.Add(-2*time.Minute), 600)
	legacy, _ := old.Encode()
	if strings.Contains(string(legacy), "diskFreeBytes") {
		t.Fatal("a record without a measurement carries the field")
	}
	if parsed, err := ParseRecord(legacy); err != nil || parsed.DiskFreeBytes != nil {
		t.Fatalf("an older record = %+v, %v", parsed, err)
	}
	report := Report{}
	report.SetNow(fixtureClock)
	parsedMine, _ := ParseRecord(encoded)
	report.Machines = Fleet(FleetInput{This: "m1e", Copy: Copy{Records: map[string]Record{"m1e": parsedMine, "m1c": old}}, Now: fixtureClock, Window: 30 * time.Minute})
	text := report.Text()
	if !strings.Contains(text, "free 198.0 GiB") {
		t.Fatalf("machine list does not show the free space:\n%s", text)
	}
	if strings.Count(text, "free ") != 1 {
		t.Fatalf("a machine without a measurement shows free space:\n%s", text)
	}
}
