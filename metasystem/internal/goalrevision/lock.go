// Package goalrevision owns the ranked admission-and-stop lock for one goal
// revision. A held lock is an in-process capability returned by acquisition;
// readable owner metadata is diagnostic evidence and never a substitute.
package goalrevision

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	baselock "github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
)

var coordinatePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// Path returns the one lock path shared by dispatch, stop, resume, and
// sensitive recovery replay.
func Path(root, goalID string, revision uint64) (string, error) {
	if !coordinatePattern.MatchString(goalID) || revision == 0 {
		return "", fmt.Errorf("invalid goal-revision lock coordinates %q revision %d", goalID, revision)
	}
	return filepath.Join(root, "artifacts", "agents", "locks", "goal-revisions",
		goalID+"-r"+strconv.FormatUint(revision, 10)+".d"), nil
}

type ownerCodec struct{}

func (ownerCodec) Encode(self baselock.Identity) ([]byte, error) {
	nanoseconds, startTicks, bootID := parseExactLabel(self.Label)
	payload, err := json.Marshal(map[string]any{
		"pid": self.Pid, "pidStartedAt": self.PidStartedAt,
		"pidStartedAtNano": nanoseconds, "startTicks": startTicks, "bootId": bootID,
		"instanceTag": self.Tag, "acquiredAt": time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		return nil, err
	}
	return append(payload, '\n'), nil
}

func (ownerCodec) Decode(data []byte) (baselock.Identity, error) {
	var value struct {
		PID              *int64 `json:"pid"`
		PIDStartedAt     int64  `json:"pidStartedAt"`
		PIDStartedAtNano int64  `json:"pidStartedAtNano"`
		StartTicks       int64  `json:"startTicks"`
		BootID           string `json:"bootId"`
		InstanceTag      string `json:"instanceTag"`
	}
	if err := json.Unmarshal(data, &value); err != nil {
		return baselock.Identity{}, err
	}
	if value.PID == nil {
		return baselock.Identity{}, fmt.Errorf("owner file names no pid")
	}
	return baselock.Identity{Pid: *value.PID, PidStartedAt: value.PIDStartedAt, Tag: value.InstanceTag,
		Label: exactLabel(value.PIDStartedAtNano, value.StartTicks, value.BootID)}, nil
}

func exactLabel(nanoseconds, startTicks int64, bootID string) string {
	return strconv.FormatInt(nanoseconds, 10) + "|" + strconv.FormatInt(startTicks, 10) + "|" + bootID
}

func parseExactLabel(label string) (int64, int64, string) {
	parts := strings.SplitN(label, "|", 3)
	if len(parts) != 3 {
		return 0, 0, ""
	}
	nanoseconds, nanoErr := strconv.ParseInt(parts[0], 10, 64)
	startTicks, ticksErr := strconv.ParseInt(parts[1], 10, 64)
	if nanoErr != nil || ticksErr != nil {
		return 0, 0, ""
	}
	return nanoseconds, startTicks, parts[2]
}

func holderLiveness(holder baselock.Identity) baselock.Liveness {
	exact, state, err := (identity.KernelProber{}).Probe(holder.Pid)
	if state == identity.Dead {
		return baselock.Dead
	}
	if err != nil || state != identity.Alive {
		return baselock.Unknown
	}
	nanoseconds, startTicks, bootID := parseExactLabel(holder.Label)
	if startTicks > 0 && bootID != "" {
		if exact.StartTicks == startTicks && exact.BootID == bootID {
			return baselock.Alive
		}
		return baselock.Dead
	}
	if nanoseconds > 0 {
		if exact.StartedAt.UnixNano() == nanoseconds {
			return baselock.Alive
		}
		return baselock.Dead
	}
	if holder.PidStartedAt > 0 {
		if exact.StartedAt.Unix() == holder.PidStartedAt {
			return baselock.Alive
		}
		return baselock.Dead
	}
	if !exact.ArgvKnown {
		return baselock.Unknown
	}
	if holder.Tag != "" && strings.Contains(strings.Join(exact.Argv, " "), holder.Tag) {
		return baselock.Alive
	}
	return baselock.Dead
}

// Held is the non-transferable result of a successful acquisition.
type Held struct{ lock *baselock.Lock }

// Acquire obtains the exact revision lock within a bounded interval. The
// caller can release only the handle returned by this acquisition.
func Acquire(root, goalID string, revision uint64, tag string) (*Held, error) {
	path, err := Path(root, goalID, revision)
	if err != nil {
		return nil, err
	}
	exact, state, err := (identity.KernelProber{}).Probe(int64(os.Getpid()))
	if err != nil || state != identity.Alive {
		return nil, fmt.Errorf("goal-revision lock cannot read its acquiring process identity")
	}
	acquired, err := baselock.Acquire(path, baselock.Identity{
		Pid: int64(os.Getpid()), PidStartedAt: exact.StartedAt.Unix(), Tag: tag,
		Label: exactLabel(exact.StartedAt.UnixNano(), exact.StartTicks, exact.BootID),
	}, baselock.Options{Wait: time.Second, Poll: 25 * time.Millisecond, Probe: holderLiveness, Codec: ownerCodec{}})
	if err != nil {
		var holder *baselock.HolderError
		if errors.As(err, &holder) {
			return nil, BusyError(path, fmt.Sprintf("%s/r%d", goalID, revision))
		}
		return nil, err
	}
	return &Held{lock: acquired}, nil
}

// Release ends this acquisition. A second release is harmless.
func (h *Held) Release() error {
	if h == nil || h.lock == nil {
		return nil
	}
	err := h.lock.Release()
	h.lock = nil
	return err
}

// BusyError names the rank, key, readable holder evidence, and retry action.
func BusyError(path, key string) error {
	holder := "unreadable"
	if data, err := os.ReadFile(filepath.Join(path, "owner.json")); err == nil {
		if decoded, decodeErr := (ownerCodec{}).Decode(data); decodeErr == nil {
			holder = fmt.Sprintf("pid=%d,tag=%s", decoded.Pid, decoded.Tag)
		}
	}
	return fmt.Errorf("LOCK_BUSY rank=goal-revision key=%s holder=%s retry=retry-after-the-named-holder-releases", key, holder)
}
