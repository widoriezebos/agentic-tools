package partner

import (
	_ "embed"
	"strings"
)

// The Partner's own skill, carried into the first prompt of a session.
//
// How to answer as this Partner is a way of working, so it is a skill in the
// kit like every other: skills/project-partner/SKILL.md, routed from wow.md,
// edited by a maintainer where every other skill is edited. It reaches the
// runtime the way the vocabulary does — as a copy beside this package, so that
// a binary built anywhere carries it — and skill_test.go fails when the copy
// and the kit's own file disagree, so the drift is caught by the test run that
// already exists rather than by a reader noticing.
//
// It is supplied rather than looked up because this runtime has no skill
// engine and needs none: one role, one skill, appended once per session. A
// mechanism for loading any skill on demand is not what a Partner with one job
// requires.

//go:embed project-partner.skill.md
var skillMarkdown string

// SkillPath is where the kit keeps the canonical file. The copy above is built
// from it, and the guard compares them.
const SkillPath = "skills/project-partner/SKILL.md"

// Skill is the skill as the kit writes it, front matter and all.
func Skill() string { return skillMarkdown }

// skillBlock is the skill as a prompt carries it: the body, under a heading
// that says where it came from, so a Partner asked where its instructions come
// from can name the file a human can open.
func skillBlock() string {
	return "How to answer here, from this kit's own " + SkillPath + "\n" +
		strings.TrimSpace(body(skillMarkdown)) + "\n"
}

// body is the skill without its front matter. The name and description are the
// kit's routing, read by whatever loads a skill; what a Partner needs is the
// instructions under them.
func body(markdown string) string {
	trimmed := strings.TrimLeft(markdown, "\n")
	if !strings.HasPrefix(trimmed, "---\n") {
		return markdown
	}
	rest := trimmed[len("---\n"):]
	if at := strings.Index(rest, "\n---\n"); at >= 0 {
		return rest[at+len("\n---\n"):]
	}
	return markdown
}
