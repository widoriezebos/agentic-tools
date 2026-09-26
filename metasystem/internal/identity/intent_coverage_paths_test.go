package identity

import (
	"runtime"
	"strings"
	"testing"
)

// The refusal edges of the durable reference and fixture-key codecs. A record
// is only as trustworthy as the reference it carries, so every shape that could
// name a different process than the one recorded must be refused on both the
// write and the read side, never approximated.

// nativeRefForRefusals is a well-formed exact reference native to this host.
func nativeRefForRefusals(t *testing.T) Ref {
	t.Helper()
	switch runtime.GOOS {
	case "darwin":
		return Ref{Pid: 41, StartedAtSec: 100, StartedAtUnixMicro: 100_000_123}
	case "linux":
		return Ref{Pid: 41, StartTicks: 7001, BootID: "boot-a"}
	default:
		t.Skip("native exact process references are supported on Darwin and Linux")
		return Ref{}
	}
}

// A reference that is not native-exact, or whose parts disagree, is never
// encoded into a record: a legacy seconds-only ref, a zero pid, a Darwin ref
// whose seconds contradict its microseconds, and a Linux boot identity that
// would split the encoded fields.
func TestARefThatCouldNameAnotherProcessIsNotEncoded(t *testing.T) {
	t.Parallel()

	native := nativeRefForRefusals(t)
	refusals := map[string]Ref{
		"a legacy seconds-only ref": {Pid: 41, StartedAtSec: 100},
		"a ref with no pid":         func() Ref { ref := native; ref.Pid = 0; return ref }(),
	}
	switch runtime.GOOS {
	case "darwin":
		refusals["a ref whose seconds contradict its microseconds"] = Ref{Pid: 41, StartedAtSec: 99, StartedAtUnixMicro: 100_000_123}
		refusals["a Linux ref on Darwin"] = Ref{Pid: 41, StartTicks: 7001, BootID: "boot-a"}
	case "linux":
		refusals["a boot identity carrying a field delimiter"] = Ref{Pid: 41, StartTicks: 7001, BootID: "boot;a"}
		refusals["a boot identity carrying a key delimiter"] = Ref{Pid: 41, StartTicks: 7001, BootID: "boot|a"}
		refusals["a Darwin ref on Linux"] = Ref{Pid: 41, StartedAtUnixMicro: 100_000_123}
	}
	for name, ref := range refusals {
		if encoded, err := EncodeRef(ref); err == nil {
			t.Errorf("%s was encoded as %q; want a refusal", name, encoded)
		}
		if encoded, err := EncodeKey(FixtureKey{Owner: ref, Test: "TestX", Nonce: "a1b2c3d4"}); err == nil {
			t.Errorf("a fixture key owned by %s was encoded as %q; want a refusal", name, encoded)
		}
	}
}

// A recorded reference string that is malformed, ambiguous, or of another
// platform's shape is refused rather than parsed into a partial identity.
func TestARecordedRefThatIsNotExactlyNativeIsRefused(t *testing.T) {
	t.Parallel()

	nativeRefForRefusals(t)
	refusals := map[string]string{
		"an empty reference":           "",
		"a field without a value":      "pid=41;micro=",
		"a field without a key":        "=41;micro=100000123",
		"a duplicated field":           "pid=41;pid=42",
		"a pid that is not a number":   "pid=forty-one;micro=100000123",
		"a pid that is not positive":   "pid=0;micro=100000123",
		"a Darwin start that is zero":  "pid=41;micro=0",
		"a Linux start that is zero":   "pid=41;ticks=0;boot=boot-a",
		"a pid with no start identity": "pid=41",
		"an unknown extra field":       "pid=41;micro=100000123;extra=1",
	}
	switch runtime.GOOS {
	case "darwin":
		refusals["a Linux reference on Darwin"] = "pid=41;ticks=7001;boot=boot-a"
	case "linux":
		refusals["a Darwin reference on Linux"] = "pid=41;micro=100000123"
	}
	for name, value := range refusals {
		if ref, err := ParseRef(value); err == nil {
			t.Errorf("%s (%q) parsed as %+v; want a refusal", name, value, ref)
		}
	}
}

// A fixture key is exactly owner, test and an eight-hex nonce. A test name
// that would carry the separator, an empty test, or a nonce of any other shape
// is refused on encode, and a key string of any other shape is refused on
// parse, including one whose owner reference is itself refused.
func TestAFixtureKeyOfAnyOtherShapeIsRefused(t *testing.T) {
	t.Parallel()

	owner := nativeRefForRefusals(t)
	encodedOwner, err := EncodeRef(owner)
	if err != nil {
		t.Fatalf("encoding the native owner: %v", err)
	}
	for name, key := range map[string]FixtureKey{
		"an empty test name":             {Owner: owner, Test: "", Nonce: "a1b2c3d4"},
		"a test name carrying the bar":   {Owner: owner, Test: "TestX|child", Nonce: "a1b2c3d4"},
		"a short nonce":                  {Owner: owner, Test: "TestX", Nonce: "a1b2c3"},
		"a nonce that is not hex":        {Owner: owner, Test: "TestX", Nonce: "a1b2c3dz"},
		"a nonce of the right length +1": {Owner: owner, Test: "TestX", Nonce: "a1b2c3d4e"},
	} {
		if encoded, err := EncodeKey(key); err == nil {
			t.Errorf("%s was encoded as %q; want a refusal", name, encoded)
		}
	}
	for name, value := range map[string]string{
		"two fields":              encodedOwner + "|TestX",
		"four fields":             encodedOwner + "|TestX|a1b2c3d4|extra",
		"an empty test":           encodedOwner + "||a1b2c3d4",
		"a nonce that is not hex": encodedOwner + "|TestX|zzzzzzzz",
		"an owner that is legacy": "pid=41;sec=100|TestX|a1b2c3d4",
	} {
		if key, err := ParseKey(value); err == nil {
			t.Errorf("a key with %s (%q) parsed as %+v; want a refusal", name, value, key)
		}
	}
	valid, err := EncodeKey(FixtureKey{Owner: owner, Test: "TestX", Nonce: "A1B2C3D4"})
	if err != nil || !strings.HasSuffix(valid, "|TestX|A1B2C3D4") {
		t.Fatalf("an upper-case hex nonce encoded as %q, %v; want it admitted unchanged", valid, err)
	}
}
