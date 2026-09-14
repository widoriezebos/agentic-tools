package run

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/unix"
)

// HintReceiver waits for an authenticated notification on a waiter's private
// channel. A true result only schedules another durable source read.
type HintReceiver interface {
	Wait(context.Context, time.Duration) (bool, error)
	Close() error
}

// OpenHintReceiver creates a waiter's private acceleration channel.
type OpenHintReceiver func(path, waitID, nonce string) (HintReceiver, error)

type fifoHintReceiver struct {
	fd      int
	payload []byte
}

// WaitHint names the durable source whose waiters should read again.
type WaitHint struct {
	Kind     string
	TargetID string
}

// HintDelivery describes best-effort delivery. A zero delivery is a normal
// dropped hint and never changes the source result.
type HintDelivery struct {
	Matched   int
	Delivered int
}

func waiterHintPath(rowPath, nonce string) string {
	return rowPath + "." + nonce + ".hint"
}

func removeWaiterHint(rowPath string, row Waiter) {
	if row.Nonce != "" && row.HintPath == waiterHintPath(rowPath, row.Nonce) {
		_ = os.Remove(row.HintPath)
	}
}

// OpenFIFOHintReceiver creates a nonce-named, owner-only FIFO. Opening both
// ends keeps publication nonblocking while the waiter is alive.
func OpenFIFOHintReceiver(path, waitID, nonce string) (HintReceiver, error) {
	if !ValidWaitID(waitID) || !nonceRe.MatchString(nonce) {
		return nil, fmt.Errorf("wait hint identity is invalid")
	}
	if err := unix.Mkfifo(path, 0o600); err != nil {
		return nil, err
	}
	fd, err := unix.Open(path, unix.O_RDWR|unix.O_NONBLOCK|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		_ = os.Remove(path)
		return nil, err
	}
	var stat unix.Stat_t
	if err := unix.Fstat(fd, &stat); err != nil || stat.Mode&unix.S_IFMT != unix.S_IFIFO {
		_ = unix.Close(fd)
		_ = os.Remove(path)
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("wait hint path is not a FIFO")
	}
	if err := unix.Fchmod(fd, 0o600); err != nil {
		_ = unix.Close(fd)
		_ = os.Remove(path)
		return nil, err
	}
	return &fifoHintReceiver{fd: fd, payload: []byte(waitID + " " + nonce + "\n")}, nil
}

func (receiver *fifoHintReceiver) Wait(ctx context.Context, duration time.Duration) (bool, error) {
	deadline := time.Now().Add(duration)
	buffer := make([]byte, 4096)
	for {
		if err := ctx.Err(); err != nil {
			return false, err
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return false, nil
		}
		chunk := remaining
		if chunk > 100*time.Millisecond {
			chunk = 100 * time.Millisecond
		}
		milliseconds := int((chunk + time.Millisecond - 1) / time.Millisecond)
		fds := []unix.PollFd{{Fd: int32(receiver.fd), Events: unix.POLLIN}}
		ready, err := unix.Poll(fds, milliseconds)
		if err == unix.EINTR {
			continue
		}
		if err != nil {
			return false, err
		}
		if ready == 0 || fds[0].Revents&unix.POLLIN == 0 {
			continue
		}
		count, readErr := unix.Read(receiver.fd, buffer)
		if readErr == unix.EAGAIN || readErr == unix.EINTR {
			continue
		}
		if readErr != nil {
			return false, readErr
		}
		for _, line := range bytes.Split(buffer[:count], []byte{'\n'}) {
			if bytes.Equal(line, receiver.payload[:len(receiver.payload)-1]) {
				return true, nil
			}
		}
	}
}

func (receiver *fifoHintReceiver) Close() error {
	if receiver.fd < 0 {
		return nil
	}
	err := unix.Close(receiver.fd)
	receiver.fd = -1
	return err
}

// NotifyWaiters delivers a nonblocking hint to matching live registrations.
// It deliberately does not report missing readers as failure: the ten-second
// source reread remains the correctness path.
func NotifyWaiters(root string, hint WaitHint) (HintDelivery, error) {
	if hint.Kind != "job" && hint.Kind != "attempt" && hint.Kind != "goal" {
		return HintDelivery{}, fmt.Errorf("wait hint kind must be job, attempt, or goal")
	}
	if !waitIdentifierRe.MatchString(hint.TargetID) {
		return HintDelivery{}, fmt.Errorf("wait hint target identifier is invalid")
	}
	paths, err := filepath.Glob(filepath.Join(WaitersDir(root), "*.json"))
	if err != nil {
		return HintDelivery{}, err
	}
	delivery := HintDelivery{}
	for _, rowPath := range paths {
		row, readErr := readV2Waiter(rowPath)
		if readErr != nil || row.Kind != hint.Kind || row.TargetID != hint.TargetID || (row.State != "registering" && row.State != "pending") {
			continue
		}
		delivery.Matched++
		expected := waiterHintPath(rowPath, row.Nonce)
		if row.Accelerator != "fifo" || row.HintPath != expected {
			continue
		}
		fd, openErr := unix.Open(expected, unix.O_WRONLY|unix.O_NONBLOCK|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
		if openErr != nil {
			continue
		}
		var stat unix.Stat_t
		statErr := unix.Fstat(fd, &stat)
		if statErr == nil && stat.Mode&unix.S_IFMT == unix.S_IFIFO {
			payload := []byte(row.WaitID + " " + row.Nonce + "\n")
			if count, writeErr := unix.Write(fd, payload); writeErr == nil && count == len(payload) {
				delivery.Delivered++
			}
		}
		_ = unix.Close(fd)
	}
	return delivery, nil
}
