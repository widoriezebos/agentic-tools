package host

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atif"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/validate"
)

// The host-turn delivery walk (D64 phase 2). Turn-shaped, not
// job-shaped: the pre-envelope check is everything decidable before
// the result envelope exists — the orchestrator schema plus turnId,
// missionId, and cycle equality against the turn record. Session
// identity deliberately stays with the runner's post-envelope
// adjudication; the walk is RESUMABLE past its rejections via reject
// digests, so a wrong-session stdout can delay but never destroy a
// valid named-file result.

// MaxHostCandidateBytes mirrors the delegate collector's ceiling.
const MaxHostCandidateBytes = 1 << 20

// HostCollectParams names the walk's inputs. RejectDigests lists
// sha256 hexes of candidates the runner has already rejected
// post-envelope; matching candidates are skipped with the rejection
// recorded.
type HostCollectParams struct {
	Root           string
	TurnRecordPath string
	TurnDir        string
	Workspace      string
	StdoutPath     string
	NamedPath      string
	TranscriptPath string
	RejectDigests  []string
}

// HostCollectVerdict is the facts document, mirroring the delegate
// collector's shape.
type HostCollectVerdict struct {
	Delivered bool     `json:"delivered"`
	Channel   string   `json:"channel"`
	Reply     string   `json:"reply,omitempty"`
	Rejected  []string `json:"rejected,omitempty"`
}

// HostDevinCollect walks stdout, the named file, then the transcript's
// designated writes, accepting the first candidate that passes the
// pre-envelope check and is not a rejected digest.
func HostDevinCollect(p HostCollectParams) (*HostCollectVerdict, error) {
	verdict := &HostCollectVerdict{Channel: "none"}
	turnRecord, err := readObjectBounded(p.TurnRecordPath)
	if err != nil {
		return nil, fmt.Errorf("cannot read the turn record: %w", err)
	}
	rejected := map[string]bool{}
	for _, digest := range p.RejectDigests {
		rejected[digest] = true
	}

	try := func(channel string, raw []byte) bool {
		if len(raw) == 0 {
			return false
		}
		digest := sha256Hex(raw)
		if rejected[digest] {
			verdict.Rejected = append(verdict.Rejected, channel+": rejected by the runner (digest "+digest[:12]+")")
			return false
		}
		if reason := hostCandidateCheck(p.Root, p.TurnDir, channel, raw, turnRecord); reason != "" {
			verdict.Rejected = append(verdict.Rejected, channel+": "+reason)
			return false
		}
		accepted := filepath.Join(p.TurnDir, "reply-accepted.json")
		if err := os.WriteFile(accepted, raw, 0o644); err != nil {
			verdict.Rejected = append(verdict.Rejected, channel+": accepted-snapshot write failed: "+err.Error())
			return false
		}
		verdict.Delivered = true
		verdict.Channel = channel
		verdict.Reply = accepted
		return true
	}

	stdoutBytes, stdoutErr := readBoundedFile(p.StdoutPath, MaxHostCandidateBytes)
	if stdoutErr != nil {
		verdict.Rejected = append(verdict.Rejected, "stdout: "+stdoutErr.Error())
	}
	if try("stdout", stdoutBytes) {
		return verdict, writeHostProvenance(p.TurnDir, verdict)
	}
	namedBytes, namedErr := readBoundedFile(p.NamedPath, MaxHostCandidateBytes)
	if namedErr != nil {
		verdict.Rejected = append(verdict.Rejected, "named-file: "+namedErr.Error())
	}
	if try("named-file", namedBytes) {
		return verdict, writeHostProvenance(p.TurnDir, verdict)
	}

	if p.TranscriptPath != "" {
		snapshot := filepath.Join(p.TurnDir, "transcript.host.snapshot")
		transcript, err := atif.Snapshot(p.TranscriptPath, snapshot)
		if err != nil {
			if errors.Is(err, atif.ErrOversize) {
				return nil, err
			}
			verdict.Rejected = append(verdict.Rejected, "transcript: "+err.Error())
		} else {
			wantBase := filepath.Base(p.NamedPath)
			for _, step := range transcript.Steps {
				for _, call := range step.ToolCalls {
					if call.FunctionName != "write" {
						continue
					}
					var args struct {
						FilePath string `json:"file_path"`
						Content  string `json:"content"`
					}
					if json.Unmarshal(call.Arguments, &args) != nil || args.Content == "" {
						continue
					}
					if filepath.Base(args.FilePath) != wantBase {
						continue
					}
					target := args.FilePath
					if !filepath.IsAbs(target) {
						target = filepath.Join(p.Workspace, target)
					}
					onDisk, err := readBoundedFile(target, MaxHostCandidateBytes)
					if err != nil || len(onDisk) == 0 || sha256Hex(onDisk) != sha256Hex([]byte(args.Content)) {
						verdict.Rejected = append(verdict.Rejected,
							"transcript step "+step.StepID.String()+": designated write did not persist intact")
						continue
					}
					if try("transcript", []byte(args.Content)) {
						return verdict, writeHostProvenance(p.TurnDir, verdict)
					}
				}
			}
			if verdict.Channel == "none" {
				verdict.Rejected = append(verdict.Rejected, "transcript: no designated, persisted write qualified")
			}
		}
	} else {
		verdict.Rejected = append(verdict.Rejected, "transcript: no export")
	}
	return verdict, writeHostProvenance(p.TurnDir, verdict)
}

// hostCandidateCheck is the pre-envelope bar: the orchestrator schema
// plus turnId, missionId, and cycle equality — everything decidable
// before the envelope exists. Session identity is the runner's,
// post-envelope, on purpose.
func hostCandidateCheck(root, turnDir, channel string, raw []byte, turnRecord map[string]any) string {
	var body map[string]any
	if json.Unmarshal(raw, &body) != nil {
		return "not a JSON object"
	}
	scratch := filepath.Join(turnDir, "collect-scratch")
	if err := os.MkdirAll(scratch, 0o755); err != nil {
		return "scratch unavailable: " + err.Error()
	}
	candidate := filepath.Join(scratch, channel+"-candidate.json")
	if err := os.WriteFile(candidate, raw, 0o644); err != nil {
		return "snapshot write failed: " + err.Error()
	}
	if problems := validate.ReturnCompleteRole(root, "orchestrator", candidate); len(problems) > 0 {
		return fmt.Sprintf("schema: %v", problems)
	}
	for _, field := range []string{"turnId", "missionId"} {
		want, _ := turnRecord[field].(string)
		got, _ := body[field].(string)
		if want != "" && got != want {
			return field + " mismatch: return has " + strconv.Quote(got)
		}
	}
	if want, ok := numberOf(turnRecord["cycle"]); ok {
		if got, gotOK := numberOf(body["cycle"]); !gotOK || got != want {
			return "cycle mismatch"
		}
	}
	return ""
}

func writeHostProvenance(turnDir string, verdict *HostCollectVerdict) error {
	record := map[string]any{
		"channel":   verdict.Channel,
		"delivered": verdict.Delivered,
		"rejected":  verdict.Rejected,
	}
	if verdict.Delivered {
		accepted, err := os.ReadFile(verdict.Reply)
		if err != nil {
			return err
		}
		record["sha256"] = sha256Hex(accepted)
	}
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(turnDir, "reply-source.json"), append(data, '\n'), 0o644)
}

func readBoundedFile(path string, ceiling int64) ([]byte, error) {
	if path == "" {
		return nil, nil
	}
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, ceiling+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > ceiling {
		return nil, fmt.Errorf("candidate exceeds the %d-byte ceiling", ceiling)
	}
	return data, nil
}

func readObjectBounded(path string) (map[string]any, error) {
	data, err := readBoundedFile(path, MaxHostCandidateBytes)
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, fmt.Errorf("absent: %s", path)
	}
	var value map[string]any
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, err
	}
	return value, nil
}

func numberOf(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case json.Number:
		f, err := typed.Float64()
		return f, err == nil
	case int:
		return float64(typed), true
	}
	return 0, false
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
