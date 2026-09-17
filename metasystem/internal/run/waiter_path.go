package run

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// PathStat is the filesystem boundary used by path observations.
type PathStat func(string) (os.FileInfo, error)

// PathWaitTargetID gives an absolute path a stable identifier that is safe in
// waiter record names.
func PathWaitTargetID(path string) string {
	digest := sha256.Sum256([]byte(filepath.Clean(path)))
	return "path-" + hex.EncodeToString(digest[:8])
}

func pathWaitEvidence(path string, info os.FileInfo) string {
	if info == nil {
		return path + ":absent"
	}
	return path + ":size=" + formatPathSize(info.Size()) + ":mtime=" + info.ModTime().UTC().Format(time.RFC3339Nano)
}

func formatPathSize(size int64) string {
	return strconv.FormatInt(size, 10)
}

// ObservePath reports one filesystem observation. The waiter owns the
// schedule, so this source never delays or retries a stat call.
func ObservePath(ctx context.Context, selector WaitSelector, stat PathStat) (SourceObservation, error) {
	select {
	case <-ctx.Done():
		return SourceObservation{}, ctx.Err()
	default:
	}
	info, err := stat(selector.Path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			observation := SourceObservation{Pending: true, Outcome: "absent", Evidence: pathWaitEvidence(selector.Path, nil)}
			if selector.Until == "absent" {
				observation.Pending = false
				observation.ExitCode = ExitGreen
				observation.Reason = "path is absent"
			}
			return observation, nil
		}
		return SourceObservation{
			Pending: true, Outcome: "stat-error", Evidence: selector.Path,
			PollError: err.Error(),
		}, nil
	}
	evidence := pathWaitEvidence(selector.Path, info)
	observation := SourceObservation{Pending: true, Outcome: "present", Evidence: evidence}
	if selector.Until == "present" && (info.IsDir() || info.Mode().IsRegular() && info.Size() > 0) {
		observation.Pending = false
		observation.ExitCode = ExitGreen
		observation.Reason = "path is present"
		observation.TerminalStamp = info.ModTime().UTC().Format(time.RFC3339Nano)
	}
	return observation, nil
}
