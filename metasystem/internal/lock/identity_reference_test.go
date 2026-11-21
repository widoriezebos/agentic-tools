package lock

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

func TestOwnerLockReferencesRoundTripAndOtherBindingsKeepTheirBytes(t *testing.T) {
	t.Parallel()
	self := Identity{Pid: 17, PidStartedAt: 99, Tag: "tag", Label: "label"}
	codec := identityJSON{}
	data, err := codec.Encode(self)
	const want = "{\"pid\":17,\"pidStartedAt\":99,\"instanceTag\":\"tag\",\"label\":\"label\"}\n"
	if err != nil || string(data) != want {
		t.Fatalf("legacy encoding %q: %v", data, err)
	}
	if got, err := codec.Decode(data); got != self || err != nil {
		t.Fatalf("legacy identity %+v: %v", got, err)
	}
	recorded := self
	recorded.Process = identity.Ref{Pid: 17, StartTicks: 55, BootID: "boot"}
	dir := filepath.Join(t.TempDir(), "lock")
	if _, err := Acquire(dir, recorded, Options{Probe: func(Identity) Liveness { return Alive }}); err != nil {
		t.Fatal(err)
	}
	wrong := self
	wrong.Label = "other"
	if err := ReleaseNamed(dir, wrong, codec); !errors.As(err, new(*NotHolderError)) {
		t.Fatalf("foreign release: %v", err)
	}
	if err := TransferNamed(dir, wrong, self, codec); err == nil {
		t.Fatal("foreign transfer succeeded")
	}
	if err := TransferNamed(dir, self, self, codec); err != nil {
		t.Fatal(err)
	}
	if got, err := Holder(dir); err != nil || got != self {
		t.Fatalf("after transfer: %+v %v", got, err)
	}
}
