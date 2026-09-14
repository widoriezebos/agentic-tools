package report

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stopreport"
)

const (
	StopPresentationSchemaVersion = 2
	StopCompletionLineByteLimit   = 144
	StopTaskLineByteLimit         = 256
	StopHumanLineByteLimit        = 401
	stopReportRetentionCount      = 20
	stopReportRetentionAge        = 24 * time.Hour
)

var stopArmingComponentPattern = regexp.MustCompile(`^component=([^ ]+) outcome=([^ ]+)(?: detail=("(?:\\.|[^"\\])*"))?(?: remedy=("(?:\\.|[^"\\])*"))?$`)

// stopPresentationLockWait leaves the Stop worker a bounded failure path when
// an overlapping presenter is wedged. Tests shorten it to prove the timeout.
var stopPresentationLockWait = 100 * time.Millisecond

type StopIdentity = stopreport.Identity

type StopControl struct {
	ShouldBlock       bool    `json:"shouldBlock"`
	BlockSource       *string `json:"blockSource"`
	Class             string  `json:"class"`
	CauseCode         string  `json:"causeCode"`
	JudgmentAvailable bool    `json:"judgmentAvailable"`
}

type StopDigest struct {
	Mode         string `json:"mode"`
	Text         string `json:"text"`
	SourcePath   string `json:"sourcePath"`
	CursorPrefix string `json:"cursorPrefix"`
	SHA256       string `json:"sha256"`
}

type StopCommandResult struct {
	ExitCode int    `json:"exitCode"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
}

type StopArmingComponent struct {
	Name              string `json:"name"`
	Outcome           string `json:"outcome"`
	Detail            string `json:"detail"`
	Remedy            string `json:"remedy"`
	Owner             string `json:"owner"`
	Restriction       string `json:"restriction"`
	NoAutomaticRemedy bool   `json:"noAutomaticRemedy"`
	HumanRequired     bool   `json:"humanRequired"`
	SupervisionRepair bool   `json:"supervisionRepair"`
}

type StopArmingResult struct {
	ExitCode   int                   `json:"exitCode"`
	Stdout     string                `json:"stdout"`
	Stderr     string                `json:"stderr"`
	Components []StopArmingComponent `json:"components"`
	Aggregate  string                `json:"aggregate"`
}

type StopNotice struct {
	Source            string `json:"source"`
	Class             string `json:"class"`
	CauseCode         string `json:"causeCode"`
	Component         string `json:"component"`
	Detail            string `json:"detail"`
	Remedy            string `json:"remedy"`
	Owner             string `json:"owner"`
	Restriction       string `json:"restriction"`
	HumanRequired     bool   `json:"humanRequired"`
	SupervisionRepair bool   `json:"supervisionRepair"`
	NoAutomaticRemedy bool   `json:"noAutomaticRemedy"`
}

type StopUnavailable struct {
	Section           string `json:"section"`
	Cause             string `json:"cause"`
	Remedy            string `json:"remedy"`
	Owner             string `json:"owner"`
	Restriction       string `json:"restriction"`
	HumanRequired     bool   `json:"humanRequired"`
	SupervisionRepair bool   `json:"supervisionRepair"`
}

type StopPresentationInput struct {
	SchemaVersion         int                        `json:"schemaVersion"`
	Identity              StopIdentity               `json:"identity"`
	Judgment              *goal.TurnVerdictFacts     `json:"judgment"`
	Control               StopControl                `json:"control"`
	CompletionObservation *StopCompletionObservation `json:"completionObservation"`
	Health                *steward.HookHealthPreview `json:"health"`
	Digest                *StopDigest                `json:"digest"`
	Receipt               *StopCommandResult         `json:"receipt"`
	Arming                *StopArmingResult          `json:"arming"`
	Notices               []StopNotice               `json:"notices"`
	Unavailable           []StopUnavailable          `json:"unavailable"`
}

type StopReportReference struct {
	Id          string `json:"id"`
	Alias       string `json:"alias"`
	Path        string `json:"path"`
	ReadCommand string `json:"readCommand"`
	SHA256      string `json:"sha256"`
}

type StopPresentationResult struct {
	SchemaVersion          int                 `json:"schemaVersion"`
	Identity               StopIdentity        `json:"identity"`
	Control                StopControl         `json:"control"`
	HumanLine              string              `json:"humanLine"`
	NeedsYourDecision      bool                `json:"needsYourDecision"`
	NeedsSupervisionRepair bool                `json:"needsSupervisionRepair"`
	Report                 StopReportReference `json:"report"`
}

// StopPresentationCollection is the command boundary's file-only input. The
// hook records each producer once and this composer only joins those bytes; it
// never repeats the turn judgment or health evaluation.
type StopPresentationCollection struct {
	Root, Runtime, Session, Attempt, MainID            string
	Machine, Lineage                                   string
	ClaimEpoch                                         int64
	Advisor                                            bool
	VerdictFile, FactsFile, CompletionFile, HealthFile string
	DigestFile, DigestCursorPrefix                     string
	ReceiptFile, ReceiptStderrFile                     string
	ArmingFile, ArmingStderrFile                       string
	ReceiptExit, ArmingExit                            int
	NoticeFile, FailureFile, OutputFile                string
}

type StopPresentationValidationError struct{ Message string }

func (e StopPresentationValidationError) Error() string { return e.Message }

func StopSessionKey(runtime, session string) string {
	return stopreport.SessionKey(runtime, session)
}

func ValidateStopStatusID(id string) error {
	if err := stopreport.ValidateID(id); err != nil {
		return StopPresentationValidationError{Message: err.Error()}
	}
	return nil
}

// ComposeStopPresentationInput joins the exact files captured by one Stop
// worker into the presenter's versioned input without re-reading live state.
func ComposeStopPresentationInput(collection StopPresentationCollection, now time.Time) error {
	if !filepath.IsAbs(collection.OutputFile) {
		return StopPresentationValidationError{Message: "Stop collection output path must be absolute"}
	}
	if collection.Advisor == (collection.VerdictFile != "") {
		return StopPresentationValidationError{Message: "Stop collection requires exactly one of an advisor outcome or retained verdict"}
	}
	if collection.VerdictFile != "" && !filepath.IsAbs(collection.VerdictFile) {
		return StopPresentationValidationError{Message: "Stop collection verdict path must be absolute"}
	}
	if collection.Advisor && collection.FactsFile != "" {
		return StopPresentationValidationError{Message: "advisor Stop collection cannot include judgment facts"}
	}
	if collection.Advisor && collection.CompletionFile != "" {
		return StopPresentationValidationError{Message: "advisor Stop collection cannot include a completion observation"}
	}
	readOptional := func(path string) ([]byte, error) {
		if path == "" {
			return nil, nil
		}
		if !filepath.IsAbs(path) {
			return nil, StopPresentationValidationError{Message: "Stop collection paths must be absolute"}
		}
		return os.ReadFile(path)
	}
	input := StopPresentationInput{
		SchemaVersion: StopPresentationSchemaVersion,
		Identity: StopIdentity{Installation: collection.Root, Runtime: collection.Runtime, Session: collection.Session,
			SessionKey: StopSessionKey(collection.Runtime, collection.Session), Attempt: collection.Attempt,
			MainId: collection.MainID, Machine: collection.Machine, Lineage: collection.Lineage,
			ObservedAt: now.UTC().Format(time.RFC3339Nano), ClaimEpoch: collection.ClaimEpoch},
		Notices: []StopNotice{}, Unavailable: []StopUnavailable{},
	}
	if collection.Advisor {
		input.Control = StopControl{Class: "advisor", JudgmentAvailable: false}
		input.Unavailable = append(input.Unavailable, StopUnavailable{
			Section: "judgment-facts", Cause: "the read-only advisor path does not run the seat's turn judgment",
			Remedy: "Use scripts/agents/second-session.sh to write independently", Owner: "seat",
		})
	} else {
		verdictBytes, err := os.ReadFile(collection.VerdictFile)
		if err != nil {
			return fmt.Errorf("read retained Stop verdict: %w", err)
		}
		var verdict goal.Verdict
		decoder := json.NewDecoder(bytes.NewReader(verdictBytes))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&verdict); err != nil {
			return StopPresentationValidationError{Message: "invalid retained Stop verdict: " + err.Error()}
		}
		if err := decoder.Decode(&struct{}{}); err != io.EOF {
			return StopPresentationValidationError{Message: "invalid retained Stop verdict: trailing JSON"}
		}
		input.Control = StopControl{ShouldBlock: verdict.ShouldBlock, BlockSource: verdict.BlockSource,
			Class: verdict.Class, CauseCode: verdict.CauseCode, JudgmentAvailable: true}
	}
	if !collection.Advisor && collection.FactsFile != "" {
		factsBytes, readErr := readOptional(collection.FactsFile)
		if readErr == nil {
			factsDecoder := json.NewDecoder(bytes.NewReader(factsBytes))
			factsDecoder.DisallowUnknownFields()
			var facts goal.TurnVerdictFacts
			if decodeErr := factsDecoder.Decode(&facts); decodeErr == nil {
				if trailingErr := factsDecoder.Decode(&struct{}{}); trailingErr == io.EOF {
					input.Judgment = &facts
				} else {
					readErr = fmt.Errorf("trailing JSON")
				}
			} else {
				readErr = decodeErr
			}
		}
		if readErr != nil || input.Judgment == nil {
			input.Unavailable = append(input.Unavailable, StopUnavailable{Section: "judgment-facts", Cause: "the frozen judgment facts were unavailable", Remedy: "The steward must restore supervision", Owner: "steward", SupervisionRepair: true})
		}
	} else if !collection.Advisor {
		input.Unavailable = append(input.Unavailable, StopUnavailable{Section: "judgment-facts", Cause: "the frozen judgment facts were unavailable", Remedy: "The steward must restore supervision", Owner: "steward", SupervisionRepair: true})
	}
	if !collection.Advisor && collection.CompletionFile != "" {
		observation, readErr := completionObservationFile(collection.CompletionFile)
		if readErr == nil {
			input.CompletionObservation = &observation
		}
		if readErr != nil || input.CompletionObservation == nil {
			input.Unavailable = append(input.Unavailable, StopUnavailable{Section: "completion-observation", Cause: "the completion observation was unavailable", Remedy: "The steward must restore supervision", Owner: "steward", SupervisionRepair: true})
		} else if len(input.CompletionObservation.Unavailable) > 0 {
			input.Unavailable = append(input.Unavailable, StopUnavailable{Section: "completion-observation", Cause: strings.Join(input.CompletionObservation.Unavailable, "; "), Remedy: "The steward must restore supervision", Owner: "steward", SupervisionRepair: true})
		}
	} else if !collection.Advisor {
		input.Unavailable = append(input.Unavailable, StopUnavailable{Section: "completion-observation", Cause: "the completion observation was unavailable", Remedy: "The steward must restore supervision", Owner: "steward", SupervisionRepair: true})
	} else {
		input.Unavailable = append(input.Unavailable, StopUnavailable{Section: "completion-observation", Cause: "the read-only advisor has no owned completion observation", Owner: "seat"})
	}
	if collection.HealthFile != "" {
		healthBytes, readErr := readOptional(collection.HealthFile)
		if readErr == nil {
			var health steward.HookHealthPreview
			healthDecoder := json.NewDecoder(bytes.NewReader(healthBytes))
			healthDecoder.DisallowUnknownFields()
			if decodeErr := healthDecoder.Decode(&health); decodeErr == nil && func() bool { return healthDecoder.Decode(&struct{}{}) == io.EOF }() {
				input.Health = &health
			} else {
				readErr = fmt.Errorf("health preview was not valid v1 JSON")
			}
		}
		if readErr != nil || input.Health == nil {
			input.Unavailable = append(input.Unavailable, StopUnavailable{Section: "health", Cause: "the health preview was unavailable", Remedy: "The steward must restore supervision", Owner: "steward", SupervisionRepair: true})
		}
	} else {
		input.Unavailable = append(input.Unavailable, StopUnavailable{Section: "health", Cause: "the health preview was unavailable", Remedy: "The steward must restore supervision", Owner: "steward", SupervisionRepair: true})
	}
	if data, readErr := readOptional(collection.DigestFile); collection.DigestFile != "" && readErr == nil {
		digest := sha256.Sum256(data)
		input.Digest = &StopDigest{Mode: "pending", Text: string(data), SourcePath: collection.DigestFile,
			CursorPrefix: collection.DigestCursorPrefix, SHA256: hex.EncodeToString(digest[:])}
	} else if collection.DigestFile != "" {
		input.Unavailable = append(input.Unavailable, StopUnavailable{Section: "digest", Cause: "the pending digest was unavailable", Remedy: "The steward must restore supervision", Owner: "steward", SupervisionRepair: true})
	} else {
		input.Unavailable = append(input.Unavailable, StopUnavailable{Section: "digest", Cause: "the pending digest was unavailable", Remedy: "The steward must restore supervision", Owner: "steward", SupervisionRepair: true})
	}
	commandResult := func(stdoutPath, stderrPath string, exit int) (*StopCommandResult, error) {
		stdout, readErr := readOptional(stdoutPath)
		if readErr != nil || stdoutPath == "" {
			return nil, readErr
		}
		stderr, stderrErr := readOptional(stderrPath)
		if stderrErr != nil {
			return nil, stderrErr
		}
		return &StopCommandResult{ExitCode: exit, Stdout: string(stdout), Stderr: string(stderr)}, nil
	}
	input.Receipt, _ = commandResult(collection.ReceiptFile, collection.ReceiptStderrFile, collection.ReceiptExit)
	if input.Receipt == nil {
		input.Unavailable = append(input.Unavailable, StopUnavailable{Section: "receipt", Cause: "the receipt check was unavailable", Remedy: "The steward must restore supervision", Owner: "steward", SupervisionRepair: true})
	} else if input.Receipt.ExitCode == 1 {
		input.Notices = append(input.Notices, StopNotice{Source: "receipt", Class: "retro-debt", Detail: "Metasystem retro due", Remedy: "run scripts/receipt.sh check for details, then skills/retro; after completing the retro, record its receipt", Owner: "seat"})
	} else if input.Receipt.ExitCode != 0 {
		input.Notices = append(input.Notices, StopNotice{Source: "receipt", Class: "infrastructure", CauseCode: "receipt-check-error", Component: "receipt", Detail: "Metasystem receipt check errored", Remedy: "run scripts/receipt.sh check to see why", Owner: "steward", SupervisionRepair: true})
	}
	if data, readErr := readOptional(collection.ArmingFile); collection.ArmingFile != "" && readErr == nil {
		stderr, stderrErr := readOptional(collection.ArmingStderrFile)
		if stderrErr != nil {
			readErr = stderrErr
		}
		components, parseErr := parseStopArmingComponents(string(data))
		if readErr == nil && parseErr == nil {
			aggregate := lastNonemptyLine(string(data))
			if aggregate == "" {
				aggregate = lastNonemptyLine(string(stderr))
			}
			input.Arming = &StopArmingResult{ExitCode: collection.ArmingExit, Stdout: string(data), Stderr: string(stderr), Components: components, Aggregate: aggregate}
			if collection.ArmingExit != 0 && len(input.Arming.Components) == 0 {
				detail := strings.TrimSpace(strings.TrimSpace(string(data)) + "\n" + strings.TrimSpace(string(stderr)))
				input.Arming.Components = append(input.Arming.Components, StopArmingComponent{Name: "supervision-arming", Outcome: "failed", Detail: detail, Remedy: "The steward must restore supervision", Owner: "steward", SupervisionRepair: true})
			}
		}
	}
	if input.Arming == nil {
		input.Unavailable = append(input.Unavailable, StopUnavailable{Section: "arming", Cause: "the supervision arming result was unavailable", Remedy: "The steward must restore supervision", Owner: "steward", SupervisionRepair: true})
	}
	if data, readErr := readOptional(collection.NoticeFile); collection.NoticeFile != "" && readErr == nil && len(data) > 0 {
		input.Notices = append(input.Notices, StopNotice{Source: "hook", Class: "attention", Detail: string(data), Owner: "seat"})
	}
	if data, readErr := readOptional(collection.FailureFile); collection.FailureFile != "" && readErr == nil && len(data) > 0 {
		input.Unavailable = append(input.Unavailable, StopUnavailable{Section: "hook", Cause: string(data), Remedy: "The steward must restore supervision", Owner: "steward", SupervisionRepair: true})
	}
	encoded, err := json.Marshal(input)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(collection.OutputFile); err == nil {
		return fmt.Errorf("stop presentation input already exists")
	} else if !os.IsNotExist(err) {
		return err
	}
	durable, err := atomicfile.WriteText(collection.OutputFile, string(encoded)+"\n", collection.Root)
	if err != nil || !durable {
		if err == nil {
			err = fmt.Errorf("crash durability is unknown")
		}
		return fmt.Errorf("publish Stop presentation input: %w", err)
	}
	return nil
}

func lastNonemptyLine(value string) string {
	lines := strings.Split(strings.TrimSpace(value), "\n")
	if len(lines) == 0 {
		return ""
	}
	return lines[len(lines)-1]
}

func parseStopArmingComponents(value string) ([]StopArmingComponent, error) {
	components := []StopArmingComponent{}
	for _, line := range strings.Split(value, "\n") {
		if !strings.HasPrefix(line, "component=") {
			continue
		}
		match := stopArmingComponentPattern.FindStringSubmatch(line)
		if match == nil {
			return nil, fmt.Errorf("arming component line is malformed")
		}
		unquote := func(raw string) (string, error) {
			if raw == "" {
				return "", nil
			}
			return strconv.Unquote(raw)
		}
		detail, err := unquote(match[3])
		if err != nil {
			return nil, fmt.Errorf("arming component detail is malformed: %w", err)
		}
		remedy, err := unquote(match[4])
		if err != nil {
			return nil, fmt.Errorf("arming component remedy is malformed: %w", err)
		}
		failed := match[2] == "failed"
		components = append(components, StopArmingComponent{
			Name: match[1], Outcome: match[2], Detail: detail, Remedy: remedy,
			Owner: "steward", SupervisionRepair: failed,
		})
	}
	return components, nil
}

func ReadStopPresentationInput(path string) (StopPresentationInput, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return StopPresentationInput{}, err
	}
	if err := validateRequiredStopBooleans(data); err != nil {
		return StopPresentationInput{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var input StopPresentationInput
	if err := decoder.Decode(&input); err != nil {
		return StopPresentationInput{}, StopPresentationValidationError{Message: "invalid Stop presentation input: " + err.Error()}
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return StopPresentationInput{}, StopPresentationValidationError{Message: "invalid Stop presentation input: trailing JSON"}
	}
	return input, nil
}

func validateRequiredStopBooleans(data []byte) error {
	var raw map[string]any
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&raw); err != nil {
		return StopPresentationValidationError{Message: "invalid Stop presentation input: " + err.Error()}
	}
	require := func(object map[string]any, fields ...string) error {
		for _, field := range fields {
			if _, ok := object[field].(bool); !ok {
				return StopPresentationValidationError{Message: "invalid Stop presentation input: required boolean " + field + " is missing or has the wrong type"}
			}
		}
		return nil
	}
	requireObject := func(parent map[string]any, field string) (map[string]any, error) {
		object, ok := parent[field].(map[string]any)
		if !ok {
			return nil, StopPresentationValidationError{Message: "invalid Stop presentation input: required object " + field + " is missing or has the wrong type"}
		}
		return object, nil
	}
	requireArray := func(parent map[string]any, field string) error {
		if _, ok := parent[field].([]any); !ok {
			return StopPresentationValidationError{Message: "invalid Stop presentation input: required array " + field + " is missing or has the wrong type"}
		}
		return nil
	}
	objectArray := func(parent map[string]any, field string) ([]map[string]any, error) {
		array, ok := parent[field].([]any)
		if !ok {
			return nil, StopPresentationValidationError{Message: "invalid Stop presentation input: required array " + field + " is missing or has the wrong type"}
		}
		out := make([]map[string]any, 0, len(array))
		for index, item := range array {
			object, ok := item.(map[string]any)
			if !ok {
				return nil, StopPresentationValidationError{Message: fmt.Sprintf("invalid Stop presentation input: %s item %d must be an object", field, index)}
			}
			out = append(out, object)
		}
		return out, nil
	}
	control, err := requireObject(raw, "control")
	if err != nil {
		return err
	}
	if err := require(control, "shouldBlock", "judgmentAvailable"); err != nil {
		return err
	}
	completionValue, completionPresent := raw["completionObservation"]
	if !completionPresent {
		return StopPresentationValidationError{Message: "invalid Stop presentation input: completionObservation is missing"}
	}
	if completionValue != nil {
		completion, err := requireObject(raw, "completionObservation")
		if err != nil {
			return err
		}
		for _, field := range []string{"records", "unavailable"} {
			if err := requireArray(completion, field); err != nil {
				return err
			}
		}
	}
	for _, field := range []string{"notices", "unavailable"} {
		items, err := objectArray(raw, field)
		if err != nil {
			return err
		}
		for _, object := range items {
			if err := require(object, "humanRequired", "supervisionRepair"); err != nil {
				return err
			}
		}
	}
	if raw["arming"] != nil {
		arming, err := requireObject(raw, "arming")
		if err != nil {
			return err
		}
		components, err := objectArray(arming, "components")
		if err != nil {
			return err
		}
		for _, object := range components {
			if err := require(object, "humanRequired", "supervisionRepair", "noAutomaticRemedy"); err != nil {
				return err
			}
		}
	}
	if raw["health"] != nil {
		health, err := requireObject(raw, "health")
		if err != nil {
			return err
		}
		interventions, err := objectArray(health, "interventions")
		if err != nil {
			return err
		}
		for _, object := range interventions {
			if err := require(object, "humanRequired", "supervisionRepair"); err != nil {
				return err
			}
		}
	}
	if raw["judgment"] != nil {
		judgment, err := requireObject(raw, "judgment")
		if err != nil {
			return err
		}
		for _, field := range []string{"identity", "verdict", "scan", "work", "ownership", "refusal"} {
			if _, err := requireObject(judgment, field); err != nil {
				return err
			}
		}
		refusal, err := requireObject(judgment, "refusal")
		if err != nil {
			return err
		}
		if err := require(refusal, "countSpent", "idleRefusal", "humanStopConsumed", "humanRequired", "supervisionRepair"); err != nil {
			return err
		}
		escalation, err := requireObject(refusal, "escalation")
		if err != nil {
			return err
		}
		if err := require(escalation, "intentPrepared", "humanRequired", "supervisionRepair"); err != nil {
			return err
		}
		actions, err := objectArray(judgment, "actions")
		if err != nil {
			return err
		}
		for _, object := range actions {
			if err := require(object, "humanRequired", "supervisionRepair"); err != nil {
				return err
			}
		}
		scan, err := requireObject(judgment, "scan")
		if err != nil {
			return err
		}
		for _, field := range []string{"open", "templateUnfilled", "waitingOnHuman", "stalePlans", "busy", "questions", "drafts"} {
			items, err := objectArray(scan, field)
			if err != nil {
				return err
			}
			for _, object := range items {
				if err := require(object, "previouslyRefused", "humanRequired"); err != nil {
					return err
				}
			}
		}
		for _, field := range []string{"jobs", "runs"} {
			if _, err := objectArray(scan, field); err != nil {
				return err
			}
		}
		for _, field := range []string{"openWorkWarnings", "unreadable", "runUnreadable"} {
			if err := requireArray(scan, field); err != nil {
				return err
			}
		}
		work, _ := requireObject(judgment, "work")
		for _, field := range []string{"claimed", "landing", "claimable", "refused"} {
			if _, err := objectArray(work, field); err != nil {
				return err
			}
		}
		for _, field := range []string{"inFlight", "nonTerminalJobs"} {
			if err := requireArray(work, field); err != nil {
				return err
			}
		}
	}
	return nil
}

func PresentStop(root, inputPath, outputPath string, now time.Time) (StopPresentationResult, error) {
	if !filepath.IsAbs(inputPath) || !filepath.IsAbs(outputPath) {
		return StopPresentationResult{}, StopPresentationValidationError{Message: "stop presentation input and output paths must be absolute"}
	}
	if filepath.Clean(inputPath) == filepath.Clean(outputPath) {
		return StopPresentationResult{}, StopPresentationValidationError{Message: "stop presentation input and output paths must differ"}
	}
	if _, err := os.Lstat(outputPath); err == nil {
		return StopPresentationResult{}, StopPresentationValidationError{Message: "Stop presentation output already exists"}
	} else if !os.IsNotExist(err) {
		return StopPresentationResult{}, StopPresentationValidationError{Message: "cannot inspect Stop presentation output: " + err.Error()}
	}
	resolvedRoot, err := stateroot.RootForCandidate(root)
	if err != nil {
		return StopPresentationResult{}, StopPresentationValidationError{Message: err.Error()}
	}
	input, err := ReadStopPresentationInput(inputPath)
	if err != nil {
		return StopPresentationResult{}, err
	}
	if err := validateStopPresentationInput(resolvedRoot, input); err != nil {
		return StopPresentationResult{}, err
	}
	decision, repair := stopInterventions(input)
	reportID := input.Identity.SessionKey + "-" + input.Identity.Attempt
	reportDir, err := stopreport.ReportDir(resolvedRoot)
	if err != nil {
		return StopPresentationResult{}, err
	}
	lockPath := filepath.Join(reportDir, input.Identity.SessionKey+".present.lock")
	lockFD, err := unix.Open(lockPath, unix.O_CREAT|unix.O_RDWR|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0o600)
	if err != nil {
		return StopPresentationResult{}, fmt.Errorf("open Stop report lock: %w", err)
	}
	lockFile := os.NewFile(uintptr(lockFD), lockPath)
	if lockFile == nil {
		_ = unix.Close(lockFD)
		return StopPresentationResult{}, fmt.Errorf("open Stop report lock: invalid file descriptor")
	}
	defer lockFile.Close()
	if info, statErr := lockFile.Stat(); statErr != nil || !info.Mode().IsRegular() {
		return StopPresentationResult{}, fmt.Errorf("stop report lock is not a regular file")
	}
	if err := lockStopPresentation(lockFile); err != nil {
		return StopPresentationResult{}, err
	}
	defer func() { _ = unix.Flock(int(lockFile.Fd()), unix.LOCK_UN) }()

	reportPath := filepath.Join(reportDir, reportID+".md")
	if _, err := os.Lstat(reportPath); err == nil {
		return StopPresentationResult{}, fmt.Errorf("stop report id collision: %s", reportID)
	} else if !os.IsNotExist(err) {
		return StopPresentationResult{}, fmt.Errorf("inspect Stop report path: %w", err)
	}
	reservation, err := stopreport.ReserveShortestAlias(resolvedRoot, reportID)
	if err != nil {
		return StopPresentationResult{}, fmt.Errorf("reserve Stop report alias: %w", err)
	}
	completion := deriveStopCompletion(input, reportDir)
	firstLine := completionLine(completion)
	readCommand := "metasystem report stop-status --id " + reservation.Alias
	title, titleKind, _ := stopTitle(input)
	secondLine := compactStopLine(title, titleKind, input.Control.ShouldBlock, decision, repair, readCommand)
	humanLine := firstLine + "\n" + secondLine
	if err := ValidateStopHumanLine(humanLine); err != nil {
		return StopPresentationResult{}, fmt.Errorf("compact Stop lines violate their byte or line contract: %w", err)
	}
	result := StopPresentationResult{
		SchemaVersion: StopPresentationSchemaVersion, Identity: input.Identity, Control: input.Control,
		HumanLine: humanLine, NeedsYourDecision: decision, NeedsSupervisionRepair: repair,
	}
	markdown, err := renderStopReport(input, humanLine, completion, decision, repair)
	if err != nil {
		return StopPresentationResult{}, err
	}
	durable, err := atomicfile.WriteText(reportPath, string(markdown), resolvedRoot)
	if err != nil {
		return StopPresentationResult{}, fmt.Errorf("publish Stop report: %w", err)
	}
	if !durable {
		return StopPresentationResult{}, fmt.Errorf("publish Stop report: crash durability is unknown")
	}
	digest := sha256.Sum256(markdown)
	reportSHA := hex.EncodeToString(digest[:])
	if err := stopreport.PublishAlias(reservation, reportSHA); err != nil {
		return StopPresentationResult{}, err
	}
	verified, verifiedIdentity, resolution, err := stopreport.Read(resolvedRoot, reservation.Alias)
	if err != nil || verifiedIdentity != input.Identity || resolution.ID != reportID || resolution.Path != reportPath || !bytes.Equal(verified, markdown) {
		if err == nil {
			err = fmt.Errorf("reserved Stop report alias did not resolve to its published report")
		}
		return StopPresentationResult{}, fmt.Errorf("verify Stop report alias: %w", err)
	}
	result.Report = StopReportReference{Id: reportID, Alias: reservation.Alias, Path: reportPath, ReadCommand: readCommand, SHA256: reportSHA}
	encoded, err := json.Marshal(result)
	if err != nil {
		return StopPresentationResult{}, err
	}
	if _, err := os.Lstat(outputPath); err == nil {
		return StopPresentationResult{}, fmt.Errorf("stop presentation output already exists")
	} else if !os.IsNotExist(err) {
		return StopPresentationResult{}, fmt.Errorf("inspect Stop presentation output: %w", err)
	}
	durable, err = atomicfile.WriteText(outputPath, string(encoded)+"\n", resolvedRoot)
	if err != nil {
		return StopPresentationResult{}, fmt.Errorf("publish Stop presentation: %w", err)
	}
	if !durable {
		return StopPresentationResult{}, fmt.Errorf("publish Stop presentation: crash durability is unknown")
	}
	if err := pruneStopReports(reportDir, input.Identity.SessionKey, reportPath, now.UTC()); err != nil {
		fmt.Fprintf(os.Stderr, "report stop-present: prune old Stop reports: %v\n", err)
	}
	return result, nil
}

func lockStopPresentation(lockFile *os.File) error {
	err := unix.Flock(int(lockFile.Fd()), unix.LOCK_EX|unix.LOCK_NB)
	deadline := time.Now().Add(stopPresentationLockWait)
	for err != nil && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
		err = unix.Flock(int(lockFile.Fd()), unix.LOCK_EX|unix.LOCK_NB)
	}
	if err != nil {
		return fmt.Errorf("lock Stop report: busy after %s: %w", stopPresentationLockWait, err)
	}
	return nil
}

func validateStopPresentationInput(root string, input StopPresentationInput) error {
	invalid := func(message string) error { return StopPresentationValidationError{Message: message} }
	if input.SchemaVersion != StopPresentationSchemaVersion {
		return invalid("unsupported Stop presentation schema version")
	}
	if input.Identity.Installation != root {
		return invalid("Stop presentation installation identity does not match the resolved root")
	}
	if input.Identity.Runtime == "" || input.Identity.Session == "" {
		return invalid("Stop presentation identity is incomplete")
	}
	if input.Identity.SessionKey != StopSessionKey(input.Identity.Runtime, input.Identity.Session) {
		return invalid("Stop presentation session key does not match runtime and session")
	}
	if !regexp.MustCompile(`^[0-9a-f]{32}$`).MatchString(input.Identity.Attempt) {
		return invalid("Stop presentation attempt must be 32 lowercase hexadecimal characters")
	}
	if observedAt, err := time.Parse(time.RFC3339Nano, input.Identity.ObservedAt); err != nil || observedAt.Location() != time.UTC {
		return invalid("Stop presentation observedAt is not UTC RFC3339Nano")
	}
	if input.Identity.ClaimEpoch < 0 {
		return invalid("Stop presentation claimEpoch cannot be negative")
	}
	if input.Control.Class == "" {
		return invalid("Stop presentation control class is required")
	}
	if input.Control.ShouldBlock != (input.Control.BlockSource != nil) {
		return invalid("Stop presentation block source does not match its decision")
	}
	if input.Control.JudgmentAvailable != (input.Judgment != nil) {
		if !(input.Control.JudgmentAvailable && input.Judgment == nil && hasUnavailable(input.Unavailable, "judgment-facts")) {
			return invalid("Stop presentation judgment availability is inconsistent")
		}
	}
	for _, observation := range []struct {
		missing bool
		section string
	}{
		{input.Judgment == nil, "judgment-facts"},
		{input.CompletionObservation == nil, "completion-observation"},
		{input.Health == nil, "health"},
		{input.Digest == nil, "digest"},
		{input.Receipt == nil, "receipt"},
		{input.Arming == nil, "arming"},
	} {
		if observation.missing && !hasUnavailable(input.Unavailable, observation.section) {
			return invalid("Stop presentation missing observation has no unavailable entry: " + observation.section)
		}
	}
	if input.Judgment != nil {
		if input.Judgment.SchemaVersion != 1 {
			return invalid("unsupported frozen judgment schema version")
		}
		identity := input.Judgment.Identity
		if identity.Installation != root || identity.Session != input.Identity.Session || identity.MainId != input.Identity.MainId {
			return invalid("frozen judgment identity does not match Stop identity")
		}
		if input.Judgment.Verdict.SchemaVersion != 1 || input.Control.ShouldBlock != input.Judgment.Verdict.ShouldBlock ||
			!sameStringPointer(input.Control.BlockSource, input.Judgment.Verdict.BlockSource) ||
			input.Control.Class != input.Judgment.Verdict.Class || input.Control.CauseCode != input.Judgment.Verdict.CauseCode {
			return invalid("Stop control does not match the frozen judgment")
		}
		scan := input.Judgment.Scan
		if scan.Open == nil || scan.TemplateUnfilled == nil || scan.OpenWorkWarnings == nil ||
			scan.WaitingOnHuman == nil || scan.StalePlans == nil || scan.Busy == nil ||
			scan.Questions == nil || scan.Drafts == nil || scan.Unreadable == nil ||
			scan.Jobs == nil || scan.Runs == nil || scan.RunUnreadable == nil ||
			input.Judgment.Work.Claimed == nil || input.Judgment.Work.Landing == nil ||
			input.Judgment.Work.Claimable == nil || input.Judgment.Work.Refused == nil ||
			input.Judgment.Work.InFlight == nil || input.Judgment.Work.NonTerminalJobs == nil ||
			input.Judgment.Actions == nil {
			return invalid("frozen judgment arrays must be present")
		}
	}
	if input.CompletionObservation != nil {
		observation := input.CompletionObservation
		if observation.SchemaVersion != StopCompletionObservationSchemaVersion || observation.Records == nil || observation.Unavailable == nil {
			return invalid("unsupported completion observation schema or missing arrays")
		}
		if observation.Identity.Installation != root || observation.Identity.Session != input.Identity.Session || observation.Identity.MainId != input.Identity.MainId {
			return invalid("completion observation identity does not match Stop identity")
		}
		if collectedAt, err := time.Parse(time.RFC3339Nano, observation.CollectedAt); err != nil || collectedAt.Location() != time.UTC {
			return invalid("completion observation collectedAt is not UTC RFC3339Nano")
		}
	}
	if input.Health != nil {
		if input.Health.SchemaVersion != 1 || input.Health.Verdict.Schema != 1 || input.Health.Interventions == nil || input.Health.Verdict.Roles == nil || len(input.Health.Interventions) != len(input.Health.Verdict.Roles) {
			return invalid("health preview interventions do not match the health roles")
		}
		if input.Health.ExitCode != input.Health.Verdict.ExitCode() || input.Health.Line != input.Health.Verdict.Line() {
			return invalid("health preview line or exit code does not match its verdict")
		}
		seen := map[steward.HealthRole]bool{}
		for index, role := range input.Health.Verdict.Roles {
			if role.Status != steward.HealthAlive && role.Status != steward.HealthDead && role.Status != steward.HealthUnknown {
				return invalid("health preview has an invalid role status")
			}
			item := input.Health.Interventions[index]
			if item.Role != role.Role || seen[item.Role] || !steward.KnownHealthRole(item.Role) {
				return invalid("health preview has a missing, duplicate, or out-of-order intervention role")
			}
			seen[item.Role] = true
		}
	}
	if input.Arming != nil && input.Arming.Components == nil {
		return invalid("arming components must be present")
	}
	if input.Notices == nil || input.Unavailable == nil {
		return invalid("Stop presentation arrays must be present")
	}
	if input.Digest != nil {
		digest := sha256.Sum256([]byte(input.Digest.Text))
		if input.Digest.SHA256 != hex.EncodeToString(digest[:]) {
			return invalid("pending digest hash does not match its captured text")
		}
	}
	return nil
}

func hasUnavailable(items []StopUnavailable, section string) bool {
	for _, item := range items {
		if item.Section == section {
			return true
		}
	}
	return false
}

func sameStringPointer(left, right *string) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func stopInterventions(input StopPresentationInput) (decision, repair bool) {
	apply := func(human, supervision bool) { decision = decision || human; repair = repair || supervision }
	if input.Judgment != nil {
		for _, list := range [][]goal.Item{input.Judgment.Scan.Open, input.Judgment.Scan.TemplateUnfilled, input.Judgment.Scan.WaitingOnHuman, input.Judgment.Scan.StalePlans, input.Judgment.Scan.Busy, input.Judgment.Scan.Questions, input.Judgment.Scan.Drafts} {
			for _, item := range list {
				apply(item.HumanRequired, false)
			}
		}
		for _, action := range input.Judgment.Actions {
			apply(action.HumanRequired, action.SupervisionRepair)
		}
		apply(input.Judgment.Refusal.HumanRequired, input.Judgment.Refusal.SupervisionRepair)
		apply(input.Judgment.Refusal.Escalation.HumanRequired, input.Judgment.Refusal.Escalation.SupervisionRepair)
	}
	if input.Health != nil {
		for _, item := range input.Health.Interventions {
			apply(item.HumanRequired, item.SupervisionRepair)
		}
	}
	if input.Arming != nil {
		for _, item := range input.Arming.Components {
			apply(item.HumanRequired, item.SupervisionRepair)
		}
	}
	for _, item := range input.Notices {
		apply(item.HumanRequired, item.SupervisionRepair)
	}
	for _, item := range input.Unavailable {
		apply(item.HumanRequired, item.SupervisionRepair)
	}
	return decision, repair
}

func stopTitle(input StopPresentationInput) (string, string, string) {
	if input.Judgment == nil {
		return "", "unknown", "work evidence was incomplete; showing Task unknown"
	}
	if input.Judgment.Verdict.LedgerStatus == "stopped" {
		return "", "none", "the stopped ledger proves no task in flight"
	}
	jobs := append([]goal.JobFact{}, input.Judgment.Scan.Jobs...)
	sort.SliceStable(jobs, func(i, j int) bool {
		return startedBefore(jobs[i].StartedAt, jobs[i].Id, jobs[j].StartedAt, jobs[j].Id)
	})
	for _, job := range jobs {
		if job.Ownership == "owned" && (job.Status == "pending-setup" || job.Status == "pending" || job.Status == "running") {
			if title, ok := goalIDTaskName(job.GoalId); ok {
				return title, "task", "selected owned nonterminal job; used goal ID " + job.GoalId
			}
			if title, ok := shortTaskLabel(job.Role, true); ok {
				return title, "task", "selected owned nonterminal job; goal ID was unusable; used role " + job.Role
			}
			return "task name unavailable", "task", "selected owned nonterminal job; goal ID and role were unusable; showing task name unavailable"
		}
	}
	runs := append([]goal.RunFact{}, input.Judgment.Scan.Runs...)
	sort.SliceStable(runs, func(i, j int) bool {
		return startedBefore(runs[i].StartedAt, runs[i].Id, runs[j].StartedAt, runs[j].Id)
	})
	for _, run := range runs {
		if run.Ownership == "owned" && (run.Status == "launching" || run.Status == "running" || run.Status == "draining") {
			if title, ok := goalIDTaskName(run.GoalId); ok {
				return title, "task", "selected owned active run; used goal ID " + run.GoalId
			}
			if title, ok := shortTaskLabel(run.Title, false); ok {
				return title, "task", "selected owned active run; goal ID was unusable; used run display " + title
			}
			return "task name unavailable", "task", "selected owned active run; goal ID and run display were unusable; showing task name unavailable"
		}
	}
	if input.Judgment.Work.Selection == "held" {
		if !input.Judgment.Work.ReadSucceeded || input.Judgment.Ownership.State != "owned" || input.Judgment.Work.Selected == nil {
			return "", "unknown", "held-work ownership evidence was incomplete; showing Task unknown"
		}
		if title, ok := goalIDTaskName(input.Judgment.Work.Selected.Id); ok {
			return title, "task", "selected held goal; used goal ID " + input.Judgment.Work.Selected.Id
		}
		return "task name unavailable", "task", "selected held goal; goal ID was unusable; showing task name unavailable"
	}
	if input.Judgment.Work.Selection == "claimable" {
		if !input.Judgment.Work.ReadSucceeded || input.Judgment.Ownership.State != "none" || input.Judgment.Work.Selected == nil {
			return "", "unknown", "claimable-work ownership evidence was incomplete; showing Task unknown"
		}
		if title, ok := goalIDTaskName(input.Judgment.Work.Selected.Id); ok {
			return title, "next", "selected claimable goal; used goal ID " + input.Judgment.Work.Selected.Id
		}
		return "task name unavailable", "next", "selected claimable goal; goal ID was unusable; showing task name unavailable"
	}
	if !input.Judgment.Work.ReadSucceeded || input.Judgment.Ownership.State == "unknown" || input.Judgment.Work.Selection == "unknown" {
		return "", "unknown", "work evidence was incomplete; showing Task unknown"
	}
	return "", "none", "no owned or selected work was proven"
}

func startedBefore(leftAt, leftID, rightAt, rightID string) bool {
	if leftAt == "" {
		leftAt = "9999"
	}
	if rightAt == "" {
		rightAt = "9999"
	}
	if leftAt == rightAt {
		return leftID < rightID
	}
	return leftAt < rightAt
}

func goalIDTaskName(id string) (string, bool) {
	if id == "" || len(id) > 100 {
		return "", false
	}
	hasLetter := false
	for _, r := range id {
		if r >= 'a' && r <= 'z' {
			hasLetter = true
			continue
		}
		if (r < '0' || r > '9') && r != '-' {
			return "", false
		}
	}
	if !hasLetter || opaqueHexTaskKey(id) {
		return "", false
	}
	return strings.Join(strings.Fields(strings.ReplaceAll(id, "-", " ")), " "), true
}

func shortTaskLabel(source string, role bool) (string, bool) {
	for _, r := range source {
		if unicode.IsControl(r) || r == '\u2028' || r == '\u2029' {
			return "", false
		}
	}
	trimmed := strings.TrimSpace(source)
	if trimmed == "" || opaqueHexTaskKey(trimmed) {
		return "", false
	}
	if role {
		trimmed = strings.NewReplacer("-", " ", "_", " ").Replace(trimmed)
	}
	label := strings.Join(strings.Fields(trimmed), " ")
	if label == "" || len(label) > 100 {
		return "", false
	}
	hasLetter := false
	for _, r := range label {
		switch {
		case unicode.IsLetter(r):
			hasLetter = true
		case unicode.IsMark(r), unicode.IsDigit(r), r == ' ', r == '-', r == '/', r == '&', r == '+', r == '(', r == ')':
		default:
			return "", false
		}
	}
	return label, hasLetter
}

func opaqueHexTaskKey(value string) bool {
	compact := strings.ReplaceAll(value, "-", "")
	if len(compact) != 32 && len(compact) != 40 && len(compact) != 64 {
		return false
	}
	for _, r := range compact {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return false
		}
	}
	return true
}

func compactStopLine(title, kind string, blocked, decision, repair bool, readCommand string) string {
	outcome := "Stop allowed"
	imperative := "; Read, then continue lawful work before stopping: "
	if blocked {
		outcome = "Stop blocked"
		imperative = "; Read: "
	}
	intervention := ""
	switch {
	case decision && repair:
		intervention = "; needs your decision and supervision repair"
	case decision:
		intervention = "; needs your decision"
	case repair:
		intervention = "; needs supervision repair"
	}
	prefix := "No task in flight"
	switch kind {
	case "task":
		prefix = "Task: " + title
	case "next":
		prefix = "No task in flight; next: " + title
	case "unknown":
		prefix = "Task unknown"
	}
	suffix := "; " + outcome + intervention + imperative + readCommand
	available := StopTaskLineByteLimit - len(suffix)
	if kind == "task" {
		available -= len("Task: ")
	} else if kind == "next" {
		available -= len("No task in flight; next: ")
	}
	if (kind == "task" || kind == "next") && len(title) > available {
		title = truncateUTF8(title, available)
	}
	if kind == "task" {
		prefix = "Task: " + title
	} else if kind == "next" {
		prefix = "No task in flight; next: " + title
	}
	return prefix + suffix
}

func truncateUTF8(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	if len(value) <= limit {
		return value
	}
	end := limit
	for end > 0 && !utf8.ValidString(value[:end]) {
		end--
	}
	return strings.TrimSpace(value[:end])
}

func normalizeCompactText(value string) string {
	var out strings.Builder
	space := false
	for _, r := range value {
		if unicode.IsControl(r) || unicode.IsSpace(r) || r == '\u2028' || r == '\u2029' {
			space = out.Len() > 0
			continue
		}
		if space {
			out.WriteByte(' ')
			space = false
		}
		out.WriteRune(r)
	}
	return strings.TrimSpace(out.String())
}

func validStopLogicalLine(line string) bool {
	if line == "" || !utf8.ValidString(line) || strings.ContainsAny(line, "\r\n") {
		return false
	}
	for _, r := range line {
		if unicode.IsControl(r) || r == '\u2028' || r == '\u2029' {
			return false
		}
	}
	return true
}

func ValidateStopHumanLine(line string) error {
	if len(line) > StopHumanLineByteLimit {
		return fmt.Errorf("stop human lines exceed %d UTF-8 bytes including their LF", StopHumanLineByteLimit)
	}
	if strings.Count(line, "\n") != 1 {
		return fmt.Errorf("stop human text must contain exactly two logical lines")
	}
	lines := strings.SplitN(line, "\n", 2)
	if len(lines[0]) > StopCompletionLineByteLimit {
		return fmt.Errorf("stop completion line exceeds %d UTF-8 bytes", StopCompletionLineByteLimit)
	}
	if len(lines[1]) > StopTaskLineByteLimit {
		return fmt.Errorf("stop task line exceeds %d UTF-8 bytes", StopTaskLineByteLimit)
	}
	if !validStopLogicalLine(lines[0]) || !validStopLogicalLine(lines[1]) {
		return fmt.Errorf("stop human text must be two valid UTF-8 logical lines without controls")
	}
	return nil
}

func renderStopReport(input StopPresentationInput, humanLine string, completion StopCompletion, decision, repair bool) ([]byte, error) {
	outcome := "Stop allowed"
	if input.Control.ShouldBlock {
		outcome = "Stop blocked"
	}
	title, kind, namingSummary := stopTitle(input)
	heading := "No task in flight"
	if kind == "task" {
		heading = "Task: " + title
	} else if kind == "next" {
		heading = "No task in flight; next: " + title
	} else if kind == "unknown" {
		heading = "Task unknown"
	}
	identityJSON, _ := json.Marshal(input.Identity)
	identityJSON = bytes.ReplaceAll(identityJSON, []byte("--"), []byte(`-\u002d`))
	var report strings.Builder
	fmt.Fprintf(&report, "# %s; %s\n\n<!-- metasystem-stop-report-v1 %s -->\n\n", heading, outcome, identityJSON)
	fmt.Fprintf(&report, "Observed at: %s\n\n", input.Identity.ObservedAt)
	if decision || repair {
		report.WriteString("## Action needed\n\n")
		if decision {
			report.WriteString("A human decision is required. The exact requests are listed first below.\n\n")
		}
		if repair {
			report.WriteString("Supervision needs repair. The exact causes, owners, and remedies are listed first below.\n\n")
		}
		writeInterventionActions(&report, input)
	}
	report.WriteString("## Seat actions\n\n")
	if input.Judgment == nil || len(input.Judgment.Actions) == 0 {
		report.WriteString("No seat action was available from the frozen judgment.\n\n")
	} else {
		for _, action := range input.Judgment.Actions {
			fmt.Fprintf(&report, "- %s (%s): %s", action.Kind, action.TargetId, action.Instruction)
			if action.Command != "" {
				fmt.Fprintf(&report, " Command: `%s`.", action.Command)
			}
			if action.Owner != "" {
				fmt.Fprintf(&report, " Owner: %s.", action.Owner)
			}
			if action.Restriction != "" {
				fmt.Fprintf(&report, " Restriction: %s.", action.Restriction)
			}
			report.WriteString("\n")
		}
		report.WriteString("\n")
	}
	report.WriteString("## Summary\n\n")
	blockSource := "none"
	if input.Control.BlockSource != nil {
		blockSource = *input.Control.BlockSource
	}
	fmt.Fprintf(&report, "- Block source: %s\n- Task naming: %s.\n", blockSource, namingSummary)
	switch completion.State {
	case "observed":
		fmt.Fprintf(&report, "- Completion: newly observed successful terminal work since %s; baseline report %s.\n", completion.IntervalStart, completion.BaselineReportId)
	case "none":
		fmt.Fprintf(&report, "- Completion: no new recorded successful terminal work since %s; baseline report %s.\n", completion.IntervalStart, completion.BaselineReportId)
	default:
		report.WriteString("- Completion: unknown for this turn because there is no complete usable previous observation for this exact seat interval.\n")
	}
	if input.Judgment != nil {
		fmt.Fprintf(&report, "- Claimable goals: %d; queued goals: %d; non-terminal jobs: %d.\n", len(input.Judgment.Work.Claimable), input.Judgment.Work.Queued, len(input.Judgment.Work.NonTerminalJobs))
		writeBacklogSummary(&report, input.Judgment)
	}
	if input.Health != nil {
		alive, dead, unknown := 0, 0, 0
		for _, role := range input.Health.Verdict.Roles {
			switch role.Status {
			case steward.HealthAlive:
				alive++
			case steward.HealthDead:
				dead++
			default:
				unknown++
			}
		}
		if dead == 0 && unknown == 0 {
			fmt.Fprintf(&report, "- Health: all %d roles alive.\n", alive)
		} else {
			fmt.Fprintf(&report, "- Health: %d alive, %d dead, %d unknown.\n", alive, dead, unknown)
		}
		for _, role := range input.Health.Verdict.Roles {
			if role.Status == steward.HealthAlive && !(role.Role == steward.RoleSpendFence && role.Remedy != "") {
				continue
			}
			fmt.Fprintf(&report, "- Health %s=%s: %s", role.Role, role.Status, role.Reason)
			if role.Remedy != "" {
				fmt.Fprintf(&report, " Remedy: %s.", role.Remedy)
			}
			report.WriteString("\n")
		}
		if input.Health.Verdict.Stopped {
			fmt.Fprintf(&report, "- Health stop state: stopped at %s with %d unresolved processes.\n", input.Health.Verdict.StopPhase, input.Health.Verdict.StopUnresolved)
		}
	}
	report.WriteString("\n")
	fmt.Fprintf(&report, "## Console text\n\n```text\n%s\n```\n\n", humanLine)
	sections := []struct {
		name  string
		value any
	}{
		{"Retained Stop control", input.Control}, {"Completion observation", input.CompletionObservation}, {"Completion", completion}, {"Original turn verdict and frozen judgment", input.Judgment}, {"Health", input.Health}, {"Pending digest", input.Digest},
		{"Receipt", input.Receipt}, {"Supervision arming", input.Arming}, {"Notices", input.Notices}, {"Unavailable observations", input.Unavailable},
	}
	for _, section := range sections {
		data, err := json.MarshalIndent(section.value, "", "  ")
		if err != nil {
			return nil, err
		}
		fmt.Fprintf(&report, "## %s\n\n```json\n%s\n```\n\n", section.name, data)
	}
	fmt.Fprintf(&report, "## Existing records\n\n- Full turn verdict: `artifacts/agents/supervision/stop-verdicts/%s.txt`\n- Stop refusal records: `artifacts/agents/supervision/stop-refusals/`\n- Narrator log: `records/narrator-digest.log`\n", goal.NormalizeSession(input.Identity.Session))
	return []byte(report.String()), nil
}

func writeInterventionActions(report *strings.Builder, input StopPresentationInput) {
	write := func(source, detail, remedy, owner, restriction string, human, repair bool) {
		if !human && !repair {
			return
		}
		fmt.Fprintf(report, "- %s: %s", source, detail)
		if remedy != "" {
			fmt.Fprintf(report, " Remedy: %s.", remedy)
		}
		if owner != "" {
			fmt.Fprintf(report, " Owner: %s.", owner)
		}
		if restriction != "" {
			fmt.Fprintf(report, " Restriction: %s.", restriction)
		}
		report.WriteString("\n")
	}
	if input.Judgment != nil {
		for _, action := range input.Judgment.Actions {
			write("judgment action "+action.Kind, action.Instruction, action.Command, action.Owner, action.Restriction, action.HumanRequired, action.SupervisionRepair)
		}
		refusal := input.Judgment.Refusal
		write("turn refusal", refusal.Detail, refusal.Remedy, refusal.Component, "", refusal.HumanRequired, refusal.SupervisionRepair)
		write("idle escalation", refusal.Escalation.Detail, refusal.Escalation.AlarmDetail, "steward", "", refusal.Escalation.HumanRequired, refusal.Escalation.SupervisionRepair)
	}
	if input.Health != nil {
		for index, intervention := range input.Health.Interventions {
			role := input.Health.Verdict.Roles[index]
			write("health "+string(role.Role), role.Reason, role.Remedy, intervention.Owner, intervention.Restriction, intervention.HumanRequired, intervention.SupervisionRepair)
		}
	}
	if input.Arming != nil {
		for _, component := range input.Arming.Components {
			write("arming "+component.Name, component.Detail, component.Remedy, component.Owner, component.Restriction, component.HumanRequired, component.SupervisionRepair)
		}
	}
	for _, notice := range input.Notices {
		write("notice "+notice.Source, notice.Detail, notice.Remedy, notice.Owner, notice.Restriction, notice.HumanRequired, notice.SupervisionRepair)
	}
	for _, unavailable := range input.Unavailable {
		write("unavailable "+unavailable.Section, unavailable.Cause, unavailable.Remedy, unavailable.Owner, unavailable.Restriction, unavailable.HumanRequired, unavailable.SupervisionRepair)
	}
	report.WriteString("\n")
}

func writeBacklogSummary(report *strings.Builder, judgment *goal.TurnVerdictFacts) {
	work := judgment.Work
	selected := "no selected goal"
	if work.Selected != nil {
		selected = work.Selected.Id
		if work.Selected.Intent != "" {
			selected += " (" + normalizeCompactText(work.Selected.Intent) + ")"
		}
	}
	other := len(work.Claimable)
	if work.Selection == "claimable" && other > 0 {
		other--
	}
	if judgment.Refusal.IdleRefusal && judgment.Refusal.Occurrence > 0 {
		if judgment.Refusal.Occurrence < 3 {
			fmt.Fprintf(report, "- Backlog: refusal %d of 3 for this unchanged backlog; at 3 request steward continuation and allow Stop unless another branch blocks; selected %s; %d other goals.\n", judgment.Refusal.Occurrence, selected, other)
		} else {
			fmt.Fprintf(report, "- Backlog: refusal %d reached the bound of 3 for this unchanged backlog; selected %s; %d other goals.\n", judgment.Refusal.Occurrence, selected, other)
		}
	}
	delegateInFlight := false
	for _, activity := range work.InFlight {
		if strings.HasPrefix(activity, "job:") {
			delegateInFlight = true
			break
		}
	}
	if delegateInFlight && len(work.Claimable) > 0 {
		fmt.Fprintf(report, "- Backlog: delegate work is in flight; claimable backlog remains; selected %s; %d other goals.\n", selected, other)
	}
}

func pruneStopReports(dir, sessionKey, current string, now time.Time) error {
	paths, err := filepath.Glob(filepath.Join(dir, sessionKey+"-*.md"))
	if err != nil {
		return err
	}
	type entry struct {
		path string
		mod  time.Time
	}
	entries := make([]entry, 0, len(paths))
	for _, path := range paths {
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			continue
		}
		entries = append(entries, entry{path: path, mod: info.ModTime()})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].mod.After(entries[j].mod) })
	for index, item := range entries {
		if item.path == current || index < stopReportRetentionCount || now.Sub(item.mod) < stopReportRetentionAge {
			continue
		}
		if err := os.Remove(item.path); err != nil {
			return err
		}
	}
	return nil
}

func StopStatusRoot(explicit string) (string, error) {
	if explicit != "" {
		return stateroot.RootForCandidate(explicit)
	}
	executable, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("locate executing metasystem binary: %w", err)
	}
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		return "", fmt.Errorf("resolve executing metasystem binary: %w", err)
	}
	if filepath.Base(executable) != "metasystem" || filepath.Base(filepath.Dir(executable)) != "bin" {
		return "", fmt.Errorf("executing binary is not installed as <installation>/bin/metasystem")
	}
	return stateroot.RootForCandidate(filepath.Dir(filepath.Dir(executable)))
}

func ReadStopStatus(root, id string) ([]byte, StopIdentity, error) {
	if err := ValidateStopStatusID(id); err != nil {
		return nil, StopIdentity{}, err
	}
	resolvedRoot, err := StopStatusRoot(root)
	if err != nil {
		return nil, StopIdentity{}, err
	}
	data, identity, _, err := stopreport.Read(resolvedRoot, id)
	return data, identity, err
}

func AbsoluteStopStatusCommand(root, id string) string {
	return fmt.Sprintf("%s report stop-status --id %s --root %s", shellQuote(filepath.Join(root, "bin", "metasystem")), id, shellQuote(root))
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
