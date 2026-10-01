package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The two output audits share one table: the wording audit
// (TestAuditMessagesAPersonReads, the words) and the layout audit
// (TestAuditOutputLayout, the shape). A key is a module-relative package
// directory, a file, or a file#Function; the longest key naming a source
// wins, per audit, and a path no key names is reported by both. group is
// the conversion group that owns the path (plans/designs/output-style.md
// §7): one file, one builder, for the words and the shape alike. A builder
// that converts a path sets its audit's mode in the same commit.
type auditMode struct {
	group    string
	messages string
	layout   string
}

const (
	auditEnforce = "enforce"
	auditReport  = "report"
)

var auditModes = map[string]auditMode{
	// Step 0's reference conversions: status and grant list.
	"cmd/metasystem/intent_table.go#runIntentTopStatus":        {layout: auditEnforce},
	"cmd/metasystem/intent_process.go#runIntentCheckoutStatus": {layout: auditEnforce},
	"cmd/metasystem/intent_status_view.go":                     {group: "G1a", layout: auditEnforce},
	"cmd/metasystem/intent_planning.go#runIntentGrantList":     {layout: auditEnforce},
	"cmd/metasystem/intent_planning.go#grantListView":          {layout: auditEnforce},
	"cmd/metasystem/intent_grant_everything.go#grantAttention": {layout: auditEnforce},
	"cmd/metasystem/intent_helm.go#helmReading.attention":      {layout: auditEnforce},

	// G1a status and helm.
	"cmd/metasystem/intent_process.go#runIntentSystemStart":   {layout: auditEnforce},
	"cmd/metasystem/intent_process.go#runIntentSystemStop":    {layout: auditEnforce},
	"cmd/metasystem/intent_process.go#runIntentSystemRestart": {layout: auditEnforce},
	"cmd/metasystem/intent_process.go#runIntentSystemStatus":  {layout: auditEnforce},
	"cmd/metasystem/intent_process.go#runIntentWorkStatus":    {layout: auditEnforce},
	"cmd/metasystem/intent_process.go#runIntentQuestionList":  {layout: auditEnforce},
	// machine start, converted at integration (round 2).
	"cmd/metasystem/intent_process.go#runIntentStartMachine": {layout: auditEnforce},
	"cmd/metasystem/intent_helm.go#runIntentHelmTake":        {layout: auditEnforce},
	"cmd/metasystem/intent_process.go#runIntentWorkStop":     {layout: auditEnforce},
	"cmd/metasystem/intent_process.go#runIntentSystemSetup":  {layout: auditEnforce},
	"cmd/metasystem/intent_process.go#runIntentDoctor":       {layout: auditEnforce},
	"cmd/metasystem/intent_table.go#runIntentDesignCheck":    {layout: auditEnforce},
	"cmd/metasystem/intent_process.go#runIntentEnroll":       {layout: auditEnforce},
	"cmd/metasystem/intent_table.go":                         {group: "G1a"},
	"cmd/metasystem/intent_process.go":                       {group: "G1a"},
	"cmd/metasystem/intent_helm.go":                          {group: "G1a"},
	"cmd/metasystem/intent_grant_everything.go":              {group: "G1a"},
	"internal/board/view.go":                                 {group: "G1a"},
	"internal/stoptransition/transition.go":                  {group: "G1a"},
	"internal/stoptransition/families.go":                    {group: "G1a"},
	// G1b machine and landing.
	"cmd/metasystem/intent_machine.go":        {group: "G1b", layout: auditEnforce},
	"cmd/metasystem/intent_landing.go":        {group: "G1b", layout: auditEnforce},
	"cmd/metasystem/intent_landing_engine.go": {group: "G1b", layout: auditEnforce},
	"cmd/metasystem/intent_landing_kernel.go": {group: "G1b", layout: auditEnforce},
	"cmd/metasystem/intent_alert.go":          {group: "G1b"},
	// G2 goals and grants.
	"cmd/metasystem/intent_goals.go":                       {group: "G2"},
	"cmd/metasystem/intent_planning.go":                    {group: "G2"},
	"cmd/metasystem/intent_goal_review.go":                 {group: "G2"},
	"cmd/metasystem/intent_goal_land_without_sitting.go":   {group: "G2"},
	"cmd/metasystem/intent_goal_permission.go":             {group: "G2"},
	"cmd/metasystem/goal_list_view.go":                     {group: "G2", layout: auditEnforce},
	"cmd/metasystem/goal_show_view.go":                     {group: "G2", layout: auditEnforce},
	"cmd/metasystem/intent_planning.go#runIntentGoalViews": {layout: auditEnforce},
	"cmd/metasystem/intent_goals.go#runIntentGoals":        {layout: auditEnforce},
	"cmd/metasystem/intent_goals.go#runIntentPause":        {layout: auditEnforce},
	"cmd/metasystem/intent_goals.go#runIntentShow":         {layout: auditEnforce},
	"cmd/metasystem/incident_list_view.go":                 {group: "G2", layout: auditEnforce},
	"cmd/metasystem/intent_planning.go#runIntentIncidents": {layout: auditEnforce},
	// G3 disk and evidence.
	"cmd/metasystem/intent_disk.go":     {group: "G3"},
	"cmd/metasystem/intent_evidence.go": {group: "G3"},
	"internal/diskstore/report.go":      {group: "G3"},
	"internal/evidence/bound.go":        {group: "G3"},
	// Their run functions print through textui; diskTrim's progress notes
	// stay on stderr beside the page.
	"cmd/metasystem/intent_disk.go#runIntentDiskShow":            {layout: auditEnforce},
	"cmd/metasystem/intent_disk.go#runIntentDiskClean":           {layout: auditEnforce},
	"cmd/metasystem/intent_evidence.go#runIntentEvidenceShow":    {layout: auditEnforce},
	"cmd/metasystem/intent_evidence.go#runIntentEvidenceExport":  {layout: auditEnforce},
	"cmd/metasystem/intent_evidence.go#runIntentEvidenceDispose": {layout: auditEnforce},
	// G4 delivery.
	"cmd/metasystem/intent_work.go":           {group: "G4"},
	"cmd/metasystem/intent_delivery.go":       {group: "G4"},
	"cmd/metasystem/intent_work_workspace.go": {group: "G4"},
	"cmd/metasystem/intent_design.go":         {group: "G4"},
	"cmd/metasystem/intent_sent_back.go":      {group: "G4"},
	"cmd/metasystem/intent_land_change.go":    {group: "G4"},
	"cmd/metasystem/intent_land_release.go":   {group: "G4"},
	"cmd/metasystem/intent_land_staged.go":    {group: "G4"},
	"cmd/metasystem/intent_manual_review.go":  {group: "G4"},
	"cmd/metasystem/intent_manual_submit.go":  {group: "G4"},
	"cmd/metasystem/intent_unit_review.go":    {group: "G4"},
	"cmd/metasystem/intent_review_binding.go": {group: "G4"},
	"cmd/metasystem/intent_review_help.go":    {group: "G4"},
	"cmd/metasystem/intent_selection.go":      {group: "G4"},
	"cmd/metasystem/intent_references.go":     {group: "G4"},
	// G4a's converted verbs: their run functions print through textui.
	"cmd/metasystem/intent_design.go#runIntentDesign":            {layout: auditEnforce},
	"cmd/metasystem/intent_work.go#runIntentBrief":               {layout: auditEnforce},
	"cmd/metasystem/intent_work.go#runIntentBuild":               {layout: auditEnforce},
	"cmd/metasystem/intent_work.go#runIntentWorkWait":            {layout: auditEnforce},
	"cmd/metasystem/intent_work.go#runIntentTest":                {layout: auditEnforce},
	"cmd/metasystem/intent_work.go#runIntentDeclareStopMoves":    {layout: auditEnforce},
	"cmd/metasystem/intent_work.go#runIntentTestWait":            {layout: auditEnforce},
	"cmd/metasystem/intent_work.go#runIntentSettings":            {layout: auditEnforce},
	"cmd/metasystem/intent_work.go#runIntentSettingsKeys":        {layout: auditEnforce},
	"cmd/metasystem/intent_work.go#runIntentSettingsSet":         {layout: auditEnforce},
	"cmd/metasystem/intent_work.go#runIntentSettingsCheck":       {layout: auditEnforce},
	"cmd/metasystem/intent_delivery.go#runIntentReview":          {layout: auditEnforce},
	"cmd/metasystem/intent_sent_back.go#runIntentReviseSentBack": {layout: auditEnforce},
	"cmd/metasystem/intent_delivery.go#runIntentLand":            {layout: auditEnforce},
	"cmd/metasystem/intent_delivery.go#runIntentWorkFinish":      {layout: auditEnforce},
	// G5 the rest of the intent verbs.
	"cmd/metasystem/intent_agent.go":             {group: "G5"},
	"cmd/metasystem/intent_app.go":               {group: "G5", layout: auditEnforce},
	"cmd/metasystem/intent_adopt.go":             {group: "G5"},
	"cmd/metasystem/intent_questions.go":         {group: "G5"},
	"cmd/metasystem/intent_operations.go":        {group: "G5"},
	"cmd/metasystem/intent_exception.go":         {group: "G5"},
	"cmd/metasystem/intent_exception_release.go": {group: "G5"},
	"cmd/metasystem/completion.go":               {group: "G5"},
	// G5's converted actions (the app verbs run closures of their file).
	"cmd/metasystem/intent_operations.go#runIntentGoalSync":            {layout: auditEnforce},
	"cmd/metasystem/intent_operations.go#runIntentSettingsCoordinator": {layout: auditEnforce},
	"cmd/metasystem/intent_operations.go#runIntentDesignShow":          {layout: auditEnforce},
	"cmd/metasystem/intent_operations.go#runIntentDesignList":          {layout: auditEnforce},
	"cmd/metasystem/intent_operations.go#runIntentDecisionList":        {layout: auditEnforce},
	"cmd/metasystem/intent_operations.go#runIntentDecisionShow":        {layout: auditEnforce},
	"cmd/metasystem/intent_agent.go#runAgentAsk":                       {layout: auditEnforce},
	"cmd/metasystem/intent_agent.go#runAgentReply":                     {layout: auditEnforce},
	"cmd/metasystem/intent_agent.go#runAgentInbox":                     {layout: auditEnforce},
	"cmd/metasystem/intent_adopt.go#runIntentSystemAdopt":              {layout: auditEnforce},
	"cmd/metasystem/intent_questions.go#runIntentQuestionRetry":        {layout: auditEnforce},
	"cmd/metasystem/intent_questions.go#runIntentQuestionWithdraw":     {layout: auditEnforce},
	"cmd/metasystem/intent_questions.go#runIntentQuestionShow":         {layout: auditEnforce},
	"cmd/metasystem/intent_questions.go#runIntentQuestionWait":         {layout: auditEnforce},
	// G6 passthroughs.
	"cmd/metasystem/test.go":             {group: "G6"},
	"cmd/metasystem/receipt_verbs.go":    {group: "G6"},
	"cmd/metasystem/report.go":           {group: "G6"},
	"cmd/metasystem/report_frontier.go":  {group: "G6"},
	"cmd/metasystem/context_verbs.go":    {group: "G6"},
	"cmd/metasystem/validate_verbs.go":   {group: "G6"},
	"cmd/metasystem/session_isolate.go":  {group: "G6"},
	"cmd/metasystem/testing_merge.go":    {group: "G6"},
	"internal/cliflags/cliflags.go":      {group: "G6"},
	"cmd/metasystem/passthrough_page.go": {group: "G6"},
	// The passthroughs' handlers print through textui.
	"cmd/metasystem/intent_table.go#runReceiptAdd":             {layout: auditEnforce},
	"cmd/metasystem/intent_table.go#runReceiptStatus":          {layout: auditEnforce},
	"cmd/metasystem/receipt_verbs.go#runReceiptRetro":          {layout: auditEnforce},
	"cmd/metasystem/report_frontier.go#runExperimentRecord":    {layout: auditEnforce},
	"cmd/metasystem/report_frontier.go#runExperimentChallenge": {layout: auditEnforce},
	"cmd/metasystem/report_frontier.go#runExperimentStatus":    {layout: auditEnforce},
	"cmd/metasystem/validate_verbs.go#runValidateStopLoss":     {layout: auditEnforce},
	"cmd/metasystem/report.go#runReportStopStatus":             {layout: auditEnforce},
	"cmd/metasystem/intent_table.go#runSessionHandoff":         {layout: auditEnforce},
	"cmd/metasystem/session_isolate.go#runSessionIsolate":      {layout: auditEnforce},
	"cmd/metasystem/test.go#runTestPlan":                       {layout: auditEnforce},
	"cmd/metasystem/testing_merge.go#runTestingAddTests":       {layout: auditEnforce},
	"cmd/metasystem/testing_merge.go#runTestingRemoveTests":    {layout: auditEnforce},
	"cmd/metasystem/intent_table.go#runTestBaseline":           {layout: auditEnforce},
	"cmd/metasystem/test.go#runTestList":                       {layout: auditEnforce},
	"cmd/metasystem/intent_table.go#runTestStatus":             {layout: auditEnforce},
	"cmd/metasystem/wait_register.go":                          {group: "G1a"},

	// Verbs whose handler is defined in another group's file (a closure in
	// the command table, or a passthrough's owner) belong to the group the
	// design names for them.
	"verb:design show":          {group: "G5"},
	"verb:design list":          {group: "G5"},
	"verb:decision list":        {group: "G5"},
	"verb:decision show":        {group: "G5"},
	"verb:question show":        {group: "G5"},
	"verb:question retry":       {group: "G5"},
	"verb:question withdraw":    {group: "G5"},
	"verb:question wait":        {group: "G5"},
	"verb:machine start":        {group: "G1b"},
	"verb:receipt add":          {group: "G6"},
	"verb:receipt status":       {group: "G6"},
	"verb:receipt retro":        {group: "G6"},
	"verb:experiment record":    {group: "G6"},
	"verb:experiment challenge": {group: "G6"},
	"verb:experiment status":    {group: "G6"},
	"verb:session handoff":      {group: "G6"},
	"verb:test status":          {group: "G6"},
	"verb:test baseline":        {group: "G6"},
}

// verbGroup is the group the table names for a verb itself, if any.
func verbGroup(name string) string { return auditModes["verb:"+name].group }

// auditKeys are the keys that may name a source, longest first: its
// function, its file, then each directory above it.
func auditKeys(file, function string) []string {
	keys := []string{}
	if function != "" {
		keys = append(keys, file+"#"+function)
	}
	keys = append(keys, file)
	for dir := filepath.ToSlash(filepath.Dir(file)); dir != "." && dir != "/"; dir = filepath.ToSlash(filepath.Dir(dir)) {
		keys = append(keys, dir)
	}
	return keys
}

// auditLookup is one column's value for a source: the longest key that
// sets it, or "" when none does.
func auditLookup(file, function string, column func(auditMode) string) string {
	for _, key := range auditKeys(file, function) {
		if mode, ok := auditModes[key]; ok && column(mode) != "" {
			return column(mode)
		}
	}
	return ""
}

func layoutModeFor(file, function string) string {
	if mode := auditLookup(file, function, func(m auditMode) string { return m.layout }); mode != "" {
		return mode
	}
	return auditReport
}

func auditGroupFor(file, function string) string {
	return auditLookup(file, function, func(m auditMode) string { return m.group })
}

// TestAuditModesNameRealPaths keeps the shared table honest: every key
// names a path that exists, a mode is report or enforce, and a group is one
// of the design's.
func TestAuditModesNameRealPaths(t *testing.T) {
	t.Parallel()
	module, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	groups := map[string]bool{"": true, "G1a": true, "G1b": true, "G2": true, "G3": true, "G4": true, "G5": true, "G6": true}
	verbs := map[string]bool{}
	for _, command := range publicIntentCommands() {
		verbs[command.name] = true
	}
	for key, mode := range auditModes {
		if verb, isVerb := strings.CutPrefix(key, "verb:"); isVerb {
			if !verbs[verb] || mode.group == "" || mode.messages != "" || mode.layout != "" {
				t.Errorf("%s: a verb key names a public verb and only its group", key)
			}
			continue
		}
		path, function, _ := strings.Cut(key, "#")
		data, err := os.ReadFile(filepath.Join(module, path))
		if err != nil {
			if info, statErr := os.Stat(filepath.Join(module, path)); statErr != nil || !info.IsDir() {
				t.Errorf("%s names no path: %v", key, err)
			}
		}
		if receiver, method, isMethod := strings.Cut(function, "."); isMethod {
			if !strings.Contains(string(data), receiver+") "+method+"(") {
				t.Errorf("%s names no method in %s", key, path)
			}
		} else if function != "" && !strings.Contains(string(data), "func "+function+"(") {
			t.Errorf("%s names no function in %s", key, path)
		}
		for _, value := range []string{mode.messages, mode.layout} {
			if value != "" && value != auditEnforce && value != auditReport {
				t.Errorf("%s: mode %q is neither report nor enforce", key, value)
			}
		}
		if !groups[mode.group] {
			t.Errorf("%s: group %q is not one of the design's", key, mode.group)
		}
	}
}
