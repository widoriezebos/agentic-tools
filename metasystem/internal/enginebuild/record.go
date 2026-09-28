package enginebuild

import (
	"bytes"
	"debug/buildinfo"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// stampSentinelUpper is the only spelling of the record sentinel in source.
// It is lowered at runtime, in the reader and the flag builder alike, so no
// compiled Go binary carries the lowercase sentinel except as a linked -X
// value, and a scan of the file finds only the record the linker placed.
const stampSentinelUpper = "METASYSTEM-BUILD-STAMP="

// stampLimit bounds a stamp token: [A-Za-z0-9-]{1,64}.
const stampLimit = 64

// StampRecordVariable is the linker symbol that carries the record.
const StampRecordVariable = "github.com/widoriezebos/agentic-tools/metasystem/internal/supervise.BuildStampRecord"

// LegacyStampVariable is the linker symbol that carried the stamp before the
// record existed. It is still linked beside the record because readers built
// before the record (an older seat-launch sequencer or steward) read only the
// -ldflags build setting; it can be dropped once no such reader remains.
const LegacyStampVariable = "github.com/widoriezebos/agentic-tools/metasystem/internal/supervise.BuildStamp"

// legacyStampAssignment is the pre-record -ldflags assignment, read from the
// build setting only for binaries built before the record existed.
const legacyStampAssignment = "supervise.BuildStamp="

// ErrStampDisagreement reports a file whose stamp records name different
// stamps: it has no stamp a reader may trust.
var ErrStampDisagreement = errors.New("engine build stamp records disagree")

func stampSentinel() string { return strings.ToLower(stampSentinelUpper) }

// ValidStamp reports whether stamp is grammar-valid: [A-Za-z0-9-]{1,64}.
func ValidStamp(stamp string) bool {
	if len(stamp) == 0 || len(stamp) > stampLimit {
		return false
	}
	for i := 0; i < len(stamp); i++ {
		if !stampByte(stamp[i]) {
			return false
		}
	}
	return true
}

func stampByte(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-'
}

// StampRecord is the byte record a stamp is linked as. The stamp must be
// grammar-valid; callers link only stamps ValidStamp accepts.
func StampRecord(stamp string) string { return stampSentinel() + stamp + ";" }

// StampLinkerFlags is the -ldflags value that links stamp into the engine
// twice: as the legacy variable, which older readers find in the -ldflags
// build setting of an untrimmed build, and as the record, which lives in the
// file's data and so survives -trimpath (which drops that build setting).
func StampLinkerFlags(stamp string) string {
	return "-X " + LegacyStampVariable + "=" + stamp + " -X " + StampRecordVariable + "=" + StampRecord(stamp)
}

// LinkedStamp is the stamp an engine derives at init from what was linked:
// the record's stamp when a record is linked, else the legacy value. A linked
// legacy value (anything but fallback) that disagrees with the record is an
// error: the engine would carry two identities, and its init refuses to run.
func LinkedStamp(record, legacy, fallback string) (string, error) {
	stamp, ok := ParseStampRecord(record)
	switch {
	case !ok:
		return legacy, nil
	case legacy != fallback && legacy != stamp:
		return "", fmt.Errorf("%w: legacy BuildStamp %q, record %q", ErrStampDisagreement, legacy, stamp)
	default:
		return stamp, nil
	}
}

// ParseStampRecord returns the stamp a whole record carries.
func ParseStampRecord(record string) (string, bool) {
	stamp, ok := strings.CutPrefix(record, stampSentinel())
	if !ok {
		return "", false
	}
	stamp, ok = strings.CutSuffix(stamp, ";")
	if !ok || !ValidStamp(stamp) {
		return "", false
	}
	return stamp, true
}

// ReadStamp reads an engine's build stamp from its bytes, never by executing
// it. It scans the whole file for stamp records; every grammar-valid record,
// and the legacy -ldflags build setting when an untrimmed build carries one,
// must name the same stamp (ErrStampDisagreement otherwise). A file without a
// record falls back to that build setting (pre-record binaries); a file with
// neither has no stamp: ("", nil). The file offset is not moved.
func ReadStamp(file *os.File) (string, error) {
	stamp, found, err := scanStampRecords(io.NewSectionReader(file, 0, 1<<62))
	if err != nil {
		return "", err
	}
	legacy := legacyStamp(file)
	if !found {
		return legacy, nil
	}
	if legacy != "" && legacy != stamp {
		return "", ErrStampDisagreement
	}
	return stamp, nil
}

func scanStampRecords(reader io.Reader) (string, bool, error) {
	sentinel := []byte(stampSentinel())
	overlap := len(sentinel) + stampLimit + 2
	chunk := make([]byte, 64<<10)
	var window []byte
	stamp, found := "", false
	for {
		n, readErr := io.ReadFull(reader, chunk)
		window = append(window, chunk[:n]...)
		atEOF := readErr == io.EOF || readErr == io.ErrUnexpectedEOF
		if readErr != nil && !atEOF {
			return "", false, readErr
		}
		// Occurrences starting inside the trailing overlap wait for the
		// next chunk, so a record split across chunks is read whole once.
		limit := len(window) - overlap
		if atEOF {
			limit = len(window)
		}
		for start := 0; start < limit; {
			at := bytes.Index(window[start:], sentinel)
			if at < 0 || start+at >= limit {
				break
			}
			at += start
			if token, ok := recordToken(window[at+len(sentinel):]); ok {
				if found && token != stamp {
					return "", false, ErrStampDisagreement
				}
				stamp, found = token, true
			}
			start = at + 1
		}
		if atEOF {
			return stamp, found, nil
		}
		if limit > 0 {
			window = append(window[:0], window[limit:]...)
		}
	}
}

// recordToken accepts a grammar-valid token followed by ';'.
func recordToken(rest []byte) (string, bool) {
	for i := 0; i < len(rest) && i <= stampLimit; i++ {
		if rest[i] == ';' {
			if i == 0 {
				return "", false
			}
			return string(rest[:i]), true
		}
		if !stampByte(rest[i]) {
			return "", false
		}
	}
	return "", false
}

func legacyStamp(file *os.File) string {
	info, err := buildinfo.Read(file)
	if err != nil {
		return ""
	}
	for _, setting := range info.Settings {
		if setting.Key != "-ldflags" {
			continue
		}
		at := strings.Index(setting.Value, legacyStampAssignment)
		if at < 0 {
			continue
		}
		value := setting.Value[at+len(legacyStampAssignment):]
		if end := strings.IndexAny(value, " \t\r\n\"'"); end >= 0 {
			value = value[:end]
		}
		return strings.TrimSpace(value)
	}
	return ""
}
