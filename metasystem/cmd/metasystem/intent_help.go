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
	HelpArgv []string `json:"helpArgv"`
}

type intentHelpFormEntry struct {
	Name     string   `json:"name"`
	Purpose  string   `json:"purpose"`
	HelpArgv []string `json:"helpArgv"`
}

type intentHelpCommand struct {
	intentHelpEntry
	Usage    []string              `json:"usage"`
	Options  []intentHelpOption    `json:"options"`
	Details  []string              `json:"details,omitempty"`
	Examples []string              `json:"examples,omitempty"`
	Forms    []intentHelpFormEntry `json:"forms,omitempty"`
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
			"Work on the assigned goal. Use goals --ready and claim only when choosing unassigned work is authorized.",
			"Use --json for public command results; put it before --check, which consumes every remaining argument.",
			"Read outcome, summary, data and decision even on nonzero exit. Success confirms this invocation, not delivery or goal conclusion.",
			"decision describes a prerequisite: supply known missing input within your authority; ask the person for human-only acts. Never invent approval.",
			"next.argv is a suggested argument vector, not permission: preserve its words and references; do not shell-evaluate it or execute it beyond your authority.",
			"Use wait with the returned reference for running work. Inspect partial, failed or refused results before repeating a mutation; retries are command-specific.",
			"Use show for records, status for live work, check for diagnosis. Audience labels guide discovery; they do not grant authority over a target.",
			"Use help COMMAND for full inputs; help review FORM for goal, submit, design, job, commit, changes, diff or finding. Add --json to help for structured discovery.",
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
	fmt.Fprintln(w, "For agents: choose the task, read its inputs, follow the result within your authority.")
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
		for _, command := range publicIntentCommands() {
			if command.group == group.name {
				fmt.Fprintf(w, "  %-12s [%s] %s\n", command.name, command.audience, command.summary)
			}
		}
	}
	fmt.Fprintln(w, "Human decisions remain visible here; help human describes their commands.")
}

func helpEntry(command intentCommand) intentHelpEntry {
	return intentHelpEntry{command.name, command.group, command.summary, command.audience,
		[]string{"metasystem", "help", command.name, "--json"}}
}

func helpOptions(command intentCommand, selected []string) []intentHelpOption {
	options := []intentHelpOption{}
	for _, flag := range command.allFlags() {
		if flag.hidden || (selected != nil && flag.name != "repo" && flag.name != "json" && !slices.Contains(selected, flag.name)) {
			continue
		}
		options = append(options, intentHelpOption{flag.name, flag.value, flag.aliases, flag.repeat, flag.rest, flag.advanced, flag.usage})
	}
	return options
}

func helpFormEntry(command intentCommand, form intentHelpForm) intentHelpFormEntry {
	return intentHelpFormEntry{form.name, form.purpose, []string{"metasystem", "help", command.name, form.name, "--json"}}
}

func describeHelp(selectors []string) (intentHelpDocument, error) {
	doc := intentHelpDocument{Topic: strings.Join(selectors, " ")}
	if len(selectors) > 2 {
		return doc, fmt.Errorf("help takes one topic or command, optionally followed by its form")
	}
	if len(selectors) > 0 {
		if command, ok := findIntentCommand(selectors[0]); ok && !command.compatibility {
			if len(selectors) == 1 {
				description := intentHelpCommand{intentHelpEntry: helpEntry(command), Usage: command.usage,
					Options: helpOptions(command, nil), Details: command.details, Examples: command.examples}
				for _, form := range command.helpForms {
					description.Forms = append(description.Forms, helpFormEntry(command, form))
				}
				doc.Command = &description
				return doc, nil
			}
			for _, form := range command.helpForms {
				if form.name == selectors[1] {
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
			return doc, fmt.Errorf("%s has no help form %q; use metasystem help %s --json to discover its forms", command.name, selectors[1], command.name)
		}
	}
	topic := "all"
	if len(selectors) == 1 {
		topic = selectors[0]
	}
	if len(selectors) == 2 || topic == "internal" || !intentTopic(topic) {
		return doc, fmt.Errorf("structured and focused help describe public tasks only; use metasystem help all to choose a command")
	}
	doc.Topic = topic
	for _, command := range publicIntentCommands() {
		if topic == "all" || topic == "agent" || (topic == "human" && command.audience != "agent") || command.group == topic {
			doc.Commands = append(doc.Commands, helpEntry(command))
		}
	}
	protocol := publicHelpProtocol()
	doc.Protocol = &protocol
	return doc, nil
}

func runIntentHelp(args []string, stdout, stderr io.Writer, registered []family) int {
	selectors := []string{}
	wantJSON := slices.Contains(args, "--json")
	// This invocation only renders the shared envelope; no owners or roots exist.
	inv := &intentInvocation{command: intentCommand{name: "help"}, stdout: stdout, stderr: stderr,
		input: intentInput{values: map[string][]string{"json": {fmt.Sprint(wantJSON)}}}}
	refuse := func(message string) int {
		if wantJSON {
			return inv.render(intentResult{Outcome: intentRefused, Summary: message, code: 2,
				next: []string{"metasystem", "help", "agent"}, nextReason: "choose a public task or its focused help"})
		}
		fmt.Fprintln(stderr, message)
		fmt.Fprintln(stderr, "usage: metasystem help [TOPIC|COMMAND] [FORM] [--json]")
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
	if wantJSON || len(selectors) > 1 {
		doc, err := describeHelp(selectors)
		if err != nil {
			return refuse(err.Error())
		}
		if wantJSON {
			return inv.render(intentResult{Outcome: intentConfirmed, Summary: "public task help", Data: doc})
		}
		writeIntentHelpForm(stdout, *doc.Form)
		return 0
	}
	if len(selectors) == 0 {
		writeIntentRootHelp(stdout)
		return 0
	}
	name := selectors[0]
	if command, ok := findIntentCommand(name); ok {
		writeIntentHelp(stdout, command)
		return 0
	}
	if intentTopic(name) {
		writeIntentTopicHelp(stdout, name, registered)
		return 0
	}
	for _, fam := range registered {
		if fam.name == name {
			writeFamilyHelp(stdout, fam)
			return 0
		}
	}
	writeUnknownIntentCommand(stderr, name)
	return 2
}

func writeIntentHelpForm(w io.Writer, form intentHelpFormDocument) {
	fmt.Fprintf(w, "review %s - %s\n", form.Name, form.Purpose)
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
