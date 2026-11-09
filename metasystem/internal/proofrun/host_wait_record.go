package proofrun

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// SlotWaitLine records one edge of a wait for a host test slot.
type SlotWaitLine struct {
	Wait      string `json:"wait"`
	Marker    string `json:"marker"`
	Directory string `json:"directory"`
	Pid       int    `json:"pid,omitempty"`
	At        string `json:"at"`
}

type slotWaitRecord struct {
	destination string
	now         func() time.Time
}
type slotWaitRecordKey struct{}

func withSlotWaitRecord(ctx context.Context, destination string, now func() time.Time) context.Context {
	if destination == "" {
		return ctx
	}
	return context.WithValue(ctx, slotWaitRecordKey{}, slotWaitRecord{destination, now})
}

func recordSlotWait(ctx context.Context, kind string, marker *os.File) {
	record, ok := ctx.Value(slotWaitRecordKey{}).(slotWaitRecord)
	if !ok {
		return
	}
	line := SlotWaitLine{Wait: kind, Marker: filepath.Base(marker.Name()), Directory: filepath.Dir(marker.Name()), At: record.now().UTC().Format(time.RFC3339Nano)}
	if kind == "start" {
		line.Pid = os.Getpid()
	}
	data, _ := json.Marshal(line)
	file, err := os.OpenFile(filepath.Join(record.destination, "waits.jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer file.Close()
	_, _ = file.Write(append(data, '\n'))
}

// SlotWaitHeld probes the recorded marker in its acquisition's admission directory.
func SlotWaitHeld(directory, marker string) (bool, error) {
	if filepath.Base(marker) != marker || !strings.HasPrefix(marker, "wait-") {
		return false, os.ErrInvalid
	}
	return heldByAnother(filepath.Join(directory, marker))
}
