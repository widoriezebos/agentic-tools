// Package tickrequest owns durable requests for an ordinary steward cycle.
// Completion publishers write here without importing the steward's decisions.
package tickrequest

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/durable"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
	"golang.org/x/sys/unix"
)

// Request coalesces all unserved completions. Batch identifies its lifetime;
// Count lets acknowledgement retain only the writes made during a cycle.
type Request struct {
	Batch      string    `json:"batch"`
	Generation uint64    `json:"generation"`
	Count      uint64    `json:"count"`
	Root       string    `json:"root"`
	Reason     string    `json:"reason"`
	At         time.Time `json:"at"`
}

func Path(root string) string {
	return filepath.Join(root, "artifacts", "agents", "steward", "tick-request.json")
}

// Reason keeps arbitrary sender text bounded and safe for a terminal.
func Reason(text string) string {
	var b strings.Builder
	for _, r := range text {
		if unicode.IsSpace(r) {
			r = ' '
		} else if unicode.IsControl(r) || unicode.In(r, unicode.Cf) {
			continue
		}
		if b.Len()+len(string(r)) > 200 {
			break
		}
		b.WriteRune(r)
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

func locked(root string, f func() error) error {
	if !filepath.IsAbs(root) {
		return fmt.Errorf("tick request needs an absolute state root")
	}
	if err := os.MkdirAll(filepath.Dir(Path(root)), 0700); err != nil {
		return err
	}
	held, err := lock.File(Path(root)+".lock", 0600, lock.Exclusive)
	if err != nil {
		return err
	}
	defer held.Release()
	return f()
}

// Write publishes only a hint: callers keep their original operation's result
// if publication fails, and the scheduled tick remains the fallback.
func Write(root, reason string, at time.Time) error {
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	root = resolved
	return locked(root, func() error {
		r, err := Read(root)
		if errors.Is(err, errForeignCheckout) {
			return err
		}
		if err != nil {
			var token [16]byte
			if _, err := rand.Read(token[:]); err != nil {
				return err
			}
			r = Request{Batch: hex.EncodeToString(token[:]), Root: filepath.Clean(root)}
		}
		r.Generation++
		r.Count++
		r.Reason = Reason(reason)
		r.At = at.UTC()
		return write(root, r)
	})
}

var errForeignCheckout = errors.New("tick request does not identify this checkout")

func write(root string, r Request) error {
	data, err := json.Marshal(r)
	if err != nil {
		return err
	}
	durable, err := atomicfile.WriteFile(Path(root), append(data, '\n'), 0600, root)
	if err == nil && !durable {
		return fmt.Errorf("tick request durability is unknown")
	}
	return err
}

// Read never consumes a request and never follows a request-file symlink.
func Read(root string) (Request, error) {
	var r Request
	fd, err := unix.Open(Path(root), unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if err != nil {
		return r, &os.PathError{Op: "open", Path: Path(root), Err: err}
	}
	file := os.NewFile(uintptr(fd), Path(root))
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return r, err
	}
	if !info.Mode().IsRegular() || info.Size() > 4096 {
		return r, fmt.Errorf("tick request is not a small regular file")
	}
	data, err := io.ReadAll(io.LimitReader(file, 4097))
	if err != nil {
		return r, err
	}
	if err = json.Unmarshal(data, &r); err != nil {
		return Request{}, fmt.Errorf("tick request is malformed")
	}
	resolved, rootErr := filepath.EvalSymlinks(root)
	requestRoot, requestErr := filepath.EvalSymlinks(r.Root)
	if rootErr != nil || requestErr != nil || !filepath.IsAbs(r.Root) || requestRoot != resolved {
		return Request{}, errForeignCheckout
	}
	if r.Batch == "" || r.Count == 0 || r.Generation == 0 || r.At.IsZero() {
		return Request{}, fmt.Errorf("tick request does not identify this checkout and a completion")
	}
	r.Reason = Reason(r.Reason)
	return r, nil
}

// Acknowledge runs after the whole cycle. A crash before it leaves the request
// for the next runner; concurrent writers keep their own pending request.
func Acknowledge(root string, served Request) error {
	if served.Batch == "" {
		return nil
	}
	return locked(root, func() error {
		current, err := Read(root)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if current.Batch != served.Batch {
			return nil
		}
		if current.Generation == served.Generation {
			return remove(root)
		}
		if current.Generation > served.Generation && current.Count > served.Count {
			current.Count -= served.Count
			return write(root, current)
		}
		return nil
	})
}

func remove(root string) error {
	if err := os.Remove(Path(root)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return durable.SyncDir(filepath.Dir(Path(root)))
}

// DiscardInvalid rechecks under the writer lock so diagnosis cannot erase a
// valid completion that replaced the bad record.
func DiscardInvalid(root string, now time.Time) error {
	return locked(root, func() error {
		r, err := Read(root)
		if err == nil && !r.At.After(now) {
			return nil
		}
		return remove(root)
	})
}
