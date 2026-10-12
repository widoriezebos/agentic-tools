package lane

import "strings"

// Remedy translates only lane mechanisms with a known person-facing act.
// Prerequisites and commands the agent executes retain their spelling.
func Remedy(command string) string {
	words := strings.Fields(command)
	if len(words) < 3 || words[0] != "metasystem" {
		return command
	}
	if words[1] == "landing" {
		switch words[2] {
		case "restart":
			return "metasystem landing resume"
		case "run":
			for _, word := range words[3:] {
				if word == "--goals" || strings.HasPrefix(word, "--goals=") {
					return "metasystem landing resume"
				}
			}
		case "return":
			if len(words) > 3 && !strings.HasPrefix(words[3], "--") {
				return "metasystem landing give-back " + words[3] + " --reason TEXT"
			}
		}
	}
	if words[1] == "work" && words[2] == "land" && len(words) > 3 && !strings.HasPrefix(words[3], "--") {
		for i, word := range words[4:] {
			if word == "--exception=GOAL_LAND_TRUNK_RED" || word == "--exception" && i+5 < len(words) && words[i+5] == "GOAL_LAND_TRUNK_RED" {
				return "metasystem landing land-anyway " + words[3] + " --reason TEXT"
			}
		}
	}
	return command
}

func LandAnywayImpact(goal, incident string) string {
	return "lets " + goal + " land while main's incident " + incident + " is open; undo: metasystem landing give-back " + goal + " --reason TEXT"
}
