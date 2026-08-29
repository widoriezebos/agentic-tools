package adapter

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/outage"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/validate"
)

// The adapter turn's terminal-outcome state machine. PURE DECISION by
// design: the CAS
// stays in the shell wrappers because adapter record writes ride
// dispatch.sh's lease-held __record-cas re-exec, and moving them here would
// change the authority path. Every error code and phase name below is
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
	LogPath          string // runtime CLI stderr, the provider-overload evidence
	CLIStatus        int64
	HandshakeDone    bool
	RepairAvailable  bool
	NamedRepairPath  string // the repair attempt's named return file (empty-delivery)
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

// writeDeliveryRepairPrompt is the empty-delivery ask: the session
// did its work and delivered nothing our channels could read; the repair
// names the exact file to write and asks for the print as well.
func writeDeliveryRepairPrompt(path, namedRepairPath string, schema []byte) error {
	var prompt strings.Builder
	prompt.WriteString("Your session completed its work, but no return was delivered:\n")
	prompt.WriteString("nothing was printed and no return file was found. Everything you\n")
	prompt.WriteString("already did still stands; only the delivery is missing.\n\n")
	prompt.WriteString("# What to do now\n\n")
	prompt.WriteString("Write your ONE JSON return object to this exact file path, and\n")
	prompt.WriteString("also print it as your final message. Do not choose a different\n")
	prompt.WriteString("path and do not repeat the work:\n\n")
	prompt.WriteString(namedRepairPath)
	prompt.WriteString("\n\n# The schema your return must satisfy\n\n")
	prompt.Write(schema)
	prompt.WriteString("\n")
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
//
// Around the pure state machine rides the outage bookkeeping
// (provider-outage-posture): a CLI that died on provider overload —
// the paid initial call or the paid repair call, whose stderr shares
// the same log — feeds the shared outage mark, and any PROVIDER
// success clears it. Provider success is the CLI completing a
// conversation, not the task succeeding: a malformed reply or a
// protocol error still proves the provider answered. Best-effort both
// ways, never part of the verdict.
func AdjudicateTurn(p AdjudicateParams) (string, error) {
	verdict, err := adjudicateTurnStage(p)
	if err == nil {
		verdict = criticFailureFold(p.RecordPath, verdict)
	}
	if err != nil || p.Root == "" {
		return verdict, err
	}
	switch {
	case p.Stage == "initial" && p.CLIStatus != 0:
		recordIfOverloaded(p.Root, p.LogPath, p.CandidatePath)
	case p.Stage == "initial":
		// A zero-status CLI can still be carrying the provider's own
		// error document: that records, never clears. Otherwise the
		// clear needs evidence a conversation happened — the
		// correlated handshake — because exit 0 alone does not prove
		// the provider answered.
		if class, evidence, hit := outage.ClassifyProviderResult(p.CandidatePath); hit {
			_, _ = outage.Record(p.Root, class, evidence, "delegate-adapter", time.Now())
		} else if p.HandshakeDone {
			_ = outage.Clear(p.Root)
		}
	case p.Stage == "after-repair" && p.RepairRC != 0:
		recordIfOverloaded(p.Root, p.LogPath, p.RepairCandidate)
	case p.Stage == "after-repair":
		if class, evidence, hit := outage.ClassifyProviderResult(p.RepairCandidate); hit {
			_, _ = outage.Record(p.Root, class, evidence, "delegate-adapter", time.Now())
		} else {
			// The repair only runs inside an established session; its
			// zero exit is a provider conversation.
			_ = outage.Clear(p.Root)
		}
	case verdict == "finish completed null completed":
		_ = outage.Clear(p.Root)
	}
	return verdict, nil
}

// criticFailureFold gives every paid critic attempt a register-consumable
// terminal class. Runtime crashes and empty delivery are transport shapes,
// not permission to skip a critique round; the dispatch register folds both
// as synthetic unproven evidence.
func criticFailureFold(recordPath, verdict string) string {
	record, err := readObject(recordPath)
	if err != nil {
		return verdict
	}
	role, _ := record["role"].(string)
	if role != "design-critic" && role != "code-critic" && role != "warden" {
		return verdict
	}
	switch verdict {
	case "finish failed runtime_error runtime":
		return "finish failed protocol_error runtime"
	case "fail-pending runtime_error handshake":
		return "fail-pending protocol_error handshake"
	case "finish failed empty_reply delivery":
		return "finish failed protocol_error delivery"
	case "fail-pending empty_reply delivery":
		return "fail-pending protocol_error delivery"
	default:
		return verdict
	}
}

// recordIfOverloaded feeds the outage mark when the failed call's
// evidence is overload-shaped: the CLI stderr log, then the structured
// provider result behind its is_error gate.
func recordIfOverloaded(root, logPath, resultPath string) {
	class, evidence, hit := outage.ClassifyLogs(logPath)
	if !hit {
		class, evidence, hit = outage.ClassifyProviderResult(resultPath)
	}
	if hit {
		_, _ = outage.Record(root, class, evidence, "delegate-adapter", time.Now())
	}
}

// adjudicateTurnStage is the state machine itself, pure decision.
func adjudicateTurnStage(p AdjudicateParams) (string, error) {
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
	case "empty-delivery":
		// The delivery-repair recommendation: CLI 0, settlement
		// certified, and the collector found nothing qualified. PURE
		// recommendation — the shell must win `job repair-claim` between
		// this verdict and the paid launch; a lost claim re-enters at
		// empty-reply. Without a correlated session there is nothing to
		// resume and the empty-reply verdicts apply directly.
		if !p.HandshakeDone || !p.RepairAvailable {
			if p.HandshakeDone {
				return "finish failed empty_reply delivery", nil
			}
			return "fail-pending empty_reply delivery", nil
		}
		schema, err := os.ReadFile(p.SchemaPath)
		if err != nil {
			return "", fmt.Errorf("adjudicate-turn cannot read the schema: %v", err)
		}
		if err := writeDeliveryRepairPrompt(p.RepairPromptPath, p.NamedRepairPath, schema); err != nil {
			return "", fmt.Errorf("adjudicate-turn cannot write the repair prompt: %v", err)
		}
		return "delivery-repair", nil
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
