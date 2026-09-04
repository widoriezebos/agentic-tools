package counselor

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

type AcceptedRiskAppend struct {
	Goal, RootJob, FindingID, Class, Title, Claim, Evidence, Why, OpID string
	RecordedAt                                                         time.Time
}

type MisclassificationAppend struct {
	Goal, OpID, Evidence string
	From, To             int
	RecordedAt           time.Time
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

func appendRegisterLine(root, relative, id string, value acceptedRiskRegisterLine) error {
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	lock, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := unix.Flock(int(lock.Fd()), unix.LOCK_EX); err != nil {
		return err
	}
	defer unix.Flock(int(lock.Fd()), unix.LOCK_UN)
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
