package dispatch

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type ReferenceMismatch struct {
	Path     string
	OpenPath string
	Expected string
	Found    string
}

func (m ReferenceMismatch) Line() string {
	return fmt.Sprintf("REFERENCE_MISMATCH path=%s open=%s expected=%s found=%s", m.Path, m.OpenPath, m.Expected, m.Found)
}

// ReadVerifiedReference reads the exact file named by a composition
// reference and returns its bytes only when its location and identity match.
func ReadVerifiedReference(root string, reference CompositionReference) ([]byte, *ReferenceMismatch) {
	expected := fmt.Sprintf("%s:%d", reference.Digest, reference.Bytes)
	mismatch := &ReferenceMismatch{
		Path: reference.Path, OpenPath: reference.OpenPath, Expected: expected,
	}
	canonicalRoot := resolvePath(root)
	if !filepath.IsAbs(reference.OpenPath) || !pathWithin(resolvePath(reference.OpenPath), canonicalRoot) {
		mismatch.Found = "escapes-root"
		return nil, mismatch
	}
	info, err := os.Lstat(reference.OpenPath)
	if err != nil || !info.Mode().IsRegular() {
		mismatch.Found = "unreadable"
		return nil, mismatch
	}
	data, err := os.ReadFile(reference.OpenPath)
	if err != nil {
		mismatch.Found = "unreadable"
		return nil, mismatch
	}
	found := fmt.Sprintf("%s:%d", digestBytes(data), len(data))
	if len(data) != reference.Bytes || digestBytes(data) != reference.Digest {
		mismatch.Found = found
		return nil, mismatch
	}
	return data, nil
}

func VerifyReferences(root, compositionPath string) ([]ReferenceMismatch, error) {
	handle, err := os.Open(compositionPath)
	if err != nil {
		return nil, fmt.Errorf("read composition record: %w", err)
	}
	defer handle.Close()

	var record CompositionRecord
	decoder := json.NewDecoder(handle)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&record); err != nil {
		return nil, fmt.Errorf("decode composition record: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			err = fmt.Errorf("multiple JSON values")
		}
		return nil, fmt.Errorf("decode composition record: %w", err)
	}

	var mismatches []ReferenceMismatch
	for _, reference := range record.References {
		if _, mismatch := ReadVerifiedReference(root, reference); mismatch != nil {
			mismatches = append(mismatches, *mismatch)
		}
	}
	if len(mismatches) == 0 {
		return nil, nil
	}
	return mismatches, nil
}
