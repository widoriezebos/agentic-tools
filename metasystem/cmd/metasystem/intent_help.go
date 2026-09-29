package main

import (
	"fmt"
	"io"
	"slices"
	"strings"
)

// Help projects the public descriptors without resolving a repository or
// constructing execution owners. Form contracts describe behavior; they never
// decide whether the caller is allowed to perform it.
type intentHelpForm struct {
	name, purpose                  string
	usage, inputs                  []string
	effects, authority, repetition string
	flags, example                 []string
	optionNotes                    map[string]intentHelpOptionNote
}

type intentHelpOptionNote struct {
	value, description string
}

type intentHelpOption struct {
	Name        string   `json:"name"`
	Value       string   `json:"value,omitempty"`
	Aliases     []string `json:"aliases,omitempty"`
	Repeat      bool     `json:"repeat,omitempty"`
	Rest        bool     `json:"rest,omitempty"`
	Advanced    bool     `json:"advanced,omitempty"`
	Description string   `json:"description"`
}

type intentHelpEntry struct {
	Name     string   `json:"name"`
	Group    string   `json:"group"`
	Summary  string   `json:"summary"`
	Audience string   `json:"audience"`
	Scope    string   `json:"scope"`
	HelpArgv []string `json:"helpArgv"`
}

type intentHelpFormEntry struct {
	Name     string   `json:"name"`
	Purpose  string   `json:"purpose"`
	HelpArgv []string `json:"helpArgv"`
}

type intentHelpCommand struct {
	intentHelpEntry
	Usage               []string              `json:"usage"`
	AdministrationUsage []string              `json:"administrationUsage,omitempty"`
	Options             []intentHelpOption    `json:"options"`
	Details             []string              `json:"details,omitempty"`
	Examples            []string              `json:"examples,omitempty"`
	Forms               []intentHelpFormEntry `json:"forms,omitempty"`
}

type intentHelpFormDocument struct {
	intentHelpFormEntry
	Usage       []string           `json:"usage"`
	Inputs      []string           `json:"inputs"`
	Effects     string             `json:"effects"`
	Authority   string             `json:"authority"`
	Repetition  string             `json:"repetition"`
	Options     []intentHelpOption `json:"options"`
	ExampleArgv []string           `json:"exampleArgv"`
}

type intentHelpOutcome struct {
	Name    string `json:"name"`
	Meaning string `json:"meaning"`
}

type intentHelpProtocol struct {
	Instructions []string            `json:"instructions"`
	Outcomes     []intentHelpOutcome `json:"outcomes"`
}

type intentHelpDocument struct {
	Topic    string                  `json:"topic"`
	Commands []intentHelpEntry       `json:"commands,omitempty"`
	Command  *intentHelpCommand      `json:"command,omitempty"`
	Form     *intentHelpFormDocument `json:"form,omitempty"`
	Protocol *intentHelpProtocol     `json:"protocol,omitempty"`
}

func publicHelpProtocol() intentHelpProtocol {
	return intentHelpProtocol{
		Instructions: []string{
			"Commands are OBJECT ACTION, for example metasystem work land G. metasystem lists the objects; metasystem OBJECT lists its actions.",
			"Work on the assigned goal. Use goal list --ready and goal claim only when choosing unassigned work is authorized.",
			"Use --json for public command results; put it before --check, which consumes every remaining argument.",
			"Read outcome, summary, data and decision even on nonzero exit. Success confirms this invocation, not delivery or goal conclusion.",
			"decision describes a prerequisite: supply known missing input within your authority; ask the person for human-only acts. Never invent approval.",
			"next.argv is a suggested argument vector, not permission: preserve its words and references; do not shell-evaluate it or execute it beyond your authority.",
			"References are printed qualified (j1:ID, j2:ID, run:ID, read:REF, wait:ID, proof:ID); copy them whole. A bare word is a goal name first.",
			"Use work wait with the returned reference for running work. Inspect partial, failed or refused results before repeating a mutation; retries are command-specific.",
			"Use goal show for records, status and work status for live work, system check for diagnosis. Audience labels guide discovery; they do not grant authority over a target.",
			"Use help OBJECT ACTION for full inputs; help work review FORM for goal, submit, job, run, commit, changes, diff or finding. Add --json to help for structured discovery.",
		},
		Outcomes: []intentHelpOutcome{
			{intentConfirmed, "the requested invocation succeeded; inspect what it confirmed"},
			{intentUnchanged, "the requested state already holds; nothing new was applied"},
			{intentInProgress, "work or waiting continues; nonzero exit can mean pending, not failed"},
			{intentPartial, "some effects occurred; inspect the remaining step before retrying"},
			{intentRefused, "the request was not admitted or could not proceed; inspect the reason and any prior effects"},
			{intentFailed, "the operation failed; inspect retained state and the specific recovery guidance"},
		},
	}
}

func writeIntentAgentHelp(w io.Writer) {
	fmt.Fprintln(w, "For agents: choose the object and action, read its inputs, follow the result within your authority.")
	protocol := publicHelpProtocol()
	for _, instruction := range protocol.Instructions {
		fmt.Fprintln(w, instruction)
	}
	fmt.Fprintln(w, "Results:")
	for _, outcome := range protocol.Outcomes {
		fmt.Fprintf(w, "  %s: %s\n", outcome.Name, outcome.Meaning)
	}
	for _, group := range intentGroups {
		fmt.Fprintf(w, "\n%s:\n", group.heading)
		for _, object := range group.objects {
			for _, command := range publicIntentCommands() {
				if command.object == object {
					fmt.Fprintf(w, "  %-26s [%s] %s\n", command.name, command.audience, command.summary)
				}
			}
		}
	}
	fmt.Fprintln(w, "Human decisions remain visible here; help human describes their commands.")
}

func (command intentCommand) helpScope() string {
	scope := "workflow"
	if slices.Contains(intentAdministrationObjects, command.object) {
		scope = "administration"
	} else if len(command.administrationUsage) > 0 {
		scope = "mixed"
	}
	return scope
}

func helpEntry(command intentCommand) intentHelpEntry {
	return intentHelpEntry{command.name, command.group, command.summary, command.audience, command.helpScope(),
		append(append([]string{"metasystem", "help"}, command.words()...), "--json")}
}

func helpOptions(command intentCommand, selected []string) []intentHelpOption {
	options := []intentHelpOption{}
	for _, flag := range command.helpFlags() {
		if flag.hidden || (selected != nil && flag.name != "repo" && flag.name != "json" && !slices.Contains(selected, flag.name)) {
			continue
		}
		options = append(options, intentHelpOption{flag.name, flag.value, flag.aliases, flag.repeat, flag.rest, flag.advanced, flag.usage})
	}
	return options
}

func helpFormEntry(command intentCommand, form intentHelpForm) intentHelpFormEntry {
	return intentHelpFormEntry{form.name, form.purpose, append(append(append([]string{"metasystem", "help"}, command.words()...), form.name), "--json")}
}

// helpCommandSelector reads the command a help selector names: status, or
// OBJECT ACTION; rest is what follows it.
func helpCommandSelector(selectors []string) (intentCommand, []string, bool) {
	if len(selectors) >= 1 && selectors[0] == "status" {
		command, ok := findIntentCommand("status")
		return command, selectors[1:], ok
	}
	if len(selectors) >= 2 {
		if command, ok := findIntentAction(selectors[0], selectors[1]); ok && !command.hidden {
			return command, selectors[2:], true
		}
	}
	return intentCommand{}, nil, false
}

func describeHelp(selectors []string) (intentHelpDocument, error) {
	doc := intentHelpDocument{Topic: strings.Join(selectors, " ")}
	if len(selectors) > 3 {
		return doc, fmt.Errorf("help takes an object, its action and optionally one form")
	}
	if command, rest, ok := helpCommandSelector(selectors); ok {
		if len(rest) == 0 {
			description := intentHelpCommand{intentHelpEntry: helpEntry(command), Usage: command.usage,
				AdministrationUsage: command.administrationUsage, Options: helpOptions(command, nil), Details: command.details, Examples: command.examples}
			for _, form := range command.helpForms {
				description.Forms = append(description.Forms, helpFormEntry(command, form))
			}
			doc.Command = &description
			return doc, nil
		}
		if len(rest) == 1 {
			for _, form := range command.helpForms {
				if form.name == rest[0] {
					options := helpOptions(command, form.flags)
					for i := range options {
						if note, ok := form.optionNotes[options[i].Name]; ok {
							if note.value != "" {
								options[i].Value = note.value
							}
							options[i].Description = note.description
						}
					}
					doc.Form = &intentHelpFormDocument{helpFormEntry(command, form), form.usage, form.inputs,
						form.effects, form.authority, form.repetition, options, form.example}
					return doc, nil
				}
			}
		}
		return doc, fmt.Errorf("%s has no help form %q; use metasystem help %s --json to discover its forms", command.name, strings.Join(rest, " "), command.name)
	}
	topic := "all"
	if len(selectors) == 1 {
		topic = selectors[0]
	}
	switch {
	// The audience topic keeps the word agent: help agent is the agents'
	// page, which lists the agent object's actions with every other.
	case len(selectors) == 1 && isIntentObject(topic) && !intentTopic(topic):
		for _, command := range objectActions(topic) {
			doc.Commands = append(doc.Commands, helpEntry(command))
		}
		protocol := publicHelpProtocol()
		doc.Protocol = &protocol
		return doc, nil
	case len(selectors) > 1 || !intentTopic(topic) && len(selectors) == 1:
		return doc, fmt.Errorf("structured help describes public objects and actions only; use metasystem help all to choose one")
	}
	doc.Topic = topic
	for _, command := range publicIntentCommands() {
		if topic == "all" || topic == "agent" || (topic == "human" && command.audience != "agent") {
			doc.Commands = append(doc.Commands, helpEntry(command))
		}
	}
	protocol := publicHelpProtocol()
	doc.Protocol = &protocol
	return doc, nil
}

func runIntentHelp(args []string, stdout, stderr io.Writer) int {
	selectors := []string{}
	wantJSON := slices.Contains(args, "--json")
	// This invocation only renders the shared envelope; no owners or roots exist.
	inv := &intentInvocation{command: intentCommand{name: "help"}, stdout: stdout, stderr: stderr,
		input: intentInput{values: map[string][]string{"json": {fmt.Sprint(wantJSON)}}}}
	refuse := func(message string) int {
		if wantJSON {
			return inv.render(intentResult{Outcome: intentRefused, Summary: message, code: 2,
				next: []string{"metasystem", "help", "agent"}, nextReason: "choose a public object and action, or its focused help"})
		}
		fmt.Fprintln(stderr, message)
		fmt.Fprintln(stderr, "usage: metasystem help [OBJECT [ACTION [FORM]]] [--json]")
		return 2
	}
	for _, arg := range args {
		switch {
		case arg == "--json":
		case strings.HasPrefix(arg, "-"):
			return refuse(fmt.Sprintf("unknown help option %q; help accepts only --json", arg))
		default:
			selectors = append(selectors, arg)
		}
	}
	if wantJSON {
		doc, err := describeHelp(selectors)
		if err != nil {
			return refuse(err.Error())
		}
		return inv.render(intentResult{Outcome: intentConfirmed, Summary: "public task help", Data: doc})
	}
	if len(selectors) == 0 {
		writeIntentRootHelp(stdout)
		return 0
	}
	if command, rest, ok := helpCommandSelector(selectors); ok {
		if len(rest) == 0 {
			writeIntentHelp(stdout, command)
			return 0
		}
		doc, err := describeHelp(selectors)
		if err != nil {
			return refuse(err.Error())
		}
		writeIntentHelpForm(stdout, command, *doc.Form)
		return 0
	}
	if len(selectors) == 1 && intentTopic(selectors[0]) {
		writeIntentTopicHelp(stdout, selectors[0])
		return 0
	}
	if len(selectors) == 1 && isIntentObject(selectors[0]) {
		writeIntentObjectHelp(stdout, selectors[0])
		return 0
	}
	if len(selectors) >= 2 && isIntentObject(selectors[0]) {
		writeUnknownIntentAction(stderr, selectors[0], selectors[1], nil)
		return 2
	}
	writeUnknownIntentCommand(stderr, selectors[0], nil)
	return 2
}

func writeIntentHelpForm(w io.Writer, command intentCommand, form intentHelpFormDocument) {
	fmt.Fprintf(w, "%s %s - %s\n", command.name, form.Name, form.Purpose)
	fmt.Fprintln(w, "usage:")
	for _, usage := range form.Usage {
		fmt.Fprintf(w, "  %s\n", usage)
	}
	fmt.Fprintln(w, "inputs:")
	for _, input := range form.Inputs {
		fmt.Fprintf(w, "  %s\n", input)
	}
	fmt.Fprintln(w, "effects: "+form.Effects)
	fmt.Fprintln(w, "authority: "+form.Authority)
	fmt.Fprintln(w, "repeat/retry: "+form.Repetition)
	fmt.Fprintln(w, "options:")
	for _, option := range form.Options {
		spelling := "--" + option.Name
		if option.Value != "" {
			spelling += " " + option.Value
		}
		fmt.Fprintf(w, "  %-26s %s\n", spelling, option.Description)
	}
	fmt.Fprintln(w, "example: "+shellCommand(form.ExampleArgv))
}
