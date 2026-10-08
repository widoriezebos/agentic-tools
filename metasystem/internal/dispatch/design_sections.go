package dispatch

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/project"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/readsubject"
)

// DesignRevisionSections joins the preceding examination's immutable section
// history and bound decisions to the replacement page.
func DesignRevisionSections(repoRoot, root string, round int64, page string, decisions []byte) (map[string]string, error) {
	agents := filepath.Join(repoRoot, "artifacts", "agents")
	subject, present, err := readsubject.ReadRoundSubject(agents, root, round)
	if err != nil || !present {
		return nil, fmt.Errorf("section history has no readable frozen page: %v", err)
	}
	if subject.DesignPage == "" {
		return nil, nil
	}
	data, err := os.ReadFile(filepath.Join(agents, root, "rounds", strconv.FormatInt(round, 10), "read.json"))
	if err != nil {
		return nil, err
	}
	var previous readsubject.Read
	if err := json.Unmarshal(data, &previous); err != nil {
		return nil, err
	}
	if !previous.Subject.Equal(subject) || previous.Subject.DesignPage != subject.DesignPage {
		return nil, fmt.Errorf("section history does not bind its frozen page")
	}
	if previous.Design == nil || previous.Design.Root != root {
		return nil, fmt.Errorf("section history has no design identity")
	}
	anchors, err := readsubject.DesignSections(root, previous.Design.RecordID, subject.DesignPage, nil, nil)
	if err != nil || len(anchors) != len(previous.Design.Sections) {
		return nil, fmt.Errorf("section history has unavailable frozen anchors: %v", err)
	}
	ids := map[string]bool{}
	for anchor := range anchors {
		id := previous.Design.Sections[anchor]
		if id == "" || ids[id] {
			return nil, fmt.Errorf("section history has ambiguous identities")
		}
		ids[id] = true
	}
	if len(decisions) != 0 {
		returned, err := os.ReadFile(filepath.Join(agents, root, "rounds", strconv.FormatInt(round, 10), "return.json"))
		if err != nil {
			return nil, err
		}
		binding := fmt.Sprintf("work=design:%s attempt=%d subject=%s examination=%s round=%d return=%x", previous.Design.RecordID, round, subject.ContentDigest, root, round, sha256.Sum256(returned))
		count := 0
		for _, line := range strings.Split(string(decisions), "\n") {
			if strings.HasPrefix(line, "Review binding: ") {
				count++
				if !strings.HasSuffix(strings.TrimSpace(line), " "+binding) {
					return nil, fmt.Errorf("section decisions answer another examination")
				}
			}
		}
		if count != 1 {
			return nil, fmt.Errorf("section decisions have no unique examination binding")
		}
	}
	record, problems, present := project.ParseRecord(subject.DesignPath, page)
	if !present || len(problems) != 0 || record.Kind != project.KindDesign {
		return nil, fmt.Errorf("section revision is not a readable design record")
	}
	return readsubject.DesignSections(root, record.ID, page, &previous, decisions)
}

// frozenDesignDecisions reads the author's exact answer from the retained
// follow-up brief. The round's editable template is not examination evidence.
func frozenDesignDecisions(agents, root, recordID, jobID, operation string, after int64) ([]byte, error) {
	path := filepath.Join(agents, "intent-review", "design-"+strings.ToLower(recordID), "chain.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("frozen section decisions request %s is unreadable: %w", path, err)
	}
	var entry struct {
		Requests []struct {
			Root, Child, OperationID, Brief, DecisionsSHA256 string
			AfterRound                                       int64
		}
	}
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, fmt.Errorf("frozen section decisions request %s is unreadable: %w", path, err)
	}
	for _, request := range entry.Requests {
		if request.Root != root || request.AfterRound != after || (request.Child != jobID && (operation == "" || request.OperationID != operation)) {
			continue
		}
		brief, err := os.ReadFile(request.Brief)
		if err != nil {
			return nil, fmt.Errorf("frozen section decisions brief %s is unreadable: %w", request.Brief, err)
		}
		marker := fmt.Sprintf("\n## The author's decisions on examination %d\n\nThe design changed since examination %d. Judge whether each accepted finding is addressed in the new version.\n\n", after, after)
		_, answer, found := strings.Cut(string(brief), marker)
		if found {
			// A frozen-draft appendix may follow the answer. The digest
			// identifies its exact boundary even when an answer cites drafts.
			for {
				if fmt.Sprintf("%x", sha256.Sum256([]byte(answer))) == request.DecisionsSHA256 {
					return []byte(answer), nil
				}
				boundary := strings.LastIndex(answer, "\n## Frozen drafts\n")
				if boundary < 0 {
					break
				}
				answer = answer[:boundary]
			}
		}
		return nil, fmt.Errorf("frozen section decisions in %s do not match the request's decisions checksum", request.Brief)
	}
	return nil, fmt.Errorf("section history has no frozen decisions request for examination %s", jobID)
}
