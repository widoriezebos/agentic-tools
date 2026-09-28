package main

import "regexp"

var missionIDRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
var treeIDRe = regexp.MustCompile(`^[0-9a-f]{40,64}$`)
