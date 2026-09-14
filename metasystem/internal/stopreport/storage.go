// Package stopreport owns durable Stop-report identities, exact short aliases,
// and verified lookup. It does not decide whether a Stop is blocked.
package stopreport

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
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
)

const marker = "\n<!-- metasystem-stop-report-v1 "

var (
	fullIDPattern = regexp.MustCompile(`^([0-9a-f]{64})-([0-9a-f]{32})$`)
	aliasPattern  = regexp.MustCompile(`^[0-9a-f]{1,32}$`)
)

// Identity is the full installation- and attempt-bound identity carried by
// every immutable Stop report.
type Identity struct {
	Installation string `json:"installation"`
	Runtime      string `json:"runtime"`
	Session      string `json:"session"`
	SessionKey   string `json:"sessionKey"`
	Attempt      string `json:"attempt"`
	MainId       string `json:"mainId"`
	Machine      string `json:"machine"`
	Lineage      string `json:"lineage"`
	ObservedAt   string `json:"observedAt"`
	ClaimEpoch   int64  `json:"claimEpoch"`
}

// Reservation is an exclusive alias tombstone owned by one publication.
// A reservation is never removed or reassigned, including after failure.
type Reservation struct {
	Root     string
	Alias    string
	ReportID string
	Path     string
}

// Resolution names the one canonical report selected by an exact alias or
// legacy full identifier. AliasSHA256 is nonempty only for an alias lookup.
type Resolution struct {
	ID          string
	Alias       string
	Path        string
	AliasSHA256 string
}

type aliasEntry struct {
	SchemaVersion int    `json:"schemaVersion"`
	State         string `json:"state"`
	Alias         string `json:"alias"`
	ReportID      string `json:"reportId"`
	SHA256        string `json:"sha256"`
}

// SessionKey is the canonical runtime/session digest used in full report IDs.
func SessionKey(runtime, session string) string {
	encoded, _ := json.Marshal([]string{runtime, session})
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}

// ValidateID accepts either an exact reserved alias or a legacy canonical ID.
func ValidateID(id string) error {
	if aliasPattern.MatchString(id) || fullIDPattern.MatchString(id) {
		return nil
	}
	return fmt.Errorf("stop status id must be 1 to 32 lowercase hexadecimal characters or 64 lowercase hexadecimal characters, a hyphen, and 32 lowercase hexadecimal characters")
}

// ValidateFullID validates the immutable canonical identifier.
func ValidateFullID(id string) error {
	if !fullIDPattern.MatchString(id) {
		return fmt.Errorf("canonical Stop report id must be 64 lowercase hexadecimal characters, a hyphen, and 32 lowercase hexadecimal characters")
	}
	return nil
}

// ReportDir prepares and validates the installation-local report directory.
func ReportDir(root string) (string, error) {
	return reportDir(root, true)
}

func reportDir(root string, create bool) (string, error) {
	dir := filepath.Join(root, "artifacts", "agents", "supervision", "stop-verdicts")
	if create {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", fmt.Errorf("prepare Stop report directory: %w", err)
		}
	}
	if _, err := resolveInstallationDirectory(root, dir); err != nil {
		return "", fmt.Errorf("stop report directory must not contain symlinks")
	}
	return dir, nil
}

// ReserveShortestAlias permanently occupies the shortest free prefix of the
// attempt component. Exclusive creation arbitrates publishers across sessions.
func ReserveShortestAlias(root, reportID string) (Reservation, error) {
	match := fullIDPattern.FindStringSubmatch(reportID)
	if match == nil {
		return Reservation{}, ValidateFullID(reportID)
	}
	reportDir, err := ReportDir(root)
	if err != nil {
		return Reservation{}, err
	}
	aliasDir := filepath.Join(reportDir, "aliases")
	if err := os.MkdirAll(aliasDir, 0o755); err != nil {
		return Reservation{}, fmt.Errorf("prepare Stop report alias directory: %w", err)
	}
	if _, err := resolveInstallationDirectory(root, aliasDir); err != nil {
		return Reservation{}, fmt.Errorf("stop report alias directory must not contain symlinks")
	}
	attempt := match[2]
	for length := 1; length <= len(attempt); length++ {
		alias := attempt[:length]
		path := filepath.Join(aliasDir, alias+".json")
		entry := aliasEntry{SchemaVersion: 1, State: "reserved", Alias: alias, ReportID: reportID, SHA256: ""}
		data, _ := json.Marshal(entry)
		file, openErr := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if os.IsExist(openErr) {
			continue
		}
		if openErr != nil {
			return Reservation{}, fmt.Errorf("reserve Stop report alias %s: %w", alias, openErr)
		}
		writeErr := func() error {
			if _, err := file.Write(append(data, '\n')); err != nil {
				_ = file.Close()
				return err
			}
			if err := file.Sync(); err != nil {
				_ = file.Close()
				return err
			}
			return file.Close()
		}()
		if writeErr != nil {
			return Reservation{}, fmt.Errorf("persist Stop report alias reservation %s: %w", alias, writeErr)
		}
		if err := syncDirectory(aliasDir); err != nil {
			return Reservation{}, fmt.Errorf("persist Stop report alias reservation %s: %w", alias, err)
		}
		return Reservation{Root: root, Alias: alias, ReportID: reportID, Path: path}, nil
	}
	return Reservation{}, fmt.Errorf("all prefixes of Stop report attempt %s are permanently reserved", attempt)
}

// PublishAlias binds an owned tombstone to a complete immutable report.
func PublishAlias(reservation Reservation, reportSHA256 string) error {
	if !validSHA256(reportSHA256) {
		return fmt.Errorf("stop report alias binding requires a complete SHA-256 digest")
	}
	wantPath := filepath.Join(reservation.Root, "artifacts", "agents", "supervision", "stop-verdicts", "aliases", reservation.Alias+".json")
	if reservation.Path != wantPath || !aliasPattern.MatchString(reservation.Alias) || ValidateFullID(reservation.ReportID) != nil {
		return fmt.Errorf("stop report alias reservation identity is invalid")
	}
	data, err := os.ReadFile(reservation.Path)
	if err != nil {
		return fmt.Errorf("read Stop report alias reservation: %w", err)
	}
	entry, err := decodeAliasEntry(data)
	if err != nil || entry.State != "reserved" || entry.Alias != reservation.Alias || entry.ReportID != reservation.ReportID || entry.SHA256 != "" {
		return fmt.Errorf("stop report alias reservation no longer matches its publisher")
	}
	entry.State = "published"
	entry.SHA256 = reportSHA256
	encoded, _ := json.Marshal(entry)
	durable, err := atomicfile.WriteText(reservation.Path, string(encoded)+"\n", reservation.Root)
	if err != nil {
		return fmt.Errorf("publish Stop report alias: %w", err)
	}
	if !durable {
		return fmt.Errorf("publish Stop report alias: crash durability is unknown")
	}
	return nil
}

// Resolve selects exactly one canonical report. Aliases are never treated as
// live prefixes and reserved or damaged entries fail closed.
func Resolve(root, id string) (Resolution, error) {
	if err := ValidateID(id); err != nil {
		return Resolution{}, err
	}
	dir, err := reportDir(root, false)
	if err != nil {
		return Resolution{}, err
	}
	if fullIDPattern.MatchString(id) {
		return Resolution{ID: id, Path: filepath.Join(dir, id+".md")}, nil
	}
	aliasDir := filepath.Join(dir, "aliases")
	if _, err := resolveInstallationDirectory(root, aliasDir); err != nil {
		return Resolution{}, fmt.Errorf("stop report alias directory is unavailable or contains symlinks")
	}
	path := filepath.Join(aliasDir, id+".json")
	info, err := os.Lstat(path)
	if err != nil {
		return Resolution{}, fmt.Errorf("read Stop report alias %s: %w", id, err)
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return Resolution{}, fmt.Errorf("stop report alias %s is not a regular file", id)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Resolution{}, fmt.Errorf("read Stop report alias %s: %w", id, err)
	}
	entry, err := decodeAliasEntry(data)
	if err != nil {
		return Resolution{}, fmt.Errorf("read Stop report alias %s: %w", id, err)
	}
	match := fullIDPattern.FindStringSubmatch(entry.ReportID)
	if entry.State != "published" || entry.Alias != id || match == nil || !strings.HasPrefix(match[2], id) || !validSHA256(entry.SHA256) {
		return Resolution{}, fmt.Errorf("stop report alias %s has no valid published binding", id)
	}
	return Resolution{ID: entry.ReportID, Alias: id, Path: filepath.Join(dir, entry.ReportID+".md"), AliasSHA256: entry.SHA256}, nil
}

// Read verifies the canonical path, one full marker, installation identity,
// and any alias digest before returning report bytes.
func Read(root, id string) ([]byte, Identity, Resolution, error) {
	resolution, err := Resolve(root, id)
	if err != nil {
		return nil, Identity{}, Resolution{}, err
	}
	info, err := os.Lstat(resolution.Path)
	if err != nil {
		return nil, Identity{}, Resolution{}, fmt.Errorf("read Stop report %s: %w", id, err)
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return nil, Identity{}, Resolution{}, fmt.Errorf("stop report %s is not a regular file", id)
	}
	dir := filepath.Dir(resolution.Path)
	resolvedDir, err := resolveInstallationDirectory(root, dir)
	if err != nil {
		return nil, Identity{}, Resolution{}, fmt.Errorf("stop report directory must not be a symlink")
	}
	resolvedPath, err := filepath.EvalSymlinks(resolution.Path)
	if err != nil || filepath.Dir(resolvedPath) != resolvedDir {
		return nil, Identity{}, Resolution{}, fmt.Errorf("stop report %s escapes its report directory", id)
	}
	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return nil, Identity{}, Resolution{}, fmt.Errorf("read Stop report %s: %w", id, err)
	}
	identity, err := ReportIdentity(data)
	if err != nil {
		return nil, Identity{}, Resolution{}, fmt.Errorf("read Stop report %s: %w", id, err)
	}
	match := fullIDPattern.FindStringSubmatch(resolution.ID)
	if identity.Installation != root || identity.SessionKey != match[1] || identity.Attempt != match[2] || identity.SessionKey != SessionKey(identity.Runtime, identity.Session) {
		return nil, Identity{}, Resolution{}, fmt.Errorf("stop report %s identity does not match its installation or filename", id)
	}
	digest := sha256.Sum256(data)
	if resolution.AliasSHA256 != "" && hex.EncodeToString(digest[:]) != resolution.AliasSHA256 {
		return nil, Identity{}, Resolution{}, fmt.Errorf("stop report alias %s digest does not match its immutable report", id)
	}
	return data, identity, resolution, nil
}

// resolveInstallationDirectory accepts symlinks that are already part of the
// installation's own spelling, such as macOS /var -> /private/var, while
// rejecting a directory redirected after the installation boundary. The
// latter could otherwise move reports or alias bindings outside their owner.
func resolveInstallationDirectory(root, dir string) (string, error) {
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	absoluteDir, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	relative, err := filepath.Rel(absoluteRoot, absoluteDir)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("directory is outside the installation")
	}
	resolvedRoot, err := filepath.EvalSymlinks(absoluteRoot)
	if err != nil {
		return "", err
	}
	resolvedDir, err := filepath.EvalSymlinks(absoluteDir)
	if err != nil {
		return "", err
	}
	want := filepath.Join(resolvedRoot, relative)
	if filepath.Clean(resolvedDir) != filepath.Clean(want) {
		return "", fmt.Errorf("directory was redirected below the installation root")
	}
	return resolvedDir, nil
}

// ReportIdentity decodes the report's sole full-identity marker.
func ReportIdentity(data []byte) (Identity, error) {
	if bytes.Count(data, []byte(marker)) != 1 {
		return Identity{}, fmt.Errorf("expected exactly one metasystem-stop-report-v1 identity")
	}
	start := bytes.Index(data, []byte(marker)) + len(marker)
	end := bytes.Index(data[start:], []byte(" -->"))
	if end < 0 {
		return Identity{}, fmt.Errorf("malformed metasystem-stop-report-v1 identity")
	}
	raw := data[start : start+end]
	if err := rejectDuplicateKeys(raw); err != nil {
		return Identity{}, fmt.Errorf("malformed metasystem-stop-report-v1 identity: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var identity Identity
	if err := decoder.Decode(&identity); err != nil {
		return Identity{}, fmt.Errorf("malformed metasystem-stop-report-v1 identity: %w", err)
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return Identity{}, fmt.Errorf("malformed metasystem-stop-report-v1 identity: trailing JSON")
	}
	return identity, nil
}

func decodeAliasEntry(data []byte) (aliasEntry, error) {
	if err := rejectDuplicateKeys(data); err != nil {
		return aliasEntry{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var entry aliasEntry
	if err := decoder.Decode(&entry); err != nil {
		return aliasEntry{}, err
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return aliasEntry{}, fmt.Errorf("trailing JSON")
	}
	if entry.SchemaVersion != 1 {
		return aliasEntry{}, fmt.Errorf("unsupported schema version")
	}
	return entry, nil
}

func rejectDuplicateKeys(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var readValue func() error
	readValue = func() error {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		delim, ok := token.(json.Delim)
		if !ok {
			return nil
		}
		switch delim {
		case '{':
			seen := map[string]bool{}
			for decoder.More() {
				keyToken, err := decoder.Token()
				if err != nil {
					return err
				}
				key, ok := keyToken.(string)
				if !ok {
					return fmt.Errorf("object key is not a string")
				}
				if seen[key] {
					return fmt.Errorf("duplicate JSON key %q", key)
				}
				seen[key] = true
				if err := readValue(); err != nil {
					return err
				}
			}
			_, err = decoder.Token()
			return err
		case '[':
			for decoder.More() {
				if err := readValue(); err != nil {
					return err
				}
			}
			_, err = decoder.Token()
			return err
		default:
			return fmt.Errorf("unexpected JSON delimiter %q", delim)
		}
	}
	if err := readValue(); err != nil {
		return err
	}
	if _, err := decoder.Token(); err != io.EOF {
		if err == nil {
			return fmt.Errorf("trailing JSON")
		}
		return err
	}
	return nil
}

func validSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil && value == strings.ToLower(value)
}

func syncDirectory(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
