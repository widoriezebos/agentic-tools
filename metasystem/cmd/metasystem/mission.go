package main

import "regexp"

var missionIDRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
