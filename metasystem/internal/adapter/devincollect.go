package adapter

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
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atif"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/validate"
)

// The delivery collector (the delegate-delivery design, D64): one owner
// for candidate enumeration, per-candidate selection, attempt
// boundaries, and provenance. The collector reports COLLECTION FACTS
// only — repair eligibility is adjudication's composition, and the
// no-session gate is the caller's, served by the presence-only mode.

// MaxCandidateBytes bounds every candidate snapshot; a role return is
// kilobytes, and anything past this is not a return.
const MaxCandidateBytes = 1 << 20

// CollectParams names the collector's inputs. Attempt is "initial" or
// "repair"; Session is required for a full walk (normalization
// reconciles against it) and unused in presence-only mode.
type CollectParams struct {
	Root, Job      string
	RoundDir       string
	Workspace      string
	StdoutPath     string
	NamedPath      string
	TranscriptPath string
	// ACPOutcomePath selects the ACP transport's EXCLUSIVE channel:
	// the typed outcome of `acp turn`. When set, the walk consults
	// nothing else — an invalid or undelivered ACP candidate fails
	// honestly and never falls through to the legacy scraping
	// channels, because evidence never crosses transports (the
	// design's channel rule).
	ACPOutcomePath string
	RecordPath     string
	Attempt        string
	Session        string
	PresenceOnly   bool
}

// CollectVerdict is the facts document. Channel is one of stdout,
// named-file, transcript, acp, or none; Reply is the accepted snapshot path
// (raw candidate bytes — the downstream pipeline keeps its own
// normalization and validation authority).
type CollectVerdict struct {
	Delivered         bool     `json:"delivered"`
	Channel           string   `json:"channel"`
	Reply             string   `json:"reply,omitempty"`
	CandidatesPresent bool     `json:"candidatesPresent"`
	WatermarkValid    bool     `json:"watermarkValid"`
	Rejected          []string `json:"rejected,omitempty"`
}

type miningAudit struct {
	StepID        string `json:"stepId"`
	ToolCallID    string `json:"toolCallId"`
	TargetPath    string `json:"targetPath"`
	Watermark     int    `json:"watermark"`
	TranscriptSHA string `json:"transcriptSha256"`
}

// DevinCollect walks the delivery channels. Mechanical failures (the
// walk itself impossible) return an error; an over-ceiling transcript
// returns atif.ErrOversize for the caller's transcript-oversize
// terminal; everything else is a verdict.
func DevinCollect(p CollectParams) (*CollectVerdict, error) {
	verdict := &CollectVerdict{Channel: "none"}

	if p.ACPOutcomePath != "" {
		return p.collectACP(verdict)
	}

	stdoutBytes, stdoutErr := readCandidate(p.StdoutPath)
	namedBytes, namedErr := readCandidate(p.NamedPath)

	// candidatesPresent replicates the SHIPPED per-channel presence bar
	// by reference (design r8): stdout counts when non-empty regardless
	// of content; the named file counts when non-empty valid JSON of any
	// kind. The transcript channel is new machinery and deliberately
	// not part of presence — presence must not widen the pinned
	// no-session taxonomy.
	stdoutPresent := stdoutErr == nil && len(stdoutBytes) > 0
	namedPresent := namedErr == nil && len(namedBytes) > 0 && json.Valid(namedBytes)
	verdict.CandidatesPresent = stdoutPresent || namedPresent
	if p.PresenceOnly {
		return verdict, nil
	}
	if p.Session == "" {
		return nil, fmt.Errorf("a full collect needs the correlated session; use presence-only without one")
	}

	// Oversized single candidates fall through with the rejection named;
	// they are never a mechanical verdict (design r4).
	if errors.Is(stdoutErr, errCandidateOversize) {
		verdict.Rejected = append(verdict.Rejected, "stdout: over the candidate ceiling")
		stdoutBytes = nil
	}
	if errors.Is(namedErr, errCandidateOversize) {
		verdict.Rejected = append(verdict.Rejected, "named-file: over the candidate ceiling")
		namedBytes = nil
	}

	if len(stdoutBytes) > 0 && p.acceptCandidate(verdict, "stdout", stdoutBytes, nil) {
		return verdict, p.writeProvenance(verdict, nil)
	}
	if len(namedBytes) > 0 && p.acceptCandidate(verdict, "named-file", namedBytes, nil) {
		return verdict, p.writeProvenance(verdict, nil)
	}

	audit, err := p.mineTranscript(verdict)
	if err != nil {
		return nil, err
	}
	return verdict, p.writeProvenance(verdict, audit)
}

var errCandidateOversize = errors.New("candidate exceeds the snapshot ceiling")

// readCandidate reads one candidate once, bounded. A missing file is an
// empty candidate, not an error; an over-ceiling file is its own error
// so the caller can name the rejection and fall through.
func readCandidate(path string) ([]byte, error) {
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
	data, err := io.ReadAll(io.LimitReader(file, MaxCandidateBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > MaxCandidateBytes {
		return nil, errCandidateOversize
	}
	return data, nil
}

// acceptCandidate runs selection on one candidate: normalize into
// scratch (the pinned fenced/wrapper/identity behaviors), then the one
// canonical validator. Acceptance writes the RAW candidate bytes as the
// accepted snapshot — the downstream pipeline keeps its own
// normalization and validation authority; selection never replaces it.
// acpOutcome is the subset of the acp turn verb's stdout wire shape
// the collector consumes; the candidate distinguishes nil (no
// candidate row) from present-but-empty by the pointer.
type acpOutcome struct {
	Row          string  `json:"row"`
	SessionID    string  `json:"sessionId"`
	StopReason   string  `json:"stopReason"`
	Candidate    *string `json:"candidate"`
	JournalError string  `json:"journalError"`
}

// collectACP is the ACP transport's exclusive walk: one channel,
// no fallthrough. The candidate rides the SAME qualification path
// as every legacy channel (snapshot, normalize, full-job validate,
// accepted snapshot) — the D62 owners downstream of selection are
// reused unchanged.
func (p CollectParams) collectACP(verdict *CollectVerdict) (*CollectVerdict, error) {
	body, err := os.ReadFile(p.ACPOutcomePath)
	if err != nil {
		return nil, fmt.Errorf("acp outcome unreadable: %w", err)
	}
	var outcome acpOutcome
	if err := json.Unmarshal(body, &outcome); err != nil {
		return nil, fmt.Errorf("acp outcome not JSON: %w", err)
	}
	present := outcome.Row == "delivered" && outcome.Candidate != nil
	verdict.CandidatesPresent = present
	if p.PresenceOnly {
		return verdict, nil
	}
	if p.Session == "" {
		return nil, fmt.Errorf("a full collect needs the correlated session; use presence-only without one")
	}
	if outcome.JournalError != "" {
		// The journal is the settlement evidence; a delivery whose
		// raw record is admittedly incomplete is not a delivery
		// (the spec's journal-owned completeness boundary).
		verdict.Rejected = append(verdict.Rejected, "acp: journal thinned: "+outcome.JournalError)
		return verdict, p.writeProvenance(verdict, nil)
	}
	if outcome.SessionID != p.Session {
		verdict.Rejected = append(verdict.Rejected, "acp: outcome session "+outcome.SessionID+" is not this turn's session")
		return verdict, p.writeProvenance(verdict, nil)
	}
	if !present {
		verdict.Rejected = append(verdict.Rejected, "acp: row="+outcome.Row+" stopReason="+outcome.StopReason+" delivered nothing")
		return verdict, p.writeProvenance(verdict, nil)
	}
	raw := []byte(*outcome.Candidate)
	if len(raw) > MaxCandidateBytes {
		verdict.Rejected = append(verdict.Rejected, "acp: over the candidate ceiling")
		return verdict, p.writeProvenance(verdict, nil)
	}
	if p.acceptCandidate(verdict, "acp", raw, nil) {
		return verdict, p.writeProvenance(verdict, nil)
	}
	return verdict, p.writeProvenance(verdict, nil)
}

func (p CollectParams) acceptCandidate(verdict *CollectVerdict, channel string, raw []byte, audit *miningAudit) bool {
	scratch := filepath.Join(p.RoundDir, "collect-scratch")
	if err := os.MkdirAll(scratch, 0o755); err != nil {
		verdict.Rejected = append(verdict.Rejected, channel+": scratch unavailable: "+err.Error())
		return false
	}
	candidate := filepath.Join(scratch, channel+"-candidate.json")
	if err := os.WriteFile(candidate, raw, 0o644); err != nil {
		verdict.Rejected = append(verdict.Rejected, channel+": snapshot write failed: "+err.Error())
		return false
	}
	normalized := filepath.Join(scratch, channel+"-normalized.json")
	markdown := filepath.Join(scratch, channel+"-normalized.md")
	if err := NormalizeReturn(candidate, "", p.RecordPath, normalized, markdown, p.Session); err != nil {
		verdict.Rejected = append(verdict.Rejected, channel+": normalization refused: "+err.Error())
		return false
	}
	// The full JOB flow, not schema-only: a schema-valid return for the
	// wrong job must be rejected here, at selection.
	if problems := validate.ReturnCompleteJobFile(p.Root, p.Job, normalized); len(problems) > 0 {
		verdict.Rejected = append(verdict.Rejected, channel+": "+strings.Join(problems, "; "))
		return false
	}
	accepted := filepath.Join(p.RoundDir, "reply-accepted.json")
	if _, err := atomicfile.WriteText(accepted, string(raw), ""); err != nil {
		verdict.Rejected = append(verdict.Rejected, channel+": accepted-snapshot write failed: "+err.Error())
		return false
	}
	verdict.Delivered = true
	verdict.Channel = channel
	verdict.Reply = accepted
	return true
}

// mineTranscript is rung 3: the last in-window write-tool call whose
// target BASENAME equals the attempt's named file (the designation
// rule) and whose target exists on disk with content matching the
// transcript argument (the filesystem success oracle).
func (p CollectParams) mineTranscript(verdict *CollectVerdict) (*miningAudit, error) {
	if p.TranscriptPath == "" {
		verdict.Rejected = append(verdict.Rejected, "transcript: no export")
		return nil, nil
	}
	snapshotPath := filepath.Join(p.RoundDir, "transcript."+p.Attempt+".snapshot")
	transcript, err := atif.Snapshot(p.TranscriptPath, snapshotPath)
	if err != nil {
		if errors.Is(err, atif.ErrOversize) {
			return nil, err
		}
		verdict.Rejected = append(verdict.Rejected, "transcript: "+err.Error())
		return nil, nil
	}

	watermarkPath := filepath.Join(p.RoundDir, "collect-watermark")
	window := 0
	switch p.Attempt {
	case "initial":
		// The watermark is written only on a complete bounded read (the
		// fail-closed rule); atif.Snapshot succeeded, so this read was one.
		if _, err := atomicfile.WriteText(watermarkPath, strconv.Itoa(len(transcript.Steps)), ""); err != nil {
			return nil, err
		}
		verdict.WatermarkValid = true
	case "repair":
		data, err := os.ReadFile(watermarkPath)
		if err != nil {
			// No valid watermark: the transcript rung is DISABLED for the
			// repair — never a guessed boundary.
			verdict.Rejected = append(verdict.Rejected, "transcript: no valid watermark; rung disabled for the repair")
			return nil, nil
		}
		window, err = strconv.Atoi(strings.TrimSpace(string(data)))
		if err != nil {
			verdict.Rejected = append(verdict.Rejected, "transcript: unreadable watermark; rung disabled for the repair")
			return nil, nil
		}
		verdict.WatermarkValid = true
	default:
		return nil, fmt.Errorf("attempt must be initial or repair, got %q", p.Attempt)
	}

	wantBase := filepath.Base(p.NamedPath)
	var chosen []byte
	var audit *miningAudit
	for index, step := range transcript.Steps {
		if index < window {
			continue
		}
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
			onDisk, err := readCandidate(target)
			if err != nil || len(onDisk) == 0 {
				verdict.Rejected = append(verdict.Rejected,
					fmt.Sprintf("transcript step %s: designated write did not persist at %s", step.StepID.String(), args.FilePath))
				continue
			}
			if sha256hex(onDisk) != sha256hex([]byte(args.Content)) {
				verdict.Rejected = append(verdict.Rejected,
					fmt.Sprintf("transcript step %s: on-disk content diverged from the recorded write", step.StepID.String()))
				continue
			}
			chosen = []byte(args.Content)
			audit = &miningAudit{
				StepID:        step.StepID.String(),
				ToolCallID:    call.ToolCallID,
				TargetPath:    args.FilePath,
				Watermark:     window,
				TranscriptSHA: sha256hex(transcript.Raw()),
			}
		}
	}
	if chosen == nil {
		verdict.Rejected = append(verdict.Rejected, "transcript: no designated, persisted write in the attempt window")
		return nil, nil
	}
	if !p.acceptCandidate(verdict, "transcript", chosen, audit) {
		return audit, nil
	}
	return audit, nil
}

// writeProvenance binds the decision to bytes: attempt, channel, the
// accepted snapshot's digest, every rejection, and the full mining
// audit when the channel was the transcript.
func (p CollectParams) writeProvenance(verdict *CollectVerdict, audit *miningAudit) error {
	record := map[string]any{
		"attempt":           p.Attempt,
		"channel":           verdict.Channel,
		"delivered":         verdict.Delivered,
		"candidatesPresent": verdict.CandidatesPresent,
		"rejected":          verdict.Rejected,
	}
	if verdict.Delivered {
		accepted, err := os.ReadFile(verdict.Reply)
		if err != nil {
			return err
		}
		record["sha256"] = sha256hex(accepted)
	}
	if verdict.Channel == "transcript" && audit != nil {
		record["mining"] = audit
	}
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	_, err = atomicfile.WriteText(filepath.Join(p.RoundDir, "reply-source.json"), string(data)+"\n", "")
	return err
}

func sha256hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
