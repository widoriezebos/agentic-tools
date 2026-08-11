package identity

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// The kernel custodian discipline, shared by every process that judges a
// recorded job custodian: the supervision reaper's sweep and the mission
// runner's drain reap must reach the same verdict from the same record, so
// the proof lives here once.

// Custodian proves a job's recorded custodian three-way against the live
// process table: it is the SAME custodian only if the pid is alive at its
// recorded start AND its command still carries the job's tag — a recycled
// pid, or a process no longer bearing the tag, is a stranger and reads as
// dead-to-us. The fixture identity file
// (METASYSTEM_FAKE_PROCESS_IDENTITY_FILE) supplies the start time and command
// when it carries an entry for the pid — the same one-source override the
// census uses — but kernel death always vetoes it. Unknown (unreadable) never
// authorizes anything.
func Custodian(pid, start int64, tag string) Liveness {
	exact, state, err := (KernelProber{}).Probe(pid)
	if err == nil && state == Dead {
		return Dead
	}
	if fixture := os.Getenv("METASYSTEM_FAKE_PROCESS_IDENTITY_FILE"); fixture != "" {
		if data, readErr := os.ReadFile(fixture); readErr == nil {
			var table map[string]struct {
				Started int64  `json:"pidStartedAt"`
				Command string `json:"command"`
			}
			if json.Unmarshal(data, &table) == nil {
				if entry, ok := table[fmt.Sprint(pid)]; ok {
					if entry.Started != start {
						return Dead
					}
					if tag != "" && !strings.Contains(entry.Command, tag) {
						return Dead
					}
					return Alive
				}
			}
		}
	}
	if err != nil || state == Unknown {
		return Unknown
	}
	if exact.StartedAt.Unix() != start {
		return Dead
	}
	if tag != "" && !strings.Contains(strings.Join(exact.Argv, " "), tag) {
		return Dead
	}
	return Alive
}
