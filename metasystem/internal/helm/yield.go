package helm

import (
	"encoding/json"
	"errors"
	"os"
	"time"
)

// Entry is one helm.log line: a take or a return.
type Entry struct {
	At       time.Time `json:"at"`
	Action   string    `json:"action"`
	By       string    `json:"by"`
	Reason   string    `json:"reason,omitempty"`
	Leader   string    `json:"leader,omitempty"`
	Replaced string    `json:"replaced,omitempty"`
}

// Log appends one line to helm.log.
func Log(root string, entry Entry) error {
	seat, err := Locate(root)
	if err != nil {
		return err
	}
	entry.At = entry.At.UTC()
	return appendLine(seat, seat.Log, entry)
}

// Yield is one boundary's yield under the helm: which gate, what it would have
// decided ("not evaluated" when it answered before evaluating), and about what.
type Yield struct {
	At       time.Time `json:"at"`
	Boundary string    `json:"boundary"`
	Gate     string    `json:"gate,omitempty"`
	Would    string    `json:"would,omitempty"`
	By       string    `json:"by,omitempty"`
	PID      int       `json:"pid,omitempty"`
	Subject  string    `json:"subject,omitempty"`
}

// RecordYield (the design's helm.Yield; the type holds that name) appends one
// line to root's helm-yields.log. It is best-effort and returns nothing: a
// failed append never turns a yield into a refusal. Empty By, At and PID are
// filled from the signature, the clock and this process.
func RecordYield(root string, y Yield) {
	defer func() { _ = recover() }()
	seat, err := Locate(root)
	if err != nil {
		return
	}
	if y.By == "" {
		y.By = Active(root).By
	}
	if y.At.IsZero() {
		y.At = time.Now()
	}
	if y.PID == 0 {
		y.PID = os.Getpid()
	}
	y.At = y.At.UTC()
	_ = appendLine(seat, seat.Yields, y)
}

func appendLine(seat Seat, path string, value any) error {
	encoded, err := json.Marshal(value)
	if err == nil {
		err = os.MkdirAll(seat.Dir, 0o700)
	}
	if err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	_, err = file.Write(append(encoded, '\n'))
	return errors.Join(err, file.Close())
}
