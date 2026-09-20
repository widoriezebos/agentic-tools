package pathpattern

import (
	"encoding/base64"
	"fmt"
	"path/filepath"
	"strings"
)

// A NUL prefix cannot be confused with any repository filename or accepted
// declaration. The version keeps retained manifest entries self-describing.
const literalV1 = "\x00literal:v1:"

func EncodeLiteral(value string) string {
	return literalV1 + base64.RawURLEncoding.EncodeToString([]byte(value))
}

// ManifestEntry separates discovered exact files from declared patterns.
// Untagged entries remain readable as version-zero manifests.
func ManifestEntry(value string) (path string, literal bool, err error) {
	if !strings.HasPrefix(value, literalV1) {
		if strings.HasPrefix(value, "\x00") {
			return "", false, fmt.Errorf("unsupported input manifest entry version")
		}
		return value, false, nil
	}
	decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(value, literalV1))
	if err != nil || len(decoded) == 0 || !filepath.IsLocal(string(decoded)) || strings.ContainsRune(string(decoded), 0) {
		return "", false, fmt.Errorf("invalid literal-v1 input manifest entry")
	}
	return string(decoded), true, nil
}

func MatchManifestEntry(value, candidate string) (bool, error) {
	path, literal, err := ManifestEntry(value)
	if err != nil {
		return false, err
	}
	if literal {
		return path == candidate, nil
	}
	pattern, err := Parse(path)
	if err != nil {
		// Version-zero manifests also carried discovered literal filenames.
		if !filepath.IsLocal(path) || path == "." || filepath.ToSlash(filepath.Clean(path)) != path || strings.ContainsRune(path, 0) {
			return false, fmt.Errorf("invalid version-zero input manifest path %q", path)
		}
		return path == candidate, nil
	}
	return pattern.Covers(candidate), nil
}
