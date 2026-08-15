package host

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DevinConfig writes the Devin CLI config for a turn. It starts from the user's
// own config so the organisation id and onboarding marker survive (a config
// without them makes the CLI print a welcome banner into the turn's stdout,
// where the host reads the return from), drops any sandbox declaration this
// organisation's policy refuses, and pins the workspace-scoped permission set
// the host runs write-capable under.
func DevinConfig(root, outputPath string) error {
	workspace := resolvePath(root)
	value := userDevinConfig()
	delete(value, "sandbox")
	value["permissions"] = map[string]any{
		"allow": []any{
			"read", "grep", "glob", "edit", "exec",
			fmt.Sprintf("Read(%s/**)", workspace),
			fmt.Sprintf("Write(%s/**)", workspace),
		},
		"ask":  []any{},
		"deny": []any{"mcp__*"},
	}
	if err := atomicWriteJSON(outputPath, value); err != nil {
		return fmt.Errorf("write devin config: %w", err)
	}
	return nil
}

// userDevinConfig reads the caller's Devin config as the base to merge onto,
// yielding an empty object when it is absent, unreadable, or not an object.
func userDevinConfig() map[string]any {
	home := os.Getenv("XDG_CONFIG_HOME")
	if home == "" {
		if userHome, err := os.UserHomeDir(); err == nil {
			home = filepath.Join(userHome, ".config")
		}
	}
	if value := loadObject(filepath.Join(home, "devin", "config.json")); value != nil {
		return value
	}
	return map[string]any{}
}

// DevinReturn extracts the turn's return from the runtime's raw stdout. Devin
// has no native structured output, so the return is the JSON object spanning
// the first '{' to the last '}', kept only when it parses as an object.
// Anything else leaves the return absent for the runner's own validation.
func DevinReturn(rawPath, outputPath string) error {
	raw, err := os.ReadFile(rawPath)
	if err != nil {
		return nil
	}
	text := string(raw)
	start := strings.IndexByte(text, '{')
	end := strings.LastIndexByte(text, '}')
	if start < 0 || end <= start {
		return nil
	}
	parsed, err := decodeJSONNumber([]byte(text[start : end+1]))
	if err != nil {
		return nil
	}
	object, ok := parsed.(map[string]any)
	if !ok {
		return nil
	}
	if err := atomicWriteJSON(outputPath, object); err != nil {
		return fmt.Errorf("write devin return: %w", err)
	}
	return nil
}

// resolvePath returns the absolute, symlink-free form of a path, matching the
// canonical form the workspace write boundary is expressed in.
func resolvePath(path string) string {
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	if real, err := filepath.EvalSymlinks(path); err == nil {
		return real
	}
	return path
}

// The devin delivery recollection, registered seam-locally
// (agnosticism audit class 5): re-run the collect ladder past the
// rejected digests, naming devin's own artifact files.
func init() {
	RegisterDeliveryRecollector("devin", func(p RecollectParams) (RecollectResult, error) {
		verdict, err := HostDevinCollect(HostCollectParams{
			Root:           p.Root,
			TurnRecordPath: p.TurnRecordPath,
			TurnDir:        p.TurnDir,
			Workspace:      p.Workspace,
			StdoutPath:     filepath.Join(p.TurnDir, "raw.out"),
			NamedPath:      filepath.Join(p.TurnDir, "devin-return.json"),
			TranscriptPath: filepath.Join(p.TurnDir, "transcript.atif.json"),
			RejectDigests:  p.RejectDigests,
		})
		if err != nil {
			return RecollectResult{}, err
		}
		return RecollectResult{Delivered: verdict.Delivered, ReplyPath: verdict.Reply}, nil
	})
}
