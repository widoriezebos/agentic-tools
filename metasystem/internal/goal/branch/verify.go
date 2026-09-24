package branch

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing"
)

const LandVerifyCode = "GOAL_LAND_VERIFY"

type Verification struct {
	Commit, Goal, Units, Expected, Actual string
}

type landingGitReader func(string, ...string) ([]byte, error)

func IsLastLanding(repo, commit, goalID string) bool {
	out, err := gitOutput(repo, "show", "-s", "--format=%(trailers:key=Goal-Last,valueonly)", commit)
	return err == nil && strings.TrimSpace(string(out)) == goalID
}

func landedManifest(repo, commit string) (goalID, units, digest string, folds map[string]bool, err error) {
	return landedManifestWithGit(repo, commit, gitOutput)
}

func landedManifestWithGit(repo, commit string, gitRead landingGitReader) (goalID, units, digest string, folds map[string]bool, err error) {
	out, err := gitRead(repo, "show", "-s", "--format=%(trailers:only,unfold=true)", commit)
	if err != nil {
		return "", "", "", nil, err
	}
	var unitValues, digestValues []string
	folds = map[string]bool{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		value = strings.TrimSpace(value)
		switch key {
		case "Goal-Unit":
			unitValues = append(unitValues, value)
		case "Goal-Digest":
			digestValues = append(digestValues, value)
		case "Goal-Fold":
			folds[value] = true
		}
	}
	if len(unitValues) != 1 || len(digestValues) != 1 || len(digestValues[0]) != 64 {
		return "", "", "", nil, operationRefusal(LandVerifyCode, "commit %s has no single landing manifest", commit)
	}
	goalID, parsed, ok := splitGoalUnits(unitValues[0])
	if !ok {
		return "", "", "", nil, operationRefusal(LandVerifyCode, "commit %s has a malformed Goal-Unit", commit)
	}
	if _, decodeErr := hex.DecodeString(digestValues[0]); decodeErr != nil {
		return "", "", "", nil, operationRefusal(LandVerifyCode, "commit %s has a malformed Goal-Digest", commit)
	}
	return goalID, unitList(parsed), digestValues[0], folds, nil
}

func digestLandingEntries(repo, commit string, folds map[string]bool) (string, error) {
	return digestLandingEntriesWithGit(repo, commit, folds, gitOutput)
}

func digestLandingEntriesWithGit(repo, commit string, folds map[string]bool, gitRead landingGitReader) (string, error) {
	entries, err := rawEntriesWithGitParsed(repo, commit, gitRead)
	if err != nil {
		return "", err
	}
	excluded := map[string]bool{}
	for _, path := range landing.WorkspaceExclusions() {
		excluded["metasystem/"+strings.TrimSuffix(path, "/")] = true
	}
	var raw strings.Builder
	for _, entry := range entries {
		skip := folds[entry.Path]
		for path := range excluded {
			if entry.Path == path || strings.HasPrefix(entry.Path, path+"/") {
				skip = true
			}
		}
		if skip {
			continue
		}
		fmt.Fprintf(&raw, ":%s %s %s %s %s%c%s%c", entry.SrcMode, entry.DstMode, entry.SrcBlob, entry.DstBlob, entry.Status, 0, entry.Path, 0)
	}
	sum := sha256.Sum256([]byte(raw.String()))
	return hex.EncodeToString(sum[:]), nil
}

func VerifyLanded(repo, commit string) (Verification, error) {
	return verifyLandedWithGit(repo, commit, gitOutput)
}

func verifyLandedWithGit(repo, commit string, gitRead landingGitReader) (Verification, error) {
	goalID, units, expected, folds, err := landedManifestWithGit(repo, commit, gitRead)
	if err != nil {
		return Verification{}, err
	}
	actual, err := digestLandingEntriesWithGit(repo, commit, folds, gitRead)
	if err != nil {
		return Verification{}, err
	}
	result := Verification{Commit: commit, Goal: goalID, Units: units, Expected: expected, Actual: actual}
	if actual != expected {
		return result, operationRefusal(LandVerifyCode, "commit %s digest is %s, want %s", commit, actual, expected)
	}
	return result, nil
}

func VerifyLandedSeries(repo, tip string) ([]Verification, error) {
	return verifyLandedSeriesWithGit(repo, tip, gitOutput)
}

func verifyLandedSeriesWithGit(repo, tip string, gitRead landingGitReader) ([]Verification, error) {
	goalID, _, _, _, err := landedManifestWithGit(repo, tip, gitRead)
	if err != nil {
		return nil, err
	}
	var reversed []string
	current := tip
	for {
		goal, _, _, _, manifestErr := landedManifestWithGit(repo, current, gitRead)
		if manifestErr != nil || goal != goalID {
			break
		}
		reversed = append(reversed, current)
		parent, parentErr := gitRead(repo, "rev-parse", current+"^")
		if parentErr != nil {
			break
		}
		current = strings.TrimSpace(string(parent))
	}
	results := make([]Verification, 0, len(reversed))
	for index := len(reversed) - 1; index >= 0; index-- {
		verified, err := verifyLandedWithGit(repo, reversed[index], gitRead)
		if err != nil {
			return nil, err
		}
		results = append(results, verified)
	}
	return results, nil
}
