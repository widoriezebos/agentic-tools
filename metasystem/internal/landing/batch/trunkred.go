package batch

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
)

// Failure identifies one failed test reported by a proof group.
type Failure struct {
	Report    string `json:"report"`
	Classname string `json:"classname"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	Reason    string `json:"reason"`
}

// RedGroup records one proof group that did not pass.
type RedGroup struct {
	ID            string    `json:"id"`
	InputManifest []string  `json:"inputManifest,omitempty"`
	Status        string    `json:"status"`
	NotRunReason  string    `json:"notRunReason"`
	LogPath       string    `json:"logPath"`
	LogDigest     string    `json:"logDigest"`
	Failures      []Failure `json:"failures"`
	// The collection evidence the known-flake predicate judges (D1). Adapter
	// is the testing-contract adapter that produced it; a command group's
	// evidence format follows a colon (command:junit-xml).
	CollectionComplete    bool      `json:"collectionComplete,omitempty"`
	Missing               []Failure `json:"missing,omitempty"`
	Unexpected            []Failure `json:"unexpected,omitempty"`
	NativeExitStatus      *int      `json:"nativeExitStatus,omitempty"`
	Adapter               string    `json:"adapter,omitempty"`
	LongestSilentSeconds  int64     `json:"longestSilentSeconds,omitempty"`
	LongestZeroCPUSeconds int64     `json:"longestZeroCpuSeconds,omitempty"`
	// Stall is the result's typed stall of the group, when the watchdog
	// stalled it.
	Stall *proofrun.GroupStall `json:"stall,omitempty"`
}

// RedGroupFromResult is the red evidence of one group that did not pass.
func RedGroupFromResult(group proofrun.GroupResult) RedGroup {
	failures := func(items []proofrun.NativeTestIdentity, failedOnly bool) []Failure {
		var out []Failure
		for _, item := range items {
			if !failedOnly || item.Status == "failed" {
				out = append(out, Failure{Report: item.Report, Classname: item.Classname, Name: item.Name, Status: item.Status, Reason: item.Reason})
			}
		}
		return out
	}
	return RedGroup{ID: group.ID, Status: group.Status, NotRunReason: group.NotRunReason, LogPath: group.LogPath, LogDigest: group.LogDigest,
		InputManifest: slices.Clone(group.InputManifest), Failures: failures(group.Observed, true), CollectionComplete: group.CollectionComplete,
		Missing: failures(group.Missing, false), Unexpected: failures(group.Unexpected, false), NativeExitStatus: group.NativeExitStatus,
		LongestSilentSeconds: group.LongestSilentSeconds, LongestZeroCPUSeconds: group.LongestZeroCPUSeconds, Stall: group.Stall}
}

// TrunkRed describes proof failures observed on a batch base tree.
type TrunkRed struct {
	BatchID    string     `json:"batchId"`
	AttemptID  string     `json:"attemptId"`
	BaseCommit string     `json:"baseCommit"`
	BaseTree   string     `json:"baseTree"`
	Groups     []RedGroup `json:"groups"`
	Joiners    []Claim    `json:"joiners"`
	SeenAt     time.Time  `json:"seenAt"`
}

// EntryRef identifies one trunk-red ledger entry and its proof group.
type EntryRef struct {
	ID    string `json:"id"`
	Group string `json:"group"`
}

// TrunkRedID returns the stable identity of a red proof group.
func TrunkRedID(group RedGroup) string {
	identityLines := make([]string, 0, len(group.Failures))
	for _, failure := range group.Failures {
		if failure.Status == "failed" {
			line, _ := json.Marshal([]string{failure.Report, failure.Classname, failure.Name})
			identityLines = append(identityLines, string(line)+"\n")
		}
	}
	identityText := group.ID + "\n"
	if len(identityLines) > 0 {
		slices.Sort(identityLines)
		identityLines = slices.Compact(identityLines)
		identityText += strings.Join(identityLines, "")
	} else {
		identityText += group.Status
	}
	digest := sha256.Sum256([]byte(identityText))
	return "tr-" + strings.ReplaceAll(group.ID, "/", "-") + "-" + fmt.Sprintf("%x", digest[:6])
}
