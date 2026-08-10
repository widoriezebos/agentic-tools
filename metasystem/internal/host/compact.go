package host

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
)

// Compact returns the JSON in a file with all insignificant whitespace removed,
// preserving key order and exact number tokens. Runtimes that take a schema as
// a single command-line argument need it on one line.
func Compact(filePath string) (string, error) {
	raw, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", filePath, err)
	}
	var buf bytes.Buffer
	if err := json.Compact(&buf, raw); err != nil {
		return "", fmt.Errorf("compact %s: %w", filePath, err)
	}
	return buf.String(), nil
}
