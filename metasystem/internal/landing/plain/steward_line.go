package plain

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
)

type StewardLine struct {
	At       time.Time     `json:"at"`
	Line     string        `json:"line"`
	Rows     []Fired       `json:"rows"`
	Report   stewardReport `json:"report"`
	Identity string        `json:"identity"`
}

func ReadStewardLine(install string) (line StewardLine, err error) {
	data, err := os.ReadFile(filepath.Join(Dir(install), "steward-line.json"))
	if err == nil {
		err = json.Unmarshal(data, &line)
		if err == nil && (line.At.IsZero() || line.Line == "" || line.Identity == "") {
			err = fmt.Errorf("the lane line has no time, text or identity")
		}
	}
	return line, err
}

func writeStewardLine(install string, h Health, now time.Time) error {
	return withLock(install, func() error {
		report, err := readStewardReport(install)
		if err != nil {
			return err
		}
		h.Report = report.Summary
		line := StewardLine{At: now.UTC(), Line: h.Line(), Rows: h.Rows(), Report: report}
		pairs := make([][2]string, 0, len(line.Rows))
		for i, row := range line.Rows {
			pairs = append(pairs, [2]string{row.Row, row.Subject})
			line.Rows[i].Act = ""
		}
		slices.SortFunc(pairs, func(a, b [2]string) int { return slices.Compare(a[:], b[:]) })
		identity, err := json.Marshal([]any{pairs, report.At})
		if err != nil {
			return err
		}
		line.Identity = fmt.Sprintf("%x", sha256.Sum256(identity))
		data, err := json.Marshal(line)
		if err != nil {
			return err
		}
		return atomicfile.WriteVolatileFile(filepath.Join(Dir(install), "steward-line.json"), data, 0644)
	})
}
