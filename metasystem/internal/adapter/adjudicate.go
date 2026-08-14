package adapter

import (
	"fmt"
	"os"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/validate"
)

// The adapter turn's terminal-outcome state machine (review
// script-adapters-01, relocated from runtime-common.sh's complete_from_cli
// and devin.sh's empty-reply extension). PURE DECISION by design: the CAS
// stays in the shell wrappers because adapter record writes ride
// dispatch.sh's lease-held __record-cas re-exec, and moving them here would
// change the authority path (D24). Every error code and phase name below is
// vocabulary internal/dispatch and internal/missionrunner adjudicate on.

// AdjudicateParams is one stage of the turn adjudication.
type AdjudicateParams struct {
	Stage            string // initial | after-repair | settle-result | empty-reply
	Root             string
	Job              string
	RecordPath       string
	SessionID        string
	SchemaPath       string
	CandidatePath    string
	TranscriptPath   string
	ReturnPath       string // round return.json the validation writes
	MarkdownPath     string // round return.md the validation writes
	ViolationPath    string
	RepairPromptPath string
	CLIStatus        int64
	HandshakeDone    bool
	RepairAvailable  bool
	RepairRC         int64
	RepairCandidate  string
	SettleAvailable  bool
	SettleOK         bool
}

// adjudicateValidate runs the same two-step validation the shell composed:
// normalization (writing the round return), then the return-completeness
// judgment. It returns the violation text ("" when valid).
func adjudicateValidate(p AdjudicateParams, candidate, transcript string) (string, error) {
	if err := NormalizeReturn(candidate, transcript, p.RecordPath, p.ReturnPath, p.MarkdownPath, p.SessionID); err != nil {
		return "return normalization failed: " + err.Error() + "\n", nil
	}
	violations := validate.ReturnCompleteJob(p.Root, p.Job)
	if len(violations) == 0 {
		return "", nil
	}
	var text strings.Builder
	for _, violation := range violations {
		fmt.Fprintf(&text, "violation: %s\n", violation)
	}
	return text.String(), nil
}

// writeRepairPrompt reproduces the shell's repair prompt byte for byte: the
// delegate's own violations, the schema, and the one-attempt ask.
func writeRepairPrompt(path, violationText string, schema []byte) error {
	var prompt strings.Builder
	prompt.WriteString("Your previous reply did not validate against the required schema.\n")
	prompt.WriteString("Everything you already did in this session still stands; only the\n")
	prompt.WriteString("shape of the reply was wrong.\n\n# What failed\n\n")
	prompt.WriteString(violationText)
	prompt.WriteString("\n# The schema your reply must satisfy\n\n")
	prompt.Write(schema)
	prompt.WriteString("\n# What to send now\n\n")
	prompt.WriteString("Reply with ONE JSON object valid against that schema and nothing\n")
	prompt.WriteString("else: no prose before or after it, no code fence, no property the\n")
	prompt.WriteString("schema does not name, and every property listed in \"required\".\n")
	prompt.WriteString("Do not repeat the work; report what you already found.\n")
	return os.WriteFile(path, []byte(prompt.String()), 0o644)
}

// AdjudicateTurn decides one stage and prints the verdict the shell
// sequencer executes:
//
//	"fail-pending <error> <phase>"  — CAS pending→failed with the tuple
//	"finish <status> <error> <phase>" — CAS running→status with the tuple
//	"repair"          — violation and repair prompt written; run the repair
//	"settle"          — repaired return validated; run the settle hook
//	"protocol-error"  — hand the violation to the protocol-error writer
func AdjudicateTurn(p AdjudicateParams) (string, error) {
	switch p.Stage {
	case "initial":
		if p.CLIStatus != 0 {
			if p.HandshakeDone {
				return "finish failed runtime_error runtime", nil
			}
			return "fail-pending runtime_error handshake", nil
		}
		if !p.HandshakeDone {
			return "fail-pending handshake_missing_session_id handshake", nil
		}
		violation, err := adjudicateValidate(p, p.CandidatePath, p.TranscriptPath)
		if err != nil {
			return "", err
		}
		if violation == "" {
			return "finish completed null completed", nil
		}
		if err := os.WriteFile(p.ViolationPath, []byte(violation), 0o644); err != nil {
			return "", fmt.Errorf("adjudicate-turn cannot write the violation: %v", err)
		}
		if !p.RepairAvailable {
			return "protocol-error", nil
		}
		schema, err := os.ReadFile(p.SchemaPath)
		if err != nil {
			return "", fmt.Errorf("adjudicate-turn cannot read the schema: %v", err)
		}
		if err := writeRepairPrompt(p.RepairPromptPath, violation, schema); err != nil {
			return "", fmt.Errorf("adjudicate-turn cannot write the repair prompt: %v", err)
		}
		return "repair", nil
	case "after-repair":
		if p.RepairRC != 0 {
			return "protocol-error", nil
		}
		violation, err := adjudicateValidate(p, p.RepairCandidate, "")
		if err != nil {
			return "", err
		}
		if violation != "" {
			if err := os.WriteFile(p.ViolationPath, []byte(violation), 0o644); err != nil {
				return "", fmt.Errorf("adjudicate-turn cannot write the violation: %v", err)
			}
			return "protocol-error", nil
		}
		if p.SettleAvailable {
			return "settle", nil
		}
		return "finish completed null completed", nil
	case "settle-result":
		if p.SettleOK {
			return "finish completed null completed", nil
		}
		return "finish failed session_identity_disagreement delivery", nil
	case "empty-reply":
		// Exit 0 with no reply is a runtime's shape for "could not do it";
		// which failure writer applies depends on where the record got to.
		if p.HandshakeDone {
			return "finish failed empty_reply delivery", nil
		}
		return "fail-pending empty_reply delivery", nil
	}
	return "", fmt.Errorf("adjudicate-turn: unknown stage %q", p.Stage)
}
