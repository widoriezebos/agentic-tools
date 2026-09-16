package identity

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"syscall"
)

// FixtureKey identifies one test fixture owned by one exact process.
type FixtureKey struct {
	Owner Ref
	Test  string
	Nonce string
}

// EncodeRef renders the platform-native exact process reference.
func EncodeRef(ref Ref) (string, error) {
	if ref.Pid < 1 || !ref.NativeExact() {
		return "", fmt.Errorf("identity: process reference is not native-exact")
	}
	switch ref.Mode() {
	case CompareDarwinMicroseconds:
		if ref.StartedAtSec != 0 && ref.StartedAtSec != ref.StartedAtUnixMicro/1_000_000 {
			return "", fmt.Errorf("identity: Darwin process reference has inconsistent start times")
		}
		return fmt.Sprintf("pid=%d;micro=%d", ref.Pid, ref.StartedAtUnixMicro), nil
	case CompareLinuxTicksBootID:
		if strings.ContainsAny(ref.BootID, ";|") {
			return "", fmt.Errorf("identity: Linux boot identity contains a reference delimiter")
		}
		return fmt.Sprintf("pid=%d;ticks=%d;boot=%s", ref.Pid, ref.StartTicks, ref.BootID), nil
	default:
		return "", fmt.Errorf("identity: process reference is not exact")
	}
}

// ParseRef parses only the exact reference shape native to this host.
func ParseRef(value string) (Ref, error) {
	fields := strings.Split(value, ";")
	values := make(map[string]string, len(fields))
	for _, field := range fields {
		key, raw, ok := strings.Cut(field, "=")
		if !ok || key == "" || raw == "" {
			return Ref{}, fmt.Errorf("identity: malformed process reference %q", value)
		}
		if _, duplicate := values[key]; duplicate {
			return Ref{}, fmt.Errorf("identity: duplicate process reference field %q", key)
		}
		values[key] = raw
	}
	pid, err := positiveInt(values["pid"])
	if err != nil {
		return Ref{}, fmt.Errorf("identity: invalid process pid: %w", err)
	}
	var ref Ref
	switch {
	case len(values) == 2 && values["micro"] != "":
		micro, parseErr := positiveInt(values["micro"])
		if parseErr != nil {
			return Ref{}, fmt.Errorf("identity: invalid Darwin start identity: %w", parseErr)
		}
		ref = Ref{Pid: pid, StartedAtSec: micro / 1_000_000, StartedAtUnixMicro: micro}
	case len(values) == 3 && values["ticks"] != "" && values["boot"] != "":
		ticks, parseErr := positiveInt(values["ticks"])
		if parseErr != nil {
			return Ref{}, fmt.Errorf("identity: invalid Linux start identity: %w", parseErr)
		}
		ref = Ref{Pid: pid, StartTicks: ticks, BootID: values["boot"]}
	default:
		return Ref{}, fmt.Errorf("identity: process reference has no exact native shape")
	}
	if !ref.NativeExact() {
		return Ref{}, fmt.Errorf("identity: process reference is not native-exact")
	}
	return ref, nil
}

// EncodeKey renders all three fixture-key fields without ambiguity.
func EncodeKey(key FixtureKey) (string, error) {
	owner, err := EncodeRef(key.Owner)
	if err != nil {
		return "", err
	}
	if key.Test == "" || strings.Contains(key.Test, "|") || !eightHex(key.Nonce) {
		return "", fmt.Errorf("identity: invalid fixture key")
	}
	return owner + "|" + key.Test + "|" + key.Nonce, nil
}

// ParseKey parses exactly one owner reference, test name, and nonce.
func ParseKey(value string) (FixtureKey, error) {
	fields := strings.Split(value, "|")
	if len(fields) != 3 || fields[1] == "" || !eightHex(fields[2]) {
		return FixtureKey{}, fmt.Errorf("identity: malformed fixture key")
	}
	owner, err := ParseRef(fields[0])
	if err != nil {
		return FixtureKey{}, err
	}
	return FixtureKey{Owner: owner, Test: fields[1], Nonce: fields[2]}, nil
}

func positiveInt(value string) (int64, error) {
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed < 1 {
		return 0, fmt.Errorf("expected a positive integer")
	}
	return parsed, nil
}

func eightHex(value string) bool {
	if len(value) != 8 {
		return false
	}
	_, err := strconv.ParseUint(value, 16, 32)
	return err == nil
}

var (
	// ErrGone means the recorded process no longer exists at its exact identity.
	ErrGone = errors.New("recorded process is gone")
	// ErrUninspectable means the process identity could not be proved either way.
	ErrUninspectable = errors.New("recorded process identity is uninspectable")
)

// SignalFunc is an injectable process signal operation.
type SignalFunc func(int, syscall.Signal) error

// SignalExact re-proves ref immediately before sending through an optional test or group sender.
func SignalExact(prober Prober, ref Ref, sig syscall.Signal, sender ...SignalFunc) error {
	if len(sender) > 1 {
		return fmt.Errorf("identity: multiple signal senders")
	}
	state := AliveRef(prober, ref)
	if state == Dead {
		return ErrGone
	}
	if state != Alive {
		return ErrUninspectable
	}
	send := SignalFunc(syscall.Kill)
	if len(sender) == 1 && sender[0] != nil {
		send = sender[0]
	}
	if err := send(int(ref.Pid), sig); err != nil && !errors.Is(err, syscall.ESRCH) {
		return err
	}
	return nil
}
