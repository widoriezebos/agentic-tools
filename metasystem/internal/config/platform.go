package config

import (
	"fmt"
	"regexp"
	"strings"
)

const DecidingPlatformsKey = "landing.deciding-platforms"

var platformPair = regexp.MustCompile(`^[a-z0-9]+/[a-z0-9]+$`)

func ValidPlatform(platform string) bool { return platformPair.MatchString(platform) }

func validateDecidingPlatforms(value string) error {
	for _, platform := range strings.Fields(value) {
		if !ValidPlatform(platform) {
			return fmt.Errorf("%s needs space-separated goos/goarch pairs", DecidingPlatformsKey)
		}
	}
	return nil
}
