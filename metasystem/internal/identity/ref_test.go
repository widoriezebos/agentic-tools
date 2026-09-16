package identity

import (
	"runtime"
	"testing"
)

func TestOwnerRefEncodesNativeShape(t *testing.T) {
	var ref Ref
	var encoded string
	switch runtime.GOOS {
	case "darwin":
		ref = Ref{Pid: 41, StartedAtSec: 100, StartedAtUnixMicro: 100_000_123}
		encoded = "pid=41;micro=100000123"
	case "linux":
		ref = Ref{Pid: 41, StartedAtSec: 100, StartTicks: 7001, BootID: "boot-a"}
		encoded = "pid=41;ticks=7001;boot=boot-a"
	default:
		t.Skip("native exact process references are supported on Darwin and Linux")
	}
	got, err := EncodeRef(ref)
	if err != nil || got != encoded {
		t.Fatalf("EncodeRef() = %q, %v; want %q", got, err, encoded)
	}
	parsed, err := ParseRef(got)
	if err != nil || parsed.Pid != ref.Pid || parsed.Mode() != ref.Mode() ||
		parsed.StartedAtUnixMicro != ref.StartedAtUnixMicro || parsed.StartTicks != ref.StartTicks || parsed.BootID != ref.BootID {
		t.Fatalf("ParseRef() = %+v, %v; want exact fields from %+v", parsed, err, ref)
	}
	key := FixtureKey{Owner: ref, Test: "TestOwnerRefEncodesNativeShape/child", Nonce: "a1b2c3d4"}
	encodedKey, err := EncodeKey(key)
	if err != nil {
		t.Fatal(err)
	}
	parsedKey, err := ParseKey(encodedKey)
	if err != nil || parsedKey.Test != key.Test || parsedKey.Nonce != key.Nonce || parsedKey.Owner.Mode() != key.Owner.Mode() ||
		parsedKey.Owner.Pid != key.Owner.Pid || parsedKey.Owner.StartedAtUnixMicro != key.Owner.StartedAtUnixMicro ||
		parsedKey.Owner.StartTicks != key.Owner.StartTicks || parsedKey.Owner.BootID != key.Owner.BootID {
		t.Fatalf("ParseKey() = %+v, %v; want exact fields from %+v", parsedKey, err, key)
	}
	if _, err := EncodeRef(Ref{Pid: 41, StartedAtSec: 100}); err == nil {
		t.Fatal("whole-second ref encoded as an exact owner reference")
	}
	if _, err := ParseKey(encoded + "|TestOwnerRefEncodesNativeShape"); err == nil {
		t.Fatal("two-field fixture key parsed successfully")
	}
}
