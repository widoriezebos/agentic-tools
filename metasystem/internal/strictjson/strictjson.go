// Package strictjson reads a JSON record the engine wrote for itself: one
// document, no field the target does not know, nothing after it.
package strictjson

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

// Read decodes path's single JSON document into target, refusing unknown
// fields and trailing content.
func Read(path string, target any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("trailing JSON in %s", path)
	}
	return nil
}
