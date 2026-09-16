package validate

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
)

type BriefBoundsUnreadable struct{ Detail string }

func (e *BriefBoundsUnreadable) Error() string { return "BRIEF_BOUNDS_UNREADABLE: " + e.Detail }

func briefBoundsUnreadable(format string, args ...any) error {
	return &BriefBoundsUnreadable{Detail: fmt.Sprintf(format, args...)}
}

// ReadRoundBriefBounds reads the admission evidence for one supplied job
// round. Legacy rounds without markers or retained admission files are
// unbounded.
func ReadRoundBriefBounds(root, rootJob, job, roundText string, jobComposition map[string]any) (dispatch.BriefBounds, error) {
	round, err := strconv.ParseInt(roundText, 10, 64)
	if err != nil || round < 1 {
		return dispatch.BriefBounds{}, briefBoundsUnreadable("round %q is not a positive integer", roundText)
	}
	roundDir := filepath.Join(root, "artifacts", "agents", rootJob, "rounds", roundText)
	compositionPath := filepath.Join(roundDir, "composition.json")
	roundComposition, roundPresent, err := readOptionalRoundFile(compositionPath)
	if err != nil {
		return dispatch.BriefBounds{}, briefBoundsUnreadable("read round composition: %v", err)
	}

	var prompt []byte
	if roundPresent || jobComposition != nil {
		prompt, err = os.ReadFile(filepath.Join(roundDir, "prompt.md"))
		if err != nil {
			return dispatch.BriefBounds{}, briefBoundsUnreadable("read round prompt: %v", err)
		}
	}
	var roundMarker, jobMarker *dispatch.AdmittedBriefMarker
	if roundPresent {
		source, _, sourceErr := validateBriefBoundsSource(root, job, roundText, roundComposition, prompt)
		if sourceErr != nil {
			return dispatch.BriefBounds{}, sourceErr
		}
		roundMarker = source.AdmittedBrief
	}
	if jobComposition != nil {
		encoded, encodeErr := json.Marshal(jobComposition)
		if encodeErr != nil {
			return dispatch.BriefBounds{}, briefBoundsUnreadable("encode job composition: %v", encodeErr)
		}
		source, _, sourceErr := validateBriefBoundsSource(root, job, roundText, encoded, prompt)
		if sourceErr != nil {
			return dispatch.BriefBounds{}, sourceErr
		}
		jobMarker = source.AdmittedBrief
	}

	recordPath := filepath.Join(roundDir, "brief-bounds.json")
	copyPath := filepath.Join(roundDir, "admitted-brief.md")
	recordBytes, recordPresent, recordErr := readRegularRoundFile(recordPath)
	copyBytes, copyPresent, copyErr := readRegularRoundFile(copyPath)
	if roundMarker == nil && jobMarker == nil {
		if recordErr != nil || copyErr != nil || recordPresent || copyPresent {
			return dispatch.BriefBounds{}, briefBoundsUnreadable("admission files exist without an admitted brief marker")
		}
		return dispatch.BriefBounds{}, nil
	}
	if roundMarker == nil || jobMarker == nil {
		return dispatch.BriefBounds{}, briefBoundsUnreadable("job and round admitted brief markers are both required")
	}
	if *roundMarker != *jobMarker {
		return dispatch.BriefBounds{}, briefBoundsUnreadable("job and round admitted brief markers do not agree")
	}
	if roundMarker.SchemaVersion != 1 {
		return dispatch.BriefBounds{}, briefBoundsUnreadable("admitted brief marker has unsupported schema version")
	}
	if recordErr != nil || !recordPresent {
		return dispatch.BriefBounds{}, briefBoundsUnreadable("read brief bounds record: %v", recordErr)
	}
	if copyErr != nil || !copyPresent {
		return dispatch.BriefBounds{}, briefBoundsUnreadable("read admitted brief copy: %v", copyErr)
	}
	if got := sourceDigest(recordBytes); got != roundMarker.RecordSHA256 {
		return dispatch.BriefBounds{}, briefBoundsUnreadable("brief bounds record digest is %s, want %s", got, roundMarker.RecordSHA256)
	}
	record, err := dispatch.DecodeBriefBoundsRecord(recordBytes)
	if err != nil {
		return dispatch.BriefBounds{}, briefBoundsUnreadable("decode brief bounds record: %v", err)
	}
	if record.JobID != job || record.RootJob != rootJob || record.Round != round {
		return dispatch.BriefBounds{}, briefBoundsUnreadable("brief bounds record identity does not match job, root job, and round")
	}
	if roundMarker.Bounded != (record.Boundary != nil) {
		return dispatch.BriefBounds{}, briefBoundsUnreadable("admitted brief marker bounded state does not match record")
	}
	if int64(len(copyBytes)) != record.AdmittedBytes {
		return dispatch.BriefBounds{}, briefBoundsUnreadable("admitted brief byte count is %d, want %d", len(copyBytes), record.AdmittedBytes)
	}
	if got := sourceDigest(copyBytes); got != record.AdmittedSHA256 {
		return dispatch.BriefBounds{}, briefBoundsUnreadable("admitted brief digest is %s, want %s", got, record.AdmittedSHA256)
	}
	return record.Bounds(), nil
}

func readOptionalRoundFile(path string) ([]byte, bool, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	return data, err == nil, err
}

func readRegularRoundFile(path string) ([]byte, bool, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, true, err
	}
	if !info.Mode().IsRegular() {
		return nil, true, fmt.Errorf("%s is not a regular file", path)
	}
	data, err := os.ReadFile(path)
	return data, true, err
}

func validateBriefBoundsSource(root, job, roundText string, composition, prompt []byte) (dispatch.CompositionSource, *dispatch.CompositionReference, error) {
	round, err := strconv.ParseInt(roundText, 10, 64)
	if err != nil || round < 1 {
		return dispatch.CompositionSource{}, nil, briefBoundsUnreadable("round %q is not a positive integer", roundText)
	}
	var record dispatch.CompositionRecord
	decoder := json.NewDecoder(bytes.NewReader(composition))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&record); err != nil {
		return dispatch.CompositionSource{}, nil, briefBoundsUnreadable("decode composition: %v", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			err = fmt.Errorf("multiple JSON values")
		}
		return dispatch.CompositionSource{}, nil, briefBoundsUnreadable("decode composition: %v", err)
	}
	if record.JobID != job {
		return dispatch.CompositionSource{}, nil, briefBoundsUnreadable("composition jobId %q does not match job %q", record.JobID, job)
	}
	if record.Round != round {
		return dispatch.CompositionSource{}, nil, briefBoundsUnreadable("composition round %d does not match round %d", record.Round, round)
	}

	var selected *dispatch.CompositionSource
	for index := range record.Sources {
		if record.Sources[index].Slot != "task-direction" {
			continue
		}
		if selected != nil {
			return dispatch.CompositionSource{}, nil, briefBoundsUnreadable("composition has multiple task-direction sources")
		}
		selected = &record.Sources[index]
	}
	if selected == nil {
		return dispatch.CompositionSource{}, nil, briefBoundsUnreadable("composition has no task-direction source")
	}
	if selected.Source != "caller:brief" {
		return dispatch.CompositionSource{}, nil, briefBoundsUnreadable("task-direction source is %q, want caller:brief", selected.Source)
	}
	if selected.StartByte < 0 || selected.StartByte > selected.EndByte || selected.EndByte > len(prompt) {
		return dispatch.CompositionSource{}, nil, briefBoundsUnreadable("task-direction range %d:%d is outside prompt length %d", selected.StartByte, selected.EndByte, len(prompt))
	}
	delivered := prompt[selected.StartByte:selected.EndByte]
	if got := sourceDigest(delivered); got != selected.DeliveredDigest {
		return dispatch.CompositionSource{}, nil, briefBoundsUnreadable("task-direction delivered digest is %s, want %s", selected.DeliveredDigest, got)
	}

	reference, err := validateBriefSourceReferenceBinding(*selected, record.References)
	if err != nil {
		return dispatch.CompositionSource{}, nil, err
	}
	if reference != nil {
		if _, mismatch := dispatch.ReadVerifiedReference(root, *reference); mismatch != nil {
			return dispatch.CompositionSource{}, nil, briefBoundsUnreadable("%s", mismatch.Line())
		}
		return *selected, reference, nil
	}
	if err := validateInlineBriefSource(*selected, delivered); err != nil {
		return dispatch.CompositionSource{}, nil, err
	}
	return *selected, nil, nil
}

func validateBriefSourceReferenceBinding(source dispatch.CompositionSource, references []dispatch.CompositionReference) (*dispatch.CompositionReference, error) {
	var selected *dispatch.CompositionReference
	for index := range references {
		if references[index].Slot != "task-direction" {
			continue
		}
		if selected != nil {
			return nil, briefBoundsUnreadable("composition has multiple task-direction references")
		}
		selected = &references[index]
	}
	if selected == nil {
		return nil, nil
	}
	if selected.Digest != source.SourceDigest {
		return nil, briefBoundsUnreadable("task-direction reference digest does not match sourceDigest")
	}
	if selected.Bytes != source.SourceBytes {
		return nil, briefBoundsUnreadable("task-direction reference bytes do not match sourceBytes")
	}
	if selected.Path == "" {
		return nil, briefBoundsUnreadable("task-direction reference path is empty")
	}
	if filepath.IsAbs(selected.Path) {
		return nil, briefBoundsUnreadable("task-direction reference path is absolute")
	}
	for _, segment := range strings.Split(selected.Path, "/") {
		if segment == ".." {
			return nil, briefBoundsUnreadable("task-direction reference path has a parent component")
		}
	}
	if !filepath.IsAbs(selected.OpenPath) {
		return nil, briefBoundsUnreadable("task-direction reference openPath is not absolute")
	}
	if !strings.HasSuffix(filepath.ToSlash(selected.OpenPath), "/"+selected.Path) {
		return nil, briefBoundsUnreadable("task-direction reference openPath does not end with its path")
	}
	return selected, nil
}

func validateInlineBriefSource(source dispatch.CompositionSource, delivered []byte) error {
	const prefix = "# Task Direction\n\n"
	if !bytes.HasPrefix(delivered, []byte(prefix)) {
		return briefBoundsUnreadable("delivered bytes are not the task-direction envelope")
	}
	remainder := delivered[len(prefix):]
	if source.SourceBytes < 0 || source.SourceBytes > len(remainder) {
		return briefBoundsUnreadable("task-direction inline sourceBytes does not match its body")
	}
	body := remainder[:source.SourceBytes]
	tail := "\n"
	if len(body) == 0 || body[len(body)-1] != '\n' {
		tail = "\n\n"
	}
	if !bytes.Equal(remainder[source.SourceBytes:], []byte(tail)) {
		return briefBoundsUnreadable("delivered bytes are not the task-direction envelope")
	}
	if sourceDigest(body) != source.SourceDigest {
		return briefBoundsUnreadable("task-direction inline sourceDigest does not match its body")
	}
	return nil
}

func sourceDigest(data []byte) string {
	return fmt.Sprintf("%x", sha256.Sum256(data))
}
