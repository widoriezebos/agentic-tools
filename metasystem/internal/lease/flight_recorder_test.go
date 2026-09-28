package lease

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The flight recorder is a witness, never an authority (records/misc/
// flight-recorder.md D-5). These are the two legs of
// flight-recorder-fixtures.sh that needed a real lease claim, ported to the
// lease owner the fixture reached through `lease announce`
// (verbs-object-action U7c).

// TestAClaimSucceedsAndEmitsNothingWhenTheEventStreamIsUnwritable: with the
// stream unwritable a claim still happens (epoch 1) and nothing is written.
func TestAClaimSucceedsAndEmitsNothingWhenTheEventStreamIsUnwritable(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("root writes through mode 000")
	}
	root := t.TempDir()
	stream := filepath.Join(root, "artifacts", "agents", "events.jsonl")
	if err := os.MkdirAll(filepath.Dir(stream), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stream, nil, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(stream, 0o644) })

	if _, err := Announce(root, "fr-fixture", int64(os.Getpid()), selfStart(t), "metasystem-main-fr", "fake", ""); err != nil {
		t.Fatalf("a lease claim failed because the witness stream was unwritable: %v", err)
	}
	lease, err := loadLease(root, true)
	if err != nil || lease.ClaimEpoch != 1 {
		t.Fatalf("the lease claim did not happen: %+v %v", lease, err)
	}
	if err := os.Chmod(stream, 0o644); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(stream); err != nil || len(data) != 0 {
		t.Fatalf("an unwritable stream received events: %q %v", data, err)
	}
}

// TestAFreshClaimWitnessesLeaseClaimed: with the stream writable, a
// successful claim leaves its lease-claimed event.
func TestAFreshClaimWitnessesLeaseClaimed(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if _, err := Announce(root, "fr-fixture2", int64(os.Getpid()), selfStart(t), "metasystem-main-fr2", "fake", ""); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "artifacts", "agents", "events.jsonl"))
	if err != nil || !strings.Contains(string(data), `"event":"lease-claimed"`) {
		t.Fatalf("a successful claim left no lease-claimed event: %q %v", data, err)
	}
}
