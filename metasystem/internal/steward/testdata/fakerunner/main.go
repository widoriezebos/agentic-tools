package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	processidentity "github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/supervise"
)

type runnerRecord struct {
	Pid          int64  `json:"pid"`
	StartTicks   int64  `json:"startTicks"`
	BootID       string `json:"bootId"`
	PidStartedAt int64  `json:"pidStartedAt"`
	StartedAt    string `json:"startedAt"`
}

func main() {
	_ = supervise.BuildStamp
	if len(os.Args) != 5 || os.Args[1] != "steward" || os.Args[2] != "run" || os.Args[3] != "--repo" {
		os.Exit(2)
	}
	root := os.Args[4]
	dir := filepath.Join(root, "artifacts", "agents", "steward")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		os.Exit(1)
	}
	_ = os.Remove(filepath.Join(dir, "stop"))
	self, state, err := (processidentity.KernelProber{}).Probe(int64(os.Getpid()))
	if err != nil || state != processidentity.Alive {
		os.Exit(1)
	}
	record, _ := json.Marshal(runnerRecord{
		Pid: int64(os.Getpid()), StartTicks: self.StartTicks, BootID: self.BootID,
		PidStartedAt: self.StartedAt.Unix(), StartedAt: time.Now().UTC().Format(time.RFC3339),
	})
	if err := os.WriteFile(filepath.Join(dir, "runner.json"), record, 0o644); err != nil {
		os.Exit(1)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "stop")); err == nil {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
}
