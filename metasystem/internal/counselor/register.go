package counselor

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"golang.org/x/sys/unix"
)

const carriedLandingsSource = "records/counselor/carried-landings.jsonl"

type AcceptedRiskAppend struct {
	Goal, RootJob, FindingID, Class, Title, Claim, Evidence, Why, OpID string
	RecordedAt                                                         time.Time
}

type MisclassificationAppend struct {
	Goal, OpID, Evidence string
	From, To             int
	RecordedAt           time.Time
}

type CarriedLandingJudge struct {
	Mode        string `json:"mode"`
	Tree        string `json:"tree"`
	Digest      string `json:"digest"`
	LiveFailure string `json:"liveFailure"`
}

type CarriedLanding struct {
	SchemaVersion int                 `json:"schemaVersion"`
	ID            string              `json:"id"`
	RecordedAt    string              `json:"recordedAt"`
	Goal          string              `json:"goal"`
	OpID          string              `json:"opid"`
	Commit        string              `json:"commit"`
	Workspace     string              `json:"workspace"`
	Project       string              `json:"project"`
	Past          string              `json:"past"`
	Battery       string              `json:"battery"`
	Missing       string              `json:"missing"`
	Failing       string              `json:"failing"`
	Judge         CarriedLandingJudge `json:"judge"`
	Ledger        string              `json:"ledger"`
	By            string              `json:"by"`
}

type CarriedAcceptedRiskAppend struct {
	Goal, Finding, By, Why, OpID, Commit string
	RecordedAt                           time.Time
}

// CarriedLandingLine derives the durable counter solely from the confirmed
// carried row. The journal is deliberately not an alternate source of truth.
func CarriedLandingLine(row goal.HistoryLine) (CarriedLanding, error) {
	values, outcome, err := carriedRecordValues(row.Reason)
	if err != nil {
		return CarriedLanding{}, err
	}
	if outcome != "landed" {
		return CarriedLanding{}, fmt.Errorf("carried row %s outcome is %s, not landed", row.Opid, outcome)
	}
	goalID := ""
	if len(row.Targets) == 1 {
		goalID = row.Targets[0]
	}
	if goalID == "" || row.Opid == "" {
		return CarriedLanding{}, fmt.Errorf("carried row requires one goal target and an opid")
	}
	recordedAt, err := time.Parse(time.RFC3339, row.At)
	if err != nil {
		return CarriedLanding{}, fmt.Errorf("carried row %s has invalid recorded time: %w", row.Opid, err)
	}
	required := []string{"commit", "workspace", "project", "past", "battery", "missing", "failing", "judge", "judgeTree", "judgeDigest", "liveFailure", "ledger", "by"}
	for _, key := range required {
		if values[key] == "" {
			return CarriedLanding{}, fmt.Errorf("carried row %s is missing %s", row.Opid, key)
		}
	}
	return CarriedLanding{
		SchemaVersion: 1, ID: "cl-" + row.Opid, RecordedAt: recordedAt.UTC().Format(time.RFC3339), Goal: goalID, OpID: row.Opid,
		Commit: values["commit"], Workspace: values["workspace"], Project: values["project"], Past: values["past"], Battery: values["battery"], Missing: values["missing"], Failing: values["failing"],
		Judge: CarriedLandingJudge{Mode: values["judge"], Tree: values["judgeTree"], Digest: values["judgeDigest"], LiveFailure: values["liveFailure"]}, Ledger: values["ledger"], By: values["by"],
	}, nil
}

func carriedRecordValues(reason string) (map[string]string, string, error) {
	fields := strings.Fields(reason)
	if len(fields) == 0 {
		return nil, "", fmt.Errorf("carried row reason is empty")
	}
	values := make(map[string]string, len(fields)-1)
	for _, field := range fields[1:] {
		key, value, ok := strings.Cut(field, "=")
		if !ok || key == "" || value == "" || values[key] != "" {
			return nil, "", fmt.Errorf("carried row has invalid field %q", field)
		}
		values[key] = value
	}
	return values, fields[0], nil
}

func AppendCarriedLanding(root string, line CarriedLanding) error {
	if line.SchemaVersion != 1 || line.ID == "" || line.OpID == "" {
		return fmt.Errorf("carried landing line is incomplete")
	}
	return appendRegisterLine(root, carriedLandingsSource, line.ID, line)
}

// AppendCarriedAcceptedRisk builds the accepted-risk specimen from the eight
// carried trailers on the immutable landed commit. It never reads a job record.
func AppendCarriedAcceptedRisk(root string, in CarriedAcceptedRiskAppend) error {
	line, err := carriedAcceptedRiskLine(root, in)
	if err != nil {
		return err
	}
	return appendRegisterLine(root, acceptedRiskRegisterSource, line.ID, line)
}

func ValidateCarriedAcceptedRisk(root string, in CarriedAcceptedRiskAppend) error {
	_, err := carriedAcceptedRiskLine(root, in)
	return err
}

// ValidateCarriedAcceptedRiskLine proves that one appended register line is
// exactly the specimen derived from a human-carried accepted-risk goal row.
func ValidateCarriedAcceptedRiskLine(root string, in CarriedAcceptedRiskAppend, encoded []byte) error {
	expected, err := carriedAcceptedRiskLine(root, in)
	if err != nil {
		return err
	}
	var got acceptedRiskRegisterLine
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	decodeErr := decoder.Decode(&got)
	var trailing any
	trailingErr := decoder.Decode(&trailing)
	if decodeErr != nil || trailingErr != io.EOF || !reflect.DeepEqual(got, expected) {
		return fmt.Errorf("accepted-risk register line is not the human-carried specimen for %s", in.Finding)
	}
	return nil
}

func carriedAcceptedRiskLine(root string, in CarriedAcceptedRiskAppend) (acceptedRiskRegisterLine, error) {
	if in.Finding == "" || in.Goal == "" || in.OpID == "" || in.By == "" || in.Why == "" || len(in.Commit) != 40 {
		return acceptedRiskRegisterLine{}, fmt.Errorf("carried accepted-risk entry is incomplete")
	}
	command := exec.Command("git", "-C", root, "log", "-1", "--format=%B", in.Commit)
	command.Env = gittree.ScrubbedEnviron()
	message, err := command.Output()
	if err != nil {
		return acceptedRiskRegisterLine{}, fmt.Errorf("read carried commit %s: %w", in.Commit, err)
	}
	keys := []string{"Carry", "Carried-By", "Carried-Tree", "Carried-Past", "Carried-Battery", "Carried-Judge", "Carried-Ledger", "Landing-Provenance"}
	lines := strings.Split(strings.ReplaceAll(string(message), "\r\n", "\n"), "\n")
	citation := acceptedRiskRegisterCitation{Kind: "commit", Target: in.Commit, Detail: in.Finding}
	facts := make([]acceptedRiskRegisterSpecimenFact, 0, len(keys))
	values := map[string]string{}
	for _, key := range keys {
		prefix := key + ": "
		matches := []string{}
		for _, line := range lines {
			if strings.HasPrefix(line, prefix) {
				matches = append(matches, line)
				values[key] = strings.TrimPrefix(line, prefix)
			}
		}
		if len(matches) != 1 || values[key] == "" {
			return acceptedRiskRegisterLine{}, fmt.Errorf("carried commit %s requires exactly one %s trailer", in.Commit, key)
		}
		facts = append(facts, acceptedRiskRegisterSpecimenFact{Fact: matches[0], Citations: []acceptedRiskRegisterCitation{citation}})
	}
	battery := strings.Fields(values["Carried-Battery"])
	if len(battery) == 0 || battery[0] != "green" && battery[0] != "red" {
		return acceptedRiskRegisterLine{}, fmt.Errorf("carried commit %s has invalid Carried-Battery trailer", in.Commit)
	}
	line := acceptedRiskRegisterLine{
		SchemaVersion: 1, ID: "ar-human-carried-" + in.Finding, RecordedAt: in.RecordedAt.UTC().Format(time.RFC3339), Kind: "accepted-risk", Class: "carried-" + battery[0], Title: in.Finding,
		AcceptanceStatus: "accepted", AcceptanceReason: in.Why, SpecimenFacts: facts,
		ReviewLinks: []acceptedRiskRegisterReviewLink{{Kind: "goal", Target: "plans/goals/" + in.Goal + ".md", Detail: "opid=" + in.OpID}, {Kind: "commit", Target: in.Commit, Detail: in.Finding}},
	}
	return line, nil
}

func init() {
	goal.BindCarriedCounselorAppend(func(root, _ string, row goal.HistoryLine, _ time.Time) error {
		line, err := CarriedLandingLine(row)
		if err != nil {
			return err
		}
		return AppendCarriedLanding(root, line)
	})
}

func AppendAcceptedRisk(root string, in AcceptedRiskAppend) error {
	title := strings.TrimSpace(in.Title)
	if title == "" {
		title = firstLine(in.Claim)
	}
	title = truncateBytes(title, 120)
	if title == "" || strings.TrimSpace(in.Why) == "" {
		return fmt.Errorf("accepted-risk register entry requires title and reason")
	}
	citation := acceptedRiskRegisterCitation{Kind: "job-record", Target: "artifacts/agents/jobs/" + in.RootJob + ".json", Detail: in.FindingID}
	var facts []acceptedRiskRegisterSpecimenFact
	for _, line := range strings.Split(strings.ReplaceAll(in.Evidence, "\r\n", "\n"), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			facts = append(facts, acceptedRiskRegisterSpecimenFact{Fact: line, Citations: []acceptedRiskRegisterCitation{citation}})
		}
	}
	if len(facts) == 0 {
		return fmt.Errorf("accepted-risk register entry requires an evidence line")
	}
	line := acceptedRiskRegisterLine{SchemaVersion: 1, ID: "ar-" + in.RootJob + "-" + in.FindingID, RecordedAt: in.RecordedAt.UTC().Format(time.RFC3339), Kind: "accepted-risk", Class: in.Class, Title: title, AcceptanceStatus: "accepted", AcceptanceReason: in.Why, SpecimenFacts: facts, ReviewLinks: []acceptedRiskRegisterReviewLink{{Kind: "goal", Target: "plans/goals/" + in.Goal + ".md", Detail: "opid=" + in.OpID}}}
	return appendRegisterLine(root, acceptedRiskRegisterSource, line.ID, line)
}

func AppendMisclassification(root string, in MisclassificationAppend) error {
	citation := acceptedRiskRegisterCitation{Kind: "goal", Target: "plans/goals/" + in.Goal + ".md", Detail: "opid=" + in.OpID}
	facts := []acceptedRiskRegisterSpecimenFact{{Fact: fmt.Sprintf("from=%d", in.From), Citations: []acceptedRiskRegisterCitation{citation}}, {Fact: fmt.Sprintf("to=%d", in.To), Citations: []acceptedRiskRegisterCitation{citation}}, {Fact: in.Evidence, Citations: []acceptedRiskRegisterCitation{citation}}}
	line := acceptedRiskRegisterLine{SchemaVersion: 1, ID: "mc-" + in.Goal + "-" + in.OpID, RecordedAt: in.RecordedAt.UTC().Format(time.RFC3339), Kind: "misclassification", Class: "tier", Title: fmt.Sprintf("tier raised %d to %d", in.From, in.To), AcceptanceStatus: "recorded", AcceptanceReason: in.Evidence, SpecimenFacts: facts, ReviewLinks: []acceptedRiskRegisterReviewLink{{Kind: "goal", Target: "plans/goals/" + in.Goal + ".md", Detail: "opid=" + in.OpID}}}
	return appendRegisterLine(root, "records/counselor/misclassification-register.jsonl", line.ID, line)
}

func appendRegisterLine(root, relative, id string, value any) (returnErr error) {
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	lockPath := path + ".lock"
	lock, err := acquireRegisterLock(lockPath)
	if err != nil {
		return err
	}
	defer func() {
		if err := releaseRegisterLock(lock, lockPath); returnErr == nil && err != nil {
			returnErr = err
		}
	}()
	if file, err := os.Open(path); err == nil {
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			var row struct {
				ID string `json:"id"`
			}
			if json.Unmarshal(scanner.Bytes(), &row) == nil && row.ID == id {
				file.Close()
				return nil
			}
		}
		file.Close()
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err = file.Write(append(encoded, '\n')); err != nil {
		return err
	}
	return file.Sync()
}

func acquireRegisterLock(lockPath string) (*os.File, error) {
	for {
		lock, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
		if err != nil {
			return nil, err
		}
		if err := unix.Flock(int(lock.Fd()), unix.LOCK_EX); err != nil {
			_ = lock.Close()
			return nil, err
		}
		opened, err := lock.Stat()
		if err != nil {
			_ = releaseRegisterLock(lock, lockPath)
			return nil, fmt.Errorf("stat opened register lock: %w", err)
		}
		current, err := os.Stat(lockPath)
		if err == nil && os.SameFile(opened, current) {
			return lock, nil
		}
		if err != nil && !os.IsNotExist(err) {
			_ = unlockAndCloseRegisterLock(lock)
			return nil, fmt.Errorf("stat register lock path: %w", err)
		}
		if err := unlockAndCloseRegisterLock(lock); err != nil {
			return nil, err
		}
	}
}

func releaseRegisterLock(lock *os.File, lockPath string) error {
	var first error
	if err := os.Remove(lockPath); err != nil && !os.IsNotExist(err) {
		first = fmt.Errorf("remove register lock: %w", err)
	}
	if err := unix.Flock(int(lock.Fd()), unix.LOCK_UN); err != nil && first == nil {
		first = fmt.Errorf("release register lock: %w", err)
	}
	if err := lock.Close(); err != nil && first == nil {
		first = fmt.Errorf("close register lock: %w", err)
	}
	return first
}

func unlockAndCloseRegisterLock(lock *os.File) error {
	if err := unix.Flock(int(lock.Fd()), unix.LOCK_UN); err != nil {
		_ = lock.Close()
		return fmt.Errorf("release stale register lock: %w", err)
	}
	if err := lock.Close(); err != nil {
		return fmt.Errorf("close stale register lock: %w", err)
	}
	return nil
}

func firstLine(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	if line, _, ok := strings.Cut(value, "\n"); ok {
		value = line
	}
	return strings.TrimSpace(value)
}
func truncateBytes(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit]
}
